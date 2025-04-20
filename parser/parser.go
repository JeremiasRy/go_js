package parser

import (
	"bytes"
	"errors"
	"regexp"
	"slices"
	"strconv"
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
	Keywords                 *regexp.Regexp
	ReservedWords            *regexp.Regexp
	ReservedWordsStrict      *regexp.Regexp
	ReservedWordsStrictBind  *regexp.Regexp
	input                    []byte
	ContainsEsc              bool
	pos                      int
	LineStart                int
	CurLine                  int
	Type                     *TokenType
	Value                    any
	start                    int
	End                      int
	startLoc                 *Location
	EndLoc                   *Location
	LastTokStart             int
	LastTokEnd               int
	LastTokStartLoc          *Location
	LastTokEndLoc            *Location
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
	InTemplateElement        bool
	CanAwait                 bool
	AllowSuper               bool
	AllowDirectSuper         bool
	InAsync                  bool
	InGenerator              bool
	InClassStaticBlock       bool
}

func (p *Parser) getEcmaVersion() int {
	if ecmaVersion, ok := p.options.ecmaVersion.(int); ok {
		return ecmaVersion
	}
	panic("Ecma verion was set to something weird")
}

func (this *Parser) raise(pos int, message string) error {
	loc := getLineInfo(this.input, pos)
	line := strconv.Itoa(loc.Line)
	column := strconv.Itoa(loc.Column)
	message += strings.Join([]string{" (", line, ":", column, ")"}, "")

	if this.SourceFile != nil {
		message += strings.Join([]string{" in ", *this.SourceFile}, "")
	}

	return errors.New(message)
}

func (this *Parser) raiseRecoverable(pos int, message string) error {
	return this.raise(pos, message)
}

// #### WHITESPACE

func (this *Parser) skipSpace() error {
Loop:
	for this.pos < len(this.input) {
		ch, size, _ := this.fullCharCodeAtPos()
		switch ch {
		case 32:
		case 160: // ' '
			this.pos = this.pos + size
		case 13:
			if this.input[this.pos+size] == 10 {
				this.pos = this.pos + size
			}
		case 10:
		case 8232:
		case 8233:
			this.pos = this.pos + size
			if this.options.Locations {
				this.CurLine = this.CurLine + 1
				this.LineStart = this.pos
			}
		case 47: // '/'
			switch this.input[this.pos+1] {
			case 42: // '*'
				return this.skipBlockComment()
			case 47:
				this.skipLineComment(2)
			default:
				break Loop
			}
		default:
			if ch > 8 && ch < 14 || ch >= 5760 && nonASCIIwhitespace.Match(utf8.AppendRune([]byte{}, ch)) {
				this.pos = this.pos + size
			} else {
				break Loop
			}
		}
	}
	return nil
}

func (this *Parser) skipBlockComment() error {
	start := this.pos
	this.pos += 2 // Skip "/*"
	end := bytes.Index(this.input[this.pos:], []byte("*/"))
	if end == -1 {
		return this.raise(start, "Unterminated comment")
	}
	this.pos += end + 2 // Move past "*/"
	return nil
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

func (this *Parser) declareName(name string, bindingType Flags, pos int) error {
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
		return this.raiseRecoverable(pos, `Identifier '${name}' has already been declared`)
	}
	return nil
}

func (p *Parser) parseNew() (*Node, error) {
	panic("unimplemented")
}

func (p *Parser) parseClass(node *Node, isStatement bool) (*Node, error) {
	panic("unimplemented")
}

func (p *Parser) parseObj(isPattern bool, refDestructuringErrors *DestructuringErrors) (*Node, error) {
	panic("unimplemented")
}

func (p *Parser) isSimpleAssignTarget(expr any) bool {
	panic("unimplemented")
}

func (p *Parser) parseParenAndDistinguishExpression(canBeArrow bool, forInit string) (*Node, error) {
	panic("unimplemented")
}

func (p *Parser) parseArrowExpression(node *Node, params []*Node, isAsync bool, forInit string) (*Node, error) {
	panic("unimplemented")
}

func (p *Parser) parseFunction(node *Node, statement Flags, allowExpressionBody bool, isAsync bool, forInit string) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parsePrivateIdent() (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) toAssignable(param any, false bool, refDestructuringErrors *DestructuringErrors) *Node {
	panic("unimplemented")
}

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
	tokenTypes[TOKEN_PARENR].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
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

	tokenTypes[TOKEN_BRACER].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
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

	tokenTypes[TOKEN_BRACEL].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if this.braceIsBlock(token.identifier) {
			this.Context = append(this.Context, TokenContexts[BRACKET_STATEMENT])
		} else {
			this.Context = append(this.Context, TokenContexts[BRACKET_EXPRESSION])
		}
		this.ExprAllowed = true

	}}

	tokenTypes[TOKEN_DOLLARBRACEL].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		this.Context = append(this.Context, TokenContexts[BRACKET_TEMPLATE])
		this.ExprAllowed = true
	}}

	tokenTypes[TOKEN_PARENL].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		statementParens := token.identifier == TOKEN_IF || token.identifier == TOKEN_FOR || token.identifier == TOKEN_WITH || token.identifier == TOKEN_WHILE

		if statementParens {

			this.Context = append(this.Context, TokenContexts[PAREN_STATEMENT])
		} else {
			this.Context = append(this.Context, TokenContexts[PAREN_EXPRESSION])
		}
		this.ExprAllowed = true
	}}

	tokenTypes[TOKEN_INCDEC].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		// no factor
	}}

	tokenTypes[TOKEN_FUNCTION].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		prevType := token.identifier

		if token.beforeExpr && prevType == TOKEN_ELSE && !(prevType == TOKEN_SEMI && this.currentContext().Identifier == PAREN_STATEMENT) && !(prevType == TOKEN_RETURN /*&& lineBreak.test(this.input.slice(this.lastTokEnd, this.start)))*/) && !((prevType == TOKEN_COLON || prevType == TOKEN_BRACEL) && this.currentContext().Identifier == BRACKET_STATEMENT) {
			this.Context = append(this.Context, TokenContexts[FUNCTION_EXPRESSION])
		} else {
			this.Context = append(this.Context, TokenContexts[FUNCTION_STATEMENT])
		}
	}}

	tokenTypes[TOKEN_COLON].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if this.currentContext().Token == "function" {
			this.Context = this.Context[:len(this.Context)-1]
		}
		this.ExprAllowed = true
	}}

	tokenTypes[TOKEN_BACKQUOTE].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if this.currentContext().Identifier == QUOTE_TEMPLATE {
			this.Context = this.Context[:len(this.Context)-1]
		} else {
			this.Context = append(this.Context, TokenContexts[QUOTE_TEMPLATE])
		}
	}}

	tokenTypes[TOKEN_STAR].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
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

	tokenTypes[TOKEN_NAME].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		allowed := false

		if this.getEcmaVersion() >= 6 && token.identifier != TOKEN_DOT {
			if this.Value == "of" && !this.ExprAllowed || this.Value == "yield" || this.inGeneratorContext() {
				allowed = true
			}
		}
		this.ExprAllowed = allowed
	}}
}

var Pp = &Parser{}
