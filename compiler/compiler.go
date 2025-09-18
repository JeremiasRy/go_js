package compiler

import (
	"cmp"
	"fmt"
	"go_js/allocator"
	"go_js/chunk"
	"go_js/constructor"
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
	FOR
	CATCH_PARAM
)

type VariableScope uint8

const (
	HEAP VariableScope = iota
	LOCAL
	GLOBAL
)

type Variable struct {
	scope      VariableScope
	type_      VariableType
	slot       int  // for locals and globals
	init       bool // used in for of loops
	undeclared bool // used for undeclared variables i.e in assignments to unknown variable {e = 2}
	fn         *object.ObjFunction
}

type Variables map[string]*Variable
type BlockScope struct {
	parent *BlockScope
	vars   Variables
	forOf  bool
}

type FunctionScope struct {
	parent     *FunctionScope
	tableScope VariableScope
	vars       Variables
	block      *BlockScope
}

var BLOCK_SCOPES = map[*parser.Node]*BlockScope{}
var FUNCTION_SCOPES = map[*parser.Node]*FunctionScope{}

func newBlockScope(parent *BlockScope) *BlockScope {
	return &BlockScope{vars: Variables{}, parent: parent}
}

func newFunctionScope(parent *FunctionScope, tableScope VariableScope) *FunctionScope {
	return &FunctionScope{parent: parent, tableScope: tableScope, vars: Variables{}, block: nil}
}

func (fs *FunctionScope) isInHeapScope() bool {
	current := fs.parent

	for current != nil {
		for _, v := range current.vars {
			if v.scope == HEAP {
				return true
			}
		}
		current = current.parent
	}
	return false
}

func (fs *FunctionScope) enterBlockScope(node *parser.Node) {
	if b, found := BLOCK_SCOPES[node]; found {
		fs.block = b
	} else {
		panic("no block scope found for ast node")
	}
}

func (fs *FunctionScope) exitBlockScope() {
	fs.block = fs.block.parent
}

func (fs *FunctionScope) getCurrentSlot() int {
	if fs.tableScope == GLOBAL && fs.block == nil {
		return len(fs.vars) - 1
	}

	count := -1
	if fs.block != nil {
		current := fs.block

		for current != nil {
			count += len(current.vars)
			current = current.parent
		}
	}

	if fs.tableScope == GLOBAL {
		return count
	}

	count += len(fs.vars)
	return count
}

func (fs *FunctionScope) addVariable(name string, type_ VariableType, undeclared bool, fn *object.ObjFunction) {
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

	variable := &Variable{scope: fs.tableScope, type_: type_, slot: -1, fn: fn, undeclared: undeclared}
	mapToAddTo[name] = variable

	if fs.block != nil {
		variable.scope = LOCAL
	}
	variable.slot = fs.getCurrentSlot()
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

func (fs *FunctionScope) currentBlockVarCount() (int, bool) {
	count := -1
	if fs.block != nil {
		count = len(fs.block.vars)
		return count, true
	}
	return count, false
}

func defineConsole(main *object.ObjFunction, symbolTable *FunctionScope) {
	console := object.NewObjectHash()
	consoleHandle := allocator.Allocate(console)
	logHandle := allocator.Allocate(object.NewLog())
	console.SetMember("log", value.EncodeHandle(logHandle))
	symbolTable.addVariable("console", CONST, false, nil)

	main.ValueChunk().WriteConstant(value.EncodeHandle(consoleHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineObjectConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	objCtor := &constructor.ObjectConstructor{}
	objCtorHandle := allocator.Allocate(objCtor)

	symbolTable.addVariable("Object", CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(objCtorHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineArrayConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	arrCtor := &constructor.ArrayConstructor{}
	arrCtorHandle := allocator.Allocate(arrCtor)

	symbolTable.addVariable("Object", CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(arrCtorHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineErrorConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	ctor := &constructor.ErrorConstructor{}
	ctorHandle := allocator.Allocate(ctor)

	symbolTable.addVariable("Error", CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(ctorHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineSetTimeout(main *object.ObjFunction, symbolTable *FunctionScope) {
	setTimeout := object.NewSetTimeout()
	setTimeouthandle := allocator.Allocate(setTimeout)

	symbolTable.addVariable("setTimeout", CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(setTimeouthandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func Compile(ast *parser.Node) (*object.ObjFunction, error) {
	main := object.NewFunction(object.MAIN_FN_NAME, 0, nil)
	var symbolTable *FunctionScope = newFunctionScope(nil, GLOBAL)

	defineConsole(main, symbolTable)
	defineErrorConstructor(main, symbolTable)
	defineSetTimeout(main, symbolTable)
	defineArrayConstructor(main, symbolTable)

	prePass(ast, symbolTable)
	generateByteCode(ast, symbolTable, main)

	main.ValueChunk().EmitByte(chunk.OP_RETURN)
	return main, nil
}

var popStack = true

func generateByteCode(current *parser.Node, symbolTable *FunctionScope, fn object.Callable) {
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
				fnValue := allocator.Allocate(variable.fn)

				fn.ValueChunk().WriteConstant(value.EncodeHandle(fnValue))
				fn.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
			}

			for _, node := range current.Body {
				generateByteCode(node, symbolTable, fn)
			}
		}
	case parser.NODE_ASSIGNMENT_EXPRESSION:
		{
			variable, _ := symbolTable.findVariable(current.Left.Name)
			generateByteCode(current.Right, symbolTable, fn)
			var defineOp uint8
			var setOp uint8

			switch variable.scope {
			case LOCAL:
				{
					defineOp = chunk.OP_DEFINE_LOCAL
					setOp = chunk.OP_SET_LOCAL
				}
			case GLOBAL:
				{
					defineOp = chunk.OP_DEFINE_GLOBAL
					setOp = chunk.OP_SET_GLOBAL
				}
			case HEAP:
				{
					defineOp = chunk.OP_DEFINE_HEAP_VAR
					setOp = chunk.OP_SET_HEAP_VAR
				}
			}

			if variable.undeclared {
				fn.ValueChunk().EmitByte(defineOp)
				variable.undeclared = false
			} else {
				fn.ValueChunk().EmitBytes(setOp, uint8(variable.slot))
			}
		}
	case parser.NODE_FUNCTION_DECLARATION:
		{
			nextFn, _ := symbolTable.findVariable(current.Identifier.Name)
			symbolTable = FUNCTION_SCOPES[current]

			fn = nextFn.fn
			isInHeapScopeAlready := symbolTable.isInHeapScope()

			functions := []*Variable{}
			for _, variable := range symbolTable.vars {
				if variable.type_ == FUNCTION {
					functions = append(functions, variable)
				}

				// if not in heap scope we'll create a new one
				if !isInHeapScopeAlready && variable.scope == HEAP {
					fn.ValueChunk().EmitByte(chunk.OP_CREATE_HEAP_SCOPE)
					isInHeapScopeAlready = true
				}
			}

			slices.SortFunc(functions, func(a *Variable, b *Variable) int {
				return cmp.Compare(a.slot, b.slot)
			})

			for _, variable := range functions {
				fnValue := allocator.Allocate(variable.fn)
				slot := fn.ValueChunk().WriteConstant(value.EncodeHandle(fnValue))

				if uint8(variable.slot) != slot {
					panic("things went south")
				}

				fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)
			}

			for _, node := range current.BodyNode.Body {
				generateByteCode(node, symbolTable, fn)
			}

			if fn.ValueChunk().Code[len(fn.ValueChunk().Code)-1] != chunk.OP_RETURN {
				fn.ValueChunk().EmitBytes(chunk.OP_PUSH_UNDEFINED, chunk.OP_RETURN)
			}

		}
	case parser.NODE_ARROW_FUNCTION_EXPRESSION:
		{
			symbolTable = FUNCTION_SCOPES[current]
			newFn := object.NewFunction("ANONYMOYS_FN", len(current.Params), nil)
			handle := allocator.Allocate(newFn)
			value := value.EncodeHandle(handle)

			if current.IsExpression {
				generateByteCode(current.BodyNode, symbolTable, newFn)
			} else {
				for _, node := range current.BodyNode.Body {
					generateByteCode(node, symbolTable, newFn)
				}
			}

			if newFn.ValueChunk().Code[len(newFn.ValueChunk().Code)-1] != chunk.OP_RETURN {
				newFn.ValueChunk().EmitBytes(chunk.OP_RETURN)
			}

			fn.ValueChunk().WriteConstant(value)
		}
	case parser.NODE_FUNCTION_EXPRESSION:
		{
			symbolTable = FUNCTION_SCOPES[current]
			newFn := object.NewFunction("ANONYMOYS_FN", len(current.Params), nil)
			handle := allocator.Allocate(newFn)
			value := value.EncodeHandle(handle)

			if current.IsExpression {
				generateByteCode(current.BodyNode, symbolTable, newFn)
			} else {
				for _, node := range current.BodyNode.Body {
					generateByteCode(node, symbolTable, newFn)
				}
			}

			if newFn.ValueChunk().Code[len(newFn.ValueChunk().Code)-1] != chunk.OP_RETURN {
				newFn.ValueChunk().EmitBytes(chunk.OP_RETURN)
			}
			fn.ValueChunk().WriteConstant(value)
		}
	case parser.NODE_BLOCK_STATEMENT:
		{
			symbolTable.enterBlockScope(current)
			for _, node := range current.Body {
				generateByteCode(node, symbolTable, fn)
			}

			if symbolTable.block != nil {
				for range len(symbolTable.block.vars) {
					fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
				}
			}
			symbolTable.exitBlockScope()
		}
	case parser.NODE_VARIABLE_DECLARATION:
		{
			for _, declaration := range current.Declarations {
				popStack = false
				generateByteCode(declaration, symbolTable, fn)
				popStack = true
			}
		}
	case parser.NODE_ARRAY_EXPRESSION:
		{
			popStack = false
			fn.ValueChunk().EmitByte(chunk.OP_CREATE_ARRAY)
			fn.ValueChunk().EmitUint32(uint32(len(current.Elements)))

			for _, item := range current.Elements {
				generateByteCode(item, symbolTable, fn)
				fn.ValueChunk().EmitByte(chunk.OP_PUSH_ELEMENT)
			}
			popStack = true
		}
	case parser.NODE_CALL_EXPRESSION:
		{
			prevPopStack := popStack
			popStack = false
			for _, node := range current.Arguments {
				generateByteCode(node, symbolTable, fn)
			}
			generateByteCode(current.Callee, symbolTable, fn)
			popStack = prevPopStack

			fn.ValueChunk().EmitByte(chunk.OP_CALL)

			if popStack {
				fn.ValueChunk().EmitByte(chunk.OP_POP)
			}
		}
	case parser.NODE_MEMBER_EXPRESSION:
		{
			generateByteCode(current.Object, symbolTable, fn)
			handle := allocator.Allocate(object.NewObjString(current.Property.Name))
			memberSlot := fn.ValueChunk().AddConstant(value.EncodeHandle(handle))

			fn.ValueChunk().EmitBytes(chunk.OP_GET_OBJECT_MEMBER, memberSlot)
		}
	case parser.NODE_VARIABLE_DECLARATOR:
		{
			name := current.Identifier.Name
			variable, _ := symbolTable.findVariable(name)

			if variable != nil {
				if current.Initializer != nil {
					generateByteCode(current.Initializer, symbolTable, fn)
				} else if current.Initializer == nil && variable.type_ != FOR {
					fn.ValueChunk().EmitByte(chunk.OP_PUSH_UNDEFINED)
				}

				// for of loop i.e for (const item of arr) {}
				if variable.type_ == FOR {
					var op uint8
					if variable.init {
						switch variable.scope {
						case GLOBAL:
							op = chunk.OP_SET_GLOBAL
						case LOCAL:
							op = chunk.OP_SET_LOCAL
						}
						fn.ValueChunk().EmitBytes(op, uint8(variable.slot))
					} else {
						switch variable.scope {
						case GLOBAL:
							op = chunk.OP_DEFINE_GLOBAL
						case LOCAL:
							op = chunk.OP_DEFINE_LOCAL
						}
						variable.init = true
						fn.ValueChunk().EmitByte(op)
					}
					return
				}

				switch variable.scope {
				case GLOBAL:
					fn.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
				case LOCAL:
					fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)
				case HEAP:
					fn.ValueChunk().EmitByte(chunk.OP_DEFINE_HEAP_VAR)
				}

			}
		}
	case parser.NODE_EXPRESSION_STATEMENT:
		{
			generateByteCode(current.Expression, symbolTable, fn)
		}
	case parser.NODE_IF_STATEMENT:
		{
			generateByteCode(current.Test, symbolTable, fn)

			fn.ValueChunk().EmitBytes(chunk.OP_JUMP_IF_FALSE, 0, 0, 0, 0)
			jumpStart := len(fn.ValueChunk().Code) - 4

			generateByteCode(current.Consequent, symbolTable, fn)
			altJump := 0
			if current.Alternate != nil {
				fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
				altJump = len(fn.ValueChunk().Code) - 4
			}
			fn.ValueChunk().PatchUint32(uint32(jumpStart), uint32(len(fn.ValueChunk().Code)))

			if current.Alternate != nil {
				generateByteCode(current.Alternate, symbolTable, fn)
				fn.ValueChunk().PatchUint32(uint32(altJump), uint32(len(fn.ValueChunk().Code)))
			}
		}
	case parser.NODE_BINARY_EXPRESSION:
		{
			popStack = false
			generateByteCode(current.Left, symbolTable, fn)
			generateByteCode(current.Right, symbolTable, fn)
			popStack = true

			switch current.BinaryOperator {
			case parser.LESS_THAN:
				fn.ValueChunk().EmitByte(chunk.OP_LESS_THAN)
			case parser.GREATER_THAN:
				fn.ValueChunk().EmitByte(chunk.OP_GREATER_THAN)
			case parser.GREATER_THAN_EQUAL:
				fn.ValueChunk().EmitByte(chunk.OP_GREATER_THAN_EQUAL)
			case parser.LESS_THAN_EQUAL:
				fn.ValueChunk().EmitByte(chunk.OP_LESS_THAN_EQUAL)
			case parser.MINUS:
				fn.ValueChunk().EmitByte(chunk.OP_SUBTRACT)
			case parser.PLUS:
				fn.ValueChunk().EmitByte(chunk.OP_ADD)

			}
		}
	case parser.NODE_LITERAL:
		{
			switch v := current.Value.(type) {
			case float64:
				{
					fn.ValueChunk().WriteConstant(value.ValueFromFloat64(v))
				}
			case []byte:
				{
					objStr := constructor.NewString(string(v))
					handle := allocator.Allocate(objStr)
					fn.ValueChunk().WriteConstant(value.EncodeHandle(handle))
				}
			case bool:
				{
					if v {
						fn.ValueChunk().WriteConstant(value.EncodeTrue())
					} else {
						fn.ValueChunk().WriteConstant(value.EncodeFalse())

					}
				}
			case nil:
				{
					if current.Raw == "null" {
						fn.ValueChunk().WriteConstant(value.EncodeNil())
					}
				}
			}
		}
	case parser.NODE_RETURN_STATEMENT:
		{
			if current.Argument == nil {
				fn.ValueChunk().EmitByte(chunk.OP_PUSH_UNDEFINED)
			} else {
				popStack = false
				generateByteCode(current.Argument, symbolTable, fn)
				popStack = true
			}

			fn.ValueChunk().EmitByte(chunk.OP_RETURN)
		}
	case parser.NODE_IDENTIFIER:
		{
			variable, _ := symbolTable.findVariable(current.Name)

			if variable == nil {
				fn.ValueChunk().EmitByte(chunk.OP_PUSH_UNDEFINED)
				return
			}

			if variable.type_ == CATCH_PARAM && !variable.init {
				variable.init = true
				switch variable.scope {
				case GLOBAL:
					fn.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
				case LOCAL:
					fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)
				case HEAP:
					fn.ValueChunk().EmitByte(chunk.OP_DEFINE_HEAP_VAR)
				}
				return
			}

			switch variable.scope {
			case GLOBAL:
				fn.ValueChunk().EmitBytes(chunk.OP_GET_GLOBAL, uint8(variable.slot))
			case LOCAL:
				fn.ValueChunk().EmitBytes(chunk.OP_GET_LOCAL, uint8(variable.slot))
			case HEAP:
				fn.ValueChunk().EmitBytes(chunk.OP_GET_HEAP_VAR, uint8(variable.slot))
			}
		}
	case parser.NODE_WHILE_STATEMENT:
		{
			testStart := uint32(len(fn.ValueChunk().Code))
			generateByteCode(current.Test, symbolTable, fn)

			fn.ValueChunk().EmitBytes(chunk.OP_JUMP_IF_FALSE, 0, 0, 0, 0)
			jumpStart := uint32(len(fn.ValueChunk().Code) - 4)

			generateByteCode(current.BodyNode, symbolTable, fn)

			fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)

			fn.ValueChunk().PatchUint32(uint32(len(fn.ValueChunk().Code)-4), testStart)
			fn.ValueChunk().PatchUint32(jumpStart, uint32(len(fn.ValueChunk().Code)))
		}
	case parser.NODE_UPDATE_EXPRESSION:
		{
			variable, _ := symbolTable.findVariable(current.Argument.Name)

			switch variable.scope {
			case GLOBAL:
				fn.ValueChunk().EmitBytes(chunk.OP_GET_GLOBAL, uint8(variable.slot))
			case LOCAL:
				fn.ValueChunk().EmitBytes(chunk.OP_GET_LOCAL, uint8(variable.slot))
			case HEAP:
				fn.ValueChunk().EmitBytes(chunk.OP_GET_HEAP_VAR, uint8(variable.slot))
			}

			if !current.Prefix {
				if !popStack {
					fn.ValueChunk().EmitByte(chunk.OP_PUSH_CURRENT)
				}

				fn.ValueChunk().WriteConstant(value.ValueFromFloat64(1))
				switch current.UpdateOperator {
				case parser.DECREMENT:
					fn.ValueChunk().EmitByte(chunk.OP_SUBTRACT)
				case parser.INCREMENT:
					fn.ValueChunk().EmitByte(chunk.OP_ADD)
				}
			} else {
				fn.ValueChunk().WriteConstant(value.ValueFromFloat64(1))
				switch current.UnaryOperator {
				case "--":
					fn.ValueChunk().EmitByte(chunk.OP_SUBTRACT)
				case "++":
					fn.ValueChunk().EmitByte(chunk.OP_ADD)
				}

				if !popStack {
					fn.ValueChunk().EmitByte(chunk.OP_PUSH_CURRENT)
				}
			}

			switch variable.scope {
			case GLOBAL:
				fn.ValueChunk().EmitBytes(chunk.OP_SET_GLOBAL, uint8(variable.slot))
			case LOCAL:
				fn.ValueChunk().EmitBytes(chunk.OP_SET_LOCAL, uint8(variable.slot))
			case HEAP:
				fn.ValueChunk().EmitBytes(chunk.OP_SET_HEAP_VAR, uint8(variable.slot))
			}
		}
	case parser.NODE_FOR_STATEMENT:
		{
			symbolTable.enterBlockScope(current.BodyNode)
			generateByteCode(current.Initializer, symbolTable, fn)

			testStart := len(fn.ValueChunk().Code)
			generateByteCode(current.Test, symbolTable, fn)

			fn.ValueChunk().EmitBytes(chunk.OP_JUMP_IF_FALSE, 0, 0, 0, 0)
			jumpStart := uint32(len(fn.ValueChunk().Code) - 4)

			for _, node := range current.BodyNode.Body {
				generateByteCode(node, symbolTable, fn)
			}

			generateByteCode(current.Update, symbolTable, fn)

			fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
			fn.ValueChunk().PatchUint32(uint32(len(fn.ValueChunk().Code)-4), uint32(testStart))

			fn.ValueChunk().PatchUint32(jumpStart, uint32(len(fn.ValueChunk().Code)))
			count, _ := symbolTable.currentBlockVarCount()

			for range count {
				fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
			}
			symbolTable.exitBlockScope()

		}
	case parser.NODE_FOR_OF_STATEMENT:
		{
			symbolTable.enterBlockScope(current.BodyNode)
			fn.ValueChunk().EmitBytes(chunk.OP_PUSH_UNDEFINED, chunk.OP_DEFINE_LOCAL)
			generateByteCode(current.Right, symbolTable, fn)
			fn.ValueChunk().EmitByte(chunk.OP_GET_ITERATOR)
			fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_NEXT, chunk.OP_JUMP_IF_TRUE, 0, 0, 0, 0)
			jumpStart := uint32(len(fn.ValueChunk().Code) - 4)
			fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_CURRENT)

			variable, _ := symbolTable.findVariable(current.Left.Declarations[0].Identifier.Name)
			fn.ValueChunk().EmitBytes(chunk.OP_SET_LOCAL, uint8(variable.slot))

			for _, node := range current.BodyNode.Body {
				generateByteCode(node, symbolTable, fn)
			}

			fn.ValueChunk().EmitByte(chunk.OP_JUMP)
			fn.ValueChunk().EmitUint32(jumpStart - 2)
			fn.ValueChunk().PatchUint32(jumpStart, uint32(len(fn.ValueChunk().Code)))
			fn.ValueChunk().EmitByte(chunk.OP_POP) // pop the iterator object

			count, _ := symbolTable.currentBlockVarCount()
			for range count {
				fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
			}
			symbolTable.exitBlockScope()
		}
	case parser.NODE_OBJECT_EXPRESSION:
		{
			fn.ValueChunk().EmitByte(chunk.OP_CREATE_OBJECT)

			for _, property := range current.Properties {
				fn.ValueChunk().WriteConstant(value.EncodeHandle(allocator.Allocate(object.NewObjString(property.Key.Name))))
				generateByteCode(property.Value.(*parser.Node), symbolTable, fn)
				fn.ValueChunk().EmitBytes(chunk.OP_SET_OBJECT_MEMBER)
			}
		}
	case parser.NODE_TEMPLATE_LITERAL:
		{
			fn.ValueChunk().EmitByte(chunk.OP_TEMPLATE_LITERAL_START)
			for i := range len(current.Quasis) {
				quasi := current.Quasis[i].Value.(parser.TemplateNodeValue)
				tail := current.Quasis[i].Tail
				var expr *parser.Node

				if i < len(current.Expressions) {
					expr = current.Expressions[i]
				}

				if len(quasi.Raw) > 0 {
					fn.ValueChunk().WriteConstant(value.EncodeHandle(allocator.Allocate(object.NewObjString(quasi.Raw))))
					fn.ValueChunk().EmitByte(chunk.OP_TEMPLATE_PUSH_STRING)
				}

				if tail {
					fn.ValueChunk().EmitByte(chunk.OP_TEMPLATE_LITERAL_END)
					break
				}

				if expr != nil {
					generateByteCode(expr, symbolTable, fn)
					if expr.IsExpression {
						fn.ValueChunk().EmitByte(chunk.OP_CALL)
					}
					fn.ValueChunk().EmitByte(chunk.OP_TEMPLATE_PUSH_STRING)
				}
			}
		}
	case parser.NODE_TRY_STATEMENT:
		{
			fn.ValueChunk().EmitBytes(chunk.OP_TRY_BLOCK_START, 0, 0, 0, 0)
			tryStart := uint32(len(fn.ValueChunk().Code) - 4)

			generateByteCode(current.Block, symbolTable, fn)
			// tryBlockPointer := current.Block
			fn.ValueChunk().EmitByte(chunk.OP_TRY_BLOCK_END)
			fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
			fn.ValueChunk().PatchUint32(tryStart, uint32(len(fn.ValueChunk().Code)))

			jumpStart := uint32(len(fn.ValueChunk().Code) - 4)

			// parser.NODE_CATCH_CLAUSE
			current = current.Handler

			if current.Param != nil {
				symbolTable.enterBlockScope(current.BodyNode)
				generateByteCode(current.Param, symbolTable, fn)
				symbolTable.exitBlockScope()
			} else {
				fn.ValueChunk().EmitByte(chunk.OP_POP) // pop thrown error value if param is not used
			}
			generateByteCode(current.BodyNode, symbolTable, fn)
			fn.ValueChunk().PatchUint32(jumpStart, uint32(len(fn.ValueChunk().Code)))

			/* This needs to happen at runtime whenever a error is thrown,
			we could store the try blocks local count at the start:
			OP_TRY_BLOCK_START <catch start> <local count>

			For now only thrown values will work
			Let's see if I get to it later

				symbolTable.enterBlockScope(tryBlockPointer)
				count, _ := symbolTable.currentBlockVarCount()
				for range count {
					fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
				}
				symbolTable.exitBlockScope()
			*/
		}
	case parser.NODE_THROW_STATEMENT:
		{
			count, _ := symbolTable.currentBlockVarCount()

			for range count {
				fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
			}
			generateByteCode(current.Argument, symbolTable, fn)
			fn.ValueChunk().EmitByte(chunk.OP_THROW)
		}
	case parser.NODE_NEW_EXPRESSION:
		{
			for _, node := range current.Arguments {
				generateByteCode(node, symbolTable, fn)
			}
			//safeguards later: len(current.Arguments) > uint8.MAX
			println(len(current.Arguments))
			generateByteCode(current.Callee, symbolTable, fn)
			fn.ValueChunk().EmitByte(chunk.OP_NEW)
			fn.ValueChunk().EmitByte(uint8(len(current.Arguments)))

		}
	}
}
