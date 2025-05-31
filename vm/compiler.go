package vm

import (
	"go_js/parser"
)

func Compile(ast *parser.Node, heap *Heap, chunk *Chunk, strings map[string]*ObjString) error {
	err := traverse(ast, heap, chunk, strings)

	if err != nil {
		return err
	}

	chunk.EmitByte(OP_EOF)
	return nil
}

func traverse(current *parser.Node, heap *Heap, chunk *Chunk, strings map[string]*ObjString) error {
	switch current.Type {
	case parser.NODE_PROGRAM:
		{
			for _, statement := range current.Body {
				traverse(statement, heap, chunk, strings)
			}
		}
	case parser.NODE_EXPRESSION_STATEMENT:
		{
			traverse(current.Expression, heap, chunk, strings)
		}
	case parser.NODE_BINARY_EXPRESSION:
		{
			traverse(current.Left, heap, chunk, strings)
			traverse(current.Right, heap, chunk, strings)
			switch current.BinaryOperator {
			case parser.PLUS:
				chunk.EmitByte(OP_ADD)
			case parser.MINUS:
				chunk.EmitByte(OP_SUBTRACT)
			case parser.DIVIDE:
				chunk.EmitByte(OP_DIVIDE)
			case parser.MULTIPLY:
				chunk.EmitByte(OP_MULTIPLY)
			}
		}
	case parser.NODE_LITERAL:
		{
			switch current.Value.(type) {
			case float64:
				{
					chunk.WriteConstant(ValueFromFloat64(current.Value.(float64)))
				}
			}
		}
	}
	return nil
}
