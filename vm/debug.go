package vm

import (
	"fmt"
	"go_js/chunk"
	"go_js/value"
)

var OpcodeNames = map[uint8]string{
	chunk.OP_CONSTANT:                 "OP_CONSTANT",
	chunk.OP_POP:                      "OP_POP",
	chunk.OP_ADD:                      "OP_ADD",
	chunk.OP_SUBTRACT:                 "OP_SUBTRACT",
	chunk.OP_MULTIPLY:                 "OP_MULTIPLY",
	chunk.OP_DIVIDE:                   "OP_DIVIDE",
	chunk.OP_NILL:                     "OP_NILL",
	chunk.OP_UNDEFINED:                "OP_UNDEFINED",
	chunk.OP_TRUE:                     "OP_TRUE",
	chunk.OP_FALSE:                    "OP_FALSE",
	chunk.OP_EQUALS:                   "OP_EQUALS",
	chunk.OP_STRICT_EQUALS:            "OP_STRICT_EQUALS",
	chunk.OP_LESS_THAN_EQUAL:          "OP_LESS_THAN_EQUAL",
	chunk.OP_GET_LOCAL:                "OP_GET_LOCAL",
	chunk.OP_SET_LOCAL:                "OP_SET_LOCAL",
	chunk.OP_DEFINE_LOCAL:             "OP_DEFINE_LOCAL",
	chunk.OP_GET_GLOBAL:               "OP_GET_GLOBAL",
	chunk.OP_SET_GLOBAL:               "OP_SET_GLOBAL",
	chunk.OP_DEFINE_GLOBAL:            "OP_DEFINE_GLOBAL",
	chunk.OP_CLOSE_UPVALUES:           "OP_CLOSE_UPVALUES",
	chunk.OP_GET_UPVALUE:              "OP_GET_UPVALUE",
	chunk.OP_SET_UPVALUE:              "OP_SET_UPVALUE",
	chunk.OP_CLOSURE:                  "OP_CLOSURE",
	chunk.OP_CALL:                     "OP_CALL",
	chunk.OP_RETURN:                   "OP_RETURN",
	chunk.OP_END_OF_FN:                "OP_END_OF_FN",
	chunk.OP_TEMPLATE_LITERAL:         "OP_TEMPLATE_LITERAL",
	chunk.OP_JUMP_IF_FALSE:            "OP_JUMP_IF_FALSE",
	chunk.OP_JUMP:                     "OP_JUMP",
	chunk.OP_DEFINE_OBJECT_MEMBER:     "OP_DEFINE_OBJECT_MEMBER",
	chunk.OP_SET_LOCAL_OBJECT_MEMBER:  "OP_SET_LOCAL_OBJECT_MEMBER",
	chunk.OP_GET_LOCAL_OBJECT_MEMBER:  "OP_GET_LOCAL_OBJECT_MEMBER",
	chunk.OP_SET_GLOBAL_OBJECT_MEMBER: "OP_SET_GLOBAL_OBJECT_MEMBER",
	chunk.OP_GET_GLOBAL_OBJECT_MEMBER: "OP_GET_GLOBAL_OBJECT_MEMBER",
	chunk.OP_PUSH_UNDEFINED:           "OP_PUSH_UNDEFINED",
	chunk.OP_EOF:                      "OP_EOF",
}

func printCode(name string, i int) {
	fmt.Printf("%04d - %-32s |\n", i, name)
}

func printConstant(chunk *value.ValueChunk, i int) int {
	constant := chunk.Constants[chunk.Code[i+1]]

	fmt.Printf("%04d - %-32s | %s\n", i, "OP_CONSTANT", constant)
	return 1
}

func printGetVariable(chunk *value.ValueChunk, i int, code uint8) int {
	fmt.Printf("%04d - %-32s | %v\n", i, OpcodeNames[code], chunk.Code[i+1])
	return 1
}

func printJump(chunk *value.ValueChunk, i int, name string) int {
	fmt.Printf("%04d - %-32s | %d\n", i, name, uint32(chunk.Code[i+4])|(uint32(chunk.Code[i+3])<<8)|(uint32(chunk.Code[i+2])<<16)|(uint32(chunk.Code[i+1])<<24))
	return 4
}

func printGet(chunk *value.ValueChunk, i int, name string) int {
	fmt.Printf("%04d - %-32s | %d %s\n", i, name, chunk.Code[i+1], chunk.Constants[chunk.Code[i+2]])
	return 2
}

func printSet(chunk *value.ValueChunk, i int, name string) int {
	fmt.Printf("%04d - %-32s | %d\n", i, name, chunk.Code[i+1])
	return 1
}

func printClosure(chunk *value.ValueChunk, i int, name string) int {
	upvalueCount := chunk.Code[i+1]

	fmt.Printf("%04d - %-32s | upvalues %d\n", i, name, upvalueCount)

	i++

	for range upvalueCount {
		isLocal := chunk.Code[i+1]
		slot := chunk.Code[i+2]

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

func printChunk(values *value.ValueChunk) {
	i := 0
	for i < len(values.Code) {

		code := values.Code[i]
		switch code {
		case chunk.OP_ADD, chunk.OP_SUBTRACT, chunk.OP_DIVIDE, chunk.OP_MULTIPLY, chunk.OP_LESS_THAN_EQUAL, chunk.OP_CALL, chunk.OP_RETURN, chunk.OP_END_OF_FN, chunk.OP_POP, chunk.OP_DEFINE_LOCAL, chunk.OP_DEFINE_GLOBAL, chunk.OP_CLOSE_UPVALUES, chunk.OP_DEFINE_OBJECT_MEMBER, chunk.OP_EOF:
			printCode(OpcodeNames[code], i)
		case chunk.OP_CONSTANT:
			offset := printConstant(values, i)
			i += offset
		case chunk.OP_GET_LOCAL, chunk.OP_GET_GLOBAL:
			offset := printGetVariable(values, i, code)
			i += offset
		case chunk.OP_JUMP_IF_FALSE, chunk.OP_JUMP:
			offset := printJump(values, i, OpcodeNames[code])
			i += offset
		case chunk.OP_GET_LOCAL_OBJECT_MEMBER, chunk.OP_GET_GLOBAL_OBJECT_MEMBER, chunk.OP_SET_GLOBAL_OBJECT_MEMBER, chunk.OP_SET_LOCAL_OBJECT_MEMBER:
			offset := printGet(values, i, OpcodeNames[code])
			i += offset
		case chunk.OP_SET_LOCAL, chunk.OP_SET_GLOBAL:
			offset := printSet(values, i, OpcodeNames[code])
			i += offset
		}
		i++
	}
}

func printFrame(cf *CallFrame) {
	println("<fn " + cf.fn.Name() + ">")
	printChunk(cf.fn.ValueChunk())
	println()
}
