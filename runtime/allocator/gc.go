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
	for _, val := range v {
		o, _ := GetObject(val.GetHandle())

		fmt.Println(o.String())
	}
	gc.roots = append(gc.roots, v...)
}
