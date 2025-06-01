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
	scopeChain := NewScope(nil)
	err := traverse(ast, heap, main, scopeChain)

	if err != nil {
		return err
	}

	main.chunk.EmitByte(OP_EOF)
	return nil
}

func traverse(current *parser.Node, heap *Heap, fn *ObjFunction, scopeChain *Scope) error {
	switch current.Type {
	case parser.NODE_PROGRAM:
		{
			for _, statement := range current.Body {
				traverse(statement, heap, fn, scopeChain)
			}
		}
	case parser.NODE_EXPRESSION_STATEMENT:

		traverse(current.Expression, heap, fn, scopeChain)

	case parser.NODE_BINARY_EXPRESSION:
		{
			traverse(current.Left, heap, fn, scopeChain)
			traverse(current.Right, heap, fn, scopeChain)
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
				traverse(declaration, heap, fn, scopeChain)
			}
		}
	case parser.NODE_VARIABLE_DECLARATOR:
		{
			traverse(current.Initializer, heap, fn, scopeChain)
			fn.chunk.EmitByte(OP_DEFINE_VARIABLE)
			scopeChain.resolver[current.Identifier.Name] = scopeChain.locals
			scopeChain.locals++
		}
	case parser.NODE_IDENTIFIER:
		{
			fn.chunk.EmitByte(OP_GET_VARIABLE)
			fn.chunk.EmitByte(uint8(scopeChain.resolver[current.Name]))
		}
	}
	return nil
}
