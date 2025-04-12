package parser

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
	Context                  []*TokContext
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

func (pp *Parser) initialContext() []*TokContext {
	return []*TokContext{ContextTypes[BRACKET_STATEMENT]}
}

func (pp *Parser) currentContext() *TokContext {
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
