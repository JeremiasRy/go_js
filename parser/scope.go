package parser

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
