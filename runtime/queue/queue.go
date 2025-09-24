package queue

import (
	"container/list"
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

func Enqueue(callback object.Callable, priority TaskPriority) {
	q.wg.Add(1)
	var l *list.List

	switch priority {
	case MICRO_TASK:
		l = q.microTask
	case TASK:
		l = q.task
	}

	l.PushBack(callback)
	QueueC <- struct{}{}
}

func Dequeue() object.Callable {
	var l *list.List

	if q.microTask.Len() > 0 {
		l = q.microTask
	} else if q.task.Len() > 0 {
		l = q.task
	}

	if l == nil {
		return nil
	}

	front := l.Front()
	l.Remove(front)
	q.wg.Done()
	return front.Value.(object.Callable)
}
