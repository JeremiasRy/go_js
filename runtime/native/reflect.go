package native

import (
	"go_js/heap"
	"go_js/value"
)

const REFLECT_OBJECT_NAME = "Reflect"

type ReflectDeleteProperty struct {
	ObjNativeFn
}

func NewReflectDeleteProperty() *ReflectDeleteProperty {
	fn := &ReflectDeleteProperty{}
	fn.name = "deleteProperty"
	fn.Arity = 2
	return fn
}

func NewReflectObject() *ObjObject {
	obj := NewObject()
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("deleteProperty"))), value.EncodeHandle(heap.Allocate(NewReflectDeleteProperty())))
	return obj
}
