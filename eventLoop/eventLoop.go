package eventloop

import (
	"go_js/object"
	"go_js/queue"
	"sync"
)

type EventLoop struct {
	jobCallbackC chan *object.ObjFunction
	jobC         chan object.Job

	wg *sync.WaitGroup
}

var el *EventLoop

func Init(wg *sync.WaitGroup) {
	if el != nil {
		return
	}

	el = &EventLoop{
		jobC:         make(chan object.Job),
		jobCallbackC: make(chan *object.ObjFunction),
		wg:           wg,
	}
}

func Start() {
	go func() {
		for job := range el.jobC {
			go func() {
				job.Work(el.jobCallbackC)
			}()
		}
	}()

	go func() {
		for fn := range el.jobCallbackC {
			queue.Enqueue(fn, queue.TASK)
		}
	}()
}

func Dispatch(job object.Job) {
	el.wg.Add(1)
	el.jobC <- job
}
