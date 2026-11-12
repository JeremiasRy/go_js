package native

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"go_js/heap"
	"go_js/object"
	"go_js/value"
)

type GlobalFunctionKind int

const (
	GLOBAL_IS_NAN GlobalFunctionKind = iota
	GLOBAL_IS_FINITE
	GLOBAL_ENCODE_URI_COMPONENT
	GLOBAL_DECODE_URI_COMPONENT
)

type GlobalFunction struct {
	ObjNativeFn
	Kind GlobalFunctionKind
}

func NewGlobalFunction(name string, kind GlobalFunctionKind) *GlobalFunction {
	fn := &GlobalFunction{Kind: kind}
	fn.name = name
	return fn
}

func EncodeURIComponent(text string) string {
	var builder strings.Builder
	for _, b := range []byte(text) {
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || strings.ContainsRune("-_.!~*'()", rune(b)) {
			builder.WriteByte(b)
		} else {
			builder.WriteString(fmt.Sprintf("%%%02X", b))
		}
	}
	return builder.String()
}

func DecodeURIComponent(text string) (string, error) {
	return url.PathUnescape(text)
}

type WeakMap struct {
	Map
}

func NewWeakMap() *WeakMap {
	m := NewMap()
	m.Members = map[string]ObjectValueEntry{}
	m.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return &WeakMap{Map: *m}
}

func (*WeakMap) Type() object.ObjType { return object.OBJ_OBJECT }
func (*WeakMap) String() string       { return "WeakMap { <items unknown> }" }

type WeakSet struct {
	Set
}

func NewWeakSet() *WeakSet {
	s := NewSet()
	s.Members = map[string]ObjectValueEntry{}
	s.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	return &WeakSet{Set: *s}
}

func (*WeakSet) Type() object.ObjType { return object.OBJ_OBJECT }
func (*WeakSet) String() string       { return "WeakSet { <items unknown> }" }

type WeakMapConstructor struct {
	ObjObject
}

func NewWeakMapConstructor() *WeakMapConstructor {
	c := &WeakMapConstructor{}
	c.Members = map[string]ObjectValueEntry{}
	return c
}

func (*WeakMapConstructor) Type() object.ObjType { return object.OBJ_MAP_CONSTRUCTOR }
func (*WeakMapConstructor) String() string       { return "function WeakMap" }
func (*WeakMapConstructor) New(params ...any) (object.Object, error) {
	w := NewWeakMap()
	if len(params) > 0 {
		if source, ok := params[0].(value.Value); ok && source.IsObject() {
			if obj, err := heap.GetObject(source.GetHandle()); err == nil {
				switch entries := obj.(type) {
				case *ObjArr:
					for i := 0; i < entries.Length(); i++ {
						entry := entries.GetElementAt(i)
						if !entry.IsObject() {
							return nil, fmt.Errorf("Iterator value is not an entry object")
						}
						entryObj, err := heap.GetObject(entry.GetHandle())
						if err != nil {
							return nil, err
						}
						pair, ok := entryObj.(*ObjArr)
						if !ok || pair.Length() < 2 {
							return nil, fmt.Errorf("Iterator value is not an entry object")
						}
						(&MapSet{}).Set(&w.Map, pair.GetElementAt(0), pair.GetElementAt(1))
					}
				case *Map:
					for _, entry := range entries.orderedEntries() {
						(&MapSet{}).Set(&w.Map, entry.Key, entry.Value)
					}
				}
			}
		}
	}
	return w, nil
}

type WeakSetConstructor struct {
	ObjObject
}

func NewWeakSetConstructor() *WeakSetConstructor {
	c := &WeakSetConstructor{}
	c.Members = map[string]ObjectValueEntry{}
	return c
}

func (*WeakSetConstructor) Type() object.ObjType { return object.OBJ_SET_CONSTRUCTOR }
func (*WeakSetConstructor) String() string       { return "function WeakSet" }
func (*WeakSetConstructor) New(params ...any) (object.Object, error) {
	w := NewWeakSet()
	if len(params) > 0 {
		if source, ok := params[0].(value.Value); ok && source.IsObject() {
			if obj, err := heap.GetObject(source.GetHandle()); err == nil {
				switch items := obj.(type) {
				case *ObjArr:
					for i := 0; i < items.Length(); i++ {
						(&SetAdd{}).Add(&w.Set, items.GetElementAt(i))
					}
				case *Set:
					for _, item := range items.order {
						(&SetAdd{}).Add(&w.Set, item)
					}
				}
			}
		}
	}
	return w, nil
}

type IntlObject struct {
	ObjObject
}

func NewIntlObject() *IntlObject {
	o := &IntlObject{}
	o.Members = map[string]ObjectValueEntry{}
	o.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	o.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("NumberFormat"))), value.EncodeHandle(heap.Allocate(NewIntlNumberFormatConstructor())))
	return o
}

func (*IntlObject) String() string { return "[object Intl]" }

type IntlNumberFormatConstructor struct {
	ObjNativeFn
}

func NewIntlNumberFormatConstructor() *IntlNumberFormatConstructor {
	c := &IntlNumberFormatConstructor{}
	c.name = "NumberFormat"
	return c
}

type IntlNumberFormatMethod struct {
	ObjNativeFn
}

func NewIntlNumberFormatMethod() *IntlNumberFormatMethod {
	m := &IntlNumberFormatMethod{}
	m.name = "format"
	return m
}

type URLObject struct {
	ObjObject
}

type URLSearchParamsGet struct {
	ObjNativeFn
	Values url.Values
}

func NewURLConstructor() *URLConstructor {
	c := &URLConstructor{}
	c.Members = map[string]ObjectValueEntry{}
	return c
}

type URLConstructor struct {
	ObjObject
}

func (*URLConstructor) Type() object.ObjType { return object.OBJ_NATIVE_FN }
func (*URLConstructor) String() string       { return "function URL" }

func (*URLConstructor) New(params ...any) (object.Object, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("URL requires a value")
	}
	input, ok := params[0].(value.Value)
	if !ok {
		return nil, fmt.Errorf("URL requires a string")
	}
	text, ok := ObjectStringValue(input)
	if !ok {
		text = String(input)
	}
	parsed, err := url.Parse(text)
	if err != nil {
		return nil, err
	}
	o := NewObjectHash()
	o.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("href"))), value.EncodeHandle(heap.Allocate(NewLightString(parsed.String()))))
	o.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("protocol"))), value.EncodeHandle(heap.Allocate(NewLightString(parsed.Scheme+":"))))
	o.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("hostname"))), value.EncodeHandle(heap.Allocate(NewLightString(parsed.Hostname()))))
	o.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("host"))), value.EncodeHandle(heap.Allocate(NewLightString(parsed.Host))))
	o.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("pathname"))), value.EncodeHandle(heap.Allocate(NewLightString(parsed.Path))))
	o.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("search"))), value.EncodeHandle(heap.Allocate(NewLightString(parsed.RawQuery))))
	searchParams := NewObjectHash()
	searchParams.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("get"))), value.EncodeHandle(heap.Allocate(&URLSearchParamsGet{Values: parsed.Query()})))
	o.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("searchParams"))), value.EncodeHandle(heap.Allocate(searchParams)))
	return o, nil
}

func LocaleNumberString(n float64) string {
	text := nativeNumberString(n)
	negative := strings.HasPrefix(text, "-")
	if negative {
		text = text[1:]
	}
	parts := strings.SplitN(text, ".", 2)
	integer := parts[0]
	if len(integer) > 3 {
		groups := []string{}
		for len(integer) > 3 {
			groups = append([]string{integer[len(integer)-3:]}, groups...)
			integer = integer[:len(integer)-3]
		}
		groups = append([]string{integer}, groups...)
		integer = strings.Join(groups, ",")
	}
	result := integer
	if len(parts) > 1 {
		result += "." + parts[1]
	}
	if negative {
		result = "-" + result
	}
	return result
}

var _ = strconv.Itoa

func (*IntlNumberFormatConstructor) New(params ...any) (object.Object, error) {
	o := NewObjectHash()
	o.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("format"))), value.EncodeHandle(heap.Allocate(NewIntlNumberFormatMethod())))
	return o, nil
}

type FunctionConstructor struct {
	ObjObject
}

func NewFunctionConstructor() *FunctionConstructor {
	f := &FunctionConstructor{}
	f.Members = map[string]ObjectValueEntry{}
	f.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("length"))), value.ValueFromFloat64(1))
	return f
}

func (*FunctionConstructor) Type() object.ObjType { return object.OBJ_NATIVE_FN }
func (*FunctionConstructor) String() string       { return "function Function" }
