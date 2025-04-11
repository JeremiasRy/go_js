package parser

import (
	"slices"
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
	Keywords                 any // RegExp equivalent
	ReservedWords            any // RegExp equivalent
	ReservedWordsStrict      any // RegExp equivalent
	ReservedWordsStrictBind  any // RegExp equivalent
	Input                    string
	ContainsEsc              bool
	Pos                      int
	LineStart                int
	CurLine                  int
	Type                     TokenType // Assumed enum/struct
	Value                    any
	Start                    int
	End                      int
	StartLoc                 SourceLocation
	EndLoc                   SourceLocation
	LastTokStart             int
	LastTokEnd               int
	LastTokStartLoc          *SourceLocation
	LastTokEndLoc            *SourceLocation
	Context                  []*TokContext // Assumed struct
	ExprAllowed              bool
	InModule                 bool
	Strict                   bool
	PotentialArrowAt         int
	PotentialArrowInForAwait bool
	YieldPos                 int
	AwaitPos                 int
	AwaitIdentPos            int
	Labels                   []Label // Assumed struct
	UndefinedExports         map[string]any
	ScopeStack               []*Scope      // Assumed struct
	RegexpState              *RegExpState  // Assumed struct
	PrivateNameStack         []PrivateName // Assumed struct
}

func (pp *Parser) initialContext() []*TokContext {
	return []*TokContext{ContextTypes[BRACKET_STATEMENT]}
}

func (pp *Parser) currentContext() *TokContext {
	return pp.Context[len(pp.Context)-1]
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
