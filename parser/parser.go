package parser

import (
	"bytes"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Label struct {
	Kind           string
	Name           string
	StatementStart int
}

type PrivateName struct {
	Declared map[string]string
	Used     []*Node
}

type Parser struct {
	options                  *Options
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
	UndefinedExports         map[string]*Node
	ScopeStack               []*Scope
	RegexpState              *RegExpState
	PrivateNameStack         []*PrivateName
	InTemplateElement        bool
	InClassStaticBlock       bool
}

func GetAst(input []byte, options *Options, startPos int) (*Node, error) {
	initEcmaUnicode()
	this := Parser{}
	opts := GetOptions(options)
	this.options = opts
	options = opts
	this.SourceFile = options.SourceFile

	if this.getEcmaVersion() >= 6 {
		this.Keywords = WordsRegexp(syntaxKeywords["6"])
	} else {
		if options.SourceType == "module" {
			this.Keywords = WordsRegexp(syntaxKeywords["5module"])
		} else {
			WordsRegexp(syntaxKeywords["5"])
		}
	}
	reserved := ""
	if options.AllowReserved == ALLOW_RESERVED_TRUE {
		if this.getEcmaVersion() >= 6 {
			reserved = reservedWords["6"]
		} else if this.getEcmaVersion() == 5 {
			reserved = reservedWords["5"]
		} else {
			reserved = reservedWords["3"]
		}
		if options.SourceType == "module" {
			reserved += " await"
		}
	}
	this.ReservedWords = WordsRegexp(reserved)
	reservedStrict := reservedWords["strict"]

	if len(reserved) != 0 {
		reservedStrict = reservedStrict + " " + reserved
	}
	this.ReservedWordsStrict = WordsRegexp(reservedStrict)
	this.ReservedWordsStrictBind = WordsRegexp(reservedStrict + " " + reservedWords["strictBind"])
	this.input = input

	// Used to signal to callers of `readWord1` whether the word
	// contained any escape sequences. This is needed because words with
	// escape sequences must not be interpreted as keywords.
	this.ContainsEsc = false

	// Set up token state

	// The current position of the tokenizer in the input.
	if startPos != 0 {
		this.pos = startPos
		this.LineStart = strings.LastIndex(string(this.input[:startPos-1]), "\n") + 1
		this.CurLine = len(lineBreak.Split(string(this.input[:this.LineStart]), -1))
	} else {
		this.pos, this.LineStart = 0, 0
		this.CurLine = 1
	}

	// Properties of the current token:
	// Its type
	this.Type = tokenTypes[TOKEN_EOF]
	// For tokens that include more information than their type, the value
	this.Value = nil
	// Its start and end offset
	this.start, this.End = this.pos, this.pos
	// And, if locations are used, the {line, column} object
	// corresponding to those offsets
	this.startLoc, this.EndLoc = this.currentPosition(), this.currentPosition()

	// Position information for the previous token
	this.LastTokEndLoc, this.LastTokStartLoc = nil, nil
	this.LastTokStart, this.LastTokEnd = this.pos, this.pos

	// The context stack is used to superficially track syntactic
	// context to predict whether a regular expression is allowed in a
	// given position.
	this.Context = this.initialContext()
	this.ExprAllowed = true

	// Figure out if it's a module code.
	this.InModule = options.SourceType == "module"
	this.Strict = this.InModule || this.strictDirective(this.pos)

	// Used to signify the start of a potential arrow function
	this.PotentialArrowAt = -1
	this.PotentialArrowInForAwait = false

	// Positions to delayed-check that yield/await does not exist in default parameters.
	this.YieldPos, this.AwaitPos, this.AwaitIdentPos = 0, 0, 0
	// Labels in scope.
	this.Labels = []Label{}
	// Thus-far undefined exports.
	this.UndefinedExports = map[string]*Node{}

	// If enabled, skip leading hashbang line.
	if this.pos == 0 && options.AllowHashBang && string(this.input[0:2]) == "#!" {
		this.skipLineComment(2)
	}

	// Scope tracking for duplicate variable names (see scope.js)
	this.ScopeStack = []*Scope{}
	this.enterScope(SCOPE_TOP)

	// For RegExp validation
	this.RegexpState = nil

	// The stack of private names.
	// Each element has two properties: 'declared' and 'used'.
	// When it exited from the outermost class definition, all used private names must be declared.
	this.PrivateNameStack = []*PrivateName{}
	this.initAllUpdateContext()

	this.nextToken()
	node, err := this.parseTopLevel(this.startNode())

	if err != nil {
		return nil, err
	}

	return node, nil
}

func (p *Parser) inFunction() bool {
	return p.currentVarScope().Flags&SCOPE_FUNCTION == SCOPE_FUNCTION
}
func (p *Parser) inAsync() bool {
	return p.currentVarScope().Flags&SCOPE_ASYNC > 0
}

func (p *Parser) inGenerator() bool {
	return p.currentVarScope().Flags&SCOPE_GENERATOR > 0
}

func (p *Parser) canAwait() bool {
	for i := len(p.ScopeStack) - 1; i >= 0; i-- {
		scope := p.ScopeStack[i]
		flags := scope.Flags

		if flags&(SCOPE_CLASS_STATIC_BLOCK|SCOPE_CLASS_FIELD_INIT) > 0 {
			return false
		}

		if flags&SCOPE_FUNCTION == SCOPE_FUNCTION {
			return flags&SCOPE_ASYNC > 0
		}
	}

	return (p.InModule && p.getEcmaVersion() >= 13) || p.options.AllowAwaitOutsideFunction
}

func (p *Parser) allowSuper() bool {
	return p.currentThisScope().Flags&SCOPE_SUPER > 0 || p.options.AllowSuperOutsideMethod
}

func (p *Parser) allowDirectSuper() bool {
	return p.currentThisScope().Flags&SCOPE_DIRECT_SUPER > 0
}
func (p *Parser) allowNewDotTarget() bool {
	for _, scope := range p.ScopeStack {
		flags := scope.Flags

		if flags&(SCOPE_CLASS_STATIC_BLOCK|SCOPE_CLASS_FIELD_INIT) > 0 || flags&SCOPE_FUNCTION == SCOPE_FUNCTION && flags&SCOPE_ARROW != SCOPE_ARROW {
			return true
		}
	}
	return false
}

func (this *Parser) treatFunctionsAsVar() bool {
	return this.treatFunctionsAsVarInScope(this.currentScope())
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
		case 32, 160: // ' '
			this.pos = this.pos + size
		case 13:
			if this.input[this.pos+size] == 10 {
				this.pos = this.pos + size
			}
			fallthrough
		case 10, 8232, 8233:
			this.pos = this.pos + size
			if this.options.Locations {
				this.CurLine = this.CurLine + 1
				this.LineStart = this.pos
			}
		case 47: // '/'
			switch this.input[this.pos+1] {
			case 42: // '*'
				this.skipBlockComment()
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

// #### CONTEXT RELATED CODE

func (this *Parser) initialContext() []*TokenContext {
	return []*TokenContext{TokenContexts[BRACKET_STATEMENT]}
}

func (this *Parser) currentContext() *TokenContext {
	return this.Context[len(this.Context)-1]
}

func (this *Parser) inGeneratorContext() bool {
	for i := len(this.Context) - 1; i >= 1; i-- {
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
