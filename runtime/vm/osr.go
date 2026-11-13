package vm

import (
	"fmt"
	"os"

	"go_js/flags"
	"go_js/jit"
	"go_js/value"
)

// tryOSR attempts on-stack replacement at a loop header. It either leaves the
// interpreter running (and sets *ip to target), resumes the interpreter at a
// deopt point, or reports that the frame completed.
func (vm *VM) tryOSR(f *CallFrame, c *value.ValueChunk, ip *int, target int) (done bool, result value.Value, err error) {
	if !flags.EnableJIT {
		*ip = target
		return false, value.UNDEFINED, nil
	}
	ctx := jitContext()
	if !ctx.NoteBackedge(c, target, flags.JITLoopThreshold) {
		*ip = target
		return false, value.UNDEFINED, nil
	}
	if !ctx.CanCompile(f.fn) {
		ctx.BlacklistLoop(c, target)
		*ip = target
		return false, value.UNDEFINED, nil
	}
	cc, compileErr := ctx.Compile(f.fn)
	if compileErr != nil {
		ctx.BlacklistLoop(c, target)
		*ip = target
		return false, value.UNDEFINED, nil
	}
	if cc.Direct {
		top := vm.stackTop - f.localStart
		for i := 0; i < top; i++ {
			if !vm.stack[f.localStart+i].IsNumber() {
				ctx.BlacklistLoop(c, target)
				*ip = target
				return false, value.UNDEFINED, nil
			}
		}
	}
	entry, ok := cc.Entry(target)
	if !ok {
		if jitTrace {
			fmt.Fprintf(os.Stderr, "OSR no entry ip=%d entries=%v\n", target, cc.Entries)
		}
		ctx.BlacklistLoop(c, target)
		*ip = target
		return false, value.UNDEFINED, nil
	}

	localStart := f.localStart
	top := vm.stackTop - localStart
	if top < 0 || top > cc.FrameSlots {
		ctx.BlacklistLoop(c, target)
		*ip = target
		return false, value.UNDEFINED, nil
	}
	bufferSize := cc.FrameSlots
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
	act.BytecodeIP = int64(target)
	act.Status = jit.StatusDone
	act.Depth = int64(vm.frameCount)
	act.DirectDepth = 1
	act.LoopBudget = jit.DefaultLoopBudget
	act.SelfValue = value.UNDEFINED
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

	ctx.Stats().NoteOSR()
	if jitTrace {
		fmt.Fprintf(os.Stderr, "OSR enter ip=%d sp=%d frame=%d fn=%s\n", target, top, vm.frameCount, c.FnName)
	}
	for {
		act.Status = jit.StatusDone
		status, enterErr := ctx.Enter(cc, entry, act)
		if enterErr != nil {
			release()
			return false, value.UNDEFINED, enterErr
		}
		switch status {
		case jit.StatusDone:
			returned := act.Result
			returnIP := f.returnIp
			vm.frameCount--
			vm.stackTop = localStart
			vm.push(returned)
			release()
			if vm.frameCount <= 0 {
				vm.stackTop = 0
				return true, returned, nil
			}
			*f = vm.currentFrame()
			*c = *f.fn.ValueChunk()
			*ip = returnIP
			return false, value.UNDEFINED, nil
		case jit.StatusDeopt:
			resume := int(act.BytecodeIP)
			if jitTrace {
				fmt.Fprintf(os.Stderr, "OSR deopt ip=%d\n", resume)
			}
			vm.copyActivationToStack(act, localStart)
			*ip = resume
			if ctx.NoteDeopt(c) {
				ctx.BlacklistChunkOSR(c)
			}
			release()
			return false, value.UNDEFINED, nil
		case jit.StatusBridge:
			if act.Request == jit.ReqStackOverflow {
				thrown := vm.makeError("RangeError", "Maximum call stack size exceeded")
				vm.copyActivationToStack(act, localStart)
				outcome, throwErr := vm.resumeAfterJITThrow(f, c, ip, int(act.BytecodeIP), thrown)
				release()
				return false, value.UNDEFINED, throwErrOrNil(outcome, throwErr)
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
					outcome, throwErr := vm.resumeAfterJITThrow(f, c, ip, int(act.BytecodeIP), jsErr.Value)
					release()
					return false, value.UNDEFINED, throwErrOrNil(outcome, throwErr)
				}
				release()
				return false, value.UNDEFINED, bridgeErr
			}
			act.Globals = vm.globalsBase()
			act.GlobalsCount = int64(globalsCount)
			act.GlobalSync = vm.globalSyncBase()
			entry, ok = cc.Entry(next)
			if !ok {
				vm.copyActivationToStack(act, localStart)
				*ip = next
				release()
				return false, value.UNDEFINED, nil
			}
		case jit.StatusThrow:
			thrown := act.Result
			vm.copyActivationToStack(act, localStart)
			outcome, throwErr := vm.resumeAfterJITThrow(f, c, ip, int(act.BytecodeIP), thrown)
			release()
			return false, value.UNDEFINED, throwErrOrNil(outcome, throwErr)
		default:
			release()
			return false, value.UNDEFINED, fmt.Errorf("jit: unknown OSR status %d", status)
		}
	}
}

func throwErrOrNil(outcome jitOutcome, err error) error {
	if outcome == jitResumed {
		return nil
	}
	return err
}
