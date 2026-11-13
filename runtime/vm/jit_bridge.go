package vm

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"

	"go_js/chunk"
	"go_js/flags"
	"go_js/heap"
	"go_js/jit"
	"go_js/native"
	"go_js/object"
	"go_js/value"
)

var (
	sharedJITOnce    sync.Once
	sharedJITContext *jit.Context
)

func PrintJITStats(w io.Writer) {
	jitContext().PrintStats(w)
}

func jitTracing() bool {
	return flags.JITTrace || os.Getenv("JIT_TRACE") == "1"
}

func jitContext() *jit.Context {
	sharedJITOnce.Do(func() {
		sharedJITContext = jit.NewContext(flags.JITMaxCodeBytes)
	})
	return sharedJITContext
}

type jitOutcome int

const (
	jitUnavailable jitOutcome = iota
	jitFinished
	jitDeoptimized
	jitResumed
)

const jitBridgeFnName = "JIT_BRIDGE"

// runJIT executes a compiled chunk until it completes, deoptimizes, or throws.
// It is always called with the JIT frame at the top of the VM frame stack.
func (vm *VM) runJIT(f *CallFrame, c *value.ValueChunk, currentIp *int, callee value.Value) (jitOutcome, error) {
	ctx := jitContext()
	jitFn := callableName(f.fn)
	cc, err := ctx.Compile(f.fn)
	if err != nil {
		if flags.Debug {
			fmt.Printf("-- JIT COMPILE FAILED: %s --\n", err)
		}
		return jitUnavailable, nil
	}

	localStart := f.localStart
	top := vm.stackTop - localStart
	if top < 0 {
		top = 0
	}
	if cc.Direct && top > cc.FrameSlots {
		return jitUnavailable, nil
	}
	bufferSize := cc.FrameSlots
	if cc.Direct {
		bufferSize = cc.FrameSlots * jit.MaxJITFrames
	}
	if top+8 > bufferSize {
		bufferSize = top + 8
	}
	buffer := make([]value.Value, bufferSize)
	for i := range buffer {
		buffer[i] = value.UNDEFINED
	}
	copy(buffer, vm.stack[localStart:vm.stackTop])

	act := ctx.AcquireActivation()
	act.Buffer = buffer
	act.Values = &buffer[0]
	act.Globals = vm.globalsBase()
	act.GlobalsCount = int64(globalsCount)
	act.GlobalSync = vm.globalSyncBase()
	if len(c.Constants) > 0 {
		act.Constants = &c.Constants[0]
	}
	act.SP = int64(top)
	act.Request = jit.ReqNone
	act.BytecodeIP = 0
	act.Status = jit.StatusDone
	act.Depth = int64(vm.frameCount)
	act.DirectDepth = 1
	act.LoopBudget = jit.DefaultLoopBudget
	act.SelfValue = callee
	act.Chunk = c
	act.Fn = f.fn
	act.LocalCount = cc.LocalCount
	act.FrameSlots = cc.FrameSlots
	act.This = f.thisCtx
	act.HeapScope = f.heapScope
	act.NewTarget = f.newTarget
	act.OwnScope = f.ownScope
	act.Args = f.arguments

	release := func() {
		ctx.ReleaseActivation(act)
	}

	entry, ok := cc.Entry(0)
	if !ok {
		if jitTracing() {
			fmt.Fprintf(os.Stderr, "JIT no entry 0; entries=%v\n", cc.Entries)
		}
		release()
		return jitUnavailable, nil
	}

	jitChunk := *c
	tracingJIT := vm.traceEnabled
	jitStart := time.Now()
	if tracingJIT {
		vm.traceJITEnter(jitFn, &jitChunk, 0, "call")
	}
	exitJIT := func(status string, result value.Value) {
		if tracingJIT {
			vm.traceJITExit(jitFn, &jitChunk, 0, "call", status, result, time.Since(jitStart), intPtrValue(currentIp))
		}
	}

	if flags.Debug {
		fmt.Println("-- CALLING JIT FUNCTION --")
	}

	for {
		act.Status = jit.StatusDone
		status, enterErr := ctx.Enter(cc, entry, act)
		if jitTracing() {
			fmt.Fprintf(os.Stderr, "JIT seg entry=%x status=%d req=%d ip=%d sp=%d res=%x frame=%d dd=%d self=%x this=%x\n", entry, status, act.Request, act.BytecodeIP, act.SP, uint64(act.Result), vm.frameCount, act.DirectDepth, uint64(act.SelfValue), uint64(act.This))
		}
		if enterErr != nil {
			exitJIT("deopt", value.UNDEFINED)
			release()
			return jitDeoptimized, enterErr
		}
		switch status {
		case jit.StatusDone:
			result := act.Result
			returnIp := f.returnIp
			vm.frameCount--
			vm.stackTop = localStart
			vm.push(result)
			if vm.frameCount > 0 {
				*f = vm.currentFrame()
				*c = *f.fn.ValueChunk()
			}
			if currentIp != nil {
				*currentIp = returnIp
			}
			if flags.Debug {
				fmt.Println("-- JIT CALL DONE --")
			}
			exitJIT("done", result)
			release()
			return jitFinished, nil
		case jit.StatusDeopt:
			if act.DirectDepth > 1 {
				exitJIT("deopt", value.UNDEFINED)
				release()
				return jitDeoptimized, fmt.Errorf("jit: deopt inside a direct-recursion frame (ip %d)", act.BytecodeIP)
			}
			ip := int(act.BytecodeIP)
			if jitTracing() {
				n := int(act.SP)
				if n > 6 {
					n = 6
				}
				fmt.Fprintf(os.Stderr, "JIT deopt fn=%s ip=%d sp=%d vals=%x\n", c.FnName, ip, act.SP, act.Buffer[:n])
			}
			ctx.Stats().NoteDeopt("runtime")
			if ctx.NoteDeopt(c) {
				ctx.BlacklistChunk(c)
			}
			vm.copyActivationToStack(act, localStart)
			f.heapScope = act.HeapScope
			f.ownScope = act.OwnScope
			f.arguments = act.Args
			vm.frames[vm.frameCount-1] = *f
			if currentIp != nil {
				*currentIp = ip
			}
			exitJIT("deopt", value.UNDEFINED)
			release()
			return jitDeoptimized, nil
		case jit.StatusBridge:
			if act.Request == jit.ReqStackOverflow {
				thrown := vm.makeError("RangeError", "Maximum call stack size exceeded")
				vm.copyActivationToStack(act, localStart)
				outcome, throwErr := vm.resumeAfterJITThrow(f, c, currentIp, int(act.BytecodeIP), thrown)
				exitJIT("throw", thrown)
				release()
				return outcome, throwErr
			}
			if act.DirectDepth > 1 {
				exitJIT("deopt", value.UNDEFINED)
				release()
				return jitDeoptimized, fmt.Errorf("jit: bridge inside a direct-recursion frame (ip %d, request %d)", act.BytecodeIP, act.Request)
			}
			vm.copyActivationToStack(act, localStart)
			var next int
			var bridgeErr error
			if act.Request == jit.ReqSafepoint {
				next, bridgeErr = vm.handleSafepoint(act)
			} else {
				next, bridgeErr = vm.handleBridge(act, localStart)
			}
			if bridgeErr != nil {
				var jsErr *JSException
				if asJSException(bridgeErr, &jsErr) && jsErr.HasValue {
					outcome, throwErr := vm.resumeAfterJITThrow(f, c, currentIp, int(act.BytecodeIP), jsErr.Value)
					exitJIT("throw", jsErr.Value)
					release()
					return outcome, throwErr
				}
				exitJIT("deopt", value.UNDEFINED)
				release()
				return jitDeoptimized, bridgeErr
			}
			act.Globals = vm.globalsBase()
			act.GlobalsCount = int64(globalsCount)
			act.GlobalSync = vm.globalSyncBase()
			entry, ok = cc.Entry(next)
			if !ok {
				vm.copyActivationToStack(act, localStart)
				f.heapScope = act.HeapScope
				f.ownScope = act.OwnScope
				f.arguments = act.Args
				vm.frames[vm.frameCount-1] = *f
				if currentIp != nil {
					*currentIp = next
				}
				exitJIT("deopt", value.UNDEFINED)
				release()
				return jitDeoptimized, nil
			}
		case jit.StatusThrow:
			thrown := act.Result
			vm.copyActivationToStack(act, localStart)
			outcome, throwErr := vm.resumeAfterJITThrow(f, c, currentIp, int(act.BytecodeIP), thrown)
			exitJIT("throw", thrown)
			release()
			return outcome, throwErr
		default:
			exitJIT("deopt", value.UNDEFINED)
			release()
			return jitDeoptimized, fmt.Errorf("jit: unknown status %d", status)
		}
	}
}

func asJSException(err error, target **JSException) bool {
	if e, ok := err.(*JSException); ok {
		*target = e
		return true
	}
	return false
}

func (vm *VM) resumeAfterJITThrow(f *CallFrame, c *value.ValueChunk, currentIp *int, ip int, thrown value.Value) (jitOutcome, error) {
	ff := *f
	cc := *c
	resumeIp := ip
	if err := vm.throwValue(thrown, &ff, &cc, &resumeIp); err != nil {
		return jitDeoptimized, err
	}
	*f = ff
	*c = cc
	if currentIp != nil {
		*currentIp = resumeIp
	}
	return jitResumed, nil
}

func (vm *VM) globalSyncBase() *uint64 {
	for len(globalSync) < globalsCount {
		globalSync = append(globalSync, 0)
	}
	if len(globalSync) == 0 {
		return nil
	}
	return &globalSync[0]
}

func (vm *VM) globalsBase() *value.Value {
	if len(globals) == 0 {
		return nil
	}
	return &globals[0]
}

func (vm *VM) copyActivationToStack(act *jit.Activation, localStart int) {
	n := int(act.SP)
	if n < 0 {
		n = 0
	}
	vm.ensureStack(localStart + n + 1)
	if n > len(act.Buffer) {
		n = len(act.Buffer)
	}
	copy(vm.stack[localStart:localStart+n], act.Buffer[:n])
	vm.stackTop = localStart + n
}

func (vm *VM) syncActivationFromStack(act *jit.Activation, localStart int) {
	n := vm.stackTop - localStart
	if n < 0 {
		n = 0
	}
	if n > len(act.Buffer) {
		buffer := make([]value.Value, n+32)
		for i := range buffer {
			buffer[i] = value.UNDEFINED
		}
		copy(buffer, act.Buffer)
		act.Buffer = buffer
		act.Values = &buffer[0]
	}
	copy(act.Buffer[:n], vm.stack[localStart:vm.stackTop])
	act.SP = int64(n)
}

// handleBridge replays the bytecode instruction at act.BytecodeIP on the same
// VM and returns the bytecode IP to resume at.
func (vm *VM) handleBridge(act *jit.Activation, localStart int) (int, error) {
	c := *act.Chunk
	ip := int(act.BytecodeIP)
	jitContext().Stats().NoteBridge()
	if jitTracing() {
		fmt.Fprintf(os.Stderr, "BRIDGE ip=%d req=%d gc=%d\n", ip, act.Request, act.GlobalsCount)
	}
	next, ok := jit.InstructionEnd(c.Code, ip)
	if !ok {
		return 0, fmt.Errorf("jit: invalid bridge bytecode ip %d", ip)
	}
	switch act.Request {
	case jit.ReqGetMember:
		if err := vm.bridgeGetMember(act); err != nil {
			return 0, err
		}
		return next, nil
	case jit.ReqSetMemberReturn, jit.ReqSetMemberKeep, jit.ReqSetMemberStrictReturn:
		if err := vm.bridgeSetMember(act, act.Request); err != nil {
			return 0, err
		}
		return next, nil
	case jit.ReqPushElement:
		if err := vm.bridgePushElement(act); err != nil {
			return 0, err
		}
		return next, nil
	case jit.ReqCreateArray:
		length, ok := jit.ReadUint32BE(c.Code, ip+1)
		if !ok {
			return 0, fmt.Errorf("jit: truncated array creation")
		}
		arr := native.NewArray(length)
		if err := vm.bridgePush(act, value.EncodeHandle(heap.Allocate(arr))); err != nil {
			return 0, err
		}
		return next, nil
	case jit.ReqCreateObject:
		if err := vm.bridgePush(act, value.EncodeHandle(heap.Allocate(native.NewObjectHash()))); err != nil {
			return 0, err
		}
		return next, nil
	}

	code := make([]byte, 0, next-ip+1)
	code = append(code, c.Code[ip:next]...)
	code = append(code, chunk.OP_JIT_HALT)
	rc := &value.ValueChunk{
		Code:        code,
		AstId:       make([]int, len(code)),
		Constants:   c.Constants,
		FnName:      jitBridgeFnName,
		ChunkID:     value.NextChunkID(),
		TraceHidden: true,
	}
	var bridgeFn object.Callable = object.NewFunction(jitBridgeFnName, 0, rc)
	if real, ok := act.Fn.(*object.ObjFunction); ok {
		clone := real.Clone().(*object.ObjFunction)
		rc.FnName = real.ValueChunk().FnName
		clone.Chunk = rc
		bridgeFn = clone
	}

	frame := CallFrame{}
	frame.initCallFrame(bridgeFn, localStart, 0, act.This)
	frame.heapScope = act.HeapScope
	frame.ownScope = act.OwnScope
	frame.newTarget = act.NewTarget
	frame.arguments = act.Args

	savedFrameCount := vm.frameCount
	savedExceptionStack := vm.exceptionStack
	vm.exceptionStack = nil
	vm.frames[vm.frameCount] = frame
	vm.frameCount++

	vm.syncActivationFromStack(act, localStart)
	_, runErr := vm.run(frame, *rc)

	vm.frameCount = savedFrameCount
	vm.exceptionStack = savedExceptionStack

	current := vm.frames[savedFrameCount]
	act.HeapScope = current.heapScope
	act.OwnScope = current.ownScope

	vm.syncActivationFromStack(act, localStart)
	if runErr != nil {
		return 0, runErr
	}
	return next, nil
}

// handleSafepoint runs deferred runtime work (notably a GC cycle) at a
// compiled loop backedge and returns the bytecode IP to resume at.
func (vm *VM) handleSafepoint(act *jit.Activation) (int, error) {
	handles := heap.HandleCount()
	if handles != vm.lastJITGC && heap.ShouldRunGCCycle() {
		vm.runGC()
		vm.lastJITGC = handles
	}
	runtime.Gosched()
	act.LoopBudget = jit.DefaultLoopBudget
	act.Globals = vm.globalsBase()
	act.GlobalsCount = int64(globalsCount)
	act.GlobalSync = vm.globalSyncBase()
	jitContext().Stats().NoteSafepoint()
	return int(act.BytecodeIP), nil
}

func (vm *VM) bridgePush(act *jit.Activation, v value.Value) error {
	top := int(act.SP)
	if top >= len(act.Buffer) {
		buffer := make([]value.Value, len(act.Buffer)*2+32)
		copy(buffer, act.Buffer)
		act.Buffer = buffer
		act.Values = &buffer[0]
	}
	act.Buffer[top] = v
	act.SP = int64(top + 1)
	return nil
}

func (vm *VM) bridgeSetMember(act *jit.Activation, req jit.Request) error {
	top := int(act.SP)
	strict := req == jit.ReqSetMemberStrictReturn
	if req == jit.ReqSetMemberKeep {
		if top < 4 {
			return fmt.Errorf("jit: member store with an empty stack")
		}
		v := act.Buffer[top-1]
		keep := act.Buffer[top-2]
		key := act.Buffer[top-3]
		obj := act.Buffer[top-4]
		if err := vm.setMemberValueStrict(obj, key, v, strict); err != nil {
			return jitMemberError(vm, err)
		}
		act.Buffer[top-4] = keep
		act.SP = int64(top - 3)
		return nil
	}
	if top < 3 {
		return fmt.Errorf("jit: member store with an empty stack")
	}
	v := act.Buffer[top-1]
	key := act.Buffer[top-2]
	obj := act.Buffer[top-3]
	if err := vm.setMemberValueStrict(obj, key, v, strict); err != nil {
		return jitMemberError(vm, err)
	}
	act.Buffer[top-3] = v
	act.SP = int64(top - 2)
	return nil
}

func jitMemberError(vm *VM, err error) error {
	name := "TypeError"
	message := err.Error()
	if jsErr, ok := err.(*JSException); ok {
		name = jsErr.Name
		message = jsErr.Message
	}
	return &JSException{Name: name, Message: message, Value: vm.makeError(name, message), HasValue: true}
}

func (vm *VM) bridgePushElement(act *jit.Activation) error {
	top := int(act.SP)
	if top < 2 {
		return fmt.Errorf("jit: push element with an empty stack")
	}
	v := act.Buffer[top-1]
	arr := act.Buffer[top-2]
	obj, err := heap.GetObject(arr.GetHandle())
	if err != nil {
		return err
	}
	array, ok := obj.(*native.ObjArr)
	if !ok {
		return fmt.Errorf("trying to initialize %s that is not an array", native.String(arr))
	}
	array.PushElement(v)
	act.SP = int64(top - 1)
	return nil
}

func (vm *VM) bridgeGetMember(act *jit.Activation) error {
	top := int(act.SP)
	if top < 2 {
		return fmt.Errorf("jit: member access with an empty stack")
	}
	member := act.Buffer[top-1]
	objValue := act.Buffer[top-2]
	top -= 2

	if objValue == value.TAG_METHOD_HANDLE {
		if top < 2 {
			return fmt.Errorf("jit: malformed method handle")
		}
		objValue = act.Buffer[top-2]
		top -= 2
	}

	throw := func(name, message string) error {
		act.SP = int64(top)
		return &JSException{Name: name, Message: message, Value: vm.makeError(name, message), HasValue: true}
	}

	if objValue == value.UNDEFINED || objValue == value.NULL {
		return throw("TypeError", "Cannot read properties of null")
	}
	if objValue == value.UNINITIALIZED {
		return throw("ReferenceError", "Cannot access uninitialized variable")
	}

	v, err := vm.getMemberValue(objValue, member)
	if err != nil {
		name := "TypeError"
		message := err.Error()
		if jsErr, ok := err.(*JSException); ok {
			name = jsErr.Name
			message = jsErr.Message
		}
		return throw(name, message)
	}

	push := func(val value.Value) {
		act.Buffer[top] = val
		top++
	}
	if v.IsObject() {
		if memberObj, err := heap.GetObject(v.GetHandle()); err == nil {
			if _, ok := memberObj.(object.NeedsContext); ok {
				push(objValue)
				push(v)
				push(value.TAG_METHOD_HANDLE)
				act.SP = int64(top)
				return nil
			}
		}
	}
	push(v)
	act.SP = int64(top)
	return nil
}
