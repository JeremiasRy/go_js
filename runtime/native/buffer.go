package native

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"strings"
)

type BufferConstructor struct {
	ObjObject
}

func NewBufferConstructor() *BufferConstructor {
	ctor := &BufferConstructor{}
	ctor.Members = map[string]ObjectValueEntry{}
	ctor.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	for _, item := range []struct {
		name string
		kind HostFunctionKind
	}{
		{"from", HOST_BUFFER_FROM},
		{"alloc", HOST_BUFFER_ALLOC},
		{"allocUnsafe", HOST_BUFFER_ALLOC},
		{"isBuffer", HOST_BUFFER_IS_BUFFER},
		{"byteLength", HOST_BUFFER_BYTE_LENGTH},
		{"concat", HOST_BUFFER_CONCAT},
	} {
		ctor.SetMember(value.EncodeHandle(heap.Allocate(NewLightString(item.name))), hostFunction(item.name, item.kind))
	}
	return ctor
}

func (*BufferConstructor) Type() object.ObjType { return object.OBJ_NATIVE_FN }
func (*BufferConstructor) String() string       { return "function Buffer" }

func NewBufferFromBytes(data []byte) *ObjTypedArray {
	buffer := NewUint8ArrayFromBytes(data)
	buffer.IsBuffer = true
	return buffer
}

func decodeBufferEncoding(text, encoding string) ([]byte, error) {
	switch strings.ToLower(encoding) {
	case "", "utf8", "utf-8":
		return []byte(text), nil
	case "hex":
		if len(text)%2 != 0 {
			text = "0" + text
		}
		return hex.DecodeString(text)
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(text)
		}
		return decoded, err
	case "latin1", "binary":
		result := make([]byte, 0, len(text))
		for _, r := range text {
			if r > 0xff {
				return nil, fmt.Errorf("Invalid character in Latin1 string")
			}
			result = append(result, byte(r))
		}
		return result, nil
	case "ascii":
		result := make([]byte, 0, len(text))
		for _, r := range text {
			if r > 0x7f {
				result = append(result, byte(r)&0x7f)
			} else {
				result = append(result, byte(r))
			}
		}
		return result, nil
	}
	return nil, fmt.Errorf("Unknown encoding: %s", encoding)
}

func BufferBytesFromValue(v value.Value, encoding string) ([]byte, error) {
	if text, ok := ObjectStringValue(v); ok {
		return decodeBufferEncoding(text, encoding)
	}
	if !v.IsObject() {
		return nil, fmt.Errorf("The first argument must be a string, Buffer, ArrayBuffer, Array, or Array-like object")
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return nil, err
	}
	switch source := obj.(type) {
	case *ObjArrayBuffer:
		data := make([]byte, len(source.Data))
		copy(data, source.Data)
		return data, nil
	case *ObjTypedArray:
		start := source.Offset
		end := start + source.Length*source.ElementSize()
		data := make([]byte, end-start)
		copy(data, source.Buffer.Data[start:end])
		return data, nil
	case *ObjArr:
		data := make([]byte, source.Length())
		for i := 0; i < source.Length(); i++ {
			item := source.GetElementAt(i)
			if item.IsNumber() {
				data[i] = byte(int64(item.AsNumber()) & 0xff)
			}
		}
		return data, nil
	case *ObjObject:
		lengthValue := source.GetMember(value.EncodeHandle(heap.Allocate(NewLightString("length"))))
		if lengthValue.IsNumber() {
			length := int(lengthValue.AsNumber())
			data := make([]byte, length)
			for i := 0; i < length; i++ {
				item := source.GetMember(IndexValue(i))
				if item.IsNumber() {
					data[i] = byte(int64(item.AsNumber()) & 0xff)
				}
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("The first argument must be a string, Buffer, ArrayBuffer, Array, or Array-like object")
}

func (*BufferConstructor) New(params ...any) (object.Object, error) {
	data := []byte{}
	if len(params) > 0 {
		if v, ok := params[0].(value.Value); ok {
			encoding := "utf8"
			if len(params) > 1 {
				if encodingValue, ok := params[1].(value.Value); ok {
					if text, ok := ObjectStringValue(encodingValue); ok {
						encoding = text
					}
				}
			}
			decoded, err := BufferBytesFromValue(v, encoding)
			if err != nil {
				return nil, err
			}
			data = decoded
		}
	}
	return NewBufferFromBytes(data), nil
}
