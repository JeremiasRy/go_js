package vm

import (
	"fmt"
	"go_js/chunk"
)

func PrintCode(op uint8) {
	switch op {
	case chunk.OP_CONSTANT:
		fmt.Printf("OP_CONSTANT")
	case chunk.OP_POP:
		fmt.Printf("OP_POP")
	case chunk.OP_ADD:
		fmt.Printf("OP_ADD")
	case chunk.OP_SUBTRACT:
		fmt.Printf("OP_SUBTRACT")
	case chunk.OP_MULTIPLY:
		fmt.Printf("OP_MULTIPLY")
	case chunk.OP_DIVIDE:
		fmt.Printf("OP_DIVIDE")
	case chunk.OP_NILL:
		fmt.Printf("OP_NILL")
	case chunk.OP_UNDEFINED:
		fmt.Printf("OP_UNDEFINED")
	case chunk.OP_TRUE:
		fmt.Printf("OP_TRUE")
	case chunk.OP_FALSE:
		fmt.Printf("OP_FALSE")
	case chunk.OP_EQUALS:
		fmt.Printf("OP_EQUALS")
	case chunk.OP_STRICT_EQUALS:
		fmt.Printf("OP_STRICT_EQUALS")
	case chunk.OP_LESS_THAN_EQUAL:
		fmt.Printf("OP_LESS_THAN_EQUAL")
	case chunk.OP_LESS_THAN:
		fmt.Printf("OP_LESS_THAN")
	case chunk.OP_GREATER_THAN_EQUAL:
		fmt.Printf("OP_GREATER_THAN_EQUAL")
	case chunk.OP_GREATER_THAN:
		fmt.Printf("OP_GREATER_THAN")
	case chunk.OP_DEFINE_LOCAL:
		fmt.Printf("OP_DEFINE_LOCAL")
	case chunk.OP_GET_LOCAL:
		fmt.Printf("OP_GET_LOCAL")
	case chunk.OP_SET_LOCAL:
		fmt.Printf("OP_SET_LOCAL")
	case chunk.OP_DEFINE_GLOBAL:
		fmt.Printf("OP_DEFINE_GLOBAL")
	case chunk.OP_GET_GLOBAL:
		fmt.Printf("OP_GET_GLOBAL")
	case chunk.OP_SET_GLOBAL:
		fmt.Printf("OP_SET_GLOBAL")
	case chunk.OP_CLOSE_UPVALUES:
		fmt.Printf("OP_CLOSE_UPVALUES")
	case chunk.OP_SET_UPVALUE:
		fmt.Printf("OP_SET_UPVALUE")
	case chunk.OP_GET_UPVALUE:
		fmt.Printf("OP_GET_UPVALUE")
	case chunk.OP_CLOSURE:
		fmt.Printf("OP_CLOSURE")
	case chunk.OP_CALL:
		fmt.Printf("OP_CALL")
	case chunk.OP_RETURN:
		fmt.Printf("OP_RETURN")
	case chunk.OP_END_OF_FN:
		fmt.Printf("OP_END_OF_FN")
	case chunk.OP_TEMPLATE_LITERAL:
		fmt.Printf("OP_TEMPLATE_LITERAL")
	case chunk.OP_JUMP_IF_FALSE:
		fmt.Printf("OP_JUMP_IF_FALSE")
	case chunk.OP_JUMP:
		fmt.Printf("OP_JUMP")
	case chunk.OP_DEFINE_OBJECT_MEMBER:
		fmt.Printf("OP_DEFINE_OBJECT_MEMBER")
	case chunk.OP_SET_LOCAL_OBJECT_MEMBER:
		fmt.Printf("OP_SET_LOCAL_OBJECT_MEMBER")
	case chunk.OP_GET_LOCAL_OBJECT_MEMBER:
		fmt.Printf("OP_GET_LOCAL_OBJECT_MEMBER")
	case chunk.OP_SET_GLOBAL_OBJECT_MEMBER:
		fmt.Printf("OP_SET_GLOBAL_OBJECT_MEMBER")
	case chunk.OP_GET_GLOBAL_OBJECT_MEMBER:
		fmt.Printf("OP_GET_GLOBAL_OBJECT_MEMBER")
	case chunk.OP_PUSH_UNDEFINED:
		fmt.Printf("OP_PUSH_UNDEFINED")
	case chunk.OP_EOF:
		fmt.Printf("OP_EOF")
	}
}
