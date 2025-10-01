package native

import (
	"fmt"
	"go_js/allocator"
	"go_js/object"
	"go_js/value"
)

const (
	PROTOTYPE_PROPERTY_STRING string = "prototype"
	OBJECT_CONSTRUCTOR_NAME   string = "Object"
	ARRAY_CONSTRUCTOR_NAME    string = "Array"
	STRING_CONSTRUCTOR_NAME   string = "String"
	PROMISE_CONSTRUCTOR_NAME  string = "Promise"
	ERROR_CONSTRUCTOR_NAME    string = "Error"
	DATE_CONSTRUCTOR_NAME     string = "Date"
	GENERATOR_NAME            string = "Generator"
)

var (
	PROTOTYPE_OBJECT    value.Value
	PROTOTYPE_ARRAY     value.Value
	PROTOTYPE_STRING    value.Value
	PROTOTYPE_DATE      value.Value
	PROTOTYPE_GENERATOR value.Value
)

type Prototype struct {
	ObjObject
	name string
}

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
	p.SetMember(KEY_PROTO, value.TAG_NIL)

	p.SetMember(KEY_TOSTRING, handle)
	p.SetMember(KEY_HASOWNPROPERTY, value.EncodeHandle(allocator.Allocate(NewHasOwnProperty())))

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

func initGeneratorPrototype() {
	if PROTOTYPE_OBJECT == 0 {
		panic("it's important that PROTOTYPE_OBJECT is initialized PROTOTYPE_GENERATOR")
	}

	p := &Prototype{
		name: GENERATOR_NAME,
	}

	p.Members = map[string]ObjectValueEntry{}

	next := value.EncodeHandle(allocator.Allocate(NewNext()))
	throw := value.EncodeHandle(allocator.Allocate(NewThrow()))
	return_ := value.EncodeHandle(allocator.Allocate(NewReturn()))

	p.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)

	p.SetMember(KEY_NEXT, next)
	p.SetMember(KEY_THROW, throw)
	p.SetMember(KEY_RETURN, return_)

	PROTOTYPE_GENERATOR = value.EncodeHandle(allocator.Allocate(p))

}
