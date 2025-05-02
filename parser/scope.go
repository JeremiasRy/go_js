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

func functionFlags(async, generator bool) Flags {
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

func (this *Parser) currentThisScope() *Scope {
	for _, scope := range this.ScopeStack {
		if scope.Flags&(SCOPE_VAR|SCOPE_CLASS_FIELD_INIT|SCOPE_CLASS_STATIC_BLOCK) != 0 && scope.Flags&SCOPE_ARROW != SCOPE_ARROW {
			return scope
		}
	}
	return nil
}

func (p *Parser) currentVarScope() *Scope {
	for i := len(p.ScopeStack) - 1; ; i-- {
		scope := p.ScopeStack[i]
		if scope.Flags&(SCOPE_VAR|SCOPE_CLASS_FIELD_INIT|SCOPE_CLASS_STATIC_BLOCK) > 0 {
			return scope
		}
	}
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
