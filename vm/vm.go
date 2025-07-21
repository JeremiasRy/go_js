package vm

import (
	"fmt"
	"go_js/parser"
	"log"
	"math"
)

type CallFrame struct {
	fn         *ObjFunction
	locals     []Value
	stackStart int
	returnIp   int
}

func NewCallFrame(fn *ObjFunction, locals []Value) *CallFrame {
	return &CallFrame{fn: fn, locals: locals, returnIp: 0}
}

func (cf *CallFrame) initCallFrame(fn *ObjFunction, stackStart int, locals []Value, returnIp int) {
	cf.fn = fn
	cf.locals = locals
	cf.returnIp = returnIp
	cf.stackStart = stackStart
}

func (cf *CallFrame) addLocal(v Value) int {
	cf.locals = append(cf.locals, v)
	return len(cf.locals) - 1
}

func (cf *CallFrame) getLocal(index int) Value {
	return cf.locals[index]
}

const STACK_MAX = math.MaxUint8
const FRAMES_MAX = 64
const DEBUG = true

var HEAP *Heap = NewHeap()

type VM struct {
	frames     []CallFrame
	frameCount int
	stack      []Value
	stackTop   int
	globals    []Value

	openUpvalues *ObjUpvalue
}

func NewVM() *VM {
	frames := make([]CallFrame, FRAMES_MAX)
	stack := make([]Value, STACK_MAX)
	return &VM{frames: frames, frameCount: 0, stack: stack, stackTop: 0, globals: []Value{}, openUpvalues: nil}
}

func (vm *VM) call(fn *ObjFunction, returnIp int) error {
	if vm.frameCount == FRAMES_MAX {
		return fmt.Errorf("too many callframes")
	}

	vm.frames[vm.frameCount].initCallFrame(fn, vm.stackTop, vm.stack[vm.stackTop-fn.arity:vm.stackTop], returnIp)
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

func (vm *VM) run() error {
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
		case OP_POP:
			{
				vm.pop()
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

				if !AsBoolean(value) {
					ip = jump
				} else {
					ip += 4
				}
			}
		case OP_JUMP:
			{
				jump := int(chunk.code[ip+3]) | int(chunk.code[ip+2])<<8 | int(chunk.code[ip])<<16 | int(chunk.code[ip])<<24
				ip = jump
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
		case OP_SET_GLOBAL:
			{
				global := chunk.code[ip]
				vm.globals[global] = vm.pop()
				ip++
			}
		case OP_DEFINE_LOCAL:
			{
				variable := vm.peek()
				frame.addLocal(variable)
			}
		case OP_GET_LOCAL:
			{
				vm.push(frame.getLocal(int(chunk.code[ip])))
				ip++
			}
		case OP_SET_LOCAL:
			{
				frame.locals[chunk.code[ip]] = vm.pop()
				ip++
			}
		case OP_GET_UPVALUE:
			{
				vm.push(*frame.fn.upvalues[chunk.code[ip]].location)
				ip++
			}
		case OP_SET_UPVALUE:
			{
				value := vm.pop()
				frame.fn.upvalues[chunk.code[ip]].location = &value
				ip++
			}
		case OP_DEFINE_OBJECT_MEMBER:
			{
				member := vm.pop()
				value := vm.pop()
				hash := vm.peek()

				_, hashObject := hash.getObject()
				hashObject.(*ObjHash).SetMember(member.String(), value)
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
				fmt.Printf("%d, %s\n", slot, member)

				_, obj := frame.getLocal(slot).getObject()

				value := obj.(*ObjHash).GetMember(member.String())
				vm.push(value)
				ip += 2
			}
		case OP_PUSH_UNDEFINED:
			{
				vm.push(EncodedUndefined())
			}
		case OP_CLOSURE:
			{
				closure := vm.pop()
				_, obj := closure.getObject()

				fn := obj.(*ObjFunction)
				upvalueCount := chunk.code[ip]
				fn.upvalues = make([]*ObjUpvalue, upvalueCount)
				for i := 0; i < int(upvalueCount); i++ {
					isLocalByte := chunk.code[ip+1]
					slot := chunk.code[ip+2]
					ip = ip + 2

					if isLocalByte > 0 {
						var prevUpvalue *ObjUpvalue
						upvalue := vm.openUpvalues
						local := &frame.locals[slot]
						stackLocation := frame.stackStart + int(slot)

						for upvalue != nil && upvalue.stackLocation > stackLocation {
							prevUpvalue = upvalue
							upvalue = upvalue.next
						}

						if upvalue != nil && upvalue.stackLocation == stackLocation {
							fn.upvalues[i] = upvalue
						} else {

							newUpvalue := &ObjUpvalue{location: local, closed: EncodeNil(), next: nil, stackLocation: stackLocation}

							if prevUpvalue == nil {
								vm.openUpvalues = newUpvalue
							} else {
								prevUpvalue.next = newUpvalue
							}
							fn.upvalues[i] = newUpvalue
						}

					} else {
						fn.upvalues[i] = frame.fn.upvalues[slot]
					}
				}
				ip++
			}
		case OP_CALL:
			{
				callee := vm.pop()

				if callee.isObject() {
					_, obj := callee.getObject()
					switch fn := obj.(type) {
					case *ObjFunction:
						{
							vm.frames[vm.frameCount-1].locals = frame.locals

							vm.frames[vm.frameCount].initCallFrame(fn, vm.stackTop, vm.stack[vm.stackTop-fn.arity:vm.stackTop], ip)
							vm.frameCount++

							frame = vm.frames[vm.frameCount-1]
							chunk = *frame.fn.chunk
							ip = 0
						}
					case *Log:
						{
							arg := vm.pop()
							fn.Log(arg)
							vm.push(EncodedUndefined())
						}
					case *Clock:
						{
							vm.push(fn.Clock())
						}
					}

				} else {
					return fmt.Errorf("%s is not a function", callee)
				}

			}
		case OP_RETURN:
			{
				ip = frame.returnIp
				value := vm.pop()

				for vm.openUpvalues != nil && vm.openUpvalues.stackLocation >= frame.stackStart {
					upvalue := vm.openUpvalues
					upvalue.Close()
					vm.openUpvalues = upvalue.next
				}

				vm.stackTop -= max(len(frame.locals), frame.fn.arity)

				vm.push(value)
				vm.frameCount--

				frame = vm.frames[vm.frameCount-1]
				chunk = *frame.fn.chunk
			}
		case OP_EOF:
			{
				fmt.Printf("Thanks!\n")
				return nil
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
	err = vm.run()

	if err != nil {
		log.Fatalf("runtime error: %s", err.Error())
	}
}
