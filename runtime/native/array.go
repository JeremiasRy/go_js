package native

import (
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
	"slices"
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

	return arr
}

func (oa *ObjArr) GetElementAt(i int) value.Value {
	return oa.items[i]
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

func (oa *ObjArr) Values() []value.Value {
	return oa.items
}

func (oa *ObjArr) Keys() []value.Value {
	arr := make([]value.Value, 0, len(oa.items))

	for i := range len(oa.items) {
		arr = append(arr, value.ValueFromFloat64(float64(i)))
	}
	return arr
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

func (*ArrayJoin) Join(arr *ObjArr, separator value.Value) value.Value {
	s := ","
	i := make([]string, len(arr.items))
	for idx, item := range arr.items {
		i[idx] = String(item)
	}

	if separator != value.UNDEFINED {
		s = String(separator)
	}

	res := strings.Join(i, s)
	return value.EncodeHandle(allocator.Allocate(NewLightString(res)))
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
