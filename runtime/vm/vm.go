package vm

import (
	"fmt"
	"go_js/allocator"
	"go_js/chunk"
	"go_js/compiler"
	"go_js/eventloop"
	"go_js/native"
	"go_js/object"
	"go_js/queue"
	"go_js/stringer"
	"go_js/value"
	"math"
	"sync"
	"time"
)

const STACK_MAX = math.MaxUint8
const FRAMES_MAX = 64

var globals []value.Value
var heapVars = make(map[int][]value.Value)
var heapScopesCount int

type CallFrame struct {
	fn         object.Callable
	thisCtx    value.Value
	localStart int
	returnIp   int
}

func (cf *CallFrame) initCallFrame(fn object.Callable, localStart int, returnIp int, this value.Value) {
	cf.fn = fn
	cf.localStart = localStart
	cf.returnIp = returnIp
	cf.thisCtx = this
}

type VM struct {
	frames         []CallFrame
	frameCount     int
	stack          []value.Value
	stackTop       int
	exceptionStack []int

	debug bool
}

func NewVM(debug bool) *VM {
	frames := make([]CallFrame, FRAMES_MAX)
	stack := make([]value.Value, STACK_MAX)
	return &VM{frames: frames, frameCount: 0, stack: stack, stackTop: 0, exceptionStack: []int{}, debug: debug}
}

func (vm *VM) Call(fn object.Callable, returnIp int, this value.Value) error {
	if vm.frameCount == FRAMES_MAX {
		return fmt.Errorf("too many callframes")
	}

	localStart := max(vm.stackTop-fn.GetArity(), 0)

	vm.frames[vm.frameCount].initCallFrame(fn, localStart, returnIp, this)
	vm.frameCount++
	return nil
}

func (vm *VM) currentFrame() CallFrame {
	return vm.frames[vm.frameCount-1]
}

func (vm *VM) push(v value.Value) {
	vm.stack[vm.stackTop] = v
	vm.stackTop++
}

func (vm *VM) peek() value.Value {
	return vm.stack[vm.stackTop-1]
}

func (vm *VM) pop() value.Value {
	vm.stackTop--
	return vm.stack[vm.stackTop]
}

func (vm *VM) addGlobal(v value.Value) {
	globals = append(globals, v)
}

func (vm *VM) getGlobal(global int) value.Value {
	return globals[global]
}

func (vm *VM) concatenate(a, b value.Value) value.Value {
	if a.IsObject() && b.IsObject() {
		aObj, _ := allocator.GetObject(a.GetHandle())
		bObj, _ := allocator.GetObject(b.GetHandle())

		if aObj.Type() == object.OBJ_STRING && bObj.Type() == object.OBJ_STRING {
			return value.EncodeHandle(allocator.Allocate(native.LightString((aObj.String() + bObj.String()))))
		} else {
			// runtime error?
		}
	}

	if a.IsObject() && !b.IsObject() {
		aObj, _ := allocator.GetObject(a.GetHandle())

		if aObj.Type() == object.OBJ_STRING {
			res := aObj.String() + stringer.String(b)

			return value.EncodeHandle(allocator.Allocate(native.LightString(res)))
		}
	} else if b.IsObject() {
		bObj, _ := allocator.GetObject(b.GetHandle())

		if bObj.Type() == object.OBJ_STRING {
			return value.EncodeHandle(allocator.Allocate(native.LightString(stringer.String(a) + bObj.String())))
		}
	}

	return value.Value(math.Float64bits(a.AsNumber() + b.AsNumber()))
}

func (vm *VM) subtract(a, b value.Value) value.Value {
	aIsObject, _ := object.IsValueObject(a)
	bIsObject, _ := object.IsValueObject(b)
	if aIsObject || bIsObject {
		if aIsObject && bIsObject {
			// todo
		} else {
			return value.EncodeNaN()
		}
	}

	return value.ValueFromFloat64(a.AsNumber() - b.AsNumber())
}

func (vm *VM) popN(n int) []value.Value {
	r := make([]value.Value, n)
	copy(r, vm.stack[vm.stackTop-n:vm.stackTop])
	vm.stackTop -= n
	return r
}

func (vm *VM) CreateTemplateString(o *object.ObjTemplateLiteral) value.Value {
	objStr := native.NewObjString(o.CreateString())
	return value.EncodeHandle(allocator.Allocate(objStr))
}

func (vm *VM) peekN(i int) value.Value {
	return vm.stack[vm.stackTop-(i+1)]
}

func (vm *VM) Run(wg *sync.WaitGroup) {
	wg.Add(1)
Run:
	fn := queue.Dequeue()
	for fn != nil {

		vm.Call(fn, 0, value.EncodedUndefined())
		vm.run()

		fn = queue.Dequeue()
	}

	if vm.debug {
		fmt.Println()
		fmt.Println("-- QUEUE DRAINED --")
	}

	tick := time.NewTicker(100 * time.Millisecond)
	hasWork := eventloop.HasWork()
	for hasWork {
		if vm.debug {
			fmt.Printf("eventloop has work: %v\n", hasWork)
		}
		select {
		case <-queue.QueueC:
			{
				goto Run
			}
		case <-tick.C:
			{
				hasWork = eventloop.HasWork()
				continue
			}
		}
	}
	wg.Done()
}

func (vm *VM) run() (value.Value, error) {
	frame := vm.currentFrame()
	valueChunk := *frame.fn.ValueChunk()
	ip := 0

	var argCount uint8

	if promise, ok := frame.fn.(*native.ObjAsyncFunction); ok {

		if promise.State != nil {
			if vm.debug {
				fmt.Println()
				fmt.Printf("-- RETURNING PROMISE %v --\n", promise.String())
				printStack(promise.State.Stack)
				fmt.Println()
			}
			for _, v := range promise.State.Stack {
				vm.push(v)
			}

			ip = promise.State.Ip
		}
	}

	if vm.debug {
		fmt.Println()
		fmt.Println("-- NEW RUNNER SPAWNED --")
		fmt.Println()
		PrintChunk(valueChunk)
	}

	for {
		// time.Sleep(time.Millisecond * 100)
		code := valueChunk.Code[ip]
		ip++

		if vm.debug {
			fmt.Println(frame.fn.String())
			printStack(vm.stack[0:vm.stackTop])
			fmt.Println(opNames[code])
		}

		switch code {
		case chunk.OP_CONSTANT:
			{
				vm.push(valueChunk.Constants[valueChunk.Code[ip]])
				ip++
			}
		case chunk.OP_POP:
			{
				vm.pop()
			}
		case chunk.OP_PUSH_CURRENT:
			{
				vm.push(vm.peek())
			}
		case chunk.OP_ADD:
			{
				b := vm.pop()
				a := vm.pop()

				vm.push(vm.concatenate(a, b))
			}
		case chunk.OP_SUBTRACT:
			{
				b := vm.pop()
				a := vm.pop()

				vm.push(vm.subtract(a, b))
			}
		case chunk.OP_DIVIDE:
			{
				b := vm.pop()
				a := vm.pop()
				// type checks required
				vm.push(value.ValueFromFloat64(a.AsNumber() / b.AsNumber()))
			}
		case chunk.OP_MULTIPLY:
			{
				b := vm.pop()
				a := vm.pop()
				// type checks required
				vm.push(value.ValueFromFloat64(a.AsNumber() * b.AsNumber()))
			}
		case chunk.OP_EXPONENTIATION:
			{

				b := vm.pop()
				a := vm.pop()
				// type checks required
				vm.push(value.ValueFromFloat64(math.Pow(a.AsNumber(), b.AsNumber())))
			}
		case chunk.OP_MODULO:
			{
				b := vm.pop()
				a := vm.pop()
				// type checks required
				vm.push(value.ValueFromFloat64(math.Mod(a.AsNumber(), b.AsNumber())))
			}
		case chunk.OP_LESS_THAN:
			{
				b := vm.pop()
				a := vm.pop()

				if a.AsNumber() < b.AsNumber() {
					vm.push(value.EncodeTrue())
				} else {
					vm.push(value.EncodeFalse())
				}
			}
		case chunk.OP_LESS_THAN_EQUAL:
			{
				b := vm.pop()
				a := vm.pop()

				if a.AsNumber() <= b.AsNumber() {
					vm.push(value.EncodeTrue())
				} else {
					vm.push(value.EncodeFalse())
				}
			}
		case chunk.OP_GREATER_THAN:
			{
				b := vm.pop()
				a := vm.pop()

				if a.AsNumber() > b.AsNumber() {
					vm.push(value.EncodeTrue())
				} else {
					vm.push(value.EncodeFalse())
				}
			}
		case chunk.OP_GREATER_THAN_EQUAL:
			{
				b := vm.pop()
				a := vm.pop()

				if a.AsNumber() >= b.AsNumber() {
					vm.push(value.EncodeTrue())
				} else {
					vm.push(value.EncodeFalse())
				}
			}
		case chunk.OP_EQUALS:
			{
				b := vm.pop()
				a := vm.pop()

				if stringer.String(a) == stringer.String(b) {
					vm.push(value.EncodeTrue())
				} else {
					vm.push(value.EncodeFalse())
				}
			}
		case chunk.OP_STRICT_EQUALS:
			{
				b := vm.pop()
				a := vm.pop()

				bIsObject, bHandle := object.IsValueObject(b)
				aIsObject, aHandle := object.IsValueObject(a)

				if bIsObject && aIsObject {
					bObj, _ := allocator.GetObject(bHandle)
					aObj, _ := allocator.GetObject(aHandle)

					if bObj.Type() == aObj.Type() {
						if bObj.String() == aObj.String() {
							vm.push(value.EncodeTrue())
						} else {
							vm.push(value.EncodeFalse())
						}

					} else {
						vm.push(value.EncodeFalse())
					}
					continue
				}

				if a == b {
					vm.push(value.EncodeTrue())
				} else {
					vm.push(value.EncodeFalse())
				}
			}
		case chunk.OP_STRICT_NOT_EQUALS:
			{
				b := vm.pop()
				a := vm.pop()

				bIsObject, bHandle := object.IsValueObject(b)
				aIsObject, aHandle := object.IsValueObject(a)

				if bIsObject && aIsObject {
					bObj, _ := allocator.GetObject(bHandle)
					aObj, _ := allocator.GetObject(aHandle)

					if bObj.Type() == aObj.Type() {

					} else {
						vm.push(value.EncodeTrue())
					}
				}

				if a == b {
					vm.push(value.EncodeFalse())
				} else {
					vm.push(value.EncodeTrue())
				}
			}
		case chunk.OP_LOGICAL_OR:
			{
				right := vm.pop()
				left := vm.pop()

				if left.AsBoolean() {
					vm.push(left)
				} else {
					vm.push(right)
				}
			}
		case chunk.OP_LOGICAL_AND:
			{
				right := vm.pop()
				left := vm.pop()

				if left.AsBoolean() && right.AsBoolean() {
					vm.push(value.EncodeTrue())
				} else {
					vm.push(value.EncodeFalse())
				}
			}
		case chunk.OP_NEGATE:
			{
				v := vm.pop()

				if v.IsNumber() {
					vm.push(value.ValueFromFloat64(-(v.AsNumber())))
				} else {
					return value.EncodedUndefined(), fmt.Errorf("no support for negating %s yet", stringer.String(v))
				}
			}
		case chunk.OP_JUMP_IF_FALSE:
			{
				v := vm.pop()
				jump := int(valueChunk.Code[ip+3]) | int(valueChunk.Code[ip+2])<<8 | int(valueChunk.Code[ip+1])<<16 | int(valueChunk.Code[ip])<<24

				if !v.AsBoolean() {
					ip = jump
				} else {
					ip += 4
				}
			}
		case chunk.OP_JUMP_IF_TRUE:
			{
				v := vm.pop()
				jump := int(valueChunk.Code[ip+3]) | int(valueChunk.Code[ip+2])<<8 | int(valueChunk.Code[ip+1])<<16 | int(valueChunk.Code[ip])<<24

				if v.AsBoolean() {
					ip = jump
				} else {
					ip += 4
				}
			}
		case chunk.OP_JUMP:
			{
				jump := int(valueChunk.Code[ip+3]) | int(valueChunk.Code[ip+2])<<8 | int(valueChunk.Code[ip+1])<<16 | int(valueChunk.Code[ip])<<24
				ip = jump
			}
		case chunk.OP_DEFINE_HEAP_VAR:
			{
				variable := vm.pop()
				if scope, found := heapVars[frame.fn.GetHeapScope()]; found {
					scope = append(scope, variable)
					heapVars[frame.fn.GetHeapScope()] = scope
				} else {
					panic("no heap scope generated for function")
				}
			}
		case chunk.OP_GET_HEAP_VAR:
			{
				heapVar := valueChunk.Code[ip]
				ip++
				vm.push(heapVars[frame.fn.GetHeapScope()][heapVar])
			}
		case chunk.OP_SET_HEAP_VAR:
			{
				heapVar := valueChunk.Code[ip]
				ip++
				heapVars[frame.fn.GetHeapScope()][heapVar] = vm.pop()
			}
		case chunk.OP_DEFINE_GLOBAL:
			{
				v := vm.pop()
				if v.IsObject() {
					obj, _ := allocator.GetObject(v.GetHandle())
					// check if we have a closure in hand
					if fn, ok := obj.(*object.ObjFunction); ok && fn.GetHeapScope() != object.NOT_IN_HEAP_SCOPE {
						v = value.EncodeHandle(allocator.Allocate(fn.Clone()))
					}
				}
				vm.addGlobal(v)
			}
		case chunk.OP_GET_GLOBAL:
			{
				global := valueChunk.Code[ip]
				vm.push(vm.getGlobal(int(global)))
				ip++
			}
		case chunk.OP_SET_GLOBAL:
			{
				global := valueChunk.Code[ip]
				v := vm.pop()
				if v.IsObject() {
					obj, _ := allocator.GetObject(v.GetHandle())
					// check if we have a closure in hand
					if fn, ok := obj.(*object.ObjFunction); ok && fn.GetHeapScope() != object.NOT_IN_HEAP_SCOPE {
						v = value.EncodeHandle(allocator.Allocate(fn.Clone()))
					}
				}
				globals[global] = v
				ip++
			}
		case chunk.OP_POP_LOCAL:
			{
				vm.stackTop--
			}
		case chunk.OP_DEFINE_LOCAL:
			{
				v := vm.pop()

				if v.IsObject() {
					handle := v.GetHandle()
					if obj, err := allocator.GetObject(handle); err == nil {
						if promise, ok := obj.(*native.ObjPromise); ok {
							vm.push(promise.Value)
							vm.pop()
						}
					}
				}
				vm.stackTop++
			}
		case chunk.OP_GET_LOCAL:
			{
				vm.push(vm.stack[frame.localStart+int(valueChunk.Code[ip])])
				ip++
			}
		case chunk.OP_SET_LOCAL:
			{
				vm.stack[frame.localStart+int(valueChunk.Code[ip])] = vm.pop()
				ip++
			}
		case chunk.OP_CREATE_OBJECT:
			{
				objHash := native.NewObjectHash()
				handle := allocator.Allocate(objHash)

				vm.push(value.EncodeHandle(handle))
			}
		case chunk.OP_SET_OBJECT_MEMBER:
			{
				v := vm.pop()
				k := vm.pop()

				hash := vm.peek()

				isObject, handle := object.IsValueObject(hash)

				if !isObject {
					return value.EncodedUndefined(), fmt.Errorf("%v is not an object", hash)
				}

				if v.IsObject() {
					obj, _ := allocator.GetObject(v.GetHandle())

					// check if we have a closure in hand
					if fn, ok := obj.(*object.ObjFunction); ok && fn.GetHeapScope() != object.NOT_IN_HEAP_SCOPE {
						v = value.EncodeHandle(allocator.Allocate(fn.Clone()))
					}
				}

				obj, _ := allocator.GetObject(handle)

				if obj, ok := obj.(object.Hashable); ok {
					obj.SetMember(k, v)
				} else {
					return value.EncodedUndefined(), fmt.Errorf("we currently don't support adding properties to %s types", stringer.String(hash))
				}
			}
		case chunk.OP_GET_OBJECT_MEMBER:
			{
				member := vm.pop()
				hash := vm.pop()
				objObject, err := allocator.GetObject(hash.GetHandle())

				if err != nil {
					return value.EncodedUndefined(), err
				}

				if arr, ok := objObject.(*native.ObjArr); ok && member.IsInteger() {
					value := arr.GetElementAt(int(member.AsNumber()))
					vm.push(value)
					continue
				}
				if str, ok := objObject.(native.LightString); ok {

					boxed := native.NewObjString(str.String())
					value := boxed.GetMember(member)

					if value.IsObject() {
						member, err := allocator.GetObject(value.GetHandle())

						if err != nil {
							continue
						}

						if _, ok := member.(native.Instancer); ok {
							value = native.NewMethodHandle(boxed, member)
						}
					}
					vm.push(value)
					continue
				}

				if object, ok := objObject.(object.Hashable); ok {
					value := object.GetMember(member)

					if value.IsObject() {
						member, err := allocator.GetObject(value.GetHandle())

						if err != nil {
							continue
						}

						if _, ok := member.(native.Instancer); ok {
							value = native.NewMethodHandle(objObject, member)
						}
					}
					vm.push(value)
				} else {
					return value.EncodedUndefined(), fmt.Errorf("cant get property: %s from: %v", stringer.String(member), hash)
				}
			}
		case chunk.OP_PUSH_UNDEFINED:
			{
				vm.push(value.EncodedUndefined())
			}
		case chunk.OP_CALL:
			{
				callee := vm.pop()

				if callee.IsObject() {
					callee, err := allocator.GetObject(callee.GetHandle())

					if err != nil {
						return value.EncodedUndefined(), err
					}

					switch fn := callee.(type) {
					// MethodHandle means we have an instance method at hand
					case *native.MethodHandle:
						{
							thisCtx := fn.ThisContext
							callee := fn.Function

							switch method := callee.(type) {
							case *native.Method:
								{
									this := value.EncodeHandle(allocator.Allocate(thisCtx))
									vm.Call(method.Fn, ip, this)
									frame = vm.currentFrame()
									valueChunk = *frame.fn.ValueChunk()
									ip = 0
									frame.localStart -= int(argCount)
								}
							case *native.ArrayPush:
								{
									arg := vm.pop()
									vm.push(method.Push(thisCtx.(*native.ObjArr), arg))
								}
							case *native.ArrayForEach:
								{
									callback := vm.pop()
									iterator := object.NewValueIterator(thisCtx.(object.Iterable))
									done := iterator.Next()

									obj, err := allocator.GetObject(callback.GetHandle())

									if err != nil {
										return value.EncodedUndefined(), err
									}
									runner := NewVM(vm.debug)
									if fn, ok := obj.(*object.ObjFunction); ok {
										for !done {

											item := iterator.Current()
											runner.push(item)
											runner.Call(fn, 0, value.EncodedUndefined())
											runner.run()
											done = iterator.Next()
										}
									} else {
										return value.EncodedUndefined(), fmt.Errorf("callback was not a function %s", stringer.String(callback))
									}

									vm.push(value.EncodedUndefined())
								}
							case *native.ArrayFilter:
								{
									callback := vm.pop()
									iterator := object.NewValueIterator(thisCtx.(object.Iterable))
									done := iterator.Next()

									obj, err := allocator.GetObject(callback.GetHandle())

									if err != nil {
										return value.EncodedUndefined(), err
									}

									arr := []value.Value{}
									runner := NewVM(vm.debug)
									if fn, ok := obj.(*object.ObjFunction); ok {
										for !done {

											item := iterator.Current()
											runner.push(item)
											runner.Call(fn, 0, value.EncodedUndefined())
											result, err := runner.run()

											if err != nil {
												return value.EncodedUndefined(), err
											}

											if result.AsBoolean() {
												arr = append(arr, item)
											}

											done = iterator.Next()
										}
									} else {
										return value.EncodedUndefined(), fmt.Errorf("callback was not a function %s", stringer.String(callback))
									}
									length := len(arr)
									objArr := native.NewArray(length)

									for _, item := range arr {
										objArr.PushElement(item)
									}

									v := value.EncodeHandle(allocator.Allocate(objArr))
									vm.push(v)
								}
							case *native.ArrayMap:
								{
									callback := vm.pop()
									iterator := object.NewValueIterator(thisCtx.(object.Iterable))
									done := iterator.Next()

									obj, err := allocator.GetObject(callback.GetHandle())

									if err != nil {
										return value.EncodedUndefined(), err
									}

									arr := []value.Value{}
									runner := NewVM(vm.debug)
									if fn, ok := obj.(*object.ObjFunction); ok {
										for !done {

											item := iterator.Current()
											runner.push(item)
											runner.Call(fn, 0, value.EncodedUndefined())
											result, err := runner.run()

											if err != nil {
												return value.EncodedUndefined(), err
											}

											arr = append(arr, result)
											done = iterator.Next()
										}
									} else {
										return value.EncodedUndefined(), fmt.Errorf("callback was not a function %s", stringer.String(callback))
									}
									length := len(arr)
									objArr := native.NewArray(length)

									for _, item := range arr {
										objArr.PushElement(item)
									}

									v := value.EncodeHandle(allocator.Allocate(objArr))
									vm.push(v)
								}
							case *native.ArrayReduce:
								{
									initialValue := vm.pop()
									callback := vm.pop()
									iterator := object.NewValueIterator(thisCtx.(object.Iterable))
									done := iterator.Next()

									obj, err := allocator.GetObject(callback.GetHandle())

									if err != nil {
										return value.EncodedUndefined(), err
									}

									runner := NewVM(vm.debug)
									if fn, ok := obj.(*object.ObjFunction); ok {
										for !done {
											item := iterator.Current()
											runner.push(initialValue)
											runner.push(item)
											runner.Call(fn, 0, value.EncodedUndefined())
											result, err := runner.run()

											if err != nil {
												return value.EncodedUndefined(), err
											}

											initialValue = result
											done = iterator.Next()
										}
									} else {
										return value.EncodedUndefined(), fmt.Errorf("callback was not a function %s", stringer.String(callback))
									}
									vm.push(initialValue)
								}
							case *native.StringToUpperCase:
								{
									vm.push(value.EncodeHandle(allocator.Allocate(method.ToUpperCase(thisCtx.(*native.ObjString)))))
								}
							case *native.StringIncludes:
								{
									arg := vm.pop()
									vm.push(method.Includes(thisCtx.(*native.ObjString), stringer.String(arg)))
								}

							case *native.ToString:
								{
									handle := allocator.Allocate(native.NewObjString(method.ToString(thisCtx)))
									vm.push(value.EncodeHandle(handle))
								}
							case *native.Log:
								{
									arg := vm.pop()

									vm.log(arg)
									vm.push(value.EncodedUndefined())
								}
							}
						}
						// Static functions
					case *native.ObjectKeys:
						{
							arg := vm.pop()
							arr := fn.Keys(arg)

							length := len(arr)
							objArr := native.NewArray(length)

							for _, item := range arr {
								objArr.PushElement(item)
							}

							v := value.EncodeHandle(allocator.Allocate(objArr))
							vm.push(v)
						}
					case *native.ObjectValues:
						{
							arg := vm.pop()
							arr := fn.Values(arg)

							length := len(arr)
							objArr := native.NewArray(length)

							for _, item := range arr {
								objArr.PushElement(item)
							}

							v := value.EncodeHandle(allocator.Allocate(objArr))
							vm.push(v)
						}
					case *native.ResolveFunc:
						{
							if vm.stackTop == 0 {
								vm.push(value.EncodedUndefined())
							}

							v := vm.pop()
							fn.Resolve(v)
						}
					case *native.SetTimeout:
						{
							ms := vm.pop().AsNumber()
							callback := vm.pop()

							handle := callback.GetHandle()
							obj, err := allocator.GetObject(handle)

							if err != nil {
								return value.EncodedUndefined(), err
							}

							if callback, ok := obj.(*object.ObjFunction); ok {
								fn.Set(int(ms), callback)
								eventloop.Dispatch(fn.Clone())
								vm.push(value.EncodedUndefined())
							}
						}
					case *native.ObjAsyncFunction:
						{
							if !fn.ReturnArgumentIsPromise {
								promise := native.NewPromise()
								fn.SetPromise(promise)
								handle := allocator.Allocate(promise)
								vm.push(value.EncodeHandle(handle))
							}

							vm.Call(fn.Clone(), ip, value.EncodedUndefined())
							frame = vm.currentFrame()
							valueChunk = *frame.fn.ValueChunk()
							ip = 0
							frame.localStart -= int(argCount)
						}
					case object.Callable:
						{
							vm.Call(fn, ip, value.EncodedUndefined())
							frame = vm.currentFrame()
							valueChunk = *frame.fn.ValueChunk()
							ip = 0
							frame.localStart -= int(argCount)
						}
					case *native.Now:
						{
							vm.push(fn.Now())
						}
					}
				} else {
					return value.EncodedUndefined(), fmt.Errorf("%s is not a function", stringer.String(callee))
				}
			}
		case chunk.OP_RETURN:
			{
				if promise, ok := frame.fn.(*native.ObjAsyncFunction); ok {
					ip = frame.returnIp
					value := value.EncodedUndefined()
					vm.frameCount--

					if vm.stackTop > 0 {
						value = vm.pop()
					}

					promise.Resolve(value)

					if vm.frameCount <= 0 {
						vm.stackTop = 0

						if vm.debug {
							fmt.Println("--")
						}
						return value, nil
					}

					vm.stackTop = frame.localStart
					vm.push(value)

					frame = vm.currentFrame()
					valueChunk = *frame.fn.ValueChunk()
				} else {
					ip = frame.returnIp
					value := value.EncodedUndefined()
					vm.frameCount--

					if vm.stackTop > 0 {
						value = vm.pop()
					}

					if vm.frameCount <= 0 {
						vm.stackTop = 0

						if vm.debug {
							fmt.Println("--")
						}
						return value, nil
					}

					vm.stackTop = frame.localStart
					vm.push(value)

					frame = vm.currentFrame()
					valueChunk = *frame.fn.ValueChunk()
				}
			}
		case chunk.OP_CREATE_ARRAY:
			{
				length := int(valueChunk.Code[ip+3]) | int(valueChunk.Code[ip+2])<<8 | int(valueChunk.Code[ip+1])<<16 | int(valueChunk.Code[ip])<<24
				ip += 4
				arr := native.NewArray(length)
				handle := allocator.Allocate(arr)
				vm.push(value.EncodeHandle(handle))
			}
		case chunk.OP_PUSH_ELEMENT:
			{
				v := vm.pop()
				arr := vm.peek()

				obj, err := allocator.GetObject(arr.GetHandle())

				if err != nil {
					return value.EncodedUndefined(), err
				}

				arrOBj, ok := obj.(*native.ObjArr)

				if !ok {
					return value.EncodedUndefined(), fmt.Errorf("trying to initialize {%s} that is not an array", stringer.String(arr))
				}

				arrOBj.PushElement(v)
			}
		case chunk.OP_GET_ITERATOR:
			{
				iteratee := vm.pop()
				type_ := valueChunk.Code[ip]
				ip++

				if !iteratee.IsObject() {
					return value.EncodedUndefined(), fmt.Errorf("%s is not an object", stringer.String(iteratee))
				}

				obj, err := allocator.GetObject(iteratee.GetHandle())

				if err != nil {
					return value.EncodedUndefined(), err
				}

				iteratorObj, ok := obj.(object.Iterable)

				if !ok {
					return value.EncodedUndefined(), fmt.Errorf("%s is not iterable", stringer.String(iteratee))
				}

				var iterator *object.Iterator

				if type_ == compiler.ITERATOR_FOR_IN {
					iterator = object.NewKeyIterator(iteratorObj)
				} else {
					iterator = object.NewValueIterator(iteratorObj)
				}

				vm.push(value.EncodeHandle(allocator.Allocate(iterator)))
			}
		case chunk.OP_ITERATOR_NEXT:
			{
				iterator := vm.peek()

				if !iterator.IsObject() {
					return value.EncodedUndefined(), fmt.Errorf("%s is not an object", stringer.String(iterator))
				}

				obj, err := allocator.GetObject(iterator.GetHandle())

				if err != nil {
					return value.EncodedUndefined(), err
				}

				iteratorObj, ok := obj.(*object.Iterator)

				if !ok {
					return value.EncodedUndefined(), fmt.Errorf("%s is not iterable", stringer.String(iterator))
				}

				done := iteratorObj.Next()

				if done {
					vm.push(value.EncodeTrue())
				} else {
					vm.push(value.EncodeFalse())
				}
			}
		case chunk.OP_ITERATOR_CURRENT:
			{
				iterator := vm.peek()

				if !iterator.IsObject() {
					return value.EncodedUndefined(), fmt.Errorf("%s is not an object", stringer.String(iterator))
				}

				obj, err := allocator.GetObject(iterator.GetHandle())

				if err != nil {
					return value.EncodedUndefined(), err
				}

				iteratorObj, ok := obj.(*object.Iterator)

				if !ok {
					return value.EncodedUndefined(), fmt.Errorf("%s is not iterable", stringer.String(iterator))
				}

				vm.push(iteratorObj.Current())
			}
		case chunk.OP_CREATE_HEAP_SCOPE:
			{
				heapScopesCount++
				heapVars[heapScopesCount] = []value.Value{}
				setHeapScopes(frame.fn.ValueChunk(), heapScopesCount)
				frame.fn.SetHeapScope(heapScopesCount)
			}
		case chunk.OP_TRY_BLOCK_START:
			{
				catchStart := int(valueChunk.Code[ip+3]) | int(valueChunk.Code[ip+2])<<8 | int(valueChunk.Code[ip+1])<<16 | int(valueChunk.Code[ip])<<24
				ip += 4
				vm.exceptionStack = append(vm.exceptionStack, catchStart)
			}
		case chunk.OP_TRY_BLOCK_END:
			{
				vm.exceptionStack = vm.exceptionStack[:len(vm.exceptionStack)-1]
			}
		case chunk.OP_NEW:
			{
				argCount := valueChunk.Code[ip]
				ip++
				callee := vm.pop()
				obj, err := allocator.GetObject(callee.GetHandle())

				if err != nil {
					return value.EncodedUndefined(), err
				}

				switch ctor := obj.(type) {
				case *native.ObjClass:
					{
						instance := ctor.NewInstance()
						ctor := instance.GetMember(native.KEY_CTOR)

						obj, err := allocator.GetObject(ctor.GetHandle())

						if err != nil {
							return value.EncodedUndefined(), fmt.Errorf("contructor was not an object %s", stringer.String(ctor))
						}

						if constructor, ok := obj.(*native.Method); ok {
							builder := NewVM(vm.debug)
							fn := object.NewFunction("builder", 0, nil)
							fn.ValueChunk().EmitBytes(chunk.OP_CALL, chunk.OP_RETURN)

							for _, v := range vm.popN(constructor.Fn.GetArity()) {
								builder.push(v)
							}

							builder.push(native.NewMethodHandle(instance, constructor))
							builder.Call(fn, 0, value.EncodedUndefined())
							builder.run()

							vm.push(value.EncodeHandle(allocator.Allocate(instance)))
						} else {
							return value.EncodedUndefined(), fmt.Errorf("contructor was not an function %s", stringer.String(ctor))
						}

					}
				case native.Constructor:
					{
						args := vm.popN(int(argCount))
						params := make([]any, argCount)

						for i, v := range args {
							params[i] = v
						}

						newObj, err := ctor.New(params...)

						if err != nil {
							return value.EncodedUndefined(), err
						}

						objHandle := allocator.Allocate(newObj)
						vm.push(value.EncodeHandle(objHandle))
					}
				case *native.PromiseConstructor:
					{
						arg := vm.pop()
						handle := arg.GetHandle()
						executor, err := allocator.GetObject(handle)

						if err != nil {
							return value.EncodedUndefined(), err
						}

						promise := native.NewPromise()
						resolve := native.NewResolveFunc(promise)
						resolveHandle := allocator.Allocate(resolve)

						if executor, ok := executor.(object.Callable); ok {
							runner := NewVM(vm.debug)
							runner.Call(executor, 0, value.EncodedUndefined())
							runner.push(value.EncodeHandle(resolveHandle))
							runner.run()
						}

						handle = allocator.Allocate(promise)
						vm.push(value.EncodeHandle(handle))
					}
				}
			}
		case chunk.OP_THROW:
			{
				to := vm.exceptionStack[len(vm.exceptionStack)-1]
				vm.exceptionStack = vm.exceptionStack[:len(vm.exceptionStack)-1]

				ip = to
			}
		case chunk.OP_ADD_ARGUMENTS_TO_LOCALS:
			{

				arr := native.NewArray(int(argCount))
				for _, v := range vm.stack[vm.stackTop-int(argCount) : vm.stackTop] {
					arr.PushElement(v)
				}
				handle := allocator.Allocate(arr)
				vm.popN(int(argCount))
				vm.push(value.EncodeHandle(handle))
			}
		case chunk.OP_STORE_ARG_COUNT:
			{
				count := valueChunk.Code[ip]
				ip++
				argCount = count
			}
		case chunk.OP_AWAIT:
			{
				awaitee := vm.pop()
				awaiteeObj, err := allocator.GetObject(awaitee.GetHandle())

				if err != nil {
					return value.EncodeFalse(), err
				}

				if promise, ok := awaiteeObj.(*native.ObjPromise); ok {
					if curentAsyncFn, ok := frame.fn.(*native.ObjAsyncFunction); ok {
						count := (vm.stackTop - frame.localStart) + 1 // +1 because we popped our promise
						stack := make([]value.Value, count)
						copy(stack, append(vm.stack[frame.localStart:vm.stackTop], awaitee))
						curentAsyncFn.Pause(stack, ip)
						curentAsyncFn.Await(promise)

						vm.frameCount--

						if vm.frameCount > 0 {
							ip = frame.returnIp
							frame = vm.currentFrame()
							valueChunk = *frame.fn.ValueChunk()
						}
					}
				}
			}
		case chunk.OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS:
			{
				amount := valueChunk.Code[ip]
				ip++

				removeMap := map[int]bool{}

				for range amount {
					idx := int(valueChunk.Code[ip])
					removeMap[idx] = true
					ip++
					heapVars[frame.fn.GetHeapScope()] = append(heapVars[frame.fn.GetHeapScope()], vm.stack[frame.localStart+idx])
				}

				localCount := (vm.stackTop - frame.localStart) - int(amount)

				locals := make([]value.Value, 0, localCount)

				for i, v := range vm.stack[frame.localStart:vm.stackTop] {
					if !removeMap[i] {
						locals = append(locals, v)
					}
				}

				vm.stackTop = frame.localStart

				for _, v := range locals {
					vm.push(v)
				}
			}
		case chunk.OP_CREATE_CLASS_START:
			{
				name := vm.pop()

				if n, err := allocator.GetObject(name.GetHandle()); err == nil && n.Type() == object.OBJ_STRING {
					name := n.String()

					class := native.NewObjClass(name)
					proto := native.NewPrototype(name)

					vm.push(value.EncodeHandle(allocator.Allocate(class)))
					vm.push(value.EncodeHandle(allocator.Allocate(proto)))
				} else {
					return value.EncodedUndefined(), fmt.Errorf("%s is not a string", stringer.String(name))
				}
			}
		case chunk.OP_CREATE_CLASS_END:
			{
				proto := vm.pop()
				class := vm.peek()

				classObj, _ := allocator.GetObject(class.GetHandle())
				protoObj, _ := allocator.GetObject(proto.GetHandle())

				classObj.(*native.ObjClass).SetPrototype(protoObj.(*native.Prototype))
			}
		case chunk.OP_PUSH_METHOD:
			{
				method := vm.pop()

				methodObj, err := allocator.GetObject(method.GetHandle())

				if err != nil {
					return value.EncodedUndefined(), fmt.Errorf("%s was not an object", stringer.String(method))
				}

				if m, ok := methodObj.(object.Callable); ok {
					if m.GetHeapScope() != object.NOT_IN_HEAP_SCOPE {
						m = m.Clone()
					}
					method = value.EncodeHandle(allocator.Allocate(native.NewMethod(m)))
				} else {
					return value.EncodedUndefined(), fmt.Errorf("%s was not an function", stringer.String(method))
				}

				key := vm.pop()

				prototype := vm.peek()

				protoObj, err := allocator.GetObject(prototype.GetHandle())

				if err != nil {
					return value.EncodedUndefined(), fmt.Errorf("%s was not an object", stringer.String(prototype))
				}

				if p, ok := protoObj.(*native.Prototype); ok {
					p.SetMember(key, method)
				} else {
					return value.EncodedUndefined(), fmt.Errorf("%s was not an prototype", stringer.String(prototype))
				}
			}
		case chunk.OP_PUSH_PROPERTY:
			{
				v := vm.pop()
				k := vm.pop()

				class := vm.peekN(1)

				classObj, err := allocator.GetObject(class.GetHandle())

				if err != nil {
					return value.EncodedUndefined(), fmt.Errorf("%s was not an object", stringer.String(class))
				}

				if c, ok := classObj.(*native.ObjClass); ok {
					c.PushProperty(k, v)
				} else {
					return value.EncodedUndefined(), fmt.Errorf("%s was not an class object", stringer.String(class))

				}
			}
		case chunk.OP_THIS:
			{
				vm.push(frame.thisCtx)
			}
		}
	}
}

func setHeapScopes(c *value.ValueChunk, heapScope int) error {
	for _, v := range c.Constants {
		if v.IsObject() {
			obj, err := allocator.GetObject(v.GetHandle())

			if err != nil {
				return err
			}

			if obj, ok := obj.(*object.ObjFunction); ok && obj.GetHeapScope() <= heapScope {
				obj.SetHeapScope(heapScope)
				setHeapScopes(obj.ValueChunk(), heapScope)
			}
		}
	}
	return nil
}

func (vm *VM) log(arg value.Value) {
	fmt.Printf("%s\n", stringer.String(arg))
}
