package vm

import (
	"encoding/json"
	"fmt"
	"go_js/parser"
	"math"
)

type CallFrame struct {
	fn *ObjFunction
	ip int
}

const STACK_MAX = 255
const FRAMES_MAX = 64

type VM struct {
	frames     [64]*CallFrame
	frameCount uint8
	stack      [255]Value
	stackTop   uint8

	strings map[string]*ObjString
	refs    []any
}

func (vm *VM) intern(s string) *ObjString {
	if obj, found := vm.strings[s]; found {
		return obj
	}

	objStr := &ObjString{obj: Obj{_type: OBJ_STRING}, s: s}
	vm.strings[s] = objStr
	return objStr
}

func NewVM() *VM {
	return &VM{
		frames:     [FRAMES_MAX]*CallFrame{},
		frameCount: 0,
		stack:      [STACK_MAX]Value{},
		stackTop:   0,
		refs:       []any{},

		strings: make(map[string]*ObjString),
	}
}

func (vm *VM) call(fn *ObjFunction) bool {
	if vm.frameCount == FRAMES_MAX {
		// runtime error
		return false
	}
	frame := &CallFrame{}
	frame.fn = fn
	frame.ip = 0

	vm.frames[vm.frameCount] = frame
	vm.frameCount++

	return true
}

func (vm *VM) currentFrame() *CallFrame {
	return vm.frames[vm.frameCount-1]
}

func (vm *VM) peek(distance uint8) Value {
	return vm.stack[vm.stackTop-1-distance]
}

func (vm *VM) pop() Value {
	v := vm.stack[vm.stackTop]
	vm.stackTop--
	return v
}

func (vm *VM) push(value Value) {
	vm.stack[vm.stackTop] = value
	vm.stackTop++
}

func (vm *VM) readByte() uint8 {
	frame := vm.currentFrame()
	code := frame.fn.chunk.code[frame.ip]
	frame.ip++
	return code
}

func (vm *VM) Run() {
	vm.currentFrame().fn.Debug()

	for {
		code := vm.readByte()
		fmt.Printf("%v\n", vm.stack)
		PrintCode(code)

		switch code {
		case OP_CONSTANT:
			{
				idx := vm.readByte()
				vm.push(vm.currentFrame().fn.chunk.constants[idx])
			}
		case OP_ADD:
			{

				b := vm.pop()
				a := vm.pop()

				fmt.Printf("a: %v \nb: %v\n", a, b)

				switch getObjType(b) {
				case OBJ_STRING:
					{
						bStr := asObj[ObjString](b)
						fmt.Printf("%v\n", bStr)
					}
				}

				switch getObjType(a) {
				case OBJ_STRING:
					{
						aStr := asObj[ObjString](a)
						fmt.Printf("%v\n", aStr)
					}
				}

				//fmt.Printf("%f + %f\n", a, b)

				//vm.push(Value(math.Float64bits(a + b)))
			}
		case OP_SUBTRACT:
			{
				b := math.Float64frombits(uint64(vm.pop()))
				a := math.Float64frombits(uint64(vm.pop()))

				fmt.Printf("%f - %f\n", a, b)

				vm.push(Value(math.Float64bits(a - b)))
			}
		case OP_MULTIPLY:
			{
				b := math.Float64frombits(uint64(vm.pop()))
				a := math.Float64frombits(uint64(vm.pop()))

				fmt.Printf("%f * %f\n", a, b)

				vm.push(Value(math.Float64bits(a * b)))
			}
		case OP_DIVIDE:
			{
				b := math.Float64frombits(uint64(vm.pop()))
				a := math.Float64frombits(uint64(vm.pop()))

				fmt.Printf("%f / %f\n", a, b)

				vm.push(Value(math.Float64bits(a / b)))
			}
		case OP_EOF:
			{
				return
			}
		}
	}
}

func Interpret(source []byte) error {
	ast, err := parser.GetAst(source, nil, 0)
	if err != nil {
		return err
	}

	bJson, _ := json.MarshalIndent(ast, "", "  ")
	println(string(bJson))

	vm := NewVM()
	fn := Compile(ast, vm)
	vm.call(fn)
	vm.Run()

	return nil
}
