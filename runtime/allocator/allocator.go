package allocator

import (
	"fmt"
	"go_js/object"
)

var allocator = make(map[uint32]uint32)
var strings = make(map[string]uint32)
var handleCount uint32 = 0
var h = NewHeap()
var clean = false

func Allocate(obj object.Object) uint32 {
	if obj.Type() == object.OBJ_STRING {
		val := obj.String()
		if ptr, found := strings[val]; found {
			handle := handleCount
			allocator[handle] = ptr

			handleCount++
			return handle
		} else {
			ptr := h.Allocate(obj)
			strings[val] = ptr
			return Allocate(obj)
		}
	}

	ptr := h.Allocate(obj)
	handle := handleCount

	allocator[handle] = ptr
	handleCount++
	return handle
}

func GetObject(handle uint32) (object.Object, error) {
	if ptr, found := allocator[handle]; found {
		return h.GetObject(ptr), nil
	}
	return nil, fmt.Errorf("no pointer found for handle %d", handle)
}

// for loose comparison shenanigans
func GetPointer(handle uint32) uint32 {
	return allocator[handle]
}

type Heap struct {
	objects  []object.Object
	freeList map[uint32]struct{}
}

func (heap *Heap) Allocate(o object.Object) uint32 {
	if len(heap.freeList) > 0 {
		var ptr uint32

		for key := range heap.freeList {
			ptr = key
		}

		delete(heap.freeList, ptr)
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
	return &Heap{objects: []object.Object{}, freeList: map[uint32]struct{}{}}
}

func (heap *Heap) Sweep() {
	for i, obj := range heap.objects {
		if obj.Marked() {
			obj.Clear()
		} else if _, found := heap.freeList[uint32(i)]; !found {
			heap.freeList[uint32(i)] = struct{}{}
		}
	}
	defragment()
}

func defragment() {
	arr := make([]object.Object, 0, len(h.objects)-len(h.freeList))
	indexMap := map[uint32]uint32{}

	for current := range h.objects {
		if _, found := h.freeList[uint32(current)]; !found {
			arr = append(arr, h.objects[current])
			indexMap[uint32(current)] = uint32(len(arr) - 1)
		}
		current++
	}

	deleteMap := map[uint32]struct{}{}

	for k, v := range allocator {
		if new, found := indexMap[v]; found {
			allocator[k] = new
		} else {
			deleteMap[k] = struct{}{}
		}
	}

	for k := range deleteMap {
		delete(allocator, k)
	}

	h.freeList = map[uint32]struct{}{}
	h.objects = arr
}
