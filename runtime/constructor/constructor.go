package constructor

import (
	"errors"
	"fmt"
	"go_js/allocator"
	"go_js/object"
	"go_js/stringer"
	"go_js/value"
)

type Constructor interface {
	New(params ...any) (object.Object, error)
}

type ErrorConstructor struct{}

func (*ErrorConstructor) String() string {
	return "Error constructor"
}

func (*ErrorConstructor) Type() object.ObjType {
	return object.OBJ_ERROR_CONSTRUCTOR
}

func (*ErrorConstructor) New(params ...any) (object.Object, error) {
	if len(params) < 1 {
		return nil, errors.New("not enough params for new Error()")
	}

	argHandle := params[0].(value.Value).GetHandle()
	arg, err := allocator.GetObject(argHandle)

	if err != nil {
		return nil, err
	}

	if _, ok := arg.(*object.ObjString); ok {
		errorObject := object.NewError()
		errorObject.Hash["message"] = value.EncodeHandle(argHandle)
		return errorObject, nil
	}

	return nil, fmt.Errorf("arg %s is of wrong type", stringer.String(params[0].(value.Value)))
}

type ObjectConstructor struct{}

func (*ObjectConstructor) String() string {
	return "function Object"
}

func (*ObjectConstructor) Type() object.ObjType {
	return object.OBJ_OBJECT_CONSTRUCTOR
}

func (*ObjectConstructor) New() object.Object {
	obj := &object.ObjObject{Hash: map[string]value.Value{}}

	return obj
}

func NewObject() *object.ObjObject {
	obj := &object.ObjObject{Hash: map[string]value.Value{}}

	return obj
}

type ArrayConstructor struct{}

func (*ArrayConstructor) String() string {
	return "function Array"
}

func (*ArrayConstructor) Type() object.ObjType {
	return object.OBJ_ARRAY_CONSTRUCTOR
}

func (*ArrayConstructor) New(params ...any) *object.ObjArr {
	length := 8 // default length

	if len(params) == 1 {
		if num, ok := params[0].(value.Value); ok {
			length = int(num.AsNumber())
		}
	}

	arr := object.NewObjArr(length)

	filter := allocator.Allocate(object.NewArrayFilter(arr))
	push := allocator.Allocate(object.NewArrayPush(arr))
	forEach := allocator.Allocate(object.NewArrayForEach(arr))

	arr.Hash["filter"] = value.EncodeHandle(filter)
	arr.Hash["push"] = value.EncodeHandle(push)
	arr.Hash["forEach"] = value.EncodeHandle(forEach)
	arr.Hash["length"] = value.ValueFromFloat64(float64(length))

	return arr
}

func NewArray(length int) *object.ObjArr {
	arr := object.NewObjArr(length)

	filter := allocator.Allocate(object.NewArrayFilter(arr))
	push := allocator.Allocate(object.NewArrayPush(arr))
	forEach := allocator.Allocate(object.NewArrayForEach(arr))

	arr.Hash["filter"] = value.EncodeHandle(filter)
	arr.Hash["push"] = value.EncodeHandle(push)
	arr.Hash["forEach"] = value.EncodeHandle(forEach)
	arr.Hash["length"] = value.ValueFromFloat64(float64(length))

	return arr
}

type StringConstructor struct{}

func (*StringConstructor) String() string {
	return "function String"
}

func (*StringConstructor) Type() object.ObjType {
	return object.OBJ_STRING_CONSTRUCTOR
}

func NewString(str string) *object.ObjString {
	objStr := object.NewObjString(str)
	objStr.Hash["toUpperCase"] = value.EncodeHandle(allocator.Allocate(object.NewStringToUpperCase(objStr)))
	objStr.Hash["includes"] = value.EncodeHandle(allocator.Allocate(object.NewStringIncludes(objStr)))

	return objStr
}
