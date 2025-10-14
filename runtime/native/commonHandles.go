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
	KEY_SPLIT       value.Value
	KEY_REPLACE     value.Value
	KEY_STARTS_WITH value.Value

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
	KEY_PROTO = value.EncodeHandle(allocator.Allocate(NewLightString(PROTOTYPE_PROPERTY_STRING)))
	KEY_LOG = value.EncodeHandle(allocator.Allocate(NewLightString("log")))

	KEY_FILTER = value.EncodeHandle(allocator.Allocate(NewLightString("filter")))
	KEY_PUSH = value.EncodeHandle(allocator.Allocate(NewLightString("push")))
	KEY_FOREACH = value.EncodeHandle(allocator.Allocate(NewLightString("forEach")))
	KEY_MAP = value.EncodeHandle(allocator.Allocate(NewLightString("map")))
	KEY_REDUCE = value.EncodeHandle(allocator.Allocate(NewLightString("reduce")))
	KEY_JOIN = value.EncodeHandle(allocator.Allocate(NewLightString("join")))
	KEY_REVERSE = value.EncodeHandle(allocator.Allocate(NewLightString("reverse")))
	KEY_SHIFT = value.EncodeHandle(allocator.Allocate(NewLightString("shift")))
	KEY_LENGTH = value.EncodeHandle(allocator.Allocate(NewLightString("length")))

	KEY_TOSTRING = value.EncodeHandle(allocator.Allocate(NewLightString("toString")))
	KEY_CREATE = value.EncodeHandle(allocator.Allocate(NewLightString("create")))
	KEY_KEYS = value.EncodeHandle(allocator.Allocate(NewLightString("keys")))
	KEY_VALUES = value.EncodeHandle(allocator.Allocate(NewLightString("values")))
	KEY_HASOWNPROPERTY = value.EncodeHandle(allocator.Allocate(NewLightString("hasOwnProperty")))

	KEY_INCLUDES = value.EncodeHandle(allocator.Allocate(NewLightString("includes")))
	KEY_TOUPPERCASE = value.EncodeHandle(allocator.Allocate(NewLightString("toUpperCase")))
	KEY_SPLIT = value.EncodeHandle(allocator.Allocate(NewLightString("split")))
	KEY_REPLACE = value.EncodeHandle(allocator.Allocate(NewLightString("replace")))
	KEY_STARTS_WITH = value.EncodeHandle(allocator.Allocate(NewLightString("startsWith")))

	KEY_CTOR = value.EncodeHandle(allocator.Allocate(NewLightString("constructor")))

	KEY_MESSAGE = value.EncodeHandle(allocator.Allocate(NewLightString("message")))

	KEY_THROW = value.EncodeHandle(allocator.Allocate(NewLightString("throw")))
	KEY_NEXT = value.EncodeHandle(allocator.Allocate(NewLightString("next")))
	KEY_RETURN = value.EncodeHandle(allocator.Allocate(NewLightString("return")))
	KEY_VALUE = value.EncodeHandle(allocator.Allocate(NewLightString("value")))
	KEY_DONE = value.EncodeHandle(allocator.Allocate(NewLightString("done")))

	KEY_HAS = value.EncodeHandle(allocator.Allocate(NewLightString("has")))
	KEY_GET = value.EncodeHandle(allocator.Allocate(NewLightString("get")))
	KEY_SET = value.EncodeHandle(allocator.Allocate(NewLightString("set")))
	KEY_SIZE = value.EncodeHandle(allocator.Allocate(NewLightString("size")))
	KEY_ADD = value.EncodeHandle(allocator.Allocate(NewLightString("add")))

	KEY_RESOLVE = value.EncodeHandle(allocator.Allocate(NewLightString("resolve")))
	KEY_THEN = value.EncodeHandle(allocator.Allocate(NewLightString("then")))
}
