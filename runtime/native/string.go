package native

import (
	"fmt"
	"go_js/object"
	"go_js/value"
	"strings"
)

type LightString string

func (LightString) Type() object.ObjType {
	return object.OBJ_STRING
}

func (str LightString) String() string {
	return string(str)
}

type ObjStringBuilder struct {
	b *strings.Builder
}

func (*ObjStringBuilder) Type() object.ObjType {
	return object.OBJ_STRING_BUILDER
}

func (osb *ObjStringBuilder) String() string {
	return fmt.Sprintf("ObjStringBuilder { %s }", osb.b.String())
}

func (osb *ObjStringBuilder) Concatenate(s string) {
	osb.b.WriteString(s)
}

// maybe a bit misleading name since we don't reset or set the builder to nil, I'm thinking letting GC to collect these
func (osb *ObjStringBuilder) Flush() LightString {
	str := LightString(osb.b.String())
	return str
}

func NewStringBuilder(init LightString) *ObjStringBuilder {
	b := &strings.Builder{}

	b.WriteString(string(init))
	b.Grow(len(init) + 10)
	return &ObjStringBuilder{
		b: b,
	}
}

type ObjString struct {
	ObjObject
	Value string
}

func NewObjString(str string) *ObjString {
	objStr := &ObjString{
		Value: str,
	}
	objStr.Members = map[string]ObjectValueEntry{}
	objStr.SetMember(KEY_PROTO, PROTOTYPE_STRING)
	objStr.SetMember(KEY_LENGTH, value.ValueFromFloat64(float64(len(str))))

	return objStr
}

func (*ObjString) Type() object.ObjType {
	return object.OBJ_STRING
}

func (str *ObjString) String() string {
	return string(str.Value)
}

// prototype methods

type StringToUpperCase struct {
	ObjNativeFn
}

func NewStringToUpperCase() *StringToUpperCase {
	toUpperCase := &StringToUpperCase{}
	toUpperCase.name = "toUpperCase"
	return toUpperCase
}

func (toUpperCase *StringToUpperCase) ToUpperCase(owner string) *ObjString {
	return NewObjString(strings.ToUpper(owner))
}

type StringIncludes struct {
	ObjNativeFn
}

func NewStringIncludes() *StringIncludes {
	fn := &StringIncludes{}

	fn.name = "includes"
	return fn
}

func (*StringIncludes) Includes(owner string, str string) value.Value {
	if strings.Contains(owner, str) {
		return value.TRUE
	}
	return value.FALSE
}

type StringLength struct {
	ObjNativeFn
}
