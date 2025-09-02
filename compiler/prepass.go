package compiler

import (
	"go_js/object"
	"go_js/parser"
)

var forOfScope = false

func prePass(current *parser.Node, symbolTable *FunctionScope) {
	switch current.Type {
	case parser.NODE_PROGRAM:

		// hoist function declarations
		for _, node := range current.Body {
			if node.Type == parser.NODE_FUNCTION_DECLARATION {
				name := node.Identifier.Name
				arity := len(node.Params)

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

			for _, param := range current.Params {
				symbolTable.addVariable(param.Name, LET, nil)
			}

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
			BLOCK_SCOPES[current] = newBlockScope(symbolTable.block, forOfScope)
			symbolTable.enterBlockScope(current)
			for _, node := range current.Body {
				prePass(node, symbolTable)
			}
			symbolTable.exitBlockScope()
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
	case parser.NODE_IF_STATEMENT:
		prePass(current.Test, symbolTable)
		prePass(current.Consequent, symbolTable)
		prePass(current.Alternate, symbolTable)
	case parser.NODE_WHILE_STATEMENT:
		prePass(current.BodyNode, symbolTable)
		symbolTable.enterBlockScope(current.BodyNode)
		prePass(current.Test, symbolTable)
		symbolTable.exitBlockScope()
	case parser.NODE_FOR_STATEMENT:
		prePass(current.BodyNode, symbolTable)
		symbolTable.enterBlockScope(current.BodyNode)
		prePass(current.Initializer, symbolTable)
		prePass(current.Test, symbolTable)
		prePass(current.Update, symbolTable)
		symbolTable.exitBlockScope()
	case parser.NODE_FOR_OF_STATEMENT:
		forOfScope = true
		prePass(current.BodyNode, symbolTable)
		forOfScope = false
		symbolTable.enterBlockScope(current.BodyNode)
		prePass(current.Left, symbolTable)
		prePass(current.Right, symbolTable)
		symbolTable.exitBlockScope()

	}

}
