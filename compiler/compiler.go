package compiler

import (
	"go_js/parser"
	"go_js/vm"
	"math"
)

func Compile(ast *parser.Node) *vm.ObjFunction {
	chunk := vm.NewChunk()
	for _, node := range ast.Body {
		traverse(node, chunk)
	}

	chunk.EmitByte(vm.OP_EOF)

	return vm.NewFunction("main", chunk)
}

func traverse(current *parser.Node, chunk *vm.Chunk) {
	switch current.Type {
	case parser.NODE_EXPRESSION_STATEMENT:
		{
			traverse(current.Expression, chunk)
		}
	case parser.NODE_BINARY_EXPRESSION:
		{

			traverse(current.Left, chunk)
			traverse(current.Right, chunk)
			switch current.BinaryOperator {
			case parser.PLUS:
				chunk.EmitByte(vm.OP_ADD)
			case parser.DIVIDE:
				chunk.EmitByte(vm.OP_DIVIDE)
			case parser.MULTIPLY:
				chunk.EmitByte(vm.OP_MULTIPLY)
			case parser.MINUS:
				chunk.EmitByte(vm.OP_SUBTRACT)
			}
		}
	case parser.NODE_LITERAL:
		{
			switch current.Value.(type) {
			case float64:
				{
					address := chunk.AddConstant(uint64(math.Float64bits(current.Value.(float64))))
					chunk.EmitByte(vm.OP_CONSTANT)
					chunk.EmitByte(address)
				}
			}
		}
	}
}
