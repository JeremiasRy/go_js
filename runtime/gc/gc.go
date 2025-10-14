package gc

import (
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
	"go_js/vm"
)

type GarbageCollector struct {
	vm    *vm.VM
	roots []value.Value

	greyStack []object.Object
}

var gc = &GarbageCollector{}

func pushGrey(o object.Object) {
	gc.greyStack = append(gc.greyStack, o)
}

func popGrey() object.Object {
	if len(gc.greyStack) == 0 {
		return nil
	}
	o := gc.greyStack[0]
	gc.greyStack = gc.greyStack[1:]

	return o
}

func Init(vm *vm.VM, roots []value.Value) {
	gc.vm = vm
	gc.roots = roots
}

func Mark() {
	for _, v := range gc.roots {
		if v.IsObject() {
			obj, _ := allocator.GetObject(v.GetHandle())
			pushGrey(obj)
		}
	}

	current := popGrey()

	for current != nil {
		switch obj := current.(type) {
		case object.Iterable:
			{
				for _, v := range append(obj.Values(), obj.Keys()...) {
					if v.IsObject() {
						obj, _ := allocator.GetObject(v.GetHandle())
						pushGrey(obj)
					}
				}
			}
		case object.Callable:
			{
				for _, v := range obj.ValueChunk().Constants {
					if v.IsObject() {
						obj, _ := allocator.GetObject(v.GetHandle())
						pushGrey(obj)
					}
				}
			}
		}
		current.Mark()
		current = popGrey()
	}

	allocator.Sweep()
}
