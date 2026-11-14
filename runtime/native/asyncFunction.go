package native

import (
	"go_js/eventloop"
	"go_js/object"
	"go_js/value"
)

type PauseState struct {
	Stack      []value.Value
	Ip         int
	LocalStart int
	LocalCount int
	HeapScope  int
	OwnScope   bool
	Exceptions []AsyncException
}

type AsyncException struct {
	JumpTo    int
	StackTop  int
	Frame     int
	IsFinally bool
}

type ObjAsyncFunction struct {
	object.ObjFunction

	State                   *PauseState
	Promise                 *ObjPromise
	Awaiting                *ObjPromise
	ReturnArgumentIsPromise bool
	RejectValue             value.Value
	HasRejection            bool
	CallerStack             []value.Value
}

func NewAsyncFunction(name string, arity int, chunk *value.ValueChunk) *ObjAsyncFunction {
	if chunk == nil {
		chunk = value.NewChunk(name)
	}

	asyncFn := &ObjAsyncFunction{}

	asyncFn.Name = name
	asyncFn.Chunk = chunk
	asyncFn.Arity = arity
	asyncFn.HeapScope = object.NOT_IN_HEAP_SCOPE

	return asyncFn
}

func (asyncFn *ObjAsyncFunction) Clone() object.Callable {
	clone := *asyncFn
	return &clone
}

func (*ObjAsyncFunction) Type() object.ObjType {
	return object.OBJ_ASYNC_FUNCTION
}

func (asyncFn *ObjAsyncFunction) GetReferencingValues() []value.Value {
	arr := asyncFn.ObjFunction.GetReferencingValues()
	if asyncFn.State != nil {
		arr = append(arr, asyncFn.State.Stack...)
	}
	if asyncFn.RejectValue.IsObject() {
		arr = append(arr, asyncFn.RejectValue)
	}
	arr = append(arr, asyncFn.CallerStack...)
	return arr
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

func (asyncFn *ObjAsyncFunction) Pause(s []value.Value, ip int, localStart int, localCount int, heapScope int, ownScope bool, exceptions []AsyncException) {
	asyncFn.State = &PauseState{
		Stack:      s,
		Ip:         ip,
		LocalStart: localStart,
		LocalCount: localCount,
		HeapScope:  heapScope,
		OwnScope:   ownScope,
		Exceptions: exceptions,
	}
}

func (asyncFn *ObjAsyncFunction) Await(p *ObjPromise) {
	asyncFn.Awaiting = p
	eventloop.Dispatch(asyncFn)
}

func (asyncFn *ObjAsyncFunction) Work(callbackChannel chan *object.JobChannelMessage, done func()) {
	asyncFn.Awaiting.Listen()
	if asyncFn.State != nil && len(asyncFn.State.Stack) > 0 {
		asyncFn.State.Stack[len(asyncFn.State.Stack)-1] = asyncFn.Awaiting.Value
	}
	if asyncFn.Awaiting.Status == REJECTED {
		asyncFn.RejectValue = asyncFn.Awaiting.Value
		asyncFn.HasRejection = true
	}
	message := &object.JobChannelMessage{
		Job: nil,
		Callback: object.Callback{
			Fn:          asyncFn,
			ThisCtx:     value.UNDEFINED,
			MicroTask:   true,
			TraceLabel:  "resume await in " + asyncFn.Name,
			TraceSource: "await",
		},
		Done: done,
	}

	callbackChannel <- message
}

func (asyncFn *ObjAsyncFunction) ReturnsPromise(v bool) {
	asyncFn.ReturnArgumentIsPromise = v
}

type Resolve struct {
	ObjNativeFn
}

func NewResolve() *Resolve {
	r := &Resolve{}
	r.name = "resolve"
	return r
}

func (asyncFn *ObjAsyncFunction) FunctionBase() *object.ObjFunction {
	return &asyncFn.ObjFunction
}
