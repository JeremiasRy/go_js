package native

import (
	"go_js/heap"
	"go_js/value"
)

const REFLECT_OBJECT_NAME = "Reflect"

type ReflectMethod struct {
	ObjNativeFn
}

func NewReflectMethod(name string) *ReflectMethod {
	fn := &ReflectMethod{}
	fn.name = name
	switch name {
	case "get", "set", "has", "deleteProperty":
		fn.Arity = 2
	case "defineProperty", "getOwnPropertyDescriptor", "setPrototypeOf":
		fn.Arity = 3
	case "apply":
		fn.Arity = 3
	case "construct":
		fn.Arity = 2
	case "ownKeys", "getPrototypeOf", "isExtensible", "preventExtensions":
		fn.Arity = 1
	}
	return fn
}

func (fn *ReflectMethod) Name() string {
	return fn.name
}

func NewReflectObject() *ObjObject {
	obj := NewObject()
	for _, name := range []string{
		"get", "set", "has", "deleteProperty", "defineProperty",
		"getOwnPropertyDescriptor", "ownKeys", "getPrototypeOf",
		"setPrototypeOf", "isExtensible", "preventExtensions", "apply", "construct",
	} {
		obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString(name))), value.EncodeHandle(heap.Allocate(NewReflectMethod(name))))
	}
	return obj
}
