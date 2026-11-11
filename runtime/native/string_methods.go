package native

import (
	"go_js/heap"
	"go_js/value"
)

type StringMethodKind int

const (
	STRING_SLICE StringMethodKind = iota
	STRING_SUBSTRING
	STRING_INDEX_OF
	STRING_LAST_INDEX_OF
	STRING_ENDS_WITH
	STRING_TRIM
	STRING_TRIM_START
	STRING_TRIM_END
	STRING_TO_LOWER_CASE
	STRING_PAD_START
	STRING_PAD_END
	STRING_REPEAT
	STRING_CODE_POINT_AT
	STRING_NORMALIZE
	STRING_REPLACE
	STRING_REPLACE_ALL
	STRING_SPLIT
)

type StringMethod struct {
	ObjNativeFn
	Kind StringMethodKind
}

func NewStringMethod(name string, kind StringMethodKind) *StringMethod {
	m := &StringMethod{Kind: kind}
	m.name = name
	return m
}

func AddStringMethods(p *Prototype) {
	methods := []struct {
		name string
		kind StringMethodKind
	}{
		{"slice", STRING_SLICE},
		{"substring", STRING_SUBSTRING},
		{"indexOf", STRING_INDEX_OF},
		{"lastIndexOf", STRING_LAST_INDEX_OF},
		{"endsWith", STRING_ENDS_WITH},
		{"trim", STRING_TRIM},
		{"trimStart", STRING_TRIM_START},
		{"trimEnd", STRING_TRIM_END},
		{"toLowerCase", STRING_TO_LOWER_CASE},
		{"padStart", STRING_PAD_START},
		{"padEnd", STRING_PAD_END},
		{"repeat", STRING_REPEAT},
		{"codePointAt", STRING_CODE_POINT_AT},
		{"normalize", STRING_NORMALIZE},
		{"replace", STRING_REPLACE},
		{"replaceAll", STRING_REPLACE_ALL},
		{"split", STRING_SPLIT},
	}
	for _, method := range methods {
		key := value.EncodeHandle(heap.Allocate(NewLightString(method.name)))
		p.SetMember(key, value.EncodeHandle(heap.Allocate(NewStringMethod(method.name, method.kind))))
		heap.PushToRoots(key)
	}
}

type StringStaticKind int

const (
	STRING_FROM_CODE_POINT StringStaticKind = iota
	STRING_FROM_CHAR_CODE
)

type StringStaticMethod struct {
	ObjNativeFn
	Kind StringStaticKind
}

func NewStringStaticMethod(name string, kind StringStaticKind) *StringStaticMethod {
	m := &StringStaticMethod{Kind: kind}
	m.name = name
	return m
}

func NewStringConstructorObject() *StringConstructor {
	s := &StringConstructor{}
	s.Members = map[string]ObjectValueEntry{}
	s.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("prototype"))), PROTOTYPE_STRING)
	s.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("fromCodePoint"))), value.EncodeHandle(heap.Allocate(NewStringStaticMethod("fromCodePoint", STRING_FROM_CODE_POINT))))
	s.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("fromCharCode"))), value.EncodeHandle(heap.Allocate(NewStringStaticMethod("fromCharCode", STRING_FROM_CHAR_CODE))))
	return s
}
