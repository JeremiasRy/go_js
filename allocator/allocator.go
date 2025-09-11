package allocator

import (
	"fmt"
	"go_js/heap"
	"go_js/object"
)

var allocator = make(map[uint32]uint32)
var strings = make(map[object.ObjString]uint32)
var handleCount uint32 = 0
var h = heap.NewHeap()

func Allocate(obj object.Object) uint32 {
	if obj, ok := obj.(object.ObjString); ok {
		if ptr, found := strings[obj]; found {
			handle := handleCount
			allocator[handle] = ptr

			handleCount++
			return handle
		} else {
			ptr := h.Allocate(obj)
			strings[obj] = ptr
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
