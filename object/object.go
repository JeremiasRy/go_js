package object

import (
	"fmt"
	"go_js/value"
	"math"
	"time"
)

type ObjType uint8

const (
	OBJ_FUNCTION ObjType = iota
	OBJ_CLOSURE
	OBJ_STRING
	OBJ_HASH
	OBJ_HEAP_VALUE
	OBJ_NATIVE_FN
)

const MAIN_FN_NAME = "PROGRAM_MAIN"

func GetObject(v value.Value) (bool, uint32) {
	if v&value.TAG_OBJ == value.TAG_OBJ {
		return true, v.GetRegister()
	}
	return false, 0
}

type Object interface {
	Type() ObjType
	String() string
}

type ObjFunction struct {
	name  string
	chunk *value.ValueChunk
	Arity int
}

func NewFunction(name string, arity int) *ObjFunction {
	return &ObjFunction{
		name:  name,
		chunk: value.NewChunk(),
		Arity: arity,
	}
}

func (*ObjFunction) Type() ObjType {
	return OBJ_FUNCTION
}

func (fn *ObjFunction) String() string {
	return "<fn " + fn.name + ">"
}

func (fn *ObjFunction) Name() string {
	return fn.name
}

func (fn *ObjFunction) ValueChunk() *value.ValueChunk {
	return fn.chunk
}

type ObjString string

func (ObjString) Type() ObjType {
	return OBJ_STRING
}

func (str ObjString) String() string {
	return string(str)
}

type ObjHash struct {
	values map[string]value.Value
}

func NewObjectHash() *ObjHash {
	return &ObjHash{
		values: map[string]value.Value{},
	}
}

func (ObjHash) Type() ObjType {
	return OBJ_HASH
}

func (ObjHash) String() string {
	return "[object Object]"
}

func (obj *ObjHash) GetMember(member string) value.Value {
	if value, found := obj.values[member]; found {
		return value
	}

	return value.EncodedUndefined()
}

func (obj *ObjHash) SetMember(member string, value value.Value) {
	obj.values[member] = value
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

func NewLog() *Log {
	log := &Log{}
	log.name = "log"
	return log
}

type Clock struct {
	ObjNativeFn
}

func NewClock() *Clock {
	clock := &Clock{}
	clock.name = "Clock"
	return clock
}

func (*Clock) Clock() value.Value {
	return value.Value(math.Float64bits(float64(time.Now().UnixMilli())))
}
