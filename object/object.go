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
	OBJ_ITERATOR
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
	name       string
	chunk      *value.ValueChunk
	Arity      int
	heapValues []value.Value
}

func NewFunction(name string, arity int) *ObjFunction {
	return &ObjFunction{
		name:       name,
		chunk:      value.NewChunk(),
		Arity:      arity,
		heapValues: []value.Value{},
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

type ObjHeapValue struct {
	value *value.Value
}

func NewHeapValue(value value.Value) *ObjHeapValue {
	return &ObjHeapValue{value: &value}
}

func (ohv *ObjHeapValue) String() string {
	return "Heap Values"
}

func (ohv *ObjHeapValue) Type() ObjType {
	return OBJ_HEAP_VALUE
}

type ObjHash struct {
	values map[string]value.Value
}

func NewObjectHash() *ObjHash {
	return &ObjHash{
		values: map[string]value.Value{},
	}
}

func (*ObjHash) Type() ObjType {
	return OBJ_HASH
}

func (oh *ObjHash) String() string {
	return fmt.Sprintf("%v", oh.values)
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

type ObjArr struct {
	ObjHash
	items            []value.Value
	initialized      bool
	initializedCount int
}

func NewObjArr(length int) *ObjArr {
	arrObj := &ObjArr{items: make([]value.Value, length), initializedCount: 0, initialized: false}
	arrObj.values = map[string]value.Value{}
	arrObj.values["length"] = value.ValueFromFloat64(float64(length))

	return arrObj
}

func (arrObj *ObjArr) PushElement(v value.Value) {
	if !arrObj.initialized {
		arrObj.items[arrObj.initializedCount] = v
		arrObj.initializedCount++

		arrObj.initialized = arrObj.initializedCount >= int(arrObj.values["length"].AsNumber())
		return
	}
	arrObj.items = append(arrObj.items, v)
	arrObj.values["length"] = value.ValueFromFloat64(float64(len(arrObj.items)))
}

func (arrObj *ObjArr) Values() []value.Value {
	return arrObj.items
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

type Iterable interface {
	Values() []value.Value
}

type Iterator struct {
	current int
	values  []value.Value
}

func NewIterator(obj Iterable) *Iterator {
	return &Iterator{values: obj.Values(), current: -1}
}

func (i *Iterator) Next() bool {
	if i.current >= len(i.values)-1 {
		return true
	}
	i.current++
	return false
}

func (i *Iterator) Current() value.Value {
	return i.values[i.current]
}

func (i *Iterator) String() string {
	return "Iterator object"
}

func (i *Iterator) Type() ObjType {
	return OBJ_ITERATOR
}
