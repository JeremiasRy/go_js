package native

import (
	"fmt"
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
)

type Map struct {
	object.GC_TAG
	ObjObject
	init    int
	values  map[value.Value]ObjectValueEntry
	handles map[value.Value]value.Value
}

func (m *Map) String() string {
	return fmt.Sprintf("Map(%d)", len(m.values))
}

func (*Map) Type() object.ObjType {
	return object.OBJ_MAP
}

func NewMap() *Map {
	m := &Map{}
	m.Members = map[string]ObjectValueEntry{}
	m.values = map[value.Value]ObjectValueEntry{}
	m.handles = map[value.Value]value.Value{}

	m.SetMember(KEY_PROTO, PROTOTYPE_MAP)
	m.SetMember(KEY_SIZE, value.ValueFromFloat64(0))
	return m
}

func (m *Map) Keys() []value.Value {
	r := make([]value.Value, len(m.values))

	for k, item := range m.values {
		r[item.init] = m.handles[k]
	}

	return r
}

func (m *Map) Values() []value.Value {
	r := make([]value.Value, len(m.values))

	for _, item := range m.values {
		r[item.init] = item.Value
	}

	return r
}

type MapKeys struct {
	ObjNativeFn
}

func NewMapKeys() *MapKeys {
	k := &MapKeys{}
	k.name = "keys"

	return k
}

func (*MapKeys) Keys(owner *Map) *object.Iterator {
	return object.NewKeyIterator(owner)
}

type MapSet struct {
	ObjNativeFn
}

func NewMapSet() *MapSet {
	s := &MapSet{}
	s.name = "set"
	return s
}

func (*MapSet) Set(owner *Map, k value.Value, v value.Value) {
	// handles can point to the same object so we'll modify our key to contain the actual pointer
	if k.IsObject() {
		ptr := value.EncodeHandle(allocator.GetPointer(k.GetHandle()))
		owner.handles[ptr] = k
		k = ptr
	}

	if _, found := owner.values[k]; found {
		entry := owner.values[k]
		entry.Value = v

		owner.values[k] = entry
		return
	}

	owner.values[k] = ObjectValueEntry{
		Value: v,
		init:  owner.init,
	}

	owner.init++
	owner.SetMember(KEY_SIZE, value.ValueFromFloat64(float64(len(owner.values))))
}

type MapGet struct {
	ObjNativeFn
}

func NewMapGet() *MapGet {
	g := &MapGet{}
	g.name = "get"
	return g
}

func (*MapGet) Get(owner *Map, k value.Value) value.Value {
	if k.IsObject() {
		k = value.EncodeHandle(allocator.GetPointer(k.GetHandle()))
	}
	if v, found := owner.values[k]; found {
		return v.Value
	}
	return value.UNDEFINED
}

type MapHas struct {
	ObjNativeFn
}

func NewMapHas() *MapHas {
	h := &MapHas{}
	h.name = "has"
	return h
}

func (*MapHas) Has(owner *Map, k value.Value) value.Value {
	if k.IsObject() {
		k = value.EncodeHandle(allocator.GetPointer(k.GetHandle()))
	}
	if _, found := owner.values[k]; found {
		return value.TRUE
	}
	return value.FALSE
}
