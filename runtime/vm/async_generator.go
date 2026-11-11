package vm

import (
	"go_js/compiler"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/value"
)

func (vm *VM) asyncGeneratorRequest(generator *native.ObjGenerator, input value.Value, mode generatorMode) value.Value {
	promise := native.NewPromise()
	promiseValue := value.EncodeHandle(heap.Allocate(promise))
	heap.PushToRoots(promiseValue)
	generator.AsyncRequests = append(generator.AsyncRequests, native.AsyncGeneratorRequest{Input: input, Mode: int(mode), Promise: promise})
	vm.startAsyncGenerator(generator)
	return promiseValue
}

func (vm *VM) startAsyncGenerator(generator *native.ObjGenerator) {
	if generator.AsyncProcessing || len(generator.AsyncRequests) == 0 {
		return
	}
	generator.AsyncProcessing = true
	request := generator.AsyncRequests[0]
	generator.AsyncRequests = generator.AsyncRequests[1:]
	result, err := vm.generatorResume(generator, request.Input, generatorMode(request.Mode))
	vm.finishAsyncGenerator(generator, request, result, err)
}

func (vm *VM) resumeAsyncGenerator(generator *native.ObjGenerator, request native.AsyncGeneratorRequest, input value.Value, rejected bool) {
	mode := generatorNextMode
	if rejected {
		mode = generatorThrowMode
	}
	result, err := vm.generatorResume(generator, input, mode)
	vm.finishAsyncGenerator(generator, request, result, err)
}

func (vm *VM) finishAsyncGenerator(generator *native.ObjGenerator, request native.AsyncGeneratorRequest, result value.Value, err error) {
	roots := []value.Value{value.EncodeHandle(heap.Allocate(generator))}
	if err != nil {
		vm.rejectPromise(request.Promise, vm.errorValue(err))
		generator.AsyncProcessing = false
		vm.startAsyncGenerator(generator)
		return
	}
	if generator.AsyncAwaiting != nil {
		promise := generator.AsyncAwaiting
		generator.AsyncAwaiting = nil
		roots = append(roots, value.EncodeHandle(heap.Allocate(promise)))
		vm.addPromiseObserver(promise, func(v value.Value) {
			vm.resumeAsyncGenerator(generator, request, v, false)
		}, func(reason value.Value) {
			vm.resumeAsyncGenerator(generator, request, reason, true)
		}, roots...)
		return
	}
	doneValue, _ := vm.getMemberValue(result, value.EncodeHandle(heap.Allocate(native.NewLightString("done"))))
	resultValue, _ := vm.getMemberValue(result, value.EncodeHandle(heap.Allocate(native.NewLightString("value"))))
	if !truthyValue(doneValue) && resultValue.IsObject() {
		if promise, ok := vm.promiseFromValue(resultValue); ok {
			roots = append(roots, resultValue)
			vm.addPromiseObserver(promise, func(v value.Value) {
				vm.resolvePromise(request.Promise, iteratorResult(v, false))
				generator.AsyncProcessing = false
				vm.startAsyncGenerator(generator)
			}, func(reason value.Value) {
				vm.rejectPromise(request.Promise, reason)
				generator.AsyncProcessing = false
				vm.startAsyncGenerator(generator)
			}, roots...)
			return
		}
	}
	vm.resolvePromise(request.Promise, result)
	generator.AsyncProcessing = false
	vm.startAsyncGenerator(generator)
}

func (vm *VM) acquireAsyncIterator(iteratee value.Value) (value.Value, error) {
	if !iteratee.IsObject() {
		return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Value is not iterable"}
	}
	if iterateeObj, err := heap.GetObject(iteratee.GetHandle()); err == nil {
		if generator, ok := iterateeObj.(*native.ObjGenerator); ok && generator.IsAsync {
			return iteratee, nil
		}
	}
	if native.SYMBOL_ASYNC_ITERATOR != 0 {
		method, err := vm.getMemberValue(iteratee, native.SYMBOL_ASYNC_ITERATOR)
		if err == nil && method != value.UNDEFINED {
			iterator, err := vm.callValue(method, iteratee, nil)
			if err != nil {
				return value.UNDEFINED, err
			}
			if !iterator.IsObject() {
				return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Result of the Symbol.asyncIterator method is not an object"}
			}
			if iteratorObj, err := heap.GetObject(iterator.GetHandle()); err == nil {
				if generator, ok := iteratorObj.(*native.ObjGenerator); ok && generator.IsAsync {
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
	return vm.acquireIterator(iteratee, compiler.ITERATOR_FOR_OF)
}

func (vm *VM) asyncIteratorNext(iteratorValue value.Value) (value.Value, error) {
	obj, err := heap.GetObject(iteratorValue.GetHandle())
	if err != nil {
		return value.UNDEFINED, err
	}
	switch iterator := obj.(type) {
	case *native.ObjGenerator:
		if iterator.IsAsync {
			return vm.asyncGeneratorRequest(iterator, value.UNDEFINED, generatorNextMode), nil
		}
		return vm.generatorResume(iterator, value.UNDEFINED, generatorNextMode)
	case *object.Iterator:
		done := iterator.Next()
		return iteratorResult(iterator.Current(), done), nil
	case *native.JSIterator:
		if iterator.Done {
			return iteratorResult(value.UNDEFINED, true), nil
		}
		return vm.callValue(iterator.NextMethod, iterator.Iterator, nil)
	}
	return value.UNDEFINED, &JSException{Name: "TypeError", Message: "not an async iterator"}
}

func (vm *VM) asyncIteratorResult(iteratorValue, result value.Value) (bool, error) {
	if !result.IsObject() {
		return false, &JSException{Name: "TypeError", Message: "Iterator result is not an object"}
	}
	doneValue, err := vm.getMemberValue(result, value.EncodeHandle(heap.Allocate(native.NewLightString("done"))))
	if err != nil {
		return false, err
	}
	done := truthyValue(doneValue)
	if !done {
		resultValue, err := vm.getMemberValue(result, value.EncodeHandle(heap.Allocate(native.NewLightString("value"))))
		if err != nil {
			return false, err
		}
		if obj, err := heap.GetObject(iteratorValue.GetHandle()); err == nil {
			switch iterator := obj.(type) {
			case *native.JSIterator:
				iterator.Current = resultValue
			case *native.ObjGenerator:
				iterator.Current = resultValue
			}
		}
	}
	return done, nil
}

func (vm *VM) asyncIteratorClose(iteratorValue value.Value) (value.Value, error) {
	obj, err := heap.GetObject(iteratorValue.GetHandle())
	if err != nil {
		return value.UNDEFINED, err
	}
	switch iterator := obj.(type) {
	case *native.ObjGenerator:
		if iterator.Done {
			return value.UNDEFINED, nil
		}
		if iterator.IsAsync {
			return vm.asyncGeneratorRequest(iterator, value.UNDEFINED, generatorReturnMode), nil
		}
		return vm.generatorResume(iterator, value.UNDEFINED, generatorReturnMode)
	case *native.JSIterator:
		if iterator.Done {
			return value.UNDEFINED, nil
		}
		returnMethod, err := vm.getMemberValue(iterator.Iterator, value.EncodeHandle(heap.Allocate(native.NewLightString("return"))))
		if err != nil || returnMethod == value.UNDEFINED {
			return value.UNDEFINED, err
		}
		return vm.callValue(returnMethod, iterator.Iterator, nil)
	}
	return value.UNDEFINED, nil
}
