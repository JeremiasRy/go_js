package vm

import (
	"fmt"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/value"
	"math"
	"sort"
	"strings"
)

func arrayAt(arr *native.ObjArr, i int) value.Value {
	if i < 0 || i >= arr.Length() {
		return value.UNDEFINED
	}
	v := arr.GetElementAt(i)
	if v == value.EMPTY_ARRAY_ITEM {
		return value.UNDEFINED
	}
	return v
}

func (vm *VM) arrayInteger(v value.Value, fallback int) int {
	if v == value.UNDEFINED {
		return fallback
	}
	n, err := vm.toNumber(v)
	if err != nil || math.IsNaN(n) {
		return 0
	}
	if math.IsInf(n, 1) {
		return int(^uint(0) >> 1)
	}
	if math.IsInf(n, -1) {
		return -int(^uint(0)>>1) - 1
	}
	return int(math.Trunc(n))
}

func normalizeIndex(index, length int) int {
	if index < 0 {
		index += length
	}
	if index < 0 {
		return 0
	}
	if index > length {
		return length
	}
	return index
}

func (vm *VM) sameValueZero(a, b value.Value) bool {
	if strictEquals(a, b) {
		return true
	}
	if a.IsNumber() && b.IsNumber() && math.IsNaN(a.AsNumber()) && math.IsNaN(b.AsNumber()) {
		return true
	}
	return false
}

func (vm *VM) arrayReceiver(this value.Value) (*native.ObjArr, error) {
	if !this.IsObject() {
		return nil, fmt.Errorf("array method called on non-object")
	}
	obj, err := heap.GetObject(this.GetHandle())
	if err != nil {
		return nil, err
	}
	arr, ok := obj.(*native.ObjArr)
	if !ok {
		return nil, fmt.Errorf("array method called on non-array")
	}
	return arr, nil
}

func (vm *VM) callArrayMethod(method *native.ArrayMethod, this value.Value, argCount int) (value.Value, error) {
	if this.IsObject() {
		obj, err := heap.GetObject(this.GetHandle())
		if err == nil {
			if _, ok := obj.(*native.ObjArr); !ok {
				if method.Kind == native.ARRAY_POP || method.Kind == native.ARRAY_UNSHIFT {
					args := vm.popN(argCount)
					return vm.genericArrayMutation(method.Kind, this, obj.(object.Hashable), args)
				}
			}
		}
	}
	arr, err := vm.arrayReceiver(this)
	if err != nil {
		return value.UNDEFINED, err
	}
	args := vm.popN(argCount)
	length := arr.Length()
	switch method.Kind {
	case native.ARRAY_INCLUDES:
		search := value.UNDEFINED
		from := 0
		if len(args) > 0 {
			search = args[0]
		}
		if len(args) > 1 {
			from = normalizeIndex(vm.arrayInteger(args[1], 0), length)
		}
		for i := from; i < length; i++ {
			if vm.sameValueZero(arrayAt(arr, i), search) {
				return value.TRUE, nil
			}
		}
		return value.FALSE, nil
	case native.ARRAY_INDEX_OF:
		search := value.UNDEFINED
		from := 0
		if len(args) > 0 {
			search = args[0]
		}
		if len(args) > 1 {
			from = normalizeIndex(vm.arrayInteger(args[1], 0), length)
		}
		for i := from; i < length; i++ {
			if arr.HasIndex(i) && strictEquals(arr.GetElementAt(i), search) {
				return native.IndexValue(i), nil
			}
		}
		return value.ValueFromFloat64(-1), nil
	case native.ARRAY_LAST_INDEX_OF:
		search := value.UNDEFINED
		from := length - 1
		if len(args) > 0 {
			search = args[0]
		}
		if len(args) > 1 {
			n := vm.arrayInteger(args[1], length-1)
			if n < 0 {
				n += length
			}
			if n < 0 {
				return value.ValueFromFloat64(-1), nil
			}
			if n < from {
				from = n
			}
		}
		for i := from; i >= 0; i-- {
			if arr.HasIndex(i) && strictEquals(arr.GetElementAt(i), search) {
				return native.IndexValue(i), nil
			}
		}
		return value.ValueFromFloat64(-1), nil
	case native.ARRAY_AT:
		index := 0
		if len(args) > 0 {
			index = vm.arrayInteger(args[0], 0)
		}
		if index < 0 {
			index += length
		}
		if index < 0 || index >= length {
			return value.UNDEFINED, nil
		}
		return arrayAt(arr, index), nil
	case native.ARRAY_POP:
		if length == 0 {
			return value.UNDEFINED, nil
		}
		result := arrayAt(arr, length-1)
		arr.DeleteIndex(length - 1)
		arr.SetLength(length - 1)
		return result, nil
	case native.ARRAY_UNSHIFT:
		oldLength := length
		arr.SetLength(oldLength + len(args))
		for i := oldLength - 1; i >= 0; i-- {
			if arr.HasIndex(i) {
				arr.SetElementAt(i+len(args), arr.GetElementAt(i))
			} else {
				arr.DeleteIndex(i + len(args))
			}
		}
		for i, arg := range args {
			arr.SetElementAt(i, arg)
		}
		arr.SetLength(oldLength + len(args))
		return native.IndexValue(oldLength + len(args)), nil
	case native.ARRAY_SLICE:
		start := 0
		end := length
		if len(args) > 0 {
			start = normalizeIndex(vm.arrayInteger(args[0], 0), length)
		}
		if len(args) > 1 && args[1] != value.UNDEFINED {
			end = normalizeIndex(vm.arrayInteger(args[1], length), length)
		}
		if start > end {
			start = end
		}
		resultLength := end - start
		result := native.NewArrayFrom(make([]value.Value, resultLength))
		for i := 0; i < resultLength; i++ {
			if arr.HasIndex(start + i) {
				result.SetElementAt(i, arr.GetElementAt(start+i))
			} else {
				result.DeleteIndex(i)
			}
		}
		result.SetLength(resultLength)
		return value.EncodeHandle(heap.Allocate(result)), nil
	case native.ARRAY_SPLICE:
		return vm.arraySplice(arr, args)
	case native.ARRAY_CONCAT:
		resultItems := make([]value.Value, 0, length)
		for i := 0; i < length; i++ {
			resultItems = append(resultItems, arr.GetElementAt(i))
		}
		for _, arg := range args {
			spread := false
			isArray := false
			if arg.IsObject() {
				obj, err := heap.GetObject(arg.GetHandle())
				if err == nil {
					if _, ok := obj.(*native.ObjArr); ok {
						spread = true
						isArray = true
					}
				}
				if native.SYMBOL_IS_CONCAT_SPREADABLE != 0 {
					if mark, err := vm.getMemberValue(arg, native.SYMBOL_IS_CONCAT_SPREADABLE); err == nil && mark != value.UNDEFINED {
						spread = truthyValue(mark)
					}
				}
			}
			if spread {
				other, _ := heap.GetObject(arg.GetHandle())
				if o, ok := other.(*native.ObjArr); ok {
					for i := 0; i < o.Length(); i++ {
						resultItems = append(resultItems, o.GetElementAt(i))
					}
				} else if hashable, ok := other.(object.Hashable); ok {
					lengthValue := hashable.GetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("length"))))
					n, _ := vm.toNumber(lengthValue)
					for i := 0; i < int(n); i++ {
						resultItems = append(resultItems, hashable.GetMember(native.IndexValue(i)))
					}
				}
			} else {
				resultItems = append(resultItems, arg)
			}
			_ = isArray
		}
		result := native.NewArrayFrom(resultItems)
		return value.EncodeHandle(heap.Allocate(result)), nil
	case native.ARRAY_FLAT:
		depth := 1
		if len(args) > 0 && args[0] != value.UNDEFINED {
			n, err := vm.toNumber(args[0])
			if err != nil {
				return value.UNDEFINED, err
			}
			if math.IsInf(n, 1) {
				depth = int(^uint(0) >> 1)
			} else if !math.IsNaN(n) {
				depth = int(math.Trunc(n))
			}
		}
		resultItems := []value.Value{}
		var flatten func([]value.Value, int)
		flatten = func(items []value.Value, level int) {
			for _, item := range items {
				if item == value.EMPTY_ARRAY_ITEM {
					continue
				}
				if level > 0 && item.IsObject() {
					obj, err := heap.GetObject(item.GetHandle())
					if err == nil {
						if child, ok := obj.(*native.ObjArr); ok {
							flatten(child.RawValues(), level-1)
							continue
						}
					}
				}
				resultItems = append(resultItems, item)
			}
		}
		flatten(arr.RawValues(), depth)
		return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(resultItems))), nil
	case native.ARRAY_TO_REVERSED:
		result := native.NewArrayFrom(make([]value.Value, length))
		for i := 0; i < length; i++ {
			result.SetElementAt(i, arrayAt(arr, length-1-i))
		}
		return value.EncodeHandle(heap.Allocate(result)), nil
	case native.ARRAY_TO_SORTED:
		copyArr := make([]value.Value, length)
		for i := 0; i < length; i++ {
			copyArr[i] = arrayAt(arr, i)
		}
		result := native.NewArrayFrom(copyArr)
		if err := vm.sortArray(result, args); err != nil {
			return value.UNDEFINED, err
		}
		return value.EncodeHandle(heap.Allocate(result)), nil
	case native.ARRAY_TO_SPLICED:
		copyArr := make([]value.Value, length)
		for i := 0; i < length; i++ {
			copyArr[i] = arrayAt(arr, i)
		}
		copy := native.NewArrayFrom(copyArr)
		if _, err := vm.arraySplice(copy, args); err != nil {
			return value.UNDEFINED, err
		}
		return value.EncodeHandle(heap.Allocate(copy)), nil
	case native.ARRAY_COPY_WITHIN:
		to := 0
		from := 0
		end := length
		if len(args) > 0 {
			to = normalizeIndex(vm.arrayInteger(args[0], 0), length)
		}
		if len(args) > 1 {
			from = normalizeIndex(vm.arrayInteger(args[1], 0), length)
		}
		if len(args) > 2 && args[2] != value.UNDEFINED {
			end = normalizeIndex(vm.arrayInteger(args[2], length), length)
		}
		count := min(end-from, length-to)
		if count > 0 {
			items := make([]value.Value, count)
			present := make([]bool, count)
			for i := 0; i < count; i++ {
				present[i] = arr.HasIndex(from + i)
				if present[i] {
					items[i] = arr.GetElementAt(from + i)
				}
			}
			for i := 0; i < count; i++ {
				if present[i] {
					arr.SetElementAt(to+i, items[i])
				} else {
					arr.DeleteIndex(to + i)
				}
			}
		}
		return this, nil
	case native.ARRAY_KEYS:
		return value.EncodeHandle(heap.Allocate(newArrayMethodIterator(arr, "keys"))), nil
	case native.ARRAY_VALUES:
		return value.EncodeHandle(heap.Allocate(newArrayMethodIterator(arr, "values"))), nil
	case native.ARRAY_ENTRIES:
		return value.EncodeHandle(heap.Allocate(newArrayMethodIterator(arr, "entries"))), nil
	case native.ARRAY_WITH:
		index := 0
		if len(args) > 0 {
			index = vm.arrayInteger(args[0], 0)
		}
		if len(args) < 2 {
			return value.UNDEFINED, fmt.Errorf("with requires a value")
		}
		if index < 0 {
			index += length
		}
		if index < 0 || index >= length {
			return value.UNDEFINED, fmt.Errorf("Invalid index")
		}
		resultItems := make([]value.Value, length)
		for i := 0; i < length; i++ {
			resultItems[i] = arrayAt(arr, i)
		}
		resultItems[index] = args[1]
		return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(resultItems))), nil
	}
	return value.UNDEFINED, nil
}

func (vm *VM) genericArrayMutation(kind native.ArrayMethodKind, this value.Value, hashable object.Hashable, args []value.Value) (value.Value, error) {
	lengthValue := hashable.GetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("length"))))
	n, err := vm.toNumber(lengthValue)
	if err != nil {
		return value.UNDEFINED, err
	}
	length := int(n)
	key := func(i int) value.Value {
		return native.IndexValue(i)
	}
	switch kind {
	case native.ARRAY_POP:
		if length == 0 {
			hashable.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("length"))), value.ValueFromFloat64(0))
			return value.UNDEFINED, nil
		}
		result := hashable.GetMember(key(length - 1))
		if deletable, ok := hashable.(object.Deletable); ok {
			deletable.Delete(key(length - 1))
		}
		hashable.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("length"))), value.ValueFromFloat64(float64(length-1)))
		return result, nil
	case native.ARRAY_UNSHIFT:
		for i := length - 1; i >= 0; i-- {
			hashable.SetMember(key(i+len(args)), hashable.GetMember(key(i)))
		}
		for i, arg := range args {
			hashable.SetMember(key(i), arg)
		}
		newLength := length + len(args)
		hashable.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("length"))), value.ValueFromFloat64(float64(newLength)))
		return native.IndexValue(newLength), nil
	}
	return value.UNDEFINED, nil
}

func (vm *VM) arraySplice(arr *native.ObjArr, args []value.Value) (value.Value, error) {
	length := arr.Length()
	start := 0
	if len(args) > 0 {
		start = normalizeIndex(vm.arrayInteger(args[0], 0), length)
	}
	deleteCount := length - start
	if len(args) == 0 {
		deleteCount = 0
	} else if len(args) == 1 {
		deleteCount = length - start
	} else {
		if args[1] == value.UNDEFINED {
			deleteCount = 0
		} else {
			deleteCount = vm.arrayInteger(args[1], 0)
			if deleteCount < 0 {
				deleteCount = 0
			}
			if deleteCount > length-start {
				deleteCount = length - start
			}
		}
	}
	items := []value.Value{}
	if len(args) > 2 {
		items = args[2:]
	}

	removed := native.NewArrayFrom(make([]value.Value, deleteCount))
	for i := 0; i < deleteCount; i++ {
		if arr.HasIndex(start + i) {
			removed.SetElementAt(i, arr.GetElementAt(start+i))
		} else {
			removed.DeleteIndex(i)
		}
	}
	removed.SetLength(deleteCount)

	oldItems := make([]value.Value, length)
	copy(oldItems, arr.RawValues())
	newItems := []value.Value{}
	newItems = append(newItems, oldItems[:start]...)
	newItems = append(newItems, items...)
	newItems = append(newItems, oldItems[start+deleteCount:]...)
	arr.SetLength(0)
	for i, item := range newItems {
		if item == value.EMPTY_ARRAY_ITEM {
			arr.SetLength(i + 1)
		} else {
			arr.SetElementAt(i, item)
		}
	}
	arr.SetLength(len(newItems))
	return value.EncodeHandle(heap.Allocate(removed)), nil
}

func (vm *VM) invokeCallback(fn value.Value, this value.Value, args []value.Value) (value.Value, error) {
	obj, err := heap.GetObject(fn.GetHandle())
	if err != nil {
		return value.UNDEFINED, err
	}
	callable, ok := obj.(object.Callable)
	if !ok {
		return value.UNDEFINED, fmt.Errorf("callback is not a function")
	}
	runner := NewVM(false, vm.output)
	for _, arg := range args {
		runner.push(arg)
	}
	f, c := runner.Call(callable, nil, this, len(args), false)
	return runner.run(f, c)
}

func (vm *VM) callArrayCallbackMethod(method *native.ArrayCallbackMethod, this value.Value, argCount int) (value.Value, error) {
	arr, err := vm.arrayReceiver(this)
	if err != nil {
		arr, err = vm.arrayLikeAsArray(this)
	}
	if err != nil {
		return value.UNDEFINED, err
	}
	args := vm.popN(argCount)
	if len(args) == 0 && method.Kind != native.ARRAY_SORT {
		return value.UNDEFINED, fmt.Errorf("callback is not a function")
	}
	callback := value.UNDEFINED
	if len(args) > 0 {
		callback = args[0]
	}
	thisArg := value.UNDEFINED
	if len(args) > 1 {
		thisArg = args[1]
	}
	length := arr.Length()
	switch method.Kind {
	case native.ARRAY_SOME:
		for i := 0; i < length; i++ {
			if !arr.HasIndex(i) {
				continue
			}
			result, err := vm.invokeCallback(callback, thisArg, []value.Value{arr.GetElementAt(i), native.IndexValue(i), this})
			if err != nil {
				return value.UNDEFINED, err
			}
			if truthyValue(result) {
				return value.TRUE, nil
			}
		}
		return value.FALSE, nil
	case native.ARRAY_EVERY:
		for i := 0; i < length; i++ {
			if !arr.HasIndex(i) {
				continue
			}
			result, err := vm.invokeCallback(callback, thisArg, []value.Value{arr.GetElementAt(i), native.IndexValue(i), this})
			if err != nil {
				return value.UNDEFINED, err
			}
			if !truthyValue(result) {
				return value.FALSE, nil
			}
		}
		return value.TRUE, nil
	case native.ARRAY_FIND, native.ARRAY_FIND_INDEX:
		for i := 0; i < length; i++ {
			result, err := vm.invokeCallback(callback, thisArg, []value.Value{arrayAt(arr, i), native.IndexValue(i), this})
			if err != nil {
				return value.UNDEFINED, err
			}
			if truthyValue(result) {
				if method.Kind == native.ARRAY_FIND {
					return arrayAt(arr, i), nil
				}
				return native.IndexValue(i), nil
			}
		}
		if method.Kind == native.ARRAY_FIND {
			return value.UNDEFINED, nil
		}
		return value.ValueFromFloat64(-1), nil
	case native.ARRAY_FIND_LAST_CB, native.ARRAY_FIND_LAST_INDEX_CB:
		for i := length - 1; i >= 0; i-- {
			result, err := vm.invokeCallback(callback, thisArg, []value.Value{arrayAt(arr, i), native.IndexValue(i), this})
			if err != nil {
				return value.UNDEFINED, err
			}
			if truthyValue(result) {
				if method.Kind == native.ARRAY_FIND_LAST_CB {
					return arrayAt(arr, i), nil
				}
				return native.IndexValue(i), nil
			}
		}
		if method.Kind == native.ARRAY_FIND_LAST_CB {
			return value.UNDEFINED, nil
		}
		return value.ValueFromFloat64(-1), nil
	case native.ARRAY_SORT:
		return this, vm.sortArray(arr, args)
	case native.ARRAY_REDUCE_RIGHT:
		hasInitial := len(args) > 1
		var accumulator value.Value
		start := length - 1
		if hasInitial {
			accumulator = args[1]
		} else {
			found := false
			for i := length - 1; i >= 0; i-- {
				if item, present := vm.arrayIterationElement(this, arr, i); present {
					accumulator = item
					start = i - 1
					found = true
					break
				}
			}
			if !found {
				return value.UNDEFINED, fmt.Errorf("Reduce of empty array with no initial value")
			}
		}
		for i := start; i >= 0; i-- {
			item, present := vm.arrayIterationElement(this, arr, i)
			if !present {
				continue
			}
			accumulator, err = vm.invokeCallback(callback, value.UNDEFINED, []value.Value{accumulator, item, native.IndexValue(i), this})
			if err != nil {
				return value.UNDEFINED, err
			}
		}
		return accumulator, nil
	case native.ARRAY_FLAT_MAP:
		result := []value.Value{}
		for i := 0; i < length; i++ {
			if !arr.HasIndex(i) {
				continue
			}
			mapped, err := vm.invokeCallback(callback, thisArg, []value.Value{arr.GetElementAt(i), native.IndexValue(i), this})
			if err != nil {
				return value.UNDEFINED, err
			}
			if mapped.IsObject() {
				if obj, err := heap.GetObject(mapped.GetHandle()); err == nil {
					if child, ok := obj.(*native.ObjArr); ok {
						for j := 0; j < child.Length(); j++ {
							if child.HasIndex(j) {
								result = append(result, child.GetElementAt(j))
							}
						}
						continue
					}
				}
			}
			result = append(result, mapped)
		}
		return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(result))), nil
	}
	return value.UNDEFINED, nil
}

func (vm *VM) sortArray(arr *native.ObjArr, args []value.Value) error {
	length := arr.Length()
	values := []value.Value{}
	undefinedCount := 0
	holeCount := 0
	for i := 0; i < length; i++ {
		if !arr.HasIndex(i) {
			holeCount++
			continue
		}
		v := arr.GetElementAt(i)
		if v == value.UNDEFINED {
			undefinedCount++
			continue
		}
		values = append(values, v)
	}
	var comparator value.Value
	hasComparator := len(args) > 0 && args[0] != value.UNDEFINED
	if hasComparator {
		comparator = args[0]
	}
	var sortErr error
	sort.SliceStable(values, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		if hasComparator {
			result, err := vm.invokeCallback(comparator, value.UNDEFINED, []value.Value{values[i], values[j]})
			if err != nil {
				sortErr = err
				return false
			}
			n, _ := vm.toNumber(result)
			return n < 0
		}
		si, _ := vm.toStringValue(values[i])
		sj, _ := vm.toStringValue(values[j])
		a, _ := vm.valueAsString(si)
		b, _ := vm.valueAsString(sj)
		return a < b
	})
	if sortErr != nil {
		return sortErr
	}
	for i := 0; i < length; i++ {
		if i < len(values) {
			arr.SetElementAt(i, values[i])
		} else if i < len(values)+undefinedCount {
			arr.SetElementAt(i, value.UNDEFINED)
		} else {
			arr.DeleteIndex(i)
		}
	}
	arr.SetLength(length)
	return nil
}

func (vm *VM) arrayLikeAsArray(this value.Value) (*native.ObjArr, error) {
	if !this.IsObject() {
		return nil, fmt.Errorf("array method called on non-object")
	}
	lengthValue, err := vm.getMemberValue(this, value.EncodeHandle(heap.Allocate(native.NewLightString("length"))))
	if err != nil {
		return nil, err
	}
	n, err := vm.toNumber(lengthValue)
	if err != nil {
		return nil, err
	}
	length := int(n)
	arr := native.NewArrayFrom(make([]value.Value, length))
	for i := 0; i < length; i++ {
		item, err := vm.getMemberValue(this, native.IndexValue(i))
		if err != nil {
			return nil, err
		}
		arr.SetElementAt(i, item)
	}
	arr.SetLength(length)
	return arr, nil
}

func (vm *VM) arrayIterationElement(this value.Value, arr *native.ObjArr, i int) (value.Value, bool) {
	if arr.HasIndex(i) {
		return arr.GetElementAt(i), true
	}
	v, err := vm.getMemberValue(this, native.IndexValue(i))
	if err == nil && v != value.UNDEFINED {
		return v, true
	}
	return value.UNDEFINED, false
}

func (vm *VM) callArrayIterationMethod(kind string, this value.Value, argCount int) (value.Value, error) {
	arr, err := vm.arrayReceiver(this)
	if err != nil {
		arr, err = vm.arrayLikeAsArray(this)
	}
	if err != nil {
		return value.UNDEFINED, err
	}
	args := vm.popN(argCount)
	if len(args) == 0 {
		return value.UNDEFINED, fmt.Errorf("callback was not a function")
	}
	callback := args[0]
	thisArg := value.UNDEFINED
	if len(args) > 1 {
		thisArg = args[1]
	}
	length := arr.Length()
	switch kind {
	case "forEach":
		for i := 0; i < length; i++ {
			item, present := vm.arrayIterationElement(this, arr, i)
			if !present {
				continue
			}
			if _, err := vm.invokeCallback(callback, thisArg, []value.Value{item, native.IndexValue(i), this}); err != nil {
				return value.UNDEFINED, err
			}
		}
		return value.UNDEFINED, nil
	case "map":
		result := native.NewArrayFrom(make([]value.Value, length))
		resultValue := value.EncodeHandle(heap.Allocate(result))
		speciesResult := value.UNDEFINED
		if native.SYMBOL_SPECIES != 0 {
			if ctor, err := vm.getMemberValue(this, value.EncodeHandle(heap.Allocate(native.NewLightString("constructor")))); err == nil && ctor.IsObject() {
				if species, err := vm.getMemberValue(ctor, native.SYMBOL_SPECIES); err == nil && species != value.UNDEFINED && species != value.NULL {
					if speciesObj, err := heap.GetObject(species.GetHandle()); err == nil {
						if speciesCtor, ok := speciesObj.(object.Callable); ok {
							resultValue, err = vm.construct(speciesCtor, []value.Value{value.ValueFromFloat64(float64(length))}, species)
							if err != nil {
								return value.UNDEFINED, err
							}
							speciesResult = resultValue
						}
					}
				}
			}
		}
		if speciesResult != value.UNDEFINED {
			for i := 0; i < length; i++ {
				item, present := vm.arrayIterationElement(this, arr, i)
				if !present {
					continue
				}
				item, err := vm.invokeCallback(callback, thisArg, []value.Value{item, native.IndexValue(i), this})
				if err != nil {
					return value.UNDEFINED, err
				}
				if err := vm.setMemberValue(resultValue, native.IndexValue(i), item); err != nil {
					return value.UNDEFINED, err
				}
			}
			vm.setMemberValue(resultValue, value.EncodeHandle(heap.Allocate(native.NewLightString("length"))), value.ValueFromFloat64(float64(length)))
			return resultValue, nil
		}
		for i := 0; i < length; i++ {
			item, present := vm.arrayIterationElement(this, arr, i)
			if !present {
				result.DeleteIndex(i)
				continue
			}
			item, err := vm.invokeCallback(callback, thisArg, []value.Value{item, native.IndexValue(i), this})
			if err != nil {
				return value.UNDEFINED, err
			}
			result.SetElementAt(i, item)
		}
		result.SetLength(length)
		return value.EncodeHandle(heap.Allocate(result)), nil
	case "filter":
		items := []value.Value{}
		for i := 0; i < length; i++ {
			item, present := vm.arrayIterationElement(this, arr, i)
			if !present {
				continue
			}
			keep, err := vm.invokeCallback(callback, thisArg, []value.Value{item, native.IndexValue(i), this})
			if err != nil {
				return value.UNDEFINED, err
			}
			if truthyValue(keep) {
				items = append(items, item)
			}
		}
		return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(items))), nil
	case "reduce":
		hasInitial := len(args) > 1
		var accumulator value.Value
		start := 0
		if hasInitial {
			accumulator = args[1]
		} else {
			found := false
			for i := 0; i < length; i++ {
				if item, present := vm.arrayIterationElement(this, arr, i); present {
					accumulator = item
					start = i + 1
					found = true
					break
				}
			}
			if !found {
				return value.UNDEFINED, fmt.Errorf("Reduce of empty array with no initial value")
			}
		}
		for i := start; i < length; i++ {
			item, present := vm.arrayIterationElement(this, arr, i)
			if !present {
				continue
			}
			accumulator, err = vm.invokeCallback(callback, value.UNDEFINED, []value.Value{accumulator, item, native.IndexValue(i), this})
			if err != nil {
				return value.UNDEFINED, err
			}
		}
		return accumulator, nil
	}
	return value.UNDEFINED, nil
}

func (vm *VM) toArrayLength(v value.Value) int {
	n, err := vm.toNumber(v)
	if err != nil || math.IsNaN(n) || n <= 0 {
		return 0
	}
	if math.IsInf(n, 1) {
		return int(^uint(0) >> 1)
	}
	maxInt := int(^uint(0) >> 1)
	if n >= float64(maxInt) {
		return maxInt
	}
	return int(math.Floor(n))
}

func newArrayMethodIterator(arr *native.ObjArr, mode string) *object.Iterator {
	return object.NewLiveIterator(func() []value.Value {
		values := make([]value.Value, 0, arr.Length())
		for i := 0; i < arr.Length(); i++ {
			switch mode {
			case "keys":
				values = append(values, native.IndexValue(i))
			case "values":
				values = append(values, arrayAt(arr, i))
			default:
				values = append(values, value.EncodeHandle(heap.Allocate(native.NewArrayFrom([]value.Value{native.IndexValue(i), arrayAt(arr, i)}))))
			}
		}
		return values
	})
}

func (vm *VM) callArrayStaticMethod(method *native.ArrayStaticMethod, argCount int) (value.Value, error) {
	args := vm.popN(argCount)
	switch method.Kind {
	case native.ARRAY_STATIC_IS_ARRAY:
		if len(args) > 0 {
			if args[0] == native.PROTOTYPE_ARRAY {
				return value.TRUE, nil
			}
			if args[0].IsObject() {
				obj, err := heap.GetObject(args[0].GetHandle())
				if err == nil {
					if proxy, ok := obj.(*native.ObjProxy); ok {
						if proxy.Revoked {
							return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Cannot perform operation on a revoked Proxy"}
						}
						obj, err = heap.GetObject(proxy.Target.GetHandle())
						if err != nil {
							return value.FALSE, nil
						}
					}
					if _, ok := obj.(*native.ObjArr); ok {
						return value.TRUE, nil
					}
				}
			}
		}
		return value.FALSE, nil
	case native.ARRAY_STATIC_OF:
		return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(args))), nil
	case native.ARRAY_STATIC_FROM:
		if len(args) == 0 {
			return value.UNDEFINED, fmt.Errorf("Array.from requires an array-like object")
		}
		source := args[0]
		var mapFn value.Value
		hasMap := len(args) > 1 && args[1] != value.UNDEFINED
		if hasMap {
			mapFn = args[1]
		}
		thisArg := value.UNDEFINED
		if len(args) > 2 {
			thisArg = args[2]
		}
		if s, ok := vm.valueAsString(source); ok {
			runes := []rune(strings.ToValidUTF8(s, "\uFFFD"))
			items := make([]value.Value, len(runes))
			for i, r := range runes {
				var item value.Value = value.EncodeHandle(heap.Allocate(native.NewLightString(string(r))))
				if hasMap {
					var err error
					item, err = vm.invokeCallback(mapFn, thisArg, []value.Value{item, native.IndexValue(i)})
					if err != nil {
						return value.UNDEFINED, err
					}
				}
				items[i] = item
			}
			return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(items))), nil
		}
		if !source.IsObject() {
			return value.UNDEFINED, fmt.Errorf("Array.from requires an array-like object")
		}
		if native.SYMBOL_ITERATOR != 0 {
			method, err := vm.getMemberValue(source, native.SYMBOL_ITERATOR)
			if err == nil && method != value.UNDEFINED && method != value.NULL {
				iterator, err := vm.acquireIterator(source, 0)
				if err != nil {
					return value.UNDEFINED, err
				}
				items := []value.Value{}
				for {
					done, err := vm.iteratorNext(iterator)
					if err != nil {
						return value.UNDEFINED, err
					}
					if done {
						break
					}
					item, err := vm.iteratorCurrent(iterator)
					if err != nil {
						return value.UNDEFINED, err
					}
					if hasMap {
						item, err = vm.invokeCallback(mapFn, thisArg, []value.Value{item, native.IndexValue(len(items))})
						if err != nil {
							vm.closeIterator(iterator)
							return value.UNDEFINED, err
						}
					}
					items = append(items, item)
				}
				return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(items))), nil
			}
		}
		lengthValue, err := vm.getMemberValue(source, value.EncodeHandle(heap.Allocate(native.NewLightString("length"))))
		if err != nil {
			return value.UNDEFINED, err
		}
		length := vm.toArrayLength(lengthValue)
		items := make([]value.Value, length)
		for i := 0; i < length; i++ {
			item, err := vm.getMemberValue(source, native.IndexValue(i))
			if err != nil {
				return value.UNDEFINED, err
			}
			if hasMap {
				item, err = vm.invokeCallback(mapFn, thisArg, []value.Value{item, native.IndexValue(i)})
				if err != nil {
					return value.UNDEFINED, err
				}
			}
			items[i] = item
		}
		return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(items))), nil
	}
	return value.UNDEFINED, nil
}
