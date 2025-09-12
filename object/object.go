package object

import (
	"fmt"
	"go_js/value"
)

type ObjType uint8

const (
	OBJ_FUNCTION ObjType = iota
	OBJ_TEMPLATE_LITERAL
	OBJ_CLOSURE
	OBJ_STRING
	OBJ_OBJECT
	OBJ_NATIVE_FN
	OBJ_ITERATOR
	OBJ_ERROR
	OBJ_ERROR_CONSTRUCTOR
)

const MAIN_FN_NAME = "PROGRAM_MAIN"

func IsValueObject(v value.Value) (bool, uint32) {
	if v&value.TAG_OBJ == value.TAG_OBJ {
		return true, v.GetHandle()
	}
	return false, 0
}

type Object interface {
	Type() ObjType
	String() string
}

type Callable interface {
	ValueChunk() *value.ValueChunk
	Arity() int
	LocalCount() int
	SetLocalCount(count int)
	HeapScope() int
	SetHeapScope(scope int)
	Name() string
}

type ObjObject struct {
	Hash map[string]value.Value
}

func NewObjectHash() *ObjObject {
	return &ObjObject{
		Hash: map[string]value.Value{},
	}
}

func (*ObjObject) Type() ObjType {
	return OBJ_OBJECT
}

func (oh *ObjObject) String() string {
	return fmt.Sprintf("%v", oh.Hash)
}

func (obj *ObjObject) GetMember(member string) value.Value {
	if value, found := obj.Hash[member]; found {
		return value
	}

	return value.EncodedUndefined()
}

func (obj *ObjObject) SetMember(member string, value value.Value) {
	obj.Hash[member] = value
}
