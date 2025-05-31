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
	name  string
	chunk *Chunk
}

type ObjString string

func (*ObjFunction) Type() ObjType {
	return OBJ_FUNCTION
}

func (*ObjString) Type() ObjType {
	return OBJ_STRING
}
