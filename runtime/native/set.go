package native

import (
	"fmt"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

type Set struct {
	ObjObject
	values map[value.Value]struct{}
	order  []value.Value
}

func (m *Set) String() string {
	return fmt.Sprintf("Set(%d)", len(m.values))
}

func (*Set) Type() object.ObjType {
	return object.OBJ_SET
}

func (s *Set) GetReferencingValues() []value.Value {
	arr := []value.Value{}
	arr = append(arr, s.order...)
	for key := range s.values {
		arr = append(arr, key)
	}
	arr = append(arr, s.ObjObject.GetReferencingValues()...)
	return arr
}

func NewSet() *Set {
	s := &Set{}

	s.Members = map[string]ObjectValueEntry{}
	s.values = map[value.Value]struct{}{}
	s.order = []value.Value{}

	s.SetMember(KEY_PROTO, PROTOTYPE_SET)
	entry := s.NewValueEntry(value.ValueFromFloat64(0))
	entry.nonWritable = true
	entry.nonEnumerable = true
	s.Members[objectKeyStringMust(KEY_SIZE)] = entry
	return s
}

func (s *Set) Keys() []value.Value {
	result := make([]value.Value, len(s.order))
	copy(result, s.order)
	return result
}

func (s *Set) Values() []value.Value {
	return s.Keys()
}

func (s *Set) Order() []value.Value {
	return s.order
}

func (s *Set) HasValue(v value.Value) bool {
	v = normalizeCollectionKey(v)
	if v.IsObject() {
		ptr := heap.GetPointer(v.GetHandle())
		for key := range s.values {
			if key.IsObject() && heap.GetPointer(key.GetHandle()) == ptr {
				return true
			}
		}
		return false
	}
	_, found := s.values[v]
	return found
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
	original := v
	v = normalizeCollectionKey(v)
	if v.IsObject() {
		ptr := heap.GetPointer(v.GetHandle())
		for key := range owner.values {
			if key.IsObject() && heap.GetPointer(key.GetHandle()) == ptr {
				return
			}
		}
	} else if _, found := owner.values[v]; found {
		return
	}
	owner.values[v] = struct{}{}
	stored := original
	if !original.IsObject() {
		stored = normalizeCollectionKey(original)
	}
	owner.order = append(owner.order, stored)
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
	if owner.HasValue(v) {
		return value.TRUE
	}
	return value.FALSE
}

type SetDelete struct {
	ObjNativeFn
}

func NewSetDelete() *SetDelete {
	d := &SetDelete{}
	d.name = "delete"
	return d
}

func (*SetDelete) Delete(owner *Set, v value.Value) value.Value {
	normalized := normalizeCollectionKey(v)
	var foundKey value.Value
	found := false
	if normalized.IsObject() {
		ptr := heap.GetPointer(normalized.GetHandle())
		for key := range owner.values {
			if key.IsObject() && heap.GetPointer(key.GetHandle()) == ptr {
				foundKey = key
				found = true
				break
			}
		}
	} else if _, ok := owner.values[normalized]; ok {
		foundKey = normalized
		found = true
	}
	if !found {
		return value.FALSE
	}
	delete(owner.values, foundKey)
	for i, item := range owner.order {
		itemNormalized := normalizeCollectionKey(item)
		match := false
		if itemNormalized.IsObject() && foundKey.IsObject() {
			match = heap.GetPointer(itemNormalized.GetHandle()) == heap.GetPointer(foundKey.GetHandle())
		} else {
			match = itemNormalized == foundKey
		}
		if match {
			owner.order = append(owner.order[:i], owner.order[i+1:]...)
			break
		}
	}
	owner.SetMember(KEY_SIZE, value.ValueFromFloat64(float64(len(owner.values))))
	return value.TRUE
}

type SetClear struct {
	ObjNativeFn
}

func NewSetClear() *SetClear {
	c := &SetClear{}
	c.name = "clear"
	return c
}

func (*SetClear) ClearSet(owner *Set) value.Value {
	owner.values = map[value.Value]struct{}{}
	owner.order = []value.Value{}
	owner.SetMember(KEY_SIZE, value.ValueFromFloat64(0))
	return value.UNDEFINED
}

type SetKeys struct {
	ObjNativeFn
}

func NewSetKeys() *SetKeys {
	k := &SetKeys{}
	k.name = "keys"
	return k
}

func (*SetKeys) Keys(owner *Set) *object.Iterator {
	return object.NewLiveIterator(owner.Keys)
}

type SetValues struct {
	ObjNativeFn
}

func NewSetValues() *SetValues {
	v := &SetValues{}
	v.name = "values"
	return v
}

func (*SetValues) Values(owner *Set) *object.Iterator {
	return object.NewLiveIterator(owner.Values)
}

type SetEntries struct {
	ObjNativeFn
}

func NewSetEntries() *SetEntries {
	e := &SetEntries{}
	e.name = "entries"
	return e
}

func (*SetEntries) Entries(owner *Set) *object.Iterator {
	return object.NewLiveIterator(func() []value.Value {
		result := make([]value.Value, len(owner.order))
		for i, item := range owner.order {
			result[i] = value.EncodeHandle(heap.Allocate(NewArrayFrom([]value.Value{item, item})))
		}
		return result
	})
}

type SetOperationKind int

const (
	SET_UNION SetOperationKind = iota
	SET_INTERSECTION
	SET_DIFFERENCE
	SET_SYMMETRIC_DIFFERENCE
	SET_IS_SUBSET_OF
	SET_IS_SUPERSET_OF
	SET_IS_DISJOINT_FROM
)

type SetOperation struct {
	ObjNativeFn
	Kind SetOperationKind
}

func NewSetOperation(name string, kind SetOperationKind) *SetOperation {
	fn := &SetOperation{Kind: kind}
	fn.name = name
	return fn
}

type SetForEach struct {
	ObjNativeFn
}

func NewSetForEach() *SetForEach {
	f := &SetForEach{}
	f.name = "forEach"
	return f
}
