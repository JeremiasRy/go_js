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

	heapScope       int
	resolveChannel  chan ResolveMessage
	continueChannel chan ResolveMessage

	Ip    int
	Stack []value.Value
}

func NewObjectPromise(name string, arity int, chunk *value.ValueChunk) *ObjPromise {
	if chunk == nil {
		chunk = value.NewChunk()
	}
	return &ObjPromise{
		name:      name,
		chunk:     chunk,
		arity:     arity,
		heapScope: -1,
		Ip:        0,
		Stack:     []value.Value{},
	}
}

func (op *ObjPromise) SetContinueChannel(c chan ResolveMessage) {
	op.continueChannel = c
}

func (op *ObjPromise) GetContinueChannel() chan ResolveMessage {
	return op.continueChannel
}

func (op *ObjPromise) SetResolveChannel(c chan ResolveMessage) {
	op.resolveChannel = c
}

func (op *ObjPromise) GetResolveChannel() chan ResolveMessage {
	return op.resolveChannel
}

func (op *ObjPromise) ResolveThySelf(resolvedValue value.Value) {
	if op.resolveChannel == nil {
		return
	}
	op.resolveChannel <- ResolveMessage{v: resolvedValue, s: RESOLVED}
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
	op.Stack = stackValues
	op.Ip = ip
}

func (op *ObjPromise) Work(callbackChannel chan *object.CallbackChannelValue, done func()) {
	select {
	case continueMessage := <-op.continueChannel:
		msg := &object.CallbackChannelValue{
			Qv:   &object.QueueValue{Fn: op, StackValues: []value.Value{continueMessage.v}},
			Done: done,
		}
		callbackChannel <- msg
	case resolveMessage := <-op.resolveChannel:
		msg := &object.CallbackChannelValue{
			Qv:   &object.QueueValue{Fn: op, StackValues: []value.Value{resolveMessage.v}},
			Done: done,
		}
		callbackChannel <- msg
	}

}

type ResolveStatus int

const (
	RESOLVED ResolveStatus = iota
	REJECTED
)

type ResolveMessage struct {
	s ResolveStatus
	v value.Value
}

type Resolve struct {
	ObjNativeFn
	c chan ResolveMessage
}

func NewResolve(c chan ResolveMessage) *Resolve {
	r := &Resolve{c: c}
	r.name = "resolve"

	return r
}

func (r *Resolve) Resolve(v value.Value) {
	msg := ResolveMessage{v: v, s: RESOLVED}
	r.c <- msg
}
