package native

import (
	"go_js/object"
	"go_js/value"
	"strings"
)

type ObjString struct {
	ObjObject
	Value string
}

type LightString string

func (LightString) Type() object.ObjType {
	return object.OBJ_STRING
}

func (str LightString) String() string {
	return string(str)
}

func NewObjString(str string) *ObjString {
	objStr := &ObjString{
		Value: str,
	}
	objStr.Members = map[string]ObjectValueEntry{}
	objStr.SetMember(KEY_PROTO, PROTOTYPE_STRING)

	return objStr
}

func (*ObjString) Type() object.ObjType {
	return object.OBJ_STRING
}

func (str *ObjString) String() string {
	return string(str.Value)
}

type StringToUpperCase struct {
	InstanceMethod
	ObjNativeFn
}

func NewStringToUpperCase() *StringToUpperCase {
	toUpperCase := &StringToUpperCase{}
	toUpperCase.name = "toUpperCase"
	return toUpperCase
}

func (toUpperCase *StringToUpperCase) ToUpperCase(owner *ObjString) *ObjString {
	return NewObjString(strings.ToUpper(owner.Value))
}

type StringIncludes struct {
	InstanceMethod
	ObjNativeFn
}

func NewStringIncludes() *StringIncludes {
	fn := &StringIncludes{}

	fn.name = "includes"
	return fn
}

func (*StringIncludes) Includes(owner *ObjString, str string) value.Value {
	if strings.Contains(owner.Value, str) {
		return value.EncodeTrue()
	}
	return value.EncodeFalse()
}
