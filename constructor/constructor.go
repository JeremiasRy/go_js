package constructor

import (
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
)

type ErrorConstructor struct{}

func (*ErrorConstructor) String() string {
	return "Error constructor"
}

func (*ErrorConstructor) Type() object.ObjType {
	return object.OBJ_ERROR_CONSTRUCTOR
}

func (*ErrorConstructor) New(message string) value.Value {
	errorObject := object.NewError()
	errorObject.Hash["message"] = value.EncodeHandle(allocator.Allocate(object.NewObjString(message)))

	return value.EncodeHandle(allocator.Allocate(errorObject))
}

type ArrayConstructor struct{}

func (*ArrayConstructor) String() string {
	return "Array constructor"
}

func (*ArrayConstructor) Type() object.ObjType {
	return object.OBJ_ARRAY_CONSTRUCTOR
}

func (*ArrayConstructor) New(length int) *object.ObjArr {
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
