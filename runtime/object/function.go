package object

import "go_js/value"

type ObjFunction struct {
	Name      string
	Chunk     *value.ValueChunk
	Arity     int
	HeapScope int
}

const NOT_IN_HEAP_SCOPE int = -1

func NewFunction(name string, arity int, chunk *value.ValueChunk) *ObjFunction {
	if chunk == nil {
		chunk = value.NewChunk()
	}
	return &ObjFunction{
		Name:      name,
		Chunk:     chunk,
		Arity:     arity,
		HeapScope: NOT_IN_HEAP_SCOPE,
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
	return "<fn " + fn.Name + ">"
}

func (fn *ObjFunction) ValueChunk() *value.ValueChunk {
	return fn.Chunk
}

func (fn *ObjFunction) GetArity() int {
	return fn.Arity
}

func (fn *ObjFunction) GetHeapScope() int {
	return fn.HeapScope
}

func (fn *ObjFunction) SetHeapScope(scope int) {
	fn.HeapScope = scope
}

func (*ObjFunction) ReturnsPromise(v bool) {
	// just to implement the interface
}
