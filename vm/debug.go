package vm

import (
	"fmt"
)

func printCode(name string) {
	fmt.Printf("%-28s |\n", name)
}

func printConstant(chunk *Chunk, i int) int {
	constant := chunk.constants[chunk.code[i+1]]

	fmt.Printf("%-28s | %s\n", "OP_CONSTANT", constant)
	return 1
}

func printGetVariable(chunk *Chunk, i int, code uint8) int {
	fmt.Printf("%-28s | %v\n", OpcodeNames[code], chunk.code[i+1])
	return 1
}

func printJump(chunk *Chunk, i int, name string) int {
	fmt.Printf("%-28s | %d\n", name, uint32(chunk.code[i+4])|(uint32(chunk.code[i+3])<<8)|(uint32(chunk.code[i+2])<<16)|(uint32(chunk.code[i+1])<<24))
	return 4
}

func printGet(chunk *Chunk, i int, name string) int {
	fmt.Printf("%-28s | %d %s\n", name, chunk.code[i+1], chunk.constants[chunk.code[i+2]])
	return 2
}

func printSet(chunk *Chunk, i int, name string) int {
	fmt.Printf("%-28s | %d\n", name, chunk.code[i+1])
	return 1
}

func printChunk(chunk *Chunk) {
	i := 0
	for i < len(chunk.code) {

		code := chunk.code[i]
		switch code {
		case OP_ADD, OP_SUBTRACT, OP_DIVIDE, OP_MULTIPLY, OP_LESS_THAN_EQUAL, OP_CALL, OP_RETURN, OP_END_OF_FN, OP_POP, OP_EOF:
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
		case OP_GET_LOCAL_OBJECT_MEMBER, OP_GET_GLOBAL_OBJECT_MEMBER:
			offset := printGet(chunk, i, OpcodeNames[code])
			i += offset
		case OP_SET_LOCAL, OP_SET_GLOBAL:
			offset := printSet(chunk, i, OpcodeNames[code])
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
