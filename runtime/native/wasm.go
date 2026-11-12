package native

import (
	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

type WasmModule struct {
	ObjObject
	Bytes []byte
}

func NewWasmModule(data []byte) *WasmModule {
	module := &WasmModule{Bytes: append([]byte{}, data...)}
	module.Members = map[string]ObjectValueEntry{}
	module.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return module
}

func (*WasmModule) Type() object.ObjType { return object.OBJ_OBJECT }
func (*WasmModule) String() string       { return "WebAssembly.Module" }

type WasmInstance struct {
	ObjObject
	Module *WasmModule
}

func NewWasmInstance(module *WasmModule) *WasmInstance {
	instance := &WasmInstance{Module: module}
	instance.Members = map[string]ObjectValueEntry{}
	instance.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	instance.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("exports"))), value.EncodeHandle(heap.Allocate(NewObjectHash())))
	return instance
}

func (*WasmInstance) Type() object.ObjType { return object.OBJ_OBJECT }
func (*WasmInstance) String() string       { return "WebAssembly.Instance" }

func NewWebAssemblyObject() *ObjObject {
	wasm := newHostObject()
	wasm.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("validate"))), hostFunction("validate", HOST_WEBASSEMBLY_VALIDATE))
	wasm.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("compile"))), hostFunction("compile", HOST_WEBASSEMBLY_COMPILE))
	wasm.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("instantiate"))), hostFunction("instantiate", HOST_WEBASSEMBLY_INSTANTIATE))
	return wasm
}

func ValidateWasm(data []byte) bool {
	return len(data) >= 8 && string(data[:4]) == "\x00asm" && data[4] == 1 && data[5] == 0 && data[6] == 0 && data[7] == 0
}
