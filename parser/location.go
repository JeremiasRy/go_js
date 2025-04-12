package parser

type SourceLocation struct {
	Start      *Location
	End        *Location
	Sourcefile *string
}

type Location struct {
	Line   int
	Column int
}

func NewSourceLocation(parser *Parser, start, end *Location) *SourceLocation {
	return &SourceLocation{Start: start, End: end, Sourcefile: parser.SourceFile}
}

func (l *Location) Offset(n int) *Location {
	return &Location{
		Line:   l.Line,
		Column: l.Column + n,
	}
}

func NewLocation(line, column int) *Location {
	return &Location{
		Line:   line,
		Column: column,
	}
}
