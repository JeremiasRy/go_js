package native

import (
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"strings"
)

var ERROR_PROTOTYPES = map[string]value.Value{}
var PROTOTYPE_ERROR value.Value

type ErrorToString struct {
	ObjNativeFn
}

func NewErrorToString() *ErrorToString {
	e := &ErrorToString{}
	e.name = "toString"
	return e
}

func (e *ErrorToString) ToString(name, message string) string {
	if name == "" {
		return message
	}
	if message == "" {
		return name
	}
	return name + ": " + message
}

func initErrorPrototypes() {
	base := &Prototype{name: ERROR_CONSTRUCTOR_NAME}
	base.Members = map[string]ObjectValueEntry{}
	base.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	base.SetMember(KEY_NAME, value.EncodeHandle(heap.Allocate(NewLightString(ERROR_CONSTRUCTOR_NAME))))
	base.SetMember(KEY_MESSAGE, value.EncodeHandle(heap.Allocate(NewLightString(""))))
	toString := value.EncodeHandle(heap.Allocate(NewErrorToString()))
	base.SetMember(KEY_TOSTRING, toString)
	PROTOTYPE_ERROR = value.EncodeHandle(heap.Allocate(base))
	ERROR_PROTOTYPES[ERROR_CONSTRUCTOR_NAME] = PROTOTYPE_ERROR
	heap.PushToRoots(PROTOTYPE_ERROR, toString, KEY_NAME, KEY_MESSAGE, KEY_TOSTRING)
	for _, name := range []string{"TypeError", "ReferenceError", "RangeError", "SyntaxError", "AggregateError"} {
		proto := &Prototype{name: name}
		proto.Members = map[string]ObjectValueEntry{}
		proto.SetMember(KEY_PROTO, PROTOTYPE_ERROR)
		proto.SetMember(KEY_NAME, value.EncodeHandle(heap.Allocate(NewLightString(name))))
		proto.SetMember(KEY_MESSAGE, value.EncodeHandle(heap.Allocate(NewLightString(""))))
		handle := value.EncodeHandle(heap.Allocate(proto))
		ERROR_PROTOTYPES[name] = handle
		heap.PushToRoots(handle)
	}
}

type ObjError struct {
	ObjObject
	errorName string
}

func NewError() *ObjError {
	return NewNamedError("Error")
}

func NewNamedError(name string) *ObjError {
	objError := &ObjError{errorName: name}
	objError.Members = map[string]ObjectValueEntry{}
	proto := ERROR_PROTOTYPES[name]
	if proto == 0 {
		proto = PROTOTYPE_ERROR
	}
	objError.SetMember(KEY_PROTO, proto)
	objError.SetMember(KEY_NAME, value.EncodeHandle(heap.Allocate(NewLightString(name))))
	objError.SetMember(KEY_MESSAGE, value.EncodeHandle(heap.Allocate(NewLightString(""))))
	return objError
}

func (oe *ObjError) String() string {
	if msg, ok := oe.Members["message"]; ok {
		return String(msg.Value)
	}
	return oe.errorName
}

func (oe *ObjError) Type() object.ObjType {
	return object.OBJ_ERROR
}

func ErrorNameAndMessage(err *ObjError) (string, string) {
	name := err.errorName
	if n, found := err.Members[objectKeyStringMust(KEY_NAME)]; found {
		name = String(n.Value)
	}
	message := ""
	if m, found := err.Members[objectKeyStringMust(KEY_MESSAGE)]; found {
		message = String(m.Value)
	}
	return name, message
}

func ErrorPrototypeChainContains(target value.Value, prototype value.Value) bool {
	if !target.IsObject() || !prototype.IsObject() {
		return false
	}
	current := target
	for current.IsObject() {
		obj, err := heap.GetObject(current.GetHandle())
		if err != nil {
			return false
		}
		p, ok := obj.(interface{ PrototypeValue() value.Value })
		if !ok {
			return false
		}
		current = p.PrototypeValue()
		if current.IsObject() && heap.GetPointer(current.GetHandle()) == heap.GetPointer(prototype.GetHandle()) {
			return true
		}
	}
	return false
}

var _ = strings.Builder{}
