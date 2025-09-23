package native

import (
	"go_js/object"
	"go_js/value"
)

type PauseState struct {
	Stack []value.Value
	Ip    int
}

type ObjAsyncFunction struct {
	object.ObjFunction

	State   *PauseState
	Promise *ObjPromise
}

func NewAsyncFunction(name string, arity int, chunk *value.ValueChunk) *ObjAsyncFunction {
	if chunk == nil {
		chunk = value.NewChunk()
	}

	asyncFn := &ObjAsyncFunction{}

	asyncFn.Name = name
	asyncFn.Chunk = chunk
	asyncFn.Arity = arity
	asyncFn.HeapScope = object.NOT_IN_HEAP_SCOPE

	return asyncFn
}

func (asyncFn *ObjAsyncFunction) Clone() *ObjAsyncFunction {
	clone := *asyncFn
	return &clone
}

func (*ObjAsyncFunction) Type() object.ObjType {
	return object.OBJ_ASYNC_FUNCTION
}

func (asyncFn *ObjAsyncFunction) String() string {
	return "<async fn " + asyncFn.Name + ">"
}

func (asyncFn *ObjAsyncFunction) SetPromise(p *ObjPromise) {
	asyncFn.Promise = p
}

func (asyncFn *ObjAsyncFunction) Resolve(v value.Value) {
	asyncFn.Promise.Status = RESOLVED
	asyncFn.Promise.Value = v
}

func (asyncFn *ObjAsyncFunction) Pause(s []value.Value, ip int) {
	asyncFn.State = &PauseState{
		Stack: s,
		Ip:    ip,
	}
}
