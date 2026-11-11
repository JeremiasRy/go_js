package native

import "go_js/value"

type FunctionCall struct {
	ObjNativeFn
}

func NewFunctionCall() *FunctionCall {
	f := &FunctionCall{}
	f.name = "call"
	return f
}

type FunctionApply struct {
	ObjNativeFn
}

func NewFunctionApply() *FunctionApply {
	f := &FunctionApply{}
	f.name = "apply"
	return f
}

type FunctionBind struct {
	ObjNativeFn
}

func NewFunctionBind() *FunctionBind {
	f := &FunctionBind{}
	f.name = "bind"
	return f
}

type ObjBoundFunction struct {
	ObjObject
	Target value.Value
	This   value.Value
	Args   []value.Value
}

func NewBoundFunction(target, this value.Value, args []value.Value) *ObjBoundFunction {
	b := &ObjBoundFunction{Target: target, This: this, Args: args}
	b.Members = map[string]ObjectValueEntry{}
	b.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return b
}

func (*ObjBoundFunction) String() string {
	return "function () { [bound] }"
}

func (b *ObjBoundFunction) GetReferencingValues() []value.Value {
	values := []value.Value{b.Target, b.This}
	values = append(values, b.Args...)
	values = append(values, b.ObjObject.GetReferencingValues()...)
	return values
}
