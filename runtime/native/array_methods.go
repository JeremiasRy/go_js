package native

import (
	"go_js/heap"
	"go_js/value"
)

type ArrayMethodKind int

const (
	ARRAY_INCLUDES ArrayMethodKind = iota
	ARRAY_INDEX_OF
	ARRAY_LAST_INDEX_OF
	ARRAY_AT
	ARRAY_POP
	ARRAY_UNSHIFT
	ARRAY_SLICE
	ARRAY_SPLICE
	ARRAY_CONCAT
	ARRAY_FLAT
	ARRAY_TO_SORTED
	ARRAY_TO_REVERSED
	ARRAY_TO_SPLICED
	ARRAY_WITH
	ARRAY_FIND_LAST
	ARRAY_FIND_LAST_INDEX
)

type ArrayMethod struct {
	ObjNativeFn
	Kind ArrayMethodKind
}

func NewArrayMethod(name string, kind ArrayMethodKind) *ArrayMethod {
	m := &ArrayMethod{Kind: kind}
	m.name = name
	return m
}

type ArrayCallbackKind int

const (
	ARRAY_SOME ArrayCallbackKind = iota
	ARRAY_EVERY
	ARRAY_FIND
	ARRAY_FIND_INDEX
	ARRAY_FIND_LAST_CB
	ARRAY_FIND_LAST_INDEX_CB
	ARRAY_SORT
	ARRAY_FLAT_MAP
)

type ArrayCallbackMethod struct {
	ObjNativeFn
	Kind ArrayCallbackKind
}

func NewArrayCallbackMethod(name string, kind ArrayCallbackKind) *ArrayCallbackMethod {
	m := &ArrayCallbackMethod{Kind: kind}
	m.name = name
	return m
}

type ArrayStaticKind int

const (
	ARRAY_STATIC_IS_ARRAY ArrayStaticKind = iota
	ARRAY_STATIC_OF
	ARRAY_STATIC_FROM
)

type ArrayStaticMethod struct {
	ObjNativeFn
	Kind ArrayStaticKind
}

func NewArrayStaticMethod(name string, kind ArrayStaticKind) *ArrayStaticMethod {
	m := &ArrayStaticMethod{Kind: kind}
	m.name = name
	return m
}

func IndexValue(i int) value.Value {
	return value.ValueFromFloat64(float64(i))
}

func NewArrayWithLength(length int) value.Value {
	arr := NewArrayFrom(make([]value.Value, length))
	for i := 0; i < length; i++ {
		arr.SetElementAt(i, value.EMPTY_ARRAY_ITEM)
	}
	return value.EncodeHandle(heap.Allocate(arr))
}

func CopyArray(arr *ObjArr) *ObjArr {
	items := make([]value.Value, len(arr.items))
	copy(items, arr.items)
	return NewArrayFrom(items)
}
