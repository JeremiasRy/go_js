package native

import (
	"sync"
	"sync/atomic"
)

var timerRegistry = map[int]*SetTimeout{}
var timerRegistryMutex sync.Mutex
var timerID atomic.Int64

func RegisterTimer(timer *SetTimeout) int {
	id := int(timerID.Add(1))
	timerRegistryMutex.Lock()
	timerRegistry[id] = timer
	timerRegistryMutex.Unlock()
	return id
}

func CancelTimer(id int) bool {
	timerRegistryMutex.Lock()
	timer, found := timerRegistry[id]
	if found {
		delete(timerRegistry, id)
	}
	timerRegistryMutex.Unlock()
	if found {
		timer.Cancel()
	}
	return found
}

type ClearTimeout struct {
	ObjNativeFn
}

func NewClearTimeout(name string) *ClearTimeout {
	c := &ClearTimeout{}
	c.name = name
	return c
}
