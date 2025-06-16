package vm

import (
	"go_js/parser"
	"math"
	"strconv"
)

type Scope struct {
	parent   *Scope
	locals   int
	resolver map[string]int
}

type Globals struct {
	globals  int
	resolver map[string]int
}

func NewScope(parent *Scope) *Scope {
	return &Scope{parent: parent, locals: 0, resolver: map[string]int{}}
}

func Compile(ast *parser.Node, heap *Heap, main *ObjFunction) error {
	scope := NewScope(nil)
	globals := &Globals{0, map[string]int{}}
	err := traverse(ast, heap, main, scope, globals)

	if err != nil {
		return err
	}

	main.chunk.EmitByte(OP_EOF)
	return nil
}

func traverse(current *parser.Node, heap *Heap, fn *ObjFunction, scope *Scope, globals *Globals) error {
	switch current.Type {
	case parser.NODE_PROGRAM:
		{
			for _, statement := range current.Body {
				traverse(statement, heap, fn, scope, globals)
			}
		}
	case parser.NODE_EXPRESSION_STATEMENT:
		{
			traverse(current.Expression, heap, fn, scope, globals)
		}
	case parser.NODE_BLOCK_STATEMENT:
		{
			for _, stmt := range current.Body {
				traverse(stmt, heap, fn, NewScope(scope), globals)
			}
		}
	case parser.NODE_BINARY_EXPRESSION:
		{
			traverse(current.Left, heap, fn, scope, globals)
			traverse(current.Right, heap, fn, scope, globals)
			switch current.BinaryOperator {
			case parser.PLUS:
				fn.chunk.EmitByte(OP_ADD)
			case parser.MINUS:
				fn.chunk.EmitByte(OP_SUBTRACT)
			case parser.DIVIDE:
				fn.chunk.EmitByte(OP_DIVIDE)
			case parser.MULTIPLY:
				fn.chunk.EmitByte(OP_MULTIPLY)
			case parser.LESS_THAN:
				fn.chunk.EmitByte(OP_LESS_THAN)
			case parser.LESS_THAN_EQUAL:
				fn.chunk.EmitByte(OP_LESS_THAN_EQUAL)
			case parser.GREATER_THAN:
				fn.chunk.EmitByte(OP_GREATER_THAN)
			case parser.GREATER_THAN_EQUAL:
				fn.chunk.EmitByte(OP_GREATER_THAN_EQUAL)
			}
		}
	case parser.NODE_IF_STATEMENT:
		{
			traverse(current.Test, heap, fn, scope, globals)
			fn.chunk.EmitByte(OP_JUMP_IF_FALSE)
			fn.chunk.EmitByte(0)
			fn.chunk.EmitByte(0)
			fn.chunk.EmitByte(0)
			fn.chunk.EmitByte(0)
			start := len(fn.chunk.code)
			traverse(current.Consequent, heap, fn, scope, globals)
			jump := uint32(len(fn.chunk.code) - start)

			fn.chunk.code[start-1] = uint8(jump & math.MaxUint8)
			fn.chunk.code[start-2] = uint8((jump >> 8))
			fn.chunk.code[start-3] = uint8((jump >> 16))
			fn.chunk.code[start-4] = uint8((jump >> 24))
		}
	case parser.NODE_LITERAL:
		{
			switch current.Value.(type) {
			case float64:
				{
					fn.chunk.WriteConstant(ValueFromFloat64(current.Value.(float64)))
				}
			case []byte:
				{
					raw := string(current.Value.([]byte))
					value := heap.AllocateString(raw)
					fn.chunk.WriteConstant(value)
				}
			}
		}
	case parser.NODE_VARIABLE_DECLARATION:
		{
			for _, declaration := range current.Declarations {
				traverse(declaration, heap, fn, scope, globals)
			}
		}
	case parser.NODE_VARIABLE_DECLARATOR:
		{
			traverse(current.Initializer, heap, fn, scope, globals)
			if fn.name == MAIN_FN_NAME {
				fn.chunk.EmitByte(OP_DEFINE_GLOBAL)
				globals.resolver[current.Identifier.Name] = globals.globals
				globals.globals++
			} else {
				fn.chunk.EmitByte(OP_DEFINE_LOCAL)
				scope.resolver[current.Identifier.Name] = scope.locals
				scope.locals++
			}
		}
	case parser.NODE_IDENTIFIER:
		{
			currentScope := scope
			for currentScope != nil {
				if slot, found := currentScope.resolver[current.Name]; found {
					fn.chunk.EmitByte(OP_GET_LOCAL)
					fn.chunk.EmitByte(uint8(slot))
					return nil
				} else {
					currentScope = currentScope.parent
				}

			}

			if global, found := globals.resolver[current.Name]; found {
				fn.chunk.EmitByte(OP_GET_GLOBAL)
				fn.chunk.EmitByte(uint8(global))
				return nil
			}
		}
	case parser.NODE_FUNCTION_DECLARATION:
		{
			function := NewFunction(current.Identifier.Name, len(current.Params))
			register := heap.Allocate(function)
			isMain := fn.name == MAIN_FN_NAME

			if isMain {
				globals.resolver[function.name] = globals.globals
				globals.globals++
			} else {
				scope.resolver[function.name] = scope.locals
				scope.locals++
			}

			scope := NewScope(scope)
			fn.chunk.WriteConstant(EncodeObject(register))
			if isMain {
				fn.chunk.EmitByte(OP_DEFINE_GLOBAL)
			} else {
				fn.chunk.EmitByte(OP_DEFINE_LOCAL)
			}

			for _, param := range current.Params {
				scope.resolver[param.Name] = scope.locals
				scope.locals++
			}

			for _, statement := range current.BodyNode.Body {
				traverse(statement, heap, function, scope, globals)
			}
		}
		// For now just gonna treat them as the same, once we start binding 'this', etc... need to separate the implementations
	case parser.NODE_FUNCTION_EXPRESSION, parser.NODE_ARROW_FUNCTION_EXPRESSION:
		{
			name := "ANONYMOUS_FN_" + strconv.Itoa(scope.locals)
			function := NewFunction(name, len(current.Params))
			register := heap.Allocate(function)
			scope.resolver[name] = int(register)
			scope.locals++

			scope := NewScope(scope)
			fn.chunk.WriteConstant(EncodeObject(register))
			// fn.chunk.EmitByte(OP_DEFINE_VARIABLE)
			for _, param := range current.Params {
				scope.resolver[param.Name] = scope.locals
				scope.locals++
			}

			if current.IsExpression {
				traverse(current.BodyNode, heap, function, scope, globals)
				function.chunk.EmitByte(OP_RETURN)
			} else {
				for _, statement := range current.BodyNode.Body {
					traverse(statement, heap, function, scope, globals)
				}
			}

		}
	case parser.NODE_CALL_EXPRESSION:
		{
			for _, arg := range current.Arguments {
				traverse(arg, heap, fn, scope, globals)
			}
			if slot, found := scope.resolver[current.Callee.Name]; found {
				fn.chunk.EmitByte(OP_GET_LOCAL)
				fn.chunk.EmitByte(uint8(slot))
			}
			if global, found := globals.resolver[current.Callee.Name]; found {
				fn.chunk.EmitByte(OP_GET_GLOBAL)
				fn.chunk.EmitByte(uint8(global))
			}
			fn.chunk.EmitByte(OP_CALL)
		}
	case parser.NODE_RETURN_STATEMENT:
		{
			traverse(current.Argument, heap, fn, scope, globals)
			fn.chunk.EmitByte(OP_RETURN)
		}
	}
	return nil
}
