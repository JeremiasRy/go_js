package jit

import (
	"testing"

	"go_js/chunk"
	"go_js/value"
)

func TestAnalyzeDirectSelfRecursion(t *testing.T) {
	ch := value.NewChunk("fib")
	ch.Constants = []value.Value{
		value.ValueFromFloat64(0),
		value.ValueFromFloat64(1),
		value.ValueFromFloat64(2),
	}
	ch.Code = []uint8{
		chunk.OP_GET_LOCAL, 0,
		chunk.OP_CONSTANT, 0,
		chunk.OP_LESS_THAN_EQUAL,
		chunk.OP_JUMP_IF_FALSE, 0, 0, 0, 13,
		chunk.OP_GET_LOCAL, 0,
		chunk.OP_RETURN,
		chunk.OP_GET_LOCAL, 0,
		chunk.OP_CONSTANT, 1,
		chunk.OP_SUBTRACT,
		chunk.OP_GET_GLOBAL, 75,
		chunk.OP_CALL, 1, 0,
		chunk.OP_RETURN,
	}
	ch.AstId = make([]int, len(ch.Code))
	info := analyzeDirect(ch, 1)
	if !info.ok {
		t.Fatalf("fib was not recognized as direct: fail at ip %d op %d", info.failIP, info.failOp)
	}
	if info.maxTop != 3 {
		t.Fatalf("maxTop = %d, want 3", info.maxTop)
	}
}

func TestAnalyzeDirectRejectsNonNumeric(t *testing.T) {
	ch := value.NewChunk("g")
	ch.Constants = []value.Value{
		value.EncodeHandle(1),
	}
	ch.Code = []uint8{
		chunk.OP_GET_LOCAL, 0,
		chunk.OP_CONSTANT, 0,
		chunk.OP_ADD,
		chunk.OP_RETURN,
	}
	ch.AstId = make([]int, len(ch.Code))
	if info := analyzeDirect(ch, 1); info.ok {
		t.Fatal("a string constant was accepted as direct numeric code")
	}
}
