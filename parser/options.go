package parser

type Options struct {
	ecmaVersion                 interface{}
	SourceType                  string
	OnInsertedSemicolon         interface{}
	OnTrailingComma             interface{}
	AllowReserved               bool
	AllowReturnOutsideFunction  bool
	AllowImportExportEverywhere bool
	AllowAwaitOutsideFunction   *bool
	AllowSuperOutsideMethod     *bool
	AllowHashBang               bool
	CheckPrivateFields          bool
	Locations                   bool
	OnToken                     interface{} // function callback or array
	OnComment                   interface{} // function callback or array
	Ranges                      bool
	Program                     interface{} // AST node type
	SourceFile                  *string
	DirectSourceFile            *string
	PreserveParens              bool
}

var DefaultOptions = Options{
	ecmaVersion:                 nil,
	SourceType:                  "script",
	OnInsertedSemicolon:         nil,
	OnTrailingComma:             nil,
	AllowReserved:               false,
	AllowReturnOutsideFunction:  false,
	AllowImportExportEverywhere: false,
	AllowAwaitOutsideFunction:   nil,
	AllowSuperOutsideMethod:     nil,
	AllowHashBang:               false,
	CheckPrivateFields:          true,
	Locations:                   false,
	OnToken:                     nil,
	OnComment:                   nil,
	Ranges:                      false,
	Program:                     nil,
	SourceFile:                  nil,
	DirectSourceFile:            nil,
	PreserveParens:              false,
}

var warnedAboutEcmaVersion = false

func GetOptions(opts *Options) Options {
	options := Options{}

	if opts == nil {
		options = DefaultOptions
	} else {
		options = DefaultOptions
		if opts.ecmaVersion != nil {
			options.ecmaVersion = opts.ecmaVersion
		}
		if opts.SourceType != "" {
			options.SourceType = opts.SourceType
		}
	}

	switch v := options.ecmaVersion.(type) {
	case string:
		if v == "latest" {
			options.ecmaVersion = 1e8
		}
	case nil:
		if !warnedAboutEcmaVersion {
			warnedAboutEcmaVersion = true
			options.ecmaVersion = 11
		}
	case int:
		if v >= 2015 {
			options.ecmaVersion = v - 2009
		}
	}

	if options.AllowReserved {
		ecmaVer, ok := options.ecmaVersion.(int)
		if !ok {
			options.AllowReserved = true
		} else {
			options.AllowReserved = ecmaVer < 5
		}
	}

	if opts == nil || !opts.AllowHashBang {
		ecmaVer, ok := options.ecmaVersion.(int)
		if ok && ecmaVer >= 14 {
			options.AllowHashBang = true
		}
	}

	if tokens, ok := options.OnToken.([]interface{}); ok {
		options.OnToken = func(token interface{}) {
			tokens = append(tokens, token)
		}
	}

	if array, ok := options.OnComment.([]*Comment); ok {
		options.OnComment = pushComment(options, array)
	}

	return options
}

type Comment struct {
	Type  string
	Value string
	Start int
	End   int
	Loc   *SourceLocation
	Range *[2]int
}

func pushComment(options Options, array []*Comment) func(bool, string, int, int, *Location, *Location) {
	return func(block bool, text string, start, end int, startLoc, endLoc *Location) {
		comment := &Comment{
			Type:  "Line",
			Value: text,
			Start: start,
			End:   end,
		}
		if block {
			comment.Type = "Block"
		}
		if options.Locations {
			comment.Loc = NewSourceLocation(Pp, startLoc, endLoc)
		}
		if options.Ranges {
			comment.Range = &[2]int{start, end}
		}
		array = append(array, comment)
	}
}
