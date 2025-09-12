package object

import "go_js/value"

type ObjFunction struct {
	name       string
	chunk      *value.ValueChunk
	arity      int
	localCount int
	heapScope  int
}

func NewFunction(name string, arity int, localVariableCount int, chunk *value.ValueChunk) *ObjFunction {
	if chunk == nil {
		chunk = value.NewChunk()
	}
	return &ObjFunction{
		name:       name,
		chunk:      chunk,
		arity:      arity,
		localCount: localVariableCount,
		heapScope:  -1,
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
func (fn *ObjFunction) LocalCount() int {
	return fn.localCount
}
func (fn *ObjFunction) SetHeapScope(scope int) {
	fn.heapScope = scope
}

func (fn *ObjFunction) HeapScope() int {
	return fn.heapScope
}

func (fn *ObjFunction) SetLocalCount(count int) {
	fn.localCount = count
}
