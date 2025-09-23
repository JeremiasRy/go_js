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
	Status PromiseStatus
	Value  value.Value
}

func NewPromise() *ObjPromise {
	return &ObjPromise{
		Status: PENDING,
		Value:  value.EncodedUndefined(),
	}
}

func (op *ObjPromise) Type() object.ObjType {
	return object.OBJ_PROMISE
}

func (op *ObjPromise) String() string {
	return fmt.Sprintf("Promise { state: %s, value: %d }", statusToString[op.Status], op.Value)
}

type PromiseConstructor struct {
}
