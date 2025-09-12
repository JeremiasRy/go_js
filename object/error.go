package object

import "fmt"

type ErrorConstructor struct{}

func (oe *ErrorConstructor) String() string {
	return "Error constructor"
}

func (oe *ErrorConstructor) Type() ObjType {
	return OBJ_ERROR
}

func (oe *ErrorConstructor) New(message string) *ObjError {
	return NewError(message)
}

type ObjError struct {
	message string
}

func NewError(message string) *ObjError {
	return &ObjError{
		message: message,
	}
}

func (oe *ObjError) String() string {
	return fmt.Sprintf("Error { message: %s }", oe.message)
}

func (oe *ObjError) Type() ObjType {
	return OBJ_ERROR
}
