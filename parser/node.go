package parser

import (
	"math/big"
	"regexp"
)

type SourceType int

const (
	TYPE_SCRIPT SourceType = iota
	TYPE_MODULE
)

type NodeType int

const (
	NODE_IDENTIFIER NodeType = iota
	NODE_LITERAL
	NODE_PROGRAM
	NODE_FUNCTION
	NODE_EXPRESSION_STATEMENT
	NODE_BLOCK_STATEMENT
	NODE_EMPTY_STATEMENT
	NODE_DEBUGGER_STATEMENT
	NODE_WITH_STATEMENT
	NODE_RETURN_STATEMENT
	NODE_LABELED_STATEMENT
	NODE_BREAK_STATEMENT
	NODE_CONTINUE_STATEMENT
	NODE_IF_STATEMENT
	NODE_SWITCH_STATEMENT
	NODE_SWITCH_CASE
	NODE_THROW_STATEMENT
	NODE_TRY_STATEMENT
	NODE_CATCH_CLAUSE
	NODE_WHILE_STATEMENT
	NODE_DO_WHILE_STATEMENT
	NODE_FOR_STATEMENT
	NODE_FOR_IN_STATEMENT
	NODE_FUNCTION_DECLARATION
	NODE_VARIABLE_DECLARATION
	NODE_VARIABLE_DECLARATOR
	NODE_THIS_EXPRESSION
	NODE_ARRAY_EXPRESSION
	NODE_OBJECT_EXPRESSION
	NODE_PROPERTY
	NODE_FUNCTION_EXPRESSION
	NODE_UNARY_EXPRESSION
	NODE_UPDATE_EXPRESSION
	NODE_BINARY_EXPRESSION
	NODE_ASSIGNMENT_EXPRESSION
	NODE_LOGICAL_EXPRESSION
	NODE_MEMBER_EXPRESSION
	NODE_CONDITIONAL_EXPRESSION
	NODE_CALL_EXPRESSION
	NODE_NEW_EXPRESSION
	NODE_SEQUENCE_EXPRESSION
	NODE_FOR_OF_STATEMENT
	NODE_SUPER
	NODE_SPREAD_ELEMENT
	NODE_ARROW_FUNCTION_EXPRESSION
	NODE_YIELD_EXPRESSION
	NODE_TEMPLATE_LITERAL
	NODE_TAGGED_TEMPLATE_EXPRESSION
	NODE_TEMPLATE_ELEMENT
	NODE_ASSIGNMENT_PROPERTY
	NODE_OBJECT_PATTERN
	NODE_ARRAY_PATTERN
	NODE_REST_ELEMENT
	NODE_ASSIGNMENT_PATTERN
	NODE_CLASS
	NODE_CLASS_BODY
	NODE_METHOD_DEFINITION
	NODE_CLASS_DECLARATION
	NODE_CLASS_EXPRESSION
	NODE_META_PROPERTY
	NODE_IMPORT_DECLARATION
	NODE_IMPORT_SPECIFIER
	NODE_IMPORT_DEFAULT_SPECIFIER
	NODE_IMPORT_NAMESPACE_SPECIFIER
	NODE_IMPORT_ATTRIBUTE
	NODE_EXPORT_NAMED_DECLARATION
	NODE_EXPORT_SPECIFIER
	NODE_ANONYMOUS_FUNCTION_DECLARATION
	NODE_ANONYMOUS_CLASS_DECLARATION
	NODE_EXPORT_DEFAULT_DECLARATION
	NODE_EXPORT_ALL_DECLARATION
	NODE_AWAIT_EXPRESSION
	NODE_CHAIN_EXPRESSION
	NODE_IMPORT_EXPRESSION
	NODE_PARENTHESIZED_EXPRESSION
	NODE_PROPERTY_DEFINITION
	NODE_PRIVATE_IDENTIFIER
	NODE_STATIC_BLOCK
	NODE_UNTYPED
)

type Kind int

const (
	KIND_NOT_INITIALIZED Kind = iota
	KIND_DECLARATION_VAR
	KIND_DECLARATION_LET
	KIND_DECLARATION_CONST
	KIND_PROPERTY_GET
	KIND_PROPERTY_SET
	KIND_PROPERTY_INIT
	KIND_PROPERTY_METHOD
	KIND_CONSTRUCTOR
)

var kindStringMap = map[Kind]string{
	KIND_PROPERTY_GET: "get",
	KIND_PROPERTY_SET: "set",
}

type Regex struct {
	Pattern string
	Flags   string
}

type BinaryOperator string

const (
	EQUALS               BinaryOperator = "=="
	NOT_EQUALS           BinaryOperator = "!="
	STRICT_EQUALS        BinaryOperator = "==="
	STRICT_NOT_EQUALS    BinaryOperator = "!=="
	LESS_THAN            BinaryOperator = "<"
	LESS_THAN_EQUAL      BinaryOperator = "<="
	GREATER_THAN         BinaryOperator = ">"
	GREATER_THAN_EQUAL   BinaryOperator = ">="
	LEFT_SHIFT           BinaryOperator = "<<"
	RIGHT_SHIFT          BinaryOperator = ">>"
	UNSIGNED_RIGHT_SHIFT BinaryOperator = ">>>"
	PLUS                 BinaryOperator = "+"
	MINUS                BinaryOperator = "-"
	MULTIPLY             BinaryOperator = "*"
	DIVIDE               BinaryOperator = "/"
	MODULUS              BinaryOperator = "%"
	BITWISE_OR           BinaryOperator = "|"
	BITWISE_XOR          BinaryOperator = "^"
	BITWISE_AND          BinaryOperator = "&"
	IN                   BinaryOperator = "in"
	INSTANCEOF           BinaryOperator = "instanceof"
	EXPONENTIATION       BinaryOperator = "**"
)

type UnaryOperator string

const (
	UNARY_NEGATE      UnaryOperator = "-"
	UNARY_PLUS        UnaryOperator = "+"
	UNARY_NOT         UnaryOperator = "!"
	UNARY_BITWISE_NOT UnaryOperator = "~"
	UNARY_TYPEOF      UnaryOperator = "typeof"
	UNARY_VOID        UnaryOperator = "void"
	UNARY_DELETE      UnaryOperator = "delete"
)

type UpdateOperator string

const (
	INCREMENT UpdateOperator = "++"
	DECREMENT UpdateOperator = "--"
)

type AssignmentOperator string

const (
	ASSIGN                      AssignmentOperator = "="
	PLUS_ASSIGN                 AssignmentOperator = "+="
	MINUS_ASSIGN                AssignmentOperator = "-="
	MULTIPLY_ASSIGN             AssignmentOperator = "*="
	DIVIDE_ASSIGN               AssignmentOperator = "/="
	MODULUS_ASSIGN              AssignmentOperator = "%="
	LEFT_SHIFT_ASSIGN           AssignmentOperator = "<<="
	RIGHT_SHIFT_ASSIGN          AssignmentOperator = ">>="
	UNSIGNED_RIGHT_SHIFT_ASSIGN AssignmentOperator = ">>>="
	BITWISE_OR_ASSIGN           AssignmentOperator = "|="
	BITWISE_XOR_ASSIGN          AssignmentOperator = "^="
	BITWISE_AND_ASSIGN          AssignmentOperator = "&="
	EXPONENTIATION_ASSIGN       AssignmentOperator = "**="
	LOGICAL_OR_ASSIGN           AssignmentOperator = "||="
	LOGICAL_AND_ASSIGN          AssignmentOperator = "&&="
	NULLISH_ASSIGN              AssignmentOperator = "??="
)

type LogicalOperator string

const (
	LOGICAL_OR         LogicalOperator = "||"
	LOGICAL_AND        LogicalOperator = "&&"
	NULLISH_COALESCING LogicalOperator = "??"
)

type Node struct {
	// Base values
	start      int
	end        int
	type_      NodeType
	range_     [2]int
	location   *SourceLocation
	sourceFile *string
	name       string
	value      any // string, bool, float64, *regexp.Regexp, *big.Int, *Node

	raw                string
	regex              *Regex
	bigint             string
	body               []*Node // Statement | ModuleDeclaration
	bodyNode           *Node   // BlockStatement | Expression
	sourceType         SourceType
	identifier         *Node   // Identifier
	params             []*Node // Pattern
	isGenerator        bool
	isExpression       bool
	isAsync            bool
	expression         *Node // Expression | Literal
	directive          string
	delegate           bool
	object             *Node   // Expression
	argument           *Node   // Expression
	label              *Node   // Identifier
	test               *Node   // Expression
	consequent         *Node   // Statement
	alternate          *Node   // Statement
	discriminant       *Node   // Expression
	cases              []*Node // SwitchCase
	consequentSlice    []*Node // Statement (renamed to avoid conflict with Consequent)
	block              *Node   // BlockStatement
	handler            *Node   // CatchClause
	finalizer          *Node   // BlockStatement
	param              *Node   // Pattern
	initializer        *Node   // VariableDeclaration | Expression
	update             *Node   // Expression
	declarations       []*Node // VariableDeclarator
	kind               Kind
	elements           []*Node // Expression | SpreadElement
	properties         []*Node // Property | SpreadElement
	key                *Node   // Expression
	isMethod           bool
	shorthand          bool
	computed           bool
	unaryOperator      UnaryOperator
	prefix             bool
	updateOperator     UpdateOperator
	binaryOperator     BinaryOperator
	left               *Node // Expression | PrivateIdentifier
	rigth              *Node
	assignmentOperator AssignmentOperator
	logicalOperator    LogicalOperator
	memberProperty     *Node // Expression | PrivateIdentifier
	optional           bool
	callee             *Node   // Expression | Super
	arguments          []*Node // Expression | SpreadElement
	expressions        []*Node // Expression
	await              bool
	isDelegate         *bool
	quasis             []*Node // TemplateElement
	quasi              *Node   // TemplateLiteral
	tag                *Node   // Expression
	tail               bool
	tmplValue          *TemplateValue
	superClass         *Node // Expression
	isStatic           bool
	meta               *Node   // Identifier
	property           *Node   // Identifier
	specifierss        []*Node // ImportSpecifier | ImportDefaultSpecifier | ImportNamespaceSpecifier
	source             *Node   // Literal
	attributes         []*Node // ImportAttribute
	imported           *Node   // Identifier | Literal
	local              *Node   // Identifier
	declaration        *Node   // Declaration
	exported           *Node   // Identifier | Literal
	options            *Node   // Expression
}

type TemplateValue struct {
	Cooked *string
	Raw    string
}

func NewNode(parser *Parser, pos int, loc *Location) *Node {
	node := &Node{
		type_: NODE_UNTYPED,
		start: pos,
		end:   0,
	}

	if parser.options.Locations {
		node.location = NewSourceLocation(parser, loc, nil)
	}

	if parser.options.DirectSourceFile != nil {
		node.sourceFile = parser.options.DirectSourceFile
	}

	if parser.options.Ranges {
		node.range_ = [2]int{pos, 0}
	}
	return node
}

func (this *Parser) startNode() *Node {
	return NewNode(this, this.start, this.startLoc)
}

func (this *Parser) startNodeAt(pos int, loc *Location) *Node {
	return NewNode(this, pos, loc)
}

func (this *Parser) finishNodeAt(node *Node, finishType NodeType, pos int, loc *Location) {
	node.type_ = finishType
	node.end = pos
	if this.options.Locations {
		node.location.End = loc
	}

	if this.options.Ranges {
		node.range_[1] = pos
	}
}

func (this *Parser) finishNode(node *Node, finishType NodeType) *Node {
	this.finishNodeAt(node, finishType, this.LastTokEnd, this.LastTokEndLoc)
	return node
}

// A bit more explicit than javaScript :D
func (this *Parser) copyNode(node *Node) *Node {
	if node == nil {
		return nil
	}

	// Create a new Node and copy basic fields
	newNode := &Node{
		start:        node.start,
		end:          node.end,
		type_:        node.type_,
		range_:       node.range_, // Array is copied by value
		name:         node.name,
		raw:          node.raw,
		bigint:       node.bigint,
		sourceType:   node.sourceType,
		isGenerator:  node.isGenerator,
		isExpression: node.isExpression,
		isAsync:      node.isAsync,
		delegate:     node.delegate,
		isMethod:     node.isMethod,
		shorthand:    node.shorthand,
		computed:     node.computed,
		prefix:       node.prefix,
		optional:     node.optional,
		tail:         node.tail,
		isStatic:     node.isStatic,
	}

	// Copy SourceFile (*string)
	if node.sourceFile != nil {
		sourceFile := *node.sourceFile
		newNode.sourceFile = &sourceFile
	}

	// Copy Loc (*SourceLocation)
	if node.location != nil {
		loc := *node.location // Assuming SourceLocation has no pointers
		newNode.location = &loc
	}

	// Copy Regex (*Regex)
	if node.regex != nil {
		regex := *node.regex // Assuming Regex has no pointers
		newNode.regex = &regex
	}

	directive := node.directive
	newNode.directive = directive

	newNode.await = node.await

	// Copy IsDelegate (*bool)
	if node.isDelegate != nil {
		isDelegate := *node.isDelegate
		newNode.isDelegate = &isDelegate
	}

	// Copy UnaryOperator (*UnaryOperator)

	unaryOp := node.unaryOperator
	newNode.unaryOperator = unaryOp

	// Copy UpdateOperator (*UpdateOperator)

	updateOp := node.updateOperator
	newNode.updateOperator = updateOp

	// Copy BinaryOperator (*BinaryOperator)
	binaryOp := node.binaryOperator
	newNode.binaryOperator = binaryOp

	newNode.assignmentOperator = node.assignmentOperator
	newNode.logicalOperator = node.logicalOperator

	// Copy TmplValue (*TemplateValue)
	if node.tmplValue != nil {
		tmplValue := *node.tmplValue
		newNode.tmplValue = &tmplValue
	}

	// Copy Value (any)
	switch v := node.value.(type) {
	case *Node:
		newNode.value = this.copyNode(v)
	case *regexp.Regexp:
		if v != nil {
			// Create a new regexp with the same pattern
			newRegexp, err := regexp.Compile(v.String())
			if err == nil {
				newNode.value = newRegexp
			}
		}
	case *big.Int:
		if v != nil {
			newBigInt := new(big.Int).Set(v)
			newNode.value = newBigInt
		}
	case string, bool, float64:
		newNode.value = v // Direct copy for value types
	}

	// Copy single Node pointers
	newNode.bodyNode = this.copyNode(node.bodyNode)
	newNode.identifier = this.copyNode(node.identifier)
	newNode.bodyNode = this.copyNode(node.bodyNode)
	newNode.expression = this.copyNode(node.expression)
	newNode.object = this.copyNode(node.object)
	newNode.argument = this.copyNode(node.argument)
	newNode.label = this.copyNode(node.label)
	newNode.test = this.copyNode(node.test)
	newNode.consequent = this.copyNode(node.consequent)
	newNode.alternate = this.copyNode(node.alternate)
	newNode.discriminant = this.copyNode(node.discriminant)
	newNode.block = this.copyNode(node.block)
	newNode.handler = this.copyNode(node.handler)
	newNode.finalizer = this.copyNode(node.finalizer)
	newNode.param = this.copyNode(node.param)
	newNode.initializer = this.copyNode(node.initializer)
	newNode.update = this.copyNode(node.update)
	newNode.key = this.copyNode(node.key)
	newNode.left = this.copyNode(node.left)
	newNode.rigth = this.copyNode(node.rigth)
	newNode.memberProperty = this.copyNode(node.memberProperty)
	newNode.callee = this.copyNode(node.callee)
	newNode.tag = this.copyNode(node.tag)
	newNode.quasi = this.copyNode(node.quasi)
	newNode.superClass = this.copyNode(node.superClass)
	newNode.meta = this.copyNode(node.meta)
	newNode.property = this.copyNode(node.property)
	newNode.source = this.copyNode(node.source)
	newNode.imported = this.copyNode(node.imported)
	newNode.local = this.copyNode(node.local)
	newNode.declaration = this.copyNode(node.declaration)
	newNode.exported = this.copyNode(node.exported)
	newNode.options = this.copyNode(node.options)

	// Copy Node slices
	newNode.body = make([]*Node, len(node.body))
	for i, param := range node.body {
		newNode.body[i] = this.copyNode(param)
	}

	newNode.params = make([]*Node, len(node.params))
	for i, param := range node.params {
		newNode.params[i] = this.copyNode(param)
	}

	newNode.cases = make([]*Node, len(node.cases))
	for i, c := range node.cases {
		newNode.cases[i] = this.copyNode(c)
	}

	newNode.consequentSlice = make([]*Node, len(node.consequentSlice))
	for i, cons := range node.consequentSlice {
		newNode.consequentSlice[i] = this.copyNode(cons)
	}

	newNode.declarations = make([]*Node, len(node.declarations))
	for i, decl := range node.declarations {
		newNode.declarations[i] = this.copyNode(decl)
	}

	newNode.elements = make([]*Node, len(node.elements))
	for i, elem := range node.elements {
		newNode.elements[i] = this.copyNode(elem)
	}

	newNode.properties = make([]*Node, len(node.properties))
	for i, prop := range node.properties {
		newNode.properties[i] = this.copyNode(prop)
	}

	newNode.arguments = make([]*Node, len(node.arguments))
	for i, arg := range node.arguments {
		newNode.arguments[i] = this.copyNode(arg)
	}

	newNode.expressions = make([]*Node, len(node.expressions))
	for i, expr := range node.expressions {
		newNode.expressions[i] = this.copyNode(expr)
	}

	newNode.quasis = make([]*Node, len(node.quasis))
	for i, quasi := range node.quasis {
		newNode.quasis[i] = this.copyNode(quasi)
	}

	newNode.specifierss = make([]*Node, len(node.specifierss))
	for i, spec := range node.specifierss {
		newNode.specifierss[i] = this.copyNode(spec)
	}

	newNode.attributes = make([]*Node, len(node.attributes))
	for i, attr := range node.attributes {
		newNode.attributes[i] = this.copyNode(attr)
	}

	return newNode
}

/*
I think I can skip this?

	this.finishNodeAt = function(node, type, pos, loc) {
	  return finishNodeAt.call(this, node, type, pos, loc)
	}

*/
