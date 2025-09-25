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
	c.Members = map[string]ObjectValueEntry{}
	c.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)

	log := value.EncodeHandle(allocator.Allocate(LightString("log")))

	c.SetMember(log, value.EncodeHandle(allocator.Allocate(NewLog())))
	return c
}
