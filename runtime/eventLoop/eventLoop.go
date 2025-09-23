package eventloop

import (
	"go_js/object"
	"go_js/queue"
	"sync"
)

type EventLoop struct {
	jobCallbackChannel chan *object.CallbackChannelValue
	jobChannel         chan *JobChannelValue
	jobs               int

	wg *sync.WaitGroup
}

type JobChannelValue struct {
	job  object.Job
	done func()
}

var el *EventLoop

func Init(wg *sync.WaitGroup) {
	if el != nil {
		return
	}

	el = &EventLoop{
		jobChannel:         make(chan *JobChannelValue),
		jobCallbackChannel: make(chan *object.CallbackChannelValue),
		wg:                 wg,
	}
}

func Start() {
	go func() {
		for message := range el.jobChannel {
			go func() {
				message.job.Work(el.jobCallbackChannel, message.done)
			}()
		}
	}()

	go func() {
		for message := range el.jobCallbackChannel {
			queue.Enqueue(message.Qv, queue.TASK)
			message.Done()
		}
	}()
}

func DispatchJob(job object.Job) {
	el.jobs++
	el.wg.Add(1)
	el.jobChannel <- &JobChannelValue{job: job, done: func() {
		el.wg.Done()
		el.jobs--
	}}
}

func Dispatch(job object.Job) {
	el.jobs++
	el.jobChannel <- &JobChannelValue{job: job, done: func() { el.jobs-- }}
}

func HasJobs() bool {
	return el.jobs > 0
}
