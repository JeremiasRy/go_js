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
	FOR_OF
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
	init  bool
	fn    *object.ObjFunction
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
	varCount   int

	block *BlockScope
}

var BLOCK_SCOPES = map[*parser.Node]*BlockScope{}
var FUNCTION_SCOPES = map[*parser.Node]*FunctionScope{}

func newBlockScope(parent *BlockScope, forOf bool) *BlockScope {
	return &BlockScope{vars: Variables{}, parent: parent, forOf: forOf}
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

func (fs *FunctionScope) exitBlockScope() {
	fs.block = fs.block.parent
}

func (fs *FunctionScope) addVariable(name string, type_ VariableType, fn *object.ObjFunction) {
	var mapToAddTo Variables

	if fs.block != nil {
		if fs.block.forOf {
			type_ = FOR_OF
		}
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

var popStack = true

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

			if fn.ValueChunk().Code[len(fn.ValueChunk().Code)-1] != chunk.OP_RETURN {
				fn.ValueChunk().EmitBytes(chunk.OP_RETURN)
			}
		}
	case parser.NODE_BLOCK_STATEMENT:
		{
			symbolTable.enterBlockScope(current)
			for _, node := range current.Body {
				generateByteCode(node, symbolTable, fn)
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
			for _, node := range current.Arguments {
				generateByteCode(node, symbolTable, fn)
			}
			generateByteCode(current.Callee, symbolTable, fn)

			fn.ValueChunk().EmitByte(chunk.OP_CALL)

			if popStack {
				fn.ValueChunk().EmitByte(chunk.OP_POP)
			}
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
				if current.Initializer != nil {
					generateByteCode(current.Initializer, symbolTable, fn)
				}

				if variable.type_ == FOR_OF {
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
					//
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

			fn.ValueChunk().PatchJump(uint32(jumpStart), uint32(len(fn.ValueChunk().Code)))

			if current.Alternate != nil {
				generateByteCode(current.Alternate, symbolTable, fn)
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
					handle := heap.AllocateString(object.ObjString(v))
					fn.ValueChunk().WriteConstant(value.EncodeObject(handle))
				}
			}
		}
	case parser.NODE_RETURN_STATEMENT:
		{
			if current.Argument == nil {
				fn.ValueChunk().EmitByte(chunk.OP_PUSH_UNDEFINED)
			} else {
				generateByteCode(current.Argument, symbolTable, fn)
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

			switch variable.scope {
			case GLOBAL:
				fn.ValueChunk().EmitBytes(chunk.OP_GET_GLOBAL, uint8(variable.slot))
			case LOCAL:
				fn.ValueChunk().EmitBytes(chunk.OP_GET_LOCAL, uint8(variable.slot))
			case HEAP:
				//
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

			fn.ValueChunk().PatchJump(uint32(len(fn.ValueChunk().Code)-4), testStart)
			fn.ValueChunk().PatchJump(jumpStart, uint32(len(fn.ValueChunk().Code)))
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
				//
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
				//
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
			symbolTable.exitBlockScope()

			generateByteCode(current.BodyNode, symbolTable, fn)
			symbolTable.enterBlockScope(current.BodyNode)
			generateByteCode(current.Update, symbolTable, fn)
			symbolTable.exitBlockScope()
			fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
			fn.ValueChunk().PatchJump(uint32(len(fn.ValueChunk().Code)-4), uint32(testStart))

			fn.ValueChunk().PatchJump(jumpStart, uint32(len(fn.ValueChunk().Code)))
		}
	case parser.NODE_FOR_OF_STATEMENT:
		{
			symbolTable.enterBlockScope(current.BodyNode)
			fn.ValueChunk().EmitByte(chunk.OP_PUSH_UNDEFINED)
			generateByteCode(current.Left, symbolTable, fn)
			generateByteCode(current.Right, symbolTable, fn)
			fn.ValueChunk().EmitByte(chunk.OP_GET_ITERATOR)
			fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_NEXT, chunk.OP_JUMP_IF_TRUE, 0, 0, 0, 0)
			jumpStart := uint32(len(fn.ValueChunk().Code) - 4)
			fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_CURRENT)
			generateByteCode(current.Left, symbolTable, fn)

			for _, node := range current.BodyNode.Body {
				generateByteCode(node, symbolTable, fn)
			}
			fn.ValueChunk().EmitByte(chunk.OP_JUMP)
			fn.ValueChunk().EmitUint32(jumpStart - 2)
			fn.ValueChunk().PatchJump(jumpStart, uint32(len(fn.ValueChunk().Code)))
			fn.ValueChunk().EmitByte(chunk.OP_POP) // pop the iterator object

		}
	}
}
