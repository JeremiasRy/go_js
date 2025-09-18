package object

import (
	"fmt"
	"go_js/value"
	"math"
	"time"
)

type ObjNativeFn struct {
	name string
}

func (ObjNativeFn) Type() ObjType {
	return OBJ_NATIVE_FN
}

func (onf *ObjNativeFn) String() string {
	return fmt.Sprintf("<native fn %s()>", onf.name)
}

type Log struct {
	ObjNativeFn
}

func NewLog() *Log {
	log := &Log{}
	log.name = "log"
	return log
}

type Clock struct {
	ObjNativeFn
}

func NewClock() *Clock {
	clock := &Clock{}
	clock.name = "Clock"
	return clock
}

func (*Clock) Clock() value.Value {
	return value.Value(math.Float64bits(float64(time.Now().UnixMilli())))
}

type SetTimeout struct {
	ObjNativeFn
	time     int
	callback *ObjFunction
}

func NewSetTimeout() *SetTimeout {
	setTimeout := &SetTimeout{}
	setTimeout.name = "setTimeout"

	return setTimeout
}

func (st *SetTimeout) Set(ms int, callback *ObjFunction) {
	st.time = ms
	st.callback = callback
}

func (st *SetTimeout) Work(callBack chan *ObjFunction) {
	tick := time.NewTicker((time.Duration(st.time) * time.Millisecond))

	for range tick.C {
		callBack <- st.callback
		break
	}
}

func (st *SetTimeout) CloneForDispatch() *SetTimeout {
	clone := *st
	return &clone
}

type Main struct {
	Fn *ObjFunction
}

func NewMain(fn *ObjFunction) *Main {
	return &Main{
		Fn: fn,
	}
}

func (m *Main) Work(callbackChannel chan *ObjFunction) {
	callbackChannel <- m.Fn
}
