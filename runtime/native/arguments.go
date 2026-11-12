package native

import (
	"go_js/object"
	"go_js/value"
)

type MappedArguments struct {
	ObjArr
	Slots []*value.Value
}

func NewMappedArguments(items []value.Value, slots []*value.Value) *MappedArguments {
	arr := NewArrayFrom(items)
	return &MappedArguments{ObjArr: *arr, Slots: slots}
}

func (*MappedArguments) Type() object.ObjType { return object.OBJ_ARRAY }

func (m *MappedArguments) GetIndex(i int) value.Value {
	if i >= 0 && i < len(m.Slots) {
		return *m.Slots[i]
	}
	if i >= 0 && i < m.ObjArr.Length() {
		return m.ObjArr.GetElementAt(i)
	}
	return value.UNDEFINED
}

func (m *MappedArguments) SetIndex(i int, v value.Value) {
	if i >= 0 && i < len(m.Slots) {
		*m.Slots[i] = v
	}
	if i >= 0 && i < m.ObjArr.Length() {
		m.ObjArr.SetElementAt(i, v)
	}
}

func (m *MappedArguments) GetReferencingValues() []value.Value {
	arr := m.ObjArr.GetReferencingValues()
	for _, slot := range m.Slots {
		if slot != nil {
			arr = append(arr, *slot)
		}
	}
	return arr
}
