package native

import (
	"math/big"

	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

type ObjBigInt struct {
	ObjObject
	Value *big.Int
}

func NewBigInt(v *big.Int) *ObjBigInt {
	obj := &ObjBigInt{Value: new(big.Int).Set(v)}
	obj.Members = map[string]ObjectValueEntry{}
	obj.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("toString"))), value.EncodeHandle(heap.Allocate(NewBigIntToString())))
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("valueOf"))), value.EncodeHandle(heap.Allocate(NewBigIntValueOf())))
	return obj
}

func (*ObjBigInt) Type() object.ObjType {
	return object.OBJ_BIGINT
}

func (b *ObjBigInt) String() string {
	return b.Value.String()
}

type BigIntToString struct {
	ObjNativeFn
}

func NewBigIntToString() *BigIntToString {
	fn := &BigIntToString{}
	fn.name = "toString"
	return fn
}

type BigIntValueOf struct {
	ObjNativeFn
}

func NewBigIntValueOf() *BigIntValueOf {
	fn := &BigIntValueOf{}
	fn.name = "valueOf"
	return fn
}

type BigIntConstructor struct {
	ObjObject
}

func NewBigIntConstructor() *BigIntConstructor {
	obj := &BigIntConstructor{}
	obj.Members = map[string]ObjectValueEntry{}
	obj.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("asIntN"))), value.EncodeHandle(heap.Allocate(NewBigIntAsIntN())))
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("asUintN"))), value.EncodeHandle(heap.Allocate(NewBigIntAsUintN())))
	return obj
}

func (*BigIntConstructor) Type() object.ObjType {
	return object.OBJ_NATIVE_FN
}

func (*BigIntConstructor) String() string {
	return "function BigInt"
}

type BigIntAsIntN struct {
	ObjNativeFn
}

func NewBigIntAsIntN() *BigIntAsIntN {
	fn := &BigIntAsIntN{}
	fn.name = "asIntN"
	return fn
}

type BigIntAsUintN struct {
	ObjNativeFn
}

func NewBigIntAsUintN() *BigIntAsUintN {
	fn := &BigIntAsUintN{}
	fn.name = "asUintN"
	return fn
}
