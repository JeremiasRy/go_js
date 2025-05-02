package parser

import (
	"regexp"
)

var literal = regexp.MustCompile(`^(?:'((?:\\[^]|[^'\\])*?)'|\"((?:\\[^]|[^"\\])*?)")`)

func (this *Parser) eat(token Token) bool {
	if this.Type.identifier == token {
		this.next(false)
		return true
	} else {
		return false
	}
}

func (this *Parser) expect(token Token) error {
	if this.eat(token) {
		return nil
	}
	return this.unexpected("Expected "+tokenToString[token], &this.pos)
}

func (this *Parser) unexpected(msg string, pos *int) error {
	if pos != nil {
		return this.raise(*pos, "Unexpected token: "+msg)
	} else {
		return this.raise(this.start, "Unexpected token: "+msg)
	}
}

func (this *Parser) canInsertSemicolon() bool {
	return this.Type.identifier == TOKEN_EOF ||
		this.Type.identifier == TOKEN_BRACER ||
		lineBreak.Match(this.input[this.LastTokEnd:this.start])
}

func (this *Parser) semicolon() error {
	if !this.eat(TOKEN_SEMI) && !this.insertSemicolon() {
		return this.unexpected("Unexpected token", nil)
	}
	return nil
}

func (this *Parser) insertSemicolon() bool {
	return this.canInsertSemicolon()
	/* Not ported:
	if this.canInsertSemicolon() {
		if (this.options.onInsertedSemicolon)
			this.options.onInsertedSemicolon(this.lastTokEnd, this.lastTokEndLoc)
		return true
	}
	return false
	*/
}

func (this *Parser) isContextual(name string) bool {
	if value, ok := this.Value.(string); ok {
		return value == name && !this.ContainsEsc && this.Type.identifier == TOKEN_NAME
	}
	return false
}

func (this *Parser) checkYieldAwaitInDefaultParams() error {
	if this.YieldPos != 0 && (!(this.AwaitPos != 0) || this.YieldPos < this.AwaitPos) {
		return this.raise(this.YieldPos, "Yield expression cannot be a default value")
	}

	if this.AwaitPos != 0 {
		this.raise(this.AwaitPos, "Await expression cannot be a default value")
	}
	return nil
}

func (this *Parser) checkPatternErrors(refDestructuringErrors *DestructuringErrors, isAssign bool) error {
	if refDestructuringErrors == nil {
		return nil
	}
	if refDestructuringErrors.trailingComma > -1 {
		return this.raiseRecoverable(refDestructuringErrors.trailingComma, "Comma is not permitted after the rest element")
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
		return this.raiseRecoverable(parens, msg)
	}
	return nil
}

func (this *Parser) afterTrailingComma(tokType Token, notNext bool) bool {
	if this.Type.identifier == tokType {
		/*
					Unimplemented:

			    	if this.options.OnTrailingComma {
						this.options.onTrailingComma(this.lastTokStart, this.lastTokStartLoc)
					}
		*/
		if !notNext {
			this.next(false)
		}

		return true
	}
	return false
}

func (this *Parser) strictDirective(start int) bool {
	if this.getEcmaVersion() < 5 {
		return false
	}

	for {
		loc := skipWhiteSpace.FindStringIndex(string(this.input[start:]))
		if loc == nil {
			loc = []int{0, 0}
		}
		start += loc[1]

		match := literal.FindStringSubmatch(string(this.input[start:]))
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
			loc = skipWhiteSpace.FindStringIndex(string(this.input[spaceStart:]))
			var spaceAfter string
			var end int
			if loc == nil {
				spaceAfter = ""
				end = spaceStart
			} else {
				spaceAfter = string(this.input[spaceStart : spaceStart+loc[1]])
				end = spaceStart + loc[1]
			}

			var next byte
			if end < len(this.input) {
				next = this.input[end]
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
					if next != '!' || (end+1 < len(this.input) && this.input[end+1] != '=') {
						return true
					}
				}
			}
		}

		start += len(match[0])

		loc = skipWhiteSpace.FindStringIndex(string(this.input[start:]))
		if loc == nil {
			loc = []int{0, 0}
		}
		start += loc[1]

		if start < len(this.input) && this.input[start] == ';' {
			start++
		}
	}
}

func (this *Parser) isSimpleAssignTarget(expr *Node) bool {
	if expr.type_ == NODE_PARENTHESIZED_EXPRESSION {
		return this.isSimpleAssignTarget(expr.expression)
	}

	return expr.type_ == NODE_IDENTIFIER || expr.type_ == NODE_MEMBER_EXPRESSION
}
