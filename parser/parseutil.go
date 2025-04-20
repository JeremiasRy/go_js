package parser

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
	return this.unexpected(&this.pos)
}

func (this *Parser) unexpected(pos *int) error {
	if pos != nil {
		return this.raise(*pos, "Unexpected token")
	} else {
		return this.raise(this.start, "Unexpected token")
	}
}

func (this *Parser) canInsertSemicolon() bool {
	return this.Type.identifier == TOKEN_EOF ||
		this.Type.identifier == TOKEN_BRACER ||
		lineBreak.Match(this.input[this.LastTokEnd:this.start])
}

func (this *Parser) isContextual(name string) bool {
	if value, ok := this.Value.(string); ok {
		return this.Type.identifier == TOKEN_NAME && value == name && !this.ContainsEsc
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
