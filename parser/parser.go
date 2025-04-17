package parser

import (
	"bytes"
	"errors"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Label struct {
	Kind string
}

type PrivateName struct {
	Declared bool
	Used     bool
}

type Parser struct {
	options                  Options
	SourceFile               *string
	Keywords                 any
	ReservedWords            any
	ReservedWordsStrict      any
	ReservedWordsStrictBind  any
	input                    []byte
	ContainsEsc              bool
	pos                      int
	LineStart                int
	CurLine                  int
	Type                     *TokenType
	Value                    any
	Start                    int
	End                      int
	StartLoc                 *Location
	EndLoc                   *Location
	LastTokStart             int
	LastTokEnd               int
	LastTokStartLoc          *Location
	LastTokEndLoc            *Location
	Context                  []*TokenContext
	ExprAllowed              bool
	InModule                 bool
	Strict                   bool
	PotentialArrowAt         int
	PotentialArrowInForAwait bool
	YieldPos                 int
	AwaitPos                 int
	AwaitIdentPos            int
	Labels                   []Label
	UndefinedExports         map[string]any
	ScopeStack               []*Scope
	RegexpState              *RegExpState
	PrivateNameStack         []PrivateName
	InTemplateElement        bool
}

// TOKEN RELATED CODE

// Move to next token
func (this *Parser) next(ignoreEscapeSequenceInKeyword bool) {
	if !ignoreEscapeSequenceInKeyword && len(this.Type.keyword) != 0 && this.ContainsEsc {
		// this.raiseRecoverable(this.start, "Escape sequence in keyword " + this.type.keyword)
	}

	if this.options.OnToken != nil {
		// TODO? Maybe? I dont need this?
	}

	this.LastTokEnd = this.End
	this.LastTokStart = this.Start
	this.LastTokEndLoc = this.StartLoc
	this.LastTokStartLoc = this.StartLoc
	this.nextToken()
}

func (this *Parser) nextToken() {
	context := this.currentContext()
	if context == nil || context.PreserveSpace {
		this.skipSpace()
	}

	this.Start = this.pos
	if this.options.Locations {
		this.StartLoc = this.currentPosition()
	}

	if this.pos >= len(this.input) {
		this.finishToken(tokenTypes[TOKEN_EOF], nil)
		return
	}

	if context.Override != nil {
		context.Override(this)
		return
	} else {
		ch, _, _ := this.fullCharCodeAtPos()
		this.readToken(ch)
	}
}

func (this *Parser) fullCharCodeAtPos() (code rune, size int, err error) {
	if this.pos < 0 || this.pos >= len(this.input) {

		return 0, 0, this.raise(this.pos, "Invalid position")
	}
	r, size := utf8.DecodeRune(this.input[this.pos:])
	if r == utf8.RuneError {

		return 0, size, this.raise(this.pos, "Invalid UTF-8 sequence")
	}
	if r <= 0xD7FF || r >= 0xDC00 {
		return r, size, nil
	}
	if this.pos+size >= len(this.input) {
		return r, size, nil
	}
	next, nextSize := utf8.DecodeRune(this.input[this.pos+size:])
	if next == utf8.RuneError {
		return r, size, nil
	}
	if next <= 0xDBFF || next >= 0xE000 {
		return r, size, nil
	}
	return (r<<10 + next - 0x35FDC00), size + nextSize, nil
}

func (this *Parser) readToken(code rune) {
	if IsIdentifierStart(code, this.options.ecmaVersion.(int) >= 6) || code == 92 {
		this.readWord()
		return
	}

	this.getTokenFromCode(code)
}

func (this *Parser) getTokenFromCode(code rune) error {
	switch code {
	case 46: // '.'
		this.readToken_dot()

	case 40: // '('
		this.pos++
		this.finishToken(tokenTypes[TOKEN_PARENL], nil)

	case 41: // ')'
		this.pos++
		this.finishToken(tokenTypes[TOKEN_PARENR], nil)

	case 59: // ';'
		this.pos++
		this.finishToken(tokenTypes[TOKEN_SEMI], nil)

	case 44: // ','
		this.pos++
		this.finishToken(tokenTypes[TOKEN_COMMA], nil)

	case 91: // '['
		this.pos++
		this.finishToken(tokenTypes[TOKEN_BRACKETL], nil)

	case 93: // ']'
		this.pos++
		this.finishToken(tokenTypes[TOKEN_BRACKETR], nil)

	case 123: // '{'
		this.pos++
		this.finishToken(tokenTypes[TOKEN_BRACEL], nil)

	case 125: // '}'
		this.pos++
		this.finishToken(tokenTypes[TOKEN_BRACER], nil)

	case 58: // ':'
		this.pos++
		this.finishToken(tokenTypes[TOKEN_COLON], nil)

	case 96: // '`'
		if this.options.ecmaVersion.(int) < 6 {
			break
		}
		this.pos++
		this.finishToken(tokenTypes[TOKEN_BACKQUOTE], nil)

	case 48: // '0'
		next := this.input[this.pos+1]
		if next == 120 || next == 88 { // 'x', 'X'
			return this.readRadixNumber(16) // hex number

		}
		if this.options.ecmaVersion.(int) >= 6 {
			if next == 111 || next == 79 { // 'o', 'O'
				return this.readRadixNumber(8) // octal number

			}
			if next == 98 || next == 66 { // 'b', 'B'
				return this.readRadixNumber(2) // binary number
			}
		}
		return this.readNumber(false)

	case 49, 50, 51, 52, 53, 54, 55, 56, 57: // '1'-'9'
		return this.readNumber(false)

	case 34, 39: // '"', "'"
		return this.readString(code)

	case 47: // '/'
		return this.readToken_slash()

	case 37, 42: // '%', '*'
		this.readToken_mult_modulo_exp(code)

	case 124, 38: // '|', '&'
		this.readToken_pipe_amp(code)

	case 94: // '^'
		this.readToken_caret()

	case 43, 45: // '+', '-'
		this.readToken_plus_min(code)

	case 60, 62: // '<', '>'
		this.readToken_lt_gt(code)

	case 61, 33: // '=', '!'
		this.readToken_eq_excl(code)

	case 63: // '?'
		this.readToken_question()

	case 126: // '~'
		this.finishOp(tokenTypes[TOKEN_PREFIX], 1)

	case 35: // '#'
		return this.readToken_numberSign()
	}
	return this.raise(this.pos, "Unexpected character '"+CodePointToString(code)+"'")
}

func (this *Parser) finishOp(token *TokenType, size int) {
	str := this.input[this.pos : this.pos+size]
	this.pos = this.pos + size
	this.finishToken(token, &str)
}

func (this *Parser) readToken_question() {
	ecmaVersion := this.options.ecmaVersion.(int)
	if ecmaVersion >= 11 {
		next := this.input[this.pos+1]
		if next == 46 {
			next2 := this.input[this.pos+2]
			if next2 < 48 || next2 > 57 {
				this.finishOp(tokenTypes[TOKEN_QUESTIONDOT], 2)
				return
			}
		}
		if next == 63 {
			if ecmaVersion >= 12 {
				next2 := this.input[this.pos+2]
				if next2 == 61 {
					this.finishOp(tokenTypes[TOKEN_ASSIGN], 3)
					return
				}
			}
			this.finishOp(tokenTypes[TOKEN_COALESCE], 2)
			return
		}
	}
	this.finishOp(tokenTypes[TOKEN_QUESTION], 1)
}

func (this *Parser) readToken_eq_excl(code rune) {
	next := this.input[this.pos+1]
	if next == 61 {

		if this.input[this.pos+2] == 62 {
			this.finishOp(tokenTypes[TOKEN_EQUALITY], 3)
		} else {
			this.finishOp(tokenTypes[TOKEN_EQUALITY], 2)
		}
		return
	}
	if code == 61 && next == 62 && this.options.ecmaVersion.(int) >= 6 { // '=>'
		this.pos += 2
		this.finishToken(tokenTypes[TOKEN_ARROW], nil)
		return
	}
	if code == 61 {
		this.finishOp(tokenTypes[TOKEN_EQUALITY], 1)
		return
	}

	this.finishOp(tokenTypes[TOKEN_PREFIX], 1)
}

func (this *Parser) readToken_lt_gt(code rune) {
	next := rune(this.input[this.pos+1])
	size := 1
	if next == code {
		if code == 62 && this.input[this.pos+2] == 62 {
			size = 3
		} else {
			size = 2
		}

		if this.input[this.pos+size] == 61 {
			this.finishOp(tokenTypes[TOKEN_ASSIGN], size+1)
			return
		}
		this.finishOp(tokenTypes[TOKEN_BITSHIFT], size)
		return
	}
	if next == 33 && code == 60 && !this.InModule && this.input[this.pos+2] == 45 &&
		this.input[this.pos+3] == 45 {
		// `<!--`, an XML-style comment that should be interpreted as a line comment
		this.skipLineComment(4)
		this.skipSpace()
		this.nextToken()
		return
	}
	if next == 61 {
		size = 2
	}
	this.finishOp(tokenTypes[TOKEN_RELATIONAL], size)
}

func (this *Parser) readToken_plus_min(code rune) {
	next := rune(this.input[this.pos+1])
	if next == code {
		if next == 45 && !this.InModule && this.input[this.pos+2] == 62 &&
			(this.LastTokEnd == 0 || lineBreak.Match([]byte(this.input[this.LastTokEnd:this.pos]))) {
			// A `-->` line comment
			this.skipLineComment(3)
			this.skipSpace()
			this.nextToken()
			return
		}
		this.finishOp(tokenTypes[TOKEN_INCDEC], 2)
		return
	}
	if next == 61 {
		this.finishOp(tokenTypes[TOKEN_ASSIGN], 2)
		return
	}
	this.finishOp(tokenTypes[TOKEN_PLUSMIN], 1)
}

func (this *Parser) skipLineComment(startSkip int) {
	ch := this.input[this.pos+startSkip]
	this.pos = this.pos + startSkip
	for this.pos < len(this.input) && !isNewLine(rune(ch)) {
		this.pos = this.pos + 1
		ch = this.input[this.pos]
	}

	if this.options.OnComment != nil {
		// TODO I don't really have onComment ported and might be that it never happens
		/*
			this.Options.OnComment.(false, this.input.slice(start+startSkip, this.pos), start, this.pos,
				startLoc, this.curPosition())
		*/
	}
}

func (this *Parser) readToken_caret() {
	next := this.input[this.pos+1]
	if next == 61 {
		this.finishOp(tokenTypes[TOKEN_ASSIGN], 2)
		return
	}
	this.finishOp(tokenTypes[TOKEN_BITWISEXOR], 1)
}

func (this *Parser) readToken_pipe_amp(code rune) {
	next := rune(this.input[this.pos+1])
	if next == code {
		if this.options.ecmaVersion.(int) >= 12 {
			next2 := this.input[this.pos+2]
			if next2 == 61 {
				this.finishOp(tokenTypes[TOKEN_ASSIGN], 3)
				return
			}

			if code == 124 {
				this.finishOp(tokenTypes[TOKEN_LOGICALOR], 2)
				return
			} else {
				this.finishOp(tokenTypes[TOKEN_LOGICALAND], 2)
				return
			}
		}
	}

	if next == 61 {
		this.finishOp(tokenTypes[TOKEN_ASSIGN], 2)
		return
	}

	if code == 124 {
		this.finishOp(tokenTypes[TOKEN_BITWISEOR], 1)
		return
	}

	this.finishOp(tokenTypes[TOKEN_BITWISEAND], 1)
}

func (this *Parser) readToken_mult_modulo_exp(code rune) {
	next := this.input[this.pos+1]
	size := 1

	var tokenType *TokenType

	if code == 42 {
		tokenType = tokenTypes[TOKEN_STAR]
	} else {
		tokenType = tokenTypes[TOKEN_MODULO]
	}

	// exponentiation operator ** and **=
	if this.options.ecmaVersion.(int) >= 7 && code == 42 && next == 42 {
		size = size + 1
		tokenType = tokenTypes[TOKEN_STAR]
		next = this.input[this.pos+2]
	}

	if next == 61 {
		this.finishOp(tokenTypes[TOKEN_ASSIGN], size+1)
		return
	}

	this.finishOp(tokenType, size)
}

func (this *Parser) readToken_slash() error {
	next := this.input[this.pos+1]
	if this.ExprAllowed {
		this.pos++
		return this.readRegexp()
	}
	if next == 61 {
		this.finishOp(tokenTypes[TOKEN_ASSIGN], 2)
		return nil
	}
	this.finishOp(tokenTypes[TOKEN_SLASH], 1)
	return nil
}

func (this *Parser) readRegexp() error {
	escaped, inClass, start := this.pos == 0, this.pos == 0, this.pos
	for {
		if this.pos >= len(this.input) {
			return this.raise(start, "Unterminated regular expression")

		}
		ch := this.input[this.pos]
		if lineBreak.Match([]byte{ch}) {
			return this.raise(start, "Unterminated regular expression")
		}

		if !escaped {
			if ch == '[' {
				inClass = true
			} else if ch == ']' && inClass {
				inClass = false
			} else if ch == '/' && !inClass {
				break
			}
			escaped = ch == '\\'
		} else {
			escaped = false
		}

		this.pos = this.pos + 1
	}

	pattern := this.input[start:this.pos]
	this.pos = this.pos + 1
	flagsStart := this.pos
	flags, err := this.readWord1()
	if err != nil {
		return this.raise(this.pos, "Failed to read regExp flags")
	}
	if this.ContainsEsc {
		return this.unexpected(&flagsStart)
	}

	// Validate pattern
	var state *RegExpState
	if this.RegexpState != nil {
		state = this.RegexpState
	} else {
		this.RegexpState = this.NewRegExpState()
		state = this.RegexpState
	}

	state.reset(start, string(pattern), flags)
	this.validateRegExpFlags(state)
	this.validateRegExpPattern(state)

	// Create Literal#value property value.

	value := &regexp.Regexp{} // new RegExp(pattern, flags)

	this.finishToken(tokenTypes[TOKEN_REGEXP], struct {
		pattern string
		flags   string
		value   *regexp.Regexp
	}{
		pattern: string(pattern),
		flags:   flags,
		value:   value,
	})
	return nil
}

func (this *Parser) validateRegExpPattern(state *RegExpState) {
	panic("unimplemented")
}

func (this *Parser) validateRegExpFlags(state *RegExpState) {
	panic("unimplemented")
}

func (this *Parser) readString(quote rune) error {
	this.pos = this.pos + 1
	out, chunkStart := []byte{}, this.pos
	for {
		if this.pos >= len(this.input) {
			return this.raise(this.Start, "Unterminated string constant")
		}
		ch, size, _ := this.fullCharCodeAtPos()
		if ch == quote {
			break
		}
		if ch == 92 { // '\'
			out = append(out, this.input[chunkStart:this.pos]...)
			escapedChar, _ := this.readEscapedChar(false)
			out = append(out, []byte(escapedChar)...)
			chunkStart = this.pos
		} else if ch == 0x2028 || ch == 0x2029 {
			if this.options.ecmaVersion.(int) < 10 {
				return this.raise(this.Start, "Unterminated string constant")

			}
			this.pos = this.pos + 1
			if this.options.Locations {
				this.CurLine++
				this.LineStart = this.pos
			}
		} else {
			if isNewLine(rune(ch)) {
				return this.raise(this.Start, "Unterminated string constant")
			}
			this.pos = this.pos + size
		}
	}
	out = append(out, this.input[chunkStart:this.pos]...)
	this.pos = this.pos + 1
	this.finishToken(tokenTypes[TOKEN_STRING], out)
	return nil
}

func (this *Parser) readNumber(startsWithDot bool) error {
	start := this.pos
	_, err := this.readInt(10, nil, true)
	if !startsWithDot && err != nil {
		return this.raise(start, "Invalid number")
	}
	octal := this.pos-start >= 2 && this.input[start] == 48
	if octal && this.Strict {
		return this.raise(start, "Invalid number")
	}
	next := this.input[this.pos]
	if !octal && !startsWithDot && this.options.ecmaVersion.(int) >= 11 && next == 110 {
		val := stringToBigInt(this.input[start:this.pos])
		this.pos = this.pos + 1
		ch, _, _ := this.fullCharCodeAtPos()
		if IsIdentifierStart(ch, false) {
			return this.raise(this.pos, "Identifier directly after number")

		}
		this.finishToken(tokenTypes[TOKEN_NUM], val)
		return nil
	}
	regExp := regexp.MustCompile("[89]")
	if octal && regExp.Match(this.input[start:this.pos]) {
		octal = false
	}
	if next == 46 && !octal { // '.'
		this.pos = this.pos + 1
		this.readInt(10, nil, false)
		next = this.input[this.pos]
	}
	if (next == 69 || next == 101) && !octal { // 'eE'
		this.pos = this.pos + 1
		next = this.input[this.pos]
		if next == 43 || next == 45 { // '+-'
			this.pos = this.pos + 1
		}

		_, err := this.readInt(10, nil, false)
		if err != nil {
			return this.raise(start, "Invalid number")
		}
	}
	ch, _, _ := this.fullCharCodeAtPos()
	if IsIdentifierStart(ch, false) {
		return this.raise(this.pos, "Identifier directly after number")

	}

	val := stringToNumber(this.input[start:this.pos], octal)
	this.finishToken(tokenTypes[TOKEN_NUM], val)
	return nil
}

func stringToNumber(b []byte, octal bool) float64 {
	/*
			This is missing and I don't have patience to do it
			  if (isLegacyOctalNumericLiteral) {
		    return parseInt(str, 8)
		  }
	*/

	numToConvert := strings.Replace(string(b), "_", "", -1)
	num, _ := strconv.ParseFloat(numToConvert, 64)
	return num
}
func stringToBigInt(b []byte) int {
	panic("unimplemented")
}

func (this *Parser) readRadixNumber(radix int) error {
	start := this.pos
	this.pos += 2 // 0x
	val, err := this.readInt(radix, nil, false)
	if err != nil {
		return this.raise(this.Start+2, string("Expected number in radix ")+strconv.Itoa(radix))
	}
	ch, _, _ := this.fullCharCodeAtPos()
	if this.options.ecmaVersion.(int) >= 11 && this.input[this.pos] == 110 {
		val = stringToBigInt(this.input[start:this.pos])
		this.pos = this.pos + 1
	} else if IsIdentifierStart(ch, false) {
		return this.raise(this.pos, "Identifier directly after number")
	}
	this.finishToken(tokenTypes[TOKEN_NUM], val)
	return nil
}

func (this *Parser) readToken_numberSign() error {
	ecmaVersion := this.options.ecmaVersion.(int)
	code := rune(35) // '#'
	if ecmaVersion >= 13 {
		this.pos = this.pos + 1
		quote, _, _ := this.fullCharCodeAtPos()
		if IsIdentifierStart(quote, true) || quote == 92 /* '\' */ {

			str, err := this.readWord1()
			if err != nil {
				return this.raise(this.pos, "Failed to read string")
			}
			this.finishToken(tokenTypes[TOKEN_PRIVATEID], str)
			return nil
		}
	}

	return this.raise(this.pos, "Unexpected character '"+CodePointToString(code)+"'")
}

func (this *Parser) tryReadTemplateToken() error {
	this.InTemplateElement = true

	err := this.readTmplToken()

	if err != nil {
		this.readInvalidTemplateToken()
	}

	this.InTemplateElement = false
	return err
}

func (this *Parser) readInvalidTemplateToken() error {
	for this.pos < len(this.input) {
		ch, size, err := this.fullCharCodeAtPos()
		if err != nil { // Error from fullCharCodeAtPos
			return this.raise(this.pos, "Invalid character in template: "+err.Error())

		}
		switch ch {
		case '\\':
			this.pos += size
		case '$':
			if this.pos+size < len(this.input) {
				next, _ := utf8.DecodeRune(this.input[this.pos+size:])
				if next == '{' {
					this.finishToken(tokenTypes[TOKEN_INVALIDTEMPLATE], this.input[this.Start:this.pos])
					return nil
				}
			}
			this.pos += size
		case '`':
			this.finishToken(tokenTypes[TOKEN_INVALIDTEMPLATE], this.input[this.Start:this.pos])
			return nil
		case '\r':
			this.pos += size
			if this.pos < len(this.input) {
				next, nextSize := utf8.DecodeRune(this.input[this.pos:])
				if next == '\n' {
					this.pos += nextSize
				}
			}
			this.CurLine++
			this.LineStart = this.pos
		case '\n', 0x2028, 0x2029:
			this.pos += size
			this.CurLine++
			this.LineStart = this.pos
		default:
			this.pos += size
		}
	}
	return this.raise(this.Start, "Unterminated template")
}

func (this *Parser) readTmplToken() error {
	out := []byte{}
	chunkStart := this.pos
	for {
		if this.pos >= len(this.input) {
			return this.raise(this.Start, "Unterminated template")
		}
		ch := this.input[this.pos]
		if ch == 96 || ch == 36 && this.input[this.pos+1] == 123 { // '`', '${'
			if this.pos == this.Start && this.Type.identifier == TOKEN_TEMPLATE || this.Type.identifier == TOKEN_INVALIDTEMPLATE {
				if ch == 36 {
					this.pos += 2
					this.finishToken(tokenTypes[TOKEN_DOLLARBRACEL], nil)
					return nil
				} else {
					this.pos = this.pos + 1
					this.finishToken(tokenTypes[TOKEN_BACKQUOTE], nil)
					return nil
				}
			}
			out = append(out, this.input[chunkStart:this.pos]...)

			this.finishToken(tokenTypes[TOKEN_TEMPLATE], out)
			return nil
		}

		if ch == 92 { // '\'
			out = append(out, this.input[chunkStart:this.pos]...)
			escaped, _ := this.readEscapedChar(true)
			out = append(out, []byte(escaped)...)
			chunkStart = this.pos
		} else if isNewLine(rune(ch)) {
			out = append(out, this.input[chunkStart:this.pos]...)
			this.pos = this.pos + 1
			switch ch {
			case 13:
				if this.input[this.pos] == 10 {
					this.pos = this.pos + 1
				}
			case 10:
				out = append(out, "\n"...)
			default:
				out = append(out, ch)
			}
			if this.options.Locations {
				this.CurLine = this.CurLine + 1
				this.LineStart = this.pos
			}
			chunkStart = this.pos
		} else {
			this.pos = this.pos + 1
		}
	}
}
func (this *Parser) unexpected(pos *int) error {
	if pos != nil {
		return this.raise(*pos, "Unexpected token")
	} else {
		return this.raise(this.Start, "Unexpected token")
	}
}
func (this *Parser) raise(pos int, message string) error {
	loc := getLineInfo(this.input, pos)
	line := strconv.Itoa(loc.Line)
	column := strconv.Itoa(loc.Column)
	message += strings.Join([]string{" (", line, ":", column, ")"}, "")

	if this.SourceFile != nil {
		message += strings.Join([]string{" in ", *this.SourceFile}, "")
	}

	return errors.New(message)
}

func (this *Parser) raiseRecoverable(pos int, message string) error {
	return this.raise(pos, message)
}

func (this *Parser) readEscapedChar(inTemplate bool) (string, error) {
	if this.pos >= len(this.input) {
		return "", this.invalidStringToken(this.pos, "Unexpected end of input after backslash")
	}
	this.pos++ // Skip backslash
	r, size := utf8.DecodeRune(this.input[this.pos:])
	if r == utf8.RuneError {

		return "", this.invalidStringToken(this.pos, "Invalid UTF-8 sequence")
	}
	this.pos += size
	ch := int(r)

	switch ch {
	case 'n':
		return "\n", nil
	case 'r':
		return "\r", nil
	case 'x':
		hexCh, err := this.readHexChar(2)
		return string(hexCh), err
	case 'u':
		code, err := this.readCodePoint()
		return CodePointToString(code), err
	case 't':
		return "\t", nil
	case 'b':
		return "\b", nil
	case 'v':
		return "\u000b", nil
	case 'f':
		return "\f", nil
	case '\r':
		if this.pos < len(this.input) && this.input[this.pos] == '\n' {
			this.pos++
		}
		fallthrough
	case '\n':
		if this.options.Locations {
			this.LineStart = this.pos
			this.CurLine++
		}
		return "", nil
	case '8', '9':
		if this.Strict {
			return "", this.invalidStringToken(this.pos-1, "Invalid escape sequence")
		}
		if inTemplate {
			return "", this.invalidStringToken(this.pos-1, "Invalid escape sequence in template string")
		}
		return string(rune(ch)), nil
	default:
		if ch >= '0' && ch <= '7' {
			// Octal escape: read up to 3 digits
			startPos := this.pos - size
			octalStr := string(rune(ch))
			for i := 0; i < 2 && this.pos < len(this.input); i++ {
				nextCh, nextSize := utf8.DecodeRune(this.input[this.pos:])
				if nextCh < '0' || nextCh > '7' {
					break
				}
				octalStr += string(nextCh)
				this.pos += nextSize
			}
			octal, err := strconv.ParseInt(octalStr, 8, 64)
			if err != nil {

				return "", this.invalidStringToken(startPos, "Invalid octal escape sequence")
			}
			if octal > 255 {
				octalStr = octalStr[:len(octalStr)-1]
				octal, _ = strconv.ParseInt(octalStr, 8, 64)
				this.pos -= size // Rewind last character
			}
			// Check for invalid octal escapes
			var nextCh rune
			if this.pos < len(this.input) {
				nextCh, _ = utf8.DecodeRune(this.input[this.pos:])
			}
			if (octalStr != "0" || nextCh == '8' || nextCh == '9') && (this.Strict || inTemplate) {
				msg := "Octal literal in strict mode"
				if inTemplate {
					msg = "Octal literal in template string"
				}

				return "", this.invalidStringToken(startPos, msg)
			}
			return string(rune(octal)), nil
		}
		if isNewLine(rune(ch)) {
			if this.options.Locations {
				this.LineStart = this.pos
				this.CurLine++
			}
			return "", nil
		}
		return string(rune(ch)), nil
	}
}

func (this *Parser) readWord() error {
	word, err := this.readWord1()
	if err != nil {
		return this.raise(this.pos, "We have failed")
	}
	t := tokenTypes[TOKEN_NAME]

	if tt, found := keywords[word]; found {
		t = tt
	}

	this.finishToken(t, word)
	return nil
}

func (this *Parser) readWord1() (string, error) {
	this.ContainsEsc = false
	word, first, chunkStart := []byte{}, true, this.pos

	astral := this.options.ecmaVersion.(int) >= 6

	for this.pos < len(this.input) {
		ch, size, _ := this.fullCharCodeAtPos()
		if IsIdentifierChar(ch, astral) {
			if ch <= 0xffff {
				this.pos = this.pos + size
			} else {
				this.pos = this.pos + size
			}
		} else if ch == 92 { // "\"
			this.ContainsEsc = true
			word = this.input[chunkStart:this.pos]
			escStart := this.pos
			this.pos = this.pos + size
			if this.input[this.pos] != 117 { // "u"

				return "", this.invalidStringToken(this.pos, "Expecting Unicode escape sequence \\uXXXX")
			}

			this.pos = this.pos + 1
			esc, _ := this.readCodePoint()

			if first {
				if !IsIdentifierStart(rune(esc), astral) {

					return "", this.invalidStringToken(escStart, "Invalid Unicode escape")
				}
			} else {
				if !IsIdentifierChar(rune(esc), astral) {

					return "", this.invalidStringToken(escStart, "Invalid Unicode escape")
				}
			}

			word = append(word, CodePointToString(esc)...)
			chunkStart = this.pos
		} else {
			break
		}
		first = false
	}
	return string(append(word, this.input[chunkStart:this.pos]...)), nil
}

func (this *Parser) invalidStringToken(pos int, message string) error {
	if this.InTemplateElement && this.options.ecmaVersion.(int) >= 9 {
		return this.raise(pos, "Invalid template literal")
	} else {
		return this.raise(pos, message)
	}
}

func (this *Parser) readCodePoint() (rune, error) {
	ch := this.input[this.pos]
	code := rune(0)

	if ch == 123 { // '{'
		if this.options.ecmaVersion.(int) < 6 {
			return 0, this.unexpected(nil)
		}
		codePos := this.pos + 1
		this.pos = this.pos + 1
		hexCh, _ := this.readHexChar(len(this.input[this.pos:]) + strings.Index(string(this.input[this.pos:]), "}") - this.pos)
		code = hexCh
		this.pos = this.pos + 1
		if code > 0x10FFFF {
			return 0, this.invalidStringToken(codePos, "Code point out of bounds")
		}
	} else {
		hexCh, _ := this.readHexChar(4)
		code = hexCh
	}
	return code, nil

}

func (this *Parser) readHexChar(len int) (rune, error) {
	codePos := this.pos
	n, err := this.readInt(16, &len, false)
	if err != nil {
		return 0, this.invalidStringToken(codePos, "Bad character escape sequence")
	}
	return rune(n), nil
}

func (this *Parser) readInt(radix int, len *int, maybeLegacyOctalNumericLiteral bool) (int, error) {
	// `len` is used for character escape sequences. In that case, disallow separators.
	allowSeparators := this.options.ecmaVersion.(int) >= 12 && len == nil

	// `maybeLegacyOctalNumericLiteral` is true if it doesn't have prefix (0x,0o,0b)
	// and isn't fraction part nor exponent part. In that case, if the first digit
	// is zero then disallow separators.
	isLegacyOctalNumericLiteral := maybeLegacyOctalNumericLiteral && this.input[this.pos] == 48

	start, total, lastCode := this.pos, 0, 0
	e := 0

	if len == nil {
		e = int(math.Inf(1))
	} else {
		e = *len
	}

	for i := 0; i < e; i = i + 1 {
		code := int(this.input[this.pos])
		val := 0
		this.pos = this.pos + 1

		if allowSeparators && code == 95 {
			if isLegacyOctalNumericLiteral {
				return 0, this.raiseRecoverable(this.pos-1, "Numeric separator is not allowed in legacy octal numeric literals")
			}
			if lastCode == 95 {
				return 0, this.raiseRecoverable(this.pos-1, "Numeric separator must be exactly one underscore")
			}
			if i == 0 {
				return 0, this.raiseRecoverable(this.pos-1, "Numeric separator is not allowed at the first of digits")
			}
			lastCode = code
			continue
		}

		if code >= 97 { // a
			val = code - 97 + 10
		} else if code >= 65 { // A
			val = code - 65 + 10
		} else if code >= 48 && code <= 57 { // 0-9
			val = code - 48
		} else {
			val = int(math.Inf(1))
		}
		if val >= radix {
			break
		}
		lastCode = code
		total = total*radix + val
	}

	if allowSeparators && lastCode == 95 {
		return 0, this.raiseRecoverable(this.pos-1, "Numeric separator is not allowed at the last of digits")

	}
	if this.pos == start || len != nil && this.pos-start != *len {
		return 0, this.raiseRecoverable(this.pos-1, "Error ? I dont know")
	}

	return total, nil
}

func (this *Parser) readToken_dot() error {
	next := this.input[this.pos+1]
	if next >= 48 && next <= 57 {
		return this.readNumber(true)
	}

	next2 := this.input[this.pos+2]
	if this.options.ecmaVersion.(int) >= 6 && next == 46 && next2 == 46 { // 46 = dot '.'
		this.pos += 3
		this.finishToken(tokenTypes[TOKEN_ELLIPSIS], nil)
		return nil
	}
	this.pos++
	this.finishToken(tokenTypes[TOKEN_DOT], nil)
	return nil

}

func (this *Parser) finishToken(tokenType *TokenType, value any) {
	this.End = this.pos
	if this.options.Locations {
		this.EndLoc = this.currentPosition()
	}
	prevType := tokenType
	this.Type = tokenType
	this.Value = value
	this.updateContext(prevType)
}

func (this *Parser) currentPosition() *Location {
	return &Location{Line: this.CurLine, Column: this.pos - this.LineStart}
}

func (this *Parser) skipSpace() error {
Loop:
	for this.pos < len(this.input) {
		ch, size, _ := this.fullCharCodeAtPos()
		switch ch {
		case 32:
		case 160: // ' '
			this.pos = this.pos + size
			break
		case 13:
			if this.input[this.pos+size] == 10 {
				this.pos = this.pos + size
			}
		case 10:
		case 8232:
		case 8233:
			this.pos = this.pos + size
			if this.options.Locations {
				this.CurLine = this.CurLine + 1
				this.LineStart = this.pos
			}
		case 47: // '/'
			switch this.input[this.pos+1] {
			case 42: // '*'
				return this.skipBlockComment()
			case 47:
				this.skipLineComment(2)
			default:
				break Loop
			}
		default:
			if ch > 8 && ch < 14 || ch >= 5760 && nonASCIIwhitespace.Match(utf8.AppendRune([]byte{}, ch)) {
				this.pos = this.pos + size
			} else {
				break Loop
			}
		}
	}
	return nil
}

func (this *Parser) skipBlockComment() error {
	start := this.pos
	this.pos += 2 // Skip "/*"
	end := bytes.Index(this.input[this.pos:], []byte("*/"))
	if end == -1 {
		return this.raise(start, "Unterminated comment")
	}
	this.pos += end + 2 // Move past "*/"
	return nil
}

// #### SCOPE RELATED CODE

func (this *Parser) braceIsBlock(prevType Token) bool {
	parent := this.currentContext().Identifier
	isExpr := this.currentContext().IsExpr

	if parent == FUNCTION_EXPRESSION || parent == FUNCTION_STATEMENT {
		return true
	}

	if prevType == TOKEN_COLON && (parent == BRACKET_STATEMENT || parent == BRACKET_EXPRESSION) {
		return !isExpr
	}

	if prevType == TOKEN_RETURN || prevType == TOKEN_NAME && this.ExprAllowed {
		// return lineBreak.test(this.input.slice(this.lastTokEnd, this.start))
	}

	if prevType == TOKEN_ELSE || prevType == TOKEN_SEMI || prevType == TOKEN_EOF || prevType == TOKEN_PARENR || prevType == TOKEN_ARROW {

		return true
	}
	if prevType == TOKEN_BRACEL {
		return parent == BRACKET_STATEMENT
	}
	if prevType == TOKEN_VAR || prevType == TOKEN_CONST || prevType == TOKEN_NAME {

		return false
	}

	return !this.ExprAllowed
}

func (this *Parser) enterScope(flags Flags) {
	this.ScopeStack = append(this.ScopeStack, NewScope(flags))
}

func (this *Parser) exitScope() {
	this.ScopeStack = this.ScopeStack[:len(this.ScopeStack)-1]
}

func (this *Parser) currentScope() *Scope {
	return this.ScopeStack[len(this.ScopeStack)-1]
}

func (this *Parser) treatFunctionsAsVar() bool {
	return this.treatFunctionsAsVarInScope(this.currentScope())
}

func (this *Parser) treatFunctionsAsVarInScope(scope *Scope) bool {
	return (scope.Flags&SCOPE_FUNCTION != 0) || (!this.InModule && scope.Flags&SCOPE_TOP != 0)
}

func (this *Parser) declareName(name string, bindingType Flags, pos Location) {
	redeclared := false

	scope := this.currentScope()
	if bindingType == BIND_LEXICAL {
		redeclared = slices.Contains(scope.Lexical, name) || slices.Contains(scope.Functions, name) || slices.Contains(scope.Var, name)
		scope.Lexical = append(scope.Lexical, name)
		if this.InModule && (scope.Flags&SCOPE_TOP != 0) {
			delete(this.UndefinedExports, name)
		}
	} else if bindingType == BIND_SIMPLE_CATCH {
		scope.Lexical = append(scope.Lexical, name)
	} else if bindingType == BIND_FUNCTION {
		if this.treatFunctionsAsVar() {
			redeclared = slices.Contains(scope.Lexical, name)
		} else {
			redeclared = slices.Contains(scope.Lexical, name) || slices.Contains(scope.Var, name)
		}
		scope.Functions = append(scope.Functions, name)
	} else {
		for _, scope := range this.ScopeStack {
			if slices.Contains(scope.Lexical, name) && !((scope.Flags&SCOPE_SIMPLE_CATCH != 0) && scope.Lexical[0] == name) || !this.treatFunctionsAsVarInScope(scope) && slices.Contains(scope.Functions, name) {
				redeclared = true
				break
			}

			scope.Var = append(scope.Var, name)
			if this.InModule && (scope.Flags&SCOPE_TOP != 0) {
				delete(this.UndefinedExports, name)
			}

			if scope.Flags&SCOPE_VAR != 0 {
				break
			}
		}
	}

	if redeclared {
		// this.raiseRecoverable(pos, `Identifier '${name}' has already been declared`)
	}
}

// #### NODE RELATED CODE

func (this *Parser) startNode() *Node {
	return NewNode(this, this.Start, this.StartLoc)
}

func (this *Parser) startNodeAt(pos int, loc *Location) *Node {
	return NewNode(this, pos, loc)
}

func (this *Parser) finishNodeAt(node *Node, finishType NodeType, pos int, loc *Location) {
	node.Type = finishType
	node.End = pos
	if this.options.Locations {
		node.Loc.End = loc
	}

	if this.options.Ranges {
		node.Range[1] = pos
	}
}

func (this *Parser) finishNode(node *Node, finishType NodeType) {
	this.finishNodeAt(node, finishType, this.LastTokEnd, this.LastTokEndLoc)
}

/*
I think I can skip this?

	this.finishNodeAt = function(node, type, pos, loc) {
	  return finishNodeAt.call(this, node, type, pos, loc)
	}

TODO ->

	this.copyNode = function(node) {
	  let newNode = new Node(this, node.start, this.startLoc)
	  for (let prop in node) newNode[prop] = node[prop]
	  return newNode
	}
*/

// #### CONTEXT RELATED CODE

func (this *Parser) initialContext() []*TokenContext {
	return []*TokenContext{TokenContexts[BRACKET_STATEMENT]}
}

func (this *Parser) currentContext() *TokenContext {
	return this.Context[len(this.Context)-1]
}

func (this *Parser) inGeneratorContext() bool {
	for i := len(this.Context); i >= 1; i-- {
		context := this.Context[i]
		if context.Token == "function" {
			return context.Generator
		}
	}
	return false
}

func (this *Parser) updateContext(prevType *TokenType) {
	update, current := this.Type, this.Type
	if len(current.keyword) != 0 && prevType.identifier == TOKEN_DOT {
		this.ExprAllowed = false
	} else if current.updateContext != nil {
		update.updateContext = current.updateContext
		update.updateContext.updateContext(prevType)
	} else {
		this.ExprAllowed = current.beforeExpr
	}
}

func (this *Parser) overrideContext(tokenCtx *TokenContext) {
	if this.currentContext().Identifier != tokenCtx.Identifier {
		this.Context[len(this.Context)-1] = tokenCtx
	}
}

func (this *Parser) initAllUpdateContext() {
	tokenTypes[TOKEN_PARENR].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if len(this.Context) == 1 {
			this.ExprAllowed = true
			return
		}

		out := this.Context[len(this.Context)-1]
		this.Context = this.Context[:len(this.Context)-1]
		if out.Identifier == BRACKET_STATEMENT && this.currentContext().Token == "function" {
			out = this.Context[len(this.Context)-1]
			this.Context = this.Context[:len(this.Context)-1]
		}
		this.ExprAllowed = !out.IsExpr
	}}

	tokenTypes[TOKEN_BRACER].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if len(this.Context) == 1 {
			this.ExprAllowed = true
			return
		}

		out := this.Context[len(this.Context)-1]
		this.Context = this.Context[:len(this.Context)-1]
		if out.Identifier == BRACKET_STATEMENT && this.currentContext().Token == "function" {
			out = this.Context[len(this.Context)-1]
			this.Context = this.Context[:len(this.Context)-1]
		}
		this.ExprAllowed = !out.IsExpr
	}}

	tokenTypes[TOKEN_BRACEL].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if this.braceIsBlock(token.identifier) {
			this.Context = append(this.Context, TokenContexts[BRACKET_STATEMENT])
		} else {
			this.Context = append(this.Context, TokenContexts[BRACKET_EXPRESSION])
		}
		this.ExprAllowed = true

	}}

	tokenTypes[TOKEN_DOLLARBRACEL].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		this.Context = append(this.Context, TokenContexts[BRACKET_TEMPLATE])
		this.ExprAllowed = true
	}}

	tokenTypes[TOKEN_PARENL].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		statementParens := token.identifier == TOKEN_IF || token.identifier == TOKEN_FOR || token.identifier == TOKEN_WITH || token.identifier == TOKEN_WHILE

		if statementParens {

			this.Context = append(this.Context, TokenContexts[PAREN_STATEMENT])
		} else {
			this.Context = append(this.Context, TokenContexts[PAREN_EXPRESSION])
		}
		this.ExprAllowed = true
	}}

	tokenTypes[TOKEN_INCDEC].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		// no factor
	}}

	tokenTypes[TOKEN_FUNCTION].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		prevType := token.identifier

		if token.beforeExpr && prevType == TOKEN_ELSE && !(prevType == TOKEN_SEMI && this.currentContext().Identifier == PAREN_STATEMENT) && !(prevType == TOKEN_RETURN /*&& lineBreak.test(this.input.slice(this.lastTokEnd, this.start)))*/) && !((prevType == TOKEN_COLON || prevType == TOKEN_BRACEL) && this.currentContext().Identifier == BRACKET_STATEMENT) {
			this.Context = append(this.Context, TokenContexts[FUNCTION_EXPRESSION])
		} else {
			this.Context = append(this.Context, TokenContexts[FUNCTION_STATEMENT])
		}
	}}

	tokenTypes[TOKEN_COLON].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if this.currentContext().Token == "function" {
			this.Context = this.Context[:len(this.Context)-1]
		}
		this.ExprAllowed = true
	}}

	tokenTypes[TOKEN_BACKQUOTE].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if this.currentContext().Identifier == QUOTE_TEMPLATE {
			this.Context = this.Context[:len(this.Context)-1]
		} else {
			this.Context = append(this.Context, TokenContexts[QUOTE_TEMPLATE])
		}
	}}

	tokenTypes[TOKEN_STAR].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		if token.identifier == TOKEN_FUNCTION {
			idx := len(this.Context) - 1

			if this.Context[idx].Identifier == FUNCTION_EXPRESSION {
				this.Context[idx] = TokenContexts[FUNCTION_EXPRESSION_GENERATOR]
			} else {
				this.Context[idx] = TokenContexts[FUNCTION_GENERATOR]
			}
			this.ExprAllowed = true
		}
	}}

	tokenTypes[TOKEN_NAME].updateContext = &UpdateContext{updateContext: func(token *TokenType) {
		allowed := false

		if this.options.ecmaVersion.(int) >= 6 && token.identifier != TOKEN_DOT {
			if this.Value == "of" && !this.ExprAllowed || this.Value == "yield" || this.inGeneratorContext() {
				allowed = true
			}
		}
		this.ExprAllowed = allowed
	}}
}

var Pp = &Parser{}
