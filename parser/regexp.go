package parser

import "regexp"

var lineBreak = regexp.MustCompile("/\r\n?|\n|\u2028|\u2029/")

type RegExpState struct {
}
