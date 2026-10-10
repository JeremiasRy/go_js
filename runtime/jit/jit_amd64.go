package jit

const PAGE_SIZE = 4096

func jitcall()

func call5Aligned(fn, a, b, c, d, e uintptr) uintptr
