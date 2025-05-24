package vm

import (
	"fmt"
	"unsafe"
)

type ObjType uint8
type ObjLike interface {
	Obj | ObjFunction | ObjString
}
type Obj struct {
	_type ObjType
}

const (
	OBJ_FUNCTION ObjType = iota
	OBJ_STRING
	OBJ_NUMBER
)

type ObjFunction struct {
	obj   Obj
	name  string
	chunk *Chunk
}

type ObjString struct {
	obj Obj
	s   string
}

func NewFunction(name string, chunk *Chunk) *ObjFunction {
	return &ObjFunction{
		obj:   Obj{_type: OBJ_FUNCTION},
		chunk: chunk,
		name:  name,
	}
}

func (objStr *ObjString) Encode() (Value, error) {
	address := Value(uintptr(unsafe.Pointer(objStr)))

	if address & ^ENCODE_MASK != 0 {
		return 0, fmt.Errorf("encodePointer: pointer address 0x%x too large for NaN boxing", address)
	}

	return TAG_OBJ | (address & ENCODE_MASK), nil
}

func (objStr *ObjString) Debug() {
	fmt.Printf("%s\n", objStr.s)
}

func (fn *ObjFunction) Debug() {
	fmt.Printf("<fn %s>\n", fn.name)
	fn.chunk.PrintCode()
}
