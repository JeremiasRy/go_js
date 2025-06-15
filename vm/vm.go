package vm

import (
	"fmt"
	"go_js/parser"
	"log"
	"math"
	"time"
)

type CallFrame struct {
	fn     *ObjFunction
	locals []Value
	ip     int
}

func NewCallFrame(fn *ObjFunction, locals []Value) *CallFrame {
	return &CallFrame{fn: fn, locals: locals, ip: 0}
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
	frames     [64]*CallFrame
	frameCount uint8
	stack      [255]Value
	stackTop   uint8
	globals    []Value

	heap *Heap
}

func NewVM(heap *Heap) *VM {
	return &VM{frames: [FRAMES_MAX]*CallFrame{}, frameCount: 0, stack: [STACK_MAX]Value{}, stackTop: 0, heap: heap, globals: []Value{}}
}

func (vm *VM) call(fn *ObjFunction) error {
	if vm.frameCount == FRAMES_MAX {
		return fmt.Errorf("too many callframes")
	}
	locals := make([]Value, fn.arity)
	copy(locals, vm.stack[vm.stackTop-uint8(fn.arity):vm.stackTop])

	vm.frames[vm.frameCount] = NewCallFrame(fn, locals)
	vm.frameCount++

	vm.stackTop -= uint8(fn.arity)
	return nil
}

func (vm *VM) readByte() uint8 {
	cf := vm.currentFrame()
	code := cf.fn.chunk.code[cf.ip]
	cf.ip++
	return code
}

func (vm *VM) readInt32() int {
	cf := vm.currentFrame()
	i := int(cf.fn.chunk.code[cf.ip+3]) | int(cf.fn.chunk.code[cf.ip+2])<<8 | int(cf.fn.chunk.code[cf.ip+1])<<16 | int(cf.fn.chunk.code[cf.ip])<<24
	cf.ip += 4
	return i
}

func (vm *VM) readConstant() Value {
	cf := vm.currentFrame()
	code := cf.fn.chunk.code[cf.ip]
	cf.ip++
	return cf.fn.chunk.constants[code]
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
func (vm *VM) addLocal(v Value) {
	vm.frames[vm.frameCount-1].AddLocal(v)
}

func (vm *VM) getLocal(slot int) Value {
	return vm.currentFrame().GetLocal(slot)
}

func (vm *VM) currentFrame() *CallFrame {
	return vm.frames[vm.frameCount-1]
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
		printFrame(vm.currentFrame())
		for _, object := range h.objects {
			if object.Type() == OBJ_FUNCTION {
				println(object.String())
				printChunk(object.(*ObjFunction).chunk)
				println()
			}
		}
	}
	start := time.Now()
	for {
		code := vm.readByte()
		if DEBUG {
			println("Current context: ", vm.currentFrame().fn.name)
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
				vm.push(vm.readConstant())
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
		case OP_JUMP_IF_FALSE:
			{
				value := vm.pop()
				jump := vm.readInt32()
				if !AsBoolean(value) {
					vm.currentFrame().ip += jump
				}
			}
		case OP_DEFINE_GLOBAL:
			{
				variable := vm.pop()
				vm.addGlobal(variable)

			}
		case OP_GET_GLOBAL:
			{
				global := vm.readByte()
				vm.push(vm.getGlobal(int(global)))
			}
		case OP_DEFINE_LOCAL:
			{
				variable := vm.pop()
				vm.addLocal(variable)
			}
		case OP_GET_LOCAL:
			{
				slot := vm.readByte()
				vm.push(vm.getLocal(int(slot)))
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
							vm.call(fn)
						}
					}
				} else {
					// .toString, __proto__, ...etc
				}

			}
		case OP_RETURN:
			{
				vm.frameCount--
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
	vm.call(main)
	vm.run()
}
