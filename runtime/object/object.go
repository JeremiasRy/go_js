package object

import (
	"go_js/value"
)

type ObjType uint8

const (
	OBJ_FUNCTION ObjType = iota
	OBJ_ASYNC_FUNCTION
	OBJ_TEMPLATE_LITERAL
	OBJ_CLOSURE
	OBJ_STRING_CONSTRUCTOR
	OBJ_STRING
	OBJ_OBJECT
	OBJ_OBJECT_CONSTRUCTOR
	OBJ_NATIVE_FN
	OBJ_ITERATOR
	OBJ_ARRAY
	OBJ_ARRAY_CONSTRUCTOR
	OBJ_ERROR
	OBJ_ERROR_CONSTRUCTOR
	OBJ_CONSOLE
	OBJ_PROMISE
	OBJ_PROMISE_CONSTRUCTOR
)

const MAIN_FN_NAME = "PROGRAM_MAIN"

func IsValueObject(v value.Value) (bool, uint32) {
	if v&value.TAG_OBJ == value.TAG_OBJ {
		return true, v.GetHandle()
	}
	return false, 0
}

type JobChannelMessage struct {
	Job      Job
	Callback Callable
	Done     func()
}

type Job interface {
	Work(callbackChannel chan *JobChannelMessage, done func())
}

type Object interface {
	Type() ObjType
	String() string
}

type Hashable interface {
	GetMember(k value.Value) value.Value
	SetMember(k, v value.Value)
}

type Callable interface {
	Object
	ValueChunk() *value.ValueChunk
	GetArity() int
	GetHeapScope() int
	SetHeapScope(scope int)
}
