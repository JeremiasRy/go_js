package native

import (
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
	"math"
	"time"
)

type DateConstructor struct {
	ObjObject
}

func NewDateConstructor() *DateConstructor {
	oc := &DateConstructor{}
	oc.Members = map[string]ObjectValueEntry{}

	oc.Members["now"] = oc.NewValueEntry(value.EncodeHandle(allocator.Allocate(NewNow())))
	return oc
}

func (*DateConstructor) String() string {
	return "function Date"
}

func (*DateConstructor) Type() object.ObjType {
	return object.OBJ_DATE_CONSTRUCTOR
}

type Now struct {
	ObjNativeFn
}

func NewNow() *Now {
	n := &Now{}
	n.name = "Now"

	return n
}

func (*Now) Now() value.Value {
	return value.Value(math.Float64bits(float64(time.Now().UnixMilli())))
}
