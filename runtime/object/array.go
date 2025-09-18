package object

import (
	"go_js/value"
)

type ObjArr struct {
	ObjObject
	items []value.Value
}

func NewObjArr(length int) *ObjArr {
	arrObj := &ObjArr{items: make([]value.Value, 0, length)}
	arrObj.Hash = make(map[string]value.Value)

	return arrObj
}

func (arrObj *ObjArr) PushElement(v value.Value) {
	arrObj.items = append(arrObj.items, v)
	arrObj.Hash["length"] = value.ValueFromFloat64(float64(len(arrObj.items)))
}

func (arrObj *ObjArr) Type() ObjType {
	return OBJ_ARRAY
}

func (arrObj *ObjArr) String() string {
	return "Array"
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
	return p.owner.Hash["length"]
}

func (arrObj *ObjArr) Values() []value.Value {
	return arrObj.items
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
