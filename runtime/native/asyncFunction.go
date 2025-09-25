package native

import (
	"go_js/eventloop"
	"go_js/object"
	"go_js/value"
)

type PauseState struct {
	Stack []value.Value
	Ip    int
}

type ObjAsyncFunction struct {
	object.ObjFunction

	State                   *PauseState
	Promise                 *ObjPromise
	Awaiting                *ObjPromise
	ReturnArgumentIsPromise bool
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
	if !asyncFn.ReturnArgumentIsPromise {
		asyncFn.Promise.Resolve(v)
	}
}

func (asyncFn *ObjAsyncFunction) Pause(s []value.Value, ip int) {
	asyncFn.State = &PauseState{
		Stack: s,
		Ip:    ip,
	}
}

func (asyncFn *ObjAsyncFunction) Await(p *ObjPromise) {
	asyncFn.Awaiting = p
	eventloop.Dispatch(asyncFn)
}

func (asyncFn *ObjAsyncFunction) Work(callbackChannel chan *object.JobChannelMessage, done func()) {
	asyncFn.Awaiting.Listen()
	message := &object.JobChannelMessage{
		Job:      nil,
		Callback: asyncFn,
		Done:     done,
	}

	callbackChannel <- message
}

func (asyncFn *ObjAsyncFunction) ReturnsPromise(v bool) {
	asyncFn.ReturnArgumentIsPromise = v
}
