package jit

import (
	"fmt"
	"go_js/chunk"
	"go_js/object"
	"go_js/value"
	"syscall"
	"unsafe"
)

const (
	RBX byte = 3 // LOCALS POINTER
	RBP byte = 5 // CONSTANTS POINTER

	XMM0 byte = 0
	XMM1 byte = 1
	XMM2 byte = 2
	XMM3 byte = 3
	XMM4 byte = 4

	MOD_MEMORY_8_BIT_OFFSET byte = 0x1
	MOD_REG_TO_REG          byte = 0x3

	PFX_REX = 0x40
	REXW    = PFX_REX | 0x08
	REXR    = PFX_REX | 0x04
	REXX    = PFX_REX | 0x02
	REXB    = PFX_REX | 0x01

	RET byte = 0xC3
)

var MOV_SD_LOAD = []byte{0xF2, 0x0F, 0x10}
var MOV_SD_STORE = []byte{0xF2, 0x0F, 0x11}
var ADD_SD = []byte{0xF2, 0x0F, 0x58}

const (
	LOCAL_START = 2
	LOCAL_END   = LOCAL_START + 7
)

var jittedFns = map[object.Callable]*Assembler{}
var jittableFns = map[object.Callable]bool{}

type Assembler struct {
	buffer []byte
	offset int

	freeRegister []byte
	valueStack   []byte
}

type JittedFn func()

func (asm *Assembler) emitBytes(b ...byte) {
	for _, b := range b {
		asm.buffer[asm.offset] = b
		asm.offset++
	}
}

func (asm *Assembler) emitUint64(val uint64) {
	asm.buffer[asm.offset] = byte(val)
	asm.buffer[asm.offset+1] = byte(val >> 8)
	asm.buffer[asm.offset+2] = byte(val >> 16)
	asm.buffer[asm.offset+3] = byte(val >> 24)
	asm.buffer[asm.offset+4] = byte(val >> 32)
	asm.buffer[asm.offset+5] = byte(val >> 40)
	asm.buffer[asm.offset+6] = byte(val >> 48)
	asm.buffer[asm.offset+7] = byte(val >> 56)

	asm.offset += 8
}

func (asm *Assembler) modRm(mod, reg, rm byte) {
	asm.emitBytes((mod << 6) | (reg << 3) | rm)
}

func (asm *Assembler) popFreeRegister() byte {
	r := asm.freeRegister[len(asm.freeRegister)-1]
	asm.freeRegister = asm.freeRegister[:len(asm.freeRegister)-1]

	return r
}

func (asm *Assembler) pushFreeRegister(r byte) {
	asm.freeRegister = append(asm.freeRegister, r)
}

func (asm *Assembler) pushValueRegister(r byte) {
	asm.valueStack = append(asm.valueStack, r)
}

func (asm *Assembler) popValueRegister() byte {
	r := asm.valueStack[len(asm.valueStack)-1]
	asm.valueStack = asm.valueStack[:len(asm.valueStack)-1]

	return r
}

func (asm *Assembler) createJITFunction() func() {
	dummy := jitcall
	jit := (uintptr)(unsafe.Pointer(&asm.buffer[0]))

	fn := &struct {
		trampoline uintptr
		jitcode    uintptr
	}{
		trampoline: **(**uintptr)(unsafe.Pointer(&dummy)),
		jitcode:    jit,
	}

	return (*(*func())(unsafe.Pointer(&fn)))
}

func JITFunction(localStart *value.Value, fn object.Callable) error {
	asm, err := compileFunction(fn, localStart)

	if err != nil {
		return err
	}

	asm.createJITFunction()()
	return nil
}

func NewAssembler() (*Assembler, error) {
	buffer, err := syscall.Mmap(
		-1,
		0,
		PAGE_SIZE,
		syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC,
		syscall.MAP_PRIVATE|syscall.MAP_ANON,
	)

	if err != nil {
		return nil, err
	}

	return &Assembler{
		buffer: buffer,
		offset: 0,

		freeRegister: []byte{XMM0, XMM1, XMM2, XMM3, XMM4},
		valueStack:   []byte{},
	}, nil
}

func (asm *Assembler) patchLocalStart(start uint64) {
	err := syscall.Mprotect(asm.buffer, syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC)

	if err != nil {
		panic("this should never happen (i hope)")
	}
	asm.buffer[LOCAL_START] = byte(start)
	asm.buffer[LOCAL_START+1] = byte(start >> 8)
	asm.buffer[LOCAL_START+2] = byte(start >> 16)
	asm.buffer[LOCAL_START+3] = byte(start >> 24)
	asm.buffer[LOCAL_START+4] = byte(start >> 32)
	asm.buffer[LOCAL_START+5] = byte(start >> 40)
	asm.buffer[LOCAL_START+6] = byte(start >> 48)
	asm.buffer[LOCAL_START+7] = byte(start >> 56)

	err = syscall.Mprotect(asm.buffer, syscall.PROT_READ|syscall.PROT_EXEC)
	if err != nil {
		panic("this should never happen (i hope)")
	}
}

func compileFunction(fn object.Callable, localStart *value.Value) (*Assembler, error) {
	locals := uint64(uintptr(unsafe.Pointer(localStart)))

	if asm, found := jittedFns[fn]; found {
		asm.patchLocalStart(locals)
		return asm, nil
	}

	asm, err := NewAssembler()

	if err != nil {
		return nil, fmt.Errorf("failed to create assembler: %s", err.Error())
	}

	constants := uint64(uintptr(unsafe.Pointer(&fn.ValueChunk().Constants[0])))

	asm.emitBytes(REXW, 0xB8+RBX)
	asm.emitUint64(locals)

	asm.emitBytes(REXW, 0xB8+RBP)
	asm.emitUint64(constants)

	i := 0
	code := fn.ValueChunk().Code

	for i < len(code) {
		op := code[i]
		i++
		switch op {
		case chunk.OP_RETURN:
			{
				v := asm.popValueRegister()

				asm.emitBytes(MOV_SD_STORE...)
				asm.modRm(MOD_MEMORY_8_BIT_OFFSET, v, RBX)
				asm.emitBytes(0)

				asm.emitBytes(RET)
			}
		case chunk.OP_CONSTANT:
			{
				off := code[i]
				i++

				freeReg := asm.popFreeRegister()

				asm.emitBytes(MOV_SD_LOAD...)
				asm.modRm(MOD_MEMORY_8_BIT_OFFSET, byte(freeReg), RBP)
				asm.emitBytes(0x08 * off)

				asm.pushValueRegister(freeReg)
			}
		case chunk.OP_GET_LOCAL:
			{
				off := code[i]
				i++

				freeReg := asm.popFreeRegister()

				asm.emitBytes(MOV_SD_LOAD...)
				asm.modRm(MOD_MEMORY_8_BIT_OFFSET, byte(freeReg), RBX)
				asm.emitBytes(0x08 * off)

				asm.pushValueRegister(freeReg)
			}
		case chunk.OP_ADD:
			{
				b := asm.popValueRegister()
				a := asm.popValueRegister()

				asm.emitBytes(ADD_SD...)
				asm.modRm(MOD_REG_TO_REG, a, b)

				asm.pushValueRegister(a)
				asm.pushFreeRegister(b)
			}
		}
	}

	err = syscall.Mprotect(asm.buffer, syscall.PROT_READ|syscall.PROT_EXEC)

	if err != nil {
		return nil, fmt.Errorf("mprotect failed: %s", err.Error())
	}

	jittedFns[fn] = asm
	return asm, nil
}

func IsJittable(fn object.Callable) bool {
	if is, found := jittableFns[fn]; found {
		return is
	}

	is := checkJittability(fn.ValueChunk())
	jittableFns[fn] = is
	return is
}

func checkJittability(c *value.ValueChunk) bool {
	for _, constant := range c.Constants {
		if !constant.IsNumber() {
			return false
		}
	}
	i := 0

	for i < len(c.Code) {
		switch c.Code[i] {
		case chunk.OP_GET_LOCAL:
			i++
		case chunk.OP_CONSTANT:
			i++
		case chunk.OP_ADD, chunk.OP_RETURN:
			{
			}
		default:
			{
				return false
			}
		}
		i++
	}

	return true
}
