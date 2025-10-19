package native

import (
	"fmt"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

type Map struct {
	ObjObject
	init   int
	values map[value.Value]ObjectValueEntry
}

func (m *Map) String() string {
	return fmt.Sprintf("Map(%d)", len(m.values))
}

func (*Map) Type() object.ObjType {
	return object.OBJ_MAP
}

func (o *Map) GetReferencingValues() []value.Value {
	arr := []value.Value{}
	for _, v := range o.Members {
		if v.Value.IsObject() {
			arr = append(arr, v.Value)
		}
	}

	for k, v := range o.values {

		if v.Value.IsObject() {
			arr = append(arr, v.Value)
		}

		if k.IsObject() {
			arr = append(arr, k)
		}
	}
	return arr
}

func NewMap() *Map {
	m := &Map{}
	m.Members = map[string]ObjectValueEntry{}
	m.values = map[value.Value]ObjectValueEntry{}

	m.SetMember(KEY_PROTO, PROTOTYPE_MAP)
	m.SetMember(KEY_SIZE, value.ValueFromFloat64(0))
	return m
}

func (m *Map) Keys() []value.Value {
	r := make([]value.Value, len(m.values))

	for k, item := range m.values {
		r[item.init] = k
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
	if k.IsObject() {
		keyPtr := heap.GetPointer(k.GetHandle())

		for k := range owner.values {
			if !k.IsObject() {
				continue
			}
			if heap.GetPointer(k.GetHandle()) == keyPtr {

				entry := owner.values[k]
				entry.Value = v

				owner.values[k] = entry
				return
			}
		}
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
		kPtr := heap.GetPointer(k.GetHandle())

		for k, v := range owner.values {
			if !k.IsObject() {
				continue
			}
			if heap.GetPointer(k.GetHandle()) == kPtr {
				return v.Value
			}
		}
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
		kPtr := heap.GetPointer(k.GetHandle())

		for k := range owner.values {
			if !k.IsObject() {
				continue
			}
			if heap.GetPointer(k.GetHandle()) == kPtr {
				return value.TRUE
			}
		}
	}
	if _, found := owner.values[k]; found {
		return value.TRUE
	}
	return value.FALSE
}
