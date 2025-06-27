package vm

import (
	"fmt"
	"go_js/parser"
	"math"
	"strconv"
)

type Scope struct {
	parent   *Scope
	count    int
	resolver map[string]*Variable
}

type VarKind int

const (
	VAR_CONST VarKind = iota
	VAR_LET
	VAR_NATIVE
	VAR_FN_ARGUMENT
	FN_NATIVE
	FN
)

type Variable struct {
	slot int
	kind VarKind
}

func (s *Scope) addVariable(name string, kind VarKind) {
	slot := s.count
	s.count++
	s.resolver[name] = &Variable{slot, kind}
}

func (s *Scope) findVariable(name string) (bool, *Variable) {
	current := s

	for current != nil {
		if variable, found := current.resolver[name]; found {
			return true, variable
		}
		current = current.parent
	}
	return false, nil
}

func NewScope(parent *Scope) *Scope {
	return &Scope{parent: parent, count: 0, resolver: map[string]*Variable{}}
}

func defineConsole(main *ObjFunction, globals *Scope) {
	globals.addVariable("console", VAR_NATIVE)

	console := NewObjectHash()
	console.values["log"] = EncodeObject(HEAP.Allocate(NewLog()))
	main.chunk.WriteConstant(EncodeObject(HEAP.Allocate(console)))
	main.chunk.EmitByte(OP_DEFINE_GLOBAL)
}

func defineClock(main *ObjFunction, globals *Scope) {
	globals.addVariable("clock", FN_NATIVE)

	main.chunk.WriteConstant(EncodeObject(HEAP.Allocate(NewClock())))
	main.chunk.EmitByte(OP_DEFINE_GLOBAL)
}

func assertKind(k parser.Kind) (VarKind, error) {
	switch k {
	case parser.KIND_DECLARATION_CONST:
		return VAR_CONST, nil
	case parser.KIND_DECLARATION_LET:
		return VAR_LET, nil
	default:
		return 0, fmt.Errorf("not supported variable kind: %d", k)
	}
}

func Compile(ast *parser.Node, main *ObjFunction) error {
	scope := NewScope(nil)
	globals := NewScope(nil)

	defineConsole(main, globals)
	defineClock(main, globals)

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

			trueJumpStart := len(fn.chunk.code)
			if current.Alternate != nil {
				fn.chunk.EmitBytes(OP_JUMP, 0, 0, 0, 0)
			}

			jump := len(fn.chunk.code)

			fn.chunk.code[start-1] = uint8(jump & math.MaxUint8)
			fn.chunk.code[start-2] = uint8((jump >> 8))
			fn.chunk.code[start-3] = uint8((jump >> 16))
			fn.chunk.code[start-4] = uint8((jump >> 24))

			if current.Alternate != nil {
				traverse(current.Alternate, fn, scope, globals)
				jump := len(fn.chunk.code)

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
				kind, err := assertKind(current.Kind)

				if err != nil {
					panic(err.Error())
				}

				name := declaration.Identifier.Name

				traverse(declaration.Initializer, fn, scope, globals)

				if isMain {
					fn.chunk.EmitByte(OP_DEFINE_GLOBAL)
					globals.addVariable(name, kind)
				} else {
					fn.chunk.EmitByte(OP_DEFINE_LOCAL)
					scope.addVariable(name, kind)
				}
			}
		}
	case parser.NODE_ASSIGNMENT_EXPRESSION:
		{
			traverse(current.Right, fn, scope, globals)
			name := current.Left.Name
			var op, slot uint8 = 0, 0

			if found, variable := scope.findVariable(name); found {
				op = OP_SET_LOCAL
				slot = uint8(variable.slot)
			} else if found, variable := globals.findVariable(name); found {
				op = OP_SET_GLOBAL
				slot = uint8(variable.slot)
			} else {
				if isMain {
					fn.chunk.EmitByte(OP_DEFINE_GLOBAL)
					globals.addVariable(name, VAR_LET)
				} else {
					fn.chunk.EmitByte(OP_DEFINE_LOCAL)
					scope.addVariable(name, VAR_LET)
				}
				return nil
			}

			fn.chunk.EmitBytes(op, slot)
		}
	case parser.NODE_IDENTIFIER:
		{
			var get, slot uint8 = 0, 0

			if found, v := scope.findVariable(current.Name); found {
				get = OP_GET_LOCAL
				slot = uint8(v.slot)
			} else if found, v = globals.findVariable(current.Name); found {
				get = OP_GET_GLOBAL
				slot = uint8(v.slot)
			} else {
				fn.chunk.EmitByte(OP_PUSH_UNDEFINED)
				return nil
			}

			fn.chunk.EmitBytes(get, slot)
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
			if found, variable := scope.findVariable(current.Object.Name); found {
				fn.chunk.EmitBytes(OP_GET_LOCAL_OBJECT_MEMBER, uint8(variable.slot), prop)
			} else if found, variable = globals.findVariable(current.Object.Name); found {
				fn.chunk.EmitBytes(OP_GET_GLOBAL_OBJECT_MEMBER, uint8(variable.slot), prop)
			} else {
				fn.chunk.EmitByte(OP_PUSH_UNDEFINED)
			}
		}
	case parser.NODE_FUNCTION_DECLARATION:
		{
			function := NewFunction(current.Identifier.Name, len(current.Params))
			register := HEAP.Allocate(function)

			if isMain {
				globals.addVariable(function.name, FN)
			} else {
				scope.addVariable(function.name, FN)
			}

			scope := NewScope(scope)
			fn.chunk.WriteConstant(EncodeObject(register))

			if isMain {
				fn.chunk.EmitByte(OP_DEFINE_GLOBAL)
			} else {
				fn.chunk.EmitByte(OP_DEFINE_LOCAL)
			}

			for _, param := range current.Params {
				scope.addVariable(param.Name, VAR_FN_ARGUMENT)
			}

			for _, statement := range current.BodyNode.Body {
				traverse(statement, function, scope, globals)
			}

			if function.chunk.code[len(function.chunk.code)-1] != OP_RETURN {
				function.chunk.WriteConstant(EncodedUndefined())
				function.chunk.EmitByte(OP_RETURN)
			}
		}
		// For now just gonna treat them as the same, once we start binding 'this', etc... need to separate the implementations
	case parser.NODE_FUNCTION_EXPRESSION, parser.NODE_ARROW_FUNCTION_EXPRESSION:
		{
			name := "ANONYMOUS_FN_" + strconv.Itoa(scope.count)
			function := NewFunction(name, len(current.Params))
			register := HEAP.Allocate(function)

			scope.addVariable(name, FN)

			scope := NewScope(scope)
			fn.chunk.WriteConstant(EncodeObject(register))

			for _, param := range current.Params {
				scope.addVariable(param.Name, VAR_FN_ARGUMENT)
			}

			if current.IsExpression {
				traverse(current.BodyNode, function, scope, globals)
			} else {
				for _, statement := range current.BodyNode.Body {
					traverse(statement, function, scope, globals)
				}
			}
			if function.chunk.code[len(function.chunk.code)-1] != OP_RETURN {
				function.chunk.WriteConstant(EncodedUndefined())
				function.chunk.EmitByte(OP_RETURN)
			}
		}
	case parser.NODE_CALL_EXPRESSION:
		{
			for _, arg := range current.Arguments {
				traverse(arg, fn, scope, globals)
			}

			if current.Callee.Type == parser.NODE_IDENTIFIER {
				if found, variable := scope.findVariable(current.Callee.Name); found {
					fn.chunk.EmitBytes(OP_GET_LOCAL, uint8(variable.slot))
				} else if found, variable := globals.findVariable(current.Callee.Name); found {
					fn.chunk.EmitBytes(OP_GET_GLOBAL, uint8(variable.slot))
				}
			} else {
				traverse(current.Callee, fn, scope, globals)
			}
			fn.chunk.EmitByte(OP_CALL)
		}
	case parser.NODE_RETURN_STATEMENT:
		{
			if current.Argument == nil {
				fn.chunk.WriteConstant(EncodedUndefined())
			} else {
				traverse(current.Argument, fn, scope, globals)
			}
			fn.chunk.EmitByte(OP_RETURN)
		}
	case parser.NODE_WHILE_STATEMENT:
		{
			loopStart := len(fn.chunk.code)
			traverse(current.Test, fn, scope, globals)
			fn.chunk.EmitBytes(OP_JUMP_IF_FALSE, 0, 0, 0, 0)
			start := len(fn.chunk.code)
			traverse(current.BodyNode, fn, scope, globals)
			fn.chunk.EmitBytes(OP_JUMP, uint8(loopStart>>24), uint8(loopStart>>16), uint8(loopStart>>8), uint8(loopStart&math.MaxUint8))

			jump := len(fn.chunk.code)

			fn.chunk.code[start-1] = uint8(jump & math.MaxUint8)
			fn.chunk.code[start-2] = uint8((jump >> 8))
			fn.chunk.code[start-3] = uint8((jump >> 16))
			fn.chunk.code[start-4] = uint8((jump >> 24))
		}
	case parser.NODE_FOR_STATEMENT:
		{
			traverse(current.Initializer, fn, scope, globals)

			testStart := len(fn.chunk.code)
			traverse(current.Test, fn, scope, globals)
			fn.chunk.EmitBytes(OP_JUMP_IF_FALSE, 0, 0, 0, 0)
			conditionJump := len(fn.chunk.code)

			fn.chunk.EmitBytes(OP_JUMP, 0, 0, 0, 0)
			bodyJump := len(fn.chunk.code)

			traverse(current.Update, fn, scope, globals)
			fn.chunk.EmitBytes(OP_POP, OP_JUMP, 0, 0, 0, 0)
			bodyStart := len(fn.chunk.code)

			traverse(current.BodyNode, fn, scope, globals)
			fn.chunk.EmitBytes(OP_JUMP, 0, 0, 0, 0)
			end := len(fn.chunk.code)

			fn.chunk.code[conditionJump-1] = uint8(end & math.MaxUint8)
			fn.chunk.code[conditionJump-2] = uint8(end >> 8)
			fn.chunk.code[conditionJump-3] = uint8(end >> 16)
			fn.chunk.code[conditionJump-4] = uint8(end >> 24)

			fn.chunk.code[bodyJump-1] = uint8(bodyStart & math.MaxUint8)
			fn.chunk.code[bodyJump-2] = uint8(bodyStart >> 8)
			fn.chunk.code[bodyJump-3] = uint8(bodyStart >> 16)
			fn.chunk.code[bodyJump-4] = uint8(bodyStart >> 24)

			fn.chunk.code[end-1] = uint8(bodyJump & math.MaxUint8)
			fn.chunk.code[end-2] = uint8(bodyJump >> 8)
			fn.chunk.code[end-3] = uint8(bodyJump >> 16)
			fn.chunk.code[end-4] = uint8(bodyJump >> 24)

			fn.chunk.code[bodyStart-1] = uint8(testStart & math.MaxUint8)
			fn.chunk.code[bodyStart-2] = uint8(testStart >> 8)
			fn.chunk.code[bodyStart-3] = uint8(testStart >> 16)
			fn.chunk.code[bodyStart-4] = uint8(testStart >> 24)
		}
	case parser.NODE_UPDATE_EXPRESSION:
		{
			var variable *Variable
			isGlobal := false

			if found, v := scope.findVariable(current.Argument.Name); found {
				variable = v
			} else if found, v = globals.findVariable(current.Argument.Name); found {
				variable = v
				isGlobal = true
			} else {
				panic("this should now propagate upward")
			}

			var get, set, slot uint8 = 0, 0, uint8(variable.slot)
			if isGlobal {
				get = OP_GET_GLOBAL
				set = OP_SET_GLOBAL
			} else {
				get = OP_GET_LOCAL
				set = OP_SET_LOCAL
			}

			if !current.Prefix {
				fn.chunk.EmitBytes(get, slot)
			}

			traverse(current.Argument, fn, scope, globals)
			fn.chunk.WriteConstant(ValueFromFloat64(1))
			switch current.UpdateOperator {
			case "++":
				fn.chunk.EmitByte(OP_ADD)
			case "--":
				fn.chunk.EmitByte(OP_SUBTRACT)
			}

			fn.chunk.EmitBytes(set, slot)

			if current.Prefix {
				fn.chunk.EmitBytes(get, slot)
			}

		}
	}

	return nil
}
