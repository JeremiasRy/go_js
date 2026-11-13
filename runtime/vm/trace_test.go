package vm

import (
	"math"
	"testing"

	"go_js/chunk"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
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
