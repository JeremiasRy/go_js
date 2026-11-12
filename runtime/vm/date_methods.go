package vm

import (
	"fmt"
	"go_js/heap"
	"go_js/native"
	"go_js/value"
	"math"
	"time"
)

func (vm *VM) dateObject(this value.Value) (*native.DateObject, error) {
	if !this.IsObject() {
		return nil, fmt.Errorf("Date method called on non-object")
	}
	obj, err := heap.GetObject(this.GetHandle())
	if err != nil {
		return nil, err
	}
	date, ok := obj.(*native.DateObject)
	if !ok {
		return nil, fmt.Errorf("Date method called on incompatible receiver")
	}
	return date, nil
}

func (vm *VM) callDateMethod(method *native.DateMethod, this value.Value, argCount int) (value.Value, error) {
	date, err := vm.dateObject(this)
	if err != nil {
		return value.UNDEFINED, err
	}
	args := vm.popN(argCount)
	num := func(i int, fallback float64) float64 {
		if i >= len(args) || args[i] == value.UNDEFINED {
			return fallback
		}
		n, err := vm.toNumber(args[i])
		if err != nil {
			return math.NaN()
		}
		return n
	}
	invalid := math.IsNaN(date.Millis)
	t := time.UnixMilli(int64(date.Millis)).UTC()
	if invalid {
		t = time.Time{}
	}
	nan := value.STANDARD_NAN
	switch method.Kind {
	case native.DATE_GET_TIME:
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_GET_UTC_FULL_YEAR:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(t.Year())), nil
	case native.DATE_GET_UTC_MONTH:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(int(t.Month()) - 1)), nil
	case native.DATE_GET_UTC_DATE:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(t.Day())), nil
	case native.DATE_GET_UTC_DAY:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(int(t.Weekday()))), nil
	case native.DATE_GET_UTC_HOURS:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(t.Hour())), nil
	case native.DATE_GET_UTC_MINUTES:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(t.Minute())), nil
	case native.DATE_GET_UTC_SECONDS:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(t.Second())), nil
	case native.DATE_GET_UTC_MILLISECONDS:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(t.Nanosecond() / int(time.Millisecond))), nil
	case native.DATE_SET_TIME:
		date.Millis = native.TimeClip(num(0, math.NaN()))
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_SET_UTC_FULL_YEAR:
		if invalid {
			date.Millis = math.NaN()
			return nan, nil
		}
		year := int(num(0, float64(t.Year())))
		month := int(num(1, float64(int(t.Month())-1))) + 1
		day := int(num(2, float64(t.Day())))
		date.Millis = native.TimeClip(float64(time.Date(year, time.Month(month), day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC).UnixMilli()))
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_SET_UTC_MONTH:
		if invalid {
			date.Millis = math.NaN()
			return nan, nil
		}
		month := int(num(0, float64(int(t.Month())-1))) + 1
		day := int(num(1, float64(t.Day())))
		date.Millis = native.TimeClip(float64(time.Date(t.Year(), time.Month(month), day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC).UnixMilli()))
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_SET_UTC_DATE:
		if invalid {
			date.Millis = math.NaN()
			return nan, nil
		}
		day := int(num(0, float64(t.Day())))
		date.Millis = native.TimeClip(float64(time.Date(t.Year(), t.Month(), day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC).UnixMilli()))
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_SET_UTC_HOURS:
		if invalid {
			date.Millis = math.NaN()
			return nan, nil
		}
		hour := int(num(0, float64(t.Hour())))
		minute := int(num(1, float64(t.Minute())))
		second := int(num(2, float64(t.Second())))
		ms := int(num(3, float64(t.Nanosecond()/int(time.Millisecond))))
		date.Millis = native.TimeClip(float64(time.Date(t.Year(), t.Month(), t.Day(), hour, minute, second, ms*int(time.Millisecond), time.UTC).UnixMilli()))
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_SET_UTC_MINUTES:
		if invalid {
			date.Millis = math.NaN()
			return nan, nil
		}
		minute := int(num(0, float64(t.Minute())))
		second := int(num(1, float64(t.Second())))
		ms := int(num(2, float64(t.Nanosecond()/int(time.Millisecond))))
		date.Millis = native.TimeClip(float64(time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), minute, second, ms*int(time.Millisecond), time.UTC).UnixMilli()))
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_SET_UTC_SECONDS:
		if invalid {
			date.Millis = math.NaN()
			return nan, nil
		}
		second := int(num(0, float64(t.Second())))
		ms := int(num(1, float64(t.Nanosecond()/int(time.Millisecond))))
		date.Millis = native.TimeClip(float64(time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), second, ms*int(time.Millisecond), time.UTC).UnixMilli()))
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_SET_UTC_MILLISECONDS:
		if invalid {
			date.Millis = math.NaN()
			return nan, nil
		}
		ms := int(num(0, float64(t.Nanosecond()/int(time.Millisecond))))
		date.Millis = native.TimeClip(float64(time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), ms*int(time.Millisecond), time.UTC).UnixMilli()))
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_TO_ISO_STRING:
		if invalid {
			return value.UNDEFINED, fmt.Errorf("Invalid time value")
		}
		return value.EncodeHandle(heap.Allocate(native.NewLightString(time.UnixMilli(int64(date.Millis)).UTC().Format("2006-01-02T15:04:05.000Z")))), nil
	case native.DATE_TO_STRING:
		return value.EncodeHandle(heap.Allocate(native.NewLightString(date.String()))), nil
	case native.DATE_TO_JSON:
		if invalid {
			return value.NULL, nil
		}
		return value.EncodeHandle(heap.Allocate(native.NewLightString(time.UnixMilli(int64(date.Millis)).UTC().Format("2006-01-02T15:04:05.000Z")))), nil
	case native.DATE_GET_FULL_YEAR:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(t.Year())), nil
	case native.DATE_GET_MONTH:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(int(t.Month()) - 1)), nil
	case native.DATE_GET_DATE:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(t.Day())), nil
	case native.DATE_GET_DAY:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(int(t.Weekday()))), nil
	case native.DATE_GET_HOURS:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(t.Hour())), nil
	case native.DATE_GET_MINUTES:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(t.Minute())), nil
	case native.DATE_GET_SECONDS:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(t.Second())), nil
	case native.DATE_GET_MILLISECONDS:
		if invalid {
			return nan, nil
		}
		return value.ValueFromFloat64(float64(t.Nanosecond() / int(time.Millisecond))), nil
	case native.DATE_GET_TIMEZONE_OFFSET:
		_, offset := time.UnixMilli(int64(date.Millis)).Zone()
		return value.ValueFromFloat64(float64(-offset / 60)), nil
	case native.DATE_SET_FULL_YEAR:
		if invalid {
			date.Millis = math.NaN()
			return nan, nil
		}
		year := int(num(0, float64(t.Year())))
		month := t.Month()
		day := t.Day()
		if len(args) > 1 {
			month = time.Month(int(num(1, float64(int(t.Month())-1))) + 1)
		}
		if len(args) > 2 {
			day = int(num(2, float64(t.Day())))
		}
		date.Millis = native.TimeClip(float64(time.Date(year, month, day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC).UnixMilli()))
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_SET_MONTH:
		if invalid {
			date.Millis = math.NaN()
			return nan, nil
		}
		month := time.Month(int(num(0, float64(int(t.Month())-1))) + 1)
		day := t.Day()
		if len(args) > 1 {
			day = int(num(1, float64(t.Day())))
		}
		date.Millis = native.TimeClip(float64(time.Date(t.Year(), month, day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC).UnixMilli()))
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_SET_DATE:
		if invalid {
			date.Millis = math.NaN()
			return nan, nil
		}
		day := int(num(0, float64(t.Day())))
		date.Millis = native.TimeClip(float64(time.Date(t.Year(), t.Month(), day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC).UnixMilli()))
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_SET_HOURS:
		if invalid {
			date.Millis = math.NaN()
			return nan, nil
		}
		hour := int(num(0, float64(t.Hour())))
		minute := t.Minute()
		second := t.Second()
		ms := t.Nanosecond() / int(time.Millisecond)
		if len(args) > 1 {
			minute = int(num(1, float64(t.Minute())))
		}
		if len(args) > 2 {
			second = int(num(2, float64(t.Second())))
		}
		if len(args) > 3 {
			ms = int(num(3, float64(t.Nanosecond()/int(time.Millisecond))))
		}
		date.Millis = native.TimeClip(float64(time.Date(t.Year(), t.Month(), t.Day(), hour, minute, second, ms*int(time.Millisecond), time.UTC).UnixMilli()))
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_SET_MINUTES:
		if invalid {
			date.Millis = math.NaN()
			return nan, nil
		}
		minute := int(num(0, float64(t.Minute())))
		second := t.Second()
		ms := t.Nanosecond() / int(time.Millisecond)
		if len(args) > 1 {
			second = int(num(1, float64(t.Second())))
		}
		if len(args) > 2 {
			ms = int(num(2, float64(t.Nanosecond()/int(time.Millisecond))))
		}
		date.Millis = native.TimeClip(float64(time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), minute, second, ms*int(time.Millisecond), time.UTC).UnixMilli()))
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_SET_SECONDS:
		if invalid {
			date.Millis = math.NaN()
			return nan, nil
		}
		second := int(num(0, float64(t.Second())))
		ms := t.Nanosecond() / int(time.Millisecond)
		if len(args) > 1 {
			ms = int(num(1, float64(t.Nanosecond()/int(time.Millisecond))))
		}
		date.Millis = native.TimeClip(float64(time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), second, ms*int(time.Millisecond), time.UTC).UnixMilli()))
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_SET_MILLISECONDS:
		if invalid {
			date.Millis = math.NaN()
			return nan, nil
		}
		ms := int(num(0, float64(t.Nanosecond()/int(time.Millisecond))))
		date.Millis = native.TimeClip(float64(time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), ms*int(time.Millisecond), time.UTC).UnixMilli()))
		return value.ValueFromFloat64(date.Millis), nil
	case native.DATE_TO_UTC_STRING:
		if invalid {
			return value.EncodeHandle(heap.Allocate(native.NewLightString("Invalid Date"))), nil
		}
		return value.EncodeHandle(heap.Allocate(native.NewLightString(time.UnixMilli(int64(date.Millis)).UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")))), nil
	case native.DATE_TO_DATE_STRING:
		if invalid {
			return value.EncodeHandle(heap.Allocate(native.NewLightString("Invalid Date"))), nil
		}
		return value.EncodeHandle(heap.Allocate(native.NewLightString(time.UnixMilli(int64(date.Millis)).UTC().Format("Mon Jan 02 2006")))), nil
	case native.DATE_TO_TIME_STRING:
		if invalid {
			return value.EncodeHandle(heap.Allocate(native.NewLightString("Invalid Date"))), nil
		}
		return value.EncodeHandle(heap.Allocate(native.NewLightString(time.UnixMilli(int64(date.Millis)).UTC().Format("15:04:05 GMT+0000 (Coordinated Universal Time)")))), nil
	case native.DATE_TO_LOCALE_STRING:
		if invalid {
			return value.EncodeHandle(heap.Allocate(native.NewLightString("Invalid Date"))), nil
		}
		local := time.UnixMilli(int64(date.Millis)).UTC()
		hour := local.Hour() % 12
		if hour == 0 {
			hour = 12
		}
		period := "AM"
		if local.Hour() >= 12 {
			period = "PM"
		}
		text := fmt.Sprintf("%d/%d/%d, %d:%s:%s %s", int(local.Month()), local.Day(), local.Year(), hour, twoDigits(local.Minute()), twoDigits(local.Second()), period)
		return value.EncodeHandle(heap.Allocate(native.NewLightString(text))), nil
	}
	return value.UNDEFINED, nil
}

func (vm *VM) callDateStaticMethod(method *native.DateStaticMethod, argCount int) (value.Value, error) {
	args := vm.popN(argCount)
	num := func(i int, fallback float64) float64 {
		if i >= len(args) || args[i] == value.UNDEFINED {
			return fallback
		}
		n, err := vm.toNumber(args[i])
		if err != nil {
			return math.NaN()
		}
		return n
	}
	if method.Kind == native.DATE_STATIC_PARSE {
		if len(args) == 0 {
			return value.STANDARD_NAN, nil
		}
		s, err := vm.toStringValue(args[0])
		if err != nil {
			return value.STANDARD_NAN, nil
		}
		str, _ := vm.valueAsString(s)
		return value.ValueFromFloat64(native.ParseDateString(str)), nil
	}
	yearValue := num(0, math.NaN())
	if math.IsNaN(yearValue) {
		return value.STANDARD_NAN, nil
	}
	year := int(yearValue)
	if year >= 0 && year <= 99 {
		year += 1900
	}
	monthValue := num(1, 0)
	dayValue := num(2, 1)
	hourValue := num(3, 0)
	minuteValue := num(4, 0)
	secondValue := num(5, 0)
	msValue := num(6, 0)
	if math.IsNaN(monthValue) || math.IsNaN(dayValue) || math.IsNaN(hourValue) || math.IsNaN(minuteValue) || math.IsNaN(secondValue) || math.IsNaN(msValue) {
		return value.STANDARD_NAN, nil
	}
	month := int(monthValue) + 1
	day := int(dayValue)
	hour := int(hourValue)
	minute := int(minuteValue)
	second := int(secondValue)
	ms := int(msValue)
	t := time.Date(year, time.Month(month), day, hour, minute, second, ms*int(time.Millisecond), time.UTC)
	return value.ValueFromFloat64(native.TimeClip(float64(t.UnixMilli()))), nil
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
