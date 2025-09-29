package stringer

import (
	"fmt"
	"go_js/allocator"
	"go_js/native"

	"go_js/value"
	"strconv"
	"strings"
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
		return strconv.FormatFloat(v.AsNumber(), 'f', -1, 64)
	}
}

func String(v value.Value) string {
	if v.IsBoolean() {
		return fmt.Sprintf("%v", v.AsBoolean())
	} else if v.IsObject() {
		obj, _ := allocator.GetObject(v.GetHandle())

		switch obj := obj.(type) {
		case *native.ObjError:
			{
				return String(obj.Members["message"].Value)
			}
		case *native.ObjArr:
			{
				return TypeDecoratedString(v)
			}
		case *native.ObjObject:
			{
				var b strings.Builder
				fmt.Fprint(&b, "{")
				for k, v := range obj.Members {
					if k == native.PROTOTYPE_KEY {
						continue
					}
					fmt.Fprintf(&b, " %s: %s,", k, TypeDecoratedString(v.Value))
				}
				fmt.Fprintln(&b, "}")
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

func TypeDecoratedString(v value.Value) string {
	if v.IsBoolean() {
		return fmt.Sprintf("%v", v.AsBoolean())
	} else if v.IsObject() {
		obj, _ := allocator.GetObject(v.GetHandle())

		switch obj := obj.(type) {
		case *native.ObjObject:
			{
				var b strings.Builder
				fmt.Fprint(&b, "{ ")
				l := len(obj.Members)
				c := 0
				for k, v := range obj.Members {
					fmt.Println(k)
					c++
					fmt.Fprintf(&b, "%s: %s", k, TypeDecoratedString(v.Value))
					if l != c {
						fmt.Fprint(&b, ", ")
					}
				}
				fmt.Fprint(&b, " }")

				return b.String()
			}
		case *native.ObjError:
			{
				var b strings.Builder
				fmt.Fprint(&b, "Error { ")
				l := len(obj.Members)
				c := 0
				for k, v := range obj.Members {
					c++
					fmt.Fprintf(&b, "%s: %s", k, TypeDecoratedString(v.Value))
					if l != c {
						fmt.Fprint(&b, ",")
					}
				}
				fmt.Fprint(&b, " }")

				return b.String()
			}
		case *native.ObjString:
			{
				return fmt.Sprintf("'%s'", obj.Value)
			}
		case native.LightString:
			{
				return fmt.Sprintf("'%s'", obj)
			}
		case *native.ObjArr:
			{
				var b strings.Builder
				fmt.Fprint(&b, "[")
				for i, v := range obj.Values() {
					fmt.Fprintf(&b, " %s", TypeDecoratedString(v))
					if i != len(obj.Values())-1 {
						fmt.Fprint(&b, ",")
					}
				}
				fmt.Fprint(&b, " ]")
				return b.String()
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
