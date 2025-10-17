package allocator

import (
	"fmt"

	"go_js/object"
	"go_js/value"
	"runtime"
)

type GarbageCollector struct {
	roots     []value.Value
	greyStack []object.Object
	stats     *runtime.MemStats
}

var gc = &GarbageCollector{stats: &runtime.MemStats{}}
var debug = false

func pushGrey(o object.Object) {
	gc.greyStack = append(gc.greyStack, o)
}

func popGrey() object.Object {
	if len(gc.greyStack) == 0 {
		return nil
	}
	o := gc.greyStack[len(gc.greyStack)-1]
	gc.greyStack = gc.greyStack[:len(gc.greyStack)-1]

	return o
}

func markAndSweep(stackValues []value.Value) {
	if debug {
		fmt.Println("-- GC DEBUG --")
	}

	for _, v := range append(gc.roots, stackValues...) {
		if v.IsObject() {
			obj, _ := GetObject(v.GetHandle())
			pushGrey(obj)
		}
	}

	current := popGrey()

	for current != nil {
		for _, v := range current.GetReferencingValues() {
			obj, _ := GetObject(v.GetHandle())
			pushGrey(obj)
		}
		current.Mark()
		current = popGrey()
	}
	h.sweep()

	if debug {
		fmt.Println("-- GC DEBUG END--")
	}
}

func (heap *Heap) sweep() {
	count := len(heap.freeList)
	for i, obj := range heap.objects {
		if obj.Marked() {
			obj.Clear()
		} else if _, found := heap.freeList[uint32(i)]; !found {
			heap.freeList[uint32(i)] = struct{}{}
		}
	}
	if debug {
		fmt.Printf("Cleaned up %d objects\n", len(heap.freeList)-count)
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
	deleteStrings := map[string]struct{}{}

	for k, v := range allocator {
		if new, found := indexMap[v]; found {
			allocator[k] = new
		} else {
			deleteMap[k] = struct{}{}
		}
	}

	for k, v := range strings {
		if new, found := indexMap[v]; found {
			strings[k] = new
		} else {
			deleteStrings[k] = struct{}{}
		}
	}

	for k := range deleteMap {
		delete(allocator, k)
	}

	for k := range deleteStrings {
		delete(strings, k)
	}

	h.freeList = map[uint32]struct{}{}
	h.objects = arr
}

func RequestGC(stackValues []value.Value) {
	if !clean {
		return
	}

	markAndSweep(stackValues)
}

func InitGC(roots []value.Value, d bool) {
	gc.roots = append(gc.roots, roots...)
	clean = true
	debug = d
}

func PushToRoots(v ...value.Value) {
	gc.roots = append(gc.roots, v...)
}
