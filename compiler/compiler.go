package compiler

import (
	"cmp"
	"fmt"
	"go_js/chunk"
	"go_js/heap"
	"go_js/object"
	"go_js/parser"
	"go_js/value"
	"slices"
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

type Variable struct {
	scope VariableScope
	type_ VariableType
	slot  int // for locals and globals
	fn    *object.ObjFunction
}

type Variables map[string]*Variable

// NEXT TASK: we need next table for code generation
type SymbolTable struct {
	prev         *SymbolTable
	next         *SymbolTable
	tableScope   VariableScope
	vars         Variables
	varCount     int
	currentBlock int
	blocks       []Variables
}

func newSymbolTable(parent *SymbolTable, tableScope VariableScope) *SymbolTable {
	new := &SymbolTable{prev: parent, tableScope: tableScope, vars: Variables{}, varCount: -1, currentBlock: -1, blocks: []Variables{}}

	if parent != nil {

		parent.next = new
	}

	return new
}

func (st *SymbolTable) addVariable(name string, type_ VariableType, fn *object.ObjFunction) {
	var mapToAddTo Variables

	if st.currentBlock >= 0 {
		mapToAddTo = st.blocks[st.currentBlock]
	} else {
		mapToAddTo = st.vars
	}
	if _, found := mapToAddTo[name]; found {
		fmt.Printf("WARN: Already found variable %s from scope\n", name)
		return
	}
	variable := &Variable{scope: st.tableScope, type_: type_, slot: -1, fn: fn}

	st.varCount++
	variable.slot = st.varCount
	mapToAddTo[name] = variable
}

func (st *SymbolTable) findVariable(name string) (*Variable, *SymbolTable) {
	current := st

	for current != nil {
		if current.currentBlock >= 0 {
			blockIndex := st.currentBlock

			for blockIndex >= 0 {
				if variable, found := current.blocks[blockIndex][name]; found {
					return variable, current
				}
				blockIndex--
			}
		}
		if variable, found := current.vars[name]; found {
			return variable, current
		}

		current = current.prev
	}

	return nil, nil
}

func (st *SymbolTable) newBlock() {
	st.currentBlock++
	st.blocks = append(st.blocks, Variables{})
}

func (st *SymbolTable) enterBlock() {
	st.currentBlock++
}

func (st *SymbolTable) exitBlock() {
	st.currentBlock--
}

func Compile(ast *parser.Node) (*object.ObjFunction, error) {
	main := object.NewFunction(object.MAIN_FN_NAME, 0)
	var symbolTable *SymbolTable = newSymbolTable(nil, GLOBAL)

	prePass(ast, symbolTable)
	generateByteCode(ast, symbolTable, main)
	main.ValueChunk().EmitByte(chunk.OP_EOF)

	return main, nil
}

func prePass(current *parser.Node, symbolTable *SymbolTable) {
	switch current.Type {
	case parser.NODE_PROGRAM:

		// hoist function declarations
		for _, node := range current.Body {
			if node.Type == parser.NODE_FUNCTION_DECLARATION {
				name := node.Identifier.Name
				arity := len(node.Arguments)

				symbolTable.addVariable(name, FUNCTION, object.NewFunction(name, arity))
			}
		}

		for _, node := range current.Body {
			prePass(node, symbolTable)
		}

	case parser.NODE_FUNCTION_DECLARATION:
		if _, found := symbolTable.vars[current.Identifier.Name]; found {

			symbolTable := newSymbolTable(symbolTable, LOCAL)
			for _, node := range current.BodyNode.Body {
				prePass(node, symbolTable)
			}
		} else {
			panic("extreme failure to hoist function declaration")
		}

	case parser.NODE_ARROW_FUNCTION_EXPRESSION:
		sTable := newSymbolTable(symbolTable, LOCAL)

		// hoist function declarations
		for _, node := range current.BodyNode.Body {
			if node.Type == parser.NODE_FUNCTION_DECLARATION {
				name := node.Identifier.Name
				arity := len(node.Arguments)

				sTable.addVariable(name, FUNCTION, object.NewFunction(name, arity))
			}
		}

		for _, node := range current.BodyNode.Body {
			prePass(node, sTable)
		}

	case parser.NODE_BLOCK_STATEMENT:
		{
			symbolTable.newBlock()
			for _, node := range current.Body {
				prePass(node, symbolTable)
			}
			symbolTable.exitBlock()
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
			symbolTable.addVariable(name, kind, nil)
		}

	case parser.NODE_RETURN_STATEMENT:
		prePass(current.Argument, symbolTable)
	case parser.NODE_OBJECT_EXPRESSION:
		for _, node := range current.Properties {
			prePass(node, symbolTable)
		}
	case parser.NODE_PROPERTY:
		prePass(current.Value.(*parser.Node), symbolTable)
	case parser.NODE_IDENTIFIER:
		variable, table := symbolTable.findVariable(current.Name)

		// check that we found anything and if it's from a upper function scope
		if table != nil && variable != nil && table != symbolTable && variable.scope != HEAP {
			table.varCount--
			variable.scope = HEAP
		}
	}
}

func generateByteCode(current *parser.Node, symbolTable *SymbolTable, fn *object.ObjFunction) {
	isMain := fn.Name() == object.MAIN_FN_NAME
	switch current.Type {
	case parser.NODE_PROGRAM:
		{
			functions := []*Variable{}
			for _, variable := range symbolTable.vars {
				if variable.type_ == FUNCTION {
					functions = append(functions, variable)
				}
			}

			slices.SortFunc(functions, func(a *Variable, b *Variable) int {
				return cmp.Compare(a.slot, b.slot)
			})

			for _, variable := range functions {
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
	case parser.NODE_FUNCTION_DECLARATION:
		{
			nextFn, _ := symbolTable.findVariable(current.Identifier.Name)
			for _, node := range current.BodyNode.Body {
				symbolTable = symbolTable.next
				generateByteCode(node, symbolTable, nextFn.fn)
			}
		}
	case parser.NODE_BLOCK_STATEMENT:
		{
			symbolTable.enterBlock()
			for _, node := range current.Body {
				generateByteCode(node, symbolTable, fn)
			}
			symbolTable.exitBlock()
		}
	case parser.NODE_VARIABLE_DECLARATION:
		{
			for _, declaration := range current.Declarations {
				generateByteCode(declaration, symbolTable, fn)
			}
		}
	case parser.NODE_VARIABLE_DECLARATOR:
		{
			name := current.Identifier.Name
			variable, _ := symbolTable.findVariable(name)
			if variable != nil {
				generateByteCode(current.Initializer, symbolTable, fn)

				// check that our slots match
				if variable.slot != int(fn.ValueChunk().Code[len(fn.ValueChunk().Code)-1]) {
					panic("failure in pre pass")
				}

				if isMain {
					fn.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
				} else {
					fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)

				}

			}
		}
	case parser.NODE_LITERAL:
		{
			switch v := current.Value.(type) {
			case float64:
				{
					fn.ValueChunk().WriteConstant(value.ValueFromFloat64(v))
				}
			}
		}
	}

}
