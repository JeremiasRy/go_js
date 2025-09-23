package native

import (
	"go_js/object"
	"go_js/value"
)

type Main struct {
	Fn object.Callable
}

func NewMain(fn object.Callable) *Main {
	return &Main{
		Fn: fn,
	}
}

func (m *Main) Work(callbackChannel chan *object.CallbackChannelValue, done func()) {
	qv := &object.QueueValue{
		Fn:          m.Fn,
		StackValues: []value.Value{},
	}
	callbackChannel <- &object.CallbackChannelValue{Qv: qv, Done: done}
}
