//go:build amd64

package jit

import (
	"fmt"
	"os"
	"sort"

	"go_js/chunk"
	"go_js/flags"
	"go_js/value"
)

type compileState struct {
	asm          *Assembler
	chunk        *value.ValueChunk
	arity        int
	localCount   int
	ipLabels     map[int]Label
	entryLabels  map[int]Label
	bindings     map[int]bool
	deoptLocs    map[int]Label
	bridgeLocs   map[int]Label
	bridgeReqs   map[int]Request
	safepoints   map[int]Label
	backedges    []Backedge
	alwaysBridge map[int]bool
	alwaysDeopt  map[int]bool
	exitLabel    Label
	throwLabel   Label
	reasons      map[string]int
	direct       bool
	fastBody     Label
	fastThunk    Label
	frameSlots   int
}

func (ctx *Context) compileLocked(ch *value.ValueChunk, arity int) (*CompiledChunk, error) {
	instructions, err := decodeChunk(ch.Code)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	byIP := make(map[int]bool, len(instructions))
	for _, instr := range instructions {
		byIP[instr.IP] = true
	}
	for _, instr := range instructions {
		switch instr.Op {
		case chunk.OP_JUMP, chunk.OP_JUMP_IF_FALSE, chunk.OP_JUMP_IF_TRUE,
			chunk.OP_JUMP_IF_NULLISH, chunk.OP_JUMP_IF_NOT_NULLISH,
			chunk.OP_JUMP_IF_NOT_UNDEFINED:
			target := instr.Operands[0]
			if !byIP[target] {
				return nil, fmt.Errorf("jump at %d targets non-instruction %d", instr.IP, target)
			}
		}
	}

	localCount := arity
	for _, instr := range instructions {
		switch instr.Op {
		case chunk.OP_GET_LOCAL, chunk.OP_SET_LOCAL, chunk.OP_DEFINE_LOCAL_SLOT, chunk.OP_GET_LOCAL_RAW:
			if slot := instr.Operands[0] + 1; slot > localCount {
				localCount = slot
			}
		}
	}

	analysis := analyzeDirect(ch, arity)
	direct := analysis.ok
	if flags.JITTrace || os.Getenv("JIT_TRACE") == "1" {
		fmt.Fprintf(os.Stderr, "JIT compile fn=%s arity=%d direct=%v maxTop=%d len=%d fail=%d/%d\n", ch.FnName, arity, direct, analysis.maxTop, len(ch.Code), analysis.failIP, analysis.failOp)
	}
	frameSlots := arity + 3*len(ch.Code) + 32
	if direct {
		frameSlots = analysis.maxTop + 1
		if localCount > frameSlots {
			frameSlots = localCount
		}
		if arity+1 > frameSlots {
			frameSlots = arity + 1
		}
	} else if localCount+32 > frameSlots {
		frameSlots = localCount + 32
	}

	asm := NewAssembler(1 << 20)
	cs := &compileState{
		asm:          asm,
		chunk:        ch,
		arity:        arity,
		localCount:   localCount,
		ipLabels:     map[int]Label{},
		entryLabels:  map[int]Label{},
		bindings:     map[int]bool{},
		deoptLocs:    map[int]Label{},
		bridgeLocs:   map[int]Label{},
		bridgeReqs:   map[int]Request{},
		safepoints:   map[int]Label{},
		alwaysBridge: map[int]bool{},
		alwaysDeopt:  map[int]bool{},
		reasons:      map[string]int{},
		direct:       direct,
		frameSlots:   frameSlots,
	}
	for _, instr := range instructions {
		cs.ipLabels[instr.IP] = asm.NewLabel()
		cs.entryLabels[instr.IP] = asm.NewLabel()
	}
	cs.ipLabels[len(ch.Code)] = asm.NewLabel()
	cs.exitLabel = asm.NewLabel()
	cs.throwLabel = asm.NewLabel()
	cs.fastBody = asm.NewLabel()
	cs.fastThunk = asm.NewLabel()

	asm.Bind(cs.entryLabels[0])
	cs.emitPrologue()
	for _, instr := range instructions {
		cs.bind(instr.IP)
		asm.Mark(instr.IP, ch.AstId[instr.IP])
		if err := cs.compileInstruction(instr); err != nil {
			return nil, fmt.Errorf("ip %d: %w", instr.IP, err)
		}
	}

	asm.Bind(cs.exitLabel)
	asm.MovStore(RBX, ActivationSP, R11)
	asm.MovRR(RSP, RBP)
	asm.Pop(RBP)
	asm.Ret()

	asm.Bind(cs.throwLabel)
	asm.MovStore(RBX, ActivationStatus, RAX)
	asm.MovRR(RSP, RBP)
	asm.Pop(RBP)
	asm.Ret()

	for ip, l := range cs.deoptLocs {
		asm.Bind(l)
		asm.MovImm64(RAX, uint64(int64(ip)))
		asm.MovStore(RBX, ActivationBytecodeIP, RAX)
		asm.MovImm64(RAX, uint64(StatusDeopt))
		asm.MovStore(RBX, ActivationStatus, RAX)
		asm.MovStore(RBX, ActivationSP, R11)
		asm.MovRR(RSP, RBP)
		asm.Pop(RBP)
		asm.Ret()
	}
	for ip, l := range cs.bridgeLocs {
		asm.Bind(l)
		asm.MovImm64(RAX, uint64(cs.bridgeReqs[ip]))
		asm.MovStore(RBX, ActivationRequest, RAX)
		asm.MovImm64(RAX, uint64(int64(ip)))
		asm.MovStore(RBX, ActivationBytecodeIP, RAX)
		asm.MovImm64(RAX, uint64(StatusBridge))
		asm.MovStore(RBX, ActivationStatus, RAX)
		asm.MovStore(RBX, ActivationSP, R11)
		asm.MovRR(RSP, RBP)
		asm.Pop(RBP)
		asm.Ret()
	}

	if direct {
		asm.Bind(cs.fastThunk)
		asm.Push(RBP)
		asm.MovRR(RBP, RSP)
		asm.CmpRR(RSP, R14)
		asm.Jcc(CondBE, cs.stackOverflowLabel())
		asm.Jmp(cs.fastBody)
	}
	for target, l := range cs.safepoints {
		asm.Bind(l)
		asm.MovImm64(RAX, uint64(ReqSafepoint))
		asm.MovStore(RBX, ActivationRequest, RAX)
		asm.MovImm64(RAX, uint64(int64(target)))
		asm.MovStore(RBX, ActivationBytecodeIP, RAX)
		asm.MovImm64(RAX, uint64(StatusBridge))
		asm.MovStore(RBX, ActivationStatus, RAX)
		asm.MovStore(RBX, ActivationSP, R11)
		asm.MovRR(RSP, RBP)
		asm.Pop(RBP)
		asm.Ret()
	}
	for _, instr := range instructions {
		if instr.IP == 0 {
			continue
		}
		asm.Bind(cs.entryLabels[instr.IP])
		asm.Push(RBP)
		asm.MovRR(RBP, RSP)
		asm.Jmp(cs.ipLabels[instr.IP])
	}

	if err := asm.resolve(); err != nil {
		return nil, err
	}
	mem, err := ctx.seal(asm.code)
	if err != nil {
		return nil, err
	}
	cc := &CompiledChunk{
		Chunk:      ch,
		Code:       asm.code,
		Mem:        mem,
		Entries:    map[int]uintptr{},
		LocalCount: localCount,
		FrameSlots: frameSlots,
		MaxSP:      frameSlots,
		Arity:      arity,
		Direct:     direct,
	}
	if direct {
		if off, ok := asm.labels[cs.fastThunk]; ok {
			cc.FastEntry = uintptr(off)
		}
	}
	for ip, l := range cs.entryLabels {
		if off, ok := asm.labels[l]; ok {
			cc.Entries[ip] = uintptr(off)
		}
	}
	heavyIPs := make([]int, 0, len(cs.alwaysBridge)+len(cs.alwaysDeopt))
	for ip := range cs.alwaysBridge {
		heavyIPs = append(heavyIPs, ip)
	}
	for ip := range cs.alwaysDeopt {
		heavyIPs = append(heavyIPs, ip)
	}
	sort.Ints(heavyIPs)
	cc.HeavyIPs = heavyIPs
	cc.Backedges = cs.backedges
	cc.OSRable = len(heavyIPs) == 0
	if os.Getenv("JIT_TRACE") == "1" {
		fmt.Fprintf(os.Stderr, "JIT osr-able=%v heavy=%d\n", cc.OSRable, len(heavyIPs))
	}
	cc.Disassembly = buildDisassembly(asm.code, mem, asm.markers)
	return cc, nil
}

func (cs *compileState) emitPrologue() {
	asm := cs.asm
	asm.Push(RBP)
	asm.MovRR(RBP, RSP)
	asm.CmpRR(RSP, R14)
	asm.Jcc(CondBE, cs.stackOverflowLabel())
	asm.CmpLoadImm32(RBX, ActivationDepth, MaxJITFrames)
	asm.Jcc(CondGE, cs.stackOverflowLabel())
	if cs.direct {
		guard := cs.deoptLabel(0, "entry-arg-not-number")
		for i := 0; i < cs.arity; i++ {
			asm.MovLoad(RAX, R15, int32(i*8))
			cs.isNumberJump(RAX, guard)
		}
	}
	asm.Bind(cs.fastBody)
}

func (cs *compileState) stackOverflowLabel() Label {
	l, ok := cs.bridgeLocs[-1]
	if ok {
		return l
	}
	l = cs.asm.NewLabel()
	cs.bridgeLocs[-1] = l
	cs.bridgeReqs[-1] = ReqStackOverflow
	return l
}

func (cs *compileState) bind(ip int) {
	if cs.bindings[ip] {
		return
	}
	cs.bindings[ip] = true
	cs.asm.Bind(cs.ipLabels[ip])
}

func (cs *compileState) deoptLabel(ip int, reason string) Label {
	if l, ok := cs.deoptLocs[ip]; ok {
		return l
	}
	l := cs.asm.NewLabel()
	cs.deoptLocs[ip] = l
	if reason != "" {
		cs.reasons[reason]++
	}
	return l
}

func (cs *compileState) deopt(ip int, reason string) {
	cs.alwaysDeopt[ip] = true
	cs.asm.Jmp(cs.deoptLabel(ip, reason))
}

func (cs *compileState) bridgeLabel(ip int, req Request) Label {
	if l, ok := cs.bridgeLocs[ip]; ok {
		cs.bridgeReqs[ip] = req
		return l
	}
	l := cs.asm.NewLabel()
	cs.bridgeLocs[ip] = l
	cs.bridgeReqs[ip] = req
	return l
}

func (cs *compileState) bridge(ip int, req Request) {
	cs.alwaysBridge[ip] = true
	cs.asm.Jmp(cs.bridgeLabel(ip, req))
}

func (cs *compileState) pushImm(v value.Value) {
	cs.asm.MovImm64(RAX, uint64(v))
	cs.pushReg(RAX)
}

func (cs *compileState) pushReg(r Reg) {
	cs.asm.MovStoreIndex(R15, R11, 3, 0, r)
	cs.asm.IncRR(R11)
}

func (cs *compileState) popReg(r Reg) {
	cs.asm.DecRR(R11)
	cs.asm.MovLoadIndex(r, R15, R11, 3, 0)
}

func (cs *compileState) loadTop(r Reg, depth int) {
	cs.asm.MovLoadIndex(r, R15, R11, 3, int32(-8*(depth+1)))
}

func (cs *compileState) storeTop(r Reg, depth int) {
	cs.asm.MovStoreIndex(R15, R11, 3, int32(-8*(depth+1)), r)
}

func (cs *compileState) isNumberJump(r Reg, notNumber Label) {
	asm := cs.asm
	ok := asm.NewLabel()
	asm.MovRR(RCX, r)
	asm.ShiftImm(5, RCX, 48)
	asm.CmpImm32(RCX, 0xfffc)
	asm.Jcc(CondAE, notNumber)
	asm.CmpImm32(RCX, 0x7ffc)
	asm.Jcc(CondNE, ok)
	asm.MovRR(RCX, r)
	asm.MovImm64(RDX, uint64(value.QNAN))
	asm.SubRR(RCX, RDX)
	asm.CmpImm32(RCX, 0x100)
	asm.Jcc(CondAE, ok)
	asm.TestRR(RCX, RCX)
	asm.Jcc(CondE, ok)
	asm.Lea(RDX, RCX, -1)
	asm.TestRR(RDX, RCX)
	asm.Jcc(CondE, notNumber)
	asm.Bind(ok)
}

func (cs *compileState) boolFromRCX() {
	asm := cs.asm
	asm.MovImm64(RAX, uint64(value.FALSE))
	asm.MovImm64(RDX, uint64(value.TRUE))
	asm.TestRR(RCX, RCX)
	asm.CMovNE(RAX, RDX)
}

// emitTruthiness leaves 1 in RCX when r is truthy and 0 otherwise.
func (cs *compileState) emitTruthiness(r Reg, ip int) {
	asm := cs.asm
	if cs.direct {
		trueLabel := asm.NewLabel()
		doneLabel := asm.NewLabel()
		asm.MovImm32(RCX, 0)
		asm.MovImm64(RDX, uint64(value.TRUE))
		asm.CmpRR(r, RDX)
		asm.Jcc(CondE, trueLabel)
		asm.MovImm64(RDX, uint64(value.FALSE))
		asm.CmpRR(r, RDX)
		asm.Jcc(CondE, doneLabel)
		asm.MovQFromGPR(XMM0, r)
		asm.XorPD(XMM1, XMM1)
		asm.Ucomisd(XMM0, XMM1)
		asm.Jcc(CondP, doneLabel)
		asm.Jcc(CondE, doneLabel)
		asm.Bind(trueLabel)
		asm.MovImm32(RCX, 1)
		asm.Bind(doneLabel)
		return
	}
	trueLabel := asm.NewLabel()
	doneLabel := asm.NewLabel()
	asm.MovImm32(RCX, 0)
	asm.MovImm64(RDX, uint64(value.TRUE))
	asm.CmpRR(r, RDX)
	asm.Jcc(CondE, trueLabel)
	for _, tag := range []value.Value{value.FALSE, value.NULL, value.UNDEFINED, value.UNINITIALIZED} {
		asm.MovImm64(RDX, uint64(tag))
		asm.CmpRR(r, RDX)
		asm.Jcc(CondE, doneLabel)
	}
	asm.MovRR(RDX, r)
	asm.ShiftImm(5, RDX, 48)
	asm.CmpImm32(RDX, 0xfffc)
	asm.Jcc(CondAE, cs.deoptLabel(ip, "object-truthiness"))
	asm.MovQFromGPR(XMM0, r)
	asm.XorPD(XMM1, XMM1)
	asm.Ucomisd(XMM0, XMM1)
	asm.Jcc(CondP, doneLabel)
	asm.Jcc(CondE, doneLabel)
	asm.Bind(trueLabel)
	asm.MovImm32(RCX, 1)
	asm.Bind(doneLabel)
}

func (cs *compileState) compileInstruction(instr Instruction) error {
	asm := cs.asm
	ip := instr.IP
	switch instr.Op {
	case chunk.OP_CONSTANT:
		idx := instr.Operands[0]
		if idx >= len(cs.chunk.Constants) {
			return fmt.Errorf("constant index %d out of range", idx)
		}
		cs.pushImm(cs.chunk.Constants[idx])
	case chunk.OP_NILL:
		cs.pushImm(value.NULL)
	case chunk.OP_UNDEFINED:
		cs.pushImm(value.UNDEFINED)
	case chunk.OP_PUSH_UNDEFINED:
		cs.pushImm(value.UNDEFINED)
	case chunk.OP_PUSH_UNINITIALIZED:
		cs.pushImm(value.UNINITIALIZED)
	case chunk.OP_PUSH_GLOBAL_OBJECT:
		cs.bridge(ip, ReqPushGlobalObject)
	case chunk.OP_THIS:
		cs.bridge(ip, ReqThis)
	case chunk.OP_NEW_TARGET:
		cs.bridge(ip, ReqNewTarget)
	case chunk.OP_POP:
		asm.DecRR(R11)
	case chunk.OP_DUP:
		cs.loadTop(RAX, 0)
		cs.pushReg(RAX)
	case chunk.OP_DUP2:
		cs.loadTop(RAX, 1)
		cs.pushReg(RAX)
		cs.loadTop(RAX, 0)
		cs.pushReg(RAX)
	case chunk.OP_PUSH_CURRENT:
		cs.loadTop(RAX, 0)
		cs.pushReg(RAX)
	case chunk.OP_DUP_CLASS:
		cs.loadTop(RAX, 1)
		cs.pushReg(RAX)
	case chunk.OP_POP_BELOW2:
		// [.., a, b, v] -> [.., v]
		cs.loadTop(RAX, 0)
		cs.storeTop(RAX, 2)
		asm.SubImm32(R11, 2)
	case chunk.OP_RETURN:
		cs.popReg(RAX)
		notMethod := asm.NewLabel()
		asm.MovImm64(RDX, uint64(value.TAG_METHOD_HANDLE))
		asm.CmpRR(RAX, RDX)
		asm.Jcc(CondNE, notMethod)
		cs.popReg(RAX)
		asm.DecRR(R11)
		asm.Bind(notMethod)
		asm.MovStore(RBX, ActivationResult, RAX)
		asm.MovImm64(RDX, 0)
		asm.MovStore(RBX, ActivationStatus, RDX)
		asm.MovRR(RSP, RBP)
		asm.Pop(RBP)
		asm.Ret()
	case chunk.OP_JUMP:
		cs.emitJump(instr.Operands[0], instr.IP)
	case chunk.OP_JUMP_IF_FALSE, chunk.OP_JUMP_IF_TRUE:
		cs.loadTop(RAX, 0)
		cs.emitTruthiness(RAX, ip)
		falsy := asm.NewLabel()
		asm.TestRR(RCX, RCX)
		if instr.Op == chunk.OP_JUMP_IF_FALSE {
			asm.Jcc(CondE, falsy)
		} else {
			asm.Jcc(CondNE, falsy)
		}
		asm.DecRR(R11)
		asm.Jmp(cs.ipLabels[instr.NextIP])
		asm.Bind(falsy)
		asm.DecRR(R11)
		cs.emitJump(instr.Operands[0], instr.IP)
	case chunk.OP_JUMP_IF_NULLISH, chunk.OP_JUMP_IF_NOT_NULLISH:
		cs.loadTop(RAX, 0)
		jump := asm.NewLabel()
		asm.MovImm64(RDX, uint64(value.NULL))
		asm.CmpRR(RAX, RDX)
		asm.Jcc(CondE, jump)
		asm.MovImm64(RDX, uint64(value.UNDEFINED))
		asm.CmpRR(RAX, RDX)
		asm.Jcc(CondE, jump)
		asm.DecRR(R11)
		if instr.Op == chunk.OP_JUMP_IF_NOT_NULLISH {
			asm.Jmp(cs.ipLabels[instr.Operands[0]])
		} else {
			asm.Jmp(cs.ipLabels[instr.NextIP])
		}
		asm.Bind(jump)
		asm.DecRR(R11)
		if instr.Op == chunk.OP_JUMP_IF_NULLISH {
			cs.emitJump(instr.Operands[0], instr.IP)
		} else {
			asm.Jmp(cs.ipLabels[instr.NextIP])
		}
	case chunk.OP_JUMP_IF_NOT_UNDEFINED:
		cs.loadTop(RAX, 0)
		jump := asm.NewLabel()
		asm.MovImm64(RDX, uint64(value.UNDEFINED))
		asm.CmpRR(RAX, RDX)
		asm.Jcc(CondE, jump)
		asm.MovImm64(RDX, uint64(value.UNINITIALIZED))
		asm.CmpRR(RAX, RDX)
		asm.Jcc(CondE, jump)
		asm.DecRR(R11)
		cs.emitJump(instr.Operands[0], instr.IP)
		asm.Bind(jump)
		asm.DecRR(R11)
		asm.Jmp(cs.ipLabels[instr.NextIP])
	case chunk.OP_GET_LOCAL:
		asm.MovLoad(RAX, R15, int32(instr.Operands[0]*8))
		if !cs.direct {
			asm.MovImm64(RDX, uint64(value.UNINITIALIZED))
			asm.CmpRR(RAX, RDX)
			asm.Jcc(CondE, cs.deoptLabel(ip, "uninitialized-local"))
		}
		cs.pushReg(RAX)
	case chunk.OP_GET_LOCAL_RAW:
		asm.MovLoad(RAX, R15, int32(instr.Operands[0]*8))
		cs.pushReg(RAX)
	case chunk.OP_SET_LOCAL:
		cs.loadTop(RAX, 0)
		if !cs.direct {
			asm.MovRR(RCX, RAX)
			asm.ShiftImm(5, RCX, 48)
			asm.CmpImm32(RCX, 0xfffc)
			asm.Jcc(CondAE, cs.bridgeLabel(ip, ReqSetLocal))
		}
		asm.MovStore(R15, int32(instr.Operands[0]*8), RAX)
		asm.DecRR(R11)
	case chunk.OP_DEFINE_LOCAL_SLOT:
		slot := instr.Operands[0]
		cs.loadTop(RAX, 0)
		if !cs.direct {
			asm.MovRR(RCX, RAX)
			asm.ShiftImm(5, RCX, 48)
			asm.CmpImm32(RCX, 0xfffc)
			asm.Jcc(CondAE, cs.bridgeLabel(ip, ReqDefineLocalSlot))
		}
		asm.DecRR(R11)
		store := asm.NewLabel()
		asm.CmpImm32(R11, int32(slot))
		asm.Jcc(CondG, store)
		asm.MovImm32(R11, int32(slot+1))
		asm.Bind(store)
		asm.MovStore(R15, int32(slot*8), RAX)
	case chunk.OP_DEFINE_LOCAL:
		if !cs.direct {
			cs.loadTop(RAX, 0)
			asm.MovRR(RCX, RAX)
			asm.ShiftImm(5, RCX, 48)
			asm.CmpImm32(RCX, 0xfffc)
			asm.Jcc(CondAE, cs.bridgeLabel(ip, ReqDefineLocal))
		}
	case chunk.OP_POP_LOCAL:
		under := asm.NewLabel()
		asm.TestRR(R11, R11)
		asm.Jcc(CondE, under)
		asm.DecRR(R11)
		asm.Bind(under)
	case chunk.OP_GET_GLOBAL:
		slot := int64(instr.Operands[0])
		asm.MovLoad(RCX, RBX, ActivationGlobalsCount)
		asm.MovImm64(RAX, uint64(slot))
		asm.CmpRR(RCX, RAX)
		asm.Jcc(CondLE, cs.deoptLabel(ip, "global-out-of-range"))
		asm.MovLoad(RAX, R12, int32(slot*8))
		asm.MovImm64(RDX, uint64(value.UNINITIALIZED))
		asm.CmpRR(RAX, RDX)
		asm.Jcc(CondE, cs.deoptLabel(ip, "uninitialized-global"))
		cs.pushReg(RAX)
	case chunk.OP_SET_GLOBAL:
		cs.emitSetGlobal(instr)
	case chunk.OP_DEFINE_GLOBAL:
		cs.bridge(ip, ReqDefineGlobal)
	case chunk.OP_GET_GLOBAL_BY_NAME:
		cs.bridge(ip, ReqGetGlobal)
	case chunk.OP_DEFINE_GLOBAL_NAMED:
		cs.bridge(ip, ReqDefineGlobal)
	case chunk.OP_SET_GLOBAL_NAMED:
		cs.bridge(ip, ReqSetGlobal)
	case chunk.OP_ADD:
		cs.emitNumericBinary(instr, ReqAdd, func() { asm.AddSD(XMM1, XMM0) })
	case chunk.OP_SUBTRACT:
		cs.emitNumericBinary(instr, ReqSubtract, func() { asm.SubSD(XMM1, XMM0) })
	case chunk.OP_MULTIPLY:
		cs.emitNumericBinary(instr, ReqMultiply, func() { asm.MulSD(XMM1, XMM0) })
	case chunk.OP_DIVIDE:
		cs.emitNumericBinary(instr, ReqDivide, func() { asm.DivSD(XMM1, XMM0) })
	case chunk.OP_MODULO:
		cs.bridge(ip, ReqModulo)
	case chunk.OP_EXPONENTIATION:
		cs.bridge(ip, ReqExponentiation)
	case chunk.OP_NEGATE:
		cs.loadTop(RAX, 0)
		cs.isNumberJump(RAX, cs.bridgeLabel(ip, ReqNegate))
		asm.MovQFromGPR(XMM0, RAX)
		asm.MovImm64(RDX, 0x8000000000000000)
		asm.MovQFromGPR(XMM1, RDX)
		asm.XorPD(XMM0, XMM1)
		asm.MovQToGPR(RAX, XMM0)
		cs.storeTop(RAX, 0)
	case chunk.OP_LESS_THAN:
		cs.emitCompare(instr, CondB, ReqLessThan)
	case chunk.OP_LESS_THAN_EQUAL:
		cs.emitCompare(instr, CondBE, ReqLessThanEqual)
	case chunk.OP_GREATER_THAN:
		cs.emitCompare(instr, CondA, ReqGreaterThan)
	case chunk.OP_GREATER_THAN_EQUAL:
		cs.emitCompare(instr, CondAE, ReqGreaterThanEqual)
	case chunk.OP_STRICT_EQUALS:
		cs.emitStrictEquals(instr, false)
	case chunk.OP_STRICT_NOT_EQUALS:
		cs.emitStrictEquals(instr, true)
	case chunk.OP_EQUALS:
		cs.bridge(ip, ReqEquals)
	case chunk.OP_NOT_EQUALS:
		cs.bridge(ip, ReqNotEquals)
	case chunk.OP_NOT:
		cs.loadTop(RAX, 0)
		cs.emitTruthiness(RAX, ip)
		asm.XorImm32(RCX, 1)
		cs.boolFromRCX()
		cs.storeTop(RAX, 0)
	case chunk.OP_VOID:
		asm.DecRR(R11)
		cs.pushImm(value.UNDEFINED)
	case chunk.OP_TO_NUMBER:
		cs.bridge(ip, ReqToNumber)
	case chunk.OP_TO_STRING:
		cs.bridge(ip, ReqToString)
	case chunk.OP_TO_PROPERTY_KEY:
		cs.bridge(ip, ReqToPropertyKey)
	case chunk.OP_TYPEOF:
		cs.bridge(ip, ReqTypeof)
	case chunk.OP_UPDATE:
		cs.emitUpdate(instr)
	case chunk.OP_B_AND:
		cs.bridge(ip, ReqBitAnd)
	case chunk.OP_B_OR:
		cs.bridge(ip, ReqBitOr)
	case chunk.OP_B_XOR:
		cs.bridge(ip, ReqBitXor)
	case chunk.OP_BITWISE_NOT:
		cs.bridge(ip, ReqBitwiseNot)
	case chunk.OP_LEFT_SHIFT:
		cs.bridge(ip, ReqLeftShift)
	case chunk.OP_RIGHT_SHIFT:
		cs.bridge(ip, ReqRightShift)
	case chunk.OP_UNSIGNED_RIGHT_SHIFT:
		cs.bridge(ip, ReqUnsignedRightShift)
	case chunk.OP_CALL:
		if cs.direct {
			cs.emitDirectCall(instr)
		} else {
			cs.bridge(ip, ReqCall)
		}
	case chunk.OP_CALL_OPTIONAL:
		cs.bridge(ip, ReqCallOptional)
	case chunk.OP_CALL_CALLEE_FIRST:
		cs.bridge(ip, ReqCallCalleeFirst)
	case chunk.OP_NEW:
		cs.bridge(ip, ReqNew)
	case chunk.OP_SUPER_CALL:
		cs.bridge(ip, ReqSuperCall)
	case chunk.OP_CREATE_OBJECT:
		cs.bridge(ip, ReqCreateObject)
	case chunk.OP_CREATE_ARRAY:
		cs.bridge(ip, ReqCreateArray)
	case chunk.OP_PUSH_ELEMENT:
		cs.bridge(ip, ReqPushElement)
	case chunk.OP_GET_OBJECT_MEMBER:
		cs.bridge(ip, ReqGetMember)
	case chunk.OP_SET_OBJECT_MEMBER, chunk.OP_SET_MEMBER_RETURN:
		cs.bridge(ip, ReqSetMemberReturn)
	case chunk.OP_SET_MEMBER_KEEP:
		cs.bridge(ip, ReqSetMemberKeep)
	case chunk.OP_SET_MEMBER_STRICT_RETURN:
		cs.bridge(ip, ReqSetMemberStrictReturn)
	case chunk.OP_MEMBER_UPDATE:
		cs.bridge(ip, ReqUpdate)
	case chunk.OP_DELETE:
		cs.bridge(ip, ReqDelete)
	case chunk.OP_DELETE_STRICT:
		cs.bridge(ip, ReqDeleteStrict)
	case chunk.OP_IN:
		cs.bridge(ip, ReqIn)
	case chunk.OP_INSTANCEOF:
		cs.bridge(ip, ReqInstanceof)
	case chunk.OP_UNWRAP_METHOD:
		cs.bridge(ip, ReqUnwrapMethod)
	case chunk.OP_LOGICAL_OR, chunk.OP_LOGICAL_AND, chunk.OP_NULL_COALESHING:
		cs.bridge(ip, ReqDefined)
	case chunk.OP_GET_HEAP_VAR:
		cs.bridge(ip, ReqGetHeapVar)
	case chunk.OP_SET_HEAP_VAR:
		cs.bridge(ip, ReqSetHeapVar)
	case chunk.OP_GET_HEAP_VAR_RAW:
		cs.bridge(ip, ReqGetHeapVarRaw)
	case chunk.OP_DEFINE_HEAP_VAR:
		cs.bridge(ip, ReqDefineHeapVar)
	case chunk.OP_DEFINE_HEAP_VAR_AT:
		cs.bridge(ip, ReqDefineHeapVarAt)
	case chunk.OP_CREATE_HEAP_SCOPE:
		cs.bridge(ip, ReqCreateHeapScope)
	case chunk.OP_COPY_HEAP_SCOPE:
		cs.bridge(ip, ReqCopyHeapScope)
	case chunk.OP_ENSURE_HEAP_SCOPE:
		cs.bridge(ip, ReqEnsureHeapScope)
	case chunk.OP_CREATE_REST_OBJECT:
		cs.bridge(ip, ReqCreateRestObject)
	case chunk.OP_CREATE_REST_OBJECT_DYNAMIC:
		cs.bridge(ip, ReqCreateRestObjectDynamic)
	case chunk.OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS:
		cs.bridge(ip, ReqDefineHeapVarsFromArguments)
	case chunk.OP_SPREAD:
		cs.bridge(ip, ReqSpread)
	case chunk.OP_SPREAD_INTO_ARRAY:
		cs.bridge(ip, ReqSpreadIntoArray)
	case chunk.OP_SET_FROM_SPREAD:
		cs.bridge(ip, ReqSetFromSpread)
	case chunk.OP_GET_ITERATOR:
		cs.bridge(ip, ReqGetIterator)
	case chunk.OP_ITERATOR_NEXT_SLOT:
		cs.bridge(ip, ReqIteratorNextSlot)
	case chunk.OP_ITERATOR_CURRENT_SLOT:
		cs.bridge(ip, ReqIteratorCurrentSlot)
	case chunk.OP_ITERATOR_CLOSE_SLOT:
		cs.bridge(ip, ReqIteratorCloseSlot)
	case chunk.OP_ITERATOR_CLOSE_MARK:
		cs.bridge(ip, ReqIteratorCloseSlot)
	case chunk.OP_ITERATOR_CLOSE_POP:
		cs.bridge(ip, ReqIteratorClosePop)
	case chunk.OP_ITERATOR_VALUE_OR_UNDEFINED:
		cs.bridge(ip, ReqIteratorValueOrUndefined)
	case chunk.OP_ITERATOR_REST:
		cs.bridge(ip, ReqIteratorRest)
	case chunk.OP_ITERATOR_NEXT, chunk.OP_ITERATOR_CURRENT:
		cs.bridge(ip, ReqIteratorNextSlot)
	case chunk.OP_SAVE_COMPLETION:
		cs.bridge(ip, ReqSaveCompletion)
	case chunk.OP_RESTORE_COMPLETION:
		cs.bridge(ip, ReqRestoreCompletion)
	case chunk.OP_GET_CATCH:
		cs.bridge(ip, ReqGetCatch)
	case chunk.OP_SET_CATCH:
		cs.bridge(ip, ReqSetCatch)
	case chunk.OP_CATCH_BIND:
		cs.bridge(ip, ReqSetCatch)
	case chunk.OP_THROW:
		cs.bridge(ip, ReqThrow)
	default:
		cs.deopt(ip, "unsupported-opcode")
	}
	return nil
}

func (cs *compileState) emitJump(target, from int) {
	if target < from {
		cs.backedges = append(cs.backedges, Backedge{From: from, To: target})
		cs.emitBackedge(target)
		return
	}
	cs.asm.Jmp(cs.ipLabels[target])
}

func (cs *compileState) emitBackedge(target int) {
	asm := cs.asm
	skip := asm.NewLabel()
	asm.DecLoad(RBX, ActivationLoopBudget)
	asm.Jcc(CondG, skip)
	asm.Jmp(cs.safepointLabel(target))
	asm.Bind(skip)
	asm.Jmp(cs.ipLabels[target])
}

func (cs *compileState) safepointLabel(target int) Label {
	if l, ok := cs.safepoints[target]; ok {
		return l
	}
	l := cs.asm.NewLabel()
	cs.safepoints[target] = l
	return l
}

func (cs *compileState) emitSetGlobal(instr Instruction) {
	asm := cs.asm
	bridge := cs.bridgeLabel(instr.IP, ReqSetGlobal)
	cs.loadTop(RAX, 0)
	asm.MovRR(RCX, RAX)
	asm.ShiftImm(5, RCX, 48)
	asm.CmpImm32(RCX, 0xfffc)
	asm.Jcc(CondAE, bridge)
	slot := int64(instr.Operands[0])
	asm.MovLoad(RCX, RBX, ActivationGlobalsCount)
	asm.MovImm64(RDX, uint64(slot))
	asm.CmpRR(RCX, RDX)
	asm.Jcc(CondLE, bridge)
	asm.MovLoad(RDX, RBX, ActivationGlobalSync)
	asm.MovLoad(RCX, RDX, int32(slot*8))
	asm.TestRR(RCX, RCX)
	asm.Jcc(CondNE, bridge)
	asm.DecRR(R11)
	asm.MovStore(R12, int32(slot*8), RAX)
}

func (cs *compileState) emitUpdate(instr Instruction) {
	asm := cs.asm
	if !cs.direct {
		cs.loadTop(RAX, 0)
		cs.isNumberJump(RAX, cs.bridgeLabel(instr.IP, ReqUpdate))
	}
	asm.MovSDLoadR11(XMM0, R15, -8)
	asm.MovImm64(RDX, 0x3FF0000000000000)
	asm.MovQFromGPR(XMM1, RDX)
	if instr.Operands[0] == 1 {
		asm.SubSD(XMM0, XMM1)
	} else {
		asm.AddSD(XMM0, XMM1)
	}
	asm.MovSDStoreR11(R15, -8, XMM0)
}

func (cs *compileState) emitNumericBinary(instr Instruction, req Request, op func()) {
	asm := cs.asm
	if !cs.direct {
		notNumber := cs.bridgeLabel(instr.IP, req)
		cs.loadTop(RAX, 1)
		cs.isNumberJump(RAX, notNumber)
		cs.loadTop(RCX, 0)
		cs.isNumberJump(RCX, notNumber)
	}
	asm.MovSDLoadIndex(XMM1, R15, R11, 3, -16)
	asm.MovSDLoadIndex(XMM0, R15, R11, 3, -8)
	op()
	asm.MovSDStoreIndex(R15, R11, 3, -16, XMM1)
	asm.DecRR(R11)
}

func (cs *compileState) emitCompare(instr Instruction, cond Cond, req Request) {
	asm := cs.asm
	if !cs.direct {
		notNumber := cs.bridgeLabel(instr.IP, req)
		cs.loadTop(RAX, 1)
		cs.isNumberJump(RAX, notNumber)
		cs.loadTop(RCX, 0)
		cs.isNumberJump(RCX, notNumber)
	}
	asm.MovSDLoadIndex(XMM0, R15, R11, 3, -16)
	asm.MovSDLoadIndex(XMM1, R15, R11, 3, -8)
	asm.Ucomisd(XMM0, XMM1)
	asm.SetCC(cond, RCX)
	unorderedSafe := false
	switch cond {
	case CondB, CondBE, CondA, CondAE:
		unorderedSafe = true
	}
	if unorderedSafe {
		asm.SetCC(CondNP, RDX)
		asm.MovZX(RCX, RCX)
		asm.MovZX(RDX, RDX)
		asm.AndRR(RCX, RDX)
	} else {
		asm.MovZX(RCX, RCX)
	}
	cs.boolFromRCX()
	cs.storeTop(RAX, 1)
	asm.DecRR(R11)
}

func (cs *compileState) emitDirectCall(instr Instruction) {
	asm := cs.asm
	argc := instr.Operands[0]
	fail := cs.bridgeLabel(instr.IP, ReqCall)
	cs.loadTop(RAX, 0)
	asm.MovLoad(RCX, RBX, ActivationSelfValue)
	asm.CmpRR(RAX, RCX)
	asm.Jcc(CondNE, fail)
	asm.IncLoad(RBX, ActivationDepth)
	asm.CmpLoadImm32(RBX, ActivationDepth, MaxJITFrames)
	asm.Jcc(CondGE, cs.stackOverflowLabel())
	for i := 0; i < argc; i++ {
		cs.loadTop(RAX, argc-i)
		asm.MovStore(R15, int32((cs.frameSlots+i)*8), RAX)
	}
	asm.Push(R11)
	asm.MovImm32(R11, int32(argc))
	asm.AddImm32(R15, int32(cs.frameSlots*8))
	asm.IncLoad(RBX, ActivationDirectDepth)
	asm.Call(cs.fastThunk)
	asm.DecLoad(RBX, ActivationDirectDepth)
	asm.SubImm32(R15, int32(cs.frameSlots*8))
	asm.Pop(R11)
	propagate := asm.NewLabel()
	done := asm.NewLabel()
	asm.CmpLoadImm32(RBX, ActivationStatus, 0)
	asm.Jcc(CondNE, propagate)
	asm.DecLoad(RBX, ActivationDepth)
	asm.MovLoad(RAX, RBX, ActivationResult)
	cs.storeTop(RAX, argc)
	asm.SubImm32(R11, int32(argc))
	asm.Jmp(done)
	asm.Bind(propagate)
	asm.Jmp(cs.exitLabel)
	asm.Bind(done)
}

func (cs *compileState) emitStrictEquals(instr Instruction, negate bool) {
	asm := cs.asm
	req := ReqStrictEquals
	if negate {
		req = ReqStrictNotEquals
	}
	if !cs.direct {
		notNumber := cs.bridgeLabel(instr.IP, req)
		cs.loadTop(RAX, 1)
		cs.isNumberJump(RAX, notNumber)
		cs.loadTop(RCX, 0)
		cs.isNumberJump(RCX, notNumber)
	}
	asm.MovSDLoadIndex(XMM0, R15, R11, 3, -16)
	asm.MovSDLoadIndex(XMM1, R15, R11, 3, -8)
	asm.Ucomisd(XMM0, XMM1)
	asm.SetCC(CondE, RCX)
	asm.SetCC(CondNP, RDX)
	asm.MovZX(RCX, RCX)
	asm.MovZX(RDX, RDX)
	asm.AndRR(RCX, RDX)
	if negate {
		asm.XorImm32(RCX, 1)
	}
	cs.boolFromRCX()
	cs.storeTop(RAX, 1)
	asm.DecRR(R11)
}
