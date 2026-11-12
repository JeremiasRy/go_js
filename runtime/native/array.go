package native

import (
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"slices"
	"strconv"
	"strings"
)

type ObjArr struct {
	ObjObject
	items []value.Value
}

func NewArray(length int) *ObjArr {
	arrObj := &ObjArr{items: make([]value.Value, 0, length)}
	arrObj.Members = map[string]ObjectValueEntry{}

	arrObj.SetMember(KEY_PROTO, PROTOTYPE_ARRAY)

	return arrObj
}

func NewArrayFrom(items []value.Value) *ObjArr {
	arrObj := &ObjArr{items: items}
	arrObj.Members = map[string]ObjectValueEntry{}

	arrObj.SetMember(KEY_PROTO, PROTOTYPE_ARRAY)
	arrObj.SetMember(KEY_LENGTH, value.ValueFromFloat64(float64(len(items))))

	return arrObj
}

func (oa *ObjArr) GetReferencingValues() []value.Value {
	arr := []value.Value{}
	for _, v := range oa.items {
		if v.IsObject() {
			arr = append(arr, v)
		}
	}
	arr = append(arr, oa.ObjObject.GetReferencingValues()...)

	return arr
}

func (oa *ObjArr) GetElementAt(i int) value.Value {
	if i < 0 || i >= len(oa.items) {
		return value.UNDEFINED
	}
	return oa.items[i]
}

func (oa *ObjArr) Length() int {
	return len(oa.items)
}

func (oa *ObjArr) SetLength(length int) {
	if length < 0 {
		length = 0
	}
	if length <= len(oa.items) {
		for i := len(oa.items) - 1; i >= length; i-- {
			key := strconv.Itoa(i)
			if entry, found := oa.Members[key]; found && !entry.Configurable() {
				oa.items = oa.items[:i+1]
				oa.SetMember(KEY_LENGTH, value.ValueFromFloat64(float64(i+1)))
				return
			}
			delete(oa.Members, key)
		}
		oa.items = oa.items[:length]
	} else {
		for len(oa.items) < length {
			oa.items = append(oa.items, value.EMPTY_ARRAY_ITEM)
		}
	}
	oa.SetMember(KEY_LENGTH, value.ValueFromFloat64(float64(length)))
}

func (oa *ObjArr) DeleteIndex(i int) bool {
	if i < 0 || i >= len(oa.items) {
		return true
	}
	oa.items[i] = value.EMPTY_ARRAY_ITEM
	return true
}

func (oa *ObjArr) HasIndex(i int) bool {
	return i >= 0 && i < len(oa.items) && oa.items[i] != value.EMPTY_ARRAY_ITEM
}

func (oa *ObjArr) HasOwn(prop value.Value) bool {
	if prop.IsNumber() {
		i := int(prop.AsNumber())
		if float64(i) == prop.AsNumber() {
			return oa.HasIndex(i)
		}
	}
	if key, ok := objectKeyString(prop); ok && key == "length" {
		return true
	}
	return oa.ObjObject.HasOwn(prop)
}

func (oa *ObjArr) Delete(prop value.Value) bool {
	if prop.IsNumber() {
		i := int(prop.AsNumber())
		if float64(i) == prop.AsNumber() {
			return oa.DeleteIndex(i)
		}
	}
	return oa.ObjObject.Delete(prop)
}

func (oa *ObjArr) SetElementAt(i int, v value.Value) {
	if len(oa.items) == i {
		oa.items = append(oa.items, v)
		oa.SetMember(KEY_LENGTH, value.ValueFromFloat64(float64(len(oa.items))))
		return
	} else if len(oa.items) < i {
		for range (i + 1) - len(oa.items) {
			oa.items = append(oa.items, value.EMPTY_ARRAY_ITEM)
		}
	}
	oa.items[i] = v
	oa.SetMember(KEY_LENGTH, value.ValueFromFloat64(float64(len(oa.items))))
}

func (oa *ObjArr) PushElement(v value.Value) {
	oa.items = append(oa.items, v)
	prevLength := oa.GetMember(KEY_LENGTH)

	num := prevLength.AsNumber()
	num += 1

	oa.SetMember(KEY_LENGTH, value.ValueFromFloat64(num))
}

func (oa *ObjArr) Type() object.ObjType {
	return object.OBJ_ARRAY
}

func (oa *ObjArr) String() string {
	return "Array"
}

func (oa *ObjArr) RawValues() []value.Value {
	return oa.items
}

func (oa *ObjArr) Values() []value.Value {
	values := make([]value.Value, len(oa.items))
	for i, item := range oa.items {
		if item == value.EMPTY_ARRAY_ITEM {
			values[i] = value.UNDEFINED
		} else {
			values[i] = item
		}
	}
	return values
}

func (oa *ObjArr) Keys() []value.Value {
	arr := make([]value.Value, 0, len(oa.items))

	for i := range len(oa.items) {
		arr = append(arr, value.ValueFromFloat64(float64(i)))
	}
	return arr
}

type ArrayToString struct {
	ObjNativeFn
}

func NewArrayToString() *ArrayToString {
	t := &ArrayToString{}
	t.name = "toString"
	return t
}

func (*ArrayToString) ToString(arr *ObjArr) value.Value {
	parts := make([]string, len(arr.items))
	for i, item := range arr.items {
		parts[i] = arrayElementString(item)
	}
	return value.EncodeHandle(heap.Allocate(NewLightString(strings.Join(parts, ","))))
}

type ArrayForEach struct {
	ObjNativeFn
}

func NewArrayForEach() *ArrayForEach {
	f := &ArrayForEach{}
	f.name = "forEach"
	return f
}

type ArrayPush struct {
	ObjNativeFn
}

func NewArrayPush() *ArrayPush {
	p := &ArrayPush{}
	p.name = "push"
	return p
}

func (p *ArrayPush) Push(owner *ObjArr, values []value.Value) value.Value {
	for _, v := range values {
		owner.PushElement(v)
	}
	return owner.GetMember(KEY_LENGTH)
}

type ArrayFilter struct {
	ObjNativeFn
}

func NewArrayFilter() *ArrayFilter {
	f := &ArrayFilter{}
	f.name = "filter"
	return f
}

type ArrayMap struct {
	ObjNativeFn
}

func NewArrayMap() *ArrayMap {
	m := &ArrayMap{}
	m.name = "map"
	return m
}

type ArrayReduce struct {
	ObjNativeFn
}

func NewArrayReduce() *ArrayReduce {
	r := &ArrayReduce{}
	r.name = "reduce"
	return r
}

type ArrayJoin struct {
	ObjNativeFn
}

func NewArrayJoin() *ArrayJoin {
	j := &ArrayJoin{}
	j.name = "join"
	j.Arity = 1
	return j
}

func arrayElementString(item value.Value) string {
	return arrayElementStringVisited(item, map[*ObjArr]bool{})
}

func arrayElementStringVisited(item value.Value, visited map[*ObjArr]bool) string {
	if item == value.EMPTY_ARRAY_ITEM || item == value.UNDEFINED || item == value.NULL {
		return ""
	}
	if item.IsObject() {
		if obj, err := heap.GetObject(item.GetHandle()); err == nil {
			if arr, ok := obj.(*ObjArr); ok {
				if visited[arr] {
					return ""
				}
				return joinArray(arr, ",", visited)
			}
		}
	}
	return String(item)
}

func joinArray(arr *ObjArr, separator string, visited map[*ObjArr]bool) string {
	visited[arr] = true
	defer delete(visited, arr)
	i := make([]string, len(arr.items))
	for idx, item := range arr.items {
		i[idx] = arrayElementStringVisited(item, visited)
	}
	return strings.Join(i, separator)
}

func (*ArrayJoin) Join(arr *ObjArr, separator value.Value) value.Value {
	s := ","
	if separator != value.UNDEFINED {
		s = String(separator)
	}
	res := joinArray(arr, s, map[*ObjArr]bool{})
	return value.EncodeHandle(heap.Allocate(NewLightString(res)))
}

type ArrayReverse struct {
	ObjNativeFn
}

func NewArrayReverse() *ArrayReverse {
	r := &ArrayReverse{}
	r.name = "reverse"
	return r
}

func (*ArrayReverse) Reverse(arr *ObjArr) {
	slices.Reverse(arr.items)
}

type ArrayShift struct {
	ObjNativeFn
}

func NewArrayShift() *ArrayShift {
	s := &ArrayShift{}
	s.name = "shift"
	return s
}

func (*ArrayShift) Shift(arr *ObjArr) value.Value {
	if len(arr.items) == 0 {
		return value.UNDEFINED
	}

	v := arr.items[0]
	arr.items = arr.items[1:]

	arr.SetMember(KEY_LENGTH, value.ValueFromFloat64(float64(len(arr.items))))

	return v
}

type ArrayFill struct {
	ObjNativeFn
}

func NewArrayFill() *ArrayFill {
	f := &ArrayFill{}
	f.name = "fill"
	return f
}

func (*ArrayFill) Fill(arr *ObjArr, item value.Value) *ObjArr {
	for i := range arr.items {
		arr.items[i] = item
	}
	return arr
}
