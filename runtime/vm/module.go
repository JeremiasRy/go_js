package vm

import (
	"go_js/compiler"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/value"
)

type moduleState struct {
	record     *compiler.ModuleRecord
	namespace  value.Value
	promise    *native.ObjPromise
	evaluating bool
	evaluated  bool
	errored    bool
	errValue   value.Value
}

type moduleThrownError struct {
	value value.Value
}

func (e *moduleThrownError) Error() string {
	return native.String(e.value)
}

var moduleStates = map[string]*moduleState{}

func (vm *VM) moduleNamespaceValue(record *compiler.ModuleRecord) value.Value {
	state, found := moduleStates[record.Path]
	if found && state.namespace != value.UNDEFINED {
		return state.namespace
	}
	if !found {
		state = &moduleState{record: record, namespace: value.UNDEFINED}
		moduleStates[record.Path] = state
	}
	namespace := native.NewModuleNamespace()
	for _, descriptor := range record.ExportDescriptors() {
		if descriptor.IsNamespace {
			dependency, err := compiler.LoadModule(descriptor.NamespacePath, "")
			if err != nil {
				continue
			}
			namespace.AddNamespaceBinding(descriptor.Name, vm.moduleNamespaceValue(dependency))
		} else {
			namespace.AddBinding(descriptor.Name, descriptor.Slot)
		}
	}
	state.namespace = value.EncodeHandle(heap.Allocate(namespace))
	heap.PushToRoots(state.namespace)
	return state.namespace
}

func (vm *VM) ensureModule(record *compiler.ModuleRecord) (value.Value, error) {
	state, found := moduleStates[record.Path]
	if !found {
		state = &moduleState{record: record, namespace: value.UNDEFINED}
		moduleStates[record.Path] = state
	}
	if state.evaluated {
		return state.namespace, nil
	}
	if state.errored {
		return value.UNDEFINED, &moduleThrownError{value: state.errValue}
	}
	if state.evaluating {
		if state.promise != nil {
			return value.EncodeHandle(heap.Allocate(state.promise)), nil
		}
		return state.namespace, nil
	}
	state.evaluating = true
	state.namespace = vm.moduleNamespaceValue(record)
	if record.Async {
		promise := native.NewPromise()
		state.promise = promise
		heap.PushToRoots(value.EncodeHandle(heap.Allocate(promise)))
		asyncFn, ok := record.Main.(*native.ObjAsyncFunction)
		if !ok {
			state.errored = true
			state.evaluating = false
			state.errValue = vm.makeError("SyntaxError", "Invalid async module")
			return value.UNDEFINED, &moduleThrownError{value: state.errValue}
		}
		asyncFn.SetPromise(promise)
		vm.addPromiseObserver(promise, func(value.Value) {
			state.evaluated = true
			state.evaluating = false
		}, func(reason value.Value) {
			state.errored = true
			state.evaluating = false
			state.errValue = reason
		}, state.namespace, record.MainValue)
		_, err := vm.callValue(record.MainValue, value.UNDEFINED, nil)
		if err != nil {
			state.errored = true
			state.evaluating = false
			state.errValue = vm.errorValue(err)
			return value.UNDEFINED, &moduleThrownError{value: state.errValue}
		}
		return value.EncodeHandle(heap.Allocate(promise)), nil
	}
	_, err := vm.callValue(record.MainValue, value.UNDEFINED, nil)
	if err != nil {
		state.errored = true
		state.evaluating = false
		state.errValue = vm.errorValue(err)
		return value.UNDEFINED, &moduleThrownError{value: state.errValue}
	}
	state.evaluated = true
	state.evaluating = false
	return state.namespace, nil
}

func (vm *VM) importModule(specifier, parent string) (value.Value, error) {
	record, err := compiler.LoadModule(specifier, parent)
	if err != nil {
		return value.UNDEFINED, &moduleThrownError{value: vm.makeError("SyntaxError", err.Error())}
	}
	return vm.ensureModule(record)
}

func (vm *VM) dynamicImport(specifier, parent string) value.Value {
	promise := native.NewPromise()
	promiseValue := value.EncodeHandle(heap.Allocate(promise))
	heap.PushToRoots(promiseValue)
	result, err := vm.importModule(specifier, parent)
	if err != nil {
		if thrown, ok := err.(*moduleThrownError); ok {
			vm.rejectPromise(promise, thrown.value)
		} else {
			vm.rejectPromise(promise, vm.errorValue(err))
		}
		return promiseValue
	}
	vm.resolvePromise(promise, result)
	return promiseValue
}

func (vm *VM) moduleNamespaceMember(namespace *native.ObjModuleNamespace, key value.Value) (value.Value, bool) {
	keyString, ok := vm.valueAsString(key)
	if !ok {
		return value.UNDEFINED, false
	}
	if slot, found := namespace.Bindings[keyString]; found {
		if slot >= globalsCount {
			return value.UNDEFINED, true
		}
		return vm.getGlobal(slot), true
	}
	if binding, found := namespace.NamespaceBindings[keyString]; found {
		return binding, true
	}
	return value.UNDEFINED, true
}

func (vm *VM) moduleNamespaceDescriptor(namespace *native.ObjModuleNamespace, key value.Value) value.Value {
	keyString, ok := vm.valueAsString(key)
	if !ok {
		return value.UNDEFINED
	}
	entry, found := namespace.GetOwnEntry(key)
	if !found {
		return value.UNDEFINED
	}
	descriptor := native.NewObjectHash()
	descriptor.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("enumerable"))), boolValue(entry.Enumerable()))
	descriptor.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("configurable"))), boolValue(entry.Configurable()))
	descriptor.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("writable"))), value.TRUE)
	valueMember, _ := vm.moduleNamespaceMember(namespace, key)
	_ = keyString
	descriptor.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("value"))), valueMember)
	return value.EncodeHandle(heap.Allocate(descriptor))
}

var _ = object.OBJ_OBJECT
