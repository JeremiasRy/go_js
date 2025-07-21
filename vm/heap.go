package vm

import (
	"go_js/object"
	"go_js/value"
)

type Heap struct {
	objects  []object.Object
	freeList []uint32

	strings map[string]object.ObjString
}

var HEAP *Heap = NewHeap()

func (heap *Heap) Allocate(object object.Object) uint32 {
	if len(HEAP.freeList) > 0 {
		register := HEAP.freeList[len(HEAP.freeList)-1]
		HEAP.freeList = HEAP.freeList[:len(HEAP.freeList)-1]
		HEAP.objects[register] = object
		return register
	}

	HEAP.objects = append(HEAP.objects, object)
	return uint32(len(HEAP.objects) - 1)
}

func (heap *Heap) GetObject(register uint32) object.Object {
	return HEAP.objects[register]
}

func (heap *Heap) AllocateString(str string) value.Value {
	if str, found := HEAP.strings[str]; found {
		register := HEAP.Allocate(str)
		return value.EncodeObject(register)
	}

	register := HEAP.Allocate(object.ObjString(str))
	return value.EncodeObject(register)

}

func NewHeap() *Heap {
	return &Heap{objects: []object.Object{}, freeList: []uint32{}, strings: map[string]object.ObjString{}}
}
