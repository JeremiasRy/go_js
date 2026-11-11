package native

import (
	"go_js/object"
	"go_js/value"
)

type JSIterator struct {
	marked     bool
	Iterator   value.Value
	NextMethod value.Value
	Current    value.Value
	Done       bool
	Started    bool
}

func NewJSIterator(iterator value.Value) *JSIterator {
	return &JSIterator{Iterator: iterator, Current: value.UNDEFINED}
}

func (*JSIterator) Type() object.ObjType {
	return object.OBJ_ITERATOR
}

func (i *JSIterator) String() string {
	return "Iterator object"
}

type IteratorNext struct {
	ObjNativeFn
}

func NewIteratorNext() *IteratorNext {
	i := &IteratorNext{}
	i.name = "next"
	return i
}

func (i *JSIterator) GetReferencingValues() []value.Value {
	return []value.Value{i.Iterator, i.NextMethod, i.Current}
}

func (i *JSIterator) Mark() {
	i.marked = true
}

func (i *JSIterator) Marked() bool {
	return i.marked
}

func (i *JSIterator) Clear() {
	i.marked = false
}
