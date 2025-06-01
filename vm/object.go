package vm

type ObjType uint8

const (
	OBJ_FUNCTION ObjType = iota
	OBJ_STRING
)

type Object interface {
	Type() ObjType
}

type ObjFunction struct {
	name   string
	chunk  *Chunk
	locals []Value
}

func (ObjFunction) Type() ObjType {
	return OBJ_FUNCTION
}

func NewFunction(name string) *ObjFunction {
	return &ObjFunction{
		name:   name,
		chunk:  NewChunk(),
		locals: []Value{},
	}
}

func (fn *ObjFunction) AddLocal(v Value) int {
	fn.locals = append(fn.locals, v)
	return len(fn.locals)
}

func (fn *ObjFunction) GetLocal(index int) Value {
	return fn.locals[index]
}

type ObjString string

func (ObjString) Type() ObjType {
	return OBJ_STRING
}
