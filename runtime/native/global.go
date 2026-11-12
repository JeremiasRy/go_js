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
	m.SetMember(KEY_PROTO, PROTOTYPE_MAP)
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
	s.SetMember(KEY_PROTO, PROTOTYPE_SET)
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
	Parsed *url.URL
}

type URLSearchParamsGet struct {
	ObjNativeFn
	Values url.Values
}

type URLSearchParamsMethodKind int

const (
	URL_SEARCH_PARAMS_GET URLSearchParamsMethodKind = iota
	URL_SEARCH_PARAMS_GET_ALL
	URL_SEARCH_PARAMS_HAS
	URL_SEARCH_PARAMS_SET
	URL_SEARCH_PARAMS_APPEND
	URL_SEARCH_PARAMS_DELETE
	URL_SEARCH_PARAMS_TO_STRING
)

type URLSearchParamsMethod struct {
	ObjNativeFn
	Kind URLSearchParamsMethodKind
}

type URLSearchParamsObject struct {
	ObjObject
	Values url.Values
}

type URLStaticMethodKind int

const (
	URL_CAN_PARSE URLStaticMethodKind = iota
)

type URLStaticMethod struct {
	ObjNativeFn
	Kind URLStaticMethodKind
}

type URLToString struct {
	ObjNativeFn
}

func NewURLToString() *URLToString {
	fn := &URLToString{}
	fn.name = "toString"
	return fn
}

func NewURLConstructor() *URLConstructor {
	c := &URLConstructor{}
	c.Members = map[string]ObjectValueEntry{}
	c.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("canParse"))), value.EncodeHandle(heap.Allocate(&URLStaticMethod{Kind: URL_CAN_PARSE})))
	return c
}

func NewURLSearchParamsConstructor() *URLSearchParamsConstructor {
	c := &URLSearchParamsConstructor{}
	c.Members = map[string]ObjectValueEntry{}
	return c
}

type URLConstructor struct {
	ObjObject
}

func (*URLConstructor) Type() object.ObjType { return object.OBJ_NATIVE_FN }
func (*URLConstructor) String() string       { return "function URL" }

func stringArgument(v value.Value) string {
	if text, ok := ObjectStringValue(v); ok {
		return text
	}
	return String(v)
}

func ParseURL(text, base string) (*url.URL, error) {
	if base == "" {
		parsed, err := url.Parse(text)
		if err != nil {
			return nil, err
		}
		if parsed.Scheme == "" {
			return nil, fmt.Errorf("Invalid URL")
		}
		return parsed, nil
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	return baseURL.Parse(text)
}

func (u *URLObject) Href() string {
	if u.Parsed == nil {
		return ""
	}
	return u.Parsed.String()
}

func (u *URLObject) Origin() string {
	if u.Parsed == nil || u.Parsed.Scheme == "" || u.Parsed.Host == "" {
		return "null"
	}
	return u.Parsed.Scheme + "://" + u.Parsed.Host
}

func (u *URLObject) Pathname() string {
	if u.Parsed == nil {
		return ""
	}
	path := u.Parsed.EscapedPath()
	if path == "" {
		return "/"
	}
	return path
}

func (u *URLObject) Search() string {
	if u.Parsed == nil || u.Parsed.RawQuery == "" {
		return ""
	}
	return "?" + u.Parsed.RawQuery
}

func (u *URLObject) Hash() string {
	if u.Parsed == nil || u.Parsed.Fragment == "" {
		return ""
	}
	return "#" + u.Parsed.Fragment
}

func (u *URLObject) Username() string {
	if u.Parsed == nil || u.Parsed.User == nil {
		return ""
	}
	return u.Parsed.User.Username()
}

func (u *URLObject) Password() string {
	if u.Parsed == nil || u.Parsed.User == nil {
		return ""
	}
	password, _ := u.Parsed.User.Password()
	return password
}

func (u *URLObject) Protocol() string {
	if u.Parsed == nil || u.Parsed.Scheme == "" {
		return ""
	}
	return u.Parsed.Scheme + ":"
}

func (u *URLObject) Hostname() string {
	if u.Parsed == nil {
		return ""
	}
	return u.Parsed.Hostname()
}

func (u *URLObject) Host() string {
	if u.Parsed == nil {
		return ""
	}
	return u.Parsed.Host
}

func (u *URLObject) Port() string {
	if u.Parsed == nil {
		return ""
	}
	return u.Parsed.Port()
}

func (u *URLObject) SearchParams() *URLSearchParamsObject {
	values := url.Values{}
	if u.Parsed != nil {
		values = u.Parsed.Query()
	}
	params := &URLSearchParamsObject{Values: values}
	params.Members = map[string]ObjectValueEntry{}
	params.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	for _, item := range []struct {
		name string
		kind URLSearchParamsMethodKind
	}{
		{"get", URL_SEARCH_PARAMS_GET},
		{"getAll", URL_SEARCH_PARAMS_GET_ALL},
		{"has", URL_SEARCH_PARAMS_HAS},
		{"set", URL_SEARCH_PARAMS_SET},
		{"append", URL_SEARCH_PARAMS_APPEND},
		{"delete", URL_SEARCH_PARAMS_DELETE},
		{"toString", URL_SEARCH_PARAMS_TO_STRING},
	} {
		method := &URLSearchParamsMethod{Kind: item.kind}
		method.name = item.name
		params.SetMember(value.EncodeHandle(heap.Allocate(NewLightString(item.name))), value.EncodeHandle(heap.Allocate(method)))
	}
	return params
}

func (u *URLObject) SetProperty(name string, v value.Value) bool {
	if u.Parsed == nil {
		return false
	}
	text := stringArgument(v)
	switch name {
	case "href":
		parsed, err := ParseURL(text, "")
		if err != nil {
			return true
		}
		u.Parsed = parsed
	case "protocol":
		u.Parsed.Scheme = strings.TrimSuffix(text, ":")
	case "username":
		u.setUser(text, u.Password())
	case "password":
		u.setUser(u.Username(), text)
	case "hostname":
		port := u.Parsed.Port()
		if port != "" {
			u.Parsed.Host = text + ":" + port
		} else {
			u.Parsed.Host = text
		}
	case "host":
		u.Parsed.Host = text
	case "port":
		host := u.Parsed.Hostname()
		if text == "" {
			u.Parsed.Host = host
		} else {
			u.Parsed.Host = host + ":" + text
		}
	case "pathname":
		u.Parsed.Path = text
		u.Parsed.RawPath = ""
	case "search":
		u.Parsed.RawQuery = strings.TrimPrefix(text, "?")
	case "hash":
		u.Parsed.Fragment = strings.TrimPrefix(text, "#")
	default:
		return false
	}
	return true
}

func (u *URLObject) setUser(username, password string) {
	if username == "" && password == "" {
		u.Parsed.User = nil
		return
	}
	if password == "" {
		u.Parsed.User = url.User(username)
		return
	}
	u.Parsed.User = url.UserPassword(username, password)
}

func (*URLConstructor) New(params ...any) (object.Object, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("URL requires a value")
	}
	input, ok := params[0].(value.Value)
	if !ok {
		return nil, fmt.Errorf("URL requires a string")
	}
	base := ""
	if len(params) > 1 {
		if baseValue, ok := params[1].(value.Value); ok {
			base = stringArgument(baseValue)
		}
	}
	parsed, err := ParseURL(stringArgument(input), base)
	if err != nil {
		return nil, err
	}
	o := &URLObject{Parsed: parsed}
	o.Members = map[string]ObjectValueEntry{}
	o.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	toString := value.EncodeHandle(heap.Allocate(NewURLToString()))
	o.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("toString"))), toString)
	o.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("toJSON"))), toString)
	return o, nil
}

type URLSearchParamsConstructor struct {
	ObjObject
}

func (*URLSearchParamsConstructor) Type() object.ObjType { return object.OBJ_NATIVE_FN }
func (*URLSearchParamsConstructor) String() string       { return "function URLSearchParams" }

func (*URLSearchParamsConstructor) New(params ...any) (object.Object, error) {
	values := url.Values{}
	if len(params) > 0 {
		if source, ok := params[0].(value.Value); ok && source != value.UNDEFINED && source != value.NULL {
			if text, ok := ObjectStringValue(source); ok {
				values = url.Values{}
				parsed, err := url.ParseQuery(strings.TrimPrefix(text, "?"))
				if err == nil {
					values = parsed
				}
			} else if source.IsObject() {
				if obj, err := heap.GetObject(source.GetHandle()); err == nil {
					if other, ok := obj.(*URLSearchParamsObject); ok {
						for key, entries := range other.Values {
							values[key] = append([]string{}, entries...)
						}
					}
				}
			}
		}
	}
	o := &URLSearchParamsObject{Values: values}
	o.Members = map[string]ObjectValueEntry{}
	o.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	for _, item := range []struct {
		name string
		kind URLSearchParamsMethodKind
	}{
		{"get", URL_SEARCH_PARAMS_GET},
		{"getAll", URL_SEARCH_PARAMS_GET_ALL},
		{"has", URL_SEARCH_PARAMS_HAS},
		{"set", URL_SEARCH_PARAMS_SET},
		{"append", URL_SEARCH_PARAMS_APPEND},
		{"delete", URL_SEARCH_PARAMS_DELETE},
		{"toString", URL_SEARCH_PARAMS_TO_STRING},
	} {
		method := &URLSearchParamsMethod{Kind: item.kind}
		method.name = item.name
		o.SetMember(value.EncodeHandle(heap.Allocate(NewLightString(item.name))), value.EncodeHandle(heap.Allocate(method)))
	}
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
