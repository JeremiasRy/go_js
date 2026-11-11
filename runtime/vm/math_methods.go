package vm

import (
	"go_js/native"
	"go_js/value"
	"math"
)

func (vm *VM) callMathMethod(method *native.MathMethod, argCount int) (value.Value, error) {
	args := vm.popN(argCount)
	number := func(i int) float64 {
		if i >= len(args) {
			return math.NaN()
		}
		n, err := vm.toNumber(args[i])
		if err != nil {
			return math.NaN()
		}
		return n
	}
	switch method.Kind {
	case native.MATH_FLOOR:
		return value.ValueFromFloat64(math.Floor(number(0))), nil
	case native.MATH_CEIL:
		n := number(0)
		result := math.Ceil(n)
		if result == 0 && n < 0 {
			result = math.Copysign(0, -1)
		}
		return value.ValueFromFloat64(result), nil
	case native.MATH_TRUNC:
		n := number(0)
		result := math.Trunc(n)
		if result == 0 && math.Signbit(n) {
			result = math.Copysign(0, -1)
		}
		return value.ValueFromFloat64(result), nil
	case native.MATH_ROUND:
		n := number(0)
		result := math.Floor(n + 0.5)
		if result == 0 && math.Signbit(n) {
			result = math.Copysign(0, -1)
		}
		return value.ValueFromFloat64(result), nil
	case native.MATH_ABS:
		return value.ValueFromFloat64(math.Abs(number(0))), nil
	case native.MATH_SIGN:
		n := number(0)
		if math.IsNaN(n) || n == 0 {
			return value.ValueFromFloat64(n), nil
		}
		if n < 0 {
			return value.ValueFromFloat64(-1), nil
		}
		return value.ValueFromFloat64(1), nil
	case native.MATH_MIN, native.MATH_MAX:
		if len(args) == 0 {
			if method.Kind == native.MATH_MIN {
				return value.ValueFromFloat64(math.Inf(1)), nil
			}
			return value.ValueFromFloat64(math.Inf(-1)), nil
		}
		result := number(0)
		for i := 1; i < len(args); i++ {
			n := number(i)
			if math.IsNaN(n) {
				result = n
				break
			}
			if method.Kind == native.MATH_MIN {
				if n < result || (n == 0 && result == 0 && math.Signbit(n)) {
					result = n
				}
			} else {
				if n > result || (n == 0 && result == 0 && !math.Signbit(n)) {
					result = n
				}
			}
		}
		return value.ValueFromFloat64(result), nil
	case native.MATH_POW:
		base, exponent := number(0), number(1)
		if math.IsInf(exponent, 0) && math.Abs(base) == 1 {
			return value.ValueFromFloat64(math.NaN()), nil
		}
		return value.ValueFromFloat64(math.Pow(base, exponent)), nil
	case native.MATH_SQRT:
		return value.ValueFromFloat64(math.Sqrt(number(0))), nil
	}
	return value.UNDEFINED, nil
}
