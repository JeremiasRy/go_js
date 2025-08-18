package compiler

import (
	"fmt"
	"go_js/chunk"
	"go_js/heap"
	"go_js/object"
	"go_js/parser"
)

type VariableType uint8

const (
	CONST VariableType = iota
	LET
	FUNCTION
)

type VariableScope uint8

const (
	HEAP VariableScope = iota
	LOCAL
	GLOBAL
)

type TableScope uint8

const (
	FN TableScope = iota
	BLOCK
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
	tableScope  TableScope
	vars        Variables
	globalCount int
	localCount  int
}

func newSymbolTable(parent *SymbolTable, tableScope TableScope) *SymbolTable {
	new := &SymbolTable{parent: parent, next: nil, tableScope: tableScope, vars: map[string]*Variable{}, globalCount: -1, localCount: -1}
	if parent != nil {
		parent.next = new
	}

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

func (st *SymbolTable) findVariable(name string) (*Variable, *SymbolTable) {
	current := st

	for current != nil {
		if variable, found := current.vars[name]; found {
			return variable, current
		}

		current = current.parent
	}

	return nil, nil
}

func Compile(ast *parser.Node) (*object.ObjFunction, error) {
	main := object.NewFunction(object.MAIN_FN_NAME, 0)
	var symbolTable *SymbolTable = newSymbolTable(nil, FN)
	prePass(ast, symbolTable, FN)
	generateByteCode(ast, symbolTable, main)
	main.ValueChunk().EmitByte(chunk.OP_EOF)

	return main, nil
}

func prePass(current *parser.Node, sTable *SymbolTable, tableScope TableScope) {
	switch current.Type {
	case parser.NODE_PROGRAM:

		// hoist function declarations
		for _, node := range current.Body {
			if node.Type == parser.NODE_FUNCTION_DECLARATION {
				name := node.Identifier.Name
				arity := len(node.Arguments)

				sTable.addVariable(name, GLOBAL, FUNCTION, object.NewFunction(name, arity))
			}
		}

		for _, node := range current.Body {
			prePass(node, sTable, tableScope)
		}

	case parser.NODE_FUNCTION_DECLARATION:
		if fnVar, found := sTable.vars[current.Identifier.Name]; found {
			if fnVar.type_ != FUNCTION {
				panic("should be a function we are declaring")
			}
			sTable := newSymbolTable(sTable, tableScope)
			prePass(current.BodyNode, sTable, FN)
		} else {
			panic("extreme failure to hoist function declaration")
		}

	case parser.NODE_ARROW_FUNCTION_EXPRESSION:
		sTable := newSymbolTable(sTable, tableScope)
		prePass(current.BodyNode, sTable, FN)

	case parser.NODE_BLOCK_STATEMENT:
		{
			// hoist function declarations
			for _, node := range current.Body {
				if node.Type == parser.NODE_FUNCTION_DECLARATION {
					name := node.Identifier.Name
					arity := len(node.Arguments)

					sTable.addVariable(name, LOCAL, FUNCTION, object.NewFunction(name, arity))
				}
			}

			for _, node := range current.Body {
				prePass(node, sTable, tableScope)
			}
		}

	case parser.NODE_VARIABLE_DECLARATION:
		var kind VariableType

		switch current.Kind {
		case parser.KIND_DECLARATION_CONST:
			kind = CONST
		case parser.KIND_DECLARATION_LET:
			kind = LET
		}

		for _, declaration := range current.Declarations {
			name := declaration.Identifier.Name
			sTable.addVariable(name, LOCAL, kind, nil)
		}

	case parser.NODE_RETURN_STATEMENT:
		prePass(current.Argument, sTable, tableScope)
	case parser.NODE_OBJECT_EXPRESSION:
		for _, node := range current.Properties {
			prePass(node, sTable, tableScope)
		}
	case parser.NODE_PROPERTY:
		prePass(current.Value.(*parser.Node), sTable, tableScope)
	case parser.NODE_IDENTIFIER:
		variable, table := sTable.findVariable(current.Name)

		// check that we found anything and if it's from a upper function scope
		if table != nil && variable != nil && table != sTable && variable.scope != HEAP {
			table.localCount--
			variable.scope = HEAP
		}
	}
}

func generateByteCode(current *parser.Node, symbolTable *SymbolTable, fn *object.ObjFunction) {
	switch current.Type {
	case parser.NODE_PROGRAM:
		{
			for _, variable := range symbolTable.vars {
				if variable.type_ != FUNCTION {
					continue
				}

				fnValue := heap.Allocate(variable.fn)
				slot := fn.ValueChunk().WriteConstant(fnValue)

				if uint8(variable.slot) != slot {
					panic("things went south")
				}
				fn.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)

			}
			for _, node := range current.Body {
				generateByteCode(node, symbolTable, fn)
			}
		}
	}

}
