package native

import (
	"encoding/json"
	"fmt"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"strconv"
	"strings"
)

const JSON_OBJECT_NAME = "JSON"

type JSONObject struct {
	ObjObject
}

func NewJSONObject() *JSONObject {
	j := &JSONObject{}
	j.Members = map[string]ObjectValueEntry{}
	j.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	j.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("parse"))), value.EncodeHandle(heap.Allocate(NewJSONParse())))
	j.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("stringify"))), value.EncodeHandle(heap.Allocate(NewJSONStringify())))
	return j
}

func (*JSONObject) Type() object.ObjType {
	return object.OBJ_OBJECT
}

func (*JSONObject) String() string {
	return "JSON"
}

type JSONParse struct {
	ObjNativeFn
}

func NewJSONParse() *JSONParse {
	j := &JSONParse{}
	j.name = "parse"
	return j
}

func ConvertJSON(v interface{}) value.Value {
	switch v := v.(type) {
	case nil:
		return value.NULL
	case bool:
		if v {
			return value.TRUE
		}
		return value.FALSE
	case float64:
		return value.ValueFromFloat64(v)
	case json.Number:
		n, _ := strconv.ParseFloat(string(v), 64)
		return value.ValueFromFloat64(n)
	case string:
		return value.EncodeHandle(heap.Allocate(NewLightString(v)))
	case []interface{}:
		items := make([]value.Value, len(v))
		for i, item := range v {
			items[i] = ConvertJSON(item)
		}
		return value.EncodeHandle(heap.Allocate(NewArrayFrom(items)))
	case map[string]interface{}:
		obj := NewObjectHash()
		for key, item := range v {
			obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString(key))), ConvertJSON(item))
		}
		return value.EncodeHandle(heap.Allocate(obj))
	}
	return value.UNDEFINED
}

func JSONStringToValue(text string) (value.Value, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var v interface{}
	if err := decoder.Decode(&v); err != nil {
		return value.UNDEFINED, err
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err == nil {
		return value.UNDEFINED, fmt.Errorf("unexpected token")
	}
	return ConvertJSON(v), nil
}

func (*JSONParse) Parse(text string) (value.Value, error) {
	return JSONStringToValue(text)
}

type JSONStringify struct {
	ObjNativeFn
}

func NewJSONStringify() *JSONStringify {
	j := &JSONStringify{}
	j.name = "stringify"
	return j
}
