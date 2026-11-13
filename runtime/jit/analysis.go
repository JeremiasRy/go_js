package jit

import (
	"go_js/chunk"
	"go_js/value"
)

type typeKind int

const (
	tUnknown typeKind = iota
	tNumber
	tBoolean
	tNull
	tUndefined
	tSelf
)

type absState struct {
	top   int
	types []typeKind
}

func (s absState) clone() absState {
	types := make([]typeKind, len(s.types))
	copy(types, s.types)
	return absState{top: s.top, types: types}
}

func (s *absState) ensure(slot int) {
	for len(s.types) <= slot {
		s.types = append(s.types, tUndefined)
	}
}

func (s *absState) push(t typeKind) {
	s.ensure(s.top)
	s.types[s.top] = t
	s.top++
}

func (s *absState) pop() (typeKind, bool) {
	if s.top <= 0 {
		return tUnknown, false
	}
	s.top--
	return s.types[s.top], true
}

func constantType(v value.Value) typeKind {
	switch {
	case v.IsNumber():
		return tNumber
	case v.IsBoolean():
		return tBoolean
	case v == value.NULL:
		return tNull
	case v == value.UNDEFINED:
		return tUndefined
	default:
		return tUnknown
	}
}

type directInfo struct {
	ok     bool
	maxTop int
	failIP int
	failOp uint8
}

// analyzeDirect decides whether a chunk can use guard-free direct self
// recursion. It requires that every reachable operation is a numeric or
// boolean operation, that every call is a self call with exactly arity
// numeric arguments, and that every return produces a number.
func analyzeDirect(ch *value.ValueChunk, arity int) directInfo {
	instructions, err := decodeChunk(ch.Code)
	if err != nil || len(instructions) == 0 {
		return directInfo{}
	}
	byIP := map[int]int{}
	for i, instr := range instructions {
		byIP[instr.IP] = i
	}
	initial := absState{top: arity, types: make([]typeKind, max(arity, 1))}
	for i := range initial.types {
		initial.types[i] = tUndefined
	}
	for i := 0; i < arity; i++ {
		initial.types[i] = tNumber
	}

	states := map[int]absState{0: initial}
	queue := []int{0}
	maxTop := arity
	ok := true
	returnsNumber := true

	merge := func(ip int, st absState) {
		existing, found := states[ip]
		if !found {
			states[ip] = st
			queue = append(queue, ip)
			return
		}
		if existing.top != st.top {
			ok = false
			return
		}
		changed := false
		for i := 0; i < existing.top; i++ {
			if existing.types[i] != st.types[i] && existing.types[i] != tUnknown {
				existing.types[i] = tUnknown
				changed = true
			}
		}
		if changed {
			states[ip] = existing
			queue = append(queue, ip)
		}
	}

	succeed := func(ip int, st absState) {
		if ip >= 0 && ip < len(ch.Code) {
			merge(ip, st)
		}
	}

	for len(queue) > 0 && ok {
		ip := queue[0]
		queue = queue[1:]
		idx, found := byIP[ip]
		if !found {
			ok = false
			break
		}
		instr := instructions[idx]
		st := states[ip].clone()
		if st.top > maxTop {
			maxTop = st.top
		}
		switch instr.Op {
		case chunk.OP_CONSTANT:
			idx := instr.Operands[0]
			if idx >= len(ch.Constants) {
				ok = false
				break
			}
			st.push(constantType(ch.Constants[idx]))
		case chunk.OP_NILL:
			st.push(tNull)
		case chunk.OP_UNDEFINED, chunk.OP_PUSH_UNDEFINED:
			st.push(tUndefined)
		case chunk.OP_GET_LOCAL, chunk.OP_GET_LOCAL_RAW:
			slot := instr.Operands[0]
			if slot < len(st.types) {
				st.push(st.types[slot])
			} else {
				st.push(tUndefined)
			}
		case chunk.OP_SET_LOCAL:
			slot := instr.Operands[0]
			t, popped := st.pop()
			if !popped {
				ok = false
				break
			}
			st.ensure(slot)
			st.types[slot] = t
		case chunk.OP_DEFINE_LOCAL_SLOT:
			slot := instr.Operands[0]
			t, popped := st.pop()
			if !popped {
				ok = false
				break
			}
			st.ensure(slot)
			st.types[slot] = t
			if st.top <= slot {
				st.ensure(slot)
				for i := st.top; i < slot; i++ {
					st.types[i] = tUndefined
				}
				st.top = slot + 1
			}
		case chunk.OP_DEFINE_LOCAL:
			if st.top <= 0 {
				ok = false
				break
			}
			if st.types[st.top-1] == tUnknown {
				ok = false
			}
		case chunk.OP_POP_LOCAL, chunk.OP_POP:
			if st.top > 0 {
				st.top--
			}
		case chunk.OP_DUP:
			if st.top <= 0 {
				ok = false
				break
			}
			st.push(st.types[st.top-1])
		case chunk.OP_DUP2:
			if st.top < 2 {
				ok = false
				break
			}
			a := st.types[st.top-2]
			b := st.types[st.top-1]
			st.push(a)
			st.push(b)
		case chunk.OP_PUSH_CURRENT:
			if st.top <= 0 {
				ok = false
				break
			}
			st.push(st.types[st.top-1])
		case chunk.OP_POP_BELOW2:
			if st.top < 3 {
				ok = false
				break
			}
			v := st.types[st.top-1]
			st.top -= 2
			st.types[st.top-1] = v
		case chunk.OP_ADD, chunk.OP_SUBTRACT, chunk.OP_MULTIPLY, chunk.OP_DIVIDE:
			b, popped := st.pop()
			if !popped {
				ok = false
				break
			}
			a, popped := st.pop()
			if !popped || a != tNumber || b != tNumber {
				ok = false
				break
			}
			st.push(tNumber)
		case chunk.OP_NEGATE, chunk.OP_TO_NUMBER, chunk.OP_UPDATE:
			t, popped := st.pop()
			if !popped || t != tNumber {
				ok = false
				break
			}
			st.push(tNumber)
		case chunk.OP_LESS_THAN, chunk.OP_LESS_THAN_EQUAL, chunk.OP_GREATER_THAN,
			chunk.OP_GREATER_THAN_EQUAL, chunk.OP_STRICT_EQUALS, chunk.OP_STRICT_NOT_EQUALS:
			b, popped := st.pop()
			if !popped {
				ok = false
				break
			}
			a, popped := st.pop()
			if !popped || a != tNumber || b != tNumber {
				ok = false
				break
			}
			st.push(tBoolean)
		case chunk.OP_NOT:
			t, popped := st.pop()
			if !popped || (t != tBoolean && t != tNumber) {
				ok = false
				break
			}
			st.push(tBoolean)
		case chunk.OP_VOID:
			if _, popped := st.pop(); !popped {
				ok = false
				break
			}
			st.push(tUndefined)
		case chunk.OP_JUMP:
			succeed(instr.Operands[0], st)
			continue
		case chunk.OP_JUMP_IF_FALSE, chunk.OP_JUMP_IF_TRUE:
			t, popped := st.pop()
			if !popped || (t != tBoolean && t != tNumber) {
				ok = false
				break
			}
			succeed(instr.Operands[0], st.clone())
			succeed(instr.NextIP, st.clone())
			continue
		case chunk.OP_GET_GLOBAL:
			st.push(tSelf)
		case chunk.OP_CALL:
			argc := instr.Operands[0]
			if instr.Operands[1] != 0 || argc != arity {
				ok = false
				break
			}
			callee, popped := st.pop()
			if !popped || callee != tSelf {
				ok = false
				break
			}
			for range argc {
				t, popped := st.pop()
				if !popped || t != tNumber {
					ok = false
					break
				}
			}
			if !ok {
				break
			}
			st.push(tNumber)
		case chunk.OP_RETURN:
			t, popped := st.pop()
			if !popped || t != tNumber {
				returnsNumber = false
			}
			continue
		default:
			ok = false
		}
		if !ok {
			return directInfo{failIP: instr.IP, failOp: instr.Op}
		}
		if st.top > maxTop {
			maxTop = st.top
		}
		if instr.NextIP < len(ch.Code) {
			succeed(instr.NextIP, st)
		}
	}
	if !ok || !returnsNumber || len(states) == 0 {
		return directInfo{}
	}
	return directInfo{ok: true, maxTop: maxTop}
}
