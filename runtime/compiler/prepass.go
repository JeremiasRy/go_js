package compiler

import (
	"go_js/native"
	"go_js/object"
	"go_js/parser"
	"log"
	"os"
	"strconv"
	"strings"
)

var catchScope = false
var classCompiler = false
var nextCatchId = 0
var strictContext = false
var CATCH_IDS = map[*parser.Node]int{}

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
			if node == nil {
				continue
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
					if objFn, ok := fn.(*object.ObjFunction); ok {
						objFn.Strict = module || nodeHasUseStrict(node)
					}
				}

				symbolTable.addVariable(name, FUNCTION, false, fn)
			}
		}

		for _, node := range current.Body {
			prePass(node, symbolTable)
		}
	case parser.NODE_ASSIGNMENT_PATTERN:
		prePass(current.Left, symbolTable)
		prePass(current.Right, symbolTable)
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
					if objFn, ok := fn.(*object.ObjFunction); ok {
						objFn.Strict = module || nodeHasUseStrict(node)
					}
				}
				symbolTable.addVariable(name, FUNCTION, false, fn)
			}
		}

		addParameterSlots(current.Params, symbolTable)
		symbolTable.arity = len(current.Params)
		markParamNames(current.Params, symbolTable)

		savedStrict := strictContext
		strictContext = strictContext || nodeHasUseStrict(current) || module
		for _, node := range current.BodyNode.Body {
			prePass(node, symbolTable)
		}
		if !strictContext {
			moveUndeclaredToGlobal(symbolTable)
		}
		strictContext = savedStrict

	case parser.NODE_ARROW_FUNCTION_EXPRESSION:
		symbolTable := newFunctionScope(symbolTable, LOCAL)
		symbolTable.isArrow = true
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
					if objFn, ok := fn.(*object.ObjFunction); ok {
						objFn.Strict = module || nodeHasUseStrict(node)
					}
				}

				symbolTable.addVariable(name, FUNCTION, false, fn)
			}
		}
		addParameterSlots(current.Params, symbolTable)
		symbolTable.arity = len(current.Params)
		markParamNames(current.Params, symbolTable)

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
						if objFn, ok := fn.(*object.ObjFunction); ok {
							objFn.Strict = module || nodeHasUseStrict(node)
						}
					}

					symbolTable.addVariable(name, FUNCTION, false, fn)
				}
			}
			addParameterSlots(current.Params, symbolTable)
			symbolTable.arity = len(current.Params)
			markParamNames(current.Params, symbolTable)

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
						if objFn, ok := fn.(*object.ObjFunction); ok {
							objFn.Strict = module || nodeHasUseStrict(node)
						}
					}
					symbolTable.addVariable(name, FUNCTION, false, fn)
				}
			}
			for _, node := range current.Body {
				prePass(node, symbolTable)
			}
			symbolTable.exitBlockScope()
		}

	case parser.NODE_VARIABLE_DECLARATION:
		kind := LET
		if current.Kind == parser.KIND_DECLARATION_CONST {
			kind = CONST
		} else if current.Kind == parser.KIND_DECLARATION_VAR {
			kind = VAR
		}
		for _, declaration := range current.Declarations {
			if kind == VAR {
				addVarPatternVariables(declaration.Identifier, symbolTable)
			} else {
				addPatternVariables(declaration.Identifier, kind, symbolTable)
			}
			prePass(declaration.Initializer, symbolTable)
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
		if current.Name == RESERVED_ARGUMENTS {
			target := symbolTable
			for target != nil && target.isArrow {
				target = target.parent
			}
			if target == nil {
				target = symbolTable
			}
			target.addArgumentsLocalToFunctionScope()
			if target != symbolTable {
				if variable, _ := symbolTable.findVariable(current.Name); variable != nil && variable.scope == LOCAL {
					oldSlot := variable.slot
					compactBlockSlots(target, oldSlot)
					variable.scope = HEAP
					variable.slot = allocateHeapSlot(target)
					target.hasHeapVars = true
				}
			}
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
			oldSlot := variable.slot
			compactBlockSlots(table, oldSlot)
			variable.scope = HEAP
			variable.slot = allocateHeapSlot(table)
			table.hasHeapVars = true
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
		symbolTable.enterBlockScope(current.BodyNode)
		prePass(current.Left, symbolTable)
		symbolTable.addVariable(ITERATOR_VAR_NAME, LET, false, nil)
		prePass(current.Right, symbolTable)
		for _, node := range current.BodyNode.Body {
			prePass(node, symbolTable)
		}
		symbolTable.exitBlockScope()
	case parser.NODE_FOR_IN_STATEMENT:
		BLOCK_SCOPES[current.BodyNode] = newBlockScope(symbolTable.block)
		symbolTable.enterBlockScope(current.BodyNode)
		prePass(current.Left, symbolTable)
		symbolTable.addVariable(ITERATOR_VAR_NAME, LET, false, nil)
		prePass(current.Right, symbolTable)
		for _, node := range current.BodyNode.Body {
			prePass(node, symbolTable)
		}
		symbolTable.exitBlockScope()
	case parser.NODE_EXPRESSION_STATEMENT:
		prePass(current.Expression, symbolTable)
	case parser.NODE_ASSIGNMENT_EXPRESSION:
		if current.Left.Type != parser.NODE_MEMBER_EXPRESSION {
			if variable, _ := symbolTable.findVariable(current.Left.Name); variable == nil {
				if strictContext {
					prePass(current.Right, symbolTable)
					return
				}
				symbolTable.addVariable(current.Left.Name, LET, true, nil)
			}
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
			prePass(current.Finalizer, symbolTable)
		}
	case parser.NODE_CATCH_CLAUSE:
		{
			if _, found := BLOCK_SCOPES[current.BodyNode]; !found {
				BLOCK_SCOPES[current.BodyNode] = newBlockScope(symbolTable.block)
			}
			symbolTable.enterBlockScope(current.BodyNode)
			catchScope = true
			if current.Param != nil && current.Param.Type == parser.NODE_IDENTIFIER {
				symbolTable.addVariable(current.Param.Name, CATCH_PARAM, false, nil)
				if variable, _ := symbolTable.findVariable(current.Param.Name); variable != nil {
					variable.catchId = nextCatchId
					CATCH_IDS[current.Param] = nextCatchId
					nextCatchId++
				}
			} else {
				prePass(current.Param, symbolTable)
			}
			for _, node := range current.BodyNode.Body {
				prePass(node, symbolTable)
			}
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
			savedStrict := strictContext
			strictContext = true
			symbolTable.addVariable(current.Identifier.Name, CONST, false, nil)
			prePass(current.SuperClass, symbolTable)
			prePass(current.BodyNode, symbolTable)
			strictContext = savedStrict
			classCompiler = false
		}
	case parser.NODE_CLASS_EXPRESSION:
		{
			classCompiler = true
			savedStrict := strictContext
			strictContext = true
			if current.Identifier != nil {
				syntheticName := "\x00class:" + current.Identifier.Name + ":" + strconv.Itoa(current.Id)
				global.addVariable(syntheticName, CONST, false, nil)
				classScope := newFunctionScope(symbolTable, LOCAL)
				classScope.vars[current.Identifier.Name] = global.vars[syntheticName]
				FUNCTION_SCOPES[current.BodyNode] = classScope
				prePass(current.SuperClass, symbolTable)
				prePass(current.BodyNode, symbolTable)
			} else {
				prePass(current.SuperClass, symbolTable)
				prePass(current.BodyNode, symbolTable)
			}
			strictContext = savedStrict
			classCompiler = false
		}
	case parser.NODE_CLASS_BODY:
		{
			classScope, found := FUNCTION_SCOPES[current]
			if !found {
				classScope = newFunctionScope(symbolTable, LOCAL)
				FUNCTION_SCOPES[current] = classScope
			}

			for _, node := range current.Body {
				prePass(node, classScope)
			}
		}
	case parser.NODE_METHOD_DEFINITION:
		{
			if current.Computed {
				prePass(current.Key, symbolTable)
			}
			symbolTable := newFunctionScope(symbolTable, LOCAL)
			FUNCTION_SCOPES[current] = symbolTable

			function := current.Value.(*parser.Node)

			addParameterSlots(function.Params, symbolTable)
			symbolTable.arity = len(function.Params)
			markParamNames(function.Params, symbolTable)

			for _, node := range function.Params {
				prePass(node, symbolTable)
			}

			for _, node := range function.BodyNode.Body {
				prePass(node, symbolTable)
			}
		}
	case parser.NODE_PROPERTY_DEFINITION:
		{
			if current.Computed {
				prePass(current.Key, symbolTable)
			}
			if initializer, ok := current.Value.(*parser.Node); ok {
				prePass(initializer, symbolTable)
			}
		}
	case parser.NODE_STATIC_BLOCK:
		{
			symbolTable := newFunctionScope(symbolTable, LOCAL)
			FUNCTION_SCOPES[current] = symbolTable
			for _, node := range current.Body {
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
		for _, specifier := range current.Specifiers {
			prePass(specifier, symbolTable)
		}
	case parser.NODE_SWITCH_STATEMENT:
		prePass(current.Discriminant, symbolTable)
		for _, node := range current.Cases {
			prePass(node, symbolTable)
		}
	case parser.NODE_SWITCH_CASE:
		{
			prePass(current.Test, symbolTable)
			prePass(current.Consequent, symbolTable)
			for _, node := range current.ConsequentSlice {
				prePass(node, symbolTable)
			}
		}
	case parser.NODE_DO_WHILE_STATEMENT:
		{
			BLOCK_SCOPES[current.BodyNode] = newBlockScope(symbolTable.block)
			prePass(current.BodyNode, symbolTable)
			symbolTable.enterBlockScope(current.BodyNode)
			prePass(current.Test, symbolTable)
			symbolTable.exitBlockScope()
		}
	case parser.NODE_LABELED_STATEMENT:
		prePass(current.BodyNode, symbolTable)
	case parser.NODE_ARRAY_EXPRESSION:
		for _, element := range current.Elements {
			prePass(element, symbolTable)
		}
	case parser.NODE_SEQUENCE_EXPRESSION:
		for _, expression := range current.Expressions {
			prePass(expression, symbolTable)
		}
	case parser.NODE_UNARY_EXPRESSION:
		prePass(current.Argument, symbolTable)
	case parser.NODE_UPDATE_EXPRESSION:
		prePass(current.Argument, symbolTable)
	case parser.NODE_CONDITIONAL_EXPRESSION:
		prePass(current.Test, symbolTable)
		prePass(current.Consequent, symbolTable)
		prePass(current.Alternate, symbolTable)
	case parser.NODE_YIELD_EXPRESSION:
		prePass(current.Argument, symbolTable)
	case parser.NODE_SPREAD_ELEMENT:
		prePass(current.Argument, symbolTable)
	case parser.NODE_REST_ELEMENT:
		prePass(current.Argument, symbolTable)
	}

}

func moveUndeclaredToGlobal(symbolTable *FunctionScope) {
	for name, variable := range symbolTable.vars {
		if variable.undeclared && variable.scope != GLOBAL {
			delete(symbolTable.vars, name)
			global.addVariable(name, LET, true, nil)
		}
	}
}

func addParameterSlots(params []*parser.Node, symbolTable *FunctionScope) {
	for i, param := range params {
		switch param.Type {
		case parser.NODE_IDENTIFIER:
			symbolTable.addVariable(param.Name, LET, false, nil)
		case parser.NODE_REST_ELEMENT:
			symbolTable.hasRestParameter = true
			if param.Argument.Type == parser.NODE_IDENTIFIER {
				symbolTable.addVariable(param.Argument.Name, LET, false, nil)
			} else {
				symbolTable.addVariable("__param"+strconv.Itoa(i), LET, false, nil)
			}
		case parser.NODE_ASSIGNMENT_PATTERN:
			if param.Left.Type == parser.NODE_IDENTIFIER {
				symbolTable.addVariable(param.Left.Name, LET, false, nil)
			} else {
				symbolTable.addVariable("__param"+strconv.Itoa(i), LET, false, nil)
			}
		default:
			symbolTable.addVariable("__param"+strconv.Itoa(i), LET, false, nil)
		}
	}
	for _, param := range params {
		pattern := param
		if pattern.Type == parser.NODE_ASSIGNMENT_PATTERN {
			pattern = pattern.Left
			prePass(param.Right, symbolTable)
		}
		switch pattern.Type {
		case parser.NODE_ARRAY_PATTERN, parser.NODE_OBJECT_PATTERN:
			addPatternVariables(pattern, LET, symbolTable)
		case parser.NODE_REST_ELEMENT:
			if pattern.Argument.Type != parser.NODE_IDENTIFIER {
				addPatternVariables(pattern.Argument, LET, symbolTable)
			}
		}
	}
}

func markParamNames(params []*parser.Node, symbolTable *FunctionScope) {
	var collect func(*parser.Node)
	collect = func(node *parser.Node) {
		if node == nil {
			return
		}
		switch node.Type {
		case parser.NODE_IDENTIFIER:
			symbolTable.paramNames[node.Name] = true
		case parser.NODE_ASSIGNMENT_PATTERN:
			collect(node.Left)
		case parser.NODE_REST_ELEMENT:
			collect(node.Argument)
		case parser.NODE_ARRAY_PATTERN:
			for _, element := range node.Elements {
				collect(element)
			}
		case parser.NODE_OBJECT_PATTERN:
			for _, property := range node.Properties {
				if property.Type == parser.NODE_PROPERTY {
					if value, ok := property.Value.(*parser.Node); ok {
						collect(value)
					}
				} else if property.Type == parser.NODE_REST_ELEMENT {
					collect(property.Argument)
				}
			}
		}
	}
	for _, param := range params {
		collect(param)
	}
}

func addVarPatternVariables(node *parser.Node, symbolTable *FunctionScope) {
	if node == nil {
		return
	}
	switch node.Type {
	case parser.NODE_IDENTIFIER:
		if _, found := symbolTable.vars[node.Name]; found {
			return
		}
		slot := 0
		if symbolTable.tableScope == LOCAL {
			slot = functionSlotCount(symbolTable)
		}
		variable := &Variable{scope: symbolTable.tableScope, type_: VAR, slot: slot, owner: symbolTable}
		symbolTable.vars[node.Name] = variable
		symbolTable.varOrder = append(symbolTable.varOrder, variable)
	case parser.NODE_ASSIGNMENT_PATTERN:
		addVarPatternVariables(node.Left, symbolTable)
	case parser.NODE_REST_ELEMENT:
		addVarPatternVariables(node.Argument, symbolTable)
	case parser.NODE_ARRAY_PATTERN:
		for _, element := range node.Elements {
			addVarPatternVariables(element, symbolTable)
		}
	case parser.NODE_OBJECT_PATTERN:
		for _, property := range node.Properties {
			switch property.Type {
			case parser.NODE_PROPERTY:
				if value, ok := property.Value.(*parser.Node); ok {
					addVarPatternVariables(value, symbolTable)
				}
			case parser.NODE_REST_ELEMENT:
				addVarPatternVariables(property.Argument, symbolTable)
			}
		}
	}
}

func addPatternVariables(node *parser.Node, kind VariableType, symbolTable *FunctionScope) {
	if node == nil {
		return
	}
	switch node.Type {
	case parser.NODE_IDENTIFIER:
		var vars Variables
		if symbolTable.block != nil {
			vars = symbolTable.block.vars
		} else {
			vars = symbolTable.vars
		}
		if variable, ok := vars[node.Name]; ok {
			variable.undeclared = false
			return
		}
		symbolTable.addVariable(node.Name, kind, false, nil)
	case parser.NODE_ASSIGNMENT_PATTERN:
		addPatternVariables(node.Left, kind, symbolTable)
	case parser.NODE_REST_ELEMENT:
		addPatternVariables(node.Argument, kind, symbolTable)
	case parser.NODE_ARRAY_PATTERN:
		for _, element := range node.Elements {
			addPatternVariables(element, kind, symbolTable)
		}
	case parser.NODE_OBJECT_PATTERN:
		for _, property := range node.Properties {
			switch property.Type {
			case parser.NODE_PROPERTY:
				if value, ok := property.Value.(*parser.Node); ok {
					addPatternVariables(value, kind, symbolTable)
				}
			case parser.NODE_REST_ELEMENT:
				addPatternVariables(property.Argument, kind, symbolTable)
			}
		}
	}
}
