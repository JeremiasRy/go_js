package vm

import (
	"fmt"
	"go_js/chunk"
	"go_js/compiler"
	"go_js/heap"
	"go_js/object"
	"go_js/parser"
	"go_js/value"
	"log"
	"math"
)

type CallFrame struct {
	fn       *object.ObjFunction
	locals   []value.Value
	returnIp int
}

func NewCallFrame(fn *object.ObjFunction, locals []value.Value) *CallFrame {
	return &CallFrame{fn: fn, locals: locals, returnIp: 0}
}

func (cf *CallFrame) initCallFrame(fn *object.ObjFunction, locals []value.Value, returnIp int) {
	cf.fn = fn
	cf.locals = locals
	cf.returnIp = returnIp
}

func (cf *CallFrame) addLocal(v value.Value) int {
	cf.locals = append(cf.locals, v)
	return len(cf.locals) - 1
}

func (cf *CallFrame) getLocal(index int) value.Value {
	return cf.locals[index]
}

const STACK_MAX = math.MaxUint8
const FRAMES_MAX = 64
const DEBUG = true

type VM struct {
	frames     []CallFrame
	frameCount int
	stack      []value.Value
	stackTop   int

	globals       []value.Value
	heapVariables map[string]value.Value
}

func NewVM() *VM {
	frames := make([]CallFrame, FRAMES_MAX)
	stack := make([]value.Value, STACK_MAX)
	return &VM{frames: frames, frameCount: 0, stack: stack, stackTop: 0, globals: []value.Value{}, heapVariables: map[string]value.Value{}}
}

func (vm *VM) call(fn *object.ObjFunction, returnIp int) error {
	if vm.frameCount == FRAMES_MAX {
		return fmt.Errorf("too many callframes")
	}

	vm.frames[vm.frameCount].initCallFrame(fn, vm.stack[vm.stackTop-fn.Arity:vm.stackTop], returnIp)
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
	vm.globals = append(vm.globals, v)
}

func (vm *VM) getGlobal(global int) value.Value {
	return vm.globals[global]
}

func (vm *VM) currentFramePointer() *CallFrame {
	return &vm.frames[vm.frameCount-1]
}

func (vm *VM) string(v value.Value) string {
	return ""
}

func (vm *VM) concatenate(a, b value.Value) value.Value {
	aIsObject, aRegister := object.GetObject(a)
	bIsObject, bRegister := object.GetObject(b)

	if aIsObject && bIsObject {
		aObj := heap.GetObject(aRegister)
		bObj := heap.GetObject(bRegister)

		if aObj.Type() == object.OBJ_STRING && bObj.Type() == object.OBJ_STRING {
			res := aObj.(object.ObjString) + bObj.(object.ObjString)
			return heap.Allocate(object.ObjString(string(res)))
		} else {
			// runtime error?
		}
	}

	if aIsObject && !bIsObject {
		aObj := heap.GetObject(aRegister)

		if aObj.Type() == object.OBJ_STRING {
			res := aObj.(object.ObjString) + object.ObjString(vm.string(b))

			return heap.Allocate(object.ObjString(string(res)))
		}
	}

	if !aIsObject && bIsObject {
		bObj := heap.GetObject(bRegister)

		if bObj.Type() == object.OBJ_STRING {
			res := object.ObjString(vm.string(a)) + bObj.(object.ObjString)

			return heap.Allocate(object.ObjString(string(res)))
		}
	}

	return value.Value(math.Float64bits(a.AsNumber() + b.AsNumber()))
}

func (vm *VM) subtract(a, b value.Value) value.Value {
	aIsObject, _ := object.GetObject(a)
	bIsObject, _ := object.GetObject(b)
	if aIsObject || bIsObject {
		if aIsObject && bIsObject {
			// todo
		} else {
			return value.EncodeNaN()
		}
	}

	return value.ValueFromFloat64(a.AsNumber() - b.AsNumber())
}

func (vm *VM) run() error {
	frame := vm.frames[vm.frameCount-1]
	valueChunk := *frame.fn.ValueChunk()
	ip := 0

	if DEBUG {
		PrintChunk(valueChunk)
	}

	for {
		//time.Sleep(time.Millisecond * 100)
		code := valueChunk.Code[ip]
		ip++

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

				bIsObject, bObjRegister := object.GetObject(b)
				aIsObject, aObjRegister := object.GetObject(a)

				if bIsObject && aIsObject {
					if heap.GetObject(bObjRegister).Type() == heap.GetObject(aObjRegister).Type() {

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
				jump := int(valueChunk.Code[ip+3]) | int(valueChunk.Code[ip+2])<<8 | int(valueChunk.Code[ip])<<16 | int(valueChunk.Code[ip])<<24

				if !v.AsBoolean() {
					ip = jump
				} else {
					ip += 4
				}
			}
		case chunk.OP_JUMP:
			{
				jump := int(valueChunk.Code[ip+3]) | int(valueChunk.Code[ip+2])<<8 | int(valueChunk.Code[ip])<<16 | int(valueChunk.Code[ip])<<24
				ip = jump
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
				vm.globals[global] = vm.pop()
				ip++
			}
		case chunk.OP_DEFINE_LOCAL:
			{
				variable := vm.peek()
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
		case chunk.OP_DEFINE_OBJECT_MEMBER:
			{
				member := vm.pop()
				value := vm.pop()
				hash := vm.peek()

				isObject, register := object.GetObject(hash)

				if !isObject {
					return fmt.Errorf("%v is not an object", hash)
				}

				hashObject := heap.GetObject(register)
				hashObject.(*object.ObjHash).SetMember(vm.string(member), value)
			}
		case chunk.OP_GET_GLOBAL_OBJECT_MEMBER:
			{
				global := int(valueChunk.Code[ip])
				member := valueChunk.Constants[valueChunk.Code[ip+1]]

				_, obj := object.GetObject(vm.getGlobal(global))

				value := heap.GetObject(obj).(*object.ObjHash).GetMember(vm.string(member))
				vm.push(value)
				ip += 2
			}
		case chunk.OP_GET_LOCAL_OBJECT_MEMBER:
			{
				slot := int(valueChunk.Code[ip])
				member := valueChunk.Constants[valueChunk.Code[ip+1]]
				fmt.Printf("%d, %s\n", slot, member)

				_, obj := object.GetObject(frame.getLocal(slot))

				value := heap.GetObject(obj).(*object.ObjHash).GetMember(vm.string(member))
				vm.push(value)
				ip += 2
			}
		case chunk.OP_PUSH_UNDEFINED:
			{
				vm.push(value.EncodedUndefined())
			}
		case chunk.OP_CALL:
			{
				callee := vm.pop()
				isObject, register := object.GetObject(callee)

				if isObject {
					obj := heap.GetObject(register)
					switch fn := obj.(type) {
					case *object.ObjFunction:
						{
							vm.frames[vm.frameCount-1].locals = frame.locals

							vm.frames[vm.frameCount].initCallFrame(fn, vm.stack[vm.stackTop-fn.Arity:vm.stackTop], ip)
							vm.frameCount++

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
					}

				} else {
					return fmt.Errorf("%s is not a function", callee)
				}

			}
		case chunk.OP_RETURN:
			{
				ip = frame.returnIp
				value := vm.pop()

				vm.stackTop -= max(len(frame.locals), frame.fn.Arity)

				vm.push(value)
				vm.frameCount--

				frame = vm.frames[vm.frameCount-1]
				valueChunk = *frame.fn.ValueChunk()
			}
		case chunk.OP_EOF:
			{
				fmt.Printf("Thanks!\n")
				return nil
			}
		}
	}

}

func (vm *VM) log(arg value.Value) {
	panic("unimplemented")
}

func Interpret(source []byte) {
	ast, err := parser.GetAst(source, nil, 0)

	if err != nil {
		log.Fatalf("Failed to parse javascript, %e", err)
	}

	if DEBUG {
		println("### Abtract Syntax Tree ###")
		parser.PrintNode(ast)
		println()
	}

	main, err := compiler.Compile(ast)

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
