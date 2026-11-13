//go:build !amd64

package jit

import (
	"fmt"

	"go_js/value"
)

func jitEnter(entry uintptr, act *Activation, stackTop, stackLimit uintptr) int64 {
	act.Status = StatusThrow
	return int64(StatusThrow)
}

func call5Aligned(fn, a, b, c, d, e uintptr) uintptr { return 0 }

func (ctx *Context) compileLocked(chunk *value.ValueChunk, arity int) (*CompiledChunk, error) {
	return nil, fmt.Errorf("jit: unsupported architecture")
}
