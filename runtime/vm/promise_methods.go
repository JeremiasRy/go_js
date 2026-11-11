package vm

import (
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/queue"
	"go_js/value"
)

func (vm *VM) promiseFromValue(v value.Value) (*native.ObjPromise, bool) {
	if !v.IsObject() {
		return nil, false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return nil, false
	}
	p, ok := obj.(*native.ObjPromise)
	return p, ok
}

func (vm *VM) promiseArray(items []value.Value) value.Value {
	return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(items)))
}

func (vm *VM) collectPromiseInput(source value.Value) ([]value.Value, error) {
	if !source.IsObject() {
		return nil, errNotIterable
	}
	iterator, err := vm.acquireIterator(source, 0)
	if err != nil {
		return nil, err
	}
	items := []value.Value{}
	for {
		done, err := vm.iteratorNext(iterator)
		if err != nil {
			return nil, err
		}
		if done {
			break
		}
		item, err := vm.iteratorCurrent(iterator)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

var errNotIterable = &JSException{Name: "TypeError", Message: "Value is not iterable"}

func (vm *VM) callPromiseAll(argCount int) (value.Value, error) {
	args := vm.popN(argCount)
	result := native.NewPromise()
	resultValue := value.EncodeHandle(heap.Allocate(result))
	source := value.UNDEFINED
	if len(args) > 0 {
		source = args[0]
	}
	items, err := vm.collectPromiseInput(source)
	if err != nil {
		vm.rejectPromise(result, vm.errorValue(err))
		return resultValue, nil
	}
	results := make([]value.Value, len(items))
	remaining := len(items)
	if remaining == 0 {
		vm.resolvePromise(result, vm.promiseArray(results))
		return resultValue, nil
	}
	for i, item := range items {
		index := i
		if p, ok := vm.promiseFromValue(item); ok {
			vm.addPromiseObserver(p, func(v value.Value) {
				results[index] = v
				remaining--
				if remaining == 0 {
					vm.resolvePromise(result, vm.promiseArray(results))
				}
			}, func(reason value.Value) {
				vm.rejectPromise(result, reason)
			})
		} else {
			results[index] = item
			remaining--
			if remaining == 0 {
				vm.resolvePromise(result, vm.promiseArray(results))
			}
		}
	}
	return resultValue, nil
}

func (vm *VM) callPromiseAllSettled(argCount int) (value.Value, error) {
	args := vm.popN(argCount)
	result := native.NewPromise()
	resultValue := value.EncodeHandle(heap.Allocate(result))
	source := value.UNDEFINED
	if len(args) > 0 {
		source = args[0]
	}
	items, err := vm.collectPromiseInput(source)
	if err != nil {
		vm.rejectPromise(result, vm.errorValue(err))
		return resultValue, nil
	}
	results := make([]value.Value, len(items))
	remaining := len(items)
	if remaining == 0 {
		vm.resolvePromise(result, vm.promiseArray(results))
		return resultValue, nil
	}
	for i, item := range items {
		index := i
		settle := func(v value.Value, status string) {
			entry := native.NewObjectHash()
			entry.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("status"))), value.EncodeHandle(heap.Allocate(native.NewLightString(status))))
			if status == "fulfilled" {
				entry.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("value"))), v)
			} else {
				entry.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("reason"))), v)
			}
			results[index] = value.EncodeHandle(heap.Allocate(entry))
			remaining--
			if remaining == 0 {
				vm.resolvePromise(result, vm.promiseArray(results))
			}
		}
		if p, ok := vm.promiseFromValue(item); ok {
			vm.addPromiseObserver(p, func(v value.Value) { settle(v, "fulfilled") }, func(reason value.Value) { settle(reason, "rejected") })
		} else {
			settle(item, "fulfilled")
		}
	}
	return resultValue, nil
}

func (vm *VM) callPromiseRace(argCount int) (value.Value, error) {
	args := vm.popN(argCount)
	result := native.NewPromise()
	resultValue := value.EncodeHandle(heap.Allocate(result))
	source := value.UNDEFINED
	if len(args) > 0 {
		source = args[0]
	}
	items, err := vm.collectPromiseInput(source)
	if err != nil {
		vm.rejectPromise(result, vm.errorValue(err))
		return resultValue, nil
	}
	settled := false
	for _, item := range items {
		if p, ok := vm.promiseFromValue(item); ok {
			vm.addPromiseObserver(p, func(v value.Value) {
				if !settled {
					settled = true
					vm.resolvePromise(result, v)
				}
			}, func(reason value.Value) {
				if !settled {
					settled = true
					vm.rejectPromise(result, reason)
				}
			})
		} else {
			item := item
			queue.Enqueue(object.Callback{Native: func() {
				if !settled {
					settled = true
					vm.resolvePromise(result, item)
				}
			}}, queue.MICRO_TASK, false)
		}
	}
	return resultValue, nil
}

func (vm *VM) callPromiseAny(argCount int) (value.Value, error) {
	args := vm.popN(argCount)
	result := native.NewPromise()
	resultValue := value.EncodeHandle(heap.Allocate(result))
	source := value.UNDEFINED
	if len(args) > 0 {
		source = args[0]
	}
	items, err := vm.collectPromiseInput(source)
	if err != nil {
		vm.rejectPromise(result, vm.errorValue(err))
		return resultValue, nil
	}
	reasons := make([]value.Value, len(items))
	remaining := len(items)
	settled := false
	if remaining == 0 {
		vm.rejectPromise(result, vm.makeAggregateError(reasons))
		return resultValue, nil
	}
	for i, item := range items {
		index := i
		if p, ok := vm.promiseFromValue(item); ok {
			vm.addPromiseObserver(p, func(v value.Value) {
				if !settled {
					settled = true
					vm.resolvePromise(result, v)
				}
			}, func(reason value.Value) {
				reasons[index] = reason
				remaining--
				if remaining == 0 && !settled {
					vm.rejectPromise(result, vm.makeAggregateError(reasons))
				}
			})
		} else {
			item := item
			queue.Enqueue(object.Callback{Native: func() {
				if !settled {
					settled = true
					vm.resolvePromise(result, item)
				}
			}}, queue.MICRO_TASK, false)
		}
	}
	return resultValue, nil
}

func (vm *VM) makeAggregateError(reasons []value.Value) value.Value {
	errValue := vm.makeError("AggregateError", "All promises were rejected")
	if errObj, err := heap.GetObject(errValue.GetHandle()); err == nil {
		if obj, ok := errObj.(*native.ObjError); ok {
			obj.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("errors"))), vm.promiseArray(reasons))
		}
	}
	return errValue
}
