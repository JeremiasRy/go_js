package constructor

import (
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
)

type ErrorConstructor struct{}

func (oe *ErrorConstructor) String() string {
	return "Error constructor"
}

func (oe *ErrorConstructor) Type() object.ObjType {
	return object.OBJ_ERROR_CONSTRUCTOR
}

func (oe *ErrorConstructor) New(message string) value.Value {
	errorObject := object.NewError()
	errorObject.Hash["message"] = value.EncodeHandle(allocator.Allocate(object.NewObjString(message)))

	return value.EncodeHandle(allocator.Allocate(errorObject))
}
