package native

import (
	"sort"

	"github.com/dlclark/regexp2"

	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

var PROTOTYPE_REGEXP value.Value

type RegExpToString struct {
	ObjNativeFn
}

func NewRegExpToString() *RegExpToString {
	r := &RegExpToString{}
	r.name = "toString"
	return r
}

type RegExpSymbolKind int

const (
	REGEXP_SYMBOL_MATCH RegExpSymbolKind = iota
	REGEXP_SYMBOL_REPLACE
	REGEXP_SYMBOL_SEARCH
	REGEXP_SYMBOL_SPLIT
	REGEXP_SYMBOL_MATCH_ALL
)

type RegExpSymbolMethod struct {
	ObjNativeFn
	Kind RegExpSymbolKind
}

func NewRegExpSymbolMethod(name string, kind RegExpSymbolKind) *RegExpSymbolMethod {
	m := &RegExpSymbolMethod{Kind: kind}
	m.name = name
	return m
}

func initRegExpPrototype() {
	proto := &Prototype{name: "RegExp"}
	proto.Members = map[string]ObjectValueEntry{}
	proto.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	proto.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("toString"))), value.EncodeHandle(heap.Allocate(NewRegExpToString())))
	proto.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("exec"))), value.EncodeHandle(heap.Allocate(NewRegExpExec())))
	proto.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("test"))), value.EncodeHandle(heap.Allocate(NewRegExpTest())))
	PROTOTYPE_REGEXP = value.EncodeHandle(heap.Allocate(proto))
	heap.PushToRoots(PROTOTYPE_REGEXP)
}

type ObjRegExp struct {
	ObjObject
	Pattern     string
	Flags       string
	LastIndex   int
	Compiled    *regexp2.Regexp
	Unicode     bool
	GroupOrder  []int
	NamedGroups []string
}

func CanonicalRegExpFlags(flags string) string {
	order := "dgimsuvy"
	result := []byte{}
	for i := 0; i < len(order); i++ {
		if hasFlag(flags, order[i]) {
			result = append(result, order[i])
		}
	}
	return string(result)
}

func NewRegExpObject(pattern, flags string) *ObjRegExp {
	flags = CanonicalRegExpFlags(flags)
	obj := &ObjRegExp{Pattern: pattern, Flags: flags, Unicode: hasFlag(flags, 'u')}
	obj.Members = map[string]ObjectValueEntry{}
	if PROTOTYPE_REGEXP == 0 {
		initRegExpPrototype()
	}
	obj.SetMember(KEY_PROTO, PROTOTYPE_REGEXP)
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("source"))), value.EncodeHandle(heap.Allocate(NewLightString(pattern))))
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("flags"))), value.EncodeHandle(heap.Allocate(NewLightString(flags))))
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("global"))), boolValueValue(hasFlag(flags, 'g')))
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("sticky"))), boolValueValue(hasFlag(flags, 'y')))
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("unicode"))), boolValueValue(hasFlag(flags, 'u')))
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("dotAll"))), boolValueValue(hasFlag(flags, 's')))
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("ignoreCase"))), boolValueValue(hasFlag(flags, 'i')))
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("multiline"))), boolValueValue(hasFlag(flags, 'm')))
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("hasIndices"))), boolValueValue(hasFlag(flags, 'd')))
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("lastIndex"))), value.ValueFromFloat64(0))
	return obj
}

func hasFlag(flags string, flag byte) bool {
	for i := 0; i < len(flags); i++ {
		if flags[i] == flag {
			return true
		}
	}
	return false
}

func (*ObjRegExp) Type() object.ObjType {
	return object.OBJ_REGEXP
}

func (r *ObjRegExp) String() string {
	return "/" + r.Pattern + "/" + r.Flags
}

type RegExpConstructor struct {
	ObjObject
}

func NewRegExpConstructor() *RegExpConstructor {
	ctor := &RegExpConstructor{}
	ctor.Members = map[string]ObjectValueEntry{}
	ctor.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	if PROTOTYPE_REGEXP != 0 {
		ctor.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("prototype"))), PROTOTYPE_REGEXP)
	}
	return ctor
}

func (*RegExpConstructor) Type() object.ObjType {
	return object.OBJ_NATIVE_FN
}

func (*RegExpConstructor) String() string {
	return "function RegExp"
}

type RegExpTest struct {
	ObjNativeFn
}

func NewRegExpTest() *RegExpTest {
	fn := &RegExpTest{}
	fn.name = "test"
	return fn
}

type RegExpExec struct {
	ObjNativeFn
}

func NewRegExpExec() *RegExpExec {
	fn := &RegExpExec{}
	fn.name = "exec"
	return fn
}

func SortRegExpGroupNames(names []string) []string {
	result := append([]string{}, names...)
	sort.Strings(result)
	return result
}
