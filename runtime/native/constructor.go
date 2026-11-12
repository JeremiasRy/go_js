package native

import (
	"fmt"
	"go_js/eventloop"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"math"
	"sync/atomic"
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

type ErrorIsError struct {
	ObjNativeFn
}

func NewErrorIsError() *ErrorIsError {
	fn := &ErrorIsError{}
	fn.name = "isError"
	return fn
}

func NewNamedErrorConstructor(name string) *ErrorConstructor {
	ec := &ErrorConstructor{errorName: name}
	ec.Members = map[string]ObjectValueEntry{}
	if proto, found := ERROR_PROTOTYPES[name]; found {
		ec.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("prototype"))), proto)
	}
	ec.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("isError"))), value.EncodeHandle(heap.Allocate(NewErrorIsError())))
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
	messageIndex := 0
	if ec.errorName == "AggregateError" {
		errors := []value.Value{}
		if len(params) > 0 {
			if first, ok := params[0].(value.Value); ok {
				if first.IsObject() {
					if obj, err := heap.GetObject(first.GetHandle()); err == nil {
						if arr, ok := obj.(*ObjArr); ok {
							for i := 0; i < arr.Length(); i++ {
								errors = append(errors, arr.GetElementAt(i))
							}
						} else if hashable, ok := obj.(object.Hashable); ok {
							lengthValue := hashable.GetMember(value.EncodeHandle(heap.Allocate(NewLightString("length"))))
							if lengthValue.IsNumber() {
								for i := 0; i < int(lengthValue.AsNumber()); i++ {
									errors = append(errors, hashable.GetMember(IndexValue(i)))
								}
							}
						}
					}
				}
			}
		}
		errorObject.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("errors"))), value.EncodeHandle(heap.Allocate(NewArrayFrom(errors))))
		messageIndex = 1
	}
	if len(params) > messageIndex {
		if argHandle, ok := params[messageIndex].(value.Value); ok {
			if argHandle != value.UNDEFINED {
				arg, err := heap.GetObject(argHandle.GetHandle())
				message := ""
				if err == nil {
					message = arg.String()
				} else if argHandle.IsNumber() || argHandle.IsBoolean() {
					message = String(argHandle)
				} else if argHandle.IsType(value.NULL) {
					message = "null"
				}
				errorObject.SetMember(KEY_MESSAGE, value.EncodeHandle(heap.Allocate(NewLightString(message))))
			}
		}
	}
	optionsIndex := messageIndex + 1
	if len(params) > optionsIndex {
		if options, ok := params[optionsIndex].(value.Value); ok && options.IsObject() {
			if obj, err := heap.GetObject(options.GetHandle()); err == nil {
				if hashable, ok := obj.(object.Hashable); ok {
					causeKey := value.EncodeHandle(heap.Allocate(NewLightString("cause")))
					if hashable.HasOwn(causeKey) {
						errorObject.SetMember(causeKey, hashable.GetMember(causeKey))
					}
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

type ArrayFromAsync struct {
	ObjNativeFn
}

func NewArrayFromAsync() *ArrayFromAsync {
	fn := &ArrayFromAsync{}
	fn.name = "fromAsync"
	return fn
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
	a.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("fromAsync"))), value.EncodeHandle(heap.Allocate(NewArrayFromAsync())))
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
	args     []value.Value
	rootKey  uintptr
	cancel   *atomic.Bool
	interval bool
}

func NewSetTimeout() *SetTimeout {
	setTimeout := &SetTimeout{cancel: &atomic.Bool{}}
	setTimeout.name = "setTimeout"

	return setTimeout
}

func NewSetInterval() *SetTimeout {
	interval := &SetTimeout{cancel: &atomic.Bool{}, interval: true}
	interval.name = "setInterval"
	return interval
}

func (st *SetTimeout) Set(ms int, fn object.Callable, this value.Value, args []value.Value) {
	st.time = ms
	st.args = append([]value.Value{}, args...)
	st.cancel = &atomic.Bool{}
	st.callback = object.Callback{
		Fn:      fn,
		ThisCtx: this,
	}
}

func (st *SetTimeout) Cancel() {
	st.cancel.Store(true)
}

func (st *SetTimeout) SetRootKey(key uintptr) {
	st.rootKey = key
}

func (st *SetTimeout) Work(callbackChannel chan *object.JobChannelMessage, done func()) {
	if st.cancel.Load() {
		done()
		return
	}
	callback := st.callback
	callback.RootKey = st.rootKey
	callback.Stack = append([]value.Value{}, st.args...)
	message := &object.JobChannelMessage{
		Job:      nil,
		Callback: callback,
		Done:     done,
	}
	if st.time == 0 && !st.interval {
		callbackChannel <- message
		return
	}
	delay := time.Duration(st.time) * time.Millisecond
	if delay <= 0 {
		delay = time.Millisecond
	}
	tick := time.NewTicker(delay)
	defer tick.Stop()
	for range tick.C {
		if st.cancel.Load() {
			done()
			return
		}
		if st.interval {
			message.Done = func() {
				done()
				if !st.cancel.Load() {
					eventloop.Dispatch(st.Clone())
				}
			}
		}
		callbackChannel <- message
		return
	}
}

func (st *SetTimeout) Clone() *SetTimeout {
	clone := *st
	return &clone
}

type MapConstructor struct {
	ObjObject
}

type MapGroupBy struct {
	ObjNativeFn
}

func NewMapGroupBy() *MapGroupBy {
	fn := &MapGroupBy{}
	fn.name = "groupBy"
	return fn
}

func NewMapConstructor() *MapConstructor {
	m := &MapConstructor{}
	m.Members = map[string]ObjectValueEntry{}
	m.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	m.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("prototype"))), PROTOTYPE_MAP)
	m.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("groupBy"))), value.EncodeHandle(heap.Allocate(NewMapGroupBy())))
	return m
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
		if v, ok := params[0].(value.Value); ok {
			if v == value.UNDEFINED || v == value.NULL {
				return m, nil
			}
			if !v.IsObject() {
				return nil, fmt.Errorf("%s is not iterable", String(v))
			}
			obj, err := heap.GetObject(v.GetHandle())
			if err != nil {
				return nil, err
			}
			setter := &MapSet{}
			addEntry := func(entryValue value.Value) error {
				if !entryValue.IsObject() {
					return fmt.Errorf("Iterator value is not an entry object")
				}
				entryObj, err := heap.GetObject(entryValue.GetHandle())
				if err != nil {
					return err
				}
				entry, ok := entryObj.(*ObjArr)
				if !ok || entry.Length() < 2 {
					return fmt.Errorf("Iterator value is not an entry object")
				}
				setter.Set(m, entry.GetElementAt(0), entry.GetElementAt(1))
				return nil
			}
			switch source := obj.(type) {
			case *ObjArr:
				for i := 0; i < source.Length(); i++ {
					if err := addEntry(source.GetElementAt(i)); err != nil {
						return nil, err
					}
				}
			case *Map:
				for _, entry := range source.orderedEntries() {
					setter.Set(m, entry.Key, entry.Value)
				}
			case *Set:
				for _, item := range source.order {
					if err := addEntry(item); err != nil {
						return nil, err
					}
				}
			default:
				return nil, fmt.Errorf("%s is not iterable", String(v))
			}
		}
	}
	return m, nil
}

type SetConstructor struct {
	ObjObject
}

func NewSetConstructor() *SetConstructor {
	s := &SetConstructor{}
	s.Members = map[string]ObjectValueEntry{}
	s.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	s.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("prototype"))), PROTOTYPE_SET)
	return s
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
			if v == value.UNDEFINED || v == value.NULL {
				return s, nil
			}
			if str, ok := ObjectStringValue(v); ok {
				for _, r := range []rune(str) {
					adder.Add(s, value.EncodeHandle(heap.Allocate(NewLightString(string(r)))))
				}
			} else if v.IsObject() {
				obj, err := heap.GetObject(v.GetHandle())
				if err != nil {
					return nil, err
				}
				switch source := obj.(type) {
				case *ObjArr:
					for i := 0; i < source.Length(); i++ {
						adder.Add(s, source.GetElementAt(i))
					}
				case *Set:
					for _, item := range source.order {
						adder.Add(s, item)
					}
				case *Map:
					for _, item := range source.ValuesList() {
						adder.Add(s, item)
					}
				default:
					return nil, fmt.Errorf("%s is not iterable", String(v))
				}
			} else {
				return nil, fmt.Errorf("%s is not iterable", String(v))
			}
		}
	}
	return s, nil
}
