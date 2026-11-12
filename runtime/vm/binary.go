package vm

import (
	"math"
	"math/big"
	"sort"

	"go_js/heap"
	"go_js/native"
	"go_js/value"
)

func binaryBigIntFromUint64(bits uint64, signed bool) value.Value {
	n := new(big.Int).SetUint64(bits)
	if signed && bits&(1<<63) != 0 {
		n.Sub(n, new(big.Int).Lsh(big.NewInt(1), 64))
	}
	return value.EncodeHandle(heap.Allocate(native.NewBigInt(n)))
}

func binaryBigIntToUint64(v value.Value) uint64 {
	if v.IsObject() {
		if obj, err := heap.GetObject(v.GetHandle()); err == nil {
			if b, ok := obj.(*native.ObjBigInt); ok {
				modulus := new(big.Int).Lsh(big.NewInt(1), 64)
				return new(big.Int).Mod(b.Value, modulus).Uint64()
			}
		}
	}
	return uint64(int64(v.AsNumber()))
}

func (vm *VM) arrayBufferFromValue(v value.Value) (*native.ObjArrayBuffer, bool) {
	if !v.IsObject() {
		return nil, false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return nil, false
	}
	buffer, ok := obj.(*native.ObjArrayBuffer)
	return buffer, ok
}

func (vm *VM) typedArrayFromValue(v value.Value) (*native.ObjTypedArray, bool) {
	if !v.IsObject() {
		return nil, false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return nil, false
	}
	array, ok := obj.(*native.ObjTypedArray)
	return array, ok
}

func (vm *VM) dataViewFromValue(v value.Value) (*native.ObjDataView, bool) {
	if !v.IsObject() {
		return nil, false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return nil, false
	}
	view, ok := obj.(*native.ObjDataView)
	return view, ok
}

func (vm *VM) constructArrayBuffer(args []value.Value) (value.Value, error) {
	length := 0
	if len(args) > 0 {
		n, err := vm.toNumber(args[0])
		if err != nil || math.IsNaN(n) || n < 0 || math.Trunc(n) != n || n > 2147483647 {
			return value.UNDEFINED, &JSException{Name: "RangeError", Message: "Invalid array buffer length"}
		}
		length = int(n)
	}
	return value.EncodeHandle(heap.Allocate(native.NewArrayBuffer(length))), nil
}

func (vm *VM) constructTypedArray(ctor *native.TypedArrayConstructor, args []value.Value) (value.Value, error) {
	elementSize := 1
	switch ctor.Kind {
	case native.TYPED_INT16, native.TYPED_UINT16:
		elementSize = 2
	case native.TYPED_INT32, native.TYPED_UINT32, native.TYPED_FLOAT32:
		elementSize = 4
	case native.TYPED_FLOAT64, native.TYPED_BIGINT64, native.TYPED_BIGUINT64:
		elementSize = 8
	}
	if len(args) == 0 {
		buffer := native.NewArrayBuffer(0)
		return value.EncodeHandle(heap.Allocate(native.NewTypedArray(buffer, value.EncodeHandle(heap.Allocate(buffer)), 0, 0, ctor.Kind))), nil
	}
	first := args[0]
	if buffer, ok := vm.arrayBufferFromValue(first); ok {
		offset := 0
		if len(args) > 1 && args[1] != value.UNDEFINED {
			n, _ := vm.toNumber(args[1])
			offset = int(n)
		}
		if offset < 0 || offset%elementSize != 0 || offset > len(buffer.Data) {
			return value.UNDEFINED, &JSException{Name: "RangeError", Message: "Invalid typed array offset"}
		}
		length := (len(buffer.Data) - offset) / elementSize
		if len(args) > 2 && args[2] != value.UNDEFINED {
			n, _ := vm.toNumber(args[2])
			length = int(n)
		}
		if length < 0 || offset+length*elementSize > len(buffer.Data) {
			return value.UNDEFINED, &JSException{Name: "RangeError", Message: "Invalid typed array length"}
		}
		return value.EncodeHandle(heap.Allocate(native.NewTypedArray(buffer, first, offset, length, ctor.Kind))), nil
	}
	if first.IsNumber() {
		n, _ := vm.toNumber(first)
		if n < 0 || math.IsNaN(n) || math.Trunc(n) != n {
			return value.UNDEFINED, &JSException{Name: "RangeError", Message: "Invalid typed array length"}
		}
		length := int(n)
		buffer := native.NewArrayBuffer(length * elementSize)
		bufferValue := value.EncodeHandle(heap.Allocate(buffer))
		return value.EncodeHandle(heap.Allocate(native.NewTypedArray(buffer, bufferValue, 0, length, ctor.Kind))), nil
	}
	items, err := vm.collectArrayLike(first)
	if err != nil {
		return value.UNDEFINED, err
	}
	buffer := native.NewArrayBuffer(len(items) * elementSize)
	bufferValue := value.EncodeHandle(heap.Allocate(buffer))
	array := native.NewTypedArray(buffer, bufferValue, 0, len(items), ctor.Kind)
	for i, item := range items {
		if ctor.Kind == native.TYPED_BIGINT64 || ctor.Kind == native.TYPED_BIGUINT64 {
			if _, ok := vm.bigIntObject(item); !ok {
				return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Cannot convert to a BigInt"}
			}
		}
		array.Write(i, item)
	}
	return value.EncodeHandle(heap.Allocate(array)), nil
}

func (vm *VM) collectArrayLike(source value.Value) ([]value.Value, error) {
	if source.IsObject() {
		if _, ok := vm.arrayBufferFromValue(source); !ok {
			items, err := vm.collectPromiseInput(source)
			if err == nil {
				return items, nil
			}
		}
		if obj, err := heap.GetObject(source.GetHandle()); err == nil {
			if arr, ok := obj.(*native.ObjArr); ok {
				items := make([]value.Value, arr.Length())
				for i := range items {
					items[i] = arr.GetElementAt(i)
				}
				return items, nil
			}
			lengthValue, err := vm.getMemberValue(source, value.EncodeHandle(heap.Allocate(native.NewLightString("length"))))
			if err == nil {
				n, _ := vm.toNumber(lengthValue)
				length := int(n)
				items := make([]value.Value, length)
				for i := 0; i < length; i++ {
					items[i], _ = vm.getMemberValue(source, native.IndexValue(i))
				}
				return items, nil
			}
		}
	}
	return nil, &JSException{Name: "TypeError", Message: "Value is not iterable"}
}

func (vm *VM) constructDataView(args []value.Value) (value.Value, error) {
	if len(args) == 0 {
		return value.UNDEFINED, &JSException{Name: "TypeError", Message: "DataView requires an ArrayBuffer"}
	}
	buffer, ok := vm.arrayBufferFromValue(args[0])
	if !ok {
		return value.UNDEFINED, &JSException{Name: "TypeError", Message: "DataView requires an ArrayBuffer"}
	}
	offset := 0
	if len(args) > 1 && args[1] != value.UNDEFINED {
		n, _ := vm.toNumber(args[1])
		offset = int(n)
	}
	length := len(buffer.Data) - offset
	if len(args) > 2 && args[2] != value.UNDEFINED {
		n, _ := vm.toNumber(args[2])
		length = int(n)
	}
	if offset < 0 || length < 0 || offset+length > len(buffer.Data) {
		return value.UNDEFINED, &JSException{Name: "RangeError", Message: "Invalid DataView bounds"}
	}
	return value.EncodeHandle(heap.Allocate(native.NewDataView(buffer, args[0], offset, length))), nil
}

func (vm *VM) typedArrayJoin(array *native.ObjTypedArray, separator string) string {
	result := ""
	for i := 0; i < array.Length; i++ {
		if i > 0 {
			result += separator
		}
		result += native.String(array.Read(i))
	}
	return result
}

func (vm *VM) typedArraySet(array *native.ObjTypedArray, source value.Value, offset int) error {
	items, err := vm.collectArrayLike(source)
	if err != nil {
		return err
	}
	if offset < 0 || offset+len(items) > array.Length {
		return &JSException{Name: "RangeError", Message: "Offset is out of bounds"}
	}
	values := make([]value.Value, len(items))
	copy(values, items)
	for i, item := range values {
		array.Write(offset+i, item)
	}
	return nil
}

func (vm *VM) typedArraySlice(array *native.ObjTypedArray, start, end int) *native.ObjTypedArray {
	if start < 0 {
		start = array.Length + start
	}
	if end < 0 {
		end = array.Length + end
	}
	start = max(0, min(start, array.Length))
	end = max(start, min(end, array.Length))
	length := end - start
	elementSize := array.ElementSize()
	buffer := native.NewArrayBuffer(length * elementSize)
	result := native.NewTypedArray(buffer, value.EncodeHandle(heap.Allocate(buffer)), 0, length, array.Kind)
	for i := 0; i < length; i++ {
		result.Write(i, array.Read(start+i))
	}
	return result
}

func (vm *VM) typedArraySort(array *native.ObjTypedArray) {
	values := make([]value.Value, array.Length)
	for i := range values {
		values[i] = array.Read(i)
	}
	sort.SliceStable(values, func(i, j int) bool {
		return vm.compareValues(values[i], values[j]) < 0
	})
	for i, v := range values {
		array.Write(i, v)
	}
}

func (vm *VM) compareValues(a, b value.Value) int {
	result, err := vm.compare(a, b)
	if err != nil {
		return 0
	}
	return result
}

func (vm *VM) typedArrayMap(array *native.ObjTypedArray, callback, thisArg value.Value) (value.Value, error) {
	elementSize := array.ElementSize()
	buffer := native.NewArrayBuffer(array.Length * elementSize)
	result := native.NewTypedArray(buffer, value.EncodeHandle(heap.Allocate(buffer)), 0, array.Length, array.Kind)
	for i := 0; i < array.Length; i++ {
		mapped, err := vm.invokeCallback(callback, thisArg, []value.Value{array.Read(i), native.IndexValue(i), value.EncodeHandle(heap.Allocate(array))})
		if err != nil {
			return value.UNDEFINED, err
		}
		result.Write(i, mapped)
	}
	return value.EncodeHandle(heap.Allocate(result)), nil
}

func (vm *VM) getTypedArrayMember(array *native.ObjTypedArray, key value.Value) (value.Value, bool) {
	if index, ok := vm.arrayIndexFromValue(key); ok {
		return array.Read(index), true
	}
	keyString, ok := vm.valueAsString(key)
	if !ok {
		return value.UNDEFINED, false
	}
	switch keyString {
	case "buffer":
		return array.BufferValue, true
	case "byteOffset":
		return value.ValueFromFloat64(float64(array.Offset)), true
	case "byteLength":
		return value.ValueFromFloat64(float64(array.Length * array.ElementSize())), true
	case "length":
		return value.ValueFromFloat64(float64(array.Length)), true
	case "join":
		return value.EncodeHandle(heap.Allocate(native.NewTypedArrayJoin())), true
	case "set":
		return value.EncodeHandle(heap.Allocate(native.NewTypedArraySet())), true
	case "subarray":
		return value.EncodeHandle(heap.Allocate(native.NewTypedArraySubarray())), true
	case "slice":
		return value.EncodeHandle(heap.Allocate(native.NewTypedArraySlice())), true
	case "sort":
		return value.EncodeHandle(heap.Allocate(native.NewTypedArraySort())), true
	case "map":
		return value.EncodeHandle(heap.Allocate(native.NewTypedArrayMap())), true
	}
	return value.UNDEFINED, false
}

func (vm *VM) getDataViewMember(view *native.ObjDataView, key value.Value) (value.Value, bool) {
	keyString, ok := vm.valueAsString(key)
	if !ok {
		return value.UNDEFINED, false
	}
	switch keyString {
	case "buffer":
		return view.BufferValue, true
	case "byteOffset":
		return value.ValueFromFloat64(float64(view.Offset)), true
	case "byteLength":
		return value.ValueFromFloat64(float64(view.Length)), true
	case "getUint8", "getUint16", "getUint32", "getInt8", "getInt16", "getInt32", "getFloat32", "getFloat64", "getBigInt64", "getBigUint64", "setUint8", "setUint16", "setUint32", "setInt8", "setInt16", "setInt32", "setFloat32", "setFloat64", "setBigInt64", "setBigUint64":
		return value.EncodeHandle(heap.Allocate(native.NewDataViewMethod(keyString))), true
	}
	return value.UNDEFINED, false
}

func (vm *VM) callDataViewMethod(view *native.ObjDataView, method *native.DataViewMethod, args []value.Value) (value.Value, error) {
	return vm.dataViewOperation(view, method.Kind, args)
}

func (vm *VM) dataViewOperation(view *native.ObjDataView, kind string, args []value.Value) (value.Value, error) {
	if len(args) == 0 {
		return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Missing offset"}
	}
	offsetNumber, _ := vm.toNumber(args[0])
	offset := int(offsetNumber)
	read := kind[0] == 'g'
	size := 1
	switch {
	case containsSuffix(kind, "Uint16"), containsSuffix(kind, "Int16"):
		size = 2
	case containsSuffix(kind, "Uint32"), containsSuffix(kind, "Int32"), containsSuffix(kind, "Float32"):
		size = 4
	case containsSuffix(kind, "Float64"), containsSuffix(kind, "BigInt64"), containsSuffix(kind, "BigUint64"):
		size = 8
	}
	if offset < 0 || offset+size > view.Length {
		return value.UNDEFINED, &JSException{Name: "RangeError", Message: "Offset is outside the bounds of the DataView"}
	}
	littleEndian := false
	argIndex := 1
	if !read {
		argIndex = 2
	}
	if len(args) > argIndex && args[argIndex] != value.UNDEFINED {
		littleEndian = truthyValue(args[argIndex])
	}
	data := view.Buffer.Data[view.Offset+offset:]
	readUint := func(n int) uint64 {
		bits := uint64(0)
		if littleEndian {
			for i := 0; i < n; i++ {
				bits |= uint64(data[i]) << (8 * i)
			}
		} else {
			for i := 0; i < n; i++ {
				bits = bits<<8 | uint64(data[i])
			}
		}
		return bits
	}
	writeUint := func(n int, bits uint64) {
		if littleEndian {
			for i := 0; i < n; i++ {
				data[i] = byte(bits >> (8 * i))
			}
		} else {
			for i := 0; i < n; i++ {
				data[n-1-i] = byte(bits >> (8 * i))
			}
		}
	}
	if read {
		switch {
		case containsSuffix(kind, "Uint8"):
			return value.ValueFromFloat64(float64(data[0])), nil
		case containsSuffix(kind, "Int8"):
			return value.ValueFromFloat64(float64(int8(data[0]))), nil
		case containsSuffix(kind, "Uint16"):
			return value.ValueFromFloat64(float64(uint16(readUint(2)))), nil
		case containsSuffix(kind, "Int16"):
			return value.ValueFromFloat64(float64(int16(readUint(2)))), nil
		case containsSuffix(kind, "Uint32"):
			return value.ValueFromFloat64(float64(uint32(readUint(4)))), nil
		case containsSuffix(kind, "Int32"):
			return value.ValueFromFloat64(float64(int32(readUint(4)))), nil
		case containsSuffix(kind, "Float32"):
			return value.ValueFromFloat64(float64(math.Float32frombits(uint32(readUint(4))))), nil
		case containsSuffix(kind, "Float64"):
			return value.ValueFromFloat64(math.Float64frombits(readUint(8))), nil
		case containsSuffix(kind, "BigInt64"):
			return binaryBigIntFromUint64(readUint(8), true), nil
		case containsSuffix(kind, "BigUint64"):
			return binaryBigIntFromUint64(readUint(8), false), nil
		}
	}
	if len(args) < 2 {
		return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Missing value"}
	}
	valueArg := args[1]
	switch {
	case containsSuffix(kind, "Uint8"):
		n, _ := vm.toNumber(valueArg)
		data[0] = byte(uint8(int64(n)))
	case containsSuffix(kind, "Int8"):
		n, _ := vm.toNumber(valueArg)
		data[0] = byte(uint8(int8(int64(n))))
	case containsSuffix(kind, "Uint16"):
		n, _ := vm.toNumber(valueArg)
		writeUint(2, uint64(uint16(int64(n))))
	case containsSuffix(kind, "Int16"):
		n, _ := vm.toNumber(valueArg)
		writeUint(2, uint64(uint16(int16(int64(n)))))
	case containsSuffix(kind, "Uint32"):
		n, _ := vm.toNumber(valueArg)
		writeUint(4, uint64(uint32(int64(n))))
	case containsSuffix(kind, "Int32"):
		n, _ := vm.toNumber(valueArg)
		writeUint(4, uint64(uint32(int32(int64(n)))))
	case containsSuffix(kind, "Float32"):
		n, _ := vm.toNumber(valueArg)
		writeUint(4, uint64(math.Float32bits(float32(n))))
	case containsSuffix(kind, "Float64"):
		n, _ := vm.toNumber(valueArg)
		writeUint(8, math.Float64bits(n))
	case containsSuffix(kind, "BigInt64"), containsSuffix(kind, "BigUint64"):
		if _, ok := vm.bigIntObject(valueArg); !ok {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Cannot convert to a BigInt"}
		}
		writeUint(8, binaryBigIntToUint64(valueArg))
	}
	return value.UNDEFINED, nil
}

func containsSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func (vm *VM) callTypedArrayStatic(method *native.TypedArrayStaticMethod, args []value.Value) (value.Value, error) {
	items := []value.Value{}
	if method.Kind == native.TYPED_ARRAY_STATIC_OF {
		items = args
	} else {
		if len(args) == 0 {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "TypedArray.from requires an array-like object"}
		}
		collected, err := vm.collectArrayLike(args[0])
		if err != nil {
			return value.UNDEFINED, err
		}
		items = collected
		if len(args) > 1 && args[1] != value.UNDEFINED {
			for i, item := range items {
				mapped, err := vm.invokeCallback(args[1], value.UNDEFINED, []value.Value{item, native.IndexValue(i)})
				if err != nil {
					return value.UNDEFINED, err
				}
				items[i] = mapped
			}
		}
	}
	arrayValue := value.EncodeHandle(heap.Allocate(native.NewArrayFrom(items)))
	return vm.constructTypedArray(method.Constructor, []value.Value{arrayValue, value.UNDEFINED})
}
