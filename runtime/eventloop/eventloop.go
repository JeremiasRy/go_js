package eventloop

import (
	"go_js/object"
	"go_js/queue"
	"go_js/value"
	"sync"
)

type EventLoop struct {
	jobCallbackChannel chan *object.JobChannelMessage
	jobChannel         chan *object.JobChannelMessage

	wg   *sync.WaitGroup
	work int
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
			queue.Enqueue(message.Callback, queue.TASK, true)
			message.Done()
		}
	}()
}

func Dispatch(job object.Job) {
	el.wg.Add(1)
	el.work++
	message := &object.JobChannelMessage{
		Job:      job,
		Callback: object.Callback{Fn: nil, ThisCtx: value.UNDEFINED},
		Done: func() {
			el.wg.Done()
			el.work--
		},
	}
	el.jobChannel <- message
}

func HasWork() bool {
	return el.work > 0
}
