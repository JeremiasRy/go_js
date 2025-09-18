package eventloop

import (
	"container/list"
	"go_js/object"
	"go_js/vm"
	"sync"
)

type EventLoop struct {
	jobCompletionChannel chan *object.ObjFunction
	jobChannel           chan object.Job
	tick                 chan struct{}
	taskQueue            *list.List

	vm *vm.VM
}

var el *EventLoop

func InitLoop(jobChannel chan object.Job, vm *vm.VM) {
	if el != nil {
		return
	}

	el = &EventLoop{
		jobChannel:           jobChannel,
		jobCompletionChannel: make(chan *object.ObjFunction),
		tick:                 make(chan struct{}),
		taskQueue:            &list.List{},
		vm:                   vm,
	}
}

func Start(wg *sync.WaitGroup) {
	go func() {
		for job := range el.jobChannel {
			go func() {
				defer wg.Done()
				wg.Add(1)
				job.Work(el.jobCompletionChannel)
			}()
		}
	}()

	go func() {
		for fn := range el.jobCompletionChannel {
			el.taskQueue.PushBack(fn)
			el.tick <- struct{}{}
		}
	}()

	go func() {
		for range el.tick {
			wg.Add(1)
			front := el.taskQueue.Front()
			el.taskQueue.Remove(front)
			el.vm.Call(front.Value.(*object.ObjFunction), 0)
			el.vm.Run()
			wg.Done()
		}
	}()
}
