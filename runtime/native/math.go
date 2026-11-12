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
	MATH_ATAN2
	MATH_HYPOT
	MATH_CBRT
	MATH_LOG2
	MATH_LOG10
	MATH_LOG1P
	MATH_EXPM1
	MATH_SINH
	MATH_COSH
	MATH_TANH
	MATH_ASINH
	MATH_ACOSH
	MATH_ATANH
	MATH_IMUL
	MATH_CLZ32
	MATH_FROUND
	MATH_RANDOM
	MATH_SUM_PRECISE
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
		{"atan2", MATH_ATAN2},
		{"hypot", MATH_HYPOT},
		{"cbrt", MATH_CBRT},
		{"log2", MATH_LOG2},
		{"log10", MATH_LOG10},
		{"log1p", MATH_LOG1P},
		{"expm1", MATH_EXPM1},
		{"sinh", MATH_SINH},
		{"cosh", MATH_COSH},
		{"tanh", MATH_TANH},
		{"asinh", MATH_ASINH},
		{"acosh", MATH_ACOSH},
		{"atanh", MATH_ATANH},
		{"imul", MATH_IMUL},
		{"clz32", MATH_CLZ32},
		{"fround", MATH_FROUND},
		{"random", MATH_RANDOM},
		{"sumPrecise", MATH_SUM_PRECISE},
	}
	for _, method := range methods {
		key := value.EncodeHandle(heap.Allocate(NewLightString(method.name)))
		m.SetMember(key, value.EncodeHandle(heap.Allocate(NewMathMethod(method.name, method.kind))))
		heap.PushToRoots(key)
	}
	return m
}
