package vm

import (
	"fmt"
	"go_js/allocator"
	"go_js/chunk"
	"go_js/compiler"
	"go_js/native"
	"go_js/object"
	"go_js/stringer"
	"go_js/value"
)

var opNames = map[uint8]string{
	chunk.OP_CONSTANT:                        "OP_CONSTANT",
	chunk.OP_POP:                             "OP_POP",
	chunk.OP_PUSH_CURRENT:                    "OP_PUSH_CURRENT",
	chunk.OP_ADD:                             "OP_ADD",
	chunk.OP_SUBTRACT:                        "OP_SUBTRACT",
	chunk.OP_MULTIPLY:                        "OP_MULTIPLY",
	chunk.OP_DIVIDE:                          "OP_DIVIDE",
	chunk.OP_MODULO:                          "OP_MODULO",
	chunk.OP_EXPONENTIATION:                  "OP_EXPONENTIATION",
	chunk.OP_EQUALS:                          "OP_EQUALS",
	chunk.OP_STRICT_EQUALS:                   "OP_STRICT_EQUALS",
	chunk.OP_STRICT_NOT_EQUALS:               "OP_STRICT_NOT_EQUALS",
	chunk.OP_LESS_THAN_EQUAL:                 "OP_LESS_THAN_EQUAL",
	chunk.OP_LESS_THAN:                       "OP_LESS_THAN",
	chunk.OP_GREATER_THAN_EQUAL:              "OP_GREATER_THAN_EQUAL",
	chunk.OP_GREATER_THAN:                    "OP_GREATER_THAN",
	chunk.OP_LOGICAL_AND:                     "OP_LOGICAL_AND",
	chunk.OP_LOGICAL_OR:                      "OP_LOGICAL_OR",
	chunk.OP_DEFINE_LOCAL:                    "OP_DEFINE_LOCAL",
	chunk.OP_GET_LOCAL:                       "OP_GET_LOCAL",
	chunk.OP_SET_LOCAL:                       "OP_SET_LOCAL",
	chunk.OP_POP_LOCAL:                       "OP_POP_LOCAL",
	chunk.OP_DEFINE_GLOBAL:                   "OP_DEFINE_GLOBAL",
	chunk.OP_GET_GLOBAL:                      "OP_GET_GLOBAL",
	chunk.OP_SET_GLOBAL:                      "OP_SET_GLOBAL",
	chunk.OP_CREATE_HEAP_SCOPE:               "OP_CREATE_HEAP_SCOPE",
	chunk.OP_DEFINE_HEAP_VAR:                 "OP_DEFINE_HEAP_VAR",
	chunk.OP_GET_HEAP_VAR:                    "OP_GET_HEAP_VAR",
	chunk.OP_SET_HEAP_VAR:                    "OP_SET_HEAP_VAR",
	chunk.OP_CALL:                            "OP_CALL",
	chunk.OP_RETURN:                          "OP_RETURN",
	chunk.OP_JUMP_IF_FALSE:                   "OP_JUMP_IF_FALSE",
	chunk.OP_JUMP_IF_TRUE:                    "OP_JUMP_IF_TRUE",
	chunk.OP_JUMP:                            "OP_JUMP",
	chunk.OP_CREATE_OBJECT:                   "OP_CREATE_OBJECT",
	chunk.OP_SET_OBJECT_MEMBER:               "OP_SET_OBJECT_MEMBER",
	chunk.OP_GET_OBJECT_MEMBER:               "OP_GET_OBJECT_MEMBER",
	chunk.OP_CREATE_ARRAY:                    "OP_CREATE_ARRAY",
	chunk.OP_PUSH_ELEMENT:                    "OP_PUSH_ELEMENT",
	chunk.OP_PUSH_UNDEFINED:                  "OP_PUSH_UNDEFINED",
	chunk.OP_GET_ITERATOR:                    "OP_GET_ITERATOR",
	chunk.OP_ITERATOR_NEXT:                   "OP_ITERATOR_NEXT",
	chunk.OP_ITERATOR_CURRENT:                "OP_ITERATOR_CURRENT",
	chunk.OP_TRY_BLOCK_START:                 "OP_TRY_BLOCK_START",
	chunk.OP_TRY_BLOCK_END:                   "OP_TRY_BLOCK_END",
	chunk.OP_THROW:                           "OP_THROW",
	chunk.OP_NEW:                             "OP_NEW",
	chunk.OP_ADD_ARGUMENTS_TO_LOCALS:         "OP_ADD_ARGUMENTS_TO_LOCALS",
	chunk.OP_STORE_ARG_COUNT:                 "OP_STORE_ARG_COUNT",
	chunk.OP_AWAIT:                           "OP_AWAIT",
	chunk.OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS: "OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS",
	chunk.OP_NEGATE:                          "OP_NEGATE",
	chunk.OP_CREATE_CLASS_START:              "OP_CREATE_CLASS_START",
	chunk.OP_CREATE_CLASS_END:                "OP_CREATE_CLASS_END",
	chunk.OP_PUSH_METHOD:                     "OP_PUSH_METHOD",
	chunk.OP_PUSH_PROPERTY:                   "OP_PUSH_PROPERTY",
	chunk.OP_THIS:                            "OP_THIS",
	chunk.OP_YIELD:                           "OP_YIELD",
	chunk.OP_IMPORT:                          "OP_IMPORT",
	chunk.OP_EXPORT:                          "OP_EXPORT",
	chunk.OP_IN:                              "OP_IN",
}

func PrintChunk(c value.ValueChunk) {
	printFunction(c)
	println()
}

func printFunction(c value.ValueChunk) {
	ip := 0
	opCode := c.Code

	for {
		if ip >= int(len(opCode)) {
			println()
			break
		}
		code := opCode[ip]
		switch code {
		case chunk.OP_EXPORT:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_IMPORT:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
				ip++
				fmt.Printf("%04d | %d \n", ip*4, opCode[ip])
			}
		case chunk.OP_YIELD:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_THIS:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_PUSH_PROPERTY:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_PUSH_METHOD:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_CREATE_CLASS_START:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_CREATE_CLASS_END:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_NEGATE:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_CONSTANT:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
				ip++
				fmt.Printf("%04d | %d \n", ip*4, opCode[ip])
			}
		case chunk.OP_GET_HEAP_VAR:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
				ip++
				fmt.Printf("%04d | %d \n", ip*4, opCode[ip])
			}
		case chunk.OP_DEFINE_HEAP_VAR:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_POP_LOCAL:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_EQUALS:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_AWAIT:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_STRICT_EQUALS:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_STRICT_NOT_EQUALS:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_THROW:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_EXPONENTIATION:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_LOGICAL_OR:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_MODULO:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_NEW:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
				ip++
				fmt.Printf("%04d | %d \n", ip*4, opCode[ip])
			}
		case chunk.OP_PUSH_CURRENT:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_CREATE_OBJECT:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_SET_OBJECT_MEMBER:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_CREATE_HEAP_SCOPE:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_ITERATOR_NEXT:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_ITERATOR_CURRENT:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_CREATE_ARRAY:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
				ip++
				length := int(opCode[ip+3]) | int(opCode[ip+2])<<8 | int(opCode[ip+1])<<16 | int(opCode[ip])<<24
				ip += 3
				fmt.Printf("%04d | %d\n", ip*4, length)
			}
		case chunk.OP_GET_ITERATOR:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
				ip++
				t := opCode[ip]
				str := ""

				if t == compiler.ITERATOR_FOR_IN {
					str = "ITERATOR_FOR_IN"
				} else {
					str = "ITERATOR_FOR_OF"
				}
				fmt.Printf("%04d | %d -> %s \n", ip*4, t, str)
			}
		case chunk.OP_PUSH_ELEMENT:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_POP:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_LESS_THAN_EQUAL:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_LESS_THAN:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_GREATER_THAN_EQUAL:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_GREATER_THAN:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_SUBTRACT:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_ADD:
			{
				fmt.Printf("%04d | %s \n", ip*4, opNames[code])
			}
		case chunk.OP_DEFINE_GLOBAL:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
			}
		case chunk.OP_GET_GLOBAL:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
				ip++
				fmt.Printf("%04d | %d \n", ip*4, opCode[ip])
			}
		case chunk.OP_SET_GLOBAL:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
				ip++
				fmt.Printf("%04d | %d \n", ip*4, opCode[ip])
			}
		case chunk.OP_DEFINE_LOCAL:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
			}
		case chunk.OP_GET_LOCAL:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
				ip++
				fmt.Printf("%04d | %d \n", ip*4, opCode[ip])
			}
		case chunk.OP_SET_LOCAL:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
				ip++
				fmt.Printf("%04d | %d \n", ip*4, opCode[ip])
			}
		case chunk.OP_GET_OBJECT_MEMBER:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
			}
		case chunk.OP_CALL:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
			}
		case chunk.OP_JUMP_IF_FALSE:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
				ip++
				jump := int(opCode[ip+3]) | int(opCode[ip+2])<<8 | int(opCode[ip+1])<<16 | int(opCode[ip])<<24
				ip += 3
				fmt.Printf("%04d | %d\n", ip*4, jump*4)
			}
		case chunk.OP_TRY_BLOCK_START:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
				ip++
				jump := int(opCode[ip+3]) | int(opCode[ip+2])<<8 | int(opCode[ip+1])<<16 | int(opCode[ip])<<24
				ip += 3
				fmt.Printf("%04d | %d\n", ip*4, jump*4)
			}
		case chunk.OP_TRY_BLOCK_END:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
			}
		case chunk.OP_JUMP_IF_TRUE:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
				ip++
				jump := int(opCode[ip+3]) | int(opCode[ip+2])<<8 | int(opCode[ip+1])<<16 | int(opCode[ip])<<24
				ip += 3
				fmt.Printf("%04d | %d\n", ip*4, jump*4)
			}
		case chunk.OP_JUMP:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
				ip++
				jump := int(opCode[ip+3]) | int(opCode[ip+2])<<8 | int(opCode[ip+1])<<16 | int(opCode[ip])<<24
				ip += 3
				fmt.Printf("%04d | %d\n", ip*4, jump*4)
			}
		case chunk.OP_PUSH_UNDEFINED:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
			}
		case chunk.OP_RETURN:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
			}
		case chunk.OP_ADD_ARGUMENTS_TO_LOCALS:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
			}
		case chunk.OP_STORE_ARG_COUNT:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
				ip++
				fmt.Printf("%04d | %d \n", ip*4, opCode[ip])
			}
		case chunk.OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
				ip++
				fmt.Printf("%04d | %d \n", ip*4, opCode[ip])

				amount := opCode[ip]

				for range amount {
					ip++
					fmt.Printf("%04d | %d \n", ip*4, opCode[ip])
				}
			}
		case chunk.OP_IN:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
			}
		}
		ip++
	}

	for _, value := range c.Constants {
		if value.IsObject() {
			obj, _ := allocator.GetObject(value.GetHandle())

			switch f := obj.(type) {
			case nil:
				continue
			case *native.SetTimeout:
				{

				}
			case object.Callable:
				{
					fmt.Println(f.String())
					printFunction(*f.ValueChunk())
				}
			}
		}
	}
}

func printStack(stack []value.Value) {
	print("[")
	for _, val := range stack {
		fmt.Printf("%s | ", stringer.TypeDecoratedString(val))
	}
	println("]")
}
