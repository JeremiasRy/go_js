//go:build amd64

package jit

func jitEnter(entry uintptr, act *Activation, stackTop, stackLimit uintptr) int64

func call5Aligned(fn, a, b, c, d, e uintptr) uintptr
