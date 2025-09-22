package native

import (
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
)

type ObjArr struct {
	ObjObject
	items []value.Value
}

func NewObjArr(length int) *ObjArr {
	arrObj := &ObjArr{items: make([]value.Value, 0, length)}
	arrObj.Hash = make(map[string]ObjectValueEntry)

	return arrObj
}

func (oa *ObjArr) GetMember(k value.Value) value.Value {

	if k.IsObject() {
		obj, err := allocator.GetObject(k.GetHandle())

		if err != nil {
			panic("coundn't receive object from allocator")
		}

		if str, ok := obj.(*ObjString); ok {
			if v, found := oa.Hash[str.Value]; found {
				return v.Value
			} else {
				return value.EncodedUndefined()
			}
		}
	}
	return oa.GetElementAt(int(k.AsNumber()))
}

func (oa *ObjArr) SetMember(m, v value.Value) {
	if m.IsObject() {
		obj, err := allocator.GetObject(m.GetHandle())

		if err != nil {
			panic("coundn't receive object from allocator")
		}

		if str, ok := obj.(*ObjString); ok {
			oa.Hash[str.Value] = oa.NewValueEntry(v)
			return
		}
	}

	oa.items[int(m.AsNumber())] = v
}

func (oa *ObjArr) GetElementAt(i int) value.Value {
	return oa.items[i]
}

func (oa *ObjArr) PushElement(v value.Value) {
	oa.items = append(oa.items, v)
	oa.Hash["length"] = oa.NewValueEntry(value.ValueFromFloat64(float64(len(oa.items))))
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
	Owner *ObjArr
}

func NewArrayForEach(owner *ObjArr) *ArrayForEach {
	f := &ArrayForEach{Owner: owner}
	f.name = "forEach"
	return f
}

type ArrayPush struct {
	ObjNativeFn
	owner *ObjArr
}

func NewArrayPush(owner *ObjArr) *ArrayPush {
	p := &ArrayPush{owner: owner}
	p.name = "push"
	return p
}

func (p *ArrayPush) Push(v value.Value) value.Value {
	p.owner.PushElement(v)
	return p.owner.Hash["length"].Value
}

type ArrayFilter struct {
	ObjNativeFn
	Owner *ObjArr
}

func NewArrayFilter(owner *ObjArr) *ArrayFilter {
	f := &ArrayFilter{Owner: owner}
	f.name = "filter"
	return f
}

type ArrayMap struct {
	ObjNativeFn
	Owner *ObjArr
}

func NewArrayMap(owner *ObjArr) *ArrayMap {
	m := &ArrayMap{Owner: owner}
	m.name = "map"
	return m
}
