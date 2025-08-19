package vm

import (
	"fmt"
	"go_js/chunk"
	"go_js/value"
)

var opNames = map[uint8]string{
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
	chunk.OP_LESS_THAN:                "OP_LESS_THAN",
	chunk.OP_GREATER_THAN_EQUAL:       "OP_GREATER_THAN_EQUAL",
	chunk.OP_GREATER_THAN:             "OP_GREATER_THAN",
	chunk.OP_DEFINE_LOCAL:             "OP_DEFINE_LOCAL",
	chunk.OP_GET_LOCAL:                "OP_GET_LOCAL",
	chunk.OP_SET_LOCAL:                "OP_SET_LOCAL",
	chunk.OP_DEFINE_GLOBAL:            "OP_DEFINE_GLOBAL",
	chunk.OP_GET_GLOBAL:               "OP_GET_GLOBAL",
	chunk.OP_SET_GLOBAL:               "OP_SET_GLOBAL",
	chunk.OP_CLOSE_UPVALUES:           "OP_CLOSE_UPVALUES",
	chunk.OP_SET_UPVALUE:              "OP_SET_UPVALUE",
	chunk.OP_GET_UPVALUE:              "OP_GET_UPVALUE",
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

func PrintChunk(c value.ValueChunk) {
	ip := 0
	println("--DEBUG BYTECODE--")
	for {
		code := c.Code[ip]
		switch code {
		case chunk.OP_CONSTANT:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
				ip++
				fmt.Printf("%04d | %d \n", ip*4, c.Code[ip])

			}
		case chunk.OP_DEFINE_GLOBAL:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
			}
		case chunk.OP_EOF:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
				println("--DEBUG BYTECODE--")
				return
			}
		}
		ip++
	}
}
