package heap

import (
	"go_js/object"
)

type Heap struct {
	objects  []object.Object
	strings  map[string]uint32
	freeList []uint32

	indirectionLayer map[uint32]uint32
	handleCount      uint32
}

var heap *Heap = NewHeap()

func Allocate(object object.Object) uint32 {
	if len(heap.freeList) > 0 {
		ptr := heap.freeList[len(heap.freeList)-1]
		heap.freeList = heap.freeList[:len(heap.freeList)-1]
		heap.objects[ptr] = object

		handle := heap.handleCount
		heap.handleCount++
		heap.indirectionLayer[handle] = ptr

		return handle
	}

	heap.objects = append(heap.objects, object)
	// add safeguards here for len(heap.objects) >= uint32.MAX
	ptr := uint32(len(heap.objects) - 1)
	handle := heap.handleCount
	heap.handleCount++

	heap.indirectionLayer[handle] = ptr
	return handle
}

func AllocateString(object object.ObjString) uint32 {
	if handle, found := heap.strings[object.String()]; found {
		return handle
	}
	handle := Allocate(object)
	heap.strings[object.String()] = handle
	return handle
}

func GetObject(handle uint32) object.Object {
	if ptr, found := heap.indirectionLayer[handle]; found {
		return heap.objects[ptr]
	}
	return nil
}

func NewHeap() *Heap {
	return &Heap{objects: []object.Object{}, freeList: []uint32{}, strings: map[string]uint32{}, handleCount: 0, indirectionLayer: map[uint32]uint32{}}
}
