package vm

type Heap struct {
	objects  []Object
	freeList []uint32
}

func (heap *Heap) Allocate(object Object) uint32 {
	if len(heap.freeList) > 0 {
		register := heap.freeList[len(heap.freeList)-1]
		heap.freeList = heap.freeList[:len(heap.freeList)-1]
		heap.objects[register] = object
		return register
	}

	heap.objects = append(heap.objects, object)
	return uint32(len(heap.objects) - 1)
}

func NewHeap() *Heap {
	return &Heap{objects: []Object{}, freeList: []uint32{}}
}
