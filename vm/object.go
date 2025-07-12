package vm

import (
	"fmt"
	"math"
	"strconv"
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

type Object interface {
	Type() ObjType
	String() string
}

type ObjFunction struct {
	name  string
	chunk *Chunk
	arity int
}

func NewFunction(name string, arity int) *ObjFunction {
	return &ObjFunction{
		name:  name,
		chunk: NewChunk(),
		arity: arity,
	}
}

func (ObjFunction) Type() ObjType {
	return OBJ_FUNCTION
}

func (fn ObjFunction) String() string {
	return "<fn " + fn.name + ">"
}

func (fn *ObjFunction) Name() string {
	return fn.name
}

func (fn *ObjFunction) Code() []uint8 {
	return fn.chunk.code
}

func (fn *ObjFunction) WriteConstant(v Value) {
	fn.chunk.WriteConstant(v)
}
func (fn *ObjFunction) EmitByte(b uint8) {
	fn.chunk.EmitByte(b)
}

func (fn *ObjFunction) EmitBytes(b ...uint8) {
	fn.chunk.EmitBytes(b...)
}

func (fn *ObjFunction) AddConstant(v Value) uint8 {
	return fn.chunk.addConstant(v)
}

func (fn *ObjFunction) IsClosure() bool {
	return false
}

func (fn *ObjFunction) ToClosure() *ObjClosure {
	return NewObjClosure(fn.name, fn.arity, fn)
}

type ObjUpvalue struct {
	location *Value
	closed   Value
}

func (upvalue *ObjUpvalue) Close() {
	upvalue.closed = *upvalue.location
	upvalue.location = &upvalue.closed
}

type ObjClosure struct {
	ObjFunction
	upvalues []*ObjUpvalue
}

func NewObjClosure(name string, arity int, objFn *ObjFunction) *ObjClosure {
	if objFn == nil {
		objFn = NewFunction(name, arity)
	}

	return &ObjClosure{
		ObjFunction: *objFn,
		upvalues:    []*ObjUpvalue{},
	}
}

func (cloure *ObjClosure) IsClosure() bool {
	return true
}

func (closure *ObjClosure) ToClosure() *ObjClosure {
	return closure
}

func (closure *ObjClosure) Name() string {
	return closure.name
}

func (ObjClosure) Type() ObjType {
	return OBJ_CLOSURE
}

func (closure ObjClosure) String() string {
	return "<fn " + closure.name + ">"
}

func (fn *ObjClosure) WriteConstant(v Value) {
	fn.chunk.WriteConstant(v)
}
func (fn *ObjClosure) EmitByte(b uint8) {
	fn.chunk.EmitByte(b)
}

func (fn *ObjClosure) EmitBytes(b ...uint8) {
	fn.chunk.EmitBytes(b...)
}
func (fn *ObjClosure) AddConstant(v Value) uint8 {
	return fn.chunk.addConstant(v)
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

func (obj *ObjHash) SetMember(member string, value Value) {
	obj.values[member] = value
}

type ObjHeapValue struct {
	value *Value
}

func (ObjHeapValue) Type() ObjType {
	return OBJ_HEAP_VALUE
}

func (objHeapValue ObjHeapValue) String() string {
	v := objHeapValue.value
	if isType(TAG_FALSE, *v) {
		return "False"
	} else if isType(TAG_TRUE, *v) {
		return "True"
	} else if isType(TAG_NIL, *v) {
		return "null"
	} else if isType(TAG_UNDEFINED, *v) {
		return "undefined"
	} else {
		return strconv.FormatFloat(v.asNumber(), 'f', -1, 64)
	}
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

func (*Log) Log(value Value) {
	fmt.Printf("%s\n", value)
}

type Clock struct {
	ObjNativeFn
}

func NewClock() *Clock {
	clock := &Clock{}
	clock.name = "Clock"
	return clock
}

func (*Clock) Clock() Value {
	return Value(math.Float64bits(float64(time.Now().UnixMilli())))
}
