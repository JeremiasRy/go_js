package native

import (
	"fmt"
	"go_js/flags"
	"go_js/heap"
	"go_js/object"
	structuredout "go_js/structuredOut"
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

	log := value.EncodeHandle(heap.Allocate(NewLightString("log")))

	c.SetMember(log, value.EncodeHandle(heap.Allocate(NewLog())))
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
		notLast := i < len(values)-1
		pad := ""
		if notLast {
			pad = " "
		}

		if flags.STRUCTURED_OUTPUT {
			structuredout.WriteToOutputBuffer(fmt.Sprintf("%v%s", String(v), pad))
		} else {
			fmt.Printf("%v%s", String(v), pad)
		}
	}
	fmt.Println()
}
