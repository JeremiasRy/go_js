package jit

import (
	"fmt"
	"go_js/chunk"
	"go_js/object"
	"go_js/value"
	"syscall"
	"unsafe"
)

const PAGE_SIZE = 4096

var jittedFns = map[object.Callable]func(){}
var jittableFns = map[object.Callable]bool{}

type Assembler struct {
	buffer []byte
	offset int
}

type JittedFn func()

func (asm *Assembler) emitBytes(b ...byte) {
	for _, b := range b {
		asm.buffer[asm.offset] = b
		asm.offset++
	}
}

func (asm *Assembler) emitUint64(val uint64) {
	asm.buffer[asm.offset] = byte(val & 0xFF)
	asm.buffer[asm.offset+1] = byte(val >> 8)
	asm.buffer[asm.offset+2] = byte(val >> 16)
	asm.buffer[asm.offset+3] = byte(val >> 24)
	asm.buffer[asm.offset+4] = byte(val >> 32)
	asm.buffer[asm.offset+5] = byte(val >> 40)
	asm.buffer[asm.offset+6] = byte(val >> 48)
	asm.buffer[asm.offset+7] = byte(val >> 56)

	asm.offset += 8
}

func Prologue(localStart *value.Value, constants *value.Value) error {
	asm, err := NewAssembler()

	if err != nil {
		return fmt.Errorf("failed to create prologue %s", err.Error())
	}

	l := uint64(uintptr(unsafe.Pointer(localStart)))
	c := uint64(uintptr(unsafe.Pointer(constants)))

	asm.emitBytes(0x48, 0xBD)
	asm.emitUint64(l)

	asm.emitBytes(0x48, 0xBB)
	asm.emitUint64(c)

	dummy := jitcall
	pointer := (uintptr)(unsafe.Pointer(&asm.buffer[0]))

	fn := &struct {
		trampoline uintptr
		jitcode    uintptr
	}{
		trampoline: **(**uintptr)(unsafe.Pointer(&dummy)),
		jitcode:    pointer,
	}

	(*(*func())(unsafe.Pointer(&fn)))()
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
	}, nil
}

func CompileFunction(fn object.Callable) error {
	assembler, err := NewAssembler()

	if err != nil {
		return fmt.Errorf("failed to create assembler: %s", err.Error())
	}

	err = syscall.Mprotect(assembler.buffer, syscall.PROT_READ|syscall.PROT_EXEC)
	if err != nil {
		return fmt.Errorf("mprotect failed: %s", err.Error())
	}

	return nil
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

func jitcall() {}
