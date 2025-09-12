package object

import "go_js/value"

type ObjArr struct {
	ObjObject
	items            []value.Value
	initialized      bool
	initializedCount int
}

func NewObjArr(length int) *ObjArr {
	arrObj := &ObjArr{items: make([]value.Value, length), initializedCount: 0, initialized: false}
	arrObj.Hash = map[string]value.Value{}
	arrObj.Hash["length"] = value.ValueFromFloat64(float64(length))

	return arrObj
}

func (arrObj *ObjArr) PushElement(v value.Value) {
	if !arrObj.initialized {
		arrObj.items[arrObj.initializedCount] = v
		arrObj.initializedCount++

		arrObj.initialized = arrObj.initializedCount >= int(arrObj.Hash["length"].AsNumber())
		return
	}
	arrObj.items = append(arrObj.items, v)
	arrObj.Hash["length"] = value.ValueFromFloat64(float64(len(arrObj.items)))
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
