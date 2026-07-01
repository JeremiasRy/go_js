package vm

import (
	"fmt"
	"strings"

	"go_js/native"
	"go_js/stringer"
	"go_js/value"
)

func PrintChunk(c value.ValueChunk) {
	sb := &strings.Builder{}
	stringer.PrintFunction(c, sb)
	println(sb.String())
}

func printStack(stack []value.Value) {
	print("[")
	for i, val := range stack {
		fmt.Printf("%s", strings.ReplaceAll(native.TypeDecoratedString(val, nil), "\n", "\\n"))
		if i < len(stack)-1 {
			fmt.Print(" | ")
		}
	}
	println(" ]")
}
