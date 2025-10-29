package jit

import (
	"encoding/hex"
	"fmt"
	"go_js/chunk"
	"go_js/flags"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"slices"
	"syscall"
	"unsafe"
)

const (
	BASE_IMM_64_MOV byte = 0xB8
	BASE_PUSH       byte = 0x50
	BASE_POP        byte = 0x58

	RBX byte = 3 // VM STACK TOP POINTER
	RSP byte = 4
	RBP byte = 5  // LOCALS POINTER
	R12 byte = 12 // GLOBALS POINTER
	R13 byte = 13 // CONSTANTS POINTER

	XMM0 byte = 0 // return register

	XMM1 byte = 1 // virtual stack start
	XMM2 byte = 2
	XMM3 byte = 3
	XMM4 byte = 4
	XMM5 byte = 5 // virtual stack end

	MOD_MEMORY_8_BYTE_OFFSET  byte = 0x1
	MOD_MEMORY_32_BYTE_OFFSET byte = 0x2
	MOD_REG_TO_REG            byte = 0x3

	PFX_REX byte = 0x40
	REXW    byte = 0x08
	REXB    byte = 0x01

	SIB_NO_INDEX byte = 0x24

	CMP_BASE byte = 39

	RET  byte = 0xC3
	CALL byte = 0xE8
	MOV  byte = 0x8B

	IMM32 byte = 0x81

	// yeah this is ugly I know..
	SPILLS_REQ_FOR_FIBO = 4
)

var MOV_SD_LOAD = []byte{0xF2, 0x0F, 0x10}
var MOV_SD_STORE = []byte{0xF2, 0x0F, 0x11}
var ADD_SD = []byte{0xF2, 0x0F, 0x58}
var SUB_SD = []byte{0xF2, 0x0F, 0x5C}
var UCOMISD = []byte{0x66, 0x0F, 0x2E}

var jittedFns = map[object.Callable]*Assembler{}
var jittableFns = map[object.Callable]struct {
	is     bool
	locals int
}{}
var jumpstack = []struct {
	when         int
	bufferOffset uint32
	from         int
}{}

type Assembler struct {
	buffer               []byte
	offset               int
	localsPatchOffset    int
	globalsPatchOffset   int
	constantsPatchOffset int

	freeRegister []byte
	valueStack   []byte
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

		freeRegister: []byte{XMM5, XMM4, XMM3, XMM2, XMM1},
		valueStack:   []byte{},
	}, nil
}

func (asm *Assembler) emitBytes(b ...byte) {
	for _, b := range b {
		asm.buffer[asm.offset] = b
		asm.offset++
	}
}

func (asm *Assembler) emitUint64(val uint64) {
	asm.emitBytes(
		byte(val),
		byte(val>>8),
		byte(val>>16),
		byte(val>>24),
		byte(val>>32),
		byte(val>>40),
		byte(val>>48),
		byte(val>>56),
	)
}

func (asm *Assembler) emitJA(offset uint32) {
	asm.emitBytes(0x0F, 0x87)
	asm.emitBytes(
		byte(offset),
		byte(offset>>8),
		byte(offset>>16),
		byte(offset>>24),
	)
}

func (asm *Assembler) emitUCOMISD(a, b byte) {
	asm.emitBytes(UCOMISD...)
	asm.modRm(MOD_REG_TO_REG, a, b)
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

func (asm *Assembler) modRm(mod, destination, source byte) {
	asm.emitBytes((mod << 6) | (destination << 3) | source)
}

func (asm *Assembler) emitMov(destination, source byte) {
	asm.emitBytes(PFX_REX|REXW, MOV)
	asm.modRm(MOD_REG_TO_REG, destination, source)
}
func (asm *Assembler) popFreeRegister() (byte, error) {
	if len(asm.freeRegister) == 0 {
		return 0, fmt.Errorf("no free register available")
	}
	r := asm.freeRegister[len(asm.freeRegister)-1]
	asm.freeRegister = asm.freeRegister[:len(asm.freeRegister)-1]

	return r, nil
}

func (asm *Assembler) pushFreeRegister(r byte) {
	asm.freeRegister = append(asm.freeRegister, r)
}

func (asm *Assembler) pushValueRegister(r byte) {
	asm.valueStack = append(asm.valueStack, r)
}

func (asm *Assembler) popValueRegister() (byte, error) {
	if len(asm.valueStack) == 0 {
		return 0, fmt.Errorf("no values in virtual stack")
	}
	r := asm.valueStack[len(asm.valueStack)-1]
	asm.valueStack = asm.valueStack[:len(asm.valueStack)-1]

	return r, nil
}

func (asm *Assembler) createJITFunction() func() {
	dummy := jitcall
	jit := uintptr((unsafe.Pointer(&asm.buffer[0])))

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

func (asm *Assembler) patchLocalStart(ptr uint64) {
	asm.buffer[asm.localsPatchOffset] = byte(ptr)
	asm.buffer[asm.localsPatchOffset+1] = byte(ptr >> 8)
	asm.buffer[asm.localsPatchOffset+2] = byte(ptr >> 16)
	asm.buffer[asm.localsPatchOffset+3] = byte(ptr >> 24)
	asm.buffer[asm.localsPatchOffset+4] = byte(ptr >> 32)
	asm.buffer[asm.localsPatchOffset+5] = byte(ptr >> 40)
	asm.buffer[asm.localsPatchOffset+6] = byte(ptr >> 48)
	asm.buffer[asm.localsPatchOffset+7] = byte(ptr >> 56)
}

func (asm *Assembler) patchGlobalsStart(ptr uint64) {
	asm.buffer[asm.globalsPatchOffset] = byte(ptr)
	asm.buffer[asm.globalsPatchOffset+1] = byte(ptr >> 8)
	asm.buffer[asm.globalsPatchOffset+2] = byte(ptr >> 16)
	asm.buffer[asm.globalsPatchOffset+3] = byte(ptr >> 24)
	asm.buffer[asm.globalsPatchOffset+4] = byte(ptr >> 32)
	asm.buffer[asm.globalsPatchOffset+5] = byte(ptr >> 40)
	asm.buffer[asm.globalsPatchOffset+6] = byte(ptr >> 48)
	asm.buffer[asm.globalsPatchOffset+7] = byte(ptr >> 56)
}

func (asm *Assembler) patchConstantsStart(ptr uint64) {
	asm.buffer[asm.constantsPatchOffset] = byte(ptr)
	asm.buffer[asm.constantsPatchOffset+1] = byte(ptr >> 8)
	asm.buffer[asm.constantsPatchOffset+2] = byte(ptr >> 16)
	asm.buffer[asm.constantsPatchOffset+3] = byte(ptr >> 24)
	asm.buffer[asm.constantsPatchOffset+4] = byte(ptr >> 32)
	asm.buffer[asm.constantsPatchOffset+5] = byte(ptr >> 40)
	asm.buffer[asm.constantsPatchOffset+6] = byte(ptr >> 48)
	asm.buffer[asm.constantsPatchOffset+7] = byte(ptr >> 56)
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

func (asm *Assembler) emitCall() {
	asm.emitBytes(CALL, 0, 0, 0, 0)
}

func (asm *Assembler) patchInt32(where int, val int) {
	asm.buffer[where] = byte(val)
	asm.buffer[where+1] = byte(val >> 8)
	asm.buffer[where+2] = byte(val >> 16)
	asm.buffer[where+3] = byte(val >> 24)
}

func (asm *Assembler) emitSubimm32(reg byte, val int32) {
	asm.emitBytes(PFX_REX|REXW, IMM32)
	asm.modRm(MOD_REG_TO_REG, 5, reg)
	asm.emitInt32(val)

}

func (asm *Assembler) emitInt32(i int32) {
	asm.emitBytes(
		byte(i),
		byte(i>>8),
		byte(i>>16),
		byte(i>>24),
	)
}

func compileFunction(fn object.Callable, localStart *value.Value, globalsStart *value.Value) (*Assembler, error) {
	locals := uint64(uintptr(unsafe.Pointer(localStart)))
	globals := uint64(uintptr(unsafe.Pointer(globalsStart)))
	var constants uint64 = 0
	if len(fn.ValueChunk().Constants) > 0 {
		constants = uint64(uintptr(unsafe.Pointer(&fn.ValueChunk().Constants[0])))
	}

	if asm, found := jittedFns[fn]; found {
		syscall.Mprotect(asm.buffer, syscall.PROT_WRITE|syscall.PROT_READ|syscall.PROT_EXEC)
		asm.patchLocalStart(locals)
		asm.patchGlobalsStart(globals)
		if constants > 0 {
			asm.patchConstantsStart(constants)
		}
		syscall.Mprotect(asm.buffer, syscall.PROT_READ|syscall.PROT_EXEC)
		return asm, nil
	}

	asm, err := NewAssembler()

	if err != nil {
		return nil, fmt.Errorf("failed to create assembler: %s", err.Error())
	}

	asm.emitPush(RBX)
	asm.emitPush(RBP)
	asm.emitPush(R12)
	asm.emitPush(R13)

	asm.localsPatchOffset = asm.offset + 2
	asm.emitMovImm64(RBX, locals)
	asm.globalsPatchOffset = asm.offset + 2
	asm.emitMovImm64(R12, globals)

	if constants > 0 {
		asm.constantsPatchOffset = asm.offset + 2
		asm.emitMovImm64(R13, constants)
	}

	arity := fn.GetArity()

	if arity > len(asm.freeRegister) {
		return nil, fmt.Errorf("too much function arguments (%d) for JIT to handle", arity)
	}

	for i := range fn.GetArity() {
		reg, _ := asm.popFreeRegister()

		asm.emitBytes(MOV_SD_LOAD...)
		asm.modRm(MOD_MEMORY_32_BYTE_OFFSET, reg, RBX)
		asm.emitInt32(int32(0x08 * i))

		asm.pushValueRegister(reg)
	}

	asm.emitCall()
	patchCall, from := asm.offset-4, asm.offset

	asm.emitBytes(MOV_SD_STORE...)
	asm.modRm(MOD_MEMORY_8_BYTE_OFFSET, XMM0, RBX)
	asm.emitBytes(0)

	asm.emitPop(R13)
	asm.emitPop(R12)
	asm.emitPop(RBP)
	asm.emitPop(RBX)
	asm.emitBytes(RET)
	asm.patchInt32(patchCall, asm.offset-from)

	fnStart := asm.offset
	// Notes for myself to understand/remember this stuff:
	// We have just called our jitted function RSP now points to the return address
	// 0x0 <return addr> <- RSP
	asm.emitPush(RBP)
	// We copy RBP, RSP moves down
	// 0x <return addr>
	// 0x <Old RBP value>    <- RSP
	asm.emitMov(RBP, RSP)
	// We now took the RSP's value and put it on RBP so our RBP is now anchored to the current stack top
	// our locals start at offset -8 from it (down is up in assembly land)
	// 0x <return addr>
	// 0x <Old RBP address>    <- RSP <- RBP
	asm.emitSubimm32(RSP, int32(fn.GetArity()+jittableFns[fn].locals+SPILLS_REQ_FOR_FIBO)*0x08)
	// Decrement rsp by the total size of our arguments + locals + spills, let's imagine it's 2
	// now we have space for our variables
	// 0x <return addr>
	// 0x <Old RBP address>     <- RBP
	// 0x <slot>
	// 0x <slot>                <- RSP

	r, err := asm.popValueRegister()

	offset := 1

	for err == nil {
		asm.emitBytes(MOV_SD_STORE...)
		asm.modRm(MOD_MEMORY_32_BYTE_OFFSET, r, RBP)
		asm.emitInt32(int32(-8 * offset))

		offset++
		asm.pushFreeRegister(r)
		r, err = asm.popValueRegister()
	}

	i := 0
	code := fn.ValueChunk().Code

	for i < len(code) {
		op := code[i]

		if len(jumpstack) > 0 && jumpstack[len(jumpstack)-1].when == i {
			off := jumpstack[len(jumpstack)-1].bufferOffset
			from := jumpstack[len(jumpstack)-1].from
			to := asm.offset

			amount := to - from
			asm.buffer[off] = byte(amount)
			asm.buffer[off+1] = byte(amount >> 8)
			asm.buffer[off+2] = byte(amount >> 16)
			asm.buffer[off+3] = byte(amount >> 24)

			jumpstack = jumpstack[:len(jumpstack)-1]
		}

		i++

		switch op {
		case chunk.OP_CALL:
			{
				argCount := int(code[i])
				i += 2                         // skip 'called with spread' flag
				r, _ := asm.popValueRegister() // popping callee from the virtual stack, it's not needed since we only support recursion
				asm.pushFreeRegister(r)

				args := []byte{}

				for range argCount {
					arg, _ := asm.popValueRegister()
					args = append(args, arg)
					asm.pushFreeRegister(arg)
				}

				spilled := []byte{}
				spillStart := 0

				if len(asm.valueStack) > 0 {
					r, err = asm.popValueRegister()
					spillStart = fn.GetArity() + jittableFns[fn].locals

					offset := 1
					for err == nil {
						asm.emitBytes(MOV_SD_STORE...)
						asm.modRm(MOD_MEMORY_32_BYTE_OFFSET, r, RBP)
						asm.emitInt32(int32((-8 * spillStart) + (-8 * offset)))
						offset++

						asm.pushFreeRegister(r)
						spilled = append(spilled, r)

						r, err = asm.popValueRegister()
					}
				}

				slices.Sort(asm.freeRegister)
				slices.Reverse(asm.freeRegister)

				for _, arg := range args {
					dest, _ := asm.popFreeRegister()
					asm.emitBytes(MOV_SD_LOAD...)
					asm.modRm(MOD_REG_TO_REG, dest, arg)
				}

				asm.emitCall()
				asm.patchInt32(asm.offset-4, fnStart-asm.offset)

				slices.Reverse(spilled)

				off := len(spilled)
				for i, r := range spilled {
					asm.emitBytes(MOV_SD_LOAD...)
					asm.modRm(MOD_MEMORY_32_BYTE_OFFSET, r, RBP)
					asm.emitInt32(int32((-8 * spillStart) + (-8 * (off - i))))

					asm.pushValueRegister(r)
				}

				asm.freeRegister = []byte{}

				for r := XMM5; r > XMM0; r-- {

					if !slices.Contains(asm.valueStack, r) {
						asm.freeRegister = append(asm.freeRegister, byte(r))
					}
				}

				r, _ = asm.popFreeRegister()

				asm.emitBytes(MOV_SD_LOAD...)
				asm.modRm(MOD_REG_TO_REG, r, XMM0)

				asm.pushValueRegister(r)
			}
		case chunk.OP_RETURN:
			{
				v, err := asm.popValueRegister()

				if err != nil {
					return nil, err
				}

				asm.emitBytes(MOV_SD_LOAD...)
				asm.modRm(MOD_REG_TO_REG, XMM0, v)

				asm.emitMov(RSP, RBP)
				asm.emitPop(RBP)

				asm.emitBytes(RET)
				asm.pushFreeRegister(v)
			}
		case chunk.OP_LESS_THAN_EQUAL:
			{
				b, err := asm.popValueRegister()

				if err != nil {
					return nil, err
				}

				a, err := asm.popValueRegister()

				if err != nil {
					return nil, err
				}

				if len(code) > i+1 && code[i] == chunk.OP_JUMP_IF_FALSE {
					i++
					asm.emitUCOMISD(a, b)
					asm.emitJA(0)

					target := int(code[i+3]) | int(code[i+2])<<8 | int(code[i+1])<<16 | int(code[i])<<24
					jumpstack = append(jumpstack, struct {
						when         int
						bufferOffset uint32
						from         int
					}{
						when:         target,
						bufferOffset: uint32(asm.offset) - 4,
						from:         asm.offset,
					})

					asm.pushFreeRegister(a)
					asm.pushFreeRegister(b)
				} else {
					return nil, fmt.Errorf("unsupported op code sequence")
				}
			}
		case chunk.OP_GET_GLOBAL:
			{
				off := code[i]
				i++

				freeReg, err := asm.popFreeRegister()

				if err != nil {
					return nil, err
				}

				asm.emitBytes(MOV_SD_LOAD[0], PFX_REX|REXB, MOV_SD_LOAD[1], MOV_SD_LOAD[2])
				asm.modRm(MOD_MEMORY_32_BYTE_OFFSET, freeReg, R12&0x7)
				asm.emitBytes(SIB_NO_INDEX)
				asm.emitInt32(int32(0x08 * off))

				asm.pushValueRegister(freeReg)
			}
		case chunk.OP_GET_LOCAL:
			{
				off := int(code[i] + 1)
				i++

				freeReg, err := asm.popFreeRegister()

				if err != nil {
					return nil, err
				}

				asm.emitBytes(MOV_SD_LOAD...)
				asm.modRm(MOD_MEMORY_32_BYTE_OFFSET, freeReg, RBP)
				asm.emitInt32(int32(-8 * off))

				asm.pushValueRegister(freeReg)
			}
		case chunk.OP_ADD:
			{
				b, err := asm.popValueRegister()

				if err != nil {
					return nil, err
				}

				a, err := asm.popValueRegister()

				if err != nil {
					return nil, err
				}
				asm.emitBytes(ADD_SD...)
				asm.modRm(MOD_REG_TO_REG, a, b)

				asm.pushValueRegister(a)
				asm.pushFreeRegister(b)
			}
		case chunk.OP_CONSTANT:
			{
				off := code[i]
				i++

				freeReg, err := asm.popFreeRegister()

				if err != nil {
					return nil, err
				}

				asm.emitBytes(MOV_SD_LOAD[0], PFX_REX|REXB, MOV_SD_LOAD[1], MOV_SD_LOAD[2])
				asm.modRm(MOD_MEMORY_32_BYTE_OFFSET, freeReg, R13&0x7)
				asm.emitInt32(int32(0x08 * off))

				asm.pushValueRegister(freeReg)
			}
		case chunk.OP_SUBTRACT:
			{
				b, err := asm.popValueRegister()

				if err != nil {
					return nil, err
				}

				a, err := asm.popValueRegister()

				if err != nil {
					return nil, err
				}

				asm.emitBytes(SUB_SD...)
				asm.modRm(MOD_REG_TO_REG, a, b)

				asm.pushValueRegister(a)
				asm.pushFreeRegister(b)
			}
		default:
			jittableFns[fn] = struct {
				is     bool
				locals int
			}{is: false}
			return nil, fmt.Errorf("unimplemented op code %d", op)
		}
	}

	err = syscall.Mprotect(asm.buffer, syscall.PROT_READ|syscall.PROT_EXEC)
	if flags.Debug {
		fmt.Println("Compiled assembly")
		fmt.Printf("\n%s\n", hex.Dump(asm.buffer[:asm.offset]))
	}
	if err != nil {
		return nil, fmt.Errorf("mprotect failed: %s", err.Error())
	}

	jittedFns[fn] = asm
	return asm, nil
}

func IsJittable(fn object.Callable, globals []value.Value) bool {
	if is, found := jittableFns[fn]; found {
		return is.is
	}

	is, localcount := checkJittability(fn, globals)
	jittableFns[fn] = struct {
		is     bool
		locals int
	}{is: is, locals: localcount}
	return is
}

func checkJittability(fn object.Callable, globals []value.Value) (is bool, localcount int) {
	if !flags.ENABLE_JIT {
		return false, 0
	}

	if fn.HasArguments() || fn.HasRestParameter() {
		return false, 0
	}

	c := fn.ValueChunk()
	for _, constant := range c.Constants {
		if !constant.IsNumber() {
			return false, 0
		}
	}

	if len(c.Code) <= 4 { // basically to limit op_const, 0, op_return type functions to be jitted, or runtime generated ctor calls
		return false, 0
	}

	i := 0

	for i < len(c.Code) {
		code := c.Code[i]
		i++
		switch code {
		case chunk.OP_GET_LOCAL:
			localcount = max(localcount, int(c.Code[i]))
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
					return false, 0
				}

				if !global.IsNumber() {
					return false, 0
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
			return false, 0
		}
	}

	return true, localcount
}

/*
53 55 41 54 41 55 48 bb  00 10 21 00 c0 00 00 00
49 bc 00 20 18 00 c0 00  00 00 f2 0f 10 8b 00 00
00 00 f2 0f 10 93 08 00  00 00 e8 0c 00 00 00 f2
0f 11 43 00 41 5d 41 5c  5d 5b c3 55 48 8b ec 48
81 ec 38 00 00 00 f2 0f  11 95 f8 ff ff ff f2 0f
11 8d f0 ff ff ff f2 0f  10 8d f8 ff ff ff f2 0f
10 95 f0 ff ff ff f2 0f  58 ca f2 0f 10 c1 48 8b
e5 5d c3
*/
