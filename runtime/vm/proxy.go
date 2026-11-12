package vm

import (
	"fmt"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/value"
)

func (vm *VM) asProxy(v value.Value) (*native.ObjProxy, bool) {
	if !v.IsObject() {
		return nil, false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return nil, false
	}
	proxy, ok := obj.(*native.ObjProxy)
	return proxy, ok
}

func (vm *VM) proxyTarget(proxy *native.ObjProxy) (value.Value, error) {
	if proxy.Revoked {
		return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Cannot perform operation on a revoked Proxy"}
	}
	return proxy.Target, nil
}

func (vm *VM) proxyTrap(proxy *native.ObjProxy, name string) (value.Value, bool, error) {
	if proxy.Revoked {
		return value.UNDEFINED, false, &JSException{Name: "TypeError", Message: "Cannot perform operation on a revoked Proxy"}
	}
	if !proxy.Handler.IsObject() {
		return value.UNDEFINED, false, nil
	}
	trap, err := vm.getMemberValue(proxy.Handler, value.EncodeHandle(heap.Allocate(native.NewLightString(name))))
	if err != nil {
		return value.UNDEFINED, false, err
	}
	if !trap.IsObject() {
		return value.UNDEFINED, false, nil
	}
	obj, err := heap.GetObject(trap.GetHandle())
	if err != nil {
		return value.UNDEFINED, false, err
	}
	if _, ok := obj.(object.Callable); !ok {
		return value.UNDEFINED, false, nil
	}
	return trap, true, nil
}

func (vm *VM) callProxyTrap(proxy *native.ObjProxy, name string, args ...value.Value) (value.Value, bool, error) {
	trap, found, err := vm.proxyTrap(proxy, name)
	if err != nil || !found {
		return value.UNDEFINED, found, err
	}
	result, err := vm.callValue(trap, proxy.Handler, args)
	return result, true, err
}

func (vm *VM) targetOwnEntry(target value.Value, key value.Value) (native.ObjectValueEntry, bool) {
	if !target.IsObject() {
		return native.ObjectValueEntry{}, false
	}
	obj, err := heap.GetObject(target.GetHandle())
	if err != nil {
		return native.ObjectValueEntry{}, false
	}
	if hashable, ok := obj.(interface {
		GetOwnEntry(value.Value) (native.ObjectValueEntry, bool)
	}); ok {
		return hashable.GetOwnEntry(key)
	}
	return native.ObjectValueEntry{}, false
}

func (vm *VM) targetHasOwn(target value.Value, key value.Value) bool {
	_, found := vm.targetOwnEntry(target, key)
	return found
}

func (vm *VM) targetExtensible(target value.Value) bool {
	if !target.IsObject() {
		return false
	}
	obj, err := heap.GetObject(target.GetHandle())
	if err != nil {
		return false
	}
	if extensible, ok := obj.(interface{ Extensible() bool }); ok {
		return extensible.Extensible()
	}
	return true
}

func (vm *VM) targetPrototype(target value.Value) value.Value {
	if !target.IsObject() {
		return value.NULL
	}
	obj, err := heap.GetObject(target.GetHandle())
	if err != nil {
		return value.NULL
	}
	if proto, ok := obj.(interface{ PrototypeValue() value.Value }); ok {
		return proto.PrototypeValue()
	}
	return value.NULL
}

func (vm *VM) ownKeysRaw(target value.Value) ([]value.Value, error) {
	if proxy, ok := vm.asProxy(target); ok {
		return vm.proxyOwnKeys(proxy)
	}
	return native.OwnPropertyKeys(target), nil
}

func (vm *VM) keyIdentity(key value.Value) (string, bool) {
	if s, ok := vm.valueAsString(key); ok {
		return "s:" + s, true
	}
	if key.IsObject() {
		obj, err := heap.GetObject(key.GetHandle())
		if err == nil {
			if _, ok := obj.(*native.ObjSymbol); ok {
				return "y:" + native.String(key), true
			}
		}
	}
	return "", false
}

func (vm *VM) proxyGet(proxy *native.ObjProxy, proxyValue, key, receiver value.Value) (value.Value, error) {
	target, err := vm.proxyTarget(proxy)
	if err != nil {
		return value.UNDEFINED, err
	}
	result, found, err := vm.callProxyTrap(proxy, "get", target, key, receiver)
	if err != nil {
		return value.UNDEFINED, err
	}
	if !found {
		return vm.getMemberValueWithReceiver(target, key, receiver)
	}
	if entry, ok := vm.targetOwnEntry(target, key); ok && !entry.Configurable() && !entry.IsAccessor() && !entry.Writable() {
		if !native.ObjectIsSame(entry.Value, result) {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Proxy get trap returned an invalid value"}
		}
	}
	return result, nil
}

func (vm *VM) proxySet(proxy *native.ObjProxy, key, v, receiver value.Value, strict bool) error {
	target, err := vm.proxyTarget(proxy)
	if err != nil {
		return err
	}
	result, found, err := vm.callProxyTrap(proxy, "set", target, key, v, receiver)
	if err != nil {
		return err
	}
	if found {
		if entry, ok := vm.targetOwnEntry(target, key); ok && !entry.Configurable() {
			if entry.IsAccessor() && entry.Set == value.UNDEFINED {
				return &JSException{Name: "TypeError", Message: "Proxy set trap violated an invariant"}
			}
			if !entry.IsAccessor() && !entry.Writable() && !native.ObjectIsSame(entry.Value, v) {
				return &JSException{Name: "TypeError", Message: "Proxy set trap violated an invariant"}
			}
		}
		if truthyValue(result) {
			return nil
		}
		if strict {
			return fmt.Errorf("Cannot assign to read only property")
		}
		return nil
	}
	return vm.setMemberValueWithReceiver(target, key, v, receiver, strict)
}

func (vm *VM) hasProperty(objValue, key value.Value) (bool, error) {
	if proxy, ok := vm.asProxy(objValue); ok {
		target, err := vm.proxyTarget(proxy)
		if err != nil {
			return false, err
		}
		result, found, err := vm.callProxyTrap(proxy, "has", target, key)
		if err != nil {
			return false, err
		}
		if found {
			has := truthyValue(result)
			if !has {
				entry, own := vm.targetOwnEntry(target, key)
				if own && !entry.Configurable() {
					return false, &JSException{Name: "TypeError", Message: "Proxy has trap violated an invariant"}
				}
				if !vm.targetExtensible(target) && vm.targetHasOwn(target, key) {
					return false, &JSException{Name: "TypeError", Message: "Proxy has trap violated an invariant"}
				}
			}
			return has, nil
		}
		return vm.hasProperty(target, key)
	}
	if !objValue.IsObject() {
		return false, nil
	}
	obj, err := heap.GetObject(objValue.GetHandle())
	if err != nil {
		return false, err
	}
	if hashable, ok := obj.(object.Hashable); ok && hashable.HasOwn(key) {
		return true, nil
	}
	if proto, ok := obj.(interface{ PrototypeValue() value.Value }); ok {
		next := proto.PrototypeValue()
		if next.IsObject() {
			return vm.hasProperty(next, key)
		}
	}
	return false, nil
}

func (vm *VM) deletePropertyValue(objValue, key value.Value) (bool, error) {
	if proxy, ok := vm.asProxy(objValue); ok {
		target, err := vm.proxyTarget(proxy)
		if err != nil {
			return false, err
		}
		result, found, err := vm.callProxyTrap(proxy, "deleteProperty", target, key)
		if err != nil {
			return false, err
		}
		if found {
			deleted := truthyValue(result)
			if deleted {
				if entry, own := vm.targetOwnEntry(target, key); own && !entry.Configurable() {
					return false, &JSException{Name: "TypeError", Message: "Proxy deleteProperty trap violated an invariant"}
				}
			}
			return deleted, nil
		}
		return vm.deletePropertyValue(target, key)
	}
	if !objValue.IsObject() {
		return true, nil
	}
	obj, err := heap.GetObject(objValue.GetHandle())
	if err != nil {
		return false, err
	}
	if _, ok := obj.(*native.ObjModuleNamespace); ok {
		return false, nil
	}
	if deletable, ok := obj.(object.Deletable); ok {
		return deletable.Delete(key), nil
	}
	return true, nil
}

func (vm *VM) arrayValues(v value.Value) ([]value.Value, bool) {
	if !v.IsObject() {
		return nil, false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return nil, false
	}
	arr, ok := obj.(*native.ObjArr)
	if !ok {
		return nil, false
	}
	items := []value.Value{}
	for i := 0; i < arr.Length(); i++ {
		if arr.HasIndex(i) {
			items = append(items, arr.GetElementAt(i))
		}
	}
	return items, true
}

func (vm *VM) proxyOwnKeys(proxy *native.ObjProxy) ([]value.Value, error) {
	target, err := vm.proxyTarget(proxy)
	if err != nil {
		return nil, err
	}
	result, found, err := vm.callProxyTrap(proxy, "ownKeys", target)
	if err != nil {
		return nil, err
	}
	if !found {
		return vm.ownKeysRaw(target)
	}
	keys, ok := vm.arrayValues(result)
	if !ok {
		return nil, &JSException{Name: "TypeError", Message: "Proxy ownKeys trap must return an array"}
	}
	seen := map[string]bool{}
	for _, key := range keys {
		identity, ok := vm.keyIdentity(key)
		if !ok {
			return nil, &JSException{Name: "TypeError", Message: "Proxy ownKeys trap returned an invalid key"}
		}
		if seen[identity] {
			return nil, &JSException{Name: "TypeError", Message: "Proxy ownKeys trap returned duplicate keys"}
		}
		seen[identity] = true
	}
	targetKeys, err := vm.ownKeysRaw(target)
	if err != nil {
		return nil, err
	}
	targetSet := map[string]bool{}
	for _, key := range targetKeys {
		identity, _ := vm.keyIdentity(key)
		targetSet[identity] = true
		if entry, own := vm.targetOwnEntry(target, key); own && !entry.Configurable() && !seen[identity] {
			return nil, &JSException{Name: "TypeError", Message: "Proxy ownKeys trap omitted a non-configurable key"}
		}
	}
	if !vm.targetExtensible(target) {
		if len(keys) != len(targetKeys) {
			return nil, &JSException{Name: "TypeError", Message: "Proxy ownKeys trap returned invalid keys"}
		}
		for identity := range seen {
			if !targetSet[identity] {
				return nil, &JSException{Name: "TypeError", Message: "Proxy ownKeys trap returned an extra key"}
			}
		}
	}
	return keys, nil
}

func (vm *VM) proxyGetOwnPropertyDescriptor(proxy *native.ObjProxy, key value.Value) (value.Value, error) {
	target, err := vm.proxyTarget(proxy)
	if err != nil {
		return value.UNDEFINED, err
	}
	result, found, err := vm.callProxyTrap(proxy, "getOwnPropertyDescriptor", target, key)
	if err != nil {
		return value.UNDEFINED, err
	}
	if !found {
		return native.NewObjectGetOwnPropertyDescriptor().Descriptor(target, key), nil
	}
	entry, own := vm.targetOwnEntry(target, key)
	if result == value.UNDEFINED {
		if own && !entry.Configurable() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Proxy getOwnPropertyDescriptor trap violated an invariant"}
		}
		return value.UNDEFINED, nil
	}
	if own && !entry.Configurable() {
		if descObj, ok := native.ObjectAsObj(result); ok {
			if configurable, err := vm.getMemberValue(result, value.EncodeHandle(heap.Allocate(native.NewLightString("configurable")))); err == nil && truthyValue(configurable) {
				return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Proxy getOwnPropertyDescriptor trap violated an invariant"}
			}
			_ = descObj
		}
	}
	return result, nil
}

func (vm *VM) proxyDefineProperty(proxy *native.ObjProxy, key, descriptor value.Value) (bool, error) {
	target, err := vm.proxyTarget(proxy)
	if err != nil {
		return false, err
	}
	result, found, err := vm.callProxyTrap(proxy, "defineProperty", target, key, descriptor)
	if err != nil {
		return false, err
	}
	if found {
		defined := truthyValue(result)
		if defined && !vm.targetExtensible(target) && !vm.targetHasOwn(target, key) {
			return false, &JSException{Name: "TypeError", Message: "Proxy defineProperty trap violated an invariant"}
		}
		return defined, nil
	}
	return native.NewObjectDefineProperty().Define(target, key, descriptor) != value.UNDEFINED, nil
}

func (vm *VM) proxyGetPrototypeOf(proxy *native.ObjProxy) (value.Value, error) {
	target, err := vm.proxyTarget(proxy)
	if err != nil {
		return value.UNDEFINED, err
	}
	result, found, err := vm.callProxyTrap(proxy, "getPrototypeOf", target)
	if err != nil {
		return value.UNDEFINED, err
	}
	if !found {
		return vm.targetPrototype(target), nil
	}
	if !vm.targetExtensible(target) && !native.ObjectIsSame(result, vm.targetPrototype(target)) {
		return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Proxy getPrototypeOf trap violated an invariant"}
	}
	return result, nil
}

func (vm *VM) proxySetPrototypeOf(proxy *native.ObjProxy, proto value.Value) (bool, error) {
	target, err := vm.proxyTarget(proxy)
	if err != nil {
		return false, err
	}
	result, found, err := vm.callProxyTrap(proxy, "setPrototypeOf", target, proto)
	if err != nil {
		return false, err
	}
	if found {
		ok := truthyValue(result)
		if ok && !vm.targetExtensible(target) && !native.ObjectIsSame(proto, vm.targetPrototype(target)) {
			return false, &JSException{Name: "TypeError", Message: "Proxy setPrototypeOf trap violated an invariant"}
		}
		return ok, nil
	}
	if !target.IsObject() {
		return false, nil
	}
	obj, err := heap.GetObject(target.GetHandle())
	if err != nil {
		return false, err
	}
	if setter, ok := obj.(interface{ SetPrototypeValue(value.Value) }); ok {
		setter.SetPrototypeValue(proto)
		return true, nil
	}
	return false, nil
}

func (vm *VM) proxyIsExtensible(proxy *native.ObjProxy) (bool, error) {
	target, err := vm.proxyTarget(proxy)
	if err != nil {
		return false, err
	}
	result, found, err := vm.callProxyTrap(proxy, "isExtensible", target)
	if err != nil {
		return false, err
	}
	if !found {
		return vm.targetExtensible(target), nil
	}
	extensible := truthyValue(result)
	if extensible != vm.targetExtensible(target) {
		return false, &JSException{Name: "TypeError", Message: "Proxy isExtensible trap violated an invariant"}
	}
	return extensible, nil
}

func (vm *VM) proxyPreventExtensions(proxy *native.ObjProxy) (bool, error) {
	target, err := vm.proxyTarget(proxy)
	if err != nil {
		return false, err
	}
	result, found, err := vm.callProxyTrap(proxy, "preventExtensions", target)
	if err != nil {
		return false, err
	}
	if found {
		prevented := truthyValue(result)
		if prevented {
			if vm.targetExtensible(target) {
				return false, &JSException{Name: "TypeError", Message: "Proxy preventExtensions trap violated an invariant"}
			}
			return true, nil
		}
		return false, nil
	}
	return vm.preventExtensionsValue(target)
}

func (vm *VM) preventExtensionsValue(target value.Value) (bool, error) {
	if !target.IsObject() {
		return false, nil
	}
	obj, err := heap.GetObject(target.GetHandle())
	if err != nil {
		return false, err
	}
	if extensible, ok := obj.(interface{ SetExtensible(bool) }); ok {
		extensible.SetExtensible(false)
		return true, nil
	}
	return false, nil
}

func (vm *VM) proxyApply(proxy *native.ObjProxy, this value.Value, args []value.Value) (value.Value, error) {
	target, err := vm.proxyTarget(proxy)
	if err != nil {
		return value.UNDEFINED, err
	}
	argsArray := value.EncodeHandle(heap.Allocate(native.NewArrayFrom(args)))
	result, found, err := vm.callProxyTrap(proxy, "apply", target, this, argsArray)
	if err != nil {
		return value.UNDEFINED, err
	}
	if !found {
		return vm.callValue(target, this, args)
	}
	return result, nil
}

func (vm *VM) proxyConstruct(proxy *native.ObjProxy, args []value.Value, newTarget value.Value) (value.Value, error) {
	target, err := vm.proxyTarget(proxy)
	if err != nil {
		return value.UNDEFINED, err
	}
	argsArray := value.EncodeHandle(heap.Allocate(native.NewArrayFrom(args)))
	result, found, err := vm.callProxyTrap(proxy, "construct", target, argsArray, newTarget)
	if err != nil {
		return value.UNDEFINED, err
	}
	if !found {
		targetObj, err := heap.GetObject(target.GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		callable, ok := targetObj.(object.Callable)
		if !ok {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Value is not a constructor"}
		}
		return vm.construct(callable, args, newTarget)
	}
	if !result.IsObject() {
		return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Proxy construct trap must return an object"}
	}
	return result, nil
}
