package native

import (
	"fmt"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"math"
	"time"
)

type Now struct {
	ObjNativeFn
}

func NewNow() *Now {
	n := &Now{}
	n.name = "Now"
	return n
}

func (*Now) Now() value.Value {
	return value.ValueFromFloat64(float64(time.Now().UnixMilli()))
}

type DateConstructor struct {
	ObjObject
}

func NewDateConstructor() *DateConstructor {
	dc := &DateConstructor{}
	dc.Members = map[string]ObjectValueEntry{}
	dc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("now"))), value.EncodeHandle(heap.Allocate(NewNow())))
	dc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("parse"))), value.EncodeHandle(heap.Allocate(NewDateStaticMethod("parse", DATE_STATIC_PARSE))))
	dc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("UTC"))), value.EncodeHandle(heap.Allocate(NewDateStaticMethod("UTC", DATE_STATIC_UTC))))
	dc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("prototype"))), PROTOTYPE_DATE)
	return dc
}

func (*DateConstructor) String() string {
	return "function Date"
}

func (*DateConstructor) Type() object.ObjType {
	return object.OBJ_DATE_CONSTRUCTOR
}

func (dc *DateConstructor) New(params ...any) (object.Object, error) {
	date := NewDateObject(math.NaN())
	if len(params) == 0 {
		date.Millis = float64(time.Now().UnixMilli())
		return date, nil
	}
	if len(params) == 1 {
		if v, ok := params[0].(value.Value); ok {
			if v.IsNumber() {
				date.Millis = TimeClip(v.AsNumber())
			} else if str, ok := ObjectStringValue(v); ok {
				date.Millis = ParseDateString(str)
			}
		}
		return date, nil
	}
	values := make([]float64, len(params))
	for i, param := range params {
		if v, ok := param.(value.Value); ok {
			if v.IsNumber() {
				values[i] = v.AsNumber()
			} else {
				values[i] = math.NaN()
			}
		}
	}
	year := int(values[0])
	if year >= 0 && year <= 99 {
		year += 1900
	}
	month := 0
	day := 1
	hour, minute, second, ms := 0, 0, 0, 0
	if len(values) > 1 {
		month = int(values[1])
	}
	if len(values) > 2 {
		day = int(values[2])
	}
	if len(values) > 3 {
		hour = int(values[3])
	}
	if len(values) > 4 {
		minute = int(values[4])
	}
	if len(values) > 5 {
		second = int(values[5])
	}
	if len(values) > 6 {
		ms = int(values[6])
	}
	t := time.Date(year, time.Month(month+1), day, hour, minute, second, ms*int(time.Millisecond), time.UTC)
	date.Millis = TimeClip(float64(t.UnixMilli()))
	return date, nil
}

type DateObject struct {
	ObjObject
	Millis float64
}

func NewDateObject(millis float64) *DateObject {
	d := &DateObject{Millis: millis}
	d.Members = map[string]ObjectValueEntry{}
	d.SetMember(KEY_PROTO, PROTOTYPE_DATE)
	return d
}

func (*DateObject) Type() object.ObjType {
	return object.OBJ_DATE
}

func (d *DateObject) String() string {
	if math.IsNaN(d.Millis) {
		return "Invalid Date"
	}
	return time.UnixMilli(int64(d.Millis)).UTC().Format("Mon Jan 02 2006 15:04:05 GMT+0000 (Coordinated Universal Time)")
}

func TimeClip(n float64) float64 {
	if math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > 8640000000000000 {
		return math.NaN()
	}
	return math.Trunc(n)
}

func ParseDateString(str string) float64 {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999Z07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, str); err == nil {
			return TimeClip(float64(t.UnixMilli()))
		}
	}
	return math.NaN()
}

type DateMethodKind int

const (
	DATE_GET_TIME DateMethodKind = iota
	DATE_GET_UTC_FULL_YEAR
	DATE_GET_UTC_MONTH
	DATE_GET_UTC_DATE
	DATE_GET_UTC_DAY
	DATE_GET_UTC_HOURS
	DATE_GET_UTC_MINUTES
	DATE_GET_UTC_SECONDS
	DATE_GET_UTC_MILLISECONDS
	DATE_SET_TIME
	DATE_SET_UTC_FULL_YEAR
	DATE_SET_UTC_MONTH
	DATE_SET_UTC_DATE
	DATE_SET_UTC_HOURS
	DATE_SET_UTC_MINUTES
	DATE_SET_UTC_SECONDS
	DATE_SET_UTC_MILLISECONDS
	DATE_TO_ISO_STRING
	DATE_TO_STRING
	DATE_TO_JSON
)

type DateMethod struct {
	ObjNativeFn
	Kind DateMethodKind
}

func NewDateMethod(name string, kind DateMethodKind) *DateMethod {
	m := &DateMethod{Kind: kind}
	m.name = name
	return m
}

type DateStaticKind int

const (
	DATE_STATIC_PARSE DateStaticKind = iota
	DATE_STATIC_UTC
)

type DateStaticMethod struct {
	ObjNativeFn
	Kind DateStaticKind
}

func NewDateStaticMethod(name string, kind DateStaticKind) *DateStaticMethod {
	m := &DateStaticMethod{Kind: kind}
	m.name = name
	return m
}

func InitDatePrototype() {
	p := &Prototype{name: DATE_CONSTRUCTOR_NAME}
	p.Members = map[string]ObjectValueEntry{}
	p.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	methods := []struct {
		name string
		kind DateMethodKind
	}{
		{"getTime", DATE_GET_TIME},
		{"getUTCFullYear", DATE_GET_UTC_FULL_YEAR},
		{"getUTCMonth", DATE_GET_UTC_MONTH},
		{"getUTCDate", DATE_GET_UTC_DATE},
		{"getUTCDay", DATE_GET_UTC_DAY},
		{"getUTCHours", DATE_GET_UTC_HOURS},
		{"getUTCMinutes", DATE_GET_UTC_MINUTES},
		{"getUTCSeconds", DATE_GET_UTC_SECONDS},
		{"getUTCMilliseconds", DATE_GET_UTC_MILLISECONDS},
		{"setTime", DATE_SET_TIME},
		{"setUTCFullYear", DATE_SET_UTC_FULL_YEAR},
		{"setUTCMonth", DATE_SET_UTC_MONTH},
		{"setUTCDate", DATE_SET_UTC_DATE},
		{"setUTCHours", DATE_SET_UTC_HOURS},
		{"setUTCMinutes", DATE_SET_UTC_MINUTES},
		{"setUTCSeconds", DATE_SET_UTC_SECONDS},
		{"setUTCMilliseconds", DATE_SET_UTC_MILLISECONDS},
		{"toISOString", DATE_TO_ISO_STRING},
		{"toString", DATE_TO_STRING},
		{"toJSON", DATE_TO_JSON},
	}
	for _, method := range methods {
		key := value.EncodeHandle(heap.Allocate(NewLightString(method.name)))
		p.SetMember(key, value.EncodeHandle(heap.Allocate(NewDateMethod(method.name, method.kind))))
		heap.PushToRoots(key)
	}
	PROTOTYPE_DATE = value.EncodeHandle(heap.Allocate(p))
	heap.PushToRoots(PROTOTYPE_DATE)
}

var _ = fmt.Sprintf
