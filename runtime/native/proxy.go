package native

import (
	"fmt"

	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

type ObjProxy struct {
	ObjObject
	Target   value.Value
	Handler  value.Value
	Revoked  bool
	Callable bool
}

func NewObjProxy(target, handler value.Value) *ObjProxy {
	proxy := &ObjProxy{Target: target, Handler: handler}
	proxy.Members = map[string]ObjectValueEntry{}
	if target.IsObject() {
		if obj, err := heap.GetObject(target.GetHandle()); err == nil {
			if _, ok := obj.(object.Callable); ok {
				proxy.Callable = true
			}
			if targetProxy, ok := obj.(*ObjProxy); ok && targetProxy.Callable {
				proxy.Callable = true
			}
			switch obj.(type) {
			case *ObjClass, *ProxyConstructor, *RegExpConstructor, *ObjectConstructor, *ArrayConstructor:
				proxy.Callable = true
			}
		}
	}
	return proxy
}

func (*ObjProxy) Type() object.ObjType {
	return object.OBJ_PROXY
}

func (p *ObjProxy) String() string {
	return "[object Proxy]"
}

func (p *ObjProxy) GetReferencingValues() []value.Value {
	arr := p.ObjObject.GetReferencingValues()
	if p.Target.IsObject() {
		arr = append(arr, p.Target)
	}
	if p.Handler.IsObject() {
		arr = append(arr, p.Handler)
	}
	return arr
}

type ProxyConstructor struct {
	ObjObject
}

func NewProxyConstructor() *ProxyConstructor {
	ctor := &ProxyConstructor{}
	ctor.Members = map[string]ObjectValueEntry{}
	ctor.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	ctor.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("revocable"))), value.EncodeHandle(heap.Allocate(NewProxyRevocable())))
	return ctor
}

func (*ProxyConstructor) Type() object.ObjType {
	return object.OBJ_NATIVE_FN
}

func (*ProxyConstructor) String() string {
	return "function Proxy"
}

type ProxyRevocable struct {
	ObjNativeFn
}

func NewProxyRevocable() *ProxyRevocable {
	fn := &ProxyRevocable{}
	fn.name = "revocable"
	fn.Arity = 2
	return fn
}

type ProxyRevoke struct {
	ObjNativeFn
	Proxy *ObjProxy
}

func NewProxyRevoke(proxy *ObjProxy) *ProxyRevoke {
	fn := &ProxyRevoke{Proxy: proxy}
	fn.name = "revoke"
	return fn
}

func (*ProxyRevoke) String() string {
	return fmt.Sprintf("<native fn revoke()>")
}
