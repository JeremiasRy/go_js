package vm

import (
	"go_js/native"
	"go_js/value"
	"math"
	"math/bits"
	"math/rand"
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
	case native.MATH_ATAN2:
		return value.ValueFromFloat64(math.Atan2(number(0), number(1))), nil
	case native.MATH_HYPOT:
		result := 0.0
		hasNaN := false
		for i := range args {
			n := math.Abs(number(i))
			if math.IsInf(n, 1) {
				return value.ValueFromFloat64(math.Inf(1)), nil
			}
			if math.IsNaN(n) {
				hasNaN = true
			}
			result += n * n
		}
		if hasNaN {
			return value.STANDARD_NAN, nil
		}
		return value.ValueFromFloat64(math.Sqrt(result)), nil
	case native.MATH_CBRT:
		return value.ValueFromFloat64(math.Cbrt(number(0))), nil
	case native.MATH_LOG2:
		return value.ValueFromFloat64(math.Log2(number(0))), nil
	case native.MATH_LOG10:
		return value.ValueFromFloat64(math.Log10(number(0))), nil
	case native.MATH_LOG1P:
		return value.ValueFromFloat64(math.Log1p(number(0))), nil
	case native.MATH_EXPM1:
		return value.ValueFromFloat64(math.Expm1(number(0))), nil
	case native.MATH_SINH:
		return value.ValueFromFloat64(math.Sinh(number(0))), nil
	case native.MATH_COSH:
		return value.ValueFromFloat64(math.Cosh(number(0))), nil
	case native.MATH_TANH:
		return value.ValueFromFloat64(math.Tanh(number(0))), nil
	case native.MATH_ASINH:
		return value.ValueFromFloat64(math.Asinh(number(0))), nil
	case native.MATH_ACOSH:
		return value.ValueFromFloat64(math.Acosh(number(0))), nil
	case native.MATH_ATANH:
		return value.ValueFromFloat64(math.Atanh(number(0))), nil
	case native.MATH_IMUL:
		a := int32(toUint32(number(0)))
		b := int32(toUint32(number(1)))
		return value.ValueFromFloat64(float64(a * b)), nil
	case native.MATH_CLZ32:
		n := toUint32(number(0))
		return value.ValueFromFloat64(float64(bits.LeadingZeros32(n))), nil
	case native.MATH_FROUND:
		return value.ValueFromFloat64(float64(float32(number(0)))), nil
	case native.MATH_RANDOM:
		return value.ValueFromFloat64(rand.Float64()), nil
	}
	return value.UNDEFINED, nil
}

func toUint32(n float64) uint32 {
	if math.IsNaN(n) || math.IsInf(n, 0) || n == 0 {
		return 0
	}
	return uint32(int64(math.Trunc(n)))
}
