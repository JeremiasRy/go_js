package parser

import "unicode/utf8"

type Label struct {
	Kind string
}

type PrivateName struct {
	Declared bool
	Used     bool
}

type Parser struct {
	Options                  Options
	SourceFile               *string
	Keywords                 any
	ReservedWords            any
	ReservedWordsStrict      any
	ReservedWordsStrictBind  any
	Input                    string
	ContainsEsc              bool
	Pos                      int
	LineStart                int
	CurLine                  int
	Type                     TokenType
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

// Move to next token
func (pp *Parser) next(ignoreEscapeSequenceInKeyword bool) {
	if !ignoreEscapeSequenceInKeyword && len(pp.Type.keyword) != 0 && pp.ContainsEsc {
		// this.raiseRecoverable(this.start, "Escape sequence in keyword " + this.type.keyword)
	}

	if pp.Options.OnToken != nil {
		// TODO? Maybe? I dont need this?
	}

	pp.LastTokEnd = pp.End
	pp.LastTokStart = pp.Start
	pp.LastTokEndLoc = pp.StartLoc
	pp.LastTokStartLoc = pp.StartLoc
	pp.nextToken()
}

func (pp *Parser) nextToken() {
	context := pp.currentContext()
	if context == nil || context.PreserveSpace {
		pp.skipSpace()
	}

	pp.Start = pp.Pos
	if pp.Options.Locations {
		pp.StartLoc = pp.currentPosition()
	}

	if pp.Pos >= len(pp.Input) {
		pp.finishToken()
		return
	}

	if context.Override != nil {
		context.Override(pp)
		return
	} else {
		pp.readToken(pp.fullCharCodeAtPos())
	}
}

func (pp *Parser) fullCharCodeAtPos() rune {
	if pp.Pos < 0 || pp.Pos >= len(pp.Input) {
		return 0 // error handling...
	}

	r, size := utf8.DecodeRuneInString(pp.Input[pp.Pos:])
	if r == utf8.RuneError {
		return 0 //  error handling...
	}

	code := int32(r)

	if code <= 0xD7FF || code >= 0xDC00 {
		return code
	}

	if pp.Pos+size >= len(pp.Input) {
		return code
	}

	nextRune, _ := utf8.DecodeRuneInString(pp.Input[pp.Pos+size:])
	next := int32(nextRune)

	if next <= 0xDBFF || next >= 0xE000 {
		return code
	}

	return (code<<10 + next - 0x35FDC00)
}

func (pp *Parser) getTokenFromCode(code rune) {

}

/*
pp.getTokenFromCode = function (code) {
  switch (code) {
    // The interpretation of a dot depends on whether it is followed
    // by a digit or another two dots.
    case 46: // '.'
      return this.readToken_dot()

    // Punctuation tokens.
    case 40: ++this.pos; return this.finishToken(tt.parenL)
    case 41: ++this.pos; return this.finishToken(tt.parenR)
    case 59: ++this.pos; return this.finishToken(tt.semi)
    case 44: ++this.pos; return this.finishToken(tt.comma)
    case 91: ++this.pos; return this.finishToken(tt.bracketL)
    case 93: ++this.pos; return this.finishToken(tt.bracketR)
    case 123: ++this.pos; return this.finishToken(tt.braceL)
    case 125: ++this.pos; return this.finishToken(tt.braceR)
    case 58: ++this.pos; return this.finishToken(tt.colon)

    case 96: // '`'
      if (this.options.ecmaVersion < 6) break
      ++this.pos
      return this.finishToken(tt.backQuote)

    case 48: // '0'
      let next = this.input.charCodeAt(this.pos + 1)
      if (next === 120 || next === 88) return this.readRadixNumber(16) // '0x', '0X' - hex number
      if (this.options.ecmaVersion >= 6) {
        if (next === 111 || next === 79) return this.readRadixNumber(8) // '0o', '0O' - octal number
        if (next === 98 || next === 66) return this.readRadixNumber(2) // '0b', '0B' - binary number
      }

    // Anything else beginning with a digit is an integer, octal
    // number, or float.
    case 49: case 50: case 51: case 52: case 53: case 54: case 55: case 56: case 57: // 1-9
      return this.readNumber(false)

    // Quotes produce strings.
    case 34: case 39: // '"', "'"
      return this.readString(code)

    // Operators are parsed inline in tiny state machines. '=' (61) is
    // often referred to. `finishOp` simply skips the amount of
    // characters it is given as second argument, and returns a token
    // of the type given by its first argument.
    case 47: // '/'
      return this.readToken_slash()

    case 37: case 42: // '%*'
      return this.readToken_mult_modulo_exp(code)

    case 124: case 38: // '|&'
      return this.readToken_pipe_amp(code)

    case 94: // '^'
      return this.readToken_caret()

    case 43: case 45: // '+-'
      return this.readToken_plus_min(code)

    case 60: case 62: // '<>'
      return this.readToken_lt_gt(code)

    case 61: case 33: // '=!'
      return this.readToken_eq_excl(code)

    case 63: // '?'
      return this.readToken_question()

    case 126: // '~'
      return this.finishOp(tt.prefix, 1)

    case 35: // '#'
      return this.readToken_numberSign()
  }

  this.raise(this.pos, "Unexpected character '" + codePointToString(code) + "'")
}
*/

func (pp *Parser) readToken(param any) {
	panic("unimplemented")
}

func (pp *Parser) finishToken() {
	panic("unimplemented")
}

func (pp *Parser) currentPosition() *SourceLocation {
	panic("unimplemented")
}

func (pp *Parser) skipSpace() {
	panic("unimplemented")
}

func (pp *Parser) initialContext() []*TokenContext {
	return []*TokenContext{TokenContexts[BRACKET_STATEMENT]}
}

func (pp *Parser) currentContext() *TokenContext {
	return pp.Context[len(pp.Context)-1]
}

func (pp *Parser) braceIsBlock(prevType Token) bool {
	parent := pp.currentContext().Identifier
	isExpr := pp.currentContext().IsExpr

	if parent == FUNCTION_EXPRESSION || parent == FUNCTION_STATEMENT {
		return true
	}

	if prevType == TOKEN_COLON && (parent == BRACKET_STATEMENT || parent == BRACKET_EXPRESSION) {
		return !isExpr
	}

	if prevType == TOKEN_RETURN || prevType == TOKEN_NAME && pp.ExprAllowed {
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

	return !pp.ExprAllowed
}

var pp = &Parser{}
