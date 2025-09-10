package object

import (
	"fmt"
	"go_js/value"
	"strings"

	"math"
	"time"
)

type ObjType uint8

const (
	OBJ_FUNCTION ObjType = iota
	OBJ_TEMPLATE_LITERAL
	OBJ_CLOSURE
	OBJ_STRING
	OBJ_HASH
	OBJ_HEAP_VALUE
	OBJ_NATIVE_FN
	OBJ_ITERATOR
)

const MAIN_FN_NAME = "PROGRAM_MAIN"

var ARR_METHOD_HANDLES map[string]uint32

func InitArrMethodMap() {
	if ARR_METHOD_HANDLES == nil {
		ARR_METHOD_HANDLES = make(map[string]uint32, 21)
	}
}

func GetObject(v value.Value) (bool, uint32) {
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

type ObjFunction struct {
	name       string
	chunk      *value.ValueChunk
	arity      int
	localCount int
	heapScope  int
}

func NewFunction(name string, arity int, localVariableCount int, chunk *value.ValueChunk) *ObjFunction {
	if chunk == nil {
		chunk = value.NewChunk()
	}
	return &ObjFunction{
		name:       name,
		chunk:      chunk,
		arity:      arity,
		localCount: localVariableCount,
		heapScope:  -1,
	}
}

func (fn *ObjFunction) Clone() *ObjFunction {
	clone := *fn
	return &clone
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
func (fn *ObjFunction) Arity() int {
	return fn.arity
}
func (fn *ObjFunction) LocalCount() int {
	return fn.localCount
}
func (fn *ObjFunction) SetHeapScope(scope int) {
	fn.heapScope = scope
}

func (fn *ObjFunction) HeapScope() int {
	return fn.heapScope
}

func (fn *ObjFunction) SetLocalCount(count int) {
	fn.localCount = count
}

type ObjString string

func (ObjString) Type() ObjType {
	return OBJ_STRING
}

func (str ObjString) String() string {
	return string(str)
}

type ObjHash struct {
	Hash map[string]value.Value
}

func NewObjectHash() *ObjHash {
	return &ObjHash{
		Hash: map[string]value.Value{},
	}
}

func (*ObjHash) Type() ObjType {
	return OBJ_HASH
}

func (oh *ObjHash) String() string {
	return fmt.Sprintf("%v", oh.Hash)
}

func (obj *ObjHash) GetMember(member string) value.Value {
	if value, found := obj.Hash[member]; found {
		return value
	}

	return value.EncodedUndefined()
}

func (obj *ObjHash) SetMember(member string, value value.Value) {
	obj.Hash[member] = value
}

type ObjArr struct {
	ObjHash
	items            []value.Value
	initialized      bool
	initializedCount int
}

func NewObjArr(length int) *ObjArr {
	arrObj := &ObjArr{items: make([]value.Value, length), initializedCount: 0, initialized: false}
	arrObj.Hash = map[string]value.Value{}
	arrObj.Hash["length"] = value.ValueFromFloat64(float64(length))
	arrObj.Hash["push"] = value.EncodeHandle(ARR_METHOD_HANDLES["push"])

	return arrObj
}

func (arrObj *ObjArr) PushElement(v value.Value) {
	if !arrObj.initialized {
		arrObj.items[arrObj.initializedCount] = v
		arrObj.initializedCount++

		arrObj.initialized = arrObj.initializedCount >= int(arrObj.Hash["length"].AsNumber())
		return
	}
	arrObj.items = append(arrObj.items, v)
	arrObj.Hash["length"] = value.ValueFromFloat64(float64(len(arrObj.items)))
}

type Push struct {
	ObjNativeFn
}

func NewPush() *Push {
	p := &Push{}
	p.name = "push"
	return &Push{}
}

func (*Push) Push(arrObj *ObjArr, v value.Value) value.Value {
	arrObj.PushElement(v)
	return arrObj.Hash["length"]
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

// for now just used for building the string at runtime. Could also be used to cache the result?
type ObjTemplateLiteral struct {
	builder *strings.Builder
}

func NewObjTemplateLiteral() *ObjTemplateLiteral {
	return &ObjTemplateLiteral{
		builder: &strings.Builder{},
	}
}

func (i *ObjTemplateLiteral) PushString(s string) error {
	_, err := fmt.Fprint(i.builder, s)
	return err
}

func (i *ObjTemplateLiteral) CreateString() string {
	str := i.builder.String()
	i.builder = nil
	return str
}

func (i *ObjTemplateLiteral) String() string {
	return "template literal builder"
}

func (i *ObjTemplateLiteral) Type() ObjType {
	return OBJ_TEMPLATE_LITERAL
}
