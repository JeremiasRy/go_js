package vm

import (
	"fmt"
)

func printCode(name string) {
	fmt.Printf("%-23s |\n", name)
}

func printConstant(chunk *Chunk, i int) int {
	constant := chunk.constants[chunk.code[i+1]]

	fmt.Printf("%-23s | %d -> %s\n", "OP_CONSTANT", chunk.code[i+1], constant.String())
	return 1
}

func printChunk(chunk *Chunk) {
	i := 0
	for i < len(chunk.code) {
		code := chunk.code[i]
		switch code {
		case OP_ADD:
			printCode(OpcodeNames[code])
		case OP_CONSTANT:
			offset := printConstant(chunk, i)
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
