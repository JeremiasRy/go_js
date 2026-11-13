package native

import (
	"sort"
	"sync"
	"sync/atomic"
)

var timerRegistry = map[int]*SetTimeout{}
var timerRegistryMutex sync.Mutex
var timerID atomic.Int64

type TimerInfo struct {
	ID      int
	DelayMS int
	Label   string
	Repeats bool
	State   string
}

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
		timer.Cancel()
	}
	timerRegistryMutex.Unlock()
	return found
}

func TimerInfoFor(id int) (TimerInfo, bool) {
	timerRegistryMutex.Lock()
	defer timerRegistryMutex.Unlock()
	timer, found := timerRegistry[id]
	if !found || timer == nil {
		return TimerInfo{}, false
	}
	return timerInfo(id, timer), true
}

func TimerSnapshot() []TimerInfo {
	timerRegistryMutex.Lock()
	result := make([]TimerInfo, 0, len(timerRegistry))
	for id, timer := range timerRegistry {
		if timer == nil {
			continue
		}
		result = append(result, timerInfo(id, timer))
	}
	timerRegistryMutex.Unlock()
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func timerInfo(id int, timer *SetTimeout) TimerInfo {
	state := "pending"
	if timer.state != nil {
		switch timer.state.Load() {
		case timerFired:
			state = "fired"
		case timerCancelled:
			state = "cancelled"
		}
	}
	label := timer.traceLabel
	if label == "" {
		label = "timer"
	}
	return TimerInfo{
		ID:      id,
		DelayMS: timer.time,
		Label:   label,
		Repeats: timer.interval,
		State:   state,
	}
}

type ClearTimeout struct {
	ObjNativeFn
}

func NewClearTimeout(name string) *ClearTimeout {
	c := &ClearTimeout{}
	c.name = name
	return c
}
