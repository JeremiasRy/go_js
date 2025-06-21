package vm

import (
	"fmt"
	"go_js/parser"
	"log"
	"math"
	"time"
)

type CallFrame struct {
	fn       *ObjFunction
	locals   []Value
	returnIp int
}

func NewCallFrame(fn *ObjFunction, locals []Value) *CallFrame {
	return &CallFrame{fn: fn, locals: locals, returnIp: 0}
}

func (cf *CallFrame) initCallFrame(fn *ObjFunction, locals []Value, returnIp int) {
	cf.fn = fn
	cf.locals = locals
	cf.returnIp = returnIp
}

func (cf *CallFrame) AddLocal(v Value) int {
	cf.locals = append(cf.locals, v)
	return len(cf.locals) - 1
}

func (cf *CallFrame) GetLocal(index int) Value {
	return cf.locals[index]
}

const STACK_MAX = 255
const FRAMES_MAX = 64
const DEBUG = false

var HEAP *Heap = NewHeap()

type VM struct {
	frames     []CallFrame
	frameCount int
	stack      []Value
	stackTop   int
	globals    []Value
}

func NewVM() *VM {
	frames := make([]CallFrame, FRAMES_MAX)
	stack := make([]Value, STACK_MAX)
	return &VM{frames: frames, frameCount: 0, stack: stack, stackTop: 0, globals: []Value{}}
}

func (vm *VM) call(fn *ObjFunction, returnIp int) error {
	if vm.frameCount == FRAMES_MAX {
		return fmt.Errorf("too many callframes")
	}

	vm.frames[vm.frameCount].initCallFrame(fn, vm.stack[vm.stackTop-fn.arity:vm.stackTop], returnIp)
	vm.frameCount++
	return nil
}

func (vm *VM) push(v Value) {
	vm.stack[vm.stackTop] = v
	vm.stackTop++
}

func (vm *VM) peek() Value {
	return vm.stack[vm.stackTop-1]
}

func (vm *VM) pop() Value {
	vm.stackTop--
	return vm.stack[vm.stackTop]
}
func (vm *VM) addGlobal(v Value) {
	vm.globals = append(vm.globals, v)
}
func (vm *VM) getGlobal(global int) Value {
	return vm.globals[global]
}

func (vm *VM) currentFramePointer() *CallFrame {
	return &vm.frames[vm.frameCount-1]
}

func (vm *VM) concatenate(a, b Value) Value {
	aIsObject := a.isObject()
	bIsObject := b.isObject()

	if a.isObject() && b.isObject() {
		_, aObj := a.getObject()
		_, bObj := b.getObject()
		if aObj.Type() == OBJ_STRING && bObj.Type() == OBJ_STRING {
			res := aObj.(ObjString) + bObj.(ObjString)
			return HEAP.AllocateString(string(res))
		} else {
			// runtime error?
		}
	}

	if aIsObject && !bIsObject {
		_, aObj := a.getObject()

		if aObj.Type() == OBJ_STRING {
			res := aObj.(ObjString) + ObjString(b.String())

			return HEAP.AllocateString(string(res))
		}
	}

	if !aIsObject && bIsObject {
		_, bObj := b.getObject()

		if bObj.Type() == OBJ_STRING {
			res := ObjString(a.String()) + bObj.(ObjString)

			return HEAP.AllocateString(string(res))
		}
	}

	return Value(math.Float64bits(a.asNumber() + b.asNumber()))
}

func (vm *VM) subtract(a, b Value) Value {
	if a.isObject() || b.isObject() {
		if a.isObject() && b.isObject() {
			// todo
		} else {
			return EncodeNaN()
		}
	}

	return ValueFromFloat64(a.asNumber() - b.asNumber())
}

func (vm *VM) run() {
	if DEBUG {
		printFrame(vm.currentFramePointer())
		for _, object := range HEAP.objects {
			if object.Type() == OBJ_FUNCTION {
				println(object.String())
				printChunk(object.(*ObjFunction).chunk)
				println()
			}
		}
	}
	frame := vm.frames[vm.frameCount-1]
	chunk := *frame.fn.chunk
	ip := 0
	start := time.Now()

	for {
		//time.Sleep(time.Millisecond * 100)
		code := chunk.code[ip]
		ip++

		if DEBUG {
			println("Current context: ", vm.currentFramePointer().fn.name)
			print("Stack: [ ")
			for _, v := range vm.stack[:vm.stackTop] {
				isObject, obj := v.getObject()
				if isObject {
					print(obj.String() + " | ")
				} else {
					print(v.String() + " | ")
				}

			}
			println("-top- ]")
			println("Instsruction: ", OpcodeNames[code])
			println("---")
		}

		switch code {
		case OP_CONSTANT:
			{
				vm.push(chunk.constants[chunk.code[ip]])
				ip++
			}
		case OP_ADD:
			{
				b := vm.pop()
				a := vm.pop()

				vm.push(vm.concatenate(a, b))
			}
		case OP_SUBTRACT:
			{
				b := vm.pop()
				a := vm.pop()

				vm.push(vm.subtract(a, b))
			}
		case OP_DIVIDE:
			{
				b := vm.pop()
				a := vm.pop()
				vm.push(ValueFromFloat64(a.asNumber() / b.asNumber()))
			}
		case OP_MULTIPLY:
			{
				b := vm.pop()
				a := vm.pop()
				vm.push(ValueFromFloat64(a.asNumber() * b.asNumber()))
			}
		case OP_LESS_THAN:
			{
				b := vm.pop()
				a := vm.pop()

				if a.asNumber() < b.asNumber() {
					vm.push(EncodeTrue())
				} else {
					vm.push(EncodeFalse())
				}
			}
		case OP_LESS_THAN_EQUAL:
			{
				b := vm.pop()
				a := vm.pop()

				if a.asNumber() <= b.asNumber() {
					vm.push(EncodeTrue())
				} else {
					vm.push(EncodeFalse())
				}
			}
		case OP_GREATER_THAN:
			{
				b := vm.pop()
				a := vm.pop()

				if a.asNumber() > b.asNumber() {
					vm.push(EncodeTrue())
				} else {
					vm.push(EncodeFalse())
				}
			}
		case OP_GREATER_THAN_EQUAL:
			{
				b := vm.pop()
				a := vm.pop()

				if a.asNumber() >= b.asNumber() {
					vm.push(EncodeTrue())
				} else {
					vm.push(EncodeFalse())
				}
			}
		case OP_STRICT_EQUALS:
			{
				b := vm.pop()
				a := vm.pop()

				bIsObject, bObj := b.getObject()
				aIsObject, aObj := a.getObject()

				if bIsObject && aIsObject {
					if bObj.Type() == aObj.Type() {

					} else {
						vm.push(EncodeFalse())
					}
				}

				if a == b {
					vm.push(EncodeTrue())
				} else {
					vm.push(EncodeFalse())
				}
			}
		case OP_JUMP_IF_FALSE:
			{
				value := vm.pop()
				jump := int(chunk.code[ip+3]) | int(chunk.code[ip+2])<<8 | int(chunk.code[ip])<<16 | int(chunk.code[ip])<<24
				ip += 4
				if !AsBoolean(value) {
					ip += jump
				}
			}
		case OP_JUMP:
			{
				jump := int(chunk.code[ip+3]) | int(chunk.code[ip+2])<<8 | int(chunk.code[ip])<<16 | int(chunk.code[ip])<<24
				ip += jump + 4
			}
		case OP_DEFINE_GLOBAL:
			{
				variable := vm.pop()
				vm.addGlobal(variable)

			}
		case OP_GET_GLOBAL:
			{
				global := chunk.code[ip]
				vm.push(vm.getGlobal(int(global)))
				ip++

			}
		case OP_DEFINE_LOCAL:
			{
				variable := vm.peek()
				frame.AddLocal(variable)
			}
		case OP_GET_LOCAL:
			{
				slot := chunk.code[ip]
				vm.push(frame.GetLocal(int(slot)))
				ip++
			}
		case OP_GET_GLOBAL_OBJECT_MEMBER:
			{
				global := int(chunk.code[ip])
				member := chunk.constants[chunk.code[ip+1]]

				_, obj := vm.getGlobal(global).getObject()

				value := obj.(*ObjHash).GetMember(member.String())
				vm.push(value)
				ip += 2
			}
		case OP_GET_LOCAL_OBJECT_MEMBER:
			{
				slot := int(chunk.code[ip])
				member := chunk.constants[chunk.code[ip+1]]

				_, obj := frame.GetLocal(slot).getObject()

				value := obj.(*ObjHash).GetMember(member.String())
				vm.push(value)
				ip += 2
			}
		case OP_PUSH_UNDEFINED:
			{
				vm.push(EncodedUndefined())
			}
		case OP_CALL:
			{
				callee := vm.pop()

				if callee.isObject() {
					_, obj := callee.getObject()
					switch obj.Type() {
					case OBJ_FUNCTION:
						{
							fn := obj.(*ObjFunction)
							vm.frames[vm.frameCount].initCallFrame(fn, vm.stack[vm.stackTop-fn.arity:vm.stackTop], ip)
							vm.frameCount++

							frame = vm.frames[vm.frameCount-1]
							chunk = *frame.fn.chunk
							ip = 0
						}
					case OBJ_NATIVE_FN:
						{
							switch native := obj.(type) {
							case *Log:
								{
									arg := vm.pop()
									native.Log(arg)
								}
							}

						}
					}
				} else {
					// .toString, __proto__, ...etc
				}

			}
		case OP_RETURN:
			{
				ip = frame.returnIp

				if vm.stackTop >= 2 {
					vm.stack[vm.stackTop-(len(frame.locals)+frame.fn.arity)] = vm.stack[vm.stackTop-1]
					vm.stackTop -= frame.fn.arity + len(frame.locals) - 1
				}

				vm.frameCount--

				frame = vm.frames[vm.frameCount-1]
				chunk = *frame.fn.chunk
			}
		case OP_EOF:
			{
				fmt.Printf("Done :) %s\n", time.Since(start))
				return
			}
		}
	}

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

	main := NewFunction(MAIN_FN_NAME, 0)

	err = Compile(ast, main)

	if err != nil {
		log.Fatalf("Failed to parse javascript, %e", err)
	}

	vm := NewVM()
	vm.call(main, 0)
	vm.run()
}
