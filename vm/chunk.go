package vm

import (
	"fmt"
	"math"
	"strconv"
	"unsafe"
)

type Value uint64

const (
	QNAN          Value = 0x7ffc000000000000
	SIGN          Value = 0x8000000000000000
	ENCODE_MASK   Value = math.MaxUint64 ^ (QNAN | SIGN)
	TAG_UNDEFINED Value = Value(QNAN | 1)
	TAG_NILL      Value = Value(QNAN | 2)
	TAG_TRUE      Value = Value(QNAN | 3)
	TAG_FALSE     Value = Value(QNAN | 4)
	TAG_OBJ       Value = Value(QNAN | SIGN)
)

func isNumber(value Value) bool {
	return value&QNAN != QNAN
}

func isUndefined(value Value) bool {
	return value == TAG_UNDEFINED
}

func isNill(value Value) bool {
	return value == TAG_NILL
}

func isBool(value Value) bool {
	return value|3 == TAG_TRUE
}

func isObj(value Value) bool {
	return value&(SIGN|QNAN) == (SIGN | QNAN)
}

func getObjType(value Value) ObjType {
	if isObj(value) {
		payload := uint64(value & ENCODE_MASK)
		return (*Obj)(unsafe.Pointer(uintptr(payload)))._type
	}
	return OBJ_NOT_A_OBJ
}

func asObj[ReturnType ObjLike](value Value) *ReturnType {
	payload := uint64(value & ENCODE_MASK)
	return (*ReturnType)(unsafe.Pointer(uintptr(payload)))
}

func getNanPayload(value Value) uint64 {
	return uint64(value & ENCODE_MASK)
}

func DebugValue(v uint64) {
	fmt.Printf("%v\n", v)
}

const (
	OP_CONSTANT uint8 = iota
	OP_ADD
	OP_SUBTRACT
	OP_MULTIPLY
	OP_DIVIDE
	OP_NILL
	OP_UNDEFINED
	OP_TRUE
	OP_FALSE
	OP_EQUALS
	OP_EQUALS_STRICT
	OP_EOF
)

type Chunk struct {
	code      []uint8
	constants []Value
}

func NewChunk() *Chunk {
	return &Chunk{
		code:      []uint8{},
		constants: []Value{},
	}
}

func (c *Chunk) EmitByte(b uint8) {
	c.code = append(c.code, b)
}

func (c *Chunk) AddConstant(v Value) uint8 {
	c.constants = append(c.constants, v)
	return uint8(len(c.constants) - 1)
}

func (c *Chunk) PrintCode() {
	i := 0
	for i < len(c.code) {
		code := c.code[i]
		switch code {
		case OP_CONSTANT:
			{
				fmt.Printf("%-14s | %s\n", "OP_CONSTANT", strconv.Itoa(int(c.code[i+1])))
				i++
			}
		case OP_ADD:
			{
				fmt.Printf("%-14s |\n", "OP_ADD")
			}
		case OP_DIVIDE:
			{
				fmt.Printf("%-14s |\n", "OP_DIVIDE")
			}
		case OP_MULTIPLY:
			{
				fmt.Printf("%-14s |\n", "OP_MULTIPLY")
			}
		case OP_SUBTRACT:
			{
				fmt.Printf("%-14s |\n", "OP_SUBTRACT")
			}
		}
		i++
	}
	println()
	println()
}

func PrintCode(c uint8) {
	switch c {
	case OP_CONSTANT:
		{
			fmt.Printf("%-14s \n", "OP_CONSTANT")

		}
	case OP_ADD:
		{
			fmt.Printf("%-14s \n", "OP_ADD")
		}
	case OP_DIVIDE:
		{
			fmt.Printf("%-14s \n", "OP_DIVIDE")
		}
	case OP_MULTIPLY:
		{
			fmt.Printf("%-14s \n", "OP_MULTIPLY")
		}
	case OP_SUBTRACT:
		{
			fmt.Printf("%-14s \n", "OP_SUBTRACT")
		}
	}
}
