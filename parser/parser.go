package parser

import (
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

type Label struct {
	Kind string
}

type PrivateName struct {
	Declared bool
	Used     bool
}

type Parser struct {
	options                  Options
	SourceFile               *string
	Keywords                 any
	ReservedWords            any
	ReservedWordsStrict      any
	ReservedWordsStrictBind  any
	input                    string
	ContainsEsc              bool
	pos                      int
	LineStart                int
	CurLine                  int
	Type                     *TokenType
	Value                    any
	Start                    int
	End                      int
	StartLoc                 *SourceLocation
	EndLoc                   *SourceLocation
	LastTokStart             int
	LastTokEnd               int
	LastTokStartLoc          *SourceLocation
	LastTokEndLoc            *SourceLocation
	Context                  []*TokenContext
	ExprAllowed              bool
	InModule                 bool
	Strict                   bool
	PotentialArrowAt         int
	PotentialArrowInForAwait bool
	YieldPos                 int
	AwaitPos                 int
	AwaitIdentPos            int
	Labels                   []Label
	UndefinedExports         map[string]any
	ScopeStack               []*Scope
	RegexpState              *RegExpState
	PrivateNameStack         []PrivateName
}

// TOKEN RELATED CODE

// Move to next token
func (this *Parser) next(ignoreEscapeSequenceInKeyword bool) {
	if !ignoreEscapeSequenceInKeyword && len(this.Type.keyword) != 0 && this.ContainsEsc {
		// this.raiseRecoverable(this.start, "Escape sequence in keyword " + this.type.keyword)
	}

	if this.options.OnToken != nil {
		// TODO? Maybe? I dont need this?
	}

	this.LastTokEnd = this.End
	this.LastTokStart = this.Start
	this.LastTokEndLoc = this.StartLoc
	this.LastTokStartLoc = this.StartLoc
	this.nextToken()
}

func (this *Parser) nextToken() {
	context := this.currentContext()
	if context == nil || context.PreserveSpace {
		this.skipSpace()
	}

	this.Start = this.pos
	if this.options.Locations {
		this.StartLoc = this.currentPosition()
	}

	if this.pos >= len(this.input) {
		this.finishToken(TokenTypes[TOKEN_EOF], nil)
		return
	}

	if context.Override != nil {
		context.Override(this)
		return
	} else {
		this.readToken(this.fullCharCodeAtPos())
	}
}

func (this *Parser) fullCharCodeAtPos() rune {
	if this.pos < 0 || this.pos >= len(this.input) {
		return 0 // error handling...
	}

	r, size := utf8.DecodeRuneInString(this.input[this.pos:])
	if r == utf8.RuneError {
		return 0 //  error handling...
	}

	code := int32(r)

	if code <= 0xD7FF || code >= 0xDC00 {
		return code
	}

	if this.pos+size >= len(this.input) {
		return code
	}

	nextRune, _ := utf8.DecodeRuneInString(this.input[this.pos+size:])
	next := int32(nextRune)

	if next <= 0xDBFF || next >= 0xE000 {
		return code
	}

	return (code<<10 + next - 0x35FDC00)
}

func (this *Parser) readToken(code rune) {
	if IsIdentifierStart(code, this.options.ecmaVersion.(int) >= 6) || code == 92 {
		this.readWord()
		return
	}

	this.getTokenFromCode(code)
}

func (this *Parser) getTokenFromCode(code rune) {
	switch code {
	case 46: // '.'
		this.readToken_dot()

	case 40: // '('
		this.pos++
		this.finishToken(TokenTypes[TOKEN_PARENL], nil)

	case 41: // ')'
		this.pos++
		this.finishToken(TokenTypes[TOKEN_PARENR], nil)

	case 59: // ';'
		this.pos++
		this.finishToken(TokenTypes[TOKEN_SEMI], nil)

	case 44: // ','
		this.pos++
		this.finishToken(TokenTypes[TOKEN_COMMA], nil)

	case 91: // '['
		this.pos++
		this.finishToken(TokenTypes[TOKEN_BRACKETL], nil)

	case 93: // ']'
		this.pos++
		this.finishToken(TokenTypes[TOKEN_BRACKETR], nil)

	case 123: // '{'
		this.pos++
		this.finishToken(TokenTypes[TOKEN_BRACEL], nil)

	case 125: // '}'
		this.pos++
		this.finishToken(TokenTypes[TOKEN_BRACER], nil)

	case 58: // ':'
		this.pos++
		this.finishToken(TokenTypes[TOKEN_COLON], nil)

	case 96: // '`'
		if this.options.ecmaVersion.(int) < 6 {
			break
		}
		this.pos++
		this.finishToken(TokenTypes[TOKEN_BACKQUOTE], nil)

	case 48: // '0'
		next := this.input[this.pos+1]
		if next == 120 || next == 88 { // 'x', 'X'
			this.readRadixNumber(16) // hex number
			return
		}
		if this.options.ecmaVersion.(int) >= 6 {
			if next == 111 || next == 79 { // 'o', 'O'
				this.readRadixNumber(8) // octal number
				return
			}
			if next == 98 || next == 66 { // 'b', 'B'
				this.readRadixNumber(2) // binary number
				return
			}
		}
		this.readNumber(false)

	case 49, 50, 51, 52, 53, 54, 55, 56, 57: // '1'-'9'
		this.readNumber(false)

	case 34, 39: // '"', "'"
		this.readString(code)

	case 47: // '/'
		this.readToken_slash()

	case 37, 42: // '%', '*'
		this.readToken_mult_modulo_exp(code)

	case 124, 38: // '|', '&'
		this.readToken_pipe_amp(code)

	case 94: // '^'
		this.readToken_caret()

	case 43, 45: // '+', '-'
		this.readToken_plus_min(code)

	case 60, 62: // '<', '>'
		this.readToken_lt_gt(code)

	case 61, 33: // '=', '!'
		this.readToken_eq_excl(code)

	case 63: // '?'
		this.readToken_question()

	case 126: // '~'
		this.finishOp(TokenTypes[TOKEN_PREFIX], 1)

	case 35: // '#'
		this.readToken_numberSign()
	}

	// this.raise(this.Pos, "Unexpected character '"+codePointToString(code)+"'")
}

func (this *Parser) finishOp(token *TokenType, size int) {
	str := this.input[this.pos : this.pos+size]
	this.pos = this.pos + size
	this.finishToken(token, &str)
}

func (this *Parser) readToken_question() {
	ecmaVersion := this.options.ecmaVersion.(int)
	if ecmaVersion >= 11 {
		next := this.input[this.pos+1]
		if next == 46 {
			next2 := this.input[this.pos+2]
			if next2 < 48 || next2 > 57 {
				this.finishOp(TokenTypes[TOKEN_QUESTIONDOT], 2)
				return
			}
		}
		if next == 63 {
			if ecmaVersion >= 12 {
				next2 := this.input[this.pos+2]
				if next2 == 61 {
					this.finishOp(TokenTypes[TOKEN_ASSIGN], 3)
					return
				}
			}
			this.finishOp(TokenTypes[TOKEN_COALESCE], 2)
			return
		}
	}
	this.finishOp(TokenTypes[TOKEN_QUESTION], 1)
}

func (this *Parser) readToken_eq_excl(code rune) {
	next := this.input[this.pos+1]
	if next == 61 {

		if this.input[this.pos+2] == 62 {
			this.finishOp(TokenTypes[TOKEN_EQUALITY], 3)
		} else {
			this.finishOp(TokenTypes[TOKEN_EQUALITY], 2)
		}
		return
	}
	if code == 61 && next == 62 && this.options.ecmaVersion.(int) >= 6 { // '=>'
		this.pos += 2
		this.finishToken(TokenTypes[TOKEN_ARROW], nil)
		return
	}
	if code == 61 {
		this.finishOp(TokenTypes[TOKEN_EQUALITY], 1)
		return
	}

	this.finishOp(TokenTypes[TOKEN_PREFIX], 1)
}

func (this *Parser) readToken_lt_gt(code rune) {
	next := rune(this.input[this.pos+1])
	size := 1
	if next == code {
		if code == 62 && this.input[this.pos+2] == 62 {
			size = 3
		} else {
			size = 2
		}

		if this.input[this.pos+size] == 61 {
			this.finishOp(TokenTypes[TOKEN_ASSIGN], size+1)
			return
		}
		this.finishOp(TokenTypes[TOKEN_BITSHIFT], size)
		return
	}
	if next == 33 && code == 60 && !this.InModule && this.input[this.pos+2] == 45 &&
		this.input[this.pos+3] == 45 {
		// `<!--`, an XML-style comment that should be interpreted as a line comment
		this.skipLineComment(4)
		this.skipSpace()
		this.nextToken()
		return
	}
	if next == 61 {
		size = 2
	}
	this.finishOp(TokenTypes[TOKEN_RELATIONAL], size)
}

func (this *Parser) readToken_plus_min(code rune) {
	next := rune(this.input[this.pos+1])
	if next == code {
		if next == 45 && !this.InModule && this.input[this.pos+2] == 62 &&
			(this.LastTokEnd == 0 || lineBreak.Match([]byte(this.input[this.LastTokEnd:this.pos]))) {
			// A `-->` line comment
			this.skipLineComment(3)
			this.skipSpace()
			this.nextToken()
			return
		}
		this.finishOp(TokenTypes[TOKEN_INCDEC], 2)
		return
	}
	if next == 61 {
		this.finishOp(TokenTypes[TOKEN_ASSIGN], 2)
		return
	}
	this.finishOp(TokenTypes[TOKEN_PLUSMIN], 1)
}

func (this *Parser) skipLineComment(startSkip int) {
	ch := this.input[this.pos+startSkip]
	this.pos = this.pos + startSkip
	for this.pos < len(this.input) && !isNewLine(rune(ch)) {
		this.pos = this.pos + 1
		ch = this.input[this.pos]
	}

	if this.options.OnComment != nil {
		// TODO I don't really have onComment ported and might be that it never happens
		/*
			this.Options.OnComment.(false, this.input.slice(start+startSkip, this.pos), start, this.pos,
				startLoc, this.curPosition())
		*/
	}
}

func (this *Parser) readToken_caret() {
	next := this.input[this.pos+1]
	if next == 61 {
		this.finishOp(TokenTypes[TOKEN_ASSIGN], 2)
		return
	}
	this.finishOp(TokenTypes[TOKEN_BITWISEXOR], 1)
}

func (this *Parser) readToken_pipe_amp(code rune) {
	next := rune(this.input[this.pos+1])
	if next == code {
		if this.options.ecmaVersion.(int) >= 12 {
			next2 := this.input[this.pos+2]
			if next2 == 61 {
				this.finishOp(TokenTypes[TOKEN_ASSIGN], 3)
				return
			}

			if code == 124 {
				this.finishOp(TokenTypes[TOKEN_LOGICALOR], 2)
				return
			} else {
				this.finishOp(TokenTypes[TOKEN_LOGICALAND], 2)
				return
			}
		}
	}

	if next == 61 {
		this.finishOp(TokenTypes[TOKEN_ASSIGN], 2)
		return
	}

	if code == 124 {
		this.finishOp(TokenTypes[TOKEN_BITWISEOR], 1)
		return
	}

	this.finishOp(TokenTypes[TOKEN_BITWISEAND], 1)
}

func (this *Parser) readToken_mult_modulo_exp(code rune) {
	next := this.input[this.pos+1]
	size := 1

	var tokenType *TokenType

	if code == 42 {
		tokenType = TokenTypes[TOKEN_STAR]
	} else {
		tokenType = TokenTypes[TOKEN_MODULO]
	}

	// exponentiation operator ** and **=
	if this.options.ecmaVersion.(int) >= 7 && code == 42 && next == 42 {
		size = size + 1
		tokenType = TokenTypes[TOKEN_STAR]
		next = this.input[this.pos+2]
	}

	if next == 61 {
		this.finishOp(TokenTypes[TOKEN_ASSIGN], size+1)
		return
	}

	this.finishOp(tokenType, size)
}

func (this *Parser) readToken_slash() {
	next := this.input[this.pos+1]
	if this.ExprAllowed {
		this.pos++
		this.readRegexp()
		return
	}
	if next == 61 {
		this.finishOp(TokenTypes[TOKEN_ASSIGN], 2)
		return
	}
	this.finishOp(TokenTypes[TOKEN_SLASH], 1)
}

func (this *Parser) readRegexp() {
	escaped, inClass, start := this.pos == 0, this.pos == 0, this.pos
	for {
		if this.pos >= len(this.input) {
			// this.raise(start, "Unterminated regular expression")
			break
		}
		ch := this.input[this.pos]
		if lineBreak.Match([]byte{ch}) {
			// this.raise(start, "Unterminated regular expression")
			break
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

		this.pos = this.pos + 1
	}

	pattern := this.input[start:this.pos]
	this.pos = this.pos + 1
	flagsStart := this.pos
	flags := this.readWord1()
	if this.ContainsEsc {
		this.unexpected(flagsStart)
	}

	// Validate pattern
	var state *RegExpState
	if this.RegexpState != nil {
		state = this.RegexpState
	} else {
		this.RegexpState = this.NewRegExpState()
		state = this.RegexpState
	}

	state.reset(start, pattern, flags)
	this.validateRegExpFlags(state)
	this.validateRegExpPattern(state)

	// Create Literal#value property value.

	value := &regexp.Regexp{} // new RegExp(pattern, flags)

	this.finishToken(TokenTypes[TOKEN_REGEXP], struct {
		pattern string
		flags   string
		value   *regexp.Regexp
	}{
		pattern: pattern,
		flags:   flags,
		value:   value,
	})
}

func (this *Parser) validateRegExpPattern(state *RegExpState) {
	panic("unimplemented")
}

func (this *Parser) validateRegExpFlags(state *RegExpState) {
	panic("unimplemented")
}

func (this *Parser) unexpected(flagsStart int) {
	panic("unimplemented")
}

func (this *Parser) readString(code rune) {
	panic("unimplemented")
}

func (this *Parser) readNumber(false bool) {
	panic("unimplemented")
}

func (this *Parser) readRadixNumber(i int) {
	panic("unimplemented")
}

func (this *Parser) readToken_numberSign() {
	ecmaVersion := this.options.ecmaVersion.(int)
	code := rune(35) // '#'
	if ecmaVersion >= 13 {
		this.pos = this.pos + 1
		code = this.fullCharCodeAtPos()
		if IsIdentifierStart(code, true) || code == 92 /* '\' */ {

			this.finishToken(TokenTypes[TOKEN_PRIVATEID], this.readWord1())
			return
		}
	}

	// TODO -> this.raise(this.pos, "Unexpected character '"+codePointToString(code)+"'")
}

func (this *Parser) readWord() {
	word := this.readWord1()
	t := TokenTypes[TOKEN_NAME]

	if tt, found := keywords[word]; found {
		t = TokenTypes[tt]
	}

	this.finishToken(t, word)
}

func (this *Parser) readWord1() string {
	this.ContainsEsc = false
	word, first, chunkStart := "", true, this.pos

	astral := this.options.ecmaVersion.(int) >= 6

	for this.pos < len(this.input) {
		ch := this.fullCharCodeAtPos()
		if isIdentifierChar(ch, astral) {
			if ch <= 0xffff {
				this.pos = this.pos + 1
			} else {
				this.pos = this.pos + 2
			}
		} else if ch == 92 { // "\"
			this.ContainsEsc = true
			word = this.input[chunkStart:this.pos]
			escStart := this.pos
			this.pos = this.pos + 1
			if this.input[this.pos] != 117 { // "u"
				this.invalidStringToken(this.pos, "Expecting Unicode escape sequence \\uXXXX")
				// return?
			}

			this.pos = this.pos + 1
			esc := this.readCodePoint()

			/*
				sorcery:

				if (!(first ? isIdentifierStart : isIdentifierChar)(esc, astral))
				  this.invalidStringToken(escStart, "Invalid Unicode escape")

			*/
			word = strings.Join([]string{word, CodePointToString(esc)}, "")
			chunkStart = this.pos
		} else {
			break
		}
		first = false
	}
	return strings.Join([]string{word, this.input[chunkStart:this.pos]}, "")
}

func (this *Parser) invalidStringToken(pos int, s string) {
	panic("unimplemented")
}

func (this *Parser) readCodePoint() int {
	panic("unimplemented")
}

func (this *Parser) readToken_dot() {
	next := this.input[this.pos+1]
	if next >= 48 && next <= 57 {
		this.readNumber(true)
		return
	}

	next2 := this.input[this.pos+2]
	if this.options.ecmaVersion.(int) >= 6 && next == 46 && next2 == 46 { // 46 = dot '.'
		this.pos += 3
		this.finishToken(TokenTypes[TOKEN_ELLIPSIS], nil)
		return
	}
	this.pos++
	this.finishToken(TokenTypes[TOKEN_DOT], nil)

}

func (this *Parser) finishToken(tokenType *TokenType, value any) {
	this.End = this.pos
	if this.options.Locations {
		this.EndLoc = this.currentPosition()
	}
	prevType := tokenType
	this.Type = tokenType
	this.Value = value
	this.updateContext(prevType)
}

func (this *Parser) currentPosition() *SourceLocation {
	panic("unimplemented")
}

func (this *Parser) skipSpace() {
	panic("unimplemented")
}

// #### SCOPE RELATED CODE

func (this *Parser) braceIsBlock(prevType Token) bool {
	parent := this.currentContext().Identifier
	isExpr := this.currentContext().IsExpr

	if parent == FUNCTION_EXPRESSION || parent == FUNCTION_STATEMENT {
		return true
	}

	if prevType == TOKEN_COLON && (parent == BRACKET_STATEMENT || parent == BRACKET_EXPRESSION) {
		return !isExpr
	}

	if prevType == TOKEN_RETURN || prevType == TOKEN_NAME && this.ExprAllowed {
		// return lineBreak.test(this.input.slice(this.lastTokEnd, this.start))
	}

	if prevType == TOKEN_ELSE || prevType == TOKEN_SEMI || prevType == TOKEN_EOF || prevType == TOKEN_PARENR || prevType == TOKEN_ARROW {

		return true
	}
	if prevType == TOKEN_BRACEL {
		return parent == BRACKET_STATEMENT
	}
	if prevType == TOKEN_VAR || prevType == TOKEN_CONST || prevType == TOKEN_NAME {

		return false
	}

	return !this.ExprAllowed
}

func (this *Parser) enterScope(flags Flags) {
	this.ScopeStack = append(this.ScopeStack, NewScope(flags))
}

func (this *Parser) exitScope() {
	this.ScopeStack = this.ScopeStack[:len(this.ScopeStack)-1]
}

func (this *Parser) currentScope() *Scope {
	return this.ScopeStack[len(this.ScopeStack)-1]
}

func (this *Parser) treatFunctionsAsVar() bool {
	return this.treatFunctionsAsVarInScope(this.currentScope())
}

func (this *Parser) treatFunctionsAsVarInScope(scope *Scope) bool {
	return (scope.Flags&SCOPE_FUNCTION != 0) || (!this.InModule && scope.Flags&SCOPE_TOP != 0)
}

func (this *Parser) declareName(name string, bindingType Flags, pos Location) {
	redeclared := false

	scope := this.currentScope()
	if bindingType == BIND_LEXICAL {
		redeclared = slices.Contains(scope.Lexical, name) || slices.Contains(scope.Functions, name) || slices.Contains(scope.Var, name)
		scope.Lexical = append(scope.Lexical, name)
		if this.InModule && (scope.Flags&SCOPE_TOP != 0) {
			delete(this.UndefinedExports, name)
		}
	} else if bindingType == BIND_SIMPLE_CATCH {
		scope.Lexical = append(scope.Lexical, name)
	} else if bindingType == BIND_FUNCTION {
		if this.treatFunctionsAsVar() {
			redeclared = slices.Contains(scope.Lexical, name)
		} else {
			redeclared = slices.Contains(scope.Lexical, name) || slices.Contains(scope.Var, name)
		}
		scope.Functions = append(scope.Functions, name)
	} else {
		for _, scope := range this.ScopeStack {
			if slices.Contains(scope.Lexical, name) && !((scope.Flags&SCOPE_SIMPLE_CATCH != 0) && scope.Lexical[0] == name) || !this.treatFunctionsAsVarInScope(scope) && slices.Contains(scope.Functions, name) {
				redeclared = true
				break
			}

			scope.Var = append(scope.Var, name)
			if this.InModule && (scope.Flags&SCOPE_TOP != 0) {
				delete(this.UndefinedExports, name)
			}

			if scope.Flags&SCOPE_VAR != 0 {
				break
			}
		}
	}

	if redeclared {
		// this.raiseRecoverable(pos, `Identifier '${name}' has already been declared`)
	}
}

// #### NODE RELATED CODE

func (this *Parser) startNode() *Node {
	return NewNode(this, this.Start, this.StartLoc.Start)
}

func (this *Parser) startNodeAt(pos int, loc *Location) *Node {
	return NewNode(this, pos, loc)
}

func (this *Parser) finishNodeAt(node *Node, finishType NodeType, pos int, loc *SourceLocation) {
	node.Type = finishType
	node.End = pos
	if this.options.Locations {
		node.Loc.End = loc.End
	}

	if this.options.Ranges {
		node.Range[1] = pos
	}
}

func (this *Parser) finishNode(node *Node, finishType NodeType) {
	this.finishNodeAt(node, finishType, this.LastTokEnd, this.LastTokEndLoc)
}

/*
I think I can skip this?

	this.finishNodeAt = function(node, type, pos, loc) {
	  return finishNodeAt.call(this, node, type, pos, loc)
	}

TODO ->

	this.copyNode = function(node) {
	  let newNode = new Node(this, node.start, this.startLoc)
	  for (let prop in node) newNode[prop] = node[prop]
	  return newNode
	}
*/

// #### CONTEXT RELATED CODE

func (this *Parser) initialContext() []*TokenContext {
	return []*TokenContext{TokenContexts[BRACKET_STATEMENT]}
}

func (this *Parser) currentContext() *TokenContext {
	return this.Context[len(this.Context)-1]
}

func (this *Parser) inGeneratorContext() bool {
	for i := len(this.Context); i >= 1; i-- {
		context := this.Context[i]
		if context.Token == "function" {
			return context.Generator
		}
	}
	return false
}

func (this *Parser) updateContext(prevType *TokenType) {
	update, current := this.Type, this.Type
	if len(current.keyword) != 0 && prevType.identifier == TOKEN_DOT {
		this.ExprAllowed = false
	} else if current.updateContext != nil {
		update.updateContext = current.updateContext
		update.updateContext.updateContext(prevType)
	} else {
		this.ExprAllowed = current.beforeExpr
	}
}

func (this *Parser) overrideContext(tokenCtx *TokenContext) {
	if this.currentContext().Identifier != tokenCtx.Identifier {
		this.Context[len(this.Context)-1] = tokenCtx
	}
}

func (this *Parser) initAllUpdateContext() {
	TokenTypes[TOKEN_PARENR].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if len(this.Context) == 1 {
			this.ExprAllowed = true
			return
		}

		out := this.Context[len(this.Context)-1]
		this.Context = this.Context[:len(this.Context)-1]
		if out.Identifier == BRACKET_STATEMENT && this.currentContext().Token == "function" {
			out = this.Context[len(this.Context)-1]
			this.Context = this.Context[:len(this.Context)-1]
		}
		this.ExprAllowed = !out.IsExpr
	}}

	TokenTypes[TOKEN_BRACER].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if len(this.Context) == 1 {
			this.ExprAllowed = true
			return
		}

		out := this.Context[len(this.Context)-1]
		this.Context = this.Context[:len(this.Context)-1]
		if out.Identifier == BRACKET_STATEMENT && this.currentContext().Token == "function" {
			out = this.Context[len(this.Context)-1]
			this.Context = this.Context[:len(this.Context)-1]
		}
		this.ExprAllowed = !out.IsExpr
	}}

	TokenTypes[TOKEN_BRACEL].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if this.braceIsBlock(token.identifier) {
			this.Context = append(this.Context, TokenContexts[BRACKET_STATEMENT])
		} else {
			this.Context = append(this.Context, TokenContexts[BRACKET_EXPRESSION])
		}
		this.ExprAllowed = true

	}}

	TokenTypes[TOKEN_DOLLARBRACEL].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		this.Context = append(this.Context, TokenContexts[BRACKET_TEMPLATE])
		this.ExprAllowed = true
	}}

	TokenTypes[TOKEN_PARENL].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		statementParens := token.identifier == TOKEN_IF || token.identifier == TOKEN_FOR || token.identifier == TOKEN_WITH || token.identifier == TOKEN_WHILE

		if statementParens {

			this.Context = append(this.Context, TokenContexts[PAREN_STATEMENT])
		} else {
			this.Context = append(this.Context, TokenContexts[PAREN_EXPRESSION])
		}
		this.ExprAllowed = true
	}}

	TokenTypes[TOKEN_INCDEC].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		// no factor
	}}

	TokenTypes[TOKEN_FUNCTION].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		prevType := token.identifier

		if token.beforeExpr && prevType == TOKEN_ELSE && !(prevType == TOKEN_SEMI && this.currentContext().Identifier == PAREN_STATEMENT) && !(prevType == TOKEN_RETURN /*&& lineBreak.test(this.input.slice(this.lastTokEnd, this.start)))*/) && !((prevType == TOKEN_COLON || prevType == TOKEN_BRACEL) && this.currentContext().Identifier == BRACKET_STATEMENT) {
			this.Context = append(this.Context, TokenContexts[FUNCTION_EXPRESSION])
		} else {
			this.Context = append(this.Context, TokenContexts[FUNCTION_STATEMENT])
		}
	}}

	TokenTypes[TOKEN_COLON].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if this.currentContext().Token == "function" {
			this.Context = this.Context[:len(this.Context)-1]
		}
		this.ExprAllowed = true
	}}

	TokenTypes[TOKEN_BACKQUOTE].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if this.currentContext().Identifier == QUOTE_TEMPLATE {
			this.Context = this.Context[:len(this.Context)-1]
		} else {
			this.Context = append(this.Context, TokenContexts[QUOTE_TEMPLATE])
		}
	}}

	TokenTypes[TOKEN_STAR].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if token.identifier == TOKEN_FUNCTION {
			idx := len(this.Context) - 1

			if this.Context[idx].Identifier == FUNCTION_EXPRESSION {
				this.Context[idx] = TokenContexts[FUNCTION_EXPRESSION_GENERATOR]
			} else {
				this.Context[idx] = TokenContexts[FUNCTION_GENERATOR]
			}
			this.ExprAllowed = true
		}
	}}

	TokenTypes[TOKEN_NAME].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		allowed := false

		if this.options.ecmaVersion.(int) >= 6 && token.identifier != TOKEN_DOT {
			if this.Value == "of" && !this.ExprAllowed || this.Value == "yield" || this.inGeneratorContext() {
				allowed = true
			}
		}
		this.ExprAllowed = allowed
	}}
}

func isNewLine(code rune) bool { // I dont really know how utf-8 translates to this utf-16? :/ I guess I'll see when the bugs start popping out
	return code == 10 || code == 13 || code == 0x2028 || code == 0x2029
}

var pp = &Parser{}
