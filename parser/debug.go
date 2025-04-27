package parser

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

func (p *Parser) printState() {
	loc := getLineInfo(p.input, p.pos)
	t := tokenToString[p.Type.identifier]
	line := strconv.Itoa(loc.Line)
	column := strconv.Itoa(loc.Column)
	message := strings.Join([]string{" (", line, ":", column, ")"}, "")

	fmt.Printf("Parser state: \n line:column"+message+"\n position: %d \n current type: %s\n\n", p.pos, t)
}

// Maps for string conversion
var nodeTypeToString = map[NodeType]string{
	NODE_IDENTIFIER:                     "Identifier",
	NODE_LITERAL:                        "Literal",
	NODE_PROGRAM:                        "Program",
	NODE_FUNCTION:                       "Function",
	NODE_EXPRESSION_STATEMENT:           "ExpressionStatement",
	NODE_BLOCK_STATEMENT:                "BlockStatement",
	NODE_EMPTY_STATEMENT:                "EmptyStatement",
	NODE_DEBUGGER_STATEMENT:             "DebuggerStatement",
	NODE_WITH_STATEMENT:                 "WithStatement",
	NODE_RETURN_STATEMENT:               "ReturnStatement",
	NODE_LABELED_STATEMENT:              "LabeledStatement",
	NODE_BREAK_STATEMENT:                "BreakStatement",
	NODE_CONTINUE_STATEMENT:             "ContinueStatement",
	NODE_IF_STATEMENT:                   "IfStatement",
	NODE_SWITCH_STATEMENT:               "SwitchStatement",
	NODE_SWITCH_CASE:                    "SwitchCase",
	NODE_THROW_STATEMENT:                "ThrowStatement",
	NODE_TRY_STATEMENT:                  "TryStatement",
	NODE_CATCH_CLAUSE:                   "CatchClause",
	NODE_WHILE_STATEMENT:                "WhileStatement",
	NODE_DO_WHILE_STATEMENT:             "DoWhileStatement",
	NODE_FOR_STATEMENT:                  "ForStatement",
	NODE_FOR_IN_STATEMENT:               "ForInStatement",
	NODE_FUNCTION_DECLARATION:           "FunctionDeclaration",
	NODE_VARIABLE_DECLARATION:           "VariableDeclaration",
	NODE_VARIABLE_DECLARATOR:            "VariableDeclarator",
	NODE_THIS_EXPRESSION:                "ThisExpression",
	NODE_ARRAY_EXPRESSION:               "ArrayExpression",
	NODE_OBJECT_EXPRESSION:              "ObjectExpression",
	NODE_PROPERTY:                       "Property",
	NODE_FUNCTION_EXPRESSION:            "FunctionExpression",
	NODE_UNARY_EXPRESSION:               "UnaryExpression",
	NODE_UPDATE_EXPRESSION:              "UpdateExpression",
	NODE_BINARY_EXPRESSION:              "BinaryExpression",
	NODE_ASSIGNMENT_EXPRESSION:          "AssignmentExpression",
	NODE_LOGICAL_EXPRESSION:             "LogicalExpression",
	NODE_MEMBER_EXPRESSION:              "MemberExpression",
	NODE_CONDITIONAL_EXPRESSION:         "ConditionalExpression",
	NODE_CALL_EXPRESSION:                "CallExpression",
	NODE_NEW_EXPRESSION:                 "NewExpression",
	NODE_SEQUENCE_EXPRESSION:            "SequenceExpression",
	NODE_FOR_OF_STATEMENT:               "ForOfStatement",
	NODE_SUPER:                          "Super",
	NODE_SPREAD_ELEMENT:                 "SpreadElement",
	NODE_ARROW_FUNCTION_EXPRESSION:      "ArrowFunctionExpression",
	NODE_YIELD_EXPRESSION:               "YieldExpression",
	NODE_TEMPLATE_LITERAL:               "TemplateLiteral",
	NODE_TAGGED_TEMPLATE_EXPRESSION:     "TaggedTemplateExpression",
	NODE_TEMPLATE_ELEMENT:               "TemplateElement",
	NODE_ASSIGNMENT_PROPERTY:            "AssignmentProperty",
	NODE_OBJECT_PATTERN:                 "ObjectPattern",
	NODE_ARRAY_PATTERN:                  "ArrayPattern",
	NODE_REST_ELEMENT:                   "RestElement",
	NODE_ASSIGNMENT_PATTERN:             "AssignmentPattern",
	NODE_CLASS:                          "Class",
	NODE_CLASS_BODY:                     "ClassBody",
	NODE_METHOD_DEFINITION:              "MethodDefinition",
	NODE_CLASS_DECLARATION:              "ClassDeclaration",
	NODE_CLASS_EXPRESSION:               "ClassExpression",
	NODE_META_PROPERTY:                  "MetaProperty",
	NODE_IMPORT_DECLARATION:             "ImportDeclaration",
	NODE_IMPORT_SPECIFIER:               "ImportSpecifier",
	NODE_IMPORT_DEFAULT_SPECIFIER:       "ImportDefaultSpecifier",
	NODE_IMPORT_NAMESPACE_SPECIFIER:     "ImportNamespaceSpecifier",
	NODE_IMPORT_ATTRIBUTE:               "ImportAttribute",
	NODE_EXPORT_NAMED_DECLARATION:       "ExportNamedDeclaration",
	NODE_EXPORT_SPECIFIER:               "ExportSpecifier",
	NODE_ANONYMOUS_FUNCTION_DECLARATION: "AnonymousFunctionDeclaration",
	NODE_ANONYMOUS_CLASS_DECLARATION:    "AnonymousClassDeclaration",
	NODE_EXPORT_DEFAULT_DECLARATION:     "ExportDefaultDeclaration",
	NODE_EXPORT_ALL_DECLARATION:         "ExportAllDeclaration",
	NODE_AWAIT_EXPRESSION:               "AwaitExpression",
	NODE_CHAIN_EXPRESSION:               "ChainExpression",
	NODE_IMPORT_EXPRESSION:              "ImportExpression",
	NODE_PARENTHESIZED_EXPRESSION:       "ParenthesizedExpression",
	NODE_PROPERTY_DEFINITION:            "PropertyDefinition",
	NODE_PRIVATE_IDENTIFIER:             "PrivateIdentifier",
	NODE_STATIC_BLOCK:                   "StaticBlock",
	NODE_UNTYPED:                        "Untyped",
}

var kindToString = map[Kind]string{
	KIND_NOT_INITIALIZED:   "NotInitialized",
	KIND_DECLARATION_VAR:   "Var",
	KIND_DECLARATION_LET:   "Let",
	KIND_DECLARATION_CONST: "Const",
	KIND_PROPERTY_GET:      "Get",
	KIND_PROPERTY_SET:      "Set",
	KIND_PROPERTY_INIT:     "Init",
	KIND_PROPERTY_METHOD:   "Method",
	KIND_CONSTRUCTOR:       "Constructor",
}

func PrintNode(node *Node) {
	printNode(node, 2)
}
func printNode(node *Node, indent int) {
	if node == nil {
		fmt.Printf("%s<nil>\n", strings.Repeat("  ", indent))
		return
	}

	indentStr := strings.Repeat("  ", indent)
	fmt.Printf("%sNode:\n", indentStr)

	// Base values
	fmt.Printf("%s  Start: %d\n", indentStr, node.Start)
	fmt.Printf("%s  End: %d\n", indentStr, node.End)
	fmt.Printf("%s  Type: %s\n", indentStr, nodeTypeToString[node.Type])
	fmt.Printf("%s  Range: [%d, %d]\n", indentStr, node.Range[0], node.Range[1])
	if node.Loc != nil {
		fmt.Printf("%s  Loc: %v\n", indentStr, node.Loc)
	}
	if node.SourceFile != nil {
		fmt.Printf("%s  SourceFile: %s\n", indentStr, *node.SourceFile)
	}

	// String fields
	if node.Name != "" {
		fmt.Printf("%s  Name: %s\n", indentStr, node.Name)
	}
	if node.Raw != "" {
		fmt.Printf("%s  Raw: %s\n", indentStr, node.Raw)
	}
	if node.Bigint != "" {
		fmt.Printf("%s  Bigint: %s\n", indentStr, node.Bigint)
	}
	if node.Directive != "" {
		fmt.Printf("%s  Directive: %s\n", indentStr, node.Directive)
	}

	// Value field (type any)
	if node.Value != nil {
		switch v := node.Value.(type) {
		case string:
			fmt.Printf("%s  Value: (string) %s\n", indentStr, v)
		case bool:
			fmt.Printf("%s  Value: (bool) %t\n", indentStr, v)
		case float64:
			fmt.Printf("%s  Value: (float64) %f\n", indentStr, v)
		case *regexp.Regexp:
			fmt.Printf("%s  Value: (regexp) %s\n", indentStr, v.String())
		case *big.Int:
			fmt.Printf("%s  Value: (big.Int) %s\n", indentStr, v.String())
		case *Node:
			fmt.Printf("%s  Value: (Node)\n", indentStr)
			printNode(v, indent+2)
		default:
			fmt.Printf("%s  Value: (unknown type) %v\n", indentStr, v)
		}
	}

	// Regex
	if node.Regex != nil {
		fmt.Printf("%s  Regex: Pattern=%s, Flags=%s\n", indentStr, node.Regex.Pattern, node.Regex.Flags)
	}

	sourceTypeString := map[SourceType]string{
		TYPE_SCRIPT: "script",
		TYPE_MODULE: "module",
	}

	// SourceType
	fmt.Printf("%s  SourceType: %s\n", indentStr, sourceTypeString[node.SourceType])

	// Boolean fields
	if node.IsGenerator {
		fmt.Printf("%s  IsGenerator: %t\n", indentStr, node.IsGenerator)
	}
	if node.IsExpression {
		fmt.Printf("%s  IsExpression: %t\n", indentStr, node.IsExpression)
	}
	if node.IsAsync {
		fmt.Printf("%s  IsAsync: %t\n", indentStr, node.IsAsync)
	}
	if node.Delegate {
		fmt.Printf("%s  Delegate: %t\n", indentStr, node.Delegate)
	}
	if node.IsMethod {
		fmt.Printf("%s  IsMethod: %t\n", indentStr, node.IsMethod)
	}
	if node.Shorthand {
		fmt.Printf("%s  Shorthand: %t\n", indentStr, node.Shorthand)
	}
	if node.Computed {
		fmt.Printf("%s  Computed: %t\n", indentStr, node.Computed)
	}
	if node.Prefix {
		fmt.Printf("%s  Prefix: %t\n", indentStr, node.Prefix)
	}
	if node.Optional {
		fmt.Printf("%s  Optional: %t\n", indentStr, node.Optional)
	}
	if node.Await {
		fmt.Printf("%s  Await: %t\n", indentStr, node.Await)
	}
	if node.IsStatic {
		fmt.Printf("%s  IsStatic: %t\n", indentStr, node.IsStatic)
	}
	if node.Tail {
		fmt.Printf("%s  Tail: %t\n", indentStr, node.Tail)
	}

	fmt.Printf("%s  UnaryOperator: %s\n", indentStr, node.UnaryOperator)

	fmt.Printf("%s  UpdateOperator: %s\n", indentStr, node.UpdateOperator)

	fmt.Printf("%s  BinaryOperator: %s\n", indentStr, node.BinaryOperator)

	if node.AssignmentOperator != nil {
		fmt.Printf("%s  AssignmentOperator: %s\n", indentStr, *node.AssignmentOperator)
	}
	if node.LogicalOperator != nil {
		fmt.Printf("%s  LogicalOperator: %s\n", indentStr, *node.LogicalOperator)
	}
	if node.IsDelegate != nil {
		fmt.Printf("%s  IsDelegate: %t\n", indentStr, *node.IsDelegate)
	}

	// Kind
	if node.Kind != KIND_NOT_INITIALIZED {
		fmt.Printf("%s  Kind: %s\n", indentStr, kindToString[node.Kind])
	}

	// Node fields
	if node.Id != nil {
		fmt.Printf("%s  Id:\n", indentStr)
		printNode(node.Id, indent+2)
	}
	if node.Expression != nil {
		fmt.Printf("%s  Expression:\n", indentStr)
		printNode(node.Expression, indent+2)
	}
	if node.Object != nil {
		fmt.Printf("%s  Object:\n", indentStr)
		printNode(node.Object, indent+2)
	}
	if node.Argument != nil {
		fmt.Printf("%s  Argument:\n", indentStr)
		printNode(node.Argument, indent+2)
	}
	if node.Label != nil {
		fmt.Printf("%s  Label:\n", indentStr)
		printNode(node.Label, indent+2)
	}
	if node.Test != nil {
		fmt.Printf("%s  Test:\n", indentStr)
		printNode(node.Test, indent+2)
	}
	if node.Consequent != nil {
		fmt.Printf("%s  Consequent:\n", indentStr)
		printNode(node.Consequent, indent+2)
	}
	if node.Alternate != nil {
		fmt.Printf("%s  Alternate:\n", indentStr)
		printNode(node.Alternate, indent+2)
	}
	if node.Discriminant != nil {
		fmt.Printf("%s  Discriminant:\n", indentStr)
		printNode(node.Discriminant, indent+2)
	}
	if node.Block != nil {
		fmt.Printf("%s  Block:\n", indentStr)
		printNode(node.Block, indent+2)
	}
	if node.Handler != nil {
		fmt.Printf("%s  Handler:\n", indentStr)
		printNode(node.Handler, indent+2)
	}
	if node.Finalizer != nil {
		fmt.Printf("%s  Finalizer:\n", indentStr)
		printNode(node.Finalizer, indent+2)
	}
	if node.Param != nil {
		fmt.Printf("%s  Param:\n", indentStr)
		printNode(node.Param, indent+2)
	}
	if node.Init != nil {
		fmt.Printf("%s  Init:\n", indentStr)
		printNode(node.Init, indent+2)
	}
	if node.Update != nil {
		fmt.Printf("%s  Update:\n", indentStr)
		printNode(node.Update, indent+2)
	}
	if node.Key != nil {
		fmt.Printf("%s  Key:\n", indentStr)
		printNode(node.Key, indent+2)
	}
	if node.Left != nil {
		fmt.Printf("%s  Left:\n", indentStr)
		printNode(node.Left, indent+2)
	}
	if node.Rigth != nil { // Note: Typo in struct (Rigth -> Right)
		fmt.Printf("%s  Rigth:\n", indentStr)
		printNode(node.Rigth, indent+2)
	}
	if node.MemberProperty != nil {
		fmt.Printf("%s  MemberProperty:\n", indentStr)
		printNode(node.MemberProperty, indent+2)
	}
	if node.Callee != nil {
		fmt.Printf("%s  Callee:\n", indentStr)
		printNode(node.Callee, indent+2)
	}
	if node.Quasi != nil {
		fmt.Printf("%s  Quasi:\n", indentStr)
		printNode(node.Quasi, indent+2)
	}
	if node.Tag != nil {
		fmt.Printf("%s  Tag:\n", indentStr)
		printNode(node.Tag, indent+2)
	}
	if node.SuperClass != nil {
		fmt.Printf("%s  SuperClass:\n", indentStr)
		printNode(node.SuperClass, indent+2)
	}
	if node.ClassBody != nil {
		fmt.Printf("%s  ClassBody:\n", indentStr)
		printNode(node.ClassBody, indent+2)
	}
	if node.Meta != nil {
		fmt.Printf("%s  Meta:\n", indentStr)
		printNode(node.Meta, indent+2)
	}
	if node.Property != nil {
		fmt.Printf("%s  Property:\n", indentStr)
		printNode(node.Property, indent+2)
	}
	if node.Source != nil {
		fmt.Printf("%s  Source:\n", indentStr)
		printNode(node.Source, indent+2)
	}
	if node.Imported != nil {
		fmt.Printf("%s  Imported:\n", indentStr)
		printNode(node.Imported, indent+2)
	}
	if node.Local != nil {
		fmt.Printf("%s  Local:\n", indentStr)
		printNode(node.Local, indent+2)
	}
	if node.Declaration != nil {
		fmt.Printf("%s  Declaration:\n", indentStr)
		printNode(node.Declaration, indent+2)
	}
	if node.Exported != nil {
		fmt.Printf("%s  Exported:\n", indentStr)
		printNode(node.Exported, indent+2)
	}
	if node.Options != nil {
		fmt.Printf("%s  Options:\n", indentStr)
		printNode(node.Options, indent+2)
	}
	if node.TmplValue != nil {
		fmt.Printf("%s  TmplValue: %v\n", indentStr, node.TmplValue)
	}

	// Slice of nodes
	if len(node.Body) > 0 {
		fmt.Printf("%s  Body: (%d elements)\n", indentStr, len(node.Body))
		for i, n := range node.Body {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.Params) > 0 {
		fmt.Printf("%s  Params: (%d elements)\n", indentStr, len(node.Params))
		for i, n := range node.Params {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.Cases) > 0 {
		fmt.Printf("%s  Cases: (%d elements)\n", indentStr, len(node.Cases))
		for i, n := range node.Cases {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.ConsequentSlice) > 0 {
		fmt.Printf("%s  ConsequentSlice: (%d elements)\n", indentStr, len(node.ConsequentSlice))
		for i, n := range node.ConsequentSlice {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.Declarations) > 0 {
		fmt.Printf("%s  Declarations: (%d elements)\n", indentStr, len(node.Declarations))
		for i, n := range node.Declarations {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.Elements) > 0 {
		fmt.Printf("%s  Elements: (%d elements)\n", indentStr, len(node.Elements))
		for i, n := range node.Elements {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.Properties) > 0 {
		fmt.Printf("%s  Properties: (%d elements)\n", indentStr, len(node.Properties))
		for i, n := range node.Properties {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.Arguments) > 0 {
		fmt.Printf("%s  Arguments: (%d elements)\n", indentStr, len(node.Arguments))
		for i, n := range node.Arguments {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.Expressions) > 0 {
		fmt.Printf("%s  Expressions: (%d elements)\n", indentStr, len(node.Expressions))
		for i, n := range node.Expressions {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.Quasis) > 0 {
		fmt.Printf("%s  Quasis: (%d elements)\n", indentStr, len(node.Quasis))
		for i, n := range node.Quasis {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.Specifiers) > 0 {
		fmt.Printf("%s  Specifiers: (%d elements)\n", indentStr, len(node.Specifiers))
		for i, n := range node.Specifiers {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.Attributes) > 0 {
		fmt.Printf("%s  Attributes: (%d elements)\n", indentStr, len(node.Attributes))
		for i, n := range node.Attributes {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}

	// BodyNode (single node, after Body slice)
	if node.BodyNode != nil {
		fmt.Printf("%s  BodyNode:\n", indentStr)
		printNode(node.BodyNode, indent+2)
	}
}

var tokenToString = map[Token]string{
	// BASIC
	TOKEN_NUM:       "Num",
	TOKEN_REGEXP:    "RegExp",
	TOKEN_STRING:    "String",
	TOKEN_NAME:      "Name",
	TOKEN_PRIVATEID: "PrivateId",
	TOKEN_EOF:       "EOF",

	// PUNCTUATION
	TOKEN_BRACKETL:        "BracketL",
	TOKEN_BRACKETR:        "BracketR",
	TOKEN_BRACEL:          "BraceL",
	TOKEN_BRACER:          "BraceR",
	TOKEN_PARENL:          "ParenL",
	TOKEN_PARENR:          "ParenR",
	TOKEN_COMMA:           "Comma",
	TOKEN_SEMI:            "Semi",
	TOKEN_COLON:           "Colon",
	TOKEN_DOT:             "Dot",
	TOKEN_QUESTION:        "Question",
	TOKEN_QUESTIONDOT:     "QuestionDot",
	TOKEN_ARROW:           "Arrow",
	TOKEN_TEMPLATE:        "Template",
	TOKEN_INVALIDTEMPLATE: "InvalidTemplate",
	TOKEN_ELLIPSIS:        "Ellipsis",
	TOKEN_BACKQUOTE:       "BackQuote",
	TOKEN_DOLLARBRACEL:    "DollarBraceL",

	// Operator token types
	TOKEN_EQ:         "Eq",
	TOKEN_ASSIGN:     "Assign",
	TOKEN_INCDEC:     "IncDec",
	TOKEN_PREFIX:     "Prefix",
	TOKEN_LOGICALOR:  "LogicalOr",
	TOKEN_LOGICALAND: "LogicalAnd",
	TOKEN_BITWISEOR:  "BitwiseOr",
	TOKEN_BITWISEXOR: "BitwiseXor",
	TOKEN_BITWISEAND: "BitwiseAnd",
	TOKEN_EQUALITY:   "Equality",
	TOKEN_RELATIONAL: "Relational",
	TOKEN_BITSHIFT:   "BitShift",
	TOKEN_PLUSMIN:    "PlusMin",
	TOKEN_MODULO:     "Modulo",
	TOKEN_STAR:       "Star",
	TOKEN_SLASH:      "Slash",
	TOKEN_STARSTAR:   "StarStar",
	TOKEN_COALESCE:   "Coalesce",

	// Keywords
	TOKEN_BREAK:      "Break",
	TOKEN_CASE:       "Case",
	TOKEN_CATCH:      "Catch",
	TOKEN_CONTINUE:   "Continue",
	TOKEN_DEBUGGER:   "Debugger",
	TOKEN_DEFAULT:    "Default",
	TOKEN_DO:         "Do",
	TOKEN_ELSE:       "Else",
	TOKEN_FINALLY:    "Finally",
	TOKEN_FOR:        "For",
	TOKEN_FUNCTION:   "Function",
	TOKEN_IF:         "If",
	TOKEN_RETURN:     "Return",
	TOKEN_SWITCH:     "Switch",
	TOKEN_THROW:      "Throw",
	TOKEN_TRY:        "Try",
	TOKEN_VAR:        "Var",
	TOKEN_CONST:      "Const",
	TOKEN_WHILE:      "While",
	TOKEN_WITH:       "With",
	TOKEN_NEW:        "New",
	TOKEN_THIS:       "This",
	TOKEN_SUPER:      "Super",
	TOKEN_CLASS:      "Class",
	TOKEN_EXTENDS:    "Extends",
	TOKEN_EXPORT:     "Export",
	TOKEN_IMPORT:     "Import",
	TOKEN_NULL:       "Null",
	TOKEN_TRUE:       "True",
	TOKEN_FALSE:      "False",
	TOKEN_IN:         "In",
	TOKEN_INSTANCEOF: "Instanceof",
	TOKEN_TYPEOF:     "Typeof",
	TOKEN_VOID:       "Void",
	TOKEN_DELETE:     "Delete",
}

func PrintTokenType(tokenType *TokenType) {
	printTokenType(tokenType, 0)
}

func printTokenType(tokenType *TokenType, indent int) {
	if tokenType == nil {
		fmt.Printf("%s<nil>\n", strings.Repeat("  ", indent))
		return
	}

	indentStr := strings.Repeat("  ", indent)
	fmt.Printf("%sTokenType:\n", indentStr)

	// String fields
	if tokenType.label != "" {
		fmt.Printf("%s  label: %s\n", indentStr, tokenType.label)
	}
	if tokenType.keyword != "" {
		fmt.Printf("%s  keyword: %s\n", indentStr, tokenType.keyword)
	}

	// Boolean fields (only print if true)
	if tokenType.beforeExpr {
		fmt.Printf("%s  beforeExpr: %t\n", indentStr, tokenType.beforeExpr)
	}
	if tokenType.startsExpr {
		fmt.Printf("%s  startsExpr: %t\n", indentStr, tokenType.startsExpr)
	}
	if tokenType.isLoop {
		fmt.Printf("%s  isLoop: %t\n", indentStr, tokenType.isLoop)
	}
	if tokenType.isAssign {
		fmt.Printf("%s  isAssign: %t\n", indentStr, tokenType.isAssign)
	}
	if tokenType.prefix {
		fmt.Printf("%s  prefix: %t\n", indentStr, tokenType.prefix)
	}
	if tokenType.postfix {
		fmt.Printf("%s  postfix: %t\n", indentStr, tokenType.postfix)
	}

	// Binop field
	if tokenType.binop != nil {
		fmt.Printf("%s  binop: %v\n", indentStr, tokenType.binop)
	}

	// UpdateContext field
	if tokenType.updateContext != nil {
		fmt.Printf("%s  updateContext: <function>\n", indentStr)
	}

	// Identifier field
	fmt.Printf("%s  identifier: %s\n", indentStr, tokenToString[tokenType.identifier])
}
