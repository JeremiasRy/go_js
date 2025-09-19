package vm

import (
	"fmt"
	"go_js/allocator"
	"go_js/chunk"
	"go_js/constructor"
	eventloop "go_js/eventLoop"
	"go_js/object"
	"go_js/queue"
	"go_js/stringer"
	"go_js/value"
	"math"
	"sync"
)

const STACK_MAX = math.MaxUint8
const FRAMES_MAX = 64

var globals []value.Value
var heapVars = make(map[int][]value.Value)
var heapScopesCount int

type CallFrame struct {
	fn         object.Callable
	localStart int
	returnIp   int
}

func NewCallFrame(fn object.Callable, localStart int) *CallFrame {
	return &CallFrame{fn: fn, localStart: localStart, returnIp: 0}
}

func (cf *CallFrame) initCallFrame(fn object.Callable, localStart int, returnIp int) {
	cf.fn = fn
	cf.localStart = localStart
	cf.returnIp = returnIp
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

func (vm *VM) Call(fn object.Callable, returnIp int) error {
	if vm.frameCount == FRAMES_MAX {
		return fmt.Errorf("too many callframes")
	}

	vm.frames[vm.frameCount].initCallFrame(fn, vm.stackTop-fn.Arity(), returnIp)
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
	aIsObject, aHandle := object.IsValueObject(a)
	bIsObject, bHandle := object.IsValueObject(b)

	if aIsObject && bIsObject {
		aObj, _ := allocator.GetObject(aHandle)
		bObj, _ := allocator.GetObject(bHandle)

		if aObj.Type() == object.OBJ_STRING && bObj.Type() == object.OBJ_STRING {
			res := aObj.(*object.ObjString).Value + bObj.(*object.ObjString).Value
			return value.EncodeHandle(allocator.Allocate(object.NewObjString(res)))
		} else {
			// runtime error?
		}
	}

	if aIsObject && !bIsObject {
		aObj, _ := allocator.GetObject(aHandle)

		if aObj.Type() == object.OBJ_STRING {
			res := aObj.(*object.ObjString).Value + stringer.String(b)

			return value.EncodeHandle(allocator.Allocate(object.NewObjString(res)))
		}
	}

	if !aIsObject && bIsObject {
		bObj, _ := allocator.GetObject(bHandle)

		if bObj.Type() == object.OBJ_STRING {
			res := object.NewObjString(stringer.String(a) + bObj.(*object.ObjString).Value)

			return value.EncodeHandle(allocator.Allocate(res))
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

// used for OP_NEW to pop arguments to constructor.New()
func (vm *VM) popN(n int) []value.Value {
	r := make([]value.Value, n)
	copy(r, vm.stack[vm.stackTop-n:vm.stackTop])
	vm.stackTop -= n
	return r
}

func (vm *VM) CreateTemplateString(o *object.ObjTemplateLiteral) value.Value {
	objStr := object.NewObjString(o.CreateString())
	return value.EncodeHandle(allocator.Allocate(objStr))
}

func (vm *VM) Run(wg *sync.WaitGroup) {
Run:
	fn := queue.Dequeue()
	for fn != nil {

		vm.Call(fn, 0)
		vm.run()

		wg.Done()
		fn = queue.Dequeue()
	}

	for range queue.QueueC {
		goto Run
	}
}

func (vm *VM) run() (value.Value, error) {
	frame := vm.currentFrame()
	valueChunk := *frame.fn.ValueChunk()
	ip := 0

	if vm.debug {
		println()
		println("-- NEW RUNNER SPAWNED --")
		println()
		PrintChunk(valueChunk)
	}

	for {
		//time.Sleep(time.Millisecond * 100)
		code := valueChunk.Code[ip]
		ip++

		if vm.debug {
			printStack(vm.stack[0:vm.stackTop])
			println(opNames[code])
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
				if scope, found := heapVars[frame.fn.HeapScope()]; found {
					scope = append(scope, variable)
					heapVars[frame.fn.HeapScope()] = scope
				} else {
					panic("no heap scope generated for function")
				}
			}
		case chunk.OP_GET_HEAP_VAR:
			{
				heapVar := valueChunk.Code[ip]
				ip++
				vm.push(heapVars[frame.fn.HeapScope()][heapVar])
			}
		case chunk.OP_SET_HEAP_VAR:
			{
				heapVar := valueChunk.Code[ip]
				ip++
				heapVars[frame.fn.HeapScope()][heapVar] = vm.pop()
			}
		case chunk.OP_DEFINE_GLOBAL:
			{
				variable := vm.pop()
				vm.addGlobal(variable)
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
				globals[global] = vm.pop()
				ip++
			}
		case chunk.OP_POP_LOCAL:
			{
				vm.stackTop--
			}
		case chunk.OP_DEFINE_LOCAL:
			{
				vm.pop()
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
				objHash := object.NewObjectHash()
				handle := allocator.Allocate(objHash)

				vm.push(value.EncodeHandle(handle))
			}
		case chunk.OP_SET_OBJECT_MEMBER:
			{
				v := vm.pop()
				member := vm.pop()
				hash := vm.peek()

				isObject, handle := object.IsValueObject(hash)

				if !isObject {
					return value.EncodedUndefined(), fmt.Errorf("%v is not an object", hash)
				}

				if v.IsObject() {
					obj, _ := allocator.GetObject(v.GetHandle())

					// check if we have a closure in hand
					if fn, ok := obj.(*object.ObjFunction); ok && fn.HeapScope() != -1 {
						v = value.EncodeHandle(allocator.Allocate(fn.Clone()))
					}
				}

				obj, _ := allocator.GetObject(handle)

				// this needs to be extended in case value is number or whatever
				if obj, ok := obj.(*object.ObjObject); ok {
					obj.SetMember(stringer.String(member), v)
				} else {
					return value.EncodedUndefined(), fmt.Errorf("we currently don't support adding properties to %s types", stringer.String(v))
				}
			}
		case chunk.OP_GET_OBJECT_MEMBER:
			{
				hash := vm.pop()
				member := valueChunk.Constants[valueChunk.Code[ip]]
				obj, err := allocator.GetObject(hash.GetHandle())

				if err != nil {
					return value.EncodedUndefined(), err
				}

				switch obj := obj.(type) {
				// should interface this...
				case *object.ObjArr:
					{
						var v value.Value
						if !member.IsType(value.TAG_OBJ) {
							v = obj.GetElementAt(int(member.AsNumber()))
						} else {
							v = obj.GetMember(stringer.String(member))
						}
						vm.push(v)
						ip++
					}
				case *object.ObjObject:
					{
						value := obj.GetMember(stringer.String(member))
						vm.push(value)
						ip++
					}
				case *object.ObjString:
					{
						value := obj.GetMember(stringer.String(member))
						vm.push(value)
						ip++
					}
				case *object.ObjError:
					{
						value := obj.GetMember(stringer.String(member))
						vm.push(value)
						ip++
					}
				default:
					{
						return value.EncodedUndefined(), fmt.Errorf("cant get property: %s from: %v", stringer.String(member), hash)
					}
				}

			}
		case chunk.OP_PUSH_UNDEFINED:
			{
				vm.push(value.EncodedUndefined())
			}
		case chunk.OP_CALL:
			{
				callee := vm.pop()
				isObject, handle := object.IsValueObject(callee)

				if isObject {
					obj, err := allocator.GetObject(handle)

					if err != nil {
						return value.EncodedUndefined(), err
					}

					switch fn := obj.(type) {
					case object.Callable:
						{
							vm.Call(fn, ip)
							frame = vm.currentFrame()
							valueChunk = *frame.fn.ValueChunk()
							ip = 0
						}
					case *object.Log:
						{
							arg := vm.pop()
							vm.log(arg)
							vm.push(value.EncodedUndefined())
						}
					case *object.Clock:
						{
							vm.push(fn.Clock())
						}
					case *object.ArrayPush:
						{
							arg := vm.pop()
							vm.push(fn.Push(arg))
						}
					case *object.ArrayForEach:
						{
							callback := vm.pop()
							iterator := object.NewIterator(fn.Owner)
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
									runner.Call(fn, 0)
									runner.run()
									done = iterator.Next()
								}
							}

							vm.push(value.EncodedUndefined())
						}
					case *object.ArrayFilter:
						{
							callback := vm.pop()
							iterator := object.NewIterator(fn.Owner)
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
									runner.Call(fn, 0)
									result, err := runner.run()

									if err != nil {
										return value.EncodedUndefined(), err
									}

									if result.AsBoolean() {
										arr = append(arr, item)
									}

									done = iterator.Next()
								}
							}
							length := len(arr)
							objArr := constructor.NewArray(length)

							for _, item := range arr {
								objArr.PushElement(item)
							}

							v := value.EncodeHandle(allocator.Allocate(objArr))
							vm.push(v)
						}
					case *object.StringToUpperCase:
						{
							vm.push(value.EncodeHandle(allocator.Allocate(fn.ToUpperCase())))
						}
					case *object.StringIncludes:
						{
							arg := vm.pop()
							vm.push(fn.Includes(stringer.String(arg)))
						}
					case *object.SetTimeout:
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
								eventloop.Dispatch(fn.CloneForDispatch())
								vm.push(value.EncodedUndefined())
							}
						}
					}
				} else {
					return value.EncodedUndefined(), fmt.Errorf("%s is not a function", stringer.String(callee))
				}
			}
		case chunk.OP_RETURN:
			{
				ip = frame.returnIp
				value := value.EncodedUndefined()

				if vm.stackTop > 0 {
					value = vm.pop()
				}

				vm.stackTop = frame.localStart
				vm.push(value)
				vm.frameCount--

				if vm.frameCount <= 0 {
					vm.stackTop = 0

					return value, nil
				}

				frame = vm.currentFrame()
				valueChunk = *frame.fn.ValueChunk()
			}
		case chunk.OP_CREATE_ARRAY:
			{
				length := int(valueChunk.Code[ip+3]) | int(valueChunk.Code[ip+2])<<8 | int(valueChunk.Code[ip+1])<<16 | int(valueChunk.Code[ip])<<24
				ip += 4
				arr := constructor.NewArray(length)
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

				arrOBj, ok := obj.(*object.ObjArr)

				if !ok {
					return value.EncodedUndefined(), fmt.Errorf("trying to initialize {%s} that is not an array", stringer.String(arr))
				}

				arrOBj.PushElement(v)
			}
		case chunk.OP_GET_ITERATOR:
			{
				iteratee := vm.pop()

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

				vm.push(value.EncodeHandle(allocator.Allocate(object.NewIterator(iteratorObj))))
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
		case chunk.OP_TEMPLATE_LITERAL_START:
			{
				builder := value.EncodeHandle(allocator.Allocate(object.NewObjTemplateLiteral()))
				vm.push(builder)
			}
		case chunk.OP_TEMPLATE_PUSH_STRING:
			{
				v := vm.pop()
				builder := vm.peek()

				obj, err := allocator.GetObject(builder.GetHandle())

				if err != nil {
					return value.EncodedUndefined(), err
				}

				if b, ok := obj.(*object.ObjTemplateLiteral); ok {
					b.PushString(stringer.String(v))
				} else {
					return value.EncodedUndefined(), fmt.Errorf("%s is not an template literal", stringer.String(builder))
				}

			}
		case chunk.OP_TEMPLATE_LITERAL_END:
			{
				builder := vm.pop()

				obj, err := allocator.GetObject(builder.GetHandle())

				if err != nil {
					return value.EncodedUndefined(), err
				}

				if b, ok := obj.(*object.ObjTemplateLiteral); ok {
					str := b.CreateString()
					handle := allocator.Allocate(object.NewObjString(str))
					vm.push(value.EncodeHandle(handle))
				} else {
					return value.EncodedUndefined(), fmt.Errorf("%s is not an template literal", stringer.String(builder))
				}
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

				if ctor, ok := obj.(constructor.Constructor); ok {
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
			}
		case chunk.OP_THROW:
			{
				to := vm.exceptionStack[len(vm.exceptionStack)-1]
				vm.exceptionStack = vm.exceptionStack[:len(vm.exceptionStack)-1]

				ip = to
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

			if obj, ok := obj.(*object.ObjFunction); ok && obj.HeapScope() <= heapScope {
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
