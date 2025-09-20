package native

import (
	"errors"
	"fmt"
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
	"math"
	"time"
)

type Constructor interface {
	New(params ...any) (object.Object, error)
}

type ErrorConstructor struct {
	ObjObject
}

func NewErrorConstructor() *ErrorConstructor {
	ec := &ErrorConstructor{}
	ec.Hash = map[string]ObjectValueEntry{}
	return ec
}

func (*ErrorConstructor) String() string {
	return "Error constructor"
}

func (*ErrorConstructor) Type() object.ObjType {
	return object.OBJ_ERROR_CONSTRUCTOR
}

func (ec *ErrorConstructor) New(params ...any) (object.Object, error) {
	if len(params) < 1 {
		return nil, errors.New("not enough params for new Error()")
	}

	argHandle := params[0].(value.Value).GetHandle()
	arg, err := allocator.GetObject(argHandle)

	if err != nil {
		return nil, err
	}

	if _, ok := arg.(*ObjString); ok {
		errorObject := NewError()
		errorObject.Hash["message"] = ec.NewValueEntry(value.EncodeHandle(argHandle))
		return errorObject, nil
	}

	return nil, fmt.Errorf("arg is of wrong type")
}

type ObjectConstructor struct {
	ObjObject
}

func NewObjectConstructor() *ObjectConstructor {
	oc := &ObjectConstructor{}
	oc.Hash = map[string]ObjectValueEntry{}

	oc.Hash["values"] = oc.NewValueEntry(value.EncodeHandle(allocator.Allocate(NewObjectValues())))
	oc.Hash["keys"] = oc.NewValueEntry(value.EncodeHandle(allocator.Allocate(NewObjectKeys())))

	return oc
}

func (*ObjectConstructor) String() string {
	return "function Object"
}

func (*ObjectConstructor) Type() object.ObjType {
	return object.OBJ_OBJECT_CONSTRUCTOR
}

func (*ObjectConstructor) New() object.Object {
	return NewObject()
}

func NewObject() *ObjObject {
	obj := &ObjObject{Hash: map[string]ObjectValueEntry{}}
	return obj
}

type ArrayConstructor struct {
}

func (*ArrayConstructor) String() string {
	return "function Array"
}

func (*ArrayConstructor) Type() object.ObjType {
	return object.OBJ_ARRAY_CONSTRUCTOR
}

func (ac *ArrayConstructor) New(params ...any) *ObjArr {
	length := 8 // default length

	if len(params) == 1 {
		if num, ok := params[0].(value.Value); ok {
			length = int(num.AsNumber())
		}
	}

	arr := NewObjArr(length)

	filter := allocator.Allocate(NewArrayFilter(arr))
	push := allocator.Allocate(NewArrayPush(arr))
	forEach := allocator.Allocate(NewArrayForEach(arr))

	arr.Hash["filter"] = arr.NewValueEntry(value.EncodeHandle(filter))
	arr.Hash["push"] = arr.NewValueEntry(value.EncodeHandle(push))
	arr.Hash["forEach"] = arr.NewValueEntry(value.EncodeHandle(forEach))
	arr.Hash["length"] = arr.NewValueEntry(value.ValueFromFloat64(float64(length)))

	return arr
}

func NewArray(length int) *ObjArr {
	arr := NewObjArr(length)

	filter := allocator.Allocate(NewArrayFilter(arr))
	push := allocator.Allocate(NewArrayPush(arr))
	forEach := allocator.Allocate(NewArrayForEach(arr))

	arr.Hash["filter"] = arr.NewValueEntry(value.EncodeHandle(filter))
	arr.Hash["push"] = arr.NewValueEntry(value.EncodeHandle(push))
	arr.Hash["forEach"] = arr.NewValueEntry(value.EncodeHandle(forEach))
	arr.Hash["length"] = arr.NewValueEntry(value.ValueFromFloat64(float64(length)))
	return arr
}

type StringConstructor struct{}

func (*StringConstructor) String() string {
	return "function String"
}

func (*StringConstructor) Type() object.ObjType {
	return object.OBJ_STRING_CONSTRUCTOR
}

func NewString(str string) *ObjString {
	objStr := NewObjString(str)
	objStr.Hash["toUpperCase"] = objStr.NewValueEntry(value.EncodeHandle(allocator.Allocate(NewStringToUpperCase(objStr))))
	objStr.Hash["includes"] = objStr.NewValueEntry(value.EncodeHandle(allocator.Allocate(NewStringIncludes(objStr))))

	return objStr
}

type Main struct {
	Fn *object.ObjFunction
}

func NewMain(fn *object.ObjFunction) *Main {
	return &Main{
		Fn: fn,
	}
}

func (m *Main) Work(callbackChannel chan *object.ObjFunction) {
	callbackChannel <- m.Fn
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

type SetTimeout struct {
	ObjNativeFn
	time     int
	callback *object.ObjFunction
}

func NewSetTimeout() *SetTimeout {
	setTimeout := &SetTimeout{}
	setTimeout.name = "setTimeout"

	return setTimeout
}

func (st *SetTimeout) Set(ms int, callback *object.ObjFunction) {
	st.time = ms
	st.callback = callback
}

func (st *SetTimeout) Work(callBack chan *object.ObjFunction) {
	if st.time == 0 {
		callBack <- st.callback
		return
	}
	tick := time.NewTicker((time.Duration(st.time) * time.Millisecond))

	for range tick.C {
		callBack <- st.callback
		break
	}
}

func (st *SetTimeout) Clone() *SetTimeout {
	clone := *st
	return &clone
}
