package native

import (
	"go_js/object"
)

type ObjError struct {
	object.GC_TAG
	ObjObject
}

func NewError() *ObjError {
	objError := &ObjError{}
	objError.Members = map[string]ObjectValueEntry{}

	return objError
}

func (oe *ObjError) String() string {
	return "Error"
}

func (oe *ObjError) Type() object.ObjType {
	return object.OBJ_ERROR
}
