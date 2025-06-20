package vm

import (
	"fmt"
)

func printCode(name string) {
	fmt.Printf("%-23s |\n", name)
}

func printConstant(chunk *Chunk, i int) int {
	constant := chunk.constants[chunk.code[i+1]]

	fmt.Printf("%-23s | %s\n", "OP_CONSTANT", constant)
	return 1
}

func printGetVariable(chunk *Chunk, i int, code uint8) int {
	fmt.Printf("%-23s | %v\n", OpcodeNames[code], chunk.code[i+1])
	return 1
}

func printJump(chunk *Chunk, i int, name string) int {
	fmt.Printf("%-23s | %d\n", name, uint32(chunk.code[i+4])|(uint32(chunk.code[i+3])<<8)|(uint32(chunk.code[i+2])<<16)|(uint32(chunk.code[i+1])<<24))
	return 4
}

func printGetMember(chunk *Chunk, i int) int {
	fmt.Printf("%-23s | %d %s\n", "OP_GET_OBJECT_MEMBER", chunk.code[i+1], chunk.constants[chunk.code[i+2]])
	return 2
}

func printChunk(chunk *Chunk) {
	i := 0
	for i < len(chunk.code) {

		code := chunk.code[i]
		switch code {
		case OP_ADD, OP_SUBTRACT, OP_DIVIDE, OP_MULTIPLY, OP_LESS_THAN_EQUAL, OP_CALL, OP_RETURN:
			printCode(OpcodeNames[code])
		case OP_CONSTANT:
			offset := printConstant(chunk, i)
			i += offset
		case OP_DEFINE_LOCAL, OP_DEFINE_GLOBAL:
			printCode(OpcodeNames[code])
		case OP_GET_LOCAL, OP_GET_GLOBAL:
			offset := printGetVariable(chunk, i, code)
			i += offset
		case OP_JUMP_IF_FALSE, OP_JUMP:
			offset := printJump(chunk, i, OpcodeNames[code])
			i += offset
		case OP_GET_OBJECT_MEMBER:
			offset := printGetMember(chunk, i)
			i += offset
		}

		i++
	}
}

func printFrame(cf *CallFrame) {
	println("<fn " + cf.fn.name + ">")
	printChunk(cf.fn.chunk)
	println()
}
