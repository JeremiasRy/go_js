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
const DEBUG = true

type VM struct {
	frames     []CallFrame
	frameCount int
	stack      []Value
	stackTop   int
	globals    []Value

	heap *Heap
}

func NewVM(heap *Heap) *VM {
	frames := make([]CallFrame, FRAMES_MAX)
	stack := make([]Value, STACK_MAX)
	return &VM{frames: frames, frameCount: 0, stack: stack, stackTop: 0, heap: heap, globals: []Value{}}
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

func (vm *VM) getHeapObject(v Value) Object {
	return vm.heap.GetObject(v.getRegister())
}

func (vm *VM) concatenate(a, b Value) Value {

	if a.isObject() && b.isObject() {
		aObj := vm.getHeapObject(a)
		bObj := vm.getHeapObject(b)

		if aObj.Type() == OBJ_STRING && bObj.Type() == OBJ_STRING {
			res := aObj.(ObjString) + bObj.(ObjString)
			return vm.heap.AllocateString(string(res))
		}
	}
	if a.isObject() && !b.isObject() {
		aObj := vm.getHeapObject(a)

		if aObj.Type() == OBJ_STRING {
			res := aObj.(ObjString) + ObjString(b.String())

			return vm.heap.AllocateString(string(res))
		}
	}

	if !a.isObject() && b.isObject() {
		bObj := vm.getHeapObject(b)

		if bObj.Type() == OBJ_STRING {
			res := ObjString(a.String()) + bObj.(ObjString)

			return vm.heap.AllocateString(string(res))
		}
	}

	return Value(math.Float64bits(a.asNumber() + b.asNumber()))
}

func (vm *VM) subtract(a, b Value) Value {

	if a.isObject() || b.isObject() {
		if a.isObject() && b.isObject() {
			// todo
		} else if !a.isObject() && b.isObject() {
			switch vm.getHeapObject(b).(type) {
			case ObjString:
				{
					return EncodeNaN()
				}
			}
		} else if a.isObject() && !b.isObject() {
			switch vm.getHeapObject(a).(type) {
			case ObjString:
				{
					return EncodeNaN()
				}
			}
		}

	}

	return ValueFromFloat64(a.asNumber() - b.asNumber())
}

func (vm *VM) run() {
	if DEBUG {
		printFrame(vm.currentFramePointer())
		for _, object := range h.objects {
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
				if v.isObject() {
					obj := vm.getHeapObject(v)
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
		case OP_JUMP_IF_FALSE:
			{
				value := vm.pop()
				jump := int(chunk.code[ip+3]) | int(chunk.code[ip+2])<<8 | int(chunk.code[ip])<<16 | int(chunk.code[ip])<<24
				ip += 4
				if !AsBoolean(value) {
					ip += jump
				}
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
				variable := vm.pop()
				frame.AddLocal(variable)
			}
		case OP_GET_LOCAL:
			{
				slot := chunk.code[ip]
				vm.push(frame.GetLocal(int(slot)))
				ip++
			}
		case OP_CALL:
			{
				callee := vm.pop()

				if callee.isObject() {
					obj := vm.heap.GetObject(callee.getRegister())
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
					}
				} else {
					// .toString, __proto__, ...etc
				}

			}
		case OP_RETURN:
			{
				ip = vm.frames[vm.frameCount-1].returnIp
				vm.stack[vm.stackTop-2] = vm.stack[vm.stackTop-1]
				vm.stackTop -= vm.frames[vm.frameCount-1].fn.arity - 1
				vm.stackTop--

				vm.frameCount--
				frame = vm.frames[vm.frameCount-1]
				chunk = *frame.fn.chunk
			}
		case OP_EOF:
			{
				fmt.Printf("Done :) stack top: %f time: %s\n", vm.stack[vm.stackTop-1].asNumber(), time.Since(start))
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

	heap := NewHeap()

	if DEBUG {
		initDebugger(heap)
	}

	main := NewFunction(MAIN_FN_NAME, 0)

	err = Compile(ast, heap, main)

	if err != nil {
		log.Fatalf("Failed to parse javascript, %e", err)
	}

	vm := NewVM(heap)
	vm.call(main, 0)
	vm.run()
}
