package vm

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/value"
)

func bigIntFromValue(v value.Value) (*big.Int, bool) {
	if !v.IsObject() {
		return nil, false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return nil, false
	}
	b, ok := obj.(*native.ObjBigInt)
	if !ok {
		return nil, false
	}
	return b.Value, true
}

func (vm *VM) compareBigInt(b *big.Int, other value.Value, bigOnLeft bool) (int, error) {
	if ob, ok := bigIntFromValue(other); ok {
		return b.Cmp(ob), nil
	}
	if s, ok := vm.valueAsString(other); ok {
		text := strings.TrimSpace(s)
		if text == "" {
			text = "0"
		}
		parsed, ok := new(big.Int).SetString(text, 0)
		if !ok {
			return 2, nil
		}
		return b.Cmp(parsed), nil
	}
	n, err := vm.toNumber(other)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(n) {
		return 2, nil
	}
	if math.IsInf(n, 1) {
		if bigOnLeft {
			return -1, nil
		}
		return 1, nil
	}
	if math.IsInf(n, -1) {
		if bigOnLeft {
			return 1, nil
		}
		return -1, nil
	}
	bf := new(big.Float).SetInt(b)
	of := big.NewFloat(n)
	return bf.Cmp(of), nil
}

func (vm *VM) bigIntObject(v value.Value) (*native.ObjBigInt, bool) {
	if !v.IsObject() {
		return nil, false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return nil, false
	}
	b, ok := obj.(*native.ObjBigInt)
	return b, ok
}

func (vm *VM) toBigInt(v value.Value) (*big.Int, error) {
	if b, ok := vm.bigIntObject(v); ok {
		return new(big.Int).Set(b.Value), nil
	}
	if v.IsBoolean() {
		if truthyValue(v) {
			return big.NewInt(1), nil
		}
		return big.NewInt(0), nil
	}
	if v.IsNumber() {
		n := v.AsNumber()
		if math.IsNaN(n) || math.IsInf(n, 0) || math.Trunc(n) != n {
			return nil, fmt.Errorf("The number %v cannot be converted to a BigInt", n)
		}
		result, _ := new(big.Int).SetString(fmt.Sprintf("%.0f", n), 10)
		if result == nil {
			result = big.NewInt(0)
		}
		return result, nil
	}
	if s, ok := vm.valueAsString(v); ok {
		text := strings.TrimSpace(s)
		if text == "" {
			return big.NewInt(0), nil
		}
		if strings.ContainsAny(text, ".eE") {
			return nil, fmt.Errorf("Cannot convert %s to a BigInt", text)
		}
		result, ok := new(big.Int).SetString(text, 0)
		if !ok {
			return nil, fmt.Errorf("Cannot convert %s to a BigInt", text)
		}
		return result, nil
	}
	return nil, fmt.Errorf("Cannot convert %s to a BigInt", native.String(v))
}

func newBigIntValue(v *big.Int) value.Value {
	return value.EncodeHandle(heap.Allocate(native.NewBigInt(v)))
}

func (vm *VM) bigIntToNumber(v value.Value) (float64, bool) {
	b, ok := vm.bigIntObject(v)
	if !ok {
		return 0, false
	}
	f, _ := new(big.Float).SetInt(b.Value).Float64()
	return f, true
}

func (vm *VM) bigIntToStringValue(v value.Value, radix int) value.Value {
	b, ok := vm.bigIntObject(v)
	if !ok {
		return value.UNDEFINED
	}
	if radix < 2 || radix > 36 {
		return value.UNDEFINED
	}
	return value.EncodeHandle(heap.Allocate(native.NewLightString(b.Value.Text(radix))))
}

func (vm *VM) bigIntAsIntN(bits int, v *big.Int, signed bool) *big.Int {
	if bits <= 0 {
		return big.NewInt(0)
	}
	modulus := new(big.Int).Lsh(big.NewInt(1), uint(bits))
	result := new(big.Int).Mod(v, modulus)
	if signed {
		half := new(big.Int).Rsh(modulus, 1)
		if result.Cmp(half) >= 0 {
			result.Sub(result, modulus)
		}
	}
	return result
}

func bigIntBinaryOp(op uint8, a, b *big.Int) (*big.Int, error) {
	switch op {
	case 0:
		return new(big.Int).Add(a, b), nil
	case 1:
		return new(big.Int).Sub(a, b), nil
	case 2:
		return new(big.Int).Mul(a, b), nil
	case 3:
		if b.Sign() == 0 {
			return nil, fmt.Errorf("Division by zero")
		}
		return new(big.Int).Quo(a, b), nil
	case 4:
		if b.Sign() == 0 {
			return nil, fmt.Errorf("Division by zero")
		}
		return new(big.Int).Rem(a, b), nil
	}
	return nil, fmt.Errorf("Unsupported BigInt operation")
}

func (vm *VM) bigIntArithmetic(op int, a, b value.Value) (value.Value, bool, error) {
	ba, aBig := vm.bigIntObject(a)
	bb, bBig := vm.bigIntObject(b)
	if !aBig && !bBig {
		return value.UNDEFINED, false, nil
	}
	if !aBig || !bBig {
		return value.UNDEFINED, true, fmt.Errorf("Cannot mix BigInt and other types")
	}
	switch op {
	case 0:
		return newBigIntValue(new(big.Int).Add(ba.Value, bb.Value)), true, nil
	case 1:
		return newBigIntValue(new(big.Int).Sub(ba.Value, bb.Value)), true, nil
	case 2:
		return newBigIntValue(new(big.Int).Mul(ba.Value, bb.Value)), true, nil
	case 3:
		if bb.Value.Sign() == 0 {
			return value.UNDEFINED, true, fmt.Errorf("Division by zero")
		}
		return newBigIntValue(new(big.Int).Quo(ba.Value, bb.Value)), true, nil
	case 4:
		if bb.Value.Sign() == 0 {
			return value.UNDEFINED, true, fmt.Errorf("Division by zero")
		}
		return newBigIntValue(new(big.Int).Rem(ba.Value, bb.Value)), true, nil
	case 5:
		if bb.Value.Sign() < 0 {
			return value.UNDEFINED, true, fmt.Errorf("Exponent must be positive")
		}
		return newBigIntValue(new(big.Int).Exp(ba.Value, bb.Value, nil)), true, nil
	}
	return value.UNDEFINED, true, fmt.Errorf("Unsupported BigInt operation")
}

func (vm *VM) bigIntBitwise(op int, a, b value.Value) (value.Value, bool, error) {
	ba, aBig := vm.bigIntObject(a)
	bb, bBig := vm.bigIntObject(b)
	if !aBig && !bBig {
		return value.UNDEFINED, false, nil
	}
	if !aBig || !bBig {
		return value.UNDEFINED, true, fmt.Errorf("Cannot mix BigInt and other types")
	}
	switch op {
	case 0:
		return newBigIntValue(new(big.Int).And(ba.Value, bb.Value)), true, nil
	case 1:
		return newBigIntValue(new(big.Int).Or(ba.Value, bb.Value)), true, nil
	case 2:
		return newBigIntValue(new(big.Int).Xor(ba.Value, bb.Value)), true, nil
	case 3:
		if !bb.Value.IsInt64() {
			return value.UNDEFINED, true, fmt.Errorf("BigInt shift count too large")
		}
		shift := bb.Value.Int64()
		if shift < 0 {
			return newBigIntValue(new(big.Int).Rsh(ba.Value, uint(-shift))), true, nil
		}
		return newBigIntValue(new(big.Int).Lsh(ba.Value, uint(shift))), true, nil
	case 4:
		if !bb.Value.IsInt64() {
			return value.UNDEFINED, true, fmt.Errorf("BigInt shift count too large")
		}
		shift := bb.Value.Int64()
		if shift < 0 {
			return newBigIntValue(new(big.Int).Lsh(ba.Value, uint(-shift))), true, nil
		}
		return newBigIntValue(new(big.Int).Rsh(ba.Value, uint(shift))), true, nil
	}
	return value.UNDEFINED, true, fmt.Errorf("Unsupported BigInt operation")
}

func (vm *VM) bigIntValueOfObject(obj object.Object) (*big.Int, bool) {
	if b, ok := obj.(*native.ObjBigInt); ok {
		return b.Value, true
	}
	return nil, false
}
