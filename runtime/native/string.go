package native

import (
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"strings"
)

func UTF16FromUnits(units []uint16) string {
	var builder []byte
	for i := 0; i < len(units); i++ {
		unit := units[i]
		if unit >= 0xD800 && unit <= 0xDBFF && i+1 < len(units) && units[i+1] >= 0xDC00 && units[i+1] <= 0xDFFF {
			code := 0x10000 + (int(unit-0xD800) << 10) + int(units[i+1]-0xDC00)
			builder = append(builder, []byte(string(rune(code)))...)
			i++
			continue
		}
		builder = append(builder, []byte(encodeCodeUnit(unit))...)
	}
	return string(builder)
}

func UTF16Slice(s string, start, end int) string {
	units := UTF16Units(s)
	if start < 0 {
		start = 0
	}
	if end > len(units) {
		end = len(units)
	}
	if start > end {
		start = end
	}
	return UTF16FromUnits(units[start:end])
}

func UTF16IndexOf(s, search string, from int) int {
	haystack := UTF16Units(s)
	needle := UTF16Units(search)
	if from < 0 {
		from = 0
	}
	if len(needle) == 0 {
		if from > len(haystack) {
			return len(haystack)
		}
		return from
	}
	for i := from; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func UTF16LastIndexOf(s, search string, from int) int {
	haystack := UTF16Units(s)
	needle := UTF16Units(search)
	if len(needle) == 0 {
		if from < 0 {
			return 0
		}
		if from > len(haystack) {
			return len(haystack)
		}
		return from
	}
	if from > len(haystack)-len(needle) {
		from = len(haystack) - len(needle)
	}
	for i := from; i >= 0; i-- {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func UTF16Length(s string) int {
	return len(UTF16Units(s))
}

func UTF16Units(s string) []uint16 {
	units := make([]uint16, 0, len(s))
	for i := 0; i < len(s); {
		b := s[i]
		var code int
		size := 1
		switch {
		case b < 0x80:
			code = int(b)
		case b&0xE0 == 0xC0:
			if i+1 >= len(s) {
				code = 0xFFFD
				break
			}
			code = int(b&0x1F)<<6 | int(s[i+1]&0x3F)
			size = 2
		case b&0xF0 == 0xE0:
			if i+2 >= len(s) {
				code = 0xFFFD
				break
			}
			code = int(b&0x0F)<<12 | int(s[i+1]&0x3F)<<6 | int(s[i+2]&0x3F)
			size = 3
		case b&0xF8 == 0xF0:
			if i+3 >= len(s) {
				code = 0xFFFD
				break
			}
			code = int(b&0x07)<<18 | int(s[i+1]&0x3F)<<12 | int(s[i+2]&0x3F)<<6 | int(s[i+3]&0x3F)
			size = 4
		default:
			code = 0xFFFD
		}
		i += size
		if code > 0xFFFF {
			code -= 0x10000
			units = append(units, uint16((code>>10)+0xD800), uint16((code&0x3FF)+0xDC00))
		} else {
			units = append(units, uint16(code))
		}
	}
	return units
}

func encodeCodeUnit(unit uint16) string {
	code := int(unit)
	if code < 0x80 {
		return string(byte(code))
	}
	if code < 0x800 {
		return string([]byte{byte(0xC0 | (code >> 6)), byte(0x80 | (code & 0x3F))})
	}
	return string([]byte{byte(0xE0 | (code >> 12)), byte(0x80 | ((code >> 6) & 0x3F)), byte(0x80 | (code & 0x3F))})
}

func UTF16CharAt(s string, index int) (string, bool) {
	units := UTF16Units(s)
	if index < 0 || index >= len(units) {
		return "", false
	}
	return encodeCodeUnit(units[index]), true
}

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
	return strings.Clone(osb.b.String())
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
	str := NewLightString(osb.String())
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
	objStr.SetMember(KEY_LENGTH, value.ValueFromFloat64(float64(UTF16Length(str))))

	return objStr
}

func (*ObjString) Type() object.ObjType {
	return object.OBJ_STRING
}

func (str *ObjString) String() string {
	return string(str.Value)
}

// prototype methods

type StringCharAt struct {
	ObjNativeFn
}

func NewStringCharAt() *StringCharAt {
	s := &StringCharAt{}
	s.name = "charAt"
	return s
}

func (*StringCharAt) CharAt(owner string, index int) value.Value {
	if ch, ok := UTF16CharAt(owner, index); ok {
		return value.EncodeHandle(heap.Allocate(NewLightString(ch)))
	}
	return value.EncodeHandle(heap.Allocate(NewLightString("")))
}

type StringCharCodeAt struct {
	ObjNativeFn
}

func NewStringCharCodeAt() *StringCharCodeAt {
	s := &StringCharCodeAt{}
	s.name = "charCodeAt"
	return s
}

func (*StringCharCodeAt) CharCodeAt(owner string, index int) value.Value {
	units := UTF16Units(owner)
	if index < 0 || index >= len(units) {
		return value.STANDARD_NAN
	}
	return value.ValueFromFloat64(float64(units[index]))
}

type StringToUpperCase struct {
	ObjNativeFn
}

func NewStringToUpperCase() *StringToUpperCase {
	toUpperCase := &StringToUpperCase{}
	toUpperCase.name = "toUpperCase"
	return toUpperCase
}

func (toUpperCase *StringToUpperCase) ToUpperCase(owner string) *ObjString {
	return NewObjString(strings.ReplaceAll(strings.ToUpper(owner), "ß", "SS"))
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
	return value.EncodeHandle(heap.Allocate(NewLightString(result)))
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
		items = append(items, value.EncodeHandle(heap.Allocate(NewLightString(s))))
	}
	return value.EncodeHandle(heap.Allocate(NewArrayFrom(items)))
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
