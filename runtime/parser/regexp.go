package parser

import "regexp"

var lineBreak = regexp.MustCompile("\r\n?|\n|\u2028|\u2029")

type RegExpState struct {
}

func (res *RegExpState) reset(i int, s string, sp string) {
	panic("Unimplemented")

}

func (p *Parser) NewRegExpState() *RegExpState {
	return &RegExpState{}
}
