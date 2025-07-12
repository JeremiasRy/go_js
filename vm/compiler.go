package vm

import (
	"fmt"
	"go_js/parser"
	"math"
	"strconv"
)

type Function interface {
	Name() string
	WriteConstant(v Value)
	AddConstant(v Value) uint8
	EmitByte(op uint8)
	EmitBytes(op ...uint8)
	Code() []uint8
	IsClosure() bool
	ToClosure() *ObjClosure
}

type Scope struct {
	parent        *Scope
	localsCount   int
	locals        map[string]*Variable
	upvaluesCount int
	upvalues      map[string]*Upvalue
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
	slot     int
	kind     VarKind
	captured bool
}

type Upvalue struct {
	local bool
	slot  int
}

func (s *Scope) addVariable(name string, kind VarKind) {
	slot := s.localsCount
	s.localsCount++
	s.locals[name] = &Variable{slot, kind, false}
}

func (s *Scope) addUpValue(name string, local bool) uint8 {
	slot := s.upvaluesCount
	s.upvaluesCount++
	s.upvalues[name] = &Upvalue{local, slot}
	return uint8(slot)
}

func resolveUpvalue(scope *Scope, name string) (uint8, bool) {
	if scope.parent == nil {
		return 0, false
	}
	if variable, found := scope.parent.locals[name]; found {
		variable.captured = true

		return scope.addUpValue(name, true), true
	}

	_, found := resolveUpvalue(scope.parent, name)

	if found {
		return scope.addUpValue(name, false), true
	}

	return 0, false
}

// returns: op, arg, found
func getVariable(name string, scope *Scope, globals *Scope) (uint8, uint8, bool) {

	var op, arg uint8
	if scope == nil {
		if variable, found := globals.locals[name]; found {
			op = OP_GET_GLOBAL
			arg = uint8(variable.slot)
			return op, arg, true
		}
		return 0, 0, false
	}

	resolved := false

	if variable, found := scope.locals[name]; found {
		op = OP_GET_LOCAL
		arg = uint8(variable.slot)
		resolved = true
	} else if arg, found = resolveUpvalue(scope, name); found {
		op = OP_GET_UPVALUE
		resolved = true
	}

	if !resolved {
		if variable, found := globals.locals[name]; found {
			op = OP_GET_GLOBAL
			arg = uint8(variable.slot)
		}
	}

	return op, arg, true
}

func setVariable(name string, scope *Scope, globals *Scope) (uint8, uint8, bool) {

	var op, arg uint8
	if scope == nil {
		if variable, found := globals.locals[name]; found {
			op = OP_SET_GLOBAL
			arg = uint8(variable.slot)
			return op, arg, true
		}
		return 0, 0, false
	}

	resolved := false

	if variable, found := scope.locals[name]; found {
		op = OP_SET_LOCAL
		arg = uint8(variable.slot)
		resolved = true
	} else if arg, found = resolveUpvalue(scope, name); found {
		op = OP_SET_UPVALUE
		resolved = true
	}

	if !resolved {
		if variable, found := globals.locals[name]; found {
			op = OP_SET_GLOBAL
			arg = uint8(variable.slot)
		}
	}

	return op, arg, true
}

func NewScope(parent *Scope) *Scope {
	return &Scope{parent: parent, localsCount: 0, locals: map[string]*Variable{}, upvaluesCount: 0, upvalues: map[string]*Upvalue{}}
}

func defineConsole(main Function, globals *Scope) {
	globals.addVariable("console", VAR_NATIVE)

	console := NewObjectHash()
	console.values["log"] = EncodeObject(HEAP.Allocate(NewLog()))
	main.WriteConstant(EncodeObject(HEAP.Allocate(console)))
	main.EmitByte(OP_DEFINE_GLOBAL)
}

func defineClock(main Function, globals *Scope) {
	globals.addVariable("clock", FN_NATIVE)

	main.WriteConstant(EncodeObject(HEAP.Allocate(NewClock())))
	main.EmitByte(OP_DEFINE_GLOBAL)
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

func Compile(ast *parser.Node, main Function) error {
	globals := NewScope(nil)

	defineConsole(main, globals)
	defineClock(main, globals)

	err := traverse(ast, main, nil, globals)

	if err != nil {
		return err
	}

	main.EmitByte(OP_EOF)
	return nil
}

func traverse(current *parser.Node, fn Function, scope *Scope, globals *Scope) error {
	isMain := fn.Name() == MAIN_FN_NAME

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
			// scope := NewScope(scope)
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
				fn.EmitByte(OP_ADD)
			case parser.MINUS:
				fn.EmitByte(OP_SUBTRACT)
			case parser.DIVIDE:
				fn.EmitByte(OP_DIVIDE)
			case parser.MULTIPLY:
				fn.EmitByte(OP_MULTIPLY)
			case parser.LESS_THAN:
				fn.EmitByte(OP_LESS_THAN)
			case parser.LESS_THAN_EQUAL:
				fn.EmitByte(OP_LESS_THAN_EQUAL)
			case parser.GREATER_THAN:
				fn.EmitByte(OP_GREATER_THAN)
			case parser.GREATER_THAN_EQUAL:
				fn.EmitByte(OP_GREATER_THAN_EQUAL)
			case parser.STRICT_EQUALS:
				fn.EmitByte(OP_STRICT_EQUALS)
			}
		}
	case parser.NODE_IF_STATEMENT:
		{
			traverse(current.Test, fn, scope, globals)
			fn.EmitBytes(OP_JUMP_IF_FALSE, 0, 0, 0, 0)

			start := len(fn.Code())
			traverse(current.Consequent, fn, scope, globals)

			trueJumpStart := len(fn.Code())
			if current.Alternate != nil {
				fn.EmitBytes(OP_JUMP, 0, 0, 0, 0)
			}

			jump := len(fn.Code())

			fn.Code()[start-1] = uint8(jump & math.MaxUint8)
			fn.Code()[start-2] = uint8((jump >> 8))
			fn.Code()[start-3] = uint8((jump >> 16))
			fn.Code()[start-4] = uint8((jump >> 24))

			if current.Alternate != nil {
				traverse(current.Alternate, fn, scope, globals)
				jump := len(fn.Code())

				fn.Code()[trueJumpStart-1] = uint8(jump & math.MaxUint8)
				fn.Code()[trueJumpStart-2] = uint8((jump >> 8))
				fn.Code()[trueJumpStart-3] = uint8((jump >> 16))
				fn.Code()[trueJumpStart-4] = uint8((jump >> 24))
			}

		}
	case parser.NODE_LITERAL:
		{
			switch current.Value.(type) {
			case float64:
				{
					fn.WriteConstant(ValueFromFloat64(current.Value.(float64)))
				}
			case []byte:
				{
					raw := string(current.Value.([]byte))
					value := HEAP.AllocateString(raw)
					fn.WriteConstant(value)
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
				if isMain {
					globals.addVariable(name, kind)
				} else {
					scope.addVariable(name, kind)
				}

				traverse(declaration.Initializer, fn, scope, globals)

				if isMain {
					fn.EmitByte(OP_DEFINE_GLOBAL)
				} else {
					fn.EmitByte(OP_DEFINE_LOCAL)
				}
			}
		}
	case parser.NODE_ASSIGNMENT_EXPRESSION:
		{
			traverse(current.Right, fn, scope, globals)
			name := current.Left.Name
			op, arg, found := setVariable(name, scope, globals)

			if !found {
				if isMain {
					fn.EmitByte(OP_DEFINE_GLOBAL)
					globals.addVariable(name, VAR_LET)
				} else {
					fn.EmitByte(OP_DEFINE_LOCAL)
					scope.addVariable(name, VAR_LET)
				}
				return nil
			}

			fn.EmitBytes(op, arg)
		}
	case parser.NODE_IDENTIFIER:
		{
			op, arg, _ := getVariable(current.Name, scope, globals)
			fn.EmitBytes(op, arg)
		}
	case parser.NODE_OBJECT_EXPRESSION:
		{
			hash := NewObjectHash()
			register := HEAP.Allocate(hash)

			fn.WriteConstant(EncodeObject(register))
			for _, prop := range current.Properties {
				traverse(prop.Value.(*parser.Node), fn, scope, globals)
				fn.WriteConstant(HEAP.AllocateString(prop.Key.Name))
				fn.EmitByte(OP_DEFINE_OBJECT_MEMBER)
			}

		}
	case parser.NODE_MEMBER_EXPRESSION:
		{
			prop := fn.AddConstant(HEAP.AllocateString(current.Property.Name))
			op, arg, found := getVariable(current.Object.Name, scope, globals)

			if !found {
				fn.EmitByte(OP_PUSH_UNDEFINED)
			}

			if op == OP_GET_LOCAL {
				op = OP_GET_LOCAL_OBJECT_MEMBER
			} else {
				op = OP_GET_GLOBAL_OBJECT_MEMBER
			} // add OP_GET_UPVALUE_OBJECT_MEMBER

			fn.EmitBytes(op, arg, prop)
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
			fn.WriteConstant(EncodeObject(register))

			if isMain {
				fn.EmitByte(OP_DEFINE_GLOBAL)
			} else {
				fn.EmitByte(OP_DEFINE_LOCAL)
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
			name := "ANONYMOUS_FN_" + strconv.Itoa(scope.localsCount)
			function := NewFunction(name, len(current.Params))
			register := HEAP.Allocate(function)

			scope.addVariable(name, FN)

			scope := NewScope(scope)
			fn.WriteConstant(EncodeObject(register))

			for _, param := range current.Params {
				scope.addVariable(param.Name, VAR_FN_ARGUMENT)
			}

			if current.IsExpression {
				traverse(current.BodyNode, function, scope, globals)
				function.chunk.EmitByte(OP_RETURN)
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
				op, arg, _ := getVariable(current.Callee.Name, scope, globals)
				fn.EmitBytes(op, arg)
			} else {
				traverse(current.Callee, fn, scope, globals)
			}
			fn.EmitByte(OP_CALL)
		}
	case parser.NODE_RETURN_STATEMENT:
		{
			if current.Argument == nil {
				fn.WriteConstant(EncodedUndefined())
			} else {
				traverse(current.Argument, fn, scope, globals)
			}
			fn.EmitByte(OP_RETURN)
		}
	case parser.NODE_WHILE_STATEMENT:
		{
			loopStart := len(fn.Code())
			traverse(current.Test, fn, scope, globals)
			fn.EmitBytes(OP_JUMP_IF_FALSE, 0, 0, 0, 0)
			start := len(fn.Code())
			traverse(current.BodyNode, fn, scope, globals)
			fn.EmitBytes(OP_JUMP, uint8(loopStart>>24), uint8(loopStart>>16), uint8(loopStart>>8), uint8(loopStart&math.MaxUint8))

			jump := len(fn.Code())

			fn.Code()[start-1] = uint8(jump & math.MaxUint8)
			fn.Code()[start-2] = uint8((jump >> 8))
			fn.Code()[start-3] = uint8((jump >> 16))
			fn.Code()[start-4] = uint8((jump >> 24))
		}
	case parser.NODE_FOR_STATEMENT:
		{
			traverse(current.Initializer, fn, scope, globals)

			testStart := len(fn.Code())
			traverse(current.Test, fn, scope, globals)
			fn.EmitBytes(OP_JUMP_IF_FALSE, 0, 0, 0, 0)
			conditionJump := len(fn.Code())

			fn.EmitBytes(OP_JUMP, 0, 0, 0, 0)
			bodyJump := len(fn.Code())

			traverse(current.Update, fn, scope, globals)
			fn.EmitBytes(OP_POP, OP_JUMP, 0, 0, 0, 0)
			bodyStart := len(fn.Code())

			traverse(current.BodyNode, fn, scope, globals)
			fn.EmitBytes(OP_JUMP, 0, 0, 0, 0)
			end := len(fn.Code())

			fn.Code()[conditionJump-1] = uint8(end & math.MaxUint8)
			fn.Code()[conditionJump-2] = uint8(end >> 8)
			fn.Code()[conditionJump-3] = uint8(end >> 16)
			fn.Code()[conditionJump-4] = uint8(end >> 24)

			fn.Code()[bodyJump-1] = uint8(bodyStart & math.MaxUint8)
			fn.Code()[bodyJump-2] = uint8(bodyStart >> 8)
			fn.Code()[bodyJump-3] = uint8(bodyStart >> 16)
			fn.Code()[bodyJump-4] = uint8(bodyStart >> 24)

			fn.Code()[end-1] = uint8(bodyJump & math.MaxUint8)
			fn.Code()[end-2] = uint8(bodyJump >> 8)
			fn.Code()[end-3] = uint8(bodyJump >> 16)
			fn.Code()[end-4] = uint8(bodyJump >> 24)

			fn.Code()[bodyStart-1] = uint8(testStart & math.MaxUint8)
			fn.Code()[bodyStart-2] = uint8(testStart >> 8)
			fn.Code()[bodyStart-3] = uint8(testStart >> 16)
			fn.Code()[bodyStart-4] = uint8(testStart >> 24)
		}
	case parser.NODE_UPDATE_EXPRESSION:
		{
			name := current.Argument.Name
			opSet, arg, found := setVariable(name, scope, globals)
			opGet, _, _ := getVariable(name, scope, globals)

			if !found {
				panic("oh no")
			}

			if !current.Prefix {
				fn.EmitBytes(opGet, arg)
			}

			traverse(current.Argument, fn, scope, globals)
			fn.WriteConstant(ValueFromFloat64(1))
			switch current.UpdateOperator {
			case "++":
				fn.EmitByte(OP_ADD)
			case "--":
				fn.EmitByte(OP_SUBTRACT)
			}

			fn.EmitBytes(opSet, arg)

			if current.Prefix {
				fn.EmitBytes(opGet, arg)
			}

		}
	}

	return nil
}
