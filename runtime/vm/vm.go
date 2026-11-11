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

func (vm *VM) ensureGlobalObject() value.Value {
	if vm.globalObject == 0 {
		vm.globalObject = value.EncodeHandle(heap.Allocate(native.NewGlobalObject()))
	}
	return vm.globalObject
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
	stackTop int
	jumpTo   int
	frame    int
}

type VM struct {	frames         []CallFrame
	frameCount     int
	stack          []value.Value
	stackTop       int
	exceptionStack []ExceptionState
	callCounts     map[object.Callable]int
	output         *strings.Builder

	exports *native.ObjObject

	completionValue value.Value
	completionPending bool
	globalObject    value.Value
	catchValues     map[int]value.Value

	main bool
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
		return
	}
	globals[globalsCount] = v
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
	if native.SYMBOL_ITERATOR != 0 {
		method, err := vm.getMemberValue(iteratee, native.SYMBOL_ITERATOR)
		if err == nil && method != value.UNDEFINED {
			iterator, err := vm.callValue(method, iteratee, nil)
			if err != nil {
				return value.UNDEFINED, err
			}
			if !iterator.IsObject() {
				return value.UNDEFINED, fmt.Errorf("Result of the Symbol.iterator method is not an object")
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
			return false, fmt.Errorf("Iterator result is not an object")
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
			return false, fmt.Errorf("Iterator result is not an object")
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
	return v
}

func (vm *VM) generatorNext(generator *native.ObjGenerator, args []value.Value) (value.Value, error) {
	if generator.Done {
		result := native.NewObjectHash()
		result.SetMember(native.KEY_VALUE, value.UNDEFINED)
		result.SetMember(native.KEY_DONE, value.TRUE)
		return value.EncodeHandle(heap.Allocate(result)), nil
	}
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
		return runner.run(f, c)
	}
	f, c := runner.Call(generator, &dummyIp, value.UNDEFINED, 0, false)
	runner.stackTop = f.localStart
	for _, local := range generator.Locals {
		runner.push(local)
	}
	input := value.UNDEFINED
	if len(args) > 0 {
		input = args[0]
	}
	runner.push(input)
	f.startIp = generator.Ip
	runner.frames[runner.frameCount-1].startIp = generator.Ip
	return runner.run(f, c)
}

func (vm *VM) construct(callable object.Callable, args []value.Value, newTarget value.Value) (value.Value, error) {
	instance := native.NewObject()
	if fn, ok := callable.(*object.ObjFunction); ok {
		if fn.Prototype == value.UNDEFINED {
			proto := native.NewObject()
			proto.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("constructor"))), value.EncodeHandle(heap.Allocate(fn)))
			fn.Prototype = value.EncodeHandle(heap.Allocate(proto))
		}
		if fn.Prototype.IsObject() {
			instance.SetPrototypeValue(fn.Prototype)
		}
	}
	instanceValue := value.EncodeHandle(heap.Allocate(instance))
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

func (vm *VM) addPromiseObserver(p *native.ObjPromise, fulfilled func(value.Value), rejected func(value.Value)) {
	observer := native.PromiseObserver{Fulfilled: fulfilled, Rejected: rejected}
	if p.Status == native.PENDING {
		p.Observers = append(p.Observers, observer)
		return
	}
	status := p.Status
	argument := p.Value
	queue.Enqueue(object.Callback{Native: func() {
		if status == native.RESOLVED {
			if fulfilled != nil {
				fulfilled(argument)
			}
		} else if rejected != nil {
			rejected(argument)
		}
	}}, queue.MICRO_TASK, false)
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
		queue.Enqueue(object.Callback{Native: func() {
			if fulfilled {
				if observer.Fulfilled != nil {
					observer.Fulfilled(argument)
				}
			} else if observer.Rejected != nil {
				observer.Rejected(argument)
			}
		}}, queue.MICRO_TASK, false)
	}
	for _, reaction := range reactions {
		reaction := reaction
		fulfilled := p.Status == native.RESOLVED
		argument := p.Value
		queue.Enqueue(object.Callback{Native: func() {
			vm.promiseReaction(reaction, fulfilled, argument)
		}}, queue.MICRO_TASK, false)
	}
}

func (vm *VM) attachPromiseReaction(p *native.ObjPromise, reaction native.PromiseReaction) {
	if p.Status == native.PENDING {
		p.Reactions = append(p.Reactions, reaction)
		return
	}
	fulfilled := p.Status == native.RESOLVED
	argument := p.Value
	queue.Enqueue(object.Callback{Native: func() {
		vm.promiseReaction(reaction, fulfilled, argument)
	}}, queue.MICRO_TASK, false)
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
				vm.addPromiseObserver(p, func(value.Value) {
					if fulfilled {
						vm.resolvePromise(derived, argument)
					} else {
						vm.rejectPromise(derived, argument)
					}
				}, func(reason value.Value) {
					vm.rejectPromise(derived, reason)
				})
				return
			}
		}
		if fulfilled {
			vm.resolvePromise(derived, argument)
		} else {
			vm.rejectPromise(derived, argument)
		}
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
				queue.Enqueue(object.Callback{Native: func() {
					resolve := value.EncodeHandle(heap.Allocate(native.NewResolveFunc(p)))
					reject := value.EncodeHandle(heap.Allocate(native.NewRejectFunc(p)))
					if _, err := vm.callValue(thenValue, v, []value.Value{resolve, reject}); err != nil {
						if p.Status == native.PENDING {
							vm.rejectPromise(p, vm.errorValue(err))
						}
					}
				}}, queue.MICRO_TASK, false)
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

func (vm *VM) callValue(fn value.Value, this value.Value, args []value.Value) (value.Value, error) {
	obj, err := heap.GetObject(fn.GetHandle())
	if err != nil {
		return value.UNDEFINED, err
	}
	callable, ok := obj.(object.Callable)
	if !ok {
		return value.UNDEFINED, fmt.Errorf("value is not callable")
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
	if !objValue.IsObject() {
		return fmt.Errorf("cannot set property on non-object")
	}
	obj, err := heap.GetObject(objValue.GetHandle())
	if err != nil {
		return err
	}
	if k, ok := vm.valueAsString(key); ok {
		key = value.EncodeHandle(heap.Allocate(native.NewLightString(k)))
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
	if own, ok := obj.(interface {
		GetOwnEntry(value.Value) (native.ObjectValueEntry, bool)
		PrototypeValue() value.Value
	}); ok {
		if entry, found := own.GetOwnEntry(key); found {
			if entry.IsAccessor() {
				if entry.Set != value.UNDEFINED {
					_, err := vm.callValue(entry.Set, objValue, []value.Value{v})
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
							_, err := vm.callValue(entry.Set, objValue, []value.Value{v})
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
				if fn.Prototype == value.UNDEFINED {
					proto := native.NewObject()
					proto.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("constructor"))), objValue)
					fn.Prototype = value.EncodeHandle(heap.Allocate(proto))
				}
				return fn.Prototype, nil
			}
		}
		if keyString, ok := vm.valueAsString(key); ok {
			if v, found := fn.Props[keyString]; found {
				return v, nil
			}
		}
	}
	if class, ok := obj.(*native.ObjClass); ok {
		if keyString, ok := vm.valueAsString(key); ok && keyString == "prototype" {
			return class.GetMember(native.KEY_PROTO), nil
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
	if _, ok := obj.(*native.ObjGenerator); ok && key == native.SYMBOL_ITERATOR {
		return value.EncodeHandle(heap.Allocate(native.NewGeneratorIterator())), nil
	}
	if _, ok := obj.(*object.Iterator); ok {
		if keyString, ok := vm.valueAsString(key); ok && keyString == "next" {
			return value.EncodeHandle(heap.Allocate(native.NewIteratorNext())), nil
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
		if _, ok := methodObj.(object.Callable); !ok {
			continue
		}
		foundMethod = true
		result, err := vm.callValue(method, v, nil)
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
		switch obj.(type) {
		case *object.ObjFunction, *native.ObjAsyncFunction, *native.ObjGenerator, *native.ObjClass, *native.ObjNativeFn:
			return "function"
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
	case *object.ObjFunction:
		if ctorObj.IsArrow || ctorObj.IsMethod {
			return false, fmt.Errorf("right-hand side of instanceof is not callable")
		}
		prototype, err = vm.getMemberValue(ctor, value.EncodeHandle(heap.Allocate(native.NewLightString("prototype"))))
		if err != nil {
			return false, err
		}
	case *native.ObjClass:
		prototype = ctorObj.GetMember(native.KEY_PROTO)
	case *native.ErrorConstructor:
		obj, err := heap.GetObject(objValue.GetHandle())
		if err != nil {
			return false, err
		}
		errObj, ok := obj.(*native.ObjError)
		if !ok {
			return false, nil
		}
		if ctorObj.ErrorName() == "Error" {
			return true, nil
		}
		nameValue := errObj.GetMember(native.KEY_NAME)
		name, _ := vm.valueAsString(nameValue)
		return name == ctorObj.ErrorName(), nil
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
		arr := native.NewArrayFrom(items)
		v := value.EncodeHandle(heap.Allocate(arr))
		if objFn, ok := fn.(*object.ObjFunction); ok && objFn.ArgumentsHeapSlot >= 0 {
			f.arguments = v
			vm.frames[vm.frameCount-1].arguments = v
		} else {
			vm.push(v)
		}
	}

	if fn.HasRestParameter() {
		arguments := vm.findArgStart()
		f.localStart = vm.stackTop

		arr := native.NewArrayFrom(arguments[fn.GetArity()-1:])
		for _, v := range arguments[:fn.GetArity()-1] {
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

		f, c := vm.Call(callback.Fn, nil, callback.ThisCtx, callback.Fn.GetArity(), false)
		_, err := vm.run(f, c)

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
			f.localStart = promise.State.LocalStart
			f.heapScope = promise.State.HeapScope
			f.ownScope = promise.State.OwnScope
			vm.stackTop = f.localStart
			vm.frames[vm.frameCount-1].localStart = f.localStart
			vm.frames[vm.frameCount-1].heapScope = f.heapScope
			vm.frames[vm.frameCount-1].ownScope = f.ownScope
			for _, v := range promise.State.Stack {
				vm.push(v)
			}
			heap.ClearAsyncFunctionStack(uintptr(unsafe.Pointer(promise)))

			ip = promise.State.Ip
			if promise.HasRejection {
				rejectValue := promise.RejectValue
				promise.HasRejection = false
				if err := vm.throwValue(rejectValue, &f, &c, &ip); err != nil {
					return value.UNDEFINED, err
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
						return value.UNDEFINED, err
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
					return value.UNDEFINED, err
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
					vm.push(value.EncodeHandle(heap.Allocate(native.NewLightString(as + bs))))
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
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil { return value.UNDEFINED, err }
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
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil { return value.UNDEFINED, err }
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
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil { return value.UNDEFINED, err }
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
						if strings.Contains(err.Error(), "Division by zero") || strings.Contains(err.Error(), "Exponent") { name = "RangeError" }
						errValue := vm.makeError(name, err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil { return value.UNDEFINED, err }
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
						if strings.Contains(err.Error(), "Division by zero") || strings.Contains(err.Error(), "Exponent") { name = "RangeError" }
						errValue := vm.makeError(name, err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil { return value.UNDEFINED, err }
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
						if strings.Contains(err.Error(), "Division by zero") || strings.Contains(err.Error(), "Exponent") { name = "RangeError" }
						errValue := vm.makeError(name, err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil { return value.UNDEFINED, err }
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
						if strings.Contains(err.Error(), "Division by zero") || strings.Contains(err.Error(), "Exponent") { name = "RangeError" }
						errValue := vm.makeError(name, err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil { return value.UNDEFINED, err }
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
						if strings.Contains(err.Error(), "Division by zero") || strings.Contains(err.Error(), "Exponent") { name = "RangeError" }
						errValue := vm.makeError(name, err.Error())
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil { return value.UNDEFINED, err }
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

				has := value.FALSE

				if obj.IsObject() {
					objValue, err := heap.GetObject(obj.GetHandle())
					if err == nil {
						if hashable, ok := objValue.(object.Hashable); ok && hashable.HasOwn(prop) {
							has = value.TRUE
						} else if v, err := vm.getMemberValue(obj, prop); err == nil && v != value.UNDEFINED {
							has = value.TRUE
						}
					}
				}

				vm.push(has)
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
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil { return value.UNDEFINED, err }
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
						if err := vm.throwValue(errValue, &f, &c, &ip); err != nil { return value.UNDEFINED, err }
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
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil { return value.UNDEFINED, err }
					continue
				}
				if _, bBig := vm.bigIntObject(b); bBig {
					errValue := vm.makeError("TypeError", "BigInts have no unsigned right shift")
					if err := vm.throwValue(errValue, &f, &c, &ip); err != nil { return value.UNDEFINED, err }
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
				result := true
				if objValue.IsObject() {
					obj, _ := heap.GetObject(objValue.GetHandle())
					if d, ok := obj.(object.Deletable); ok {
						result = d.Delete(k)
					}
				}
				vm.push(boolValue(result))
			}
		case chunk.OP_DELETE_STRICT:
			{
				k := vm.pop()
				objValue := vm.pop()
				result := true
				if objValue.IsObject() {
					obj, _ := heap.GetObject(objValue.GetHandle())
					if d, ok := obj.(object.Deletable); ok {
						result = d.Delete(k)
					}
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
				vm.stackTop--
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
					obj, err := heap.GetObject(v.GetHandle())

					if err != nil {
						return value.UNDEFINED, fmt.Errorf("failed to receive object at OP_DEFINE_LOCAL %s", err)
					}
					switch obj.(type) {
					case *object.ObjFunction:
						{
							v = cloneFunctionValue(v, f.heapScope)
						}
					}
				}
				vm.stack[vm.stackTop] = v
				vm.stackTop++
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
					if _, ok := obj.(*object.ObjFunction); ok {
						v = cloneFunctionValue(v, f.heapScope)
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
							handle := heap.Allocate(promise)
							vm.push(value.EncodeHandle(handle))
						}

						f, c = vm.Call(fn.Clone(), &ip, value.UNDEFINED, argCount, calledWithSpread)
						f.localStart -= int(argCount)
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
				case *native.ReflectDeleteProperty:
					{
						key := vm.pop()
						target := vm.pop()
						result := false
						if target.IsObject() {
							if obj, err := heap.GetObject(target.GetHandle()); err == nil {
								if d, ok := obj.(object.Deletable); ok {
									result = d.Delete(key)
								}
							}
						}
						vm.push(boolValue(result))
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
							result.SetMember(native.KEY_DONE, value.FALSE)
							result.SetMember(native.KEY_VALUE, current)
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
							errValue := vm.makeError("TypeError", err.Error())
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
						this, _ := heap.GetObject(thisCtx.GetHandle())
						handle := heap.Allocate(native.NewObjString(fn.ToString(this)))
						vm.push(value.EncodeHandle(handle))
					}
				case *native.Log:
					{

						args := vm.popN(argCount)
						fn.Log(args, vm.output)
						vm.push(value.UNDEFINED)
					}
				case *native.Next:
					{
						args := vm.popN(argCount)
						this, _ := heap.GetObject(thisCtx.GetHandle())
						generator := this.(*native.ObjGenerator)
						if generator.Done {
							result := native.NewObjectHash()
							result.SetMember(native.KEY_VALUE, value.UNDEFINED)
							result.SetMember(native.KEY_DONE, value.TRUE)
							vm.push(value.EncodeHandle(heap.Allocate(result)))
							continue
						}
						if !generator.Started {
							generator.Started = true
							for _, arg := range generator.Args {
								vm.push(arg)
							}
							f, c = vm.Call(generator, &ip, value.UNDEFINED, len(generator.Args), false)
							ip = 0
						} else {
							f, c = vm.Call(generator, &ip, value.UNDEFINED, 0, false)
							vm.stackTop = f.localStart
							for _, local := range generator.Locals {
								vm.push(local)
							}
							input := value.UNDEFINED
							if len(args) > 0 {
								input = args[0]
							}
							vm.push(input)
							ip = generator.Ip
						}
					}
				case *native.GeneratorIterator:
					{
						vm.push(thisCtx)
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
						generator.Done = true
						result := native.NewObjectHash()
						result.SetMember(native.KEY_VALUE, returned)
						result.SetMember(native.KEY_DONE, value.TRUE)
						vm.push(value.EncodeHandle(heap.Allocate(result)))
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
						if !generator.Started || generator.Done {
							errValue := thrown
							if err := vm.throwValue(errValue, &f, &c, &ip); err != nil {
								return value.UNDEFINED, err
							}
							continue
						}
						generator.PendingThrow = thrown
						generator.HasPendingThrow = true
						generator.Started = true
						f, c = vm.Call(generator, &ip, value.UNDEFINED, 0, false)
						vm.stackTop = f.localStart
						for _, local := range generator.Locals {
							vm.push(local)
						}
						ip = generator.Ip
						if err := vm.throwValue(generator.PendingThrow, &f, &c, &ip); err != nil {
							return value.UNDEFINED, err
						}
						generator.HasPendingThrow = false
					}
				case *native.ObjectKeys:
					{
						arg := vm.pop()
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
						vm.resolvePromise(fn.Promise(), v)
						vm.push(value.UNDEFINED)
					}
				case *native.RejectFunc:
					{
						v := vm.pop()
						vm.rejectPromise(fn.Promise(), v)
						vm.push(value.UNDEFINED)
					}
				case *native.SetTimeout:
					{
						ms := vm.pop().AsNumber()
						callback := vm.pop()

						handle := callback.GetHandle()
						obj, err := heap.GetObject(handle)

						if err != nil {
							return value.UNDEFINED, err
						}

						if callback, ok := obj.(*object.ObjFunction); ok {
							fn.Set(int(ms), callback, thisCtx)
							eventloop.Dispatch(fn.Clone())
							vm.push(value.UNDEFINED)
						}
					}
				case *native.Now:
					vm.push(fn.Now())
				case *native.Create:
					{
						arg := vm.pop()
						o := native.NewObjectFromObject(arg)
						vm.push(value.EncodeHandle(heap.Allocate(o)))
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
				case *native.ObjectGetOwnPropertySymbols:
					{
						target := vm.pop()
						symbols := fn.Symbols(target)
						vm.push(value.EncodeHandle(heap.Allocate(native.NewArrayFrom(symbols))))
					}
				case *native.ObjectGetOwnPropertyNames:
					{
						target := vm.pop()
						names := fn.Names(target)
						vm.push(value.EncodeHandle(heap.Allocate(native.NewArrayFrom(names))))
					}
				case *native.ObjectDefineProperty:
					{
						descriptor := vm.pop()
						key := vm.pop()
						target := vm.pop()
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
						vm.push(fn.Descriptor(target, key))
					}
				case *native.ObjectGetPrototypeOf:
					{
						target := vm.pop()
						vm.push(fn.GetPrototypeOf(target))
					}
				case *native.ObjectSetPrototypeOf:
					{
						proto := vm.pop()
						target := vm.pop()
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
						queue.Enqueue(object.Callback{Fn: fn.(object.Callable), ThisCtx: value.UNDEFINED, Stack: []value.Value{v}}, queue.MICRO_TASK, false)
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
				calledWithSpread := spread == 1
				f, c = vm.Call(fn, &ip, thisCtx, argCount, calledWithSpread)
			}
		case chunk.OP_RETURN:
			{
				if generator, ok := f.fn.(*native.ObjGenerator); ok {
					generator.Done = true
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

					promise.Resolve(value)

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
		case chunk.OP_ITERATOR_CURRENT:
			{
				iterator := vm.peek()
				current, err := vm.iteratorCurrent(iterator)
				if err != nil {
					return value.UNDEFINED, err
				}
				vm.push(current)
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
		case chunk.OP_CREATE_HEAP_SCOPE:
			{
				f.heapScope = heap.CreateHeapScope(f.heapScope)
				f.ownScope = true
			}
		case chunk.OP_CLONE_FUNCTION:
			{
				v := vm.peek()
				if v.IsObject() {
					if obj, err := heap.GetObject(v.GetHandle()); err == nil {
						if fn, ok := obj.(*object.ObjFunction); ok {
							vm.pop()
							clone := fn.Clone().(*object.ObjFunction)
							clone.Cloned = true
							clone.SetHeapScope(f.heapScope)
							if clone.IsArrow {
								clone.LexicalThis = f.thisCtx
								if outer, ok := f.fn.(*object.ObjFunction); ok && outer.IsArrow {
									clone.LexicalNewTarget = outer.LexicalNewTarget
								} else {
									clone.LexicalNewTarget = f.newTarget
								}
							}
							vm.push(value.EncodeHandle(heap.Allocate(clone)))
						}
					}
				}
			}
		case chunk.OP_ENSURE_HEAP_SCOPE:
			{
				if !f.ownScope {
					f.heapScope = heap.CreateHeapScope(f.heapScope)
					f.ownScope = true
				}
			}
		case chunk.OP_COPY_HEAP_SCOPE:
			{
				f.heapScope = heap.CopyHeapScope(f.heapScope)
			}
		case chunk.OP_TRY_BLOCK_START:
			{
				catchStart := int(c.Code[ip+3]) | int(c.Code[ip+2])<<8 | int(c.Code[ip+1])<<16 | int(c.Code[ip])<<24
				ip += 4
				vm.exceptionStack = append(vm.exceptionStack, ExceptionState{jumpTo: catchStart, stackTop: vm.stackTop, frame: vm.frameCount})
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
				case *native.ObjClass:
					{
						instance := ctor.NewInstance()
						ctor := instance.GetMember(native.KEY_CTOR)
						instanceValue := value.EncodeHandle(heap.Allocate(instance))
						if !ctor.IsObject() {
							vm.popN(int(argCount))
							vm.push(instanceValue)
							continue
						}

						obj, err := heap.GetObject(ctor.GetHandle())

						if err != nil {
							return value.UNDEFINED, fmt.Errorf("contructor was not an object %s", native.String(ctor))
						}

						if constructor, ok := obj.(object.Callable); ok {
							builder := NewVM(false, vm.output)
							fn := object.NewFunction("builder", 0, nil)
							fn.ValueChunk().EmitBytes(chunk.OP_CALL, uint8(constructor.GetArity()), 0, chunk.OP_RETURN)
							instance := instanceValue

							for _, v := range vm.popN(constructor.GetArity()) {
								builder.push(v)
							}

							builder.push(instance)
							builder.push(ctor)
							builder.push(value.TAG_METHOD_HANDLE)
							f, c := builder.Call(fn, nil, value.UNDEFINED, argCount, false)
							builder.run(f, c)

							vm.push(instance)
						} else {
							return value.UNDEFINED, fmt.Errorf("contructor was not an function %s", native.String(ctor))
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
				if len(vm.exceptionStack) == 0 {
					if asyncFn, ok := f.fn.(*native.ObjAsyncFunction); ok {
						asyncFn.Promise.Reject(err)
						vm.frameCount--
						if vm.frameCount > 0 {
							ip = f.returnIp
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
				if curentAsyncFn, ok := f.fn.(*native.ObjAsyncFunction); ok {
					count := (vm.stackTop - f.localStart) + 1
					stack := make([]value.Value, count)
					copy(stack, append(vm.stack[f.localStart:vm.stackTop], value.EncodeHandle(heap.Allocate(promise))))
					curentAsyncFn.Pause(stack, ip, f.localStart, f.heapScope, f.ownScope)
					curentAsyncFn.Await(promise)
					heap.StoreAsyncFunctionStack(uintptr(unsafe.Pointer(curentAsyncFn)), stack)

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
					name := n.String()

					class := native.NewObjClass(name)
					proto := native.NewPrototype(name)

					vm.push(value.EncodeHandle(heap.Allocate(class)))
					vm.push(value.EncodeHandle(heap.Allocate(proto)))
				} else {
					return value.UNDEFINED, fmt.Errorf("%s is not a string", native.String(name))
				}
			}
		case chunk.OP_CREATE_CLASS_END:
			{
				proto := vm.pop()
				class := vm.peek()

				classObj, _ := heap.GetObject(class.GetHandle())
				protoObj, _ := heap.GetObject(proto.GetHandle())
				protoObj.(*native.Prototype).SetMember(native.KEY_PROTO, native.PROTOTYPE_OBJECT)
				classObj.(*native.ObjClass).SetPrototype(proto)
			}
		case chunk.OP_PUSH_METHOD:
			{
				method := vm.pop()
				methodObj, err := heap.GetObject(method.GetHandle())

				if err != nil {
					return value.UNDEFINED, fmt.Errorf("%s was not an object", native.String(method))
				}

				if _, ok := methodObj.(object.Callable); ok {
					method = cloneFunctionValue(method, f.heapScope)
				} else {
					return value.UNDEFINED, fmt.Errorf("%s was not an function", native.String(method))
				}

				key := vm.pop()
				prototype := vm.peek()

				protoObj, err := heap.GetObject(prototype.GetHandle())

				if err != nil {
					return value.UNDEFINED, fmt.Errorf("%s was not an object", native.String(prototype))
				}

				if p, ok := protoObj.(*native.Prototype); ok {
					p.SetMember(key, method)
				} else {
					return value.UNDEFINED, fmt.Errorf("%s was not an prototype", native.String(prototype))
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
		case chunk.OP_THIS:
			{
				if fn, ok := f.fn.(*object.ObjFunction); ok && fn.IsArrow {
					vm.push(fn.LexicalThis)
				} else {
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
				v := native.NewObjectHash()
				v.SetMember(native.KEY_VALUE, vm.pop())
				d := value.FALSE

				locals := make([]value.Value, vm.stackTop-f.localStart)
				for i, v := range vm.stack[f.localStart:vm.stackTop] {
					locals[i] = v
					vm.pop()
				}

				f.fn.(*native.ObjGenerator).Ip = ip
				f.fn.(*native.ObjGenerator).Locals = locals

				v.SetMember(native.KEY_DONE, d)

				ip = f.returnIp
				vm.frameCount--
				f = vm.currentFrame()
				c = *f.fn.ValueChunk()
				vm.push(value.EncodeHandle(heap.Allocate(v)))
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
