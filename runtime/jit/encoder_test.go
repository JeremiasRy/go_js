package jit

import (
	"bytes"
	"testing"
)

func TestEncoderGoldenBytes(t *testing.T) {
	cases := []struct {
		name string
		emit func(a *Assembler)
		want []byte
	}{
		{"push rbp", func(a *Assembler) { a.Push(RBP) }, []byte{0x55}},
		{"mov rax, imm64", func(a *Assembler) { a.MovImm64(RAX, 0x1122334455667788) },
			[]byte{0x48, 0xB8, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11}},
		{"mov rcx, rax", func(a *Assembler) { a.MovRR(RCX, RAX) }, []byte{0x48, 0x8B, 0xC8}},
		{"mov [rbx+0], rax", func(a *Assembler) { a.MovStore(RBX, 0, RAX) },
			[]byte{0x48, 0x89, 0x83, 0x00, 0x00, 0x00, 0x00}},
		{"mov rax, [rbx+8]", func(a *Assembler) { a.MovLoad(RAX, RBX, 8) },
			[]byte{0x48, 0x8B, 0x83, 0x08, 0x00, 0x00, 0x00}},
		{"movsd xmm0, [r15+8]", func(a *Assembler) { a.MovSDLoad(XMM0, R15, 8) },
			[]byte{0xF2, 0x41, 0x0F, 0x10, 0x87, 0x08, 0x00, 0x00, 0x00}},
		{"addsd xmm1, xmm0", func(a *Assembler) { a.AddSD(XMM1, XMM0) },
			[]byte{0xF2, 0x0F, 0x58, 0xC8}},
		{"ucomisd xmm0, xmm1", func(a *Assembler) { a.Ucomisd(XMM0, XMM1) },
			[]byte{0x66, 0x0F, 0x2E, 0xC1}},
		{"setbe cl", func(a *Assembler) { a.SetCC(CondBE, RCX) }, []byte{0x0F, 0x96, 0xC1}},
		{"inc r11", func(a *Assembler) { a.IncRR(R11) }, []byte{0x49, 0xFF, 0xC3}},
		{"ret", func(a *Assembler) { a.Ret() }, []byte{0xC3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := NewAssembler(1024)
			tc.emit(a)
			if !bytes.Equal(a.code, tc.want) {
				t.Fatalf("got %x, want %x", a.code, tc.want)
			}
		})
	}
}

func TestEncoderBranchFixup(t *testing.T) {
	a := NewAssembler(1024)
	l := a.NewLabel()
	a.Jcc(CondE, l)
	a.MovImm64(RAX, 0x1122334455667788)
	a.Bind(l)
	a.Ret()
	if err := a.resolve(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.code[:6], []byte{0x0F, 0x84, 0x0A, 0x00, 0x00, 0x00}) {
		t.Fatalf("unexpected branch bytes: %x", a.code[:6])
	}
	if a.code[16] != 0xC3 {
		t.Fatalf("target was not bound at the expected offset: %x", a.code)
	}
}
