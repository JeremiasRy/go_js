package vm

import (
	"go_js/parser"
)

type Scope struct {
	parent   *Scope
	locals   int
	resolver map[string]int
}

func NewScope(parent *Scope) *Scope {
	return &Scope{parent: parent, locals: 0, resolver: map[string]int{}}
}

func Compile(ast *parser.Node, heap *Heap, main *ObjFunction) error {
	scope := NewScope(nil)
	err := traverse(ast, heap, main, scope)

	if err != nil {
		return err
	}

	main.chunk.EmitByte(OP_EOF)
	return nil
}

func traverse(current *parser.Node, heap *Heap, fn *ObjFunction, scope *Scope) error {
	switch current.Type {
	case parser.NODE_PROGRAM:
		{
			for _, statement := range current.Body {
				traverse(statement, heap, fn, scope)
			}
		}
	case parser.NODE_EXPRESSION_STATEMENT:
		{
			traverse(current.Expression, heap, fn, scope)
		}

	case parser.NODE_BINARY_EXPRESSION:
		{
			traverse(current.Left, heap, fn, scope)
			traverse(current.Right, heap, fn, scope)
			switch current.BinaryOperator {
			case parser.PLUS:
				fn.chunk.EmitByte(OP_ADD)
			case parser.MINUS:
				fn.chunk.EmitByte(OP_SUBTRACT)
			case parser.DIVIDE:
				fn.chunk.EmitByte(OP_DIVIDE)
			case parser.MULTIPLY:
				fn.chunk.EmitByte(OP_MULTIPLY)
			}
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
				traverse(declaration, heap, fn, scope)
			}
		}
	case parser.NODE_VARIABLE_DECLARATOR:
		{
			traverse(current.Initializer, heap, fn, scope)
			fn.chunk.EmitByte(OP_DEFINE_VARIABLE)
			scope.resolver[current.Identifier.Name] = scope.locals
			scope.locals++
		}
	case parser.NODE_IDENTIFIER:
		{
			fn.chunk.EmitByte(OP_GET_VARIABLE)
			fn.chunk.EmitByte(uint8(scope.resolver[current.Name]))
		}
	case parser.NODE_FUNCTION_DECLARATION:
		{
			function := NewFunction(current.Identifier.Name, len(current.Params))
			register := heap.Allocate(function)
			scope.resolver[function.name] = int(register)

			scope := NewScope(scope)

			fn.chunk.WriteConstant(EncodeObject(register))
			fn.chunk.EmitByte(OP_DEFINE_VARIABLE)
			for _, param := range current.Params {
				scope.resolver[param.Name] = scope.locals
				scope.locals++
			}

			for _, statement := range current.BodyNode.Body {
				traverse(statement, heap, function, scope)
			}
		}
	case parser.NODE_CALL_EXPRESSION:
		{
			for _, arg := range current.Arguments {
				traverse(arg, heap, fn, scope)
			}
			fn.chunk.EmitByte(OP_GET_VARIABLE)
			fn.chunk.EmitByte(uint8(scope.resolver[current.Callee.Name]))
			fn.chunk.EmitByte(OP_CALL)
		}
	case parser.NODE_RETURN_STATEMENT:
		{
			traverse(current.Argument, heap, fn, scope)
			fn.chunk.EmitByte(OP_RETURN)
		}
	}
	return nil
}
