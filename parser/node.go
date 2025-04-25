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

type DeclarationKind int

const (
	DECLARATION_KIND_NOT_INITIALIZED DeclarationKind = iota
	VAR
	LET
	CONST
)

type PropertyKind int

const (
	PROPERTY_KIND_NOT_INITIALIZED PropertyKind = iota
	GET
	SET
	INIT
)

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
	Start      int
	End        int
	Type       NodeType
	Range      [2]int
	Loc        *SourceLocation
	SourceFile *string

	Name               string
	Value              any // string, bool, float64, *regexp.Regexp, *big.Int, *Node
	Raw                string
	Regex              *Regex
	Bigint             string
	Body               []*Node // Statement | ModuleDeclaration
	BodyNode           *Node   // BlockStatement | Expression
	SourceType         SourceType
	Id                 *Node   // Identifier
	Params             []*Node // Pattern
	IsGenerator        bool
	IsExpression       bool
	IsAsync            bool
	Expression         *Node // Expression | Literal
	Directive          string
	Delegate           bool
	Object             *Node   // Expression
	Argument           *Node   // Expression
	Label              *Node   // Identifier
	Test               *Node   // Expression
	Consequent         *Node   // Statement
	Alternate          *Node   // Statement
	Discriminant       *Node   // Expression
	Cases              []*Node // SwitchCase
	ConsequentSlice    []*Node // Statement (renamed to avoid conflict with Consequent)
	Block              *Node   // BlockStatement
	Handler            *Node   // CatchClause
	Finalizer          *Node   // BlockStatement
	Param              *Node   // Pattern
	Init               *Node   // VariableDeclaration | Expression
	Update             *Node   // Expression
	Declarations       []*Node // VariableDeclarator
	DeclarationKind    DeclarationKind
	Elements           []*Node // Expression | SpreadElement
	Properties         []*Node // Property | SpreadElement
	Key                *Node   // Expression
	PropertyKind       PropertyKind
	IsMethod           bool
	Shorthand          bool
	Computed           bool
	UnaryOperator      *UnaryOperator
	Prefix             bool
	UpdateOperator     *UpdateOperator
	BinaryOperator     *BinaryOperator
	Left               *Node // Expression | PrivateIdentifier
	Rigth              *Node
	AssignmentOperator *AssignmentOperator
	LogicalOperator    *LogicalOperator
	MemberProperty     *Node // Expression | PrivateIdentifier
	Optional           bool
	Callee             *Node   // Expression | Super
	Arguments          []*Node // Expression | SpreadElement
	Expressions        []*Node // Expression
	Await              bool
	IsDelegate         *bool
	Quasis             []*Node // TemplateElement
	Tag                *Node   // Expression
	Quasi              *Node   // TemplateLiteral
	Tail               bool
	TmplValue          *TemplateValue
	SuperClass         *Node // Expression
	ClassBody          *Node // ClassBody
	IsStatic           bool
	Meta               *Node   // Identifier
	Property           *Node   // Identifier
	Specifiers         []*Node // ImportSpecifier | ImportDefaultSpecifier | ImportNamespaceSpecifier
	Source             *Node   // Literal
	Attributes         []*Node // ImportAttribute
	Imported           *Node   // Identifier | Literal
	Local              *Node   // Identifier
	Declaration        *Node   // Declaration
	Exported           *Node   // Identifier | Literal
	Options            *Node   // Expression
}

type TemplateValue struct {
	Cooked *string
	Raw    string
}

func NewNode(parser *Parser, pos int, loc *Location) *Node {
	node := &Node{
		Type:  NODE_UNTYPED,
		Start: pos,
		End:   0,
	}

	if parser.options.Locations {
		node.Loc = NewSourceLocation(parser, loc, nil)
	}

	if parser.options.DirectSourceFile != nil {
		node.SourceFile = parser.options.DirectSourceFile
	}

	if parser.options.Ranges {
		node.Range = [2]int{pos, 0}
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
	node.Type = finishType
	node.End = pos
	if this.options.Locations {
		node.Loc.End = loc
	}

	if this.options.Ranges {
		node.Range[1] = pos
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
		Start:        node.Start,
		End:          node.End,
		Type:         node.Type,
		Range:        node.Range, // Array is copied by value
		Name:         node.Name,
		Raw:          node.Raw,
		Bigint:       node.Bigint,
		SourceType:   node.SourceType,
		IsGenerator:  node.IsGenerator,
		IsExpression: node.IsExpression,
		IsAsync:      node.IsAsync,
		Delegate:     node.Delegate,
		IsMethod:     node.IsMethod,
		Shorthand:    node.Shorthand,
		Computed:     node.Computed,
		Prefix:       node.Prefix,
		Optional:     node.Optional,
		Tail:         node.Tail,
		IsStatic:     node.IsStatic,
	}

	// Copy SourceFile (*string)
	if node.SourceFile != nil {
		sourceFile := *node.SourceFile
		newNode.SourceFile = &sourceFile
	}

	// Copy Loc (*SourceLocation)
	if node.Loc != nil {
		loc := *node.Loc // Assuming SourceLocation has no pointers
		newNode.Loc = &loc
	}

	// Copy Regex (*Regex)
	if node.Regex != nil {
		regex := *node.Regex // Assuming Regex has no pointers
		newNode.Regex = &regex
	}

	directive := node.Directive
	newNode.Directive = directive

	newNode.Await = node.Await

	// Copy IsDelegate (*bool)
	if node.IsDelegate != nil {
		isDelegate := *node.IsDelegate
		newNode.IsDelegate = &isDelegate
	}

	// Copy UnaryOperator (*UnaryOperator)
	if node.UnaryOperator != nil {
		unaryOp := *node.UnaryOperator
		newNode.UnaryOperator = &unaryOp
	}

	// Copy UpdateOperator (*UpdateOperator)
	if node.UpdateOperator != nil {
		updateOp := *node.UpdateOperator
		newNode.UpdateOperator = &updateOp
	}

	// Copy BinaryOperator (*BinaryOperator)
	if node.BinaryOperator != nil {
		binaryOp := *node.BinaryOperator
		newNode.BinaryOperator = &binaryOp
	}

	// Copy AssignmentOperator (*AssignmentOperator)
	if node.AssignmentOperator != nil {
		assignOp := *node.AssignmentOperator
		newNode.AssignmentOperator = &assignOp
	}

	// Copy LogicalOperator (*LogicalOperator)
	if node.LogicalOperator != nil {
		logicalOp := *node.LogicalOperator
		newNode.LogicalOperator = &logicalOp
	}

	// Copy TmplValue (*TemplateValue)
	if node.TmplValue != nil {
		tmplValue := *node.TmplValue
		newNode.TmplValue = &tmplValue
	}

	// Copy Value (any)
	switch v := node.Value.(type) {
	case *Node:
		newNode.Value = this.copyNode(v)
	case *regexp.Regexp:
		if v != nil {
			// Create a new regexp with the same pattern
			newRegexp, err := regexp.Compile(v.String())
			if err == nil {
				newNode.Value = newRegexp
			}
		}
	case *big.Int:
		if v != nil {
			newBigInt := new(big.Int).Set(v)
			newNode.Value = newBigInt
		}
	case string, bool, float64:
		newNode.Value = v // Direct copy for value types
	}

	// Copy single Node pointers
	newNode.BodyNode = this.copyNode(node.BodyNode)
	newNode.Id = this.copyNode(node.Id)
	newNode.BodyNode = this.copyNode(node.BodyNode)
	newNode.Expression = this.copyNode(node.Expression)
	newNode.Object = this.copyNode(node.Object)
	newNode.Argument = this.copyNode(node.Argument)
	newNode.Label = this.copyNode(node.Label)
	newNode.Test = this.copyNode(node.Test)
	newNode.Consequent = this.copyNode(node.Consequent)
	newNode.Alternate = this.copyNode(node.Alternate)
	newNode.Discriminant = this.copyNode(node.Discriminant)
	newNode.Block = this.copyNode(node.Block)
	newNode.Handler = this.copyNode(node.Handler)
	newNode.Finalizer = this.copyNode(node.Finalizer)
	newNode.Param = this.copyNode(node.Param)
	newNode.Init = this.copyNode(node.Init)
	newNode.Update = this.copyNode(node.Update)
	newNode.Key = this.copyNode(node.Key)
	newNode.Left = this.copyNode(node.Left)
	newNode.Rigth = this.copyNode(node.Rigth)
	newNode.MemberProperty = this.copyNode(node.MemberProperty)
	newNode.Callee = this.copyNode(node.Callee)
	newNode.Tag = this.copyNode(node.Tag)
	newNode.Quasi = this.copyNode(node.Quasi)
	newNode.SuperClass = this.copyNode(node.SuperClass)
	newNode.ClassBody = this.copyNode(node.ClassBody)
	newNode.Meta = this.copyNode(node.Meta)
	newNode.Property = this.copyNode(node.Property)
	newNode.Source = this.copyNode(node.Source)
	newNode.Imported = this.copyNode(node.Imported)
	newNode.Local = this.copyNode(node.Local)
	newNode.Declaration = this.copyNode(node.Declaration)
	newNode.Exported = this.copyNode(node.Exported)
	newNode.Options = this.copyNode(node.Options)

	// Copy Node slices
	newNode.Body = make([]*Node, len(node.Body))
	for i, param := range node.Body {
		newNode.Body[i] = this.copyNode(param)
	}

	newNode.Params = make([]*Node, len(node.Params))
	for i, param := range node.Params {
		newNode.Params[i] = this.copyNode(param)
	}

	newNode.Cases = make([]*Node, len(node.Cases))
	for i, c := range node.Cases {
		newNode.Cases[i] = this.copyNode(c)
	}

	newNode.ConsequentSlice = make([]*Node, len(node.ConsequentSlice))
	for i, cons := range node.ConsequentSlice {
		newNode.ConsequentSlice[i] = this.copyNode(cons)
	}

	newNode.Declarations = make([]*Node, len(node.Declarations))
	for i, decl := range node.Declarations {
		newNode.Declarations[i] = this.copyNode(decl)
	}

	newNode.Elements = make([]*Node, len(node.Elements))
	for i, elem := range node.Elements {
		newNode.Elements[i] = this.copyNode(elem)
	}

	newNode.Properties = make([]*Node, len(node.Properties))
	for i, prop := range node.Properties {
		newNode.Properties[i] = this.copyNode(prop)
	}

	newNode.Arguments = make([]*Node, len(node.Arguments))
	for i, arg := range node.Arguments {
		newNode.Arguments[i] = this.copyNode(arg)
	}

	newNode.Expressions = make([]*Node, len(node.Expressions))
	for i, expr := range node.Expressions {
		newNode.Expressions[i] = this.copyNode(expr)
	}

	newNode.Quasis = make([]*Node, len(node.Quasis))
	for i, quasi := range node.Quasis {
		newNode.Quasis[i] = this.copyNode(quasi)
	}

	newNode.Specifiers = make([]*Node, len(node.Specifiers))
	for i, spec := range node.Specifiers {
		newNode.Specifiers[i] = this.copyNode(spec)
	}

	newNode.Attributes = make([]*Node, len(node.Attributes))
	for i, attr := range node.Attributes {
		newNode.Attributes[i] = this.copyNode(attr)
	}

	return newNode
}

/*
I think I can skip this?

	this.finishNodeAt = function(node, type, pos, loc) {
	  return finishNodeAt.call(this, node, type, pos, loc)
	}

*/
