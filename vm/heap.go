package vm

type Heap struct {
	objects  []Object
	freeList []uint32

	strings map[string]ObjString
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

func (heap *Heap) GetObject(register uint32) Object {
	return heap.objects[register]
}

func (heap *Heap) AllocateString(str string) Value {
	if str, found := heap.strings[str]; found {
		register := heap.Allocate(str)
		return EncodeObject(register)
	}

	register := heap.Allocate(ObjString(str))
	return EncodeObject(register)

}

func NewHeap() *Heap {
	return &Heap{objects: []Object{}, freeList: []uint32{}, strings: map[string]ObjString{}}
}
