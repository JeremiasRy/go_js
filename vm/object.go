package vm

type ObjType uint8

const (
	OBJ_FUNCTION ObjType = iota
	OBJ_STRING
)

const MAIN_FN_NAME = "PROGRAM_MAIN"

type Object interface {
	Type() ObjType
	String() string
}

type ObjFunction struct {
	name  string
	chunk *Chunk
	arity int
}

func (ObjFunction) Type() ObjType {
	return OBJ_FUNCTION
}

func NewFunction(name string, arity int) *ObjFunction {
	return &ObjFunction{
		name:  name,
		chunk: NewChunk(),
		arity: arity,
	}
}

func (fn ObjFunction) String() string {
	return "<fn " + fn.name + ">"
}

type ObjString string

func (ObjString) Type() ObjType {
	return OBJ_STRING
}

func (str ObjString) String() string {
	return string("\"" + str + "\"")
}
