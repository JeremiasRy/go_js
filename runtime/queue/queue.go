package queue

import (
	"container/list"
	"errors"
	"go_js/object"
	"go_js/trace"
	"strconv"
	"sync"
	"sync/atomic"
)

type TaskPriority uint8

const (
	MICRO_TASK TaskPriority = iota
	TASK
)

type Queue struct {
	microTask *list.List
	task      *list.List

	mu sync.Mutex
	wg *sync.WaitGroup
}

var q *Queue
var QueueC = make(chan struct{})
var nextTraceID atomic.Uint64

func Init(wg *sync.WaitGroup) {
	if q != nil {
		return
	}

	q = &Queue{
		microTask: &list.List{},
		task:      &list.List{},
		wg:        wg,
	}
}

func Enqueue(callback object.Callback, priority TaskPriority, sendMessage bool) {
	enabled := trace.Enabled()
	q.wg.Add(1)
	q.mu.Lock()
	var l *list.List
	kind := "task"
	queueName := "tasks"

	switch priority {
	case MICRO_TASK:
		l = q.microTask
		kind = "micro_task"
		queueName = "micro_tasks"
	case TASK:
		l = q.task
	}

	if enabled {
		callback.TraceID = nextTraceID.Add(1)
		if callback.TraceLabel == "" {
			callback.TraceLabel = "callback"
		}
	}

	l.PushBack(callback)

	if enabled {
		item := queueItem(callback, kind)
		trace.RecordQueueEvent(trace.QueueEvent{
			Type:   "enqueue",
			Queue:  queueName,
			Item:   item,
			Queues: snapshotLocked(),
			Detail: "queued " + item.Label,
		})
	}

	q.mu.Unlock()

	if sendMessage {
		select {
		case QueueC <- struct{}{}:
		default:
		}
	}
}

func HasTasks() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.microTask.Len() > 0 || q.task.Len() > 0
}

func Dequeue() (object.Callback, TaskPriority, error) {
	q.mu.Lock()
	var l *list.List
	priority := MICRO_TASK

	if q.microTask.Len() > 0 {
		l = q.microTask
	} else if q.task.Len() > 0 {
		l = q.task
		priority = TASK
	}

	if l == nil {
		q.mu.Unlock()
		return object.Callback{}, priority, errors.New("no tasks to dequeue")
	}

	front := l.Front()
	l.Remove(front)
	q.mu.Unlock()
	q.wg.Done()
	return front.Value.(object.Callback), priority, nil
}

func Snapshot() trace.Queues {
	if !trace.Enabled() {
		return trace.Queues{MicroTasks: []trace.QueueItem{}, Tasks: []trace.QueueItem{}}
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return snapshotLocked()
}

func snapshotLocked() trace.Queues {
	limit := trace.CurrentLimits().MaxQueueItems
	microTasks, microOmitted := snapshotList(q.microTask, "micro_task", limit)
	tasks, taskOmitted := snapshotList(q.task, "task", limit)
	return trace.Queues{
		MicroTasks:        microTasks,
		Tasks:             tasks,
		MicroTasksOmitted: microOmitted,
		TasksOmitted:      taskOmitted,
	}
}

func snapshotList(items *list.List, kind string, limit int) ([]trace.QueueItem, int) {
	result := make([]trace.QueueItem, 0, items.Len())
	omitted := 0
	index := 0
	for element := items.Front(); element != nil; element = element.Next() {
		if limit > 0 && index >= limit {
			omitted++
			continue
		}
		callback := element.Value.(object.Callback)
		result = append(result, queueItem(callback, kind))
		index++
	}
	return result, omitted
}

func queueItem(callback object.Callback, kind string) trace.QueueItem {
	label := callback.TraceLabel
	if label == "" {
		label = "callback"
	}
	return trace.QueueItem{
		ID:     "q" + strconv.FormatUint(callback.TraceID, 10),
		Kind:   kind,
		Label:  label,
		Source: callback.TraceSource,
		Native: callback.Native != nil,
	}
}
