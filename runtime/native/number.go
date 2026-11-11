package native

import (
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"math"
	"strconv"
)

const NUMBER_CONSTRUCTOR_NAME = "Number"

type NumberConstructor struct {
	ObjObject
}

func NewNumberConstructor() *NumberConstructor {
	nc := &NumberConstructor{}
	nc.Members = map[string]ObjectValueEntry{}
	nc.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	nc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("isNaN"))), value.EncodeHandle(heap.Allocate(NewNumberIsNaN())))
	nc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("isFinite"))), value.EncodeHandle(heap.Allocate(NewNumberIsFinite())))
	nc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("isInteger"))), value.EncodeHandle(heap.Allocate(NewNumberIsInteger())))
	nc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("isSafeInteger"))), value.EncodeHandle(heap.Allocate(NewNumberIsSafeInteger())))
	nc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("MAX_SAFE_INTEGER"))), value.ValueFromFloat64(9007199254740991))
	nc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("MIN_SAFE_INTEGER"))), value.ValueFromFloat64(-9007199254740991))
	nc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("EPSILON"))), value.ValueFromFloat64(2.220446049250313e-16))
	nc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("MIN_VALUE"))), value.ValueFromFloat64(5e-324))
	nc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("MAX_VALUE"))), value.ValueFromFloat64(math.MaxFloat64))
	return nc
}

func (*NumberConstructor) String() string {
	return "function Number"
}

func (*NumberConstructor) Type() object.ObjType {
	return object.OBJ_OBJECT_CONSTRUCTOR
}

type NumberIsNaN struct {
	ObjNativeFn
}

func NewNumberIsNaN() *NumberIsNaN {
	n := &NumberIsNaN{}
	n.name = "isNaN"
	return n
}

type NumberIsFinite struct {
	ObjNativeFn
}

func NewNumberIsFinite() *NumberIsFinite {
	n := &NumberIsFinite{}
	n.name = "isFinite"
	return n
}

type NumberIsInteger struct {
	ObjNativeFn
}

func NewNumberIsInteger() *NumberIsInteger {
	n := &NumberIsInteger{}
	n.name = "isInteger"
	return n
}

type NumberIsSafeInteger struct {
	ObjNativeFn
}

func NewNumberIsSafeInteger() *NumberIsSafeInteger {
	n := &NumberIsSafeInteger{}
	n.name = "isSafeInteger"
	return n
}

func NumberIsNaNValue(v value.Value) value.Value {
	if v.IsNumber() {
		if math.IsNaN(v.AsNumber()) {
			return value.TRUE
		}
	}
	return value.FALSE
}

func NumberIsFiniteValue(v value.Value) value.Value {
	if v.IsNumber() {
		n := v.AsNumber()
		if !math.IsNaN(n) && !math.IsInf(n, 0) {
			return value.TRUE
		}
	}
	return value.FALSE
}

func NumberIsIntegerValue(v value.Value) value.Value {
	if v.IsNumber() {
		n := v.AsNumber()
		if !math.IsNaN(n) && !math.IsInf(n, 0) && math.Trunc(n) == n {
			return value.TRUE
		}
	}
	return value.FALSE
}

func NumberIsSafeIntegerValue(v value.Value) value.Value {
	if NumberIsIntegerValue(v) == value.TRUE {
		n := v.AsNumber()
		if math.Abs(n) <= 9007199254740991 {
			return value.TRUE
		}
	}
	return value.FALSE
}

func NumberToString(n float64, radix int) string {
	if radix == 10 {
		return nativeNumberString(n)
	}
	if radix < 2 || radix > 36 {
		return ""
	}
	if math.IsNaN(n) {
		return "NaN"
	}
	if math.IsInf(n, 0) {
		return nativeNumberString(n)
	}
	negative := n < 0
	if negative {
		n = -n
	}
	integer := int64(n)
	if integer == 0 {
		return "0"
	}
	digits := "0123456789abcdefghijklmnopqrstuvwxyz"
	result := ""
	for integer > 0 {
		result = string(digits[integer%int64(radix)]) + result
		integer /= int64(radix)
	}
	if negative {
		result = "-" + result
	}
	return result
}

func ParseNumberRadix(v value.Value, radix int) (value.Value, bool) {
	if v.IsObject() {
		obj, err := heap.GetObject(v.GetHandle())
		if err != nil {
			return value.UNDEFINED, false
		}
		str := obj.String()
		n, err := strconv.ParseInt(str, radix, 64)
		if err != nil {
			return value.UNDEFINED, false
		}
		return value.ValueFromFloat64(float64(n)), true
	}
	return value.UNDEFINED, false
}
