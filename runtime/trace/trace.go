package trace

import (
	"sync"
	"time"
)

const SchemaVersion = 1

const (
	DefaultMaxFrames     = 10000
	DefaultMaxStack      = 64
	DefaultMaxQueueItems = 32
	DefaultMaxTimers     = 16
	DefaultMaxValueChars = 160
)

type Limits struct {
	MaxFrames     int `json:"max_frames"`
	MaxStack      int `json:"max_stack"`
	MaxQueueItems int `json:"max_queue_items"`
	MaxTimers     int `json:"max_timers"`
	MaxValueChars int `json:"max_value_chars"`
}

func DefaultLimits() Limits {
	return Limits{
		MaxFrames:     DefaultMaxFrames,
		MaxStack:      DefaultMaxStack,
		MaxQueueItems: DefaultMaxQueueItems,
		MaxTimers:     DefaultMaxTimers,
		MaxValueChars: DefaultMaxValueChars,
	}
}

func (l Limits) normalized() Limits {
	defaults := DefaultLimits()
	if l.MaxFrames <= 0 {
		l.MaxFrames = defaults.MaxFrames
	}
	if l.MaxStack <= 0 {
		l.MaxStack = defaults.MaxStack
	}
	if l.MaxQueueItems <= 0 {
		l.MaxQueueItems = defaults.MaxQueueItems
	}
	if l.MaxTimers <= 0 {
		l.MaxTimers = defaults.MaxTimers
	}
	if l.MaxValueChars <= 0 {
		l.MaxValueChars = defaults.MaxValueChars
	}
	return l
}

type Execution struct {
	SchemaVersion int     `json:"schema_version"`
	Status        string  `json:"status"`
	Frames        []Frame `json:"frames"`
	Truncated     bool    `json:"truncated"`
	TotalFrames   int     `json:"total_frames"`
	DroppedFrames int     `json:"dropped_frames"`
	DurationMS    float64 `json:"duration_ms"`
	Limits        Limits  `json:"limits"`
}

func NewExecutionPlaceholder() *Execution {
	return &Execution{
		SchemaVersion: SchemaVersion,
		Status:        "not_requested",
		Frames:        []Frame{},
		Limits:        DefaultLimits(),
	}
}

type Frame struct {
	Seq       int            `json:"seq"`
	Kind      string         `json:"kind"`
	AtMS      float64        `json:"at_ms"`
	Note      string         `json:"note,omitempty"`
	Event     *Event         `json:"event,omitempty"`
	VM        *VMState       `json:"vm,omitempty"`
	EventLoop EventLoopState `json:"event_loop"`
	Queues    Queues         `json:"queues"`
	Output    *OutputDelta   `json:"output,omitempty"`
}

type OutputDelta struct {
	Delta  string `json:"delta"`
	Length int    `json:"length"`
}

type VMState struct {
	ID            int            `json:"id"`
	ParentID      int            `json:"parent_id,omitempty"`
	ActiveFrame   int            `json:"active_frame"`
	OpPtr         int            `json:"op_ptr"`
	Op            string         `json:"op"`
	AstID         int            `json:"ast_id"`
	Fn            string         `json:"fn"`
	ChunkID       string         `json:"chunk_id"`
	Frames        []VMStackFrame `json:"frames"`
	FramesOmitted int            `json:"frames_omitted,omitempty"`
	Stack         []ValueView    `json:"stack"`
	StackBase     int            `json:"stack_base"`
	StackTop      int            `json:"stack_top"`
	StackOmitted  int            `json:"stack_omitted,omitempty"`
	JIT           *JITState      `json:"jit,omitempty"`
}

type VMStackFrame struct {
	Index      int    `json:"index"`
	Fn         string `json:"fn"`
	ChunkID    string `json:"chunk_id"`
	OpPtr      int    `json:"op_ptr"`
	Op         string `json:"op"`
	AstID      int    `json:"ast_id"`
	LocalStart int    `json:"local_start"`
}

type ValueView struct {
	Type      string        `json:"type"`
	Display   string        `json:"display"`
	ID        string        `json:"id,omitempty"`
	Synthetic bool          `json:"synthetic,omitempty"`
	Promise   *PromiseView  `json:"promise,omitempty"`
	Function  *FunctionView `json:"function,omitempty"`
}

type PromiseView struct {
	State     string     `json:"state"`
	Value     *ValueView `json:"value,omitempty"`
	Reactions int        `json:"reactions,omitempty"`
	Observers int        `json:"observers,omitempty"`
}

type FunctionView struct {
	Name  string `json:"name"`
	Arity int    `json:"arity"`
	Async bool   `json:"async,omitempty"`
}

type Queues struct {
	MicroTasks        []QueueItem `json:"micro_tasks"`
	Tasks             []QueueItem `json:"tasks"`
	MicroTasksOmitted int         `json:"micro_tasks_omitted,omitempty"`
	TasksOmitted      int         `json:"tasks_omitted,omitempty"`
}

type QueueItem struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Label  string `json:"label"`
	Source string `json:"source,omitempty"`
	Fn     string `json:"fn,omitempty"`
	Native bool   `json:"native,omitempty"`
}

type EventLoopState struct {
	Phase         string      `json:"phase"`
	NextQueue     string      `json:"next_queue,omitempty"`
	CurrentJob    *QueueItem  `json:"current_job,omitempty"`
	Work          int64       `json:"work"`
	Timers        []TimerItem `json:"timers"`
	TimersOmitted int         `json:"timers_omitted,omitempty"`
}

type TimerItem struct {
	ID      int    `json:"id"`
	DelayMS int    `json:"delay_ms"`
	Label   string `json:"label"`
	Repeats bool   `json:"repeats,omitempty"`
	State   string `json:"state"`
}

type JITState struct {
	Mode       string     `json:"mode"`
	State      string     `json:"state"`
	Fn         string     `json:"fn"`
	OpPtr      int        `json:"op_ptr"`
	Result     *ValueView `json:"result,omitempty"`
	DurationMS float64    `json:"duration_ms,omitempty"`
}

type Event struct {
	Type        string     `json:"type"`
	Queue       string     `json:"queue,omitempty"`
	Item        *QueueItem `json:"item,omitempty"`
	Fn          string     `json:"fn,omitempty"`
	ResumeOpPtr int        `json:"resume_op_ptr,omitempty"`
	Promise     *ValueView `json:"promise,omitempty"`
	Timer       *TimerItem `json:"timer,omitempty"`
	Detail      string     `json:"detail,omitempty"`
}

type QueueEvent struct {
	Type   string
	Queue  string
	Item   QueueItem
	Queues Queues
	Detail string
}

type Recorder struct {
	mu sync.Mutex

	limits      Limits
	normalLimit int
	frames      []Frame
	total       int
	dropped     int
	nextSeq     int
	start       time.Time

	lastOutputLen int
	lastVM        *VMState

	pending []QueueEvent

	currentJob *QueueItem
	booted     bool
	done       bool
	finished   bool
}

func NewRecorder(limits Limits) *Recorder {
	limits = limits.normalized()
	normalLimit := limits.MaxFrames - 1
	reserve := limits.MaxFrames / 10
	if reserve > 1024 {
		reserve = 1024
	}
	normalLimit -= reserve
	if normalLimit < 0 {
		normalLimit = 0
	}
	return &Recorder{
		limits:      limits,
		normalLimit: normalLimit,
		frames:      []Frame{},
		start:       time.Now(),
	}
}

func (r *Recorder) Limits() Limits {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.limits
}

func (r *Recorder) LastOutputLen() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastOutputLen
}

func (r *Recorder) SetLastVM(vm *VMState) {
	r.mu.Lock()
	r.lastVM = vm
	r.mu.Unlock()
}

func (r *Recorder) LastVM() *VMState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastVM
}

func (r *Recorder) SetCurrentJob(job *QueueItem) {
	var copy *QueueItem
	if job != nil {
		value := *job
		copy = &value
	}
	r.mu.Lock()
	r.currentJob = copy
	r.mu.Unlock()
}

func (r *Recorder) ClearCurrentJob() {
	r.mu.Lock()
	r.currentJob = nil
	r.mu.Unlock()
}

func (r *Recorder) CurrentJob() *QueueItem {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.currentJob == nil {
		return nil
	}
	value := *r.currentJob
	return &value
}

func (r *Recorder) SetBooted(booted bool) {
	r.mu.Lock()
	r.booted = booted
	r.mu.Unlock()
}

func (r *Recorder) IsBooted() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.booted
}

func (r *Recorder) Done() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.done
}

func (r *Recorder) RecordQueueEvent(event QueueEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return
	}
	limit := r.limits.MaxFrames * 2
	if limit < 1 {
		limit = 1
	}
	if len(r.pending) >= limit {
		return
	}
	r.pending = append(r.pending, event)
}

func (r *Recorder) TakeQueueEvents() []QueueEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.pending) == 0 {
		return nil
	}
	events := r.pending
	r.pending = nil
	return events
}

func (r *Recorder) AppendFrame(frame Frame, outputLen int, output string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	seq := r.nextSeq
	r.nextSeq++
	frame.Seq = seq
	frame.AtMS = float64(time.Since(r.start).Microseconds()) / 1000
	r.total++

	isEnd := frame.Kind == "end"
	interesting := frame.Kind == "event" || frame.Kind == "jit"
	limit := r.normalLimit
	if interesting {
		limit = r.limits.MaxFrames - 1
	}
	if !isEnd && len(r.frames) >= limit {
		r.dropped++
		return false
	}

	if outputLen > r.lastOutputLen && output != "" {
		if outputLen > len(output) {
			outputLen = len(output)
		}
		start := r.lastOutputLen
		if start < 0 || start > len(output) {
			start = len(output)
		}
		if outputLen > start {
			frame.Output = &OutputDelta{
				Delta:  output[start:outputLen],
				Length: outputLen,
			}
			r.lastOutputLen = outputLen
		}
	}

	r.frames = append(r.frames, frame)
	if isEnd {
		r.finished = true
		r.done = true
	}
	return true
}

func (r *Recorder) Execution() *Execution {
	r.mu.Lock()
	defer r.mu.Unlock()

	frames := make([]Frame, len(r.frames))
	copy(frames, r.frames)

	status := "complete"
	if r.dropped > 0 {
		status = "truncated"
	}

	return &Execution{
		SchemaVersion: SchemaVersion,
		Status:        status,
		Frames:        frames,
		Truncated:     r.dropped > 0,
		TotalFrames:   r.total,
		DroppedFrames: r.dropped,
		DurationMS:    float64(time.Since(r.start).Microseconds()) / 1000,
		Limits:        r.limits,
	}
}

var (
	globalMu sync.RWMutex
	active   *Recorder
)

func Enable(limits Limits) *Recorder {
	recorder := NewRecorder(limits)
	globalMu.Lock()
	active = recorder
	globalMu.Unlock()
	return recorder
}

func Disable() {
	globalMu.Lock()
	active = nil
	globalMu.Unlock()
}

func Active() *Recorder {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return active
}

func Enabled() bool {
	return Active() != nil
}

func CurrentLimits() Limits {
	if recorder := Active(); recorder != nil {
		return recorder.Limits()
	}
	return DefaultLimits()
}

func LastOutputLen() int {
	if recorder := Active(); recorder != nil {
		return recorder.LastOutputLen()
	}
	return 0
}

func SetLastVM(vm *VMState) {
	if recorder := Active(); recorder != nil {
		recorder.SetLastVM(vm)
	}
}

func LastVM() *VMState {
	if recorder := Active(); recorder != nil {
		return recorder.LastVM()
	}
	return nil
}

func SetCurrentJob(job *QueueItem) {
	if recorder := Active(); recorder != nil {
		recorder.SetCurrentJob(job)
	}
}

func ClearCurrentJob() {
	if recorder := Active(); recorder != nil {
		recorder.ClearCurrentJob()
	}
}

func CurrentJob() *QueueItem {
	if recorder := Active(); recorder != nil {
		return recorder.CurrentJob()
	}
	return nil
}

func SetBooted(booted bool) {
	if recorder := Active(); recorder != nil {
		recorder.SetBooted(booted)
	}
}

func IsBooted() bool {
	if recorder := Active(); recorder != nil {
		return recorder.IsBooted()
	}
	return false
}

func RecordQueueEvent(event QueueEvent) {
	if recorder := Active(); recorder != nil {
		recorder.RecordQueueEvent(event)
	}
}

func TakeQueueEvents() []QueueEvent {
	if recorder := Active(); recorder != nil {
		return recorder.TakeQueueEvents()
	}
	return nil
}

func AppendFrame(frame Frame, outputLen int, output string) bool {
	if recorder := Active(); recorder != nil {
		return recorder.AppendFrame(frame, outputLen, output)
	}
	return false
}

func ExecutionInfo() *Execution {
	if recorder := Active(); recorder != nil {
		return recorder.Execution()
	}
	return NewExecutionPlaceholder()
}
