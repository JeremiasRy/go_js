package vm

import (
	"math"
	"strconv"
)

type Value uint64

const (
	QNAN        Value = 0x7ffc000000000000
	SIGN        Value = 0x8000000000000000
	ENCODE_MASK Value = math.MaxUint64 ^ (QNAN | SIGN)
	TAG_OBJ     Value = QNAN | SIGN
)

func (v *Value) isObject() bool {
	return *v&TAG_OBJ == TAG_OBJ
}

func (v *Value) asNumber() float64 {
	return math.Float64frombits(uint64(*v))
}

func (v *Value) getRegister() uint32 {
	return uint32(*v & ENCODE_MASK)
}

func EncodeObject(register uint32) Value {
	return TAG_OBJ | Value(register)
}

func ValueFromFloat64(number float64) Value {
	return Value(math.Float64bits(number))
}

func (v *Value) String() string {
	var strValue string

	if !v.isObject() {
		strValue = strconv.FormatFloat(v.asNumber(), 'g', -1, 64)
	} else {
		//
	}
	return strValue
}
