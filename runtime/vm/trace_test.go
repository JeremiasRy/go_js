package vm

import (
	"math"
	"testing"

	"go_js/value"
)

func TestTraceValueViewPrimitives(t *testing.T) {
	vm := NewVM(false, nil)
	cases := []struct {
		value   value.Value
		kind    string
		display string
	}{
		{value.NULL, "null", "null"},
		{value.UNDEFINED, "undefined", "undefined"},
		{value.TRUE, "boolean", "true"},
		{value.FALSE, "boolean", "false"},
		{value.ValueFromFloat64(42), "number", "42"},
		{value.ValueFromFloat64(math.NaN()), "number", "NaN"},
		{value.ValueFromFloat64(math.Inf(1)), "number", "Infinity"},
		{value.ValueFromFloat64(math.Inf(-1)), "number", "-Infinity"},
		{value.Value(math.Float64bits(math.Copysign(0, -1))), "number", "-0"},
		{value.TAG_METHOD_HANDLE, "method_handle", "<method>"},
		{value.TAG_ARGUMENT_START, "argument_start", "<arguments>"},
		{value.EMPTY_ARRAY_ITEM, "empty", "<empty>"},
	}
	for _, test := range cases {
		view := vm.valueView(test.value)
		if view.Type != test.kind || view.Display != test.display {
			t.Fatalf("value %#x: expected %s %q, got %s %q", test.value, test.kind, test.display, view.Type, view.Display)
		}
	}
}
