package stringer

import (
	"fmt"
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
	"strings"
)

func DebugString(v value.Value) string {
	if v.IsBoolean() {
		return fmt.Sprintf("%v", v.AsBoolean())
	} else if v.IsObject() {
		handle := v.GetHandle()
		obj, _ := allocator.GetObject(handle)

		if obj, ok := obj.(*object.ObjObject); ok {
			lines := []string{}
			for k, v := range obj.Hash {
				lines = append(lines, fmt.Sprintf("%s: %s", k, DebugString(v)))
			}
			return strings.Join(lines, ", ")
		}
		return obj.String()

	} else if v.IsNaN() {
		return "NaN"
	} else if v.IsType(value.TAG_UNDEFINED) {
		return "undefined"
	} else if v.IsType(value.TAG_NIL) {
		return "null"
	} else {
		return fmt.Sprintf("%f", v.AsNumber())
	}
}

func String(v value.Value) string {
	if v.IsBoolean() {
		return fmt.Sprintf("%v", v.AsBoolean())
	} else if v.IsObject() {
		obj, _ := allocator.GetObject(v.GetHandle())
		return obj.String()
	} else if v.IsNaN() {
		return "NaN"
	} else if v.IsType(value.TAG_UNDEFINED) {
		return "undefined"
	} else if v.IsType(value.TAG_NIL) {
		return "null"
	} else {
		return fmt.Sprintf("%f", v.AsNumber())
	}
}
