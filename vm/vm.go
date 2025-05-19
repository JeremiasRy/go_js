package vm

import (
	"fmt"
	"math"
)

type CallFrame struct {
	fn *ObjFunction
	ip int
}

type VM struct {
	frames       []*CallFrame
	currentFrame *CallFrame
	stack        []Value
}

func NewVM(fn *ObjFunction) *VM {
	return &VM{
		frames:       []*CallFrame{},
		currentFrame: &CallFrame{fn: fn, ip: 0},
		stack:        []Value{},
	}
}

func (vm *VM) peek(distance int) Value {
	return vm.stack[len(vm.stack)-1-distance]
}

func (vm *VM) pop() Value {
	v := vm.stack[len(vm.stack)-1]
	vm.stack = vm.stack[:len(vm.stack)-1]
	return v
}

func (vm *VM) push(value Value) {
	vm.stack = append(vm.stack, value)
}

func (vm *VM) readByte() uint8 {
	code := vm.currentFrame.fn.chunk.code[vm.currentFrame.ip]
	vm.currentFrame.ip++
	return code
}

func (vm *VM) Run() {
	vm.currentFrame.fn.Debug()

	for {
		code := vm.readByte()
		fmt.Printf("%v\n", vm.stack)
		PrintCode(code)

		switch code {
		case OP_CONSTANT:
			{
				idx := vm.readByte()
				vm.push(vm.currentFrame.fn.chunk.constants[idx])
			}
		case OP_ADD:
			{
				b := math.Float64frombits(uint64(vm.pop()))
				a := math.Float64frombits(uint64(vm.pop()))

				fmt.Printf("%f + %f\n", a, b)

				vm.push(Value(math.Float64bits(a + b)))
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
				fmt.Printf("%f\n", math.Float64frombits(uint64(vm.stack[len(vm.stack)-1])))
				return
			}
		}
	}
}
