package vm

import (
	"fmt"
	"go_js/heap"
	"go_js/native"
	"go_js/value"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var parseFloatPattern = regexp.MustCompile(`^[+-]?(?:Infinity|(?:(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?))`)

func parseIntString(s string, radix int) value.Value {
	s = strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == '\uFEFF' })
	sign := 1.0
	if strings.HasPrefix(s, "+") {
		s = s[1:]
	} else if strings.HasPrefix(s, "-") {
		sign = -1
		s = s[1:]
	}
	if radix == 0 {
		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			radix = 16
			s = s[2:]
		} else {
			radix = 10
		}
	} else if radix == 16 && (strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X")) {
		s = s[2:]
	}
	if radix < 2 || radix > 36 {
		return value.STANDARD_NAN
	}
	digits := ""
	for _, r := range s {
		d := -1
		switch {
		case r >= '0' && r <= '9':
			d = int(r - '0')
		case r >= 'a' && r <= 'z':
			d = int(r-'a') + 10
		case r >= 'A' && r <= 'Z':
			d = int(r-'A') + 10
		}
		if d < 0 || d >= radix {
			break
		}
		digits += string(r)
	}
	if digits == "" {
		return value.STANDARD_NAN
	}
	n, err := strconv.ParseUint(digits, radix, 64)
	if err != nil {
		f, _ := strconv.ParseFloat(digits, 64)
		return value.ValueFromFloat64(math.Copysign(f, sign))
	}
	result := float64(n) * sign
	if result == 0 && sign < 0 {
		result = math.Copysign(0, -1)
	}
	return value.ValueFromFloat64(result)
}

func parseFloatString(s string) value.Value {
	s = strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == '\uFEFF' })
	match := parseFloatPattern.FindString(s)
	if match == "" {
		return value.STANDARD_NAN
	}
	if match == "Infinity" || match == "+Infinity" {
		return value.ValueFromFloat64(math.Inf(1))
	}
	if match == "-Infinity" {
		return value.ValueFromFloat64(math.Inf(-1))
	}
	n, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return value.STANDARD_NAN
	}
	return value.ValueFromFloat64(n)
}

func (vm *VM) numberValue(v value.Value) (float64, error) {
	return vm.toNumber(v)
}

func (vm *VM) callNumberFormatMethod(method *native.NumberFormatMethod, this value.Value, argCount int) (value.Value, error) {
	args := vm.popN(argCount)
	n, err := vm.numberValue(this)
	if err != nil {
		return value.UNDEFINED, err
	}
	radix := 10
	switch method.Kind {
	case native.NUMBER_TO_FIXED:
		digits := 0
		if len(args) > 0 {
			digits = vm.arrayInteger(args[0], 0)
		}
		if digits < 0 || digits > 100 {
			return value.UNDEFINED, fmt.Errorf("toFixed() digits argument must be between 0 and 100")
		}
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return value.EncodeHandle(heap.Allocate(native.NewLightString(nativeNumberString(n)))), nil
		}
		if n == 0 {
			n = 0
		}
		scale := math.Pow10(digits)
		rounded := math.Round(n*scale) / scale
		return value.EncodeHandle(heap.Allocate(native.NewLightString(strconv.FormatFloat(rounded, 'f', digits, 64)))), nil
	case native.NUMBER_TO_PRECISION:
		if len(args) == 0 || args[0] == value.UNDEFINED {
			return value.EncodeHandle(heap.Allocate(native.NewLightString(nativeNumberString(n)))), nil
		}
		precision := vm.arrayInteger(args[0], 0)
		if precision < 1 || precision > 100 {
			return value.UNDEFINED, fmt.Errorf("toPrecision() precision argument must be between 1 and 100")
		}
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return value.EncodeHandle(heap.Allocate(native.NewLightString(nativeNumberString(n)))), nil
		}
		return value.EncodeHandle(heap.Allocate(native.NewLightString(strconv.FormatFloat(n, 'g', precision, 64)))), nil
	case native.NUMBER_TO_EXPONENTIAL:
		digits := -1
		if len(args) > 0 && args[0] != value.UNDEFINED {
			digits = vm.arrayInteger(args[0], 0)
		}
		if digits < 0 || digits > 100 {
			return value.UNDEFINED, fmt.Errorf("toExponential() argument must be between 0 and 100")
		}
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return value.EncodeHandle(heap.Allocate(native.NewLightString(nativeNumberString(n)))), nil
		}
		s := strconv.FormatFloat(n, 'e', digits, 64)
		if idx := strings.IndexByte(s, 'e'); idx >= 0 {
			mantissa, exponent := s[:idx], s[idx+1:]
			sign := ""
			if len(exponent) > 0 && (exponent[0] == '+' || exponent[0] == '-') {
				sign = string(exponent[0])
				exponent = exponent[1:]
			}
			exponent = strings.TrimLeft(exponent, "0")
			if exponent == "" {
				exponent = "0"
			}
			s = mantissa + "e" + sign + exponent
		}
		return value.EncodeHandle(heap.Allocate(native.NewLightString(s))), nil
	case native.NUMBER_TO_STRING:
		if len(args) > 0 && args[0] != value.UNDEFINED {
			radix = vm.arrayInteger(args[0], 10)
			if radix < 2 || radix > 36 {
				return value.UNDEFINED, fmt.Errorf("toString() radix must be between 2 and 36")
			}
		}
		return value.EncodeHandle(heap.Allocate(native.NewLightString(native.NumberToString(n, radix)))), nil
	}
	return value.UNDEFINED, nil
}

func nativeNumberString(n float64) string {
	if math.IsNaN(n) {
		return "NaN"
	}
	if math.IsInf(n, 1) {
		return "Infinity"
	}
	if math.IsInf(n, -1) {
		return "-Infinity"
	}
	if n == 0 {
		return "0"
	}
	abs := math.Abs(n)
	if abs >= 1e-6 && abs < 1e21 {
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	return strings.ReplaceAll(strconv.FormatFloat(n, 'e', -1, 64), "e+0", "e+")
}

func nativeParseNumberString(s string) (float64, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(s, "\uFEFF"))
	if trimmed == "" {
		return 0, nil
	}
	switch trimmed {
	case "Infinity", "+Infinity":
		return math.Inf(1), nil
	case "-Infinity":
		return math.Inf(-1), nil
	}
	if strings.HasPrefix(trimmed, "0x") || strings.HasPrefix(trimmed, "0X") {
		n, err := strconv.ParseUint(trimmed[2:], 16, 64)
		if err != nil {
			return math.NaN(), nil
		}
		return float64(n), nil
	}
	n, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return math.NaN(), nil
	}
	return n, nil
}
