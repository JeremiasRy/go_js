package vm

import (
	"fmt"
	"go_js/parser"
	"log"
)

type CallFrame struct {
	fn *ObjFunction
	ip int
}

func (cf *CallFrame) readByte() uint8 {
	code := cf.fn.chunk.code[cf.ip]
	cf.ip++
	return code
}

func (cf *CallFrame) readConstant() Value {
	code := cf.fn.chunk.code[cf.ip]
	cf.ip++
	return cf.fn.chunk.constants[code]
}

func NewCallFrame(fn *ObjFunction) *CallFrame {
	return &CallFrame{fn: fn, ip: 0}
}

const STACK_MAX = 255
const FRAMES_MAX = 64
const DEBUG = true

type VM struct {
	frames     [64]*CallFrame
	frameCount uint8
	stack      [255]Value
	stackTop   uint8

	strings map[string]ObjString
	heap    *Heap
}

func (vm *VM) call(fn *ObjFunction) error {
	if vm.frameCount == FRAMES_MAX {
		return fmt.Errorf("too many callframes")
	}
	vm.frames[vm.frameCount] = NewCallFrame(fn)
	vm.frameCount++
	return nil
}

func (vm *VM) readByte() uint8 {
	return vm.frames[vm.frameCount-1].readByte()
}

func (vm *VM) readConstant() Value {
	return vm.frames[vm.frameCount-1].readConstant()
}

func (vm *VM) push(v Value) {
	vm.stack[vm.stackTop] = v
	vm.stackTop++
}

func (vm *VM) pop() Value {
	vm.stackTop--
	return vm.stack[vm.stackTop]
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
			if str, found := vm.strings[string(res)]; found {
				register := vm.heap.Allocate(str)
				return EncodeObject(register)
			} else {
				register := vm.heap.Allocate(res)
				return EncodeObject(register)
			}
		}
	}
	if a.isObject() && !b.isObject() {
		aObj := vm.getHeapObject(a)

		if aObj.Type() == OBJ_STRING {
			res := aObj.(ObjString) + ObjString(b.String())

			if str, found := vm.strings[string(res)]; found {
				register := vm.heap.Allocate(str)
				return EncodeObject(register)
			} else {
				register := vm.heap.Allocate(res)
				return EncodeObject(register)
			}
		}
	}

	if !a.isObject() && b.isObject() {
		bObj := vm.getHeapObject(b)

		if bObj.Type() == OBJ_STRING {
			res := ObjString(a.String()) + bObj.(ObjString)

			if str, found := vm.strings[string(res)]; found {
				register := vm.heap.Allocate(str)
				return EncodeObject(register)
			} else {
				register := vm.heap.Allocate(res)
				return EncodeObject(register)
			}
		}
	}
	return Value(a.asNumber() + b.asNumber())
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
		printFrame(vm.frames[vm.frameCount-1])
	}

	for {
		code := vm.readByte()

		if DEBUG {
			print("[")
			for _, v := range vm.stack[:vm.stackTop] {
				if v.isObject() {
					obj := vm.getHeapObject(v)
					switch obj := obj.(type) {
					case ObjString:
						print("\"" + obj + "\" | ")
					}
				} else {
					print(v.String() + " | ")
				}

			}
			println("]")
			println(OpcodeNames[code])
		}

		switch code {
		case OP_CONSTANT:
			vm.push(vm.readConstant())
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
		case OP_EOF:
			{
				println("Done :)")
				return
			}
		}
	}

}

func NewVM(heap *Heap, strings map[string]ObjString) *VM {
	return &VM{frames: [FRAMES_MAX]*CallFrame{}, frameCount: 0, stack: [STACK_MAX]Value{}, stackTop: 0, strings: strings, heap: heap}
}

func Interpret(source []byte) {
	ast, err := parser.GetAst(source, nil, 0)

	if err != nil {
		log.Fatalf("Failed to parse javascript, %e", err)
	}

	parser.PrintNode(ast)
	chunk := NewChunk()
	heap := NewHeap()
	strings := map[string]ObjString{}

	err = Compile(ast, heap, chunk, strings)

	if err != nil {
		log.Fatalf("Failed to parse javascript, %e", err)
	}

	vm := NewVM(heap, strings)
	main := &ObjFunction{name: "main", chunk: chunk}

	vm.call(main)
	vm.run()
}
