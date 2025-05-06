package parser

import (
	"regexp"
)

var literal = regexp.MustCompile(`^(?:'((?:\\[^]|[^'\\])*?)'|\"((?:\\[^]|[^"\\])*?)")`)

func (p *Parser) eat(token Token) bool {
	if p.Type.identifier == token {
		p.next(false)
		return true
	} else {
		return false
	}
}

func (p *Parser) expect(token Token) error {
	if p.eat(token) {
		return nil
	}
	return p.unexpected("Expected "+tokenToString[token], &p.pos)
}

func (p *Parser) unexpected(msg string, pos *int) error {
	if pos != nil {
		return p.raise(*pos, "Unexpected token: "+msg)
	} else {
		return p.raise(p.start, "Unexpected token: "+msg)
	}
}

func (p *Parser) canInsertSemicolon() bool {
	return p.Type.identifier == TOKEN_EOF ||
		p.Type.identifier == TOKEN_BRACER ||
		lineBreak.Match(p.input[p.LastTokEnd:p.start])
}

func (p *Parser) semicolon() error {
	if !p.eat(TOKEN_SEMI) && !p.insertSemicolon() {
		return p.unexpected("Unexpected token", nil)
	}
	return nil
}

func (p *Parser) insertSemicolon() bool {
	return p.canInsertSemicolon()
	/* Not ported:
	if p.canInsertSemicolon() {
		if (p.options.onInsertedSemicolon)
			p.options.onInsertedSemicolon(p.lastTokEnd, p.lastTokEndLoc)
		return true
	}
	return false
	*/
}

func (p *Parser) isContextual(name string) bool {
	if value, ok := p.Value.(string); ok {
		return value == name && !p.ContainsEsc && p.Type.identifier == TOKEN_NAME
	}
	return false
}

func (p *Parser) checkYieldAwaitInDefaultParams() error {
	if p.YieldPos != 0 && (!(p.AwaitPos != 0) || p.YieldPos < p.AwaitPos) {
		return p.raise(p.YieldPos, "Yield expression cannot be a default value")
	}

	if p.AwaitPos != 0 {
		p.raise(p.AwaitPos, "Await expression cannot be a default value")
	}
	return nil
}

func (p *Parser) checkPatternErrors(refDestructuringErrors *DestructuringErrors, isAssign bool) error {
	if refDestructuringErrors == nil {
		return nil
	}
	if refDestructuringErrors.trailingComma > -1 {
		return p.raiseRecoverable(refDestructuringErrors.trailingComma, "Comma is not permitted after the rest element")
	}
	var parens int
	if isAssign {
		parens = refDestructuringErrors.parenthesizedAssign
	} else {
		parens = refDestructuringErrors.parenthesizedBind
	}
	if parens > -1 {
		var msg string
		if isAssign {
			msg = "Assigning to rvalue"
		} else {
			msg = "Parenthesized pattern"
		}
		return p.raiseRecoverable(parens, msg)
	}
	return nil
}

func (p *Parser) afterTrailingComma(tokType Token, notNext bool) bool {
	if p.Type.identifier == tokType {
		/*
					Unimplemented:

			    	if p.options.OnTrailingComma {
						p.options.onTrailingComma(p.lastTokStart, p.lastTokStartLoc)
					}
		*/
		if !notNext {
			p.next(false)
		}

		return true
	}
	return false
}

func (p *Parser) strictDirective(start int) bool {
	if p.getEcmaVersion() < 5 {
		return false
	}

	for {
		loc := skipWhiteSpace.FindStringIndex(string(p.input[start:]))
		if loc == nil {
			loc = []int{0, 0}
		}
		start += loc[1]

		match := literal.FindStringSubmatch(string(p.input[start:]))
		if match == nil {
			return false
		}
		var content string
		if match[1] != "" {
			content = match[1]
		} else {
			content = match[2]
		}

		if content == "use strict" {
			spaceStart := start + len(match[0])
			loc = skipWhiteSpace.FindStringIndex(string(p.input[spaceStart:]))
			var spaceAfter string
			var end int
			if loc == nil {
				spaceAfter = ""
				end = spaceStart
			} else {
				spaceAfter = string(p.input[spaceStart : spaceStart+loc[1]])
				end = spaceStart + loc[1]
			}

			var next byte
			if end < len(p.input) {
				next = p.input[end]
			} else {
				next = 0
			}

			if next == ';' || next == '}' {
				return true
			}

			lineBreak := regexp.MustCompile(`\n|\r\n?|\u2028|\u2029`)
			if lineBreak.MatchString(spaceAfter) {
				quote := match[0][0]
				if next == 0 || !regexp.MustCompile(`[(\[.`+string(quote)+`+\-/*%<>=,?\^&]`).MatchString(string(next)) {
					if next != '!' || (end+1 < len(p.input) && p.input[end+1] != '=') {
						return true
					}
				}
			}
		}

		start += len(match[0])

		loc = skipWhiteSpace.FindStringIndex(string(p.input[start:]))
		if loc == nil {
			loc = []int{0, 0}
		}
		start += loc[1]

		if start < len(p.input) && p.input[start] == ';' {
			start++
		}
	}
}

func (p *Parser) isSimpleAssignTarget(expr *Node) bool {
	if expr.Type == NODE_PARENTHESIZED_EXPRESSION {
		return p.isSimpleAssignTarget(expr.Expression)
	}

	return expr.Type == NODE_IDENTIFIER || expr.Type == NODE_MEMBER_EXPRESSION
}
