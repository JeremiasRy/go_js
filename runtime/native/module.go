package native

import (
	"go_js/object"
	"go_js/value"
)

type ObjModuleNamespace struct {
	ObjObject
	Bindings          map[string]int
	NamespaceBindings map[string]value.Value
}

func NewModuleNamespace() *ObjModuleNamespace {
	namespace := &ObjModuleNamespace{
		ObjObject:         ObjObject{Members: map[string]ObjectValueEntry{}},
		Bindings:          map[string]int{},
		NamespaceBindings: map[string]value.Value{},
	}
	namespace.SetExtensible(false)
	return namespace
}

func (*ObjModuleNamespace) Type() object.ObjType {
	return object.OBJ_OBJECT
}

func (namespace *ObjModuleNamespace) String() string {
	return "[object Module]"
}

func (namespace *ObjModuleNamespace) GetReferencingValues() []value.Value {
	arr := namespace.ObjObject.GetReferencingValues()
	for _, v := range namespace.NamespaceBindings {
		if v.IsObject() {
			arr = append(arr, v)
		}
	}
	return arr
}

func (namespace *ObjModuleNamespace) AddBinding(name string, slot int) {
	namespace.Bindings[name] = slot
	entry := namespace.NewValueEntry(value.UNDEFINED)
	entry.nonConfigurable = true
	namespace.Members[name] = entry
}

func (namespace *ObjModuleNamespace) AddNamespaceBinding(name string, v value.Value) {
	namespace.NamespaceBindings[name] = v
	entry := namespace.NewValueEntry(v)
	entry.nonConfigurable = true
	namespace.Members[name] = entry
}
