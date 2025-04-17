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

func getLineInfo(input []byte, offset int) *Location {
	line, cur := 1, 0
	for {
		nextBreak := nextLineBreak(input, cur, offset)
		if nextBreak < 0 {
			return &Location{Line: line, Column: offset - cur}
		}
		line++
		cur = nextBreak
	}
}
