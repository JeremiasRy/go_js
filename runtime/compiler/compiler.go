package compiler

import (
	"cmp"
	"fmt"
	"go_js/allocator"
	"go_js/chunk"
	"go_js/native"
	"go_js/object"
	"go_js/parser"
	"go_js/value"
	"slices"
)

const CONSOLE_OBJECT_NAME string = "console"
const RESERVED_ARGUMENTS string = "arguments"
const UNDEFINED_IDENTIFIER string = "undefined"
const SET_TIMEOUT_NAME string = "setTimeout"

type VariableType uint8

const (
	CONST VariableType = iota
	LET
	FUNCTION
	METHOD
	FOR
	CATCH_PARAM
)

type VariableScope uint8

const (
	HEAP VariableScope = iota
	LOCAL
	GLOBAL
	THIS
)

type Variable struct {
	scope      VariableScope
	type_      VariableType
	slot       int  // for locals and globals
	init       bool // used in for of loops
	undeclared bool // used for undeclared variables i.e in assignments to unknown variable {e = 2}
	fn         object.Callable
}

var ThisVariable *Variable = &Variable{scope: THIS}

type Variables map[string]*Variable
type BlockScope struct {
	parent *BlockScope
	vars   Variables
}

type FunctionScope struct {
	parent              *FunctionScope
	tableScope          VariableScope
	vars                Variables
	block               *BlockScope
	arity               int
	needsArgumentsSlice bool
}

const (
	ITERATOR_FOR_OF uint8 = iota
	ITERATOR_FOR_IN
)

var BLOCK_SCOPES = map[*parser.Node]*BlockScope{}
var FUNCTION_SCOPES = map[*parser.Node]*FunctionScope{}

var operatorMap = map[parser.BinaryOperator]uint8{
	parser.LESS_THAN:          chunk.OP_LESS_THAN,
	parser.GREATER_THAN:       chunk.OP_GREATER_THAN,
	parser.GREATER_THAN_EQUAL: chunk.OP_GREATER_THAN_EQUAL,
	parser.LESS_THAN_EQUAL:    chunk.OP_LESS_THAN_EQUAL,
	parser.MINUS:              chunk.OP_SUBTRACT,
	parser.PLUS:               chunk.OP_ADD,
	parser.DIVIDE:             chunk.OP_DIVIDE,
	parser.MULTIPLY:           chunk.OP_MULTIPLY,
	parser.EXPONENTIATION:     chunk.OP_EXPONENTIATION,
	parser.MODULUS:            chunk.OP_MODULO,
	parser.EQUALS:             chunk.OP_EQUALS,
	parser.STRICT_EQUALS:      chunk.OP_STRICT_EQUALS,
	parser.STRICT_NOT_EQUALS:  chunk.OP_STRICT_NOT_EQUALS,
	"||":                      chunk.OP_LOGICAL_OR,
	"&&":                      chunk.OP_LOGICAL_AND,
}

var unaryOperatorMap = map[parser.UnaryOperator]uint8{
	parser.UNARY_NEGATE: chunk.OP_NEGATE,
}

func newBlockScope(parent *BlockScope) *BlockScope {
	return &BlockScope{vars: Variables{}, parent: parent}
}

func newFunctionScope(parent *FunctionScope, tableScope VariableScope) *FunctionScope {
	return &FunctionScope{parent: parent, tableScope: tableScope, vars: Variables{}, block: nil}
}

func (fs *FunctionScope) addArgumentsLocalToFunctionScope() {
	variable := &Variable{scope: LOCAL, type_: CONST, slot: -1, init: false, undeclared: false, fn: nil}
	fs.vars[RESERVED_ARGUMENTS] = variable
	for _, variable := range fs.vars {
		if variable.slot >= fs.arity {
			variable.slot++
		}
	}
	fs.vars[RESERVED_ARGUMENTS].slot = fs.arity
	fs.needsArgumentsSlice = true
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

func (fs *FunctionScope) addVariable(name string, type_ VariableType, undeclared bool, fn object.Callable) {
	var mapToAddTo Variables

	if fs.block != nil {
		mapToAddTo = fs.block.vars
	} else {
		mapToAddTo = fs.vars
	}

	if _, found := mapToAddTo[name]; found {
		//fmt.Printf("WARN: Already found variable %s from scope\n", name)
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

	// nothing found check the function scope
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
	console := native.NewObjectConsole()
	consoleHandle := allocator.Allocate(console)
	symbolTable.addVariable(CONSOLE_OBJECT_NAME, CONST, false, nil)

	main.ValueChunk().WriteConstant(value.EncodeHandle(consoleHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineObjectConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	objCtor := native.NewObjectConstructor()
	objCtorHandle := allocator.Allocate(objCtor)

	symbolTable.addVariable(native.OBJECT_CONSTRUCTOR_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(objCtorHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineArrayConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	arrCtor := &native.ArrayConstructor{}
	arrCtorHandle := allocator.Allocate(arrCtor)

	symbolTable.addVariable(native.ARRAY_CONSTRUCTOR_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(arrCtorHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineErrorConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	ctor := &native.ErrorConstructor{}
	ctorHandle := allocator.Allocate(ctor)

	symbolTable.addVariable(native.ERROR_CONSTRUCTOR_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(ctorHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineSetTimeout(main *object.ObjFunction, symbolTable *FunctionScope) {
	setTimeout := native.NewSetTimeout()
	setTimeouthandle := allocator.Allocate(setTimeout)

	symbolTable.addVariable(SET_TIMEOUT_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(setTimeouthandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func definePromiseConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	promiseCtor := native.NewPromiseConstructor()
	promiseCtorHandle := allocator.Allocate(promiseCtor)

	symbolTable.addVariable(native.PROMISE_CONSTRUCTOR_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(promiseCtorHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineDateConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	dateCtor := native.NewDateConstructor()
	dateCtorHandle := allocator.Allocate(dateCtor)

	symbolTable.addVariable(native.DATE_CONSTRUCTOR_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(dateCtorHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func Compile(ast *parser.Node) (*object.ObjFunction, error) {
	main := object.NewFunction(object.MAIN_FN_NAME, 0, nil)
	var symbolTable *FunctionScope = newFunctionScope(nil, GLOBAL)

	defineObjectConstructor(main, symbolTable)
	defineConsole(main, symbolTable)
	defineSetTimeout(main, symbolTable)
	defineErrorConstructor(main, symbolTable)
	defineArrayConstructor(main, symbolTable)
	definePromiseConstructor(main, symbolTable)
	defineDateConstructor(main, symbolTable)

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

			var variable *Variable
			isMember := false

			switch current.Left.Type {
			case parser.NODE_MEMBER_EXPRESSION:
				isMember = true
				if current.Left.Object.Type == parser.NODE_THIS_EXPRESSION {
					variable = ThisVariable
					break
				}
				variable, _ = symbolTable.findVariable(current.Left.Object.Name)
			default:
				variable, _ = symbolTable.findVariable(current.Left.Name)

			}

			var defineOp uint8
			var setOp uint8
			var getOp uint8

			switch variable.scope {
			case LOCAL:
				{
					defineOp = chunk.OP_DEFINE_LOCAL
					setOp = chunk.OP_SET_LOCAL
					getOp = chunk.OP_GET_LOCAL
				}
			case GLOBAL:
				{
					defineOp = chunk.OP_DEFINE_GLOBAL
					setOp = chunk.OP_SET_GLOBAL
					getOp = chunk.OP_GET_GLOBAL
				}
			case HEAP:
				{
					defineOp = chunk.OP_DEFINE_HEAP_VAR
					setOp = chunk.OP_SET_HEAP_VAR
					getOp = chunk.OP_GET_HEAP_VAR
				}
			case THIS:
				{
					getOp = chunk.OP_THIS
				}
			}

			if isMember {
				setOp = chunk.OP_SET_OBJECT_MEMBER
				fn.ValueChunk().EmitByte(getOp)

				if getOp != chunk.OP_THIS {
					fn.ValueChunk().EmitByte(uint8(variable.slot))
				}

				switch current.AssignmentOperator {
				case parser.ASSIGN:
					{
						if current.Left.Computed {
							generateByteCode(current.Left.Property, symbolTable, fn)
						} else {
							str := native.LightString(current.Left.Property.Name)
							handle := allocator.Allocate(str)
							fn.ValueChunk().WriteConstant(value.EncodeHandle(handle))
						}
						generateByteCode(current.Right, symbolTable, fn)
						fn.ValueChunk().EmitBytes(setOp, chunk.OP_POP)
					}
				}
			} else {
				switch current.AssignmentOperator {
				case parser.ASSIGN:
					{
						prevPopStack := popStack
						popStack = false
						generateByteCode(current.Right, symbolTable, fn)
						popStack = prevPopStack

						if variable.undeclared {
							fn.ValueChunk().EmitByte(defineOp)
							variable.undeclared = false
						} else {
							fn.ValueChunk().EmitBytes(setOp, uint8(variable.slot))
						}
					}
				case parser.PLUS_ASSIGN:
					{
						fn.ValueChunk().EmitBytes(getOp, uint8(variable.slot))
						generateByteCode(current.Right, symbolTable, fn)
						fn.ValueChunk().EmitByte(chunk.OP_ADD)
						fn.ValueChunk().EmitBytes(setOp, uint8(variable.slot))
					}
				}
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

			if fn.GetArity() > 0 {
				heapVars := []int{}

				for i, n := range current.Params {
					if v, found := symbolTable.vars[n.Name]; found {
						if v.scope == HEAP {
							heapVars = append(heapVars, i)
						}
					}
				}

				if len(heapVars) > 0 {
					fn.ValueChunk().EmitBytes(chunk.OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS, uint8(len(heapVars)))
					for _, slot := range heapVars {
						fn.ValueChunk().EmitByte(uint8(slot))
					}
				}
			}

			if symbolTable.needsArgumentsSlice {
				fn.ValueChunk().EmitByte(chunk.OP_ADD_ARGUMENTS_TO_LOCALS)
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
			v := value.EncodeHandle(handle)

			functions := []*Variable{}
			isInHeapScopeAlready := symbolTable.isInHeapScope()
			for _, variable := range symbolTable.vars {
				if variable.type_ == FUNCTION {
					functions = append(functions, variable)
				}

				// if not in heap scope we'll create a new one
				if !isInHeapScopeAlready && variable.scope == HEAP {
					newFn.ValueChunk().EmitByte(chunk.OP_CREATE_HEAP_SCOPE)
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

			if len(current.Params) > 0 {
				heapVars := []int{}

				for i, n := range current.Params {
					if v, found := symbolTable.vars[n.Name]; found {
						if v.scope == HEAP {
							heapVars = append(heapVars, i)
						}
					}
				}

				if len(heapVars) > 0 {
					newFn.ValueChunk().EmitBytes(chunk.OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS, uint8(len(heapVars)))
					for _, slot := range heapVars {
						newFn.ValueChunk().EmitByte(uint8(slot))
					}
				}
			}

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

			fn.ValueChunk().WriteConstant(v)
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

			if current.Callee != nil {
				callee, _ := symbolTable.findVariable(current.Callee.Name)
				if callee != nil && callee.fn != nil && len(current.Arguments) > callee.fn.GetArity() {
					fn.ValueChunk().EmitBytes(chunk.OP_STORE_ARG_COUNT, uint8(len(current.Arguments)))
				}
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
			switch current.Property.Type {
			case parser.NODE_LITERAL:
				generateByteCode(current.Property, symbolTable, fn)

			case parser.NODE_IDENTIFIER:
				if current.Computed {
					generateByteCode(current.Property, symbolTable, fn)
				} else {
					handle := allocator.Allocate(native.LightString(current.Property.Name))
					fn.ValueChunk().WriteConstant(value.EncodeHandle(handle))
				}
			case parser.NODE_UNARY_EXPRESSION:
				{
					generateByteCode(current.Property, symbolTable, fn)
				}
			}
			fn.ValueChunk().EmitByte(chunk.OP_GET_OBJECT_MEMBER)
		}
	case parser.NODE_THIS_EXPRESSION:
		{
			fn.ValueChunk().EmitByte(chunk.OP_THIS)
		}
	case parser.NODE_VARIABLE_DECLARATOR:
		{
			if current.Identifier.Type == parser.NODE_ARRAY_PATTERN {
				pattern := current.Identifier
				var arr *Variable
				if current.Initializer.Type == parser.NODE_IDENTIFIER {
					arr, _ = symbolTable.findVariable(current.Initializer.Name)
				} else {
					generateByteCode(current.Initializer, symbolTable, fn)
				}

				var getOp uint8

				switch arr.scope {
				case LOCAL:
					getOp = chunk.OP_GET_LOCAL
				case GLOBAL:
					getOp = chunk.OP_GET_GLOBAL
				case HEAP:
					getOp = chunk.OP_GET_HEAP_VAR
				}

				for i, element := range pattern.Elements {
					var defineOp uint8

					el, _ := symbolTable.findVariable(element.Name)

					switch el.scope {
					case LOCAL:
						defineOp = chunk.OP_DEFINE_LOCAL
					case GLOBAL:
						defineOp = chunk.OP_DEFINE_GLOBAL
					case HEAP:
						defineOp = chunk.OP_DEFINE_HEAP_VAR
					}

					fn.ValueChunk().EmitBytes(getOp, uint8(arr.slot))
					fn.ValueChunk().WriteConstant(value.ValueFromFloat64(float64(i)))
					fn.ValueChunk().EmitBytes(chunk.OP_GET_OBJECT_MEMBER, defineOp)
				}
				return
			}

			if current.Identifier.Type == parser.NODE_OBJECT_PATTERN {
				pattern := current.Identifier
				var obj *Variable

				if current.Initializer.Type == parser.NODE_IDENTIFIER {
					obj, _ = symbolTable.findVariable(current.Initializer.Name)
				} else {
					generateByteCode(current.Initializer, symbolTable, fn)
				}

				var getOp uint8

				switch obj.scope {
				case LOCAL:
					getOp = chunk.OP_GET_LOCAL
				case GLOBAL:
					getOp = chunk.OP_GET_GLOBAL
				case HEAP:
					getOp = chunk.OP_GET_HEAP_VAR
				}

				for _, prop := range pattern.Properties {
					var defineOp uint8
					k := prop.Key.Name
					v := prop.Value.(*parser.Node).Name

					property, _ := symbolTable.findVariable(v)

					switch property.scope {
					case LOCAL:
						defineOp = chunk.OP_DEFINE_LOCAL
					case GLOBAL:
						defineOp = chunk.OP_DEFINE_GLOBAL
					case HEAP:
						defineOp = chunk.OP_DEFINE_HEAP_VAR
					}

					fn.ValueChunk().EmitBytes(getOp, uint8(obj.slot))

					if prop.Shorthand {
						fn.ValueChunk().WriteConstant(value.EncodeHandle(allocator.Allocate(native.LightString(v))))
					} else {
						fn.ValueChunk().WriteConstant(value.EncodeHandle(allocator.Allocate(native.LightString(k))))

					}
					fn.ValueChunk().EmitBytes(chunk.OP_GET_OBJECT_MEMBER, defineOp)
				}
				return
			}

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

			fn.ValueChunk().EmitByte(operatorMap[current.BinaryOperator])
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
					fn.ValueChunk().WriteConstant(value.EncodeHandle(allocator.Allocate(native.LightString(v))))
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
			fn.ReturnsPromise(checkIfNewArgumentIsPromise(current.Argument))

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
			if current.Name == UNDEFINED_IDENTIFIER {
				fn.ValueChunk().WriteConstant(value.EncodedUndefined())
				return
			}
			variable, _ := symbolTable.findVariable(current.Name)

			if variable == nil {
				msg := value.EncodeHandle(allocator.Allocate(native.LightString(fmt.Sprintf("identifier %s is undeclared", current.Name))))

				err := native.NewError()
				err.SetMember(native.KEY_MESSAGE, msg)

				fn.ValueChunk().WriteConstant(value.EncodeHandle(allocator.Allocate(err)))
				fn.ValueChunk().EmitByte(chunk.OP_THROW)
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
			count, _ := symbolTable.currentBlockVarCount()

			if count > 1 {
				for count > 1 {
					fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
					count--
				}
			}

			generateByteCode(current.Update, symbolTable, fn)

			fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
			fn.ValueChunk().PatchUint32(uint32(len(fn.ValueChunk().Code)-4), uint32(testStart))

			fn.ValueChunk().PatchUint32(jumpStart, uint32(len(fn.ValueChunk().Code)))
			fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL) // pop initalizer var needs to be fixed...
			symbolTable.exitBlockScope()

		}
	case parser.NODE_FOR_OF_STATEMENT:
		{
			symbolTable.enterBlockScope(current.BodyNode)
			for range symbolTable.block.vars {
				fn.ValueChunk().EmitBytes(chunk.OP_PUSH_UNDEFINED, chunk.OP_DEFINE_LOCAL)
			}
			generateByteCode(current.Right, symbolTable, fn)
			fn.ValueChunk().EmitBytes(chunk.OP_GET_ITERATOR, ITERATOR_FOR_OF)
			fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_NEXT, chunk.OP_JUMP_IF_TRUE, 0, 0, 0, 0)
			jumpStart := uint32(len(fn.ValueChunk().Code) - 4)
			fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_CURRENT)

			parseForDotDotLoopVariable(current, symbolTable, fn)

			for _, node := range current.BodyNode.Body {
				generateByteCode(node, symbolTable, fn)
			}
			if len(symbolTable.block.vars) > 1 {
				fn.ValueChunk().EmitByte(chunk.OP_POP)
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
	case parser.NODE_FOR_IN_STATEMENT:
		{
			symbolTable.enterBlockScope(current.BodyNode)
			fn.ValueChunk().EmitBytes(chunk.OP_PUSH_UNDEFINED, chunk.OP_DEFINE_LOCAL)
			generateByteCode(current.Right, symbolTable, fn)
			fn.ValueChunk().EmitBytes(chunk.OP_GET_ITERATOR, ITERATOR_FOR_IN)
			fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_NEXT, chunk.OP_JUMP_IF_TRUE, 0, 0, 0, 0)
			jumpStart := uint32(len(fn.ValueChunk().Code) - 4)
			fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_CURRENT)

			parseForDotDotLoopVariable(current, symbolTable, fn)

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

			prevPopstack := popStack
			popStack = false
			for _, property := range current.Properties {
				fn.ValueChunk().WriteConstant(value.EncodeHandle(allocator.Allocate(native.LightString(property.Key.Name))))
				generateByteCode(property.Value.(*parser.Node), symbolTable, fn)
				fn.ValueChunk().EmitBytes(chunk.OP_SET_OBJECT_MEMBER)
			}
			popStack = prevPopstack
		}
	case parser.NODE_TEMPLATE_LITERAL:
		{
			start := native.LightString(current.Quasis[0].Value.(parser.TemplateNodeValue).Raw)
			fn.ValueChunk().WriteConstant(value.EncodeHandle(allocator.Allocate(start)))

			if len(current.Expressions) > 0 {
				generateByteCode(current.Expressions[0], symbolTable, fn)
			}

			fn.ValueChunk().EmitByte(chunk.OP_ADD)
			i := 1

			for i < len(current.Quasis) {
				quasi := current.Quasis[i].Value.(parser.TemplateNodeValue)
				if len(quasi.Raw) == 0 {
					break
				}
				fn.ValueChunk().WriteConstant(value.EncodeHandle(allocator.Allocate(native.LightString(quasi.Raw))))
				fn.ValueChunk().EmitByte(chunk.OP_ADD)

				if i < len(current.Expressions) {
					generateByteCode(current.Expressions[i], symbolTable, fn)
					fn.ValueChunk().EmitByte(chunk.OP_ADD)
				}
				i++
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
				fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)
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
			generateByteCode(current.Callee, symbolTable, fn)
			fn.ValueChunk().EmitByte(chunk.OP_NEW)
			fn.ValueChunk().EmitByte(uint8(len(current.Arguments)))

		}
	case parser.NODE_CONDITIONAL_EXPRESSION:
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
	case parser.NODE_LOGICAL_EXPRESSION:
		{
			generateByteCode(current.Left, symbolTable, fn)
			generateByteCode(current.Right, symbolTable, fn)
			fn.ValueChunk().EmitByte(operatorMap[current.BinaryOperator])
		}
	case parser.NODE_AWAIT_EXPRESSION:
		{
			popStack = false
			generateByteCode(current.Argument, symbolTable, fn)
			popStack = true
			fn.ValueChunk().EmitByte(chunk.OP_AWAIT)
		}
	case parser.NODE_UNARY_EXPRESSION:
		{
			generateByteCode(current.Argument, symbolTable, fn)
			fn.ValueChunk().EmitByte(unaryOperatorMap[current.UnaryOperator])
		}
	case parser.NODE_CLASS_DECLARATION:
		{
			fn.ValueChunk().WriteConstant(value.EncodeHandle(allocator.Allocate(native.LightString(current.Identifier.Name))))
			fn.ValueChunk().EmitByte(chunk.OP_CREATE_CLASS_START)

			generateByteCode(current.BodyNode, symbolTable, fn)

			fn.ValueChunk().EmitByte(chunk.OP_CREATE_CLASS_END)
			switch symbolTable.tableScope {
			case GLOBAL:
				fn.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
			case LOCAL:
				fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)
			}
		}
	case parser.NODE_CLASS_BODY:
		{
			symbolTable = FUNCTION_SCOPES[current]
			for _, node := range current.Body {
				generateByteCode(node, symbolTable, fn)
			}
		}
	case parser.NODE_METHOD_DEFINITION:
		{
			symbolTable = FUNCTION_SCOPES[current]
			name := current.Key.Name

			function := current.Value.(*parser.Node)
			method := object.NewFunction(fmt.Sprintf("Class method %s", name), len(function.Params), nil)

			for _, node := range function.BodyNode.Body {
				generateByteCode(node, symbolTable, method)
			}
			if method.ValueChunk().Code[len(method.ValueChunk().Code)-1] != chunk.OP_RETURN {
				method.ValueChunk().EmitBytes(chunk.OP_PUSH_UNDEFINED, chunk.OP_RETURN)
			}

			fn.ValueChunk().WriteConstant(value.EncodeHandle(allocator.Allocate(native.LightString(name))))
			fn.ValueChunk().WriteConstant(value.EncodeHandle(allocator.Allocate(method)))
			fn.ValueChunk().EmitByte(chunk.OP_PUSH_METHOD)
		}
	case parser.NODE_PROPERTY_DEFINITION:
		{
			name := current.Key.Name
			fn.ValueChunk().WriteConstant(value.EncodeHandle(allocator.Allocate(native.LightString(name))))
			generateByteCode(current.Value.(*parser.Node), symbolTable, fn)

			fn.ValueChunk().EmitByte(chunk.OP_PUSH_PROPERTY)
		}
	case parser.NODE_YIELD_EXPRESSION:
		{
			generateByteCode(current.Argument, symbolTable, fn)
			fn.ValueChunk().EmitByte(chunk.OP_YIELD)
		}
	}
}

func parseForDotDotLoopVariable(current *parser.Node, symbolTable *FunctionScope, fn object.Callable) {
	current = current.Left
	switch current.Type {
	case parser.NODE_VARIABLE_DECLARATION:
		{
			for _, declaration := range current.Declarations {
				switch declaration.Identifier.Type {
				case parser.NODE_IDENTIFIER:
					{
						variable, _ := symbolTable.findVariable(declaration.Identifier.Name)
						fn.ValueChunk().EmitBytes(chunk.OP_SET_LOCAL, uint8(variable.slot))
						return
					}
				case parser.NODE_OBJECT_PATTERN:
					{
						pattern := declaration.Identifier
						for i, prop := range pattern.Properties {
							var defineOp uint8
							k := prop.Key.Name
							v := prop.Value.(*parser.Node).Name

							property, _ := symbolTable.findVariable(v)

							switch property.scope {
							case LOCAL:
								defineOp = chunk.OP_SET_LOCAL
							case GLOBAL:
								defineOp = chunk.OP_SET_GLOBAL
							case HEAP:
								defineOp = chunk.OP_SET_HEAP_VAR
							}

							if prop.Shorthand {
								fn.ValueChunk().WriteConstant(value.EncodeHandle(allocator.Allocate(native.LightString(v))))
							} else {
								fn.ValueChunk().WriteConstant(value.EncodeHandle(allocator.Allocate(native.LightString(k))))

							}
							fn.ValueChunk().EmitBytes(chunk.OP_GET_OBJECT_MEMBER, defineOp, uint8(property.slot))
							if i < len(pattern.Properties) {
								fn.ValueChunk().EmitByte(chunk.OP_ITERATOR_CURRENT)
							}
						}
						return
					}
				default:
					parser.PrintNode(current)
					panic("unsupported for of variable declaration")
				}
			}
		}
	}
}

func checkIfNewArgumentIsPromise(current *parser.Node) bool {
	switch current.Type {
	case parser.NODE_NEW_EXPRESSION:
		{
			if current.Callee.Type == parser.NODE_IDENTIFIER {
				return current.Callee.Name == native.PROMISE_CONSTRUCTOR_NAME
			}
		}
	}
	return false
}
