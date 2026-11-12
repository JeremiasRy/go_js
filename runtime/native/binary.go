package native

import (
	"math"
	"math/big"

	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

func bigIntFromUint64(bits uint64, signed bool) value.Value {
	n := new(big.Int).SetUint64(bits)
	if signed && bits&(1<<63) != 0 {
		n.Sub(n, new(big.Int).Lsh(big.NewInt(1), 64))
	}
	return value.EncodeHandle(heap.Allocate(NewBigInt(n)))
}

func bigIntToUint64(v value.Value) uint64 {
	if v.IsObject() {
		if obj, err := heap.GetObject(v.GetHandle()); err == nil {
			if b, ok := obj.(*ObjBigInt); ok {
				modulus := new(big.Int).Lsh(big.NewInt(1), 64)
				return new(big.Int).Mod(b.Value, modulus).Uint64()
			}
		}
	}
	return uint64(int64(v.AsNumber()))
}

type ObjArrayBuffer struct {
	ObjObject
	Data []byte
}

func NewArrayBuffer(length int) *ObjArrayBuffer {
	buffer := &ObjArrayBuffer{Data: make([]byte, length)}
	buffer.Members = map[string]ObjectValueEntry{}
	buffer.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return buffer
}

func (*ObjArrayBuffer) Type() object.ObjType {
	return object.OBJ_ARRAY_BUFFER
}

func (b *ObjArrayBuffer) String() string {
	return "[object ArrayBuffer]"
}

type ArrayBufferConstructor struct {
	ObjObject
}

func NewArrayBufferConstructor() *ArrayBufferConstructor {
	ctor := &ArrayBufferConstructor{}
	ctor.Members = map[string]ObjectValueEntry{}
	ctor.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	ctor.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("isView"))), value.EncodeHandle(heap.Allocate(NewArrayBufferIsView())))
	return ctor
}

func (*ArrayBufferConstructor) Type() object.ObjType {
	return object.OBJ_NATIVE_FN
}

func (*ArrayBufferConstructor) String() string {
	return "function ArrayBuffer"
}

type ArrayBufferIsView struct {
	ObjNativeFn
}

func NewArrayBufferIsView() *ArrayBufferIsView {
	fn := &ArrayBufferIsView{}
	fn.name = "isView"
	return fn
}

type ArrayBufferSlice struct {
	ObjNativeFn
}

func NewArrayBufferSlice() *ArrayBufferSlice {
	fn := &ArrayBufferSlice{}
	fn.name = "slice"
	return fn
}

type ArrayBufferTransfer struct {
	ObjNativeFn
}

func NewArrayBufferTransfer() *ArrayBufferTransfer {
	fn := &ArrayBufferTransfer{}
	fn.name = "transfer"
	return fn
}

const (
	TYPED_UINT8 uint8 = iota
	TYPED_INT8
	TYPED_UINT8_CLAMPED
	TYPED_INT16
	TYPED_FLOAT64
	TYPED_BIGINT64
	TYPED_BIGUINT64
	TYPED_UINT16
	TYPED_INT32
	TYPED_UINT32
	TYPED_FLOAT32
	TYPED_FLOAT16
)

func float32ToFloat16Bits(f float32) uint16 {
	bits := math.Float32bits(f)
	sign := uint16(bits>>16) & 0x8000
	exponent := int((bits >> 23) & 0xff)
	mantissa := bits & 0x7fffff
	if exponent == 0xff {
		if mantissa != 0 {
			return sign | 0x7e00
		}
		return sign | 0x7c00
	}
	halfExponent := exponent - 127 + 15
	if halfExponent >= 31 {
		return sign | 0x7c00
	}
	if halfExponent <= 0 {
		if halfExponent < -10 {
			return sign
		}
		mantissa |= 0x800000
		shift := uint(14 - halfExponent)
		rounded := mantissa >> shift
		remainder := mantissa & ((1 << shift) - 1)
		half := uint32(1) << (shift - 1)
		if remainder > half || (remainder == half && rounded&1 != 0) {
			rounded++
		}
		return sign | uint16(rounded)
	}
	rounded := mantissa >> 13
	remainder := mantissa & 0x1fff
	if remainder > 0x1000 || (remainder == 0x1000 && rounded&1 != 0) {
		rounded++
		if rounded == 0x400 {
			rounded = 0
			halfExponent++
			if halfExponent >= 31 {
				return sign | 0x7c00
			}
		}
	}
	return sign | uint16(halfExponent)<<10 | uint16(rounded)
}

func float16BitsToFloat32(bits uint16) float32 {
	sign := uint32(bits&0x8000) << 16
	exponent := uint32(bits>>10) & 0x1f
	mantissa := uint32(bits & 0x3ff)
	if exponent == 0 {
		if mantissa == 0 {
			return math.Float32frombits(sign)
		}
		shift := uint32(0)
		for mantissa&0x400 == 0 {
			mantissa <<= 1
			shift++
		}
		mantissa &= 0x3ff
		exponent32 := uint32(127 - 15 - int(shift) + 1)
		return math.Float32frombits(sign | exponent32<<23 | mantissa<<13)
	}
	if exponent == 31 {
		return math.Float32frombits(sign | 0x7f800000 | mantissa<<13)
	}
	return math.Float32frombits(sign | (exponent-15+127)<<23 | mantissa<<13)
}

type ObjTypedArray struct {
	ObjObject
	Buffer      *ObjArrayBuffer
	BufferValue value.Value
	Offset      int
	Length      int
	Kind        uint8
}

func NewTypedArray(buffer *ObjArrayBuffer, bufferValue value.Value, offset, length int, kind uint8) *ObjTypedArray {
	array := &ObjTypedArray{Buffer: buffer, BufferValue: bufferValue, Offset: offset, Length: length, Kind: kind}
	array.Members = map[string]ObjectValueEntry{}
	array.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return array
}

func (*ObjTypedArray) Type() object.ObjType {
	return object.OBJ_TYPED_ARRAY
}

func (a *ObjTypedArray) GetReferencingValues() []value.Value {
	arr := []value.Value{a.BufferValue}
	arr = append(arr, a.ObjObject.GetReferencingValues()...)
	return arr
}

func (a *ObjTypedArray) String() string {
	return "[object TypedArray]"
}

func (a *ObjTypedArray) ElementSize() int {
	switch a.Kind {
	case TYPED_UINT8, TYPED_INT8, TYPED_UINT8_CLAMPED:
		return 1
	case TYPED_INT16, TYPED_UINT16, TYPED_FLOAT16:
		return 2
	case TYPED_INT32, TYPED_UINT32, TYPED_FLOAT32:
		return 4
	case TYPED_FLOAT64, TYPED_BIGINT64, TYPED_BIGUINT64:
		return 8
	}
	return 1
}

func (a *ObjTypedArray) Read(index int) value.Value {
	if index < 0 || index >= a.Length {
		return value.UNDEFINED
	}
	offset := a.Offset + index*a.ElementSize()
	data := a.Buffer.Data
	switch a.Kind {
	case TYPED_UINT8:
		return value.ValueFromFloat64(float64(data[offset]))
	case TYPED_INT8:
		return value.ValueFromFloat64(float64(int8(data[offset])))
	case TYPED_UINT8_CLAMPED:
		return value.ValueFromFloat64(float64(data[offset]))
	case TYPED_INT16:
		v := uint16(data[offset]) | uint16(data[offset+1])<<8
		return value.ValueFromFloat64(float64(int16(v)))
	case TYPED_UINT16:
		v := uint16(data[offset]) | uint16(data[offset+1])<<8
		return value.ValueFromFloat64(float64(v))
	case TYPED_INT32:
		v := uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24
		return value.ValueFromFloat64(float64(int32(v)))
	case TYPED_UINT32:
		v := uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24
		return value.ValueFromFloat64(float64(v))
	case TYPED_FLOAT16:
		v := uint16(data[offset]) | uint16(data[offset+1])<<8
		return value.ValueFromFloat64(float64(float16BitsToFloat32(v)))
	case TYPED_FLOAT32:
		v := uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24
		return value.ValueFromFloat64(float64(math.Float32frombits(v)))
	case TYPED_FLOAT64:
		bits := uint64(0)
		for i := 0; i < 8; i++ {
			bits |= uint64(data[offset+i]) << (8 * i)
		}
		return value.ValueFromFloat64(math.Float64frombits(bits))
	case TYPED_BIGINT64, TYPED_BIGUINT64:
		bits := uint64(0)
		for i := 0; i < 8; i++ {
			bits |= uint64(data[offset+i]) << (8 * i)
		}
		return bigIntFromUint64(bits, a.Kind == TYPED_BIGINT64)
	}
	return value.UNDEFINED
}

func (a *ObjTypedArray) Write(index int, v value.Value) {
	if index < 0 || index >= a.Length {
		return
	}
	offset := a.Offset + index*a.ElementSize()
	data := a.Buffer.Data
	switch a.Kind {
	case TYPED_UINT8:
		data[offset] = byte(uint8(int64(v.AsNumber())))
	case TYPED_INT8:
		data[offset] = byte(uint8(int8(int64(v.AsNumber()))))
	case TYPED_UINT8_CLAMPED:
		n := v.AsNumber()
		var clamped float64
		if math.IsNaN(n) {
			clamped = 0
		} else if n <= 0 {
			clamped = 0
		} else if n >= 255 {
			clamped = 255
		} else {
			clamped = math.RoundToEven(n)
		}
		data[offset] = byte(uint8(clamped))
	case TYPED_INT16:
		u := uint16(int64(v.AsNumber()))
		data[offset] = byte(u)
		data[offset+1] = byte(u >> 8)
	case TYPED_UINT16:
		u := uint16(int64(v.AsNumber()))
		data[offset] = byte(u)
		data[offset+1] = byte(u >> 8)
	case TYPED_INT32:
		u := uint32(int32(int64(v.AsNumber())))
		for i := 0; i < 4; i++ {
			data[offset+i] = byte(u >> (8 * i))
		}
	case TYPED_UINT32:
		u := uint32(int64(v.AsNumber()))
		for i := 0; i < 4; i++ {
			data[offset+i] = byte(u >> (8 * i))
		}
	case TYPED_FLOAT16:
		u := float32ToFloat16Bits(float32(v.AsNumber()))
		data[offset] = byte(u)
		data[offset+1] = byte(u >> 8)
	case TYPED_FLOAT32:
		u := math.Float32bits(float32(v.AsNumber()))
		for i := 0; i < 4; i++ {
			data[offset+i] = byte(u >> (8 * i))
		}
	case TYPED_FLOAT64:
		bits := math.Float64bits(v.AsNumber())
		for i := 0; i < 8; i++ {
			data[offset+i] = byte(bits >> (8 * i))
		}
	case TYPED_BIGINT64, TYPED_BIGUINT64:
		bits := bigIntToUint64(v)
		for i := 0; i < 8; i++ {
			data[offset+i] = byte(bits >> (8 * i))
		}
	}
}

type TypedArrayConstructor struct {
	ObjObject
	Kind uint8
	Name string
}

func NewTypedArrayConstructor(name string, kind uint8, elementSize int) *TypedArrayConstructor {
	ctor := &TypedArrayConstructor{Kind: kind, Name: name}
	ctor.Members = map[string]ObjectValueEntry{}
	ctor.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	ctor.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("BYTES_PER_ELEMENT"))), value.ValueFromFloat64(float64(elementSize)))
	ctor.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("from"))), value.EncodeHandle(heap.Allocate(NewTypedArrayStaticMethod("from", TYPED_ARRAY_STATIC_FROM, ctor))))
	ctor.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("of"))), value.EncodeHandle(heap.Allocate(NewTypedArrayStaticMethod("of", TYPED_ARRAY_STATIC_OF, ctor))))
	return ctor
}

func (*TypedArrayConstructor) Type() object.ObjType {
	return object.OBJ_NATIVE_FN
}

func (c *TypedArrayConstructor) String() string {
	return "function " + c.Name
}

type TypedArrayJoin struct {
	ObjNativeFn
}

func NewTypedArrayJoin() *TypedArrayJoin {
	fn := &TypedArrayJoin{}
	fn.name = "join"
	return fn
}

type TypedArrayToBase64 struct {
	ObjNativeFn
}

func NewTypedArrayToBase64() *TypedArrayToBase64 {
	fn := &TypedArrayToBase64{}
	fn.name = "toBase64"
	return fn
}

type TypedArraySet struct {
	ObjNativeFn
}

func NewTypedArraySet() *TypedArraySet {
	fn := &TypedArraySet{}
	fn.name = "set"
	return fn
}

type TypedArraySubarray struct {
	ObjNativeFn
}

func NewTypedArraySubarray() *TypedArraySubarray {
	fn := &TypedArraySubarray{}
	fn.name = "subarray"
	return fn
}

type TypedArraySlice struct {
	ObjNativeFn
}

func NewTypedArraySlice() *TypedArraySlice {
	fn := &TypedArraySlice{}
	fn.name = "slice"
	return fn
}

type TypedArraySort struct {
	ObjNativeFn
}

func NewTypedArraySort() *TypedArraySort {
	fn := &TypedArraySort{}
	fn.name = "sort"
	return fn
}

type TypedArrayMap struct {
	ObjNativeFn
}

func NewTypedArrayMap() *TypedArrayMap {
	fn := &TypedArrayMap{}
	fn.name = "map"
	return fn
}

type ObjDataView struct {
	ObjObject
	Buffer      *ObjArrayBuffer
	BufferValue value.Value
	Offset      int
	Length      int
}

func NewDataView(buffer *ObjArrayBuffer, bufferValue value.Value, offset, length int) *ObjDataView {
	view := &ObjDataView{Buffer: buffer, BufferValue: bufferValue, Offset: offset, Length: length}
	view.Members = map[string]ObjectValueEntry{}
	view.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return view
}

func (*ObjDataView) Type() object.ObjType {
	return object.OBJ_DATA_VIEW
}

func (v *ObjDataView) GetReferencingValues() []value.Value {
	arr := []value.Value{v.BufferValue}
	arr = append(arr, v.ObjObject.GetReferencingValues()...)
	return arr
}

func (v *ObjDataView) String() string {
	return "[object DataView]"
}

type DataViewConstructor struct {
	ObjObject
}

func NewDataViewConstructor() *DataViewConstructor {
	ctor := &DataViewConstructor{}
	ctor.Members = map[string]ObjectValueEntry{}
	ctor.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return ctor
}

func (*DataViewConstructor) Type() object.ObjType {
	return object.OBJ_NATIVE_FN
}

func (*DataViewConstructor) String() string {
	return "function DataView"
}

type DataViewMethod struct {
	ObjNativeFn
	Kind string
}

func NewDataViewMethod(kind string) *DataViewMethod {
	fn := &DataViewMethod{Kind: kind}
	fn.name = kind
	return fn
}

type TypedArrayIterator struct {
	ObjNativeFn
}

func NewTypedArrayIterator() *TypedArrayIterator {
	i := &TypedArrayIterator{}
	i.name = "iterator"
	return i
}

type TypedArrayStaticKind int

const (
	TYPED_ARRAY_STATIC_FROM TypedArrayStaticKind = iota
	TYPED_ARRAY_STATIC_OF
)

type TypedArrayStaticMethod struct {
	ObjNativeFn
	Kind        TypedArrayStaticKind
	Constructor *TypedArrayConstructor
}

func NewTypedArrayStaticMethod(name string, kind TypedArrayStaticKind, ctor *TypedArrayConstructor) *TypedArrayStaticMethod {
	m := &TypedArrayStaticMethod{Kind: kind, Constructor: ctor}
	m.name = name
	return m
}
