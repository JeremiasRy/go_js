package value

import (
	"go_js/chunk"
	"math"
)

type Value uint64

const (
	STANDARD_NAN      Value = 0x7ff8000000000000
	QNAN              Value = 0x7ffc000000000000
	SIGN              Value = 0x8000000000000000
	ENCODE_MASK       Value = math.MaxUint64 ^ TAG_OBJ
	TAG_OBJ           Value = QNAN | SIGN
	NULL              Value = QNAN | 1
	UNDEFINED         Value = QNAN | (1 << 1)
	TRUE              Value = QNAN | (1 << 2)
	FALSE             Value = QNAN | (1 << 3)
	TAG_METHOD_HANDLE Value = QNAN | (1 << 4)
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

// read uint32 and incerement ip
func (c ValueChunk) ReadInt(ip *int) int {
	start := *ip

	fourth := int(c.Code[start]) << 24
	third := int(c.Code[start+1]) << 16
	second := int(c.Code[start+2]) << 8
	first := int(c.Code[start+3])

	result := first | second | third | fourth

	if result >= math.MaxUint32 {
		panic("we cap at uint32 with operands")
	}

	*ip = start + 4

	return result
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

func (c *ValueChunk) PatchUint32(from uint32, u32 uint32) {
	c.Code[from+3] = uint8(u32 & math.MaxUint8)
	c.Code[from+2] = uint8(u32>>8) & math.MaxUint8
	c.Code[from+1] = uint8(u32>>16) & math.MaxUint8
	c.Code[from] = uint8(u32>>24) & math.MaxUint8
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

func (v Value) AsNumber() float64 {
	return math.Float64frombits(uint64(v))
}

func (v Value) GetHandle() uint32 {
	return uint32(v & ENCODE_MASK)
}

func EncodeHandle(handle uint32) Value {
	return TAG_OBJ | Value(handle)
}

func ValueFromFloat64(number float64) Value {
	return Value(math.Float64bits(number))
}

func (v Value) IsBoolean() bool {
	return (TRUE&v == TRUE) || (FALSE&v == FALSE)
}

func (v Value) IsType(tag Value) bool {
	if tag == TAG_OBJ {
		return v&TAG_OBJ == TAG_OBJ
	}
	return v == tag
}

func (v Value) IsNumber() bool {
	return !v.IsObject() && !v.IsBoolean() && !v.IsType(NULL) && !v.IsType(UNDEFINED) && !v.IsType(TAG_METHOD_HANDLE)
}

func (v Value) IsInteger() bool {
	if v.IsNumber() {
		f := v.AsNumber()
		_, frac := math.Modf(f)

		return frac == 0.0
	}
	return false
}
