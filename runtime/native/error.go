package native

import (
	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

type ObjError struct {
	ObjObject
	errorName string
}

func NewError() *ObjError {
	return NewNamedError("Error")
}

func NewNamedError(name string) *ObjError {
	objError := &ObjError{errorName: name}
	objError.Members = map[string]ObjectValueEntry{}
	objError.SetMember(KEY_PROTO, errorPrototype(name))
	objError.SetMember(KEY_NAME, value.EncodeHandle(heap.Allocate(NewLightString(name))))
	objError.SetMember(KEY_MESSAGE, value.EncodeHandle(heap.Allocate(NewLightString(""))))
	return objError
}

func (oe *ObjError) String() string {
	if msg, ok := oe.Members["message"]; ok {
		return String(msg.Value)
	}
	return oe.errorName
}

func (oe *ObjError) Type() object.ObjType {
	return object.OBJ_ERROR
}

func errorPrototype(name string) value.Value {
	if name == "Error" {
		return PROTOTYPE_OBJECT
	}
	return PROTOTYPE_OBJECT
}
