package parser

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

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

func (t Token) MarshalJSON() ([]byte, error) {
	name, ok := tokenToString[t]

	if !ok {
		name = "UnknownToken"
	}

	return json.Marshal(name)
}

type Binop struct {
	prec int
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

// TOKEN RELATED CODE

// Move to next token
func (p *Parser) next(ignoreEscapeSequenceInKeyword bool) error {
	if !ignoreEscapeSequenceInKeyword && len(p.Type.keyword) != 0 && p.ContainsEsc {
		return p.raiseRecoverable(p.start, "Escape sequence in keyword "+p.Type.keyword)
	}

	if p.options.OnToken != nil {
		// TODO? Maybe? I dont need this?
	}

	p.LastTokEnd = p.End
	p.LastTokStart = p.start
	p.LastTokEndLoc = p.startLoc
	p.LastTokStartLoc = p.startLoc
	p.nextToken()
	return nil
}

func (p *Parser) nextToken() {
	context := p.currentContext()
	if context == nil || !context.PreserveSpace {
		p.skipSpace()
	}

	p.start = p.pos
	if p.options.Locations {
		p.startLoc = p.currentPosition()
	}

	if p.pos >= len(p.input) {
		p.finishToken(tokenTypes[TOKEN_EOF], nil)
		return
	}

	if context.Override != nil {
		context.Override(p)
		return
	} else {
		ch, size, _ := p.fullCharCodeAtPos()
		p.readToken(ch, size)
	}
}

func (p *Parser) fullCharCodeAtPos() (code rune, size int, err error) {
	if p.pos < 0 || p.pos >= len(p.input) {
		return 0, 0, p.raise(p.pos, "Invalid position")
	}
	r, size := utf8.DecodeRune(p.input[p.pos:])

	if r == utf8.RuneError {

		return 0, size, p.raise(p.pos, "Invalid UTF-8 sequence")
	}
	if r <= 0xD7FF || r >= 0xDC00 {
		return r, size, nil
	}
	if p.pos+size >= len(p.input) {
		return r, size, nil
	}
	next, nextSize := utf8.DecodeRune(p.input[p.pos+size:])
	if next == utf8.RuneError {
		return r, size, nil
	}
	if next <= 0xDBFF || next >= 0xE000 {
		return r, size, nil
	}
	return (r<<10 + next - 0x35FDC00), size + nextSize, nil
}

func (p *Parser) readToken(code rune, size int) {
	if IsIdentifierStart(code, p.getEcmaVersion() >= 6) || code == 92 {
		p.readWord()
		return
	}
	p.getTokenFromCode(code, size)
}

func (p *Parser) finishToken(Type *TokenType, value any) {
	p.End = p.pos
	if p.options.Locations {
		p.EndLoc = p.currentPosition()
	}
	prevType := p.Type
	p.Type = Type
	p.Value = value
	p.updateContext(prevType)
}

func (p *Parser) getTokenFromCode(code rune, size int) error {
	switch code {
	case 46: // '.'
		p.readToken_dot()
		return nil
	case 40: // '('
		p.pos = p.pos + size
		p.finishToken(tokenTypes[TOKEN_PARENL], nil)
		return nil

	case 41: // ')'
		p.pos = p.pos + size
		p.finishToken(tokenTypes[TOKEN_PARENR], nil)
		return nil

	case 59: // ';'
		p.pos = p.pos + size
		p.finishToken(tokenTypes[TOKEN_SEMI], nil)
		return nil

	case 44: // ','
		p.pos = p.pos + size
		p.finishToken(tokenTypes[TOKEN_COMMA], nil)
		return nil

	case 91: // '['
		p.pos = p.pos + size
		p.finishToken(tokenTypes[TOKEN_BRACKETL], nil)
		return nil

	case 93: // ']'
		p.pos = p.pos + size
		p.finishToken(tokenTypes[TOKEN_BRACKETR], nil)
		return nil

	case 123: // '{'
		p.pos = p.pos + size
		p.finishToken(tokenTypes[TOKEN_BRACEL], nil)
		return nil

	case 125: // '}'
		p.pos = p.pos + size
		p.finishToken(tokenTypes[TOKEN_BRACER], nil)

		return nil

	case 58: // ':'
		p.pos = p.pos + size
		p.finishToken(tokenTypes[TOKEN_COLON], nil)
		return nil

	case 96: // '`'
		if p.getEcmaVersion() < 6 {
			break
		}
		p.pos = p.pos + size
		p.finishToken(tokenTypes[TOKEN_BACKQUOTE], nil)
		return nil

	case 48: // '0'
		next := p.input[p.pos+1]
		if next == 120 || next == 88 { // 'x', 'X'
			return p.readRadixNumber(16) // hex number

		}
		if p.getEcmaVersion() >= 6 {
			if next == 111 || next == 79 { // 'o', 'O'
				return p.readRadixNumber(8) // octal number

			}
			if next == 98 || next == 66 { // 'b', 'B'
				return p.readRadixNumber(2) // binary number
			}
		}
		return p.readNumber(false)

	case 49, 50, 51, 52, 53, 54, 55, 56, 57: // '1'-'9'
		return p.readNumber(false)

	case 34, 39: // '"', "'"
		return p.readString(code)

	case 47: // '/'
		return p.readToken_slash()

	case 37, 42: // '%', '*'
		p.readToken_mult_modulo_exp(code)
		return nil

	case 124, 38: // '|', '&'
		p.readToken_pipe_amp(code)
		return nil

	case 94: // '^'
		p.readToken_caret()
		return nil

	case 43, 45: // '+', '-'
		p.readToken_plus_min(code)
		return nil

	case 60, 62: // '<', '>'
		p.readToken_lt_gt(code)
		return nil

	case 61, 33: // '=', '!'
		p.readToken_eq_excl(code)
		return nil

	case 63: // '?'
		p.readToken_question()
		return nil

	case 126: // '~'
		p.finishOp(tokenTypes[TOKEN_PREFIX], 1)
		return nil

	case 35: // '#'
		return p.readToken_numberSign()
	}
	return p.raise(p.pos, "Unexpected character '"+CodePointToString(code)+"'")
}

func (p *Parser) finishOp(token *TokenType, size int) {
	str := p.input[p.pos : p.pos+size]
	p.pos = p.pos + size
	p.finishToken(token, str)
}

func (p *Parser) readToken_question() {
	ecmaVersion := p.options.ecmaVersion.(int)
	if ecmaVersion >= 11 {
		next := p.input[p.pos+1]
		if next == 46 {
			next2 := p.input[p.pos+2]
			if next2 < 48 || next2 > 57 {
				p.finishOp(tokenTypes[TOKEN_QUESTIONDOT], 2)
				return
			}
		}
		if next == 63 {
			if ecmaVersion >= 12 {
				next2 := p.input[p.pos+2]
				if next2 == 61 {
					p.finishOp(tokenTypes[TOKEN_ASSIGN], 3)
					return
				}
			}
			p.finishOp(tokenTypes[TOKEN_COALESCE], 2)
			return
		}
	}
	p.finishOp(tokenTypes[TOKEN_QUESTION], 1)
}

func (p *Parser) readToken_eq_excl(code rune) {
	next := p.input[p.pos+1]

	if code == 61 && next == 62 && p.getEcmaVersion() >= 6 {
		p.pos += 2
		p.finishToken(tokenTypes[TOKEN_ARROW], nil)
		return
	}
	if next == 61 {
		size := 2
		if p.input[p.pos+2] == 61 {
			size = 3 // === or !==
		}
		p.finishOp(tokenTypes[TOKEN_EQUALITY], size)
		return
	}

	if code == 61 && next == 62 && p.getEcmaVersion() >= 6 { // '=>'
		p.pos += 2
		p.finishToken(tokenTypes[TOKEN_ARROW], nil)
		return
	}
	if code == 61 {
		p.finishOp(tokenTypes[TOKEN_EQ], 1)
		return
	}

	p.finishOp(tokenTypes[TOKEN_PREFIX], 1)
}

func (p *Parser) readToken_lt_gt(code rune) {
	next := rune(p.input[p.pos+1])
	size := 1
	if next == code {
		if code == 62 && p.input[p.pos+2] == 62 {
			size = 3
		} else {
			size = 2
		}

		if p.input[p.pos+size] == 61 {
			p.finishOp(tokenTypes[TOKEN_ASSIGN], size+1)
			return
		}
		p.finishOp(tokenTypes[TOKEN_BITSHIFT], size)
		return
	}
	if next == 33 && code == 60 && !p.InModule && p.input[p.pos+2] == 45 &&
		p.input[p.pos+3] == 45 {
		// `<!--`, an XML-style comment that should be interpreted as a line comment
		p.skipLineComment(4)
		p.skipSpace()
		p.nextToken()
		return
	}
	if next == 61 {
		size = 2
	}
	p.finishOp(tokenTypes[TOKEN_RELATIONAL], size)
}

func (p *Parser) readToken_plus_min(code rune) {
	next := rune(p.input[p.pos+1])
	if next == code {
		if next == 45 && !p.InModule && p.input[p.pos+2] == 62 &&
			(p.LastTokEnd == 0 || lineBreak.Match([]byte(p.input[p.LastTokEnd:p.pos]))) {
			// A `-->` line comment
			p.skipLineComment(3)
			p.skipSpace()
			p.nextToken()
			return
		}
		p.finishOp(tokenTypes[TOKEN_INCDEC], 2)
		return
	}
	if next == 61 {
		p.finishOp(tokenTypes[TOKEN_ASSIGN], 2)
		return
	}
	p.finishOp(tokenTypes[TOKEN_PLUSMIN], 1)
}

func (p *Parser) skipLineComment(startSkip int) {
	ch := p.input[p.pos+startSkip]
	p.pos = p.pos + startSkip
	for p.pos < len(p.input) && !isNewLine(rune(ch)) {
		p.pos = p.pos + 1
		ch = p.input[p.pos]
	}

	if p.options.OnComment != nil {
		// TODO I don't really have onComment ported and might be that it never happens
		/*
			p.Options.OnComment.(false, p.input.slice(start+startSkip, p.pos), start, p.pos,
				startLoc, p.curPosition())
		*/
	}
}

func (p *Parser) readToken_caret() {
	next := p.input[p.pos+1]
	if next == 61 {
		p.finishOp(tokenTypes[TOKEN_ASSIGN], 2)
		return
	}
	p.finishOp(tokenTypes[TOKEN_BITWISEXOR], 1)
}

func (p *Parser) readToken_pipe_amp(code rune) {
	next := rune(p.input[p.pos+1])
	if next == code {
		if p.getEcmaVersion() >= 12 {
			next2 := p.input[p.pos+2]
			if next2 == 61 {
				p.finishOp(tokenTypes[TOKEN_ASSIGN], 3)
				return
			}

			if code == 124 {
				p.finishOp(tokenTypes[TOKEN_LOGICALOR], 2)
				return
			} else {
				p.finishOp(tokenTypes[TOKEN_LOGICALAND], 2)
				return
			}
		}
	}

	if next == 61 {
		p.finishOp(tokenTypes[TOKEN_ASSIGN], 2)
		return
	}

	if code == 124 {
		p.finishOp(tokenTypes[TOKEN_BITWISEOR], 1)
		return
	}

	p.finishOp(tokenTypes[TOKEN_BITWISEAND], 1)
}

func (p *Parser) readToken_mult_modulo_exp(code rune) {
	next := p.input[p.pos+1]
	size := 1

	var tokenType *TokenType

	if code == 42 {
		tokenType = tokenTypes[TOKEN_STAR]
	} else {
		tokenType = tokenTypes[TOKEN_MODULO]
	}

	// exponentiation operator ** and **=
	if p.getEcmaVersion() >= 7 && code == 42 && next == 42 {
		size = size + 1
		tokenType = tokenTypes[TOKEN_STAR]
		next = p.input[p.pos+2]
	}

	if next == 61 {
		p.finishOp(tokenTypes[TOKEN_ASSIGN], size+1)
		return
	}

	p.finishOp(tokenType, size)
}

func (p *Parser) readToken_slash() error {
	next := p.input[p.pos+1]
	if p.ExprAllowed {
		p.pos = p.pos + 1
		return p.readRegexp()
	}
	if next == 61 {
		p.finishOp(tokenTypes[TOKEN_ASSIGN], 2)
		return nil
	}
	p.finishOp(tokenTypes[TOKEN_SLASH], 1)
	return nil
}

func (p *Parser) readRegexp() error {
	escaped, inClass, start := p.pos == 0, p.pos == 0, p.pos
	for {
		if p.pos >= len(p.input) {
			return p.raise(start, "Unterminated regular expression")

		}
		ch := p.input[p.pos]
		if lineBreak.Match([]byte{ch}) {
			return p.raise(start, "Unterminated regular expression")
		}

		if !escaped {
			if ch == '[' {
				inClass = true
			} else if ch == ']' && inClass {
				inClass = false
			} else if ch == '/' && !inClass {
				break
			}
			escaped = ch == '\\'
		} else {
			escaped = false
		}

		p.pos = p.pos + 1
	}

	pattern := p.input[start:p.pos]
	p.pos = p.pos + 1
	flagsStart := p.pos
	flags, err := p.readWord1()
	if err != nil {
		return p.raise(p.pos, "Failed to read regExp flags")
	}
	if p.ContainsEsc {
		return p.unexpected("", &flagsStart)
	}

	// Validate pattern
	var state *RegExpState
	if p.RegexpState != nil {
		state = p.RegexpState
	} else {
		p.RegexpState = p.NewRegExpState()
		state = p.RegexpState
	}

	state.reset(start, string(pattern), flags)
	p.validateRegExpFlags(state)
	p.validateRegExpPattern(state)

	// Create Literal#value property value.

	value := &regexp.Regexp{} // new RegExp(pattern, flags)

	p.finishToken(tokenTypes[TOKEN_REGEXP], struct {
		pattern string
		flags   string
		value   *regexp.Regexp
	}{
		pattern: string(pattern),
		flags:   flags,
		value:   value,
	})
	return nil
}

func (p *Parser) validateRegExpPattern(state *RegExpState) {
	panic("unimplemented")
}

func (p *Parser) validateRegExpFlags(state *RegExpState) {
	panic("unimplemented")
}

func (p *Parser) readString(quote rune) error {
	p.pos = p.pos + 1
	// Potential improvement: Use bytes.Buffer
	out, chunkStart := []byte{}, p.pos
	for {
		if p.pos >= len(p.input) {
			return p.raise(p.start, "Unterminated string constant")
		}
		ch, size, _ := p.fullCharCodeAtPos()
		if ch == quote {
			break
		}
		if ch == 92 { // '\'
			out = append(out, p.input[chunkStart:p.pos]...)
			escapedChar, _ := p.readEscapedChar(false)
			out = append(out, []byte(escapedChar)...)
			chunkStart = p.pos
		} else if ch == 0x2028 || ch == 0x2029 {
			if p.getEcmaVersion() < 10 {
				return p.raise(p.start, "Unterminated string constant")

			}
			p.pos = p.pos + 1
			if p.options.Locations {
				p.CurLine++
				p.LineStart = p.pos
			}
		} else {
			if isNewLine(rune(ch)) {
				return p.raise(p.start, "Unterminated string constant")
			}
			p.pos = p.pos + size
		}
	}
	out = append(out, p.input[chunkStart:p.pos]...)
	p.pos = p.pos + 1
	p.finishToken(tokenTypes[TOKEN_STRING], out)
	return nil
}

func (p *Parser) readNumber(startsWithDot bool) error {
	start := p.pos
	_, err := p.readInt(10, nil, true)
	if !startsWithDot && err != nil {
		return p.raise(start, "Invalid number")
	}
	octal := p.pos-start >= 2 && p.input[start] == 48
	if octal && p.Strict {
		return p.raise(start, "Invalid number")
	}
	next := math.MaxInt
	if p.pos < len(p.input) {
		next = int(p.input[p.pos])
	}

	if !octal && !startsWithDot && p.getEcmaVersion() >= 11 && next == 110 {
		val := stringToBigInt(p.input[start:p.pos])
		p.pos = p.pos + 1
		ch, _, _ := p.fullCharCodeAtPos()
		if IsIdentifierStart(ch, false) {
			return p.raise(p.pos, "Identifier directly after number")

		}
		p.finishToken(tokenTypes[TOKEN_NUM], val)
		return nil
	}
	regExp := regexp.MustCompile("[89]")
	if octal && regExp.Match(p.input[start:p.pos]) {
		octal = false
	}
	if next == 46 && !octal { // '.'
		p.pos = p.pos + 1
		p.readInt(10, nil, false)
		next = int(p.input[p.pos])
	}
	if (next == 69 || next == 101) && !octal { // 'eE'
		p.pos = p.pos + 1
		next = int(p.input[p.pos])
		if next == 43 || next == 45 { // '+-'
			p.pos = p.pos + 1
		}

		_, err := p.readInt(10, nil, false)
		if err != nil {
			return p.raise(start, "Invalid number")
		}
	}
	ch, _, _ := p.fullCharCodeAtPos()

	if IsIdentifierStart(ch, false) {
		return p.raise(p.pos, "Identifier directly after number")
	}

	val := stringToNumber(p.input[start:p.pos], octal)
	p.finishToken(tokenTypes[TOKEN_NUM], val)
	return nil
}

func stringToNumber(b []byte, octal bool) float64 {
	/*
			This is missing and I don't have patience to do it
			  if (isLegacyOctalNumericLiteral) {
		    return parseInt(str, 8)
		  }
	*/

	numToConvert := strings.Replace(string(b), "_", "", -1)
	num, _ := strconv.ParseFloat(numToConvert, 64)
	return num
}
func stringToBigInt(b []byte) int {
	panic("unimplemented")
}

func (p *Parser) readRadixNumber(radix int) error {
	start := p.pos
	p.pos += 2 // 0x
	val, err := p.readInt(radix, nil, false)
	if err != nil {
		return p.raise(p.start+2, string("Expected number in radix ")+strconv.Itoa(radix))
	}
	ch, _, _ := p.fullCharCodeAtPos()
	if p.getEcmaVersion() >= 11 && p.input[p.pos] == 110 {
		val = stringToBigInt(p.input[start:p.pos])
		p.pos = p.pos + 1
	} else if IsIdentifierStart(ch, false) {
		return p.raise(p.pos, "Identifier directly after number")
	}
	p.finishToken(tokenTypes[TOKEN_NUM], val)
	return nil
}

func (p *Parser) readToken_numberSign() error {
	ecmaVersion := p.options.ecmaVersion.(int)
	code := rune(35) // '#'
	if ecmaVersion >= 13 {
		p.pos = p.pos + 1
		quote, _, _ := p.fullCharCodeAtPos()
		if IsIdentifierStart(quote, true) || quote == 92 /* '\' */ {

			str, err := p.readWord1()
			if err != nil {
				return p.raise(p.pos, "Failed to read string")
			}
			p.finishToken(tokenTypes[TOKEN_PRIVATEID], str)
			return nil
		}
	}

	return p.raise(p.pos, "Unexpected character '"+CodePointToString(code)+"'")
}

func (p *Parser) tryReadTemplateToken() error {
	p.InTemplateElement = true

	err := p.readTmplToken()

	if err != nil {
		p.readInvalidTemplateToken()
	}

	p.InTemplateElement = false
	return err
}

func (p *Parser) readInvalidTemplateToken() error {
	for p.pos < len(p.input) {
		ch, size, err := p.fullCharCodeAtPos()
		if err != nil { // Error from fullCharCodeAtPos
			return p.raise(p.pos, "Invalid character in template: "+err.Error())

		}
		switch ch {
		case '\\':
			p.pos += size
		case '$':
			if p.pos+size < len(p.input) {
				next, _ := utf8.DecodeRune(p.input[p.pos+size:])
				if next == '{' {
					p.finishToken(tokenTypes[TOKEN_INVALIDTEMPLATE], p.input[p.start:p.pos])
					return nil
				}
			}
			p.pos += size
		case '`':
			p.finishToken(tokenTypes[TOKEN_INVALIDTEMPLATE], p.input[p.start:p.pos])
			return nil
		case '\r':
			p.pos += size
			if p.pos < len(p.input) {
				next, nextSize := utf8.DecodeRune(p.input[p.pos:])
				if next == '\n' {
					p.pos += nextSize
				}
			}
			p.CurLine++
			p.LineStart = p.pos
		case '\n', 0x2028, 0x2029:
			p.pos += size
			p.CurLine++
			p.LineStart = p.pos
		default:
			p.pos += size
		}
	}
	return p.raise(p.start, "Unterminated template")
}

func (p *Parser) readTmplToken() error {
	// Potential improvement: use bytes.Buffer
	out := []byte{}
	chunkStart := p.pos
	for {
		if p.pos >= len(p.input) {
			return p.raise(p.start, "Unterminated template")
		}
		ch := p.input[p.pos]
		if ch == 96 || ch == 36 && p.input[p.pos+1] == 123 { // '`', '${'
			if p.pos == p.start && p.Type.identifier == TOKEN_TEMPLATE || p.Type.identifier == TOKEN_INVALIDTEMPLATE {
				if ch == 36 {
					p.pos += 2
					p.finishToken(tokenTypes[TOKEN_DOLLARBRACEL], nil)
					return nil
				} else {
					p.pos = p.pos + 1
					p.finishToken(tokenTypes[TOKEN_BACKQUOTE], nil)
					return nil
				}
			}
			out = append(out, p.input[chunkStart:p.pos]...)

			p.finishToken(tokenTypes[TOKEN_TEMPLATE], out)
			return nil
		}

		if ch == 92 { // '\'
			out = append(out, p.input[chunkStart:p.pos]...)
			escaped, _ := p.readEscapedChar(true)
			out = append(out, []byte(escaped)...)
			chunkStart = p.pos
		} else if isNewLine(rune(ch)) {
			out = append(out, p.input[chunkStart:p.pos]...)
			p.pos = p.pos + 1
			switch ch {
			case 13:
				if p.input[p.pos] == 10 {
					p.pos = p.pos + 1
				}
			case 10:
				out = append(out, "\n"...)
			default:
				out = append(out, ch)
			}
			if p.options.Locations {
				p.CurLine = p.CurLine + 1
				p.LineStart = p.pos
			}
			chunkStart = p.pos
		} else {
			p.pos = p.pos + 1
		}
	}
}

func (p *Parser) readEscapedChar(inTemplate bool) (string, error) {
	if p.pos >= len(p.input) {
		return "", p.invalidStringToken(p.pos, "Unexpected end of input after backslash")
	}
	p.pos = p.pos + 1 // Skip backslash
	r, size := utf8.DecodeRune(p.input[p.pos:])
	if r == utf8.RuneError {

		return "", p.invalidStringToken(p.pos, "Invalid UTF-8 sequence")
	}
	p.pos += size
	ch := int(r)

	switch ch {
	case 'n':
		return "\n", nil
	case 'r':
		return "\r", nil
	case 'x':
		hexCh, err := p.readHexChar(2)
		return string(hexCh), err
	case 'u':
		code, err := p.readCodePoint()
		return CodePointToString(code), err
	case 't':
		return "\t", nil
	case 'b':
		return "\b", nil
	case 'v':
		return "\u000b", nil
	case 'f':
		return "\f", nil
	case '\r':
		if p.pos < len(p.input) && p.input[p.pos] == '\n' {
			p.pos = p.pos + size
		}
		fallthrough
	case '\n':
		if p.options.Locations {
			p.LineStart = p.pos
			p.CurLine++
		}
		return "", nil
	case '8', '9':
		if p.Strict {
			return "", p.invalidStringToken(p.pos-1, "Invalid escape sequence")
		}
		if inTemplate {
			return "", p.invalidStringToken(p.pos-1, "Invalid escape sequence in template string")
		}
		return string(rune(ch)), nil
	default:
		if ch >= '0' && ch <= '7' {
			// Octal escape: read up to 3 digits
			startPos := p.pos - size
			octalStr := string(rune(ch))
			for i := 0; i < 2 && p.pos < len(p.input); i++ {
				nextCh, nextSize := utf8.DecodeRune(p.input[p.pos:])
				if nextCh < '0' || nextCh > '7' {
					break
				}
				octalStr += string(nextCh)
				p.pos += nextSize
			}
			octal, err := strconv.ParseInt(octalStr, 8, 64)
			if err != nil {

				return "", p.invalidStringToken(startPos, "Invalid octal escape sequence")
			}
			if octal > 255 {
				octalStr = octalStr[:len(octalStr)-1]
				octal, _ = strconv.ParseInt(octalStr, 8, 64)
				p.pos -= size // Rewind last character
			}
			// Check for invalid octal escapes
			var nextCh rune
			if p.pos < len(p.input) {
				nextCh, _ = utf8.DecodeRune(p.input[p.pos:])
			}
			if (octalStr != "0" || nextCh == '8' || nextCh == '9') && (p.Strict || inTemplate) {
				msg := "Octal literal in strict mode"
				if inTemplate {
					msg = "Octal literal in template string"
				}

				return "", p.invalidStringToken(startPos, msg)
			}
			return string(rune(octal)), nil
		}
		if isNewLine(rune(ch)) {
			if p.options.Locations {
				p.LineStart = p.pos
				p.CurLine++
			}
			return "", nil
		}
		return string(rune(ch)), nil
	}
}

func (p *Parser) readWord() error {
	word, err := p.readWord1()
	if err != nil {
		return p.raise(p.pos, "We have failed")
	}
	t := tokenTypes[TOKEN_NAME]

	if tt, found := keywords[word]; found {
		t = tt
	}

	p.finishToken(t, word)
	return nil
}

func (p *Parser) readWord1() (string, error) {
	p.ContainsEsc = false
	word, first, chunkStart := []byte{}, true, p.pos

	astral := p.getEcmaVersion() >= 6

	for p.pos < len(p.input) {
		ch, size, _ := p.fullCharCodeAtPos()
		if IsIdentifierChar(ch, astral) {
			if ch <= 0xffff {
				p.pos = p.pos + size
			} else {
				p.pos = p.pos + size
			}
		} else if ch == 92 { // "\"
			p.ContainsEsc = true
			word = p.input[chunkStart:p.pos]
			escStart := p.pos
			p.pos = p.pos + size
			if p.input[p.pos] != 117 { // "u"

				return "", p.invalidStringToken(p.pos, "Expecting Unicode escape sequence \\uXXXX")
			}

			p.pos = p.pos + 1
			esc, _ := p.readCodePoint()

			if first {
				if !IsIdentifierStart(rune(esc), astral) {

					return "", p.invalidStringToken(escStart, "Invalid Unicode escape")
				}
			} else {
				if !IsIdentifierChar(rune(esc), astral) {

					return "", p.invalidStringToken(escStart, "Invalid Unicode escape")
				}
			}

			word = append(word, CodePointToString(esc)...)
			chunkStart = p.pos
		} else {
			break
		}
		first = false
	}
	return string(append(word, p.input[chunkStart:p.pos]...)), nil
}

func (p *Parser) invalidStringToken(pos int, message string) error {
	if p.InTemplateElement && p.getEcmaVersion() >= 9 {
		return p.raise(pos, "Invalid template literal")
	} else {
		return p.raise(pos, message)
	}
}

func (p *Parser) readCodePoint() (rune, error) {
	ch := p.input[p.pos]
	code := rune(0)

	if ch == 123 { // '{'
		if p.getEcmaVersion() < 6 {
			return 0, p.unexpected("ecma version < 6 and a brace left was present '{'", nil)
		}
		codePos := p.pos + 1
		p.pos = p.pos + 1
		hexCh, _ := p.readHexChar(len(p.input[p.pos:]) + strings.Index(string(p.input[p.pos:]), "}") - p.pos)
		code = hexCh
		p.pos = p.pos + 1
		if code > 0x10FFFF {
			return 0, p.invalidStringToken(codePos, "Code point out of bounds")
		}
	} else {
		hexCh, _ := p.readHexChar(4)
		code = hexCh
	}
	return code, nil

}

func (p *Parser) readHexChar(len int) (rune, error) {
	codePos := p.pos
	n, err := p.readInt(16, &len, false)
	if err != nil {
		return 0, p.invalidStringToken(codePos, "Bad character escape sequence")
	}
	return rune(n), nil
}

func (p *Parser) readInt(radix int, length *int, maybeLegacyOctalNumericLiteral bool) (int, error) {
	// `len` is used for character escape sequences. In that case, disallow separators.
	allowSeparators := p.getEcmaVersion() >= 12 && length == nil

	// `maybeLegacyOctalNumericLiteral` is true if it doesn't have prefix (0x,0o,0b)
	// and isn't fraction part nor exponent part. In that case, if the first digit
	// is zero then disallow separators.
	isLegacyOctalNumericLiteral := maybeLegacyOctalNumericLiteral && p.input[p.pos] == 48

	start, total, lastCode := p.pos, 0, 0
	e := 0

	if length == nil {
		e = math.MaxInt64
	} else {
		e = *length
	}
	for i := range e {
		code := math.MinInt64
		if p.pos < len(p.input) {
			code = int(p.input[p.pos])
		}

		val := 0

		if allowSeparators && code == 95 {
			if isLegacyOctalNumericLiteral {
				return 0, p.raiseRecoverable(p.pos-1, "Numeric separator is not allowed in legacy octal numeric literals")
			}
			if lastCode == 95 {
				return 0, p.raiseRecoverable(p.pos-1, "Numeric separator must be exactly one underscore")
			}
			if i == 0 {
				return 0, p.raiseRecoverable(p.pos-1, "Numeric separator is not allowed at the first of digits")
			}
			lastCode = code
			p.pos = p.pos + 1
			continue
		}

		if code >= 97 { // a
			val = code - 97 + 10
		} else if code >= 65 { // A
			val = code - 65 + 10
		} else if code >= 48 && code <= 57 { // 0-9
			val = code - 48
		} else {
			val = math.MaxInt64
		}
		if val >= radix {
			break
		}
		lastCode = code
		total = total*radix + val

		if p.pos < len(p.input) {
			p.pos = p.pos + 1
		}

	}

	if allowSeparators && lastCode == 95 {
		return 0, p.raiseRecoverable(p.pos-1, "Numeric separator is not allowed at the last of digits")

	}
	if p.pos == start || length != nil && p.pos-start != *length {
		return 0, p.raiseRecoverable(p.pos-1, "Error ? I dont know")
	}
	return total, nil
}

func (p *Parser) readToken_dot() error {
	next := p.input[p.pos+1]
	if next >= 48 && next <= 57 {
		return p.readNumber(true)
	}

	next2 := p.input[p.pos+2]
	if p.getEcmaVersion() >= 6 && next == 46 && next2 == 46 { // 46 = dot '.'
		p.pos += 3
		p.finishToken(tokenTypes[TOKEN_ELLIPSIS], nil)
		return nil
	}
	p.pos = p.pos + 1
	p.finishToken(tokenTypes[TOKEN_DOT], nil)
	return nil

}

func (p *Parser) currentPosition() *Location {
	return &Location{Line: p.CurLine, Column: p.pos - p.LineStart}
}
