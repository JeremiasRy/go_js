package parser

type SourceLocation struct {
	Start Location
	End   Location
}

type Location struct {
	Line   int
	Column int
}

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
	Value                    interface{}
	Start                    int
	End                      int
	StartLoc                 SourceLocation
	EndLoc                   SourceLocation
	LastTokStart             int
	LastTokEnd               int
	LastTokStartLoc          *SourceLocation
	LastTokEndLoc            *SourceLocation
	Context                  []TokContext // Assumed struct
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
	ScopeStack               []Scope       // Assumed struct
	RegexpState              *RegExpState  // Assumed struct
	PrivateNameStack         []PrivateName // Assumed struct
}
