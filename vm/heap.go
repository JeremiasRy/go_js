package vm

type Heap struct {
	objects  []Object
	freeList []uint32

	strings map[string]ObjString
}

func (heap *Heap) Allocate(object Object) uint32 {
	if len(HEAP.freeList) > 0 {
		register := HEAP.freeList[len(HEAP.freeList)-1]
		HEAP.freeList = HEAP.freeList[:len(HEAP.freeList)-1]
		HEAP.objects[register] = object
		return register
	}

	HEAP.objects = append(HEAP.objects, object)
	return uint32(len(HEAP.objects) - 1)
}

func (heap *Heap) GetObject(register uint32) Object {
	return HEAP.objects[register]
}

func (heap *Heap) AllocateString(str string) Value {
	if str, found := HEAP.strings[str]; found {
		register := HEAP.Allocate(str)
		return EncodeObject(register)
	}

	register := HEAP.Allocate(ObjString(str))
	return EncodeObject(register)

}

func NewHeap() *Heap {
	return &Heap{objects: []Object{}, freeList: []uint32{}, strings: map[string]ObjString{}}
}
