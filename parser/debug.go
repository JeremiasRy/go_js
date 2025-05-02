package parser

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

func (p *Parser) PrintState() {
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
	fmt.Printf("%s  Start: %d\n", indentStr, node.start)
	fmt.Printf("%s  End: %d\n", indentStr, node.end)
	fmt.Printf("%s  Type: %s\n", indentStr, nodeTypeToString[node.type_])
	fmt.Printf("%s  Range: [%d, %d]\n", indentStr, node.range_[0], node.range_[1])
	if node.location != nil {
		fmt.Printf("%s  Loc: %v\n", indentStr, node.location)
	}
	if node.sourceFile != nil {
		fmt.Printf("%s  SourceFile: %s\n", indentStr, *node.sourceFile)
	}

	// String fields
	if node.name != "" {
		fmt.Printf("%s  Name: %s\n", indentStr, node.name)
	}
	if node.raw != "" {
		fmt.Printf("%s  Raw: %s\n", indentStr, node.raw)
	}
	if node.bigint != "" {
		fmt.Printf("%s  Bigint: %s\n", indentStr, node.bigint)
	}
	if node.directive != "" {
		fmt.Printf("%s  Directive: %s\n", indentStr, node.directive)
	}

	// Value field (type any)
	if node.value != nil {
		switch v := node.value.(type) {
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
	if node.regex != nil {
		fmt.Printf("%s  Regex: Pattern=%s, Flags=%s\n", indentStr, node.regex.Pattern, node.regex.Flags)
	}

	sourceTypeString := map[SourceType]string{
		TYPE_SCRIPT: "script",
		TYPE_MODULE: "module",
	}

	// SourceType
	fmt.Printf("%s  SourceType: %s\n", indentStr, sourceTypeString[node.sourceType])

	// Boolean fields
	if node.isGenerator {
		fmt.Printf("%s  IsGenerator: %t\n", indentStr, node.isGenerator)
	}
	if node.isExpression {
		fmt.Printf("%s  IsExpression: %t\n", indentStr, node.isExpression)
	}
	if node.isAsync {
		fmt.Printf("%s  IsAsync: %t\n", indentStr, node.isAsync)
	}
	if node.delegate {
		fmt.Printf("%s  Delegate: %t\n", indentStr, node.delegate)
	}
	if node.isMethod {
		fmt.Printf("%s  IsMethod: %t\n", indentStr, node.isMethod)
	}
	if node.shorthand {
		fmt.Printf("%s  Shorthand: %t\n", indentStr, node.shorthand)
	}
	if node.computed {
		fmt.Printf("%s  Computed: %t\n", indentStr, node.computed)
	}
	if node.prefix {
		fmt.Printf("%s  Prefix: %t\n", indentStr, node.prefix)
	}
	if node.optional {
		fmt.Printf("%s  Optional: %t\n", indentStr, node.optional)
	}
	if node.await {
		fmt.Printf("%s  Await: %t\n", indentStr, node.await)
	}
	if node.isStatic {
		fmt.Printf("%s  IsStatic: %t\n", indentStr, node.isStatic)
	}
	if node.tail {
		fmt.Printf("%s  Tail: %t\n", indentStr, node.tail)
	}

	fmt.Printf("%s  UnaryOperator: %s\n", indentStr, node.unaryOperator)

	fmt.Printf("%s  UpdateOperator: %s\n", indentStr, node.updateOperator)

	fmt.Printf("%s  BinaryOperator: %s\n", indentStr, node.binaryOperator)

	fmt.Printf("%s  AssignmentOperator: %s\n", indentStr, node.assignmentOperator)

	fmt.Printf("%s  LogicalOperator: %s\n", indentStr, node.logicalOperator)

	if node.isDelegate != nil {
		fmt.Printf("%s  IsDelegate: %t\n", indentStr, *node.isDelegate)
	}

	// Kind
	if node.kind != KIND_NOT_INITIALIZED {
		fmt.Printf("%s  Kind: %s\n", indentStr, kindToString[node.kind])
	}

	// Node fields
	if node.identifier != nil {
		fmt.Printf("%s  Id:\n", indentStr)
		printNode(node.identifier, indent+2)
	}
	if node.expression != nil {
		fmt.Printf("%s  Expression:\n", indentStr)
		printNode(node.expression, indent+2)
	}
	if node.object != nil {
		fmt.Printf("%s  Object:\n", indentStr)
		printNode(node.object, indent+2)
	}
	if node.argument != nil {
		fmt.Printf("%s  Argument:\n", indentStr)
		printNode(node.argument, indent+2)
	}
	if node.label != nil {
		fmt.Printf("%s  Label:\n", indentStr)
		printNode(node.label, indent+2)
	}
	if node.test != nil {
		fmt.Printf("%s  Test:\n", indentStr)
		printNode(node.test, indent+2)
	}
	if node.consequent != nil {
		fmt.Printf("%s  Consequent:\n", indentStr)
		printNode(node.consequent, indent+2)
	}
	if node.alternate != nil {
		fmt.Printf("%s  Alternate:\n", indentStr)
		printNode(node.alternate, indent+2)
	}
	if node.discriminant != nil {
		fmt.Printf("%s  Discriminant:\n", indentStr)
		printNode(node.discriminant, indent+2)
	}
	if node.block != nil {
		fmt.Printf("%s  Block:\n", indentStr)
		printNode(node.block, indent+2)
	}
	if node.handler != nil {
		fmt.Printf("%s  Handler:\n", indentStr)
		printNode(node.handler, indent+2)
	}
	if node.finalizer != nil {
		fmt.Printf("%s  Finalizer:\n", indentStr)
		printNode(node.finalizer, indent+2)
	}
	if node.param != nil {
		fmt.Printf("%s  Param:\n", indentStr)
		printNode(node.param, indent+2)
	}
	if node.initializer != nil {
		fmt.Printf("%s  Init:\n", indentStr)
		printNode(node.initializer, indent+2)
	}
	if node.update != nil {
		fmt.Printf("%s  Update:\n", indentStr)
		printNode(node.update, indent+2)
	}
	if node.key != nil {
		fmt.Printf("%s  Key:\n", indentStr)
		printNode(node.key, indent+2)
	}
	if node.left != nil {
		fmt.Printf("%s  Left:\n", indentStr)
		printNode(node.left, indent+2)
	}
	if node.rigth != nil { // Note: Typo in struct (Rigth -> Right)
		fmt.Printf("%s  Rigth:\n", indentStr)
		printNode(node.rigth, indent+2)
	}
	if node.memberProperty != nil {
		fmt.Printf("%s  MemberProperty:\n", indentStr)
		printNode(node.memberProperty, indent+2)
	}
	if node.callee != nil {
		fmt.Printf("%s  Callee:\n", indentStr)
		printNode(node.callee, indent+2)
	}
	if node.quasi != nil {
		fmt.Printf("%s  Quasi:\n", indentStr)
		printNode(node.quasi, indent+2)
	}
	if node.tag != nil {
		fmt.Printf("%s  Tag:\n", indentStr)
		printNode(node.tag, indent+2)
	}
	if node.superClass != nil {
		fmt.Printf("%s  SuperClass:\n", indentStr)
		printNode(node.superClass, indent+2)
	}
	if node.meta != nil {
		fmt.Printf("%s  Meta:\n", indentStr)
		printNode(node.meta, indent+2)
	}
	if node.property != nil {
		fmt.Printf("%s  Property:\n", indentStr)
		printNode(node.property, indent+2)
	}
	if node.source != nil {
		fmt.Printf("%s  Source:\n", indentStr)
		printNode(node.source, indent+2)
	}
	if node.imported != nil {
		fmt.Printf("%s  Imported:\n", indentStr)
		printNode(node.imported, indent+2)
	}
	if node.local != nil {
		fmt.Printf("%s  Local:\n", indentStr)
		printNode(node.local, indent+2)
	}
	if node.declaration != nil {
		fmt.Printf("%s  Declaration:\n", indentStr)
		printNode(node.declaration, indent+2)
	}
	if node.exported != nil {
		fmt.Printf("%s  Exported:\n", indentStr)
		printNode(node.exported, indent+2)
	}
	if node.options != nil {
		fmt.Printf("%s  Options:\n", indentStr)
		printNode(node.options, indent+2)
	}
	if node.tmplValue != nil {
		fmt.Printf("%s  TmplValue: %v\n", indentStr, node.tmplValue)
	}

	// Slice of nodes
	if len(node.body) > 0 {
		fmt.Printf("%s  Body: (%d elements)\n", indentStr, len(node.body))
		for i, n := range node.body {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.params) > 0 {
		fmt.Printf("%s  Params: (%d elements)\n", indentStr, len(node.params))
		for i, n := range node.params {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.cases) > 0 {
		fmt.Printf("%s  Cases: (%d elements)\n", indentStr, len(node.cases))
		for i, n := range node.cases {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.consequentSlice) > 0 {
		fmt.Printf("%s  ConsequentSlice: (%d elements)\n", indentStr, len(node.consequentSlice))
		for i, n := range node.consequentSlice {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.declarations) > 0 {
		fmt.Printf("%s  Declarations: (%d elements)\n", indentStr, len(node.declarations))
		for i, n := range node.declarations {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.elements) > 0 {
		fmt.Printf("%s  Elements: (%d elements)\n", indentStr, len(node.elements))
		for i, n := range node.elements {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.properties) > 0 {
		fmt.Printf("%s  Properties: (%d elements)\n", indentStr, len(node.properties))
		for i, n := range node.properties {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.arguments) > 0 {
		fmt.Printf("%s  Arguments: (%d elements)\n", indentStr, len(node.arguments))
		for i, n := range node.arguments {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.expressions) > 0 {
		fmt.Printf("%s  Expressions: (%d elements)\n", indentStr, len(node.expressions))
		for i, n := range node.expressions {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.quasis) > 0 {
		fmt.Printf("%s  Quasis: (%d elements)\n", indentStr, len(node.quasis))
		for i, n := range node.quasis {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.specifierss) > 0 {
		fmt.Printf("%s  Specifiers: (%d elements)\n", indentStr, len(node.specifierss))
		for i, n := range node.specifierss {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}
	if len(node.attributes) > 0 {
		fmt.Printf("%s  Attributes: (%d elements)\n", indentStr, len(node.attributes))
		for i, n := range node.attributes {
			fmt.Printf("%s    [%d]:\n", indentStr, i)
			printNode(n, indent+4)
		}
	}

	// BodyNode (single node, after Body slice)
	if node.bodyNode != nil {
		fmt.Printf("%s  BodyNode:\n", indentStr)
		printNode(node.bodyNode, indent+2)
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
