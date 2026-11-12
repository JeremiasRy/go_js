package native

import (
	"fmt"
	"go_js/heap"
	"go_js/value"
)

func cloneObjectMembers(dst, src *ObjObject, memo map[uint32]value.Value) {
	dst.init = src.init
	dst.nonExtensible = src.nonExtensible
	for key, entry := range src.Members {
		cloned := entry
		if key == PROTOTYPE_PROPERTY_STRING {
			dst.Members[key] = cloned
			continue
		}
		cloned.Value = cloneValue(entry.Value, memo)
		cloned.Get = cloneValue(entry.Get, memo)
		cloned.Set = cloneValue(entry.Set, memo)
		dst.Members[key] = cloned
	}
}

func cloneValue(v value.Value, memo map[uint32]value.Value) value.Value {
	if !v.IsObject() {
		return v
	}
	ptr := heap.GetPointer(v.GetHandle())
	if existing, found := memo[ptr]; found {
		return existing
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return v
	}
	switch source := obj.(type) {
	case *LightString, *ObjString, *ObjBigInt:
		return v
	case *ObjObject:
		target := NewObjectHash()
		result := value.EncodeHandle(heap.Allocate(target))
		memo[ptr] = result
		cloneObjectMembers(target, source, memo)
		return result
	case *ObjArr:
		target := NewArrayFrom(make([]value.Value, len(source.items)))
		result := value.EncodeHandle(heap.Allocate(target))
		memo[ptr] = result
		for i, item := range source.items {
			if item != value.EMPTY_ARRAY_ITEM {
				target.items[i] = cloneValue(item, memo)
			}
		}
		cloneObjectMembers(&target.ObjObject, &source.ObjObject, memo)
		return result
	case *Map:
		target := NewMap()
		result := value.EncodeHandle(heap.Allocate(target))
		memo[ptr] = result
		setter := &MapSet{}
		for _, entry := range source.orderedEntries() {
			setter.Set(target, cloneValue(entry.Key, memo), cloneValue(entry.Value, memo))
		}
		cloneObjectMembers(&target.ObjObject, &source.ObjObject, memo)
		return result
	case *Set:
		target := NewSet()
		result := value.EncodeHandle(heap.Allocate(target))
		memo[ptr] = result
		adder := &SetAdd{}
		for _, item := range source.Order() {
			adder.Add(target, cloneValue(item, memo))
		}
		cloneObjectMembers(&target.ObjObject, &source.ObjObject, memo)
		return result
	case *DateObject:
		target := NewDateObject(source.Millis)
		result := value.EncodeHandle(heap.Allocate(target))
		memo[ptr] = result
		cloneObjectMembers(&target.ObjObject, &source.ObjObject, memo)
		return result
	case *ObjRegExp:
		target := NewRegExpObject(source.Pattern, source.Flags)
		target.LastIndex = source.LastIndex
		result := value.EncodeHandle(heap.Allocate(target))
		memo[ptr] = result
		cloneObjectMembers(&target.ObjObject, &source.ObjObject, memo)
		return result
	case *ObjError:
		target := NewNamedError(source.errorName)
		result := value.EncodeHandle(heap.Allocate(target))
		memo[ptr] = result
		cloneObjectMembers(&target.ObjObject, &source.ObjObject, memo)
		return result
	case *BoxedPrimitive:
		target := NewBoxedPrimitive(cloneValue(source.Primitive, memo))
		result := value.EncodeHandle(heap.Allocate(target))
		memo[ptr] = result
		cloneObjectMembers(&target.ObjObject, &source.ObjObject, memo)
		return result
	case *ObjArrayBuffer:
		target := NewArrayBuffer(len(source.Data))
		copy(target.Data, source.Data)
		target.Shared = source.Shared
		result := value.EncodeHandle(heap.Allocate(target))
		memo[ptr] = result
		cloneObjectMembers(&target.ObjObject, &source.ObjObject, memo)
		return result
	case *ObjTypedArray:
		bufferValue := cloneValue(source.BufferValue, memo)
		buffer, _ := heap.GetObject(bufferValue.GetHandle())
		arrayBuffer, _ := buffer.(*ObjArrayBuffer)
		target := NewTypedArray(arrayBuffer, bufferValue, source.Offset, source.Length, source.Kind)
		target.IsBuffer = source.IsBuffer
		result := value.EncodeHandle(heap.Allocate(target))
		memo[ptr] = result
		cloneObjectMembers(&target.ObjObject, &source.ObjObject, memo)
		return result
	case *ObjDataView:
		bufferValue := cloneValue(source.BufferValue, memo)
		buffer, _ := heap.GetObject(bufferValue.GetHandle())
		arrayBuffer, _ := buffer.(*ObjArrayBuffer)
		target := NewDataView(arrayBuffer, bufferValue, source.Offset, source.Length)
		result := value.EncodeHandle(heap.Allocate(target))
		memo[ptr] = result
		cloneObjectMembers(&target.ObjObject, &source.ObjObject, memo)
		return result
	case *ObjWeakRef:
		return value.UNDEFINED
	case *ObjSymbol:
		return value.UNDEFINED
	}
	return value.UNDEFINED
}

func StructuredClone(v value.Value) (value.Value, error) {
	result := cloneValue(v, map[uint32]value.Value{})
	if result == value.UNDEFINED && v != value.UNDEFINED {
		return value.UNDEFINED, fmt.Errorf("Value could not be cloned")
	}
	return result, nil
}
