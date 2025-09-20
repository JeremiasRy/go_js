package native

import (
	"fmt"
	"go_js/object"
)

type ObjNativeFn struct {
	name string
}

func (ObjNativeFn) Type() object.ObjType {
	return object.OBJ_NATIVE_FN
}

func (onf *ObjNativeFn) String() string {
	return fmt.Sprintf("<native fn %s()>", onf.name)
}
