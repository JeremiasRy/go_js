package parser

import "slices"

type Flags int

const (
	SCOPE_TOP                Flags = 1
	SCOPE_FUNCTION           Flags = 2
	SCOPE_ASYNC              Flags = 4
	SCOPE_GENERATOR          Flags = 8
	SCOPE_ARROW              Flags = 16
	SCOPE_SIMPLE_CATCH       Flags = 32
	SCOPE_SUPER              Flags = 64
	SCOPE_DIRECT_SUPER       Flags = 128
	SCOPE_CLASS_STATIC_BLOCK Flags = 256
	SCOPE_CLASS_FIELD_INIT   Flags = 512
	SCOPE_VAR                      = SCOPE_TOP | SCOPE_FUNCTION | SCOPE_CLASS_STATIC_BLOCK
)

func FunctionFlags(async, generator bool) Flags {
	flags := SCOPE_FUNCTION

	if async {
		flags = flags | SCOPE_ASYNC
	}

	if generator {
		flags = flags | SCOPE_GENERATOR
	}

	return flags
}

// Used in checkLVal* and declareName to determine the type of a binding
const (
	BIND_NONE         Flags = 0 // Not a binding
	BIND_VAR          Flags = 1 // Var-style binding
	BIND_LEXICAL      Flags = 2 // Let- or const-style binding
	BIND_FUNCTION     Flags = 3 // Function declaration
	BIND_SIMPLE_CATCH Flags = 4 // Simple (identifier pattern) catch binding
	BIND_OUTSIDE      Flags = 5 // Special case for function names as bound inside the function)
)

type Scope struct {
	Flags     Flags
	Var       []string
	Lexical   []string
	Functions []string
}

func NewScope(flags Flags) *Scope {
	return &Scope{
		Flags:     flags,
		Var:       []string{},
		Lexical:   []string{},
		Functions: []string{},
	}
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
