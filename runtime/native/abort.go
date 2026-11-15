package native

import (
	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

const ABORT_CONTROLLER_NAME = "AbortController"

type AbortSignal struct {
	ObjObject
	Aborted   bool
	Listeners []value.Value
}

func NewAbortSignal() *AbortSignal {
	signal := &AbortSignal{}
	signal.Members = map[string]ObjectValueEntry{}
	signal.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	signal.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("aborted"))), value.FALSE)
	signal.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("reason"))), value.UNDEFINED)
	return signal
}

func (*AbortSignal) Type() object.ObjType {
	return object.OBJ_OBJECT
}

func (*AbortSignal) String() string {
	return "[object AbortSignal]"
}

func (signal *AbortSignal) GetReferencingValues() []value.Value {
	arr := signal.ObjObject.GetReferencingValues()
	arr = append(arr, signal.Listeners...)
	return arr
}

type AbortController struct {
	ObjObject
	Signal *AbortSignal
}

func NewAbortController() *AbortController {
	controller := &AbortController{}
	controller.Members = map[string]ObjectValueEntry{}
	controller.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)

	signal := NewAbortSignal()
	signalValue := value.EncodeHandle(heap.Allocate(signal))
	signal.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("addEventListener"))), value.EncodeHandle(heap.Allocate(NewAbortSignalAddEventListener())))
	signal.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("removeEventListener"))), value.EncodeHandle(heap.Allocate(NewAbortSignalRemoveEventListener())))

	controller.Signal = signal
	controller.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("signal"))), signalValue)
	controller.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("abort"))), value.EncodeHandle(heap.Allocate(NewAbortControllerAbort())))
	return controller
}

func (*AbortController) Type() object.ObjType {
	return object.OBJ_OBJECT
}

func (*AbortController) String() string {
	return "[object AbortController]"
}

type AbortControllerConstructor struct {
	ObjNativeFn
}

func NewAbortControllerConstructor() *AbortControllerConstructor {
	c := &AbortControllerConstructor{}
	c.name = ABORT_CONTROLLER_NAME
	return c
}

func (*AbortControllerConstructor) Type() object.ObjType {
	return object.OBJ_NATIVE_FN
}

func (*AbortControllerConstructor) String() string {
	return "function AbortController() { [native code] }"
}

type AbortControllerAbort struct {
	ObjNativeFn
}

func NewAbortControllerAbort() *AbortControllerAbort {
	fn := &AbortControllerAbort{}
	fn.name = "abort"
	return fn
}

type AbortSignalAddEventListener struct {
	ObjNativeFn
}

func NewAbortSignalAddEventListener() *AbortSignalAddEventListener {
	fn := &AbortSignalAddEventListener{}
	fn.name = "addEventListener"
	return fn
}

type AbortSignalRemoveEventListener struct {
	ObjNativeFn
}

func NewAbortSignalRemoveEventListener() *AbortSignalRemoveEventListener {
	fn := &AbortSignalRemoveEventListener{}
	fn.name = "removeEventListener"
	return fn
}
