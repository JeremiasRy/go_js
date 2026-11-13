package jit

import (
	"testing"
	"unsafe"
)

func TestActivationLayout(t *testing.T) {
	a := Activation{}
	check := func(name string, got, want uintptr) {
		t.Helper()
		if got != want {
			t.Fatalf("%s offset = %d, want %d", name, got, want)
		}
	}
	check("Values", unsafe.Offsetof(a.Values), ActivationValues)
	check("Globals", unsafe.Offsetof(a.Globals), ActivationGlobals)
	check("Constants", unsafe.Offsetof(a.Constants), ActivationConstants)
	check("SP", unsafe.Offsetof(a.SP), ActivationSP)
	check("Request", unsafe.Offsetof(a.Request), ActivationRequest)
	check("BytecodeIP", unsafe.Offsetof(a.BytecodeIP), ActivationBytecodeIP)
	check("Status", unsafe.Offsetof(a.Status), ActivationStatus)
	check("Result", unsafe.Offsetof(a.Result), ActivationResult)
	check("Arg0", unsafe.Offsetof(a.Arg0), ActivationArg0)
	check("Arg1", unsafe.Offsetof(a.Arg1), ActivationArg1)
	check("Arg2", unsafe.Offsetof(a.Arg2), ActivationArg2)
	check("Arg3", unsafe.Offsetof(a.Arg3), ActivationArg3)
	check("Depth", unsafe.Offsetof(a.Depth), ActivationDepth)
	check("SelfValue", unsafe.Offsetof(a.SelfValue), ActivationSelfValue)
	check("GoSP", unsafe.Offsetof(a.GoSP), ActivationGoSP)
	check("GlobalsCount", unsafe.Offsetof(a.GlobalsCount), ActivationGlobalsCount)
}
