package object

import "go_js/value"

type ObjError struct {
	ObjObject
}

func NewError() *ObjError {
	objError := &ObjError{}
	objError.Hash = map[string]value.Value{}
	return objError
}

func (oe *ObjError) String() string {
	return "Error"
}

func (oe *ObjError) Type() ObjType {
	return OBJ_ERROR
}
