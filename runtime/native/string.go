package native

import (
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
	"strings"
)

type LightString struct {
	marked bool
	s      string
}

func (i *LightString) Mark() {
	i.marked = true
}

func (i *LightString) Marked() bool {
	return i.marked
}

func (i *LightString) Clear() {
	i.marked = false
}

func (LightString) Type() object.ObjType {
	return object.OBJ_STRING
}

func (str *LightString) String() string {
	return str.s
}

func (*LightString) GetReferencingValues() []value.Value {
	return []value.Value{}
}

func NewLightString(s string) *LightString {
	return &LightString{
		s: s,
	}
}

type ObjStringBuilder struct {
	marked bool
	b      *strings.Builder
}

func (*ObjStringBuilder) GetReferencingValues() []value.Value {
	return []value.Value{}
}

func (*ObjStringBuilder) Type() object.ObjType {
	return object.OBJ_STRING_BUILDER
}

func (osb *ObjStringBuilder) String() string {
	return osb.b.String()
}
func (osb *ObjStringBuilder) Mark() {
	osb.marked = true
}
func (osb *ObjStringBuilder) Clear() {
	osb.marked = false
}
func (osb *ObjStringBuilder) Marked() bool {
	return osb.marked
}

func (osb *ObjStringBuilder) Concatenate(s string) {
	osb.b.WriteString(s)
}

func (osb *ObjStringBuilder) Flush() *LightString {
	str := NewLightString(osb.b.String())
	return str
}

func NewStringBuilder(init *LightString) *ObjStringBuilder {
	b := &strings.Builder{}

	b.WriteString(string(init.s))
	b.Grow(len(init.s) + 10)
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

type StringReplace struct {
	ObjNativeFn
}

func NewStringReplace() *StringReplace {
	r := &StringReplace{}
	r.name = "replace"

	return r
}

func (*StringReplace) Replace(owner object.Object, searchValue string, replaceValue string) value.Value {
	result := strings.Replace(owner.String(), searchValue, replaceValue, 1)
	return value.EncodeHandle(allocator.Allocate(NewLightString(result)))
}

type StringSplit struct {
	ObjNativeFn
}

func NewStringSplit() *StringSplit {
	s := &StringSplit{}
	s.name = "split"

	return s
}

func (*StringSplit) Split(owner object.Object, separator string) value.Value {
	items := []value.Value{}

	for _, s := range strings.Split(owner.String(), separator) {
		items = append(items, value.EncodeHandle(allocator.Allocate(NewLightString(s))))
	}
	return value.EncodeHandle(allocator.Allocate(NewArrayFrom(items)))
}

type StringStartsWith struct {
	ObjNativeFn
}

func NewStringStartsWith() *StringStartsWith {
	s := &StringStartsWith{}
	s.name = "startsWith"

	return s
}

func (*StringStartsWith) StartsWith(owner object.Object, pattern string) value.Value {
	str := owner.String()

	if pattern == str[:len(pattern)] {
		return value.TRUE
	}
	return value.FALSE
}
