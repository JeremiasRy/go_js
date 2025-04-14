package parser

import (
	"slices"
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

// TOKEN RELATED CODE

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
		pp.finishToken(TokenTypes[TOKEN_EOF])
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

func (pp *Parser) readToken(code rune) {
	if IsIdentifierStart(code, pp.Options.EcmaVersion.(int) >= 6) || code == 92 {
		pp.readWord()
		return
	}

	pp.getTokenFromCode(code)
}

func (pp *Parser) readWord() {
	panic("unimplemented")
}

func (pp *Parser) getTokenFromCode(code rune) {
	switch code {
	case 46: // '.'
		pp.readToken_dot()

	case 40: // '('
		pp.Pos++
		pp.finishToken(TokenTypes[TOKEN_PARENL])

	case 41: // ')'
		pp.Pos++
		pp.finishToken(TokenTypes[TOKEN_PARENR])

	case 59: // ';'
		pp.Pos++
		pp.finishToken(TokenTypes[TOKEN_SEMI])

	case 44: // ','
		pp.Pos++
		pp.finishToken(TokenTypes[TOKEN_COMMA])

	case 91: // '['
		pp.Pos++
		pp.finishToken(TokenTypes[TOKEN_BRACKETL])

	case 93: // ']'
		pp.Pos++
		pp.finishToken(TokenTypes[TOKEN_BRACKETR])

	case 123: // '{'
		pp.Pos++
		pp.finishToken(TokenTypes[TOKEN_BRACEL])

	case 125: // '}'
		pp.Pos++
		pp.finishToken(TokenTypes[TOKEN_BRACER])

	case 58: // ':'
		pp.Pos++
		pp.finishToken(TokenTypes[TOKEN_COLON])

	case 96: // '`'
		if pp.Options.EcmaVersion.(int) < 6 {
			break
		}
		pp.Pos++
		pp.finishToken(TokenTypes[TOKEN_BACKQUOTE])

	case 48: // '0'
		next := pp.Input[pp.Pos+1]
		if next == 120 || next == 88 { // 'x', 'X'
			pp.readRadixNumber(16) // hex number
			return
		}
		if pp.Options.EcmaVersion.(int) >= 6 {
			if next == 111 || next == 79 { // 'o', 'O'
				pp.readRadixNumber(8) // octal number
				return
			}
			if next == 98 || next == 66 { // 'b', 'B'
				pp.readRadixNumber(2) // binary number
				return
			}
		}
		pp.readNumber(false)

	case 49, 50, 51, 52, 53, 54, 55, 56, 57: // '1'-'9'
		pp.readNumber(false)

	case 34, 39: // '"', "'"
		pp.readString(code)

	case 47: // '/'
		pp.readToken_slash()

	case 37, 42: // '%', '*'
		pp.readToken_mult_modulo_exp(code)

	case 124, 38: // '|', '&'
		pp.readToken_pipe_amp(code)

	case 94: // '^'
		pp.readToken_caret()

	case 43, 45: // '+', '-'
		pp.readToken_plus_min(code)

	case 60, 62: // '<', '>'
		pp.readToken_lt_gt(code)

	case 61, 33: // '=', '!'
		pp.readToken_eq_excl(code)

	case 63: // '?'
		pp.readToken_question()

	case 126: // '~'
		pp.finishOp(TokenTypes[TOKEN_PREFIX], 1)

	case 35: // '#'
		pp.readToken_numberSign()
	}

	// pp.raise(pp.Pos, "Unexpected character '"+codePointToString(code)+"'")
}

func (pp *Parser) finishOp(token *TokenType, size int) {

}

func (pp *Parser) readToken_question() {
	panic("unimplemented")
}

func (pp *Parser) readToken_eq_excl(code rune) {
	panic("unimplemented")
}

func (pp *Parser) readToken_lt_gt(code rune) {
	panic("unimplemented")
}

func (pp *Parser) readToken_plus_min(code rune) {
	panic("unimplemented")
}

func (pp *Parser) readToken_caret() {
	panic("unimplemented")
}

func (pp *Parser) readToken_pipe_amp(code rune) {
	panic("unimplemented")
}

func (pp *Parser) readToken_mult_modulo_exp(code rune) {
	panic("unimplemented")
}

func (pp *Parser) readToken_slash() {
	panic("unimplemented")
}

func (pp *Parser) readString(code rune) {
	panic("unimplemented")
}

func (pp *Parser) readNumber(false bool) {
	panic("unimplemented")
}

func (pp *Parser) readRadixNumber(i int) {
	panic("unimplemented")
}

func (pp *Parser) readToken_numberSign() {
	panic("unimplemented")
}

func (pp *Parser) readToken_dot() {
	panic("unimplemented")
}

func (pp *Parser) finishToken(tokenType *TokenType) {
	panic("unimplemented")
}

func (pp *Parser) currentPosition() *SourceLocation {
	panic("unimplemented")
}

func (pp *Parser) skipSpace() {
	panic("unimplemented")
}

// #### SCOPE RELATED CODE

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

func (pp *Parser) enterScope(flags Flags) {
	pp.ScopeStack = append(pp.ScopeStack, NewScope(flags))
}

func (pp *Parser) exitScope() {
	pp.ScopeStack = pp.ScopeStack[:len(pp.ScopeStack)-1]
}

func (pp *Parser) currentScope() *Scope {
	return pp.ScopeStack[len(pp.ScopeStack)-1]
}

func (pp *Parser) treatFunctionsAsVar() bool {
	return pp.treatFunctionsAsVarInScope(pp.currentScope())
}

func (pp *Parser) treatFunctionsAsVarInScope(scope *Scope) bool {
	return (scope.Flags&SCOPE_FUNCTION != 0) || (!pp.InModule && scope.Flags&SCOPE_TOP != 0)
}

func (pp *Parser) declareName(name string, bindingType Flags, pos Location) {
	redeclared := false

	scope := pp.currentScope()
	if bindingType == BIND_LEXICAL {
		redeclared = slices.Contains(scope.Lexical, name) || slices.Contains(scope.Functions, name) || slices.Contains(scope.Var, name)
		scope.Lexical = append(scope.Lexical, name)
		if pp.InModule && (scope.Flags&SCOPE_TOP != 0) {
			delete(pp.UndefinedExports, name)
		}
	} else if bindingType == BIND_SIMPLE_CATCH {
		scope.Lexical = append(scope.Lexical, name)
	} else if bindingType == BIND_FUNCTION {
		if pp.treatFunctionsAsVar() {
			redeclared = slices.Contains(scope.Lexical, name)
		} else {
			redeclared = slices.Contains(scope.Lexical, name) || slices.Contains(scope.Var, name)
		}
		scope.Functions = append(scope.Functions, name)
	} else {
		for _, scope := range pp.ScopeStack {
			if slices.Contains(scope.Lexical, name) && !((scope.Flags&SCOPE_SIMPLE_CATCH != 0) && scope.Lexical[0] == name) || !pp.treatFunctionsAsVarInScope(scope) && slices.Contains(scope.Functions, name) {
				redeclared = true
				break
			}

			scope.Var = append(scope.Var, name)
			if pp.InModule && (scope.Flags&SCOPE_TOP != 0) {
				delete(pp.UndefinedExports, name)
			}

			if scope.Flags&SCOPE_VAR != 0 {
				break
			}
		}
	}

	if redeclared {
		// pp.raiseRecoverable(pos, `Identifier '${name}' has already been declared`)
	}
}

// #### NODE RELATED CODE

func (pp *Parser) startNode() *Node {
	return NewNode(pp, pp.Start, pp.StartLoc.Start)
}

func (pp *Parser) startNodeAt(pos int, loc *Location) *Node {
	return NewNode(pp, pos, loc)
}

func (pp *Parser) finishNodeAt(node *Node, finishType NodeType, pos int, loc *SourceLocation) {
	node.Type = finishType
	node.End = pos
	if pp.Options.Locations {
		node.Loc.End = loc.End
	}

	if pp.Options.Ranges {
		node.Range[1] = pos
	}
}

func (pp *Parser) finishNode(node *Node, finishType NodeType) {
	pp.finishNodeAt(node, finishType, pp.LastTokEnd, pp.LastTokEndLoc)
}

/*
I think I can skip this?

	pp.finishNodeAt = function(node, type, pos, loc) {
	  return finishNodeAt.call(this, node, type, pos, loc)
	}

TODO ->

	pp.copyNode = function(node) {
	  let newNode = new Node(this, node.start, this.startLoc)
	  for (let prop in node) newNode[prop] = node[prop]
	  return newNode
	}
*/

// #### CONTEXT RELATED CODE

func (pp *Parser) initialContext() []*TokenContext {
	return []*TokenContext{TokenContexts[BRACKET_STATEMENT]}
}

func (pp *Parser) currentContext() *TokenContext {
	return pp.Context[len(pp.Context)-1]
}

func (pp *Parser) inGeneratorContext() bool {
	for i := len(pp.Context); i >= 1; i-- {
		context := pp.Context[i]
		if context.Token == "function" {
			return context.Generator
		}
	}
	return false
}

func (pp *Parser) updateContext(prevType *TokenType) {
	update, current := pp.Type, pp.Type
	if len(current.keyword) != 0 && prevType.identifier == TOKEN_DOT {
		pp.ExprAllowed = false
	} else if current.updateContext != nil {
		update.updateContext = current.updateContext
		update.updateContext.updateContext(prevType)
	} else {
		pp.ExprAllowed = current.beforeExpr
	}
}

func (pp *Parser) overrideContext(tokenCtx *TokenContext) {
	if pp.currentContext().Identifier != tokenCtx.Identifier {
		pp.Context[len(pp.Context)-1] = tokenCtx
	}
}

func (pp *Parser) initAllUpdateContext() {
	TokenTypes[TOKEN_PARENR].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if len(pp.Context) == 1 {
			pp.ExprAllowed = true
			return
		}

		out := pp.Context[len(pp.Context)-1]
		pp.Context = pp.Context[:len(pp.Context)-1]
		if out.Identifier == BRACKET_STATEMENT && pp.currentContext().Token == "function" {
			out = pp.Context[len(pp.Context)-1]
			pp.Context = pp.Context[:len(pp.Context)-1]
		}
		pp.ExprAllowed = !out.IsExpr
	}}

	TokenTypes[TOKEN_BRACER].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if len(pp.Context) == 1 {
			pp.ExprAllowed = true
			return
		}

		out := pp.Context[len(pp.Context)-1]
		pp.Context = pp.Context[:len(pp.Context)-1]
		if out.Identifier == BRACKET_STATEMENT && pp.currentContext().Token == "function" {
			out = pp.Context[len(pp.Context)-1]
			pp.Context = pp.Context[:len(pp.Context)-1]
		}
		pp.ExprAllowed = !out.IsExpr
	}}

	TokenTypes[TOKEN_BRACEL].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if pp.braceIsBlock(token.identifier) {
			pp.Context = append(pp.Context, TokenContexts[BRACKET_STATEMENT])
		} else {
			pp.Context = append(pp.Context, TokenContexts[BRACKET_EXPRESSION])
		}
		pp.ExprAllowed = true

	}}

	TokenTypes[TOKEN_DOLLARBRACEL].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		pp.Context = append(pp.Context, TokenContexts[BRACKET_TEMPLATE])
		pp.ExprAllowed = true
	}}

	TokenTypes[TOKEN_PARENL].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		statementParens := token.identifier == TOKEN_IF || token.identifier == TOKEN_FOR || token.identifier == TOKEN_WITH || token.identifier == TOKEN_WHILE

		if statementParens {

			pp.Context = append(pp.Context, TokenContexts[PAREN_STATEMENT])
		} else {
			pp.Context = append(pp.Context, TokenContexts[PAREN_EXPRESSION])
		}
		pp.ExprAllowed = true
	}}

	TokenTypes[TOKEN_INCDEC].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		// no factor
	}}

	TokenTypes[TOKEN_FUNCTION].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		prevType := token.identifier

		if token.beforeExpr && prevType == TOKEN_ELSE && !(prevType == TOKEN_SEMI && pp.currentContext().Identifier == PAREN_STATEMENT) && !(prevType == TOKEN_RETURN /*&& lineBreak.test(this.input.slice(this.lastTokEnd, this.start)))*/) && !((prevType == TOKEN_COLON || prevType == TOKEN_BRACEL) && pp.currentContext().Identifier == BRACKET_STATEMENT) {
			pp.Context = append(pp.Context, TokenContexts[FUNCTION_EXPRESSION])
		} else {
			pp.Context = append(pp.Context, TokenContexts[FUNCTION_STATEMENT])
		}
	}}

	TokenTypes[TOKEN_COLON].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if pp.currentContext().Token == "function" {
			pp.Context = pp.Context[:len(pp.Context)-1]
		}
		pp.ExprAllowed = true
	}}

	TokenTypes[TOKEN_BACKQUOTE].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if pp.currentContext().Identifier == QUOTE_TEMPLATE {
			pp.Context = pp.Context[:len(pp.Context)-1]
		} else {
			pp.Context = append(pp.Context, TokenContexts[QUOTE_TEMPLATE])
		}
	}}

	TokenTypes[TOKEN_STAR].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if token.identifier == TOKEN_FUNCTION {
			idx := len(pp.Context) - 1

			if pp.Context[idx].Identifier == FUNCTION_EXPRESSION {
				pp.Context[idx] = TokenContexts[FUNCTION_EXPRESSION_GENERATOR]
			} else {
				pp.Context[idx] = TokenContexts[FUNCTION_GENERATOR]
			}
			pp.ExprAllowed = true
		}
	}}

	TokenTypes[TOKEN_NAME].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		allowed := false

		if pp.Options.EcmaVersion.(int) >= 6 && token.identifier != TOKEN_DOT {
			if pp.Value == "of" && !pp.ExprAllowed || pp.Value == "yield" || pp.inGeneratorContext() {
				allowed = true
			}
		}
		pp.ExprAllowed = allowed
	}}
}

var pp = &Parser{}
