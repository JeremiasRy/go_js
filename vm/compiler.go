package vm

import (
	"fmt"
	"go_js/parser"
	"math"
	"strconv"
)

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

var FUNCTION_SCOPES = map[string]*Scope{}

func (s *Scope) addVariable(name string, kind VarKind) uint8 {
	slot := s.localsCount
	s.localsCount++
	s.locals[name] = &Variable{slot, kind, false}
	return uint8(slot)
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

// returns: op, arg, found
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
	globals := NewScope(nil)

	defineConsole(main, globals)
	defineClock(main, globals)

	err := traverse(ast, main, nil, globals)

	if err != nil {
		return err
	}

	main.chunk.EmitByte(OP_EOF)
	return nil
}

func traverse(current *parser.Node, fn *ObjFunction, scope *Scope, globals *Scope) error {
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
			switch val := current.Value.(type) {
			case float64:
				{
					fn.chunk.WriteConstant(ValueFromFloat64(val))
				}
			case []byte:
				{
					raw := string(val)
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
				if isMain {
					globals.addVariable(name, kind)
				} else {
					scope.addVariable(name, kind)
				}

				traverse(declaration.Initializer, fn, scope, globals)

				if isMain {
					fn.chunk.EmitByte(OP_DEFINE_GLOBAL)
				} else {
					fn.chunk.EmitByte(OP_DEFINE_LOCAL)
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
					fn.chunk.EmitByte(OP_DEFINE_GLOBAL)
					globals.addVariable(name, VAR_LET)
				} else {
					fn.chunk.EmitByte(OP_DEFINE_LOCAL)
					scope.addVariable(name, VAR_LET)
				}
				return nil
			}

			fn.chunk.EmitBytes(op, arg)
		}
	case parser.NODE_IDENTIFIER:
		{
			op, arg, _ := getVariable(current.Name, scope, globals)
			fn.chunk.EmitBytes(op, arg)

			if op == OP_GET_UPVALUE {
				fn.isClosure = true
			}
		}
	case parser.NODE_OBJECT_EXPRESSION:
		{
			hash := NewObjectHash()
			register := HEAP.Allocate(hash)

			fn.chunk.WriteConstant(EncodeObject(register))
			for _, prop := range current.Properties {
				traverse(prop.Value.(*parser.Node), fn, scope, globals)
				fn.chunk.WriteConstant(HEAP.AllocateString(prop.Key.Name))
				fn.chunk.EmitByte(OP_DEFINE_OBJECT_MEMBER)
			}

		}
	case parser.NODE_MEMBER_EXPRESSION:
		{
			prop := fn.chunk.addConstant(HEAP.AllocateString(current.Property.Name))
			op, arg, found := getVariable(current.Object.Name, scope, globals)

			if !found {
				fn.chunk.EmitByte(OP_PUSH_UNDEFINED)
			}

			if op == OP_GET_LOCAL {
				op = OP_GET_LOCAL_OBJECT_MEMBER
			} else {
				op = OP_GET_GLOBAL_OBJECT_MEMBER
			} // add OP_GET_UPVALUE_OBJECT_MEMBER

			fn.chunk.EmitBytes(op, arg, prop)
		}
	case parser.NODE_FUNCTION_DECLARATION:
		{
			start := len(fn.chunk.code)
			function := NewFunction(current.Identifier.Name, len(current.Params))
			register := HEAP.Allocate(function)

			if isMain {
				globals.addVariable(function.name, FN)
			} else {
				scope.addVariable(function.name, FN)
			}

			scope := NewScope(scope)
			FUNCTION_SCOPES[current.Identifier.Name] = scope

			slot := fn.chunk.WriteConstant(EncodeObject(register))

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

			if function.isClosure {
				fn.chunk.code = append(fn.chunk.code[:start], append([]uint8{OP_CONSTANT, slot, OP_CLOSURE}, fn.chunk.code[start:]...)...)
			}
		}
		// For now just gonna treat them as the same, once we start binding 'this', etc... need to separate the implementations
	case parser.NODE_FUNCTION_EXPRESSION, parser.NODE_ARROW_FUNCTION_EXPRESSION:
		{
			start := len(fn.chunk.code)

			name := "ANONYMOUS_FN_" + strconv.Itoa(scope.localsCount)
			println(name)

			function := NewFunction(name, len(current.Params))
			register := HEAP.Allocate(function)

			scope.addVariable(name, FN)

			scope := NewScope(scope)
			FUNCTION_SCOPES[name] = scope
			slot := fn.chunk.WriteConstant(EncodeObject(register))

			for _, param := range current.Params {
				scope.addVariable(param.Name, VAR_FN_ARGUMENT)
			}

			if current.IsExpression {
				println("is expression")
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

			if function.isClosure {
				closureChunk := []uint8{OP_CONSTANT, slot, OP_CLOSURE}
				scope := FUNCTION_SCOPES[fn.name]

				fmt.Printf("%v\n", scope)

				closureChunk = append(closureChunk, uint8(scope.upvaluesCount))

				for _, up := range scope.upvalues {
					if up.local {
						closureChunk = append(closureChunk, 1)
					} else {
						closureChunk = append(closureChunk, 0)
					}
					closureChunk = append(closureChunk, uint8(up.slot))
				}

				fn.chunk.code = append(fn.chunk.code[:start], append(closureChunk, fn.chunk.code[start:]...)...)
			}
		}
	case parser.NODE_CALL_EXPRESSION:
		{
			for _, arg := range current.Arguments {
				traverse(arg, fn, scope, globals)
			}

			if current.Callee.Type == parser.NODE_IDENTIFIER {
				op, arg, _ := getVariable(current.Callee.Name, scope, globals)
				fn.chunk.EmitBytes(op, arg)
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
			name := current.Argument.Name
			opSet, arg, found := setVariable(name, scope, globals)
			opGet, _, _ := getVariable(name, scope, globals)

			if opGet == OP_GET_UPVALUE {
				fn.isClosure = true
			}

			if !found {
				panic("oh no")
			}

			if !current.Prefix {
				fn.chunk.EmitBytes(opGet, arg)
			}

			fn.chunk.EmitBytes(opGet, arg)
			fn.chunk.WriteConstant(ValueFromFloat64(1))
			switch current.UpdateOperator {
			case "++":
				fn.chunk.EmitByte(OP_ADD)
			case "--":
				fn.chunk.EmitByte(OP_SUBTRACT)
			}

			fn.chunk.EmitBytes(opSet, arg)

			if current.Prefix {
				fn.chunk.EmitBytes(opGet, arg)
			}

		}
	}

	return nil
}
