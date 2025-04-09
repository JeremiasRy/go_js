package parser

type Options struct {
	EcmaVersion                 interface{}
	SourceType                  string
	OnInsertedSemicolon         interface{}
	OnTrailingComma             interface{}
	AllowReserved               *bool
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
	EcmaVersion:                 nil,
	SourceType:                  "script",
	OnInsertedSemicolon:         nil,
	OnTrailingComma:             nil,
	AllowReserved:               nil,
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

	// Copy default values and override with provided options
	if opts == nil {
		options = DefaultOptions
	} else {
		options = DefaultOptions
		// In Go, we'd typically use reflection or explicit field checks
		// For simplicity, manually checking key fields
		if opts.EcmaVersion != nil {
			options.EcmaVersion = opts.EcmaVersion
		}
		if opts.SourceType != "" {
			options.SourceType = opts.SourceType
		}
		// ... similar checks for other fields would go here
	}

	// Handle ecmaVersion special cases
	switch v := options.EcmaVersion.(type) {
	case string:
		if v == "latest" {
			options.EcmaVersion = 1e8
		}
	case nil:
		if !warnedAboutEcmaVersion {
			warnedAboutEcmaVersion = true
			options.EcmaVersion = 11
		}
	case int:
		if v >= 2015 {
			options.EcmaVersion = v - 2009
		}
	}

	// Handle allowReserved default
	if options.AllowReserved == nil {
		ecmaVer, ok := options.EcmaVersion.(int)
		if !ok {
			options.AllowReserved = new(bool)
			*options.AllowReserved = true // default case
		} else {
			val := ecmaVer < 5
			options.AllowReserved = &val
		}
	}

	// Handle allowHashBang default
	if opts == nil || !opts.AllowHashBang { // Simplistic check
		ecmaVer, ok := options.EcmaVersion.(int)
		if ok && ecmaVer >= 14 {
			options.AllowHashBang = true
		}
	}

	// Handle onToken array case
	if tokens, ok := options.OnToken.([]interface{}); ok {
		options.OnToken = func(token interface{}) {
			tokens = append(tokens, token)
		}
	}

	// Handle onComment array case
	if array, ok := options.OnComment.([]interface{}); ok {
		options.OnComment = pushComment(options, array)
	}

	return options
}

func pushComment(options Options, array []interface{}) func(bool, string, int, int, Location, Location) {
	return func(block bool, text string, start, end int, startLoc, endLoc Location) {
		comment := struct {
			Type  string
			Value string
			Start int
			End   int
			Loc   *SourceLocation
			Range *[2]int
		}{
			Type:  "Line",
			Value: text,
			Start: start,
			End:   end,
		}
		if block {
			comment.Type = "Block"
		}
		if options.Locations {
			comment.Loc = NewSourceLocation(pp, startLoc, endLoc)
		}
		if options.Ranges {
			comment.Range = &[2]int{start, end}
		}
		array = append(array, comment)
	}
}
