package native

import (
	"go_js/allocator"

	"go_js/value"
)

var (
	// prototype
	KEY_PROTO value.Value

	// console
	KEY_LOG value.Value

	// array
	KEY_FILTER  value.Value
	KEY_FOREACH value.Value
	KEY_MAP     value.Value
	KEY_REDUCE  value.Value
	KEY_PUSH    value.Value
	KEY_JOIN    value.Value
	KEY_SHIFT   value.Value
	KEY_REVERSE value.Value
	KEY_LENGTH  value.Value

	// string
	KEY_INCLUDES    value.Value
	KEY_TOUPPERCASE value.Value

	// object
	KEY_TOSTRING       value.Value
	KEY_CREATE         value.Value
	KEY_KEYS           value.Value
	KEY_VALUES         value.Value
	KEY_HASOWNPROPERTY value.Value

	// common class syntax
	KEY_CTOR value.Value

	// error
	KEY_MESSAGE value.Value

	// generator
	KEY_THROW  value.Value
	KEY_NEXT   value.Value
	KEY_RETURN value.Value
	KEY_VALUE  value.Value
	KEY_DONE   value.Value

	// map-set
	KEY_HAS  value.Value
	KEY_GET  value.Value
	KEY_SET  value.Value
	KEY_SIZE value.Value
	KEY_ADD  value.Value

	// promise
	KEY_RESOLVE value.Value
	KEY_THEN    value.Value
)

func createCommonHandles() {
	KEY_PROTO = value.EncodeHandle(allocator.Allocate(LightString(PROTOTYPE_PROPERTY_STRING)))
	KEY_LOG = value.EncodeHandle(allocator.Allocate(LightString("log")))

	KEY_FILTER = value.EncodeHandle(allocator.Allocate(LightString("filter")))
	KEY_PUSH = value.EncodeHandle(allocator.Allocate(LightString("push")))
	KEY_FOREACH = value.EncodeHandle(allocator.Allocate(LightString("forEach")))
	KEY_MAP = value.EncodeHandle(allocator.Allocate(LightString("map")))
	KEY_REDUCE = value.EncodeHandle(allocator.Allocate(LightString("reduce")))
	KEY_JOIN = value.EncodeHandle(allocator.Allocate(LightString("join")))
	KEY_REVERSE = value.EncodeHandle(allocator.Allocate(LightString("reverse")))
	KEY_SHIFT = value.EncodeHandle(allocator.Allocate(LightString("shift")))
	KEY_LENGTH = value.EncodeHandle(allocator.Allocate(LightString("length")))

	KEY_TOSTRING = value.EncodeHandle(allocator.Allocate(LightString("toString")))
	KEY_CREATE = value.EncodeHandle(allocator.Allocate(LightString("create")))
	KEY_KEYS = value.EncodeHandle(allocator.Allocate(LightString("keys")))
	KEY_VALUES = value.EncodeHandle(allocator.Allocate(LightString("values")))
	KEY_HASOWNPROPERTY = value.EncodeHandle(allocator.Allocate(LightString("hasOwnProperty")))

	KEY_INCLUDES = value.EncodeHandle(allocator.Allocate(LightString("includes")))
	KEY_TOUPPERCASE = value.EncodeHandle(allocator.Allocate(LightString("toUpperCase")))

	KEY_CTOR = value.EncodeHandle(allocator.Allocate(LightString("constructor")))

	KEY_MESSAGE = value.EncodeHandle(allocator.Allocate(LightString("message")))

	KEY_THROW = value.EncodeHandle(allocator.Allocate(LightString("throw")))
	KEY_NEXT = value.EncodeHandle(allocator.Allocate(LightString("next")))
	KEY_RETURN = value.EncodeHandle(allocator.Allocate(LightString("return")))
	KEY_VALUE = value.EncodeHandle(allocator.Allocate(LightString("value")))
	KEY_DONE = value.EncodeHandle(allocator.Allocate(LightString("done")))

	KEY_HAS = value.EncodeHandle(allocator.Allocate(LightString("has")))
	KEY_GET = value.EncodeHandle(allocator.Allocate(LightString("get")))
	KEY_SET = value.EncodeHandle(allocator.Allocate(LightString("set")))
	KEY_SIZE = value.EncodeHandle(allocator.Allocate(LightString("size")))
	KEY_ADD = value.EncodeHandle(allocator.Allocate(LightString("add")))

	KEY_RESOLVE = value.EncodeHandle(allocator.Allocate(LightString("resolve")))
	KEY_THEN = value.EncodeHandle(allocator.Allocate(LightString("then")))
}
