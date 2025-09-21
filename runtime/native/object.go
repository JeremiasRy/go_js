package native

import (
	"fmt"
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
)

type ObjectValueEntry struct {
	Value value.Value
	init  int
}

type ObjObject struct {
	init int
	Hash map[string]ObjectValueEntry
}

func NewObjectHash() *ObjObject {
	return &ObjObject{
		init: 0,
		Hash: map[string]ObjectValueEntry{},
	}
}

func (*ObjObject) Type() object.ObjType {
	return object.OBJ_OBJECT
}

func (oh *ObjObject) String() string {
	return fmt.Sprintf("%v", oh.Hash)
}

func (obj *ObjObject) GetMember(k value.Value) value.Value {
	if k.IsObject() {
		member, err := allocator.GetObject(k.GetHandle())

		if err != nil {
			panic("coundn't receive object from allocator")
		}

		if str, ok := member.(*ObjString); ok {
			if entry, found := obj.Hash[str.Value]; found {
				return entry.Value
			}
			return value.EncodedUndefined()
		}
	}

	// todo: should string intern everything to our Hash i.e obj[true] = 23, obj[23] = true
	return value.EncodedUndefined()
}

func (obj *ObjObject) SetMember(k, v value.Value) {
	if k.IsObject() {
		key, err := allocator.GetObject(k.GetHandle())

		if err != nil {
			panic("coundn't receive object from allocator")
		}

		if str, ok := key.(*ObjString); ok {
			obj.Hash[str.Value] = obj.NewValueEntry(v)
		}
	}
}

func (obj *ObjObject) NewValueEntry(v value.Value) ObjectValueEntry {
	obj.init++
	return ObjectValueEntry{Value: v, init: obj.init}
}

type ObjectValues struct {
	ObjNativeFn
}

func NewObjectValues() *ObjectValues {
	ov := &ObjectValues{}
	ov.name = "values"
	return ov
}

func (*ObjectValues) Values(o value.Value) []value.Value {
	if o.IsObject() {
		obj, err := allocator.GetObject(o.GetHandle())

		if err != nil {
			panic("couldn't receive object from allocator")
		}

		switch obj := obj.(type) {
		case *ObjObject:
			{
				r := make([]value.Value, len(obj.Hash))
				for _, v := range obj.Hash {
					r[v.init-1] = v.Value
				}
				return r
			}
		}
	}
	return []value.Value{}
}

type ObjectKeys struct {
	ObjNativeFn
}

func NewObjectKeys() *ObjectKeys {
	ok := &ObjectKeys{}
	ok.name = "keys"
	return ok
}

func (*ObjectKeys) Keys(o value.Value) []value.Value {
	r := []value.Value{}
	if o.IsObject() {
		obj, err := allocator.GetObject(o.GetHandle())

		if err != nil {
			panic("couldn't receive object from allocator")
		}

		switch obj := obj.(type) {
		case *ObjObject:
			{
				r := make([]value.Value, len(obj.Hash))
				for k, v := range obj.Hash {
					val := value.EncodeHandle(allocator.Allocate(NewString(k)))
					r[v.init-1] = val
				}
				return r
			}
		}
	}
	return r
}
