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
