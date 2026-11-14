package vm

import (
	"math"
	"strings"
	"sync"
	"testing"

	"go_js/chunk"
	"go_js/eventloop"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/queue"
	"go_js/trace"
	"go_js/value"
)

func TestSnapshotPausedFramesUseCalleeReturnIP(t *testing.T) {
	vm := NewVM(false, nil)

	callerChunk := value.NewChunk("caller")
	callerChunk.Code = []uint8{chunk.OP_CONSTANT, 0, chunk.OP_POP, chunk.OP_CONSTANT, 0}
	callerChunk.AstId = []int{1, 1, 2, 3, 3}

	calleeChunk := value.NewChunk("callee")
	calleeChunk.Code = []uint8{chunk.OP_CONSTANT, 0, chunk.OP_RETURN}
	calleeChunk.AstId = []int{4, 4, 4}

	caller := object.NewFunction("caller", 0, callerChunk)
	callee := object.NewFunction("callee", 0, calleeChunk)

	var callerFrame CallFrame
	callerFrame.initCallFrame(caller, 0, 0, value.UNDEFINED)
	vm.frames[0] = callerFrame
	vm.frameCount = 1

	var calleeFrame CallFrame
	calleeFrame.initCallFrame(callee, 0, 2, value.UNDEFINED)
	vm.frames[1] = calleeFrame
	vm.frameCount = 2

	state := vm.snapshotVM(&vm.frames[1], calleeChunk, 0, calleeChunk.Code[0])
	if len(state.Frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(state.Frames))
	}
	paused := state.Frames[0]
	if paused.ChunkID != callerChunk.ChunkID || paused.OpPtr != 2 || paused.Op != "OP_POP" {
		t.Fatalf("expected caller paused at OP_POP (ptr 2), got chunk=%s ptr=%d op=%s", paused.ChunkID, paused.OpPtr, paused.Op)
	}
	active := state.Frames[1]
	if active.ChunkID != calleeChunk.ChunkID || active.OpPtr != 0 || active.Op != "OP_CONSTANT" {
		t.Fatalf("expected callee at OP_CONSTANT (ptr 0), got chunk=%s ptr=%d op=%s", active.ChunkID, active.OpPtr, active.Op)
	}
	if state.ActiveFrame != 1 {
		t.Fatalf("expected active frame 1, got %d", state.ActiveFrame)
	}
}

func TestSnapshotOmitsHiddenFrames(t *testing.T) {
	vm := NewVM(false, nil)

	hiddenChunk := value.NewChunk("hidden")
	hiddenChunk.TraceHidden = true
	hiddenChunk.Code = []uint8{chunk.OP_RETURN}
	hiddenChunk.AstId = []int{1}
	visibleChunk := value.NewChunk("visible")
	visibleChunk.Code = []uint8{chunk.OP_CONSTANT, 0, chunk.OP_RETURN}
	visibleChunk.AstId = []int{2, 2, 2}

	hidden := object.NewFunction("hidden", 0, hiddenChunk)
	visible := object.NewFunction("visible", 0, visibleChunk)

	var hiddenFrame CallFrame
	hiddenFrame.initCallFrame(hidden, 0, 0, value.UNDEFINED)
	vm.frames[0] = hiddenFrame
	vm.frameCount = 1

	var visibleFrame CallFrame
	visibleFrame.initCallFrame(visible, 0, 0, value.UNDEFINED)
	vm.frames[1] = visibleFrame
	vm.frameCount = 2

	state := vm.snapshotVM(&vm.frames[1], visibleChunk, 0, visibleChunk.Code[0])
	if len(state.Frames) != 1 {
		t.Fatalf("expected hidden frame to be omitted, got %d frames", len(state.Frames))
	}
	if state.Frames[0].ChunkID != visibleChunk.ChunkID {
		t.Fatalf("expected visible chunk, got %s", state.Frames[0].ChunkID)
	}
	if state.ActiveFrame != 0 {
		t.Fatalf("expected active frame 0, got %d", state.ActiveFrame)
	}
}

func TestTraceValueViewNamesConcreteObjects(t *testing.T) {
	vm := NewVM(false, nil)

	console := native.NewObjectConsole()
	consoleView := vm.valueView(value.EncodeHandle(heap.Allocate(console)))
	if consoleView.Type != "object" || consoleView.Display != "[console]" {
		t.Fatalf("expected console object name, got %s %q", consoleView.Type, consoleView.Display)
	}

	class := native.NewObjClass("Person")
	instance := class.NewInstance()
	instanceView := vm.valueView(value.EncodeHandle(heap.Allocate(instance)))
	if instanceView.Type != "object" || instanceView.Display != "[Person]" {
		t.Fatalf("expected instance class name, got %s %q", instanceView.Type, instanceView.Display)
	}

	anonymous := &native.ObjObject{Members: map[string]native.ObjectValueEntry{}}
	anonymousView := vm.valueView(value.EncodeHandle(heap.Allocate(anonymous)))
	if anonymousView.Type != "object" || anonymousView.Display != "[Object object]" {
		t.Fatalf("expected anonymous object display, got %s %q", anonymousView.Type, anonymousView.Display)
	}
}

func TestTraceValueViewPrimitives(t *testing.T) {
	vm := NewVM(false, nil)
	cases := []struct {
		value   value.Value
		kind    string
		display string
	}{
		{value.NULL, "null", "null"},
		{value.UNDEFINED, "undefined", "undefined"},
		{value.TRUE, "boolean", "true"},
		{value.FALSE, "boolean", "false"},
		{value.ValueFromFloat64(42), "number", "42"},
		{value.ValueFromFloat64(math.NaN()), "number", "NaN"},
		{value.ValueFromFloat64(math.Inf(1)), "number", "Infinity"},
		{value.ValueFromFloat64(math.Inf(-1)), "number", "-Infinity"},
		{value.Value(math.Float64bits(math.Copysign(0, -1))), "number", "-0"},
		{value.TAG_METHOD_HANDLE, "method_handle", "<method>"},
		{value.TAG_ARGUMENT_START, "argument_start", "<arguments>"},
		{value.EMPTY_ARRAY_ITEM, "empty", "<empty>"},
	}
	for _, test := range cases {
		view := vm.valueView(test.value)
		if view.Type != test.kind || view.Display != test.display {
			t.Fatalf("value %#x: expected %s %q, got %s %q", test.value, test.kind, test.display, view.Type, view.Display)
		}
	}
}

func TestTraceDisplayTruncation(t *testing.T) {
	truncated := []struct {
		display string
		max     int
		want    string
	}{
		{"hello", 10, "hello"},
		{"hello", 4, "hel…"},
		{strings.Repeat("a", 10), 5, "aaaa…"},
		{"héllo", 4, "hél…"},
	}
	for _, test := range truncated {
		if got := truncateDisplay(test.display, test.max); got != test.want {
			t.Fatalf("truncateDisplay(%q, %d): expected %q, got %q", test.display, test.max, test.want, got)
		}
	}

	quoted := []struct {
		display string
		max     int
		want    string
	}{
		{"hello", 20, `"hello"`},
		{"hello", 6, `"hell…`},
		{"a\nb", 20, `"a\nb"`},
		{"hé", 20, `"hé"`},
	}
	for _, test := range quoted {
		if got := quotedDisplay(test.display, test.max); got != test.want {
			t.Fatalf("quotedDisplay(%q, %d): expected %q, got %q", test.display, test.max, test.want, got)
		}
	}
}

func newTraceTestChunk() *value.ValueChunk {
	codeChunk := value.NewChunk("main")
	codeChunk.Code = make([]uint8, 20)
	codeChunk.AstId = make([]int, 20)
	for i := range codeChunk.Code {
		codeChunk.Code[i] = chunk.OP_POP
		codeChunk.AstId[i] = i
	}
	return codeChunk
}

func newTraceTestFrame(codeChunk *value.ValueChunk) CallFrame {
	fn := object.NewFunction("main", 0, codeChunk)
	var frame CallFrame
	frame.initCallFrame(fn, 0, 0, value.UNDEFINED)
	return frame
}

func TestTraceBudgetKeepsEventStateOutputAndAccounting(t *testing.T) {
	var wg sync.WaitGroup
	queue.Init(&wg)
	eventloop.Init(&wg)

	output := &strings.Builder{}
	runner := NewVM(true, output)
	runner.BeginTrace(trace.Limits{
		MaxFrames:     13,
		MaxStack:      8,
		MaxQueueItems: 4,
		MaxTimers:     4,
		MaxValueChars: 80,
	})
	t.Cleanup(trace.Disable)

	codeChunk := newTraceTestChunk()
	codeChunk.Code[12] = chunk.OP_RETURN
	frame := newTraceTestFrame(codeChunk)
	runner.frames[0] = frame
	runner.frameCount = 1

	output.WriteString("before ")
	for i := 0; i < 10; i++ {
		runner.traceStep(&frame, codeChunk, i, codeChunk.Code[i])
	}
	runner.traceStep(&frame, codeChunk, 10, codeChunk.Code[10])
	if recorder := trace.Active(); recorder == nil || !recorder.StepBudgetExhausted() {
		t.Fatal("expected the shared step budget to be exhausted")
	}
	runner.traceStep(&frame, codeChunk, 11, codeChunk.Code[11])
	runner.traceStep(&frame, codeChunk, 12, codeChunk.Code[12])

	output.WriteString("after")
	item := trace.QueueItem{ID: "q1", Kind: "micro_task", Label: "callback"}
	trace.RecordQueueEvent(trace.QueueEvent{
		Type:   "enqueue",
		Queue:  "micro_tasks",
		Item:   item,
		Queues: trace.Queues{MicroTasks: []trace.QueueItem{item}, Tasks: []trace.QueueItem{}},
	})
	runner.traceStep(&frame, codeChunk, 13, codeChunk.Code[13])

	execution := runner.FinishTrace(output.String())
	if !execution.Truncated {
		t.Fatal("expected a truncated execution")
	}
	if execution.TotalFrames != 17 || execution.DroppedFrames != 4 {
		t.Fatalf("unexpected totals: total=%d dropped=%d", execution.TotalFrames, execution.DroppedFrames)
	}
	if len(execution.Frames) != 13 {
		t.Fatalf("expected 13 stored frames, got %d", len(execution.Frames))
	}

	stepFrames := 0
	var event *trace.Frame
	var reconstructed strings.Builder
	for i := range execution.Frames {
		stored := &execution.Frames[i]
		if stored.Kind == "step" {
			stepFrames++
		}
		if stored.Event != nil && stored.Event.Type == "enqueue" {
			event = stored
		}
		if stored.Output != nil {
			reconstructed.WriteString(stored.Output.Delta)
		}
	}
	if stepFrames != 10 {
		t.Fatalf("expected 10 stored step frames, got %d", stepFrames)
	}
	if event == nil {
		t.Fatal("expected the enqueue event after truncation")
	}
	if event.Seq != 14 {
		t.Fatalf("expected the event at seq 14, got %d", event.Seq)
	}
	if event.VM == nil || event.VM.Op != "OP_RETURN" {
		t.Fatalf("expected the event to carry the latest VM position, got %+v", event.VM)
	}
	if event.Output == nil || event.Output.Delta != "after" {
		t.Fatalf("expected the event to carry the post-truncation output, got %+v", event.Output)
	}
	if reconstructed.String() != output.String() {
		t.Fatalf("reconstructed output %q does not match full output %q", reconstructed.String(), output.String())
	}
	if last := execution.Frames[len(execution.Frames)-1]; last.Kind != "end" {
		t.Fatalf("expected the last frame to be end, got %q", last.Kind)
	}
}

func TestSubVMTraceSharesExhaustedStepBudget(t *testing.T) {
	var wg sync.WaitGroup
	queue.Init(&wg)
	eventloop.Init(&wg)

	output := &strings.Builder{}
	runner := NewVM(true, output)
	runner.BeginTrace(trace.Limits{
		MaxFrames:     8,
		MaxStack:      8,
		MaxQueueItems: 4,
		MaxTimers:     4,
		MaxValueChars: 80,
	})
	t.Cleanup(trace.Disable)

	codeChunk := newTraceTestChunk()
	frame := newTraceTestFrame(codeChunk)
	runner.frames[0] = frame
	runner.frameCount = 1

	for i := 0; i < 7; i++ {
		runner.traceStep(&frame, codeChunk, i, codeChunk.Code[i])
	}
	if recorder := trace.Active(); recorder == nil || !recorder.StepBudgetExhausted() {
		t.Fatal("expected the shared step budget to be exhausted")
	}

	sub := runner.newSubVM()
	sub.frames[0] = frame
	sub.frameCount = 1
	for i := 0; i < 5; i++ {
		sub.traceStep(&frame, codeChunk, i, codeChunk.Code[i])
	}

	execution := runner.FinishTrace(output.String())
	stepFrames := 0
	for _, stored := range execution.Frames {
		if stored.Kind == "step" {
			stepFrames++
		}
	}
	if stepFrames != 6 {
		t.Fatalf("expected the sub-VM to stop emitting step frames, got %d step frames", stepFrames)
	}
	if execution.TotalFrames != 14 || execution.DroppedFrames != 6 {
		t.Fatalf("unexpected totals: total=%d dropped=%d", execution.TotalFrames, execution.DroppedFrames)
	}
	if last := execution.Frames[len(execution.Frames)-1]; last.Kind != "end" {
		t.Fatalf("expected the last frame to be end, got %q", last.Kind)
	}
}
