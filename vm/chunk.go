package vm

const (
	OP_CONSTANT uint8 = iota
	OP_ADD
	OP_SUBTRACT
	OP_MULTIPLY
	OP_DIVIDE
	OP_NILL
	OP_UNDEFINED
	OP_TRUE
	OP_FALSE
	OP_EQUALS
	OP_EQUALS_STRICT
	OP_LESS_THAN_EQUAL
	OP_DEFINE_LOCAL
	OP_GET_LOCAL
	OP_DEFINE_GLOBAL
	OP_GET_GLOBAL
	OP_CALL
	OP_RETURN
	OP_TEMPLATE_LITERAL
	OP_JUMP_IF_FALSE
	OP_EOF
)

var OpcodeNames = map[uint8]string{
	OP_CONSTANT:         "OP_CONSTANT",
	OP_ADD:              "OP_ADD",
	OP_SUBTRACT:         "OP_SUBTRACT",
	OP_MULTIPLY:         "OP_MULTIPLY",
	OP_DIVIDE:           "OP_DIVIDE",
	OP_NILL:             "OP_NILL",
	OP_UNDEFINED:        "OP_UNDEFINED",
	OP_TRUE:             "OP_TRUE",
	OP_FALSE:            "OP_FALSE",
	OP_EQUALS:           "OP_EQUALS",
	OP_EQUALS_STRICT:    "OP_EQUALS_STRICT",
	OP_LESS_THAN_EQUAL:  "OP_LESS_THAN_EQUAL",
	OP_DEFINE_LOCAL:     "OP_DEFINE_LOCAL",
	OP_GET_LOCAL:        "OP_GET_LOCAL",
	OP_GET_GLOBAL:       "OP_GET_GLOBAL",
	OP_DEFINE_GLOBAL:    "OP_DEFINE_GLOBAL",
	OP_CALL:             "OP_CALL",
	OP_RETURN:           "OP_RETURN",
	OP_TEMPLATE_LITERAL: "OP_TEMPLATE_LITERAL",
	OP_JUMP_IF_FALSE:    "OP_JUMP_IF_FALSE",
	OP_EOF:              "OP_EOF",
}

type Chunk struct {
	code      []uint8
	constants []Value
}

func NewChunk() *Chunk {
	return &Chunk{
		code:      []uint8{},
		constants: []Value{},
	}
}

func (c *Chunk) WriteConstant(v Value) {
	c.EmitByte(OP_CONSTANT)
	c.EmitByte(c.addConstant(v))
}

func (c *Chunk) EmitByte(b uint8) {
	c.code = append(c.code, b)
}

func (c *Chunk) addConstant(v Value) uint8 {
	c.constants = append(c.constants, v)
	return uint8(len(c.constants) - 1)
}
