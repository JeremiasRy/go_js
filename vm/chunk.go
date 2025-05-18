package vm

import (
	"fmt"
	"strconv"
)

const QNAN uint64 = 0x7ffc000000000000
const SIGN uint64 = 0x8000000000000000

const (
	TAG_UNDEFINED uint64 = QNAN | 1
	TAG_NILL      uint64 = QNAN | 2
	TAG_TRUE      uint64 = QNAN | 3
	TAG_FALSE     uint64 = QNAN | 4
	TAG_OBJ       uint64 = QNAN | SIGN
)

func isNumber(value uint64) bool {
	return value&QNAN != QNAN
}

func isUndefined(value uint64) bool {
	return value == TAG_UNDEFINED
}

func isNill(value uint64) bool {
	return value == TAG_NILL
}

func isBool(value uint64) bool {
	return value|3 == TAG_TRUE
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
	constants []uint64
}

func NewChunk() *Chunk {
	return &Chunk{
		code:      []uint8{},
		constants: []uint64{},
	}
}

func (c *Chunk) EmitByte(b uint8) {
	c.code = append(c.code, b)
}

func (c *Chunk) AddConstant(v uint64) uint8 {
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
