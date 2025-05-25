package vm

import (
	"encoding/json"
	"fmt"
	"go_js/parser"
	"math"
	"strconv"
	"strings"
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

func numValueToString(v Value) string {
	return strconv.FormatFloat(math.Float64frombits(uint64(v)), 'f', -1, 64)
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
	v := vm.stack[vm.stackTop-1]
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

func (vm *VM) concatenate(a Value, b Value) (Value, error) {
	aType := getObjType(a)
	bType := getObjType(b)

	if aType == OBJ_NUMBER && bType == OBJ_NUMBER {
		a := math.Float64frombits(uint64(a))
		b := math.Float64frombits(uint64(b))

		return Value(math.Float64bits(a + b)), nil
	}

	if aType == OBJ_STRING && bType == OBJ_STRING {
		a := asObj[ObjString](a)
		b := asObj[ObjString](b)

		res := a.s + b.s

		encoded, err := vm.intern(res).Encode()
		if err != nil {
			return 0, fmt.Errorf("failed to encode string -%e-", err)
		}
		return encoded, nil
	}

	if aType == OBJ_STRING && bType == OBJ_NUMBER {
		a := asObj[ObjString](a).s
		b := numValueToString(b)

		res := a + b

		encoded, err := vm.intern(res).Encode()
		if err != nil {
			return 0, fmt.Errorf("failed to encode string -%e-", err)
		}
		return encoded, nil
	}

	if aType == OBJ_NUMBER && bType == OBJ_STRING {
		a := numValueToString(a)
		b := asObj[ObjString](b).s

		res := a + b

		encoded, err := vm.intern(res).Encode()
		if err != nil {
			return 0, fmt.Errorf("failed to encode string -%e-", err)
		}
		return encoded, nil
	}

	return 0, fmt.Errorf("no suitable operation found")
}

func (vm *VM) DebugStack() {
	debugStack := []string{}

	for _, v := range vm.stack[0:vm.stackTop] {
		switch getObjType(v) {
		case OBJ_STRING:
			{
				debugStack = append(debugStack, v.String())
			}
		case OBJ_NUMBER:
			{
				debugStack = append(debugStack, v.String())
			}
		}
	}
	fmt.Printf("[%v]\n", strings.Join(debugStack, " | "))
}

func (vm *VM) Run() {
	vm.currentFrame().fn.chunk.PrintCode()
	for {
		code := vm.readByte()
		vm.DebugStack()
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

				c, err := vm.concatenate(a, b)

				if err != nil {
					// runtime error
				}
				vm.push(c)
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
				println("Done :)")
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
