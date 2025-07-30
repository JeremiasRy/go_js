package compiler

import (
	"fmt"
	"go_js/object"
	"go_js/parser"
)

type VariableType int

const (
	FUNCTION VariableType = iota
	CONST
	LET
)

type VariableScope int

const (
	HEAP VariableScope = iota
	LOCAL
	GLOBAL
)

type Variable struct {
	scope VariableScope
	type_ VariableType
	slot  int // for locals and globals
}

type Variables map[string]*Variable

type SymbolTable struct {
	parent      *SymbolTable
	vars        Variables
	globalCount int
	localCount  int
}

func newSymbolTable(parent *SymbolTable) *SymbolTable {
	return &SymbolTable{parent: parent, vars: map[string]*Variable{}, globalCount: -1, localCount: -1}
}

func (st *SymbolTable) addVariable(name string, scope VariableScope, type_ VariableType) {
	if _, found := st.vars[name]; found {
		fmt.Printf("WARN: Already found variable %s from symbol table\n", name)
		return
	}
	variable := &Variable{scope: scope, type_: type_, slot: -1}

	switch scope {
	case LOCAL:
		st.localCount++
		variable.slot = st.localCount
		st.vars[name] = variable
	case GLOBAL:
		st.globalCount++
		variable.slot = st.globalCount
		st.vars[name] = variable
	case HEAP:
		st.vars[name] = variable
	}

}

func Compile(ast *parser.Node) (*object.ObjFunction, error) {
	main := object.NewFunction(object.MAIN_FN_NAME, 0)
	return main, nil
}

func prePass(current *parser.Node) {

}

func generateByteCode() {

}
