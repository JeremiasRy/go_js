package native

import (
	"errors"
	"fmt"
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
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
	ec.Members = map[string]ObjectValueEntry{}
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

	var arg object.Object
	var handle value.Value

	if argHandle, ok := params[0].(value.Value); ok {
		a, err := allocator.GetObject(argHandle.GetHandle())
		handle = argHandle
		if err != nil {
			return nil, err
		}
		arg = a
	}

	if arg.Type() == object.OBJ_STRING {
		errorObject := NewError()
		errorObject.SetMember(KEY_MESSAGE, handle)
		return errorObject, nil
	}

	return nil, fmt.Errorf("arg is of wrong type")
}

type ObjectConstructor struct {
	ObjObject
}

func NewObjectConstructor() *ObjectConstructor {
	oc := &ObjectConstructor{}
	oc.Members = map[string]ObjectValueEntry{}

	oc.SetMember(KEY_VALUES, value.EncodeHandle(allocator.Allocate(NewObjectValues())))
	oc.SetMember(KEY_KEYS, value.EncodeHandle(allocator.Allocate(NewObjectKeys())))
	oc.SetMember(KEY_CREATE, value.EncodeHandle(allocator.Allocate(NewCreate())))

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
	obj := &ObjObject{Members: map[string]ObjectValueEntry{}}

	obj.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
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
	return arr
}

func NewArray(length int) *ObjArr {
	arr := NewObjArr(length)
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
	objStr.Members = map[string]ObjectValueEntry{}
	objStr.SetMember(KEY_PROTO, PROTOTYPE_STRING)
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

func (m *Main) Work(callbackChannel chan *object.JobChannelMessage, done func()) {
	msg := &object.JobChannelMessage{
		Callback: m.Fn.Clone(),
		Done:     done,
	}
	callbackChannel <- msg
}

type Log struct {
	ObjNativeFn
}

func NewLog() *Log {
	log := &Log{}
	log.name = "log"
	return log
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

func (st *SetTimeout) Work(callbackChannel chan *object.JobChannelMessage, done func()) {
	message := &object.JobChannelMessage{
		Job:      nil,
		Callback: st.callback,
		Done:     done,
	}
	if st.time == 0 {
		callbackChannel <- message
		return
	}
	tick := time.NewTicker((time.Duration(st.time) * time.Millisecond))

	for range tick.C {
		callbackChannel <- message
		break
	}
}

func (st *SetTimeout) Clone() *SetTimeout {
	clone := *st
	return &clone
}

type MapConstructor struct{}

func NewMapConstructor() *MapConstructor {
	return &MapConstructor{}
}

func (*MapConstructor) String() string {
	return "Function Map"
}

func (*MapConstructor) Type() object.ObjType {
	return object.OBJ_MAP_CONSTRUCTOR
}

func (*MapConstructor) New(params ...any) (object.Object, error) {
	return NewMap(), nil
}

type SetConstructor struct{}

func NewSetConstructor() *SetConstructor {
	return &SetConstructor{}
}

func (*SetConstructor) String() string {
	return "Function Set"
}

func (*SetConstructor) Type() object.ObjType {
	return object.OBJ_SET_CONSTRUCTOR
}

func (*SetConstructor) New(params ...any) (object.Object, error) {
	return NewSet(), nil
}
