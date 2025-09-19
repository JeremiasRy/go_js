package stringer

import (
	"fmt"
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
	"strconv"
)

func DebugString(v value.Value) string {
	if v.IsBoolean() {
		return fmt.Sprintf("%v", v.AsBoolean())
	} else if v.IsObject() {
		handle := v.GetHandle()
		obj, _ := allocator.GetObject(handle)
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

		switch obj := obj.(type) {
		case *object.ObjError:
			{
				return String(obj.Hash["message"])
			}
		}
		return obj.String()
	} else if v.IsNaN() {
		return "NaN"
	} else if v.IsType(value.TAG_UNDEFINED) {
		return "undefined"
	} else if v.IsType(value.TAG_NIL) {
		return "null"
	} else {
		return strconv.FormatFloat(v.AsNumber(), 'f', -1, 64)
	}
}
