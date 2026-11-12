package heap

import (
	"fmt"
	"go_js/flags"
	"go_js/object"
	"go_js/value"
)

const (
	defragMinHeapSlots   = 128
	defragMinFreeSlots   = 64
	defragFreePercentage = 50
)

type HeapScope struct {
	Parent int
	Values []value.Value
}

type Heap struct {
	heapVars       map[int]*HeapScope
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
	asyncScopeKeepAlive map[uintptr]int
	callbackRoots       map[uintptr][]value.Value
	nextCallbackRoot    uintptr
	greyStack           []object.Object
	gcCycleDeterminator uint32
}

var a = &Heap{
	heapVars:       map[int]*HeapScope{},
	heapScopeCount: 0,
	handleLayer:    map[uint32]uint32{},
	handleCount:    0,
	strings:        map[string]uint32{},

	objects:  make([]object.Object, 0, 50),
	freeList: map[uint32]struct{}{},

	roots:               []value.Value{},
	asyncWorkKeepAlive:  map[uintptr][]value.Value{},
	asyncScopeKeepAlive: map[uintptr]int{},
	callbackRoots:       map[uintptr][]value.Value{},
	nextCallbackRoot:    1,
	greyStack:           []object.Object{},
	gcCycleDeterminator: 20,
	init:                false,
}

func ShouldRunGCCycle() bool {
	return a.shouldRunGCCycle()
}

func GetHeapVar(scope, depth, slot int) value.Value {
	return a.getHeapVar(scope, depth, slot)
}

func HasHeapVar(scope, depth, slot int) bool {
	scope = a.resolveScope(scope, depth)
	values, found := a.heapVars[scope]
	return found && slot >= 0 && slot < len(values.Values)
}

func SetHeapVar(scope, depth, slot int, v value.Value) {
	a.setHeapVar(scope, depth, slot, v)
}

func DefineHeapVar(scope int, v value.Value) {
	a.defineHeapVar(scope, v)
}

func DefineHeapVarAt(scope, slot int, v value.Value) {
	a.defineHeapVarAt(scope, slot, v)
}

func CreateHeapScope(parent int) (scope int) {
	scope = a.createHeapScope(parent)
	return scope
}

func ScopeValues(scope int) []value.Value {
	if s, found := a.heapVars[scope]; found {
		return s.Values
	}
	return nil
}

func ParentScope(scope int) int {
	if s, found := a.heapVars[scope]; found {
		return s.Parent
	}
	return 0
}

func CopyHeapScope(old int) (scope int) {
	scope = a.createHeapScope(0)
	if old > 0 {
		values := a.heapVars[old]
		copied := make([]value.Value, len(values.Values))
		copy(copied, values.Values)
		a.heapVars[scope] = &HeapScope{Parent: values.Parent, Values: copied}
	}
	return scope
}

func (a *Heap) resolveScope(scope, depth int) int {
	for i := 0; i < depth && scope > 0; i++ {
		scope = a.heapVars[scope].Parent
	}
	return scope
}

func (a *Heap) shouldRunGCCycle() bool {
	if a.gcCycleDeterminator <= 0 {
		a.gcCycleDeterminator = 1
	}
	return a.handleCount%a.gcCycleDeterminator == 0
}

func (a *Heap) getHeapVar(scope, depth, slot int) value.Value {
	scope = a.resolveScope(scope, depth)
	values := a.heapVars[scope]
	if values == nil || slot < 0 || slot >= len(values.Values) {
		return value.UNINITIALIZED
	}
	return values.Values[slot]
}

func (a *Heap) setHeapVar(scope, depth, slot int, v value.Value) {
	scope = a.resolveScope(scope, depth)
	for len(a.heapVars[scope].Values) <= slot {
		a.heapVars[scope].Values = append(a.heapVars[scope].Values, value.UNINITIALIZED)
	}
	a.heapVars[scope].Values[slot] = v
}

func (a *Heap) defineHeapVar(scope int, v value.Value) {
	a.heapVars[scope].Values = append(a.heapVars[scope].Values, v)
}

func (a *Heap) defineHeapVarAt(scope, slot int, v value.Value) {
	for len(a.heapVars[scope].Values) <= slot {
		a.heapVars[scope].Values = append(a.heapVars[scope].Values, value.UNINITIALIZED)
	}
	a.heapVars[scope].Values[slot] = v
}

func (a *Heap) createHeapScope(parent int) (scope int) {
	a.heapScopeCount++
	a.heapVars[a.heapScopeCount] = &HeapScope{Parent: parent}
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
func markValue(v value.Value) {
	if !v.IsObject() {
		return
	}
	obj, err := GetObject(v.GetHandle())
	if err != nil {
		return
	}
	pushGrey(obj)
}

func (a *Heap) markScope(scope int) {
	for scope > 0 {
		values, found := a.heapVars[scope]
		if !found {
			return
		}
		for _, v := range values.Values {
			markValue(v)
		}
		scope = values.Parent
	}
}

func markAndSweep(stackValues []value.Value) {
	if flags.Debug {
		fmt.Println("-- GC DEBUG --")
		fmt.Printf("Stack values %d\n", len(stackValues))
	}

	for _, v := range append(a.roots, stackValues...) {
		markValue(v)
	}

	for _, items := range a.asyncWorkKeepAlive {
		for _, v := range items {
			markValue(v)
		}
	}

	for _, scope := range a.asyncScopeKeepAlive {
		a.markScope(scope)
	}

	for _, items := range a.callbackRoots {
		for _, v := range items {
			markValue(v)
		}
	}

	current := popGrey()

	for current != nil {
		if current.Marked() {
			current = popGrey()
			continue
		}
		current.Mark()
		for _, v := range current.GetReferencingValues() {
			markValue(v)
		}
		if callable, ok := current.(object.Callable); ok && callable.GetHeapScope() != object.NOT_IN_HEAP_SCOPE {
			a.markScope(callable.GetHeapScope())
		}
		current = popGrey()
	}
	a.sweep()

	if flags.Debug {
		fmt.Println("-- GC DEBUG END--")
	}
}

func (a *Heap) sweep() {
	count := len(a.freeList)

	for i, obj := range a.objects {
		if obj.Marked() {
			obj.Clear()
		} else if _, found := a.freeList[uint32(i)]; !found {
			if flags.Debug {
				fmt.Println("Cleaning up:", obj.String())
			}
			a.freeList[uint32(i)] = struct{}{}
			if obj.Type() == object.OBJ_STRING {
				str := obj.String()
				if ptr, found := a.strings[str]; found && ptr == uint32(i) {
					delete(a.strings, str)
				}
			}
		}
	}

	if flags.Debug {
		fmt.Printf("cleaned up %d objects #####\n", len(a.freeList)-count)
	}

	if len(a.freeList)-count == 0 {
		if flags.Debug {
			fmt.Println("Increasing GC CYCLE", a.gcCycleDeterminator*2)
		}
		a.gcCycleDeterminator *= 2
	} else {
		if flags.Debug {
			fmt.Println("Decreasing GC CYCLE", a.gcCycleDeterminator/2)
		}
		a.gcCycleDeterminator /= 2
	}

	if a.shouldDefragment() {
		if flags.Debug {
			fmt.Printf("Defragmenting heap: %d objects, %d free slots\n", len(a.objects), len(a.freeList))
		}
		a.defragment()
	}
}

func (a *Heap) shouldDefragment() bool {
	objects := len(a.objects)
	free := len(a.freeList)
	if objects < defragMinHeapSlots || free < defragMinFreeSlots {
		return false
	}
	return free*100 >= objects*defragFreePercentage
}

func (a *Heap) defragment() {
	if len(a.objects) == 0 || len(a.freeList) == 0 {
		return
	}

	remap := make([]uint32, len(a.objects))
	write := 0
	for read, obj := range a.objects {
		if _, free := a.freeList[uint32(read)]; free {
			continue
		}
		remap[read] = uint32(write + 1)
		a.objects[write] = obj
		write++
	}

	for handle, pointer := range a.handleLayer {
		if int(pointer) >= len(remap) || remap[pointer] == 0 {
			delete(a.handleLayer, handle)
			continue
		}
		a.handleLayer[handle] = remap[pointer] - 1
	}

	for str, pointer := range a.strings {
		if int(pointer) >= len(remap) || remap[pointer] == 0 {
			delete(a.strings, str)
			continue
		}
		a.strings[str] = remap[pointer] - 1
	}

	clear(a.objects[write:])
	a.objects = a.objects[:write]
	a.freeList = make(map[uint32]struct{})
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

func RequestGC(stackValues []value.Value) {
	if !a.init {
		return
	}

	markAndSweep(stackValues)
}

func InitGC(roots []value.Value) {
	a.roots = append(a.roots, roots...)
	a.init = true
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

func StoreAsyncFunctionScope(ptr uintptr, scope int) {
	a.asyncScopeKeepAlive[ptr] = scope
}
func ClearAsyncFunctionScope(ptr uintptr) {
	delete(a.asyncScopeKeepAlive, ptr)
}

func NewCallbackRoot() uintptr {
	key := a.nextCallbackRoot
	a.nextCallbackRoot++
	return key
}

func StoreCallbackRoot(key uintptr, values ...value.Value) {
	a.callbackRoots[key] = values
}

func ClearCallbackRoot(key uintptr) {
	delete(a.callbackRoots, key)
}

func EncodeObject(obj object.Object) value.Value {
	for handle, ptr := range a.handleLayer {
		if a.objects[ptr] == obj {
			return value.EncodeHandle(handle)
		}
	}
	return value.EncodeHandle(a.allocate(obj))
}
