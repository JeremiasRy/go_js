package object

import "go_js/value"

type Iterable interface {
	Values() []value.Value
}

type Iterator struct {
	current int
	values  []value.Value
}

func NewIterator(obj Iterable) *Iterator {
	return &Iterator{values: obj.Values(), current: -1}
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
