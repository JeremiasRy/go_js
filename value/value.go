package value

import (
	"go_js/chunk"
	"math"
)

type Value uint64

const (
	STANDARD_NAN  Value = 0x7ff8000000000000
	QNAN          Value = 0x7ffc000000000000
	SIGN          Value = 0x8000000000000000
	ENCODE_MASK   Value = math.MaxUint64 ^ (QNAN | SIGN)
	TAG_OBJ       Value = QNAN | SIGN
	TAG_NIL       Value = QNAN | 0x0000000000000001
	TAG_UNDEFINED Value = QNAN | 0x0000000000000002
	TAG_TRUE      Value = QNAN | 0x2000000000000
	TAG_FALSE     Value = QNAN | 0x1000000000000
)

type ValueChunk struct {
	Code      []uint8
	Constants []Value
}

func NewChunk() *ValueChunk {
	return &ValueChunk{
		Code:      []uint8{},
		Constants: []Value{},
	}
}

func (c *ValueChunk) WriteConstant(v Value) uint8 {
	arg := c.AddConstant(v)
	c.EmitBytes(chunk.OP_CONSTANT, arg)
	return arg
}

func (c *ValueChunk) AddConstant(v Value) uint8 {
	c.Constants = append(c.Constants, v)
	return uint8(len(c.Constants) - 1)
}

func (c *ValueChunk) PatchJump(jumpStart uint32, to uint32) {
	c.Code[jumpStart+3] = uint8(to & math.MaxUint8)
	c.Code[jumpStart+2] = uint8(to>>8) & math.MaxUint8
	c.Code[jumpStart+1] = uint8(to>>16) & math.MaxUint8
	c.Code[jumpStart] = uint8(to>>24) & math.MaxUint8
}

func (c *ValueChunk) EmitByte(b uint8) {
	c.Code = append(c.Code, b)
}

func (c *ValueChunk) EmitBytes(b ...uint8) {
	c.Code = append(c.Code, b...)
}

func (c *ValueChunk) EmitUint32(u32 uint32) {
	fourth := uint8(u32 & math.MaxUint8)
	third := uint8(u32>>8) & math.MaxUint8
	second := uint8(u32>>16) & math.MaxUint8
	first := uint8(u32>>24) & math.MaxUint8
	c.Code = append(c.Code, first, second, third, fourth)
}

func (v Value) IsObject() bool {
	return v&TAG_OBJ == TAG_OBJ
}

func (v Value) IsNaN() bool {
	return (v&STANDARD_NAN == STANDARD_NAN) &&
		(v != TAG_OBJ) && (v != TAG_NIL) && (v != TAG_UNDEFINED) && (v != TAG_FALSE) && (v != TAG_TRUE)
}

func (v Value) AsNumber() float64 {
	return math.Float64frombits(uint64(v))
}

func (v Value) GetRegister() uint32 {
	return uint32(v & ENCODE_MASK)
}

func EncodeObject(handle uint32) Value {
	return TAG_OBJ | Value(handle)
}

func EncodeNil() Value {
	return TAG_NIL
}

func EncodedUndefined() Value {
	return TAG_UNDEFINED
}

func EncodeNaN() Value {
	return Value(math.Float64bits(math.NaN()))
}

func ValueFromFloat64(number float64) Value {
	return Value(math.Float64bits(number))
}

func EncodeTrue() Value {
	return TAG_TRUE
}

func EncodeFalse() Value {
	return TAG_FALSE
}

func (v Value) IsBoolean() bool {
	return (TAG_TRUE&v == TAG_TRUE) || (TAG_FALSE&v == TAG_FALSE)
}

func (v Value) IsType(tag Value) bool {
	return tag&v == tag
}

// TODO extend this to handle 'falsy' values: nill, undefined, "", 0, 1, etc...
func (v Value) AsBoolean() bool {
	return TAG_TRUE&v == TAG_TRUE
}
