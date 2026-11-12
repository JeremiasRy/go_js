package queue

import (
	"container/list"
	"errors"
	"go_js/object"
	"sync"
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
	q.wg.Add(1)
	q.mu.Lock()
	var l *list.List

	switch priority {
	case MICRO_TASK:
		l = q.microTask
	case TASK:
		l = q.task
	}

	l.PushBack(callback)
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

func Dequeue() (object.Callback, error) {
	q.mu.Lock()
	var l *list.List

	if q.microTask.Len() > 0 {
		l = q.microTask
	} else if q.task.Len() > 0 {
		l = q.task
	}

	if l == nil {
		q.mu.Unlock()
		return object.Callback{}, errors.New("no tasks to dequeue")
	}

	front := l.Front()
	l.Remove(front)
	q.mu.Unlock()
	q.wg.Done()
	return front.Value.(object.Callback), nil
}
