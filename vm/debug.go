package vm

import (
	"fmt"
)

var heap *Heap

func initDebugger(h *Heap) {
	heap = h
}

func printCode(name string) {
	fmt.Printf("%-23s |\n", name)
}

func printConstant(chunk *Chunk, i int) int {
	constant := chunk.constants[chunk.code[i+1]]
	var output string
	if constant.isObject() {
		output = heap.GetObject(constant.getRegister()).String()
	} else {
		output = constant.String()
	}

	fmt.Printf("%-23s | %d -> %s\n", "OP_CONSTANT", chunk.code[i+1], output)
	return 1
}

func printGetVariable(chunk *Chunk, i int) int {
	fmt.Printf("%-23s | %v\n", "OP_GET_VARIABLE", chunk.code[i+1])
	return 1
}

func printChunk(chunk *Chunk) {
	i := 0
	for i < len(chunk.code) {

		code := chunk.code[i]
		switch code {
		case OP_ADD, OP_SUBTRACT, OP_DIVIDE, OP_MULTIPLY, OP_CALL, OP_RETURN:
			printCode(OpcodeNames[code])
		case OP_CONSTANT:
			offset := printConstant(chunk, i)
			i += offset
		case OP_DEFINE_VARIABLE:
			printCode(OpcodeNames[OP_DEFINE_VARIABLE])
		case OP_GET_VARIABLE:
			offset := printGetVariable(chunk, i)
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
