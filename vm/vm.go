package vm

import (
	"fmt"
	"go_js/allocator"
	"go_js/chunk"
	"go_js/compiler"
	"go_js/object"
	"go_js/parser"
	"go_js/stringer"
	"go_js/value"
	"log"
	"math"
	"time"
)

type CallFrame struct {
	fn        object.Callable
	locals    []value.Value
	nextLocal int
	returnIp  int
}

func NewCallFrame(fn object.Callable, locals []value.Value) *CallFrame {
	return &CallFrame{fn: fn, locals: locals, returnIp: 0}
}

func (cf *CallFrame) initCallFrame(fn object.Callable, locals []value.Value, returnIp int) {
	cf.fn = fn
	cf.locals = locals
	cf.returnIp = returnIp
	cf.nextLocal = fn.Arity()
}

func (cf *CallFrame) addLocal(v value.Value) {
	cf.locals[cf.nextLocal] = v
	cf.nextLocal++
}

func (cf *CallFrame) getLocal(index int) value.Value {
	return cf.locals[index]
}

const STACK_MAX = math.MaxUint8
const FRAMES_MAX = 64

var globals []value.Value

type VM struct {
	frames     []CallFrame
	frameCount int
	stack      []value.Value
	stackTop   int

	heapVars        map[int][]value.Value
	heapScopesCount int
}

func NewVM() *VM {
	frames := make([]CallFrame, FRAMES_MAX)
	stack := make([]value.Value, STACK_MAX)
	return &VM{frames: frames, frameCount: 0, stack: stack, stackTop: 0, heapVars: map[int][]value.Value{}, heapScopesCount: -1}
}

func (vm *VM) call(fn object.Callable, returnIp int) error {
	if vm.frameCount == FRAMES_MAX {
		return fmt.Errorf("too many callframes")
	}

	vm.frames[vm.frameCount].initCallFrame(fn, vm.stack[vm.stackTop-fn.Arity():vm.stackTop], returnIp)
	vm.frameCount++
	return nil
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
			res := aObj.(object.ObjString) + bObj.(object.ObjString)
			return value.EncodeHandle(allocator.Allocate(object.ObjString(res)))
		} else {
			// runtime error?
		}
	}

	if aIsObject && !bIsObject {
		aObj, _ := allocator.GetObject(aHandle)

		if aObj.Type() == object.OBJ_STRING {
			res := aObj.(object.ObjString) + object.ObjString(stringer.String(b))

			return value.EncodeHandle(allocator.Allocate(object.ObjString(string(res))))
		}
	}

	if !aIsObject && bIsObject {
		bObj, _ := allocator.GetObject(bHandle)

		if bObj.Type() == object.OBJ_STRING {
			res := object.ObjString(stringer.String(a)) + bObj.(object.ObjString)

			return value.EncodeHandle(allocator.Allocate(object.ObjString(string(res))))
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

func (vm *VM) CreateTemplateString(o *object.ObjTemplateLiteral) value.Value {
	str := object.ObjString(o.CreateString())
	return value.EncodeHandle(allocator.Allocate(str))
}

func (vm *VM) run() error {
	start := time.Now()
	frame := vm.frames[vm.frameCount-1]
	valueChunk := *frame.fn.ValueChunk()
	ip := 0

	if DEBUG {
		println()
		println("-- NEW RUNNER SPAWNED --")
		println()
		fmt.Printf("<fn %s>\n", frame.fn.Name())
		PrintChunk(valueChunk)
	}

	for {
		if ip > len(valueChunk.Code)-1 {
			if DEBUG {

			}
			return nil
		}
		//time.Sleep(time.Millisecond * 100)
		code := valueChunk.Code[ip]
		ip++

		if DEBUG {
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

					} else {
						vm.push(value.EncodeFalse())
					}
				}

				if a == b {
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
				if scope, found := vm.heapVars[frame.fn.HeapScope()]; found {
					scope = append(scope, variable)
					vm.heapVars[frame.fn.HeapScope()] = scope
				} else {
					panic("no heap scope generated for function")
				}
			}
		case chunk.OP_GET_HEAP_VAR:
			{
				heapVar := valueChunk.Code[ip]
				ip++
				vm.push(vm.heapVars[frame.fn.HeapScope()][heapVar])
			}
		case chunk.OP_SET_HEAP_VAR:
			{
				heapVar := valueChunk.Code[ip]
				ip++
				vm.heapVars[frame.fn.HeapScope()][heapVar] = vm.pop()
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
		case chunk.OP_DEFINE_LOCAL:
			{
				variable := vm.pop()
				frame.addLocal(variable)
			}
		case chunk.OP_GET_LOCAL:
			{
				vm.push(frame.getLocal(int(valueChunk.Code[ip])))
				ip++
			}
		case chunk.OP_SET_LOCAL:
			{
				frame.locals[valueChunk.Code[ip]] = vm.pop()
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
					return fmt.Errorf("%v is not an object", hash)
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
				if obj, ok := obj.(*object.ObjHash); ok {
					obj.SetMember(stringer.String(member), v)
				} else {
					return fmt.Errorf("we currently don't support adding properties to %s types", stringer.String(v))
				}
			}
		case chunk.OP_GET_OBJECT_MEMBER:
			{
				hash := vm.pop()
				member := valueChunk.Constants[valueChunk.Code[ip]]
				obj, err := allocator.GetObject(hash.GetHandle())

				if err != nil {
					return err
				}

				switch obj := obj.(type) {
				// should interface this...
				case *object.ObjArr:
					{
						value := obj.GetMember(stringer.String(member))
						vm.push(value)
						ip++
					}
				case *object.ObjHash:
					{
						value := obj.GetMember(stringer.String(member))
						vm.push(value)
						ip++
					}
				default:
					{
						return fmt.Errorf("runtime error: cant get property: %s from: %v", stringer.String(member), hash)
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
						return err
					}

					switch fn := obj.(type) {
					case object.Callable:
						{
							bottom := vm.stackTop - fn.Arity()
							localCount := max(fn.LocalCount(), 0)
							top := vm.stackTop + localCount

							vm.frames[vm.frameCount].initCallFrame(fn, vm.stack[bottom:top], ip)
							vm.frameCount++
							vm.stackTop += localCount

							frame = vm.frames[vm.frameCount-1]
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
								return err
							}

							if fn, ok := obj.(*object.ObjFunction); ok {
								for !done {
									runner := NewVM()
									item := iterator.Current()
									runner.push(item)
									runner.call(fn, 0)
									runner.run()
									done = iterator.Next()
								}
							}
							vm.push(value.EncodedUndefined())
						}
					}
				} else {
					return fmt.Errorf("%s is not a function", stringer.String(callee))
				}
			}
		case chunk.OP_RETURN:
			{
				ip = frame.returnIp
				value := vm.pop()
				vm.stackTop -= len(frame.locals)
				vm.push(value)
				vm.frameCount--

				if vm.frameCount <= 0 {
					if DEBUG {
						println()
						println("-- RUNNER EXITING --")
						println()
					}
					return nil
				}

				frame = vm.frames[vm.frameCount-1]
				valueChunk = *frame.fn.ValueChunk()
			}
		case chunk.OP_CREATE_ARRAY:
			{
				length := int(valueChunk.Code[ip+3]) | int(valueChunk.Code[ip+2])<<8 | int(valueChunk.Code[ip+1])<<16 | int(valueChunk.Code[ip])<<24
				ip += 4
				arr := object.NewObjArr(length)
				arr.Hash["push"] = value.EncodeHandle(allocator.Allocate(object.NewArrayPush(arr)))
				arr.Hash["forEach"] = value.EncodeHandle(allocator.Allocate(object.NewArrayForEach(arr)))
				handle := allocator.Allocate(arr)
				vm.push(value.EncodeHandle(handle))
			}
		case chunk.OP_PUSH_ELEMENT:
			{
				value := vm.pop()
				arr := vm.peek()

				obj, err := allocator.GetObject(arr.GetHandle())

				if err != nil {
					return err
				}

				arrOBj, ok := obj.(*object.ObjArr)

				if !ok {
					return fmt.Errorf("trying to initialize {%s} that is not an array", stringer.String(arr))
				}

				arrOBj.PushElement(value)
			}
		case chunk.OP_GET_ITERATOR:
			{
				iteratee := vm.pop()

				if !iteratee.IsObject() {
					return fmt.Errorf("%s is not an object", stringer.String(iteratee))
				}

				obj, err := allocator.GetObject(iteratee.GetHandle())

				if err != nil {
					return err
				}

				iteratorObj, ok := obj.(object.Iterable)

				if !ok {
					return fmt.Errorf("%s is not iterable", stringer.String(iteratee))
				}

				vm.push(value.EncodeHandle(allocator.Allocate(object.NewIterator(iteratorObj))))
			}
		case chunk.OP_ITERATOR_NEXT:
			{
				iterator := vm.peek()

				if !iterator.IsObject() {
					return fmt.Errorf("%s is not an object", stringer.String(iterator))
				}

				obj, err := allocator.GetObject(iterator.GetHandle())

				if err != nil {
					return err
				}

				iteratorObj, ok := obj.(*object.Iterator)

				if !ok {
					return fmt.Errorf("%s is not iterable", stringer.String(iterator))
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
					return fmt.Errorf("%s is not an object", stringer.String(iterator))
				}

				obj, err := allocator.GetObject(iterator.GetHandle())

				if err != nil {
					return err
				}

				iteratorObj, ok := obj.(*object.Iterator)

				if !ok {
					return fmt.Errorf("%s is not iterable", stringer.String(iterator))
				}

				vm.push(iteratorObj.Current())
			}
		case chunk.OP_CREATE_HEAP_SCOPE:
			{
				vm.heapScopesCount++
				vm.heapVars[vm.heapScopesCount] = []value.Value{}
				setHeapScopes(frame.fn.ValueChunk(), vm.heapScopesCount)
				frame.fn.SetHeapScope(vm.heapScopesCount)
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
					return err
				}

				if b, ok := obj.(*object.ObjTemplateLiteral); ok {
					b.PushString(stringer.String(v))
				} else {
					return fmt.Errorf("%s is not an template literal", stringer.String(builder))
				}

			}
		case chunk.OP_TEMPLATE_LITERAL_END:
			{
				builder := vm.pop()

				obj, err := allocator.GetObject(builder.GetHandle())

				if err != nil {
					return err
				}

				if b, ok := obj.(*object.ObjTemplateLiteral); ok {
					str := b.CreateString()
					handle := allocator.Allocate(object.ObjString(str))
					vm.push(value.EncodeHandle(handle))
				} else {
					return fmt.Errorf("%s is not an template literal", stringer.String(builder))
				}

			}
		case chunk.OP_EOF:
			{
				fmt.Printf("Thanks! %s\n", time.Since(start))
				return nil
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

func Interpret(source []byte) {
	startAstParse := time.Now()
	ast, err := parser.GetAst(source, nil, 0)
	fmt.Printf("AST parsed in %s\n", time.Since(startAstParse))

	if err != nil {
		log.Fatalf("Failed to parse javascript, %e", err)
	}

	if DEBUG {
		println("### Abtract Syntax Tree ###")
		parser.PrintNode(ast)
		println()
	}

	startCompile := time.Now()
	main, err := compiler.Compile(ast)
	fmt.Printf("AST Compiled in %s\n", time.Since(startCompile))

	if err != nil {
		log.Fatalf("Failed to parse javascript, %e", err)
	}

	vm := NewVM()
	vm.call(main, 0)
	err = vm.run()

	if err != nil {
		log.Fatalf("runtime error: %s", err.Error())
	}
}
