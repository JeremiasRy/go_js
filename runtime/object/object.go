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
	OBJ_STRING_BUILDER
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
	OBJ_RESOLVE_FUNCTION
	OBJ_PROTOTYPE
	OBJ_METHOD_HANDLE
	OBJ_CLASS
	OBJ_CLASS_METHOD
	OBJ_DATE_CONSTRUCTOR
	OBJ_MAP_CONSTRUCTOR
	OBJ_MAP
	OBJ_SET
	OBJ_SET_CONSTRUCTOR
)

const MAIN_FN_NAME = "PROGRAM_MAIN"

func IsValueObject(v value.Value) (bool, uint32) {
	if v&value.TAG_OBJ == value.TAG_OBJ {
		return true, v.GetHandle()
	}
	return false, 0
}

type Callback struct {
	Fn      Callable
	ThisCtx value.Value
	Stack   []value.Value
}

type JobChannelMessage struct {
	Job      Job
	Callback Callback
	Done     func()
}

type Job interface {
	Work(callbackChannel chan *JobChannelMessage, done func())
}

type Object interface {
	Type() ObjType
	String() string
	Mark()
	Marked() bool
	Clear()
}

type GC_TAG struct {
	marked bool
}

func (tag *GC_TAG) Mark() {
	tag.marked = true
}

func (tag *GC_TAG) Marked() bool {
	return tag.marked
}

func (tag *GC_TAG) Clear() {
	tag.marked = false
}

type Hashable interface {
	GetMember(k value.Value) value.Value
	SetMember(k, v value.Value)
	HasOwn(prop value.Value) bool
}

type Callable interface {
	Object
	ValueChunk() *value.ValueChunk
	GetArity() int
	GetHeapScope() int
	SetHeapScope(scope int)
	ReturnsPromise(v bool)
	Clone() Callable
	HasRestParameter() bool
	SetHasRestParameter()
	HasArguments() bool
	SetHasArguments()
}
