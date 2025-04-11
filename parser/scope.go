package parser

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
