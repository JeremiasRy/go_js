package native

import (
	"fmt"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"math"
	"sort"
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

func normalizeCollectionKey(v value.Value) value.Value {
	if v.IsNumber() {
		n := v.AsNumber()
		if math.IsNaN(n) {
			return value.STANDARD_NAN
		}
		if n == 0 {
			return value.ValueFromFloat64(0)
		}
	}
	if v.IsObject() {
		if obj, err := heap.GetObject(v.GetHandle()); err == nil {
			if builder, ok := obj.(*ObjStringBuilder); ok {
				return value.EncodeHandle(heap.Allocate(NewLightString(builder.String())))
			}
		}
	}
	return v
}

func NewMap() *Map {
	m := &Map{}
	m.Members = map[string]ObjectValueEntry{}
	m.values = map[value.Value]ObjectValueEntry{}

	m.SetMember(KEY_PROTO, PROTOTYPE_MAP)
	entry := m.NewValueEntry(value.ValueFromFloat64(0))
	entry.nonWritable = true
	entry.nonEnumerable = true
	m.Members[objectKeyStringMust(KEY_SIZE)] = entry
	return m
}

func objectKeyStringMust(k value.Value) string {
	key, _ := objectKeyString(k)
	return key
}

func (m *Map) orderedEntries() []struct {
	Key   value.Value
	Value value.Value
} {
	entries := make([]struct {
		Key   value.Value
		Value value.Value
	}, 0, len(m.values))
	for k, item := range m.values {
		entries = append(entries, struct {
			Key   value.Value
			Value value.Value
		}{k, item.Value})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return m.values[entries[i].Key].init < m.values[entries[j].Key].init
	})
	return entries
}

func (m *Map) Keys() []value.Value {
	entries := m.orderedEntries()
	r := make([]value.Value, len(entries))
	for i, entry := range entries {
		r[i] = entry.Key
	}
	return r
}

func (m *Map) Values() []value.Value {
	entries := m.orderedEntries()
	r := make([]value.Value, len(entries))
	for i, entry := range entries {
		r[i] = value.EncodeHandle(heap.Allocate(NewArrayFrom([]value.Value{entry.Key, entry.Value})))
	}
	return r
}

func (m *Map) ValuesList() []value.Value {
	entries := m.orderedEntries()
	r := make([]value.Value, len(entries))
	for i, entry := range entries {
		r[i] = entry.Value
	}
	return r
}

func (m *Map) Entries() []value.Value {
	return m.Values()
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
	return object.NewLiveIterator(owner.Keys)
}

type MapValues struct {
	ObjNativeFn
}

func NewMapValues() *MapValues {
	v := &MapValues{}
	v.name = "values"
	return v
}

func (*MapValues) Values(owner *Map) *object.Iterator {
	return object.NewLiveIterator(owner.ValuesList)
}

type MapEntries struct {
	ObjNativeFn
}

func NewMapEntries() *MapEntries {
	e := &MapEntries{}
	e.name = "entries"
	return e
}

func (*MapEntries) Entries(owner *Map) *object.Iterator {
	return object.NewLiveIterator(owner.Entries)
}

type MapDelete struct {
	ObjNativeFn
}

func NewMapDelete() *MapDelete {
	d := &MapDelete{}
	d.name = "delete"
	return d
}

func (*MapDelete) Delete(owner *Map, k value.Value) value.Value {
	k = normalizeCollectionKey(k)
	if k.IsObject() {
		kPtr := heap.GetPointer(k.GetHandle())
		for key := range owner.values {
			if key.IsObject() && heap.GetPointer(key.GetHandle()) == kPtr {
				delete(owner.values, key)
				owner.SetMember(KEY_SIZE, value.ValueFromFloat64(float64(len(owner.values))))
				return value.TRUE
			}
		}
	}
	if _, found := owner.values[k]; found {
		delete(owner.values, k)
		owner.SetMember(KEY_SIZE, value.ValueFromFloat64(float64(len(owner.values))))
		return value.TRUE
	}
	return value.FALSE
}

type MapClear struct {
	ObjNativeFn
}

func NewMapClear() *MapClear {
	c := &MapClear{}
	c.name = "clear"
	return c
}

func (*MapClear) ClearMap(owner *Map) value.Value {
	owner.values = map[value.Value]ObjectValueEntry{}
	owner.init = 0
	owner.SetMember(KEY_SIZE, value.ValueFromFloat64(0))
	return value.UNDEFINED
}

type MapForEach struct {
	ObjNativeFn
}

func NewMapForEach() *MapForEach {
	f := &MapForEach{}
	f.name = "forEach"
	return f
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
	k = normalizeCollectionKey(k)
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
	k = normalizeCollectionKey(k)
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
	k = normalizeCollectionKey(k)

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
