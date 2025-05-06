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
	p := Parser{}
	opts := GetOptions(options)
	p.options = opts
	options = opts
	p.SourceFile = options.SourceFile

	if p.getEcmaVersion() >= 6 {
		p.Keywords = WordsRegexp(syntaxKeywords["6"])
	} else {
		if options.SourceType == "module" {
			p.Keywords = WordsRegexp(syntaxKeywords["5module"])
		} else {
			WordsRegexp(syntaxKeywords["5"])
		}
	}
	reserved := ""
	if options.AllowReserved != ALLOW_RESERVED_TRUE {
		if p.getEcmaVersion() >= 6 {
			reserved = reservedWords["6"]
		} else if p.getEcmaVersion() == 5 {
			reserved = reservedWords["5"]
		} else {
			reserved = reservedWords["3"]
		}
		if options.SourceType == "module" {
			reserved += " await"
		}
	}

	p.ReservedWords = WordsRegexp(reserved)
	reservedStrict := reservedWords["strict"]

	if len(reserved) != 0 {
		reservedStrict = reservedStrict + " " + reserved
	}
	p.ReservedWordsStrict = WordsRegexp(reservedStrict)
	p.ReservedWordsStrictBind = WordsRegexp(reservedStrict + " " + reservedWords["strictBind"])
	p.input = input

	// Used to signal to callers of `readWord1` whether the word
	// contained any escape sequences. This is needed because words with
	// escape sequences must not be interpreted as keywords.
	p.ContainsEsc = false

	// Set up token state

	// The current position of the tokenizer in the input.
	if startPos != 0 {
		p.pos = startPos
		p.LineStart = strings.LastIndex(string(p.input[:startPos-1]), "\n") + 1
		p.CurLine = len(lineBreak.Split(string(p.input[:p.LineStart]), -1))
	} else {
		p.pos, p.LineStart = 0, 0
		p.CurLine = 1
	}

	// Properties of the current token:
	// Its type
	p.Type = tokenTypes[TOKEN_EOF]
	// For tokens that include more information than their type, the value
	p.Value = nil
	// Its start and end offset
	p.start, p.End = p.pos, p.pos
	// And, if locations are used, the {line, column} object
	// corresponding to those offsets
	p.startLoc, p.EndLoc = p.currentPosition(), p.currentPosition()

	// Position information for the previous token
	p.LastTokEndLoc, p.LastTokStartLoc = nil, nil
	p.LastTokStart, p.LastTokEnd = p.pos, p.pos

	// The context stack is used to superficially track syntactic
	// context to predict whether a regular expression is allowed in a
	// given position.
	p.Context = p.initialContext()
	p.ExprAllowed = true

	// Figure out if it's a module code.
	p.InModule = options.SourceType == "module"
	p.Strict = p.InModule || p.strictDirective(p.pos)

	// Used to signify the start of a potential arrow function
	p.PotentialArrowAt = -1
	p.PotentialArrowInForAwait = false

	// Positions to delayed-check that yield/await does not exist in default parameters.
	p.YieldPos, p.AwaitPos, p.AwaitIdentPos = 0, 0, 0
	// Labels in scope.
	p.Labels = []Label{}
	// Thus-far undefined exports.
	p.UndefinedExports = map[string]*Node{}

	// If enabled, skip leading hashbang line.
	if p.pos == 0 && options.AllowHashBang && string(p.input[0:2]) == "#!" {
		p.skipLineComment(2)
	}

	// Scope tracking for duplicate variable names (see scope.js)
	p.ScopeStack = []*Scope{}
	p.enterScope(SCOPE_TOP)

	// For RegExp validation
	p.RegexpState = nil

	// The stack of private names.
	// Each element has two properties: 'declared' and 'used'.
	// When it exited from the outermost class definition, all used private names must be declared.
	p.PrivateNameStack = []*PrivateName{}
	p.initAllUpdateContext()

	p.nextToken()
	node, err := p.parseTopLevel(p.startNode())

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

func (p *Parser) treatFunctionsAsVar() bool {
	return p.treatFunctionsAsVarInScope(p.currentScope())
}
func (p *Parser) getEcmaVersion() int {
	if ecmaVersion, ok := p.options.ecmaVersion.(int); ok {
		return ecmaVersion
	}
	panic("Ecma verion was set to something weird")
}

func (p *Parser) raise(pos int, message string) error {
	loc := getLineInfo(p.input, pos)
	line := strconv.Itoa(loc.Line)
	column := strconv.Itoa(loc.Column)
	message += strings.Join([]string{" (", line, ":", column, ")"}, "")

	if p.SourceFile != nil {
		message += strings.Join([]string{" in ", *p.SourceFile}, "")
	}

	return errors.New(message)
}

func (p *Parser) raiseRecoverable(pos int, message string) error {
	return p.raise(pos, message)
}

// #### WHITESPACE

func (p *Parser) skipSpace() error {
Loop:
	for p.pos < len(p.input) {
		ch, size, _ := p.fullCharCodeAtPos()
		switch ch {
		case 32, 160: // ' '
			p.pos = p.pos + size
		case 13:
			if p.input[p.pos+size] == 10 {
				p.pos = p.pos + size
			}
			fallthrough
		case 10, 8232, 8233:
			p.pos = p.pos + size
			if p.options.Locations {
				p.CurLine = p.CurLine + 1
				p.LineStart = p.pos
			}
		case 47: // '/'
			switch p.input[p.pos+1] {
			case 42: // '*'
				p.skipBlockComment()
			case 47:
				p.skipLineComment(2)
			default:
				break Loop
			}
		default:
			if ch > 8 && ch < 14 || ch >= 5760 && nonASCIIwhitespace.Match(utf8.AppendRune([]byte{}, ch)) {
				p.pos = p.pos + size
			} else {
				break Loop
			}
		}
	}
	return nil
}

func (p *Parser) skipBlockComment() error {
	start := p.pos
	p.pos += 2 // Skip "/*"
	end := bytes.Index(p.input[p.pos:], []byte("*/"))
	if end == -1 {
		return p.raise(start, "Unterminated comment")
	}
	p.pos += end + 2 // Move past "*/"
	return nil
}

// #### SCOPE RELATED CODE

func (p *Parser) braceIsBlock(prevType Token) bool {
	parent := p.currentContext().Identifier
	isExpr := p.currentContext().IsExpr

	if parent == FUNCTION_EXPRESSION || parent == FUNCTION_STATEMENT {
		return true
	}

	if prevType == TOKEN_COLON && (parent == BRACKET_STATEMENT || parent == BRACKET_EXPRESSION) {
		return !isExpr
	}

	if prevType == TOKEN_RETURN || prevType == TOKEN_NAME && p.ExprAllowed {
		// return lineBreak.test(p.input.slice(p.lastTokEnd, p.start))
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

	return !p.ExprAllowed
}

func (p *Parser) enterScope(flags Flags) {
	p.ScopeStack = append(p.ScopeStack, NewScope(flags))
}

func (p *Parser) exitScope() {
	p.ScopeStack = p.ScopeStack[:len(p.ScopeStack)-1]
}

func (p *Parser) currentScope() *Scope {
	return p.ScopeStack[len(p.ScopeStack)-1]
}

// #### CONTEXT RELATED CODE

func (p *Parser) initialContext() []*TokenContext {
	return []*TokenContext{TokenContexts[BRACKET_STATEMENT]}
}

func (p *Parser) currentContext() *TokenContext {
	return p.Context[len(p.Context)-1]
}

func (p *Parser) inGeneratorContext() bool {
	for i := len(p.Context) - 1; i >= 1; i-- {
		context := p.Context[i]
		if context.Token == "function" {
			return context.Generator
		}
	}
	return false
}

func (p *Parser) updateContext(prevType *TokenType) {
	update, current := p.Type, p.Type
	if len(current.keyword) != 0 && prevType.identifier == TOKEN_DOT {
		p.ExprAllowed = false
	} else if current.updateContext != nil {
		update.updateContext = current.updateContext
		update.updateContext.updateContext(prevType)
	} else {
		p.ExprAllowed = current.beforeExpr
	}
}

func (p *Parser) overrideContext(tokenCtx *TokenContext) {
	if p.currentContext().Identifier != tokenCtx.Identifier {
		p.Context[len(p.Context)-1] = tokenCtx
	}
}

func (p *Parser) initAllUpdateContext() {
	tokenTypes[TOKEN_PARENR].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if len(p.Context) == 1 {
			p.ExprAllowed = true
			return
		}

		out := p.Context[len(p.Context)-1]
		p.Context = p.Context[:len(p.Context)-1]
		if out.Identifier == BRACKET_STATEMENT && p.currentContext().Token == "function" {
			out = p.Context[len(p.Context)-1]
			p.Context = p.Context[:len(p.Context)-1]
		}
		p.ExprAllowed = !out.IsExpr
	}}

	tokenTypes[TOKEN_BRACER].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if len(p.Context) == 1 {
			p.ExprAllowed = true
			return
		}

		out := p.Context[len(p.Context)-1]
		p.Context = p.Context[:len(p.Context)-1]
		if out.Identifier == BRACKET_STATEMENT && p.currentContext().Token == "function" {
			out = p.Context[len(p.Context)-1]
			p.Context = p.Context[:len(p.Context)-1]
		}
		p.ExprAllowed = !out.IsExpr
	}}

	tokenTypes[TOKEN_BRACEL].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if p.braceIsBlock(token.identifier) {
			p.Context = append(p.Context, TokenContexts[BRACKET_STATEMENT])
		} else {
			p.Context = append(p.Context, TokenContexts[BRACKET_EXPRESSION])
		}
		p.ExprAllowed = true

	}}

	tokenTypes[TOKEN_DOLLARBRACEL].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		p.Context = append(p.Context, TokenContexts[BRACKET_TEMPLATE])
		p.ExprAllowed = true
	}}

	tokenTypes[TOKEN_PARENL].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		statementParens := token.identifier == TOKEN_IF || token.identifier == TOKEN_FOR || token.identifier == TOKEN_WITH || token.identifier == TOKEN_WHILE

		if statementParens {

			p.Context = append(p.Context, TokenContexts[PAREN_STATEMENT])
		} else {
			p.Context = append(p.Context, TokenContexts[PAREN_EXPRESSION])
		}
		p.ExprAllowed = true
	}}

	tokenTypes[TOKEN_INCDEC].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		// no factor
	}}

	tokenTypes[TOKEN_FUNCTION].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		prevType := token.identifier

		if token.beforeExpr && prevType == TOKEN_ELSE && !(prevType == TOKEN_SEMI && p.currentContext().Identifier == PAREN_STATEMENT) && !(prevType == TOKEN_RETURN /*&& lineBreak.test(p.input.slice(p.lastTokEnd, p.start)))*/) && !((prevType == TOKEN_COLON || prevType == TOKEN_BRACEL) && p.currentContext().Identifier == BRACKET_STATEMENT) {
			p.Context = append(p.Context, TokenContexts[FUNCTION_EXPRESSION])
		} else {
			p.Context = append(p.Context, TokenContexts[FUNCTION_STATEMENT])
		}
	}}

	tokenTypes[TOKEN_COLON].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if p.currentContext().Token == "function" {
			p.Context = p.Context[:len(p.Context)-1]
		}
		p.ExprAllowed = true
	}}

	tokenTypes[TOKEN_BACKQUOTE].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if p.currentContext().Identifier == QUOTE_TEMPLATE {
			p.Context = p.Context[:len(p.Context)-1]
		} else {
			p.Context = append(p.Context, TokenContexts[QUOTE_TEMPLATE])
		}
	}}

	tokenTypes[TOKEN_STAR].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if token.identifier == TOKEN_FUNCTION {
			idx := len(p.Context) - 1

			if p.Context[idx].Identifier == FUNCTION_EXPRESSION {
				p.Context[idx] = TokenContexts[FUNCTION_EXPRESSION_GENERATOR]
			} else {
				p.Context[idx] = TokenContexts[FUNCTION_GENERATOR]
			}
			p.ExprAllowed = true
		}
	}}

	tokenTypes[TOKEN_NAME].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		allowed := false

		if p.getEcmaVersion() >= 6 && token.identifier != TOKEN_DOT {
			if p.Value == "of" && !p.ExprAllowed || p.Value == "yield" || p.inGeneratorContext() {
				allowed = true
			}
		}
		p.ExprAllowed = allowed
	}}
}

var Pp = &Parser{}
