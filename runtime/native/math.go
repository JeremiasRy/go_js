package native

import (
	"go_js/heap"
	"go_js/value"
)

const MATH_OBJECT_NAME = "Math"

type MathMethodKind int

const (
	MATH_FLOOR MathMethodKind = iota
	MATH_CEIL
	MATH_TRUNC
	MATH_ROUND
	MATH_ABS
	MATH_SIGN
	MATH_MIN
	MATH_MAX
	MATH_POW
	MATH_SQRT
)

type MathMethod struct {
	ObjNativeFn
	Kind MathMethodKind
}

func NewMathMethod(name string, kind MathMethodKind) *MathMethod {
	m := &MathMethod{Kind: kind}
	m.name = name
	return m
}

func NewMathObject() *ObjObject {
	m := NewObjectHash()
	methods := []struct {
		name string
		kind MathMethodKind
	}{
		{"floor", MATH_FLOOR},
		{"ceil", MATH_CEIL},
		{"trunc", MATH_TRUNC},
		{"round", MATH_ROUND},
		{"abs", MATH_ABS},
		{"sign", MATH_SIGN},
		{"min", MATH_MIN},
		{"max", MATH_MAX},
		{"pow", MATH_POW},
		{"sqrt", MATH_SQRT},
	}
	for _, method := range methods {
		key := value.EncodeHandle(heap.Allocate(NewLightString(method.name)))
		m.SetMember(key, value.EncodeHandle(heap.Allocate(NewMathMethod(method.name, method.kind))))
		heap.PushToRoots(key)
	}
	return m
}
