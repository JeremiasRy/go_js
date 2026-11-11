package vm

import (
	"fmt"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/value"
)

func (vm *VM) callObjectMethod(method *native.ObjectMethod, argCount int) (value.Value, error) {
	args := vm.popN(argCount)
	switch method.Kind {
	case native.OBJECT_IS:
		if len(args) < 2 {
			return value.FALSE, nil
		}
		if native.ObjectIsSame(args[0], args[1]) {
			return value.TRUE, nil
		}
		return value.FALSE, nil
	case native.OBJECT_ASSIGN:
		if len(args) == 0 || args[0] == value.NULL || args[0] == value.UNDEFINED {
			return value.UNDEFINED, fmt.Errorf("Cannot convert undefined or null to object")
		}
		target := args[0]
		for _, source := range args[1:] {
			if source == value.NULL || source == value.UNDEFINED {
				continue
			}
			if s, ok := vm.valueAsString(source); ok {
				for i, r := range []rune(s) {
					if err := vm.setMemberValue(target, native.IndexValue(i), value.EncodeHandle(heap.Allocate(native.NewLightString(string(r))))); err != nil {
						return value.UNDEFINED, err
					}
				}
				continue
			}
			obj, err := heap.GetObject(source.GetHandle())
			if err != nil {
				continue
			}
			if src, ok := obj.(*native.ObjObject); ok {
				for _, item := range objectEnumerableItems(src) {
					k := native.ObjectPropertyKeyValue(item.Key)
					v, err := vm.getMemberValue(source, k)
					if err != nil {
						return value.UNDEFINED, err
					}
					if targetObj, ok := native.ObjectAsObj(target); ok {
						if entry, found := targetObj.GetOwnEntry(k); found && !entry.IsAccessor() && !entry.Writable() {
							return value.UNDEFINED, fmt.Errorf("Cannot assign to read only property")
						}
					}
					if err := vm.setMemberValue(target, k, v); err != nil {
						return value.UNDEFINED, err
					}
				}
			}
		}
		return target, nil
	case native.OBJECT_ENTRIES:
		if len(args) == 0 {
			return value.UNDEFINED, fmt.Errorf("Cannot convert undefined or null to object")
		}
		source := args[0]
		entries := []value.Value{}
		if s, ok := vm.valueAsString(source); ok {
			for i, r := range []rune(s) {
				key := native.IndexValue(i)
				val := value.EncodeHandle(heap.Allocate(native.NewLightString(string(r))))
				entries = append(entries, native.NewPairArray(key, val))
			}
			return native.NewEntryArray(entries), nil
		}
		if obj, ok := native.ObjectAsObj(source); ok {
			for _, item := range objectEnumerableItems(obj) {
				k := native.ObjectPropertyKeyValue(item.Key)
				if len(item.Key) > 0 && item.Key[0] == 0 {
					continue
				}
				v, err := vm.getMemberValue(source, k)
				if err != nil {
					return value.UNDEFINED, err
				}
				entries = append(entries, native.NewPairArray(k, v))
			}
		}
		return native.NewEntryArray(entries), nil
	case native.OBJECT_FROM_ENTRIES:
		result := native.NewObjectHash()
		if len(args) == 0 {
			return value.EncodeHandle(heap.Allocate(result)), nil
		}
		entries := args[0]
		if entries.IsObject() {
			iterator, err := vm.acquireIterator(entries, 0)
			if err != nil {
				return value.UNDEFINED, fmt.Errorf("Iterator value is not an entry object")
			}
			for {
				done, err := vm.iteratorNext(iterator)
				if err != nil {
					return value.UNDEFINED, err
				}
				if done {
					break
				}
				entry, err := vm.iteratorCurrent(iterator)
				if err != nil {
					return value.UNDEFINED, err
				}
				if !entry.IsObject() {
					vm.closeIterator(iterator)
					return value.UNDEFINED, fmt.Errorf("Iterator value is not an entry object")
				}
				key, err := vm.getMemberValue(entry, native.IndexValue(0))
				if err != nil {
					return value.UNDEFINED, err
				}
				val, err := vm.getMemberValue(entry, native.IndexValue(1))
				if err != nil {
					return value.UNDEFINED, err
				}
				key, err = vm.toPropertyKey(key)
				if err != nil {
					return value.UNDEFINED, err
				}
				result.SetMember(key, val)
			}
		}
		return value.EncodeHandle(heap.Allocate(result)), nil
	case native.OBJECT_GET_OWN_PROPERTY_DESCRIPTORS:
		if len(args) == 0 {
			return value.UNDEFINED, fmt.Errorf("Cannot convert undefined or null to object")
		}
		result := native.NewObjectHash()
		if obj, ok := native.ObjectAsObj(args[0]); ok {
			descriptorMethod := &native.ObjectGetOwnPropertyDescriptor{}
			for _, item := range orderedEntriesForVM(obj) {
				k := native.ObjectPropertyKeyValue(item.Key)
				desc := descriptorMethod.Descriptor(args[0], k)
				result.SetMember(k, desc)
			}
		}
		return value.EncodeHandle(heap.Allocate(result)), nil
	case native.OBJECT_DEFINE_PROPERTIES:
		if len(args) < 2 {
			return value.UNDEFINED, nil
		}
		descriptorMethod := &native.ObjectDefineProperty{}
		if obj, ok := native.ObjectAsObj(args[1]); ok {
			for _, item := range objectEnumerableItems(obj) {
				k := native.ObjectPropertyKeyValue(item.Key)
				desc, err := vm.getMemberValue(args[1], k)
				if err != nil {
					return value.UNDEFINED, err
				}
				descriptorMethod.Define(args[0], k, desc)
			}
		}
		return args[0], nil
	case native.OBJECT_PREVENT_EXTENSIONS:
		if len(args) > 0 {
			if proxy, ok := vm.asProxy(args[0]); ok {
				prevented, err := vm.proxyPreventExtensions(proxy)
				if err != nil {
					return value.UNDEFINED, err
				}
				if !prevented {
					return value.UNDEFINED, fmt.Errorf("Cannot prevent extensions")
				}
				return args[0], nil
			}
			if obj, ok := native.ObjectAsObj(args[0]); ok {
				obj.SetExtensible(false)
			}
			return args[0], nil
		}
		return value.UNDEFINED, nil
	case native.OBJECT_IS_EXTENSIBLE:
		if len(args) > 0 {
			if proxy, ok := vm.asProxy(args[0]); ok {
				extensible, err := vm.proxyIsExtensible(proxy)
				if err != nil {
					return value.UNDEFINED, err
				}
				return boolValue(extensible), nil
			}
			if obj, ok := native.ObjectAsObj(args[0]); ok && obj.Extensible() {
				return value.TRUE, nil
			}
		}
		return value.FALSE, nil
	case native.OBJECT_SEAL:
		if len(args) > 0 {
			if obj, ok := native.ObjectAsObj(args[0]); ok {
				obj.Seal()
			}
			return args[0], nil
		}
		return value.UNDEFINED, nil
	case native.OBJECT_IS_SEALED:
		if len(args) > 0 {
			if obj, ok := native.ObjectAsObj(args[0]); ok && obj.IsSealed() {
				return value.TRUE, nil
			}
		}
		return value.FALSE, nil
	case native.OBJECT_FREEZE:
		if len(args) > 0 {
			if obj, ok := native.ObjectAsObj(args[0]); ok {
				obj.Freeze()
			}
			return args[0], nil
		}
		return value.UNDEFINED, nil
	case native.OBJECT_IS_FROZEN:
		if len(args) > 0 {
			if obj, ok := native.ObjectAsObj(args[0]); ok && obj.IsFrozen() {
				return value.TRUE, nil
			}
		}
		return value.FALSE, nil
	}
	return value.UNDEFINED, nil
}

func objectEnumerableItems(obj *native.ObjObject) []struct {
	Key   string
	Entry native.ObjectValueEntry
} {
	items := []struct {
		Key   string
		Entry native.ObjectValueEntry
	}{}
	for _, item := range orderedEntriesForVM(obj) {
		if item.Entry.Enumerable() {
			items = append(items, item)
		}
	}
	return items
}

func orderedEntriesForVM(obj *native.ObjObject) []struct {
	Key   string
	Entry native.ObjectValueEntry
} {
	// Reuse the native ordering by round-tripping through a temporary object.
	items := []struct {
		Key   string
		Entry native.ObjectValueEntry
	}{}
	for key, entry := range obj.Members {
		if key == native.PROTOTYPE_PROPERTY_STRING {
			continue
		}
		items = append(items, struct {
			Key   string
			Entry native.ObjectValueEntry
		}{key, entry})
	}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			ii, iok := canonicalIndex(items[i].Key)
			ji, jok := canonicalIndex(items[j].Key)
			swap := false
			if iok && jok {
				swap = ii > ji
			} else if iok != jok {
				swap = !iok
			} else if items[i].Entry.GetInit() > items[j].Entry.GetInit() {
				swap = true
			}
			if swap {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	return items
}

func canonicalIndex(key string) (uint64, bool) {
	if key == "" || (len(key) > 1 && key[0] == '0') {
		return 0, false
	}
	var result uint64
	for i := 0; i < len(key); i++ {
		if key[i] < '0' || key[i] > '9' {
			return 0, false
		}
		result = result*10 + uint64(key[i]-'0')
		if result >= 4294967295 {
			return 0, false
		}
	}
	return result, true
}

var _ = object.OBJ_OBJECT
