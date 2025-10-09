package native

import (
	"go_js/object"
	"go_js/value"
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

func (oa *ObjArr) GetElementAt(i int) value.Value {
	return oa.items[i]
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

func (p *ArrayPush) Push(owner *ObjArr, v value.Value) value.Value {
	owner.PushElement(v)
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
