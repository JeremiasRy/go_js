package native

import (
	"fmt"
	"go_js/object"
	"go_js/value"
)

type ObjPromise struct {
	name  string
	chunk *value.ValueChunk
	arity int

	heapScope int
	ip        int
	stack     []value.Value
}

func (op *ObjPromise) Name() string {
	return op.name
}

func (op *ObjPromise) Type() object.ObjType {
	return object.OBJ_ASYNC_FUNCTION
}

func (op *ObjPromise) String() string {
	return fmt.Sprintf("<async fn %s>", op.name)
}

func (op *ObjPromise) ValueChunk() *value.ValueChunk {
	return op.chunk
}
func (op *ObjPromise) Arity() int {
	return op.arity
}
func (op *ObjPromise) SetHeapScope(scope int) {
	op.heapScope = scope
}

func (op *ObjPromise) HeapScope() int {
	return op.heapScope
}

func (op *ObjPromise) Pause(stackValues []value.Value, ip int) {
	op.stack = stackValues
	op.ip = ip
}
