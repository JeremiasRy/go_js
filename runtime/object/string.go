package object

import (
	"fmt"
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
	objStr.Hash = map[string]value.Value{}

	return objStr
}

func (*ObjString) Type() ObjType {
	return OBJ_STRING
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
