package native

import (
	"fmt"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

type GeneratorException struct {
	JumpTo    int
	StackTop  int
	Frame     int
	IsFinally bool
}

type AsyncGeneratorRequest struct {
	Input   value.Value
	Mode    int
	Promise *ObjPromise
}

type ObjGenerator struct {
	object.ObjFunction
	ObjObject

	Ip                int
	Locals            []value.Value
	Args              []value.Value
	Started           bool
	Done              bool
	Running           bool
	PendingThrow      value.Value
	HasPendingThrow   bool
	PendingReturn     bool
	ReturnValue       value.Value
	CompletionMark    value.Value
	Current           value.Value
	Exceptions        []GeneratorException
	SavedHeapScope    int
	SavedOwnScope     bool
	HasSavedScope     bool
	Delegating        bool
	DelegateStarted   bool
	DelegateIterator  value.Value
	DelegateHasResult bool
	DelegateResult    value.Value
	IsAsync           bool
	AsyncAwaiting     *ObjPromise
	AsyncProcessing   bool
	AsyncRequests     []AsyncGeneratorRequest
}

func NewAsyncGenerator(name string, arity int, chunk *value.ValueChunk) *ObjGenerator {
	g := NewGenerator(name, arity, chunk)
	g.IsAsync = true
	return g
}

func NewGenerator(name string, arity int, chunk *value.ValueChunk) *ObjGenerator {
	if chunk == nil {
		chunk = value.NewChunk(name)
	}
	g := &ObjGenerator{}
	g.Name = name
	g.Chunk = chunk
	g.Arity = arity
	g.HeapScope = object.NOT_IN_HEAP_SCOPE

	g.Members = map[string]ObjectValueEntry{}
	g.SetMember(KEY_PROTO, PROTOTYPE_GENERATOR)
	return g

}

func (i *ObjGenerator) Mark() {
	i.marked = true
}

func (i *ObjGenerator) Marked() bool {
	return i.marked
}

func (i *ObjGenerator) Clear() {
	i.marked = false
}

func (i *ObjGenerator) GetReferencingValues() []value.Value {
	arr := i.ObjObject.GetReferencingValues()
	arr = append(arr, i.ObjFunction.GetReferencingValues()...)
	arr = append(arr, i.Locals...)
	arr = append(arr, i.Args...)
	arr = append(arr, i.Current, i.PendingThrow, i.ReturnValue, i.CompletionMark, i.DelegateIterator, i.DelegateResult)
	if i.AsyncAwaiting != nil {
		arr = append(arr, value.EncodeHandle(heap.Allocate(i.AsyncAwaiting)))
	}
	for _, request := range i.AsyncRequests {
		arr = append(arr, request.Input)
		if request.Promise != nil {
			arr = append(arr, value.EncodeHandle(heap.Allocate(request.Promise)))
		}
	}
	return arr
}

func (gn *ObjGenerator) Clone() object.Callable {
	clone := *gn
	return &clone
}
func (*ObjGenerator) Type() object.ObjType {
	return object.OBJ_FUNCTION
}

func (gn *ObjGenerator) String() string {
	return fmt.Sprintf("<%s %s>", GENERATOR_NAME, gn.Name)
}

func (gn *ObjGenerator) ValueChunk() *value.ValueChunk {
	return gn.Chunk
}

func (gn *ObjGenerator) GetArity() int {
	return gn.Arity
}

func (gn *ObjGenerator) GetHeapScope() int {
	return gn.HeapScope
}

func (gn *ObjGenerator) SetHeapScope(scope int) {
	gn.HeapScope = scope
}

func (*ObjGenerator) ReturnsPromise(v bool) {
	// just to implement the interface
}

type Next struct {
	ObjNativeFn
}

func NewNext() *Next {
	n := &Next{}
	n.name = "next"
	return n
}

type Return struct {
	ObjNativeFn
}

func NewReturn() *Return {
	r := &Return{}
	r.name = "return"
	return r
}

type Throw struct {
	ObjNativeFn
}

func NewThrow() *Throw {
	t := &Throw{}
	t.name = "throw"
	return t
}

type GeneratorIterator struct {
	ObjNativeFn
}

func NewGeneratorIterator() *GeneratorIterator {
	g := &GeneratorIterator{}
	g.name = "iterator"
	return g
}

func (gn *ObjGenerator) FunctionBase() *object.ObjFunction {
	return &gn.ObjFunction
}
