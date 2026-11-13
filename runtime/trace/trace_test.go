package trace

import (
	"encoding/json"
	"strings"
	"testing"
)

func stepFrame(note string) Frame {
	return Frame{
		Kind: "step",
		Note: note,
		EventLoop: EventLoopState{
			Phase:  "executing",
			Timers: []TimerItem{},
		},
		Queues: Queues{
			MicroTasks: []QueueItem{},
			Tasks:      []QueueItem{},
		},
	}
}

func TestAppendTruncationKeepsEndFrame(t *testing.T) {
	recorder := NewRecorder(Limits{MaxFrames: 5})
	for i := 0; i < 10; i++ {
		recorder.AppendFrame(stepFrame("step"), 0, "")
	}
	if !recorder.AppendFrame(Frame{Kind: "end"}, 0, "") {
		t.Fatal("expected end frame to be stored")
	}

	execution := recorder.Execution()
	if len(execution.Frames) != 5 {
		t.Fatalf("expected 5 stored frames, got %d", len(execution.Frames))
	}
	if execution.TotalFrames != 11 {
		t.Fatalf("expected 11 total frames, got %d", execution.TotalFrames)
	}
	if execution.DroppedFrames != 6 {
		t.Fatalf("expected 6 dropped frames, got %d", execution.DroppedFrames)
	}
	if !execution.Truncated || execution.Status != "truncated" {
		t.Fatalf("expected truncated execution, got %+v", execution)
	}
	if execution.Frames[len(execution.Frames)-1].Kind != "end" {
		t.Fatalf("expected last frame to be end, got %q", execution.Frames[len(execution.Frames)-1].Kind)
	}
}

func TestSequenceIsMonotonicAcrossDroppedFrames(t *testing.T) {
	recorder := NewRecorder(Limits{MaxFrames: 4})
	for i := 0; i < 8; i++ {
		recorder.AppendFrame(stepFrame("step"), 0, "")
	}
	recorder.AppendFrame(Frame{Kind: "end"}, 0, "")

	execution := recorder.Execution()
	expected := []int{0, 1, 2, 8}
	if len(execution.Frames) != len(expected) {
		t.Fatalf("expected %d frames, got %d", len(expected), len(execution.Frames))
	}
	for i, seq := range expected {
		if execution.Frames[i].Seq != seq {
			t.Fatalf("frame %d: expected seq %d, got %d", i, seq, execution.Frames[i].Seq)
		}
	}
	for i := 1; i < len(execution.Frames); i++ {
		if execution.Frames[i].Seq <= execution.Frames[i-1].Seq {
			t.Fatalf("sequence is not increasing: %+v", execution.Frames)
		}
	}
}

func TestTruncationKeepsInterestingTailFrames(t *testing.T) {
	recorder := NewRecorder(Limits{MaxFrames: 20})
	for i := 0; i < 100; i++ {
		recorder.AppendFrame(stepFrame("step"), 0, "")
	}
	for i := 0; i < 5; i++ {
		frame := stepFrame("jit")
		frame.Kind = "jit"
		recorder.AppendFrame(frame, 0, "")
	}
	recorder.AppendFrame(Frame{Kind: "end"}, 0, "")

	execution := recorder.Execution()
	if len(execution.Frames) != 20 {
		t.Fatalf("expected 20 stored frames, got %d", len(execution.Frames))
	}
	jitFrames := 0
	for _, frame := range execution.Frames {
		if frame.Kind == "jit" {
			jitFrames++
		}
	}
	if jitFrames != 2 {
		t.Fatalf("expected the reserved tail to keep 2 jit frames, got %d", jitFrames)
	}
	if execution.Frames[len(execution.Frames)-1].Kind != "end" {
		t.Fatalf("expected last frame to be end, got %q", execution.Frames[len(execution.Frames)-1].Kind)
	}
	if execution.TotalFrames != 106 || execution.DroppedFrames != 86 {
		t.Fatalf("unexpected totals: total=%d dropped=%d", execution.TotalFrames, execution.DroppedFrames)
	}
}

func TestQueueEventsFlushInOrder(t *testing.T) {
	recorder := NewRecorder(DefaultLimits())
	recorder.RecordQueueEvent(QueueEvent{Type: "enqueue", Queue: "tasks", Item: QueueItem{ID: "q1"}})
	recorder.RecordQueueEvent(QueueEvent{Type: "enqueue", Queue: "micro_tasks", Item: QueueItem{ID: "q2"}})

	events := recorder.TakeQueueEvents()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Item.ID != "q1" || events[1].Item.ID != "q2" {
		t.Fatalf("events out of order: %+v", events)
	}
	if remaining := recorder.TakeQueueEvents(); len(remaining) != 0 {
		t.Fatalf("expected pending events to be cleared, got %d", len(remaining))
	}
}

func TestOutputDeltasSumToFullOutput(t *testing.T) {
	recorder := NewRecorder(DefaultLimits())
	recorder.AppendFrame(stepFrame("one"), 5, "hello")
	recorder.AppendFrame(stepFrame("unchanged"), 5, "")
	recorder.AppendFrame(stepFrame("two"), 11, "hello world")

	execution := recorder.Execution()
	if execution.Frames[0].Output == nil || execution.Frames[0].Output.Delta != "hello" {
		t.Fatalf("expected first delta to be hello, got %+v", execution.Frames[0].Output)
	}
	if execution.Frames[1].Output != nil {
		t.Fatalf("expected no delta when output did not grow, got %+v", execution.Frames[1].Output)
	}
	if execution.Frames[2].Output == nil || execution.Frames[2].Output.Delta != " world" {
		t.Fatalf("expected second delta to be ' world', got %+v", execution.Frames[2].Output)
	}

	var output strings.Builder
	for _, frame := range execution.Frames {
		if frame.Output != nil {
			output.WriteString(frame.Output.Delta)
		}
	}
	if output.String() != "hello world" {
		t.Fatalf("reconstructed output %q does not match full output", output.String())
	}
}

func TestExecutionJSONShape(t *testing.T) {
	recorder := NewRecorder(DefaultLimits())
	recorder.AppendFrame(stepFrame("step"), 0, "")
	recorder.AppendFrame(Frame{
		Kind:      "end",
		Event:     &Event{Type: "done"},
		EventLoop: EventLoopState{Phase: "done", Timers: []TimerItem{}},
		Queues:    Queues{MicroTasks: []QueueItem{}, Tasks: []QueueItem{}},
	}, 0, "")

	data, err := json.Marshal(recorder.Execution())
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, expected := range []string{
		`"schema_version":1`,
		`"status":"complete"`,
		`"frames":[`,
		`"event_loop"`,
		`"queues"`,
		`"max_frames":10000`,
		`"max_stack":64`,
		`"max_queue_items":32`,
		`"max_timers":16`,
		`"max_value_chars":160`,
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected JSON to contain %s, got %s", expected, text)
		}
	}
}

func TestTruncationPreservesOutputOnEndFrame(t *testing.T) {
	recorder := NewRecorder(Limits{MaxFrames: 2})
	recorder.AppendFrame(stepFrame("stored"), 5, "hello")
	recorder.AppendFrame(stepFrame("dropped"), 10, "hello worl")
	recorder.AppendFrame(stepFrame("dropped"), 10, "hello worl")
	recorder.AppendFrame(Frame{Kind: "end"}, 10, "hello worl")

	execution := recorder.Execution()
	if len(execution.Frames) != 2 {
		t.Fatalf("expected 2 stored frames, got %d", len(execution.Frames))
	}
	if execution.Frames[0].Output == nil || execution.Frames[0].Output.Delta != "hello" {
		t.Fatalf("expected first delta to be hello, got %+v", execution.Frames[0].Output)
	}
	end := execution.Frames[1]
	if end.Kind != "end" {
		t.Fatalf("expected end frame, got %q", end.Kind)
	}
	if end.Output == nil || end.Output.Delta != " worl" {
		t.Fatalf("expected end frame to carry the truncated output, got %+v", end.Output)
	}
}

func TestMaxFramesOneStillKeepsEndFrame(t *testing.T) {
	recorder := NewRecorder(Limits{MaxFrames: 1})
	for i := 0; i < 5; i++ {
		recorder.AppendFrame(stepFrame("step"), 0, "")
	}
	recorder.AppendFrame(Frame{Kind: "end"}, 5, "hello")

	execution := recorder.Execution()
	if len(execution.Frames) != 1 {
		t.Fatalf("expected exactly the end frame, got %d frames", len(execution.Frames))
	}
	if execution.Frames[0].Kind != "end" {
		t.Fatalf("expected end frame, got %q", execution.Frames[0].Kind)
	}
	if execution.TotalFrames != 6 || execution.DroppedFrames != 5 {
		t.Fatalf("unexpected totals: total=%d dropped=%d", execution.TotalFrames, execution.DroppedFrames)
	}
}

func TestQueueEventWithEmptySnapshotAndNoVMIsSafe(t *testing.T) {
	recorder := NewRecorder(DefaultLimits())
	recorder.RecordQueueEvent(QueueEvent{})
	events := recorder.TakeQueueEvents()
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	if recorder.LastVM() != nil {
		t.Fatal("expected no last VM")
	}
	recorder.AppendFrame(Frame{Kind: "event", Event: &Event{Type: "enqueue"}}, 0, "")
	execution := recorder.Execution()
	if len(execution.Frames) != 1 {
		t.Fatalf("expected one frame, got %d", len(execution.Frames))
	}
}

func TestPendingQueueEventsAreBounded(t *testing.T) {
	recorder := NewRecorder(Limits{MaxFrames: 2})
	for i := 0; i < 10; i++ {
		recorder.RecordQueueEvent(QueueEvent{Type: "enqueue"})
	}
	if events := recorder.TakeQueueEvents(); len(events) != 4 {
		t.Fatalf("expected pending queue events capped at 4, got %d", len(events))
	}
}
