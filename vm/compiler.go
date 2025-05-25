package vm

import (
	"go_js/parser"
	"log"
	"math"
	"runtime"
)

func Compile(ast *parser.Node, vm *VM) *ObjFunction {
	chunk := NewChunk()
	for _, node := range ast.Body {
		traverse(node, chunk, vm)
	}

	chunk.EmitByte(OP_EOF)

	return NewFunction("main", chunk)
}

func traverse(current *parser.Node, chunk *Chunk, vm *VM) {
	switch current.Type {
	case parser.NODE_EXPRESSION_STATEMENT:
		{
			traverse(current.Expression, chunk, vm)
		}
	case parser.NODE_BINARY_EXPRESSION:
		{
			traverse(current.Left, chunk, vm)
			traverse(current.Right, chunk, vm)
			switch current.BinaryOperator {
			case parser.PLUS:
				chunk.EmitByte(OP_ADD)
			case parser.DIVIDE:
				chunk.EmitByte(OP_DIVIDE)
			case parser.MULTIPLY:
				chunk.EmitByte(OP_MULTIPLY)
			case parser.MINUS:
				chunk.EmitByte(OP_SUBTRACT)
			}
		}
	case parser.NODE_LITERAL:
		{
			switch current.Value.(type) {
			case float64:
				{
					where := chunk.AddConstant(Value(math.Float64bits(current.Value.(float64))))
					chunk.EmitByte(OP_CONSTANT)
					chunk.EmitByte(where)
				}
			case []byte:
				{
					/*
						We might need to keep a ref pool, let's see if random segfaults start to appear later

						obj := vm.intern(string(current.Value.([]byte)))
						vm.refs = append(vm.refs, obj) Lets see if this becomes an issue
					*/
					encoded, err := vm.intern(string(current.Value.([]byte))).Encode()
					if err != nil {
						log.Fatalf("Error while encoding pointer -%e-", err)
					}
					where := chunk.AddConstant(encoded)
					chunk.EmitByte(OP_CONSTANT)
					chunk.EmitByte(where)
					runtime.GC() // just to see if segfaults start
				}
			}
		}
	case parser.NODE_TEMPLATE_LITERAL:
		{
			max := int(math.Max(float64(len(current.Quasis)), float64(len(current.Expressions))))
			templateArr := []*parser.Node{}

			for i := range max {
				if i < len(current.Quasis) {
					if !(current.Quasis[i].Start == current.Quasis[i].End && !current.Quasis[i].Tail) {
						templateArr = append(templateArr, current.Quasis[i])
					}
				}

				if i < len(current.Expressions) {
					templateArr = append(templateArr, current.Expressions[i])
				}
			}

			current, next, iterations := 0, 1, -1

			for current < len(templateArr) {
				c := templateArr[current]
				var n *parser.Node

				if next < len(templateArr) {
					n = templateArr[next]
				}

				if n != nil {
					iterations = iterations + 1
				}

				if c.Type != parser.NODE_TEMPLATE_ELEMENT {
					traverse(c, chunk, vm)
					if n != nil && !n.Tail {
						str := n.Value.(parser.TemplateNodeValue).Raw
						encoded, err := vm.intern(str).Encode()

						if err != nil {
							log.Fatalf("failed to read template string %e", err)
						}
						where := chunk.AddConstant(encoded)
						chunk.EmitByte(OP_CONSTANT)
						chunk.EmitByte(where)

						chunk.EmitByte(OP_ADD)
					}
				} else {
					str := c.Value.(parser.TemplateNodeValue).Raw
					encoded, err := vm.intern(str).Encode()

					if err != nil {
						log.Fatalf("failed to read template string %e", err)
					}
					where := chunk.AddConstant(encoded)
					chunk.EmitByte(OP_CONSTANT)
					chunk.EmitByte(where)

					if n != nil {
						traverse(n, chunk, vm)
					}
					chunk.EmitByte(OP_ADD)
				}
				current, next = next+1, next+2
			}

			for range iterations {
				chunk.EmitByte(OP_ADD)
			}
		}
	}

}
