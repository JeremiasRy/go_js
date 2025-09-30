package native

import (
	"fmt"
	"go_js/object"
	"go_js/value"
)

type ObjGenerator struct {
	object.ObjFunction
	ObjObject

	Ip     int
	Locals []value.Value
}

func NewGenerator(name string, arity int, chunk *value.ValueChunk) *ObjGenerator {
	if chunk == nil {
		chunk = value.NewChunk()
	}
	g := &ObjGenerator{}
	g.Name = name
	g.Chunk = chunk
	g.Arity = arity
	g.HeapScope = object.NOT_IN_HEAP_SCOPE

	g.Members = map[string]ObjectValueEntry{}
	g.SetMember(KEY_PROTO, PROTOTYPE_GENERATOR)
	return g

}

func (gn *ObjGenerator) Clone() object.Callable {
	clone := *gn
	return &clone
}
func (*ObjGenerator) Type() object.ObjType {
	return object.OBJ_FUNCTION
}

func (gn *ObjGenerator) String() string {
	return fmt.Sprintf("<%s %s>", GENERATOR_NAME, gn.Name)
}

func (gn *ObjGenerator) ValueChunk() *value.ValueChunk {
	return gn.Chunk
}

func (gn *ObjGenerator) GetArity() int {
	return gn.Arity
}

func (gn *ObjGenerator) GetHeapScope() int {
	return gn.HeapScope
}

func (gn *ObjGenerator) SetHeapScope(scope int) {
	gn.HeapScope = scope
}

func (*ObjGenerator) ReturnsPromise(v bool) {
	// just to implement the interface
}

type Next struct {
	ObjNativeFn
}

func NewNext() *Next {
	n := &Next{}
	n.name = "next"
	return n
}

type Return struct {
	ObjNativeFn
}

func NewReturn() *Return {
	r := &Return{}
	r.name = "return"
	return r
}

type Throw struct {
	ObjNativeFn
}

func NewThrow() *Throw {
	t := &Throw{}
	t.name = "throw"
	return t
}
