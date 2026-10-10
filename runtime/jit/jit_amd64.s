#include "funcdata.h"
#include "textflag.h"

TEXT ·jitcall(SB),NOSPLIT,$0
        LEAQ argframe+0(FP), DI
        MOVQ 8(DX), AX
        JMP AX

// func call5Aligned(fn, a, b, c, d, e uintptr) uintptr
// NOFRAME: the toolchain otherwise inserts `push rbp`, and that extra push
// leaves Zydis with RSP 0-mod-16. SysV requires 8-mod-16 on entry, and Zydis
// then null-derefs. Force 0-mod-16 immediately before CALL regardless of how
// we were entered.
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
