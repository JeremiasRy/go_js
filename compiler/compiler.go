package compiler

import (
	"go_js/object"
	"go_js/parser"
)

func Compile(ast *parser.Node) (*object.ObjFunction, error) {
	main := object.NewFunction(object.MAIN_FN_NAME, 0)
	return main, nil
}

func declareVariables(current *parser.Node) {

}

func generateByteCode() {

}
