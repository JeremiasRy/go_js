package native

import (
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"math"
)

type ObjectMethodKind int

const (
	OBJECT_IS ObjectMethodKind = iota
	OBJECT_ASSIGN
	OBJECT_ENTRIES
	OBJECT_FROM_ENTRIES
	OBJECT_GET_OWN_PROPERTY_DESCRIPTORS
	OBJECT_DEFINE_PROPERTIES
	OBJECT_PREVENT_EXTENSIONS
	OBJECT_IS_EXTENSIBLE
	OBJECT_SEAL
	OBJECT_IS_SEALED
	OBJECT_FREEZE
	OBJECT_IS_FROZEN
)

type ObjectMethod struct {
	ObjNativeFn
	Kind ObjectMethodKind
}

func NewObjectMethod(name string, kind ObjectMethodKind) *ObjectMethod {
	m := &ObjectMethod{Kind: kind}
	m.name = name
	return m
}

func NewObjectMethodConstructor() *ObjectConstructor {
	oc := NewObjectConstructor()
	methods := []struct {
		name string
		kind ObjectMethodKind
	}{
		{"is", OBJECT_IS},
		{"assign", OBJECT_ASSIGN},
		{"entries", OBJECT_ENTRIES},
		{"fromEntries", OBJECT_FROM_ENTRIES},
		{"getOwnPropertyDescriptors", OBJECT_GET_OWN_PROPERTY_DESCRIPTORS},
		{"defineProperties", OBJECT_DEFINE_PROPERTIES},
		{"preventExtensions", OBJECT_PREVENT_EXTENSIONS},
		{"isExtensible", OBJECT_IS_EXTENSIBLE},
		{"seal", OBJECT_SEAL},
		{"isSealed", OBJECT_IS_SEALED},
		{"freeze", OBJECT_FREEZE},
		{"isFrozen", OBJECT_IS_FROZEN},
	}
	for _, method := range methods {
		key := value.EncodeHandle(heap.Allocate(NewLightString(method.name)))
		oc.SetMember(key, value.EncodeHandle(heap.Allocate(NewObjectMethod(method.name, method.kind))))
		heap.PushToRoots(key)
	}
	return oc
}

func ObjectAsObj(v value.Value) (*ObjObject, bool) {
	if !v.IsObject() {
		return nil, false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return nil, false
	}
	switch o := obj.(type) {
	case *ObjObject:
		return o, true
	case *ObjArr:
		return &o.ObjObject, true
	case *ObjString:
		return &o.ObjObject, true
	case *ObjError:
		return &o.ObjObject, true
	}
	return nil, false
}

func ObjectEnumerableKeys(o *ObjObject) []string {
	keys := []string{}
	for _, item := range orderedMemberEntries(o) {
		if item.Entry.Enumerable() {
			keys = append(keys, item.Key)
		}
	}
	return keys
}

func ObjectIsSame(a, b value.Value) bool {
	if a.IsNumber() && b.IsNumber() {
		an, bn := a.AsNumber(), b.AsNumber()
		if math.IsNaN(an) && math.IsNaN(bn) {
			return true
		}
		if an == 0 && bn == 0 {
			return math.Signbit(an) == math.Signbit(bn)
		}
		return an == bn
	}
	if a.IsObject() && b.IsObject() {
		if as, ok := ObjectStringValue(a); ok {
			if bs, ok := ObjectStringValue(b); ok {
				return as == bs
			}
		}
		return heap.GetPointer(a.GetHandle()) == heap.GetPointer(b.GetHandle())
	}
	return a == b
}

func ObjectStringValue(v value.Value) (string, bool) {
	if !v.IsObject() {
		return "", false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return "", false
	}
	switch obj := obj.(type) {
	case *LightString:
		return obj.String(), true
	case *ObjString:
		return obj.String(), true
	case *ObjStringBuilder:
		return obj.String(), true
	}
	return "", false
}

func NewEntryArray(entries []value.Value) value.Value {
	return value.EncodeHandle(heap.Allocate(NewArrayFrom(entries)))
}

func NewPairArray(key, value_ value.Value) value.Value {
	return value.EncodeHandle(heap.Allocate(NewArrayFrom([]value.Value{key, value_})))
}

func ObjectPropertyKeyValue(key string) value.Value {
	return keyValueFromStringLocal(key)
}

func keyValueFromStringLocal(key string) value.Value {
	if len(key) > 0 && key[0] == 0 {
		if symbol, found := SymbolByKey(key); found {
			return symbol
		}
	}
	return value.EncodeHandle(heap.Allocate(NewLightString(key)))
}

var _ = object.OBJ_OBJECT
