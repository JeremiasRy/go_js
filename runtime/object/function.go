package object

import "go_js/value"

type ObjFunction struct {
	name  string
	chunk *value.ValueChunk
	arity int

	heapScope int
}

const NOT_IN_HEAP_SCOPE = -1

func NewFunction(name string, arity int, chunk *value.ValueChunk) *ObjFunction {
	if chunk == nil {
		chunk = value.NewChunk()
	}
	return &ObjFunction{
		name:      name,
		chunk:     chunk,
		arity:     arity,
		heapScope: NOT_IN_HEAP_SCOPE,
	}
}

func (fn *ObjFunction) Clone() *ObjFunction {
	clone := *fn
	return &clone
}
func (*ObjFunction) Type() ObjType {
	return OBJ_FUNCTION
}

func (fn *ObjFunction) String() string {
	return "<fn " + fn.name + ">"
}

func (fn *ObjFunction) Name() string {
	return fn.name
}

func (fn *ObjFunction) ValueChunk() *value.ValueChunk {
	return fn.chunk
}
func (fn *ObjFunction) Arity() int {
	return fn.arity
}
func (fn *ObjFunction) SetHeapScope(scope int) {
	fn.heapScope = scope
}

func (fn *ObjFunction) HeapScope() int {
	return fn.heapScope
}
