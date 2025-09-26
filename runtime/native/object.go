package native

import (
	"fmt"
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
	"strconv"
)

type MethodHandle struct {
	ThisContext object.Object
	Function    object.Object
}

func (m *MethodHandle) String() string {
	return fmt.Sprintf("Method handle { ThisContext: %s, Function: %s }", m.ThisContext, m.Function)
}

func (m *MethodHandle) Type() object.ObjType {
	return object.OBJ_METHOD_HANDLE
}

func NewMethodHandle(thisContext, fn object.Object) value.Value {
	h := &MethodHandle{ThisContext: thisContext, Function: fn}
	return value.EncodeHandle(allocator.Allocate(h))
}

type ObjectValueEntry struct {
	Value value.Value
	init  int
}

type ObjObject struct {
	init    int
	Members map[string]ObjectValueEntry
}

func NewObjectHash() *ObjObject {
	return &ObjObject{
		init:    0,
		Members: map[string]ObjectValueEntry{},
	}

}

func (*ObjObject) Type() object.ObjType {
	return object.OBJ_OBJECT
}

func (oh *ObjObject) String() string {
	return "[Object object]"
}

type ToString struct {
	InstanceMethod
	ObjNativeFn
}

func NewToString() *ToString {
	ts := &ToString{}
	ts.name = "toString"

	return ts
}

func (ts *ToString) ToString(obj object.Object) string {
	return obj.String()
}

func (obj *ObjObject) GetMember(k value.Value) value.Value {
	if k.IsObject() {
		o, err := allocator.GetObject(k.GetHandle())

		if err != nil {
			panic("coundn't receive object from allocator")
		}

		if o.Type() == object.OBJ_STRING {
			key := o.String()

			if v, found := obj.Members[key]; found {
				return v.Value
			}

			proto := obj.Members[PROTOTYPE_KEY].Value
			protoObj, err := allocator.GetObject(proto.GetHandle())

			if err != nil {
				panic("couldnt get prototype from object")
			}

			if protoObj, ok := protoObj.(*Prototype); ok {
				return protoObj.GetMember(k)
			}
		}
	}

	if k.IsNumber() {
		key := strconv.FormatFloat(k.AsNumber(), 'f', -1, 64)

		if v, found := obj.Members[key]; found {
			return v.Value
		}

		proto := obj.Members[PROTOTYPE_KEY].Value
		protoObj, err := allocator.GetObject(proto.GetHandle())

		if err != nil {
			panic("couldnt get prototype from object")
		}

		if protoObj, ok := protoObj.(*Prototype); ok {
			return protoObj.GetMember(k)
		}
	}

	// todo: should string intern everything to our Hash
	return value.EncodedUndefined()
}

func (obj *ObjObject) SetMember(k, v value.Value) {
	if k.IsObject() {
		o, err := allocator.GetObject(k.GetHandle())

		if err != nil {
			panic("coundn't receive object from allocator")
		}

		if o.Type() == object.OBJ_STRING {
			key := o.String()
			if entry, ok := obj.Members[key]; ok {
				entry.Value = v
				obj.Members[key] = entry
			} else {
				obj.Members[key] = obj.NewValueEntry(v)
			}
		}
	}

	if k.IsNumber() {
		key := strconv.FormatFloat(k.AsNumber(), 'f', -1, 64)

		if entry, ok := obj.Members[key]; ok {
			entry.Value = v
			obj.Members[key] = entry
		} else {
			obj.Members[key] = obj.NewValueEntry(v)
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
				r := make([]value.Value, len(obj.Members))
				for _, v := range obj.Members {
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
				r := make([]value.Value, len(obj.Members))
				for k, v := range obj.Members {
					val := value.EncodeHandle(allocator.Allocate(NewString(k)))
					r[v.init-1] = val
				}
				return r
			}
		}
	}
	return r
}
