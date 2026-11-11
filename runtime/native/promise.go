package native

import (
	"fmt"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

type PromiseStatus uint8

const (
	PENDING PromiseStatus = iota
	RESOLVED
	REJECTED
)

var statusToString = map[PromiseStatus]string{
	PENDING:  "<pending>",
	RESOLVED: "<resolved>",
	REJECTED: "<rejected>",
}

type ObjPromise struct {
	ObjObject
	object.Ctx
	Status PromiseStatus
	Value  value.Value

	c            chan struct{}
	Reactions    []PromiseReaction
	Observers    []PromiseObserver
	Resolving    bool
	ResolutionID int
}

type PromiseObserver struct {
	Fulfilled func(value.Value)
	Rejected  func(value.Value)
	Roots     []value.Value
}

type PromiseReaction struct {
	OnFulfilled value.Value
	OnRejected  value.Value
	Derived     value.Value
	Finalizer   value.Value
	IsFinally   bool
}

func NewPromise() *ObjPromise {

	promise := &ObjPromise{
		Status: PENDING,
		Value:  value.UNDEFINED,

		c: make(chan struct{}, 1),
	}
	promise.Members = map[string]ObjectValueEntry{}
	promise.SetMember(KEY_PROTO, PROTOTYPE_PROMISE)

	return promise
}

func (op *ObjPromise) GetReferencingValues() []value.Value {
	arr := []value.Value{}

	arr = append(arr, op.Value)

	for _, reaction := range op.Reactions {
		arr = append(arr, reaction.OnFulfilled, reaction.OnRejected, reaction.Finalizer, reaction.Derived)
	}

	for _, observer := range op.Observers {
		arr = append(arr, observer.Roots...)
	}

	arr = append(arr, op.ObjObject.GetReferencingValues()...)

	return arr
}

func (op *ObjPromise) Type() object.ObjType {
	return object.OBJ_PROMISE
}

func (op *ObjPromise) String() string {
	return fmt.Sprintf("Promise { state: %s, value: %d }", statusToString[op.Status], op.Value)
}

func (op *ObjPromise) Resolve(v value.Value) {
	if op.Status != PENDING {
		return
	}
	op.Status = RESOLVED
	op.Value = v
	op.Resolving = false

	op.c <- struct{}{}
}

func (op *ObjPromise) Reject(v value.Value) {
	if op.Status != PENDING {
		return
	}
	op.Status = REJECTED
	op.Value = v
	op.Resolving = false

	op.c <- struct{}{}
}

func (op *ObjPromise) Listen() {
	for range op.c {
		return
	}
}

type Then struct {
	ObjNativeFn
}

func NewThen() *Then {
	t := &Then{}
	t.name = "then"
	return t
}

type PromiseCatch struct {
	ObjNativeFn
}

func NewPromiseCatch() *PromiseCatch {
	c := &PromiseCatch{}
	c.name = "catch"
	return c
}

type PromiseFinally struct {
	ObjNativeFn
}

func NewPromiseFinally() *PromiseFinally {
	f := &PromiseFinally{}
	f.name = "finally"
	return f
}

type PromiseResolve struct {
	ObjNativeFn
}

func NewPromiseResolve() *PromiseResolve {
	r := &PromiseResolve{}
	r.name = "resolve"
	return r
}

type PromiseReject struct {
	ObjNativeFn
}

func NewPromiseReject() *PromiseReject {
	r := &PromiseReject{}
	r.name = "reject"
	return r
}

type PromiseAll struct {
	ObjNativeFn
}

func NewPromiseAll() *PromiseAll {
	p := &PromiseAll{}
	p.name = "all"
	return p
}

type PromiseAllSettled struct {
	ObjNativeFn
}

func NewPromiseAllSettled() *PromiseAllSettled {
	p := &PromiseAllSettled{}
	p.name = "allSettled"
	return p
}

type PromiseRace struct {
	ObjNativeFn
}

func NewPromiseRace() *PromiseRace {
	p := &PromiseRace{}
	p.name = "race"
	return p
}

type PromiseAny struct {
	ObjNativeFn
}

func NewPromiseAny() *PromiseAny {
	p := &PromiseAny{}
	p.name = "any"
	return p
}

type PromiseConstructor struct {
	ObjObject
}

func NewPromiseConstructor() *PromiseConstructor {
	pctor := &PromiseConstructor{}
	pctor.Members = map[string]ObjectValueEntry{}

	pctor.SetMember(KEY_PROTO, PROTOTYPE_PROMISE_CONSTRUCTOR)
	pctor.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("resolve"))), value.EncodeHandle(heap.Allocate(NewPromiseResolve())))
	pctor.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("reject"))), value.EncodeHandle(heap.Allocate(NewPromiseReject())))
	pctor.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("all"))), value.EncodeHandle(heap.Allocate(NewPromiseAll())))
	pctor.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("allSettled"))), value.EncodeHandle(heap.Allocate(NewPromiseAllSettled())))
	pctor.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("race"))), value.EncodeHandle(heap.Allocate(NewPromiseRace())))
	pctor.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("any"))), value.EncodeHandle(heap.Allocate(NewPromiseAny())))
	return pctor
}

func (pCtor *PromiseConstructor) Type() object.ObjType {
	return object.OBJ_PROMISE_CONSTRUCTOR
}

func (pCtor *PromiseConstructor) String() string {
	return "function Promise"
}

type ResolveFunc struct {
	object.Ctx
	owner                *ObjPromise
	marked               bool
	Called               *bool
	AllowDuringResolving bool
	ID                   int
}

func NewResolveFuncTracked(owner *ObjPromise, called *bool, id int) *ResolveFunc {
	return &ResolveFunc{owner: owner, Called: called, AllowDuringResolving: true, ID: id}
}

func (*ResolveFunc) GetReferencingValues() []value.Value {
	return []value.Value{}
}

func NewResolveFunc(owner *ObjPromise) *ResolveFunc {
	return &ResolveFunc{owner: owner}
}

func (*ResolveFunc) Type() object.ObjType {
	return object.OBJ_RESOLVE_FUNCTION
}

func (*ResolveFunc) String() string {
	return "function Resolve"
}

func (o *ResolveFunc) Mark() {
	o.marked = true
}

func (o *ResolveFunc) Marked() bool {
	return o.marked
}

func (o *ResolveFunc) Clear() {
	o.marked = false
}

func (resolve *ResolveFunc) Resolve(v value.Value) {
	if resolve.Called != nil {
		*resolve.Called = true
	}
	resolve.owner.Resolve(v)
}

func (resolve *ResolveFunc) Promise() *ObjPromise {
	return resolve.owner
}

type RejectFunc struct {
	object.Ctx
	owner  *ObjPromise
	marked bool
	ID     int
}

func NewRejectFuncWithID(owner *ObjPromise, id int) *RejectFunc {
	return &RejectFunc{owner: owner, ID: id}
}

func (*RejectFunc) GetReferencingValues() []value.Value {
	return []value.Value{}
}

func NewRejectFunc(owner *ObjPromise) *RejectFunc {
	return &RejectFunc{owner: owner}
}

func (*RejectFunc) Type() object.ObjType {
	return object.OBJ_RESOLVE_FUNCTION
}

func (*RejectFunc) String() string {
	return "function Reject"
}

func (o *RejectFunc) Mark() {
	o.marked = true
}

func (o *RejectFunc) Marked() bool {
	return o.marked
}

func (o *RejectFunc) Clear() {
	o.marked = false
}

func (reject *RejectFunc) Reject(v value.Value) {
	reject.owner.Reject(v)
}

func (reject *RejectFunc) Promise() *ObjPromise {
	return reject.owner
}
