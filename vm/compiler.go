package vm

import (
	"go_js/parser"
	"math"
	"strconv"
)

type Scope struct {
	parent   *Scope
	count    int
	resolver map[string]int
}

func (s *Scope) findSlot(name string) (bool, uint8) {
	current := s

	for current != nil {
		if slot, found := current.resolver[name]; found {
			return true, uint8(slot)
		}
		current = current.parent
	}
	return false, 0
}

func NewScope(parent *Scope) *Scope {
	return &Scope{parent: parent, count: 0, resolver: map[string]int{}}
}

func defineConsole(main *ObjFunction, globals *Scope) {
	globals.resolver["console"] = 0
	globals.count++

	log := &Log{}
	log.name = "log"
	console := NewObjectHash()
	console.values["log"] = EncodeObject(HEAP.Allocate(log))
	main.chunk.WriteConstant(EncodeObject(HEAP.Allocate(console)))
	main.chunk.EmitByte(OP_DEFINE_GLOBAL)
}

func Compile(ast *parser.Node, main *ObjFunction) error {
	scope := NewScope(nil)
	globals := &Scope{nil, 0, map[string]int{}}

	defineConsole(main, globals)

	err := traverse(ast, main, scope, globals)

	if err != nil {
		return err
	}

	main.chunk.EmitByte(OP_EOF)
	return nil
}

func traverse(current *parser.Node, fn *ObjFunction, scope *Scope, globals *Scope) error {
	isMain := fn.name == MAIN_FN_NAME

	switch current.Type {
	case parser.NODE_PROGRAM:
		{
			for _, statement := range current.Body {
				traverse(statement, fn, scope, globals)
			}
		}
	case parser.NODE_EXPRESSION_STATEMENT:
		{
			traverse(current.Expression, fn, scope, globals)
		}
	case parser.NODE_BLOCK_STATEMENT:
		{
			scope := NewScope(scope)
			for _, stmt := range current.Body {
				traverse(stmt, fn, scope, globals)
			}
		}
	case parser.NODE_BINARY_EXPRESSION:
		{
			traverse(current.Left, fn, scope, globals)
			traverse(current.Right, fn, scope, globals)
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
			case parser.STRICT_EQUALS:
				fn.chunk.EmitByte(OP_STRICT_EQUALS)
			}
		}
	case parser.NODE_IF_STATEMENT:
		{
			traverse(current.Test, fn, scope, globals)
			fn.chunk.EmitBytes(OP_JUMP_IF_FALSE, 0, 0, 0, 0)

			start := len(fn.chunk.code)
			traverse(current.Consequent, fn, scope, globals)
			fn.chunk.EmitBytes(OP_JUMP, 0, 0, 0, 0)

			trueJumpStart := len(fn.chunk.code)

			jump := uint32(len(fn.chunk.code) - start)

			fn.chunk.code[start-1] = uint8(jump & math.MaxUint8)
			fn.chunk.code[start-2] = uint8((jump >> 8))
			fn.chunk.code[start-3] = uint8((jump >> 16))
			fn.chunk.code[start-4] = uint8((jump >> 24))

			if current.Alternate != nil {
				traverse(current.Alternate, fn, scope, globals)
				jump := uint32(len(fn.chunk.code) - trueJumpStart)

				fn.chunk.code[trueJumpStart-1] = uint8(jump & math.MaxUint8)
				fn.chunk.code[trueJumpStart-2] = uint8((jump >> 8))
				fn.chunk.code[trueJumpStart-3] = uint8((jump >> 16))
				fn.chunk.code[trueJumpStart-4] = uint8((jump >> 24))
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
					value := HEAP.AllocateString(raw)
					fn.chunk.WriteConstant(value)
				}
			}
		}
	case parser.NODE_VARIABLE_DECLARATION:
		{
			for _, declaration := range current.Declarations {
				traverse(declaration, fn, scope, globals)
			}
		}
	case parser.NODE_VARIABLE_DECLARATOR:
		{
			traverse(current.Initializer, fn, scope, globals)
			if fn.name == MAIN_FN_NAME {
				fn.chunk.EmitByte(OP_DEFINE_GLOBAL)
				globals.resolver[current.Identifier.Name] = globals.count
				globals.count++
			} else {
				fn.chunk.EmitByte(OP_DEFINE_LOCAL)
				scope.resolver[current.Identifier.Name] = scope.count
				scope.count++
			}
		}
	case parser.NODE_IDENTIFIER:
		{
			found, slot := scope.findSlot(current.Name)

			if found {
				fn.chunk.EmitBytes(OP_GET_LOCAL, slot)
				return nil
			}

			found, slot = globals.findSlot(current.Name)

			if found {
				fn.chunk.EmitBytes(OP_GET_GLOBAL, slot)
				return nil
			}

			fn.chunk.EmitByte(OP_PUSH_UNDEFINED)
		}
	case parser.NODE_OBJECT_EXPRESSION:
		{
			hash := NewObjectHash()
			register := HEAP.Allocate(hash)

			for _, prop := range current.Properties {
				key := prop.Key.Name
				valueNode := prop.Value.(*parser.Node)

				switch valueNode.Type {
				case parser.NODE_LITERAL:
					{
						switch v := valueNode.Value.(type) {
						case float64:
							{
								hash.values[key] = ValueFromFloat64(v)
							}
						case []byte:
							{
								raw := string(v)
								value := HEAP.AllocateString(raw)
								hash.values[key] = value
							}
						}
					}
				}
			}

			fn.chunk.WriteConstant(EncodeObject(register))
		}
	case parser.NODE_MEMBER_EXPRESSION:
		{
			prop := fn.chunk.addConstant(HEAP.AllocateString(current.Property.Name))
			found, slot := scope.findSlot(current.Object.Name)

			if found {
				fn.chunk.EmitBytes(OP_GET_LOCAL_OBJECT_MEMBER, slot, prop)
				return nil
			}

			found, slot = globals.findSlot(current.Object.Name)

			if found {
				fn.chunk.EmitBytes(OP_GET_GLOBAL_OBJECT_MEMBER, slot, prop)
				return nil
			}

			fn.chunk.EmitByte(OP_PUSH_UNDEFINED)
		}
	case parser.NODE_FUNCTION_DECLARATION:
		{
			function := NewFunction(current.Identifier.Name, len(current.Params))
			register := HEAP.Allocate(function)

			if isMain {
				globals.resolver[function.name] = globals.count
				globals.count++
			} else {
				scope.resolver[function.name] = scope.count
				scope.count++
			}

			scope := NewScope(scope)
			fn.chunk.WriteConstant(EncodeObject(register))

			if isMain {
				fn.chunk.EmitByte(OP_DEFINE_GLOBAL)
			} else {
				fn.chunk.EmitByte(OP_DEFINE_LOCAL)
			}

			for _, param := range current.Params {
				scope.resolver[param.Name] = scope.count
				scope.count++
			}

			for _, statement := range current.BodyNode.Body {
				traverse(statement, function, scope, globals)
			}
			function.chunk.EmitByte(OP_RETURN)
		}
		// For now just gonna treat them as the same, once we start binding 'this', etc... need to separate the implementations
	case parser.NODE_FUNCTION_EXPRESSION, parser.NODE_ARROW_FUNCTION_EXPRESSION:
		{
			name := "ANONYMOUS_FN_" + strconv.Itoa(scope.count)
			function := NewFunction(name, len(current.Params))
			register := HEAP.Allocate(function)
			scope.resolver[name] = int(register)
			scope.count++

			scope := NewScope(scope)
			fn.chunk.WriteConstant(EncodeObject(register))

			for _, param := range current.Params {
				scope.resolver[param.Name] = scope.count
				scope.count++
			}

			if current.IsExpression {
				traverse(current.BodyNode, function, scope, globals)
				function.chunk.EmitByte(OP_RETURN)
			} else {
				for _, statement := range current.BodyNode.Body {
					traverse(statement, function, scope, globals)
				}
			}

		}
	case parser.NODE_CALL_EXPRESSION:
		{
			for _, arg := range current.Arguments {
				traverse(arg, fn, scope, globals)
			}

			if current.Callee.Type == parser.NODE_IDENTIFIER {
				if slot, found := scope.resolver[current.Callee.Name]; found {
					fn.chunk.EmitByte(OP_GET_LOCAL)
					fn.chunk.EmitByte(uint8(slot))
				} else if global, found := globals.resolver[current.Callee.Name]; found {
					fn.chunk.EmitByte(OP_GET_GLOBAL)
					fn.chunk.EmitByte(uint8(global))
				}
			} else {
				traverse(current.Callee, fn, scope, globals)

			}
			fn.chunk.EmitByte(OP_CALL)
		}
	case parser.NODE_RETURN_STATEMENT:
		{
			traverse(current.Argument, fn, scope, globals)
			fn.chunk.EmitByte(OP_RETURN)
		}
	}
	return nil
}
