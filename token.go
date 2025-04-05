package tokens

type Token int

const (
	// BASIC
	T_NUM Token = iota
	T_REGEXP
	T_STRING
	T_NAME
	T_PRIVATEID
	T_EOF

	// PUNCTUATION
	T_BRACKETL
	T_BRACKETR
	T_BRACEL
	T_BRACER
	T_PARENL
	T_PARENR
	T_COMMA
	T_SEMI
	T_COLON
	T_DOT
	T_QUESTION
	T_QUESTIONDOT
	T_ARROW
	T_TEMPLATE
	T_INVALIDTEMPLATE
	T_ELLIPSIS
	T_BACKQUOTE
	T_DOLLARBRACEL

	// Operator token types
	T_EQ
	T_ASSIGN
	T_INCDEC
	T_PREFIX
	T_LOGICALOR
	T_LOGICALAND
	T_BITWISEOR
	T_BITWISEXOR
	T_BITWISEAND
	T_EQUALITY
	T_RELATIONAL
	T_BITSHIFT
	T_PLUSMIN
	T_MODULO
	T_STAR
	T_SLASH
	T_STARSTAR
	T_COALESCE

	// Keywords
	T_BREAK
	T_CASE
	T_CATCH
	T_CONTINUE
	T_DEBUGGER
	T_DEFAULT
	T_DO
	T_ELSE
	T_FINALLY
	T_FOR
	T_FUNCTION
	T_IF
	T_RETURN
	T_SWITCH
	T_THROW
	T_TRY
	T_VAR
	T_CONST
	T_WHILE
	T_WITH
	T_NEW
	T_THIS
	T_SUPER
	T_CLASS
	T_EXTENDS
	T_EXPORT
	T_IMPORT
	T_NULL
	T_TRUE
	T_FALSE
	T_IN
	T_INSTANCEOF
	T_TYPEOF
	T_VOID
	T_DELETE
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
}

var Tokens = make(map[Token]*TokenType)

func InitTokens() {
	// Basic token types
	Tokens[T_NUM] = newToken("num", "", map[string]bool{"startsExpr": true}, nil)
	Tokens[T_REGEXP] = newToken("regexp", "", map[string]bool{"startsExpr": true}, nil)
	Tokens[T_STRING] = newToken("string", "", map[string]bool{"startsExpr": true}, nil)
	Tokens[T_NAME] = newToken("name", "", map[string]bool{"startsExpr": true}, nil)
	Tokens[T_PRIVATEID] = newToken("privateId", "", map[string]bool{"startsExpr": true}, nil)
	Tokens[T_EOF] = newToken("eof", "", map[string]bool{}, nil)

	// Punctuation token types
	Tokens[T_BRACKETL] = newToken("[", "", map[string]bool{"beforeExpr": true, "startsExpr": true}, nil)
	Tokens[T_BRACKETR] = newToken("]", "", map[string]bool{}, nil)
	Tokens[T_BRACEL] = newToken("{", "", map[string]bool{"beforeExpr": true, "startsExpr": true}, nil)
	Tokens[T_BRACER] = newToken("}", "", map[string]bool{}, nil)
	Tokens[T_PARENL] = newToken("(", "", map[string]bool{"beforeExpr": true, "startsExpr": true}, nil)
	Tokens[T_PARENR] = newToken(")", "", map[string]bool{}, nil)
	Tokens[T_COMMA] = newToken(",", "", map[string]bool{"beforeExpr": true}, nil)
	Tokens[T_SEMI] = newToken(";", "", map[string]bool{"beforeExpr": true}, nil)
	Tokens[T_COLON] = newToken(":", "", map[string]bool{"beforeExpr": true}, nil)
	Tokens[T_DOT] = newToken(".", "", map[string]bool{}, nil)
	Tokens[T_QUESTION] = newToken("?", "", map[string]bool{"beforeExpr": true}, nil)
	Tokens[T_QUESTIONDOT] = newToken("?.", "", map[string]bool{}, nil)
	Tokens[T_ARROW] = newToken("=>", "", map[string]bool{"beforeExpr": true}, nil)
	Tokens[T_TEMPLATE] = newToken("template", "", map[string]bool{}, nil)
	Tokens[T_INVALIDTEMPLATE] = newToken("invalidTemplate", "", map[string]bool{}, nil)
	Tokens[T_ELLIPSIS] = newToken("...", "", map[string]bool{"beforeExpr": true}, nil)
	Tokens[T_BACKQUOTE] = newToken("`", "", map[string]bool{"startsExpr": true}, nil)
	Tokens[T_DOLLARBRACEL] = newToken("${", "", map[string]bool{"beforeExpr": true, "startsExpr": true}, nil)

	// Operator token types
	Tokens[T_EQ] = newToken("=", "", map[string]bool{"beforeExpr": true, "isAssign": true}, nil)
	Tokens[T_ASSIGN] = newToken("_=", "", map[string]bool{"beforeExpr": true, "isAssign": true}, nil)
	Tokens[T_INCDEC] = newToken("++/--", "", map[string]bool{"prefix": true, "postfix": true, "startsExpr": true}, nil)
	Tokens[T_PREFIX] = newToken("!/~", "", map[string]bool{"beforeExpr": true, "prefix": true, "startsExpr": true}, nil)
	Tokens[T_LOGICALOR] = newToken("||", "", map[string]bool{}, &Binop{prec: 1})
	Tokens[T_LOGICALAND] = newToken("&&", "", map[string]bool{}, &Binop{prec: 2})
	Tokens[T_BITWISEOR] = newToken("|", "", map[string]bool{}, &Binop{prec: 3})
	Tokens[T_BITWISEXOR] = newToken("^", "", map[string]bool{}, &Binop{prec: 4})
	Tokens[T_BITWISEAND] = newToken("&", "", map[string]bool{}, &Binop{prec: 5})
	Tokens[T_EQUALITY] = newToken("==/!=/===/!==", "", map[string]bool{}, &Binop{prec: 6})
	Tokens[T_RELATIONAL] = newToken("</>/<=/>=", "", map[string]bool{}, &Binop{prec: 7})
	Tokens[T_BITSHIFT] = newToken("<</>>/>>>", "", map[string]bool{}, &Binop{prec: 8})
	Tokens[T_PLUSMIN] = newToken("+/-", "", map[string]bool{"beforeExpr": true, "prefix": true, "startsExpr": true}, &Binop{prec: 9})
	Tokens[T_MODULO] = newToken("%", "", map[string]bool{}, &Binop{prec: 10})
	Tokens[T_STAR] = newToken("*", "", map[string]bool{}, &Binop{prec: 10})
	Tokens[T_SLASH] = newToken("/", "", map[string]bool{}, &Binop{prec: 10})
	Tokens[T_STARSTAR] = newToken("**", "", map[string]bool{"beforeExpr": true}, nil)
	Tokens[T_COALESCE] = newToken("??", "", map[string]bool{}, &Binop{prec: 1})

	// Keywords
	Tokens[T_BREAK] = newToken("break", "break", map[string]bool{}, nil)
	Tokens[T_CASE] = newToken("case", "case", map[string]bool{"beforeExpr": true}, nil)
	Tokens[T_CATCH] = newToken("catch", "catch", map[string]bool{}, nil)
	Tokens[T_CONTINUE] = newToken("continue", "continue", map[string]bool{}, nil)
	Tokens[T_DEBUGGER] = newToken("debugger", "debugger", map[string]bool{}, nil)
	Tokens[T_DEFAULT] = newToken("default", "default", map[string]bool{"beforeExpr": true}, nil)
	Tokens[T_DO] = newToken("do", "do", map[string]bool{"isLoop": true, "beforeExpr": true}, nil)
	Tokens[T_ELSE] = newToken("else", "else", map[string]bool{"beforeExpr": true}, nil)
	Tokens[T_FINALLY] = newToken("finally", "finally", map[string]bool{}, nil)
	Tokens[T_FOR] = newToken("for", "for", map[string]bool{"isLoop": true}, nil)
	Tokens[T_FUNCTION] = newToken("function", "function", map[string]bool{"startsExpr": true}, nil)
	Tokens[T_IF] = newToken("if", "if", map[string]bool{}, nil)
	Tokens[T_RETURN] = newToken("return", "return", map[string]bool{"beforeExpr": true}, nil)
	Tokens[T_SWITCH] = newToken("switch", "switch", map[string]bool{}, nil)
	Tokens[T_THROW] = newToken("throw", "throw", map[string]bool{"beforeExpr": true}, nil)
	Tokens[T_TRY] = newToken("try", "try", map[string]bool{}, nil)
	Tokens[T_VAR] = newToken("var", "var", map[string]bool{}, nil)
	Tokens[T_CONST] = newToken("const", "const", map[string]bool{}, nil)
	Tokens[T_WHILE] = newToken("while", "while", map[string]bool{"isLoop": true}, nil)
	Tokens[T_WITH] = newToken("with", "with", map[string]bool{}, nil)
	Tokens[T_NEW] = newToken("new", "new", map[string]bool{"beforeExpr": true, "startsExpr": true}, nil)
	Tokens[T_THIS] = newToken("this", "this", map[string]bool{"startsExpr": true}, nil)
	Tokens[T_SUPER] = newToken("super", "super", map[string]bool{"startsExpr": true}, nil)
	Tokens[T_CLASS] = newToken("class", "class", map[string]bool{"startsExpr": true}, nil)
	Tokens[T_EXTENDS] = newToken("extends", "extends", map[string]bool{"beforeExpr": true}, nil)
	Tokens[T_EXPORT] = newToken("export", "export", map[string]bool{}, nil)
	Tokens[T_IMPORT] = newToken("import", "import", map[string]bool{"startsExpr": true}, nil)
	Tokens[T_NULL] = newToken("null", "null", map[string]bool{"startsExpr": true}, nil)
	Tokens[T_TRUE] = newToken("true", "true", map[string]bool{"startsExpr": true}, nil)
	Tokens[T_FALSE] = newToken("false", "false", map[string]bool{"startsExpr": true}, nil)
	Tokens[T_IN] = newToken("in", "in", map[string]bool{"beforeExpr": true}, &Binop{prec: 7})
	Tokens[T_INSTANCEOF] = newToken("instanceof", "instanceof", map[string]bool{"beforeExpr": true}, &Binop{prec: 7})
	Tokens[T_TYPEOF] = newToken("typeof", "typeof", map[string]bool{"beforeExpr": true, "prefix": true, "startsExpr": true}, nil)
	Tokens[T_VOID] = newToken("void", "void", map[string]bool{"beforeExpr": true, "prefix": true, "startsExpr": true}, nil)
	Tokens[T_DELETE] = newToken("delete", "delete", map[string]bool{"beforeExpr": true, "prefix": true, "startsExpr": true}, nil)
}

func newToken(label string, keyword string, overrides map[string]bool, binop *Binop) *TokenType {
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
	}
}
