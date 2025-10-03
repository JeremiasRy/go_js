package native

import (
	"fmt"
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
)

type Set struct {
	ObjObject
	values map[value.Value]struct{}
}

func (m *Set) String() string {
	return fmt.Sprintf("Set(%d)", len(m.values))
}

func (*Set) Type() object.ObjType {
	return object.OBJ_SET
}

func NewSet() *Set {
	s := &Set{}

	s.Members = map[string]ObjectValueEntry{}
	s.values = map[value.Value]struct{}{}

	s.SetMember(KEY_PROTO, PROTOTYPE_SET)
	s.SetMember(KEY_SIZE, value.ValueFromFloat64(0))
	return s
}

type SetAdd struct {
	ObjNativeFn
}

func NewSetAdd() *SetAdd {
	sa := &SetAdd{}
	sa.name = "add"

	return sa
}

func (*SetAdd) Add(owner *Set, v value.Value) {
	if v.IsObject() {
		// decode handle to pointer
		v = value.EncodeHandle(allocator.GetPointer(v.GetHandle()))
	}

	owner.values[v] = struct{}{}
	owner.SetMember(KEY_SIZE, value.ValueFromFloat64(float64(len(owner.values))))
}

type SetHas struct {
	ObjNativeFn
}

func NewSetHas() *SetHas {
	sh := &SetHas{}
	sh.name = "has"

	return sh
}

func (*SetHas) Has(owner *Set, v value.Value) value.Value {
	if v.IsObject() {
		// decode handle to pointer
		v = value.EncodeHandle(allocator.GetPointer(v.GetHandle()))
	}

	if _, found := owner.values[v]; found {
		return value.TRUE
	}
	return value.FALSE
}
