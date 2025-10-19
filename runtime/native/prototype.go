package native

import (
	"fmt"
	"go_js/heap"
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
	MAP_CONSTRUCTOR_NAME      string = "Map"
	SET_CONSTRUCTOR_NAME      string = "Set"
	QUEUE_MICRO_TASK_NAME     string = "queueMicrotask"
	PARSE_INT_NAME            string = "parseInt"
)

var (
	PROTOTYPE_OBJECT              value.Value
	PROTOTYPE_ARRAY               value.Value
	PROTOTYPE_STRING              value.Value
	PROTOTYPE_DATE                value.Value
	PROTOTYPE_GENERATOR           value.Value
	PROTOTYPE_MAP                 value.Value
	PROTOTYPE_SET                 value.Value
	PROTOTYPE_PROMISE_CONSTRUCTOR value.Value
	PROTOTYPE_PROMISE             value.Value
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

	handle := value.EncodeHandle(heap.Allocate(NewToString()))
	p.SetMember(KEY_PROTO, value.NULL)

	p.SetMember(KEY_TOSTRING, handle)
	p.SetMember(KEY_HASOWNPROPERTY, value.EncodeHandle(heap.Allocate(NewHasOwnProperty())))

	PROTOTYPE_OBJECT = value.EncodeHandle(heap.Allocate(p))
	heap.PushToRoots(
		PROTOTYPE_OBJECT,
		KEY_PROTO,
		KEY_TOSTRING,
		KEY_HASOWNPROPERTY,
		KEY_MESSAGE,
	)
}

func initArrayPrototype() {
	if PROTOTYPE_OBJECT == 0 {
		panic("it's important that PROTOTYPE_OBJECT is initialized before PROTOTYPE_ARRAY")
	}
	p := &Prototype{
		name: ARRAY_CONSTRUCTOR_NAME,
	}
	p.Members = map[string]ObjectValueEntry{}

	filter := value.EncodeHandle(heap.Allocate(NewArrayFilter()))
	push := value.EncodeHandle(heap.Allocate(NewArrayPush()))
	forEach := value.EncodeHandle(heap.Allocate(NewArrayForEach()))
	map_ := value.EncodeHandle(heap.Allocate(NewArrayMap()))
	reduce := value.EncodeHandle(heap.Allocate(NewArrayReduce()))
	join := value.EncodeHandle(heap.Allocate(NewArrayJoin()))
	shift := value.EncodeHandle(heap.Allocate(NewArrayShift()))
	reverse := value.EncodeHandle(heap.Allocate(NewArrayReverse()))
	fill := value.EncodeHandle(heap.Allocate(NewArrayFill()))

	p.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)

	p.SetMember(KEY_FILTER, filter)
	p.SetMember(KEY_PUSH, push)
	p.SetMember(KEY_FOREACH, forEach)
	p.SetMember(KEY_MAP, map_)
	p.SetMember(KEY_REDUCE, reduce)
	p.SetMember(KEY_JOIN, join)
	p.SetMember(KEY_SHIFT, shift)
	p.SetMember(KEY_REVERSE, reverse)
	p.SetMember(KEY_FILL, fill)
	p.SetMember(KEY_LENGTH, value.ValueFromFloat64(0))

	PROTOTYPE_ARRAY = value.EncodeHandle(heap.Allocate(p))
	heap.PushToRoots(
		PROTOTYPE_ARRAY,
		KEY_FILTER,
		KEY_PUSH,
		KEY_FOREACH,
		KEY_MAP,
		KEY_REDUCE,
		KEY_JOIN,
		KEY_SHIFT,
		KEY_REVERSE,
		KEY_FILL,
		KEY_LENGTH,
	)
}

func initStringPrototype() {
	if PROTOTYPE_OBJECT == 0 {
		panic("it's important that PROTOTYPE_OBJECT is initialized PROTOTYPE_STRING")
	}

	p := &Prototype{
		name: STRING_CONSTRUCTOR_NAME,
	}

	p.Members = map[string]ObjectValueEntry{}

	includes := value.EncodeHandle(heap.Allocate(NewStringIncludes()))
	toUpperCase := value.EncodeHandle(heap.Allocate(NewStringToUpperCase()))
	replace := value.EncodeHandle(heap.Allocate(NewStringReplace()))
	split := value.EncodeHandle(heap.Allocate(NewStringSplit()))
	startsWith := value.EncodeHandle(heap.Allocate(NewStringStartsWith()))

	p.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)

	p.SetMember(KEY_INCLUDES, includes)
	p.SetMember(KEY_TOUPPERCASE, toUpperCase)
	p.SetMember(KEY_REPLACE, replace)
	p.SetMember(KEY_SPLIT, split)
	p.SetMember(KEY_STARTS_WITH, startsWith)

	PROTOTYPE_STRING = value.EncodeHandle(heap.Allocate(p))
	heap.PushToRoots(
		PROTOTYPE_STRING,
		KEY_INCLUDES,
		KEY_TOUPPERCASE,
		KEY_REPLACE,
		KEY_SPLIT,
		KEY_STARTS_WITH,
	)
}

func initGeneratorPrototype() {
	if PROTOTYPE_OBJECT == 0 {
		panic("it's important that PROTOTYPE_OBJECT is initialized PROTOTYPE_GENERATOR")
	}

	p := &Prototype{
		name: GENERATOR_NAME,
	}

	p.Members = map[string]ObjectValueEntry{}

	next := value.EncodeHandle(heap.Allocate(NewNext()))
	throw := value.EncodeHandle(heap.Allocate(NewThrow()))
	return_ := value.EncodeHandle(heap.Allocate(NewReturn()))

	p.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)

	p.SetMember(KEY_NEXT, next)
	p.SetMember(KEY_THROW, throw)
	p.SetMember(KEY_RETURN, return_)

	PROTOTYPE_GENERATOR = value.EncodeHandle(heap.Allocate(p))
	heap.PushToRoots(
		PROTOTYPE_GENERATOR,
		KEY_NEXT,
		KEY_THROW,
		KEY_RETURN,
	)
}

func initMapPrototype() {
	if PROTOTYPE_OBJECT == 0 {
		panic("it's important that PROTOTYPE_OBJECT is initialized PROTOTYPE_MAP")
	}

	p := &Prototype{
		name: MAP_CONSTRUCTOR_NAME,
	}

	p.Members = map[string]ObjectValueEntry{}

	mapHas := value.EncodeHandle(heap.Allocate(NewMapHas()))
	mapGet := value.EncodeHandle(heap.Allocate(NewMapGet()))
	mapSet := value.EncodeHandle(heap.Allocate(NewMapSet()))
	mapKeys := value.EncodeHandle(heap.Allocate(NewMapKeys()))

	p.SetMember(KEY_HAS, mapHas)
	p.SetMember(KEY_SET, mapSet)
	p.SetMember(KEY_GET, mapGet)
	p.SetMember(KEY_KEYS, mapKeys)

	PROTOTYPE_MAP = value.EncodeHandle(heap.Allocate(p))
	heap.PushToRoots(
		PROTOTYPE_MAP,
		KEY_HAS,
		KEY_SET,
		KEY_GET,
		KEY_KEYS,
		KEY_SIZE,
	)
}

func initSetPrototype() {
	if PROTOTYPE_OBJECT == 0 {
		panic("it's important that PROTOTYPE_OBJECT is initialized PROTOTYPE_SET")
	}
	p := &Prototype{
		name: SET_CONSTRUCTOR_NAME,
	}

	p.Members = map[string]ObjectValueEntry{}

	setHas := value.EncodeHandle(heap.Allocate(NewSetHas()))
	setAdd := value.EncodeHandle(heap.Allocate(NewSetAdd()))

	p.SetMember(KEY_HAS, setHas)
	p.SetMember(KEY_ADD, setAdd)

	PROTOTYPE_SET = value.EncodeHandle(heap.Allocate(p))
	heap.PushToRoots(
		PROTOTYPE_SET,
		KEY_HAS,
		KEY_ADD,
	)
}

func initPromiseConstructorPrototype() {
	if PROTOTYPE_OBJECT == 0 {
		panic("it's important that PROTOTYPE_OBJECT is initialized PROTOTYPE_PROMISE")
	}
	p := &Prototype{
		name: PROMISE_CONSTRUCTOR_NAME,
	}

	p.Members = map[string]ObjectValueEntry{}

	resolve := value.EncodeHandle(heap.Allocate(NewResolve()))

	p.SetMember(KEY_RESOLVE, resolve)

	PROTOTYPE_PROMISE_CONSTRUCTOR = value.EncodeHandle(heap.Allocate(p))
	heap.PushToRoots(
		PROTOTYPE_PROMISE_CONSTRUCTOR,
		KEY_RESOLVE,
	)
}

func initPromisePrototype() {
	if PROTOTYPE_OBJECT == 0 {
		panic("it's important that PROTOTYPE_OBJECT is initialized PROTOTYPE_RESOLVED_PROMISE")
	}

	p := &Prototype{
		name: PROMISE_CONSTRUCTOR_NAME,
	}

	p.Members = map[string]ObjectValueEntry{}

	then := value.EncodeHandle(heap.Allocate(NewThen()))

	p.SetMember(KEY_THEN, then)

	PROTOTYPE_PROMISE = value.EncodeHandle(heap.Allocate(p))
	heap.PushToRoots(
		PROTOTYPE_PROMISE,
		KEY_THEN,
	)
}
