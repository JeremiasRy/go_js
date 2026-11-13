package vm

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"go_js/compiler"
	"go_js/eventloop"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/queue"
	"go_js/value"
	"math"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

func (vm *VM) hostBytes(v value.Value) ([]byte, error) {
	if v == value.UNDEFINED || v == value.NULL {
		return nil, nil
	}
	if !v.IsObject() {
		return nil, &JSException{Name: "TypeError", Message: "Value is not an ArrayBuffer or typed array"}
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return nil, err
	}
	switch source := obj.(type) {
	case *native.ObjArrayBuffer:
		return source.Data, nil
	case *native.ObjTypedArray:
		start := source.Offset
		end := start + source.Length*source.ElementSize()
		if end > len(source.Buffer.Data) {
			end = len(source.Buffer.Data)
		}
		return source.Buffer.Data[start:end], nil
	case *native.ObjDataView:
		start := source.Offset
		end := start + source.Length
		if end > len(source.Buffer.Data) {
			end = len(source.Buffer.Data)
		}
		return source.Buffer.Data[start:end], nil
	}
	return nil, &JSException{Name: "TypeError", Message: "Value is not an ArrayBuffer or typed array"}
}

func hostString(v value.Value) string {
	if text, ok := native.ObjectStringValue(v); ok {
		return text
	}
	return native.String(v)
}

func (vm *VM) callHostFunction(fn *native.HostFunction, thisCtx value.Value, argCount int) (value.Value, error) {
	args := vm.popN(argCount)
	switch fn.Kind {
	case native.HOST_TEXT_ENCODER_ENCODE:
		text := ""
		if len(args) > 0 {
			text = hostString(args[0])
		}
		array := native.NewUint8ArrayFromBytes([]byte(text))
		return value.EncodeHandle(heap.Allocate(array)), nil
	case native.HOST_TEXT_ENCODER_ENCODE_INTO:
		if len(args) < 2 {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "encodeInto requires a source and destination"}
		}
		text := hostString(args[0])
		destinationValue := args[1]
		if !destinationValue.IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Destination must be a Uint8Array"}
		}
		destinationObj, err := heap.GetObject(destinationValue.GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		destination, ok := destinationObj.(*native.ObjTypedArray)
		if !ok || destination.Kind != native.TYPED_UINT8 {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Destination must be a Uint8Array"}
		}
		encoded := []byte(text)
		written := min(len(encoded), destination.Length)
		for written > 0 && !utf8.RuneStart(encoded[written]) {
			written--
		}
		copy(destination.Buffer.Data[destination.Offset:destination.Offset+written], encoded[:written])
		read := len(text)
		if written < len(encoded) {
			read = len(string(encoded[:written]))
		}
		result := native.NewObjectHash()
		result.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("read"))), value.ValueFromFloat64(float64(read)))
		result.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("written"))), value.ValueFromFloat64(float64(written)))
		return value.EncodeHandle(heap.Allocate(result)), nil
	case native.HOST_TEXT_DECODER_DECODE:
		var data []byte
		if len(args) > 0 {
			var err error
			data, err = vm.hostBytes(args[0])
			if err != nil {
				return value.UNDEFINED, err
			}
		}
		text := strings.ToValidUTF8(string(data), "\uFFFD")
		if strings.HasPrefix(text, "\uFEFF") {
			ignoreBOM := false
			if thisCtx.IsObject() {
				if bom, err := vm.getMemberValue(thisCtx, value.EncodeHandle(heap.Allocate(native.NewLightString("ignoreBOM")))); err == nil {
					ignoreBOM = truthyValue(bom)
				}
			}
			if !ignoreBOM {
				text = strings.TrimPrefix(text, "\uFEFF")
			}
		}
		return value.EncodeHandle(heap.Allocate(native.NewLightString(text))), nil
	case native.HOST_ATOB:
		if len(args) == 0 {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "atob requires a string"}
		}
		input := strings.Map(func(r rune) rune {
			if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' {
				return -1
			}
			return r
		}, hostString(args[0]))
		decoded, err := base64.StdEncoding.DecodeString(input)
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(input)
		}
		if err != nil {
			return value.UNDEFINED, &JSException{Name: "InvalidCharacterError", Message: "The string to be decoded is not correctly encoded"}
		}
		var builder strings.Builder
		for _, b := range decoded {
			builder.WriteRune(rune(b))
		}
		return value.EncodeHandle(heap.Allocate(native.NewLightString(builder.String()))), nil
	case native.HOST_BTOA:
		if len(args) == 0 {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "btoa requires a string"}
		}
		text := hostString(args[0])
		decoded := make([]byte, 0, len(text))
		for _, r := range text {
			if r > 0xff {
				return value.UNDEFINED, &JSException{Name: "InvalidCharacterError", Message: "The string to be encoded contains characters outside of the Latin1 range"}
			}
			decoded = append(decoded, byte(r))
		}
		return value.EncodeHandle(heap.Allocate(native.NewLightString(base64.StdEncoding.EncodeToString(decoded)))), nil
	case native.HOST_STRUCTURED_CLONE:
		if len(args) == 0 {
			return value.UNDEFINED, nil
		}
		result, err := native.StructuredClone(args[0])
		if err != nil {
			return value.UNDEFINED, &JSException{Name: "DataCloneError", Message: err.Error()}
		}
		return result, nil
	case native.HOST_CRYPTO_GET_RANDOM_VALUES:
		if len(args) == 0 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "crypto.getRandomValues requires an integer typed array"}
		}
		arrayObj, err := heap.GetObject(args[0].GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		array, ok := arrayObj.(*native.ObjTypedArray)
		if !ok || array.Kind == native.TYPED_FLOAT16 || array.Kind == native.TYPED_FLOAT32 || array.Kind == native.TYPED_FLOAT64 || array.Kind == native.TYPED_BIGINT64 || array.Kind == native.TYPED_BIGUINT64 {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "crypto.getRandomValues requires an integer typed array"}
		}
		size := array.Length * array.ElementSize()
		if size > 65536 {
			return value.UNDEFINED, &JSException{Name: "QuotaExceededError", Message: "The requested length exceeds 65,536 bytes"}
		}
		if _, err := rand.Read(array.Buffer.Data[array.Offset : array.Offset+size]); err != nil {
			return value.UNDEFINED, err
		}
		return args[0], nil
	case native.HOST_CRYPTO_RANDOM_UUID:
		var data [16]byte
		if _, err := rand.Read(data[:]); err != nil {
			return value.UNDEFINED, err
		}
		data[6] = (data[6] & 0x0f) | 0x40
		data[8] = (data[8] & 0x3f) | 0x80
		text := hex.EncodeToString(data[:])
		uuid := text[0:8] + "-" + text[8:12] + "-" + text[12:16] + "-" + text[16:20] + "-" + text[20:32]
		return value.EncodeHandle(heap.Allocate(native.NewLightString(uuid))), nil
	case native.HOST_PERFORMANCE_NOW:
		now := time.Now()
		return value.ValueFromFloat64(float64(now.Sub(hostPerformanceOrigin).Nanoseconds()) / 1e6), nil
	case native.HOST_REQUEST_ANIMATION_FRAME:
		if len(args) == 0 {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "requestAnimationFrame requires a callback"}
		}
		timestamp := value.ValueFromFloat64(float64(time.Now().Sub(hostPerformanceOrigin).Nanoseconds()) / 1e6)
		id, err := vm.scheduleTimer(native.NewSetTimeout(), args[0], value.UNDEFINED, 16, []value.Value{timestamp})
		if err != nil {
			return value.UNDEFINED, err
		}
		return value.ValueFromFloat64(float64(id)), nil
	case native.HOST_CANCEL_ANIMATION_FRAME:
		if len(args) > 0 {
			id, _ := vm.toNumber(args[0])
			native.CancelTimer(int(id))
		}
		return value.UNDEFINED, nil
	case native.HOST_SET_IMMEDIATE:
		if len(args) == 0 {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "setImmediate requires a callback"}
		}
		extraArgs := []value.Value{}
		if len(args) > 1 {
			extraArgs = args[1:]
		}
		id, err := vm.scheduleTimer(native.NewSetTimeout(), args[0], value.UNDEFINED, 0, extraArgs)
		if err != nil {
			return value.UNDEFINED, err
		}
		return value.ValueFromFloat64(float64(id)), nil
	case native.HOST_CLEAR_IMMEDIATE:
		if len(args) > 0 {
			id, _ := vm.toNumber(args[0])
			native.CancelTimer(int(id))
		}
		return value.UNDEFINED, nil
	case native.HOST_PROCESS_CWD:
		directory, err := os.Getwd()
		if err != nil {
			return value.UNDEFINED, err
		}
		return value.EncodeHandle(heap.Allocate(native.NewLightString(directory))), nil
	case native.HOST_PROCESS_NEXT_TICK:
		if len(args) == 0 {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "process.nextTick requires a callback"}
		}
		if err := vm.scheduleMicrotask(args[0], args[1:]); err != nil {
			return value.UNDEFINED, err
		}
		return value.UNDEFINED, nil
	case native.HOST_PROCESS_EXIT:
		code := 0
		if len(args) > 0 {
			n, _ := vm.toNumber(args[0])
			code = int(n)
		}
		os.Exit(code)
	case native.HOST_BUFFER_FROM:
		if len(args) == 0 {
			return value.EncodeHandle(heap.Allocate(native.NewBufferFromBytes(nil))), nil
		}
		encoding := "utf8"
		if len(args) > 1 {
			if text, ok := native.ObjectStringValue(args[1]); ok {
				encoding = text
			}
		}
		data, err := native.BufferBytesFromValue(args[0], encoding)
		if err != nil {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: err.Error()}
		}
		return value.EncodeHandle(heap.Allocate(native.NewBufferFromBytes(data))), nil
	case native.HOST_BUFFER_ALLOC:
		size := 0
		if len(args) > 0 {
			n, _ := vm.toNumber(args[0])
			if n < 0 || math.IsNaN(n) || math.IsInf(n, 0) || n > 2147483647 {
				return value.UNDEFINED, &JSException{Name: "RangeError", Message: "Invalid typed array length"}
			}
			size = int(n)
		}
		data := make([]byte, size)
		if len(args) > 1 && args[1] != value.UNDEFINED {
			var fill []byte
			if args[1].IsNumber() {
				fill = []byte{byte(int64(args[1].AsNumber()) & 0xff)}
			} else {
				fill, _ = native.BufferBytesFromValue(args[1], "utf8")
			}
			if len(fill) > 0 {
				for i := range data {
					data[i] = fill[i%len(fill)]
				}
			}
		}
		return value.EncodeHandle(heap.Allocate(native.NewBufferFromBytes(data))), nil
	case native.HOST_BUFFER_IS_BUFFER:
		result := false
		if len(args) > 0 && args[0].IsObject() {
			if obj, err := heap.GetObject(args[0].GetHandle()); err == nil {
				if array, ok := obj.(*native.ObjTypedArray); ok && array.IsBuffer {
					result = true
				}
			}
		}
		return boolValue(result), nil
	case native.HOST_BUFFER_BYTE_LENGTH:
		if len(args) == 0 {
			return value.ValueFromFloat64(0), nil
		}
		encoding := "utf8"
		if len(args) > 1 {
			if text, ok := native.ObjectStringValue(args[1]); ok {
				encoding = text
			}
		}
		data, err := native.BufferBytesFromValue(args[0], encoding)
		if err != nil {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: err.Error()}
		}
		return value.ValueFromFloat64(float64(len(data))), nil
	case native.HOST_BUFFER_TO_STRING:
		if !thisCtx.IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Buffer.toString called on incompatible receiver"}
		}
		obj, err := heap.GetObject(thisCtx.GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		array, ok := obj.(*native.ObjTypedArray)
		if !ok {
			return value.EncodeHandle(heap.Allocate(native.NewLightString(""))), nil
		}
		start := array.Offset
		end := start + array.Length*array.ElementSize()
		data := array.Buffer.Data[start:end]
		encoding := "utf8"
		if len(args) > 0 && args[0] != value.UNDEFINED {
			encoding = hostString(args[0])
		}
		switch strings.ToLower(encoding) {
		case "hex":
			return value.EncodeHandle(heap.Allocate(native.NewLightString(hex.EncodeToString(data)))), nil
		case "base64":
			return value.EncodeHandle(heap.Allocate(native.NewLightString(base64.StdEncoding.EncodeToString(data)))), nil
		case "latin1", "binary":
			var builder strings.Builder
			for _, b := range data {
				builder.WriteRune(rune(b))
			}
			return value.EncodeHandle(heap.Allocate(native.NewLightString(builder.String()))), nil
		default:
			return value.EncodeHandle(heap.Allocate(native.NewLightString(string(data)))), nil
		}
	case native.HOST_BUFFER_CONCAT:
		if len(args) == 0 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Buffer.concat requires an array of Buffers"}
		}
		listObj, err := heap.GetObject(args[0].GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		list, ok := listObj.(*native.ObjArr)
		if !ok {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Buffer.concat requires an array of Buffers"}
		}
		data := []byte{}
		for i := 0; i < list.Length(); i++ {
			item := list.GetElementAt(i)
			if !item.IsObject() {
				continue
			}
			obj, err := heap.GetObject(item.GetHandle())
			if err != nil {
				continue
			}
			array, ok := obj.(*native.ObjTypedArray)
			if !ok {
				continue
			}
			start := array.Offset
			end := start + array.Length*array.ElementSize()
			data = append(data, array.Buffer.Data[start:end]...)
		}
		if len(args) > 1 && args[1] != value.UNDEFINED {
			n, _ := vm.toNumber(args[1])
			total := int(n)
			if total < len(data) {
				data = data[:total]
			} else {
				data = append(data, make([]byte, total-len(data))...)
			}
		}
		return value.EncodeHandle(heap.Allocate(native.NewBufferFromBytes(data))), nil
	case native.HOST_FETCH:
		p := native.NewPromise()
		promiseValue := value.EncodeHandle(heap.Allocate(p))
		if len(args) == 0 {
			vm.rejectPromise(p, vm.makeError("TypeError", "fetch requires a URL"))
			return promiseValue, nil
		}
		urlText := hostString(args[0])
		method := "GET"
		var body []byte
		header := make(http.Header)
		if len(args) > 1 && args[1].IsObject() {
			if methodValue, err := vm.getMemberValue(args[1], value.EncodeHandle(heap.Allocate(native.NewLightString("method")))); err == nil && methodValue != value.UNDEFINED {
				method = hostString(methodValue)
			}
			if bodyValue, err := vm.getMemberValue(args[1], value.EncodeHandle(heap.Allocate(native.NewLightString("body")))); err == nil && bodyValue != value.UNDEFINED && bodyValue != value.NULL {
				body = []byte(hostString(bodyValue))
			}
			if headersValue, err := vm.getMemberValue(args[1], value.EncodeHandle(heap.Allocate(native.NewLightString("headers")))); err == nil && headersValue.IsObject() {
				if base, ok := native.ObjectAsObj(headersValue); ok {
					for _, key := range native.ObjectEnumerableKeys(base) {
						value, err := vm.getMemberValue(headersValue, value.EncodeHandle(heap.Allocate(native.NewLightString(key))))
						if err == nil {
							header.Set(key, hostString(value))
						}
					}
				}
			}
		}
		response, err := native.FetchHTTP(urlText, method, header, body)
		if err != nil {
			vm.rejectPromise(p, vm.makeError("TypeError", err.Error()))
		} else {
			vm.resolvePromise(p, value.EncodeHandle(heap.Allocate(response)))
		}
		return promiseValue, nil
	case native.HOST_RESPONSE_TEXT, native.HOST_RESPONSE_JSON, native.HOST_RESPONSE_ARRAY_BUFFER:
		if !thisCtx.IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Response method called on incompatible receiver"}
		}
		obj, err := heap.GetObject(thisCtx.GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		response, ok := obj.(*native.ObjFetchResponse)
		if !ok {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Response method called on incompatible receiver"}
		}
		response.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("bodyUsed"))), value.TRUE)
		p := native.NewPromise()
		switch fn.Kind {
		case native.HOST_RESPONSE_TEXT:
			vm.resolvePromise(p, value.EncodeHandle(heap.Allocate(native.NewLightString(string(response.Body)))))
		case native.HOST_RESPONSE_JSON:
			parsed, err := native.JSONStringToValue(string(response.Body))
			if err != nil {
				vm.rejectPromise(p, vm.makeError("SyntaxError", err.Error()))
			} else {
				vm.resolvePromise(p, parsed)
			}
		case native.HOST_RESPONSE_ARRAY_BUFFER:
			buffer := native.NewArrayBuffer(len(response.Body))
			copy(buffer.Data, response.Body)
			vm.resolvePromise(p, value.EncodeHandle(heap.Allocate(buffer)))
		}
		return value.EncodeHandle(heap.Allocate(p)), nil
	case native.HOST_RESPONSE_CLONE:
		if !thisCtx.IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Response method called on incompatible receiver"}
		}
		obj, err := heap.GetObject(thisCtx.GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		response, ok := obj.(*native.ObjFetchResponse)
		if !ok {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Response method called on incompatible receiver"}
		}
		clone := native.NewFetchResponse(response.Status, response.StatusText, append([]byte{}, response.Body...), response.Header.Clone(), response.URL)
		return value.EncodeHandle(heap.Allocate(clone)), nil
	case native.HOST_HEADERS_GET, native.HOST_HEADERS_HAS, native.HOST_HEADERS_ENTRIES:
		if !thisCtx.IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Headers method called on incompatible receiver"}
		}
		obj, err := heap.GetObject(thisCtx.GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		headers, ok := obj.(*native.ObjFetchHeaders)
		if !ok {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Headers method called on incompatible receiver"}
		}
		name := ""
		if len(args) > 0 {
			name = hostString(args[0])
		}
		switch fn.Kind {
		case native.HOST_HEADERS_GET:
			return value.EncodeHandle(heap.Allocate(native.NewLightString(headers.Header.Get(name)))), nil
		case native.HOST_HEADERS_HAS:
			return boolValue(headers.Header.Get(name) != ""), nil
		default:
			items := []value.Value{}
			for key, values := range headers.Header {
				for _, entry := range values {
					pair := native.NewArrayFrom([]value.Value{
						value.EncodeHandle(heap.Allocate(native.NewLightString(key))),
						value.EncodeHandle(heap.Allocate(native.NewLightString(entry))),
					})
					items = append(items, value.EncodeHandle(heap.Allocate(pair)))
				}
			}
			return value.EncodeHandle(heap.Allocate(native.NewArrayFrom(items))), nil
		}
	case native.HOST_WEAK_REF_DEREF:
		if !thisCtx.IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "WeakRef.deref called on incompatible receiver"}
		}
		obj, err := heap.GetObject(thisCtx.GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		ref, ok := obj.(*native.ObjWeakRef)
		if !ok {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "WeakRef.deref called on incompatible receiver"}
		}
		if !ref.Target.IsObject() || !heap.IsLiveValue(ref.Target) || heap.ObjectAt(ref.TargetHandle) != ref.TargetObject {
			return value.UNDEFINED, nil
		}
		return ref.Target, nil
	case native.HOST_FINALIZATION_REGISTER:
		if !thisCtx.IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "FinalizationRegistry.register called on incompatible receiver"}
		}
		obj, err := heap.GetObject(thisCtx.GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		registry, ok := obj.(*native.ObjFinalizationRegistry)
		if !ok {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "FinalizationRegistry.register called on incompatible receiver"}
		}
		if len(args) < 2 || !args[0].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "FinalizationRegistry target must be an object"}
		}
		hasToken := len(args) > 2 && args[2] != value.UNDEFINED
		if hasToken && !args[2].IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "FinalizationRegistry unregister token must be an object"}
		}
		token := value.UNDEFINED
		if hasToken {
			token = args[2]
		}
		cell := registry.Register(args[0], args[1], token, hasToken)
		registryValue := thisCtx
		heap.RegisterFinalizer(args[0].GetHandle(), cell.ID, func() {
			if !heap.IsLiveValue(registryValue) {
				return
			}
			fired := registry.TakeCell(cell.ID)
			if fired == nil || !registry.Cleanup.IsObject() {
				return
			}
			cleanupObj, err := heap.GetObject(registry.Cleanup.GetHandle())
			if err != nil {
				return
			}
			callable, ok := cleanupObj.(object.Callable)
			if !ok {
				return
			}
			key := heap.NewCallbackRoot()
			heap.StoreCallbackRoot(key, registry.Cleanup, fired.Held)
			queue.Enqueue(object.Callback{Fn: callable, ThisCtx: value.UNDEFINED, Stack: []value.Value{fired.Held}, RootKey: key, TraceLabel: "FinalizationRegistry cleanup", TraceSource: "gc"}, queue.TASK, false)
		})
		return value.UNDEFINED, nil
	case native.HOST_FINALIZATION_UNREGISTER:
		if !thisCtx.IsObject() || len(args) == 0 {
			return value.FALSE, nil
		}
		obj, err := heap.GetObject(thisCtx.GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		registry, ok := obj.(*native.ObjFinalizationRegistry)
		if !ok {
			return value.FALSE, nil
		}
		return boolValue(registry.Unregister(args[0])), nil
	case native.HOST_FINALIZATION_CLEANUP:
		return value.UNDEFINED, nil
	case native.HOST_ATOMICS_ADD, native.HOST_ATOMICS_SUB, native.HOST_ATOMICS_AND, native.HOST_ATOMICS_OR, native.HOST_ATOMICS_XOR, native.HOST_ATOMICS_EXCHANGE, native.HOST_ATOMICS_COMPARE_EXCHANGE, native.HOST_ATOMICS_LOAD, native.HOST_ATOMICS_STORE:
		if len(args) < 2 {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Atomics operation requires a typed array and index"}
		}
		array, ok := vm.typedArrayFromValue(args[0])
		if !ok {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Atomics operation requires an integer typed array"}
		}
		indexNumber, _ := vm.toNumber(args[1])
		index := int(indexNumber)
		if index < 0 || index >= array.Length {
			return value.UNDEFINED, &JSException{Name: "RangeError", Message: "Invalid atomic access index"}
		}
		oldValue := array.Read(index)
		oldNumber, _ := vm.toNumber(oldValue)
		if fn.Kind == native.HOST_ATOMICS_LOAD {
			return oldValue, nil
		}
		if len(args) < 3 {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Atomics operation requires a value"}
		}
		newNumber, _ := vm.toNumber(args[2])
		switch fn.Kind {
		case native.HOST_ATOMICS_ADD:
			array.Write(index, value.ValueFromFloat64(oldNumber+newNumber))
		case native.HOST_ATOMICS_SUB:
			array.Write(index, value.ValueFromFloat64(oldNumber-newNumber))
		case native.HOST_ATOMICS_AND:
			array.Write(index, value.ValueFromFloat64(float64(uint32(oldNumber)&uint32(newNumber))))
		case native.HOST_ATOMICS_OR:
			array.Write(index, value.ValueFromFloat64(float64(uint32(oldNumber)|uint32(newNumber))))
		case native.HOST_ATOMICS_XOR:
			array.Write(index, value.ValueFromFloat64(float64(uint32(oldNumber)^uint32(newNumber))))
		case native.HOST_ATOMICS_EXCHANGE:
			array.Write(index, value.ValueFromFloat64(newNumber))
		case native.HOST_ATOMICS_COMPARE_EXCHANGE:
			if len(args) < 4 {
				return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Atomics.compareExchange requires expected and replacement values"}
			}
			expected, _ := vm.toNumber(args[2])
			replacement, _ := vm.toNumber(args[3])
			if oldNumber == expected {
				array.Write(index, value.ValueFromFloat64(replacement))
			}
		case native.HOST_ATOMICS_STORE:
			array.Write(index, value.ValueFromFloat64(newNumber))
			return value.ValueFromFloat64(newNumber), nil
		}
		return oldValue, nil
	case native.HOST_ATOMICS_IS_LOCK_FREE:
		size := 0
		if len(args) > 0 {
			n, _ := vm.toNumber(args[0])
			size = int(n)
		}
		return boolValue(size == 1 || size == 2 || size == 4 || size == 8), nil
	case native.HOST_ATOMICS_WAIT:
		if len(args) < 3 {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Atomics.wait requires a typed array, index, and expected value"}
		}
		array, ok := vm.typedArrayFromValue(args[0])
		if !ok {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Atomics.wait requires an integer typed array"}
		}
		indexNumber, _ := vm.toNumber(args[1])
		index := int(indexNumber)
		if index < 0 || index >= array.Length {
			return value.UNDEFINED, &JSException{Name: "RangeError", Message: "Invalid atomic access index"}
		}
		expected, _ := vm.toNumber(args[2])
		actual, _ := vm.toNumber(array.Read(index))
		if actual != expected {
			return value.EncodeHandle(heap.Allocate(native.NewLightString("not-equal"))), nil
		}
		return value.EncodeHandle(heap.Allocate(native.NewLightString("timed-out"))), nil
	case native.HOST_ATOMICS_NOTIFY:
		return value.ValueFromFloat64(0), nil
	case native.HOST_WEBASSEMBLY_VALIDATE:
		if len(args) == 0 {
			return value.FALSE, nil
		}
		data, err := vm.hostBytes(args[0])
		if err != nil {
			return value.FALSE, nil
		}
		return boolValue(native.ValidateWasm(data)), nil
	case native.HOST_WEBASSEMBLY_COMPILE, native.HOST_WEBASSEMBLY_INSTANTIATE:
		p := native.NewPromise()
		promiseValue := value.EncodeHandle(heap.Allocate(p))
		if len(args) == 0 {
			vm.rejectPromise(p, vm.makeError("TypeError", "WebAssembly.Module requires a BufferSource"))
			return promiseValue, nil
		}
		var data []byte
		module := (*native.WasmModule)(nil)
		if args[0].IsObject() {
			if obj, err := heap.GetObject(args[0].GetHandle()); err == nil {
				if existing, ok := obj.(*native.WasmModule); ok {
					module = existing
				}
			}
		}
		if module == nil {
			var err error
			data, err = vm.hostBytes(args[0])
			if err != nil {
				vm.rejectPromise(p, vm.makeError("TypeError", err.Error()))
				return promiseValue, nil
			}
			if !native.ValidateWasm(data) {
				vm.rejectPromise(p, vm.makeError("CompileError", "WebAssembly.Module(): invalid module"))
				return promiseValue, nil
			}
			module = native.NewWasmModule(data)
		}
		moduleValue := value.EncodeHandle(heap.Allocate(module))
		if fn.Kind == native.HOST_WEBASSEMBLY_COMPILE {
			vm.resolvePromise(p, moduleValue)
			return promiseValue, nil
		}
		instance := native.NewWasmInstance(module)
		instanceValue := value.EncodeHandle(heap.Allocate(instance))
		result := native.NewObjectHash()
		result.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("module"))), moduleValue)
		result.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("instance"))), instanceValue)
		vm.resolvePromise(p, value.EncodeHandle(heap.Allocate(result)))
		return promiseValue, nil
	case native.HOST_TEMPORAL_NOW:
		instant := native.NewTemporalInstant(time.Now().UnixNano())
		return value.EncodeHandle(heap.Allocate(instant)), nil
	case native.HOST_TEMPORAL_INSTANT_FROM:
		if len(args) == 0 {
			return value.UNDEFINED, &JSException{Name: "RangeError", Message: "Invalid Temporal.Instant"}
		}
		nanos, err := native.ParseTemporalInstant(hostString(args[0]))
		if err != nil {
			return value.UNDEFINED, &JSException{Name: "RangeError", Message: err.Error()}
		}
		instant := native.NewTemporalInstant(nanos)
		return value.EncodeHandle(heap.Allocate(instant)), nil
	case native.HOST_TEMPORAL_INSTANT_FROM_EPOCH_MILLISECONDS:
		milliseconds := 0.0
		if len(args) > 0 {
			milliseconds, _ = vm.toNumber(args[0])
		}
		instant := native.NewTemporalInstant(int64(milliseconds * 1e6))
		return value.EncodeHandle(heap.Allocate(instant)), nil
	case native.HOST_TEMPORAL_INSTANT_TO_STRING:
		if !thisCtx.IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Invalid Temporal.Instant receiver"}
		}
		obj, err := heap.GetObject(thisCtx.GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		instant, ok := obj.(*native.ObjTemporalInstant)
		if !ok {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Invalid Temporal.Instant receiver"}
		}
		text := time.Unix(0, instant.Nanoseconds).UTC().Format(time.RFC3339Nano)
		return value.EncodeHandle(heap.Allocate(native.NewLightString(text))), nil
	case native.HOST_TEMPORAL_INSTANT_EPOCH_MILLISECONDS, native.HOST_TEMPORAL_INSTANT_EPOCH_NANOSECONDS:
		if !thisCtx.IsObject() {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Invalid Temporal.Instant receiver"}
		}
		obj, err := heap.GetObject(thisCtx.GetHandle())
		if err != nil {
			return value.UNDEFINED, err
		}
		instant, ok := obj.(*native.ObjTemporalInstant)
		if !ok {
			return value.UNDEFINED, &JSException{Name: "TypeError", Message: "Invalid Temporal.Instant receiver"}
		}
		if fn.Kind == native.HOST_TEMPORAL_INSTANT_EPOCH_NANOSECONDS {
			return newBigIntValue(big.NewInt(instant.Nanoseconds)), nil
		}
		return value.ValueFromFloat64(float64(instant.Nanoseconds) / 1e6), nil
	}
	return value.UNDEFINED, fmt.Errorf("unsupported host function %d", fn.Kind)
}

var hostPerformanceOrigin = time.Now()

var _ = math.NaN

func (vm *VM) scheduleTimer(fn *native.SetTimeout, callback, thisCtx value.Value, delay int, args []value.Value) (int, error) {
	if !callback.IsObject() {
		return 0, &JSException{Name: "TypeError", Message: "The callback argument must be of type function"}
	}
	obj, err := heap.GetObject(callback.GetHandle())
	if err != nil {
		return 0, &JSException{Name: "TypeError", Message: "The callback argument must be of type function"}
	}
	callbackFn, ok := obj.(object.Callable)
	if !ok {
		return 0, &JSException{Name: "TypeError", Message: "The callback argument must be of type function"}
	}
	fn.Set(delay, callbackFn, thisCtx, args)
	name := fn.Name()
	fn.SetTraceMetadata(fmt.Sprintf("%s(%dms) callback", name, delay), name)
	clone := fn.Clone()
	key := heap.NewCallbackRoot()
	clone.SetRootKey(key)
	roots := append([]value.Value{callback, thisCtx}, args...)
	heap.StoreCallbackRoot(key, roots...)
	id := native.RegisterTimer(clone)
	eventloop.Dispatch(clone)
	return id, nil
}

func (vm *VM) scheduleMicrotask(callback value.Value, args []value.Value) error {
	if !callback.IsObject() {
		return &JSException{Name: "TypeError", Message: "Callback must be a function"}
	}
	obj, err := heap.GetObject(callback.GetHandle())
	if err != nil {
		return err
	}
	callable, ok := obj.(object.Callable)
	if !ok {
		return &JSException{Name: "TypeError", Message: "Callback must be a function"}
	}
	key := heap.NewCallbackRoot()
	roots := append([]value.Value{callback}, args...)
	heap.StoreCallbackRoot(key, roots...)
	queue.Enqueue(object.Callback{Fn: callable, ThisCtx: value.UNDEFINED, Stack: args, RootKey: key, MicroTask: true, TraceLabel: "process.nextTick", TraceSource: "process.nextTick"}, queue.MICRO_TASK, false)
	return nil
}

var requireCache = map[string]value.Value{}

func requireCandidate(base, request string) string {
	if filepath.IsAbs(request) {
		return filepath.Clean(request)
	}
	return filepath.Clean(filepath.Join(base, request))
}

func resolveRequireFile(path string) string {
	candidates := []string{path, path + ".js", path + ".json"}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		packagePath := filepath.Join(path, "package.json")
		if data, err := os.ReadFile(packagePath); err == nil {
			if parsed, err := native.JSONStringToValue(string(data)); err == nil && parsed.IsObject() {
				if obj, err := heap.GetObject(parsed.GetHandle()); err == nil {
					if hashable, ok := obj.(object.Hashable); ok {
						mainValue := hashable.GetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("main"))))
						if main, ok := native.ObjectStringValue(mainValue); ok {
							if resolved := resolveRequireFile(filepath.Join(path, main)); resolved != "" {
								return resolved
							}
						}
					}
				}
			}
		}
		for _, candidate := range []string{filepath.Join(path, "index.js"), filepath.Join(path, "index.json")} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	return ""
}

func (vm *VM) requireModule(baseDir, request string) (value.Value, error) {
	path := resolveRequireFile(requireCandidate(baseDir, request))
	if path == "" {
		return value.UNDEFINED, &JSException{Name: "Error", Message: "Cannot find module '" + request + "'"}
	}
	if cached, found := requireCache[path]; found {
		return cached, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return value.UNDEFINED, err
	}
	if filepath.Ext(path) == ".json" {
		parsed, err := native.JSONStringToValue(string(data))
		if err != nil {
			return value.UNDEFINED, &JSException{Name: "SyntaxError", Message: err.Error()}
		}
		requireCache[path] = parsed
		if parsed.IsObject() {
			heap.PushToRoots(parsed)
		}
		return parsed, nil
	}
	exports := native.NewObjectHash()
	exportsValue := value.EncodeHandle(heap.Allocate(exports))
	requireCache[path] = exportsValue
	heap.PushToRoots(exportsValue)
	module := native.NewObjectHash()
	moduleValue := value.EncodeHandle(heap.Allocate(module))
	module.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("exports"))), exportsValue)
	module.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("filename"))), value.EncodeHandle(heap.Allocate(native.NewLightString(path))))
	module.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("id"))), value.EncodeHandle(heap.Allocate(native.NewLightString(path))))
	module.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("loaded"))), value.FALSE)
	requireFn := native.NewRequireFunction(filepath.Dir(path))
	requireValue := value.EncodeHandle(heap.Allocate(requireFn))
	filenameValue := value.EncodeHandle(heap.Allocate(native.NewLightString(path)))
	dirnameValue := value.EncodeHandle(heap.Allocate(native.NewLightString(filepath.Dir(path))))
	fn, err := compiler.CompileDynamicFunction([]string{"exports", "require", "module", "__filename", "__dirname"}, string(data))
	if err != nil {
		delete(requireCache, path)
		return value.UNDEFINED, &JSException{Name: "SyntaxError", Message: err.Error()}
	}
	if _, err := vm.callValue(heap.EncodeObject(fn), value.UNDEFINED, []value.Value{exportsValue, requireValue, moduleValue, filenameValue, dirnameValue}); err != nil {
		delete(requireCache, path)
		return value.UNDEFINED, err
	}
	finalExports := module.GetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("exports"))))
	if finalExports == value.UNDEFINED {
		finalExports = exportsValue
	}
	module.SetMember(value.EncodeHandle(heap.Allocate(native.NewLightString("loaded"))), value.TRUE)
	requireCache[path] = finalExports
	if finalExports.IsObject() {
		heap.PushToRoots(finalExports)
	}
	return finalExports, nil
}
