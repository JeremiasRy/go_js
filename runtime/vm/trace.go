package vm

import (
	"fmt"
	"math"
	"strconv"
	"sync/atomic"
	"time"

	"go_js/eventloop"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/queue"
	"go_js/trace"
	"go_js/value"
)

const maxTraceCallFrames = 64

var nextVMTraceID atomic.Int64

func newTraceVMID() int {
	return int(nextVMTraceID.Add(1)) + 1
}

func callableName(fn object.Callable) string {
	if fn == nil {
		return ""
	}
	if chunk := fn.ValueChunk(); chunk != nil && chunk.FnName != "" {
		return chunk.FnName
	}
	return fn.String()
}

func astIDAt(c *value.ValueChunk, opIp int) int {
	if c == nil || opIp < 0 || opIp >= len(c.AstId) {
		return -1
	}
	return c.AstId[opIp]
}

func opNameAt(c *value.ValueChunk, opIp int) string {
	if c == nil || opIp < 0 || opIp >= len(c.Code) {
		return ""
	}
	return opName(c.Code[opIp])
}

func (vm *VM) BeginTrace(limits trace.Limits) {
	if vm == nil {
		return
	}
	trace.Enable(limits)
	vm.traceEnabled = true
	trace.SetBooted(false)
	queues := queue.Snapshot()
	loop := vm.loopState(queues, false)
	loop.Phase = "booting"
	vm.appendTraceFrame(trace.Frame{
		Kind:      "boot",
		Note:      "program compiled; event loop about to start",
		EventLoop: loop,
		Queues:    queues,
	})
}

func (vm *VM) FinishTrace(output string) *trace.Execution {
	if !vm.traceEnabled {
		return trace.NewExecutionPlaceholder()
	}
	vm.flushTraceEvents()
	queues := queue.Snapshot()
	frame := trace.Frame{
		Kind: "end",
		Note: "program finished",
		Event: &trace.Event{
			Type:   "done",
			Detail: "program finished successfully",
		},
		EventLoop: trace.EventLoopState{
			Phase:  "done",
			Timers: []trace.TimerItem{},
		},
		Queues: queues,
	}
	trace.AppendFrame(frame, len(output), output)
	execution := trace.ExecutionInfo()
	vm.traceEnabled = false
	trace.Disable()
	return execution
}

func (vm *VM) traceStep(f *CallFrame, c *value.ValueChunk, opIp int, code uint8) {
	if !vm.traceEnabled || c == nil || c.TraceHidden {
		return
	}

	vm.flushTraceEvents()
	queues := queue.Snapshot()
	loop := vm.loopState(queues, false)
	vmState := vm.snapshotVM(f, c, opIp, code)

	trace.SetLastVM(vmState)
	vm.appendTraceFrame(trace.Frame{
		Kind:      "step",
		VM:        vmState,
		EventLoop: loop,
		Queues:    queues,
	})
}

func (vm *VM) traceJobStart(callback object.Callback, priority queue.TaskPriority) {
	if !vm.traceEnabled {
		return
	}
	vm.flushTraceEvents()

	kind := "task"
	queueName := "tasks"
	if priority == queue.MICRO_TASK {
		kind = "micro_task"
		queueName = "micro_tasks"
	}
	label := callback.TraceLabel
	if label == "" {
		label = "callback"
	}
	item := trace.QueueItem{
		ID:     "q" + strconv.FormatUint(callback.TraceID, 10),
		Kind:   kind,
		Label:  label,
		Source: callback.TraceSource,
		Native: callback.Native != nil,
	}
	if callback.Fn != nil {
		item.Fn = callableName(callback.Fn)
	}

	trace.SetBooted(true)
	trace.SetCurrentJob(&item)

	queues := queue.Snapshot()
	loop := vm.loopState(queues, false)
	eventItem := item
	event := &trace.Event{
		Type:   "job_start",
		Queue:  queueName,
		Item:   &eventItem,
		Detail: "started " + label,
	}
	vm.appendTraceFrame(trace.Frame{
		Kind:      "event",
		Note:      "running " + label,
		Event:     event,
		VM:        trace.LastVM(),
		EventLoop: loop,
		Queues:    queues,
	})
}

func (vm *VM) traceJobEnd() {
	if !vm.traceEnabled {
		return
	}
	vm.flushTraceEvents()

	job := trace.CurrentJob()
	queues := queue.Snapshot()
	loop := vm.loopState(queues, false)
	var event *trace.Event
	note := "finished the current job"
	if job != nil {
		event = &trace.Event{
			Type:   "job_end",
			Queue:  queueNameForItem(job),
			Item:   job,
			Detail: "finished " + job.Label,
		}
		note = "finished " + job.Label
	}
	vm.appendTraceFrame(trace.Frame{
		Kind:      "event",
		Note:      note,
		Event:     event,
		VM:        trace.LastVM(),
		EventLoop: loop,
		Queues:    queues,
	})
	trace.ClearCurrentJob()
	trace.SetLastVM(nil)
}

func (vm *VM) traceWaiting() {
	if !vm.traceEnabled || !trace.IsBooted() {
		return
	}
	vm.flushTraceEvents()
	queues := queue.Snapshot()
	loop := vm.loopState(queues, false)
	vm.appendTraceFrame(trace.Frame{
		Kind: "event",
		Note: "event loop is waiting for timers or async work",
		Event: &trace.Event{
			Type:   "waiting",
			Detail: "waiting for timers or async jobs",
		},
		EventLoop: loop,
		Queues:    queues,
	})
}

func (vm *VM) traceEvent(event trace.Event, note string) {
	if !vm.traceEnabled {
		return
	}
	vm.flushTraceEvents()
	queues := queue.Snapshot()
	loop := vm.loopState(queues, false)
	eventCopy := event
	vm.appendTraceFrame(trace.Frame{
		Kind:      "event",
		Note:      note,
		Event:     &eventCopy,
		VM:        trace.LastVM(),
		EventLoop: loop,
		Queues:    queues,
	})
}

func (vm *VM) traceTimerScheduled(id, delay int, label string, repeats bool) {
	if !vm.traceEnabled {
		return
	}
	timer := &trace.TimerItem{
		ID:      id,
		DelayMS: delay,
		Label:   label,
		Repeats: repeats,
		State:   "pending",
	}
	vm.traceEvent(trace.Event{
		Type:   "timer_scheduled",
		Timer:  timer,
		Detail: "scheduled " + label,
	}, "scheduled "+label)
}

func (vm *VM) traceTimerCancelled(id int) {
	if !vm.traceEnabled {
		return
	}
	info, found := native.TimerInfoFor(id)
	if !found {
		return
	}
	timer := &trace.TimerItem{
		ID:      info.ID,
		DelayMS: info.DelayMS,
		Label:   info.Label,
		Repeats: info.Repeats,
		State:   info.State,
	}
	vm.traceEvent(trace.Event{
		Type:   "timer_cancelled",
		Timer:  timer,
		Detail: "cancelled " + timer.Label,
	}, "cancelled "+timer.Label)
}

func (vm *VM) flushTraceEvents() {
	if !vm.traceEnabled {
		return
	}
	events := trace.TakeQueueEvents()
	for _, queueEvent := range events {
		loop := vm.loopState(queueEvent.Queues, false)
		item := queueEvent.Item
		note := queueEvent.Detail
		if note == "" {
			note = "queued " + item.Label
		}
		vm.appendTraceFrame(trace.Frame{
			Kind: "event",
			Note: note,
			Event: &trace.Event{
				Type:   queueEvent.Type,
				Queue:  queueEvent.Queue,
				Item:   &item,
				Detail: queueEvent.Detail,
			},
			VM:        trace.LastVM(),
			EventLoop: loop,
			Queues:    queueEvent.Queues,
		})
	}
}

func (vm *VM) appendTraceFrame(frame trace.Frame) {
	if !vm.traceEnabled {
		return
	}
	length := 0
	output := ""
	if vm.output != nil {
		length = vm.output.Len()
		if length > trace.LastOutputLen() {
			output = vm.output.String()
		}
	}
	trace.AppendFrame(frame, length, output)
}

func queueNameForItem(item *trace.QueueItem) string {
	if item != nil && item.Kind == "micro_task" {
		return "micro_tasks"
	}
	return "tasks"
}

func (vm *VM) snapshotVM(f *CallFrame, c *value.ValueChunk, opIp int, code uint8) *trace.VMState {
	limits := trace.CurrentLimits()
	state := &trace.VMState{
		ID:          vm.traceID,
		ParentID:    vm.traceParentID,
		ActiveFrame: -1,
		OpPtr:       opIp,
		Op:          opName(code),
		AstID:       astIDAt(c, opIp),
		Frames:      []trace.VMStackFrame{},
		Stack:       []trace.ValueView{},
	}
	if c != nil {
		state.Fn = c.FnName
		state.ChunkID = c.ChunkID
	}
	if state.Fn == "" && f != nil {
		state.Fn = callableName(f.fn)
	}

	start := 0
	if vm.frameCount > maxTraceCallFrames {
		start = vm.frameCount - maxTraceCallFrames
		state.FramesOmitted = start
	}
	for i := start; i < vm.frameCount; i++ {
		callFrame := vm.frames[i]
		chunk := callFrame.fn.ValueChunk()
		if chunk != nil && chunk.TraceHidden {
			continue
		}
		frameOpPtr := 0
		if i+1 < vm.frameCount {
			frameOpPtr = vm.frames[i+1].returnIp
		}
		frameOp := opNameAt(chunk, frameOpPtr)
		frameAstID := astIDAt(chunk, frameOpPtr)
		frameFn := ""
		frameChunkID := ""
		if chunk != nil {
			frameFn = chunk.FnName
			frameChunkID = chunk.ChunkID
		}
		if frameFn == "" {
			frameFn = callableName(callFrame.fn)
		}
		if i == vm.frameCount-1 {
			frameOpPtr = opIp
			frameOp = opName(code)
			frameAstID = astIDAt(c, opIp)
			if c != nil {
				frameFn = c.FnName
				frameChunkID = c.ChunkID
			}
		}
		state.Frames = append(state.Frames, trace.VMStackFrame{
			Index:      len(state.Frames),
			Fn:         frameFn,
			ChunkID:    frameChunkID,
			OpPtr:      frameOpPtr,
			Op:         frameOp,
			AstID:      frameAstID,
			LocalStart: callFrame.localStart,
		})
	}
	if len(state.Frames) > maxTraceCallFrames {
		drop := len(state.Frames) - maxTraceCallFrames
		state.Frames = state.Frames[drop:]
		state.FramesOmitted += drop
		for i := range state.Frames {
			state.Frames[i].Index = i
		}
	}
	if len(state.Frames) > 0 {
		state.ActiveFrame = len(state.Frames) - 1
		top := state.Frames[state.ActiveFrame]
		state.Fn = top.Fn
		state.ChunkID = top.ChunkID
		state.OpPtr = top.OpPtr
		state.Op = top.Op
		state.AstID = top.AstID
	}

	top := vm.stackTop
	if top < 0 {
		top = 0
	}
	if top > len(vm.stack) {
		top = len(vm.stack)
	}
	startStack := 0
	if limits.MaxStack > 0 && top > limits.MaxStack {
		startStack = top - limits.MaxStack
	}
	for i := startStack; i < top; i++ {
		state.Stack = append(state.Stack, vm.valueView(vm.stack[i]))
	}
	state.StackBase = startStack
	state.StackTop = top
	state.StackOmitted = startStack

	return state
}

func (vm *VM) loopState(queues trace.Queues, done bool) trace.EventLoopState {
	limits := trace.CurrentLimits()
	eventLoop := eventloop.SnapshotState()
	timers := native.TimerSnapshot()
	omitted := 0
	if limits.MaxTimers > 0 && len(timers) > limits.MaxTimers {
		omitted = len(timers) - limits.MaxTimers
		timers = timers[:limits.MaxTimers]
	}
	timerItems := make([]trace.TimerItem, 0, len(timers))
	timerArmed := false
	for _, timer := range timers {
		if timer.State == "pending" {
			timerArmed = true
		}
		timerItems = append(timerItems, trace.TimerItem{
			ID:      timer.ID,
			DelayMS: timer.DelayMS,
			Label:   timer.Label,
			Repeats: timer.Repeats,
			State:   timer.State,
		})
	}

	currentJob := trace.CurrentJob()
	booted := trace.IsBooted()
	phase := "idle"
	nextQueue := ""
	if len(queues.MicroTasks) > 0 {
		nextQueue = "micro_tasks"
	} else if len(queues.Tasks) > 0 {
		nextQueue = "tasks"
	}

	switch {
	case done:
		phase = "done"
	case !booted:
		phase = "booting"
	case currentJob != nil:
		phase = "executing"
	case nextQueue != "":
		phase = "queued"
	case eventLoop.Work > 0 || timerArmed:
		phase = "waiting"
	}

	return trace.EventLoopState{
		Phase:         phase,
		NextQueue:     nextQueue,
		CurrentJob:    currentJob,
		Work:          eventLoop.Work,
		Timers:        timerItems,
		TimersOmitted: omitted,
	}
}

func (vm *VM) valueView(v value.Value) trace.ValueView {
	return vm.valueViewDepth(v, 0, trace.CurrentLimits().MaxValueChars)
}

func (vm *VM) valueViewDepth(v value.Value, depth int, maxChars int) trace.ValueView {
	if maxChars <= 0 {
		maxChars = trace.DefaultMaxValueChars
	}

	switch v {
	case value.NULL:
		return trace.ValueView{Type: "null", Display: "null"}
	case value.UNDEFINED:
		return trace.ValueView{Type: "undefined", Display: "undefined"}
	case value.TRUE:
		return trace.ValueView{Type: "boolean", Display: "true"}
	case value.FALSE:
		return trace.ValueView{Type: "boolean", Display: "false"}
	case value.TAG_METHOD_HANDLE:
		return trace.ValueView{Type: "method_handle", Display: "<method>"}
	case value.TAG_ARGUMENT_START:
		return trace.ValueView{Type: "argument_start", Display: "<arguments>"}
	case value.EMPTY_ARRAY_ITEM:
		return trace.ValueView{Type: "empty", Display: "<empty>"}
	case value.UNINITIALIZED:
		return trace.ValueView{Type: "undefined", Display: "<uninitialized>"}
	}

	if !v.IsObject() {
		if v.IsNumber() {
			n := v.AsNumber()
			display := native.String(v)
			if n == 0 && math.Signbit(n) {
				display = "-0"
			}
			return trace.ValueView{Type: "number", Display: truncateDisplay(display, maxChars)}
		}
		return trace.ValueView{Type: "unknown", Display: "<unknown>"}
	}

	obj, err := heap.GetObject(v.GetHandle())
	if err != nil || obj == nil {
		return trace.ValueView{Type: "unknown", Display: "<invalid>"}
	}

	id := fmt.Sprintf("obj:%d", v.GetHandle())
	view := trace.ValueView{ID: id}

	switch o := obj.(type) {
	case *native.ObjPromise:
		return vm.promiseValueView(o, id, depth, maxChars)
	case *native.ObjAsyncFunction:
		view.Type = "function"
		view.Display = "<fn " + o.Name + ">"
		view.Function = &trace.FunctionView{Name: o.Name, Arity: o.Arity, Async: true}
		return view
	case *native.ObjGenerator:
		view.Type = "function"
		view.Display = o.String()
		view.Function = &trace.FunctionView{Name: o.Name, Arity: o.Arity, Async: o.IsAsync}
		return view
	case *native.ObjArr:
		view.Type = "array"
		view.Display = fmt.Sprintf("Array(%d)", o.Length())
		return view
	case *native.ObjError:
		name, message := native.ErrorNameAndMessage(o)
		display := name
		if message != "" {
			display = name + ": " + message
		}
		view.Type = "error"
		view.Display = truncateDisplay(display, maxChars)
		return view
	case *native.ObjClass:
		view.Type = "class"
		view.Display = truncateDisplay(o.String(), maxChars)
		return view
	case *native.Map:
		view.Type = "map"
		view.Display = o.String()
		return view
	case *native.Set:
		view.Type = "set"
		view.Display = o.String()
		return view
	case *native.ObjRegExp:
		view.Type = "regexp"
		view.Display = truncateDisplay(o.String(), maxChars)
		return view
	case *native.JSIterator:
		view.Type = "iterator"
		view.Display = truncateDisplay(o.String(), maxChars)
		return view
	case *native.LightString:
		view.Type = "string"
		view.Display = quotedDisplay(o.String(), maxChars)
		return view
	case *native.ObjString:
		view.Type = "string"
		view.Display = quotedDisplay(o.String(), maxChars)
		return view
	case *native.ObjStringBuilder:
		view.Type = "string"
		view.Display = quotedDisplay(o.String(), maxChars)
		return view
	case *native.Instance:
		view.Type = "object"
		view.Display = "[Object object]"
		if name := o.Name(); name != "" {
			view.Display = name
		}
		return view
	case *native.ObjObject:
		view.Type = "object"
		view.Display = "[Object object]"
		return view
	}

	if callable, ok := obj.(object.Callable); ok {
		name := callableName(callable)
		view.Type = "function"
		view.Display = "<fn " + name + ">"
		view.Function = &trace.FunctionView{Name: name, Arity: callable.GetArity()}
		return view
	}

	view.Type = "object"
	view.Display = "[Object object]"
	return view
}

func (vm *VM) promiseValueView(p *native.ObjPromise, id string, depth int, maxChars int) trace.ValueView {
	state := "pending"
	switch p.Status {
	case native.RESOLVED:
		state = "fulfilled"
	case native.REJECTED:
		state = "rejected"
	}
	promise := &trace.PromiseView{
		State:     state,
		Reactions: len(p.Reactions),
		Observers: len(p.Observers),
	}
	if depth == 0 && p.Status != native.PENDING {
		settled := vm.valueViewDepth(p.Value, depth+1, maxChars)
		promise.Value = &settled
	}
	return trace.ValueView{
		Type:    "promise",
		Display: "Promise <" + state + ">",
		ID:      id,
		Promise: promise,
	}
}

func (vm *VM) awaitPromiseView(awaitee value.Value, promise *native.ObjPromise) trace.ValueView {
	id := ""
	if awaitee.IsObject() {
		if obj, err := heap.GetObject(awaitee.GetHandle()); err == nil && obj == promise {
			id = fmt.Sprintf("obj:%d", awaitee.GetHandle())
		}
	}
	return vm.promiseValueView(promise, id, 0, trace.CurrentLimits().MaxValueChars)
}

func truncateDisplay(display string, maxChars int) string {
	if maxChars <= 0 {
		maxChars = trace.DefaultMaxValueChars
	}
	runes := []rune(display)
	if len(runes) <= maxChars {
		return display
	}
	if maxChars <= 1 {
		return "…"
	}
	return string(runes[:maxChars-1]) + "…"
}

func quotedDisplay(display string, maxChars int) string {
	return truncateDisplay(strconv.Quote(truncateDisplay(display, maxChars)), maxChars)
}

func jitNoteView() trace.ValueView {
	return trace.ValueView{
		Type:      "note",
		Display:   "Executing JIT...",
		Synthetic: true,
	}
}

func (vm *VM) traceJITEnter(fn string, c *value.ValueChunk, entryIP int, mode string) {
	if !vm.traceEnabled {
		return
	}
	vm.flushTraceEvents()

	queues := queue.Snapshot()
	loop := vm.loopState(queues, false)

	var callFrame *CallFrame
	if vm.frameCount > 0 {
		frame := vm.currentFrame()
		callFrame = &frame
	}
	state := vm.snapshotVM(callFrame, c, entryIP, byteAt(c, entryIP))
	state.Stack = append(state.Stack, jitNoteView())
	state.StackTop++
	limits := trace.CurrentLimits()
	if limits.MaxStack > 0 && len(state.Stack) > limits.MaxStack {
		drop := len(state.Stack) - limits.MaxStack
		state.Stack = state.Stack[drop:]
		state.StackBase += drop
		state.StackOmitted += drop
	}
	state.JIT = &trace.JITState{
		Mode:  mode,
		State: "running",
		Fn:    fn,
		OpPtr: entryIP,
	}
	trace.SetLastVM(state)

	note := fn + " is running as JIT code"
	vm.appendTraceFrame(trace.Frame{
		Kind: "jit",
		Note: note,
		Event: &trace.Event{
			Type:   "jit_enter",
			Fn:     fn,
			Detail: note,
		},
		VM:        state,
		EventLoop: loop,
		Queues:    queues,
	})
}

func (vm *VM) traceJITExit(fn string, fallback *value.ValueChunk, entryIP int, mode, status string, result value.Value, duration time.Duration, opIP int) {
	if !vm.traceEnabled {
		return
	}
	vm.flushTraceEvents()

	queues := queue.Snapshot()
	loop := vm.loopState(queues, false)

	var callFrame *CallFrame
	var c *value.ValueChunk
	if vm.frameCount > 0 {
		frame := vm.currentFrame()
		callFrame = &frame
		c = frame.fn.ValueChunk()
	}
	state := vm.snapshotVM(callFrame, c, opIP, byteAt(c, opIP))
	if len(state.Frames) == 0 && fallback != nil {
		state.Fn = fallback.FnName
		state.ChunkID = fallback.ChunkID
		state.OpPtr = entryIP
		state.Op = opNameAt(fallback, entryIP)
		state.AstID = astIDAt(fallback, entryIP)
	}
	jitState := &trace.JITState{
		Mode:       mode,
		State:      status,
		Fn:         fn,
		OpPtr:      entryIP,
		DurationMS: float64(duration.Microseconds()) / 1000,
	}
	if status == "done" {
		resultView := vm.valueView(result)
		jitState.Result = &resultView
	}
	state.JIT = jitState
	trace.SetLastVM(state)

	note := "JIT returned; the result is on the stack"
	switch status {
	case "deopt":
		note = "JIT deoptimized; interpreter resumes"
	case "throw":
		note = "JIT threw; the exception continues in the interpreter"
	}
	vm.appendTraceFrame(trace.Frame{
		Kind: "jit",
		Note: note,
		Event: &trace.Event{
			Type:   "jit_exit",
			Fn:     fn,
			Detail: note,
		},
		VM:        state,
		EventLoop: loop,
		Queues:    queues,
	})
}

func byteAt(c *value.ValueChunk, ip int) uint8 {
	if c == nil || ip < 0 || ip >= len(c.Code) {
		return 0
	}
	return c.Code[ip]
}

func intPtrValue(pointer *int) int {
	if pointer == nil {
		return 0
	}
	return *pointer
}
