#include "textflag.h"

// func jitEnter(entry uintptr, act *Activation, stackTop uintptr, stackLimit uintptr) int64
//
// Switches to the dedicated JIT native stack, loads the activation state into
// the JIT ABI registers, calls the compiled entry point, and restores the Go
// stack and callee-saved registers.
TEXT ·jitEnter(SB),NOSPLIT,$64-40
	MOVQ entry+0(FP), AX
	MOVQ act+8(FP), BX
	MOVQ stackTop+16(FP), CX
	MOVQ stackLimit+24(FP), DX

	MOVQ BP, 0(SP)
	MOVQ BX, 8(SP)
	MOVQ R12, 16(SP)
	MOVQ R13, 24(SP)
	MOVQ R14, 32(SP)
	MOVQ R15, 40(SP)

	MOVQ SP, 112(BX)

	MOVQ 0(BX), R15
	MOVQ 8(BX), R12
	MOVQ 16(BX), R13
	MOVQ 24(BX), R11
	MOVQ DX, R14

	MOVQ CX, SP
	CALL AX

	MOVQ 112(BX), SP
	MOVQ 48(BX), AX
	MOVQ AX, ret+32(FP)

	MOVQ 0(SP), BP
	MOVQ 8(SP), BX
	MOVQ 16(SP), R12
	MOVQ 24(SP), R13
	MOVQ 32(SP), R14
	MOVQ 40(SP), R15
	RET

// func call5Aligned(fn, a, b, c, d, e uintptr) uintptr
// Zydis needs RSP 8-mod-16 at entry. Force 0-mod-16 immediately before CALL.
TEXT ·call5Aligned(SB),NOSPLIT|NOFRAME,$0-56
	MOVQ 8(SP), AX
	MOVQ 16(SP), DI
	MOVQ 24(SP), SI
	MOVQ 32(SP), DX
	MOVQ 40(SP), CX
	MOVQ 48(SP), R8
	PUSHQ R12
	MOVQ SP, R12
	ANDQ $-16, SP
	CALL AX
	MOVQ R12, SP
	POPQ R12
	MOVQ AX, 56(SP)
	RET
