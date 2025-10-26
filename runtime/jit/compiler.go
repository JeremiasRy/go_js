package jit

import (
	"fmt"
	"go_js/chunk"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"syscall"
	"unsafe"
)

const (
	BASE_IMM_64_MOV byte = 0xB8
	BASE_PUSH       byte = 0x50
	BASE_POP        byte = 0x58

	RBX byte = 3  // LOCALS POINTER
	RBP byte = 5  // CONSTANTS POINTER
	R12 byte = 12 // GLOBALS POINTER

	XMM0 byte = 0
	XMM1 byte = 1
	XMM2 byte = 2
	XMM3 byte = 3
	XMM4 byte = 4

	MOD_MEMORY_8_BYTE_OFFSET byte = 0x1
	MOD_REG_TO_REG           byte = 0x3

	PFX_REX byte = 0x40
	REXW    byte = 0x08
	REXB    byte = 0x01

	SIB_NO_INDEX byte = 0x24

	RET byte = 0xC3
)

var MOV_SD_LOAD = []byte{0xF2, 0x0F, 0x10}
var MOV_SD_STORE = []byte{0xF2, 0x0F, 0x11}
var ADD_SD = []byte{0xF2, 0x0F, 0x58}
var SUB_SD = []byte{0xF2, 0x0F, 0x5C}

var jittedFns = map[object.Callable]*Assembler{}
var jittableFns = map[object.Callable]bool{}

type Assembler struct {
	buffer             []byte
	offset             int
	localsPatchOffset  int
	globalsPatchOffset int

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

func (asm *Assembler) emitPop(reg byte) {
	if reg >= 8 {
		asm.emitBytes(PFX_REX|REXB, BASE_POP+(reg&0x7))
	} else {
		asm.emitBytes(BASE_POP + reg)
	}
}

func (asm *Assembler) emitPush(reg byte) {
	if reg >= 8 {
		asm.emitBytes(PFX_REX|REXB, BASE_PUSH+(reg&0x7))
	} else {
		asm.emitBytes(BASE_PUSH + reg)
	}
}

func (asm *Assembler) modRm(mod, from, to byte) {
	asm.emitBytes((mod << 6) | (from << 3) | to)
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

func JITFunction(localStart *value.Value, globals *value.Value, fn object.Callable) error {
	asm, err := compileFunction(fn, localStart, globals)

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
	syscall.Mprotect(asm.buffer, syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC)

	asm.buffer[asm.localsPatchOffset] = byte(start)
	asm.buffer[asm.localsPatchOffset+1] = byte(start >> 8)
	asm.buffer[asm.localsPatchOffset+2] = byte(start >> 16)
	asm.buffer[asm.localsPatchOffset+3] = byte(start >> 24)
	asm.buffer[asm.localsPatchOffset+4] = byte(start >> 32)
	asm.buffer[asm.localsPatchOffset+5] = byte(start >> 40)
	asm.buffer[asm.localsPatchOffset+6] = byte(start >> 48)
	asm.buffer[asm.localsPatchOffset+7] = byte(start >> 56)

	syscall.Mprotect(asm.buffer, syscall.PROT_READ|syscall.PROT_EXEC)
}

func (asm *Assembler) patchGlobalsStart(start uint64) {
	syscall.Mprotect(asm.buffer, syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC)

	asm.buffer[asm.globalsPatchOffset] = byte(start)
	asm.buffer[asm.globalsPatchOffset+1] = byte(start >> 8)
	asm.buffer[asm.globalsPatchOffset+2] = byte(start >> 16)
	asm.buffer[asm.globalsPatchOffset+3] = byte(start >> 24)
	asm.buffer[asm.globalsPatchOffset+4] = byte(start >> 32)
	asm.buffer[asm.globalsPatchOffset+5] = byte(start >> 40)
	asm.buffer[asm.globalsPatchOffset+6] = byte(start >> 48)
	asm.buffer[asm.globalsPatchOffset+7] = byte(start >> 56)

	syscall.Mprotect(asm.buffer, syscall.PROT_READ|syscall.PROT_EXEC)
}

func (asm *Assembler) emitMovImm64(reg byte, value uint64) {
	rexPrefix := PFX_REX | REXW
	regCode := reg

	if reg >= 8 {
		rexPrefix |= REXB
		regCode = reg & 0x7
	}
	asm.emitBytes(rexPrefix, BASE_IMM_64_MOV+regCode)
	asm.emitUint64(value)
}

func compileFunction(fn object.Callable, localStart *value.Value, globalsStart *value.Value) (*Assembler, error) {
	locals := uint64(uintptr(unsafe.Pointer(localStart)))
	globals := uint64(uintptr(unsafe.Pointer(globalsStart)))

	fmt.Println("global start", globals)

	if asm, found := jittedFns[fn]; found {
		asm.patchLocalStart(locals)
		asm.patchGlobalsStart(globals)
		return asm, nil
	}

	asm, err := NewAssembler()

	asm.emitPush(RBP)
	asm.emitPush(RBX)
	asm.emitPush(R12)

	if err != nil {
		return nil, fmt.Errorf("failed to create assembler: %s", err.Error())
	}

	asm.localsPatchOffset = asm.offset + 2
	asm.emitMovImm64(RBX, locals)
	asm.globalsPatchOffset = asm.offset + 2
	asm.emitMovImm64(R12, globals)

	if len(fn.ValueChunk().Constants) > 0 {
		constants := uint64(uintptr(unsafe.Pointer(&fn.ValueChunk().Constants[0])))
		asm.emitMovImm64(RBP, constants)
	}

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
				asm.modRm(MOD_MEMORY_8_BYTE_OFFSET, v, RBX)
				asm.emitBytes(0)

				asm.emitPop(RBP)
				asm.emitPop(RBX)
				asm.emitPop(R12)

				asm.emitBytes(RET)
			}
		case chunk.OP_GET_GLOBAL:
			{
				off := code[i]
				i++

				freeReg := asm.popFreeRegister()

				asm.emitBytes(MOV_SD_LOAD[0], PFX_REX|REXB, MOV_SD_LOAD[1], MOV_SD_LOAD[2])
				asm.modRm(MOD_MEMORY_8_BYTE_OFFSET, freeReg, R12&0x7)
				asm.emitBytes(SIB_NO_INDEX)
				asm.emitBytes(0x08 * off)

				asm.pushValueRegister(freeReg)
			}
		case chunk.OP_GET_LOCAL:
			{
				off := code[i]
				i++

				freeReg := asm.popFreeRegister()

				asm.emitBytes(MOV_SD_LOAD...)
				asm.modRm(MOD_MEMORY_8_BYTE_OFFSET, freeReg, RBX)
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
		case chunk.OP_CONSTANT:
			{
				off := code[i]
				i++

				freeReg := asm.popFreeRegister()

				asm.emitBytes(MOV_SD_LOAD...)
				asm.modRm(MOD_MEMORY_8_BYTE_OFFSET, freeReg, RBP)
				asm.emitBytes(0x08 * off)

				asm.pushValueRegister(freeReg)
			}
		case chunk.OP_SUBTRACT:
			{
				b := asm.popValueRegister()
				a := asm.popValueRegister()

				asm.emitBytes(SUB_SD...)
				asm.modRm(MOD_REG_TO_REG, a, b)

				asm.pushValueRegister(a)
				asm.pushFreeRegister(b)
			}
		default:
			jittableFns[fn] = false
			return nil, fmt.Errorf("unimplemented op code")
		}
	}

	err = syscall.Mprotect(asm.buffer, syscall.PROT_READ|syscall.PROT_EXEC)

	if err != nil {
		return nil, fmt.Errorf("mprotect failed: %s", err.Error())
	}

	jittedFns[fn] = asm
	return asm, nil
}

func IsJittable(fn object.Callable, globals []value.Value) bool {
	if is, found := jittableFns[fn]; found {
		return is
	}

	is := checkJittability(fn, globals)
	jittableFns[fn] = is
	return is
}

func checkJittability(fn object.Callable, globals []value.Value) bool {
	c := fn.ValueChunk()
	for _, constant := range c.Constants {
		if !constant.IsNumber() {
			return false
		}
	}
	i := 0

	for i < len(c.Code) {
		code := c.Code[i]
		i++
		switch code {
		case chunk.OP_GET_LOCAL:
			i++
		case chunk.OP_CONSTANT:
			i++
		case chunk.OP_ADD, chunk.OP_SUBTRACT, chunk.OP_LESS_THAN_EQUAL, chunk.OP_RETURN:
		case chunk.OP_GET_GLOBAL:
			{
				slot := c.Code[i]
				i++
				global := globals[slot]

				if global.IsObject() {
					obj, _ := heap.GetObject(global.GetHandle())
					if obj.Type() == object.OBJ_FUNCTION && obj == fn {
						continue
					}
					return false
				}

				if !global.IsNumber() {
					return false
				}
			}
		case chunk.OP_CALL:
			{
				i += 2
			}
		case chunk.OP_JUMP_IF_FALSE:
			{
				i += 4
			}
		default:
			return false
		}
	}

	return true
}
