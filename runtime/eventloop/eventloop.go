package eventloop

import (
	"fmt"
	"go_js/object"
	"go_js/queue"
	"go_js/trace"
	"go_js/value"
	"sort"
	"sync"
	"sync/atomic"
)

type EventLoop struct {
	jobCallbackChannel chan *object.JobChannelMessage
	jobChannel         chan *object.JobChannelMessage

	wg        *sync.WaitGroup
	work      atomic.Int64
	mu        sync.Mutex
	closed    bool
	nextJobID uint64
	workItems map[uint64]trace.WorkItem
}

var el *EventLoop

func Init(wg *sync.WaitGroup) {
	if el != nil {
		return
	}

	el = &EventLoop{
		jobChannel:         make(chan *object.JobChannelMessage),
		jobCallbackChannel: make(chan *object.JobChannelMessage),
		wg:                 wg,
		workItems:          map[uint64]trace.WorkItem{},
	}
}

func Start() {
	go func() {
		for message := range el.jobChannel {
			go func() {
				message.Job.Work(el.jobCallbackChannel, message.Done)
			}()
		}
	}()

	go func() {
		for message := range el.jobCallbackChannel {
			priority := queue.TASK
			if message.Callback.MicroTask {
				priority = queue.MICRO_TASK
			}
			queue.Enqueue(message.Callback, priority, true)
			message.Done()
		}
	}()
}

func Dispatch(job object.Job) {
	el.mu.Lock()
	if el.closed {
		el.mu.Unlock()
		return
	}
	el.wg.Add(1)
	el.work.Add(1)
	el.nextJobID++
	jobID := el.nextJobID
	item := trace.WorkItem{Kind: "job", Label: "background job"}
	if traced, ok := job.(interface{ TraceWorkItem() trace.WorkItem }); ok {
		item = traced.TraceWorkItem()
	}
	item.ID = fmt.Sprintf("w%d", jobID)
	el.workItems[jobID] = item
	el.mu.Unlock()
	message := &object.JobChannelMessage{
		Job:      job,
		Callback: object.Callback{Fn: nil, ThisCtx: value.UNDEFINED},
		Done: func() {
			el.mu.Lock()
			delete(el.workItems, jobID)
			el.mu.Unlock()
			el.wg.Done()
			el.work.Add(-1)
		},
	}
	el.jobChannel <- message
}

func HasWork() bool {
	return el.work.Load() > 0
}

type Snapshot struct {
	Work   int64
	Items  []trace.WorkItem
	Closed bool
}

func SnapshotState() Snapshot {
	el.mu.Lock()
	defer el.mu.Unlock()

	ids := make([]uint64, 0, len(el.workItems))
	for id := range el.workItems {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	items := make([]trace.WorkItem, 0, len(ids))
	for _, id := range ids {
		items = append(items, el.workItems[id])
	}

	return Snapshot{
		Work:   el.work.Load(),
		Items:  items,
		Closed: el.closed,
	}
}

func TryClose() bool {
	el.mu.Lock()
	defer el.mu.Unlock()
	if el.closed {
		return true
	}
	if el.work.Load() == 0 && !queue.HasTasks() {
		el.closed = true
		return true
	}
	return false
}
