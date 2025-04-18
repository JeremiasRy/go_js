package parser

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
	VAR DeclarationKind = iota
	LET
	CONST
)

type PropertyKind int

const (
	GET PropertyKind = iota
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

	Name               *string
	Value              any // string, bool, float64, *regexp.Regexp, *big.Int
	Raw                *string
	Regex              *Regex
	Bigint             *string
	Body               []*Node // Statement | ModuleDeclaration
	SourceType         SourceType
	Id                 *Node   // Identifier
	Params             []*Node // Pattern
	BodyNode           *Node   // BlockStatement | Expression
	IsGenerator        *bool
	IsExpression       *bool
	IsAsync            *bool
	Expression         *Node // Expression | Literal
	Directive          *string
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
	DeclarationKind    *DeclarationKind
	Elements           []*Node // Expression | SpreadElement
	Properties         []*Node // Property | SpreadElement
	Key                *Node   // Expression
	ValueNode          *Node   // Expression
	PropertyKind       *PropertyKind
	Method             *bool
	Shorthand          *bool
	Computed           *bool
	UnaryOperator      *UnaryOperator
	Prefix             *bool
	UpdateOperator     *UpdateOperator
	BinaryOperator     *BinaryOperator
	Left               *Node // Expression | PrivateIdentifier
	Rigth              *Node
	AssignmentOperator *AssignmentOperator
	LogicalOperator    *LogicalOperator
	MemberProperty     *Node // Expression | PrivateIdentifier
	Optional           *bool
	Callee             *Node   // Expression | Super
	Arguments          []*Node // Expression | SpreadElement
	Expressions        []*Node // Expression
	Await              *bool
	IsDelegate         *bool
	Quasis             []*Node // TemplateElement
	Tag                *Node   // Expression
	Quasi              *Node   // TemplateLiteral
	Tail               *bool
	TmplValue          *TemplateValue
	SuperClass         *Node // Expression
	ClassBody          *Node // ClassBody
	IsSstatic          *bool
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
