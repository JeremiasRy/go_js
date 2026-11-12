package vm

import (
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/value"
)

func (vm *VM) callReflectMethod(fn *native.ReflectMethod, args []value.Value) (value.Value, error) {
	switch fn.Name() {
	case "get":
		if len(args) < 2 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.get called on non-object"}
		}
		receiver := args[0]
		if len(args) > 2 {
			receiver = args[2]
		}
		return vm.getMemberValueWithReceiver(args[0], args[1], receiver)
	case "set":
		if len(args) < 3 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.set called on non-object"}
		}
		receiver := args[0]
		if len(args) > 3 {
			receiver = args[3]
		}
		if err := vm.setMemberValueWithReceiver(args[0], args[1], args[2], receiver, true); err != nil {
			return value.FALSE, nil
		}
		return value.TRUE, nil
	case "has":
		if len(args) < 2 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.has called on non-object"}
		}
		has, err := vm.hasProperty(args[0], args[1])
		if err != nil {
			return value.UNDEFINED, err
		}
		return boolValue(has), nil
	case "deleteProperty":
		if len(args) < 2 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.deleteProperty called on non-object"}
		}
		deleted, err := vm.deletePropertyValue(args[0], args[1])
		if err != nil {
			return value.UNDEFINED, err
		}
		return boolValue(deleted), nil
	case "defineProperty":
		if len(args) < 3 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.defineProperty called on non-object"}
		}
		if proxy, ok := vm.asProxy(args[0]); ok {
			defined, err := vm.proxyDefineProperty(proxy, args[1], args[2])
			if err != nil {
				return value.UNDEFINED, err
			}
			return boolValue(defined), nil
		}
		if !vm.targetExtensible(args[0]) && !vm.targetHasOwn(args[0], args[1]) {
			return value.FALSE, nil
		}
		result := native.NewObjectDefineProperty().Define(args[0], args[1], args[2])
		return boolValue(result != value.UNDEFINED), nil
	case "getOwnPropertyDescriptor":
		if len(args) < 2 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.getOwnPropertyDescriptor called on non-object"}
		}
		if proxy, ok := vm.asProxy(args[0]); ok {
			return vm.proxyGetOwnPropertyDescriptor(proxy, args[1])
		}
		return native.NewObjectGetOwnPropertyDescriptor().Descriptor(args[0], args[1]), nil
	case "ownKeys":
		if len(args) < 1 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.ownKeys called on non-object"}
		}
		keys, err := vm.ownKeysRaw(args[0])
		if err != nil {
			return value.UNDEFINED, err
		}
		return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(keys))), nil
	case "getPrototypeOf":
		if len(args) < 1 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.getPrototypeOf called on non-object"}
		}
		if proxy, ok := vm.asProxy(args[0]); ok {
			return vm.proxyGetPrototypeOf(proxy)
		}
		return vm.targetPrototype(args[0]), nil
	case "setPrototypeOf":
		if len(args) < 2 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.setPrototypeOf called on non-object"}
		}
		if proxy, ok := vm.asProxy(args[0]); ok {
			result, err := vm.proxySetPrototypeOf(proxy, args[1])
			if err != nil {
				return value.UNDEFINED, err
			}
			return boolValue(result), nil
		}
		result := native.NewObjectSetPrototypeOf().SetPrototypeOf(args[0], args[1])
		return boolValue(result != value.UNDEFINED), nil
	case "isExtensible":
		if len(args) < 1 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.isExtensible called on non-object"}
		}
		if proxy, ok := vm.asProxy(args[0]); ok {
			result, err := vm.proxyIsExtensible(proxy)
			if err != nil {
				return value.UNDEFINED, err
			}
			return boolValue(result), nil
		}
		return boolValue(vm.targetExtensible(args[0])), nil
	case "preventExtensions":
		if len(args) < 1 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.preventExtensions called on non-object"}
		}
		if proxy, ok := vm.asProxy(args[0]); ok {
			result, err := vm.proxyPreventExtensions(proxy)
			if err != nil {
				return value.UNDEFINED, err
			}
			return boolValue(result), nil
		}
		result, err := vm.preventExtensionsValue(args[0])
		if err != nil {
			return value.UNDEFINED, err
		}
		return boolValue(result), nil
	case "apply":
		if len(args) < 3 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.apply called on non-object"}
		}
		obj, err := heap.GetObject(args[0].GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		if proxy, ok := obj.(*native.ObjProxy); ok {
			callArgs, ok := vm.arrayValues(args[2])
			if !ok {
				return value.UNDEFINED, &JSException{Name: "TypeError", Message: "CreateListFromArrayLike called on non-object"}
			}
			return vm.proxyApply(proxy, args[1], callArgs)
		}
		if _, ok := obj.(object.Callable); !ok {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.apply target is not callable"}
		}
		callArgs, ok := vm.arrayValues(args[2])
		if !ok {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "CreateListFromArrayLike called on non-object"}
		}
		return vm.callValue(args[0], args[1], callArgs)
	case "construct":
		if len(args) < 2 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.construct called on non-object"}
		}
		obj, err := heap.GetObject(args[0].GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		if fn, ok := obj.(*object.ObjFunction); ok && (fn.IsArrow || fn.IsMethod) {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Value is not a constructor"}
		}
		callable, ok := obj.(object.Callable)
		if !ok {
			if proxy, isProxy := obj.(*native.ObjProxy); !isProxy || !proxy.Callable {
				return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Value is not a constructor"}
			}
		}
		constructArgs, ok := vm.arrayValues(args[1])
		if !ok {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "CreateListFromArrayLike called on non-object"}
		}
		newTarget := args[0]
		if len(args) > 2 {
			newTarget = args[2]
		}
		if !newTarget.IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.construct newTarget is not a constructor"}
		}
		newTargetObj, err := heap.GetObject(newTarget.GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		if fn, ok := newTargetObj.(*object.ObjFunction); ok && (fn.IsArrow || fn.IsMethod) {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.construct newTarget is not a constructor"}
		}
		if _, ok := newTargetObj.(object.Callable); !ok {
			if proxy, isProxy := newTargetObj.(*native.ObjProxy); !isProxy || !proxy.Callable {
				return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Reflect.construct newTarget is not a constructor"}
			}
		}
		if callable == nil {
			if proxy, ok := obj.(*native.ObjProxy); ok {
				return vm.proxyConstruct(proxy, constructArgs, newTarget)
			}
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Value is not a constructor"}
		}
		return vm.construct(callable, constructArgs, newTarget)
	}
	return value.UNDEFINED, nil
}
