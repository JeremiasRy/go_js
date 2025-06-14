package vm

import (
	"math"
	"strconv"
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
	TAG_TRUE      Value = QNAN | 0x0000000000000003
	TAG_FALSE     Value = QNAN | 0x0000000000000004
)

func (v Value) isObject() bool {
	return v&TAG_OBJ == TAG_OBJ
}

func (v Value) isNaN() bool {
	return (v&STANDARD_NAN == STANDARD_NAN) &&
		(v != TAG_OBJ) && (v != TAG_NIL) && (v != TAG_UNDEFINED) && (v != TAG_FALSE) && (v != TAG_TRUE)
}

func (v Value) asNumber() float64 {
	return math.Float64frombits(uint64(v))
}

func (v Value) getRegister() uint32 {
	return uint32(v & ENCODE_MASK)
}

func EncodeObject(register uint32) Value {
	return TAG_OBJ | Value(register)
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

func IsBoolean(v Value) bool {
	return (TAG_TRUE&v == TAG_TRUE) || (TAG_FALSE&v == TAG_FALSE)
}

// TODO extend this to handle 'falsy' values: nill, undefined, "", 0, 1, etc...
func AsBoolean(v Value) bool {
	return TAG_TRUE&v == TAG_TRUE
}

func isType(tag Value, v Value) bool {
	return tag&v == tag
}

func (v *Value) String() string {
	if isType(TAG_FALSE, *v) {
		return "False"
	} else if isType(TAG_TRUE, *v) {
		return "True"
	} else if !v.isObject() {
		return strconv.FormatFloat(v.asNumber(), 'g', -1, 64)
	} else {
		return "<heap object> -> r" + strconv.Itoa(int(v.getRegister()))
	}
}
