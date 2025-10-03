package compiler

import (
	"go_js/native"
	"go_js/object"
	"go_js/parser"
	"log"
	"os"
	"strings"
)

var catchScope = false
var classCompiler = false

func prePass(current *parser.Node, symbolTable *FunctionScope) {
	if current == nil {
		return
	}
	switch current.Type {
	case parser.NODE_PROGRAM:

		// hoist function declarations
		for _, node := range current.Body {
			if node.Type == parser.NODE_EXPORT_NAMED_DECLARATION {
				node = node.Declaration
			}
			if node.Type == parser.NODE_FUNCTION_DECLARATION {
				name := node.Identifier.Name
				arity := len(node.Params)

				var fn object.Callable
				if node.IsAsync {
					fn = native.NewAsyncFunction(name, arity, nil)
				} else if node.IsGenerator {
					fn = native.NewGenerator(name, arity, nil)
				} else {
					fn = object.NewFunction(name, arity, nil)
				}

				symbolTable.addVariable(name, FUNCTION, false, fn)
			}
		}

		for _, node := range current.Body {
			prePass(node, symbolTable)
		}
	case parser.NODE_BINARY_EXPRESSION:
		prePass(current.Left, symbolTable)
		prePass(current.Right, symbolTable)

	case parser.NODE_FUNCTION_DECLARATION:
		symbolTable := newFunctionScope(symbolTable, LOCAL)
		FUNCTION_SCOPES[current] = symbolTable

		// hoisting
		for _, node := range current.BodyNode.Body {
			if node.Type == parser.NODE_FUNCTION_DECLARATION {
				name := node.Identifier.Name
				arity := len(node.Params)

				var fn object.Callable
				if node.IsAsync {
					fn = native.NewAsyncFunction(name, arity, nil)
				} else if node.IsGenerator {
					fn = native.NewGenerator(name, arity, nil)
				} else {
					fn = object.NewFunction(name, arity, nil)
				}
				symbolTable.addVariable(name, FUNCTION, false, fn)
			}
		}

		for _, param := range current.Params {
			switch param.Type {
			case parser.NODE_IDENTIFIER:
				symbolTable.addVariable(param.Name, LET, false, nil)
			case parser.NODE_REST_ELEMENT:
				symbolTable.hasRestParameter = true
				switch param.Argument.Type {
				case parser.NODE_IDENTIFIER:
					symbolTable.addVariable(param.Argument.Name, LET, false, nil)
				default:
					panic("unsupported rest element argument")
				}
			}
		}

		symbolTable.arity = len(current.Params)

		for _, node := range current.BodyNode.Body {
			prePass(node, symbolTable)
		}

	case parser.NODE_ARROW_FUNCTION_EXPRESSION:
		symbolTable := newFunctionScope(symbolTable, LOCAL)
		FUNCTION_SCOPES[current] = symbolTable

		// hoist function declarations
		for _, node := range current.BodyNode.Body {
			if node.Type == parser.NODE_FUNCTION_DECLARATION {
				name := node.Identifier.Name
				arity := len(node.Arguments)

				var fn object.Callable
				if node.IsAsync {
					fn = native.NewAsyncFunction(name, arity, nil)
				} else {
					fn = object.NewFunction(name, arity, nil)
				}

				symbolTable.addVariable(name, FUNCTION, false, fn)
			}
		}
		for _, param := range current.Params {
			switch param.Type {
			case parser.NODE_IDENTIFIER:
				symbolTable.addVariable(param.Name, LET, false, nil)
			case parser.NODE_REST_ELEMENT:
				symbolTable.hasRestParameter = true
				switch param.Argument.Type {
				case parser.NODE_IDENTIFIER:
					symbolTable.addVariable(param.Argument.Name, LET, false, nil)
				default:
					panic("unsupported rest element argument")
				}
			}
		}
		symbolTable.arity = len(current.Params)

		if current.IsExpression {
			prePass(current.BodyNode, symbolTable)
		} else {
			for _, node := range current.BodyNode.Body {
				prePass(node, symbolTable)
			}
		}

	case parser.NODE_FUNCTION_EXPRESSION:
		{
			symbolTable := newFunctionScope(symbolTable, LOCAL)
			FUNCTION_SCOPES[current] = symbolTable

			// hoist function declarations
			for _, node := range current.BodyNode.Body {
				if node.Type == parser.NODE_FUNCTION_DECLARATION {
					name := node.Identifier.Name
					arity := len(node.Arguments)
					var fn object.Callable

					if node.IsAsync {
						fn = native.NewAsyncFunction(name, arity, nil)
					} else {
						fn = object.NewFunction(name, arity, nil)
					}

					symbolTable.addVariable(name, FUNCTION, false, fn)
				}
			}
			for _, param := range current.Params {
				switch param.Type {
				case parser.NODE_IDENTIFIER:
					symbolTable.addVariable(param.Name, LET, false, nil)
				case parser.NODE_REST_ELEMENT:
					symbolTable.hasRestParameter = true
					switch param.Argument.Type {
					case parser.NODE_IDENTIFIER:
						symbolTable.addVariable(param.Argument.Name, LET, false, nil)
					default:
						panic("unsupported rest element argument")
					}
				}
			}
			symbolTable.arity = len(current.Params)

			if current.IsExpression {
				prePass(current.BodyNode, symbolTable)
			} else {
				for _, node := range current.BodyNode.Body {
					prePass(node, symbolTable)
				}
			}
		}

	case parser.NODE_BLOCK_STATEMENT:
		{
			if _, found := BLOCK_SCOPES[current]; !found {
				BLOCK_SCOPES[current] = newBlockScope(symbolTable.block)
			}
			symbolTable.enterBlockScope(current)
			for _, node := range current.Body {
				prePass(node, symbolTable)
			}
			symbolTable.exitBlockScope()
		}

	case parser.NODE_VARIABLE_DECLARATION:
		for _, declaration := range current.Declarations {
			ident := declaration.Identifier
			switch ident.Type {
			case parser.NODE_ARRAY_PATTERN:
				for _, node := range ident.Elements {
					symbolTable.addVariable(node.Name, LET, false, nil)
				}
			case parser.NODE_OBJECT_PATTERN:
				for _, node := range ident.Properties {
					switch node.Type {
					case parser.NODE_PROPERTY:
						symbolTable.addVariable(node.Value.(*parser.Node).Name, LET, false, nil)
					case parser.NODE_REST_ELEMENT:
						symbolTable.addVariable(node.Argument.Name, LET, false, nil)
					}
				}
			default:
				for _, declaration := range current.Declarations {
					name := declaration.Identifier.Name
					symbolTable.addVariable(name, LET, false, nil)
					prePass(declaration.Initializer, symbolTable)
				}
			}
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
		if !symbolTable.needsArgumentsSlice && current.Name == RESERVED_ARGUMENTS {
			symbolTable.addArgumentsLocalToFunctionScope()
			return
		}

		variable, table := symbolTable.findVariable(current.Name)

		// maybe a combined flag? Or something not so ugly?
		if (catchScope || classCompiler) && variable == nil {
			symbolTable.addVariable(current.Name, LET, true, nil)
			return
		}

		if catchScope && variable.undeclared {
			variable.type_ = CATCH_PARAM
		}

		// check that we found anything and if it's from a upper function scope
		if table != nil && table != symbolTable && variable.scope == LOCAL {
			heapScopeVarCount := 0
			current := symbolTable

			for current != nil {
				for _, v := range current.vars {
					if v.scope == HEAP {
						heapScopeVarCount++
					} else if current == table && v.slot > variable.slot {
						v.slot--
					}
				}
				current = current.parent
			}

			variable.scope = HEAP
			variable.slot = heapScopeVarCount

		}
	case parser.NODE_IF_STATEMENT:
		prePass(current.Test, symbolTable)
		prePass(current.Consequent, symbolTable)
		if current.Alternate != nil {
			prePass(current.Alternate, symbolTable)
		}
	case parser.NODE_WHILE_STATEMENT:
		prePass(current.BodyNode, symbolTable)
		symbolTable.enterBlockScope(current.BodyNode)
		prePass(current.Test, symbolTable)
		symbolTable.exitBlockScope()
	case parser.NODE_FOR_STATEMENT:
		BLOCK_SCOPES[current.BodyNode] = newBlockScope(symbolTable.block)
		symbolTable.enterBlockScope(current.BodyNode)
		prePass(current.Initializer, symbolTable)
		prePass(current.BodyNode, symbolTable)
		symbolTable.enterBlockScope(current.BodyNode)
		prePass(current.Test, symbolTable)
		prePass(current.Update, symbolTable)
		symbolTable.exitBlockScope()
	case parser.NODE_FOR_OF_STATEMENT:
		BLOCK_SCOPES[current.BodyNode] = newBlockScope(symbolTable.block)
		prePass(current.BodyNode, symbolTable)
		symbolTable.enterBlockScope(current.BodyNode)
		prePass(current.Left, symbolTable)
		prePass(current.Right, symbolTable)
		symbolTable.exitBlockScope()
	case parser.NODE_FOR_IN_STATEMENT:
		prePass(current.BodyNode, symbolTable)
		symbolTable.enterBlockScope(current.BodyNode)
		prePass(current.Left, symbolTable)
		prePass(current.Right, symbolTable)
		symbolTable.exitBlockScope()
	case parser.NODE_EXPRESSION_STATEMENT:
		prePass(current.Expression, symbolTable)
	case parser.NODE_ASSIGNMENT_EXPRESSION:
		if variable, _ := symbolTable.findVariable(current.Left.Name); current.Left.Type != parser.NODE_MEMBER_EXPRESSION && variable == nil {
			symbolTable.addVariable(current.Left.Name, LET, true, nil)
		}
		prePass(current.Left, symbolTable)
		prePass(current.Right, symbolTable)
	case parser.NODE_CALL_EXPRESSION:
		prePass(current.Callee, symbolTable)
		for _, node := range current.Arguments {
			prePass(node, symbolTable)
		}
	case parser.NODE_TEMPLATE_LITERAL:
		for _, node := range current.Expressions {
			prePass(node, symbolTable)
		}
	case parser.NODE_MEMBER_EXPRESSION:
		{
			prePass(current.Object, symbolTable)
			for _, node := range current.Arguments {
				prePass(node, symbolTable)
			}
		}
	case parser.NODE_TRY_STATEMENT:
		{
			prePass(current.Block, symbolTable)
			prePass(current.Handler, symbolTable)
		}
	case parser.NODE_CATCH_CLAUSE:
		{
			prePass(current.BodyNode, symbolTable)
			symbolTable.enterBlockScope(current.BodyNode)
			catchScope = true
			prePass(current.Param, symbolTable)
			catchScope = false
			symbolTable.exitBlockScope()
		}
	case parser.NODE_THROW_STATEMENT:
		{
			prePass(current.Argument, symbolTable)
		}
	case parser.NODE_NEW_EXPRESSION:
		{
			prePass(current.Callee, symbolTable)
			for _, node := range current.Arguments {
				prePass(node, symbolTable)
			}
		}
	case parser.NODE_AWAIT_EXPRESSION:
		{
			prePass(current.Argument, symbolTable)
		}
	case parser.NODE_CLASS_DECLARATION:
		{
			classCompiler = true
			symbolTable.addVariable(current.Identifier.Name, CONST, false, nil)
			prePass(current.BodyNode, symbolTable)
			classCompiler = false
		}
	case parser.NODE_CLASS_BODY:
		{
			symbolTable := newFunctionScope(symbolTable, LOCAL)
			FUNCTION_SCOPES[current] = symbolTable

			for _, node := range current.Body {
				prePass(node, symbolTable)
			}
		}
	case parser.NODE_METHOD_DEFINITION:
		{
			symbolTable := newFunctionScope(symbolTable, LOCAL)
			FUNCTION_SCOPES[current] = symbolTable

			function := current.Value.(*parser.Node)

			for _, node := range function.Params {
				prePass(node, symbolTable)
			}

			for _, node := range function.BodyNode.Body {
				prePass(node, symbolTable)
			}
		}
	case parser.NODE_IMPORT_DECLARATION:
		{
			src := string(current.Source.Value.([]byte))

			prev := ROOT_SCRIPT_LOCATION
			if src[0] == '.' {
				src = ROOT_SCRIPT_LOCATION + src[1:]
				ROOT_SCRIPT_LOCATION = strings.Join(strings.Split(src, "/")[:len(strings.Split(src, "/"))-1], "/")
			}

			b, err := os.ReadFile(src)

			if err != nil {
				log.Fatalf("failed to read module source %s", err)
			}

			node, err := parser.GetAst(b, nil, 0)

			if err != nil {
				log.Fatalf("failed to create ast %s", err)
			}

			imports[src] = node
			prePass(node, global)
			ROOT_SCRIPT_LOCATION = prev

			for _, node := range current.Specifiers {
				prePass(node, symbolTable)
			}
		}
	case parser.NODE_IMPORT_SPECIFIER:
		prePass(current.Local, symbolTable)
	case parser.NODE_EXPORT_NAMED_DECLARATION:
		prePass(current.Declaration, symbolTable)
	case parser.NODE_REST_ELEMENT:

	}

}
