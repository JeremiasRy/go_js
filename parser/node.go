package parser

import (
	"encoding/json"
	"fmt"
)

type SourceType int

const (
	TYPE_SCRIPT SourceType = iota
	TYPE_MODULE
)

func (st *SourceType) MarshalJSON() ([]byte, error) {
	if *st == TYPE_SCRIPT {
		return json.Marshal("TYPE_SCRIPT")
	}

	if *st == TYPE_MODULE {
		return json.Marshal("TYPE_MODULE")
	}

	return json.Marshal("TYPE_UNKNOWN")
}

func (st *SourceType) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return fmt.Errorf("failed to unmarshal source type: %v", err)
	}

	if name == "TYPE_SCRIPT" {
		*st = TYPE_SCRIPT
	}

	if name == "TYPE_MODULE" {
		*st = TYPE_MODULE
	}
	return nil
}

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

func (nt *NodeType) MarshalJSON() ([]byte, error) {
	name, ok := nodeTypeToString[*nt]

	if !ok {
		name = "UnknownNodeType"
	}

	return json.Marshal(name)
}

func (nt *NodeType) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return fmt.Errorf("failed to unmarshal NodeType: %v", err)
	}
	value, ok := stringToNodeType[name]
	if !ok {
		return fmt.Errorf("unknown NodeType name: %s", name)
	}
	*nt = value
	return nil
}

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

func (k *Kind) MarshalJSON() ([]byte, error) {
	if *k == KIND_NOT_INITIALIZED {
		return json.Marshal("null")
	}

	name, ok := kindToString[*k]

	if !ok {
		name = "UnknownKind"
	}

	return json.Marshal(name)
}

func (k *Kind) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return fmt.Errorf("failed to unmarshal Kind: %v", err)
	}
	value, ok := stringToKind[name]
	if !ok {
		return fmt.Errorf("unknown Kind name: %s", name)
	}
	*k = value
	return nil
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
	Start              int                `json:"start"`
	End                int                `json:"end"`
	Type               NodeType           `json:"type"`
	Range              [2]int             `json:"range"`
	Location           *SourceLocation    `json:"location,omitempty"`
	SourceFile         *string            `json:"sourceFile,omitempty"`
	Name               string             `json:"name,omitempty"`
	Value              any                `json:"value,omitempty"`
	Raw                string             `json:"raw,omitempty"`
	Regex              *Regex             `json:"regex,omitempty"`
	Bigint             string             `json:"bigint,omitempty"`
	Body               []*Node            `json:"body,omitempty"`
	BodyNode           *Node              `json:"bodyNode,omitempty"`
	SourceType         SourceType         `json:"sourceType,omitempty"`
	Identifier         *Node              `json:"identifier,omitempty"`
	Params             []*Node            `json:"params,omitempty"`
	IsGenerator        bool               `json:"isGenerator,omitempty"`
	IsExpression       bool               `json:"isExpression,omitempty"`
	IsAsync            bool               `json:"isAsync,omitempty"`
	Expression         *Node              `json:"expression,omitempty"`
	Directive          string             `json:"directive,omitempty"`
	Delegate           bool               `json:"delegate,omitempty"`
	Object             *Node              `json:"object,omitempty"`
	Argument           *Node              `json:"argument,omitempty"`
	Label              *Node              `json:"label,omitempty"`
	Test               *Node              `json:"test,omitempty"`
	Consequent         *Node              `json:"consequent,omitempty"`
	Alternate          *Node              `json:"alternate,omitempty"`
	Discriminant       *Node              `json:"discriminant,omitempty"`
	Cases              []*Node            `json:"cases,omitempty"`
	ConsequentSlice    []*Node            `json:"consequentSlice,omitempty"`
	Block              *Node              `json:"block,omitempty"`
	Handler            *Node              `json:"handler,omitempty"`
	Finalizer          *Node              `json:"finalizer,omitempty"`
	Param              *Node              `json:"param,omitempty"`
	Initializer        *Node              `json:"initializer,omitempty"`
	Update             *Node              `json:"update,omitempty"`
	Declarations       []*Node            `json:"declarations,omitempty"`
	Kind               Kind               `json:"kind,omitempty"`
	Elements           []*Node            `json:"elements,omitempty"`
	Properties         []*Node            `json:"properties,omitempty"`
	Key                *Node              `json:"key,omitempty"`
	IsMethod           bool               `json:"isMethod,omitempty"`
	Shorthand          bool               `json:"shorthand,omitempty"`
	Computed           bool               `json:"computed,omitempty"`
	UnaryOperator      UnaryOperator      `json:"unaryOperator,omitempty"`
	Prefix             bool               `json:"prefix,omitempty"`
	UpdateOperator     UpdateOperator     `json:"updateOperator,omitempty"`
	BinaryOperator     BinaryOperator     `json:"binaryOperator,omitempty"`
	Left               *Node              `json:"left,omitempty"`
	Right              *Node              `json:"right,omitempty"`
	AssignmentOperator AssignmentOperator `json:"assignmentOperator,omitempty"`
	LogicalOperator    LogicalOperator    `json:"logicalOperator,omitempty"`
	MemberProperty     *Node              `json:"memberProperty,omitempty"`
	Optional           bool               `json:"optional,omitempty"`
	Callee             *Node              `json:"callee,omitempty"`
	Arguments          []*Node            `json:"arguments,omitempty"`
	Expressions        []*Node            `json:"expressions,omitempty"`
	Await              bool               `json:"await,omitempty"`
	IsDelegate         bool               `json:"isDelegate,omitempty"`
	Quasis             []*Node            `json:"quasis,omitempty"`
	Quasi              *Node              `json:"quasi,omitempty"`
	Tag                *Node              `json:"tag,omitempty"`
	Tail               bool               `json:"tail,omitempty"`
	TmplValue          *TemplateValue     `json:"tmplValue,omitempty"`
	SuperClass         *Node              `json:"superClass,omitempty"`
	IsStatic           bool               `json:"isStatic,omitempty"`
	Meta               *Node              `json:"meta,omitempty"`
	Property           *Node              `json:"property,omitempty"`
	Specifiers         []*Node            `json:"specifiers,omitempty"`
	Source             *Node              `json:"source,omitempty"`
	Attributes         []*Node            `json:"attributes,omitempty"`
	Imported           *Node              `json:"imported,omitempty"`
	Local              *Node              `json:"local,omitempty"`
	Declaration        *Node              `json:"declaration,omitempty"`
	Exported           *Node              `json:"exported,omitempty"`
	Options            *Node              `json:"options,omitempty"`
}

// Marshal sometimes treat []byte as Base64
func (n Node) MarshalJSON() ([]byte, error) {
	type Alias Node

	var jsonValue any
	if str, ok := n.Value.([]byte); ok {
		jsonValue = string(str)
	} else {
		jsonValue = n.Value
	}

	return json.Marshal(&struct {
		*Alias
		Value any `json:"value,omitempty"`
	}{
		Alias: (*Alias)(&n),
		Value: jsonValue,
	})
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
		node.Location = NewSourceLocation(parser, loc, nil)
	}

	if parser.options.DirectSourceFile != nil {
		node.SourceFile = parser.options.DirectSourceFile
	}

	if parser.options.Ranges {
		node.Range = [2]int{pos, 0}
	}
	return node
}

func (p *Parser) startNode() *Node {
	return NewNode(p, p.start, p.startLoc)
}

func (p *Parser) startNodeAt(pos int, loc *Location) *Node {
	return NewNode(p, pos, loc)
}

func (p *Parser) finishNodeAt(node *Node, finishType NodeType, pos int, loc *Location) {
	node.Type = finishType
	node.End = pos
	if p.options.Locations {
		node.Location.End = loc
	}

	if p.options.Ranges {
		node.Range[1] = pos
	}
}

func (p *Parser) finishNode(node *Node, finishType NodeType) *Node {
	p.finishNodeAt(node, finishType, p.LastTokEnd, p.LastTokEndLoc)
	return node
}

func (p *Parser) copyNode(node *Node) (*Node, error) {
	if node == nil {
		return nil, nil
	}

	data, err := json.Marshal(node)

	if err != nil {
		return nil, err
	}
	var copyNode Node
	err = json.Unmarshal(data, &copyNode)
	if err != nil {
		return nil, err
	}
	return &copyNode, nil
}

/*
I think I can skip this?

	p.finishNodeAt = function(node, type, pos, loc) {
	  return finishNodeAt.call(this, node, type, pos, loc)
	}

*/
