package native

import (
	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

type ObjRegExp struct {
	ObjObject
	Pattern   string
	Flags     string
	LastIndex int
}

func NewRegExpObject(pattern, flags string) *ObjRegExp {
	obj := &ObjRegExp{Pattern: pattern, Flags: flags}
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
