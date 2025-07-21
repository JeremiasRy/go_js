package heap

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

func Allocate(object object.Object) value.Value {
	if len(HEAP.freeList) > 0 {
		register := HEAP.freeList[len(HEAP.freeList)-1]
		HEAP.freeList = HEAP.freeList[:len(HEAP.freeList)-1]
		HEAP.objects[register] = object
		return value.EncodeObject(register)
	}

	HEAP.objects = append(HEAP.objects, object)
	return value.EncodeObject(uint32(len(HEAP.objects) - 1))
}

func GetObject(register uint32) object.Object {
	return HEAP.objects[register]
}

func NewHeap() *Heap {
	return &Heap{objects: []object.Object{}, freeList: []uint32{}, strings: map[string]object.ObjString{}}
}
