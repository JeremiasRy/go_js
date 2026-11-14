package value

import (
	"go_js/chunk"
	"math"
	"strconv"
	"sync/atomic"
)

const UN_INITIALIZED_COMPTIME_AST_ID = -1

var comptTimeAstId = UN_INITIALIZED_COMPTIME_AST_ID

type Value uint64

const (
	STANDARD_NAN       Value = 0x7ff8000000000000
	QNAN               Value = 0x7ffc000000000000
	SIGN               Value = 0x8000000000000000
	ENCODE_MASK        Value = math.MaxUint64 ^ TAG_OBJ
	TAG_OBJ            Value = QNAN | SIGN
	NULL               Value = QNAN | 1
	UNDEFINED          Value = QNAN | (1 << 1)
	TRUE               Value = QNAN | (1 << 2)
	FALSE              Value = QNAN | (1 << 3)
	TAG_METHOD_HANDLE  Value = QNAN | (1 << 4)
	TAG_ARGUMENT_START Value = QNAN | (1 << 5)
	EMPTY_ARRAY_ITEM   Value = QNAN | (1 << 6)
	UNINITIALIZED      Value = QNAN | (1 << 7)
)

type LocalScope struct {
	StartIP int
	Names   []string
}

type ValueChunk struct {
	CurrentAstId int
	Code         []uint8
	AstId        []int
	Constants    []Value
	FnName       string
	ChunkID      string
	TraceHidden  bool
	LocalScopes  []LocalScope
}

var nextChunkID atomic.Uint64

func NextChunkID() string {
	return "c" + strconv.FormatUint(nextChunkID.Add(1), 10)
}

func NewChunk(fnName string) *ValueChunk {
	return &ValueChunk{
		AstId:     []int{},
		Code:      []uint8{},
		Constants: []Value{},
		FnName:    fnName,
		ChunkID:   NextChunkID(),
	}
}

func (c *ValueChunk) SetCompTimeAstId(id int) {
	c.CurrentAstId = id
}

func (c *ValueChunk) RegisterLocalScope(names []string) {
	scope := LocalScope{StartIP: len(c.Code), Names: names}
	if n := len(c.LocalScopes); n > 0 && c.LocalScopes[n-1].StartIP == scope.StartIP {
		c.LocalScopes[n-1] = scope
		return
	}
	c.LocalScopes = append(c.LocalScopes, scope)
}

func (c *ValueChunk) LocalNamesAt(ip int) []string {
	low, high := 0, len(c.LocalScopes)-1
	found := -1
	for low <= high {
		mid := (low + high) / 2
		if c.LocalScopes[mid].StartIP <= ip {
			found = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	if found < 0 {
		return nil
	}
	return c.LocalScopes[found].Names
}

// read uint32 and incerement ip
func (c *ValueChunk) ReadInt(ip *int) int {
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
	// the names are wrong... I don't have the patience to fix, but I'll still spend some time to write this comment
	first := uint8(u32 & math.MaxUint8)
	second := uint8(u32>>8) & math.MaxUint8
	third := uint8(u32>>16) & math.MaxUint8
	fourth := uint8(u32>>24) & math.MaxUint8

	c.Code[from+3] = first
	c.Code[from+2] = second
	c.Code[from+1] = third
	c.Code[from] = fourth
}

func (c *ValueChunk) EmitByte(b uint8) {
	c.Code = append(c.Code, b)
	c.AstId = append(c.AstId, c.CurrentAstId)
}

func (c *ValueChunk) EmitBytes(b ...uint8) {
	for _, b := range b {
		c.Code = append(c.Code, b)
		c.AstId = append(c.AstId, c.CurrentAstId)
	}
}

func (c *ValueChunk) EmitUint32(u32 uint32) {
	fourth := uint8(u32 & math.MaxUint8)
	third := uint8(u32>>8) & math.MaxUint8
	second := uint8(u32>>16) & math.MaxUint8
	first := uint8(u32>>24) & math.MaxUint8
	c.Code = append(c.Code, first, second, third, fourth)
	c.AstId = append(c.AstId, c.CurrentAstId, c.CurrentAstId, c.CurrentAstId, c.CurrentAstId)
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
	return v == TRUE || v == FALSE
}

func (v Value) IsType(tag Value) bool {
	if tag == TAG_OBJ {
		return v&TAG_OBJ == TAG_OBJ
	}
	return v == tag
}

func (v Value) IsNumber() bool {
	return !v.IsObject() && !v.IsBoolean() && !v.IsType(NULL) && !v.IsType(UNDEFINED) && !v.IsType(TAG_METHOD_HANDLE) && !v.IsType(UNINITIALIZED) && !v.IsType(EMPTY_ARRAY_ITEM) && !v.IsType(TAG_ARGUMENT_START)
}

func (v Value) IsInteger() bool {
	if v.IsNumber() {
		f := v.AsNumber()
		_, frac := math.Modf(f)

		return frac == 0.0
	}
	return false
}
