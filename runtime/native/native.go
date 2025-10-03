package native

import (
	"fmt"
	"go_js/object"
)

type ObjNativeFn struct {
	object.Ctx
	name string
}

func (ObjNativeFn) Type() object.ObjType {
	return object.OBJ_NATIVE_FN
}

func (onf *ObjNativeFn) String() string {
	return fmt.Sprintf("<native fn %s()>", onf.name)
}

func Init() {
	createCommonHandles()
	initObjectPrototype()

	initArrayPrototype()
	initStringPrototype()
	initGeneratorPrototype()
	initMapPrototype()
	initSetPrototype()
}
