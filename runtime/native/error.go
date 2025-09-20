package native

import (
	"go_js/object"
)

type ObjError struct {
	ObjObject
}

func NewError() *ObjError {
	objError := &ObjError{}
	objError.Hash = map[string]ObjectValueEntry{}

	return objError
}

func (oe *ObjError) String() string {
	return "Error"
}

func (oe *ObjError) Type() object.ObjType {
	return object.OBJ_ERROR
}
