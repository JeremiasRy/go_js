package vm

import (
	"fmt"
	"strings"

	"go_js/chunk"
	"go_js/compiler"
	"go_js/heap"
	"go_js/jit"
	"go_js/native"
	"go_js/object"
	"go_js/parser"
	"go_js/value"
)

type StructuredOut struct {
	Output string                `json:"output"`
	Code   map[string]*DebugInfo `json:"debug_info"`
	Ast    *parser.Node          `json:"ast"`
}

const NO_MORE_OPS = -1

func opName(code uint8) string {
	if name, found := chunk.OpNames[code]; found {
		return name
	}
	return fmt.Sprintf("OP_UNKNOWN_%d", code)
}

func readUint32At(code []uint8, ip int) (int, bool) {
	if ip+4 > len(code) {
		return 0, false
	}
	return int(code[ip])<<24 | int(code[ip+1])<<16 | int(code[ip+2])<<8 | int(code[ip+3]), true
}

func ReadOp(ip int, c value.ValueChunk, sb *strings.Builder) (int, bool) {
	opCode := c.Code
	if ip >= len(opCode) {
		return ip, false
	}
	code := opCode[ip]
	next := ip + 1

	fmt.Fprintf(sb, "%04d | %s \n", ip*4, opName(code))

	switch code {
	case chunk.OP_JUMP_IF_FALSE, chunk.OP_JUMP_IF_TRUE, chunk.OP_JUMP,
		chunk.OP_JUMP_IF_NULLISH, chunk.OP_JUMP_IF_NOT_NULLISH,
		chunk.OP_JUMP_IF_NOT_UNDEFINED, chunk.OP_TRY_BLOCK_START,
		chunk.OP_TRY_FINALLY_START:
		operand, ok := readUint32At(opCode, next)
		if !ok {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | %d\n", next*4, operand*4)
		next += 4

	case chunk.OP_CREATE_ARRAY:
		length, ok := readUint32At(opCode, next)
		if !ok {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | %d\n", next*4, length)
		next += 4

	case chunk.OP_CALL, chunk.OP_CALL_OPTIONAL, chunk.OP_CALL_CALLEE_FIRST:
		if next+2 > len(opCode) {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | %d \n", next*4, opCode[next])
		next++
		spread := "false"
		if opCode[next] != 0 {
			spread = "true"
		}
		fmt.Fprintf(sb, "%04d | spread: %s \n", next*4, spread)
		next++

	case chunk.OP_GET_HEAP_VAR, chunk.OP_SET_HEAP_VAR, chunk.OP_GET_HEAP_VAR_RAW:
		if next+2 > len(opCode) {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | depth: %d \n", next*4, opCode[next])
		next++
		fmt.Fprintf(sb, "%04d | slot: %d \n", next*4, opCode[next])
		next++

	case chunk.OP_SET_GLOBAL_NAMED:
		if next+2 > len(opCode) {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | slot: %d \n", next*4, opCode[next])
		next++
		fmt.Fprintf(sb, "%04d | name: %d \n", next*4, opCode[next])
		next++

	case chunk.OP_MEMBER_UPDATE:
		if next+2 > len(opCode) {
			return len(opCode), true
		}
		prefix := "false"
		if opCode[next] == 1 {
			prefix = "true"
		}
		fmt.Fprintf(sb, "%04d | prefix: %s \n", next*4, prefix)
		next++
		delta := "+1"
		if opCode[next] == 1 {
			delta = "-1"
		}
		fmt.Fprintf(sb, "%04d | delta: %s \n", next*4, delta)
		next++

	case chunk.OP_NEW_REGEXP:
		if next+2 > len(opCode) {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | pattern: %d \n", next*4, opCode[next])
		next++
		fmt.Fprintf(sb, "%04d | flags: %d \n", next*4, opCode[next])
		next++

	case chunk.OP_PUSH_METHOD:
		if next+2 > len(opCode) {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | flags: %d \n", next*4, opCode[next])
		next++
		fmt.Fprintf(sb, "%04d | kind: %d \n", next*4, opCode[next])
		next++

	case chunk.OP_SUPER_CALL:
		if next+2 > len(opCode) {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | arguments: %d \n", next*4, opCode[next])
		next++
		fmt.Fprintf(sb, "%04d | %d \n", next*4, opCode[next])
		next++

	case chunk.OP_GET_ITERATOR:
		if next >= len(opCode) {
			return len(opCode), true
		}
		t := opCode[next]
		str := "ITERATOR_FOR_OF"
		if t == compiler.ITERATOR_FOR_IN {
			str = "ITERATOR_FOR_IN"
		}
		fmt.Fprintf(sb, "%04d | %d -> %s \n", next*4, t, str)
		next++

	case chunk.OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS:
		if next >= len(opCode) {
			return len(opCode), true
		}
		amount := opCode[next]
		fmt.Fprintf(sb, "%04d | amount: %d \n", next*4, amount)
		next++
		for range amount {
			if next+2 > len(opCode) {
				return len(opCode), true
			}
			fmt.Fprintf(sb, "%04d | var index: %d \n", next*4, opCode[next])
			next++
			fmt.Fprintf(sb, "%04d | var slot: %d \n", next*4, opCode[next])
			next++
		}

	case chunk.OP_CREATE_REST_OBJECT:
		if next >= len(opCode) {
			return len(opCode), true
		}
		amount := opCode[next]
		fmt.Fprintf(sb, "%04d | %d \n", next*4, amount)
		next++
		for range amount {
			if next >= len(opCode) {
				return len(opCode), true
			}
			idx := opCode[next]
			label := fmt.Sprintf("%d", idx)
			if int(idx) < len(c.Constants) {
				label = native.String(c.Constants[idx])
			}
			fmt.Fprintf(sb, "%04d | %s \n", next*4, label)
			next++
		}

	case chunk.OP_CONSTANT, chunk.OP_GET_GLOBAL, chunk.OP_SET_GLOBAL,
		chunk.OP_GET_LOCAL, chunk.OP_SET_LOCAL, chunk.OP_DEFINE_LOCAL_SLOT,
		chunk.OP_GET_LOCAL_RAW, chunk.OP_GET_GLOBAL_BY_NAME,
		chunk.OP_DEFINE_GLOBAL_NAMED, chunk.OP_NEW,
		chunk.OP_DEFINE_HEAP_VAR_AT, chunk.OP_DEFINE_ARGUMENTS_HEAP,
		chunk.OP_CATCH_BIND, chunk.OP_GET_CATCH, chunk.OP_SET_CATCH,
		chunk.OP_ITERATOR_NEXT_SLOT, chunk.OP_ITERATOR_CURRENT_SLOT,
		chunk.OP_ITERATOR_CLOSE_SLOT, chunk.OP_ITERATOR_CLOSE_MARK,
		chunk.OP_ASYNC_ITERATOR_NEXT_SLOT, chunk.OP_ASYNC_ITERATOR_RESULT_SLOT,
		chunk.OP_ASYNC_ITERATOR_VALUE_SLOT, chunk.OP_ASYNC_ITERATOR_CLOSE_SLOT,
		chunk.OP_TRIM_STACK_SLOT, chunk.OP_STACK_MARK, chunk.OP_STACK_TRIM,
		chunk.OP_STACK_UNMARK, chunk.OP_CREATE_REST_OBJECT_DYNAMIC,
		chunk.OP_PUSH_CLASS_FIELD, chunk.OP_UPDATE:
		if next >= len(opCode) {
			return len(opCode), true
		}
		fmt.Fprintf(sb, "%04d | %d \n", next*4, opCode[next])
		next++
	}

	return next, true
}

func PrintFunction(c value.ValueChunk, sb *strings.Builder) {
	ip, more := ReadOp(0, c, sb)
	for more {
		ip, more = ReadOp(ip, c, sb)
	}

	fmt.Fprintln(sb)

	for _, value := range c.Constants {
		if value.IsObject() {
			obj, _ := heap.GetObject(value.GetHandle())
			switch f := obj.(type) {
			case object.Callable:
				PrintFunction(*f.ValueChunk(), sb)
			}
		}
	}
}

func PrintChunk(c value.ValueChunk) {
	sb := &strings.Builder{}
	PrintFunction(c, sb)
	println(sb.String())
}

type OpDetail struct {
	Op    string `json:"op"`
	AstId int    `json:"ast_id"`
	OpPtr int    `json:"op_ptr"`
}

type DebugInfo struct {
	Jit      []jit.JitDetail `json:"jit"`
	ByteCode []OpDetail      `json:"byte_code"`
}

func StructureOutput(c value.ValueChunk, r map[string]*DebugInfo) {
	structureOutput(&c, r, map[*value.ValueChunk]bool{})
}

func structureOutput(c *value.ValueChunk, r map[string]*DebugInfo, seen map[*value.ValueChunk]bool) {
	if c == nil || seen[c] {
		return
	}
	seen[c] = true

	info := &DebugInfo{
		Jit:      []jit.JitDetail{},
		ByteCode: []OpDetail{},
	}
	r[c.FnName] = info

	ip := 0
	sb := &strings.Builder{}

	for ip < len(c.Code) {
		opcodeIp := ip
		astId := c.AstId[ip]
		ip, _ = ReadOp(ip, *c, sb)
		info.ByteCode = append(info.ByteCode, OpDetail{Op: sb.String(), AstId: astId, OpPtr: opcodeIp})
		sb.Reset()
	}

	if found, disassembly := jit.GetJittedDisassemblyForChunk(c); found {
		info.Jit = disassembly
	}

	for _, constant := range c.Constants {
		if !constant.IsObject() {
			continue
		}
		obj, err := heap.GetObject(constant.GetHandle())
		if err != nil {
			continue
		}
		fn, ok := obj.(object.Callable)
		if !ok {
			continue
		}
		structureOutput(fn.ValueChunk(), r, seen)
	}
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
