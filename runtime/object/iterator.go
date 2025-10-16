package object

import (
	"go_js/value"
)

type Iterable interface {
	Values() []value.Value
	Keys() []value.Value
}

type Iterator struct {
	marked  bool
	current int
	values  []value.Value
}

func NewValueIterator(obj Iterable) *Iterator {
	return &Iterator{values: obj.Values(), current: -1}
}

func NewKeyIterator(obj Iterable) *Iterator {
	return &Iterator{values: obj.Keys(), current: -1}
}

func (i *Iterator) Next() bool {
	if i.current >= len(i.values)-1 {
		return true
	}
	i.current++
	return false
}

func (i *Iterator) Current() value.Value {
	return i.values[i.current]
}

func (i *Iterator) String() string {
	return "Iterator object"
}

func (i *Iterator) Type() ObjType {
	return OBJ_ITERATOR
}

func (i *Iterator) GetReferencingValues() []value.Value {
	arr := []value.Value{}
	for _, v := range i.values {
		if v.IsObject() {
			arr = append(arr, v)
		}
	}
	return arr
}

func (i *Iterator) Mark() {
	i.marked = true
}

func (i *Iterator) Marked() bool {
	return i.marked
}

func (i *Iterator) Clear() {
	i.marked = false
}
