package vm

import "fmt"

type ObjType uint8

const (
	OBJ_FUNCTION ObjType = iota
	OBJ_STRING
	OBJ_HASH
	OBJ_NATIVE_FN
)

const MAIN_FN_NAME = "PROGRAM_MAIN"

type Object interface {
	Type() ObjType
	String() string
}

type ObjFunction struct {
	name  string
	chunk *Chunk
	arity int
}

func (ObjFunction) Type() ObjType {
	return OBJ_FUNCTION
}

func NewFunction(name string, arity int) *ObjFunction {
	return &ObjFunction{
		name:  name,
		chunk: NewChunk(),
		arity: arity,
	}
}

func (fn ObjFunction) String() string {
	return "<fn " + fn.name + ">"
}

type ObjString string

func (ObjString) Type() ObjType {
	return OBJ_STRING
}

func (str ObjString) String() string {
	return string(str)
}

type ObjHash struct {
	values map[string]Value
}

func NewObjectHash() *ObjHash {
	return &ObjHash{
		values: map[string]Value{},
	}
}

func (ObjHash) Type() ObjType {
	return OBJ_HASH
}

func (ObjHash) String() string {
	return "[object Object]"
}

func (obj *ObjHash) GetMember(member string) Value {
	if value, found := obj.values[member]; found {
		return value
	}

	return EncodedUndefined()
}

type ObjNativeFn struct {
	name string
}

func (ObjNativeFn) Type() ObjType {
	return OBJ_NATIVE_FN
}

func (onf *ObjNativeFn) String() string {
	return fmt.Sprintf("<native fn %s()>", onf.name)
}

type Log struct {
	ObjNativeFn
}

func (*Log) Log(value Value) {
	println(value.String())
}
