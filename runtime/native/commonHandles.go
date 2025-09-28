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
	KEY_LENGTH  value.Value

	// string
	KEY_INCLUDES    value.Value
	KEY_TOUPPERCASE value.Value

	// object
	KEY_TOSTRING value.Value

	// common class syntax
	KEY_CTOR value.Value

	// error
	KEY_MESSAGE value.Value
)

func createCommonHandles() {
	KEY_PROTO = value.EncodeHandle(allocator.Allocate(LightString(PROTOTYPE_KEY)))
	KEY_LOG = value.EncodeHandle(allocator.Allocate(LightString("log")))

	KEY_FILTER = value.EncodeHandle(allocator.Allocate(LightString("filter")))
	KEY_PUSH = value.EncodeHandle(allocator.Allocate(LightString("push")))
	KEY_FOREACH = value.EncodeHandle(allocator.Allocate(LightString("forEach")))
	KEY_MAP = value.EncodeHandle(allocator.Allocate(LightString("map")))
	KEY_REDUCE = value.EncodeHandle(allocator.Allocate(LightString("reduce")))
	KEY_LENGTH = value.EncodeHandle(allocator.Allocate(LightString("length")))

	KEY_TOSTRING = value.EncodeHandle(allocator.Allocate(LightString("toString")))

	KEY_INCLUDES = value.EncodeHandle(allocator.Allocate(LightString("includes")))
	KEY_TOUPPERCASE = value.EncodeHandle(allocator.Allocate(LightString("toUpperCase")))

	KEY_CTOR = value.EncodeHandle(allocator.Allocate(LightString("constructor")))

	KEY_MESSAGE = value.EncodeHandle(allocator.Allocate(LightString("message")))
}
