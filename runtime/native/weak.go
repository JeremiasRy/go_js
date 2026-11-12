package native

import (
	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

type ObjWeakRef struct {
	ObjObject
	Target       value.Value
	TargetObject object.Object
	TargetHandle uint32
}

func NewObjWeakRef(target value.Value) *ObjWeakRef {
	ref := &ObjWeakRef{Target: target}
	if target.IsObject() {
		ref.TargetHandle = target.GetHandle()
		ref.TargetObject, _ = heap.GetObject(target.GetHandle())
	}
	ref.Members = map[string]ObjectValueEntry{}
	ref.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	ref.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("deref"))), hostFunction("deref", HOST_WEAK_REF_DEREF))
	return ref
}

func (*ObjWeakRef) Type() object.ObjType {
	return object.OBJ_OBJECT
}

func (*ObjWeakRef) String() string {
	return "WeakRef { <target> }"
}

func (*ObjWeakRef) GetReferencingValues() []value.Value {
	return []value.Value{}
}

type WeakRefConstructor struct {
	ObjObject
}

func NewWeakRefConstructor() *WeakRefConstructor {
	ctor := &WeakRefConstructor{}
	ctor.Members = map[string]ObjectValueEntry{}
	ctor.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return ctor
}

func (*WeakRefConstructor) Type() object.ObjType { return object.OBJ_NATIVE_FN }
func (*WeakRefConstructor) String() string       { return "function WeakRef" }

func (*WeakRefConstructor) New(params ...any) (object.Object, error) {
	if len(params) == 0 {
		return nil, errWeakRefTarget
	}
	target, ok := params[0].(value.Value)
	if !ok || !target.IsObject() {
		return nil, errWeakRefTarget
	}
	return NewObjWeakRef(target), nil
}

type weakRefError string

func (e weakRefError) Error() string { return string(e) }

const errWeakRefTarget = weakRefError("WeakRef target must be an object")

type FinalizationCell struct {
	ID           uint64
	TargetHandle uint32
	TargetPtr    uint32
	Held         value.Value
	Token        value.Value
	HasToken     bool
}

type ObjFinalizationRegistry struct {
	ObjObject
	Cleanup value.Value
	Cells   map[uint64]*FinalizationCell
	Tokens  map[value.Value]uint64
	NextID  uint64
}

func NewObjFinalizationRegistry(cleanup value.Value) *ObjFinalizationRegistry {
	registry := &ObjFinalizationRegistry{
		Cleanup: cleanup,
		Cells:   map[uint64]*FinalizationCell{},
		Tokens:  map[value.Value]uint64{},
	}
	registry.Members = map[string]ObjectValueEntry{}
	registry.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	registry.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("register"))), hostFunction("register", HOST_FINALIZATION_REGISTER))
	registry.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("unregister"))), hostFunction("unregister", HOST_FINALIZATION_UNREGISTER))
	registry.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("cleanupSome"))), hostFunction("cleanupSome", HOST_FINALIZATION_CLEANUP))
	return registry
}

func (*ObjFinalizationRegistry) Type() object.ObjType { return object.OBJ_OBJECT }
func (*ObjFinalizationRegistry) String() string       { return "FinalizationRegistry { }" }

func (r *ObjFinalizationRegistry) GetReferencingValues() []value.Value {
	items := []value.Value{r.Cleanup}
	for _, cell := range r.Cells {
		items = append(items, cell.Held)
		if cell.HasToken {
			items = append(items, cell.Token)
		}
	}
	items = append(items, r.ObjObject.GetReferencingValues()...)
	return items
}

func (r *ObjFinalizationRegistry) Register(target value.Value, held value.Value, token value.Value, hasToken bool) *FinalizationCell {
	r.NextID++
	cell := &FinalizationCell{
		ID:           r.NextID,
		TargetHandle: target.GetHandle(),
		TargetPtr:    heap.GetPointer(target.GetHandle()),
		Held:         held,
		Token:        token,
		HasToken:     hasToken,
	}
	r.Cells[cell.ID] = cell
	if hasToken {
		r.Tokens[token] = cell.ID
	}
	return cell
}

func (r *ObjFinalizationRegistry) TakeCell(id uint64) *FinalizationCell {
	cell, found := r.Cells[id]
	if !found {
		return nil
	}
	delete(r.Cells, id)
	if cell.HasToken {
		if tokenID, found := r.Tokens[cell.Token]; found && tokenID == id {
			delete(r.Tokens, cell.Token)
		}
	}
	return cell
}

func (r *ObjFinalizationRegistry) Unregister(token value.Value) bool {
	id, found := r.Tokens[token]
	if !found {
		return false
	}
	cell := r.TakeCell(id)
	if cell == nil {
		return false
	}
	heap.UnregisterFinalizer(cell.TargetPtr, cell.ID)
	return true
}

type FinalizationRegistryConstructor struct {
	ObjObject
}

func NewFinalizationRegistryConstructor() *FinalizationRegistryConstructor {
	ctor := &FinalizationRegistryConstructor{}
	ctor.Members = map[string]ObjectValueEntry{}
	ctor.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return ctor
}

func (*FinalizationRegistryConstructor) Type() object.ObjType { return object.OBJ_NATIVE_FN }
func (*FinalizationRegistryConstructor) String() string       { return "function FinalizationRegistry" }

func (*FinalizationRegistryConstructor) New(params ...any) (object.Object, error) {
	if len(params) == 0 {
		return nil, errFinalizationCallback
	}
	callback, ok := params[0].(value.Value)
	if !ok || !callback.IsObject() {
		return nil, errFinalizationCallback
	}
	return NewObjFinalizationRegistry(callback), nil
}

const errFinalizationCallback = weakRefError("FinalizationRegistry cleanup callback must be a function")
