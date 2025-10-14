package native

import (
	"fmt"
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
)

type Console struct {
	ObjObject
}

func (*Console) Type() object.ObjType {
	return object.OBJ_CONSOLE
}

func (*Console) String() string {
	return "Object [console]"
}

func NewObjectConsole() *Console {
	c := &Console{}
	c.Members = map[string]ObjectValueEntry{}
	c.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)

	log := value.EncodeHandle(allocator.Allocate(LightString("log")))

	c.SetMember(log, value.EncodeHandle(allocator.Allocate(NewLog())))
	return c
}

type Log struct {
	ObjNativeFn
}

func NewLog() *Log {
	log := &Log{}
	log.name = "log"
	return log
}

func (*Log) Log(values []value.Value) {
	for i, v := range values {
		fmt.Printf("%v", String(v))

		if i < len(values)-1 {
			fmt.Print(" ")
		}
	}
	fmt.Println()
}
