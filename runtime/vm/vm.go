package vm

import (
	"fmt"
	"go_js/chunk"
	"go_js/compiler"
	"go_js/diagnostic"
	"go_js/eventloop"
	"go_js/flags"
	"go_js/heap"
	"go_js/jit"
	"go_js/native"
	"go_js/object"
	"go_js/queue"
	"go_js/value"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unsafe"
)

const STACK_MAX = math.MaxUint8
const FRAMES_MAX = 64
const IS_HOT_PATH = 0

var ROOT_SCRIPT_LOCATION string
var globals = make([]value.Value, 30)
var globalsCount = 0
var globalVarSlots = map[string]int{}
var globalVarNames = map[int]string{}
var imports = make(map[string]value.Value)
var scriptMode = true

func SetScriptMode(isScript bool) {
	scriptMode = isScript
}

var sharedGlobalObject value.Value

func (vm *VM) ensureGlobalObject() value.Value {
	if sharedGlobalObject == 0 {
		sharedGlobalObject = value.EncodeHandle(heap.Allocate(native.NewGlobalObject()))
	}
	vm.globalObject = sharedGlobalObject
	return sharedGlobalObject
}

type CallFrame struct {
	fn         object.Callable
	thisCtx    value.Value
	localStart int
	returnIp   int
	heapScope  int
	ownScope   bool
	newTarget  value.Value
	arguments  value.Value
	startIp    int
}

func InitFileRoot(path string) {
	ROOT_SCRIPT_LOCATION = path
}

func (cf *CallFrame) initCallFrame(fn object.Callable, localStart int, returnIp int, this value.Value) {
	cf.fn = fn
	cf.localStart = localStart
	cf.returnIp = returnIp
	cf.thisCtx = this
	cf.heapScope = fn.GetHeapScope()
	cf.ownScope = false
	cf.newTarget = value.UNDEFINED
	cf.startIp = 0
}

type JSException struct {
	Name    string
	Message string
}

func (e *JSException) Error() string {
	return e.Message
}

type ExceptionState struct {
	stackTop  int
	jumpTo    int
	frame     int
	isFinally bool
}

type VM struct {
	frames         []CallFrame
	frameCount     int
	stack          []value.Value
	stackTop       int
	exceptionStack []ExceptionState
	callCounts     map[object.Callable]int
	output         *strings.Builder

	exports *native.ObjObject

	completionValue   value.Value
	completionPending bool
	globalObject      value.Value
	catchValues       map[int]value.Value

	main          bool
	loopMarks     map[int][]int
	loopIterators map[int]value.Value
}

func NewVM(main bool, output *strings.Builder) *VM {
	frames := make([]CallFrame, FRAMES_MAX)
	stack := make([]value.Value, STACK_MAX)

	return &VM{
		output:         output,
		frames:         frames,
		frameCount:     0,
		stack:          stack,
		stackTop:       0,
		exceptionStack: []ExceptionState{},
		main:           main,
		callCounts:     map[object.Callable]int{},
		catchValues:    map[int]value.Value{},
		loopMarks:      map[int][]int{},
		loopIterators:  map[int]value.Value{},
	}
}

func (vm *VM) currentFrame() CallFrame {
	return vm.frames[vm.frameCount-1]
}

func (vm *VM) push(v value.Value) {
	vm.stack[vm.stackTop] = v
	vm.stackTop++
}

func (vm *VM) peek() value.Value {
	return vm.stack[vm.stackTop-1]
}

func (vm *VM) pop() value.Value {
	vm.stackTop--
	return vm.stack[vm.stackTop]
}

func (vm *VM) popUnwrapped() value.Value {
	v := vm.pop()
	if isMethodHandle(v) {
		v = vm.pop()
		vm.pop()
	}
	return v
}

func (vm *VM) addGlobal(v value.Value) {
	if globalsCount >= len(globals) {
		globals = append(globals, v)
	} else {
		globals[globalsCount] = v
	}
	globalsCount++
}

func (vm *VM) getGlobal(global int) value.Value {
	return globals[global]
}

func (vm *VM) concatenate(a, b value.Value) value.Value {
	if !a.IsObject() && b.IsObject() {
		a = value.EncodeHandle(heap.Allocate(native.NewLightString(native.String(a))))
	}

	if a.IsObject() {
		var bObj object.Object
		if !b.IsObject() {
			bObj = native.NewLightString(native.String(b))
		}
		aObj, _ := heap.GetObject(a.GetHandle())

		if bObj == nil {
			bObj, _ = heap.GetObject(b.GetHandle())
		}

		switch aObj := aObj.(type) {
		case *native.ObjStringBuilder:
			{
				switch str := bObj.(type) {
				case *native.ObjStringBuilder:
					{
						aObj.Concatenate(str.Flush().String())
					}
				default:
					{
						aObj.Concatenate(bObj.String())
					}
				}
				return a
			}
		case *native.LightString:
			{
				builder := native.NewStringBuilder(aObj)
				switch str := bObj.(type) {
				case *native.ObjStringBuilder:
					{
						builder.Concatenate(str.Flush().String())
					}
				default:
					{
						builder.Concatenate(bObj.String())
					}
				}

				return value.EncodeHandle(heap.Allocate(builder))
			}
		case *native.ObjString:
			{
				switch str := bObj.(type) {
				case *native.ObjStringBuilder:
					{
						return value.EncodeHandle(heap.Allocate(native.NewLightString(aObj.String() + str.Flush().String())))
					}
				default:
					{
						return value.EncodeHandle(heap.Allocate(native.NewLightString(aObj.String() + bObj.String())))
					}
				}
			}
		}
	}

	return value.Value(math.Float64bits(a.AsNumber() + b.AsNumber()))
}

func (vm *VM) subtract(a, b value.Value) value.Value {
	aIsObject, _ := object.IsValueObject(a)
	bIsObject, _ := object.IsValueObject(b)
	if aIsObject || bIsObject {
		if aIsObject && bIsObject {
			// todo
		} else {
			return value.STANDARD_NAN
		}
	}

	return value.ValueFromFloat64(a.AsNumber() - b.AsNumber())
}

// ORDER IS IMPORTANT! thanks.
func compBoolToNum(boolean value.Value, number value.Value) bool {
	if !boolean.IsBoolean() {
		panic("you didnt listen")
	}
	cmpTo := 0.0

	if truthyValue(boolean) {
		cmpTo = 1.0
	}

	return number.AsNumber() == cmpTo
}

func compNumToObject(num value.Value, obj object.Object) bool {
	numString := strconv.FormatFloat(num.AsNumber(), 'f', -1, 64)
	switch obj := obj.(type) {
	case *native.LightString:
		return numString == obj.String()
	case *native.ObjString:
		return numString == obj.String()
	case *native.ObjStringBuilder:
		return numString == obj.String()
	default:
		return numString == native.ObjToPrimitive(obj)
	}
}

func compBoolToObject(boolean value.Value, obj object.Object) bool {
	cmpTo := 0.0

	if truthyValue(boolean) {
		cmpTo = 1.0
	}

	str := ""

	switch obj := obj.(type) {
	case *native.LightString:
		str = obj.String()
	case *native.ObjString:
		str = obj.String()
	case *native.ObjStringBuilder:
		str = obj.String()
	default:
		str = native.ObjToPrimitive(obj)
	}

	if len(str) == 0 {
		str = "0"
	}

	res, err := strconv.ParseFloat(str, 64)

	if err != nil {
		fmt.Println(err)
		return false
	}

	return res == cmpTo
}

func compStringToObject(str object.Object, obj object.Object) bool {
	if obj.Type() == object.OBJ_STRING_BUILDER {
		return str.String() == obj.String()
	}

	return str.String() == native.ObjToPrimitive(obj)
}

func looseComparison(a, b value.Value) bool {
	if a.IsBoolean() && b.IsBoolean() {
		return a == b
	}

	if a.IsNumber() && b.IsBoolean() {
		return compBoolToNum(b, a)
	}

	if a.IsBoolean() && b.IsNumber() {
		return compBoolToNum(a, b)
	}

	if a.IsObject() && b.IsObject() {
		aObj, _ := heap.GetObject(a.GetHandle())
		bObj, _ := heap.GetObject(b.GetHandle())

		if (aObj.Type() == object.OBJ_STRING) && bObj.Type() == object.OBJ_STRING {
			return aObj.String() == bObj.String()
		}

		if aObj.Type() == object.OBJ_STRING && bObj.Type() != object.OBJ_STRING {
			return compStringToObject(aObj, bObj)
		}

		if aObj.Type() != object.OBJ_STRING && bObj.Type() == object.OBJ_STRING {
			return compStringToObject(bObj, aObj)
		}

		return heap.GetPointer(a.GetHandle()) == heap.GetPointer(b.GetHandle())
	}

	if (a == value.NULL || a == value.UNDEFINED) && (b == value.NULL || b == value.UNDEFINED) {
		return true
	}

	if a.IsObject() == !b.IsObject() {
		obj, _ := heap.GetObject(a.GetHandle())
		if b.IsNumber() {
			return compNumToObject(b, obj)
		}

		if b.IsBoolean() {
			return compBoolToObject(b, obj)
		}
	}

	if !a.IsObject() == b.IsObject() {
		bObj, _ := heap.GetObject(b.GetHandle())

		if a.IsNumber() {
			return compNumToObject(a, bObj)
		}

		if a.IsBoolean() {
			return compBoolToObject(a, bObj)
		}
	}

	return truthyValue(a) == truthyValue(b)
}

func truthyValue(v value.Value) bool {
	if v.IsObject() {
		obj, _ := heap.GetObject(v.GetHandle())

		switch obj := obj.(type) {
		case *native.ObjString:
			return len(obj.Value) > 0
		case *native.LightString, *native.StringConstructor:
			return len(obj.String()) > 0
		case *native.ObjBigInt:
			return obj.Value.Sign() != 0
		default:
			return true
		}
	}

	if v.IsType(value.NULL) || v.IsType(value.UNDEFINED) || v.IsType(value.UNINITIALIZED) {
		return false
	}

	if v.IsNumber() {
		n := v.AsNumber()
		return n != 0 && !math.IsNaN(n)
	}

	return value.TRUE == v
}

func (vm *VM) normalizeThis(fn object.Callable, this value.Value) value.Value {
	if this == value.UNDEFINED || this == value.NULL {
		if fnObj, ok := fn.(*object.ObjFunction); ok && fnObj.Strict {
			return this
		}
		if vm.main && scriptMode {
			return vm.ensureGlobalObject()
		}
	}
	return this
}

func (vm *VM) arrayIndexFromValue(v value.Value) (int, bool) {
	if v.IsInteger() {
		n := v.AsNumber()
		if n >= 0 && n < 4294967295 {
			return int(n), true
		}
	}
	if s, ok := vm.valueAsString(v); ok && s != "" {
		if s == "0" {
			return 0, true
		}
		if s[0] == '0' {
			return 0, false
		}
		n := 0
		for i := 0; i < len(s); i++ {
			if s[i] < '0' || s[i] > '9' {
				return 0, false
			}
			n = n*10 + int(s[i]-'0')
			if n >= 4294967295 {
				return 0, false
			}
		}
		return n, true
	}
	return 0, false
}

func (vm *VM) acquireIterator(iteratee value.Value, iteratorType uint8) (value.Value, error) {
	if !iteratee.IsObject() {
		return value.UNDEFINED, fmt.Errorf("%s is not iterable", native.String(iteratee))
	}
	obj, err := heap.GetObject(iteratee.GetHandle())
	if err != nil {
		return value.UNDEFINED, err
	}
	if _, ok := obj.(*object.Iterator); ok {
		return iteratee, nil
	}
	if _, ok := obj.(*native.JSIterator); ok {
		return iteratee, nil
	}
	if _, ok := obj.(*native.ObjGenerator); ok {
		return iteratee, nil
	}
	if iteratorType == compiler.ITERATOR_FOR_IN {
		if iterable, ok := obj.(object.Iterable); ok {
			return value.EncodeHandle(heap.Allocate(object.NewKeyIterator(iterable))), nil
		}
		return value.UNDEFINED, fmt.Errorf("%s is not iterable", native.String(iteratee))
	}
	if collection, ok := obj.(*native.Map); ok {
		return value.EncodeHandle(heap.Allocate(object.NewLiveIterator(collection.Entries))), nil
	}
	if collection, ok := obj.(*native.Set); ok {
		return value.EncodeHandle(heap.Allocate(object.NewLiveIterator(collection.Values))), nil
	}
	if native.SYMBOL_ITERATOR != 0 {
		method, err := vm.getMemberValue(iteratee, native.SYMBOL_ITERATOR)
		if err == nil && method != value.UNDEFINED {
			if methodObj, err := heap.GetObject(method.GetHandle()); err == nil {
				if _, ok := methodObj.(*native.ArrayIterator); ok {
					if arr, ok := obj.(*native.ObjArr); ok {
						return value.EncodeHandle(heap.Allocate(object.NewValueIterator(arr))), nil
					}
				}
			}
			iterator, err := vm.callValue(method, iteratee, nil)
			if err != nil {
				return value.UNDEFINED, err
			}
			if !iterator.IsObject() {
				return value.UNDEFINED, fmt.Errorf("Result of the Symbol.iterator method is not an object")
			}
			if iteratorObj, err := heap.GetObject(iterator.GetHandle()); err == nil {
				switch iteratorObj.(type) {
				case *object.Iterator, *native.ObjGenerator:
					return iterator, nil
				}
			}
			next, err := vm.getMemberValue(iterator, value.EncodeHandle(heap.Allocate(native.NewLightString("next"))))
			if err != nil {
				return value.UNDEFINED, err
			}
			jsIterator := native.NewJSIterator(iterator)
			jsIterator.NextMethod = next
			return value.EncodeHandle(heap.Allocate(jsIterator)), nil
		}
	}
	if s, ok := vm.valueAsString(iteratee); ok {
		items := []value.Value{}
		for _, r := range []rune(s) {
			items = append(items, value.EncodeHandle(heap.Allocate(native.NewLightString(string(r)))))
		}
		return value.EncodeHandle(heap.Allocate(object.NewValueIterator(native.NewArrayFrom(items)))), nil
	}
	if iterable, ok := obj.(object.Iterable); ok {
		switch obj.(type) {
		case *native.ObjArr, *native.Map, *native.Set:
			return value.EncodeHandle(heap.Allocate(object.NewValueIterator(iterable))), nil
		}
	}
	return value.UNDEFINED, fmt.Errorf("%s is not iterable", native.String(iteratee))
}

func (vm *VM) iteratorNext(iteratorValue value.Value) (bool, error) {
	obj, err := heap.GetObject(iteratorValue.GetHandle())
	if err != nil {
		return false, err
	}
	switch iterator := obj.(type) {
	case *object.Iterator:
		return iterator.Next(), nil
	case *native.JSIterator:
		if iterator.Done {
			return true, nil
		}
		result, err := vm.callValue(iterator.NextMethod, iterator.Iterator, nil)
		if err != nil {
			return false, err
		}
		if !result.IsObject() {
			return false, &JSException{Name: "TypeError", Message: "Iterator result is not an object"}
		}
		doneValue, err := vm.getMemberValue(result, value.EncodeHandle(heap.Allocate(native.NewLightString("done"))))
		if err != nil {
			return false, err
		}
		iterator.Done = truthyValue(doneValue)
		if !iterator.Done {
			valueResult, err := vm.getMemberValue(result, value.EncodeHandle(heap.Allocate(native.NewLightString("value"))))
			if err != nil {
				return false, err
			}
			iterator.Current = valueResult
		} else {
			iterator.Current = value.UNDEFINED
		}
		return iterator.Done, nil
	case *native.ObjGenerator:
		result, err := vm.generatorNext(iterator, nil)
		if err != nil {
			return false, err
		}
		if !result.IsObject() {
			return false, &JSException{Name: "TypeError", Message: "Iterator result is not an object"}
		}
		doneValue, err := vm.getMemberValue(result, value.EncodeHandle(heap.Allocate(native.NewLightString("done"))))
		if err != nil {
			return false, err
		}
		iterator.Done = truthyValue(doneValue)
		if !iterator.Done {
			valueResult, err := vm.getMemberValue(result, value.EncodeHandle(heap.Allocate(native.NewLightString("value"))))
			if err != nil {
				return false, err
			}
			iterator.Current = valueResult
		} else {
			iterator.Current = value.UNDEFINED
		}
		return iterator.Done, nil
	}
	return false, fmt.Errorf("not an iterator")
}

func (vm *VM) iteratorCurrent(iteratorValue value.Value) (value.Value, error) {
	obj, err := heap.GetObject(iteratorValue.GetHandle())
	if err != nil {
		return value.UNDEFINED, err
	}
	switch iterator := obj.(type) {
	case *object.Iterator:
		return iterator.Current(), nil
	case *native.JSIterator:
		return iterator.Current, nil
	case *native.ObjGenerator:
		return iterator.Current, nil
	}
	return value.UNDEFINED, fmt.Errorf("not an iterator")
}

func (vm *VM) closeIterator(iteratorValue value.Value) error {
	obj, err := heap.GetObject(iteratorValue.GetHandle())
	if err != nil {
		return err
	}
	if generator, ok := obj.(*native.ObjGenerator); ok {
		if generator.Done {
			return nil
		}
		_, err := vm.generatorResume(generator, value.UNDEFINED, generatorReturnMode)
		return err
	}
	jsIterator, ok := obj.(*native.JSIterator)
	if !ok || jsIterator.Done {
		return nil
	}
	returnMethod, err := vm.getMemberValue(jsIterator.Iterator, value.EncodeHandle(heap.Allocate(native.NewLightString("return"))))
	if err != nil || returnMethod == value.UNDEFINED {
		return nil
	}
	_, err = vm.callValue(returnMethod, jsIterator.Iterator, nil)
	return err
}

func keyValueFromString(key string) value.Value {
	if len(key) > 0 && key[0] == 0 {
		if symbol, found := native.SymbolByKey(key); found {
			return symbol
		}
	}
	return value.EncodeHandle(heap.Allocate(native.NewLightString(key)))
}

func cloneFunctionValue(v value.Value, scope int) value.Value {
	if !v.IsObject() {
		return v
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return v
	}
	if fn, ok := obj.(*object.ObjFunction); ok {
		if fn.Cloned {
			return v
		}
		clone := fn.Clone().(*object.ObjFunction)
		clone.Cloned = true
		if fn.GetHeapScope() == object.NOT_IN_HEAP_SCOPE {
			clone.SetHeapScope(scope)
		}
		return value.EncodeHandle(heap.Allocate(clone))
	}
	if fn, ok := obj.(*native.ObjAsyncFunction); ok {
		if fn.GetHeapScope() == object.NOT_IN_HEAP_SCOPE {
			clone := fn.Clone().(*native.ObjAsyncFunction)
			clone.SetHeapScope(scope)
			return value.EncodeHandle(heap.Allocate(clone))
		}
	}
	if fn, ok := obj.(*native.ObjGenerator); ok {
		if fn.GetHeapScope() == object.NOT_IN_HEAP_SCOPE {
			clone := fn.Clone().(*native.ObjGenerator)
			clone.SetHeapScope(scope)
			return value.EncodeHandle(heap.Allocate(clone))
		}
	}
	return v
}

type generatorMode uint8

const (
	generatorNextMode generatorMode = iota
	generatorReturnMode
	generatorThrowMode
)

func iteratorResult(resultValue value.Value, done bool) value.Value {
	result := native.NewObjectHash()
	result.SetMember(native.KEY_VALUE, resultValue)
	if done {
		result.SetMember(native.KEY_DONE, value.TRUE)
	} else {
		result.SetMember(native.KEY_DONE, value.FALSE)
	}
	return value.EncodeHandle(heap.Allocate(result))
}

func (vm *VM) iteratorResultParts(result value.Value) (bool, value.Value, error) {
	if !result.IsObject() {
		return false, value.UNDEFINED, fmt.Errorf("Iterator result is not an object")
	}
	doneValue, err := vm.getMemberValue(result, value.EncodeHandle(heap.Allocate(native.NewLightString("done"))))
	if err != nil {
		return false, value.UNDEFINED, err
	}
	done := truthyValue(doneValue)
	resultValue, err := vm.getMemberValue(result, value.EncodeHandle(heap.Allocate(native.NewLightString("value"))))
	if err != nil {
		return false, value.UNDEFINED, err
	}
	return done, resultValue, nil
}

func (vm *VM) iteratorNextWithInput(iteratorValue value.Value, input value.Value) (value.Value, error) {
	obj, err := heap.GetObject(iteratorValue.GetHandle())
	if err != nil {
		return value.UNDEFINED, err
	}
	switch iterator := obj.(type) {
	case *object.Iterator:
		done := iterator.Next()
		return iteratorResult(iterator.Current(), done), nil
	case *native.JSIterator:
		if iterator.Done {
			return iteratorResult(value.UNDEFINED, true), nil
		}
		return vm.callValue(iterator.NextMethod, iterator.Iterator, []value.Value{input})
	case *native.ObjGenerator:
		return vm.generatorResume(iterator, input, generatorNextMode)
	}
	return value.UNDEFINED, fmt.Errorf("not an iterator")
}

func (vm *VM) callIteratorMethod(iteratorValue value.Value, name string, argument value.Value) (value.Value, bool, error) {
	obj, err := heap.GetObject(iteratorValue.GetHandle())
	if err != nil {
		return value.UNDEFINED, false, err
	}
	if generator, ok := obj.(*native.ObjGenerator); ok {
		mode := generatorReturnMode
		if name == "throw" {
			mode = generatorThrowMode
		}
		result, err := vm.generatorResume(generator, argument, mode)
		return result, true, err
	}
	target := iteratorValue
	if jsIterator, ok := obj.(*native.JSIterator); ok {
		target = jsIterator.Iterator
	}
	method, err := vm.getMemberValue(target, value.EncodeHandle(heap.Allocate(native.NewLightString(name))))
	if err != nil || method == value.UNDEFINED {
		return value.UNDEFINED, false, err
	}
	result, err := vm.callValue(method, target, []value.Value{argument})
	return result, true, err
}

func (vm *VM) jumpGeneratorFinally(f *CallFrame, c *value.ValueChunk, ip *int, generator *native.ObjGenerator) bool {
	for len(vm.exceptionStack) > 0 {
		state := vm.exceptionStack[len(vm.exceptionStack)-1]
		if state.frame < vm.frameCount {
			return false
		}
		vm.exceptionStack = vm.exceptionStack[:len(vm.exceptionStack)-1]
		if state.isFinally {
			vm.frameCount = state.frame
			*f = vm.currentFrame()
			*c = *f.fn.ValueChunk()
			*ip = state.jumpTo
			vm.stackTop = state.stackTop
			vm.push(generator.CompletionMark)
			return true
		}
	}
	return false
}

func (vm *VM) generatorResume(generator *native.ObjGenerator, input value.Value, mode generatorMode) (value.Value, error) {
	if generator.Running {
		return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Generator is already running"}
	}
	if generator.Done {
		switch mode {
		case generatorReturnMode:
			return iteratorResult(input, true), nil
		case generatorThrowMode:
			return value.UNDEFINED, &JSException{Name: "Error", Message: native.String(input)}
		default:
			return iteratorResult(value.UNDEFINED, true), nil
		}
	}
	if !generator.Started {
		if mode == generatorReturnMode {
			generator.Done = true
			return iteratorResult(input, true), nil
		}
		if mode == generatorThrowMode {
			generator.Done = true
			return value.UNDEFINED, &JSException{Name: "Error", Message: native.String(input)}
		}
	}

	generator.Running = true
	runner := NewVM(false, vm.output)
	dummy := object.NewFunction("generator-caller", 0, nil)
	dummy.ValueChunk().EmitByte(chunk.OP_RETURN)
	dummyIp := 0
	runner.Call(dummy, nil, value.UNDEFINED, 0, false)

	if !generator.Started {
		generator.Started = true
		for _, arg := range generator.Args {
			runner.push(arg)
		}
		f, c := runner.Call(generator, &dummyIp, value.UNDEFINED, len(generator.Args), false)
		result, err := runner.run(f, c)
		generator.Running = false
		return result, err
	}

	f, c := runner.Call(generator, &dummyIp, value.UNDEFINED, 0, false)
	runner.stackTop = f.localStart
	for _, local := range generator.Locals {
		runner.push(local)
	}
	if generator.HasSavedScope {
		f.heapScope = generator.SavedHeapScope
		f.ownScope = generator.SavedOwnScope
		runner.frames[runner.frameCount-1].heapScope = f.heapScope
		runner.frames[runner.frameCount-1].ownScope = f.ownScope
	}
	for _, exception := range generator.Exceptions {
		runner.exceptionStack = append(runner.exceptionStack, ExceptionState{jumpTo: exception.JumpTo, stackTop: exception.StackTop, frame: runner.frameCount, isFinally: exception.IsFinally})
	}
	generator.Exceptions = nil
	ip := generator.Ip
	f.startIp = ip
	runner.frames[runner.frameCount-1].startIp = ip

	if generator.Delegating {
		if mode == generatorReturnMode {
			result, hasMethod, err := runner.callIteratorMethod(generator.DelegateIterator, "return", input)
			if err != nil {
				generator.Done = true
				generator.Running = false
				return value.UNDEFINED, err
			}
			if hasMethod {
				done, resultValue, err := runner.iteratorResultParts(result)
				if err != nil {
					generator.Done = true
					generator.Running = false
					return value.UNDEFINED, err
				}
				if !done {
					generator.Running = false
					return iteratorResult(resultValue, false), nil
				}
				input = resultValue
			}
			generator.Delegating = false
			generator.DelegateIterator = value.UNDEFINED
		} else if mode == generatorThrowMode {
			result, hasMethod, err := runner.callIteratorMethod(generator.DelegateIterator, "throw", input)
			if err != nil {
				generator.Done = true
				generator.Running = false
				return value.UNDEFINED, err
			}
			if !hasMethod {
				if _, _, closeErr := runner.callIteratorMethod(generator.DelegateIterator, "return", value.UNDEFINED); closeErr != nil {
					generator.Done = true
					generator.Running = false
					return value.UNDEFINED, closeErr
				}
				generator.Delegating = false
				generator.DelegateIterator = value.UNDEFINED
				input = runner.makeError("TypeError", "The iterator does not provide a 'throw' method")
			} else {
				done, resultValue, err := runner.iteratorResultParts(result)
				if err != nil {
					generator.Done = true
					generator.Running = false
					return value.UNDEFINED, err
				}
				if !done {
					generator.Running = false
					return iteratorResult(resultValue, false), nil
				}
				generator.Delegating = false
				generator.DelegateIterator = value.UNDEFINED
				generator.DelegateHasResult = true
				generator.DelegateResult = resultValue
				mode = generatorNextMode
			}
		}
	}

	switch mode {
	case generatorNextMode:
		if !generator.DelegateHasResult {
			runner.push(input)
		}
	case generatorReturnMode:
		generator.PendingReturn = true
		generator.ReturnValue = input
		generator.CompletionMark = value.EncodeHandle(heap.Allocate(native.NewObjectHash()))
		if !runner.jumpGeneratorFinally(&f, &c, &ip, generator) {
			generator.Done = true
			generator.Running = false
			generator.PendingReturn = false
			return iteratorResult(input, true), nil
		}
	case generatorThrowMode:
		generator.PendingReturn = false
		if err := runner.throwValue(input, &f, &c, &ip); err != nil {
			generator.Done = true
			generator.Running = false
			return value.UNDEFINED, err
		}
	}

	f.startIp = ip
	runner.frames[runner.frameCount-1].startIp = ip
	result, err := runner.run(f, c)
	generator.Running = false
	return result, err
}

func (vm *VM) suspendGenerator(generator *native.ObjGenerator, ip int, f *CallFrame, yieldValue value.Value) value.Value {
	result := iteratorResult(yieldValue, false)
	locals := make([]value.Value, vm.stackTop-f.localStart)
	for i, v := range vm.stack[f.localStart:vm.stackTop] {
		locals[i] = v
		vm.pop()
	}
	generator.Ip = ip
	generator.Locals = locals
	generator.Current = yieldValue
	generator.SavedHeapScope = f.heapScope
	generator.SavedOwnScope = f.ownScope
	generator.HasSavedScope = true
	generator.Exceptions = generator.Exceptions[:0]
	for _, state := range vm.exceptionStack {
		generator.Exceptions = append(generator.Exceptions, native.GeneratorException{JumpTo: state.jumpTo, StackTop: state.stackTop, Frame: state.frame, IsFinally: state.isFinally})
	}
	vm.exceptionStack = vm.exceptionStack[:0]
	generator.Running = false
	return result
}

func (vm *VM) generatorNext(generator *native.ObjGenerator, args []value.Value) (value.Value, error) {
	input := value.UNDEFINED
	if len(args) > 0 {
		input = args[0]
	}
	return vm.generatorResume(generator, input, generatorNextMode)
}
func (vm *VM) construct(callable object.Callable, args []value.Value, newTarget value.Value) (value.Value, error) {
	instance := native.NewObject()
	if fn, ok := callable.(*object.ObjFunction); ok {
		if fn.Prototype == value.UNDEFINED {
			proto := native.NewObject()
			proto.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("constructor"))), value.EncodeHandle(heap.Allocate(fn)))
			fn.Prototype = value.EncodeHandle(heap.Allocate(proto))
		}
	}
	prototype := value.UNDEFINED
	if newTarget.IsObject() {
		if proto, err := vm.getMemberValue(newTarget, value.EncodeHandle(heap.Allocate(native.NewLightString("prototype")))); err == nil && proto.IsObject() {
			prototype = proto
		}
	}
	if !prototype.IsObject() {
		if fn, ok := callable.(*object.ObjFunction); ok && fn.Prototype.IsObject() {
			prototype = fn.Prototype
		}
	}
	if prototype.IsObject() {
		instance.SetPrototypeValue(prototype)
	}
	instanceValue := heap.EncodeObject(instance)
	runner := NewVM(false, vm.output)
	for _, arg := range args {
		runner.push(arg)
	}
	f, c := runner.Call(callable, nil, instanceValue, len(args), false)
	f.newTarget = newTarget
	runner.frames[runner.frameCount-1].newTarget = newTarget
	result, err := runner.run(f, c)
	if err != nil {
		return value.UNDEFINED, err
	}
	if result.IsObject() {
		return result, nil
	}
	return instanceValue, nil
}

func (vm *VM) installPrivateMembers(instance *native.Instance, class *native.ObjClass) error {
	if class.SuperClass.IsObject() {
		if parent, err := heap.GetObject(class.SuperClass.GetHandle()); err == nil {
			if parentClass, ok := parent.(*native.ObjClass); ok {
				if err := vm.installPrivateMembers(instance, parentClass); err != nil {
					return err
				}
			}
		}
	}
	instanceValue := heap.EncodeObject(instance)
	for _, member := range class.PrivateMembers() {
		memberValue := member.Value
		if memberValue.IsObject() {
			if fn, err := heap.GetObject(memberValue.GetHandle()); err == nil {
				if objFn, ok := fn.(*object.ObjFunction); ok {
					clone := objFn.Clone().(*object.ObjFunction)
					clone.HomeObject = value.EncodeHandle(heap.Allocate(class))
					memberValue = value.EncodeHandle(heap.Allocate(clone))
				}
			}
		}
		member.Value = memberValue
		native.InstallPrivateMember(&instance.ObjObject, member)
	}
	_ = instanceValue
	return nil
}

func (vm *VM) initializeInstanceFields(instance *native.Instance, class *native.ObjClass) error {
	instanceValue := heap.EncodeObject(instance)
	instance.SetInitialized(true)
	for _, member := range class.InstanceFields() {
		result, err := vm.callValue(member.Value, instanceValue, nil)
		if err != nil {
			return err
		}
		if strings.HasPrefix(member.Key, "\x00#") {
			native.InstallPrivateMember(&instance.ObjObject, native.ClassMember{Key: member.Key, Value: result, Kind: native.CLASS_FIELD})
		} else {
			instance.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString(member.Key))), result)
		}
	}
	return nil
}

func (vm *VM) initializeClassInstance(instance *native.Instance, class *native.ObjClass) error {
	if err := vm.installPrivateMembers(instance, class); err != nil {
		return err
	}
	return vm.initializeInstanceFields(instance, class)
}

func (vm *VM) initializeDefaultInstance(instance *native.Instance, class *native.ObjClass) error {
	if class.SuperClass.IsObject() {
		if parent, err := heap.GetObject(class.SuperClass.GetHandle()); err == nil {
			if parentClass, ok := parent.(*native.ObjClass); ok {
				if err := vm.initializeDefaultInstance(instance, parentClass); err != nil {
					return err
				}
			}
		}
	}
	return vm.initializeClassInstance(instance, class)
}

func (vm *VM) classConstructor(class *native.ObjClass) (value.Value, bool) {
	if !class.Prototype.IsObject() {
		return value.UNDEFINED, false
	}
	proto, err := heap.GetObject(class.Prototype.GetHandle())
	if err != nil {
		return value.UNDEFINED, false
	}
	if hashable, ok := proto.(object.Hashable); ok {
		constructor := hashable.GetMember(native.KEY_CTOR)
		return constructor, constructor != value.UNDEFINED
	}
	return value.UNDEFINED, false
}

func (vm *VM) callClassConstructor(constructor value.Value, this value.Value, args []value.Value, newTarget value.Value) (value.Value, error) {
	obj, err := heap.GetObject(constructor.GetHandle())
	if err != nil {
		return value.UNDEFINED, err
	}
	callable, ok := obj.(object.Callable)
	if !ok {
		return value.UNDEFINED, fmt.Errorf("Value is not a constructor")
	}
	runner := NewVM(false, vm.output)
	for _, arg := range args {
		runner.push(arg)
	}
	f, c := runner.Call(callable, nil, this, len(args), false)
	f.newTarget = newTarget
	runner.frames[runner.frameCount-1].newTarget = newTarget
	return runner.run(f, c)
}

func (vm *VM) superBase(home value.Value, this value.Value) value.Value {
	if !home.IsObject() {
		return value.UNDEFINED
	}
	obj, err := heap.GetObject(home.GetHandle())
	if err != nil {
		return value.UNDEFINED
	}
	switch homeObj := obj.(type) {
	case *native.ObjClass:
		if this.IsObject() {
			if thisObj, err := heap.GetObject(this.GetHandle()); err == nil {
				if _, ok := thisObj.(*native.Instance); ok {
					if homeObj.Prototype.IsObject() {
						if proto, err := heap.GetObject(homeObj.Prototype.GetHandle()); err == nil {
							if prototype, ok := proto.(*native.Prototype); ok {
								return prototype.PrototypeValue()
							}
						}
					}
					return value.UNDEFINED
				}
			}
		}
		return homeObj.SuperClass
	case *native.Prototype:
		return homeObj.PrototypeValue()
	case *native.ObjObject:
		return homeObj.PrototypeValue()
	}
	return value.UNDEFINED
}

func (vm *VM) instanceFromValue(v value.Value) (*native.Instance, bool) {
	if !v.IsObject() {
		return nil, false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return nil, false
	}
	instance, ok := obj.(*native.Instance)
	return instance, ok
}

func (vm *VM) privateTarget(objValue, key value.Value) (object.Hashable, bool) {
	if !objValue.IsObject() {
		return nil, false
	}
	obj, err := heap.GetObject(objValue.GetHandle())
	if err != nil {
		return nil, false
	}
	hashable, ok := obj.(object.Hashable)
	if !ok || !hashable.HasOwn(key) {
		return nil, false
	}
	return hashable, true
}

func (vm *VM) enqueueNativeCallback(roots []value.Value, fn func()) {
	key := heap.NewCallbackRoot()
	heap.StoreCallbackRoot(key, roots...)
	queue.Enqueue(object.Callback{Native: fn, RootKey: key}, queue.MICRO_TASK, false)
}

func (vm *VM) addPromiseObserver(p *native.ObjPromise, fulfilled func(value.Value), rejected func(value.Value), roots ...value.Value) {
	observer := native.PromiseObserver{Fulfilled: fulfilled, Rejected: rejected, Roots: roots}
	if p.Status == native.PENDING {
		p.Observers = append(p.Observers, observer)
		return
	}
	status := p.Status
	argument := p.Value
	callbackRoots := append(append([]value.Value{}, roots...), argument)
	vm.enqueueNativeCallback(callbackRoots, func() {
		if status == native.RESOLVED {
			if fulfilled != nil {
				fulfilled(argument)
			}
		} else if rejected != nil {
			rejected(argument)
		}
	})
}

func (vm *VM) settlePromise(p *native.ObjPromise) {
	if p.Status == native.PENDING {
		return
	}
	reactions := p.Reactions
	p.Reactions = nil
	observers := p.Observers
	p.Observers = nil
	for _, observer := range observers {
		observer := observer
		fulfilled := p.Status == native.RESOLVED
		argument := p.Value
		callbackRoots := append(append([]value.Value{}, observer.Roots...), argument)
		vm.enqueueNativeCallback(callbackRoots, func() {
			if fulfilled {
				if observer.Fulfilled != nil {
					observer.Fulfilled(argument)
				}
			} else if observer.Rejected != nil {
				observer.Rejected(argument)
			}
		})
	}
	for _, reaction := range reactions {
		reaction := reaction
		fulfilled := p.Status == native.RESOLVED
		argument := p.Value
		callbackRoots := []value.Value{argument, reaction.OnFulfilled, reaction.OnRejected, reaction.Finalizer, reaction.Derived}
		vm.enqueueNativeCallback(callbackRoots, func() {
			vm.promiseReaction(reaction, fulfilled, argument)
		})
	}
}

func (vm *VM) attachPromiseReaction(p *native.ObjPromise, reaction native.PromiseReaction) {
	if p.Status == native.PENDING {
		p.Reactions = append(p.Reactions, reaction)
		return
	}
	fulfilled := p.Status == native.RESOLVED
	argument := p.Value
	callbackRoots := []value.Value{argument, reaction.OnFulfilled, reaction.OnRejected, reaction.Finalizer, reaction.Derived}
	vm.enqueueNativeCallback(callbackRoots, func() {
		vm.promiseReaction(reaction, fulfilled, argument)
	})
}

func (vm *VM) promiseReaction(reaction native.PromiseReaction, fulfilled bool, argument value.Value) {
	derivedObj, err := heap.GetObject(reaction.Derived.GetHandle())
	if err != nil {
		return
	}
	derived := derivedObj.(*native.ObjPromise)
	if reaction.IsFinally {
		if reaction.Finalizer.IsObject() {
			result, err := vm.callValue(reaction.Finalizer, value.UNDEFINED, nil)
			if err != nil {
				vm.rejectPromise(derived, vm.errorValue(err))
				return
			}
			if p, ok := vm.promiseFromValue(result); ok {
				roots := []value.Value{argument, reaction.Finalizer, reaction.Derived}
				vm.addPromiseObserver(p, func(value.Value) {
					if fulfilled {
						vm.resolvePromise(derived, argument)
					} else {
						vm.rejectPromise(derived, argument)
					}
				}, func(reason value.Value) {
					vm.rejectPromise(derived, reason)
				}, roots...)
				return
			}
		}
		callbackRoots := []value.Value{reaction.Derived, argument}
		vm.enqueueNativeCallback(callbackRoots, func() {
			if fulfilled {
				vm.resolvePromise(derived, argument)
			} else {
				vm.rejectPromise(derived, argument)
			}
		})
		return
	}
	handler := reaction.OnRejected
	if fulfilled {
		handler = reaction.OnFulfilled
	}
	callable := false
	if handler.IsObject() {
		if handlerObj, err := heap.GetObject(handler.GetHandle()); err == nil {
			_, callable = handlerObj.(object.Callable)
		}
	}
	if !callable {
		if fulfilled {
			vm.resolvePromise(derived, argument)
		} else {
			vm.rejectPromise(derived, argument)
		}
		return
	}
	result, err := vm.callValue(handler, value.UNDEFINED, []value.Value{argument})
	if err != nil {
		vm.rejectPromise(derived, vm.errorValue(err))
		return
	}
	vm.resolvePromise(derived, result)
}

func (vm *VM) throwGeneratorError(err error, f *CallFrame, c *value.ValueChunk, ip *int) error {
	name := "Error"
	message := err.Error()
	if jsErr, ok := err.(*JSException); ok {
		name = jsErr.Name
		message = jsErr.Message
	}
	return vm.throwValue(vm.makeError(name, message), f, c, ip)
}

func (vm *VM) errorValue(err error) value.Value {
	name := "Error"
	message := err.Error()
	if jsErr, ok := err.(*JSException); ok {
		name = jsErr.Name
		message = jsErr.Message
	}
	return vm.makeError(name, message)
}

func (vm *VM) rejectPromise(p *native.ObjPromise, reason value.Value) {
	p.Reject(reason)
	vm.settlePromise(p)
}

func (vm *VM) resolvePromise(p *native.ObjPromise, v value.Value) {
	if !v.IsObject() {
		p.Resolve(v)
		vm.settlePromise(p)
		return
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		vm.rejectPromise(p, vm.errorValue(err))
		return
	}
	if other, ok := obj.(*native.ObjPromise); ok {
		if other == p {
			vm.rejectPromise(p, vm.makeError("TypeError", "Chaining cycle detected for promise"))
			return
		}
		p.Resolving = true
		vm.attachPromiseReaction(other, native.PromiseReaction{Derived: value.EncodeHandle(heap.Allocate(p))})
		return
	}
	thenValue, err := vm.getMemberValue(v, value.EncodeHandle(heap.Allocate(native.NewLightString("then"))))
	if err != nil {
		vm.rejectPromise(p, vm.errorValue(err))
		return
	}
	if thenValue.IsObject() {
		if thenObj, err := heap.GetObject(thenValue.GetHandle()); err == nil {
			if _, ok := thenObj.(object.Callable); ok {
				p.Resolving = true
				p.ResolutionID++
				resolutionID := p.ResolutionID
				callbackRoots := []value.Value{thenValue, v}
				vm.enqueueNativeCallback(callbackRoots, func() {
					called := false
					resolve := value.EncodeHandle(heap.Allocate(native.NewResolveFuncTracked(p, &called, resolutionID)))
					reject := value.EncodeHandle(heap.Allocate(native.NewRejectFuncWithID(p, resolutionID)))
					if _, err := vm.callValue(thenValue, v, []value.Value{resolve, reject}); err != nil {
						if !called && p.Status == native.PENDING {
							vm.rejectPromise(p, vm.errorValue(err))
						}
					}
				})
				return
			}
		}
	}
	p.Resolve(v)
	vm.settlePromise(p)
}

func (vm *VM) invokeCallableWithThis(target, this value.Value, args []value.Value) (value.Value, error) {
	runner := NewVM(false, vm.output)
	fn := object.NewFunction("native-call", 0, nil)
	for _, arg := range args {
		fn.ValueChunk().WriteConstant(arg)
	}
	fn.ValueChunk().WriteConstant(this)
	fn.ValueChunk().WriteConstant(target)
	fn.ValueChunk().WriteConstant(value.TAG_METHOD_HANDLE)
	fn.ValueChunk().EmitBytes(chunk.OP_CALL, uint8(len(args)), 0, chunk.OP_RETURN)
	frame, code := runner.Call(fn, nil, value.UNDEFINED, 0, false)
	return runner.run(frame, code)
}

func builderFromValue(v value.Value) (*native.ObjStringBuilder, bool) {
	if !v.IsObject() {
		return nil, false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return nil, false
	}
	builder, ok := obj.(*native.ObjStringBuilder)
	return builder, ok
}

func (vm *VM) callValue(fn value.Value, this value.Value, args []value.Value) (value.Value, error) {
	obj, err := heap.GetObject(fn.GetHandle())
	if err != nil {
		return value.UNDEFINED, err
	}
	callable, ok := obj.(object.Callable)
	if !ok {
		if obj.Type() != object.OBJ_NATIVE_FN {
			return value.UNDEFINED, fmt.Errorf("value is not callable")
		}
		return vm.invokeCallableWithThis(fn, this, args)
	}
	runner := NewVM(false, vm.output)
	for _, arg := range args {
		runner.push(arg)
	}
	f, c := runner.Call(callable, nil, this, len(args), false)
	return runner.run(f, c)
}

func (vm *VM) setMemberValue(objValue value.Value, key, v value.Value) error {
	return vm.setMemberValueStrict(objValue, key, v, false)
}

func (vm *VM) syncGlobalVar(objValue, key, v value.Value) {
	if vm.globalObject == 0 || objValue != vm.globalObject {
		return
	}
	keyString, ok := vm.valueAsString(key)
	if !ok {
		return
	}
	if slot, found := globalVarSlots[keyString]; found {
		globals[slot] = v
	}
}

func (vm *VM) setMemberValueStrict(objValue value.Value, key, v value.Value, strict bool) error {
	return vm.setMemberValueWithReceiver(objValue, key, v, objValue, strict)
}

func (vm *VM) setMemberValueWithReceiver(objValue value.Value, key, v, receiver value.Value, strict bool) error {
	if !objValue.IsObject() {
		return fmt.Errorf("cannot set property on non-object")
	}
	obj, err := heap.GetObject(objValue.GetHandle())
	if err != nil {
		return err
	}
	if proxy, ok := obj.(*native.ObjProxy); ok {
		return vm.proxySet(proxy, key, v, receiver, strict)
	}
	if _, ok := obj.(*native.ObjModuleNamespace); ok {
		return fmt.Errorf("Cannot assign to read only property")
	}
	if k, ok := vm.valueAsString(key); ok {
		key = value.EncodeHandle(heap.Allocate(native.NewLightString(k)))
	}
	if keyString, ok := vm.valueAsString(key); ok && keyString == "__proto__" {
		if own, ok := obj.(interface {
			GetOwnEntry(value.Value) (native.ObjectValueEntry, bool)
			SetPrototypeValue(value.Value)
			PrototypeValue() value.Value
			Extensible() bool
		}); ok {
			if _, found := own.GetOwnEntry(key); !found {
				if v == value.NULL || v.IsObject() {
					if own.Extensible() {
						own.SetPrototypeValue(v)
					} else if strict {
						return fmt.Errorf("Cannot set prototype of non-extensible object")
					}
				} else if strict {
					return fmt.Errorf("Object prototype may only be an Object or null")
				}
				return nil
			}
		}
	}
	if args, ok := obj.(*native.MappedArguments); ok {
		if i, isIndex := vm.arrayIndexFromValue(key); isIndex {
			args.SetIndex(i, v)
			return nil
		}
	}
	if fn, ok := obj.(*object.ObjFunction); ok {
		if keyString, ok := vm.valueAsString(key); ok {
			if keyString == "prototype" {
				fn.Prototype = v
				return nil
			}
			if keyString != "name" && keyString != "length" {
				if fn.Props == nil {
					fn.Props = map[string]value.Value{}
				}
				fn.Props[keyString] = v
				return nil
			}
		}
	}
	if arr, ok := obj.(*native.ObjArr); ok {
		if keyString, ok := vm.valueAsString(key); ok && keyString == "length" {
			if entry, found := arr.GetOwnEntry(key); found && !entry.Writable() {
				if strict {
					return fmt.Errorf("Cannot assign to read only property 'length'")
				}
				return nil
			}
			n, err := vm.toNumber(v)
			if err != nil {
				return err
			}
			if math.IsNaN(n) || n < 0 || math.Trunc(n) != n || n >= 4294967296 {
				return &JSException{Name: "RangeError", Message: "Invalid array length"}
			}
			arr.SetLength(int(n))
			return nil
		}
	}
	if arr, ok := obj.(*native.ObjArr); ok {
		if i, isIndex := vm.arrayIndexFromValue(key); isIndex {
			if !arr.Extensible() {
				if strict {
					return fmt.Errorf("Cannot add property, object is not extensible")
				}
				return nil
			}
			if entry, found := arr.GetOwnEntry(value.EncodeHandle(heap.Allocate(native.NewLightString("length")))); found && !entry.Writable() {
				if strict {
					return fmt.Errorf("Cannot assign to read only property")
				}
				return nil
			}
			arr.SetElementAt(i, v)
			return nil
		}
	}
	if array, ok := obj.(*native.ObjTypedArray); ok {
		if index, isIndex := vm.arrayIndexFromValue(key); isIndex {
			if index < 0 || index >= array.Length {
				return nil
			}
			if array.Kind == native.TYPED_BIGINT64 || array.Kind == native.TYPED_BIGUINT64 {
				if _, ok := vm.bigIntObject(v); !ok {
					return &JSException{Name: "TypeError", Message: "Cannot convert to a BigInt"}
				}
			}
			array.Write(index, v)
			return nil
		}
	}
	if re, ok := obj.(*native.ObjRegExp); ok {
		if keyString, ok := vm.valueAsString(key); ok && keyString == "lastIndex" {
			n, _ := vm.toNumber(v)
			re.LastIndex = int(n)
		}
	}
	if own, ok := obj.(interface {
		GetOwnEntry(value.Value) (native.ObjectValueEntry, bool)
		PrototypeValue() value.Value
	}); ok {
		if entry, found := own.GetOwnEntry(key); found {
			if entry.IsAccessor() {
				if entry.Set != value.UNDEFINED {
					_, err := vm.callValue(entry.Set, receiver, []value.Value{v})
					return err
				}
				if strict {
					return fmt.Errorf("Cannot set property which has only a getter")
				}
				return nil
			}
			if !entry.Writable() {
				if strict {
					return fmt.Errorf("Cannot assign to read only property")
				}
				return nil
			}
			if !native.ObjectIsSame(receiver, objValue) {
				if proxy, ok := vm.asProxy(receiver); ok {
					if _, err := vm.proxyGetOwnPropertyDescriptor(proxy, key); err != nil {
						return err
					}
					descriptor := native.NewObjectHash()
					descriptor.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("value"))), v)
					descriptor.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("writable"))), value.TRUE)
					descriptor.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("enumerable"))), value.TRUE)
					descriptor.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("configurable"))), value.TRUE)
					defined, err := vm.proxyDefineProperty(proxy, key, value.EncodeHandle(heap.Allocate(descriptor)))
					if err != nil {
						return err
					}
					if !defined && strict {
						return fmt.Errorf("Cannot define property on receiver")
					}
					return nil
				}
			}
		} else {
			if o, ok := obj.(interface{ Extensible() bool }); ok && !o.Extensible() {
				if strict {
					return fmt.Errorf("Cannot add property, object is not extensible")
				}
				return nil
			}
			current := own.PrototypeValue()
			for current.IsObject() {
				currentObj, err := heap.GetObject(current.GetHandle())
				if err != nil {
					break
				}
				po, ok := currentObj.(interface {
					GetOwnEntry(value.Value) (native.ObjectValueEntry, bool)
					PrototypeValue() value.Value
				})
				if !ok {
					break
				}
				if entry, found := po.GetOwnEntry(key); found {
					if entry.IsAccessor() {
						if entry.Set != value.UNDEFINED {
							_, err := vm.callValue(entry.Set, receiver, []value.Value{v})
							return err
						}
						if strict {
							return fmt.Errorf("Cannot set property which has only a getter")
						}
						return nil
					}
					break
				}
				current = po.PrototypeValue()
			}
		}
		if hashable, ok := obj.(object.Hashable); ok {
			hashable.SetMember(key, v)
		}
		vm.syncGlobalVar(objValue, key, v)
		return nil
	}
	if hashable, ok := obj.(object.Hashable); ok {
		hashable.SetMember(key, v)
		vm.syncGlobalVar(objValue, key, v)
		return nil
	}
	return fmt.Errorf("cannot set property on this value")
}

func (vm *VM) getMemberValue(objValue value.Value, key value.Value) (value.Value, error) {
	return vm.getMemberValueWithReceiver(objValue, key, objValue)
}

func (vm *VM) getMemberValueWithReceiver(objValue value.Value, key value.Value, receiver value.Value) (value.Value, error) {
	if !objValue.IsObject() {
		if objValue.IsNumber() && native.PROTOTYPE_NUMBER.IsObject() {
			return vm.getMemberValueWithReceiver(native.PROTOTYPE_NUMBER, key, receiver)
		}
		return value.UNDEFINED, nil
	}
	obj, err := heap.GetObject(objValue.GetHandle())
	if err != nil {
		return value.UNDEFINED, err
	}
	if proxy, ok := obj.(*native.ObjProxy); ok {
		return vm.proxyGet(proxy, objValue, key, receiver)
	}
	if keyString, ok := vm.valueAsString(key); ok && keyString == "__proto__" {
		if own, ok := obj.(interface {
			GetOwnEntry(value.Value) (native.ObjectValueEntry, bool)
			PrototypeValue() value.Value
		}); ok {
			if _, found := own.GetOwnEntry(key); !found {
				return own.PrototypeValue(), nil
			}
		}
	}
	if re, ok := obj.(*native.ObjRegExp); ok {
		if keyString, ok := vm.valueAsString(key); ok && keyString == "lastIndex" {
			return value.ValueFromFloat64(float64(re.LastIndex)), nil
		}
		switch key {
		case native.SYMBOL_MATCH:
			return value.EncodeHandle(heap.Allocate(native.NewRegExpSymbolMethod("match", native.REGEXP_SYMBOL_MATCH))), nil
		case native.SYMBOL_REPLACE:
			return value.EncodeHandle(heap.Allocate(native.NewRegExpSymbolMethod("replace", native.REGEXP_SYMBOL_REPLACE))), nil
		case native.SYMBOL_SEARCH:
			return value.EncodeHandle(heap.Allocate(native.NewRegExpSymbolMethod("search", native.REGEXP_SYMBOL_SEARCH))), nil
		case native.SYMBOL_SPLIT:
			return value.EncodeHandle(heap.Allocate(native.NewRegExpSymbolMethod("split", native.REGEXP_SYMBOL_SPLIT))), nil
		case native.SYMBOL_MATCH_ALL:
			return value.EncodeHandle(heap.Allocate(native.NewRegExpSymbolMethod("matchAll", native.REGEXP_SYMBOL_MATCH_ALL))), nil
		}
	}
	if namespace, ok := obj.(*native.ObjModuleNamespace); ok {
		if member, found := vm.moduleNamespaceMember(namespace, key); found {
			return member, nil
		}
	}
	if boxed, ok := obj.(*native.BoxedPrimitive); ok {
		if keyString, ok := vm.valueAsString(key); ok && keyString == "valueOf" {
			return value.EncodeHandle(heap.Allocate(native.NewBoxedValueOf())), nil
		}
		if _, ok := vm.valueAsString(boxed.Primitive); ok {
			return vm.getMemberValueWithReceiver(boxed.Primitive, key, receiver)
		}
	}
	if s, isString := vm.valueAsString(objValue); isString {
		if index, ok := vm.arrayIndexFromValue(key); ok {
			if ch, ok := native.UTF16CharAt(s, index); ok {
				return value.EncodeHandle(heap.Allocate(native.NewLightString(ch))), nil
			}
			return value.UNDEFINED, nil
		}
	}
	if fn, ok := obj.(*object.ObjFunction); ok {
		if keyString, ok := vm.valueAsString(key); ok {
			switch keyString {
			case "call":
				return value.EncodeHandle(heap.Allocate(native.NewFunctionCall())), nil
			case "apply":
				return value.EncodeHandle(heap.Allocate(native.NewFunctionApply())), nil
			case "bind":
				return value.EncodeHandle(heap.Allocate(native.NewFunctionBind())), nil
			case "name":
				return value.EncodeHandle(heap.Allocate(native.NewLightString(fn.Name))), nil
			case "length":
				return value.ValueFromFloat64(float64(fn.Length)), nil
			case "prototype":
				if fn.IsArrow || fn.IsMethod {
					return value.UNDEFINED, nil
				}
				return native.EnsureFunctionPrototype(fn, objValue), nil
			}
		}
		if keyString, ok := vm.valueAsString(key); ok {
			if v, found := fn.Props[keyString]; found {
				return v, nil
			}
		}
		return vm.getMemberValue(native.PROTOTYPE_OBJECT, key)
	}
	if _, ok := obj.(*native.ObjAsyncFunction); ok {
		return vm.getMemberValue(native.PROTOTYPE_OBJECT, key)
	}
	if _, ok := obj.(*native.ObjGenerator); ok {
		return vm.getMemberValue(native.PROTOTYPE_OBJECT, key)
	}
	if class, ok := obj.(*native.ObjClass); ok {
		if keyString, ok := vm.valueAsString(key); ok {
			if keyString == "prototype" {
				return class.Prototype, nil
			}
			if keyString == "name" {
				return value.EncodeHandle(heap.Allocate(native.NewLightString(class.Name()))), nil
			}
		}
	}
	if array, ok := obj.(*native.ObjTypedArray); ok {
		if member, found := vm.getTypedArrayMember(array, key); found {
			return member, nil
		}
	}
	if view, ok := obj.(*native.ObjDataView); ok {
		if member, found := vm.getDataViewMember(view, key); found {
			return member, nil
		}
	}
	if buffer, ok := obj.(*native.ObjArrayBuffer); ok {
		if keyString, ok := vm.valueAsString(key); ok {
			switch keyString {
			case "byteLength":
				return value.ValueFromFloat64(float64(len(buffer.Data))), nil
			case "slice":
				return value.EncodeHandle(heap.Allocate(native.NewArrayBufferSlice())), nil
			}
		}
	}
	if nativeFn, ok := obj.(interface{ NativeArity() int }); ok {
		if keyString, ok := vm.valueAsString(key); ok && keyString == "length" {
			return value.ValueFromFloat64(float64(nativeFn.NativeArity())), nil
		}
	}
	if obj.Type() == object.OBJ_NATIVE_FN {
		if keyString, ok := vm.valueAsString(key); ok {
			switch keyString {
			case "call":
				return value.EncodeHandle(heap.Allocate(native.NewFunctionCall())), nil
			case "apply":
				return value.EncodeHandle(heap.Allocate(native.NewFunctionApply())), nil
			case "bind":
				return value.EncodeHandle(heap.Allocate(native.NewFunctionBind())), nil
			}
		}
	}
	if _, ok := obj.(*native.ObjBoundFunction); ok {
		if keyString, ok := vm.valueAsString(key); ok {
			switch keyString {
			case "call":
				return value.EncodeHandle(heap.Allocate(native.NewFunctionCall())), nil
			case "apply":
				return value.EncodeHandle(heap.Allocate(native.NewFunctionApply())), nil
			case "bind":
				return value.EncodeHandle(heap.Allocate(native.NewFunctionBind())), nil
			}
		}
	}
	if generator, ok := obj.(*native.ObjGenerator); ok {
		if key == native.SYMBOL_ITERATOR || (generator.IsAsync && key == native.SYMBOL_ASYNC_ITERATOR) {
			return value.EncodeHandle(heap.Allocate(native.NewGeneratorIterator())), nil
		}
	}
	if _, ok := obj.(*native.ObjArr); ok && key == native.SYMBOL_ITERATOR {
		return value.EncodeHandle(heap.Allocate(native.NewArrayIterator())), nil
	}
	if _, ok := obj.(*native.Map); ok && key == native.SYMBOL_ITERATOR {
		return value.EncodeHandle(heap.Allocate(native.NewMapEntries())), nil
	}
	if _, ok := obj.(*native.Set); ok && key == native.SYMBOL_ITERATOR {
		return value.EncodeHandle(heap.Allocate(native.NewSetValues())), nil
	}
	if _, ok := obj.(*native.LightString); ok && key == native.SYMBOL_ITERATOR {
		return value.EncodeHandle(heap.Allocate(native.NewStringIterator())), nil
	}
	if _, ok := obj.(*native.ObjString); ok && key == native.SYMBOL_ITERATOR {
		return value.EncodeHandle(heap.Allocate(native.NewStringIterator())), nil
	}
	if _, ok := obj.(*object.Iterator); ok {
		if keyString, ok := vm.valueAsString(key); ok && keyString == "next" {
			return value.EncodeHandle(heap.Allocate(native.NewIteratorNext())), nil
		}
	}
	if args, ok := obj.(*native.MappedArguments); ok {
		if i, isIndex := vm.arrayIndexFromValue(key); isIndex {
			return args.GetIndex(i), nil
		}
		if keyString, ok := vm.valueAsString(key); ok && keyString == "length" {
			return value.ValueFromFloat64(float64(args.Length())), nil
		}
	}
	if arr, ok := obj.(*native.ObjArr); ok {
		if i, isIndex := vm.arrayIndexFromValue(key); isIndex {
			if arr.HasIndex(i) {
				return arr.GetElementAt(i), nil
			}
		}
	}
	if objString, ok := obj.(*native.LightString); ok {
		return native.NewObjString(objString.String()).GetMember(key), nil
	}
	if objString, ok := obj.(*native.ObjStringBuilder); ok {
		return native.NewObjString(objString.String()).GetMember(key), nil
	}
	if own, ok := obj.(interface {
		GetOwnEntry(value.Value) (native.ObjectValueEntry, bool)
		PrototypeValue() value.Value
	}); ok {
		if entry, found := own.GetOwnEntry(key); found {
			if entry.IsAccessor() {
				if entry.Get == value.UNDEFINED {
					return value.UNDEFINED, nil
				}
				return vm.callValue(entry.Get, receiver, nil)
			}
			return entry.Value, nil
		}
		proto := own.PrototypeValue()
		if proto.IsObject() && heap.GetPointer(proto.GetHandle()) != heap.GetPointer(objValue.GetHandle()) {
			return vm.getMemberValueWithReceiver(proto, key, receiver)
		}
		return value.UNDEFINED, nil
	}
	if hashable, ok := obj.(object.Hashable); ok {
		return hashable.GetMember(key), nil
	}
	return value.UNDEFINED, nil
}

func (vm *VM) toPrimitive(v value.Value, hint string) (value.Value, error) {
	if !v.IsObject() {
		return v, nil
	}
	if obj, err := heap.GetObject(v.GetHandle()); err == nil {
		if _, ok := obj.(*native.ObjSymbol); ok {
			return value.UNDEFINED, fmt.Errorf("Cannot convert a Symbol value to a string")
		}
		if boxed, ok := obj.(*native.BoxedPrimitive); ok {
			return boxed.Primitive, nil
		}
		if _, ok := obj.(*native.ObjBigInt); ok {
			return v, nil
		}
		if _, ok := obj.(*native.ObjStringBuilder); ok {
			return v, nil
		}
	}
	if native.SYMBOL_TO_PRIMITIVE != 0 {
		method, err := vm.getMemberValue(v, native.SYMBOL_TO_PRIMITIVE)
		if err == nil && method != value.UNDEFINED && method != value.NULL {
			methodObj, err := heap.GetObject(method.GetHandle())
			if err != nil {
				return value.UNDEFINED, err
			}
			if _, ok := methodObj.(object.Callable); !ok {
				return value.UNDEFINED, fmt.Errorf("Symbol.toPrimitive is not a function")
			}
			hintValue := value.EncodeHandle(heap.Allocate(native.NewLightString(hint)))
			result, err := vm.callValue(method, v, []value.Value{hintValue})
			if err != nil {
				return value.UNDEFINED, err
			}
			if result.IsObject() {
				if _, ok := vm.valueAsString(result); !ok {
					return value.UNDEFINED, fmt.Errorf("Cannot convert object to primitive value")
				}
			}
			return result, nil
		}
	}
	order := []string{"valueOf", "toString"}
	if hint == "string" {
		order = []string{"toString", "valueOf"}
	}
	foundMethod := false
	for _, name := range order {
		method, err := vm.getMemberValue(v, value.EncodeHandle(heap.Allocate(native.NewLightString(name))))
		if err != nil || !method.IsObject() {
			continue
		}
		methodObj, err := heap.GetObject(method.GetHandle())
		if err != nil {
			continue
		}
		isCallable := false
		if _, ok := methodObj.(object.Callable); ok {
			isCallable = true
		} else if methodObj.Type() == object.OBJ_NATIVE_FN {
			isCallable = true
		}
		if !isCallable {
			continue
		}
		foundMethod = true
		var result value.Value
		if _, ok := methodObj.(object.Callable); ok {
			result, err = vm.callValue(method, v, nil)
		} else {
			result, err = vm.invokeCallableWithThis(method, v, nil)
		}
		if err == nil {
			if !result.IsObject() {
				return result, nil
			}
			if _, ok := vm.valueAsString(result); ok {
				return result, nil
			}
		}
	}

	obj, _ := heap.GetObject(v.GetHandle())
	switch obj := obj.(type) {
	case *native.ObjArr:
		return vm.arrayToString(obj)
	case *native.LightString:
		return value.EncodeHandle(heap.Allocate(native.NewLightString(obj.String()))), nil
	case *native.ObjString:
		return value.EncodeHandle(heap.Allocate(native.NewLightString(obj.String()))), nil
	case *native.ObjStringBuilder:
		return value.EncodeHandle(heap.Allocate(native.NewLightString(obj.String()))), nil
	}
	if foundMethod {
		return value.UNDEFINED, fmt.Errorf("cannot convert object to primitive value")
	}
	return value.EncodeHandle(heap.Allocate(native.NewLightString("[object Object]"))), nil
}

func (vm *VM) arrayToString(arr *native.ObjArr) (value.Value, error) {
	parts := make([]string, len(arr.Values()))
	for i, item := range arr.Values() {
		if item == value.UNDEFINED || item == value.NULL || item == value.EMPTY_ARRAY_ITEM {
			parts[i] = ""
			continue
		}
		s, err := vm.toStringValue(item)
		if err != nil {
			return value.UNDEFINED, err
		}
		str, _ := vm.valueAsString(s)
		parts[i] = str
	}
	return value.EncodeHandle(heap.Allocate(native.NewLightString(strings.Join(parts, ",")))), nil
}

func (vm *VM) toNumber(v value.Value) (float64, error) {
	if s, ok := vm.valueAsString(v); ok {
		return parseNumericString(s)
	}
	if v.IsObject() {
		primitive, err := vm.toPrimitive(v, "number")
		if err != nil {
			return 0, err
		}
		return vm.toNumber(primitive)
	}
	if v.IsBoolean() {
		if v == value.TRUE {
			return 1, nil
		}
		return 0, nil
	}
	if v.IsType(value.NULL) {
		return 0, nil
	}
	if v.IsType(value.UNDEFINED) {
		return math.NaN(), nil
	}
	if v.IsNumber() {
		return v.AsNumber(), nil
	}
	return math.NaN(), nil
}

func parseNumericString(str string) (float64, error) {
	s := strings.TrimFunc(str, func(r rune) bool {
		return unicode.IsSpace(r) || r == '\uFEFF'
	})
	if s == "" {
		return 0, nil
	}
	switch s {
	case "Infinity", "+Infinity":
		return math.Inf(1), nil
	case "-Infinity":
		return math.Inf(-1), nil
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		n, err := strconv.ParseUint(s[2:], 16, 64)
		if err != nil {
			return math.NaN(), nil
		}
		return float64(n), nil
	}
	if strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0O") {
		n, err := strconv.ParseUint(s[2:], 8, 64)
		if err != nil {
			return math.NaN(), nil
		}
		return float64(n), nil
	}
	if strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B") {
		n, err := strconv.ParseUint(s[2:], 2, 64)
		if err != nil {
			return math.NaN(), nil
		}
		return float64(n), nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN(), nil
	}
	return n, nil
}

func formatNumber(n float64) string {
	if math.IsNaN(n) {
		return "NaN"
	}
	if math.IsInf(n, 1) {
		return "Infinity"
	}
	if math.IsInf(n, -1) {
		return "-Infinity"
	}
	if n == 0 {
		return "0"
	}
	abs := math.Abs(n)
	if abs >= 1e-6 && abs < 1e21 {
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	s := strconv.FormatFloat(n, 'e', -1, 64)
	if idx := strings.IndexByte(s, 'e'); idx >= 0 {
		mantissa, exponent := s[:idx], s[idx+1:]
		exponent = strings.TrimPrefix(exponent, "+")
		exp, _ := strconv.Atoi(exponent)
		exponent = strconv.Itoa(exp)
		if exp >= 0 {
			exponent = "+" + exponent
		}
		s = mantissa + "e" + exponent
	}
	return s
}

func (vm *VM) toPropertyKey(v value.Value) (value.Value, error) {
	if v.IsObject() {
		if obj, err := heap.GetObject(v.GetHandle()); err == nil {
			if _, ok := obj.(*native.ObjSymbol); ok {
				return v, nil
			}
		}
	}
	return vm.toStringValue(v)
}

func (vm *VM) toStringValue(v value.Value) (value.Value, error) {
	if s, ok := vm.valueAsString(v); ok {
		return value.EncodeHandle(heap.Allocate(native.NewLightString(s))), nil
	}
	if b, ok := bigIntFromValue(v); ok {
		return value.EncodeHandle(heap.Allocate(native.NewLightString(b.String()))), nil
	}
	if v.IsObject() {
		if obj, err := heap.GetObject(v.GetHandle()); err == nil {
			if arr, ok := obj.(*native.ObjArr); ok {
				return vm.arrayToString(arr)
			}
		}
		primitive, err := vm.toPrimitive(v, "string")
		if err != nil {
			return value.UNDEFINED, err
		}
		return vm.toStringValue(primitive)
	}
	if v.IsBoolean() {
		if v == value.TRUE {
			return value.EncodeHandle(heap.Allocate(native.NewLightString("true"))), nil
		}
		return value.EncodeHandle(heap.Allocate(native.NewLightString("false"))), nil
	}
	if v.IsType(value.UNDEFINED) {
		return value.EncodeHandle(heap.Allocate(native.NewLightString("undefined"))), nil
	}
	if v.IsType(value.NULL) {
		return value.EncodeHandle(heap.Allocate(native.NewLightString("null"))), nil
	}
	if v.IsNumber() {
		return value.EncodeHandle(heap.Allocate(native.NewLightString(formatNumber(v.AsNumber())))), nil
	}
	return v, nil
}

func (vm *VM) toInt32(v value.Value) (int32, error) {
	n, err := vm.toNumber(v)
	if err != nil {
		return 0, err
	}
	return int32(uint32(int64(n))), nil
}

func (vm *VM) toUint32(v value.Value) (uint32, error) {
	n, err := vm.toNumber(v)
	if err != nil {
		return 0, err
	}
	return uint32(int64(n)), nil
}

func (vm *VM) typeofValue(v value.Value) string {
	if v.IsObject() {
		obj, _ := heap.GetObject(v.GetHandle())
		if obj.Type() == object.OBJ_NATIVE_FN || obj.Type() == object.OBJ_OBJECT_CONSTRUCTOR || obj.Type() == object.OBJ_ARRAY_CONSTRUCTOR || obj.Type() == object.OBJ_ERROR_CONSTRUCTOR || obj.Type() == object.OBJ_DATE_CONSTRUCTOR || obj.Type() == object.OBJ_MAP_CONSTRUCTOR || obj.Type() == object.OBJ_SET_CONSTRUCTOR || obj.Type() == object.OBJ_PROMISE_CONSTRUCTOR || obj.Type() == object.OBJ_STRING_CONSTRUCTOR || obj.Type() == object.OBJ_SYMBOL_CONSTRUCTOR || obj.Type() == object.OBJ_CLASS {
			return "function"
		}
		switch obj := obj.(type) {
		case *object.ObjFunction, *native.ObjAsyncFunction, *native.ObjGenerator, *native.ObjClass, *native.ArrayConstructor, *native.ObjectConstructor, *native.ErrorConstructor, *native.DateConstructor, *native.MapConstructor, *native.SetConstructor, *native.PromiseConstructor, *native.StringConstructor, *native.NumberConstructor, *native.BooleanConstructor, *native.SymbolConstructor, *native.BigIntConstructor, *native.RegExpConstructor, *native.ProxyConstructor, *native.TypedArrayConstructor, *native.ArrayBufferConstructor, *native.DataViewConstructor:
			return "function"
		case *native.ObjProxy:
			if obj.Callable {
				return "function"
			}
		case *native.ObjBigInt:
			return "bigint"
		case *native.LightString, *native.ObjStringBuilder:
			return "string"
		case *native.ObjSymbol:
			return "symbol"
		}
		return "object"
	}
	if v.IsBoolean() {
		return "boolean"
	}
	if v.IsType(value.UNDEFINED) {
		return "undefined"
	}
	if v.IsType(value.NULL) {
		return "object"
	}
	return "number"
}

func (vm *VM) instanceOf(objValue, ctor value.Value) (bool, error) {
	if !objValue.IsObject() {
		return false, nil
	}
	if !ctor.IsObject() {
		return false, fmt.Errorf("right-hand side of instanceof is not callable")
	}
	ctorObj, err := heap.GetObject(ctor.GetHandle())
	if err != nil {
		return false, err
	}
	if native.SYMBOL_HAS_INSTANCE != 0 {
		if hashable, ok := ctorObj.(object.Hashable); ok && hashable.HasOwn(native.SYMBOL_HAS_INSTANCE) {
			method := hashable.GetMember(native.SYMBOL_HAS_INSTANCE)
			if method.IsObject() {
				result, err := vm.callValue(method, ctor, []value.Value{objValue})
				if err != nil {
					return false, err
				}
				return truthyValue(result), nil
			}
		}
	}
	if bound, ok := ctorObj.(*native.ObjBoundFunction); ok {
		return vm.instanceOf(objValue, bound.Target)
	}
	var prototype value.Value
	switch ctorObj := ctorObj.(type) {
	case *native.ObjectConstructor:
		return true, nil
	case *native.PromiseConstructor:
		if obj, err := heap.GetObject(objValue.GetHandle()); err == nil {
			_, ok := obj.(*native.ObjPromise)
			return ok, nil
		}
		return false, nil
	case *native.TypedArrayConstructor:
		if obj, err := heap.GetObject(objValue.GetHandle()); err == nil {
			array, ok := obj.(*native.ObjTypedArray)
			return ok && array.Kind == ctorObj.Kind, nil
		}
		return false, nil
	case *native.ArrayBufferConstructor:
		if obj, err := heap.GetObject(objValue.GetHandle()); err == nil {
			_, ok := obj.(*native.ObjArrayBuffer)
			return ok, nil
		}
		return false, nil
	case *native.DataViewConstructor:
		if obj, err := heap.GetObject(objValue.GetHandle()); err == nil {
			_, ok := obj.(*native.ObjDataView)
			return ok, nil
		}
		return false, nil
	case *object.ObjFunction:
		if ctorObj.IsArrow || ctorObj.IsMethod {
			return false, fmt.Errorf("right-hand side of instanceof is not callable")
		}
		prototype, err = vm.getMemberValue(ctor, value.EncodeHandle(heap.Allocate(native.NewLightString("prototype"))))
		if err != nil {
			return false, err
		}
	case *native.ObjClass:
		prototype = ctorObj.Prototype
	case *native.ErrorConstructor:
		prototype, ok := native.ERROR_PROTOTYPES[ctorObj.ErrorName()]
		if !ok {
			return false, nil
		}
		return native.ErrorPrototypeChainContains(objValue, prototype), nil
	}
	if !prototype.IsObject() {
		return false, fmt.Errorf("function has non-object prototype in instanceof check")
	}
	protoPtr := heap.GetPointer(prototype.GetHandle())
	current := objValue
	for current.IsObject() {
		obj, err := heap.GetObject(current.GetHandle())
		if err != nil {
			return false, err
		}
		hashable, ok := obj.(object.Hashable)
		if !ok {
			break
		}
		next := hashable.GetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("__proto__"))))
		if next == value.UNDEFINED {
			next = hashable.GetMember(native.KEY_PROTO)
		}
		if !next.IsObject() {
			break
		}
		if heap.GetPointer(next.GetHandle()) == protoPtr {
			return true, nil
		}
		current = next
	}
	return false, nil
}

func isMethodHandle(v value.Value) bool {
	return v == value.TAG_METHOD_HANDLE
}

func boolValue(b bool) value.Value {
	if b {
		return value.TRUE
	}
	return value.FALSE
}

func (vm *VM) valueAsString(v value.Value) (string, bool) {
	if !v.IsObject() {
		return "", false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return "", false
	}
	switch obj := obj.(type) {
	case *native.LightString:
		return obj.String(), true
	case *native.ObjString:
		return obj.String(), true
	case *native.ObjStringBuilder:
		return obj.String(), true
	}
	return "", false
}

func strictEquals(a, b value.Value) bool {
	if a.IsObject() && b.IsObject() {
		as, aString := valueAsStringStatic(a)
		bs, bString := valueAsStringStatic(b)
		if aString && bString {
			return as == bs
		}
		if aString != bString {
			return false
		}
		if ab, ok := bigIntFromValue(a); ok {
			if bb, ok := bigIntFromValue(b); ok {
				return ab.Cmp(bb) == 0
			}
			return false
		}
		return heap.GetPointer(a.GetHandle()) == heap.GetPointer(b.GetHandle())
	}
	if a.IsNumber() && b.IsNumber() {
		an, bn := a.AsNumber(), b.AsNumber()
		if math.IsNaN(an) || math.IsNaN(bn) {
			return false
		}
		return an == bn
	}
	return a == b
}

func valueAsStringStatic(v value.Value) (string, bool) {
	if !v.IsObject() {
		return "", false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return "", false
	}
	switch obj := obj.(type) {
	case *native.LightString:
		return obj.String(), true
	case *native.ObjString:
		return obj.String(), true
	case *native.ObjStringBuilder:
		return obj.String(), true
	}
	return "", false
}

func (vm *VM) compare(a, b value.Value) (int, error) {
	if ab, ok := bigIntFromValue(a); ok {
		return vm.compareBigInt(ab, b, true)
	}
	if bb, ok := bigIntFromValue(b); ok {
		return vm.compareBigInt(bb, a, false)
	}
	pa, err := vm.toPrimitive(a, "number")
	if err != nil {
		return 0, err
	}
	pb, err := vm.toPrimitive(b, "number")
	if err != nil {
		return 0, err
	}
	as, aString := vm.valueAsString(pa)
	bs, bString := vm.valueAsString(pb)
	if aString && bString {
		if as < bs {
			return -1, nil
		}
		if as > bs {
			return 1, nil
		}
		return 0, nil
	}
	an, err := vm.toNumber(pa)
	if err != nil {
		return 0, err
	}
	bn, err := vm.toNumber(pb)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(an) || math.IsNaN(bn) {
		return 2, nil
	}
	if an < bn {
		return -1, nil
	}
	if an > bn {
		return 1, nil
	}
	return 0, nil
}

func (vm *VM) lessThan(a, b value.Value) (bool, error) {
	result, err := vm.compare(a, b)
	return result == -1, err
}

func (vm *VM) looseBigIntEquals(b *big.Int, other value.Value) bool {
	if ob, ok := bigIntFromValue(other); ok {
		return b.Cmp(ob) == 0
	}
	if other.IsType(value.NULL) || other.IsType(value.UNDEFINED) {
		return false
	}
	converted, err := vm.toBigInt(other)
	if err != nil {
		return false
	}
	return b.Cmp(converted) == 0
}

func (vm *VM) looseEquals(a, b value.Value) (bool, error) {
	if ab, ok := bigIntFromValue(a); ok {
		return vm.looseBigIntEquals(ab, b), nil
	}
	if bb, ok := bigIntFromValue(b); ok {
		return vm.looseBigIntEquals(bb, a), nil
	}
	if strictEquals(a, b) {
		return true, nil
	}
	as, aString := vm.valueAsString(a)
	bs, bString := vm.valueAsString(b)
	if aString && bString {
		return as == bs, nil
	}
	if (aString && !b.IsObject()) || (bString && !a.IsObject()) {
		an, err := vm.toNumber(a)
		if err != nil {
			return false, err
		}
		bn, err := vm.toNumber(b)
		if err != nil {
			return false, err
		}
		return an == bn, nil
	}
	if (a.IsType(value.NULL) && b.IsType(value.UNDEFINED)) || (a.IsType(value.UNDEFINED) && b.IsType(value.NULL)) {
		return true, nil
	}
	if a.IsType(value.NULL) || a.IsType(value.UNDEFINED) || b.IsType(value.NULL) || b.IsType(value.UNDEFINED) {
		return false, nil
	}
	if a.IsObject() && !aString && b.IsObject() && !bString {
		return false, nil
	}
	if a.IsObject() && !aString {
		primitive, err := vm.toPrimitive(a, "default")
		if err != nil {
			return false, err
		}
		return vm.looseEquals(primitive, b)
	}
	if b.IsObject() && !bString {
		primitive, err := vm.toPrimitive(b, "default")
		if err != nil {
			return false, err
		}
		return vm.looseEquals(a, primitive)
	}
	if a.IsObject() && b.IsObject() {
		return false, nil
	}
	if a.IsBoolean() || b.IsBoolean() {
		an, err := vm.toNumber(a)
		if err != nil {
			return false, err
		}
		bn, err := vm.toNumber(b)
		if err != nil {
			return false, err
		}
		return an == bn, nil
	}
	return false, nil
}

func (vm *VM) throwValue(v value.Value, f *CallFrame, c *value.ValueChunk, ip *int) error {
	if len(vm.exceptionStack) == 0 {
		name := "Error"
		if v.IsObject() {
			if obj, err := heap.GetObject(v.GetHandle()); err == nil {
				if e, ok := obj.(*native.ObjError); ok {
					if n := e.GetMember(native.KEY_NAME); n != value.UNDEFINED {
						if ns, ok := vm.valueAsString(n); ok {
							name = ns
						}
					}
				}
			}
		}
		return &JSException{Name: name, Message: native.String(v)}
	}
	state := vm.exceptionStack[len(vm.exceptionStack)-1]
	vm.exceptionStack = vm.exceptionStack[:len(vm.exceptionStack)-1]

	vm.frameCount = state.frame
	*f = vm.currentFrame()
	*c = *f.fn.ValueChunk()
	*ip = state.jumpTo
	vm.stackTop = state.stackTop
	vm.push(v)
	return nil
}

func (vm *VM) makeError(name, message string) value.Value {
	err := native.NewError()
	err.SetMember(native.KEY_MESSAGE, value.EncodeHandle(heap.Allocate(native.NewLightString(message))))
	err.SetMember(native.KEY_NAME, value.EncodeHandle(heap.Allocate(native.NewLightString(name))))
	return value.EncodeHandle(heap.Allocate(err))
}

func (vm *VM) popN(n int) []value.Value {
	r := make([]value.Value, n)
	copy(r, vm.stack[vm.stackTop-n:vm.stackTop])
	vm.stackTop -= n
	return r
}

func (vm *VM) CreateTemplateString(o *object.ObjTemplateLiteral) value.Value {
	objStr := native.NewObjString(o.CreateString())
	return value.EncodeHandle(heap.Allocate(objStr))
}

func (vm *VM) peekN(i int) value.Value {
	return vm.stack[vm.stackTop-(i+1)]
}

func (vm *VM) findArgStart() []value.Value {
	arguments := []value.Value{}

	v := vm.pop()
	for v != value.TAG_ARGUMENT_START {
		arguments = append(arguments, v)
		v = vm.pop()
	}
	slices.Reverse(arguments)

	return arguments
}

// I think an opts struct would be good refactor here...
func (vm *VM) Call(fn object.Callable, currentIp *int, this value.Value, argCount int, calledWithSpread bool) (f CallFrame, c value.ValueChunk) {
	localStart := max(vm.stackTop-argCount, 0)
	returnIp := 0
	vm.callCounts[fn]++

	if currentIp != nil {
		returnIp = *currentIp
	}

	vm.frames[vm.frameCount].initCallFrame(fn, localStart, returnIp, this)
	vm.frameCount++

	f = vm.currentFrame()
	c = *f.fn.ValueChunk()

	if currentIp != nil && !calledWithSpread && vm.callCounts[fn] > IS_HOT_PATH && jit.IsJittable(fn, globals) {
		if flags.Debug {
			fmt.Println("-- CALLING JIT FUNCTION --")
		}

		err := jit.JITFunction(&vm.stack[f.localStart], &globals[0], fn)

		if err == nil {
			if currentIp != nil {
				*currentIp = f.returnIp
			}

			vm.frameCount--

			f = vm.currentFrame()
			c = *f.fn.ValueChunk()

			// jitted function writes to the start of locals, if this is more than one we need to pop the arguments
			if argCount > 0 {
				vm.popN(argCount - 1)
			}

			if flags.Debug {
				fmt.Println("-- JIT CALL DONE --")
			}
			return
		}

		if flags.Debug {
			fmt.Printf("failed to run JIT code %s\n", err.Error())
		}
	}

	if currentIp != nil {
		*currentIp = 0
	}

	if fn.HasArguments() {
		items := make([]value.Value, argCount)
		copy(items, vm.stack[vm.stackTop-argCount:vm.stackTop])
		if argCount > fn.GetArity() {
			vm.stackTop = f.localStart + fn.GetArity()
		}
		var arr object.Object = native.NewArrayFrom(items)
		if objFn, ok := fn.(*object.ObjFunction); ok {
			mappable := !objFn.Strict && !fn.HasRestParameter()
			for _, isDefault := range objFn.DefaultParams {
				if isDefault {
					mappable = false
					break
				}
			}
			if mappable {
				slots := []*value.Value{}
				for i := 0; i < min(fn.GetArity(), argCount); i++ {
					slots = append(slots, &vm.stack[f.localStart+i])
				}
				arr = native.NewMappedArguments(items, slots)
			}
		}
		v := value.EncodeHandle(heap.Allocate(arr))
		if objFn, ok := fn.(*object.ObjFunction); ok && objFn.ArgumentsHeapSlot >= 0 {
			f.arguments = v
			vm.frames[vm.frameCount-1].arguments = v
		} else {
			vm.push(v)
		}
	}

	if fn.HasRestParameter() {
		var arguments []value.Value
		if calledWithSpread {
			arguments = vm.findArgStart()
		} else {
			start := vm.stackTop - argCount
			arguments = append([]value.Value{}, vm.stack[start:vm.stackTop]...)
			if start > 0 && vm.stack[start-1] == value.TAG_ARGUMENT_START {
				start--
			}
			vm.stackTop = start
		}
		f.localStart = vm.stackTop

		restStart := min(fn.GetArity()-1, len(arguments))
		arr := native.NewArrayFrom(arguments[restStart:])
		for _, v := range arguments[:restStart] {
			vm.push(v)
		}
		v := value.EncodeHandle(heap.Allocate(arr))
		vm.push(v)
	}

	if calledWithSpread {
		arguments := vm.findArgStart()
		f.localStart = vm.stackTop

		for _, v := range arguments {
			vm.push(v)
		}
		vm.frames[vm.frameCount-1].localStart = f.localStart
	}

	if !fn.HasRestParameter() && !calledWithSpread {
		for vm.stackTop < f.localStart+fn.GetArity() {
			missing := value.UNDEFINED
			if objFn, ok := fn.(*object.ObjFunction); ok {
				index := vm.stackTop - f.localStart
				if index >= 0 && index < len(objFn.DefaultParams) && objFn.DefaultParams[index] {
					missing = value.UNINITIALIZED
				}
			}
			vm.push(missing)
		}
	}

	return
}

func (vm *VM) Run(wg *sync.WaitGroup) {
	wg.Add(1)
Run:
	callback, errDequeue := queue.Dequeue()
	for errDequeue == nil {
		if callback.Native != nil {
			callback.Native()
			if callback.RootKey != 0 {
				heap.ClearCallbackRoot(callback.RootKey)
			}
			callback, errDequeue = queue.Dequeue()
			continue
		}
		if len(callback.Stack) > 0 {
			for _, v := range callback.Stack {
				vm.push(v)
			}
		}

		if vm.main && scriptMode && callback.Fn.ValueChunk().FnName == object.MAIN_FN_NAME {
			callback.ThisCtx = vm.ensureGlobalObject()
		}

		var f CallFrame
		var c value.ValueChunk
		if asyncFn, ok := callback.Fn.(*native.ObjAsyncFunction); ok && asyncFn.State != nil {
			f.initCallFrame(asyncFn, asyncFn.State.LocalStart, 0, callback.ThisCtx)
			vm.frames[vm.frameCount] = f
			vm.frameCount++
			f = vm.currentFrame()
			c = *f.fn.ValueChunk()
		} else {
			f, c = vm.Call(callback.Fn, nil, callback.ThisCtx, callback.Fn.GetArity(), false)
		}
		_, err := vm.run(f, c)
		if callback.RootKey != 0 {
			heap.ClearCallbackRoot(callback.RootKey)
		}

		if err != nil {
			name := "Error"
			if jsErr, ok := err.(*JSException); ok {
				name = jsErr.Name
			}
			diagnostic.Fatal("runtime", name, err)
		}
		callback, errDequeue = queue.Dequeue()
	}

	if flags.Debug {
		fmt.Println()
		fmt.Println("-- QUEUE DRAINED --")
	}

	tick := time.NewTicker(100 * time.Millisecond)
	hasWork := eventloop.HasWork()

	if flags.Debug {
		fmt.Printf("eventloop has work: %v\n", hasWork)
	}
	for hasWork {
		select {
		case <-queue.QueueC:
			{
				goto Run
			}
		case <-tick.C:
			{
				hasWork = eventloop.HasWork()
				continue
			}
		}
	}
	wg.Done()
}

func (vm *VM) runGC() {
	if !vm.main {
		return
	}
	g := map[uint32]value.Value{}

	for _, v := range globals {
		if v.IsObject() {
			ptr := heap.GetPointer(v.GetHandle())
			g[ptr] = v
		}
	}

	glob := []value.Value{}

	for _, v := range g {
		glob = append(glob, v)
	}
	if vm.globalObject.IsObject() {
		if _, found := g[heap.GetPointer(vm.globalObject.GetHandle())]; !found {
			glob = append(glob, vm.globalObject)
		}
	}
	if vm.completionValue.IsObject() {
		glob = append(glob, vm.completionValue)
	}
	for _, v := range vm.catchValues {
		if v.IsObject() {
			glob = append(glob, v)
		}
	}
	for i := 0; i < vm.frameCount; i++ {
		scope := vm.frames[i].heapScope
		for scope > 0 {
			glob = append(glob, heap.ScopeValues(scope)...)
			scope = heap.ParentScope(scope)
		}
	}

	heap.RequestGC(append(vm.stack, glob...))
}

func (vm *VM) run(f CallFrame, c value.ValueChunk) (value.Value, error) {
	ip := f.startIp

	if promise, ok := f.fn.(*native.ObjAsyncFunction); ok {

		if promise.State != nil {
			if flags.Debug {
				fmt.Println()
				fmt.Printf("-- RETURNING PROMISE %v --\n", promise.String())
				printStack(promise.State.Stack)
				fmt.Println()
			}
			if vm.stackTop > promise.State.LocalStart {
				promise.CallerStack = append([]value.Value(nil), vm.stack[promise.State.LocalStart:vm.stackTop]...)
			} else {
				promise.CallerStack = nil
			}
			f.localStart = promise.State.LocalStart
			f.heapScope = promise.State.HeapScope
			f.ownScope = promise.State.OwnScope
			vm.stackTop = f.localStart
			vm.frames[vm.frameCount-1].localStart = f.localStart
			vm.frames[vm.frameCount-1].heapScope = f.heapScope
			vm.frames[vm.frameCount-1].ownScope = f.ownScope
			for _, exception := range promise.State.Exceptions {
				vm.exceptionStack = append(vm.exceptionStack, ExceptionState{jumpTo: exception.JumpTo, stackTop: exception.StackTop, frame: vm.frameCount, isFinally: exception.IsFinally})
			}
			for _, v := range promise.State.Stack {
				vm.push(v)
			}
			heap.ClearAsyncFunctionStack(uintptr(unsafe.Pointer(promise)))
			heap.ClearAsyncFunctionScope(uintptr(unsafe.Pointer(promise)))

			ip = promise.State.Ip
			if promise.HasRejection {
				rejectValue := promise.RejectValue
				promise.HasRejection = false
				if err := vm.throwValue(rejectValue, &f, &c, &ip); err != nil {
					if promise.Promise != nil {
						vm.rejectPromise(promise.Promise, rejectValue)
					}
					vm.frameCount--
					if vm.frameCount > 0 {
						ip = f.returnIp
						vm.stackTop = f.localStart
						for _, v := range promise.CallerStack {
							vm.push(v)
						}
						promise.CallerStack = nil
						f = vm.currentFrame()
						c = *f.fn.ValueChunk()
					} else {
						vm.stackTop = 0
						return value.UNDEFINED, nil
					}
				}
			}
		}
	}

	if flags.Debug {
		fmt.Println()
		fmt.Println("-- NEW RUNNER SPAWNED --")
		fmt.Println()
		PrintChunk(c)
	}

	for {
		// time.Sleep(time.Millisecond * 100)
		code := c.Code[ip]
		ip++

		if heap.ShouldRunGCCycle() {
			vm.runGC()
		}

		if flags.Debug {
			fmt.Println(f.fn.String())
			printStack(vm.stack[:vm.stackTop])
			fmt.Println(chunk.OpNames[code])
		}

		switch code {
		case chunk.OP_NEW_REGEXP:
			{
				patternValue := c.Constants[c.Code[ip]]
				ip++
				flagsValue := c.Constants[c.Code[ip]]
				ip++
				pattern, _ := vm.valueAsString(patternValue)
				flags, _ := vm.valueAsString(flagsValue)
				obj, err := vm.compileRegExp(pattern, flags)
				if err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(value.EncodeHandle(heap.Allocate(obj)))
			}
		case chunk.OP_CONSTANT:
			{
				vm.push(c.Constants[c.Code[ip]])
				ip++
			}
		case chunk.OP_POP:
			{
				vm.pop()
			}
		case chunk.OP_DUP:
			{
				vm.push(vm.peek())
			}
		case chunk.OP_DUP2:
			{
				a := vm.peekN(1)
				b := vm.peekN(0)
				vm.push(a)
				vm.push(b)
			}
		case chunk.OP_UNWRAP_METHOD:
			{
				v := vm.pop()
				if isMethodHandle(v) {
					v = vm.pop()
					vm.pop()
				}
				vm.push(v)
			}
		case chunk.OP_POP_BELOW2:
			{
				v := vm.pop()
				vm.pop()
				vm.pop()
				vm.push(v)
			}
		case chunk.OP_SET_MEMBER_STRICT_RETURN:
			{
				v := vm.pop()
				k := vm.pop()
				objValue := vm.pop()
				if err := vm.setMemberValueStrict(objValue, k, v, true); err != nil {
					name := "TypeError"
					message := err.Error()
					if jsErr, ok := err.(*JSException); ok {
						name = jsErr.Name
						message = jsErr.Message
					}
					errValue := vm.makeError(name, message)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(v)
			}
		case chunk.OP_SET_MEMBER_RETURN:
			{
				v := vm.pop()
				k := vm.pop()
				objValue := vm.pop()
				if err := vm.setMemberValue(objValue, k, v); err != nil {
					name := "TypeError"
					message := err.Error()
					if jsErr, ok := err.(*JSException); ok {
						name = jsErr.Name
						message = jsErr.Message
					}
					errValue := vm.makeError(name, message)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(v)
			}
		case chunk.OP_SET_MEMBER_KEEP:
			{
				v := vm.pop()
				keep := vm.pop()
				k := vm.pop()
				objValue := vm.pop()
				if err := vm.setMemberValue(objValue, k, v); err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(keep)
			}
		case chunk.OP_MEMBER_UPDATE:
			{
				prefix := c.Code[ip] == 1
				ip++
				delta := 1.0
				if c.Code[ip] == 1 {
					delta = -1
				}
				ip++
				k := vm.pop()
				objValue := vm.pop()
				old, err := vm.getMemberValue(objValue, k)
				if err != nil {
					return value.UNDEFINED, err
				}
				if b, ok := vm.bigIntObject(old); ok {
					d := big.NewInt(1)
					if delta < 0 {
						d.Neg(d)
					}
					updatedValue := newBigIntValue(new(big.Int).Add(b.Value, d))
					if err := vm.setMemberValue(objValue, k, updatedValue); err != nil {
						errValue := vm.makeError("TypeError", err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
					if prefix {
						vm.push(updatedValue)
					} else {
						vm.push(old)
					}
					continue
				}
				n, err := vm.toNumber(old)
				if err != nil {
					return value.UNDEFINED, err
				}
				updated := n + delta
				updatedValue := value.ValueFromFloat64(updated)
				if err := vm.setMemberValue(objValue, k, updatedValue); err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				if prefix {
					vm.push(updatedValue)
				} else {
					vm.push(value.ValueFromFloat64(n))
				}
			}
		case chunk.OP_PUSH_CURRENT:
			{
				vm.push(vm.peek())
			}
		case chunk.OP_ADD:
			{
				b := vm.pop()
				a := vm.pop()
				if ba, aBig := vm.bigIntObject(a); aBig {
					bb, bBig := vm.bigIntObject(b)
					if !bBig {
						if _, isString := vm.valueAsString(b); !isString {
							errValue := vm.makeError("TypeError", "Cannot mix BigInt and other types")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
					} else {
						result, _ := bigIntBinaryOp(0, ba.Value, bb.Value)
						vm.push(newBigIntValue(result))
						continue
					}
				} else if _, bBig := vm.bigIntObject(b); bBig {
					if _, isString := vm.valueAsString(a); !isString {
						errValue := vm.makeError("TypeError", "Cannot mix BigInt and other types")
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
				}

				pa, err := vm.toPrimitive(a, "default")
				if err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				pb, err := vm.toPrimitive(b, "default")
				if err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				paBuilder, aBuilder := builderFromValue(pa)
				pbBuilder, bBuilder := builderFromValue(pb)
				if aBuilder {
					if bBuilder {
						paBuilder.Concatenate(pbBuilder.String())
					} else if bs, ok := vm.valueAsString(pb); ok {
						paBuilder.Concatenate(bs)
					} else {
						bsValue, err := vm.toStringValue(pb)
						if err != nil {
							return value.UNDEFINED, err
						}
						bs, _ := vm.valueAsString(bsValue)
						paBuilder.Concatenate(bs)
					}
					vm.push(pa)
					continue
				}
				if bBuilder {
					as, ok := vm.valueAsString(pa)
					if !ok {
						asValue, err := vm.toStringValue(pa)
						if err != nil {
							return value.UNDEFINED, err
						}
						as, _ = vm.valueAsString(asValue)
					}
					result := native.NewStringBuilder(native.NewLightString(as))
					result.Concatenate(pbBuilder.String())
					vm.push(value.EncodeHandle(heap.Allocate(result)))
					continue
				}
				as, aIsString := vm.valueAsString(pa)
				bs, bIsString := vm.valueAsString(pb)
				if aIsString || bIsString {
					if !aIsString {
						asValue, err := vm.toStringValue(pa)
						if err != nil {
							return value.UNDEFINED, err
						}
						as, _ = vm.valueAsString(asValue)
					}
					if !bIsString {
						bsValue, err := vm.toStringValue(pb)
						if err != nil {
							return value.UNDEFINED, err
						}
						bs, _ = vm.valueAsString(bsValue)
					}
					result := native.NewStringBuilder(native.NewLightString(as))
					result.Concatenate(bs)
					vm.push(value.EncodeHandle(heap.Allocate(result)))
				} else {
					an, err := vm.toNumber(pa)
					if err != nil {
						return value.UNDEFINED, err
					}
					bn, err := vm.toNumber(pb)
					if err != nil {
						return value.UNDEFINED, err
					}
					vm.push(value.ValueFromFloat64(an + bn))
				}
			}
		case chunk.OP_B_AND:
			{
				b := vm.pop()
				a := vm.pop()
				if v, handled, err := vm.bigIntBitwise(0, a, b); handled {
					if err != nil {
						errValue := vm.makeError("TypeError", err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
					vm.push(v)
					continue
				}
				ai, err := vm.toInt32(a)
				if err != nil {
					return value.UNDEFINED, err
				}
				bi, err := vm.toInt32(b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(value.ValueFromFloat64(float64(ai & bi)))
			}
		case chunk.OP_B_OR:
			{
				b := vm.pop()
				a := vm.pop()
				if v, handled, err := vm.bigIntBitwise(1, a, b); handled {
					if err != nil {
						errValue := vm.makeError("TypeError", err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
					vm.push(v)
					continue
				}
				ai, err := vm.toInt32(a)
				if err != nil {
					return value.UNDEFINED, err
				}
				bi, err := vm.toInt32(b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(value.ValueFromFloat64(float64(ai | bi)))
			}
		case chunk.OP_B_XOR:
			{
				b := vm.pop()
				a := vm.pop()
				if v, handled, err := vm.bigIntBitwise(2, a, b); handled {
					if err != nil {
						errValue := vm.makeError("TypeError", err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
					vm.push(v)
					continue
				}
				ai, err := vm.toInt32(a)
				if err != nil {
					return value.UNDEFINED, err
				}
				bi, err := vm.toInt32(b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(value.ValueFromFloat64(float64(ai ^ bi)))
			}
		case chunk.OP_SUBTRACT:
			{
				b := vm.pop()
				a := vm.pop()
				if v, handled, err := vm.bigIntArithmetic(1, a, b); handled {
					if err != nil {
						name := "TypeError"
						if strings.Contains(err.Error(), "Division by zero") || strings.Contains(err.Error(), "Exponent") {
							name = "RangeError"
						}
						errValue := vm.makeError(name, err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
					vm.push(v)
					continue
				}
				an, err := vm.toNumber(a)
				if err != nil {
					return value.UNDEFINED, err
				}
				bn, err := vm.toNumber(b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(value.ValueFromFloat64(an - bn))
			}
		case chunk.OP_DIVIDE:
			{
				b := vm.pop()
				a := vm.pop()
				if v, handled, err := vm.bigIntArithmetic(3, a, b); handled {
					if err != nil {
						name := "TypeError"
						if strings.Contains(err.Error(), "Division by zero") || strings.Contains(err.Error(), "Exponent") {
							name = "RangeError"
						}
						errValue := vm.makeError(name, err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
					vm.push(v)
					continue
				}
				an, err := vm.toNumber(a)
				if err != nil {
					return value.UNDEFINED, err
				}
				bn, err := vm.toNumber(b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(value.ValueFromFloat64(an / bn))
			}
		case chunk.OP_MULTIPLY:
			{
				b := vm.pop()
				a := vm.pop()
				if v, handled, err := vm.bigIntArithmetic(2, a, b); handled {
					if err != nil {
						name := "TypeError"
						if strings.Contains(err.Error(), "Division by zero") || strings.Contains(err.Error(), "Exponent") {
							name = "RangeError"
						}
						errValue := vm.makeError(name, err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
					vm.push(v)
					continue
				}
				an, err := vm.toNumber(a)
				if err != nil {
					return value.UNDEFINED, err
				}
				bn, err := vm.toNumber(b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(value.ValueFromFloat64(an * bn))
			}
		case chunk.OP_EXPONENTIATION:
			{
				b := vm.pop()
				a := vm.pop()
				if v, handled, err := vm.bigIntArithmetic(5, a, b); handled {
					if err != nil {
						name := "TypeError"
						if strings.Contains(err.Error(), "Division by zero") || strings.Contains(err.Error(), "Exponent") {
							name = "RangeError"
						}
						errValue := vm.makeError(name, err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
					vm.push(v)
					continue
				}
				an, err := vm.toNumber(a)
				if err != nil {
					return value.UNDEFINED, err
				}
				bn, err := vm.toNumber(b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(value.ValueFromFloat64(math.Pow(an, bn)))
			}
		case chunk.OP_MODULO:
			{
				b := vm.pop()
				a := vm.pop()
				if v, handled, err := vm.bigIntArithmetic(4, a, b); handled {
					if err != nil {
						name := "TypeError"
						if strings.Contains(err.Error(), "Division by zero") || strings.Contains(err.Error(), "Exponent") {
							name = "RangeError"
						}
						errValue := vm.makeError(name, err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
					vm.push(v)
					continue
				}
				an, err := vm.toNumber(a)
				if err != nil {
					return value.UNDEFINED, err
				}
				bn, err := vm.toNumber(b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(value.ValueFromFloat64(math.Mod(an, bn)))
			}
		case chunk.OP_LESS_THAN:
			{
				b := vm.pop()
				a := vm.pop()
				cmp, err := vm.compare(a, b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(boolValue(cmp == -1))
			}
		case chunk.OP_LESS_THAN_EQUAL:
			{
				b := vm.pop()
				a := vm.pop()
				cmp, err := vm.compare(a, b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(boolValue(cmp == -1 || cmp == 0))
			}
		case chunk.OP_GREATER_THAN:
			{
				b := vm.pop()
				a := vm.pop()
				cmp, err := vm.compare(a, b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(boolValue(cmp == 1))
			}
		case chunk.OP_GREATER_THAN_EQUAL:
			{
				b := vm.pop()
				a := vm.pop()
				cmp, err := vm.compare(a, b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(boolValue(cmp == 1 || cmp == 0))
			}
		case chunk.OP_EQUALS:
			{
				b := vm.popUnwrapped()
				a := vm.popUnwrapped()
				result, err := vm.looseEquals(a, b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(boolValue(result))
			}
		case chunk.OP_NOT_EQUALS:
			{
				b := vm.popUnwrapped()
				a := vm.popUnwrapped()
				result, err := vm.looseEquals(a, b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(boolValue(!result))
			}
		case chunk.OP_STRICT_EQUALS:
			{
				b := vm.popUnwrapped()
				a := vm.popUnwrapped()

				vm.push(boolValue(strictEquals(a, b)))
			}
		case chunk.OP_STRICT_NOT_EQUALS:
			{
				b := vm.popUnwrapped()
				a := vm.popUnwrapped()

				vm.push(boolValue(!strictEquals(a, b)))
			}
		case chunk.OP_LOGICAL_OR:
			{
				right := vm.pop()
				left := vm.pop()

				if truthyValue(left) {
					vm.push(left)
				} else {
					vm.push(right)
				}
			}
		case chunk.OP_LOGICAL_AND:
			{
				right := vm.pop()
				left := vm.pop()

				if truthyValue(left) {
					vm.push(right)
				} else {
					vm.push(left)
				}
			}
		case chunk.OP_IN:
			{
				obj := vm.pop()
				prop := vm.pop()

				has, err := vm.hasProperty(obj, prop)
				if err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(boolValue(has))
			}
		case chunk.OP_NULL_COALESHING:
			{
				right := vm.pop()
				left := vm.pop()

				if left == value.NULL || left == value.UNDEFINED {
					vm.push(right)
				} else {
					vm.push(left)
				}
			}
		case chunk.OP_TO_NUMBER:
			{
				v := vm.pop()
				if _, ok := vm.bigIntObject(v); ok {
					errValue := vm.makeError("TypeError", "Cannot convert a BigInt value to a number")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				n, err := vm.toNumber(v)
				if err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(value.ValueFromFloat64(n))
			}
		case chunk.OP_UPDATE:
			{
				decrement := c.Code[ip] == 1
				ip++
				v := vm.pop()
				if b, ok := vm.bigIntObject(v); ok {
					delta := big.NewInt(1)
					if decrement {
						delta.Neg(delta)
					}
					vm.push(newBigIntValue(new(big.Int).Add(b.Value, delta)))
					continue
				}
				n, err := vm.toNumber(v)
				if err != nil {
					return value.UNDEFINED, err
				}
				if decrement {
					n--
				} else {
					n++
				}
				vm.push(value.ValueFromFloat64(n))
			}
		case chunk.OP_TO_PROPERTY_KEY:
			{
				key, err := vm.toPropertyKey(vm.pop())
				if err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(key)
			}
		case chunk.OP_TO_STRING:
			{
				s, err := vm.toStringValue(vm.pop())
				if err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(s)
			}
		case chunk.OP_TYPEOF:
			{
				v := vm.pop()
				if isMethodHandle(v) {
					v = vm.pop()
					vm.pop()
				}
				vm.push(value.EncodeHandle(heap.Allocate(native.NewLightString(vm.typeofValue(v)))))
			}
		case chunk.OP_VOID:
			{
				vm.pop()
				vm.push(value.UNDEFINED)
			}
		case chunk.OP_BITWISE_NOT:
			{
				v := vm.pop()
				if b, ok := vm.bigIntObject(v); ok {
					vm.push(newBigIntValue(new(big.Int).Not(b.Value)))
					continue
				}
				n, err := vm.toInt32(v)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(value.ValueFromFloat64(float64(^n)))
			}
		case chunk.OP_LEFT_SHIFT:
			{
				b := vm.pop()
				a := vm.pop()
				if v, handled, err := vm.bigIntBitwise(3, a, b); handled {
					if err != nil {
						errValue := vm.makeError("TypeError", err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
					vm.push(v)
					continue
				}
				ai, err := vm.toInt32(a)
				if err != nil {
					return value.UNDEFINED, err
				}
				bi, err := vm.toUint32(b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(value.ValueFromFloat64(float64(ai << (bi & 0x1f))))
			}
		case chunk.OP_RIGHT_SHIFT:
			{
				b := vm.pop()
				a := vm.pop()
				if v, handled, err := vm.bigIntBitwise(4, a, b); handled {
					if err != nil {
						errValue := vm.makeError("TypeError", err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
					vm.push(v)
					continue
				}
				ai, err := vm.toInt32(a)
				if err != nil {
					return value.UNDEFINED, err
				}
				bi, err := vm.toUint32(b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(value.ValueFromFloat64(float64(ai >> (bi & 0x1f))))
			}
		case chunk.OP_UNSIGNED_RIGHT_SHIFT:
			{
				b := vm.pop()
				a := vm.pop()
				if _, aBig := vm.bigIntObject(a); aBig {
					errValue := vm.makeError("TypeError", "BigInts have no unsigned right shift")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				if _, bBig := vm.bigIntObject(b); bBig {
					errValue := vm.makeError("TypeError", "BigInts have no unsigned right shift")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				ai, err := vm.toUint32(a)
				if err != nil {
					return value.UNDEFINED, err
				}
				bi, err := vm.toUint32(b)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(value.ValueFromFloat64(float64(ai >> (bi & 0x1f))))
			}
		case chunk.OP_DELETE:
			{
				k := vm.pop()
				objValue := vm.pop()
				result, err := vm.deletePropertyValue(objValue, k)
				if err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(boolValue(result))
			}
		case chunk.OP_DELETE_STRICT:
			{
				k := vm.pop()
				objValue := vm.pop()
				result, err := vm.deletePropertyValue(objValue, k)
				if err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				if !result {
					errValue := vm.makeError("TypeError", "Cannot delete property")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(value.TRUE)
			}
		case chunk.OP_INSTANCEOF:
			{
				ctor := vm.pop()
				objValue := vm.pop()
				result, err := vm.instanceOf(objValue, ctor)
				if err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(boolValue(result))
			}
		case chunk.OP_SAVE_COMPLETION:
			{
				vm.completionValue = vm.pop()
				vm.completionPending = true
			}
		case chunk.OP_RESTORE_COMPLETION:
			{
				vm.push(vm.completionValue)
				vm.completionPending = false
			}
		case chunk.OP_PUSH_UNINITIALIZED:
			{
				vm.push(value.UNINITIALIZED)
			}
		case chunk.OP_PUSH_GLOBAL_OBJECT:
			{
				vm.push(vm.ensureGlobalObject())
			}
		case chunk.OP_GET_GLOBAL_BY_NAME:
			{
				name := c.Constants[c.Code[ip]]
				ip++
				global := vm.ensureGlobalObject()
				obj, _ := heap.GetObject(global.GetHandle())
				hashable := obj.(object.Hashable)
				if !hashable.HasOwn(name) {
					nameString, _ := vm.valueAsString(name)
					errValue := vm.makeError("ReferenceError", nameString+" is not defined")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				v := hashable.GetMember(name)
				vm.push(v)
			}
		case chunk.OP_NEGATE:
			{
				v := vm.pop()
				if b, ok := vm.bigIntObject(v); ok {
					vm.push(newBigIntValue(new(big.Int).Neg(b.Value)))
					continue
				}

				if v.IsNumber() {
					vm.push(value.ValueFromFloat64(-(v.AsNumber())))
				} else {
					return value.UNDEFINED, fmt.Errorf("no support for negating %s yet", native.String(v))
				}
			}
		case chunk.OP_ARG_START:
			{
				vm.push(value.TAG_ARGUMENT_START)
			}
		case chunk.OP_SPREAD:
			{
				v := vm.pop()
				iterator, err := vm.acquireIterator(v, compiler.ITERATOR_FOR_OF)
				if err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				for {
					done, err := vm.iteratorNext(iterator)
					if err != nil {
						return value.UNDEFINED, err
					}
					if done {
						break
					}
					item, err := vm.iteratorCurrent(iterator)
					if err != nil {
						return value.UNDEFINED, err
					}
					vm.push(item)
				}
			}
		case chunk.OP_SPREAD_INTO_ARRAY:
			{
				v := vm.pop()
				arrayValue := vm.peek()
				obj, err := heap.GetObject(arrayValue.GetHandle())
				if err != nil {
					return value.UNDEFINED, err
				}
				arr, ok := obj.(*native.ObjArr)
				if !ok {
					return value.UNDEFINED, fmt.Errorf("spread target is not an array")
				}
				iterator, err := vm.acquireIterator(v, compiler.ITERATOR_FOR_OF)
				if err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				for {
					done, err := vm.iteratorNext(iterator)
					if err != nil {
						return value.UNDEFINED, err
					}
					if done {
						break
					}
					item, err := vm.iteratorCurrent(iterator)
					if err != nil {
						return value.UNDEFINED, err
					}
					arr.PushElement(item)
				}
			}
		case chunk.OP_JUMP_IF_FALSE:
			{
				v := vm.pop()
				jump := c.ReadInt(&ip)

				if !truthyValue(v) {
					ip = jump
				}
			}
		case chunk.OP_JUMP_IF_TRUE:
			{
				v := vm.pop()
				jump := c.ReadInt(&ip)

				if truthyValue(v) {
					ip = jump
				}
			}
		case chunk.OP_JUMP_IF_NULLISH:
			{
				v := vm.pop()
				jump := c.ReadInt(&ip)

				if v == value.NULL || v == value.UNDEFINED {
					ip = jump
				}
			}
		case chunk.OP_JUMP_IF_NOT_UNDEFINED:
			{
				v := vm.pop()
				jump := c.ReadInt(&ip)

				if v != value.UNDEFINED && v != value.UNINITIALIZED {
					ip = jump
				}
			}
		case chunk.OP_JUMP_IF_NOT_NULLISH:
			{
				v := vm.pop()
				jump := c.ReadInt(&ip)

				if v != value.NULL && v != value.UNDEFINED {
					ip = jump
				}
			}
		case chunk.OP_JUMP:
			{
				jump := c.ReadInt(&ip)
				ip = jump
			}
		case chunk.OP_DEFINE_HEAP_VAR:
			{
				variable := vm.pop()
				if variable == value.TAG_METHOD_HANDLE {
					variable = vm.pop()
					vm.pop() // pop this context
				}
				variable = cloneFunctionValue(variable, f.heapScope)
				heap.DefineHeapVar(f.heapScope, variable)
			}
		case chunk.OP_DEFINE_HEAP_VAR_AT:
			{
				slot := int(c.Code[ip])
				ip++
				variable := vm.pop()
				if variable == value.TAG_METHOD_HANDLE {
					variable = vm.pop()
					vm.pop() // pop this context
				}
				variable = cloneFunctionValue(variable, f.heapScope)
				heap.DefineHeapVarAt(f.heapScope, slot, variable)
			}
		case chunk.OP_DEFINE_ARGUMENTS_HEAP:
			{
				slot := int(c.Code[ip])
				ip++
				heap.DefineHeapVarAt(f.heapScope, slot, f.arguments)
			}
		case chunk.OP_GET_HEAP_VAR_RAW:
			{
				depth := int(c.Code[ip])
				ip++
				slot := int(c.Code[ip])
				ip++
				vm.push(heap.GetHeapVar(f.heapScope, depth, slot))
			}
		case chunk.OP_GET_HEAP_VAR:
			{
				depth := int(c.Code[ip])
				ip++
				slot := c.Code[ip]
				ip++
				if !heap.HasHeapVar(f.heapScope, depth, int(slot)) {
					errValue := vm.makeError("ReferenceError", "Cannot access variable before initialization")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				v := heap.GetHeapVar(f.heapScope, depth, int(slot))
				if v == value.UNINITIALIZED {
					errValue := vm.makeError("ReferenceError", "Cannot access variable before initialization")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(v)
			}
		case chunk.OP_SET_HEAP_VAR:
			{
				depth := int(c.Code[ip])
				ip++
				slot := c.Code[ip]
				ip++
				val := vm.pop()
				heap.SetHeapVar(f.heapScope, depth, int(slot), cloneFunctionValue(val, f.heapScope))
			}
		case chunk.OP_DEFINE_GLOBAL:
			{
				v := vm.pop()
				if v == value.TAG_METHOD_HANDLE {
					v = vm.pop()
					vm.pop() // pop this context
				}
				if v.IsObject() {
					obj, err := heap.GetObject(v.GetHandle())

					if err != nil {
						return value.UNDEFINED, fmt.Errorf("failed to receive object at OP_DEFINE_GLOBAL %s", err)
					}
					switch obj := obj.(type) {
					case *object.ObjFunction:
						{
							v = cloneFunctionValue(v, f.heapScope)
						}
					case *native.ObjStringBuilder:
						{
							v = value.EncodeHandle(heap.Allocate(obj.Flush()))
						}
					}
					if fn, ok := obj.(*object.ObjFunction); ok && vm.main && scriptMode && fn.Name != "ANONYMOYS_FN" && fn.Name != object.MAIN_FN_NAME && fn.Name != object.SETUP_FN_NAME {
						global := vm.ensureGlobalObject()
						globalObj, _ := heap.GetObject(global.GetHandle())
						globalObj.(object.Hashable).SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString(fn.Name))), v)
					}
				}
				vm.addGlobal(v)
			}
		case chunk.OP_DEFINE_GLOBAL_NAMED:
			{
				name := c.Constants[c.Code[ip]]
				ip++
				v := vm.pop()
				vm.addGlobal(v)
				global := vm.ensureGlobalObject()
				globalObj, _ := heap.GetObject(global.GetHandle())
				globalObj.(object.Hashable).SetMember(name, v)
			}
		case chunk.OP_GET_GLOBAL:
			{
				global := c.Code[ip]
				ip++
				if int(global) >= globalsCount {
					errValue := vm.makeError("ReferenceError", "Cannot access variable before initialization")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				v := vm.getGlobal(int(global))
				if v == value.UNINITIALIZED {
					errValue := vm.makeError("ReferenceError", "Cannot access variable before initialization")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(v)
			}
		case chunk.OP_SET_GLOBAL:
			{
				global := c.Code[ip]
				ip++
				if int(global) >= globalsCount {
					for len(globals) <= int(global) {
						globals = append(globals, value.UNINITIALIZED)
					}
					globalsCount = int(global) + 1
				}
				v := vm.pop()
				if v.IsObject() {
					obj, err := heap.GetObject(v.GetHandle())

					if err != nil {
						return value.UNDEFINED, fmt.Errorf("failed to receive object at OP_DEFINE_GLOBAL %s", err)
					}
					switch obj.(type) {
					case *object.ObjFunction:
						{
							v = cloneFunctionValue(v, f.heapScope)
						}
					}
				}
				globals[global] = v
				if name, ok := globalVarNames[int(global)]; ok {
					globalValue := vm.ensureGlobalObject()
					globalObj, _ := heap.GetObject(globalValue.GetHandle())
					globalObj.(object.Hashable).SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString(name))), v)
				}
			}
		case chunk.OP_SET_GLOBAL_NAMED:
			{
				global := c.Code[ip]
				ip++
				name := c.Constants[c.Code[ip]]
				ip++
				if int(global) >= globalsCount {
					for len(globals) <= int(global) {
						globals = append(globals, value.UNINITIALIZED)
					}
					globalsCount = int(global) + 1
				}
				v := vm.pop()
				if v.IsObject() {
					obj, err := heap.GetObject(v.GetHandle())
					if err != nil {
						return value.UNDEFINED, fmt.Errorf("failed to receive object at OP_SET_GLOBAL_NAMED %s", err)
					}
					if _, ok := obj.(*object.ObjFunction); ok {
						v = cloneFunctionValue(v, f.heapScope)
					}
				}
				globals[global] = v
				globalValue := vm.ensureGlobalObject()
				globalObj, _ := heap.GetObject(globalValue.GetHandle())
				if nameString, ok := vm.valueAsString(name); ok {
					globalVarSlots[nameString] = int(global)
					globalVarNames[int(global)] = nameString
				}
				if obj, ok := globalObj.(*native.ObjObject); ok {
					obj.DefineNonConfigurableMember(name, v)
				} else {
					globalObj.(object.Hashable).SetMember(name, v)
				}
			}
		case chunk.OP_POP_LOCAL:
			{
				if vm.stackTop > f.localStart {
					vm.stackTop--
				}
			}
		case chunk.OP_CATCH_BIND:
			{
				id := int(c.Code[ip])
				ip++
				vm.catchValues[id] = vm.pop()
			}
		case chunk.OP_GET_CATCH:
			{
				id := int(c.Code[ip])
				ip++
				v, found := vm.catchValues[id]
				if !found {
					v = value.UNDEFINED
				}
				vm.push(v)
			}
		case chunk.OP_SET_CATCH:
			{
				id := int(c.Code[ip])
				ip++
				vm.catchValues[id] = vm.pop()
			}
		case chunk.OP_DEFINE_LOCAL:
			{
				v := vm.pop()
				if v == value.TAG_METHOD_HANDLE {
					v = vm.pop()
					vm.pop() // pop this context
				}

				if v.IsObject() {
					v = cloneFunctionValue(v, f.heapScope)
				}
				vm.stack[vm.stackTop] = v
				vm.stackTop++
			}
		case chunk.OP_DEFINE_LOCAL_SLOT:
			{
				slot := int(c.Code[ip])
				ip++
				v := vm.pop()
				if v == value.TAG_METHOD_HANDLE {
					v = vm.pop()
					vm.pop()
				}
				if v.IsObject() {
					v = cloneFunctionValue(v, f.heapScope)
				}
				target := f.localStart + slot
				for vm.stackTop <= target {
					vm.stack[vm.stackTop] = value.UNDEFINED
					vm.stackTop++
				}
				vm.stack[target] = v
				if vm.stackTop <= target {
					vm.stackTop = target + 1
				}
			}
		case chunk.OP_GET_LOCAL_RAW:
			{
				slot := int(c.Code[ip])
				ip++
				vm.push(vm.stack[f.localStart+slot])
			}
		case chunk.OP_GET_LOCAL:
			{
				slot := int(c.Code[ip])
				ip++
				v := vm.stack[f.localStart+slot]
				if v == value.UNINITIALIZED {
					errValue := vm.makeError("ReferenceError", "Cannot access variable before initialization")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(v)
			}
		case chunk.OP_SET_LOCAL:
			{
				slot := int(c.Code[ip])
				ip++
				v := vm.pop()
				if v.IsObject() {
					obj, err := heap.GetObject(v.GetHandle())

					if err != nil {
						return value.UNDEFINED, fmt.Errorf("failed to receive object at OP_DEFINE_GLOBAL %s", err)
					}
					switch obj := obj.(type) {
					case *object.ObjFunction:
						{
							v = cloneFunctionValue(v, f.heapScope)
						}
					case *native.ObjStringBuilder:
						{
							v = value.EncodeHandle(heap.Allocate(obj.Flush()))
						}
					}
				}
				vm.stack[f.localStart+slot] = v
			}
		case chunk.OP_CREATE_OBJECT:
			{
				objHash := native.NewObjectHash()
				handle := heap.Allocate(objHash)

				vm.push(value.EncodeHandle(handle))
			}
		case chunk.OP_SET_OBJECT_PROTOTYPE:
			{
				proto := vm.pop()
				objValue := vm.peek()
				if objValue.IsObject() {
					if obj, err := heap.GetObject(objValue.GetHandle()); err == nil {
						if o, ok := obj.(interface{ SetPrototypeValue(value.Value) }); ok {
							o.SetPrototypeValue(proto)
						}
					}
				}
			}
		case chunk.OP_SET_OBJECT_GETTER:
			{
				getter := vm.pop()
				k := vm.pop()
				getter = cloneFunctionValue(getter, f.heapScope)
				objValue := vm.peek()
				if objValue.IsObject() {
					obj, _ := heap.GetObject(objValue.GetHandle())
					if o, ok := obj.(*native.ObjObject); ok {
						o.SetAccessor(k, getter, value.UNDEFINED)
					}
				}
			}
		case chunk.OP_SET_OBJECT_SETTER:
			{
				setter := vm.pop()
				k := vm.pop()
				setter = cloneFunctionValue(setter, f.heapScope)
				objValue := vm.peek()
				if objValue.IsObject() {
					obj, _ := heap.GetObject(objValue.GetHandle())
					if o, ok := obj.(*native.ObjObject); ok {
						o.SetAccessor(k, value.UNDEFINED, setter)
					}
				}
			}
		case chunk.OP_SET_OBJECT_MEMBER:
			{
				v := vm.pop()
				k := vm.pop()

				hash := vm.peek()

				if !hash.IsObject() {
					return value.UNDEFINED, fmt.Errorf("%v is not an object", hash)
				}

				if v.IsObject() {
					obj, _ := heap.GetObject(v.GetHandle())

					// check if we have a closure in hand
					if fn, ok := obj.(*object.ObjFunction); ok {
						v = cloneFunctionValue(v, f.heapScope)
						if fn.IsMethod {
							if cloned, err := heap.GetObject(v.GetHandle()); err == nil {
								if clonedFn, ok := cloned.(*object.ObjFunction); ok {
									clonedFn.HomeObject = hash
								}
							}
						}
					}
				}

				if k.IsObject() {
					keyObj, _ := heap.GetObject(k.GetHandle())
					if b, ok := keyObj.(*native.ObjStringBuilder); ok {
						key := b.Flush()
						k = value.EncodeHandle(heap.Allocate(key))
					}
				}

				if err := vm.setMemberValue(hash, k, v); err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
			}
		case chunk.OP_GET_OBJECT_MEMBER:
			{
				member := vm.pop()
				objValue := vm.pop()

				if objValue == value.TAG_METHOD_HANDLE {
					objValue = vm.pop()
					vm.pop() // pop this context
				}

				if objValue == value.UNDEFINED || objValue == value.NULL {
					errValue := vm.makeError("TypeError", "Cannot read properties of null")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}

				if objValue == value.UNINITIALIZED {
					errValue := vm.makeError("ReferenceError", "Cannot access uninitialized variable")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}

				v, err := vm.getMemberValue(objValue, member)
				if err != nil {
					name := "TypeError"
					message := err.Error()
					if jsErr, ok := err.(*JSException); ok {
						name = jsErr.Name
						message = jsErr.Message
					}
					errValue := vm.makeError(name, message)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}

				if v.IsObject() {
					memberObj, err := heap.GetObject(v.GetHandle())
					if err == nil {
						if _, ok := memberObj.(object.NeedsContext); ok {
							vm.push(objValue)
							vm.push(v)
							vm.push(value.TAG_METHOD_HANDLE)
							continue
						}
					}
				}
				vm.push(v)
			}
		case chunk.OP_PUSH_UNDEFINED:
			{
				vm.push(value.UNDEFINED)
			}
		case chunk.OP_CALL:
			{
				argCount := int(c.Code[ip])
				ip++
				spread := c.Code[ip]
				ip++

				calledWithSpread := false
				if spread == 1 {
					calledWithSpread = true
				}
				callee := vm.pop()
				thisCtx := value.UNDEFINED

				if callee == value.TAG_METHOD_HANDLE {
					callee = vm.pop()
					thisCtx = vm.pop()
				}

				fn, err := heap.GetObject(callee.GetHandle())

				if err != nil {
					return value.UNDEFINED, err
				}

				switch fn := fn.(type) {
				case *native.ObjProxy:
					{
						args := vm.popN(argCount)
						result, err := vm.proxyApply(fn, thisCtx, args)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.ProxyConstructor:
					{
						errValue := vm.makeError("TypeError", "Constructor Proxy requires 'new'")
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
				case *native.ProxyRevocable:
					{
						args := vm.popN(argCount)
						if len(args) < 2 || !args[0].IsObject() || !args[1].IsObject() {
							errValue := vm.makeError("TypeError", "Cannot create proxy with a non-object as target or handler")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						proxy := native.NewObjProxy(args[0], args[1])
						proxyValue := value.EncodeHandle(heap.Allocate(proxy))
						result := native.NewObjectHash()
						result.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("proxy"))), proxyValue)
						result.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("revoke"))), value.EncodeHandle(heap.Allocate(native.NewProxyRevoke(proxy))))
						vm.push(value.EncodeHandle(heap.Allocate(result)))
					}
				case *native.ProxyRevoke:
					{
						fn.Proxy.Revoked = true
						vm.push(value.UNDEFINED)
					}
				case *native.ObjBoundFunction:
					{
						args := vm.popN(argCount)
						target := fn.Target
						allArgs := append(append([]value.Value{}, fn.Args...), args...)
						if targetObj, err := heap.GetObject(target.GetHandle()); err == nil {
							if inner, ok := targetObj.(*native.ObjBoundFunction); ok {
								target = inner.Target
								allArgs = append(append([]value.Value{}, inner.Args...), allArgs...)
							}
						}
						targetObj, err := heap.GetObject(target.GetHandle())
						if err != nil {
							errValue := vm.makeError("TypeError", "Value is not callable")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						callable, ok := targetObj.(object.Callable)
						if !ok {
							errValue := vm.makeError("TypeError", "Value is not callable")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						for _, arg := range allArgs {
							vm.push(arg)
						}
						f, c = vm.Call(callable, &ip, vm.normalizeThis(callable, fn.This), len(allArgs), false)
					}
				case *native.ObjAsyncFunction:
					{
						fn = fn.Clone().(*native.ObjAsyncFunction)
						if !fn.ReturnArgumentIsPromise {
							promise := native.NewPromise()
							fn.SetPromise(promise)
							base := vm.stackTop - argCount
							copy(vm.stack[base+1:vm.stackTop+1], vm.stack[base:vm.stackTop])
							vm.stack[base] = value.EncodeHandle(heap.Allocate(promise))
							vm.stackTop++
						}

						f, c = vm.Call(fn.Clone(), &ip, value.UNDEFINED, argCount, calledWithSpread)
					}
				case *native.ObjGenerator:
					{
						args := vm.popN(argCount)
						gen := fn.Clone().(*native.ObjGenerator)
						gen.Args = args
						vm.push(value.EncodeHandle(heap.Allocate(gen)))
					}
				case object.Callable:
					{
						callThis := thisCtx
						if fnObj, ok := fn.(*object.ObjFunction); ok {
							if vm.main && scriptMode && !fnObj.Strict && callThis == value.UNDEFINED {
								callThis = vm.ensureGlobalObject()
							}
						}
						if vm.main && scriptMode && fn.ValueChunk().FnName == object.MAIN_FN_NAME {
							callThis = vm.ensureGlobalObject()
						}
						f, c = vm.Call(fn, &ip, callThis, argCount, calledWithSpread)
					}
				case *native.Resolve:
					{
						// kinda ugly here, I 'compile' an async function that only returns so that it resolves the promise immediatly
						v := vm.pop()
						fn := native.NewAsyncFunction("RESOLVED_FN", 0, nil)
						fn.ValueChunk().EmitBytes(chunk.OP_RETURN)
						promise := native.NewPromise()
						fn.SetPromise(promise)
						vm.push(value.EncodeHandle(heap.Allocate(promise)))

						f, c = vm.Call(fn, &ip, value.UNDEFINED, 0, calledWithSpread)
						vm.push(v)
					}
				case *native.Then:
					{
						args := vm.popN(argCount)
						pObj, _ := heap.GetObject(thisCtx.GetHandle())
						p := pObj.(*native.ObjPromise)
						derived := native.NewPromise()
						derivedValue := value.EncodeHandle(heap.Allocate(derived))
						onFulfilled := value.UNDEFINED
						onRejected := value.UNDEFINED
						if len(args) > 0 {
							onFulfilled = args[0]
						}
						if len(args) > 1 {
							onRejected = args[1]
						}
						vm.attachPromiseReaction(p, native.PromiseReaction{OnFulfilled: onFulfilled, OnRejected: onRejected, Derived: derivedValue})
						vm.push(derivedValue)
					}
				case *native.PromiseCatch:
					{
						args := vm.popN(argCount)
						pObj, _ := heap.GetObject(thisCtx.GetHandle())
						p := pObj.(*native.ObjPromise)
						derived := native.NewPromise()
						derivedValue := value.EncodeHandle(heap.Allocate(derived))
						onRejected := value.UNDEFINED
						if len(args) > 0 {
							onRejected = args[0]
						}
						vm.attachPromiseReaction(p, native.PromiseReaction{OnRejected: onRejected, Derived: derivedValue})
						vm.push(derivedValue)
					}
				case *native.PromiseFinally:
					{
						args := vm.popN(argCount)
						pObj, _ := heap.GetObject(thisCtx.GetHandle())
						p := pObj.(*native.ObjPromise)
						derived := native.NewPromise()
						derivedValue := value.EncodeHandle(heap.Allocate(derived))
						finalizer := value.UNDEFINED
						if len(args) > 0 {
							finalizer = args[0]
						}
						vm.attachPromiseReaction(p, native.PromiseReaction{Finalizer: finalizer, IsFinally: true, Derived: derivedValue})
						vm.push(derivedValue)
					}
				case *native.PromiseResolve:
					{
						args := vm.popN(argCount)
						if len(args) > 0 && args[0].IsObject() {
							if obj, err := heap.GetObject(args[0].GetHandle()); err == nil {
								if _, ok := obj.(*native.ObjPromise); ok {
									vm.push(args[0])
									break
								}
							}
						}
						p := native.NewPromise()
						pValue := value.EncodeHandle(heap.Allocate(p))
						argument := value.UNDEFINED
						if len(args) > 0 {
							argument = args[0]
						}
						vm.resolvePromise(p, argument)
						vm.push(pValue)
					}
				case *native.PromiseReject:
					{
						args := vm.popN(argCount)
						p := native.NewPromise()
						reason := value.UNDEFINED
						if len(args) > 0 {
							reason = args[0]
						}
						vm.rejectPromise(p, reason)
						vm.push(value.EncodeHandle(heap.Allocate(p)))
					}
				case *native.PromiseAll:
					{
						result, err := vm.callPromiseAll(argCount, thisCtx)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.PromiseAllSettled:
					{
						result, err := vm.callPromiseAllSettled(argCount)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.PromiseRace:
					{
						result, err := vm.callPromiseRace(argCount)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.PromiseAny:
					{
						result, err := vm.callPromiseAny(argCount)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.ArrayBufferIsView:
					{
						args := vm.popN(argCount)
						result := false
						if len(args) > 0 && args[0].IsObject() {
							if obj, err := heap.GetObject(args[0].GetHandle()); err == nil {
								switch obj.(type) {
								case *native.ObjTypedArray, *native.ObjDataView:
									result = true
								}
							}
						}
						vm.push(boolValue(result))
					}
				case *native.ArrayBufferSlice:
					{
						args := vm.popN(argCount)
						buffer, _ := vm.arrayBufferFromValue(thisCtx)
						length := len(buffer.Data)
						start := 0
						end := length
						if len(args) > 0 {
							n, _ := vm.toNumber(args[0])
							start = int(n)
							if start < 0 {
								start = max(0, length+start)
							}
							start = min(start, length)
						}
						if len(args) > 1 && args[1] != value.UNDEFINED {
							n, _ := vm.toNumber(args[1])
							end = int(n)
							if end < 0 {
								end = max(0, length+end)
							}
							end = min(end, length)
						}
						if end < start {
							end = start
						}
						result := native.NewArrayBuffer(end - start)
						copy(result.Data, buffer.Data[start:end])
						vm.push(value.EncodeHandle(heap.Allocate(result)))
					}
				case *native.TypedArrayJoin:
					{
						args := vm.popN(argCount)
						separator := ","
						if len(args) > 0 && args[0] != value.UNDEFINED {
							separator = native.String(args[0])
						}
						array, _ := vm.typedArrayFromValue(thisCtx)
						vm.push(value.EncodeHandle(heap.Allocate(native.NewLightString(vm.typedArrayJoin(array, separator)))))
					}
				case *native.TypedArraySet:
					{
						args := vm.popN(argCount)
						array, _ := vm.typedArrayFromValue(thisCtx)
						offset := 0
						if len(args) > 1 {
							n, _ := vm.toNumber(args[1])
							offset = int(n)
						}
						source := value.UNDEFINED
						if len(args) > 0 {
							source = args[0]
						}
						if err := vm.typedArraySet(array, source, offset); err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(value.UNDEFINED)
					}
				case *native.TypedArraySubarray:
					{
						args := vm.popN(argCount)
						array, _ := vm.typedArrayFromValue(thisCtx)
						start := 0
						end := array.Length
						if len(args) > 0 {
							n, _ := vm.toNumber(args[0])
							start = int(n)
						}
						if len(args) > 1 && args[1] != value.UNDEFINED {
							n, _ := vm.toNumber(args[1])
							end = int(n)
						}
						if start < 0 {
							start = max(0, array.Length+start)
						}
						if end < 0 {
							end = max(0, array.Length+end)
						}
						start = min(start, array.Length)
						end = min(max(start, end), array.Length)
						length := end - start
						result := native.NewTypedArray(array.Buffer, array.BufferValue, array.Offset+start*array.ElementSize(), length, array.Kind)
						vm.push(value.EncodeHandle(heap.Allocate(result)))
					}
				case *native.TypedArraySlice:
					{
						args := vm.popN(argCount)
						array, _ := vm.typedArrayFromValue(thisCtx)
						start := 0
						end := array.Length
						if len(args) > 0 {
							n, _ := vm.toNumber(args[0])
							start = int(n)
						}
						if len(args) > 1 && args[1] != value.UNDEFINED {
							n, _ := vm.toNumber(args[1])
							end = int(n)
						}
						result := vm.typedArraySlice(array, start, end)
						vm.push(value.EncodeHandle(heap.Allocate(result)))
					}
				case *native.TypedArraySort:
					{
						vm.popN(argCount)
						array, _ := vm.typedArrayFromValue(thisCtx)
						vm.typedArraySort(array)
						vm.push(thisCtx)
					}
				case *native.TypedArrayMap:
					{
						args := vm.popN(argCount)
						array, _ := vm.typedArrayFromValue(thisCtx)
						callback := value.UNDEFINED
						thisArg := value.UNDEFINED
						if len(args) > 0 {
							callback = args[0]
						}
						if len(args) > 1 {
							thisArg = args[1]
						}
						result, err := vm.typedArrayMap(array, callback, thisArg)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.DataViewMethod:
					{
						args := vm.popN(argCount)
						view, _ := vm.dataViewFromValue(thisCtx)
						result, err := vm.callDataViewMethod(view, fn, args)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.RegExpToString:
					{
						vm.popN(argCount)
						text := "/(?:)/"
						if re, ok := vm.regexpFromValue(thisCtx); ok {
							text = re.String()
						}
						vm.push(value.EncodeHandle(heap.Allocate(native.NewLightString(text))))
					}
				case *native.RegExpSymbolMethod:
					{
						args := vm.popN(argCount)
						re, ok := vm.regexpFromValue(thisCtx)
						if !ok {
							errValue := vm.makeError("TypeError", "RegExp method called on incompatible receiver")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						owner := ""
						if len(args) > 0 {
							owner = mustString(vm, args[0])
						}
						var result value.Value
						var err error
						switch fn.Kind {
						case native.REGEXP_SYMBOL_MATCH:
							result, err = vm.regexpSymbolMatch(re, owner)
						case native.REGEXP_SYMBOL_REPLACE:
							replacement := value.UNDEFINED
							if len(args) > 1 {
								replacement = args[1]
							}
							result, err = vm.stringReplaceRegex(owner, re, replacement, strings.Contains(re.Flags, "g"))
						case native.REGEXP_SYMBOL_SEARCH:
							match, findErr := vm.regexpFind(re, newRegexpText(owner, re.Unicode), 0, false)
							err = findErr
							if match == nil {
								result = value.ValueFromFloat64(-1)
							} else {
								result = value.ValueFromFloat64(float64(newRegexpText(owner, re.Unicode).utf16Index(match.Index)))
							}
						case native.REGEXP_SYMBOL_SPLIT:
							limit := -1
							if len(args) > 1 && args[1] != value.UNDEFINED {
								n, _ := vm.toNumber(args[1])
								if !math.IsNaN(n) {
									limit = int(math.Trunc(n))
									if limit < 0 {
										limit = -1
									}
								}
							}
							result, err = vm.stringSplitRegex(owner, re, limit)
						case native.REGEXP_SYMBOL_MATCH_ALL:
							result, err = vm.stringMatchAll(owner, []value.Value{thisCtx})
						}
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.RegExpConstructor:
					{
						args := vm.popN(argCount)
						result, err := vm.constructRegExp(args, false)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.RegExpTest, *native.RegExpExec:
					{
						re, _ := vm.regexpFromValue(thisCtx)
						kind := "test"
						if _, ok := fn.(*native.RegExpExec); ok {
							kind = "exec"
						}
						result, err := vm.callRegExpMethod(re, kind, argCount)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}

				case *native.MapGet:
					{
						arg := vm.pop()
						owner, _ := heap.GetObject(thisCtx.GetHandle())

						vm.push(fn.Get(owner.(*native.Map), arg))
					}
				case *native.MapHas:
					{
						arg := vm.pop()
						owner, _ := heap.GetObject(thisCtx.GetHandle())

						vm.push(fn.Has(owner.(*native.Map), arg))
					}
				case *native.MapSet:
					{
						v := vm.pop()
						k := vm.pop()
						owner, _ := heap.GetObject(thisCtx.GetHandle())

						fn.Set(owner.(*native.Map), k, v)
						vm.push(thisCtx)
					}
				case *native.MapKeys:
					{
						owner, _ := heap.GetObject(thisCtx.GetHandle())
						keys := fn.Keys(owner.(*native.Map))
						vm.push(value.EncodeHandle(heap.Allocate(keys)))
					}
				case *native.MapValues:
					{
						owner, _ := heap.GetObject(thisCtx.GetHandle())
						iterator := fn.Values(owner.(*native.Map))
						vm.push(value.EncodeHandle(heap.Allocate(iterator)))
					}
				case *native.MapEntries:
					{
						owner, _ := heap.GetObject(thisCtx.GetHandle())
						iterator := fn.Entries(owner.(*native.Map))
						vm.push(value.EncodeHandle(heap.Allocate(iterator)))
					}
				case *native.MapDelete:
					{
						arg := vm.pop()
						owner, _ := heap.GetObject(thisCtx.GetHandle())
						vm.push(fn.Delete(owner.(*native.Map), arg))
					}
				case *native.MapClear:
					{
						owner, _ := heap.GetObject(thisCtx.GetHandle())
						vm.push(fn.ClearMap(owner.(*native.Map)))
					}
				case *native.MapForEach:
					{
						args := vm.popN(argCount)
						callback := value.UNDEFINED
						thisArg := value.UNDEFINED
						if len(args) > 0 {
							callback = args[0]
						}
						if len(args) > 1 {
							thisArg = args[1]
						}
						owner, _ := heap.GetObject(thisCtx.GetHandle())
						m := owner.(*native.Map)
						keys := m.Keys()
						values := m.ValuesList()
						for i, key := range keys {
							if _, err := vm.invokeCallback(callback, thisArg, []value.Value{values[i], key, thisCtx}); err != nil {
								errValue := vm.makeError("TypeError", err.Error())
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
						}
						vm.push(value.UNDEFINED)
					}
				case *native.ErrorConstructor:
					{
						args := vm.popN(argCount)
						params := make([]any, len(args))
						for i, arg := range args {
							params[i] = arg
						}
						obj, err := fn.New(params...)
						if err != nil {
							errValue := vm.makeError("TypeError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(value.EncodeHandle(heap.Allocate(obj)))
					}
				case *native.ErrorToString:
					{
						vm.popN(argCount)
						name := ""
						message := ""
						if thisCtx.IsObject() {
							if nameValue, err := vm.getMemberValue(thisCtx, native.KEY_NAME); err == nil && nameValue != value.UNDEFINED {
								name, _ = vm.valueAsString(nameValue)
							}
							if messageValue, err := vm.getMemberValue(thisCtx, native.KEY_MESSAGE); err == nil && messageValue != value.UNDEFINED {
								message, _ = vm.valueAsString(messageValue)
							}
						}
						vm.push(value.EncodeHandle(heap.Allocate(native.NewLightString(fn.ToString(name, message)))))
					}
				case *native.FunctionConstructor:
					{
						args := vm.popN(argCount)
						params := []string{}
						body := ""
						if len(args) > 0 {
							for i := 0; i < len(args)-1; i++ {
								params = append(params, mustString(vm, args[i]))
							}
							body = mustString(vm, args[len(args)-1])
						}
						fn, err := compiler.CompileDynamicFunction(params, body)
						if err != nil {
							errValue := vm.makeError("SyntaxError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(value.EncodeHandle(heap.Allocate(fn)))
					}
				case *native.DateConstructor:
					{
						vm.popN(argCount)
						text := time.Now().UTC().Format("Mon Jan 02 2006 15:04:05 GMT+0000 (Coordinated Universal Time)")
						vm.push(value.EncodeHandle(heap.Allocate(native.NewLightString(text))))
					}
				case *native.DateMethod:
					{
						result, err := vm.callDateMethod(fn, thisCtx, argCount)
						if err != nil {
							errValue := vm.makeError("RangeError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.DateStaticMethod:
					{
						result, err := vm.callDateStaticMethod(fn, argCount)
						if err != nil {
							return value.UNDEFINED, err
						}
						vm.push(result)
					}
				case *native.IteratorNext:
					{
						done, err := vm.iteratorNext(thisCtx)
						if err != nil {
							return value.UNDEFINED, err
						}
						result := native.NewObjectHash()
						if done {
							result.SetMember(native.KEY_DONE, value.TRUE)
							result.SetMember(native.KEY_VALUE, value.UNDEFINED)
						} else {
							current, err := vm.iteratorCurrent(thisCtx)
							if err != nil {
								return value.UNDEFINED, err
							}
							result.SetMember(native.KEY_VALUE, current)
							result.SetMember(native.KEY_DONE, value.FALSE)
						}
						vm.push(value.EncodeHandle(heap.Allocate(result)))
					}
				case *native.SetAdd:
					{
						arg := vm.pop()
						owner, _ := heap.GetObject(thisCtx.GetHandle())
						fn.Add(owner.(*native.Set), arg)
						vm.push(thisCtx)
					}
				case *native.SetDelete:
					{
						arg := vm.pop()
						owner, _ := heap.GetObject(thisCtx.GetHandle())
						vm.push(fn.Delete(owner.(*native.Set), arg))
					}
				case *native.SetClear:
					{
						owner, _ := heap.GetObject(thisCtx.GetHandle())
						vm.push(fn.ClearSet(owner.(*native.Set)))
					}
				case *native.SetKeys:
					{
						owner, _ := heap.GetObject(thisCtx.GetHandle())
						iterator := fn.Keys(owner.(*native.Set))
						vm.push(value.EncodeHandle(heap.Allocate(iterator)))
					}
				case *native.SetValues:
					{
						owner, _ := heap.GetObject(thisCtx.GetHandle())
						iterator := fn.Values(owner.(*native.Set))
						vm.push(value.EncodeHandle(heap.Allocate(iterator)))
					}
				case *native.SetEntries:
					{
						owner, _ := heap.GetObject(thisCtx.GetHandle())
						iterator := fn.Entries(owner.(*native.Set))
						vm.push(value.EncodeHandle(heap.Allocate(iterator)))
					}
				case *native.SetForEach:
					{
						args := vm.popN(argCount)
						callback := value.UNDEFINED
						thisArg := value.UNDEFINED
						if len(args) > 0 {
							callback = args[0]
						}
						if len(args) > 1 {
							thisArg = args[1]
						}
						owner, _ := heap.GetObject(thisCtx.GetHandle())
						s := owner.(*native.Set)
						for i := 0; i < len(s.Order()); i++ {
							item := s.Order()[i]
							if _, err := vm.invokeCallback(callback, thisArg, []value.Value{item, item, thisCtx}); err != nil {
								errValue := vm.makeError("TypeError", err.Error())
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
						}
						vm.push(value.UNDEFINED)
					}
				case *native.ArrayPush:
					{
						arg := vm.popN(argCount)
						this, _ := heap.GetObject(thisCtx.GetHandle())
						arr := this.(*native.ObjArr)
						lengthEntry, found := arr.GetOwnEntry(value.EncodeHandle(heap.Allocate(native.NewLightString("length"))))
						if !arr.Extensible() || (found && !lengthEntry.Writable()) {
							errValue := vm.makeError("TypeError", "Cannot add property, object is not extensible")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(fn.Push(arr, arg))
					}
				case *native.ArrayForEach:
					{
						result, err := vm.callArrayIterationMethod("forEach", thisCtx, argCount)
						if err != nil {
							errValue := vm.makeError("TypeError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.ArrayFilter:
					{
						result, err := vm.callArrayIterationMethod("filter", thisCtx, argCount)
						if err != nil {
							errValue := vm.makeError("TypeError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.ArrayMap:
					{
						result, err := vm.callArrayIterationMethod("map", thisCtx, argCount)
						if err != nil {
							errValue := vm.makeError("TypeError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.ArrayReduce:
					{
						result, err := vm.callArrayIterationMethod("reduce", thisCtx, argCount)
						if err != nil {
							errValue := vm.makeError("TypeError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.StringConstructor:
					{
						if argCount == 0 {
							vm.push(value.EncodeHandle(heap.Allocate(native.NewLightString(""))))
						} else {
							arg := vm.pop()
							if description, ok := native.SymbolDescription(arg); ok {
								vm.push(value.EncodeHandle(heap.Allocate(native.NewLightString("Symbol(" + description + ")"))))
							} else {
								s, err := vm.toStringValue(arg)
								if err != nil {
									return value.UNDEFINED, err
								}
								vm.push(s)
							}
						}
					}
				case *native.SymbolConstructor:
					{
						description := ""
						if argCount > 0 {
							arg := vm.pop()
							if arg != value.UNDEFINED {
								if s, ok := vm.valueAsString(arg); ok {
									description = s
								} else {
									description = native.String(arg)
								}
							}
						}
						symbolValue := value.EncodeHandle(heap.Allocate(native.NewSymbol(description)))
						native.RegisterSymbol(symbolValue)
						vm.push(symbolValue)
					}
				case *native.SymbolFor:
					{
						description := ""
						if argCount > 0 {
							arg := vm.pop()
							if s, ok := vm.valueAsString(arg); ok {
								description = s
							} else {
								description = native.String(arg)
							}
						}
						vm.push(native.SymbolForDescription(description))
					}
				case *native.SymbolKeyFor:
					{
						result := value.UNDEFINED
						if argCount > 0 {
							arg := vm.pop()
							if description, ok := native.SymbolDescription(arg); ok {
								if _, found := native.SymbolKeyForDescription(description); found {
									result = value.EncodeHandle(heap.Allocate(native.NewLightString(description)))
								}
							} else {
								errValue := vm.makeError("TypeError", "Symbol.keyFor requires a symbol")
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
						}
						vm.push(result)
					}
				case *native.ArrayConstructor:
					{
						args := vm.popN(argCount)
						params := make([]any, len(args))
						for i, arg := range args {
							params[i] = arg
						}
						obj, err := fn.New(params...)
						if err != nil {
							errValue := vm.makeError("RangeError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(value.EncodeHandle(heap.Allocate(obj)))
					}
				case *native.ObjectConstructor:
					{
						if argCount > 0 {
							arg := vm.pop()
							if arg.IsObject() {
								if _, ok := bigIntFromValue(arg); ok {
									vm.push(native.NewBoxedPrimitiveValue(arg))
								} else {
									vm.push(arg)
								}
							} else {
								vm.push(value.EncodeHandle(heap.Allocate(native.NewObject())))
							}
						} else {
							vm.push(value.EncodeHandle(heap.Allocate(native.NewObject())))
						}
					}
				case *native.ArrayMethod:
					{
						result, err := vm.callArrayMethod(fn, thisCtx, argCount)
						if err != nil {
							name := "TypeError"
							if fn.Kind == native.ARRAY_WITH {
								name = "RangeError"
							}
							errValue := vm.makeError(name, err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.ArrayCallbackMethod:
					{
						result, err := vm.callArrayCallbackMethod(fn, thisCtx, argCount)
						if err != nil {
							errValue := vm.makeError("Error", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.ArrayStaticMethod:
					{
						result, err := vm.callArrayStaticMethod(fn, argCount)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.ArrayJoin:
					{
						s := value.UNDEFINED

						if argCount == 1 {
							s = vm.pop()
						}

						arr, _ := heap.GetObject(thisCtx.GetHandle())

						vm.push(fn.Join(arr.(*native.ObjArr), s))
					}
				case *native.ArrayToString:
					{
						arr, _ := heap.GetObject(thisCtx.GetHandle())
						vm.push(fn.ToString(arr.(*native.ObjArr)))
					}
				case *native.ArrayShift:
					{
						arr, _ := heap.GetObject(thisCtx.GetHandle())
						vm.push(fn.Shift(arr.(*native.ObjArr)))
					}
				case *native.ArrayReverse:
					{
						arr, _ := heap.GetObject(thisCtx.GetHandle())
						fn.Reverse(arr.(*native.ObjArr))
						vm.push(thisCtx)
					}
				case *native.ArrayFill:
					{
						arr, _ := heap.GetObject(thisCtx.GetHandle())
						arg := vm.pop()

						fn.Fill(arr.(*native.ObjArr), arg)
						vm.push(thisCtx)
					}
				case *native.BoxedValueOf:
					{
						if boxed, err := heap.GetObject(thisCtx.GetHandle()); err == nil {
							if primitive, ok := boxed.(*native.BoxedPrimitive); ok {
								vm.push(primitive.Primitive)
								continue
							}
						}
						vm.push(thisCtx)
					}
				case *native.BooleanConstructor:
					{
						if argCount == 0 {
							vm.push(value.FALSE)
						} else {
							arg := vm.pop()
							vm.push(boolValue(truthyValue(arg)))
						}
					}
				case *native.NumberConstructor:
					{
						if argCount == 0 {
							vm.push(value.ValueFromFloat64(0))
						} else {
							arg := vm.pop()
							if f, ok := vm.bigIntToNumber(arg); ok {
								vm.push(value.ValueFromFloat64(f))
								continue
							}
							n, err := vm.toNumber(arg)
							if err != nil {
								return value.UNDEFINED, err
							}
							vm.push(value.ValueFromFloat64(n))
						}
					}
				case *native.NumberFormatMethod:
					{
						result, err := vm.callNumberFormatMethod(fn, thisCtx, argCount)
						if err != nil {
							errValue := vm.makeError("RangeError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.MathMethod:
					{
						result, err := vm.callMathMethod(fn, argCount)
						if err != nil {
							return value.UNDEFINED, err
						}
						vm.push(result)
					}
				case *native.StringMethod:
					{
						result, err := vm.callStringMethod(fn, thisCtx, argCount)
						if err != nil {
							name := "TypeError"
							if fn.Kind == native.STRING_REPEAT || fn.Kind == native.STRING_NORMALIZE {
								name = "RangeError"
							}
							errValue := vm.makeError(name, err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.StringStaticMethod:
					{
						result, err := vm.callStringStaticMethod(fn, argCount)
						if err != nil {
							errValue := vm.makeError("RangeError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.StringCharAt:
					{
						index := 0
						if argCount >= 1 {
							v := vm.pop()
							if v != value.UNDEFINED {
								n, err := vm.toNumber(v)
								if err != nil {
									return value.UNDEFINED, err
								}
								if !math.IsNaN(n) {
									index = int(math.Trunc(n))
								}
							}
						}
						this, _ := heap.GetObject(thisCtx.GetHandle())
						vm.push(fn.CharAt(this.String(), index))
					}
				case *native.StringCharCodeAt:
					{
						index := 0
						if argCount >= 1 {
							v := vm.pop()
							if v != value.UNDEFINED {
								n, err := vm.toNumber(v)
								if err != nil {
									return value.UNDEFINED, err
								}
								if !math.IsNaN(n) {
									index = int(math.Trunc(n))
								}
							}
						}
						this, _ := heap.GetObject(thisCtx.GetHandle())
						vm.push(fn.CharCodeAt(this.String(), index))
					}
				case *native.StringStartsWith:
					{
						args := vm.popN(argCount)
						pattern := ""
						if len(args) > 0 {
							pattern = native.String(args[0])
						}
						units := native.UTF16Units(native.String(thisCtx))
						needle := native.UTF16Units(pattern)
						from := 0
						if len(args) > 1 && args[1] != value.UNDEFINED {
							n, _ := vm.toNumber(args[1])
							from = int(math.Trunc(n))
						}
						from = max(0, min(from, len(units)))
						if len(needle) > len(units)-from {
							vm.push(value.FALSE)
						} else {
							match := true
							for i := range needle {
								if units[from+i] != needle[i] {
									match = false
									break
								}
							}
							vm.push(boolValue(match))
						}
					}
				case *native.StringToUpperCase:
					{
						this, _ := heap.GetObject(thisCtx.GetHandle())
						vm.push(value.EncodeHandle(heap.Allocate(fn.ToUpperCase(this.String()))))
					}
				case *native.StringIncludes:
					{
						args := vm.popN(argCount)
						if len(args) > 0 && args[0].IsObject() {
							if argObj, err := heap.GetObject(args[0].GetHandle()); err == nil {
								if _, isRegExp := argObj.(*native.ObjRegExp); isRegExp {
									errValue := vm.makeError("TypeError", "First argument must not be a regular expression")
									if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
										return value.UNDEFINED, err
									}
									continue
								}
							}
						}
						owner := native.String(thisCtx)
						search := ""
						if len(args) > 0 {
							search = native.String(args[0])
						}
						units := native.UTF16Units(owner)
						from := 0
						if len(args) > 1 && args[1] != value.UNDEFINED {
							n, _ := vm.toNumber(args[1])
							from = int(math.Trunc(n))
						}
						from = max(0, min(from, len(units)))
						vm.push(boolValue(native.UTF16IndexOf(owner, search, from) >= 0))
					}
				case *native.StringSplit:
					{
						separator := vm.pop()

						owner, _ := heap.GetObject(thisCtx.GetHandle())
						sep, _ := heap.GetObject(separator.GetHandle())
						vm.push(fn.Split(owner, sep.String()))
					}
				case *native.StringReplace:
					{
						replaceValue := vm.pop()
						searchValue := vm.pop()

						owner, _ := heap.GetObject(thisCtx.GetHandle())
						replace, _ := heap.GetObject(replaceValue.GetHandle())
						search, _ := heap.GetObject(searchValue.GetHandle())

						vm.push(fn.Replace(owner, search.String(), replace.String()))
					}
				case *native.ObjectValueOf:
					{
						vm.push(thisCtx)
					}
				case *native.ToString:
					{
						vm.push(value.EncodeHandle(heap.Allocate(native.NewLightString(native.ObjectToStringTag(thisCtx)))))
					}
				case *native.IsPrototypeOf:
					{
						args := vm.popN(argCount)
						result := false
						if len(args) > 0 && args[0].IsObject() {
							current := args[0]
							for current.IsObject() {
								currentObj, err := heap.GetObject(current.GetHandle())
								if err != nil {
									break
								}
								p, ok := currentObj.(interface{ PrototypeValue() value.Value })
								if !ok {
									break
								}
								current = p.PrototypeValue()
								if !current.IsObject() {
									break
								}
								if heap.GetPointer(current.GetHandle()) == heap.GetPointer(thisCtx.GetHandle()) {
									result = true
									break
								}
							}
						}
						vm.push(boolValue(result))
					}
				case *native.PropertyIsEnumerable:
					{
						args := vm.popN(argCount)
						result := false
						if len(args) > 0 && thisCtx.IsObject() {
							if obj, err := heap.GetObject(thisCtx.GetHandle()); err == nil {
								if o, ok := obj.(interface {
									GetOwnEntry(value.Value) (native.ObjectValueEntry, bool)
								}); ok {
									if entry, found := o.GetOwnEntry(args[0]); found {
										result = entry.Enumerable()
									}
								}
							}
						}
						vm.push(boolValue(result))
					}
				case *native.Log:
					{

						args := vm.popN(argCount)
						fn.Log(args, vm.output)
						vm.push(value.UNDEFINED)
					}
				case *native.ConsoleMethod:
					{
						args := vm.popN(argCount)
						console := (*native.Console)(nil)
						if thisCtx.IsObject() {
							if obj, err := heap.GetObject(thisCtx.GetHandle()); err == nil {
								console, _ = obj.(*native.Console)
							}
						}
						label := "default"
						if len(args) > 0 {
							label = mustString(vm, args[0])
						}
						switch fn.Kind {
						case native.CONSOLE_ERROR:
							logger := &native.Log{}
							logger.Log(args, vm.output)
						case native.CONSOLE_TABLE:
							native.ConsoleTable(args, vm.output)
						case native.CONSOLE_TIME:
							if console != nil {
								console.Start(label)
							}
						case native.CONSOLE_TIME_END:
							if console != nil {
								duration, found := console.End(label)
								if found {
									text := fmt.Sprintf("%s: %.3fms\n", label, float64(duration.Microseconds())/1000)
									if vm.output != nil {
										vm.output.WriteString(text)
									} else {
										fmt.Print(text)
									}
								}
							}
						}
						vm.push(value.UNDEFINED)
					}
				case *native.Next:
					{
						args := vm.popN(argCount)
						input := value.UNDEFINED
						if len(args) > 0 {
							input = args[0]
						}
						this, _ := heap.GetObject(thisCtx.GetHandle())
						generator := this.(*native.ObjGenerator)
						if generator.IsAsync {
							vm.push(vm.asyncGeneratorRequest(generator, input, generatorNextMode))
							continue
						}
						result, err := vm.generatorResume(generator, input, generatorNextMode)
						if err != nil {
							if throwErr := vm.throwGeneratorError(err, &f, &c, &ip); throwErr != nil {
								return value.UNDEFINED, throwErr
							}
							continue
						}
						vm.push(result)
					}
				case *native.GeneratorIterator:
					{
						vm.push(thisCtx)
					}
				case *native.ArrayIterator:
					{
						this, _ := heap.GetObject(thisCtx.GetHandle())
						arr, ok := this.(*native.ObjArr)
						if !ok {
							errValue := vm.makeError("TypeError", "Array iterator called on non-array")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(value.EncodeHandle(heap.Allocate(object.NewValueIterator(arr))))
					}
				case *native.StringIterator:
					{
						text, err := vm.toStringValue(thisCtx)
						if err != nil {
							return value.UNDEFINED, err
						}
						owner, _ := vm.valueAsString(text)
						units := native.UTF16Units(owner)
						values := make([]value.Value, 0, len(units))
						for i := 0; i < len(units); i++ {
							width := 1
							if units[i] >= 0xD800 && units[i] <= 0xDBFF && i+1 < len(units) && units[i+1] >= 0xDC00 && units[i+1] <= 0xDFFF {
								width = 2
							}
							values = append(values, value.EncodeHandle(heap.Allocate(native.NewLightString(native.UTF16FromUnits(units[i:i+width])))))
							i += width - 1
						}
						iterator := object.NewLiveIterator(func() []value.Value { return values })
						vm.push(value.EncodeHandle(heap.Allocate(iterator)))
					}
				case *native.Return:
					{
						args := vm.popN(argCount)
						returned := value.UNDEFINED
						if len(args) > 0 {
							returned = args[0]
						}
						this, _ := heap.GetObject(thisCtx.GetHandle())
						generator := this.(*native.ObjGenerator)
						if generator.IsAsync {
							vm.push(vm.asyncGeneratorRequest(generator, returned, generatorReturnMode))
							continue
						}
						result, err := vm.generatorResume(generator, returned, generatorReturnMode)
						if err != nil {
							if throwErr := vm.throwGeneratorError(err, &f, &c, &ip); throwErr != nil {
								return value.UNDEFINED, throwErr
							}
							continue
						}
						vm.push(result)
					}
				case *native.Throw:
					{
						args := vm.popN(argCount)
						thrown := value.UNDEFINED
						if len(args) > 0 {
							thrown = args[0]
						}
						this, _ := heap.GetObject(thisCtx.GetHandle())
						generator := this.(*native.ObjGenerator)
						if generator.IsAsync {
							vm.push(vm.asyncGeneratorRequest(generator, thrown, generatorThrowMode))
							continue
						}
						result, err := vm.generatorResume(generator, thrown, generatorThrowMode)
						if err != nil {
							if throwErr := vm.throwGeneratorError(err, &f, &c, &ip); throwErr != nil {
								return value.UNDEFINED, throwErr
							}
							continue
						}
						vm.push(result)
					}
				case *native.ReflectMethod:
					{
						args := vm.popN(argCount)
						result, err := vm.callReflectMethod(fn, args)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.ObjectKeys:
					{
						arg := vm.pop()
						if proxy, ok := vm.asProxy(arg); ok {
							keys, err := vm.proxyOwnKeys(proxy)
							if err != nil {
								errValue := vm.errorValue(err)
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							filtered := []value.Value{}
							for _, key := range keys {
								desc, err := vm.proxyGetOwnPropertyDescriptor(proxy, key)
								if err != nil {
									errValue := vm.errorValue(err)
									if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
										return value.UNDEFINED, err
									}
									filtered = nil
									break
								}
								if desc == value.UNDEFINED {
									continue
								}
								enumerable, err := vm.getMemberValue(desc, value.EncodeHandle(heap.Allocate(native.NewLightString("enumerable"))))
								if err == nil && truthyValue(enumerable) {
									filtered = append(filtered, key)
								}
							}
							vm.push(value.EncodeHandle(heap.Allocate(native.NewArrayFrom(filtered))))
							continue
						}
						arr := fn.Keys(arg)

						objArr := native.NewArrayFrom(arr)
						v := value.EncodeHandle(heap.Allocate(objArr))
						vm.push(v)
					}
				case *native.ObjectValues:
					{
						arg := vm.pop()
						arr := fn.Values(arg)

						objArr := native.NewArrayFrom(arr)

						v := value.EncodeHandle(heap.Allocate(objArr))
						vm.push(v)
					}
				case *native.ResolveFunc:
					{
						if vm.stackTop == 0 {
							vm.push(value.UNDEFINED)
						}

						v := vm.pop()
						p := fn.Promise()
						if fn.ID != p.ResolutionID {
							vm.push(value.UNDEFINED)
							continue
						}
						if p.Resolving && !fn.AllowDuringResolving {
							vm.push(value.UNDEFINED)
							continue
						}
						if fn.Called != nil {
							*fn.Called = true
						}
						vm.resolvePromise(p, v)
						vm.push(value.UNDEFINED)
					}
				case *native.RejectFunc:
					{
						v := vm.pop()
						p := fn.Promise()
						if fn.ID != p.ResolutionID {
							vm.push(value.UNDEFINED)
							continue
						}
						vm.rejectPromise(p, v)
						vm.push(value.UNDEFINED)
					}
				case *native.SetTimeout:
					{
						ms := vm.pop()
						callback := vm.pop()
						delay, _ := vm.toNumber(ms)
						obj, err := heap.GetObject(callback.GetHandle())
						if err != nil {
							errValue := vm.makeError("TypeError", "The callback argument must be of type function")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						callbackFn, ok := obj.(object.Callable)
						if !ok {
							errValue := vm.makeError("TypeError", "The callback argument must be of type function")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						fn.Set(int(delay), callbackFn, thisCtx)
						clone := fn.Clone()
						key := heap.NewCallbackRoot()
						clone.SetRootKey(key)
						heap.StoreCallbackRoot(key, callback, thisCtx)
						id := native.RegisterTimer(clone)
						eventloop.Dispatch(clone)
						vm.push(value.ValueFromFloat64(float64(id)))
					}
				case *native.ClearTimeout:
					{
						args := vm.popN(argCount)
						if len(args) > 0 {
							id, _ := vm.toNumber(args[0])
							native.CancelTimer(int(id))
						}
						vm.push(value.UNDEFINED)
					}
				case *native.Now:
					vm.push(fn.Now())
				case *native.Create:
					{
						args := vm.popN(argCount)
						proto := value.NULL
						if len(args) > 0 && args[0] != value.UNDEFINED {
							proto = args[0]
						}
						o := native.NewObjectFromObject(proto)
						result := value.EncodeHandle(heap.Allocate(o))
						if len(args) > 1 && args[1].IsObject() {
							if descObj, err := heap.GetObject(args[1].GetHandle()); err == nil {
								if descriptors, ok := descObj.(*native.ObjObject); ok {
									definer := &native.ObjectDefineProperty{}
									for _, item := range native.ObjectEnumerableKeys(descriptors) {
										key := native.ObjectPropertyKeyValue(item)
										desc, err := vm.getMemberValue(args[1], key)
										if err != nil {
											return value.UNDEFINED, err
										}
										if definer.Define(result, key, desc) == value.UNDEFINED {
											errValue := vm.makeError("TypeError", "Cannot define property")
											if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
												return value.UNDEFINED, err
											}
										}
									}
								}
							}
						}
						vm.push(result)
					}
				case *native.ObjectMethod:
					{
						result, err := vm.callObjectMethod(fn, argCount)
						if err != nil {
							errValue := vm.makeError("TypeError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.ObjectGetOwnPropertyNames:
					{
						target := vm.pop()
						if proxy, ok := vm.asProxy(target); ok {
							keys, err := vm.proxyOwnKeys(proxy)
							if err != nil {
								errValue := vm.errorValue(err)
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							names := []value.Value{}
							for _, key := range keys {
								if _, ok := vm.valueAsString(key); ok {
									names = append(names, key)
								}
							}
							vm.push(value.EncodeHandle(heap.Allocate(native.NewArrayFrom(names))))
							continue
						}
						names := fn.Names(target)
						vm.push(value.EncodeHandle(heap.Allocate(native.NewArrayFrom(names))))
					}
				case *native.ObjectGetOwnPropertySymbols:
					{
						target := vm.pop()
						if proxy, ok := vm.asProxy(target); ok {
							keys, err := vm.proxyOwnKeys(proxy)
							if err != nil {
								errValue := vm.errorValue(err)
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							symbols := []value.Value{}
							for _, key := range keys {
								if _, ok := vm.valueAsString(key); !ok {
									symbols = append(symbols, key)
								}
							}
							vm.push(value.EncodeHandle(heap.Allocate(native.NewArrayFrom(symbols))))
							continue
						}
						symbols := fn.Symbols(target)
						vm.push(value.EncodeHandle(heap.Allocate(native.NewArrayFrom(symbols))))
					}
				case *native.ObjectDefineProperty:
					{
						descriptor := vm.pop()
						key := vm.pop()
						target := vm.pop()
						if proxy, ok := vm.asProxy(target); ok {
							result, err := vm.proxyDefineProperty(proxy, key, descriptor)
							if err != nil {
								errValue := vm.errorValue(err)
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							if !result {
								errValue := vm.makeError("TypeError", "Cannot redefine property")
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							vm.push(target)
							continue
						}
						result := fn.Define(target, key, descriptor)
						if result == value.UNDEFINED {
							errValue := vm.makeError("TypeError", "Cannot redefine property")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.ObjectGetOwnPropertyDescriptor:
					{
						key := vm.pop()
						target := vm.pop()
						if target.IsObject() {
							if targetObj, err := heap.GetObject(target.GetHandle()); err == nil {
								if namespace, ok := targetObj.(*native.ObjModuleNamespace); ok {
									vm.push(vm.moduleNamespaceDescriptor(namespace, key))
									continue
								}
							}
						}
						if proxy, ok := vm.asProxy(target); ok {
							result, err := vm.proxyGetOwnPropertyDescriptor(proxy, key)
							if err != nil {
								errValue := vm.errorValue(err)
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							vm.push(result)
							continue
						}
						vm.push(fn.Descriptor(target, key))
					}
				case *native.ObjectGetPrototypeOf:
					{
						target := vm.pop()
						if proxy, ok := vm.asProxy(target); ok {
							result, err := vm.proxyGetPrototypeOf(proxy)
							if err != nil {
								errValue := vm.errorValue(err)
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							vm.push(result)
							continue
						}
						vm.push(fn.GetPrototypeOf(target))
					}
				case *native.ObjectSetPrototypeOf:
					{
						proto := vm.pop()
						target := vm.pop()
						if proxy, ok := vm.asProxy(target); ok {
							result, err := vm.proxySetPrototypeOf(proxy, proto)
							if err != nil {
								errValue := vm.errorValue(err)
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							if !result {
								errValue := vm.makeError("TypeError", "Cyclic __proto__ value or non-extensible object")
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							vm.push(target)
							continue
						}
						result := fn.SetPrototypeOf(target, proto)
						if result == value.UNDEFINED {
							errValue := vm.makeError("TypeError", "Cyclic __proto__ value or non-extensible object")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.ObjectHasOwn:
					{
						prop := vm.pop()
						target := vm.pop()
						vm.push(fn.HasOwn(target, prop))
					}
				case *native.HasOwnProperty:
					{
						arg := vm.pop()

						if fn.HasOwn(thisCtx, arg) {
							vm.push(value.TRUE)
						} else {
							vm.push(value.FALSE)
						}
					}
				case *native.SetHas:
					{
						arg := vm.pop()
						owner, _ := heap.GetObject(thisCtx.GetHandle())
						vm.push(fn.Has(owner.(*native.Set), arg))
					}
				case *native.QueueMicroTask:
					{
						v := vm.pop()

						fn, _ := heap.GetObject(v.GetHandle())
						key := heap.NewCallbackRoot()
						heap.StoreCallbackRoot(key, v)
						queue.Enqueue(object.Callback{Fn: fn.(object.Callable), ThisCtx: value.UNDEFINED, Stack: []value.Value{v}, RootKey: key}, queue.MICRO_TASK, false)
					}
				case *native.JSONParse:
					{
						result, err := vm.callJSONParse(argCount)
						if err != nil {
							errValue := vm.makeError("SyntaxError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.JSONStringify:
					{
						result, err := vm.callJSONStringify(argCount)
						if err != nil {
							errValue := vm.makeError("TypeError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.NumberIsNaN:
					{
						arg := vm.pop()
						vm.push(native.NumberIsNaNValue(arg))
					}
				case *native.NumberIsFinite:
					{
						arg := vm.pop()
						vm.push(native.NumberIsFiniteValue(arg))
					}
				case *native.NumberIsInteger:
					{
						arg := vm.pop()
						vm.push(native.NumberIsIntegerValue(arg))
					}
				case *native.NumberIsSafeInteger:
					{
						arg := vm.pop()
						vm.push(native.NumberIsSafeIntegerValue(arg))
					}
				case *native.BigIntToString:
					{
						args := vm.popN(argCount)
						radix := 10
						if len(args) > 0 && args[0] != value.UNDEFINED {
							n, _ := vm.toNumber(args[0])
							radix = int(n)
						}
						vm.push(vm.bigIntToStringValue(thisCtx, radix))
					}
				case *native.BigIntValueOf:
					{
						vm.popN(argCount)
						vm.push(thisCtx)
					}
				case *native.ArrayBufferConstructor, *native.TypedArrayConstructor, *native.DataViewConstructor:
					{
						errValue := vm.makeError("TypeError", "Constructor requires new")
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
				case *native.BigIntConstructor:
					{
						args := vm.popN(argCount)
						if len(args) == 0 {
							errValue := vm.makeError("TypeError", "Cannot convert undefined to a BigInt")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						result, err := vm.toBigInt(args[0])
						if err != nil {
							name := "RangeError"
							if args[0].IsType(value.NULL) || args[0].IsType(value.UNDEFINED) {
								name = "TypeError"
							} else if _, ok := vm.valueAsString(args[0]); ok {
								name = "SyntaxError"
							} else if args[0].IsObject() {
								name = "TypeError"
							}
							errValue := vm.makeError(name, err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(newBigIntValue(result))
					}
				case *native.BigIntAsIntN, *native.BigIntAsUintN:
					{
						args := vm.popN(argCount)
						signed := false
						if _, ok := fn.(*native.BigIntAsIntN); ok {
							signed = true
						}
						if len(args) < 2 {
							errValue := vm.makeError("TypeError", "BigInt.asIntN requires two arguments")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						bitsNumber, err := vm.toNumber(args[0])
						if err != nil || math.IsNaN(bitsNumber) || bitsNumber < 0 {
							errValue := vm.makeError("RangeError", "Invalid bit length")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						bigValue, ok := vm.bigIntObject(args[1])
						if !ok {
							errValue := vm.makeError("TypeError", "Cannot convert to a BigInt")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(newBigIntValue(vm.bigIntAsIntN(int(bitsNumber), bigValue.Value, signed)))
					}
				case *native.FunctionCall:
					{
						args := vm.popN(argCount)
						target := thisCtx
						thisArg := value.UNDEFINED
						rest := []value.Value{}
						if len(args) > 0 {
							thisArg = args[0]
							rest = args[1:]
						}
						obj, err := heap.GetObject(target.GetHandle())
						if err != nil {
							errValue := vm.makeError("TypeError", "Value is not callable")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						if bound, ok := obj.(*native.ObjBoundFunction); ok {
							allArgs := append(append([]value.Value{}, bound.Args...), rest...)
							boundTarget := bound.Target
							if innerObj, err := heap.GetObject(boundTarget.GetHandle()); err == nil {
								if inner, ok := innerObj.(*native.ObjBoundFunction); ok {
									boundTarget = inner.Target
									allArgs = append(append([]value.Value{}, inner.Args...), allArgs...)
								}
							}
							targetObj, err := heap.GetObject(boundTarget.GetHandle())
							if err != nil {
								errValue := vm.makeError("TypeError", "Value is not callable")
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							boundCallable, ok := targetObj.(object.Callable)
							if !ok {
								errValue := vm.makeError("TypeError", "Value is not callable")
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							for _, arg := range allArgs {
								vm.push(arg)
							}
							f, c = vm.Call(boundCallable, &ip, vm.normalizeThis(boundCallable, bound.This), len(allArgs), false)
							break
						}
						if _, ok := obj.(object.Callable); !ok {
							result, err := vm.invokeCallableWithThis(target, thisArg, rest)
							if err != nil {
								errValue := vm.errorValue(err)
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							vm.push(result)
							continue
						}
						callable, ok := obj.(object.Callable)
						if !ok {
							switch nativeFn := obj.(type) {
							case *native.StringMethod:
								for _, arg := range rest {
									vm.push(arg)
								}
								result, err := vm.callStringMethod(nativeFn, thisArg, len(rest))
								if err != nil {
									errValue := vm.makeError("TypeError", err.Error())
									if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
										return value.UNDEFINED, err
									}
									continue
								}
								vm.push(result)
								continue
							case *native.StringStaticMethod:
								for _, arg := range rest {
									vm.push(arg)
								}
								result, err := vm.callStringStaticMethod(nativeFn, len(rest))
								if err != nil {
									errValue := vm.makeError("RangeError", err.Error())
									if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
										return value.UNDEFINED, err
									}
									continue
								}
								vm.push(result)
								continue
							case *native.ArrayMethod:
								for _, arg := range rest {
									vm.push(arg)
								}
								result, err := vm.callArrayMethod(nativeFn, thisArg, len(rest))
								if err != nil {
									errValue := vm.makeError("Error", err.Error())
									if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
										return value.UNDEFINED, err
									}
									continue
								}
								vm.push(result)
								continue
							case *native.ArrayCallbackMethod:
								for _, arg := range rest {
									vm.push(arg)
								}
								result, err := vm.callArrayCallbackMethod(nativeFn, thisArg, len(rest))
								if err != nil {
									errValue := vm.makeError("Error", err.Error())
									if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
										return value.UNDEFINED, err
									}
									continue
								}
								vm.push(result)
								continue
							}
							errValue := vm.makeError("TypeError", "Value is not callable")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						thisArg = vm.normalizeThis(callable, thisArg)
						for _, arg := range rest {
							vm.push(arg)
						}
						f, c = vm.Call(callable, &ip, thisArg, len(rest), false)
					}
				case *native.FunctionApply:
					{
						args := vm.popN(argCount)
						target := thisCtx
						thisArg := value.UNDEFINED
						argArray := value.UNDEFINED
						if len(args) > 0 {
							thisArg = args[0]
						}
						if len(args) > 1 {
							argArray = args[1]
						}
						obj, err := heap.GetObject(target.GetHandle())
						if err != nil {
							errValue := vm.makeError("TypeError", "Value is not callable")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						callable, ok := obj.(object.Callable)
						if !ok {
							errValue := vm.makeError("TypeError", "Value is not callable")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						rest := []value.Value{}
						if argArray != value.UNDEFINED && argArray != value.NULL {
							if !argArray.IsObject() {
								errValue := vm.makeError("TypeError", "CreateListFromArrayLike called on non-object")
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							lengthValue, err := vm.getMemberValue(argArray, value.EncodeHandle(heap.Allocate(native.NewLightString("length"))))
							if err != nil {
								return value.UNDEFINED, err
							}
							n, err := vm.toNumber(lengthValue)
							if err != nil {
								return value.UNDEFINED, err
							}
							length := int(n)
							for i := 0; i < length; i++ {
								item, err := vm.getMemberValue(argArray, value.ValueFromFloat64(float64(i)))
								if err != nil {
									return value.UNDEFINED, err
								}
								rest = append(rest, item)
							}
						}
						thisArg = vm.normalizeThis(callable, thisArg)
						for _, arg := range rest {
							vm.push(arg)
						}
						f, c = vm.Call(callable, &ip, thisArg, len(rest), false)
					}
				case *native.FunctionBind:
					{
						args := vm.popN(argCount)
						target := thisCtx
						thisArg := value.UNDEFINED
						boundArgs := []value.Value{}
						if len(args) > 0 {
							thisArg = args[0]
							boundArgs = args[1:]
						}
						length := 0
						name := ""
						if targetObj, err := heap.GetObject(target.GetHandle()); err == nil {
							if inner, ok := targetObj.(*native.ObjBoundFunction); ok {
								target = inner.Target
								thisArg = inner.This
								boundArgs = append(append([]value.Value{}, inner.Args...), boundArgs...)
								length = int(inner.GetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("length")))).AsNumber())
								nameValue, _ := vm.valueAsString(inner.GetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("name")))))
								name = nameValue
							} else if fn, ok := targetObj.(*object.ObjFunction); ok {
								length = fn.Length
								name = fn.Name
							}
						}
						if length > len(boundArgs) {
							length -= len(boundArgs)
						} else {
							length = 0
						}
						bound := native.NewBoundFunction(target, thisArg, boundArgs)
						bound.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("length"))), value.ValueFromFloat64(float64(length)))
						bound.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("name"))), value.EncodeHandle(heap.Allocate(native.NewLightString("bound "+name))))
						vm.push(value.EncodeHandle(heap.Allocate(bound)))
					}
				case *native.ParseInt:
					{
						args := vm.popN(argCount)
						text := "undefined"
						if len(args) > 0 {
							s, err := vm.toStringValue(args[0])
							if err != nil {
								return value.UNDEFINED, err
							}
							text, _ = vm.valueAsString(s)
						}
						radix := 0
						if len(args) > 1 && args[1] != value.UNDEFINED {
							n, err := vm.toNumber(args[1])
							if err != nil {
								return value.UNDEFINED, err
							}
							if !math.IsNaN(n) {
								radix = int(int32(uint32(int64(n))))
							}
						}
						vm.push(parseIntString(text, radix))
					}
				case *native.GlobalFunction:
					{
						args := vm.popN(argCount)
						switch fn.Kind {
						case native.GLOBAL_IS_NAN:
							n, err := vm.toNumber(args[0])
							vm.push(boolValue(err == nil && math.IsNaN(n)))
						case native.GLOBAL_IS_FINITE:
							n, err := vm.toNumber(args[0])
							vm.push(boolValue(err == nil && !math.IsNaN(n) && !math.IsInf(n, 0)))
						case native.GLOBAL_ENCODE_URI_COMPONENT:
							text := ""
							if len(args) > 0 {
								text = mustString(vm, args[0])
							}
							vm.push(value.EncodeHandle(heap.Allocate(native.NewLightString(native.EncodeURIComponent(text)))))
						case native.GLOBAL_DECODE_URI_COMPONENT:
							text := ""
							if len(args) > 0 {
								text = mustString(vm, args[0])
							}
							decoded, err := native.DecodeURIComponent(text)
							if err != nil {
								errValue := vm.makeError("URIError", err.Error())
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							vm.push(value.EncodeHandle(heap.Allocate(native.NewLightString(decoded))))
						}
					}
				case *native.IntlNumberFormatMethod:
					{
						args := vm.popN(argCount)
						number := 0.0
						if len(args) > 0 {
							number, _ = vm.toNumber(args[0])
						}
						vm.push(value.EncodeHandle(heap.Allocate(native.NewLightString(native.LocaleNumberString(number)))))
					}
				case *native.URLSearchParamsGet:
					{
						args := vm.popN(argCount)
						name := ""
						if len(args) > 0 {
							name = mustString(vm, args[0])
						}
						vm.push(value.EncodeHandle(heap.Allocate(native.NewLightString(fn.Values.Get(name)))))
					}
				case *native.ParseFloat:
					{
						text := "undefined"
						if argCount > 0 {
							arg := vm.pop()
							s, err := vm.toStringValue(arg)
							if err != nil {
								return value.UNDEFINED, err
							}
							text, _ = vm.valueAsString(s)
						}
						vm.push(parseFloatString(text))
					}
				default:
					errValue := vm.makeError("TypeError", "Value is not callable")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}

			}
		case chunk.OP_CALL_OPTIONAL:
			fallthrough
		case chunk.OP_CALL_CALLEE_FIRST:
			{
				optional := code == chunk.OP_CALL_OPTIONAL
				argCount := int(c.Code[ip])
				ip++
				spread := c.Code[ip]
				ip++

				args := vm.popN(argCount)
				callee := vm.pop()
				thisCtx := value.UNDEFINED
				if callee == value.TAG_METHOD_HANDLE {
					callee = vm.pop()
					thisCtx = vm.pop()
				}
				if optional && (callee == value.UNDEFINED || callee == value.NULL) {
					vm.push(value.UNDEFINED)
					continue
				}
				obj, err := heap.GetObject(callee.GetHandle())
				if err != nil {
					errValue := vm.makeError("TypeError", "Value is not callable")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				fn, ok := obj.(object.Callable)
				if !ok {
					errValue := vm.makeError("TypeError", "Value is not callable")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				for _, arg := range args {
					vm.push(arg)
				}
				callThis := thisCtx
				if fnObj, ok := fn.(interface{ FunctionBase() *object.ObjFunction }); ok {
					base := fnObj.FunctionBase()
					if vm.main && scriptMode && !base.Strict && callThis == value.UNDEFINED && !base.IsArrow {
						callThis = vm.ensureGlobalObject()
					}
				} else if fnObj, ok := fn.(*object.ObjFunction); ok && vm.main && scriptMode && !fnObj.Strict && callThis == value.UNDEFINED {
					callThis = vm.ensureGlobalObject()
				}
				calledWithSpread := spread == 1
				f, c = vm.Call(fn, &ip, callThis, argCount, calledWithSpread)
			}
		case chunk.OP_RETURN:
			{
				if generator, ok := f.fn.(*native.ObjGenerator); ok {
					generator.Done = true
					generator.Running = false
					generator.PendingReturn = false
					generator.Delegating = false
					generator.Exceptions = nil
					returned := vm.pop()
					result := native.NewObjectHash()
					result.SetMember(native.KEY_VALUE, returned)
					result.SetMember(native.KEY_DONE, value.TRUE)
					vm.push(value.EncodeHandle(heap.Allocate(result)))
				}
				for len(vm.exceptionStack) > 0 && vm.exceptionStack[len(vm.exceptionStack)-1].frame >= vm.frameCount {
					vm.exceptionStack = vm.exceptionStack[:len(vm.exceptionStack)-1]
				}
				if promise, ok := f.fn.(*native.ObjAsyncFunction); ok {
					ip = f.returnIp
					value := value.UNDEFINED
					vm.frameCount--

					if vm.stackTop > 0 {
						value = vm.pop()
						if isMethodHandle(value) {
							value = vm.pop()
							vm.pop()
						}
					}

					if promise.Promise != nil {
						vm.resolvePromise(promise.Promise, value)
					} else {
						promise.Resolve(value)
					}

					if vm.frameCount <= 0 {
						vm.stackTop = 0

						if flags.Debug {
							fmt.Println("--")
						}
						return value, nil
					}

					vm.stackTop = f.localStart

					if promise.ReturnArgumentIsPromise {
						vm.push(value)
					}
					for _, v := range promise.CallerStack {
						vm.push(v)
					}
					promise.CallerStack = nil

					f = vm.currentFrame()
					c = *f.fn.ValueChunk()
				} else {
					ip = f.returnIp
					value := value.UNDEFINED
					vm.frameCount--

					if vm.stackTop > 0 {
						value = vm.pop()
						if isMethodHandle(value) {
							value = vm.pop()
							vm.pop()
						}
					}

					if vm.frameCount <= 0 {
						vm.stackTop = 0

						if flags.Debug {
							fmt.Println("--")
						}
						return value, nil
					}

					vm.stackTop = f.localStart
					vm.push(value)

					f = vm.currentFrame()
					c = *f.fn.ValueChunk()
				}
			}
		case chunk.OP_CREATE_ARRAY:
			{
				length := c.ReadInt(&ip)
				arr := native.NewArray(length)
				handle := heap.Allocate(arr)
				vm.push(value.EncodeHandle(handle))
			}
		case chunk.OP_PUSH_ELEMENT:
			{
				v := vm.pop()
				arr := vm.peek()

				obj, err := heap.GetObject(arr.GetHandle())

				if err != nil {
					return value.UNDEFINED, err
				}

				arrOBj, ok := obj.(*native.ObjArr)

				if !ok {
					return value.UNDEFINED, fmt.Errorf("trying to initialize {%s} that is not an array", native.String(arr))
				}

				arrOBj.PushElement(v)
			}
		case chunk.OP_GET_ITERATOR:
			{
				iteratee := vm.pop()
				type_ := c.Code[ip]
				ip++
				iterator, err := vm.acquireIterator(iteratee, type_)
				if err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(iterator)
			}
		case chunk.OP_GET_ASYNC_ITERATOR:
			{
				iteratee := vm.pop()
				iterator, err := vm.acquireAsyncIterator(iteratee)
				if err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(iterator)
			}
		case chunk.OP_ASYNC_ITERATOR_NEXT_SLOT:
			{
				slot := int(c.Code[ip])
				ip++
				iterator := vm.stack[f.localStart+slot]
				result, err := vm.asyncIteratorNext(iterator)
				if err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(result)
			}
		case chunk.OP_ASYNC_ITERATOR_RESULT_SLOT:
			{
				slot := int(c.Code[ip])
				ip++
				result := vm.pop()
				iterator := vm.stack[f.localStart+slot]
				done, err := vm.asyncIteratorResult(iterator, result)
				if err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(boolValue(done))
			}
		case chunk.OP_ASYNC_ITERATOR_VALUE_SLOT:
			{
				slot := int(c.Code[ip])
				ip++
				iterator := vm.stack[f.localStart+slot]
				current, err := vm.iteratorCurrent(iterator)
				if err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(current)
			}
		case chunk.OP_ASYNC_ITERATOR_CLOSE_SLOT:
			{
				slot := int(c.Code[ip])
				ip++
				iterator := vm.stack[f.localStart+slot]
				result, err := vm.asyncIteratorClose(iterator)
				if err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(result)
			}
		case chunk.OP_ITERATOR_NEXT:
			{
				iterator := vm.peek()
				done, err := vm.iteratorNext(iterator)
				if err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(boolValue(done))
			}
		case chunk.OP_ITERATOR_NEXT_SLOT:
			{
				slot := int(c.Code[ip])
				ip++
				iterator := vm.stack[f.localStart+slot]
				done, err := vm.iteratorNext(iterator)
				if err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(boolValue(done))
			}
		case chunk.OP_ITERATOR_CURRENT:
			{
				iterator := vm.peek()
				current, err := vm.iteratorCurrent(iterator)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(current)
			}
		case chunk.OP_ITERATOR_CURRENT_SLOT:
			{
				slot := int(c.Code[ip])
				ip++
				iterator := vm.stack[f.localStart+slot]
				current, err := vm.iteratorCurrent(iterator)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(current)
			}
		case chunk.OP_TRIM_STACK_SLOT:
			{
				slot := int(c.Code[ip])
				ip++
				target := f.localStart + slot + 1
				if target < vm.stackTop {
					vm.stackTop = target
				}
			}
		case chunk.OP_STACK_MARK:
			{
				id := int(c.Code[ip])
				ip++
				vm.loopMarks[id] = append(vm.loopMarks[id], vm.stackTop)
				if vm.stackTop > 0 {
					vm.loopIterators[id] = vm.stack[vm.stackTop-1]
				}
			}
		case chunk.OP_STACK_TRIM:
			{
				id := int(c.Code[ip])
				ip++
				marks := vm.loopMarks[id]
				if len(marks) > 0 {
					vm.stackTop = marks[len(marks)-1]
				}
			}
		case chunk.OP_STACK_UNMARK:
			{
				id := int(c.Code[ip])
				ip++
				marks := vm.loopMarks[id]
				if len(marks) > 0 {
					vm.loopMarks[id] = marks[:len(marks)-1]
				}
				delete(vm.loopIterators, id)
			}
		case chunk.OP_ITERATOR_VALUE_OR_UNDEFINED:
			{
				done := vm.pop()
				iterator := vm.peek()
				if truthyValue(done) {
					vm.push(value.UNDEFINED)
				} else {
					current, err := vm.iteratorCurrent(iterator)
					if err != nil {
						return value.UNDEFINED, err
					}
					vm.push(current)
				}
			}
		case chunk.OP_ITERATOR_REST:
			{
				iterator := vm.peek()
				items := []value.Value{}
				for {
					done, err := vm.iteratorNext(iterator)
					if err != nil {
						return value.UNDEFINED, err
					}
					if done {
						break
					}
					current, err := vm.iteratorCurrent(iterator)
					if err != nil {
						return value.UNDEFINED, err
					}
					items = append(items, current)
				}
				vm.push(value.EncodeHandle(heap.Allocate(native.NewArrayFrom(items))))
			}
		case chunk.OP_ITERATOR_CLOSE_POP:
			{
				iterator := vm.pop()
				if err := vm.closeIterator(iterator); err != nil {
					errValue := vm.makeError("Error", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
			}
		case chunk.OP_ITERATOR_CLOSE_SLOT:
			{
				slot := int(c.Code[ip])
				ip++
				iterator := vm.stack[f.localStart+slot]
				if err := vm.closeIterator(iterator); err != nil {
					if vm.completionPending {
						continue
					}
					errValue := vm.makeError("Error", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
			}
		case chunk.OP_ITERATOR_CLOSE_MARK:
			{
				id := int(c.Code[ip])
				ip++
				iterator, found := vm.loopIterators[id]
				if !found {
					continue
				}
				if err := vm.closeIterator(iterator); err != nil {
					if vm.completionPending {
						continue
					}
					errValue := vm.makeError("Error", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
			}
		case chunk.OP_CREATE_HEAP_SCOPE:
			{
				f.heapScope = heap.CreateHeapScope(f.heapScope)
				f.ownScope = true
				vm.frames[vm.frameCount-1].heapScope = f.heapScope
				vm.frames[vm.frameCount-1].ownScope = f.ownScope
			}
		case chunk.OP_CLONE_FUNCTION:
			{
				v := vm.pop()
				clone := cloneFunctionValue(v, f.heapScope)
				if clone.IsObject() {
					if obj, err := heap.GetObject(clone.GetHandle()); err == nil {
						if fn, ok := obj.(interface{ FunctionBase() *object.ObjFunction }); ok {
							base := fn.FunctionBase()
							if base.IsArrow {
								base.LexicalThis = f.thisCtx
								if outer, ok := f.fn.(interface{ FunctionBase() *object.ObjFunction }); ok && outer.FunctionBase().IsArrow {
									base.LexicalNewTarget = outer.FunctionBase().LexicalNewTarget
								} else {
									base.LexicalNewTarget = f.newTarget
								}
							}
						}
					}
				}
				vm.push(clone)
			}
		case chunk.OP_ENSURE_HEAP_SCOPE:
			{
				if !f.ownScope {
					f.heapScope = heap.CreateHeapScope(f.heapScope)
					f.ownScope = true
					vm.frames[vm.frameCount-1].heapScope = f.heapScope
					vm.frames[vm.frameCount-1].ownScope = f.ownScope
				}
			}
		case chunk.OP_COPY_HEAP_SCOPE:
			{
				f.heapScope = heap.CopyHeapScope(f.heapScope)
				vm.frames[vm.frameCount-1].heapScope = f.heapScope
			}
		case chunk.OP_TRY_BLOCK_START, chunk.OP_TRY_FINALLY_START:
			{
				catchStart := int(c.Code[ip+3]) | int(c.Code[ip+2])<<8 | int(c.Code[ip+1])<<16 | int(c.Code[ip])<<24
				ip += 4
				vm.exceptionStack = append(vm.exceptionStack, ExceptionState{jumpTo: catchStart, stackTop: vm.stackTop, frame: vm.frameCount, isFinally: code == chunk.OP_TRY_FINALLY_START})
			}
		case chunk.OP_TRY_BLOCK_END:
			{
				if len(vm.exceptionStack) > 0 {
					vm.exceptionStack = vm.exceptionStack[:len(vm.exceptionStack)-1]
				}
			}
		case chunk.OP_NEW:
			{
				argCount := int(c.Code[ip])
				ip++
				callee := vm.pop()
				obj, err := heap.GetObject(callee.GetHandle())

				if err != nil {
					return value.UNDEFINED, err
				}

				switch ctor := obj.(type) {
				case *native.FunctionConstructor:
					{
						args := vm.popN(int(argCount))
						params := []string{}
						body := ""
						if len(args) > 0 {
							for i := 0; i < len(args)-1; i++ {
								params = append(params, mustString(vm, args[i]))
							}
							body = mustString(vm, args[len(args)-1])
						}
						fn, err := compiler.CompileDynamicFunction(params, body)
						if err != nil {
							errValue := vm.makeError("SyntaxError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(value.EncodeHandle(heap.Allocate(fn)))
					}
				case *native.BooleanConstructor:
					{
						primitive := value.FALSE
						if argCount > 0 {
							arg := vm.pop()
							primitive = boolValue(truthyValue(arg))
						}
						vm.push(native.NewBoxedPrimitiveValue(primitive))
					}
				case *native.NumberConstructor:
					{
						primitive := value.ValueFromFloat64(0)
						if argCount > 0 {
							arg := vm.pop()
							n, err := vm.toNumber(arg)
							if err != nil {
								return value.UNDEFINED, err
							}
							primitive = value.ValueFromFloat64(n)
						}
						vm.push(native.NewBoxedPrimitiveValue(primitive))
					}
				case *native.StringConstructor:
					{
						s := ""
						if argCount > 0 {
							arg := vm.pop()
							converted, err := vm.toStringValue(arg)
							if err != nil {
								return value.UNDEFINED, err
							}
							s, _ = vm.valueAsString(converted)
						}
						primitive := value.EncodeHandle(heap.Allocate(native.NewLightString(s)))
						vm.push(native.NewBoxedPrimitiveValue(primitive))
					}
				case *native.SymbolConstructor:
					{
						errValue := vm.makeError("TypeError", "Symbol is not a constructor")
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
				case *native.ProxyConstructor:
					{
						args := vm.popN(int(argCount))
						if len(args) < 2 || !args[0].IsObject() || !args[1].IsObject() {
							errValue := vm.makeError("TypeError", "Cannot create proxy with a non-object as target or handler")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						proxy := native.NewObjProxy(args[0], args[1])
						vm.push(value.EncodeHandle(heap.Allocate(proxy)))
					}
				case *native.ObjProxy:
					{
						args := vm.popN(int(argCount))
						result, err := vm.proxyConstruct(ctor, args, callee)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.ObjClass:
					{
						args := vm.popN(int(argCount))
						instance := ctor.NewInstance()
						instanceValue := heap.EncodeObject(instance)
						constructor, found := vm.classConstructor(ctor)
						if !found {
							if err := vm.initializeDefaultInstance(instance, ctor); err != nil {
								errValue := vm.errorValue(err)
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							vm.push(instanceValue)
							continue
						}
						result, err := vm.callClassConstructor(constructor, instanceValue, args, callee)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						constructorClass := ctor
						if constructorObj, err := heap.GetObject(constructor.GetHandle()); err == nil {
							if fn, ok := constructorObj.(*object.ObjFunction); ok {
								if homeObj, err := heap.GetObject(fn.HomeObject.GetHandle()); err == nil {
									if homeClass, ok := homeObj.(*native.ObjClass); ok {
										constructorClass = homeClass
									}
								}
								if fn.IsDerivedConstructor && result != value.UNDEFINED && !result.IsObject() {
									errValue := vm.makeError("TypeError", "Derived constructors may only return object or undefined")
									if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
										return value.UNDEFINED, err
									}
									continue
								}
							}
						}
						if constructorClass != ctor {
							if err := vm.initializeClassInstance(instance, ctor); err != nil {
								errValue := vm.errorValue(err)
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
						}
						if result.IsObject() && result != instanceValue {
							vm.push(result)
						} else {
							vm.push(instanceValue)
						}
					}
				case *object.ObjFunction:
					{
						if ctor.IsArrow || ctor.IsMethod {
							errValue := vm.makeError("TypeError", "Value is not a constructor")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						args := vm.popN(int(argCount))
						result, err := vm.construct(ctor, args, callee)
						if err != nil {
							errValue := vm.makeError("TypeError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.ObjBoundFunction:
					{
						args := vm.popN(int(argCount))
						target := ctor.Target
						allArgs := append(append([]value.Value{}, ctor.Args...), args...)
						if targetObj, err := heap.GetObject(target.GetHandle()); err == nil {
							if inner, ok := targetObj.(*native.ObjBoundFunction); ok {
								target = inner.Target
								allArgs = append(append([]value.Value{}, inner.Args...), allArgs...)
							}
						}
						targetObj, err := heap.GetObject(target.GetHandle())
						if err != nil {
							errValue := vm.makeError("TypeError", "Value is not a constructor")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						callable, ok := targetObj.(object.Callable)
						if !ok {
							errValue := vm.makeError("TypeError", "Value is not a constructor")
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						result, err := vm.construct(callable, allArgs, callee)
						if err != nil {
							errValue := vm.makeError("TypeError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.RegExpConstructor:
					{
						args := vm.popN(int(argCount))
						result, err := vm.constructRegExp(args, true)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.ArrayBufferConstructor:
					{
						args := vm.popN(int(argCount))
						result, err := vm.constructArrayBuffer(args)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.TypedArrayConstructor:
					{
						args := vm.popN(int(argCount))
						result, err := vm.constructTypedArray(ctor, args)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.DataViewConstructor:
					{
						args := vm.popN(int(argCount))
						result, err := vm.constructDataView(args)
						if err != nil {
							errValue := vm.errorValue(err)
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						vm.push(result)
					}
				case *native.BigIntConstructor:
					{
						errValue := vm.makeError("TypeError", "BigInt is not a constructor")
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
				case native.Constructor:
					{
						args := vm.popN(int(argCount))
						if _, ok := ctor.(*native.ArrayConstructor); ok && len(args) == 1 && args[0].IsNumber() {
							n := args[0].AsNumber()
							if n < 0 || math.IsNaN(n) || math.Trunc(n) != n || n >= 4294967296 {
								errValue := vm.makeError("RangeError", "Invalid array length")
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
						}
						params := make([]any, argCount)

						for i, v := range args {
							params[i] = v
						}

						newObj, err := ctor.New(params...)

						if err != nil {
							errValue := vm.makeError("TypeError", err.Error())
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}

						objHandle := heap.Allocate(newObj)
						vm.push(value.EncodeHandle(objHandle))
					}
				case *native.PromiseConstructor:
					{
						args := vm.popN(int(argCount))
						promise := native.NewPromise()
						resolve := native.NewResolveFunc(promise)
						reject := native.NewRejectFunc(promise)
						if len(args) > 0 {
							if executor, ok := func() (object.Callable, bool) {
								executorObj, err := heap.GetObject(args[0].GetHandle())
								if err != nil {
									return nil, false
								}
								callable, ok := executorObj.(object.Callable)
								return callable, ok
							}(); ok {
								runner := NewVM(false, vm.output)
								runner.push(value.EncodeHandle(heap.Allocate(resolve)))
								runner.push(value.EncodeHandle(heap.Allocate(reject)))
								f, c := runner.Call(executor, nil, value.UNDEFINED, 2, false)
								_, err := runner.run(f, c)
								if err != nil {
									name := "Error"
									message := err.Error()
									if jsErr, ok := err.(*JSException); ok {
										name = jsErr.Name
										message = jsErr.Message
									}
									promise.Reject(vm.makeError(name, message))
								}
							} else {
								errValue := vm.makeError("TypeError", "Promise resolver is not a function")
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
						}
						vm.push(value.EncodeHandle(heap.Allocate(promise)))
					}
				}
			}
		case chunk.OP_THROW:
			{
				err := vm.pop()
				if generator, ok := f.fn.(*native.ObjGenerator); ok && generator.PendingReturn {
					if err == generator.CompletionMark {
						if vm.jumpGeneratorFinally(&f, &c, &ip, generator) {
							continue
						}
						generator.Done = true
						generator.Running = false
						generator.PendingReturn = false
						generator.Exceptions = nil
						result := iteratorResult(generator.ReturnValue, true)
						vm.stackTop = 0
						return result, nil
					}
					generator.PendingReturn = false
				}
				if len(vm.exceptionStack) == 0 {
					if generator, ok := f.fn.(*native.ObjGenerator); ok {
						generator.Done = true
						generator.Running = false
						generator.Exceptions = nil
					}
					if asyncFn, ok := f.fn.(*native.ObjAsyncFunction); ok {
						if asyncFn.Promise != nil {
							vm.rejectPromise(asyncFn.Promise, err)
						}
						vm.frameCount--
						if vm.frameCount > 0 {
							ip = f.returnIp
							vm.stackTop = f.localStart
							for _, v := range asyncFn.CallerStack {
								vm.push(v)
							}
							asyncFn.CallerStack = nil
							f = vm.currentFrame()
							c = *f.fn.ValueChunk()
						} else {
							vm.stackTop = 0
							return value.UNDEFINED, nil
						}
						continue
					}
					if err.IsObject() {
						obj, _ := heap.GetObject(err.GetHandle())
						if e, ok := obj.(*native.ObjError); ok {
							name := "Error"
							if n := e.GetMember(native.KEY_NAME); n != value.UNDEFINED {
								if ns, ok := vm.valueAsString(n); ok {
									name = ns
								}
							}
							return value.UNDEFINED, &JSException{Name: name, Message: native.String(err)}
						}
					}
					return value.UNDEFINED, &JSException{Name: "Error", Message: native.String(err)}
				}
				state := vm.exceptionStack[len(vm.exceptionStack)-1]
				vm.exceptionStack = vm.exceptionStack[:len(vm.exceptionStack)-1]

				vm.frameCount = state.frame
				f = vm.currentFrame()
				c = *f.fn.ValueChunk()
				ip = state.jumpTo
				vm.stackTop = state.stackTop
				vm.push(err)
			}
		case chunk.OP_AWAIT:
			{
				awaitee := vm.pop()
				var promise *native.ObjPromise
				if awaitee.IsObject() {
					if awaiteeObj, err := heap.GetObject(awaitee.GetHandle()); err == nil {
						promise, _ = awaiteeObj.(*native.ObjPromise)
					}
				}
				if promise == nil {
					if awaitee.IsObject() {
						if thenValue, err := vm.getMemberValue(awaitee, value.EncodeHandle(heap.Allocate(native.NewLightString("then")))); err == nil && thenValue.IsObject() {
							if thenObj, err := heap.GetObject(thenValue.GetHandle()); err == nil {
								if _, ok := thenObj.(object.Callable); ok {
									p := native.NewPromise()
									resolve := value.EncodeHandle(heap.Allocate(native.NewResolveFunc(p)))
									reject := value.EncodeHandle(heap.Allocate(native.NewRejectFunc(p)))
									vm.callValue(thenValue, awaitee, []value.Value{resolve, reject})
									promise = p
								}
							}
						}
					}
				}
				if promise == nil {
					promise = native.NewPromise()
					promise.Resolve(awaitee)
				}
				if promise.Status == native.REJECTED && vm.completionPending {
					promise = native.NewPromise()
					promise.Resolve(value.UNDEFINED)
				}
				if generator, ok := f.fn.(*native.ObjGenerator); ok && generator.IsAsync {
					vm.suspendGenerator(generator, ip, &f, value.UNDEFINED)
					generator.AsyncAwaiting = promise
					ip = f.returnIp
					vm.frameCount--
					if vm.frameCount > 0 {
						f = vm.currentFrame()
						c = *f.fn.ValueChunk()
						vm.push(value.UNDEFINED)
					} else {
						vm.stackTop = 0
						return value.UNDEFINED, nil
					}
					continue
				}
				if curentAsyncFn, ok := f.fn.(*native.ObjAsyncFunction); ok {
					count := (vm.stackTop - f.localStart) + 1
					stack := make([]value.Value, count)
					copy(stack, append(vm.stack[f.localStart:vm.stackTop], value.EncodeHandle(heap.Allocate(promise))))
					exceptions := []native.AsyncException{}
					kept := vm.exceptionStack[:0]
					frameIndex := vm.frameCount - 1
					for _, state := range vm.exceptionStack {
						if state.frame > frameIndex {
							exceptions = append(exceptions, native.AsyncException{JumpTo: state.jumpTo, StackTop: state.stackTop, Frame: state.frame, IsFinally: state.isFinally})
						} else {
							kept = append(kept, state)
						}
					}
					vm.exceptionStack = kept
					curentAsyncFn.Pause(stack, ip, f.localStart, f.heapScope, f.ownScope, exceptions)
					if promise.Status != native.PENDING {
						if len(curentAsyncFn.State.Stack) > 0 {
							curentAsyncFn.State.Stack[len(curentAsyncFn.State.Stack)-1] = promise.Value
						}
						if promise.Status == native.REJECTED {
							curentAsyncFn.RejectValue = promise.Value
							curentAsyncFn.HasRejection = true
						}
						key := heap.NewCallbackRoot()
						heap.StoreCallbackRoot(key, curentAsyncFn.State.Stack...)
						queue.Enqueue(object.Callback{Fn: curentAsyncFn, ThisCtx: value.UNDEFINED, RootKey: key, MicroTask: true}, queue.MICRO_TASK, false)
					} else {
						curentAsyncFn.Await(promise)
					}
					heap.StoreAsyncFunctionStack(uintptr(unsafe.Pointer(curentAsyncFn)), stack)
					heap.StoreAsyncFunctionScope(uintptr(unsafe.Pointer(curentAsyncFn)), f.heapScope)

					vm.stackTop = f.localStart
					for _, v := range curentAsyncFn.CallerStack {
						vm.push(v)
					}
					curentAsyncFn.CallerStack = nil

					vm.frameCount--

					if vm.frameCount > 0 {
						ip = f.returnIp
						f = vm.currentFrame()
						c = *f.fn.ValueChunk()
					} else {
						vm.stackTop = 0
						return value.UNDEFINED, nil
					}
				}
			}
		case chunk.OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS:
			{
				amount := c.Code[ip]
				ip++

				removeMap := map[int]bool{}

				for range amount {
					idx := int(c.Code[ip])
					ip++
					slot := int(c.Code[ip])
					ip++
					removeMap[idx] = true
					heap.DefineHeapVarAt(f.heapScope, slot, vm.stack[f.localStart+idx])
				}

				localCount := (vm.stackTop - f.localStart) - int(amount)

				locals := make([]value.Value, 0, localCount)

				for i, v := range vm.stack[f.localStart:vm.stackTop] {
					if !removeMap[i] {
						locals = append(locals, v)
					}
				}

				vm.stackTop = f.localStart

				for _, v := range locals {
					vm.push(v)
				}
			}
		case chunk.OP_CREATE_CLASS_START:
			{
				name := vm.pop()
				if n, err := heap.GetObject(name.GetHandle()); err == nil && n.Type() == object.OBJ_STRING {
					className := n.String()
					class := native.NewObjClass(className)
					proto := native.NewPrototype(className)
					proto.SetMember(native.KEY_PROTO, native.PROTOTYPE_OBJECT)
					classValue := value.EncodeHandle(heap.Allocate(class))
					protoValue := value.EncodeHandle(heap.Allocate(proto))
					class.SetPrototype(protoValue)
					vm.push(classValue)
					vm.push(protoValue)
				} else {
					return value.UNDEFINED, fmt.Errorf("%s is not a string", native.String(name))
				}
			}
		case chunk.OP_CREATE_CLASS_EXTENDS:
			{
				name := vm.pop()
				super := vm.pop()
				if n, err := heap.GetObject(name.GetHandle()); err == nil && n.Type() == object.OBJ_STRING {
					className := n.String()
					class := native.NewObjClass(className)
					proto := native.NewPrototype(className)
					class.SetSuperClass(super)
					superProto, err := vm.getMemberValue(super, value.EncodeHandle(heap.Allocate(native.NewLightString("prototype"))))
					if err != nil {
						return value.UNDEFINED, err
					}
					if superProto.IsObject() {
						proto.SetMember(native.KEY_PROTO, superProto)
					} else {
						proto.SetMember(native.KEY_PROTO, native.PROTOTYPE_OBJECT)
					}
					classValue := value.EncodeHandle(heap.Allocate(class))
					protoValue := value.EncodeHandle(heap.Allocate(proto))
					class.SetPrototype(protoValue)
					vm.push(classValue)
					vm.push(protoValue)
				} else {
					return value.UNDEFINED, fmt.Errorf("%s is not a string", native.String(name))
				}
			}
		case chunk.OP_CREATE_CLASS_END:
			{
				vm.pop()
			}
		case chunk.OP_PUSH_METHOD:
			{
				flags := c.Code[ip]
				ip++
				kind := native.ClassMemberKind(c.Code[ip])
				ip++
				method := vm.pop()
				methodObj, err := heap.GetObject(method.GetHandle())
				if err != nil {
					return value.UNDEFINED, fmt.Errorf("%s was not an object", native.String(method))
				}
				if _, ok := methodObj.(object.Callable); !ok {
					return value.UNDEFINED, fmt.Errorf("%s was not an function", native.String(method))
				}
				method = cloneFunctionValue(method, f.heapScope)
				key := vm.pop()
				classValue := vm.peekN(1)
				protoValue := vm.peek()
				classObj, _ := heap.GetObject(classValue.GetHandle())
				class := classObj.(*native.ObjClass)
				target := protoValue
				isStatic := flags&1 != 0
				isPrivate := flags&2 != 0
				if isStatic {
					target = classValue
				}
				if methodObj, err := heap.GetObject(method.GetHandle()); err == nil {
					if fn, ok := methodObj.(*object.ObjFunction); ok {
						if kind == native.CLASS_CONSTRUCTOR {
							fn.HomeObject = classValue
						} else {
							fn.HomeObject = target
						}
					}
				}
				if isPrivate {
					if isStatic {
						native.DefineClassMember(class, key, method, kind)
					} else {
						class.AddPrivateMember(key, native.ClassMember{Value: method, Kind: kind, Private: true})
					}
				} else {
					targetObj, _ := heap.GetObject(target.GetHandle())
					native.DefineClassMember(targetObj, key, method, kind)
				}
			}
		case chunk.OP_PUSH_PROPERTY:
			{
				v := vm.pop()
				k := vm.pop()
				class := vm.peekN(1)
				classObj, err := heap.GetObject(class.GetHandle())
				if err != nil {
					return value.UNDEFINED, fmt.Errorf("%s was not an object", native.String(class))
				}
				if c, ok := classObj.(*native.ObjClass); ok {
					c.PushProperty(k, v)
				} else {
					return value.UNDEFINED, fmt.Errorf("%s was not an class object", native.String(class))
				}
			}
		case chunk.OP_PUSH_CLASS_FIELD:
			{
				flags := c.Code[ip]
				ip++
				initializer := vm.pop()
				key := vm.pop()
				classValue := vm.peekN(1)
				classObj, _ := heap.GetObject(classValue.GetHandle())
				class := classObj.(*native.ObjClass)
				initializer = cloneFunctionValue(initializer, f.heapScope)
				if initializerObj, err := heap.GetObject(initializer.GetHandle()); err == nil {
					if fn, ok := initializerObj.(*object.ObjFunction); ok {
						fn.HomeObject = classValue
					}
				}
				if flags&1 != 0 {
					result, err := vm.callValue(initializer, classValue, nil)
					if err != nil {
						errValue := vm.errorValue(err)
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
					if flags&2 != 0 {
						native.DefineClassMember(class, key, result, native.CLASS_FIELD)
					} else {
						class.SetMember(key, result)
					}
				} else {
					class.AddInstanceField(key, initializer)
				}
			}
		case chunk.OP_RUN_STATIC_BLOCK:
			{
				block := vm.pop()
				classValue := vm.peekN(1)
				block = cloneFunctionValue(block, f.heapScope)
				if blockObj, err := heap.GetObject(block.GetHandle()); err == nil {
					if fn, ok := blockObj.(*object.ObjFunction); ok {
						fn.HomeObject = classValue
					}
				}
				if _, err := vm.callValue(block, classValue, nil); err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
				}
			}
		case chunk.OP_INIT_INSTANCE_FIELDS:
			{
				instance, ok := vm.instanceFromValue(f.thisCtx)
				if !ok {
					errValue := vm.makeError("ReferenceError", "Must call super constructor before accessing 'this'")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				home, _ := heap.GetObject(f.fn.(*object.ObjFunction).HomeObject.GetHandle())
				class, ok := home.(*native.ObjClass)
				if !ok {
					errValue := vm.makeError("TypeError", "Invalid class home object")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				if err := vm.initializeClassInstance(instance, class); err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
				}
			}
		case chunk.OP_SUPER_CALL:
			{
				argCount := int(c.Code[ip])
				ip++
				ip++
				args := vm.popN(argCount)
				instance, ok := vm.instanceFromValue(f.thisCtx)
				if !ok {
					errValue := vm.makeError("ReferenceError", "Must call super constructor before accessing 'this'")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				if instance.Initialized() {
					errValue := vm.makeError("ReferenceError", "Super constructor may only be called once")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				home, _ := heap.GetObject(f.fn.(*object.ObjFunction).HomeObject.GetHandle())
				class, ok := home.(*native.ObjClass)
				if !ok {
					errValue := vm.makeError("TypeError", "Invalid class home object")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				if class.SuperClass.IsObject() {
					if parentObj, err := heap.GetObject(class.SuperClass.GetHandle()); err == nil {
						if parentClass, ok := parentObj.(*native.ObjClass); ok {
							if constructor, found := vm.classConstructor(parentClass); found {
								if _, err := vm.callClassConstructor(constructor, f.thisCtx, args, f.newTarget); err != nil {
									errValue := vm.errorValue(err)
									if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
										return value.UNDEFINED, err
									}
									continue
								}
							} else if err := vm.initializeClassInstance(instance, parentClass); err != nil {
								errValue := vm.errorValue(err)
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
						} else if parentError, ok := parentObj.(*native.ErrorConstructor); ok {
							params := make([]any, len(args))
							for i, arg := range args {
								params[i] = arg
							}
							errorObject, err := parentError.New(params...)
							if err != nil {
								errValue := vm.errorValue(err)
								if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
									return value.UNDEFINED, err
								}
								continue
							}
							if errObj, ok := errorObject.(*native.ObjError); ok {
								for key, entry := range errObj.Members {
									if key == native.PROTOTYPE_PROPERTY_STRING {
										continue
									}
									instance.Members[key] = entry
								}
							}
						}
					}
				}
				if err := vm.initializeClassInstance(instance, class); err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				instance.SetInitialized(true)
				vm.push(value.UNDEFINED)
			}
		case chunk.OP_SUPER_GET:
			{
				key := vm.pop()
				base := vm.superBase(f.fn.(*object.ObjFunction).HomeObject, f.thisCtx)
				if !base.IsObject() {
					vm.push(value.UNDEFINED)
					continue
				}
				member, err := vm.getMemberValueWithReceiver(base, key, f.thisCtx)
				if err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				if member.IsObject() {
					if memberObj, err := heap.GetObject(member.GetHandle()); err == nil {
						if _, ok := memberObj.(object.NeedsContext); ok {
							vm.push(f.thisCtx)
							vm.push(member)
							vm.push(value.TAG_METHOD_HANDLE)
							continue
						}
					}
				}
				vm.push(member)
			}
		case chunk.OP_SUPER_SET_RETURN:
			{
				v := vm.pop()
				key := vm.pop()
				fn, ok := f.fn.(*object.ObjFunction)
				if !ok {
					errValue := vm.makeError("TypeError", "Invalid super property access")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				base := vm.superBase(fn.HomeObject, f.thisCtx)
				if err := vm.setMemberValueWithReceiver(base, key, v, f.thisCtx, false); err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(v)
			}
		case chunk.OP_GET_PRIVATE_MEMBER:
			{
				key := vm.pop()
				objValue := vm.pop()
				if _, ok := vm.privateTarget(objValue, key); !ok {
					errValue := vm.makeError("TypeError", "Cannot read private member from an object whose class did not declare it")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				member, err := vm.getMemberValue(objValue, key)
				if err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				if member.IsObject() {
					if memberObj, err := heap.GetObject(member.GetHandle()); err == nil {
						if _, ok := memberObj.(object.NeedsContext); ok {
							vm.push(objValue)
							vm.push(member)
							vm.push(value.TAG_METHOD_HANDLE)
							continue
						}
					}
				}
				vm.push(member)
			}
		case chunk.OP_SET_PRIVATE_MEMBER_RETURN, chunk.OP_SET_PRIVATE_MEMBER_STRICT_RETURN:
			{
				v := vm.pop()
				key := vm.pop()
				objValue := vm.pop()
				if _, ok := vm.privateTarget(objValue, key); !ok {
					errValue := vm.makeError("TypeError", "Cannot write private member to an object whose class did not declare it")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				var err error
				if code == chunk.OP_SET_PRIVATE_MEMBER_STRICT_RETURN {
					err = vm.setMemberValueStrict(objValue, key, v, true)
				} else {
					err = vm.setMemberValue(objValue, key, v)
				}
				if err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(v)
			}
		case chunk.OP_HAS_PRIVATE_MEMBER:
			{
				key := vm.pop()
				objValue := vm.pop()
				if !objValue.IsObject() {
					errValue := vm.makeError("TypeError", "Cannot use 'in' operator to search for private name in primitive")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				_, ok := vm.privateTarget(objValue, key)
				vm.push(boolValue(ok))
			}
		case chunk.OP_THIS:
			{
				if fn, ok := f.fn.(*object.ObjFunction); ok && fn.IsArrow {
					vm.push(fn.LexicalThis)
				} else {
					if instance, ok := vm.instanceFromValue(f.thisCtx); ok && !instance.Initialized() {
						errValue := vm.makeError("ReferenceError", "Must call super constructor before accessing 'this'")
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
					vm.push(f.thisCtx)
				}
			}
		case chunk.OP_NEW_TARGET:
			{
				if fn, ok := f.fn.(*object.ObjFunction); ok && fn.IsArrow {
					vm.push(fn.LexicalNewTarget)
				} else {
					vm.push(f.newTarget)
				}
			}
		case chunk.OP_YIELD:
			{
				generator := f.fn.(*native.ObjGenerator)
				result := vm.suspendGenerator(generator, ip, &f, vm.pop())
				ip = f.returnIp
				vm.frameCount--
				f = vm.currentFrame()
				c = *f.fn.ValueChunk()
				vm.push(result)
			}
		case chunk.OP_YIELD_DELEGATE:
			{
				generator := f.fn.(*native.ObjGenerator)
				if generator.DelegateHasResult {
					resultValue := generator.DelegateResult
					generator.DelegateHasResult = false
					generator.DelegateResult = value.UNDEFINED
					vm.push(resultValue)
					continue
				}
				if !generator.Delegating {
					delegate := vm.pop()
					iterator, err := vm.acquireIterator(delegate, compiler.ITERATOR_FOR_OF)
					if err != nil {
						errValue := vm.makeError("TypeError", err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						continue
					}
					generator.DelegateIterator = iterator
					generator.Delegating = true
					generator.DelegateStarted = false
				}
				input := value.UNDEFINED
				if generator.DelegateStarted {
					input = vm.pop()
				} else {
					generator.DelegateStarted = true
				}
				result, err := vm.iteratorNextWithInput(generator.DelegateIterator, input)
				if err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				done, resultValue, err := vm.iteratorResultParts(result)
				if err != nil {
					errValue := vm.makeError("TypeError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				if done {
					generator.Delegating = false
					generator.DelegateIterator = value.UNDEFINED
					generator.DelegateStarted = false
					vm.push(resultValue)
					continue
				}
				yieldResult := vm.suspendGenerator(generator, ip-1, &f, resultValue)
				ip = f.returnIp
				vm.frameCount--
				f = vm.currentFrame()
				c = *f.fn.ValueChunk()
				vm.push(yieldResult)
			}
		case chunk.OP_IMPORT_MODULE:
			{
				source := vm.pop()
				pathString := ""
				if source.IsObject() {
					if sourceObj, err := heap.GetObject(source.GetHandle()); err == nil {
						pathString = sourceObj.String()
					}
				}
				result, err := vm.importModule(pathString, "")
				if err != nil {
					errValue := value.UNDEFINED
					if thrown, ok := err.(*moduleThrownError); ok {
						errValue = thrown.value
					} else {
						errValue = vm.errorValue(err)
					}
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(result)
			}
		case chunk.OP_GET_MODULE_NAMESPACE:
			{
				source := vm.pop()
				pathString := ""
				if source.IsObject() {
					if sourceObj, err := heap.GetObject(source.GetHandle()); err == nil {
						pathString = sourceObj.String()
					}
				}
				record, err := compiler.LoadModule(pathString, "")
				if err != nil {
					errValue := vm.makeError("SyntaxError", err.Error())
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				vm.push(vm.moduleNamespaceValue(record))
			}
		case chunk.OP_DYNAMIC_IMPORT:
			{
				parentValue := vm.pop()
				specifierValue := vm.pop()
				parent := ""
				if parentValue.IsObject() {
					if parentObj, err := heap.GetObject(parentValue.GetHandle()); err == nil {
						parent = parentObj.String()
					}
				}
				specifier, err := vm.toStringValue(specifierValue)
				if err != nil {
					errValue := vm.errorValue(err)
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
						return value.UNDEFINED, err
					}
					continue
				}
				specifierString := ""
				if specifier.IsObject() {
					if specifierObj, err := heap.GetObject(specifier.GetHandle()); err == nil {
						specifierString = specifierObj.String()
					}
				}
				vm.push(vm.dynamicImport(specifierString, parent))
			}
		case chunk.OP_IMPORT:
			{
				source := vm.pop()
				src, _ := heap.GetObject(source.GetHandle())

				str := src.String()
				prev := ROOT_SCRIPT_LOCATION

				if str[0] == '.' {
					str = ROOT_SCRIPT_LOCATION + str[1:]
					ROOT_SCRIPT_LOCATION = strings.Join(strings.Split(str, "/")[:len(strings.Split(str, "/"))-1], "/")
				}

				module, err := compiler.CompileModule(str)

				if err != nil {
					return value.UNDEFINED, fmt.Errorf("failed to parser module %e", err)
				}

				importer := NewVM(false, vm.output)

				f, c := importer.Call(module, nil, value.UNDEFINED, 0, false)
				importer.run(f, c)

				imports[str] = value.EncodeHandle(heap.Allocate(importer.exports))
				ROOT_SCRIPT_LOCATION = prev
			}
		case chunk.OP_EXPORT:
			{
				k := vm.pop()
				v := vm.pop()

				if v == value.TAG_METHOD_HANDLE {
					v = vm.pop()
					vm.pop() // pop this context
				}

				if vm.exports == nil {
					vm.exports = native.NewObjectHash()
				}

				vm.exports.SetMember(k, v)
			}
		case chunk.OP_NOT:
			{
				arg := vm.pop()
				if truthyValue(arg) {

					vm.push(value.FALSE)
				} else {
					vm.push(value.TRUE)
				}
			}
		case chunk.OP_CREATE_REST_OBJECT:
			{
				origin := vm.pop()
				obj, _ := heap.GetObject(origin.GetHandle())

				amountOfExludedKeys := c.Code[ip]
				ip++

				exludeMap := map[value.Value]struct{}{}

				for range amountOfExludedKeys {
					key := c.Constants[c.Code[ip]]
					ptr := value.EncodeHandle(heap.GetPointer(key.GetHandle()))
					exludeMap[ptr] = struct{}{}
					ip++
				}

				newObj := native.NewObjectHash()

				if src, ok := obj.(*native.ObjObject); ok {
					for keyString, entry := range src.Members {
						if keyString == native.PROTOTYPE_PROPERTY_STRING || !entry.Enumerable() {
							continue
						}
						k := keyValueFromString(keyString)
						if k.IsObject() {
							ptr := value.EncodeHandle(heap.GetPointer(k.GetHandle()))
							if _, found := exludeMap[ptr]; found {
								continue
							}
						}
						v, err := vm.getMemberValue(origin, k)
						if err != nil {
							return value.UNDEFINED, err
						}
						newObj.SetMember(k, v)
					}
				}
				vm.push(value.EncodeHandle(heap.Allocate(newObj)))
			}
		case chunk.OP_CREATE_REST_OBJECT_DYNAMIC:
			{
				count := int(c.Code[ip])
				ip++
				excluded := make([]value.Value, count)
				for i := count - 1; i >= 0; i-- {
					excluded[i] = vm.pop()
				}
				origin := vm.pop()
				obj, _ := heap.GetObject(origin.GetHandle())
				newObj := native.NewObjectHash()
				if src, ok := obj.(*native.ObjObject); ok {
					for keyString, entry := range src.Members {
						if keyString == native.PROTOTYPE_PROPERTY_STRING || !entry.Enumerable() {
							continue
						}
						k := keyValueFromString(keyString)
						skip := false
						for _, ex := range excluded {
							if strictEquals(k, ex) {
								skip = true
								break
							}
						}
						if skip {
							continue
						}
						v, err := vm.getMemberValue(origin, k)
						if err != nil {
							return value.UNDEFINED, err
						}
						newObj.SetMember(k, v)
					}
				}
				vm.push(value.EncodeHandle(heap.Allocate(newObj)))
			}
		case chunk.OP_SET_FROM_SPREAD:
			{
				source := vm.pop()
				destination := vm.peek()
				obj, _ := heap.GetObject(source.GetHandle())

				dstObj, _ := heap.GetObject(destination.GetHandle())

				if src, ok := obj.(*native.ObjObject); ok {
					if dst, ok := dstObj.(*native.ObjObject); ok {
						for keyString, entry := range src.Members {
							if keyString == native.PROTOTYPE_PROPERTY_STRING || !entry.Enumerable() {
								continue
							}
							k := keyValueFromString(keyString)
							v, err := vm.getMemberValue(source, k)
							if err != nil {
								return value.UNDEFINED, err
							}
							dst.SetMember(k, v)
						}
					}
				} else if s, ok := vm.valueAsString(source); ok {
					if dst, ok := dstObj.(*native.ObjObject); ok {
						for i, r := range []rune(s) {
							dst.SetMember(native.IndexValue(i), value.EncodeHandle(heap.Allocate(native.NewLightString(string(r)))))
						}
					}
				}
			}
		}
	}
}

func setHeapScopes(c *value.ValueChunk, heapScope int) error {
	for _, v := range c.Constants {
		if v.IsObject() {
			obj, err := heap.GetObject(v.GetHandle())

			if err != nil {
				return err
			}

			if obj, ok := obj.(*object.ObjFunction); ok && obj.GetHeapScope() <= heapScope {
				obj.SetHeapScope(heapScope)
				setHeapScopes(obj.ValueChunk(), heapScope)
			}
		}
	}
	return nil
}

func (vm *VM) log(arg value.Value) {
	fmt.Printf("%s\n", native.String(arg))
}
