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

func NewObjString(str string) *ObjString {
	objStr := &ObjString{
		Value: str,
	}
	objStr.Hash = map[string]ObjectValueEntry{}

	return objStr
}

func (*ObjString) Type() object.ObjType {
	return object.OBJ_STRING
}

func (str *ObjString) String() string {
	return string(str.Value)
}

type StringToUpperCase struct {
	ObjNativeFn
	owner *ObjString
}

func NewStringToUpperCase(owner *ObjString) *StringToUpperCase {
	toUpperCase := &StringToUpperCase{
		owner: owner,
	}

	toUpperCase.name = "toUpperCase"

	return toUpperCase
}

func (toUpperCase *StringToUpperCase) ToUpperCase() *ObjString {
	return NewObjString(strings.ToUpper(toUpperCase.owner.Value))
}

type StringIncludes struct {
	ObjNativeFn
	owner *ObjString
}

func NewStringIncludes(owner *ObjString) *StringIncludes {
	fn := &StringIncludes{
		owner: owner,
	}

	fn.name = "includes"
	return fn
}

func (includes *StringIncludes) Includes(str string) value.Value {
	if strings.Contains(includes.owner.Value, str) {
		return value.EncodeTrue()
	}
	return value.EncodeFalse()
}
