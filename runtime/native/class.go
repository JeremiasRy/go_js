package native

import (
	"fmt"

	"go_js/object"
	"go_js/value"
)

type ObjClass struct {
	ObjObject
	name       string
	properties map[value.Value]value.Value
}

func (*ObjClass) Type() object.ObjType {
	return object.OBJ_CLASS
}

func (oc *ObjClass) String() string {
	return fmt.Sprintf("[class %s]", oc.name)
}

func (o *ObjClass) GetReferencingValues() []value.Value {
	arr := []value.Value{}
	for _, v := range o.Members {
		if v.Value.IsObject() {
			arr = append(arr, v.Value)
		}
	}

	for k, v := range o.properties {
		arr = append(arr, k, v)
	}

	return arr
}

func NewObjClass(name string) *ObjClass {
	oc := &ObjClass{
		name:       name,
		properties: map[value.Value]value.Value{},
	}
	oc.Members = map[string]ObjectValueEntry{}

	return oc
}

func (oc *ObjClass) PushProperty(k value.Value, v value.Value) {
	oc.properties[k] = v
}

func (oc *ObjClass) SetPrototype(proto value.Value) {
	oc.SetMember(KEY_PROTO, proto)
}

type Instance struct {
	ObjObject
	instanceOf string
}

func (oc *ObjClass) NewInstance() *Instance {
	i := &Instance{
		instanceOf: oc.name,
	}
	i.Members = map[string]ObjectValueEntry{}
	i.SetMember(KEY_PROTO, oc.GetMember(KEY_PROTO))

	for k, v := range oc.properties {
		i.SetMember(k, v)
	}

	return i
}
