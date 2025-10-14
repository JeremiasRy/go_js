package native

import (
	"fmt"
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
	"strconv"
	"strings"
)

type ObjNativeFn struct {
	object.GC_TAG
	object.Ctx
	name  string
	Arity int
}

func (ObjNativeFn) Type() object.ObjType {
	return object.OBJ_NATIVE_FN
}

func (onf *ObjNativeFn) String() string {
	return fmt.Sprintf("<native fn %s()>", onf.name)
}

type QueueMicroTask struct {
	ObjNativeFn
}

func NewQueueMicroTask() *QueueMicroTask {
	q := &QueueMicroTask{}
	q.name = QUEUE_MICRO_TASK_NAME
	return q
}

type ParseInt struct {
	ObjNativeFn
}

func NewParseInt() *ParseInt {
	p := &ParseInt{}
	p.name = "parseInt"

	return p
}

func (*ParseInt) ParseInteger(str string, base int) (value.Value, error) {
	i, err := strconv.ParseInt(str, base, 64)

	if err != nil {
		return value.UNDEFINED, err
	}

	return value.ValueFromFloat64(float64(i)), nil
}

func Init() {
	createCommonHandles()
	initObjectPrototype()

	initArrayPrototype()
	initStringPrototype()
	initGeneratorPrototype()
	initMapPrototype()
	initSetPrototype()
	initPromiseConstructorPrototype()
	initPromisePrototype()
}

func String(v value.Value) string {
	if v.IsObject() {
		obj, _ := allocator.GetObject(v.GetHandle())

		switch obj := obj.(type) {
		case *ObjError:
			{
				return String(obj.Members["message"].Value)
			}
		case *ObjArr:
			{
				return TypeDecoratedString(v)
			}
		case *ObjObject:
			{
				var b strings.Builder
				fmt.Fprint(&b, "{")
				l := len(obj.Members) - 1
				c := 0
				for k, v := range obj.Members {
					if k == PROTOTYPE_PROPERTY_STRING {
						continue
					}
					fmt.Fprintf(&b, " %s: %s", k, TypeDecoratedString(v.Value))
					c++
					if c < l {
						fmt.Fprint(&b, ", ")
					}
				}
				fmt.Fprintln(&b, " }")
				return b.String()
			}
		}
		return obj.String()
	}

	if v.IsBoolean() {
		return fmt.Sprintf("%v", v == value.TRUE)
	} else if v.IsType(value.UNDEFINED) {
		return "undefined"
	} else if v.IsType(value.NULL) {
		return "null"
	} else if v.IsType(value.EMPTY_ARRAY_ITEM) {
		return "EMPTY_ITEM"
	}

	return strconv.FormatFloat(v.AsNumber(), 'f', -1, 64)
}

func TypeDecoratedString(v value.Value) string {
	if v.IsObject() {
		obj, _ := allocator.GetObject(v.GetHandle())

		switch obj := obj.(type) {
		case *ObjObject:
			{
				var b strings.Builder
				fmt.Fprint(&b, "{ ")
				l := len(obj.Members) - 1
				c := 0
				for k, v := range obj.Members {
					if k == PROTOTYPE_PROPERTY_STRING {
						continue
					}
					c++
					fmt.Fprintf(&b, "%s: %s", k, TypeDecoratedString(v.Value))
					if c < l {
						fmt.Fprint(&b, ", ")
					}
				}
				fmt.Fprint(&b, " }")

				return b.String()
			}
		case *ObjError:
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
		case *ObjString:
			{
				return fmt.Sprintf("'%s'", obj.Value)
			}
		case *LightString:
			{

				return fmt.Sprintf("'%s'", obj)
			}
		case *ObjArr:
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
		return fmt.Sprintf("%v", v == value.TRUE)
	} else if v.IsType(value.UNDEFINED) {
		return "undefined"
	} else if v.IsType(value.NULL) {
		return "null"
	} else if v.IsType(value.TAG_METHOD_HANDLE) {
		return "METHOD_HANDLE"
	} else if v.IsType(value.TAG_ARGUMENT_START) {
		return "ARG_START"
	} else if v.IsType(value.EMPTY_ARRAY_ITEM) {
		return "EMPTY_ITEM"
	}
	return strconv.FormatFloat(v.AsNumber(), 'f', -1, 64)
}

func DebugString(v value.Value) string {
	if v.IsObject() {
		handle := v.GetHandle()
		obj, _ := allocator.GetObject(handle)
		return obj.String()

	}
	if v.IsBoolean() {
		return fmt.Sprintf("%v", v == value.TRUE)
	} else if v.IsType(value.UNDEFINED) {
		return "undefined"
	} else if v.IsType(value.NULL) {
		return "null"
	} else if v.IsType(value.EMPTY_ARRAY_ITEM) {
		return "EMPTY_ITEM"
	}
	return strconv.FormatFloat(v.AsNumber(), 'f', -1, 64)
}

func ObjToPrimitive(o object.Object) string {
	switch obj := o.(type) {
	case *ObjArr:
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
		return "[object Object]"
	}

}
