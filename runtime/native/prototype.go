package native

import (
	"fmt"
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
)

const (
	PROTOTYPE_KEY            string = "__proto__"
	OBJECT_CONSTRUCTOR_NAME  string = "Object"
	ARRAY_CONSTRUCTOR_NAME   string = "Array"
	STRING_CONSTRUCTOR_NAME  string = "String"
	PROMISE_CONSTRUCTOR_NAME string = "Promise"
	ERROR_CONSTRUCTOR_NAME   string = "Error"
	DATE_CONSTRUCTOR_NAME    string = "Date"
)

var (
	PROTOTYPE_OBJECT value.Value
	PROTOTYPE_ARRAY  value.Value
	PROTOTYPE_STRING value.Value
	PROTOTYPE_DATE   value.Value
)

type Prototype struct {
	ObjObject
	name string
}

// somewhat ugly empty struct to recognize instance methods
type InstanceMethod struct{}

func (InstanceMethod) Instance() {}

type Instancer interface {
	Instance()
}

// end of ugly

func (p *Prototype) Name() string {
	return p.name
}

func (p *Prototype) String() string {
	return fmt.Sprintf("Prototype %s", p.name)
}

func (p *Prototype) Type() object.ObjType {
	return object.OBJ_PROTOTYPE
}

func NewPrototype(name string) *Prototype {
	p := &Prototype{
		name: name,
	}
	p.Members = map[string]ObjectValueEntry{}

	return p
}

func initObjectPrototype() {
	p := &Prototype{
		name: OBJECT_CONSTRUCTOR_NAME,
	}
	p.Members = map[string]ObjectValueEntry{}

	handle := value.EncodeHandle(allocator.Allocate(NewToString()))
	p.SetMember(KEY_PROTO, value.EncodeNil())

	p.SetMember(KEY_TOSTRING, handle)

	PROTOTYPE_OBJECT = value.EncodeHandle(allocator.Allocate(p))
}

func initArrayPrototype() {
	if PROTOTYPE_OBJECT == 0 {
		panic("it's important that PROTOTYPE_OBJECT is initialized before PROTOTYPE_ARRAY")
	}
	p := &Prototype{
		name: ARRAY_CONSTRUCTOR_NAME,
	}
	p.Members = map[string]ObjectValueEntry{}

	filter := value.EncodeHandle(allocator.Allocate(NewArrayFilter()))
	push := value.EncodeHandle(allocator.Allocate(NewArrayPush()))
	forEach := value.EncodeHandle(allocator.Allocate(NewArrayForEach()))
	map_ := value.EncodeHandle(allocator.Allocate(NewArrayMap()))
	reduce := value.EncodeHandle(allocator.Allocate(NewArrayReduce()))

	p.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)

	p.SetMember(KEY_FILTER, filter)
	p.SetMember(KEY_PUSH, push)
	p.SetMember(KEY_FOREACH, forEach)
	p.SetMember(KEY_MAP, map_)
	p.SetMember(KEY_REDUCE, reduce)
	p.SetMember(KEY_LENGTH, value.ValueFromFloat64(0))

	PROTOTYPE_ARRAY = value.EncodeHandle(allocator.Allocate(p))
}

func initStringPrototype() {
	if PROTOTYPE_OBJECT == 0 {
		panic("it's important that PROTOTYPE_OBJECT is initialized PROTOTYPE_STRING")
	}

	p := &Prototype{
		name: STRING_CONSTRUCTOR_NAME,
	}

	p.Members = map[string]ObjectValueEntry{}

	includes := value.EncodeHandle(allocator.Allocate(NewStringIncludes()))
	toUpperCase := value.EncodeHandle(allocator.Allocate(NewStringToUpperCase()))

	p.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)

	p.SetMember(KEY_INCLUDES, includes)
	p.SetMember(KEY_TOUPPERCASE, toUpperCase)

	PROTOTYPE_STRING = value.EncodeHandle(allocator.Allocate(p))
}
