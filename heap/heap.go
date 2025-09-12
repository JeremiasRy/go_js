package heap

import "go_js/object"

type Heap struct {
	objects  []object.Object
	freeList []uint32
}

func (heap *Heap) Allocate(o object.Object) uint32 {
	if len(heap.freeList) > 0 {
		ptr := heap.freeList[len(heap.freeList)-1]
		heap.freeList = heap.freeList[:len(heap.freeList)-1]
		heap.objects[ptr] = o

		return ptr
	}

	heap.objects = append(heap.objects, o)
	// add safeguards here for len(heap.objects) >= uint32.MAX
	ptr := uint32(len(heap.objects) - 1)

	return ptr
}

func (heap *Heap) GetObject(ptr uint32) object.Object {
	return heap.objects[ptr]
}

func NewHeap() *Heap {
	return &Heap{objects: []object.Object{}, freeList: []uint32{}}
}
