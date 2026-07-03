package native

import (
	"fmt"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"strings"
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

func (*Log) Log(values []value.Value, sb *strings.Builder) {
	for i, v := range values {
		notLast := i < len(values)-1
		pad := ""
		if notLast {
			pad = " "
		}

		if sb != nil {
			fmt.Fprintf(sb, "%v%s", String(v), pad)
		} else {
			fmt.Printf("%v%s", String(v), pad)
		}
	}
	if sb != nil {
		fmt.Fprintln(sb)
	} else {
		fmt.Println()
	}
}
