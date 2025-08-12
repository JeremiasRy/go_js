package compiler

import (
	"fmt"
	"go_js/object"
	"go_js/parser"
)

type VariableType int

const (
	CONST VariableType = iota
	LET
	FUNCTION
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
	fn    *object.ObjFunction
}

type Variables map[string]*Variable

type SymbolTable struct {
	parent      *SymbolTable
	next        *SymbolTable
	vars        Variables
	globalCount int
	localCount  int
}

var symbolTable *SymbolTable = newSymbolTable(nil)

func newSymbolTable(parent *SymbolTable) *SymbolTable {
	new := &SymbolTable{parent: parent, next: nil, vars: map[string]*Variable{}, globalCount: -1, localCount: -1}
	parent.next = new

	return new
}

func (st *SymbolTable) addVariable(name string, scope VariableScope, type_ VariableType, fn *object.ObjFunction) {
	if _, found := st.vars[name]; found {
		fmt.Printf("WARN: Already found variable %s from symbol table\n", name)
		return
	}
	variable := &Variable{scope: scope, type_: type_, slot: -1, fn: fn}

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
	prePass(ast, main, symbolTable)
	return main, nil
}

func prePass(current *parser.Node, fn *object.ObjFunction, sTable *SymbolTable) {
	switch current.Type {
	case parser.NODE_PROGRAM:

		// hoist function declarations
		for _, node := range current.Body {
			if node.Type == parser.NODE_FUNCTION_DECLARATION {
				name := node.Identifier.Name
				arity := len(node.Arguments)

				symbolTable.addVariable(name, GLOBAL, FUNCTION, object.NewFunction(name, arity))
			}
		}

		for _, node := range current.Body {
			prePass(node, fn, sTable)
		}

	case parser.NODE_FUNCTION_DECLARATION:
		if fnVar, found := symbolTable.vars[current.Identifier.Name]; found {
			if fnVar.type_ != FUNCTION {
				panic("should be a function we are declaring")
			}
			prePass(current.BodyNode, fnVar.fn, sTable)
		} else {
			panic("extreme failure to hoist function declaration")
		}

	case parser.NODE_BLOCK_STATEMENT:
		{
			sTable := newSymbolTable(sTable)

			// hoist function declarations
			for _, node := range current.Body {
				if node.Type == parser.NODE_FUNCTION_DECLARATION {
					name := node.Identifier.Name
					arity := len(node.Arguments)

					symbolTable.addVariable(name, GLOBAL, FUNCTION, object.NewFunction(name, arity))
				}
			}

			for _, node := range current.Body {
				prePass(node, fn, sTable)
			}
		}
	}
}

func generateByteCode() {

}
