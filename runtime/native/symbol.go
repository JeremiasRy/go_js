package native

import (
	"fmt"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"sync/atomic"
)

const SYMBOL_CONSTRUCTOR_NAME = "Symbol"

var symbolCounter uint64
var symbolRegistry = map[string]value.Value{}
var symbolByKey = map[string]value.Value{}

var (
	SYMBOL_ITERATOR             value.Value
	SYMBOL_ASYNC_ITERATOR       value.Value
	SYMBOL_HAS_INSTANCE         value.Value
	SYMBOL_SPECIES              value.Value
	SYMBOL_IS_CONCAT_SPREADABLE value.Value
	SYMBOL_TO_PRIMITIVE         value.Value
	SYMBOL_TO_STRING_TAG        value.Value
	SYMBOL_MATCH                value.Value
	SYMBOL_REPLACE              value.Value
	SYMBOL_SEARCH               value.Value
	SYMBOL_SPLIT                value.Value
	SYMBOL_MATCH_ALL            value.Value
)

type ObjSymbol struct {
	ObjObject
	Description string
	Key         string
	WellKnown   string
}

func NewSymbol(description string) *ObjSymbol {
	id := atomic.AddUint64(&symbolCounter, 1)
	s := &ObjSymbol{Description: description, Key: fmt.Sprintf("\x00symbol:%d", id)}
	s.Members = map[string]ObjectValueEntry{}
	s.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	if description != "" {
		s.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("description"))), value.EncodeHandle(heap.Allocate(NewLightString(description))))
	}
	return s
}

func (*ObjSymbol) Type() object.ObjType {
	return object.OBJ_SYMBOL
}

func (s *ObjSymbol) String() string {
	return "Symbol(" + s.Description + ")"
}

type SymbolConstructor struct {
	ObjObject
}

func NewSymbolConstructor() *SymbolConstructor {
	sc := &SymbolConstructor{}
	sc.Members = map[string]ObjectValueEntry{}
	sc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("for"))), value.EncodeHandle(heap.Allocate(NewSymbolFor())))
	sc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("keyFor"))), value.EncodeHandle(heap.Allocate(NewSymbolKeyFor())))
	for _, name := range []string{"iterator", "asyncIterator", "hasInstance", "species", "isConcatSpreadable", "toPrimitive", "toStringTag", "match", "replace", "search", "split", "matchAll"} {
		symbol := NewSymbol(name)
		symbol.WellKnown = name
		handle := value.EncodeHandle(heap.Allocate(symbol))
		RegisterSymbol(handle)
		sc.SetMember(value.EncodeHandle(heap.Allocate(NewLightString(name))), handle)
		heap.PushToRoots(handle)
		switch name {
		case "iterator":
			SYMBOL_ITERATOR = handle
		case "asyncIterator":
			SYMBOL_ASYNC_ITERATOR = handle
		case "hasInstance":
			SYMBOL_HAS_INSTANCE = handle
		case "species":
			SYMBOL_SPECIES = handle
		case "isConcatSpreadable":
			SYMBOL_IS_CONCAT_SPREADABLE = handle
		case "toPrimitive":
			SYMBOL_TO_PRIMITIVE = handle
		case "toStringTag":
			SYMBOL_TO_STRING_TAG = handle
		case "match":
			SYMBOL_MATCH = handle
		case "replace":
			SYMBOL_REPLACE = handle
		case "search":
			SYMBOL_SEARCH = handle
		case "split":
			SYMBOL_SPLIT = handle
		case "matchAll":
			SYMBOL_MATCH_ALL = handle
		}
	}
	return sc
}

func (*SymbolConstructor) Type() object.ObjType {
	return object.OBJ_SYMBOL_CONSTRUCTOR
}

func (*SymbolConstructor) String() string {
	return "function Symbol"
}

type SymbolFor struct {
	ObjNativeFn
}

func NewSymbolFor() *SymbolFor {
	s := &SymbolFor{}
	s.name = "for"
	return s
}

type SymbolKeyFor struct {
	ObjNativeFn
}

func NewSymbolKeyFor() *SymbolKeyFor {
	s := &SymbolKeyFor{}
	s.name = "keyFor"
	return s
}

func SymbolForDescription(description string) value.Value {
	if existing, found := symbolRegistry[description]; found {
		return existing
	}
	symbol := NewSymbol(description)
	symbol.WellKnown = description
	result := value.EncodeHandle(heap.Allocate(symbol))
	symbolRegistry[description] = result
	RegisterSymbol(result)
	heap.PushToRoots(result)
	return result
}

func SymbolDescription(v value.Value) (string, bool) {
	if !v.IsObject() {
		return "", false
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return "", false
	}
	symbol, ok := obj.(*ObjSymbol)
	if !ok {
		return "", false
	}
	return symbol.Description, true
}

func SymbolKeyForDescription(description string) (value.Value, bool) {
	v, found := symbolRegistry[description]
	return v, found
}

func RegisterSymbol(v value.Value) {
	if !v.IsObject() {
		return
	}
	obj, err := heap.GetObject(v.GetHandle())
	if err != nil {
		return
	}
	if symbol, ok := obj.(*ObjSymbol); ok {
		symbolByKey[symbol.Key] = v
	}
}

func SymbolByKey(key string) (value.Value, bool) {
	v, found := symbolByKey[key]
	return v, found
}
