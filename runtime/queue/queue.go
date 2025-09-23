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
	wg        *sync.WaitGroup
	microTask *list.List
	task      *list.List
}

var q *Queue

// If vm has drained out the queue, but there are still jobs active, this channel is used to notify the VM to pick up a job
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

func Enqueue(v *object.QueueValue, priority TaskPriority) {
	var l *list.List

	switch priority {
	case MICRO_TASK:
		l = q.microTask
	case TASK:
		l = q.task
	}

	l.PushBack(v)
	QueueC <- struct{}{}
}

func Dequeue() *object.QueueValue {
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
	return front.Value.(*object.QueueValue)
}
