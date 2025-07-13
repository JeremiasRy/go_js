package vm

const (
	OP_CONSTANT uint8 = iota
	OP_POP
	OP_ADD
	OP_SUBTRACT
	OP_MULTIPLY
	OP_DIVIDE
	OP_NILL
	OP_UNDEFINED
	OP_TRUE
	OP_FALSE
	OP_EQUALS
	OP_STRICT_EQUALS
	OP_LESS_THAN_EQUAL
	OP_LESS_THAN
	OP_GREATER_THAN_EQUAL
	OP_GREATER_THAN
	OP_DEFINE_LOCAL
	OP_GET_LOCAL
	OP_SET_LOCAL
	OP_DEFINE_GLOBAL
	OP_GET_GLOBAL
	OP_SET_GLOBAL
	OP_CLOSE_UPVALUES
	OP_SET_UPVALUE
	OP_GET_UPVALUE
	OP_CLOSURE
	OP_CALL
	OP_RETURN
	OP_END_OF_FN
	OP_TEMPLATE_LITERAL
	OP_JUMP_IF_FALSE
	OP_JUMP
	OP_DEFINE_OBJECT_MEMBER
	OP_SET_LOCAL_OBJECT_MEMBER
	OP_GET_LOCAL_OBJECT_MEMBER
	OP_SET_GLOBAL_OBJECT_MEMBER
	OP_GET_GLOBAL_OBJECT_MEMBER
	OP_PUSH_UNDEFINED
	OP_EOF
)

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

func (c *Chunk) WriteConstant(v Value) uint8 {
	arg := c.addConstant(v)
	c.EmitBytes(OP_CONSTANT, arg)
	return arg
}

func (c *Chunk) EmitByte(b uint8) {
	c.code = append(c.code, b)
}

func (c *Chunk) EmitBytes(b ...uint8) {
	c.code = append(c.code, b...)
}

func (c *Chunk) addConstant(v Value) uint8 {
	c.constants = append(c.constants, v)
	return uint8(len(c.constants) - 1)
}
