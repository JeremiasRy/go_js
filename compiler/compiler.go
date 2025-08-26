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
type BlockScope struct {
	parent *BlockScope
	vars   Variables
}

// NEXT TASK: we need next table for code generation
type FunctionScope struct {
	parent     *FunctionScope
	tableScope VariableScope
	vars       Variables
	varCount   int

	block *BlockScope
}

var BLOCK_SCOPES = map[*parser.Node]*BlockScope{}
var FUNCTION_SCOPES = map[*parser.Node]*FunctionScope{}

func newBlockScope(parent *BlockScope) *BlockScope {
	return &BlockScope{vars: Variables{}, parent: parent}
}

func newFunctionScope(parent *FunctionScope, tableScope VariableScope) *FunctionScope {
	return &FunctionScope{parent: parent, tableScope: tableScope, vars: Variables{}, varCount: -1, block: nil}
}

func (fs *FunctionScope) enterBlockScope(node *parser.Node) {
	if b, found := BLOCK_SCOPES[node]; found {
		fs.block = b
	} else {
		panic("no block scope found for ast node")
	}
}

func (fs *FunctionScope) exitScope() {
	fs.block = fs.block.parent
}

func (fs *FunctionScope) addVariable(name string, type_ VariableType, fn *object.ObjFunction) {
	var mapToAddTo Variables

	if fs.block != nil {
		mapToAddTo = fs.block.vars
	} else {
		mapToAddTo = fs.vars
	}

	if _, found := mapToAddTo[name]; found {
		fmt.Printf("WARN: Already found variable %s from scope\n", name)
		return
	}
	variable := &Variable{scope: fs.tableScope, type_: type_, slot: -1, fn: fn}

	fs.varCount++
	variable.slot = fs.varCount
	mapToAddTo[name] = variable
}

func (fs *FunctionScope) findVariable(name string) (*Variable, *FunctionScope) {
	current := fs

	// check that are we in a block statement
	block := fs.block

	for block != nil {
		if variable, found := block.vars[name]; found {
			return variable, current
		}
		block = block.parent
	}

	for current != nil {
		if variable, found := current.vars[name]; found {
			return variable, current
		}

		current = current.parent
	}

	return nil, nil
}

func defineConsole(main *object.ObjFunction, symbolTable *FunctionScope) {
	console := object.NewObjectHash()
	global := heap.Allocate(console)
	log := heap.Allocate(object.NewLog())
	console.SetMember("log", value.EncodeObject(log))
	symbolTable.addVariable("console", CONST, nil)

	main.ValueChunk().WriteConstant(value.EncodeObject(global))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func Compile(ast *parser.Node) (*object.ObjFunction, error) {
	main := object.NewFunction(object.MAIN_FN_NAME, 0)
	var symbolTable *FunctionScope = newFunctionScope(nil, GLOBAL)
	defineConsole(main, symbolTable)
	prePass(ast, symbolTable)

	generateByteCode(ast, symbolTable, main)

	main.ValueChunk().EmitByte(chunk.OP_EOF)
	return main, nil
}

func prePass(current *parser.Node, symbolTable *FunctionScope) {
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

			symbolTable := newFunctionScope(symbolTable, LOCAL)
			FUNCTION_SCOPES[current] = symbolTable
			for _, node := range current.BodyNode.Body {
				prePass(node, symbolTable)
			}
		} else {
			panic("extreme failure to hoist function declaration")
		}

	case parser.NODE_ARROW_FUNCTION_EXPRESSION:
		symbolTable := newFunctionScope(symbolTable, LOCAL)
		FUNCTION_SCOPES[current] = symbolTable

		// hoist function declarations
		for _, node := range current.BodyNode.Body {
			if node.Type == parser.NODE_FUNCTION_DECLARATION {
				name := node.Identifier.Name
				arity := len(node.Arguments)

				symbolTable.addVariable(name, FUNCTION, object.NewFunction(name, arity))
			}
		}

		for _, node := range current.BodyNode.Body {
			prePass(node, symbolTable)
		}

	case parser.NODE_BLOCK_STATEMENT:
		{
			BLOCK_SCOPES[current] = newBlockScope(symbolTable.block)
			symbolTable.enterBlockScope(current)
			for _, node := range current.Body {
				prePass(node, symbolTable)
			}
			symbolTable.exitScope()
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

func generateByteCode(current *parser.Node, symbolTable *FunctionScope, fn *object.ObjFunction) {
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

				fn.ValueChunk().WriteConstant(value.EncodeObject(fnValue))
				fn.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
			}
			for _, node := range current.Body {
				generateByteCode(node, symbolTable, fn)
			}
		}
	case parser.NODE_FUNCTION_DECLARATION:
		{
			nextFn, _ := symbolTable.findVariable(current.Identifier.Name)
			symbolTable = FUNCTION_SCOPES[current]

			fn = nextFn.fn

			// hoisting...
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
				slot := fn.ValueChunk().WriteConstant(value.EncodeObject(fnValue))

				if uint8(variable.slot) != slot {
					panic("things went south")
				}

				fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)
			}

			for _, node := range current.BodyNode.Body {
				generateByteCode(node, symbolTable, fn)
			}
			fn.ValueChunk().EmitBytes(chunk.OP_PUSH_UNDEFINED, chunk.OP_RETURN)
		}
	case parser.NODE_BLOCK_STATEMENT:
		{
			symbolTable.enterBlockScope(current)
			for _, node := range current.Body {
				generateByteCode(node, symbolTable, fn)
			}
			symbolTable.exitScope()
		}
	case parser.NODE_VARIABLE_DECLARATION:
		{
			for _, declaration := range current.Declarations {
				generateByteCode(declaration, symbolTable, fn)
			}
		}
	case parser.NODE_CALL_EXPRESSION:
		{
			for _, node := range current.Arguments {
				generateByteCode(node, symbolTable, fn)
			}
			generateByteCode(current.Callee, symbolTable, fn)

			fn.ValueChunk().EmitByte(chunk.OP_CALL)
		}
	case parser.NODE_MEMBER_EXPRESSION:
		{
			variable, _ := symbolTable.findVariable(current.Object.Name)
			member := heap.AllocateString(object.ObjString(current.Property.Name))
			memberSlot := fn.ValueChunk().AddConstant(value.EncodeObject(member))

			var op uint8

			switch variable.scope {
			case LOCAL:
				op = chunk.OP_GET_LOCAL_OBJECT_MEMBER
			case GLOBAL:
				op = chunk.OP_GET_GLOBAL_OBJECT_MEMBER
			}

			fn.ValueChunk().EmitBytes(op, uint8(variable.slot), memberSlot)
		}
	case parser.NODE_VARIABLE_DECLARATOR:
		{
			name := current.Identifier.Name
			variable, _ := symbolTable.findVariable(name)
			if variable != nil {
				generateByteCode(current.Initializer, symbolTable, fn)

				switch variable.scope {
				case GLOBAL:
					fn.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
				case LOCAL:
					fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)
				case HEAP:
					//
				}

			}
		}
	case parser.NODE_EXPRESSION_STATEMENT:
		{
			generateByteCode(current.Expression, symbolTable, fn)
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
	case parser.NODE_IDENTIFIER:
		{
			variable, _ := symbolTable.findVariable(current.Name)
			if variable == nil {
				println("didnt find ", current.Name)
				fn.ValueChunk().EmitByte(chunk.OP_PUSH_UNDEFINED)
				return
			}

			switch variable.scope {
			case GLOBAL:
				fn.ValueChunk().EmitBytes(chunk.OP_GET_GLOBAL, uint8(variable.slot))
			case LOCAL:
				fn.ValueChunk().EmitBytes(chunk.OP_GET_LOCAL, uint8(variable.slot))
			case HEAP:
				//
			}

		}
	}

}
