package object

import (
	"fmt"
	"strings"
)

// for now just used for building the string at runtime. Could also be used to cache the result?
type ObjTemplateLiteral struct {
	builder *strings.Builder
}

func NewObjTemplateLiteral() *ObjTemplateLiteral {
	return &ObjTemplateLiteral{
		builder: &strings.Builder{},
	}
}

func (i *ObjTemplateLiteral) PushString(s string) error {
	_, err := fmt.Fprint(i.builder, s)
	return err
}

func (i *ObjTemplateLiteral) CreateString() string {
	str := i.builder.String()
	i.builder = nil
	return str
}

func (i *ObjTemplateLiteral) String() string {
	return "template literal builder"
}

func (i *ObjTemplateLiteral) Type() ObjType {
	return OBJ_TEMPLATE_LITERAL
}
