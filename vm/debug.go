package vm

import (
	"fmt"
	"go_js/chunk"
	"go_js/heap"
	"go_js/object"
	"go_js/stringer"
	"go_js/value"
)

const DEBUG = true

var opNames = map[uint8]string{
	chunk.OP_CONSTANT:                "OP_CONSTANT",
	chunk.OP_POP:                     "OP_POP",
	chunk.OP_PUSH_CURRENT:            "OP_PUSH_CURRENT",
	chunk.OP_ADD:                     "OP_ADD",
	chunk.OP_SUBTRACT:                "OP_SUBTRACT",
	chunk.OP_MULTIPLY:                "OP_MULTIPLY",
	chunk.OP_DIVIDE:                  "OP_DIVIDE",
	chunk.OP_TRUE:                    "OP_TRUE",
	chunk.OP_FALSE:                   "OP_FALSE",
	chunk.OP_EQUALS:                  "OP_EQUALS",
	chunk.OP_STRICT_EQUALS:           "OP_STRICT_EQUALS",
	chunk.OP_LESS_THAN_EQUAL:         "OP_LESS_THAN_EQUAL",
	chunk.OP_LESS_THAN:               "OP_LESS_THAN",
	chunk.OP_GREATER_THAN_EQUAL:      "OP_GREATER_THAN_EQUAL",
	chunk.OP_GREATER_THAN:            "OP_GREATER_THAN",
	chunk.OP_DEFINE_LOCAL:            "OP_DEFINE_LOCAL",
	chunk.OP_GET_LOCAL:               "OP_GET_LOCAL",
	chunk.OP_SET_LOCAL:               "OP_SET_LOCAL",
	chunk.OP_DEFINE_GLOBAL:           "OP_DEFINE_GLOBAL",
	chunk.OP_GET_GLOBAL:              "OP_GET_GLOBAL",
	chunk.OP_SET_GLOBAL:              "OP_SET_GLOBAL",
	chunk.OP_CREATE_HEAP_SCOPE:       "OP_CREATE_HEAP_SCOPE",
	chunk.OP_DEFINE_HEAP_VAR:         "OP_DEFINE_HEAP_VAR",
	chunk.OP_GET_HEAP_VAR:            "OP_GET_HEAP_VAR",
	chunk.OP_SET_HEAP_VAR:            "OP_SET_HEAP_VAR",
	chunk.OP_CLOSURE:                 "OP_CLOSURE",
	chunk.OP_CALL:                    "OP_CALL",
	chunk.OP_RETURN:                  "OP_RETURN",
	chunk.OP_JUMP_IF_FALSE:           "OP_JUMP_IF_FALSE",
	chunk.OP_JUMP_IF_TRUE:            "OP_JUMP_IF_TRUE",
	chunk.OP_JUMP:                    "OP_JUMP",
	chunk.OP_CREATE_OBJECT:           "OP_CREATE_OBJECT",
	chunk.OP_SET_OBJECT_MEMBER:       "OP_SET_OBJECT_MEMBER",
	chunk.OP_GET_OBJECT_MEMBER:       "OP_GET_OBJECT_MEMBER",
	chunk.OP_SET_LOCAL_OBJECT_MEMBER: "OP_SET_LOCAL_OBJECT_MEMBER",
	chunk.OP_GET_LOCAL_OBJECT_MEMBER: "OP_GET_LOCAL_OBJECT_MEMBER",
	chunk.OP_CREATE_ARRAY:            "OP_CREATE_ARRAY",
	chunk.OP_PUSH_ELEMENT:            "OP_PUSH_ELEMENT",
	chunk.OP_PUSH_UNDEFINED:          "OP_PUSH_UNDEFINED",
	chunk.OP_GET_ITERATOR:            "OP_GET_ITERATOR",
	chunk.OP_ITERATOR_NEXT:           "OP_ITERATOR_NEXT",
	chunk.OP_ITERATOR_CURRENT:        "OP_ITERATOR_CURRENT",
	chunk.OP_EOF:                     "OP_EOF",
}

func PrintChunk(c value.ValueChunk) {
	println("--DEBUG BYTECODE--\n")
	println("<MAIN PROGRAM>")
	printFunction(c)
	println()

	println("--DEBUG BYTECODE--")
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
				ip++
				fmt.Printf("%04d | %d \n", ip*4, opCode[ip])
			}
		case chunk.OP_GET_LOCAL_OBJECT_MEMBER:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
				ip++
				fmt.Printf("%04d | %d \n", ip*4, opCode[ip])
				ip++
				fmt.Printf("%04d | %d \n", ip*4, opCode[ip])
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
		case chunk.OP_EOF:
			{
				fmt.Printf("%04d | %s\n", ip*4, opNames[code])
				break
			}
		}
		ip++
	}

	for _, value := range c.Constants {
		if value.IsObject() {
			obj := heap.GetObject(value.GetHandle())

			switch f := obj.(type) {
			case *object.ObjFunction:
				{
					fmt.Printf("<fn %s>\n", f.Name())
					printFunction(*f.ValueChunk())
				}
			}
		}
	}
}

func printStack(stack []value.Value) {
	print("[")
	for _, val := range stack {
		fmt.Printf("%s | ", stringer.DebugString(val))
	}
	println("]")
}
