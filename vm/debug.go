package vm

import (
	"fmt"
)

var OpcodeNames = map[uint8]string{
	OP_CONSTANT:                 "OP_CONSTANT",
	OP_POP:                      "OP_POP",
	OP_ADD:                      "OP_ADD",
	OP_SUBTRACT:                 "OP_SUBTRACT",
	OP_MULTIPLY:                 "OP_MULTIPLY",
	OP_DIVIDE:                   "OP_DIVIDE",
	OP_NILL:                     "OP_NILL",
	OP_UNDEFINED:                "OP_UNDEFINED",
	OP_TRUE:                     "OP_TRUE",
	OP_FALSE:                    "OP_FALSE",
	OP_EQUALS:                   "OP_EQUALS",
	OP_STRICT_EQUALS:            "OP_STRICT_EQUALS",
	OP_LESS_THAN_EQUAL:          "OP_LESS_THAN_EQUAL",
	OP_GET_LOCAL:                "OP_GET_LOCAL",
	OP_SET_LOCAL:                "OP_SET_LOCAL",
	OP_DEFINE_LOCAL:             "OP_DEFINE_LOCAL",
	OP_GET_GLOBAL:               "OP_GET_GLOBAL",
	OP_SET_GLOBAL:               "OP_SET_GLOBAL",
	OP_DEFINE_GLOBAL:            "OP_DEFINE_GLOBAL",
	OP_CLOSE_UPVALUES:           "OP_CLOSE_UPVALUES",
	OP_GET_UPVALUE:              "OP_GET_UPVALUE",
	OP_SET_UPVALUE:              "OP_SET_UPVALUE",
	OP_CLOSURE:                  "OP_CLOSURE",
	OP_CALL:                     "OP_CALL",
	OP_RETURN:                   "OP_RETURN",
	OP_END_OF_FN:                "OP_END_OF_FN",
	OP_TEMPLATE_LITERAL:         "OP_TEMPLATE_LITERAL",
	OP_JUMP_IF_FALSE:            "OP_JUMP_IF_FALSE",
	OP_JUMP:                     "OP_JUMP",
	OP_DEFINE_OBJECT_MEMBER:     "OP_DEFINE_OBJECT_MEMBER",
	OP_SET_LOCAL_OBJECT_MEMBER:  "OP_SET_LOCAL_OBJECT_MEMBER",
	OP_GET_LOCAL_OBJECT_MEMBER:  "OP_GET_LOCAL_OBJECT_MEMBER",
	OP_SET_GLOBAL_OBJECT_MEMBER: "OP_SET_GLOBAL_OBJECT_MEMBER",
	OP_GET_GLOBAL_OBJECT_MEMBER: "OP_GET_GLOBAL_OBJECT_MEMBER",
	OP_PUSH_UNDEFINED:           "OP_PUSH_UNDEFINED",
	OP_EOF:                      "OP_EOF",
}

func printCode(name string, i int) {
	fmt.Printf("%04d - %-32s |\n", i, name)
}

func printConstant(chunk *Chunk, i int) int {
	constant := chunk.constants[chunk.code[i+1]]

	fmt.Printf("%04d - %-32s | %s\n", i, "OP_CONSTANT", constant)
	return 1
}

func printGetVariable(chunk *Chunk, i int, code uint8) int {
	fmt.Printf("%04d - %-32s | %v\n", i, OpcodeNames[code], chunk.code[i+1])
	return 1
}

func printJump(chunk *Chunk, i int, name string) int {
	fmt.Printf("%04d - %-32s | %d\n", i, name, uint32(chunk.code[i+4])|(uint32(chunk.code[i+3])<<8)|(uint32(chunk.code[i+2])<<16)|(uint32(chunk.code[i+1])<<24))
	return 4
}

func printGet(chunk *Chunk, i int, name string) int {
	fmt.Printf("%04d - %-32s | %d %s\n", i, name, chunk.code[i+1], chunk.constants[chunk.code[i+2]])
	return 2
}

func printSet(chunk *Chunk, i int, name string) int {
	fmt.Printf("%04d - %-32s | %d\n", i, name, chunk.code[i+1])
	return 1
}

func printClosure(chunk *Chunk, i int, name string) int {
	upvalueCount := chunk.code[i+1]

	fmt.Printf("%04d - %-32s | upvalues %d\n", i, name, upvalueCount)

	i++

	for range upvalueCount {
		isLocal := chunk.code[i+1]
		slot := chunk.code[i+2]

		str := ""

		if isLocal > 0 {
			str = "local"
		} else {
			str = "upvalue"
		}

		fmt.Printf("%04d   %-32s | %s %d\n", i+2, "", str, slot)
		i = i + 2
	}

	return 2*int(upvalueCount) + 1
}

func printChunk(chunk *Chunk) {
	i := 0
	for i < len(chunk.code) {

		code := chunk.code[i]
		switch code {
		case OP_ADD, OP_SUBTRACT, OP_DIVIDE, OP_MULTIPLY, OP_LESS_THAN_EQUAL, OP_CALL, OP_RETURN, OP_END_OF_FN, OP_POP, OP_DEFINE_LOCAL, OP_DEFINE_GLOBAL, OP_CLOSE_UPVALUES, OP_DEFINE_OBJECT_MEMBER, OP_EOF:
			printCode(OpcodeNames[code], i)
		case OP_CONSTANT:
			offset := printConstant(chunk, i)
			i += offset
		case OP_GET_LOCAL, OP_GET_GLOBAL, OP_SET_UPVALUE, OP_GET_UPVALUE:
			offset := printGetVariable(chunk, i, code)
			i += offset
		case OP_JUMP_IF_FALSE, OP_JUMP:
			offset := printJump(chunk, i, OpcodeNames[code])
			i += offset
		case OP_GET_LOCAL_OBJECT_MEMBER, OP_GET_GLOBAL_OBJECT_MEMBER, OP_SET_GLOBAL_OBJECT_MEMBER, OP_SET_LOCAL_OBJECT_MEMBER:
			offset := printGet(chunk, i, OpcodeNames[code])
			i += offset
		case OP_SET_LOCAL, OP_SET_GLOBAL:
			offset := printSet(chunk, i, OpcodeNames[code])
			i += offset
		case OP_CLOSURE:
			{
				offset := printClosure(chunk, i, OpcodeNames[code])
				i += offset
			}
		}
		i++
	}
}

func printFrame(cf *CallFrame) {
	println("<fn " + cf.fn.name + ">")
	printChunk(cf.fn.chunk)
	println()
}
