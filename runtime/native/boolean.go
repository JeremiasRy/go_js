package native

import (
	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

const BOOLEAN_CONSTRUCTOR_NAME = "Boolean"

type BooleanConstructor struct {
	ObjObject
}

func NewBooleanConstructor() *BooleanConstructor {
	b := &BooleanConstructor{}
	b.Members = map[string]ObjectValueEntry{}
	return b
}

func (*BooleanConstructor) Type() object.ObjType {
	return object.OBJ_OBJECT_CONSTRUCTOR
}

func (*BooleanConstructor) String() string {
	return "function Boolean"
}

type BoxedPrimitive struct {
	ObjObject
	Primitive value.Value
}

func NewBoxedPrimitive(primitive value.Value) *BoxedPrimitive {
	b := &BoxedPrimitive{Primitive: primitive}
	b.Members = map[string]ObjectValueEntry{}
	b.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return b
}

func (b *BoxedPrimitive) String() string {
	return String(b.Primitive)
}

func (b *BoxedPrimitive) GetReferencingValues() []value.Value {
	values := []value.Value{b.Primitive}
	values = append(values, b.ObjObject.GetReferencingValues()...)
	return values
}

func NewBoxedPrimitiveValue(primitive value.Value) value.Value {
	return value.EncodeHandle(heap.Allocate(NewBoxedPrimitive(primitive)))
}

type BoxedValueOf struct {
	ObjNativeFn
}

func NewBoxedValueOf() *BoxedValueOf {
	b := &BoxedValueOf{}
	b.name = "valueOf"
	return b
}

