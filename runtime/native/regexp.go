package native

import (
	"sort"

	"github.com/dlclark/regexp2"

	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

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
	obj.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
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
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("test"))), value.EncodeHandle(heap.Allocate(NewRegExpTest())))
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("exec"))), value.EncodeHandle(heap.Allocate(NewRegExpExec())))
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
