package native

import (
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
	"strconv"
)

type ObjectValueEntry struct {
	Value value.Value
	init  int
}

type ObjObject struct {
	marked  bool
	init    int
	Members map[string]ObjectValueEntry
}

func (o *ObjObject) GetReferencingValues() []value.Value {
	arr := []value.Value{}
	for _, v := range o.Members {
		if v.Value.IsObject() {
			arr = append(arr, v.Value)
		}
	}
	return arr
}

func (o *ObjObject) Mark() {
	o.marked = true
}

func (o *ObjObject) Marked() bool {
	return o.marked
}

func (o *ObjObject) Clear() {
	o.marked = false
}

func NewObjectHash() *ObjObject {
	o := &ObjObject{
		init:    0,
		Members: map[string]ObjectValueEntry{},
	}
	o.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return o
}

func NewObjectFromObject(parent value.Value) *ObjObject {
	o := &ObjObject{
		init:    0,
		Members: map[string]ObjectValueEntry{},
	}

	o.SetMember(KEY_PROTO, parent)
	return o
}

func (*ObjObject) Type() object.ObjType {
	return object.OBJ_OBJECT
}

func (oh *ObjObject) String() string {
	return "[Object object]"
}

func (o *ObjObject) Keys() []value.Value {
	keys := []value.Value{}

	for k := range o.Members {
		if k == PROTOTYPE_PROPERTY_STRING {
			continue
		}
		keys = append(keys, value.EncodeHandle(allocator.Allocate(NewLightString(k))))
	}

	p := o.Members[PROTOTYPE_PROPERTY_STRING].Value

	for p != value.NULL {
		obj, _ := allocator.GetObject(p.GetHandle())
		if proto, ok := obj.(*ObjObject); ok {
			for k := range proto.Members {
				if k == PROTOTYPE_PROPERTY_STRING {
					continue
				}
				keys = append(keys, value.EncodeHandle(allocator.Allocate(NewLightString(k))))
			}
		}
		if obj, ok := obj.(object.Hashable); ok {
			p = obj.GetMember(KEY_PROTO)
		} else {
			p = value.NULL
		}
	}
	return keys
}

func (o *ObjObject) Values() []value.Value {
	panic("unimplemented")
}

type ToString struct {
	ObjNativeFn
}

func NewToString() *ToString {
	ts := &ToString{}
	ts.name = "toString"

	return ts
}

type Create struct {
	ObjNativeFn
}

func NewCreate() *Create {
	c := &Create{}
	c.name = "create"

	return c
}

type HasOwnProperty struct {
	ObjNativeFn
}

func NewHasOwnProperty() *HasOwnProperty {
	c := &HasOwnProperty{}
	c.name = "hasOwnProperty"

	return c
}

func (*HasOwnProperty) HasOwn(obj value.Value, prop value.Value) bool {
	o, err := allocator.GetObject(obj.GetHandle())
	if err != nil {
		return false
	}
	return o.(object.Hashable).HasOwn(prop)
}

func (ts *ToString) ToString(obj object.Object) string {
	return obj.String()
}

func (obj *ObjObject) HasOwn(prop value.Value) bool {
	property, err := allocator.GetObject(prop.GetHandle())

	if err != nil || property.Type() != object.OBJ_STRING {
		return false
	}

	key := property.String()

	for k := range obj.Members {
		if k == key {
			return true
		}
	}
	return false
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

			proto := obj.Members[PROTOTYPE_PROPERTY_STRING].Value

			if proto.IsType(value.NULL) {
				return value.UNDEFINED
			}

			protoObj, err := allocator.GetObject(proto.GetHandle())

			if err != nil {
				panic("couldnt get prototype from object")
			}

			if protoObj, ok := protoObj.(object.Hashable); ok {
				return protoObj.GetMember(k)
			}
		}
	}

	if k.IsNumber() {
		key := strconv.FormatFloat(k.AsNumber(), 'f', -1, 64)

		if v, found := obj.Members[key]; found {
			return v.Value
		}

		proto := obj.Members[PROTOTYPE_PROPERTY_STRING].Value

		if proto.IsType(value.NULL) {
			return value.UNDEFINED
		}

		protoObj, err := allocator.GetObject(proto.GetHandle())

		if err != nil {
			panic("couldnt get prototype from object")
		}

		if protoObj, ok := protoObj.(*Prototype); ok {
			return protoObj.GetMember(k)
		}
	}

	// todo: should string intern everything to our Hash
	return value.UNDEFINED
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
				for k, v := range obj.Members {
					if k == PROTOTYPE_PROPERTY_STRING {
						continue
					}
					r[v.init-1] = v.Value
				}
				// remove __proto__
				return r[1:]
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
					if k == PROTOTYPE_PROPERTY_STRING {
						continue
					}
					val := value.EncodeHandle(allocator.Allocate(NewLightString(k)))
					r[v.init-1] = val
				}
				// remove __proto__
				return r[1:]
			}
		}
	}
	return r
}
