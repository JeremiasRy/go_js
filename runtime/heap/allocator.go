package heap

import (
	"fmt"
	"go_js/object"
	"go_js/value"
)

type Heap struct {
	heapVars       map[int][]value.Value
	heapScopeCount int

	handleLayer map[uint32]uint32
	handleCount uint32
	strings     map[string]uint32

	objects  []object.Object
	freeList map[uint32]struct{}

	// GC
	init                bool
	roots               []value.Value
	asyncWorkKeepAlive  map[uintptr][]value.Value
	greyStack           []object.Object
	gcCycleDeterminator uint32
}

var debug = false

var a = &Heap{
	heapVars:       map[int][]value.Value{},
	heapScopeCount: 0,
	handleLayer:    map[uint32]uint32{},
	handleCount:    0,
	strings:        map[string]uint32{},

	objects:  make([]object.Object, 0, 50),
	freeList: map[uint32]struct{}{},

	roots:               []value.Value{},
	asyncWorkKeepAlive:  map[uintptr][]value.Value{},
	greyStack:           []object.Object{},
	gcCycleDeterminator: 20,
	init:                false,
}

func ShouldRunGCCycle() bool {
	return a.shouldRunGCCycle()
}

func GetHeapVar(scope, slot int) value.Value {
	return a.getHeapVar(scope, slot)
}

func SetHeapVar(scope, slot int, v value.Value) {
	a.setHeapVar(scope, slot, v)
}

func DefineHeapVar(scope int, v value.Value) {
	a.defineHeapVar(scope, v)
}

func CreateHeapScope() (scope int) {
	scope = a.createHeapScope()
	return scope
}

func (a *Heap) shouldRunGCCycle() bool {
	return a.handleCount%uint32(a.gcCycleDeterminator) == 0
}

func (a *Heap) getHeapVar(scope, slot int) value.Value {
	return a.heapVars[scope][slot]
}

func (a *Heap) setHeapVar(scope, slot int, v value.Value) {
	a.heapVars[scope][slot] = v
}

func (a *Heap) defineHeapVar(scope int, v value.Value) {
	a.heapVars[scope] = append(a.heapVars[scope], v)
}

func (a *Heap) createHeapScope() (scope int) {
	a.heapScopeCount++
	a.heapVars[a.heapScopeCount] = []value.Value{}
	scope = a.heapScopeCount
	return scope
}

func (a *Heap) allocate(obj object.Object) (handle uint32) {
	if obj.Type() == object.OBJ_STRING {
		val := obj.String()
		if ptr, found := a.strings[val]; found {
			handle = a.handleCount
			a.handleLayer[handle] = ptr

			a.handleCount++
			return handle
		} else {
			ptr := a.heapAllocate(obj)
			a.strings[val] = ptr
			return a.allocate(obj)
		}
	}

	ptr := a.heapAllocate(obj)
	handle = a.handleCount

	a.handleLayer[handle] = ptr
	a.handleCount++
	return handle
}

func (a *Heap) heapAllocate(obj object.Object) (pointer uint32) {
	if len(a.freeList) > 0 {
		for key := range a.freeList {
			pointer = key
			break
		}

		delete(a.freeList, pointer)
		a.objects[pointer] = obj
		return pointer
	}

	a.objects = append(a.objects, obj)
	// add safeguards here for len(a.objects) >= uint32.MAX
	pointer = uint32(len(a.objects) - 1)
	return pointer
}

func (a *Heap) getObject(handle uint32) (object.Object, error) {
	if ptr, found := a.handleLayer[handle]; found {
		return a.objects[ptr], nil
	}
	return nil, fmt.Errorf("no pointer found for handle %d", handle)
}

func (a *Heap) getPointer(handle uint32) (pointer uint32) {
	pointer = a.handleLayer[handle]
	return pointer
}

func Allocate(obj object.Object) uint32 {
	return a.allocate(obj)
}

func GetObject(handle uint32) (obj object.Object, err error) {
	obj, err = a.getObject(handle)
	return obj, err
}

func GetPointer(handle uint32) uint32 {
	return a.getPointer(handle)
}

// GARBAGE COLLECTION
func markAndSweep(stackValues []value.Value) {
	if debug {
		fmt.Println("-- GC DEBUG --")
		fmt.Printf("Stack values %d\n", len(stackValues))
	}

	for _, v := range append(a.roots, stackValues...) {
		if v.IsObject() {
			obj, _ := GetObject(v.GetHandle())
			pushGrey(obj)
		}
	}

	for _, items := range a.asyncWorkKeepAlive {
		for _, v := range items {
			if v.IsObject() {
				obj, _ := GetObject(v.GetHandle())
				pushGrey(obj)
			}
		}
	}

	current := popGrey()

	for current != nil {
		for _, v := range current.GetReferencingValues() {
			obj, _ := GetObject(v.GetHandle())
			if fn, ok := obj.(*object.ObjFunction); ok && fn.HeapScope != object.NOT_IN_HEAP_SCOPE {
				for _, v := range a.heapVars[fn.HeapScope] {
					if v.IsObject() {
						obj, _ := GetObject(v.GetHandle())
						pushGrey(obj)
					}
				}
			}

			pushGrey(obj)
		}
		current.Mark()
		current = popGrey()
	}
	a.sweep()

	if debug {
		fmt.Println("-- GC DEBUG END--")
	}
}

func (a *Heap) sweep() {
	count := len(a.freeList)

	for i, obj := range a.objects {
		if obj.Marked() {
			obj.Clear()
		} else if _, found := a.freeList[uint32(i)]; !found {
			if debug {
				fmt.Println("Cleaning up:", obj.String())
			}
			a.freeList[uint32(i)] = struct{}{}
			if fn, ok := obj.(*object.ObjFunction); ok && fn.HeapScope != object.NOT_IN_HEAP_SCOPE {
				if debug {
					fmt.Println("removing heap scope", fn.HeapScope)
				}
				delete(a.heapVars, fn.HeapScope)
			}
		}
	}

	if debug {
		fmt.Printf("cleaned up %d objects #####\n", len(a.freeList)-count)
	}

	if len(a.freeList)-count == 0 {
		if debug {
			fmt.Println("Increasing GC CYCLE", a.gcCycleDeterminator*2)
		}
		a.gcCycleDeterminator *= 2
	} else {
		if debug {
			fmt.Println("Decreasing GC CYCLE", a.gcCycleDeterminator/2)
		}
		a.gcCycleDeterminator /= 2
	}

	if len(a.freeList) > 0 && len(a.objects)/len(a.freeList) > 1 {
		defragment()
	}
}
func pushGrey(o object.Object) {
	a.greyStack = append(a.greyStack, o)
}

func popGrey() object.Object {
	if len(a.greyStack) == 0 {
		return nil
	}
	o := a.greyStack[len(a.greyStack)-1]
	a.greyStack = a.greyStack[:len(a.greyStack)-1]

	return o
}

func defragment() {
	arr := make([]object.Object, 0, len(a.objects)-len(a.freeList))
	indexMap := map[uint32]uint32{}

	for current := range a.objects {
		if _, found := a.freeList[uint32(current)]; !found {
			arr = append(arr, a.objects[current])
			indexMap[uint32(current)] = uint32(len(arr) - 1)
		}
		current++
	}

	deleteMap := map[uint32]struct{}{}
	deleteStrings := map[string]struct{}{}

	for k, v := range a.handleLayer {
		if new, found := indexMap[v]; found {
			a.handleLayer[k] = new
		} else {
			deleteMap[k] = struct{}{}
		}
	}

	for k, v := range a.strings {
		if new, found := indexMap[v]; found {
			a.strings[k] = new
		} else {
			deleteStrings[k] = struct{}{}
		}
	}

	for k := range deleteMap {
		delete(a.handleLayer, k)
	}

	for k := range deleteStrings {
		delete(a.strings, k)
	}

	a.freeList = map[uint32]struct{}{}
	a.objects = arr
}

func RequestGC(stackValues []value.Value) {
	if !a.init {
		return
	}

	markAndSweep(stackValues)
}

func InitGC(roots []value.Value, d bool) {
	a.roots = append(a.roots, roots...)
	a.init = true
	debug = d
}

func PushToRoots(v ...value.Value) {
	a.roots = append(a.roots, v...)
}

func StoreAsyncFunctionStack(ptr uintptr, v []value.Value) {
	a.asyncWorkKeepAlive[ptr] = v
}
func ClearAsyncFunctionStack(ptr uintptr) {
	delete(a.asyncWorkKeepAlive, ptr)
}
