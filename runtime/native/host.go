package native

import (
	"fmt"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"os"
	"runtime"
	"strings"
	"time"
)

type HostFunctionKind int

const (
	HOST_TEXT_ENCODER_ENCODE HostFunctionKind = iota
	HOST_TEXT_ENCODER_ENCODE_INTO
	HOST_TEXT_DECODER_DECODE
	HOST_ATOB
	HOST_BTOA
	HOST_STRUCTURED_CLONE
	HOST_CRYPTO_GET_RANDOM_VALUES
	HOST_CRYPTO_RANDOM_UUID
	HOST_PERFORMANCE_NOW
	HOST_FETCH
	HOST_RESPONSE_TEXT
	HOST_RESPONSE_JSON
	HOST_RESPONSE_ARRAY_BUFFER
	HOST_RESPONSE_CLONE
	HOST_HEADERS_GET
	HOST_HEADERS_HAS
	HOST_HEADERS_ENTRIES
	HOST_REQUEST_ANIMATION_FRAME
	HOST_CANCEL_ANIMATION_FRAME
	HOST_SET_IMMEDIATE
	HOST_CLEAR_IMMEDIATE
	HOST_PROCESS_CWD
	HOST_PROCESS_NEXT_TICK
	HOST_PROCESS_EXIT
	HOST_BUFFER_FROM
	HOST_BUFFER_ALLOC
	HOST_BUFFER_IS_BUFFER
	HOST_BUFFER_BYTE_LENGTH
	HOST_BUFFER_CONCAT
	HOST_BUFFER_TO_STRING
	HOST_WEAK_REF_DEREF
	HOST_FINALIZATION_REGISTER
	HOST_FINALIZATION_UNREGISTER
	HOST_FINALIZATION_CLEANUP
	HOST_ATOMICS_ADD
	HOST_ATOMICS_SUB
	HOST_ATOMICS_AND
	HOST_ATOMICS_OR
	HOST_ATOMICS_XOR
	HOST_ATOMICS_EXCHANGE
	HOST_ATOMICS_COMPARE_EXCHANGE
	HOST_ATOMICS_LOAD
	HOST_ATOMICS_STORE
	HOST_ATOMICS_IS_LOCK_FREE
	HOST_ATOMICS_WAIT
	HOST_ATOMICS_NOTIFY
	HOST_WEBASSEMBLY_VALIDATE
	HOST_WEBASSEMBLY_COMPILE
	HOST_WEBASSEMBLY_INSTANTIATE
	HOST_TEMPORAL_NOW
	HOST_TEMPORAL_INSTANT_FROM
	HOST_TEMPORAL_INSTANT_FROM_EPOCH_MILLISECONDS
	HOST_TEMPORAL_INSTANT_TO_STRING
	HOST_TEMPORAL_INSTANT_EPOCH_MILLISECONDS
	HOST_TEMPORAL_INSTANT_EPOCH_NANOSECONDS
)

type HostFunction struct {
	ObjNativeFn
	Kind HostFunctionKind
}

func NewHostFunction(name string, kind HostFunctionKind) *HostFunction {
	fn := &HostFunction{Kind: kind}
	fn.name = name
	return fn
}

func hostFunction(name string, kind HostFunctionKind) value.Value {
	return value.EncodeHandle(heap.Allocate(NewHostFunction(name, kind)))
}

func hostTruthy(v value.Value) bool {
	if v.IsObject() {
		return true
	}
	if v.IsType(value.NULL) || v.IsType(value.UNDEFINED) || v.IsType(value.UNINITIALIZED) {
		return false
	}
	if v.IsNumber() {
		return v.AsNumber() != 0
	}
	return v == value.TRUE
}

func newHostObject() *ObjObject {
	obj := NewObjectHash()
	obj.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return obj
}

type TextEncoderConstructor struct {
	ObjObject
}

func NewTextEncoderConstructor() *TextEncoderConstructor {
	ctor := &TextEncoderConstructor{}
	ctor.Members = map[string]ObjectValueEntry{}
	ctor.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return ctor
}

func (*TextEncoderConstructor) Type() object.ObjType { return object.OBJ_NATIVE_FN }
func (*TextEncoderConstructor) String() string       { return "function TextEncoder" }

func (*TextEncoderConstructor) New(params ...any) (object.Object, error) {
	encoder := newHostObject()
	encoder.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("encoding"))), value.EncodeHandle(heap.Allocate(NewLightString("utf-8"))))
	encoder.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("encode"))), hostFunction("encode", HOST_TEXT_ENCODER_ENCODE))
	encoder.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("encodeInto"))), hostFunction("encodeInto", HOST_TEXT_ENCODER_ENCODE_INTO))
	return encoder, nil
}

type TextDecoderConstructor struct {
	ObjObject
}

func NewTextDecoderConstructor() *TextDecoderConstructor {
	ctor := &TextDecoderConstructor{}
	ctor.Members = map[string]ObjectValueEntry{}
	ctor.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return ctor
}

func (*TextDecoderConstructor) Type() object.ObjType { return object.OBJ_NATIVE_FN }
func (*TextDecoderConstructor) String() string       { return "function TextDecoder" }

func (*TextDecoderConstructor) New(params ...any) (object.Object, error) {
	encoding := "utf-8"
	fatal := false
	ignoreBOM := false
	if len(params) > 0 {
		if v, ok := params[0].(value.Value); ok && v != value.UNDEFINED {
			if text, ok := ObjectStringValue(v); ok {
				encoding = text
			}
		}
	}
	if len(params) > 1 {
		if options, ok := params[1].(value.Value); ok && options.IsObject() {
			if obj, err := heap.GetObject(options.GetHandle()); err == nil {
				if hashable, ok := obj.(object.Hashable); ok {
					fatalValue := hashable.GetMember(value.EncodeHandle(heap.Allocate(NewLightString("fatal"))))
					fatal = hostTruthy(fatalValue)
					ignoreBOMValue := hashable.GetMember(value.EncodeHandle(heap.Allocate(NewLightString("ignoreBOM"))))
					ignoreBOM = hostTruthy(ignoreBOMValue)
				}
			}
		}
	}
	decoder := newHostObject()
	decoder.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("encoding"))), value.EncodeHandle(heap.Allocate(NewLightString(encoding))))
	decoder.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("fatal"))), boolValueValue(fatal))
	decoder.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("ignoreBOM"))), boolValueValue(ignoreBOM))
	decoder.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("decode"))), hostFunction("decode", HOST_TEXT_DECODER_DECODE))
	return decoder, nil
}

func NewCryptoObject() *ObjObject {
	crypto := newHostObject()
	crypto.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("getRandomValues"))), hostFunction("getRandomValues", HOST_CRYPTO_GET_RANDOM_VALUES))
	crypto.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("randomUUID"))), hostFunction("randomUUID", HOST_CRYPTO_RANDOM_UUID))
	return crypto
}

var performanceStart = time.Now()

func NewPerformanceObject() *ObjObject {
	performance := newHostObject()
	performance.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("now"))), hostFunction("now", HOST_PERFORMANCE_NOW))
	performance.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("timeOrigin"))), value.ValueFromFloat64(float64(performanceStart.UnixNano())/1e6))
	return performance
}

func NewFetchFunction() *HostFunction {
	return NewHostFunction("fetch", HOST_FETCH)
}

func NewRequestAnimationFrameFunction() *HostFunction {
	return NewHostFunction("requestAnimationFrame", HOST_REQUEST_ANIMATION_FRAME)
}

func NewCancelAnimationFrameFunction() *HostFunction {
	return NewHostFunction("cancelAnimationFrame", HOST_CANCEL_ANIMATION_FRAME)
}

func NewSetImmediateFunction() *HostFunction {
	return NewHostFunction("setImmediate", HOST_SET_IMMEDIATE)
}

func NewClearImmediateFunction() *HostFunction {
	return NewHostFunction("clearImmediate", HOST_CLEAR_IMMEDIATE)
}

func NewAtobFunction() *HostFunction {
	return NewHostFunction("atob", HOST_ATOB)
}

func NewBtoaFunction() *HostFunction {
	return NewHostFunction("btoa", HOST_BTOA)
}

func NewStructuredCloneFunction() *HostFunction {
	return NewHostFunction("structuredClone", HOST_STRUCTURED_CLONE)
}

func NewProcessObject() *ObjObject {
	process := newHostObject()
	env := NewObjectHash()
	for _, entry := range os.Environ() {
		key, valueText, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		env.SetMember(value.EncodeHandle(heap.Allocate(NewLightString(key))), value.EncodeHandle(heap.Allocate(NewLightString(valueText))))
	}
	process.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("env"))), value.EncodeHandle(heap.Allocate(env)))
	argv := make([]value.Value, len(os.Args))
	for i, arg := range os.Args {
		argv[i] = value.EncodeHandle(heap.Allocate(NewLightString(arg)))
	}
	process.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("argv"))), value.EncodeHandle(heap.Allocate(NewArrayFrom(argv))))
	process.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("execPath"))), value.EncodeHandle(heap.Allocate(NewLightString(os.Args[0]))))
	process.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("platform"))), value.EncodeHandle(heap.Allocate(NewLightString(runtime.GOOS))))
	process.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("arch"))), value.EncodeHandle(heap.Allocate(NewLightString(runtime.GOARCH))))
	process.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("pid"))), value.ValueFromFloat64(float64(os.Getpid())))
	process.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("cwd"))), hostFunction("cwd", HOST_PROCESS_CWD))
	process.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("nextTick"))), hostFunction("nextTick", HOST_PROCESS_NEXT_TICK))
	process.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("exit"))), hostFunction("exit", HOST_PROCESS_EXIT))
	versions := NewObjectHash()
	versions.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("go"))), value.EncodeHandle(heap.Allocate(NewLightString(runtime.Version()))))
	process.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("versions"))), value.EncodeHandle(heap.Allocate(versions)))
	return process
}

type RequireFunction struct {
	ObjNativeFn
	BaseDir string
}

func NewRequireFunction(baseDir string) *RequireFunction {
	fn := &RequireFunction{BaseDir: baseDir}
	fn.name = "require"
	return fn
}

type SharedArrayBufferConstructor struct {
	ObjObject
}

func NewSharedArrayBufferConstructor() *SharedArrayBufferConstructor {
	ctor := &SharedArrayBufferConstructor{}
	ctor.Members = map[string]ObjectValueEntry{}
	ctor.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return ctor
}

func (*SharedArrayBufferConstructor) Type() object.ObjType { return object.OBJ_NATIVE_FN }
func (*SharedArrayBufferConstructor) String() string       { return "function SharedArrayBuffer" }

func (*SharedArrayBufferConstructor) New(params ...any) (object.Object, error) {
	length := 0
	if len(params) > 0 {
		if v, ok := params[0].(value.Value); ok && v.IsNumber() {
			n := v.AsNumber()
			if n < 0 || n != float64(int64(n)) {
				return nil, fmt.Errorf("Invalid SharedArrayBuffer length")
			}
			length = int(n)
		}
	}
	return NewSharedArrayBuffer(length), nil
}

func NewAtomicsObject() *ObjObject {
	atomics := newHostObject()
	for _, item := range []struct {
		name string
		kind HostFunctionKind
	}{
		{"add", HOST_ATOMICS_ADD},
		{"sub", HOST_ATOMICS_SUB},
		{"and", HOST_ATOMICS_AND},
		{"or", HOST_ATOMICS_OR},
		{"xor", HOST_ATOMICS_XOR},
		{"exchange", HOST_ATOMICS_EXCHANGE},
		{"compareExchange", HOST_ATOMICS_COMPARE_EXCHANGE},
		{"load", HOST_ATOMICS_LOAD},
		{"store", HOST_ATOMICS_STORE},
		{"isLockFree", HOST_ATOMICS_IS_LOCK_FREE},
		{"wait", HOST_ATOMICS_WAIT},
		{"notify", HOST_ATOMICS_NOTIFY},
	} {
		atomics.SetMember(value.EncodeHandle(heap.Allocate(NewLightString(item.name))), hostFunction(item.name, item.kind))
	}
	return atomics
}
