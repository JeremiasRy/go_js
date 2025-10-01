package native

import (
	"fmt"
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
	object.Ctx
	Status PromiseStatus
	Value  value.Value

	c chan struct{}
}

func NewPromise() *ObjPromise {
	return &ObjPromise{
		Status: PENDING,
		Value:  value.TAG_UNDEFINED,

		c: make(chan struct{}, 1),
	}
}

func (op *ObjPromise) Type() object.ObjType {
	return object.OBJ_PROMISE
}

func (op *ObjPromise) String() string {
	return fmt.Sprintf("Promise { state: %s, value: %d }", statusToString[op.Status], op.Value)
}

func (op *ObjPromise) Resolve(v value.Value) {
	op.Status = RESOLVED
	op.Value = v

	op.c <- struct{}{}
}

func (op *ObjPromise) Reject(v value.Value) {
	op.Status = REJECTED
	op.Value = v

	op.c <- struct{}{}
}

func (op *ObjPromise) Listen() {
	for range op.c {
		return
	}
}

type PromiseConstructor struct{}

func NewPromiseConstructor() *PromiseConstructor {
	return &PromiseConstructor{}
}

func (pCtor *PromiseConstructor) Type() object.ObjType {
	return object.OBJ_PROMISE_CONSTRUCTOR
}

func (pCtor *PromiseConstructor) String() string {
	return "function Promise"
}

type ResolveFunc struct {
	object.Ctx
	owner *ObjPromise
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

func (resolve *ResolveFunc) Resolve(v value.Value) {
	resolve.owner.Resolve(v)
}
