package vm

import "fmt"

type ObjType uint8
type Obj interface {
	Type() ObjType
	Debug()
}

const (
	OBJ_FUNCTION ObjType = iota
)

type ObjFunction struct {
	name  string
	chunk *Chunk
}

func NewFunction(name string, chunk *Chunk) *ObjFunction {
	return &ObjFunction{
		chunk: chunk,
		name:  name,
	}
}

func (*ObjFunction) Type() ObjType {
	return OBJ_FUNCTION
}

func (fn *ObjFunction) Debug() {
	fmt.Printf("<fn %s>\n", fn.name)
	fn.chunk.PrintCode()
}
