package compiler

import (
	"cmp"
	"fmt"
	"math"
	"strconv"

	"go_js/chunk"
	"go_js/flags"
	"go_js/heap"
	"go_js/native"
	"go_js/object"
	"go_js/parser"
	"go_js/value"
	"log"
	"slices"
)

const CONSOLE_OBJECT_NAME string = "console"
const RESERVED_ARGUMENTS string = "arguments"
const UNDEFINED_IDENTIFIER string = "undefined"
const SET_TIMEOUT_NAME string = "setTimeout"

type VariableType uint8

const (
	CONST VariableType = iota
	LET
	FUNCTION
	METHOD
	FOR
	CATCH_PARAM
)

type VariableScope uint8

const (
	HEAP VariableScope = iota
	LOCAL
	GLOBAL
	THIS
)

const INITIAL_SETUP_AST_ID = -2

type Variable struct {
	scope      VariableScope
	type_      VariableType
	slot       int  // for locals and globals
	init       bool // used in for of loops
	declared   bool
	undeclared bool // used for undeclared variables i.e in assignments to unknown variable {e = 2}
	fn         object.Callable
	allocated  bool // for imported functions
	catchId    int
}

var ThisVariable *Variable = &Variable{scope: THIS}

type Variables map[string]*Variable
type BlockScope struct {
	parent *BlockScope
	vars   Variables
}

type FunctionScope struct {
	parent           *FunctionScope
	tableScope       VariableScope
	vars             Variables
	varOrder         []*Variable
	block            *BlockScope
	arity            int
	hasRestParameter bool
	needsArguments   bool
	heapVarCount     int
}

const (
	ITERATOR_FOR_OF uint8 = iota
	ITERATOR_FOR_IN
)

var BLOCK_SCOPES = map[*parser.Node]*BlockScope{}
var FUNCTION_SCOPES = map[*parser.Node]*FunctionScope{}

var operatorMap = map[parser.BinaryOperator]uint8{
	parser.LESS_THAN:          chunk.OP_LESS_THAN,
	parser.GREATER_THAN:       chunk.OP_GREATER_THAN,
	parser.GREATER_THAN_EQUAL: chunk.OP_GREATER_THAN_EQUAL,
	parser.LESS_THAN_EQUAL:    chunk.OP_LESS_THAN_EQUAL,
	parser.MINUS:              chunk.OP_SUBTRACT,
	parser.PLUS:               chunk.OP_ADD,
	parser.DIVIDE:             chunk.OP_DIVIDE,
	parser.MULTIPLY:           chunk.OP_MULTIPLY,
	parser.EXPONENTIATION:     chunk.OP_EXPONENTIATION,
	parser.MODULUS:            chunk.OP_MODULO,
	parser.EQUALS:             chunk.OP_EQUALS,
	parser.NOT_EQUALS:         chunk.OP_NOT_EQUALS,
	parser.STRICT_EQUALS:      chunk.OP_STRICT_EQUALS,
	parser.STRICT_NOT_EQUALS:  chunk.OP_STRICT_NOT_EQUALS,
	"||":                      chunk.OP_LOGICAL_OR,
	"&&":                      chunk.OP_LOGICAL_AND,
	parser.IN:                 chunk.OP_IN,
	"??":                      chunk.OP_NULL_COALESHING,
	parser.BITWISE_AND:        chunk.OP_B_AND,
	parser.BITWISE_XOR:        chunk.OP_B_XOR,
	parser.BITWISE_OR:         chunk.OP_B_OR,
	parser.LEFT_SHIFT:         chunk.OP_LEFT_SHIFT,
	parser.RIGHT_SHIFT:        chunk.OP_RIGHT_SHIFT,
	parser.UNSIGNED_RIGHT_SHIFT: chunk.OP_UNSIGNED_RIGHT_SHIFT,
	parser.INSTANCEOF:           chunk.OP_INSTANCEOF,
}

var unaryOperatorMap = map[parser.UnaryOperator]uint8{
	parser.UNARY_NEGATE: chunk.OP_NEGATE,
	parser.UNARY_NOT:    chunk.OP_NOT,
}

const (
	ITERATOR_VAR_NAME = "__gojs_iterator"
)

var ROOT_SCRIPT_LOCATION string
var imports = make(map[string]*parser.Node)

const (
	logicalAnd        parser.BinaryOperator = "&&"
	logicalOr         parser.BinaryOperator = "||"
	nullishCoalescing parser.BinaryOperator = "??"
)

type BreakPointType uint8

const (
	BREAKPOINT_CONTINUE BreakPointType = iota
	BREAKPOINT_BREAK
)

type LoopBreakPoint struct {
	position int
	type_    BreakPointType
}

type Loop struct {
	points       []*LoopBreakPoint
	block        *BlockScope
	label        string
	isSwitch     bool
	iteratorSlot int
}

var loopStack = []*Loop{}
var finalizerStack = []*parser.Node{}
var chainJumpStack = [][]int{}
var pendingLoopLabel = ""
var wantMethodHandle = false

func currentLoop() *Loop {
	if len(loopStack) == 0 {
		return nil
	}
	return loopStack[len(loopStack)-1]
}

func findLoop(label string, breakAllowed bool) *Loop {
	if label == "" {
		for i := len(loopStack) - 1; i >= 0; i-- {
			if breakAllowed || !loopStack[i].isSwitch {
				return loopStack[i]
			}
		}
		return nil
	}
	for i := len(loopStack) - 1; i >= 0; i-- {
		if loopStack[i].label == label {
			return loopStack[i]
		}
	}
	return nil
}

func pushLoop(block *BlockScope) {
	loopStack = append(loopStack, &Loop{points: []*LoopBreakPoint{}, block: block, label: pendingLoopLabel, iteratorSlot: -1})
	pendingLoopLabel = ""
}

func pushLabeledLoop(block *BlockScope, label string) {
	loopStack = append(loopStack, &Loop{points: []*LoopBreakPoint{}, block: block, label: label, iteratorSlot: -1})
}

func pushSwitchLoop(block *BlockScope) {
	loopStack = append(loopStack, &Loop{points: []*LoopBreakPoint{}, block: block, label: pendingLoopLabel, isSwitch: true, iteratorSlot: -1})
	pendingLoopLabel = ""
}

func popLoop() {
	if len(loopStack) == 0 {
		return
	}
	loopStack = loopStack[:len(loopStack)-1]
}

func emitIteratorClosesTo(fn object.Callable, target *Loop, includeTarget bool) {
	for i := len(loopStack) - 1; i >= 0; i-- {
		l := loopStack[i]
		if l == target {
			if includeTarget && l.iteratorSlot >= 0 {
				fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_CLOSE_SLOT, uint8(l.iteratorSlot))
			}
			break
		}
		if l.iteratorSlot >= 0 {
			fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_CLOSE_SLOT, uint8(l.iteratorSlot))
		}
	}
}

func emitAllIteratorCloses(fn object.Callable) {
	for i := len(loopStack) - 1; i >= 0; i-- {
		if loopStack[i].iteratorSlot >= 0 {
			fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_CLOSE_SLOT, uint8(loopStack[i].iteratorSlot))
		}
	}
}

func emitAbruptFinalizers(symbolTable *FunctionScope, fn object.Callable) {
	for i := len(finalizerStack) - 1; i >= 0; i-- {
		finalizer := finalizerStack[i]
		saved := finalizerStack
		finalizerStack = finalizerStack[:i]
		if block, found := BLOCK_SCOPES[finalizer]; found {
			oldParent := block.parent
			block.parent = symbolTable.block
			generateByteCode(finalizer, symbolTable, fn)
			block.parent = oldParent
		} else {
			generateByteCode(finalizer, symbolTable, fn)
		}
		finalizerStack = saved
	}
}

func InitRootScriptLocation(src string) {
	ROOT_SCRIPT_LOCATION = src
}
func newBlockScope(parent *BlockScope) *BlockScope {
	return &BlockScope{vars: Variables{}, parent: parent}
}

func newFunctionScope(parent *FunctionScope, tableScope VariableScope) *FunctionScope {
	return &FunctionScope{parent: parent, tableScope: tableScope, vars: Variables{}, block: nil}
}

func (fs *FunctionScope) addArgumentsLocalToFunctionScope() {
	if _, found := fs.vars[RESERVED_ARGUMENTS]; found {
		return
	}
	variable := &Variable{scope: LOCAL, type_: CONST, slot: -1, init: false, declared: true, undeclared: false, fn: nil}
	fs.vars[RESERVED_ARGUMENTS] = variable
	for _, variable := range fs.vars {
		if variable.slot >= fs.arity {
			variable.slot++
		}
	}
	for block := fs.block; block != nil; block = block.parent {
		for _, variable := range block.vars {
			if variable.slot >= fs.arity {
				variable.slot++
			}
		}
	}
	fs.vars[RESERVED_ARGUMENTS].slot = fs.arity
	fs.needsArguments = true
}

func (fs *FunctionScope) isInHeapScope() bool {
	current := fs.parent

	for current != nil {
		for _, v := range current.vars {
			if v.scope == HEAP {
				return true
			}
		}
		current = current.parent
	}
	return false
}

func (fs *FunctionScope) enterBlockScope(node *parser.Node) {
	if b, found := BLOCK_SCOPES[node]; found {
		fs.block = b
	} else {
		panic("no block scope found for ast node")
	}
}

func (fs *FunctionScope) exitBlockScope() {
	fs.block = fs.block.parent
}

func blockSlotCount(block *BlockScope) int {
	count := 0
	for _, variable := range block.vars {
		if variable.scope != HEAP && variable.type_ != CATCH_PARAM {
			count++
		}
	}
	return count
}

func functionSlotCount(fs *FunctionScope) int {
	count := 0
	for _, variable := range fs.vars {
		if variable.scope != HEAP && variable.type_ != CATCH_PARAM {
			count++
		}
	}
	return count
}
func (fs *FunctionScope) getCurrentSlot() int {
	if fs.tableScope == GLOBAL && fs.block == nil {
		return functionSlotCount(fs) - 1
	}

	count := -1
	if fs.block != nil {
		current := fs.block

		for current != nil {
			count += blockSlotCount(current)
			current = current.parent
		}
	}

	if fs.tableScope == GLOBAL {
		return count
	}

	count += functionSlotCount(fs)
	return count
}

func (fs *FunctionScope) addVariable(name string, type_ VariableType, undeclared bool, fn object.Callable) {
	var mapToAddTo Variables

	if fs.block != nil {
		mapToAddTo = fs.block.vars
	} else {
		mapToAddTo = fs.vars
	}

	if _, found := mapToAddTo[name]; found {
		//fmt.Printf("WARN: Already found variable %s from scope\n", name)
		return
	}

	variable := &Variable{scope: fs.tableScope, type_: type_, slot: -1, fn: fn, undeclared: undeclared}
	mapToAddTo[name] = variable
	if mapToAddTo != nil && fs.block == nil {
		fs.varOrder = append(fs.varOrder, variable)
	}

	if fs.block != nil {
		variable.scope = LOCAL
	}
	variable.slot = fs.getCurrentSlot()
}

func (fs *FunctionScope) findVariable(name string) (*Variable, *FunctionScope) {
	current := fs

	for current != nil {
		block := current.block
		for block != nil {
			if variable, found := block.vars[name]; found {
				return variable, current
			}
			block = block.parent
		}

		if variable, found := current.vars[name]; found {
			return variable, current
		}

		current = current.parent
	}
	return nil, nil
}

func (fs *FunctionScope) currentBlockVarCount() (int, bool) {
	count := -1
	if fs.block != nil {
		count = len(fs.block.vars)
		return count, true
	}
	return count, false
}

func defineConsole(main *object.ObjFunction, symbolTable *FunctionScope) {
	console := native.NewObjectConsole()
	consoleHandle := heap.Allocate(console)
	symbolTable.addVariable(CONSOLE_OBJECT_NAME, CONST, false, nil)

	main.ValueChunk().WriteConstant(value.EncodeHandle(consoleHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineObjectConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	objCtor := native.NewObjectMethodConstructor()
	objCtorHandle := heap.Allocate(objCtor)

	symbolTable.addVariable(native.OBJECT_CONSTRUCTOR_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(objCtorHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineArrayConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	arrCtor := native.NewArrayConstructor()
	arrCtorHandle := heap.Allocate(arrCtor)

	symbolTable.addVariable(native.ARRAY_CONSTRUCTOR_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(arrCtorHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineErrorConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	for _, name := range []string{"Error", "TypeError", "ReferenceError", "RangeError", "SyntaxError", "AggregateError"} {
		ctor := native.NewNamedErrorConstructor(name)
		ctorHandle := heap.Allocate(ctor)

		symbolTable.addVariable(name, CONST, false, nil)
		main.ValueChunk().WriteConstant(value.EncodeHandle(ctorHandle))
		main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
	}
}

func defineSetTimeout(main *object.ObjFunction, symbolTable *FunctionScope) {
	setTimeout := native.NewSetTimeout()
	setTimeouthandle := heap.Allocate(setTimeout)

	symbolTable.addVariable(SET_TIMEOUT_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(setTimeouthandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func definePromiseConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	promiseCtor := native.NewPromiseConstructor()
	promiseCtorHandle := heap.Allocate(promiseCtor)

	symbolTable.addVariable(native.PROMISE_CONSTRUCTOR_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(promiseCtorHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineDateConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	dateCtor := native.NewDateConstructor()
	dateCtorHandle := heap.Allocate(dateCtor)

	symbolTable.addVariable(native.DATE_CONSTRUCTOR_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(dateCtorHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineMapConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	mapCtor := native.NewMapConstructor()
	mapCtorHandle := heap.Allocate(mapCtor)

	symbolTable.addVariable(native.MAP_CONSTRUCTOR_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(mapCtorHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineSetConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	setCtor := native.NewSetConstructor()
	setCtorHandle := heap.Allocate(setCtor)

	symbolTable.addVariable(native.SET_CONSTRUCTOR_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(setCtorHandle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineQueueMicroTask(main *object.ObjFunction, symbolTable *FunctionScope) {
	queueMicroTask := native.NewQueueMicroTask()
	handle := heap.Allocate(queueMicroTask)

	symbolTable.addVariable(native.QUEUE_MICRO_TASK_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(handle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineParseInt(main *object.ObjFunction, symbolTable *FunctionScope) {
	parseInt := native.NewParseInt()
	handle := heap.Allocate(parseInt)

	symbolTable.addVariable(native.PARSE_INT_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(handle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
	native.PARSE_INT_VALUE = value.EncodeHandle(handle)
}

func defineParseFloat(main *object.ObjFunction, symbolTable *FunctionScope) {
	parseFloat := native.NewParseFloat()
	handle := heap.Allocate(parseFloat)

	symbolTable.addVariable("parseFloat", CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(handle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
	native.PARSE_FLOAT_VALUE = value.EncodeHandle(handle)
}

func defineNumberConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	numberCtor := native.NewNumberConstructor()
	handle := heap.Allocate(numberCtor)

	symbolTable.addVariable(native.NUMBER_CONSTRUCTOR_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(handle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineJSONObject(main *object.ObjFunction, symbolTable *FunctionScope) {
	jsonObject := native.NewJSONObject()
	handle := heap.Allocate(jsonObject)

	symbolTable.addVariable(native.JSON_OBJECT_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(handle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineSymbolConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	symbolCtor := native.NewSymbolConstructor()
	handle := heap.Allocate(symbolCtor)

	symbolTable.addVariable(native.SYMBOL_CONSTRUCTOR_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(handle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineStringConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	stringCtor := native.NewStringConstructorObject()
	handle := heap.Allocate(stringCtor)

	symbolTable.addVariable(native.STRING_CONSTRUCTOR_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(handle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineMathObject(main *object.ObjFunction, symbolTable *FunctionScope) {
	mathObject := native.NewMathObject()
	handle := heap.Allocate(mathObject)

	symbolTable.addVariable(native.MATH_OBJECT_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(handle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

func defineBooleanConstructor(main *object.ObjFunction, symbolTable *FunctionScope) {
	booleanCtor := native.NewBooleanConstructor()
	handle := heap.Allocate(booleanCtor)

	symbolTable.addVariable(native.BOOLEAN_CONSTRUCTOR_NAME, CONST, false, nil)
	main.ValueChunk().WriteConstant(value.EncodeHandle(handle))
	main.ValueChunk().EmitByte(chunk.OP_DEFINE_GLOBAL)
}

var global *FunctionScope = newFunctionScope(nil, GLOBAL)
var module = false

type PopRValue struct {
	s []bool
}

var popReturnValue = &PopRValue{s: []bool{true}}

func (p *PopRValue) current() bool {
	return p.s[len(p.s)-1]
}

func (p *PopRValue) push(v bool) {
	p.s = append(p.s, v)
}

func (p *PopRValue) pop() {
	p.s = p.s[:len(p.s)-1]
}

func SetModule(isModule bool) {
	module = isModule
}

func Compile(ast *parser.Node) (*object.ObjFunction, error) {
	savedStrict := strictContext
	strictContext = module
	setup := object.NewFunction(object.SETUP_FN_NAME, 0, nil)
	setup.ValueChunk().SetCompTimeAstId(INITIAL_SETUP_AST_ID)
	defineObjectConstructor(setup, global)
	defineConsole(setup, global)
	defineSetTimeout(setup, global)
	defineErrorConstructor(setup, global)
	defineArrayConstructor(setup, global)
	definePromiseConstructor(setup, global)
	defineDateConstructor(setup, global)
	defineMapConstructor(setup, global)
	defineSetConstructor(setup, global)
	defineQueueMicroTask(setup, global)
	defineParseInt(setup, global)
	defineParseFloat(setup, global)
	defineNumberConstructor(setup, global)
	defineJSONObject(setup, global)
	defineSymbolConstructor(setup, global)
	defineStringConstructor(setup, global)
	defineMathObject(setup, global)
	defineBooleanConstructor(setup, global)
	for _, variable := range global.vars {
		variable.declared = true
	}

	main := object.NewFunction(object.MAIN_FN_NAME, 0, nil)

	mainValue := value.EncodeHandle(heap.Allocate(main))
	setup.ValueChunk().WriteConstant(mainValue)
	setup.ValueChunk().EmitBytes(chunk.OP_CALL, 0, 0, chunk.OP_RETURN)

	if flags.StructuredOutput {
		main.ValueChunk().SetCompTimeAstId(ast.Id)
	}
	prePass(ast, global)
	for i, variable := range global.varOrder {
		variable.slot = i
	}
	generateByteCode(ast, global, main)
	main.ValueChunk().EmitByte(chunk.OP_RETURN)
	strictContext = savedStrict

	return setup, nil
}

func CompileModule(src string) (*object.ObjFunction, error) {
	savedModule := module
	module = true
	main := object.NewFunction(fmt.Sprintf("MODULE %s", src), 0, nil)
	generateByteCode(imports[src], global, main)

	main.ValueChunk().EmitByte(chunk.OP_RETURN)
	module = savedModule
	return main, nil
}

var astTrain = []*parser.Node{}

func withAstTracking(node *parser.Node, fn object.Callable, action func()) {
	if !flags.StructuredOutput {
		action()
		return
	}

	fn.ValueChunk().SetCompTimeAstId(node.Id)
	for _, n := range astTrain {
		n.AstTrain = append(n.AstTrain, node)
	}
	astTrain = append(astTrain, node)

	action()

	arr := make([]int, 0, len(node.AstTrain))
	for _, n := range node.AstTrain {
		arr = append(arr, n.Id)
	}
	node.AstTrainIds = arr

	astTrain = astTrain[:len(astTrain)-1]
	if len(astTrain) > 0 {
		fn.ValueChunk().SetCompTimeAstId(astTrain[len(astTrain)-1].Id)
	}
}

func generateByteCode(current *parser.Node, symbolTable *FunctionScope, fn object.Callable) {
	withAstTracking(current, fn, func() {
		switch current.Type {
		case parser.NODE_PROGRAM:
			{
				type functionEntry struct {
					name     string
					variable *Variable
				}
				functions := []functionEntry{}
				for name, variable := range symbolTable.vars {
					if variable.type_ == FUNCTION {
						functions = append(functions, functionEntry{name, variable})
					}
				}

				slices.SortFunc(functions, func(a, b functionEntry) int {
					return cmp.Compare(a.variable.slot, b.variable.slot)
				})

				for _, entry := range functions {
					variable := entry.variable
					if !variable.allocated {
						fnValue := heap.Allocate(variable.fn)

						fn.ValueChunk().WriteConstant(value.EncodeHandle(fnValue))
						if module {
							fn.ValueChunk().EmitBytes(chunk.OP_SET_GLOBAL, uint8(variable.slot))
						} else {
							name := fn.ValueChunk().AddConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(entry.name))))
							fn.ValueChunk().EmitBytes(chunk.OP_SET_GLOBAL_NAMED, uint8(variable.slot), name)
						}
						variable.allocated = true
					}
				}

				for _, node := range current.Body {
					generateByteCode(node, symbolTable, fn)
				}

			}
		case parser.NODE_ASSIGNMENT_EXPRESSION:
			{
				popReturnValue.push(false)
				left := current.Left
				operator := current.AssignmentOperator

				if left.Type == parser.NODE_OBJECT_PATTERN || left.Type == parser.NODE_ARRAY_PATTERN {
					generatePatternAssignment(left, current.Right, symbolTable, fn)
					popReturnValue.pop()
					if popReturnValue.current() {
						fn.ValueChunk().EmitByte(chunk.OP_POP)
					}
					return
				}

				if left.Type == parser.NODE_MEMBER_EXPRESSION {
					generateByteCode(left.Object, symbolTable, fn)
					if left.Computed {
						generateByteCode(left.Property, symbolTable, fn)
						fn.ValueChunk().EmitByte(chunk.OP_TO_PROPERTY_KEY)
					} else {
						key := heap.Allocate(native.NewLightString(left.Property.Name))
						fn.ValueChunk().WriteConstant(value.EncodeHandle(key))
					}

					memberSetOp := chunk.OP_SET_MEMBER_RETURN
					if isStrictFunction(fn) {
						memberSetOp = chunk.OP_SET_MEMBER_STRICT_RETURN
					}

					switch operator {
					case parser.ASSIGN:
						generateByteCode(current.Right, symbolTable, fn)
						fn.ValueChunk().EmitByte(memberSetOp)
					case parser.LOGICAL_AND_ASSIGN, parser.LOGICAL_OR_ASSIGN, parser.NULLISH_ASSIGN:
						fn.ValueChunk().EmitBytes(chunk.OP_DUP2, chunk.OP_GET_OBJECT_MEMBER, chunk.OP_DUP)
						var jump uint8
						switch operator {
						case parser.LOGICAL_AND_ASSIGN:
							jump = chunk.OP_JUMP_IF_FALSE
						case parser.LOGICAL_OR_ASSIGN:
							jump = chunk.OP_JUMP_IF_TRUE
						default:
							jump = chunk.OP_JUMP_IF_NOT_NULLISH
						}
						fn.ValueChunk().EmitBytes(jump, 0, 0, 0, 0)
						skipJump := len(fn.ValueChunk().Code) - 4
						fn.ValueChunk().EmitByte(chunk.OP_POP)
						generateByteCode(current.Right, symbolTable, fn)
						fn.ValueChunk().EmitByte(memberSetOp)
						fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
						endJump := len(fn.ValueChunk().Code) - 4
						fn.ValueChunk().PatchUint32(uint32(skipJump), uint32(len(fn.ValueChunk().Code)))
						fn.ValueChunk().EmitByte(chunk.OP_POP_BELOW2)
						fn.ValueChunk().PatchUint32(uint32(endJump), uint32(len(fn.ValueChunk().Code)))
					default:
						fn.ValueChunk().EmitBytes(chunk.OP_DUP2, chunk.OP_GET_OBJECT_MEMBER)
						generateByteCode(current.Right, symbolTable, fn)
						fn.ValueChunk().EmitByte(assignmentOpcode(operator))
						fn.ValueChunk().EmitByte(memberSetOp)
					}
				} else {
					variable, owner := symbolTable.findVariable(left.Name)
					if variable == nil {
						emitThrow(fn, "ReferenceError", left.Name+" is not defined")
						popReturnValue.pop()
						return
					}
					if (variable.type_ == LET || variable.type_ == CONST) && !variable.declared && !variable.undeclared && owner == symbolTable {
						emitThrow(fn, "ReferenceError", "Cannot access '"+left.Name+"' before initialization")
						popReturnValue.pop()
						return
					}
					if variable.type_ == CONST {
						switch operator {
						case parser.LOGICAL_AND_ASSIGN, parser.LOGICAL_OR_ASSIGN, parser.NULLISH_ASSIGN:
						default:
							emitThrow(fn, "TypeError", "Assignment to constant variable.")
							popReturnValue.pop()
							return
						}
					}

					var defineOp, setOp, getOp uint8
					switch variable.scope {
					case LOCAL:
						defineOp = chunk.OP_DEFINE_LOCAL
						setOp = chunk.OP_SET_LOCAL
						getOp = chunk.OP_GET_LOCAL
					case GLOBAL:
						defineOp = chunk.OP_DEFINE_GLOBAL
						setOp = chunk.OP_SET_GLOBAL
						getOp = chunk.OP_GET_GLOBAL
					case HEAP:
						defineOp = chunk.OP_DEFINE_HEAP_VAR
						setOp = chunk.OP_SET_HEAP_VAR
						getOp = chunk.OP_GET_HEAP_VAR
					case THIS:
						getOp = chunk.OP_THIS
					}
					if variable.type_ == CATCH_PARAM {
						setOp = chunk.OP_SET_CATCH
						getOp = chunk.OP_GET_CATCH
					}

					switch operator {
					case parser.ASSIGN:
						generateByteCode(current.Right, symbolTable, fn)
						fn.ValueChunk().EmitByte(chunk.OP_DUP)
						if variable.undeclared {
							if defineOp == chunk.OP_DEFINE_GLOBAL && !module {
								nameArg := fn.ValueChunk().AddConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(left.Name))))
								fn.ValueChunk().EmitBytes(chunk.OP_DEFINE_GLOBAL_NAMED, nameArg)
							} else {
								fn.ValueChunk().EmitByte(defineOp)
							}
							variable.undeclared = false
							variable.declared = true
						} else {
							fn.ValueChunk().EmitBytes(setOp, variableOperand(variable))
						}
					case parser.LOGICAL_AND_ASSIGN, parser.LOGICAL_OR_ASSIGN, parser.NULLISH_ASSIGN:
						fn.ValueChunk().EmitBytes(getOp, variableOperand(variable), chunk.OP_DUP)
						var jump uint8
						switch operator {
						case parser.LOGICAL_AND_ASSIGN:
							jump = chunk.OP_JUMP_IF_FALSE
						case parser.LOGICAL_OR_ASSIGN:
							jump = chunk.OP_JUMP_IF_TRUE
						default:
							jump = chunk.OP_JUMP_IF_NOT_NULLISH
						}
						fn.ValueChunk().EmitBytes(jump, 0, 0, 0, 0)
						skipJump := len(fn.ValueChunk().Code) - 4
						fn.ValueChunk().EmitByte(chunk.OP_POP)
						if variable.type_ == CONST {
							emitThrow(fn, "TypeError", "Assignment to constant variable.")
						} else {
							generateByteCode(current.Right, symbolTable, fn)
							fn.ValueChunk().EmitByte(chunk.OP_DUP)
							fn.ValueChunk().EmitBytes(setOp, variableOperand(variable))
						}
						fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
						endJump := len(fn.ValueChunk().Code) - 4
						fn.ValueChunk().PatchUint32(uint32(skipJump), uint32(len(fn.ValueChunk().Code)))
						fn.ValueChunk().PatchUint32(uint32(endJump), uint32(len(fn.ValueChunk().Code)))
					default:
						fn.ValueChunk().EmitBytes(getOp, variableOperand(variable))
						generateByteCode(current.Right, symbolTable, fn)
						fn.ValueChunk().EmitByte(assignmentOpcode(operator))
						fn.ValueChunk().EmitByte(chunk.OP_DUP)
						fn.ValueChunk().EmitBytes(setOp, variableOperand(variable))
					}
				}

				popReturnValue.pop()
				if popReturnValue.current() {
					fn.ValueChunk().EmitByte(chunk.OP_POP)
				}
			}
		case parser.NODE_FUNCTION_DECLARATION:
			{
				nextFn, _ := symbolTable.findVariable(current.Identifier.Name)
				symbolTable = FUNCTION_SCOPES[current]
				markParamsDeclared(current.Params, symbolTable)
				if fnObj, ok := nextFn.fn.(*object.ObjFunction); ok {
					fnObj.Strict = module || nodeHasUseStrict(current)
				}

				fn = nextFn.fn
				isInHeapScopeAlready := symbolTable.isInHeapScope()
				if symbolTable.hasRestParameter && symbolTable.needsArguments {
					log.Fatal("Let's not do both arguments anr rest parameter")
				}

				if symbolTable.hasRestParameter {
					fn.SetHasRestParameter()
				}

				if symbolTable.needsArguments {
					fn.SetHasArguments()
				}

				functions := []*Variable{}
				for _, variable := range symbolTable.vars {
					if variable.type_ == FUNCTION {
						functions = append(functions, variable)
					}

					// if not in heap scope we'll create a new one
					if !isInHeapScopeAlready && variable.scope == HEAP {
						fn.ValueChunk().EmitByte(chunk.OP_CREATE_HEAP_SCOPE)
						isInHeapScopeAlready = true
					}
				}

				slices.SortFunc(functions, func(a *Variable, b *Variable) int {
					return cmp.Compare(a.slot, b.slot)
				})

				for _, variable := range functions {
					fnValue := heap.Allocate(variable.fn)

					slot := fn.ValueChunk().WriteConstant(value.EncodeHandle(fnValue))

					if uint8(variable.slot) != slot {
						panic("things went south")
					}

					fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)
				}

				if fn.GetArity() > 0 {
					heapVars := []int{}

					for i, n := range current.Params {
						withAstTracking(n, fn, func() {
							if v, found := symbolTable.vars[parameterName(n)]; found {
								if v.scope == HEAP {
									heapVars = append(heapVars, i)
								}
							}
						})
					}

					if len(heapVars) > 0 {
						fn.ValueChunk().EmitBytes(chunk.OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS, uint8(len(heapVars)))
						for _, slot := range heapVars {
							fn.ValueChunk().EmitByte(uint8(slot))
						}
					}
				}

				emitParameterDefaults(current.Params, symbolTable, fn)

				popReturnValue.push(true)
				withAstTracking(current.BodyNode, fn, func() {
					for _, node := range current.BodyNode.Body {
						generateByteCode(node, symbolTable, fn)
					}
				})
				popReturnValue.pop()

				if len(fn.ValueChunk().Code) == 0 {
					fn.ValueChunk().EmitBytes(chunk.OP_PUSH_UNDEFINED, chunk.OP_RETURN)
				} else if fn.ValueChunk().Code[len(fn.ValueChunk().Code)-1] != chunk.OP_RETURN {
					fn.ValueChunk().EmitBytes(chunk.OP_PUSH_UNDEFINED, chunk.OP_RETURN)
				}

			}
		case parser.NODE_ARROW_FUNCTION_EXPRESSION:
			{
				outerSymbolTable := symbolTable
				symbolTable = FUNCTION_SCOPES[current]
				markParamsDeclared(current.Params, symbolTable)
				newFn := object.NewFunction("ANONYMOYS_FN", len(current.Params), nil)
				newFn.Strict = module || nodeHasUseStrict(current)
				newFn.IsArrow = true

				if symbolTable.hasRestParameter {
					newFn.SetHasRestParameter()
				}

				if symbolTable.needsArguments {
					newFn.SetHasArguments()
				}

				handle := heap.Allocate(newFn)
				v := value.EncodeHandle(handle)

				functions := []*Variable{}
				isInHeapScopeAlready := symbolTable.isInHeapScope()
				for _, variable := range symbolTable.vars {
					if variable.type_ == FUNCTION {
						functions = append(functions, variable)
					}

					// if not in heap scope we'll create a new one
					if !isInHeapScopeAlready && variable.scope == HEAP {
						newFn.ValueChunk().EmitByte(chunk.OP_CREATE_HEAP_SCOPE)
						isInHeapScopeAlready = true
					}
				}

				slices.SortFunc(functions, func(a *Variable, b *Variable) int {
					return cmp.Compare(a.slot, b.slot)
				})

				for _, variable := range functions {
					fnValue := heap.Allocate(variable.fn)
					slot := fn.ValueChunk().WriteConstant(value.EncodeHandle(fnValue))

					if uint8(variable.slot) != slot {
						panic("things went south")
					}

					fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)
				}

				if len(current.Params) > 0 {
					heapVars := []int{}

					for i, n := range current.Params {
						withAstTracking(n, fn, func() {
							if v, found := symbolTable.vars[parameterName(n)]; found {
								if v.scope == HEAP {
									heapVars = append(heapVars, i)
								}
							}
						})
					}

					if len(heapVars) > 0 {
						newFn.ValueChunk().EmitBytes(chunk.OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS, uint8(len(heapVars)))
						for _, slot := range heapVars {
							newFn.ValueChunk().EmitByte(uint8(slot))
						}
					}
				}

				emitParameterDefaults(current.Params, symbolTable, newFn)

				if current.IsExpression {
					generateByteCode(current.BodyNode, symbolTable, newFn)
				} else {
					popReturnValue.push(true)
					for _, node := range current.BodyNode.Body {
						generateByteCode(node, symbolTable, newFn)
					}
					popReturnValue.pop()
				}

				if len(newFn.ValueChunk().Code) == 0 {
					newFn.ValueChunk().EmitBytes(chunk.OP_PUSH_UNDEFINED, chunk.OP_RETURN)
				} else if newFn.ValueChunk().Code[len(newFn.ValueChunk().Code)-1] != chunk.OP_RETURN {
					newFn.ValueChunk().EmitBytes(chunk.OP_RETURN)
				}

				symbolTable = outerSymbolTable
				fn.ValueChunk().WriteConstant(v)
				fn.ValueChunk().EmitByte(chunk.OP_CLONE_FUNCTION)
			}
		case parser.NODE_FUNCTION_EXPRESSION:
			{
				outerSymbolTable := symbolTable
				symbolTable = FUNCTION_SCOPES[current]
				markParamsDeclared(current.Params, symbolTable)
				newFn := object.NewFunction("ANONYMOYS_FN", len(current.Params), nil)
				newFn.Strict = module || nodeHasUseStrict(current)
				if symbolTable.hasRestParameter {
					newFn.SetHasRestParameter()
				}
				handle := heap.Allocate(newFn)
				v := value.EncodeHandle(handle)

				if current.IsExpression {
					generateByteCode(current.BodyNode, symbolTable, newFn)

				} else {
					popReturnValue.push(true)
					for _, node := range current.BodyNode.Body {
						generateByteCode(node, symbolTable, newFn)
					}
					popReturnValue.pop()
				}

				if len(newFn.ValueChunk().Code) == 0 {
					newFn.ValueChunk().EmitBytes(chunk.OP_PUSH_UNDEFINED, chunk.OP_RETURN)
				} else if newFn.ValueChunk().Code[len(newFn.ValueChunk().Code)-1] != chunk.OP_RETURN {
					newFn.ValueChunk().EmitBytes(chunk.OP_RETURN)
				}
				symbolTable = outerSymbolTable
				fn.ValueChunk().WriteConstant(v)
				fn.ValueChunk().EmitByte(chunk.OP_CLONE_FUNCTION)
			}
		case parser.NODE_BLOCK_STATEMENT:
			{
				symbolTable.enterBlockScope(current)
				functions := []*Variable{}
				for _, variable := range symbolTable.block.vars {
					if variable.type_ == FUNCTION {
						functions = append(functions, variable)
					}
				}
				slices.SortFunc(functions, func(a, b *Variable) int {
					return cmp.Compare(a.slot, b.slot)
				})
				for _, variable := range functions {
					fnValue := heap.Allocate(variable.fn)
					fn.ValueChunk().WriteConstant(value.EncodeHandle(fnValue))
					fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)
				}
				for _, node := range current.Body {
					generateByteCode(node, symbolTable, fn)
				}

				if symbolTable.block != nil {
					for _, variable := range symbolTable.block.vars {
						if variable.type_ != CATCH_PARAM {
							fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
						}
					}
				}
				symbolTable.exitBlockScope()
			}
		case parser.NODE_VARIABLE_DECLARATION:
			{
				popReturnValue.push(false)
				for _, declaration := range current.Declarations {
					generateByteCode(declaration, symbolTable, fn)
				}
				popReturnValue.pop()
			}

		case parser.NODE_ARRAY_EXPRESSION:
			{

				popReturnValue.push(false)
				fn.ValueChunk().EmitByte(chunk.OP_CREATE_ARRAY)
				fn.ValueChunk().EmitUint32(uint32(len(current.Elements)))

				for _, item := range current.Elements {
					if item == nil {
						fn.ValueChunk().WriteConstant(value.EMPTY_ARRAY_ITEM)
						fn.ValueChunk().EmitByte(chunk.OP_PUSH_ELEMENT)
						continue
					}
					if item.Type == parser.NODE_SPREAD_ELEMENT {
						generateByteCode(item.Argument, symbolTable, fn)
						fn.ValueChunk().EmitByte(chunk.OP_SPREAD_INTO_ARRAY)
						continue
					}
					generateByteCode(item, symbolTable, fn)
					fn.ValueChunk().EmitByte(chunk.OP_PUSH_ELEMENT)
				}
				popReturnValue.pop()
			}
		case parser.NODE_CALL_EXPRESSION:
			{
				popReturnValue.push(false)
				calleeFirst := current.Optional || containsOptional(current.Callee)
				if calleeFirst {
					savedHandle := wantMethodHandle
					wantMethodHandle = true
					generateByteCode(current.Callee, symbolTable, fn)
					wantMethodHandle = savedHandle
					if referenceIsDiscarded(current.Callee) {
						fn.ValueChunk().EmitByte(chunk.OP_UNWRAP_METHOD)
					}

					var optionalJumpStart int
					if current.Optional {
						fn.ValueChunk().EmitByte(chunk.OP_DUP)
						fn.ValueChunk().EmitBytes(chunk.OP_JUMP_IF_NULLISH, 0, 0, 0, 0)
						optionalJumpStart = len(fn.ValueChunk().Code) - 4
						if len(chainJumpStack) > 0 {
							top := len(chainJumpStack) - 1
							chainJumpStack[top] = append(chainJumpStack[top], optionalJumpStart)
						}
					}

					for _, node := range current.Arguments {
						generateByteCode(node, symbolTable, fn)
					}

					op := chunk.OP_CALL_CALLEE_FIRST
					if current.Optional {
						op = chunk.OP_CALL_OPTIONAL
					}
					fn.ValueChunk().EmitBytes(op, uint8(len(current.Arguments)), 0)
				} else {
					var calledWithSpread uint8 = 0 // false
					if callee, _ := symbolTable.findVariable(current.Callee.Name); callee != nil && callee.fn != nil {
						if argumentsContainSpread(current.Arguments) || callee.fn.HasRestParameter() {
							fn.ValueChunk().EmitByte(chunk.OP_ARG_START)
							if !callee.fn.HasRestParameter() && argumentsContainSpread(current.Arguments) {
								calledWithSpread = 1
							}
						}
					}

					if len(current.Arguments) == 0 && calleeIsPromiseFunction(current.Callee) {
						fn.ValueChunk().EmitByte(chunk.OP_PUSH_UNDEFINED)
					}

					for _, node := range current.Arguments {
						generateByteCode(node, symbolTable, fn)
					}

					savedHandle := wantMethodHandle
					wantMethodHandle = true
					generateByteCode(current.Callee, symbolTable, fn)
					wantMethodHandle = savedHandle
					if referenceIsDiscarded(current.Callee) {
						fn.ValueChunk().EmitByte(chunk.OP_UNWRAP_METHOD)
					}

					fn.ValueChunk().EmitBytes(chunk.OP_CALL, uint8(len(current.Arguments)), calledWithSpread)
				}

				popReturnValue.pop()

				if popReturnValue.current() {
					fn.ValueChunk().EmitByte(chunk.OP_POP)
				}
			}
		case parser.NODE_MEMBER_EXPRESSION:
			{
				savedHandle := wantMethodHandle
				wantMethodHandle = false
				generateByteCode(current.Object, symbolTable, fn)
				wantMethodHandle = savedHandle
				if current.Optional {
					fn.ValueChunk().EmitByte(chunk.OP_DUP)
					fn.ValueChunk().EmitBytes(chunk.OP_JUMP_IF_NULLISH, 0, 0, 0, 0)
					jumpStart := len(fn.ValueChunk().Code) - 4
					if len(chainJumpStack) > 0 {
						top := len(chainJumpStack) - 1
						chainJumpStack[top] = append(chainJumpStack[top], jumpStart)
					}
				}
				if current.Computed {
					generateByteCode(current.Property, symbolTable, fn)
					fn.ValueChunk().EmitByte(chunk.OP_TO_PROPERTY_KEY)
				} else {
					withAstTracking(current.Property, fn, func() {
						handle := heap.Allocate(native.NewLightString(current.Property.Name))
						fn.ValueChunk().WriteConstant(value.EncodeHandle(handle))
					})
				}
				fn.ValueChunk().EmitByte(chunk.OP_GET_OBJECT_MEMBER)
				if !savedHandle {
					fn.ValueChunk().EmitByte(chunk.OP_UNWRAP_METHOD)
				}
			}
		case parser.NODE_THIS_EXPRESSION:
			{
				fn.ValueChunk().EmitByte(chunk.OP_THIS)
			}
		case parser.NODE_VARIABLE_DECLARATOR:
			{
				if current.Identifier.Type == parser.NODE_ARRAY_PATTERN || current.Identifier.Type == parser.NODE_OBJECT_PATTERN {
					generatePatternDeclaration(current.Identifier, current.Initializer, symbolTable, fn)
					return
				}
				if current.Identifier.Type == parser.NODE_ARRAY_PATTERN {
					pattern := current.Identifier
					var arr *Variable
					if current.Initializer.Type == parser.NODE_IDENTIFIER {
						arr, _ = symbolTable.findVariable(current.Initializer.Name)
					} else {
						generateByteCode(current.Initializer, symbolTable, fn)

					}

					var getOp uint8

					if arr != nil {
						switch arr.scope {
						case LOCAL:
							getOp = chunk.OP_GET_LOCAL
						case GLOBAL:
							getOp = chunk.OP_GET_GLOBAL
						case HEAP:
							getOp = chunk.OP_GET_HEAP_VAR
						}
					}

					for i, element := range pattern.Elements {
						var defineOp uint8

						el, _ := symbolTable.findVariable(element.Name)

						switch el.scope {
						case LOCAL:
							defineOp = chunk.OP_DEFINE_LOCAL
						case GLOBAL:
							defineOp = chunk.OP_DEFINE_GLOBAL
						case HEAP:
							defineOp = chunk.OP_DEFINE_HEAP_VAR
						}

						if arr != nil {
							fn.ValueChunk().EmitBytes(getOp, uint8(arr.slot))
						} else {
							// GET_OBJECT_MEMBER pops the object so we'll keep pushing it to the stack
							fn.ValueChunk().EmitByte(chunk.OP_PUSH_CURRENT)
						}
						withAstTracking(element, fn, func() {
							fn.ValueChunk().WriteConstant(value.ValueFromFloat64(float64(i)))
							fn.ValueChunk().EmitBytes(chunk.OP_GET_OBJECT_MEMBER, defineOp)
						})
					}

					if arr == nil {
						fn.ValueChunk().EmitByte(chunk.OP_POP)
					}
					markPatternDeclared(current.Identifier, symbolTable)
					return
				}

				if current.Identifier.Type == parser.NODE_OBJECT_PATTERN {
					pattern := current.Identifier
					var obj *Variable

					if current.Initializer.Type == parser.NODE_IDENTIFIER {
						obj, _ = symbolTable.findVariable(current.Initializer.Name)
					} else {
						generateByteCode(current.Initializer, symbolTable, fn)

					}

					var getOp uint8

					switch obj.scope {
					case LOCAL:
						getOp = chunk.OP_GET_LOCAL
					case GLOBAL:
						getOp = chunk.OP_GET_GLOBAL
					case HEAP:
						getOp = chunk.OP_GET_HEAP_VAR
					}

					exclude := []value.Value{}

					for _, prop := range pattern.Properties {
						switch prop.Type {
						case parser.NODE_PROPERTY:
							name := prop.Value.(*parser.Node).Name
							property, _ := symbolTable.findVariable(name)
							var defineOp uint8
							var k value.Value
							withAstTracking(prop, fn, func() {
								k = value.EncodeHandle(heap.Allocate(native.NewLightString(prop.Key.Name)))
							})
							v := value.EncodeHandle(heap.Allocate(native.NewLightString(name)))

							switch property.scope {
							case LOCAL:
								defineOp = chunk.OP_DEFINE_LOCAL
							case GLOBAL:
								defineOp = chunk.OP_DEFINE_GLOBAL
							case HEAP:
								defineOp = chunk.OP_DEFINE_HEAP_VAR
							}

							fn.ValueChunk().EmitBytes(getOp, uint8(obj.slot))

							if prop.Shorthand {
								fn.ValueChunk().WriteConstant(v)
							} else {
								fn.ValueChunk().WriteConstant(k)
							}
							fn.ValueChunk().EmitBytes(chunk.OP_GET_OBJECT_MEMBER, defineOp)
							exclude = append(exclude, k)
						case parser.NODE_REST_ELEMENT:
							item, _ := symbolTable.findVariable(prop.Argument.Name)
							fn.ValueChunk().EmitBytes(getOp, uint8(obj.slot), chunk.OP_CREATE_REST_OBJECT, uint8(len(exclude)))
							for _, exclude := range exclude {
								slot := fn.ValueChunk().AddConstant(exclude)
								fn.ValueChunk().EmitByte(slot)
							}

							var defineOp uint8

							switch item.scope {
							case LOCAL:
								defineOp = chunk.OP_DEFINE_LOCAL
							case HEAP:
								defineOp = chunk.OP_DEFINE_HEAP_VAR
							case GLOBAL:
								defineOp = chunk.OP_DEFINE_GLOBAL
							}

							fn.ValueChunk().EmitByte(defineOp)
						}

					}
					markPatternDeclared(current.Identifier, symbolTable)
					return
				}

				name := current.Identifier.Name
				variable, _ := symbolTable.findVariable(name)

				if variable != nil {
					if current.Initializer != nil {
						generateByteCode(current.Initializer, symbolTable, fn)

					} else if current.Initializer == nil && variable.type_ != FOR {
						fn.ValueChunk().EmitByte(chunk.OP_PUSH_UNDEFINED)
					}

					variable.declared = true

					// for of loop i.e for (const item of arr) {}
					if variable.type_ == FOR {
						var op uint8
						if variable.init {
							switch variable.scope {
							case GLOBAL:
								op = chunk.OP_SET_GLOBAL
							case LOCAL:
								op = chunk.OP_SET_LOCAL
							}
							fn.ValueChunk().EmitBytes(op, uint8(variable.slot))
						} else {
							switch variable.scope {
							case GLOBAL:
								op = chunk.OP_DEFINE_GLOBAL
							case LOCAL:
								op = chunk.OP_DEFINE_LOCAL
							}
							variable.init = true
							fn.ValueChunk().EmitByte(op)
						}
						return
					}

					switch variable.scope {
					case GLOBAL:
						fn.ValueChunk().EmitBytes(chunk.OP_SET_GLOBAL, uint8(variable.slot))
					case LOCAL:
						fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)
					case HEAP:
						fn.ValueChunk().EmitBytes(chunk.OP_DEFINE_HEAP_VAR_AT, uint8(variable.slot))
					}

				}
			}
		case parser.NODE_EXPRESSION_STATEMENT:
			{
				if current.Directive != "" {
					return
				}
				popReturnValue.push(false)
				generateByteCode(current.Expression, symbolTable, fn)
				popReturnValue.pop()
				fn.ValueChunk().EmitByte(chunk.OP_POP)
			}
		case parser.NODE_IF_STATEMENT:
			{
				popReturnValue.push(false)
				generateByteCode(current.Test, symbolTable, fn)

				popReturnValue.pop()
				fn.ValueChunk().EmitBytes(chunk.OP_JUMP_IF_FALSE, 0, 0, 0, 0)
				jumpStart := len(fn.ValueChunk().Code) - 4

				generateByteCode(current.Consequent, symbolTable, fn)

				altJump := 0
				if current.Alternate != nil {
					fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
					altJump = len(fn.ValueChunk().Code) - 4
				}
				fn.ValueChunk().PatchUint32(uint32(jumpStart), uint32(len(fn.ValueChunk().Code)))

				if current.Alternate != nil {
					generateByteCode(current.Alternate, symbolTable, fn)

					fn.ValueChunk().PatchUint32(uint32(altJump), uint32(len(fn.ValueChunk().Code)))
				}
			}
		case parser.NODE_BINARY_EXPRESSION:
			{
				popReturnValue.push(false)
				generateByteCode(current.Left, symbolTable, fn)
				generateByteCode(current.Right, symbolTable, fn)

				popReturnValue.pop()
				fn.ValueChunk().EmitByte(operatorMap[current.BinaryOperator])
			}
		case parser.NODE_LITERAL:
			{
				switch v := current.Value.(type) {
				case float64:
					{
						fn.ValueChunk().WriteConstant(value.ValueFromFloat64(v))
					}
				case int:
					{
						fn.ValueChunk().WriteConstant(value.ValueFromFloat64(float64(v)))
					}
				case int64:
					{
						fn.ValueChunk().WriteConstant(value.ValueFromFloat64(float64(v)))
					}
				case uint64:
					{
						fn.ValueChunk().WriteConstant(value.ValueFromFloat64(float64(v)))
					}
				case []byte:
					{
						fn.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(string(v)))))
					}
				case bool:
					{
						if v {
							fn.ValueChunk().WriteConstant(value.TRUE)
						} else {
							fn.ValueChunk().WriteConstant(value.FALSE)
						}
					}
				case nil:
					{
						if current.Raw == "null" {
							fn.ValueChunk().WriteConstant(value.NULL)
						}
					}
				}
			}
		case parser.NODE_RETURN_STATEMENT:
			{
				fn.ReturnsPromise(argumentIsPromise(current.Argument))

				if current.Argument == nil {
					fn.ValueChunk().EmitByte(chunk.OP_PUSH_UNDEFINED)
				} else {
					popReturnValue.push(false)
					generateByteCode(current.Argument, symbolTable, fn)
					popReturnValue.pop()
				}

				if len(finalizerStack) > 0 {
					fn.ValueChunk().EmitByte(chunk.OP_SAVE_COMPLETION)
					emitAllIteratorCloses(fn)
					emitAbruptFinalizers(symbolTable, fn)
					fn.ValueChunk().EmitByte(chunk.OP_RESTORE_COMPLETION)
				} else {
					emitAllIteratorCloses(fn)
				}

				fn.ValueChunk().EmitByte(chunk.OP_RETURN)
			}
		case parser.NODE_IDENTIFIER:
			{
				if current.Name == UNDEFINED_IDENTIFIER {
					fn.ValueChunk().WriteConstant(value.UNDEFINED)
					return
				}
				if current.Name == "NaN" {
					fn.ValueChunk().WriteConstant(value.STANDARD_NAN)
					return
				}
				if current.Name == "Infinity" {
					fn.ValueChunk().WriteConstant(value.ValueFromFloat64(math.Inf(1)))
					return
				}
				if current.Name == "globalThis" {
					fn.ValueChunk().EmitByte(chunk.OP_PUSH_GLOBAL_OBJECT)
					return
				}
				variable, owner := symbolTable.findVariable(current.Name)

				if variable != nil && owner != nil && owner != symbolTable && variable.scope == LOCAL {
					oldSlot := variable.slot
					compactBlockSlots(owner, oldSlot)
					variable.scope = HEAP
					variable.slot = owner.heapVarCount
					owner.heapVarCount++
				}

				if variable == nil {
					arg := fn.ValueChunk().AddConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(current.Name))))
					fn.ValueChunk().EmitBytes(chunk.OP_GET_GLOBAL_BY_NAME, arg)
					return
				}

				if (variable.type_ == LET || variable.type_ == CONST) && !variable.declared && !variable.undeclared && owner == symbolTable {
					err := native.NewError()
					err.SetMember(native.KEY_MESSAGE, value.EncodeHandle(heap.Allocate(native.NewLightString("Cannot access '"+current.Name+"' before initialization"))))
					err.SetMember(native.KEY_NAME, value.EncodeHandle(heap.Allocate(native.NewLightString("ReferenceError"))))
					fn.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(err)))
					fn.ValueChunk().EmitByte(chunk.OP_THROW)
					return
				}

				if variable.type_ == CATCH_PARAM {
					fn.ValueChunk().EmitBytes(chunk.OP_GET_CATCH, uint8(variable.catchId))
					return
				}

				switch variable.scope {
				case GLOBAL:
					fn.ValueChunk().EmitBytes(chunk.OP_GET_GLOBAL, uint8(variable.slot))
				case LOCAL:
					fn.ValueChunk().EmitBytes(chunk.OP_GET_LOCAL, uint8(variable.slot))
				case HEAP:
					fn.ValueChunk().EmitBytes(chunk.OP_GET_HEAP_VAR, uint8(variable.slot))
				}
			}
		case parser.NODE_WHILE_STATEMENT:
			{
				testStart := uint32(len(fn.ValueChunk().Code))
				generateByteCode(current.Test, symbolTable, fn)

				fn.ValueChunk().EmitBytes(chunk.OP_JUMP_IF_FALSE, 0, 0, 0, 0)
				jumpStart := uint32(len(fn.ValueChunk().Code) - 4)

				pushLoop(BLOCK_SCOPES[current.BodyNode])

				generateByteCode(current.BodyNode, symbolTable, fn)

				fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)

				fn.ValueChunk().PatchUint32(uint32(len(fn.ValueChunk().Code)-4), testStart)
				loopEnd := uint32(len(fn.ValueChunk().Code))
				fn.ValueChunk().PatchUint32(jumpStart, loopEnd)

				for _, breakpoint := range currentLoop().points {
					pos := breakpoint.position
					switch breakpoint.type_ {
					case BREAKPOINT_BREAK:
						fn.ValueChunk().PatchUint32(uint32(pos), loopEnd)
					case BREAKPOINT_CONTINUE:
						fn.ValueChunk().PatchUint32(uint32(pos), testStart)
					}
				}
				popLoop()

			}
		case parser.NODE_CONTINUE_STATEMENT:
			{
				label := ""
				if current.Label != nil {
					label = current.Label.Name
				}
				l := findLoop(label, false)

				emitIteratorClosesTo(fn, l, false)
				emitAbruptFinalizers(symbolTable, fn)

				currentBlock := symbolTable.block

				for l != nil && l.block != nil && currentBlock != nil {
					if currentBlock == l.block {
						break
					}
					for range currentBlock.vars {
						fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
					}
					currentBlock = currentBlock.parent
				}

				fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
				if l != nil {
					l.points = append(l.points, &LoopBreakPoint{type_: BREAKPOINT_CONTINUE, position: len(fn.ValueChunk().Code) - 4})
				}
			}
		case parser.NODE_BREAK_STATEMENT:
			{
				label := ""
				if current.Label != nil {
					label = current.Label.Name
				}
				l := findLoop(label, true)

				emitIteratorClosesTo(fn, l, true)
				emitAbruptFinalizers(symbolTable, fn)

				currentBlock := symbolTable.block
				for l != nil && l.block != nil && currentBlock != nil {
					for range currentBlock.vars {
						fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
					}
					if currentBlock == l.block {
						break
					}
					currentBlock = currentBlock.parent
				}
				fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
				if l != nil {
					l.points = append(l.points, &LoopBreakPoint{type_: BREAKPOINT_BREAK, position: len(fn.ValueChunk().Code) - 4})
				}

			}
		case parser.NODE_UPDATE_EXPRESSION:
			{
				if current.Argument.Type == parser.NODE_MEMBER_EXPRESSION {
					generateByteCode(current.Argument.Object, symbolTable, fn)
					if current.Argument.Computed {
						generateByteCode(current.Argument.Property, symbolTable, fn)
					} else {
						handle := heap.Allocate(native.NewLightString(current.Argument.Property.Name))
						fn.ValueChunk().WriteConstant(value.EncodeHandle(handle))
					}
					prefix := byte(0)
					if current.Prefix {
						prefix = 1
					}
					delta := byte(0)
					if isDecrement(current) {
						delta = 1
					}
					fn.ValueChunk().EmitBytes(chunk.OP_MEMBER_UPDATE, prefix, delta)
					if popReturnValue.current() {
						fn.ValueChunk().EmitByte(chunk.OP_POP)
					}
					return
				}

				variable, _ := symbolTable.findVariable(current.Argument.Name)
				if variable == nil {
					emitThrow(fn, "ReferenceError", current.Argument.Name+" is not defined")
					return
				}
				if variable.type_ == CONST {
					emitThrow(fn, "TypeError", "Assignment to constant variable.")
					return
				}

				var setOp uint8
				if variable.type_ == CATCH_PARAM {
					fn.ValueChunk().EmitBytes(chunk.OP_GET_CATCH, uint8(variable.catchId))
					setOp = chunk.OP_SET_CATCH
				} else {
					switch variable.scope {
					case GLOBAL:
						fn.ValueChunk().EmitBytes(chunk.OP_GET_GLOBAL, uint8(variable.slot))
						setOp = chunk.OP_SET_GLOBAL
					case LOCAL:
						fn.ValueChunk().EmitBytes(chunk.OP_GET_LOCAL, uint8(variable.slot))
						setOp = chunk.OP_SET_LOCAL
					case HEAP:
						fn.ValueChunk().EmitBytes(chunk.OP_GET_HEAP_VAR, uint8(variable.slot))
						setOp = chunk.OP_SET_HEAP_VAR
					}
				}

				fn.ValueChunk().EmitByte(chunk.OP_TO_NUMBER)
				if !current.Prefix {
					fn.ValueChunk().EmitByte(chunk.OP_DUP)
				}
				fn.ValueChunk().WriteConstant(value.ValueFromFloat64(1))
				if isDecrement(current) {
					fn.ValueChunk().EmitByte(chunk.OP_SUBTRACT)
				} else {
					fn.ValueChunk().EmitByte(chunk.OP_ADD)
				}
				if current.Prefix {
					fn.ValueChunk().EmitByte(chunk.OP_DUP)
				}
				fn.ValueChunk().EmitByte(setOp)
				if variable.scope != THIS {
					fn.ValueChunk().EmitByte(variableOperand(variable))
				}
				if popReturnValue.current() {
					fn.ValueChunk().EmitByte(chunk.OP_POP)
				}
			}
		case parser.NODE_FOR_STATEMENT:
			{
				symbolTable.enterBlockScope(current.BodyNode)
				if blockHasHeapVars(symbolTable.block) {
					fn.ValueChunk().EmitByte(chunk.OP_ENSURE_HEAP_SCOPE)
				}
				generateByteCode(current.Initializer, symbolTable, fn)

				testStart := len(fn.ValueChunk().Code)
				generateByteCode(current.Test, symbolTable, fn)

				fn.ValueChunk().EmitBytes(chunk.OP_JUMP_IF_FALSE, 0, 0, 0, 0)
				jumpStart := uint32(len(fn.ValueChunk().Code) - 4)
				pushLoop(BLOCK_SCOPES[current.BodyNode])
				for _, node := range current.BodyNode.Body {
					generateByteCode(node, symbolTable, fn)
				}

				initializerLocals := initializerLocalVariableCount(current.Initializer, symbolTable)
				count := blockLocalVarCount(symbolTable.block) - initializerLocals

				if blockHasHeapVars(symbolTable.block) {
					fn.ValueChunk().EmitByte(chunk.OP_COPY_HEAP_SCOPE)
				}

				updateStart := uint32(len(fn.ValueChunk().Code))
				for count > 0 {
					fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
					count--
				}

				generateByteCode(current.Update, symbolTable, fn)

				fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
				fn.ValueChunk().PatchUint32(uint32(len(fn.ValueChunk().Code)-4), uint32(testStart))

				loopEnd := uint32(len(fn.ValueChunk().Code))
				fn.ValueChunk().PatchUint32(jumpStart, loopEnd)

				for range initializerLocals {
					fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
				}
				cleanupEnd := uint32(len(fn.ValueChunk().Code))
				for _, breakpoint := range currentLoop().points {
					pos := breakpoint.position
					switch breakpoint.type_ {
					case BREAKPOINT_BREAK:
						fn.ValueChunk().PatchUint32(uint32(pos), cleanupEnd)
					case BREAKPOINT_CONTINUE:
						fn.ValueChunk().PatchUint32(uint32(pos), updateStart)
					}
				}
				popLoop()
				symbolTable.exitBlockScope()
			}
		case parser.NODE_FOR_OF_STATEMENT:
			{
				symbolTable.enterBlockScope(current.BodyNode)
				for _, variable := range symbolTable.block.vars {
					variable.declared = true
				}
				bindings := loopBindingNameSet(current.Left)
				bindings[ITERATOR_VAR_NAME] = true
				if blockHasHeapVars(symbolTable.block) {
					fn.ValueChunk().EmitByte(chunk.OP_ENSURE_HEAP_SCOPE)
				}
				variables := sortedBlockVariables(symbolTable.block)
				for _, variable := range variables {
					if !bindings[variableName(symbolTable.block, variable)] {
						continue
					}
					fn.ValueChunk().EmitByte(chunk.OP_PUSH_UNDEFINED)
					if variable.scope == HEAP {
						fn.ValueChunk().EmitBytes(chunk.OP_DEFINE_HEAP_VAR_AT, uint8(variable.slot))
					} else {
						fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)
					}
				}
				popReturnValue.push(false)
				generateByteCode(current.Right, symbolTable, fn)

				popReturnValue.pop()
				fn.ValueChunk().EmitBytes(chunk.OP_GET_ITERATOR, ITERATOR_FOR_OF)
				iteratorVariable, _ := symbolTable.findVariable(ITERATOR_VAR_NAME)
				fn.ValueChunk().EmitBytes(chunk.OP_SET_LOCAL, uint8(iteratorVariable.slot))
				fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_NEXT, chunk.OP_JUMP_IF_TRUE, 0, 0, 0, 0)
				jumpStart := uint32(len(fn.ValueChunk().Code) - 4)
				if blockHasHeapVars(symbolTable.block) {
					fn.ValueChunk().EmitByte(chunk.OP_COPY_HEAP_SCOPE)
				}
				fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_CURRENT)

				parseForDotDotLoopVariable(current, symbolTable, fn)

				pushLoop(BLOCK_SCOPES[current.BodyNode])
				currentLoop().iteratorSlot = int(iteratorVariable.slot)
				for _, node := range current.BodyNode.Body {
					generateByteCode(node, symbolTable, fn)
				}

				continuationStart := len(fn.ValueChunk().Code)
				bodyLocals := bodyLocalVariableCount(symbolTable.block, bindings)
				for range bodyLocals {
					fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
				}
				fn.ValueChunk().EmitByte(chunk.OP_JUMP)
				fn.ValueChunk().EmitUint32(jumpStart - 2)
				loopEnd := uint32(len(fn.ValueChunk().Code))
				fn.ValueChunk().PatchUint32(jumpStart, loopEnd)

				fn.ValueChunk().EmitByte(chunk.OP_POP) // pop the iterator object

				boundLocals := 0
				for name, variable := range symbolTable.block.vars {
					if name != ITERATOR_VAR_NAME && bindings[name] && variable.scope != HEAP && variable.type_ != CATCH_PARAM {
						boundLocals++
					}
				}
				for range boundLocals {
					fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
				}
				cleanupEnd := uint32(len(fn.ValueChunk().Code))
				for _, breakpoint := range currentLoop().points {
					pos := breakpoint.position
					switch breakpoint.type_ {
					case BREAKPOINT_BREAK:
						fn.ValueChunk().PatchUint32(uint32(pos), cleanupEnd)
					case BREAKPOINT_CONTINUE:
						fn.ValueChunk().PatchUint32(uint32(pos), uint32(continuationStart))
					}
				}

				popLoop()
				symbolTable.exitBlockScope()
			}
		case parser.NODE_FOR_IN_STATEMENT:
			{
				symbolTable.enterBlockScope(current.BodyNode)
				for _, variable := range symbolTable.block.vars {
					variable.declared = true
				}
				bindings := loopBindingNameSet(current.Left)
				bindings[ITERATOR_VAR_NAME] = true
				if blockHasHeapVars(symbolTable.block) {
					fn.ValueChunk().EmitByte(chunk.OP_ENSURE_HEAP_SCOPE)
				}
				variables := sortedBlockVariables(symbolTable.block)
				for _, variable := range variables {
					if !bindings[variableName(symbolTable.block, variable)] {
						continue
					}
					fn.ValueChunk().EmitByte(chunk.OP_PUSH_UNDEFINED)
					if variable.scope == HEAP {
						fn.ValueChunk().EmitBytes(chunk.OP_DEFINE_HEAP_VAR_AT, uint8(variable.slot))
					} else {
						fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)
					}
				}
				generateByteCode(current.Right, symbolTable, fn)

				fn.ValueChunk().EmitBytes(chunk.OP_GET_ITERATOR, ITERATOR_FOR_IN)
				iteratorVariable, _ := symbolTable.findVariable(ITERATOR_VAR_NAME)
				fn.ValueChunk().EmitBytes(chunk.OP_SET_LOCAL, uint8(iteratorVariable.slot))
				fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_NEXT, chunk.OP_JUMP_IF_TRUE, 0, 0, 0, 0)
				jumpStart := uint32(len(fn.ValueChunk().Code) - 4)
				if blockHasHeapVars(symbolTable.block) {
					fn.ValueChunk().EmitByte(chunk.OP_COPY_HEAP_SCOPE)
				}
				fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_CURRENT)

				parseForDotDotLoopVariable(current, symbolTable, fn)

				pushLoop(BLOCK_SCOPES[current.BodyNode])
				for _, node := range current.BodyNode.Body {
					generateByteCode(node, symbolTable, fn)
				}

				continuationStart := len(fn.ValueChunk().Code)
				bodyLocals := bodyLocalVariableCount(symbolTable.block, bindings)
				for range bodyLocals {
					fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
				}
				fn.ValueChunk().EmitByte(chunk.OP_JUMP)
				fn.ValueChunk().EmitUint32(jumpStart - 2)
				loopEnd := uint32(len(fn.ValueChunk().Code))
				fn.ValueChunk().PatchUint32(jumpStart, loopEnd)
				fn.ValueChunk().EmitByte(chunk.OP_POP) // pop the iterator object

				boundLocals := 0
				for name, variable := range symbolTable.block.vars {
					if name != ITERATOR_VAR_NAME && bindings[name] && variable.scope != HEAP && variable.type_ != CATCH_PARAM {
						boundLocals++
					}
				}
				for range boundLocals {
					fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
				}
				cleanupEnd := uint32(len(fn.ValueChunk().Code))
				for _, breakpoint := range currentLoop().points {
					pos := breakpoint.position
					switch breakpoint.type_ {
					case BREAKPOINT_BREAK:
						fn.ValueChunk().PatchUint32(uint32(pos), cleanupEnd)
					case BREAKPOINT_CONTINUE:
						fn.ValueChunk().PatchUint32(uint32(pos), uint32(continuationStart))
					}
				}

				popLoop()
				symbolTable.exitBlockScope()
			}
		case parser.NODE_OBJECT_EXPRESSION:
			{
				fn.ValueChunk().EmitByte(chunk.OP_CREATE_OBJECT)

				popReturnValue.push(false)
				for _, property := range current.Properties {
					switch property.Type {
					case parser.NODE_PROPERTY:
						if !property.Computed && objectPropertyKeyName(property.Key) == "__proto__" {
							generateByteCode(property.Value.(*parser.Node), symbolTable, fn)
							fn.ValueChunk().EmitByte(chunk.OP_SET_OBJECT_PROTOTYPE)
							continue
						}
						if property.Computed {
							generateByteCode(property.Key, symbolTable, fn)
							fn.ValueChunk().EmitByte(chunk.OP_TO_PROPERTY_KEY)
						} else {
							withAstTracking(property.Key, fn, func() {
								fn.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(objectPropertyKeyName(property.Key)))))
							})
						}
						generateByteCode(property.Value.(*parser.Node), symbolTable, fn)

						switch property.Kind {
						case parser.KIND_PROPERTY_GET:
							fn.ValueChunk().EmitByte(chunk.OP_SET_OBJECT_GETTER)
						case parser.KIND_PROPERTY_SET:
							fn.ValueChunk().EmitByte(chunk.OP_SET_OBJECT_SETTER)
						default:
							fn.ValueChunk().EmitByte(chunk.OP_SET_OBJECT_MEMBER)
						}
					case parser.NODE_SPREAD_ELEMENT:
						generateByteCode(property.Argument, symbolTable, fn)
						fn.ValueChunk().EmitByte(chunk.OP_SET_FROM_SPREAD)
					}
				}
				popReturnValue.pop()
			}
		case parser.NODE_TEMPLATE_LITERAL:
			{
				var start *native.LightString
				withAstTracking(current.Quasis[0], fn, func() {
					start = native.NewLightString(current.Quasis[0].Value.(parser.TemplateNodeValue).Cooked)
					fn.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(start)))
				})

				if len(current.Quasis) == 1 {
					return
				}

				if len(current.Expressions) > 0 {
					generateByteCode(current.Expressions[0], symbolTable, fn)
					fn.ValueChunk().EmitByte(chunk.OP_TO_STRING)
				}

				fn.ValueChunk().EmitByte(chunk.OP_ADD)
				i := 1

				for i < len(current.Quasis) {

					quasi := current.Quasis[i].Value.(parser.TemplateNodeValue)
					withAstTracking(current.Quasis[i], fn, func() {
						fn.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(quasi.Cooked))))
					})
					fn.ValueChunk().EmitByte(chunk.OP_ADD)

					if i < len(current.Expressions) {
						generateByteCode(current.Expressions[i], symbolTable, fn)
						fn.ValueChunk().EmitByte(chunk.OP_TO_STRING)

						fn.ValueChunk().EmitByte(chunk.OP_ADD)
					}
					i++
				}
			}
		case parser.NODE_TRY_STATEMENT:
			{
				if current.Finalizer != nil {
					finalizerStack = append(finalizerStack, current.Finalizer)

					fn.ValueChunk().EmitBytes(chunk.OP_TRY_BLOCK_START, 0, 0, 0, 0)
					syntheticCatch := len(fn.ValueChunk().Code) - 4

					if current.Handler != nil {
						fn.ValueChunk().EmitBytes(chunk.OP_TRY_BLOCK_START, 0, 0, 0, 0)
						innerCatch := len(fn.ValueChunk().Code) - 4

						generateByteCode(current.Block, symbolTable, fn)

						fn.ValueChunk().EmitByte(chunk.OP_TRY_BLOCK_END)
						fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
						afterCatch := len(fn.ValueChunk().Code) - 4
						fn.ValueChunk().PatchUint32(uint32(innerCatch), uint32(len(fn.ValueChunk().Code)))

						handler := current.Handler
						if handler.Param != nil {
							emitCatchParam(handler.Param, symbolTable, fn)
						} else {
							fn.ValueChunk().EmitByte(chunk.OP_POP)
						}
						generateByteCode(handler.BodyNode, symbolTable, fn)

						fn.ValueChunk().PatchUint32(uint32(afterCatch), uint32(len(fn.ValueChunk().Code)))
					} else {
						generateByteCode(current.Block, symbolTable, fn)
					}

					fn.ValueChunk().EmitByte(chunk.OP_TRY_BLOCK_END)
					fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
					normalFinally := len(fn.ValueChunk().Code) - 4

					fn.ValueChunk().PatchUint32(uint32(syntheticCatch), uint32(len(fn.ValueChunk().Code)))

					saved := finalizerStack
					finalizerStack = saved[:len(saved)-1]
					generateByteCode(current.Finalizer, symbolTable, fn)
					finalizerStack = saved
					fn.ValueChunk().EmitByte(chunk.OP_THROW)

					fn.ValueChunk().PatchUint32(uint32(normalFinally), uint32(len(fn.ValueChunk().Code)))
					finalizerStack = saved[:len(saved)-1]
					generateByteCode(current.Finalizer, symbolTable, fn)
					finalizerStack = saved[:len(saved)-1]
				} else {
					fn.ValueChunk().EmitBytes(chunk.OP_TRY_BLOCK_START, 0, 0, 0, 0)
					tryStart := len(fn.ValueChunk().Code) - 4

					generateByteCode(current.Block, symbolTable, fn)

					fn.ValueChunk().EmitByte(chunk.OP_TRY_BLOCK_END)
					fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
					jumpStart := len(fn.ValueChunk().Code) - 4
					fn.ValueChunk().PatchUint32(uint32(tryStart), uint32(len(fn.ValueChunk().Code)))

					handler := current.Handler
					if handler.Param != nil {
						emitCatchParam(handler.Param, symbolTable, fn)
					} else {
						fn.ValueChunk().EmitByte(chunk.OP_POP)
					}
					generateByteCode(handler.BodyNode, symbolTable, fn)

					fn.ValueChunk().PatchUint32(uint32(jumpStart), uint32(len(fn.ValueChunk().Code)))
				}
			}
		case parser.NODE_THROW_STATEMENT:
			{
				count := blockLocalVarCount(symbolTable.block)

				for range count {
					fn.ValueChunk().EmitByte(chunk.OP_POP_LOCAL)
				}
				generateByteCode(current.Argument, symbolTable, fn)

				fn.ValueChunk().EmitByte(chunk.OP_THROW)
			}
		case parser.NODE_NEW_EXPRESSION:
			{
				for _, node := range current.Arguments {
					generateByteCode(node, symbolTable, fn)
				}
				// safeguards later: len(current.Arguments) > uint8.MAX
				generateByteCode(current.Callee, symbolTable, fn)

				fn.ValueChunk().EmitByte(chunk.OP_NEW)
				fn.ValueChunk().EmitByte(uint8(len(current.Arguments)))

			}
		case parser.NODE_CONDITIONAL_EXPRESSION:
			{
				generateByteCode(current.Test, symbolTable, fn)

				fn.ValueChunk().EmitBytes(chunk.OP_JUMP_IF_FALSE, 0, 0, 0, 0)
				jumpStart := len(fn.ValueChunk().Code) - 4

				generateByteCode(current.Consequent, symbolTable, fn)

				altJump := 0
				if current.Alternate != nil {
					fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
					altJump = len(fn.ValueChunk().Code) - 4
				}
				fn.ValueChunk().PatchUint32(uint32(jumpStart), uint32(len(fn.ValueChunk().Code)))

				if current.Alternate != nil {
					generateByteCode(current.Alternate, symbolTable, fn)

					fn.ValueChunk().PatchUint32(uint32(altJump), uint32(len(fn.ValueChunk().Code)))
				}
			}
		case parser.NODE_LOGICAL_EXPRESSION:
			{
				generateByteCode(current.Left, symbolTable, fn)

				var jump uint8
				switch current.BinaryOperator {
				case logicalAnd:
					jump = chunk.OP_JUMP_IF_FALSE
				case logicalOr:
					jump = chunk.OP_JUMP_IF_TRUE
				case nullishCoalescing:
					jump = chunk.OP_JUMP_IF_NOT_NULLISH
				default:
					jump = chunk.OP_JUMP_IF_FALSE
				}

				fn.ValueChunk().EmitByte(chunk.OP_DUP)
				fn.ValueChunk().EmitBytes(jump, 0, 0, 0, 0)
				jumpStart := len(fn.ValueChunk().Code) - 4

				fn.ValueChunk().EmitByte(chunk.OP_POP)
				generateByteCode(current.Right, symbolTable, fn)

				fn.ValueChunk().PatchUint32(uint32(jumpStart), uint32(len(fn.ValueChunk().Code)))
			}
		case parser.NODE_AWAIT_EXPRESSION:
			{
				popReturnValue.push(false)
				generateByteCode(current.Argument, symbolTable, fn)

				popReturnValue.pop()
				fn.ValueChunk().EmitByte(chunk.OP_AWAIT)
			}
		case parser.NODE_UNARY_EXPRESSION:
			{
				switch current.UnaryOperator {
				case parser.UNARY_TYPEOF:
					if current.Argument.Type == parser.NODE_IDENTIFIER {
						if variable, _ := symbolTable.findVariable(current.Argument.Name); variable == nil {
							fn.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(native.NewLightString("undefined"))))
							return
						}
					}
					generateByteCode(current.Argument, symbolTable, fn)
					fn.ValueChunk().EmitByte(chunk.OP_TYPEOF)
				case parser.UNARY_VOID:
					generateByteCode(current.Argument, symbolTable, fn)
					fn.ValueChunk().EmitByte(chunk.OP_VOID)
				case parser.UNARY_PLUS:
					generateByteCode(current.Argument, symbolTable, fn)
					fn.ValueChunk().EmitByte(chunk.OP_TO_NUMBER)
				case parser.UNARY_BITWISE_NOT:
					generateByteCode(current.Argument, symbolTable, fn)
					fn.ValueChunk().EmitByte(chunk.OP_BITWISE_NOT)
				case parser.UNARY_DELETE:
					if current.Argument != nil && current.Argument.Type == parser.NODE_CHAIN_EXPRESSION {
						generateByteCode(current.Argument, symbolTable, fn)
						fn.ValueChunk().EmitByte(chunk.OP_POP)
						fn.ValueChunk().WriteConstant(value.TRUE)
						return
					}
					argument := current.Argument
					for argument != nil && (argument.Type == parser.NODE_CHAIN_EXPRESSION || argument.Type == parser.NODE_PARENTHESIZED_EXPRESSION) {
						argument = argument.Expression
					}
					if argument != nil && argument.Type == parser.NODE_MEMBER_EXPRESSION {
						generateByteCode(argument.Object, symbolTable, fn)
						if argument.Computed {
							generateByteCode(argument.Property, symbolTable, fn)
						} else {
							handle := heap.Allocate(native.NewLightString(argument.Property.Name))
							fn.ValueChunk().WriteConstant(value.EncodeHandle(handle))
						}
						if isStrictFunction(fn) {
							fn.ValueChunk().EmitByte(chunk.OP_DELETE_STRICT)
						} else {
							fn.ValueChunk().EmitByte(chunk.OP_DELETE)
						}
					} else {
						generateByteCode(current.Argument, symbolTable, fn)
						fn.ValueChunk().EmitByte(chunk.OP_POP)
						fn.ValueChunk().WriteConstant(value.TRUE)
					}
				default:
					generateByteCode(current.Argument, symbolTable, fn)
					fn.ValueChunk().EmitByte(unaryOperatorMap[current.UnaryOperator])
				}
			}
		case parser.NODE_CLASS_DECLARATION:
			{
				withAstTracking(current.Identifier, fn, func() {
					fn.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(current.Identifier.Name))))
				})
				fn.ValueChunk().EmitByte(chunk.OP_CREATE_CLASS_START)

				generateByteCode(current.BodyNode, symbolTable, fn)

				fn.ValueChunk().EmitByte(chunk.OP_CREATE_CLASS_END)
				switch symbolTable.tableScope {
				case GLOBAL:
					if variable, _ := symbolTable.findVariable(current.Identifier.Name); variable != nil {
						fn.ValueChunk().EmitBytes(chunk.OP_SET_GLOBAL, uint8(variable.slot))
					}
				case LOCAL:
					fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)
				}
				if variable, _ := symbolTable.findVariable(current.Identifier.Name); variable != nil {
					variable.declared = true
				}
			}
		case parser.NODE_CLASS_BODY:
			{
				symbolTable = FUNCTION_SCOPES[current]
				for _, node := range current.Body {
					generateByteCode(node, symbolTable, fn)
				}
			}
		case parser.NODE_METHOD_DEFINITION:
			{
				symbolTable = FUNCTION_SCOPES[current]
				name := current.Key.Name

				function := current.Value.(*parser.Node)
				method := object.NewFunction(fmt.Sprintf("Class method %s", name), len(function.Params), nil)
				method.Strict = true
				markParamsDeclared(function.Params, symbolTable)

				for _, node := range function.BodyNode.Body {
					generateByteCode(node, symbolTable, method)
				}

				if method.ValueChunk().Code[len(method.ValueChunk().Code)-1] != chunk.OP_RETURN {
					method.ValueChunk().EmitBytes(chunk.OP_PUSH_UNDEFINED, chunk.OP_RETURN)
				}

				withAstTracking(current.Key, fn, func() {
					fn.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(name))))
				})
				fn.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(method)))
				fn.ValueChunk().EmitByte(chunk.OP_PUSH_METHOD)
			}
		case parser.NODE_PROPERTY_DEFINITION:
			{
				name := current.Key.Name
				withAstTracking(current.Key, fn, func() {
					fn.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(name))))
				})
				generateByteCode(current.Value.(*parser.Node), symbolTable, fn)

				fn.ValueChunk().EmitByte(chunk.OP_PUSH_PROPERTY)
			}
		case parser.NODE_YIELD_EXPRESSION:
			{
				generateByteCode(current.Argument, symbolTable, fn)

				fn.ValueChunk().EmitByte(chunk.OP_YIELD)
			}
		case parser.NODE_IMPORT_DECLARATION:
			{
				generateByteCode(current.Source, symbolTable, fn)

				fn.ValueChunk().EmitBytes(chunk.OP_IMPORT)
			}
		case parser.NODE_EXPORT_NAMED_DECLARATION:
			{
				generateByteCode(current.Declaration, symbolTable, fn)

				declaration := current.Declaration

				switch declaration.Type {
				case parser.NODE_VARIABLE_DECLARATION:
					{
						declarator := declaration.Declarations[0]
						v, _ := symbolTable.findVariable(declarator.Identifier.Name)
						fn.ValueChunk().EmitBytes(chunk.OP_GET_GLOBAL, uint8(v.slot))
						withAstTracking(declarator.Identifier, fn, func() {

							fn.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(declarator.Identifier.Name))))
							fn.ValueChunk().EmitByte(chunk.OP_EXPORT)
						})
					}
				case parser.NODE_FUNCTION_DECLARATION:
					{
						v, _ := symbolTable.findVariable(declaration.Identifier.Name)
						fn.ValueChunk().EmitBytes(chunk.OP_GET_GLOBAL, uint8(v.slot))
						withAstTracking(declaration.Identifier, fn, func() {
							fn.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(declaration.Identifier.Name))))
							fn.ValueChunk().EmitByte(chunk.OP_EXPORT)
						})

					}
				}
			}
		case parser.NODE_CHAIN_EXPRESSION:
			{
				chainJumpStack = append(chainJumpStack, []int{})
				generateByteCode(current.Expression, symbolTable, fn)
				jumps := chainJumpStack[len(chainJumpStack)-1]
				chainJumpStack = chainJumpStack[:len(chainJumpStack)-1]

				if len(jumps) > 0 {
					fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
					endJump := len(fn.ValueChunk().Code) - 4
					cleanup := uint32(len(fn.ValueChunk().Code))
					for _, jump := range jumps {
						fn.ValueChunk().PatchUint32(uint32(jump), cleanup)
					}
					fn.ValueChunk().EmitBytes(chunk.OP_POP, chunk.OP_PUSH_UNDEFINED)
					fn.ValueChunk().PatchUint32(uint32(endJump), uint32(len(fn.ValueChunk().Code)))
				}
			}
		case parser.NODE_SPREAD_ELEMENT:
			{
				generateByteCode(current.Argument, symbolTable, fn)

				fn.ValueChunk().EmitByte(chunk.OP_SPREAD)
			}
		case parser.NODE_PARENTHESIZED_EXPRESSION:
			{
				generateByteCode(current.Expression, symbolTable, fn)
			}
		case parser.NODE_SEQUENCE_EXPRESSION:
			{
				popReturnValue.push(false)
				for i, expression := range current.Expressions {
					if i == len(current.Expressions)-1 {
						popReturnValue.pop()
						generateByteCode(expression, symbolTable, fn)
						popReturnValue.push(false)
					} else {
						generateByteCode(expression, symbolTable, fn)
						fn.ValueChunk().EmitByte(chunk.OP_POP)
					}
				}
				popReturnValue.pop()
			}
		case parser.NODE_DO_WHILE_STATEMENT:
			{
				bodyStart := uint32(len(fn.ValueChunk().Code))
				pushLoop(BLOCK_SCOPES[current.BodyNode])
				generateByteCode(current.BodyNode, symbolTable, fn)

				testStart := uint32(len(fn.ValueChunk().Code))
				generateByteCode(current.Test, symbolTable, fn)
				fn.ValueChunk().EmitBytes(chunk.OP_JUMP_IF_TRUE, 0, 0, 0, 0)
				fn.ValueChunk().PatchUint32(uint32(len(fn.ValueChunk().Code)-4), bodyStart)
				loopEnd := uint32(len(fn.ValueChunk().Code))

				for _, breakpoint := range currentLoop().points {
					switch breakpoint.type_ {
					case BREAKPOINT_BREAK:
						fn.ValueChunk().PatchUint32(uint32(breakpoint.position), loopEnd)
					case BREAKPOINT_CONTINUE:
						fn.ValueChunk().PatchUint32(uint32(breakpoint.position), testStart)
					}
				}
				popLoop()
			}
		case parser.NODE_LABELED_STATEMENT:
			{
				label := ""
				if current.Label != nil {
					label = current.Label.Name
				}
				body := current.BodyNode
				switch body.Type {
				case parser.NODE_FOR_STATEMENT, parser.NODE_FOR_OF_STATEMENT, parser.NODE_FOR_IN_STATEMENT, parser.NODE_WHILE_STATEMENT, parser.NODE_DO_WHILE_STATEMENT, parser.NODE_SWITCH_STATEMENT:
					pendingLoopLabel = label
					generateByteCode(body, symbolTable, fn)
					pendingLoopLabel = ""
				default:
					pushLabeledLoop(nil, label)
					generateByteCode(body, symbolTable, fn)
					end := uint32(len(fn.ValueChunk().Code))
					for _, breakpoint := range currentLoop().points {
						if breakpoint.type_ == BREAKPOINT_BREAK {
							fn.ValueChunk().PatchUint32(uint32(breakpoint.position), end)
						}
					}
					popLoop()
				}
			}
		case parser.NODE_SWITCH_STATEMENT:
			{
				pushSwitchLoop(nil)
				popReturnValue.push(false)
				generateByteCode(current.Discriminant, symbolTable, fn)
				popReturnValue.pop()

				count := len(current.Cases)
				matchJumps := make([]int, count)
				for i := range matchJumps {
					matchJumps[i] = -1
				}
				defaultIndex := -1

				for i, c := range current.Cases {
					if c.Test == nil {
						defaultIndex = i
						continue
					}
					fn.ValueChunk().EmitByte(chunk.OP_DUP)
					popReturnValue.push(false)
					generateByteCode(c.Test, symbolTable, fn)
					popReturnValue.pop()
					fn.ValueChunk().EmitByte(chunk.OP_STRICT_EQUALS)
					fn.ValueChunk().EmitBytes(chunk.OP_JUMP_IF_TRUE, 0, 0, 0, 0)
					matchJumps[i] = len(fn.ValueChunk().Code) - 4
				}

				fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
				noMatchJump := len(fn.ValueChunk().Code) - 4

				entryPositions := make([]int, count)
				contentPositions := make([]int, count)
				fallJumps := make([]int, count)
				for i := range fallJumps {
					fallJumps[i] = -1
				}

				for i, c := range current.Cases {
					entryPositions[i] = len(fn.ValueChunk().Code)
					fn.ValueChunk().EmitByte(chunk.OP_POP)
					contentPositions[i] = len(fn.ValueChunk().Code)

					statements := c.ConsequentSlice
					if len(statements) == 0 && c.Consequent != nil && c.Consequent.Type != parser.NODE_UNTYPED {
						statements = []*parser.Node{c.Consequent}
					}
					for _, statement := range statements {
						generateByteCode(statement, symbolTable, fn)
					}

					if i < count-1 {
						fn.ValueChunk().EmitBytes(chunk.OP_JUMP, 0, 0, 0, 0)
						fallJumps[i] = len(fn.ValueChunk().Code) - 4
					}
				}

				switchEnd := len(fn.ValueChunk().Code)
				for i, jump := range matchJumps {
					if jump >= 0 {
						fn.ValueChunk().PatchUint32(uint32(jump), uint32(entryPositions[i]))
					}
				}
				if defaultIndex >= 0 {
					fn.ValueChunk().PatchUint32(uint32(noMatchJump), uint32(entryPositions[defaultIndex]))
				} else {
					fn.ValueChunk().PatchUint32(uint32(noMatchJump), uint32(switchEnd))
				}
				for i, jump := range fallJumps {
					if jump >= 0 {
						fn.ValueChunk().PatchUint32(uint32(jump), uint32(contentPositions[i+1]))
					}
				}

				for _, breakpoint := range currentLoop().points {
					if breakpoint.type_ == BREAKPOINT_BREAK {
						fn.ValueChunk().PatchUint32(uint32(breakpoint.position), uint32(switchEnd))
					}
				}
				popLoop()
			}
		}
	})
}

func parseForDotDotLoopVariable(current *parser.Node, symbolTable *FunctionScope, fn object.Callable) {
	withAstTracking(current.Left, fn, func() {
		current = current.Left
		switch current.Type {
		case parser.NODE_VARIABLE_DECLARATION:
			{
				for _, declaration := range current.Declarations {
					switch declaration.Identifier.Type {
					case parser.NODE_IDENTIFIER:
						{
							variable, _ := symbolTable.findVariable(declaration.Identifier.Name)
							if variable.scope == HEAP {
								fn.ValueChunk().EmitBytes(chunk.OP_SET_HEAP_VAR, uint8(variable.slot))
							} else {
								fn.ValueChunk().EmitBytes(chunk.OP_SET_LOCAL, uint8(variable.slot))
							}
							return
						}
					case parser.NODE_OBJECT_PATTERN:
						{
							pattern := declaration.Identifier
							for i, prop := range pattern.Properties {
								var defineOp uint8
								k := prop.Key.Name
								v := prop.Value.(*parser.Node).Name

								property, _ := symbolTable.findVariable(v)

								switch property.scope {
								case LOCAL:
									defineOp = chunk.OP_SET_LOCAL
								case GLOBAL:
									defineOp = chunk.OP_SET_GLOBAL
								case HEAP:
									defineOp = chunk.OP_SET_HEAP_VAR
								}
								// GET_MEMBER pops the object
								if i < len(pattern.Properties)-1 {
									fn.ValueChunk().EmitByte(chunk.OP_PUSH_CURRENT)
								}

								if prop.Shorthand {
									fn.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(v))))
								} else {
									fn.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(native.NewLightString(k))))

								}

								fn.ValueChunk().EmitBytes(chunk.OP_GET_OBJECT_MEMBER, defineOp, uint8(property.slot))

							}
							return
						}
					case parser.NODE_ARRAY_PATTERN:
						{
							pattern := declaration.Identifier
							fn.ValueChunk().EmitBytes(chunk.OP_GET_ITERATOR, ITERATOR_FOR_OF)
							for _, element := range pattern.Elements {
								if element == nil {
									fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_NEXT, chunk.OP_POP)
									continue
								}
								if element.Type == parser.NODE_REST_ELEMENT {
									fn.ValueChunk().EmitByte(chunk.OP_ITERATOR_REST)
									variable, _ := symbolTable.findVariable(element.Argument.Name)
									emitSetVariable(fn, variable)
									continue
								}
								fn.ValueChunk().EmitByte(chunk.OP_ITERATOR_NEXT)
								fn.ValueChunk().EmitByte(chunk.OP_ITERATOR_VALUE_OR_UNDEFINED)
								variable, _ := symbolTable.findVariable(element.Name)
								emitSetVariable(fn, variable)
							}
							fn.ValueChunk().EmitByte(chunk.OP_ITERATOR_CLOSE_POP)
							return
						}
					default:
						parser.PrintNode(current)
						panic("unsupported for of variable declaration")
					}
				}
			}
		}
	})
}

func argumentIsPromise(current *parser.Node) bool {
	switch current.Type {
	case parser.NODE_NEW_EXPRESSION:
		{
			if current.Callee.Type == parser.NODE_IDENTIFIER {
				return current.Callee.Name == native.PROMISE_CONSTRUCTOR_NAME
			}
		}
	}
	return false
}

func assignmentOpcode(operator parser.AssignmentOperator) uint8 {
	switch operator {
	case parser.PLUS_ASSIGN:
		return chunk.OP_ADD
	case parser.MINUS_ASSIGN:
		return chunk.OP_SUBTRACT
	case parser.MULTIPLY_ASSIGN:
		return chunk.OP_MULTIPLY
	case parser.DIVIDE_ASSIGN:
		return chunk.OP_DIVIDE
	case parser.MODULUS_ASSIGN:
		return chunk.OP_MODULO
	case parser.EXPONENTIATION_ASSIGN:
		return chunk.OP_EXPONENTIATION
	case parser.LEFT_SHIFT_ASSIGN:
		return chunk.OP_LEFT_SHIFT
	case parser.RIGHT_SHIFT_ASSIGN:
		return chunk.OP_RIGHT_SHIFT
	case parser.UNSIGNED_RIGHT_SHIFT_ASSIGN:
		return chunk.OP_UNSIGNED_RIGHT_SHIFT
	case parser.BITWISE_AND_ASSIGN:
		return chunk.OP_B_AND
	case parser.BITWISE_OR_ASSIGN:
		return chunk.OP_B_OR
	case parser.BITWISE_XOR_ASSIGN:
		return chunk.OP_B_XOR
	}
	return chunk.OP_ADD
}

func emitAssignmentTarget(node *parser.Node, symbolTable *FunctionScope, fn object.Callable) {
	if node == nil {
		return
	}
	switch node.Type {
	case parser.NODE_IDENTIFIER:
		variable, _ := symbolTable.findVariable(node.Name)
		if variable != nil {
			emitSetVariable(fn, variable)
		}
	case parser.NODE_MEMBER_EXPRESSION:
		fn.ValueChunk().EmitByte(chunk.OP_SAVE_COMPLETION)
		generateByteCode(node.Object, symbolTable, fn)
		if node.Computed {
			generateByteCode(node.Property, symbolTable, fn)
			fn.ValueChunk().EmitByte(chunk.OP_TO_PROPERTY_KEY)
		} else {
			key := heap.Allocate(native.NewLightString(node.Property.Name))
			fn.ValueChunk().WriteConstant(value.EncodeHandle(key))
		}
		fn.ValueChunk().EmitBytes(chunk.OP_RESTORE_COMPLETION, chunk.OP_SET_MEMBER_RETURN, chunk.OP_POP)
	}
}

func generatePatternAssignment(pattern, right *parser.Node, symbolTable *FunctionScope, fn object.Callable) {
	if right == nil {
		fn.ValueChunk().EmitByte(chunk.OP_PUSH_UNDEFINED)
	} else {
		popReturnValue.push(false)
		generateByteCode(right, symbolTable, fn)
		popReturnValue.pop()
	}
	fn.ValueChunk().EmitByte(chunk.OP_DUP)

	var assign func(*parser.Node)
	assign = func(node *parser.Node) {
		if node == nil {
			return
		}
		switch node.Type {
		case parser.NODE_IDENTIFIER, parser.NODE_MEMBER_EXPRESSION:
			emitAssignmentTarget(node, symbolTable, fn)
		case parser.NODE_ASSIGNMENT_PATTERN:
			fn.ValueChunk().EmitByte(chunk.OP_DUP)
			fn.ValueChunk().EmitBytes(chunk.OP_JUMP_IF_NOT_UNDEFINED, 0, 0, 0, 0)
			skipJump := len(fn.ValueChunk().Code) - 4
			fn.ValueChunk().EmitByte(chunk.OP_POP)
			popReturnValue.push(false)
			generateByteCode(node.Right, symbolTable, fn)
			popReturnValue.pop()
			fn.ValueChunk().PatchUint32(uint32(skipJump), uint32(len(fn.ValueChunk().Code)))
			assign(node.Left)
		case parser.NODE_REST_ELEMENT:
			assign(node.Argument)
		case parser.NODE_ARRAY_PATTERN:
			fn.ValueChunk().EmitBytes(chunk.OP_GET_ITERATOR, ITERATOR_FOR_OF)
			for _, element := range node.Elements {
				if element == nil {
					fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_NEXT, chunk.OP_POP)
					continue
				}
				if element.Type == parser.NODE_REST_ELEMENT {
					fn.ValueChunk().EmitByte(chunk.OP_ITERATOR_REST)
					assign(element.Argument)
					continue
				}
				fn.ValueChunk().EmitByte(chunk.OP_ITERATOR_NEXT)
				fn.ValueChunk().EmitByte(chunk.OP_ITERATOR_VALUE_OR_UNDEFINED)
				assign(element)
			}
			fn.ValueChunk().EmitByte(chunk.OP_ITERATOR_CLOSE_POP)
		case parser.NODE_OBJECT_PATTERN:
			hasRest := false
			for _, property := range node.Properties {
				if property.Type == parser.NODE_REST_ELEMENT {
					hasRest = true
				}
			}
			excludedCount := 0
			for _, property := range node.Properties {
				switch property.Type {
				case parser.NODE_PROPERTY:
					if hasRest {
						excludedCount++
						if property.Computed {
							popReturnValue.push(false)
							generateByteCode(property.Key, symbolTable, fn)
							popReturnValue.pop()
							fn.ValueChunk().EmitByte(chunk.OP_TO_PROPERTY_KEY)
						} else {
							key := value.EncodeHandle(heap.Allocate(native.NewLightString(objectPropertyKeyName(property.Key))))
							fn.ValueChunk().WriteConstant(key)
						}
						fn.ValueChunk().EmitBytes(chunk.OP_DUP2, chunk.OP_GET_OBJECT_MEMBER)
						if valueNode, ok := property.Value.(*parser.Node); ok {
							assign(valueNode)
						}
					} else {
						fn.ValueChunk().EmitByte(chunk.OP_PUSH_CURRENT)
						if property.Computed {
							popReturnValue.push(false)
							generateByteCode(property.Key, symbolTable, fn)
							popReturnValue.pop()
							fn.ValueChunk().EmitByte(chunk.OP_TO_PROPERTY_KEY)
						} else {
							key := value.EncodeHandle(heap.Allocate(native.NewLightString(objectPropertyKeyName(property.Key))))
							fn.ValueChunk().WriteConstant(key)
						}
						fn.ValueChunk().EmitByte(chunk.OP_GET_OBJECT_MEMBER)
						if valueNode, ok := property.Value.(*parser.Node); ok {
							assign(valueNode)
						}
					}
				case parser.NODE_REST_ELEMENT:
					if hasRest {
						fn.ValueChunk().EmitBytes(chunk.OP_CREATE_REST_OBJECT_DYNAMIC, uint8(excludedCount))
					} else {
						fn.ValueChunk().EmitBytes(chunk.OP_PUSH_CURRENT, chunk.OP_CREATE_REST_OBJECT, 0)
					}
					if property.Argument != nil {
						assign(property.Argument)
					}
				}
			}
			if !hasRest {
				fn.ValueChunk().EmitByte(chunk.OP_POP)
			}
		}
	}
	assign(pattern)
}

func collectPatternTargets(node *parser.Node, targets *[]*parser.Node) {
	if node == nil {
		return
	}
	switch node.Type {
	case parser.NODE_IDENTIFIER:
		*targets = append(*targets, node)
	case parser.NODE_ASSIGNMENT_PATTERN:
		collectPatternTargets(node.Left, targets)
	case parser.NODE_REST_ELEMENT:
		collectPatternTargets(node.Argument, targets)
	case parser.NODE_ARRAY_PATTERN:
		for _, element := range node.Elements {
			collectPatternTargets(element, targets)
		}
	case parser.NODE_OBJECT_PATTERN:
		for _, property := range node.Properties {
			if property.Type == parser.NODE_PROPERTY {
				if value, ok := property.Value.(*parser.Node); ok {
					collectPatternTargets(value, targets)
				}
			} else if property.Type == parser.NODE_REST_ELEMENT {
				collectPatternTargets(property.Argument, targets)
			}
		}
	}
}

func patternValueNode(node *parser.Node) *parser.Node {
	if node != nil && node.Type == parser.NODE_ASSIGNMENT_PATTERN {
		return node.Left
	}
	return node
}

func emitSetVariable(fn object.Callable, variable *Variable) {
	var op uint8
	switch variable.scope {
	case GLOBAL:
		op = chunk.OP_SET_GLOBAL
	case LOCAL:
		op = chunk.OP_SET_LOCAL
	case HEAP:
		op = chunk.OP_SET_HEAP_VAR
	case THIS:
		op = chunk.OP_SET_LOCAL
	}
	fn.ValueChunk().EmitBytes(op, variableOperand(variable))
}

func generatePatternDeclaration(pattern, initializer *parser.Node, symbolTable *FunctionScope, fn object.Callable) {
	targets := []*parser.Node{}
	collectPatternTargets(pattern, &targets)

	locals := []*Variable{}
	heaps := []*Variable{}
	globals := []*Variable{}
	seen := map[*Variable]bool{}
	for _, target := range targets {
		variable, _ := symbolTable.findVariable(target.Name)
		if variable == nil || seen[variable] {
			continue
		}
		seen[variable] = true
		switch variable.scope {
		case LOCAL:
			locals = append(locals, variable)
		case HEAP:
			heaps = append(heaps, variable)
		case GLOBAL:
			globals = append(globals, variable)
		}
	}
	slices.SortFunc(locals, func(a, b *Variable) int { return cmp.Compare(a.slot, b.slot) })
	slices.SortFunc(heaps, func(a, b *Variable) int { return cmp.Compare(a.slot, b.slot) })
	slices.SortFunc(globals, func(a, b *Variable) int { return cmp.Compare(a.slot, b.slot) })

	for range locals {
		fn.ValueChunk().EmitBytes(chunk.OP_PUSH_UNDEFINED, chunk.OP_DEFINE_LOCAL)
	}
	for _, variable := range heaps {
		fn.ValueChunk().EmitBytes(chunk.OP_PUSH_UNDEFINED, chunk.OP_DEFINE_HEAP_VAR_AT, uint8(variable.slot))
	}
	for _, variable := range globals {
		fn.ValueChunk().EmitBytes(chunk.OP_PUSH_UNDEFINED, chunk.OP_SET_GLOBAL, uint8(variable.slot))
	}

	if initializer != nil {
		if initializer.Type == parser.NODE_IDENTIFIER {
			variable, _ := symbolTable.findVariable(initializer.Name)
			if variable != nil {
				switch variable.scope {
				case GLOBAL:
					fn.ValueChunk().EmitBytes(chunk.OP_GET_GLOBAL, variableOperand(variable))
				case LOCAL:
					fn.ValueChunk().EmitBytes(chunk.OP_GET_LOCAL, variableOperand(variable))
				case HEAP:
					fn.ValueChunk().EmitBytes(chunk.OP_GET_HEAP_VAR, variableOperand(variable))
				}
			} else {
				fn.ValueChunk().EmitByte(chunk.OP_PUSH_UNDEFINED)
			}
		} else {
			popReturnValue.push(false)
			generateByteCode(initializer, symbolTable, fn)
			popReturnValue.pop()
		}
	} else {
		fn.ValueChunk().EmitByte(chunk.OP_PUSH_UNDEFINED)
	}

	var assign func(*parser.Node)
	assign = func(node *parser.Node) {
		if node == nil {
			return
		}
		switch node.Type {
		case parser.NODE_IDENTIFIER:
			variable, _ := symbolTable.findVariable(node.Name)
			if variable != nil {
				emitSetVariable(fn, variable)
			}
		case parser.NODE_ASSIGNMENT_PATTERN:
			fn.ValueChunk().EmitByte(chunk.OP_DUP)
			fn.ValueChunk().EmitBytes(chunk.OP_JUMP_IF_NOT_UNDEFINED, 0, 0, 0, 0)
			skipJump := len(fn.ValueChunk().Code) - 4
			fn.ValueChunk().EmitByte(chunk.OP_POP)
			popReturnValue.push(false)
			generateByteCode(node.Right, symbolTable, fn)
			popReturnValue.pop()
			fn.ValueChunk().PatchUint32(uint32(skipJump), uint32(len(fn.ValueChunk().Code)))
			assign(node.Left)
		case parser.NODE_REST_ELEMENT:
			assign(node.Argument)
		case parser.NODE_ARRAY_PATTERN:
			fn.ValueChunk().EmitBytes(chunk.OP_GET_ITERATOR, ITERATOR_FOR_OF)
			for _, element := range node.Elements {
				if element == nil {
					fn.ValueChunk().EmitBytes(chunk.OP_ITERATOR_NEXT, chunk.OP_POP)
					continue
				}
				if element.Type == parser.NODE_REST_ELEMENT {
					fn.ValueChunk().EmitByte(chunk.OP_ITERATOR_REST)
					assign(element.Argument)
					continue
				}
				fn.ValueChunk().EmitByte(chunk.OP_ITERATOR_NEXT)
				fn.ValueChunk().EmitByte(chunk.OP_ITERATOR_VALUE_OR_UNDEFINED)
				assign(element)
			}
			fn.ValueChunk().EmitByte(chunk.OP_ITERATOR_CLOSE_POP)
		case parser.NODE_OBJECT_PATTERN:
			hasRest := false
			for _, property := range node.Properties {
				if property.Type == parser.NODE_REST_ELEMENT {
					hasRest = true
				}
			}
			excluded := []value.Value{}
			excludedCount := 0
			for _, property := range node.Properties {
				switch property.Type {
				case parser.NODE_PROPERTY:
					if hasRest {
						excludedCount++
						if property.Computed {
							popReturnValue.push(false)
							generateByteCode(property.Key, symbolTable, fn)
							fn.ValueChunk().EmitByte(chunk.OP_TO_PROPERTY_KEY)
							popReturnValue.pop()
						} else {
							key := value.EncodeHandle(heap.Allocate(native.NewLightString(objectPropertyKeyName(property.Key))))
							fn.ValueChunk().WriteConstant(key)
						}
						fn.ValueChunk().EmitByte(chunk.OP_DUP2)
						fn.ValueChunk().EmitByte(chunk.OP_GET_OBJECT_MEMBER)
						if valueNode, ok := property.Value.(*parser.Node); ok {
							assign(valueNode)
						}
					} else {
						fn.ValueChunk().EmitByte(chunk.OP_PUSH_CURRENT)
						if property.Computed {
							popReturnValue.push(false)
							generateByteCode(property.Key, symbolTable, fn)
							fn.ValueChunk().EmitByte(chunk.OP_TO_PROPERTY_KEY)
							popReturnValue.pop()
						} else {
							key := value.EncodeHandle(heap.Allocate(native.NewLightString(objectPropertyKeyName(property.Key))))
							fn.ValueChunk().WriteConstant(key)
						}
						fn.ValueChunk().EmitByte(chunk.OP_GET_OBJECT_MEMBER)
						if valueNode, ok := property.Value.(*parser.Node); ok {
							assign(valueNode)
						}
						if !property.Computed {
							key := value.EncodeHandle(heap.Allocate(native.NewLightString(objectPropertyKeyName(property.Key))))
							excluded = append(excluded, key)
						}
					}
				case parser.NODE_REST_ELEMENT:
					if hasRest {
						fn.ValueChunk().EmitBytes(chunk.OP_CREATE_REST_OBJECT_DYNAMIC, uint8(excludedCount))
					} else {
						fn.ValueChunk().EmitByte(chunk.OP_PUSH_CURRENT)
						fn.ValueChunk().EmitByte(chunk.OP_CREATE_REST_OBJECT)
						fn.ValueChunk().EmitByte(uint8(len(excluded)))
						for _, key := range excluded {
							fn.ValueChunk().EmitByte(fn.ValueChunk().AddConstant(key))
						}
					}
					if property.Argument != nil {
						assign(property.Argument)
					}
				}
			}
			if !hasRest {
				fn.ValueChunk().EmitByte(chunk.OP_POP)
			}
		}
	}
	assign(pattern)
	markPatternDeclared(pattern, symbolTable)
}

func objectPropertyKeyName(node *parser.Node) string {
	if node == nil {
		return ""
	}
	switch value := node.Value.(type) {
	case []byte:
		return string(value)
	case string:
		return value
	case float64:
		return formatJSNumber(value)
	case int:
		return fmt.Sprintf("%d", value)
	case int64:
		return fmt.Sprintf("%d", value)
	case bool:
		if value {
			return "true"
		}
		return "false"
	}
	return node.Name
}

func formatJSNumber(n float64) string {
	return nativeNumberStringForCompiler(n)
}

func nativeNumberStringForCompiler(n float64) string {
	if math.IsNaN(n) {
		return "NaN"
	}
	if math.IsInf(n, 1) {
		return "Infinity"
	}
	if math.IsInf(n, -1) {
		return "-Infinity"
	}
	if n == 0 {
		return "0"
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}

func isStrictFunction(fn object.Callable) bool {
	if fnObj, ok := fn.(*object.ObjFunction); ok {
		return module || fnObj.Strict
	}
	return module
}

func markPatternDeclared(node *parser.Node, symbolTable *FunctionScope) {
	if node == nil {
		return
	}
	switch node.Type {
	case parser.NODE_IDENTIFIER:
		if variable, _ := symbolTable.findVariable(node.Name); variable != nil {
			variable.declared = true
		}
	case parser.NODE_ASSIGNMENT_PATTERN:
		markPatternDeclared(node.Left, symbolTable)
	case parser.NODE_REST_ELEMENT:
		markPatternDeclared(node.Argument, symbolTable)
	case parser.NODE_ARRAY_PATTERN:
		for _, element := range node.Elements {
			markPatternDeclared(element, symbolTable)
		}
	case parser.NODE_OBJECT_PATTERN:
		for _, property := range node.Properties {
			if property.Type == parser.NODE_PROPERTY {
				if value, ok := property.Value.(*parser.Node); ok {
					markPatternDeclared(value, symbolTable)
				}
			} else if property.Type == parser.NODE_REST_ELEMENT {
				markPatternDeclared(property.Argument, symbolTable)
			}
		}
	}
}

func nodeHasUseStrict(node *parser.Node) bool {
	if node == nil || node.BodyNode == nil {
		return false
	}
	for _, statement := range node.BodyNode.Body {
		if statement.Directive != "" {
			return statement.Directive == "use strict"
		}
		return false
	}
	return false
}

func parameterName(node *parser.Node) string {
	if node == nil {
		return ""
	}
	switch node.Type {
	case parser.NODE_ASSIGNMENT_PATTERN:
		return parameterName(node.Left)
	case parser.NODE_REST_ELEMENT:
		return parameterName(node.Argument)
	}
	return node.Name
}

func emitParameterDefaults(params []*parser.Node, symbolTable *FunctionScope, fn object.Callable) {
	for _, param := range params {
		if param == nil || param.Type != parser.NODE_ASSIGNMENT_PATTERN {
			continue
		}
		if param.Left == nil || param.Left.Type != parser.NODE_IDENTIFIER {
			continue
		}
		variable, _ := symbolTable.findVariable(param.Left.Name)
		if variable == nil {
			continue
		}
		getOp := chunk.OP_GET_LOCAL
		setOp := chunk.OP_SET_LOCAL
		if variable.scope == HEAP {
			getOp = chunk.OP_GET_HEAP_VAR
			setOp = chunk.OP_SET_HEAP_VAR
		}
		fn.ValueChunk().EmitBytes(getOp, variableOperand(variable), chunk.OP_DUP)
		fn.ValueChunk().EmitBytes(chunk.OP_JUMP_IF_NOT_UNDEFINED, 0, 0, 0, 0)
		skipJump := len(fn.ValueChunk().Code) - 4
		fn.ValueChunk().EmitByte(chunk.OP_POP)
		popReturnValue.push(false)
		generateByteCode(param.Right, symbolTable, fn)
		popReturnValue.pop()
		fn.ValueChunk().EmitBytes(setOp, variableOperand(variable))
		fn.ValueChunk().PatchUint32(uint32(skipJump), uint32(len(fn.ValueChunk().Code)))
	}
}

func markParamsDeclared(params []*parser.Node, symbolTable *FunctionScope) {
	for _, param := range params {
		markPatternDeclared(param, symbolTable)
	}
}

func initializerLocalVariableCount(node *parser.Node, symbolTable *FunctionScope) int {
	if node == nil || node.Type != parser.NODE_VARIABLE_DECLARATION {
		return 0
	}
	count := 0
	for _, declaration := range node.Declarations {
		if declaration.Identifier.Type == parser.NODE_IDENTIFIER {
			if variable, _ := symbolTable.findVariable(declaration.Identifier.Name); variable != nil && variable.scope != HEAP && variable.type_ != CATCH_PARAM {
				count++
			}
		}
	}
	return count
}

func compactBlockSlots(table *FunctionScope, oldSlot int) {
	for block := table.block; block != nil; block = block.parent {
		for _, v := range block.vars {
			if v.scope != HEAP && v.slot > oldSlot {
				v.slot--
			}
		}
	}
	for _, v := range table.vars {
		if v.scope != HEAP && v.slot > oldSlot {
			v.slot--
		}
	}
}

func blockLocalVarCount(block *BlockScope) int {
	if block == nil {
		return 0
	}
	count := 0
	for _, variable := range block.vars {
		if variable.scope != HEAP && variable.type_ != CATCH_PARAM {
			count++
		}
	}
	return count
}

func blockHasHeapVars(block *BlockScope) bool {
	if block == nil {
		return false
	}
	for _, variable := range block.vars {
		if variable.scope == HEAP {
			return true
		}
	}
	return false
}

func sortedBlockVariables(block *BlockScope) []*Variable {
	variables := make([]*Variable, 0, len(block.vars))
	for _, variable := range block.vars {
		variables = append(variables, variable)
	}
	slices.SortFunc(variables, func(a, b *Variable) int {
		return cmp.Compare(a.slot, b.slot)
	})
	return variables
}

func variableName(block *BlockScope, target *Variable) string {
	for name, variable := range block.vars {
		if variable == target {
			return name
		}
	}
	return ""
}

func loopBindingNameSet(node *parser.Node) map[string]bool {
	names := map[string]bool{}
	if node == nil || node.Type != parser.NODE_VARIABLE_DECLARATION {
		return names
	}
	var collect func(*parser.Node)
	collect = func(n *parser.Node) {
		if n == nil {
			return
		}
		switch n.Type {
		case parser.NODE_IDENTIFIER:
			names[n.Name] = true
		case parser.NODE_ASSIGNMENT_PATTERN:
			collect(n.Left)
		case parser.NODE_REST_ELEMENT:
			collect(n.Argument)
		case parser.NODE_ARRAY_PATTERN:
			for _, element := range n.Elements {
				collect(element)
			}
		case parser.NODE_OBJECT_PATTERN:
			for _, property := range n.Properties {
				if property.Type == parser.NODE_PROPERTY {
					if value, ok := property.Value.(*parser.Node); ok {
						collect(value)
					}
				} else if property.Type == parser.NODE_REST_ELEMENT {
					collect(property.Argument)
				}
			}
		}
	}
	for _, declaration := range node.Declarations {
		collect(declaration.Identifier)
	}
	return names
}

func bodyLocalVariableCount(block *BlockScope, bindings map[string]bool) int {
	count := 0
	for name, variable := range block.vars {
		if bindings[name] {
			continue
		}
		if variable.scope != HEAP && variable.type_ != CATCH_PARAM {
			count++
		}
	}
	return count
}

func variableOperand(variable *Variable) uint8 {
	if variable.type_ == CATCH_PARAM {
		return uint8(variable.catchId)
	}
	return uint8(variable.slot)
}

func emitCatchParam(param *parser.Node, symbolTable *FunctionScope, fn object.Callable) {
	if param.Type == parser.NODE_IDENTIFIER {
		if id, found := CATCH_IDS[param]; found {
			fn.ValueChunk().EmitBytes(chunk.OP_CATCH_BIND, uint8(id))
			return
		}
	}
	fn.ValueChunk().EmitByte(chunk.OP_DEFINE_LOCAL)
}

func emitThrow(fn object.Callable, name, message string) {
	err := native.NewError()
	err.SetMember(native.KEY_MESSAGE, value.EncodeHandle(heap.Allocate(native.NewLightString(message))))
	err.SetMember(native.KEY_NAME, value.EncodeHandle(heap.Allocate(native.NewLightString(name))))
	fn.ValueChunk().WriteConstant(value.EncodeHandle(heap.Allocate(err)))
	fn.ValueChunk().EmitByte(chunk.OP_THROW)
}

func isDecrement(node *parser.Node) bool {
	return node.UpdateOperator == parser.DECREMENT || node.UnaryOperator == parser.UnaryOperator("--")
}

func referenceIsDiscarded(node *parser.Node) bool {
	if node == nil {
		return false
	}
	switch node.Type {
	case parser.NODE_SEQUENCE_EXPRESSION:
		return true
	case parser.NODE_PARENTHESIZED_EXPRESSION:
		return referenceIsDiscarded(node.Expression)
	}
	return false
}

func containsOptional(node *parser.Node) bool {
	if node == nil {
		return false
	}
	switch node.Type {
	case parser.NODE_MEMBER_EXPRESSION, parser.NODE_CALL_EXPRESSION:
		if node.Optional {
			return true
		}
		if containsOptional(node.Object) {
			return true
		}
		return containsOptional(node.Callee)
	case parser.NODE_CHAIN_EXPRESSION, parser.NODE_PARENTHESIZED_EXPRESSION:
		return containsOptional(node.Expression)
	}
	return false
}

func argumentsContainSpread(arguments []*parser.Node) bool {	for _, node := range arguments {
		switch node.Type {
		case parser.NODE_SPREAD_ELEMENT:
			{
				return true
			}
		}
	}

	return false
}

func calleeIsPromiseFunction(current *parser.Node) bool {
	switch current.Type {
	case parser.NODE_MEMBER_EXPRESSION:
		if current.Object.Type == parser.NODE_IDENTIFIER && current.Property.Type == parser.NODE_IDENTIFIER {
			return current.Object.Name == native.PROMISE_CONSTRUCTOR_NAME && current.Property.Name == "resolve"
		}
	}
	return false
}
