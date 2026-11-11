package native

import (
	"fmt"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"math"
	"time"
)

type Constructor interface {
	New(params ...any) (object.Object, error)
}

type ErrorConstructor struct {
	ObjObject
	errorName string
}

func NewErrorConstructor() *ErrorConstructor {
	return NewNamedErrorConstructor("Error")
}

func NewNamedErrorConstructor(name string) *ErrorConstructor {
	ec := &ErrorConstructor{errorName: name}
	ec.Members = map[string]ObjectValueEntry{}
	return ec
}

func (*ErrorConstructor) String() string {
	return "Error constructor"
}

func (ec *ErrorConstructor) ErrorName() string {
	return ec.errorName
}

func (*ErrorConstructor) Type() object.ObjType {
	return object.OBJ_ERROR_CONSTRUCTOR
}

func (ec *ErrorConstructor) New(params ...any) (object.Object, error) {
	errorObject := NewNamedError(ec.errorName)
	if len(params) > 0 {
		if argHandle, ok := params[0].(value.Value); ok {
			if argHandle != value.UNDEFINED {
				arg, err := heap.GetObject(argHandle.GetHandle())
				if err == nil {
					errorObject.SetMember(KEY_MESSAGE, value.EncodeHandle(heap.Allocate(NewLightString(arg.String()))))
				}
			}
		}
	}
	return errorObject, nil
}

type ObjectConstructor struct {
	ObjObject
}

func NewObjectConstructor() *ObjectConstructor {
	oc := &ObjectConstructor{}
	oc.Members = map[string]ObjectValueEntry{}

	oc.SetMember(KEY_VALUES, value.EncodeHandle(heap.Allocate(NewObjectValues())))
	oc.SetMember(KEY_KEYS, value.EncodeHandle(heap.Allocate(NewObjectKeys())))
	oc.SetMember(KEY_CREATE, value.EncodeHandle(heap.Allocate(NewCreate())))
	oc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("prototype"))), PROTOTYPE_OBJECT)
	oc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("hasOwn"))), value.EncodeHandle(heap.Allocate(NewObjectHasOwn())))
	oc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("getPrototypeOf"))), value.EncodeHandle(heap.Allocate(NewObjectGetPrototypeOf())))
	oc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("setPrototypeOf"))), value.EncodeHandle(heap.Allocate(NewObjectSetPrototypeOf())))
	oc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("getOwnPropertySymbols"))), value.EncodeHandle(heap.Allocate(NewObjectGetOwnPropertySymbols())))
	oc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("getOwnPropertyNames"))), value.EncodeHandle(heap.Allocate(NewObjectGetOwnPropertyNames())))
	oc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("defineProperty"))), value.EncodeHandle(heap.Allocate(NewObjectDefineProperty())))
	oc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("getOwnPropertyDescriptor"))), value.EncodeHandle(heap.Allocate(NewObjectGetOwnPropertyDescriptor())))

	return oc
}

func (*ObjectConstructor) String() string {
	return "function Object"
}

func (*ObjectConstructor) Type() object.ObjType {
	return object.OBJ_OBJECT_CONSTRUCTOR
}

func (*ObjectConstructor) New() object.Object {
	return NewObject()
}

func NewObject() *ObjObject {
	obj := &ObjObject{Members: map[string]ObjectValueEntry{}}

	obj.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return obj
}

func NewGlobalObject() *ObjObject {
	obj := NewObject()
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("undefined"))), value.UNDEFINED)
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("NaN"))), value.STANDARD_NAN)
	inf := value.ValueFromFloat64(math.Inf(1))
	obj.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("Infinity"))), inf)
	return obj
}

type ArrayConstructor struct {
	ObjObject
}

func NewArrayConstructor() *ArrayConstructor {
	a := &ArrayConstructor{}
	a.Members = map[string]ObjectValueEntry{}
	a.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("prototype"))), PROTOTYPE_ARRAY)
	a.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("isArray"))), value.EncodeHandle(heap.Allocate(NewArrayStaticMethod("isArray", ARRAY_STATIC_IS_ARRAY))))
	a.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("of"))), value.EncodeHandle(heap.Allocate(NewArrayStaticMethod("of", ARRAY_STATIC_OF))))
	a.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("from"))), value.EncodeHandle(heap.Allocate(NewArrayStaticMethod("from", ARRAY_STATIC_FROM))))
	return a
}

func (*ArrayConstructor) String() string {
	return "function Array"
}

func (*ArrayConstructor) Type() object.ObjType {
	return object.OBJ_ARRAY_CONSTRUCTOR
}

func (*ArrayConstructor) New(params ...any) (object.Object, error) {
	if len(params) == 0 {
		return NewArrayFrom([]value.Value{}), nil
	}
	if len(params) == 1 {
		if v, ok := params[0].(value.Value); ok {
			if v.IsNumber() {
				n := v.AsNumber()
				if n < 0 || math.Trunc(n) != n || n >= 4294967296 {
					return nil, fmt.Errorf("Invalid array length")
				}
				length := int(n)
				items := make([]value.Value, length)
				for i := range items {
					items[i] = value.EMPTY_ARRAY_ITEM
				}
				return NewArrayFrom(items), nil
			}
		}
	}
	items := make([]value.Value, len(params))
	for i, param := range params {
		if v, ok := param.(value.Value); ok {
			items[i] = v
		}
	}
	return NewArrayFrom(items), nil
}

type StringConstructor struct {
	ObjObject
}

func (*StringConstructor) String() string {
	return "function String"
}

func (*StringConstructor) Type() object.ObjType {
	return object.OBJ_STRING_CONSTRUCTOR
}

func NewString(str string) *ObjString {
	objStr := NewObjString(str)
	objStr.Members = map[string]ObjectValueEntry{}
	objStr.SetMember(KEY_PROTO, PROTOTYPE_STRING)
	return objStr
}

type Main struct {
	Fn *object.ObjFunction
}

func NewMain(fn *object.ObjFunction) *Main {
	return &Main{
		Fn: fn,
	}
}

func (m *Main) Work(callbackChannel chan *object.JobChannelMessage, done func()) {
	msg := &object.JobChannelMessage{
		Callback: object.Callback{
			Fn:      m.Fn.Clone(),
			ThisCtx: value.UNDEFINED,
		},
		Done: done,
	}
	callbackChannel <- msg
}

type SetTimeout struct {
	ObjNativeFn
	time     int
	callback object.Callback
}

func NewSetTimeout() *SetTimeout {
	setTimeout := &SetTimeout{}
	setTimeout.name = "setTimeout"

	return setTimeout
}

func (st *SetTimeout) Set(ms int, fn *object.ObjFunction, this value.Value) {
	st.time = ms
	st.callback = object.Callback{
		Fn:      fn,
		ThisCtx: this,
	}
}

func (st *SetTimeout) Work(callbackChannel chan *object.JobChannelMessage, done func()) {
	message := &object.JobChannelMessage{
		Job:      nil,
		Callback: st.callback,
		Done:     done,
	}
	if st.time == 0 {
		callbackChannel <- message
		return
	}
	tick := time.NewTicker((time.Duration(st.time) * time.Millisecond))

	for range tick.C {
		callbackChannel <- message
		break
	}
}

func (st *SetTimeout) Clone() *SetTimeout {
	clone := *st
	return &clone
}

type MapConstructor struct {
	ObjObject
}

func NewMapConstructor() *MapConstructor {
	return &MapConstructor{}
}

func (*MapConstructor) String() string {
	return "Function Map"
}

func (*MapConstructor) Type() object.ObjType {
	return object.OBJ_MAP_CONSTRUCTOR
}

func (*MapConstructor) New(params ...any) (object.Object, error) {
	m := NewMap()
	if len(params) > 0 {
		if v, ok := params[0].(value.Value); ok && v.IsObject() {
			if obj, err := heap.GetObject(v.GetHandle()); err == nil {
				if arr, ok := obj.(*ObjArr); ok {
					setter := &MapSet{}
					for i := 0; i < arr.Length(); i++ {
						entryValue := arr.GetElementAt(i)
						if !entryValue.IsObject() {
							return nil, fmt.Errorf("Iterator value is not an entry object")
						}
						entryObj, err := heap.GetObject(entryValue.GetHandle())
						if err != nil {
							return nil, err
						}
						entry, ok := entryObj.(*ObjArr)
						if !ok || entry.Length() < 2 {
							return nil, fmt.Errorf("Iterator value is not an entry object")
						}
						setter.Set(m, entry.GetElementAt(0), entry.GetElementAt(1))
					}
				}
			}
		}
	}
	return m, nil
}

type SetConstructor struct {
	ObjObject
}

func NewSetConstructor() *SetConstructor {
	return &SetConstructor{}
}

func (*SetConstructor) String() string {
	return "Function Set"
}

func (*SetConstructor) Type() object.ObjType {
	return object.OBJ_SET_CONSTRUCTOR
}

func (*SetConstructor) New(params ...any) (object.Object, error) {
	s := NewSet()
	adder := &SetAdd{}
	if len(params) > 0 {
		if v, ok := params[0].(value.Value); ok {
			if str, ok := ObjectStringValue(v); ok {
				for _, r := range []rune(str) {
					adder.Add(s, value.EncodeHandle(heap.Allocate(NewLightString(string(r)))))
				}
			} else if v.IsObject() {
				if obj, err := heap.GetObject(v.GetHandle()); err == nil {
					switch source := obj.(type) {
					case *ObjArr:
						for i := 0; i < source.Length(); i++ {
							adder.Add(s, source.GetElementAt(i))
						}
					case *Set:
						for _, item := range source.order {
							adder.Add(s, item)
						}
					}
				}
			}
		}
	}
	return s, nil
}
