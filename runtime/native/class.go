package native

import (
	"fmt"
	"go_js/allocator"
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

func (oc *ObjClass) SetPrototype(prototype *Prototype) {
	prototype.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	oc.SetMember(KEY_PROTO, value.EncodeHandle(allocator.Allocate(prototype)))
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

type Method struct {
	InstanceMethod
	Fn object.Callable
}

func (m *Method) String() string {
	return "Method"
}

func (m *Method) Type() object.ObjType {
	return object.OBJ_CLASS_METHOD
}

func NewMethod(fn object.Callable) *Method {
	return &Method{Fn: fn}
}
