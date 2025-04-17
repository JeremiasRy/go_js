package parser

// TOKEN
type Token int

const (
	// BASIC
	TOKEN_NUM Token = iota
	TOKEN_REGEXP
	TOKEN_STRING
	TOKEN_NAME
	TOKEN_PRIVATEID
	TOKEN_EOF

	// PUNCTUATION
	TOKEN_BRACKETL
	TOKEN_BRACKETR
	TOKEN_BRACEL
	TOKEN_BRACER
	TOKEN_PARENL
	TOKEN_PARENR
	TOKEN_COMMA
	TOKEN_SEMI
	TOKEN_COLON
	TOKEN_DOT
	TOKEN_QUESTION
	TOKEN_QUESTIONDOT
	TOKEN_ARROW
	TOKEN_TEMPLATE
	TOKEN_INVALIDTEMPLATE
	TOKEN_ELLIPSIS
	TOKEN_BACKQUOTE
	TOKEN_DOLLARBRACEL

	// Operator token types
	TOKEN_EQ
	TOKEN_ASSIGN
	TOKEN_INCDEC
	TOKEN_PREFIX
	TOKEN_LOGICALOR
	TOKEN_LOGICALAND
	TOKEN_BITWISEOR
	TOKEN_BITWISEXOR
	TOKEN_BITWISEAND
	TOKEN_EQUALITY
	TOKEN_RELATIONAL
	TOKEN_BITSHIFT
	TOKEN_PLUSMIN
	TOKEN_MODULO
	TOKEN_STAR
	TOKEN_SLASH
	TOKEN_STARSTAR
	TOKEN_COALESCE

	// Keywords
	TOKEN_BREAK
	TOKEN_CASE
	TOKEN_CATCH
	TOKEN_CONTINUE
	TOKEN_DEBUGGER
	TOKEN_DEFAULT
	TOKEN_DO
	TOKEN_ELSE
	TOKEN_FINALLY
	TOKEN_FOR
	TOKEN_FUNCTION
	TOKEN_IF
	TOKEN_RETURN
	TOKEN_SWITCH
	TOKEN_THROW
	TOKEN_TRY
	TOKEN_VAR
	TOKEN_CONST
	TOKEN_WHILE
	TOKEN_WITH
	TOKEN_NEW
	TOKEN_THIS
	TOKEN_SUPER
	TOKEN_CLASS
	TOKEN_EXTENDS
	TOKEN_EXPORT
	TOKEN_IMPORT
	TOKEN_NULL
	TOKEN_TRUE
	TOKEN_FALSE
	TOKEN_IN
	TOKEN_INSTANCEOF
	TOKEN_TYPEOF
	TOKEN_VOID
	TOKEN_DELETE
)

type Binop struct {
	prec uint
}

type UpdateContext struct {
	updateContext func(*TokenType)
}

type TokenType struct {
	label         string
	keyword       string
	beforeExpr    bool
	startsExpr    bool
	isLoop        bool
	isAssign      bool
	prefix        bool
	postfix       bool
	binop         *Binop
	updateContext *UpdateContext
	identifier    Token
}

var tokenTypes = map[Token]*TokenType{
	// Basic token types
	TOKEN_NUM:       newToken("num", "", map[string]bool{"startsExpr": true}, nil, TOKEN_NUM),
	TOKEN_REGEXP:    newToken("regexp", "", map[string]bool{"startsExpr": true}, nil, TOKEN_REGEXP),
	TOKEN_STRING:    newToken("string", "", map[string]bool{"startsExpr": true}, nil, TOKEN_STRING),
	TOKEN_NAME:      newToken("name", "", map[string]bool{"startsExpr": true}, nil, TOKEN_NAME),
	TOKEN_PRIVATEID: newToken("privateId", "", map[string]bool{"startsExpr": true}, nil, TOKEN_PRIVATEID),
	TOKEN_EOF:       newToken("eof", "", map[string]bool{}, nil, TOKEN_EOF),

	// Punctuation token types
	TOKEN_BRACKETL:        newToken("[", "", map[string]bool{"beforeExpr": true, "startsExpr": true}, nil, TOKEN_BRACKETL),
	TOKEN_BRACKETR:        newToken("]", "", map[string]bool{}, nil, TOKEN_BRACKETR),
	TOKEN_BRACEL:          newToken("{", "", map[string]bool{"beforeExpr": true, "startsExpr": true}, nil, TOKEN_BRACEL),
	TOKEN_BRACER:          newToken("}", "", map[string]bool{}, nil, TOKEN_BRACER),
	TOKEN_PARENL:          newToken("(", "", map[string]bool{"beforeExpr": true, "startsExpr": true}, nil, TOKEN_PARENL),
	TOKEN_PARENR:          newToken(")", "", map[string]bool{}, nil, TOKEN_PARENR),
	TOKEN_COMMA:           newToken(",", "", map[string]bool{"beforeExpr": true}, nil, TOKEN_COMMA),
	TOKEN_SEMI:            newToken(";", "", map[string]bool{"beforeExpr": true}, nil, TOKEN_SEMI),
	TOKEN_COLON:           newToken(":", "", map[string]bool{"beforeExpr": true}, nil, TOKEN_COLON),
	TOKEN_DOT:             newToken(".", "", map[string]bool{}, nil, TOKEN_DOT),
	TOKEN_QUESTION:        newToken("?", "", map[string]bool{"beforeExpr": true}, nil, TOKEN_QUESTION),
	TOKEN_QUESTIONDOT:     newToken("?.", "", map[string]bool{}, nil, TOKEN_QUESTIONDOT),
	TOKEN_ARROW:           newToken("=>", "", map[string]bool{"beforeExpr": true}, nil, TOKEN_ARROW),
	TOKEN_TEMPLATE:        newToken("template", "", map[string]bool{}, nil, TOKEN_TEMPLATE),
	TOKEN_INVALIDTEMPLATE: newToken("invalidTemplate", "", map[string]bool{}, nil, TOKEN_INVALIDTEMPLATE),
	TOKEN_ELLIPSIS:        newToken("...", "", map[string]bool{"beforeExpr": true}, nil, TOKEN_ELLIPSIS),
	TOKEN_BACKQUOTE:       newToken("`", "", map[string]bool{"startsExpr": true}, nil, TOKEN_BACKQUOTE),
	TOKEN_DOLLARBRACEL:    newToken("${", "", map[string]bool{"beforeExpr": true, "startsExpr": true}, nil, TOKEN_DOLLARBRACEL),

	// Operator token types
	TOKEN_EQ:         newToken("=", "", map[string]bool{"beforeExpr": true, "isAssign": true}, nil, TOKEN_EQ),
	TOKEN_ASSIGN:     newToken("_=", "", map[string]bool{"beforeExpr": true, "isAssign": true}, nil, TOKEN_ASSIGN),
	TOKEN_INCDEC:     newToken("++/--", "", map[string]bool{"prefix": true, "postfix": true, "startsExpr": true}, nil, TOKEN_INCDEC),
	TOKEN_PREFIX:     newToken("!/~", "", map[string]bool{"beforeExpr": true, "prefix": true, "startsExpr": true}, nil, TOKEN_PREFIX),
	TOKEN_LOGICALOR:  newToken("||", "", map[string]bool{}, &Binop{prec: 1}, TOKEN_LOGICALOR),
	TOKEN_LOGICALAND: newToken("&&", "", map[string]bool{}, &Binop{prec: 2}, TOKEN_LOGICALAND),
	TOKEN_BITWISEOR:  newToken("|", "", map[string]bool{}, &Binop{prec: 3}, TOKEN_BITWISEOR),
	TOKEN_BITWISEXOR: newToken("^", "", map[string]bool{}, &Binop{prec: 4}, TOKEN_BITWISEXOR),
	TOKEN_BITWISEAND: newToken("&", "", map[string]bool{}, &Binop{prec: 5}, TOKEN_BITWISEAND),
	TOKEN_EQUALITY:   newToken("==/!=/===/!==", "", map[string]bool{}, &Binop{prec: 6}, TOKEN_EQUALITY),
	TOKEN_RELATIONAL: newToken("</>/<=/>=", "", map[string]bool{}, &Binop{prec: 7}, TOKEN_RELATIONAL),
	TOKEN_BITSHIFT:   newToken("<</>>/>>>", "", map[string]bool{}, &Binop{prec: 8}, TOKEN_BITSHIFT),
	TOKEN_PLUSMIN:    newToken("+/-", "", map[string]bool{"beforeExpr": true, "prefix": true, "startsExpr": true}, &Binop{prec: 9}, TOKEN_PLUSMIN),
	TOKEN_MODULO:     newToken("%", "", map[string]bool{}, &Binop{prec: 10}, TOKEN_MODULO),
	TOKEN_STAR:       newToken("*", "", map[string]bool{}, &Binop{prec: 10}, TOKEN_STAR),
	TOKEN_SLASH:      newToken("/", "", map[string]bool{}, &Binop{prec: 10}, TOKEN_SLASH),
	TOKEN_STARSTAR:   newToken("**", "", map[string]bool{"beforeExpr": true}, nil, TOKEN_STARSTAR),
	TOKEN_COALESCE:   newToken("??", "", map[string]bool{}, &Binop{prec: 1}, TOKEN_COALESCE),

	// Keywords
	TOKEN_BREAK:      newToken("break", "break", map[string]bool{}, nil, TOKEN_BREAK),
	TOKEN_CASE:       newToken("case", "case", map[string]bool{"beforeExpr": true}, nil, TOKEN_CASE),
	TOKEN_CATCH:      newToken("catch", "catch", map[string]bool{}, nil, TOKEN_CATCH),
	TOKEN_CONTINUE:   newToken("continue", "continue", map[string]bool{}, nil, TOKEN_CONTINUE),
	TOKEN_DEBUGGER:   newToken("debugger", "debugger", map[string]bool{}, nil, TOKEN_DEBUGGER),
	TOKEN_DEFAULT:    newToken("default", "default", map[string]bool{"beforeExpr": true}, nil, TOKEN_DEFAULT),
	TOKEN_DO:         newToken("do", "do", map[string]bool{"isLoop": true, "beforeExpr": true}, nil, TOKEN_DO),
	TOKEN_ELSE:       newToken("else", "else", map[string]bool{"beforeExpr": true}, nil, TOKEN_ELSE),
	TOKEN_FINALLY:    newToken("finally", "finally", map[string]bool{}, nil, TOKEN_FINALLY),
	TOKEN_FOR:        newToken("for", "for", map[string]bool{"isLoop": true}, nil, TOKEN_FOR),
	TOKEN_FUNCTION:   newToken("function", "function", map[string]bool{"startsExpr": true}, nil, TOKEN_FUNCTION),
	TOKEN_IF:         newToken("if", "if", map[string]bool{}, nil, TOKEN_IF),
	TOKEN_RETURN:     newToken("return", "return", map[string]bool{"beforeExpr": true}, nil, TOKEN_RETURN),
	TOKEN_SWITCH:     newToken("switch", "switch", map[string]bool{}, nil, TOKEN_SWITCH),
	TOKEN_THROW:      newToken("throw", "throw", map[string]bool{"beforeExpr": true}, nil, TOKEN_THROW),
	TOKEN_TRY:        newToken("try", "try", map[string]bool{}, nil, TOKEN_TRY),
	TOKEN_VAR:        newToken("var", "var", map[string]bool{}, nil, TOKEN_VAR),
	TOKEN_CONST:      newToken("const", "const", map[string]bool{}, nil, TOKEN_CONST),
	TOKEN_WHILE:      newToken("while", "while", map[string]bool{"isLoop": true}, nil, TOKEN_WHILE),
	TOKEN_WITH:       newToken("with", "with", map[string]bool{}, nil, TOKEN_WITH),
	TOKEN_NEW:        newToken("new", "new", map[string]bool{"beforeExpr": true, "startsExpr": true}, nil, TOKEN_NEW),
	TOKEN_THIS:       newToken("this", "this", map[string]bool{"startsExpr": true}, nil, TOKEN_THIS),
	TOKEN_SUPER:      newToken("super", "super", map[string]bool{"startsExpr": true}, nil, TOKEN_SUPER),
	TOKEN_CLASS:      newToken("class", "class", map[string]bool{"startsExpr": true}, nil, TOKEN_CLASS),
	TOKEN_EXTENDS:    newToken("extends", "extends", map[string]bool{"beforeExpr": true}, nil, TOKEN_EXTENDS),
	TOKEN_EXPORT:     newToken("export", "export", map[string]bool{}, nil, TOKEN_EXPORT),
	TOKEN_IMPORT:     newToken("import", "import", map[string]bool{"startsExpr": true}, nil, TOKEN_IMPORT),
	TOKEN_NULL:       newToken("null", "null", map[string]bool{"startsExpr": true}, nil, TOKEN_NULL),
	TOKEN_TRUE:       newToken("true", "true", map[string]bool{"startsExpr": true}, nil, TOKEN_TRUE),
	TOKEN_FALSE:      newToken("false", "false", map[string]bool{"startsExpr": true}, nil, TOKEN_FALSE),
	TOKEN_IN:         newToken("in", "in", map[string]bool{"beforeExpr": true}, &Binop{prec: 7}, TOKEN_IN),
	TOKEN_INSTANCEOF: newToken("instanceof", "instanceof", map[string]bool{"beforeExpr": true}, &Binop{prec: 7}, TOKEN_INSTANCEOF),
	TOKEN_TYPEOF:     newToken("typeof", "typeof", map[string]bool{"beforeExpr": true, "prefix": true, "startsExpr": true}, nil, TOKEN_TYPEOF),
	TOKEN_VOID:       newToken("void", "void", map[string]bool{"beforeExpr": true, "prefix": true, "startsExpr": true}, nil, TOKEN_VOID),
	TOKEN_DELETE:     newToken("delete", "delete", map[string]bool{"beforeExpr": true, "prefix": true, "startsExpr": true}, nil, TOKEN_DELETE),
}

var keywords = map[string]*TokenType{
	"break":      tokenTypes[TOKEN_BREAK],
	"case":       tokenTypes[TOKEN_CASE],
	"catch":      tokenTypes[TOKEN_CATCH],
	"continue":   tokenTypes[TOKEN_CONTINUE],
	"debugger":   tokenTypes[TOKEN_DEBUGGER],
	"default":    tokenTypes[TOKEN_DEFAULT],
	"do":         tokenTypes[TOKEN_DO],
	"else":       tokenTypes[TOKEN_ELSE],
	"finally":    tokenTypes[TOKEN_FINALLY],
	"for":        tokenTypes[TOKEN_FOR],
	"function":   tokenTypes[TOKEN_FUNCTION],
	"if":         tokenTypes[TOKEN_IF],
	"return":     tokenTypes[TOKEN_RETURN],
	"switch":     tokenTypes[TOKEN_SWITCH],
	"throw":      tokenTypes[TOKEN_THROW],
	"try":        tokenTypes[TOKEN_TRY],
	"var":        tokenTypes[TOKEN_VAR],
	"const":      tokenTypes[TOKEN_CONST],
	"while":      tokenTypes[TOKEN_WHILE],
	"with":       tokenTypes[TOKEN_WITH],
	"new":        tokenTypes[TOKEN_NEW],
	"this":       tokenTypes[TOKEN_THIS],
	"super":      tokenTypes[TOKEN_SUPER],
	"class":      tokenTypes[TOKEN_CLASS],
	"extends":    tokenTypes[TOKEN_EXTENDS],
	"export":     tokenTypes[TOKEN_EXPORT],
	"import":     tokenTypes[TOKEN_IMPORT],
	"null":       tokenTypes[TOKEN_NULL],
	"true":       tokenTypes[TOKEN_TRUE],
	"false":      tokenTypes[TOKEN_FALSE],
	"in":         tokenTypes[TOKEN_IN],
	"instanceof": tokenTypes[TOKEN_INSTANCEOF],
	"typeof":     tokenTypes[TOKEN_TYPEOF],
	"void":       tokenTypes[TOKEN_VOID],
	"delete":     tokenTypes[TOKEN_DELETE],
}

func newToken(label string, keyword string, overrides map[string]bool, binop *Binop, identifier Token) *TokenType {
	defaults := map[string]bool{
		"beforeExpr": false,
		"startsExpr": false,
		"isLoop":     false,
		"isAssign":   false,
		"prefix":     false,
		"postfix":    false,
	}

	for k, v := range overrides {
		defaults[k] = v
	}

	return &TokenType{
		label:         label,
		keyword:       keyword,
		beforeExpr:    defaults["beforeExpr"],
		startsExpr:    defaults["startsExpr"],
		isLoop:        defaults["isLoop"],
		isAssign:      defaults["isAssign"],
		prefix:        defaults["prefix"],
		postfix:       defaults["postfix"],
		binop:         binop,
		updateContext: nil,
		identifier:    identifier,
	}
}

// TOKEN CONTEXT
type TokenContextType int

const (
	BRACKET_STATEMENT TokenContextType = iota
	BRACKET_EXPRESSION
	BRACKET_TEMPLATE
	PAREN_STATEMENT
	PAREN_EXPRESSION
	QUOTE_TEMPLATE
	FUNCTION_STATEMENT
	FUNCTION_EXPRESSION
	FUNCTION_EXPRESSION_GENERATOR
	FUNCTION_GENERATOR
)

var TokenContexts = map[TokenContextType]*TokenContext{
	BRACKET_STATEMENT:             newTokContext("{", false, false, false, nil, BRACKET_STATEMENT),
	BRACKET_EXPRESSION:            newTokContext("{", true, false, false, nil, BRACKET_EXPRESSION),
	BRACKET_TEMPLATE:              newTokContext("${", false, false, false, nil, BRACKET_TEMPLATE),
	PAREN_STATEMENT:               newTokContext("(", false, false, false, nil, PAREN_STATEMENT),
	PAREN_EXPRESSION:              newTokContext("(", true, false, false, nil, PAREN_EXPRESSION),
	QUOTE_TEMPLATE:                newTokContext("`", true, true, false, func(p *Parser) { p.tryReadTemplateToken() }, QUOTE_TEMPLATE),
	FUNCTION_STATEMENT:            newTokContext("function", false, false, false, nil, FUNCTION_STATEMENT),
	FUNCTION_EXPRESSION:           newTokContext("function", true, false, false, nil, FUNCTION_EXPRESSION),
	FUNCTION_EXPRESSION_GENERATOR: newTokContext("function", true, false, true, nil, FUNCTION_EXPRESSION_GENERATOR),
	FUNCTION_GENERATOR:            newTokContext("function", false, false, true, nil, FUNCTION_GENERATOR),
}

type TokenContext struct {
	Token         string
	IsExpr        bool
	PreserveSpace bool
	Override      func(*Parser)
	Generator     bool
	Identifier    TokenContextType
}

func newTokContext(token string, isExpr, preserveSpace, generator bool, override func(*Parser), identifier TokenContextType) *TokenContext {
	return &TokenContext{
		Token:         token,
		IsExpr:        isExpr,
		PreserveSpace: preserveSpace,
		Generator:     generator,
		Override:      override,
		Identifier:    identifier,
	}
}
