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

func (oa *ObjArr) GetMember(v value.Value) value.Value {
	if v.IsObject() {
		obj, err := allocator.GetObject(v.GetHandle())

		if err != nil {
			panic("coundn't receive object from allocator")
		}

		if str, ok := obj.(*ObjString); ok {
			return oa.Hash[str.Value].Value
		}
	}

	return oa.GetElementAt(int(v.AsNumber()))
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

func (arrObj *ObjArr) GetElementAt(i int) value.Value {
	return arrObj.items[i]
}

func (arrObj *ObjArr) PushElement(v value.Value) {
	arrObj.items = append(arrObj.items, v)
	arrObj.Hash["length"] = arrObj.NewValueEntry(value.ValueFromFloat64(float64(len(arrObj.items))))
}

func (arrObj *ObjArr) Type() object.ObjType {
	return object.OBJ_ARRAY
}

func (arrObj *ObjArr) String() string {
	return "Array"
}

func (arrObj *ObjArr) Values() []value.Value {
	return arrObj.items
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
