package jit

import (
	"fmt"
)

type Reg byte

const (
	RAX Reg = 0
	RCX Reg = 1
	RDX Reg = 2
	RBX Reg = 3
	RSP Reg = 4
	RBP Reg = 5
	RSI Reg = 6
	RDI Reg = 7
	R8  Reg = 8
	R9  Reg = 9
	R10 Reg = 10
	R11 Reg = 11
	R12 Reg = 12
	R13 Reg = 13
	R14 Reg = 14
	R15 Reg = 15

	XMM0 Reg = 0
	XMM1 Reg = 1
	XMM2 Reg = 2
	XMM3 Reg = 3
	XMM4 Reg = 4
	XMM5 Reg = 5
	XMM6 Reg = 6
	XMM7 Reg = 7
)

type Cond byte

const (
	CondO  Cond = 0x0
	CondNO Cond = 0x1
	CondB  Cond = 0x2
	CondAE Cond = 0x3
	CondE  Cond = 0x4
	CondNE Cond = 0x5
	CondBE Cond = 0x6
	CondA  Cond = 0x7
	CondS  Cond = 0x8
	CondNS Cond = 0x9
	CondP  Cond = 0xA
	CondNP Cond = 0xB
	CondL  Cond = 0xC
	CondGE Cond = 0xD
	CondLE Cond = 0xE
	CondG  Cond = 0xF
)

type Label int

type fixup struct {
	offset int
	label  Label
}

type codeMarker struct {
	offset int
	opPtr  int
	astID  int
}

type Assembler struct {
	code    []byte
	labels  map[Label]int
	fixups  []fixup
	next    Label
	markers []codeMarker
	maxSize int
}

func NewAssembler(maxSize int) *Assembler {
	if maxSize <= 0 {
		maxSize = 1 << 20
	}
	return &Assembler{
		code:    make([]byte, 0, 1024),
		labels:  map[Label]int{},
		maxSize: maxSize,
	}
}

func (a *Assembler) Offset() int { return len(a.code) }

func (a *Assembler) NewLabel() Label {
	l := a.next
	a.next++
	return l
}

func (a *Assembler) Bind(l Label) {
	a.labels[l] = len(a.code)
}

func (a *Assembler) Mark(opPtr, astID int) {
	a.markers = append(a.markers, codeMarker{offset: len(a.code), opPtr: opPtr, astID: astID})
}

func (a *Assembler) emit(b ...byte) {
	if len(a.code)+len(b) > a.maxSize {
		panic(fmt.Sprintf("jit: code buffer exceeds %d bytes", a.maxSize))
	}
	a.code = append(a.code, b...)
}

func (a *Assembler) emitUint32(v uint32) {
	a.emit(byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

func (a *Assembler) emitUint64(v uint64) {
	a.emitUint32(uint32(v))
	a.emitUint32(uint32(v >> 32))
}

func rex(w bool, r, x, b Reg) byte {
	var rex byte = 0x40
	if w {
		rex |= 0x08
	}
	if r >= 8 {
		rex |= 0x04
	}
	if x >= 8 {
		rex |= 0x02
	}
	if b >= 8 {
		rex |= 0x01
	}
	return rex
}

func (a *Assembler) emitREX(w bool, r, x, b Reg) {
	if w || r >= 8 || x >= 8 || b >= 8 {
		a.emit(rex(w, r, x, b))
	}
}

func modrm(mod byte, reg, rm Reg) byte {
	return (mod << 6) | ((byte(reg) & 7) << 3) | (byte(rm) & 7)
}

func (a *Assembler) emitModRM(mod byte, reg, rm Reg) {
	a.emit(modrm(mod, reg, rm))
}

func (a *Assembler) emitSIB(scale byte, index, base Reg) {
	a.emit((scale << 6) | ((byte(index) & 7) << 3) | (byte(base) & 7))
}

// memDisp32 encodes [base + disp32]. A base of RSP/R12 needs a SIB byte.
func (a *Assembler) memDisp32(reg, base Reg, disp int32) {
	a.emitModRM(0x2, reg, base)
	if base&7 == RSP&7 {
		a.emitSIB(0, RSP, base)
	}
	a.emitUint32(uint32(disp))
}

// memIndexDisp32 encodes [base + index*scale + disp32].
func (a *Assembler) memIndexDisp32(reg, base, index Reg, scale byte, disp int32) {
	a.emitModRM(0x2, reg, RSP)
	a.emitSIB(scale, index, base)
	a.emitUint32(uint32(disp))
}

func (a *Assembler) MovRR(dst, src Reg) {
	a.emitREX(true, dst, 0, src)
	a.emit(0x8B)
	a.emitModRM(0x3, dst, src)
}

func (a *Assembler) MovImm64(dst Reg, v uint64) {
	a.emitREX(true, 0, 0, dst)
	a.emit(0xB8 + byte(dst&7))
	a.emitUint64(v)
}

func (a *Assembler) MovImm32(dst Reg, v int32) {
	a.emitREX(false, 0, 0, dst)
	a.emit(0xC7)
	a.emitModRM(0x3, 0, dst)
	a.emitUint32(uint32(v))
}

func (a *Assembler) MovLoad(dst, base Reg, disp int32) {
	a.emitREX(true, dst, 0, base)
	a.emit(0x8B)
	a.memDisp32(dst, base, disp)
}

func (a *Assembler) MovStore(base Reg, disp int32, src Reg) {
	a.emitREX(true, src, 0, base)
	a.emit(0x89)
	a.memDisp32(src, base, disp)
}

func (a *Assembler) Lea(dst, base Reg, disp int32) {
	a.emitREX(true, dst, 0, base)
	a.emit(0x8D)
	a.memDisp32(dst, base, disp)
}

func (a *Assembler) AddRR(dst, src Reg) {
	a.emitREX(true, src, 0, dst)
	a.emit(0x01)
	a.emitModRM(0x3, src, dst)
}

func (a *Assembler) SubRR(dst, src Reg) {
	a.emitREX(true, src, 0, dst)
	a.emit(0x29)
	a.emitModRM(0x3, src, dst)
}

func (a *Assembler) AndRR(dst, src Reg) {
	a.emitREX(true, src, 0, dst)
	a.emit(0x21)
	a.emitModRM(0x3, src, dst)
}

func (a *Assembler) OrRR(dst, src Reg) {
	a.emitREX(true, src, 0, dst)
	a.emit(0x09)
	a.emitModRM(0x3, src, dst)
}

func (a *Assembler) XorRR(dst, src Reg) {
	a.emitREX(true, src, 0, dst)
	a.emit(0x31)
	a.emitModRM(0x3, src, dst)
}

func (a *Assembler) cmpGroup(group byte, dst Reg, imm int32) {
	a.emitREX(true, 0, 0, dst)
	a.emit(0x81)
	a.emitModRM(0x3, Reg(group), dst)
	a.emitUint32(uint32(imm))
}

func (a *Assembler) CmpImm32(dst Reg, imm int32) { a.cmpGroup(7, dst, imm) }
func (a *Assembler) AddImm32(dst Reg, imm int32) { a.cmpGroup(0, dst, imm) }
func (a *Assembler) SubImm32(dst Reg, imm int32) { a.cmpGroup(5, dst, imm) }
func (a *Assembler) AndImm32(dst Reg, imm int32) { a.cmpGroup(4, dst, imm) }
func (a *Assembler) OrImm32(dst Reg, imm int32)  { a.cmpGroup(1, dst, imm) }
func (a *Assembler) XorImm32(dst Reg, imm int32) { a.cmpGroup(6, dst, imm) }

func (a *Assembler) CmpRR(dst, src Reg) {
	a.emitREX(true, src, 0, dst)
	a.emit(0x39)
	a.emitModRM(0x3, src, dst)
}

func (a *Assembler) TestRR(dst, src Reg) {
	a.emitREX(true, src, 0, dst)
	a.emit(0x85)
	a.emitModRM(0x3, src, dst)
}

func (a *Assembler) TestImm32(dst Reg, imm int32) {
	a.emitREX(true, 0, 0, dst)
	a.emit(0xF7)
	a.emitModRM(0x3, 0, dst)
	a.emitUint32(uint32(imm))
}

func (a *Assembler) CmpLoadImm32(base Reg, disp int32, imm int32) {
	a.emitREX(true, 0, 0, base)
	a.emit(0x81)
	a.memDisp32(Reg(7), base, disp)
	a.emitUint32(uint32(imm))
}

func (a *Assembler) CmpLoadReg(base Reg, disp int32, src Reg) {
	a.emitREX(true, src, 0, base)
	a.emit(0x39)
	a.memDisp32(src, base, disp)
}

func (a *Assembler) IncLoad(base Reg, disp int32) {
	a.emitREX(true, 0, 0, base)
	a.emit(0xFF)
	a.memDisp32(Reg(0), base, disp)
}

func (a *Assembler) DecLoad(base Reg, disp int32) {
	a.emitREX(true, 0, 0, base)
	a.emit(0xFF)
	a.memDisp32(Reg(1), base, disp)
}

func (a *Assembler) SetCC(cond Cond, dst Reg) {
	a.emitREX(false, 0, 0, dst)
	a.emit(0x0F, 0x90+byte(cond))
	a.emitModRM(0x3, 0, dst)
}

func (a *Assembler) MovZX(dst Reg, src Reg) {
	a.emitREX(false, dst, 0, src)
	a.emit(0x0F, 0xB6)
	a.emitModRM(0x3, dst, src)
}

func (a *Assembler) CMovNE(dst, src Reg) {
	a.emitREX(true, dst, 0, src)
	a.emit(0x0F, 0x45)
	a.emitModRM(0x3, dst, src)
}

func (a *Assembler) CMovE(dst, src Reg) {
	a.emitREX(true, dst, 0, src)
	a.emit(0x0F, 0x44)
	a.emitModRM(0x3, dst, src)
}

func (a *Assembler) Push(r Reg) {
	if r >= 8 {
		a.emit(0x41)
	}
	a.emit(0x50 + byte(r&7))
}

func (a *Assembler) Pop(r Reg) {
	if r >= 8 {
		a.emit(0x41)
	}
	a.emit(0x58 + byte(r&7))
}

func (a *Assembler) Ret() { a.emit(0xC3) }

func (a *Assembler) Jmp(l Label) {
	a.emit(0xE9)
	off := len(a.code)
	a.emitUint32(0)
	a.fixups = append(a.fixups, fixup{offset: off, label: l})
}

func (a *Assembler) Jcc(cond Cond, l Label) {
	a.emit(0x0F, 0x80+byte(cond))
	off := len(a.code)
	a.emitUint32(0)
	a.fixups = append(a.fixups, fixup{offset: off, label: l})
}

func (a *Assembler) Call(l Label) {
	a.emit(0xE8)
	off := len(a.code)
	a.emitUint32(0)
	a.fixups = append(a.fixups, fixup{offset: off, label: l})
}

func (a *Assembler) CallReg(r Reg) {
	a.emitREX(false, 0, 0, r)
	a.emit(0xFF)
	a.emitModRM(0x3, 2, r)
}

func (a *Assembler) MovSDLoad(dst Reg, base Reg, disp int32) {
	a.emit(0xF2)
	a.emitREX(false, dst, 0, base)
	a.emit(0x0F, 0x10)
	a.memDisp32(dst, base, disp)
}

func (a *Assembler) MovSDStore(base Reg, disp int32, src Reg) {
	a.emit(0xF2)
	a.emitREX(false, src, 0, base)
	a.emit(0x0F, 0x11)
	a.memDisp32(src, base, disp)
}

func (a *Assembler) MovSDRR(dst, src Reg) {
	a.emit(0xF2)
	a.emitREX(false, dst, 0, src)
	a.emit(0x0F, 0x10)
	a.emitModRM(0x3, dst, src)
}

func (a *Assembler) MovQFromGPR(dst, src Reg) {
	a.emit(0x66)
	a.emitREX(true, dst, 0, src)
	a.emit(0x0F, 0x6E)
	a.emitModRM(0x3, dst, src)
}

func (a *Assembler) MovQToGPR(dst, src Reg) {
	a.emit(0x66)
	a.emitREX(true, src, 0, dst)
	a.emit(0x0F, 0x7E)
	a.emitModRM(0x3, src, dst)
}

func (a *Assembler) AddSD(dst, src Reg) {
	a.emit(0xF2)
	a.emitREX(false, dst, 0, src)
	a.emit(0x0F, 0x58)
	a.emitModRM(0x3, dst, src)
}

func (a *Assembler) SubSD(dst, src Reg) {
	a.emit(0xF2)
	a.emitREX(false, dst, 0, src)
	a.emit(0x0F, 0x5C)
	a.emitModRM(0x3, dst, src)
}

func (a *Assembler) MulSD(dst, src Reg) {
	a.emit(0xF2)
	a.emitREX(false, dst, 0, src)
	a.emit(0x0F, 0x59)
	a.emitModRM(0x3, dst, src)
}

func (a *Assembler) DivSD(dst, src Reg) {
	a.emit(0xF2)
	a.emitREX(false, dst, 0, src)
	a.emit(0x0F, 0x5E)
	a.emitModRM(0x3, dst, src)
}

func (a *Assembler) Ucomisd(a1, a2 Reg) {
	a.emit(0x66)
	a.emitREX(false, a1, 0, a2)
	a.emit(0x0F, 0x2E)
	a.emitModRM(0x3, a1, a2)
}

func (a *Assembler) XorPD(dst, src Reg) {
	a.emit(0x66)
	a.emitREX(false, dst, 0, src)
	a.emit(0x0F, 0x57)
	a.emitModRM(0x3, dst, src)
}

func (a *Assembler) Cvtsi2sd(dst, src Reg) {
	a.emit(0xF2)
	a.emitREX(true, dst, 0, src)
	a.emit(0x0F, 0x2A)
	a.emitModRM(0x3, dst, src)
}

func (a *Assembler) Cvttsd2si(dst, src Reg) {
	a.emit(0xF2)
	a.emitREX(true, dst, 0, src)
	a.emit(0x0F, 0x2C)
	a.emitModRM(0x3, dst, src)
}

func (a *Assembler) IncRR(r Reg) {
	a.emitREX(true, 0, 0, r)
	a.emit(0xFF)
	a.emitModRM(0x3, 0, r)
}

func (a *Assembler) DecRR(r Reg) {
	a.emitREX(true, 0, 0, r)
	a.emit(0xFF)
	a.emitModRM(0x3, 1, r)
}

func (a *Assembler) MovLoadImm(dst, base Reg, disp int32) {
	a.MovLoad(dst, base, disp)
}

func (a *Assembler) MovZXLoadIndex8(dst, base, index Reg, scale byte, disp int32) {
	a.emitREX(false, dst, index, base)
	a.emit(0x0F, 0xB6)
	a.memIndexDisp32(dst, base, index, scale, disp)
}

func (a *Assembler) MovLoadIndex(dst, base, index Reg, scale byte, disp int32) {
	a.emitREX(true, dst, index, base)
	a.emit(0x8B)
	a.memIndexDisp32(dst, base, index, scale, disp)
}

func (a *Assembler) MovStoreIndex(base, index Reg, scale byte, disp int32, src Reg) {
	a.emitREX(true, src, index, base)
	a.emit(0x89)
	a.memIndexDisp32(src, base, index, scale, disp)
}

func (a *Assembler) MovSDLoadIndex(dst, base, index Reg, scale byte, disp int32) {
	a.emit(0xF2)
	a.emitREX(false, dst, index, base)
	a.emit(0x0F, 0x10)
	a.memIndexDisp32(dst, base, index, scale, disp)
}

func (a *Assembler) MovSDLoadR11(dst, base Reg, disp int32) {
	a.emit(0xF2)
	a.emitREX(false, dst, R11, base)
	a.emit(0x0F, 0x10)
	a.memIndexDisp32(dst, base, R11, 3, disp)
}

func (a *Assembler) MovSDStoreR11(base Reg, disp int32, src Reg) {
	a.emit(0xF2)
	a.emitREX(false, src, R11, base)
	a.emit(0x0F, 0x11)
	a.memIndexDisp32(src, base, R11, 3, disp)
}

func (a *Assembler) MovSDStoreIndex(base, index Reg, scale byte, disp int32, src Reg) {
	a.emit(0xF2)
	a.emitREX(false, src, index, base)
	a.emit(0x0F, 0x11)
	a.memIndexDisp32(src, base, index, scale, disp)
}

func (a *Assembler) IMulRR(dst, src Reg) {
	a.emitREX(true, dst, 0, src)
	a.emit(0x0F, 0xAF)
	a.emitModRM(0x3, dst, src)
}

func (a *Assembler) ShiftImm(group byte, dst Reg, imm byte) {
	a.emitREX(true, 0, 0, dst)
	a.emit(0xC1)
	a.emitModRM(0x3, Reg(group), dst)
	a.emit(imm)
}

func (a *Assembler) Neg(dst Reg) {
	a.emitREX(true, 0, 0, dst)
	a.emit(0xF7)
	a.emitModRM(0x3, 3, dst)
}

func (a *Assembler) Not(dst Reg) {
	a.emitREX(true, 0, 0, dst)
	a.emit(0xF7)
	a.emitModRM(0x3, 2, dst)
}

type lineDisasm struct {
	offset int
	opPtr  int
	astID  int
}

func (a *Assembler) markerAt(offset int) (codeMarker, bool) {
	lo, hi := 0, len(a.markers)-1
	res := codeMarker{opPtr: -1, astID: -1}
	found := false
	for lo <= hi {
		mid := (lo + hi) / 2
		if a.markers[mid].offset <= offset {
			res = a.markers[mid]
			found = true
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return res, found
}

func (a *Assembler) resolve() error {
	for _, f := range a.fixups {
		target, ok := a.labels[f.label]
		if !ok {
			return fmt.Errorf("jit: unbound label %d", f.label)
		}
		rel := target - (f.offset + 4)
		a.code[f.offset] = byte(rel)
		a.code[f.offset+1] = byte(rel >> 8)
		a.code[f.offset+2] = byte(rel >> 16)
		a.code[f.offset+3] = byte(rel >> 24)
	}
	return nil
}
