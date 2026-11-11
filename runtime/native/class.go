package native

import (
	"fmt"

	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

type ClassMemberKind uint8

const (
	CLASS_FIELD ClassMemberKind = iota
	CLASS_METHOD
	CLASS_GETTER
	CLASS_SETTER
	CLASS_CONSTRUCTOR
)

type ClassMember struct {
	Key     string
	Value   value.Value
	Kind    ClassMemberKind
	Static  bool
	Private bool
}

type ObjClass struct {
	ObjObject
	name           string
	Prototype      value.Value
	SuperClass     value.Value
	instanceFields []ClassMember
	privateMembers []ClassMember
}

func (*ObjClass) Type() object.ObjType {
	return object.OBJ_CLASS
}

func (oc *ObjClass) String() string {
	return fmt.Sprintf("[class %s]", oc.name)
}

func (oc *ObjClass) Name() string {
	return oc.name
}

func (oc *ObjClass) GetReferencingValues() []value.Value {
	arr := oc.ObjObject.GetReferencingValues()
	if oc.Prototype.IsObject() {
		arr = append(arr, oc.Prototype)
	}
	if oc.SuperClass.IsObject() {
		arr = append(arr, oc.SuperClass)
	}
	for _, member := range oc.instanceFields {
		arr = append(arr, member.Value)
	}
	for _, member := range oc.privateMembers {
		arr = append(arr, member.Value)
	}
	return arr
}

func NewObjClass(name string) *ObjClass {
	oc := &ObjClass{
		name:       name,
		SuperClass: PROTOTYPE_OBJECT,
	}
	oc.Members = map[string]ObjectValueEntry{}
	return oc
}

func (oc *ObjClass) GetMember(k value.Value) value.Value {
	key, ok := objectKeyString(k)
	if !ok {
		return value.UNDEFINED
	}
	if entry, found := oc.Members[key]; found {
		if entry.accessor {
			return entry.Get
		}
		return entry.Value
	}
	if oc.SuperClass.IsObject() {
		obj, err := heap.GetObject(oc.SuperClass.GetHandle())
		if err == nil {
			if hashable, ok := obj.(object.Hashable); ok {
				return hashable.GetMember(k)
			}
		}
	}
	return value.UNDEFINED
}

func (oc *ObjClass) SetPrototype(proto value.Value) {
	oc.Prototype = proto
}

func (oc *ObjClass) SetSuperClass(super value.Value) {
	oc.SuperClass = super
}

func (oc *ObjClass) PrototypeValue() value.Value {
	return oc.SuperClass
}

func (oc *ObjClass) PushProperty(k value.Value, v value.Value) {
	oc.AddInstanceField(k, v)
}

func (oc *ObjClass) AddInstanceField(k value.Value, initializer value.Value) {
	key, ok := objectKeyString(k)
	if !ok {
		return
	}
	oc.instanceFields = append(oc.instanceFields, ClassMember{Key: key, Value: initializer, Kind: CLASS_FIELD})
}

func (oc *ObjClass) AddPrivateMember(k value.Value, member ClassMember) {
	key, ok := objectKeyString(k)
	if !ok {
		return
	}
	member.Key = key
	oc.privateMembers = append(oc.privateMembers, member)
}

func (oc *ObjClass) InstanceFields() []ClassMember {
	return oc.instanceFields
}

func (oc *ObjClass) PrivateMembers() []ClassMember {
	return oc.privateMembers
}

func (oc *ObjClass) SetMemberEntry(k value.Value, entry ObjectValueEntry) {
	key, ok := objectKeyString(k)
	if !ok {
		return
	}
	oc.Members[key] = entry
}

func (oc *ObjClass) NewValueEntry(v value.Value) ObjectValueEntry {
	return oc.ObjObject.NewValueEntry(v)
}

type Instance struct {
	ObjObject
	instanceOf  string
	initialized bool
	class       *ObjClass
}

func (i *Instance) Initialized() bool {
	return i.initialized
}

func (i *Instance) SetInitialized(initialized bool) {
	i.initialized = initialized
}

func (i *Instance) Class() *ObjClass {
	return i.class
}

func (oc *ObjClass) NewInstance() *Instance {
	i := &Instance{
		instanceOf: oc.name,
		class:      oc,
	}
	i.Members = map[string]ObjectValueEntry{}
	proto := oc.Prototype
	if !proto.IsObject() {
		proto = PROTOTYPE_OBJECT
	}
	i.SetMember(KEY_PROTO, proto)
	return i
}

func DefineClassMember(target object.Object, k, v value.Value, kind ClassMemberKind) bool {
	key, ok := objectKeyString(k)
	if !ok {
		return false
	}
	var members map[string]ObjectValueEntry
	switch target := target.(type) {
	case *ObjClass:
		members = target.Members
	case *Prototype:
		members = target.Members
	default:
		return false
	}
	entry, found := members[key]
	if !found {
		entry = ObjectValueEntry{}
	}
	entry.nonEnumerable = true
	entry.nonConfigurable = true
	switch kind {
	case CLASS_GETTER:
		entry.accessor = true
		entry.Get = v
	case CLASS_SETTER:
		entry.accessor = true
		entry.Set = v
	default:
		entry.accessor = false
		entry.Value = v
		entry.nonWritable = false
	}
	members[key] = entry
	return true
}

func InstallPrivateMember(target *ObjObject, member ClassMember) {
	entry, found := target.Members[member.Key]
	if !found {
		entry = ObjectValueEntry{}
	}
	switch member.Kind {
	case CLASS_GETTER:
		entry.accessor = true
		entry.Get = member.Value
	case CLASS_SETTER:
		entry.accessor = true
		entry.Set = member.Value
	default:
		entry.accessor = false
		entry.Value = member.Value
	}
	entry.nonEnumerable = true
	entry.nonConfigurable = true
	target.Members[member.Key] = entry
}
