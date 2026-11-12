package eventloop

import (
	"go_js/object"
	"go_js/queue"
	"go_js/value"
	"sync"
	"sync/atomic"
)

type EventLoop struct {
	jobCallbackChannel chan *object.JobChannelMessage
	jobChannel         chan *object.JobChannelMessage

	wg   *sync.WaitGroup
	work atomic.Int64
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
	el.wg.Add(1)
	el.work.Add(1)
	message := &object.JobChannelMessage{
		Job:      job,
		Callback: object.Callback{Fn: nil, ThisCtx: value.UNDEFINED},
		Done: func() {
			el.wg.Done()
			el.work.Add(-1)
		},
	}
	el.jobChannel <- message
}

func HasWork() bool {
	return el.work.Load() > 0
}
