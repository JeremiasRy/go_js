package stringer

import (
	"fmt"
	"go_js/allocator"
	"go_js/native"
	"go_js/object"

	"go_js/value"
	"strconv"
	"strings"
)

func DebugString(v value.Value) string {
	if v.IsObject() {
		handle := v.GetHandle()
		obj, _ := allocator.GetObject(handle)
		return obj.String()

	}
	if v.IsBoolean() {
		return fmt.Sprintf("%v", v&value.TRUE == value.TRUE)
	} else if v.IsType(value.UNDEFINED) {
		return "undefined"
	} else if v.IsType(value.NULL) {
		return "null"
	}
	return strconv.FormatFloat(v.AsNumber(), 'f', -1, 64)
}

func String(v value.Value) string {
	if v.IsObject() {
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
					if k == native.PROTOTYPE_PROPERTY_STRING {
						continue
					}
					fmt.Fprintf(&b, " %s: %s,", k, TypeDecoratedString(v.Value))
				}
				fmt.Fprintln(&b, "}")
			}
		}
		return obj.String()
	}

	if v.IsBoolean() {
		return fmt.Sprintf("%v", v&value.TRUE == value.TRUE)
	} else if v.IsType(value.UNDEFINED) {
		return "undefined"
	} else if v.IsType(value.NULL) {
		return "null"
	}

	return strconv.FormatFloat(v.AsNumber(), 'f', -1, 64)

}

func TypeDecoratedString(v value.Value) string {
	if v.IsObject() {
		obj, _ := allocator.GetObject(v.GetHandle())

		switch obj := obj.(type) {
		case *native.ObjObject:
			{
				var b strings.Builder
				fmt.Fprint(&b, "{ ")
				l := len(obj.Members)
				c := 0
				for k, v := range obj.Members {
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
	}

	if v.IsBoolean() {
		return fmt.Sprintf("%v", v&value.TRUE == value.TRUE)
	} else if v.IsType(value.UNDEFINED) {
		return "undefined"
	} else if v.IsType(value.NULL) {
		return "null"
	} else if v.IsType(value.TAG_METHOD_HANDLE) {
		return "METHOD_HANDLE"
	}
	return strconv.FormatFloat(v.AsNumber(), 'f', -1, 64)
}

func ObjToPrimitive(o object.Object) string {
	switch obj := o.(type) {
	case *native.ObjArr:
		{
			if len(obj.Values()) == 0 {
				return ""
			}
			arr := []string{}

			for _, i := range obj.Values() {
				arr = append(arr, String(i))
			}

			return strings.Join(arr, ",")
		}
	default:
		return "[Object object]"
	}

}
