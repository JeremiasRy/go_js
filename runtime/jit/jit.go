package jit

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"go_js/chunk"
	"go_js/object"
	"go_js/value"
)

type JitDetail struct {
	Inst      string `json:"inst"`
	OpCodePtr int    `json:"op_code_ptr"`
	AstId     int    `json:"ast_id"`
}

type Status int64

const (
	StatusDone Status = iota
	StatusDeopt
	StatusBridge
	StatusThrow
)

type Request int64

const (
	ReqNone Request = iota
	ReqStackOverflow
	ReqThrow
	ReqAdd
	ReqSubtract
	ReqMultiply
	ReqDivide
	ReqModulo
	ReqExponentiation
	ReqNegate
	ReqEquals
	ReqNotEquals
	ReqStrictEquals
	ReqStrictNotEquals
	ReqLessThan
	ReqLessThanEqual
	ReqGreaterThan
	ReqGreaterThanEqual
	ReqBitAnd
	ReqBitOr
	ReqBitXor
	ReqBitwiseNot
	ReqLeftShift
	ReqRightShift
	ReqUnsignedRightShift
	ReqNot
	ReqToNumber
	ReqToString
	ReqToPropertyKey
	ReqTypeof
	ReqVoid
	ReqUpdate
	ReqCall
	ReqCallOptional
	ReqCallCalleeFirst
	ReqNew
	ReqSuperCall
	ReqGetMember
	ReqSetMemberReturn
	ReqSetMemberKeep
	ReqSetMemberStrictReturn
	ReqDelete
	ReqDeleteStrict
	ReqIn
	ReqInstanceof
	ReqCreateObject
	ReqCreateArray
	ReqPushElement
	ReqGetIterator
	ReqIteratorNextSlot
	ReqIteratorCurrentSlot
	ReqIteratorCloseSlot
	ReqIteratorValueOrUndefined
	ReqIteratorRest
	ReqIteratorClosePop
	ReqGetHeapVar
	ReqSetHeapVar
	ReqGetHeapVarRaw
	ReqDefineHeapVar
	ReqDefineHeapVarAt
	ReqCreateHeapScope
	ReqCopyHeapScope
	ReqEnsureHeapScope
	ReqDefineLocal
	ReqDefineLocalSlot
	ReqSetLocal
	ReqGetGlobal
	ReqSetGlobal
	ReqDefineGlobal
	ReqPushCurrent
	ReqThis
	ReqUnwrapMethod
	ReqPushGlobalObject
	ReqCreateRestObject
	ReqCreateRestObjectDynamic
	ReqSpread
	ReqSpreadIntoArray
	ReqDefineHeapVarsFromArguments
	ReqSetFromSpread
	ReqJumpIfNullish
	ReqJumpIfNotUndefined
	ReqJumpIfNotNullish
	ReqNewTarget
	ReqThrowValue
	ReqDeopt
	ReqGetCatch
	ReqSetCatch
	ReqSafepoint
	ReqSaveCompletion
	ReqRestoreCompletion
	ReqDefined
)

// Activation is the fixed-layout bridge between Go and compiled code. The
// offsets of the fields up to SelfValue are part of the JIT ABI and are
// asserted by activation_layout_test.go.
type Activation struct {
	Values       *value.Value
	Globals      *value.Value
	Constants    *value.Value
	SP           int64
	Request      Request
	BytecodeIP   int64
	Status       Status
	Result       value.Value
	Arg0         value.Value
	Arg1         value.Value
	Arg2         value.Value
	Arg3         value.Value
	Depth        int64
	SelfValue    value.Value
	GoSP         uintptr
	GlobalsCount int64
	DirectDepth  int64
	GlobalSync   *uint64
	LoopBudget   int64

	Chunk       *value.ValueChunk
	Fn          object.Callable
	LocalCount  int
	FrameSlots  int
	Buffer      []value.Value
	EvalBase    int
	This        value.Value
	HeapScope   int
	NewTarget   value.Value
	OwnScope    bool
	Args        value.Value
	ReturnValue value.Value
	CallDepth   int
	DirectReady bool
}

const (
	ActivationValues       = 0
	ActivationGlobals      = 8
	ActivationConstants    = 16
	ActivationSP           = 24
	ActivationRequest      = 32
	ActivationBytecodeIP   = 40
	ActivationStatus       = 48
	ActivationResult       = 56
	ActivationArg0         = 64
	ActivationArg1         = 72
	ActivationArg2         = 80
	ActivationArg3         = 88
	ActivationDepth        = 96
	ActivationSelfValue    = 104
	ActivationGoSP         = 112
	ActivationGlobalsCount = 120
	ActivationDirectDepth  = 128
	ActivationGlobalSync   = 136
	ActivationLoopBudget   = 144
)

const DefaultLoopBudget = 65536

const JITNativeStackSize = 64 * 1024
const JITNativeStackSlack = 512
const MaxJITFrames = 1024

type nativeStack struct {
	mem   []byte
	top   uintptr
	limit uintptr
}

func newNativeStack() (*nativeStack, error) {
	page := os.Getpagesize()
	mem, err := syscall.Mmap(
		-1,
		0,
		JITNativeStackSize+page,
		syscall.PROT_NONE,
		syscall.MAP_PRIVATE|syscall.MAP_ANON,
	)
	if err != nil {
		return nil, err
	}
	if err := syscall.Mprotect(mem[page:], syscall.PROT_READ|syscall.PROT_WRITE); err != nil {
		syscall.Munmap(mem)
		return nil, err
	}
	top := uintptr(unsafe.Pointer(&mem[len(mem)-16])) &^ uintptr(15)
	limit := uintptr(unsafe.Pointer(&mem[page])) + JITNativeStackSlack
	return &nativeStack{mem: mem, top: top, limit: limit}, nil
}

type StackPool struct {
	mu   sync.Mutex
	free []*nativeStack
}

func (p *StackPool) acquire() (*nativeStack, error) {
	p.mu.Lock()
	if n := len(p.free); n > 0 {
		s := p.free[n-1]
		p.free = p.free[:n-1]
		p.mu.Unlock()
		return s, nil
	}
	p.mu.Unlock()
	return newNativeStack()
}

func (p *StackPool) release(s *nativeStack) {
	p.mu.Lock()
	p.free = append(p.free, s)
	p.mu.Unlock()
}

type Stats struct {
	mu              sync.Mutex
	Compiled        int64
	CompileFailures int64
	Deopts          int64
	BridgeRequests  int64
	OSREntries      int64
	Safepoints      int64
	CodeBytes       int64
	CodeLimitHits   int64
	DeoptReasons    map[string]int64
}

func (s *Stats) NoteDeopt(reason string) {
	atomic.AddInt64(&s.Deopts, 1)
	s.mu.Lock()
	if s.DeoptReasons == nil {
		s.DeoptReasons = map[string]int64{}
	}
	s.DeoptReasons[reason]++
	s.mu.Unlock()
}

func (s *Stats) NoteBridge() {
	atomic.AddInt64(&s.BridgeRequests, 1)
}

func (s *Stats) NoteOSR() {
	atomic.AddInt64(&s.OSREntries, 1)
}

func (s *Stats) NoteSafepoint() {
	atomic.AddInt64(&s.Safepoints, 1)
}

func (s *Stats) NoteFail(reason string) {
	atomic.AddInt64(&s.CompileFailures, 1)
	s.mu.Lock()
	if s.DeoptReasons == nil {
		s.DeoptReasons = map[string]int64{}
	}
	s.DeoptReasons["compile: "+reason]++
	s.mu.Unlock()
}

type CompiledChunk struct {
	Chunk       *value.ValueChunk
	Code        []byte
	Mem         []byte
	Entries     map[int]uintptr
	LocalCount  int
	FrameSlots  int
	MaxSP       int
	Arity       int
	FastEntry   uintptr
	Direct      bool
	Disassembly []JitDetail
	CompileErr  string
}

func (cc *CompiledChunk) Entry(ip int) (uintptr, bool) {
	offset, ok := cc.Entries[ip]
	if !ok {
		return 0, false
	}
	return uintptr(unsafe.Pointer(&cc.Mem[0])) + offset, true
}

type loopKey struct {
	chunk *value.ValueChunk
	ip    int
}

type Context struct {
	mu           sync.Mutex
	compiled     map[*value.ValueChunk]*CompiledChunk
	failures     map[*value.ValueChunk]string
	backedges    map[loopKey]int
	blacklisted  map[loopKey]bool
	deopts       map[*value.ValueChunk]int
	codeBytes    int
	maxCodeBytes int
	stacks       StackPool
	stats        Stats
	activations  sync.Pool
}

func NewContext(maxCodeBytes int) *Context {
	if maxCodeBytes <= 0 {
		maxCodeBytes = 16 << 20
	}
	ctx := &Context{
		compiled:     map[*value.ValueChunk]*CompiledChunk{},
		failures:     map[*value.ValueChunk]string{},
		backedges:    map[loopKey]int{},
		blacklisted:  map[loopKey]bool{},
		deopts:       map[*value.ValueChunk]int{},
		maxCodeBytes: maxCodeBytes,
	}
	ctx.activations.New = func() any { return &Activation{} }
	return ctx
}

func (ctx *Context) Stats() *Stats { return &ctx.stats }

// NoteBackedge counts a backward branch and reports whether the loop is hot
// enough to attempt OSR.
func (ctx *Context) NoteBackedge(chunk *value.ValueChunk, ip, threshold int) bool {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	key := loopKey{chunk: chunk, ip: ip}
	if ctx.blacklisted[key] {
		return false
	}
	ctx.backedges[key]++
	return ctx.backedges[key] >= threshold
}

func (ctx *Context) BlacklistLoop(chunk *value.ValueChunk, ip int) {
	ctx.mu.Lock()
	ctx.blacklisted[loopKey{chunk: chunk, ip: ip}] = true
	ctx.mu.Unlock()
}

// NoteDeopt records a deoptimization and reports whether the chunk should stop
// being compiled, to avoid deopt storms.
func (ctx *Context) NoteDeopt(chunk *value.ValueChunk) bool {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.deopts[chunk]++
	return ctx.deopts[chunk] > 16
}

func (ctx *Context) BlacklistChunkOSR(chunk *value.ValueChunk) {
	ctx.mu.Lock()
	for key := range ctx.backedges {
		if key.chunk == chunk {
			ctx.blacklisted[key] = true
		}
	}
	ctx.mu.Unlock()
}

func (ctx *Context) AcquireActivation() *Activation {
	act := ctx.activations.Get().(*Activation)
	*act = Activation{}
	return act
}

func (ctx *Context) ReleaseActivation(act *Activation) {
	act.Buffer = nil
	act.Chunk = nil
	act.Fn = nil
	ctx.activations.Put(act)
}

func (ctx *Context) acquireStack() (*nativeStack, error) {
	return ctx.stacks.acquire()
}

func (ctx *Context) releaseStack(s *nativeStack) {
	ctx.stacks.release(s)
}

func (ctx *Context) Compiled(chunk *value.ValueChunk) (*CompiledChunk, bool) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	cc, ok := ctx.compiled[chunk]
	return cc, ok
}

func (ctx *Context) Enter(cc *CompiledChunk, entry uintptr, act *Activation) (Status, error) {
	stack, err := ctx.acquireStack()
	if err != nil {
		return StatusThrow, err
	}
	status := jitEnter(entry, act, stack.top, stack.limit)
	ctx.releaseStack(stack)
	return Status(status), nil
}

func (ctx *Context) Disassembly(chunk *value.ValueChunk) (bool, []JitDetail) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if cc, ok := ctx.compiled[chunk]; ok {
		return true, cc.Disassembly
	}
	return false, nil
}

func (ctx *Context) seal(code []byte) ([]byte, error) {
	size := len(code)
	if size == 0 {
		return nil, fmt.Errorf("jit: empty code")
	}
	page := os.Getpagesize()
	rounded := (size + page - 1) &^ (page - 1)
	if ctx.codeBytes+rounded > ctx.maxCodeBytes {
		atomic.AddInt64(&ctx.stats.CodeLimitHits, 1)
		return nil, fmt.Errorf("jit: code cache limit reached (%d bytes)", ctx.maxCodeBytes)
	}
	mem, err := syscall.Mmap(
		-1,
		0,
		rounded,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_PRIVATE|syscall.MAP_ANON,
	)
	if err != nil {
		return nil, err
	}
	copy(mem, code)
	if err := syscall.Mprotect(mem, syscall.PROT_READ|syscall.PROT_EXEC); err != nil {
		syscall.Munmap(mem)
		return nil, err
	}
	ctx.codeBytes += rounded
	atomic.AddInt64(&ctx.stats.CodeBytes, int64(rounded))
	return mem, nil
}

func (ctx *Context) CanCompile(fn object.Callable) bool {
	if fn == nil {
		return false
	}
	if _, ok := fn.(*object.ObjFunction); !ok {
		return false
	}
	c := fn.ValueChunk()
	if c == nil || len(c.Code) == 0 {
		return false
	}
	if fn.HasArguments() || fn.HasRestParameter() {
		return false
	}
	if hasInterpreterOnlyOps(c.Code) {
		return false
	}
	return true
}

func hasInterpreterOnlyOps(code []byte) bool {
	for _, op := range code {
		switch op {
		case chunk.OP_TRY_BLOCK_START, chunk.OP_TRY_BLOCK_END, chunk.OP_TRY_FINALLY_START,
			chunk.OP_THROW, chunk.OP_CATCH_BIND, chunk.OP_GET_CATCH, chunk.OP_SET_CATCH,
			chunk.OP_AWAIT, chunk.OP_YIELD, chunk.OP_YIELD_DELEGATE,
			chunk.OP_GET_ASYNC_ITERATOR, chunk.OP_ASYNC_ITERATOR_NEXT_SLOT,
			chunk.OP_ASYNC_ITERATOR_RESULT_SLOT, chunk.OP_ASYNC_ITERATOR_VALUE_SLOT,
			chunk.OP_ASYNC_ITERATOR_CLOSE_SLOT, chunk.OP_ENTER_WITH, chunk.OP_EXIT_WITH,
			chunk.OP_WITH_GET, chunk.OP_WITH_SET,
			chunk.OP_SAVE_COMPLETION, chunk.OP_RESTORE_COMPLETION:
			return true
		}
	}
	return false
}

func (ctx *Context) Compile(fn object.Callable) (*CompiledChunk, error) {
	if !ctx.CanCompile(fn) {
		return nil, fmt.Errorf("jit: function is not compilable")
	}
	chunk := fn.ValueChunk()
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if cc, ok := ctx.compiled[chunk]; ok {
		if cc.CompileErr != "" {
			return nil, fmt.Errorf("%s", cc.CompileErr)
		}
		return cc, nil
	}
	if reason, failed := ctx.failures[chunk]; failed {
		return nil, fmt.Errorf("%s", reason)
	}
	cc, err := ctx.compileLocked(chunk, fn.GetArity())
	if err != nil {
		ctx.failures[chunk] = err.Error()
		ctx.stats.NoteFail(err.Error())
		cc = &CompiledChunk{Chunk: chunk, CompileErr: err.Error()}
		ctx.compiled[chunk] = cc
		return nil, err
	}
	ctx.compiled[chunk] = cc
	atomic.AddInt64(&ctx.stats.Compiled, 1)
	return cc, nil
}
