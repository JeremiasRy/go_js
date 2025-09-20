package native

import (
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
	c.Hash = map[string]ObjectValueEntry{}

	c.Hash["log"] = c.NewValueEntry(value.EncodeHandle(allocator.Allocate(NewLog())))
	return c
}
