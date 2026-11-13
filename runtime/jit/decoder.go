package jit

import (
	"fmt"
	"go_js/chunk"
)

type Instruction struct {
	IP       int
	Op       uint8
	Operands []int
	NextIP   int
}

func readUint32BE(code []byte, ip int) (int, bool) {
	if ip+4 > len(code) {
		return 0, false
	}
	v := int(code[ip])<<24 | int(code[ip+1])<<16 | int(code[ip+2])<<8 | int(code[ip+3])
	return v, true
}

func decodeChunk(code []byte) ([]Instruction, error) {
	instructions := make([]Instruction, 0, len(code)/2)
	ip := 0
	for ip < len(code) {
		op := code[ip]
		instr := Instruction{IP: ip, Op: op}
		next := ip + 1
		switch op {
		case chunk.OP_JUMP_IF_FALSE, chunk.OP_JUMP_IF_TRUE, chunk.OP_JUMP,
			chunk.OP_JUMP_IF_NULLISH, chunk.OP_JUMP_IF_NOT_NULLISH,
			chunk.OP_JUMP_IF_NOT_UNDEFINED, chunk.OP_TRY_BLOCK_START,
			chunk.OP_TRY_FINALLY_START:
			target, ok := readUint32BE(code, next)
			if !ok {
				return nil, fmt.Errorf("truncated jump at %d", ip)
			}
			instr.Operands = []int{target}
			next += 4
		case chunk.OP_CREATE_ARRAY:
			length, ok := readUint32BE(code, next)
			if !ok {
				return nil, fmt.Errorf("truncated array length at %d", ip)
			}
			instr.Operands = []int{length}
			next += 4
		case chunk.OP_CALL, chunk.OP_CALL_OPTIONAL, chunk.OP_CALL_CALLEE_FIRST:
			if next+2 > len(code) {
				return nil, fmt.Errorf("truncated call at %d", ip)
			}
			instr.Operands = []int{int(code[next]), int(code[next+1])}
			next += 2
		case chunk.OP_GET_HEAP_VAR, chunk.OP_SET_HEAP_VAR, chunk.OP_GET_HEAP_VAR_RAW:
			if next+2 > len(code) {
				return nil, fmt.Errorf("truncated heap variable at %d", ip)
			}
			instr.Operands = []int{int(code[next]), int(code[next+1])}
			next += 2
		case chunk.OP_SET_GLOBAL_NAMED:
			if next+2 > len(code) {
				return nil, fmt.Errorf("truncated global at %d", ip)
			}
			instr.Operands = []int{int(code[next]), int(code[next+1])}
			next += 2
		case chunk.OP_MEMBER_UPDATE:
			if next+2 > len(code) {
				return nil, fmt.Errorf("truncated member update at %d", ip)
			}
			instr.Operands = []int{int(code[next]), int(code[next+1])}
			next += 2
		case chunk.OP_NEW_REGEXP:
			if next+2 > len(code) {
				return nil, fmt.Errorf("truncated regexp at %d", ip)
			}
			instr.Operands = []int{int(code[next]), int(code[next+1])}
			next += 2
		case chunk.OP_PUSH_METHOD:
			if next+2 > len(code) {
				return nil, fmt.Errorf("truncated method at %d", ip)
			}
			instr.Operands = []int{int(code[next]), int(code[next+1])}
			next += 2
		case chunk.OP_SUPER_CALL:
			if next+2 > len(code) {
				return nil, fmt.Errorf("truncated super call at %d", ip)
			}
			instr.Operands = []int{int(code[next]), int(code[next+1])}
			next += 2
		case chunk.OP_WITH_GET, chunk.OP_WITH_SET:
			if next+2 > len(code) {
				return nil, fmt.Errorf("truncated with at %d", ip)
			}
			instr.Operands = []int{int(code[next]), int(code[next+1])}
			next += 2
		case chunk.OP_GET_ITERATOR:
			if next >= len(code) {
				return nil, fmt.Errorf("truncated iterator at %d", ip)
			}
			instr.Operands = []int{int(code[next])}
			next++
		case chunk.OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS:
			if next >= len(code) {
				return nil, fmt.Errorf("truncated heap vars at %d", ip)
			}
			amount := int(code[next])
			next++
			if next+amount*2 > len(code) {
				return nil, fmt.Errorf("truncated heap vars operands at %d", ip)
			}
			operands := []int{amount}
			for range amount {
				operands = append(operands, int(code[next]), int(code[next+1]))
				next += 2
			}
			instr.Operands = operands
		case chunk.OP_CREATE_REST_OBJECT:
			if next >= len(code) {
				return nil, fmt.Errorf("truncated rest object at %d", ip)
			}
			amount := int(code[next])
			next++
			if next+amount > len(code) {
				return nil, fmt.Errorf("truncated rest object operands at %d", ip)
			}
			operands := []int{amount}
			for range amount {
				operands = append(operands, int(code[next]))
				next++
			}
			instr.Operands = operands
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
			if next >= len(code) {
				return nil, fmt.Errorf("truncated operand at %d", ip)
			}
			instr.Operands = []int{int(code[next])}
			next++
		}
		if next > len(code) {
			return nil, fmt.Errorf("instruction at %d overruns chunk", ip)
		}
		instr.NextIP = next
		instructions = append(instructions, instr)
		ip = next
	}
	return instructions, nil
}

func ReadUint32BE(code []byte, ip int) (int, bool) {
	return readUint32BE(code, ip)
}

func InstructionEnd(code []byte, ip int) (int, bool) {
	if ip < 0 || ip >= len(code) {
		return 0, false
	}
	instrs, err := decodeChunk(code[ip:])
	if err != nil || len(instrs) == 0 {
		return 0, false
	}
	return ip + instrs[0].NextIP, true
}
