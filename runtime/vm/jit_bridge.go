package vm

import (
	"fmt"
	"os"
	"runtime"
	"sync"

	"go_js/chunk"
	"go_js/flags"
	"go_js/heap"
	"go_js/jit"
	"go_js/object"
	"go_js/value"
)

var (
	sharedJITOnce    sync.Once
	sharedJITContext *jit.Context
	jitTrace         = os.Getenv("JIT_TRACE") == "1"
)

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
		if jitTrace {
			fmt.Fprintf(os.Stderr, "JIT no entry 0; entries=%v\n", cc.Entries)
		}
		release()
		return jitUnavailable, nil
	}

	if flags.Debug {
		fmt.Println("-- CALLING JIT FUNCTION --")
	}

	for {
		act.Status = jit.StatusDone
		status, enterErr := ctx.Enter(cc, entry, act)
		if jitTrace {
			fmt.Fprintf(os.Stderr, "JIT seg entry=%x status=%d req=%d ip=%d sp=%d res=%x frame=%d dd=%d self=%x\n", entry, status, act.Request, act.BytecodeIP, act.SP, uint64(act.Result), vm.frameCount, act.DirectDepth, uint64(act.SelfValue))
		}
		if enterErr != nil {
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
			release()
			return jitFinished, nil
		case jit.StatusDeopt:
			if act.DirectDepth > 1 {
				release()
				return jitDeoptimized, fmt.Errorf("jit: deopt inside a direct-recursion frame (ip %d)", act.BytecodeIP)
			}
			ip := int(act.BytecodeIP)
			vm.copyActivationToStack(act, localStart)
			if currentIp != nil {
				*currentIp = ip
			}
			release()
			return jitDeoptimized, nil
		case jit.StatusBridge:
			if act.Request == jit.ReqStackOverflow {
				thrown := vm.makeError("RangeError", "Maximum call stack size exceeded")
				vm.copyActivationToStack(act, localStart)
				outcome, throwErr := vm.resumeAfterJITThrow(f, c, currentIp, int(act.BytecodeIP), thrown)
				release()
				return outcome, throwErr
			}
			if act.DirectDepth > 1 {
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
					release()
					return outcome, throwErr
				}
				release()
				return jitDeoptimized, bridgeErr
			}
			act.Globals = vm.globalsBase()
			act.GlobalsCount = int64(globalsCount)
			act.GlobalSync = vm.globalSyncBase()
			entry, ok = cc.Entry(next)
			if !ok {
				vm.copyActivationToStack(act, localStart)
				if currentIp != nil {
					*currentIp = next
				}
				release()
				return jitDeoptimized, nil
			}
		case jit.StatusThrow:
			thrown := act.Result
			vm.copyActivationToStack(act, localStart)
			outcome, throwErr := vm.resumeAfterJITThrow(f, c, currentIp, int(act.BytecodeIP), thrown)
			release()
			return outcome, throwErr
		default:
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
	if jitTrace {
		fmt.Fprintf(os.Stderr, "BRIDGE ip=%d req=%d gc=%d\n", ip, act.Request, act.GlobalsCount)
	}
	next, ok := jit.InstructionEnd(c.Code, ip)
	if !ok {
		return 0, fmt.Errorf("jit: invalid bridge bytecode ip %d", ip)
	}
	if act.Request == jit.ReqGetMember {
		if err := vm.bridgeGetMember(act); err != nil {
			return 0, err
		}
		return next, nil
	}

	code := make([]byte, 0, next-ip+1)
	code = append(code, c.Code[ip:next]...)
	code = append(code, chunk.OP_JIT_HALT)
	rc := &value.ValueChunk{
		Code:      code,
		AstId:     make([]int, len(code)),
		Constants: c.Constants,
		FnName:    jitBridgeFnName,
	}
	dummy := object.NewFunction(jitBridgeFnName, 0, rc)

	frame := CallFrame{}
	frame.initCallFrame(dummy, localStart, 0, act.This)
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
