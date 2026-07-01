package stringer

import (
	"fmt"
	"go_js/chunk"
	"go_js/compiler"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/value"
	"strings"
)

func PrintFunction(c value.ValueChunk, sb *strings.Builder) {
	ip := 0
	opCode := c.Code

	for {
		if ip >= int(len(opCode)) {
			sb.WriteString("\n")
			break
		}
		code := opCode[ip]
		switch code {
		case chunk.OP_EXPORT, chunk.OP_YIELD, chunk.OP_THIS, chunk.OP_PUSH_PROPERTY,
			chunk.OP_PUSH_METHOD, chunk.OP_CREATE_CLASS_START, chunk.OP_CREATE_CLASS_END,
			chunk.OP_NEGATE, chunk.OP_DEFINE_HEAP_VAR, chunk.OP_POP_LOCAL,
			chunk.OP_EQUALS, chunk.OP_AWAIT, chunk.OP_STRICT_EQUALS,
			chunk.OP_STRICT_NOT_EQUALS, chunk.OP_THROW, chunk.OP_EXPONENTIATION,
			chunk.OP_LOGICAL_OR, chunk.OP_MODULO, chunk.OP_PUSH_CURRENT,
			chunk.OP_CREATE_OBJECT, chunk.OP_SET_OBJECT_MEMBER, chunk.OP_CREATE_HEAP_SCOPE,
			chunk.OP_ITERATOR_NEXT, chunk.OP_ITERATOR_CURRENT, chunk.OP_PUSH_ELEMENT,
			chunk.OP_POP, chunk.OP_LESS_THAN_EQUAL, chunk.OP_LESS_THAN,
			chunk.OP_GREATER_THAN_EQUAL, chunk.OP_GREATER_THAN, chunk.OP_SUBTRACT,
			chunk.OP_ADD, chunk.OP_DEFINE_GLOBAL, chunk.OP_GET_OBJECT_MEMBER,
			chunk.OP_TRY_BLOCK_END, chunk.OP_PUSH_UNDEFINED,
			chunk.OP_RETURN, chunk.OP_IN,
			chunk.OP_NULL_COALESHING, chunk.OP_SPREAD, chunk.OP_SET_FROM_SPREAD:
			fmt.Fprintf(sb, "%04d | %s \n", ip*4, chunk.OpNames[code])

		case chunk.OP_IMPORT, chunk.OP_CONSTANT, chunk.OP_GET_HEAP_VAR,
			chunk.OP_GET_GLOBAL, chunk.OP_SET_GLOBAL, chunk.OP_GET_LOCAL,
			chunk.OP_SET_LOCAL, chunk.OP_NEW:
			fmt.Fprintf(sb, "%04d | %s \n", ip*4, chunk.OpNames[code])
			ip++
			fmt.Fprintf(sb, "%04d | %d \n", ip*4, opCode[ip])

		case chunk.OP_CALL:
			fmt.Fprintf(sb, "%04d | %s \n", ip*4, chunk.OpNames[code])
			ip++
			fmt.Fprintf(sb, "%04d | %d \n", ip*4, opCode[ip])
			ip++
			if opCode[ip] == 0 {
				fmt.Fprintf(sb, "%04d | spread: false \n", ip*4)
			} else {
				fmt.Fprintf(sb, "%04d | spread: true \n", ip*4)
			}
		case chunk.OP_JUMP_IF_FALSE, chunk.OP_TRY_BLOCK_START,
			chunk.OP_JUMP_IF_TRUE, chunk.OP_JUMP:
			ip++
			fmt.Fprintf(sb, "%04d | %s \n", ip*4, chunk.OpNames[code])
			operand := int(opCode[ip+3]) | int(opCode[ip+2])<<8 | int(opCode[ip+1])<<16 | int(opCode[ip])<<24
			ip += 3
			fmt.Fprintf(sb, "%04d | %d\n", ip*4, operand*4)

		case chunk.OP_CREATE_ARRAY:
			fmt.Fprintf(sb, "%04d | %s \n", ip*4, chunk.OpNames[code])
			ip++
			length := int(opCode[ip+3]) | int(opCode[ip+2])<<8 | int(opCode[ip+1])<<16 | int(opCode[ip])<<24
			ip += 3
			fmt.Fprintf(sb, "%04d | %d\n", ip*4, length)

		case chunk.OP_GET_ITERATOR:
			fmt.Fprintf(sb, "%04d | %s \n", ip*4, chunk.OpNames[code])
			ip++
			t := opCode[ip]
			str := "ITERATOR_FOR_OF"
			if t == compiler.ITERATOR_FOR_IN {
				str = "ITERATOR_FOR_IN"
			}
			fmt.Fprintf(sb, "%04d | %d -> %s \n", ip*4, t, str)

		case chunk.OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS:
			fmt.Fprintf(sb, "%04d | %s\n", ip*4, chunk.OpNames[code])
			ip++
			amount := opCode[ip]
			fmt.Fprintf(sb, "%04d | amount: %d \n", ip*4, amount)
			for range amount {
				ip++
				fmt.Fprintf(sb, "%04d | var index: %d \n", ip*4, opCode[ip])
			}
		case chunk.OP_CREATE_REST_OBJECT:
			fmt.Fprintf(sb, "%04d | %s\n", ip*4, chunk.OpNames[code])
			ip++
			amount := opCode[ip]
			fmt.Fprintf(sb, "%04d | %d \n", ip*4, amount)
			for range amount {
				ip++
				fmt.Fprintf(sb, "%04d | %s \n", ip*4, native.String(c.Constants[opCode[ip]]))
			}
		}
		ip++
	}

	for _, value := range c.Constants {
		if value.IsObject() {
			obj, _ := heap.GetObject(value.GetHandle())
			switch f := obj.(type) {
			case object.Callable:
				fmt.Fprintf(sb, "%s\n", f.String())
				PrintFunction(*f.ValueChunk(), sb)
			}
		}
	}
}
