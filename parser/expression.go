package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// EXPRESSION PARSING

func (this *Parser) checkPropClash(prop *Node, propHash *PropertyHash, refDestructuringErrors *DestructuringErrors) error {
	if this.getEcmaVersion() >= 9 && prop.Type == NODE_SPREAD_ELEMENT {
		return nil
	}

	if this.getEcmaVersion() >= 6 && (prop.Computed || prop.IsMethod || prop.Shorthand) {
		return nil
	}

	key := prop.Key
	var name string

	switch key.Type {
	case NODE_IDENTIFIER:
		name = key.Name
	case NODE_LITERAL:
		if val, ok := key.Value.(string); ok {
			name = val
		} else {
			panic("Node was incorrectly typed expected string value from NODE_LITERAL")
		}
	default:
		return nil
	}

	kind := prop.Kind

	if this.getEcmaVersion() >= 6 {
		if name == "__proto__" && kind == KIND_PROPERTY_INIT {
			if propHash.proto {
				if refDestructuringErrors != nil {
					if refDestructuringErrors.doubleProto < 0 {
						refDestructuringErrors.doubleProto = key.Start
					}
				} else {
					return this.raiseRecoverable(key.Start, "Redefinition of __proto__ property")
				}
			}
			propHash.proto = true
		}
		return nil
	}

	name = "$" + name
	if other, found := propHash.m[name]; found {
		redefinition := false
		if kind == KIND_PROPERTY_INIT {
			redefinition = this.Strict && other[KIND_PROPERTY_INIT] || other[KIND_PROPERTY_GET] || other[KIND_PROPERTY_SET]
		} else {
			redefinition = other[KIND_PROPERTY_INIT] || other[kind]
		}
		if redefinition {
			this.raiseRecoverable(key.Start, "Redefinition of property")
		}
	} else {
		newInfo := map[Kind]bool{
			KIND_PROPERTY_INIT: false,
			KIND_PROPERTY_GET:  false,
			KIND_PROPERTY_SET:  false,
		}
		newInfo[kind] = true
		propHash.m[name] = newInfo
	}

	return nil
}

func (this *Parser) parseExpression(forInit string, refDestructuringErrors *DestructuringErrors) (*Node, error) {
	startPos, startLoc := this.start, this.startLoc
	expr, err := this.parseMaybeAssign(forInit, refDestructuringErrors, nil)

	if err != nil {
		return nil, err
	}
	if this.Type.identifier == TOKEN_COMMA {
		node := this.startNodeAt(startPos, startLoc)
		node.Expressions = []*Node{expr}

		for this.eat(TOKEN_COMMA) {
			maybeAssign, err := this.parseMaybeAssign(forInit, refDestructuringErrors, nil)
			if err != nil {
				return nil, err
			}
			node.Expressions = append(node.Expressions, maybeAssign)
		}

		return this.finishNode(node, NODE_SEQUENCE_EXPRESSION), nil
	}
	return expr, nil
}

func (this *Parser) parseMaybeAssign(forInit string, refDestructuringErrors *DestructuringErrors, afterLeftParse *struct {
	call func(p *Parser, l *Node, s int, sl *Location) (*Node, error)
}) (*Node, error) {
	if this.isContextual("yield") {
		if this.inGenerator() {
			yield, err := this.parseYield(forInit)
			if err != nil {
				return nil, err
			}
			return yield, nil
		} else {
			// The tokenizer will assume an expression is allowed after
			// `yield`, but this isn't that kind of yield
			this.ExprAllowed = false
		}
	}

	ownDestructuringErrors, oldParenAssign, oldTrailingComma, oldDoubleProto := false, -1, -1, -1
	if refDestructuringErrors != nil {
		oldParenAssign = refDestructuringErrors.parenthesizedAssign
		oldTrailingComma = refDestructuringErrors.trailingComma
		oldDoubleProto = refDestructuringErrors.doubleProto
		refDestructuringErrors.parenthesizedAssign = -1
		refDestructuringErrors.trailingComma = -1
	} else {
		refDestructuringErrors = NewDestructuringErrors()
		ownDestructuringErrors = true
	}

	startPos, startLoc := this.start, this.startLoc

	if this.Type.identifier == TOKEN_PARENL || this.Type.identifier == TOKEN_NAME {
		this.PotentialArrowAt = this.start
		this.PotentialArrowInForAwait = forInit == "await"
	}
	left, err := this.parseMaybeConditional(forInit, refDestructuringErrors)

	if err != nil {
		return nil, err
	}

	if afterLeftParse != nil {
		newLeft, err := afterLeftParse.call(this, left, startPos, startLoc)
		if err != nil {
			return nil, err
		}
		left = newLeft
	}

	if this.Type.isAssign {
		node := this.startNodeAt(startPos, startLoc)
		var op AssignmentOperator

		if byteSlice, ok := this.Value.([]byte); ok {
			op = AssignmentOperator(byteSlice)
		} else {
			return nil, fmt.Errorf("invalid this.Value expected []byte, got: %q", this.Value)
		}
		node.AssignmentOperator = op
		if this.Type.identifier == TOKEN_EQ {
			left, err = this.toAssignable(left, false, refDestructuringErrors)

			if err != nil {
				return nil, err
			}
		}

		if !ownDestructuringErrors {
			refDestructuringErrors.parenthesizedAssign = -1
			refDestructuringErrors.trailingComma = -1
			refDestructuringErrors.doubleProto = -1
		}
		if refDestructuringErrors.shorthandAssign >= left.Start {
			refDestructuringErrors.shorthandAssign = -1 // reset because shorthand default was used correctly
		}

		if this.Type.identifier == TOKEN_EQ {
			this.checkLValPattern(left, 0, struct {
				check bool
				hash  map[string]bool
			}{check: false, hash: map[string]bool{}})
		} else {
			this.checkLValSimple(left, 0, struct {
				check bool
				hash  map[string]bool
			}{check: false, hash: map[string]bool{}})
		}

		node.Left = left
		this.next(false)
		right, err := this.parseMaybeAssign(forInit, refDestructuringErrors, nil)

		if err != nil {
			return nil, err
		}
		node.Right = right

		if oldDoubleProto > -1 {
			refDestructuringErrors.doubleProto = oldDoubleProto

		}
		return this.finishNode(node, NODE_ASSIGNMENT_EXPRESSION), nil
	} else {
		if ownDestructuringErrors {
			_, err := this.checkExpressionErrors(refDestructuringErrors, true)
			if err != nil {
				return nil, err
			}
		}
	}
	if oldParenAssign > -1 {
		refDestructuringErrors.parenthesizedAssign = oldParenAssign
	}
	if oldTrailingComma > -1 {
		refDestructuringErrors.trailingComma = oldTrailingComma
	}
	return left, nil
}

func (this *Parser) parseMaybeConditional(forInit string, refDestructuringErrors *DestructuringErrors) (*Node, error) {
	startPos, startLoc := this.start, this.startLoc
	expr, err := this.parseExprOps(forInit, refDestructuringErrors)

	if err != nil {
		return nil, err
	}

	exprError, _ := this.checkExpressionErrors(refDestructuringErrors, false)
	if exprError {
		return expr, nil
	}
	if this.eat(TOKEN_QUESTION) {
		node := this.startNodeAt(startPos, startLoc)
		node.Test = expr
		maybeAssign, err := this.parseMaybeAssign("", nil, nil)
		if err != nil {
			return nil, err
		}
		node.Consequent = maybeAssign

		errExpect := this.expect(TOKEN_COLON)
		if errExpect != nil {
			return nil, err
		}

		maybeAssignElse, errElse := this.parseMaybeAssign(forInit, nil, nil)
		if errElse != nil {
			return nil, errElse
		}
		node.Alternate = maybeAssignElse
		return this.finishNode(node, NODE_CONDITIONAL_EXPRESSION), nil
	}
	return expr, nil
}

func (this *Parser) checkExpressionErrors(refDestructuringErrors *DestructuringErrors, andThrow bool) (bool, error) {
	if refDestructuringErrors == nil {
		return false, nil
	}
	shorthandAssign, doubleProto := refDestructuringErrors.shorthandAssign, refDestructuringErrors.doubleProto
	if !andThrow {
		return shorthandAssign >= 0 || doubleProto >= 0, nil
	}

	if shorthandAssign >= 0 {
		return true, this.raise(shorthandAssign, "Shorthand property assignments are valid only in destructuring patterns")
	}

	if doubleProto >= 0 {
		return true, this.raiseRecoverable(doubleProto, "Redefinition of __proto__ property")
	}
	return false, nil
}

func (this *Parser) parseSubscripts(base *Node, startPos int, startLoc *Location, noCalls bool, forInit string) (*Node, error) {
	maybeAsyncArrow, optionalChained := this.getEcmaVersion() >= 8 && base.Type == NODE_IDENTIFIER && base.Name == "async" &&
		this.LastTokEnd == base.End && !this.canInsertSemicolon() && base.End-base.Start == 5 &&
		this.PotentialArrowAt == base.Start, false

	for {
		element, err := this.parseSubscript(base, startPos, startLoc, noCalls, maybeAsyncArrow, optionalChained, forInit)

		if err != nil {
			return nil, err
		}

		if element.Optional {
			optionalChained = true
		}

		if element == base || element.Type == NODE_ARROW_FUNCTION_EXPRESSION {
			if optionalChained {
				chainNode := this.startNodeAt(startPos, startLoc)
				chainNode.Expression = element
				element = this.finishNode(chainNode, NODE_CHAIN_EXPRESSION)
			}
			return element, nil
		}
		base = element
	}
}

func (this *Parser) parseSubscript(base *Node, startPos int, startLoc *Location, noCalls bool, maybeAsyncArrow bool, optionalChained bool, forInit string) (*Node, error) {
	optionalSupported := this.getEcmaVersion() >= 11
	optional := optionalSupported && this.eat(TOKEN_QUESTIONDOT)

	if noCalls && optional {
		return nil, this.raise(this.LastTokStart, "Optional chaining cannot appear in the callee of new expressions")
	}

	computed := this.eat(TOKEN_BRACKETL)

	if computed || optional && this.Type.identifier != TOKEN_PARENL && this.Type.identifier != TOKEN_BACKQUOTE || this.eat(TOKEN_DOT) {
		node := this.startNodeAt(startPos, startLoc)
		node.Object = base
		if computed {
			prop, err := this.parseExpression("", nil)
			if err != nil {
				return nil, err
			}
			node.Property = prop
			err = this.expect(TOKEN_BRACKETR)

			if err != nil {
				return nil, err
			}
		} else if this.Type.identifier == TOKEN_PRIVATEID && base.Type != NODE_SUPER {
			privIdent, err := this.parsePrivateIdent()
			if err != nil {
				return nil, err
			}
			node.Property = privIdent
		} else {
			ident, err := this.parseIdent(this.options.AllowReserved != ALLOW_RESERVED_NEVER)
			if err != nil {
				return nil, err
			}
			node.Property = ident
		}
		node.Computed = computed
		if optionalSupported {
			node.Optional = optional
		}
		base = this.finishNode(node, NODE_MEMBER_EXPRESSION)
	} else if !noCalls && this.eat(TOKEN_PARENL) {
		refDestructuringErrors, oldYieldPos, oldAwaitPos, oldAwaitIdentPos := NewDestructuringErrors(), this.YieldPos, this.AwaitPos, this.AwaitIdentPos
		this.YieldPos = 0
		this.AwaitPos = 0
		this.AwaitIdentPos = 0
		exprList, err := this.parseExprList(TOKEN_PARENR, this.getEcmaVersion() >= 8, false, refDestructuringErrors)

		if err != nil {
			return nil, err
		}

		if maybeAsyncArrow && !optional && this.shouldParseAsyncArrow() {
			this.checkPatternErrors(refDestructuringErrors, false)
			this.checkYieldAwaitInDefaultParams()
			if this.AwaitIdentPos > 0 {
				return nil, this.raise(this.AwaitIdentPos, "Cannot use 'await' as identifier inside an async function")
			}

			this.YieldPos = oldYieldPos
			this.AwaitPos = oldAwaitPos
			this.AwaitIdentPos = oldAwaitIdentPos
			asyncArr, err := this.parseSubscriptAsyncArrow(startPos, startLoc, exprList, forInit)
			return asyncArr, err
		}

		_, err = this.checkExpressionErrors(refDestructuringErrors, true)

		if err != nil {
			return nil, err
		}

		if oldYieldPos != 0 {
			this.YieldPos = oldYieldPos
		}

		if oldAwaitPos != 0 {
			this.AwaitPos = oldAwaitPos
		}

		if oldAwaitIdentPos != 0 {
			this.AwaitIdentPos = oldAwaitIdentPos
		}
		node := this.startNodeAt(startPos, startLoc)
		node.Callee = base
		node.Arguments = exprList
		if optionalSupported {
			node.Optional = optional
		}
		base = this.finishNode(node, NODE_CALL_EXPRESSION)
	} else if this.Type.identifier == TOKEN_BACKQUOTE {
		if optional || optionalChained {
			return nil, this.raise(this.start, "Optional chaining cannot appear in the tag of tagged template expressions")
		}
		node := this.startNodeAt(startPos, startLoc)
		node.Tag = base
		tmpl, err := this.parseTemplate(struct{ isTagged bool }{isTagged: true})
		if err != nil {
			return nil, err
		}
		node.Quasi = tmpl
		base = this.finishNode(node, NODE_TAGGED_TEMPLATE_EXPRESSION)
	}
	return base, nil
}

func isLocalVariableAccess(node *Node) bool {
	return node.Type == NODE_IDENTIFIER ||
		node.Type == NODE_PARENTHESIZED_EXPRESSION && isLocalVariableAccess(node.Expression)
}

func (this *Parser) parseAwait(forInit string) (*Node, error) {
	if !(this.AwaitPos != 0) {
		this.AwaitPos = this.start
	}

	node := this.startNode()
	this.next(false)
	maybeUnary, err := this.parseMaybeUnary(nil, true, false, forInit)
	if err != nil {
		return nil, err
	}
	node.Argument = maybeUnary
	return this.finishNode(node, NODE_AWAIT_EXPRESSION), nil
}

func (this *Parser) parseExprSubscripts(refDestructuringErrors *DestructuringErrors, forInit string) (*Node, error) {
	startPos, startLoc := this.start, this.startLoc
	expr, err := this.parseExprAtom(refDestructuringErrors, forInit, false)

	if err != nil {
		return nil, err
	}
	if expr.Type == NODE_ARROW_FUNCTION_EXPRESSION && string(this.input[this.LastTokStart:this.LastTokEnd]) != ")" {
		return expr, nil

	}
	result, err := this.parseSubscripts(expr, startPos, startLoc, false, forInit)
	if err != nil {
		return nil, err
	}
	if refDestructuringErrors != nil && result.Type == NODE_MEMBER_EXPRESSION {
		if refDestructuringErrors.parenthesizedAssign >= result.Start {
			refDestructuringErrors.parenthesizedAssign = -1
		}
		if refDestructuringErrors.parenthesizedBind >= result.Start {
			refDestructuringErrors.parenthesizedBind = -1
		}
		if refDestructuringErrors.trailingComma >= result.Start {
			refDestructuringErrors.trailingComma = -1
		}
	}
	return result, nil
}

func (this *Parser) buildBinary(startPos int, startLoc *Location, left *Node, right *Node, op BinaryOperator, logical bool) (*Node, error) {
	if right.Type == NODE_PRIVATE_IDENTIFIER {
		return nil, this.raise(right.Start, "Private identifier can only be left side of binary expression")
	}
	node := this.startNodeAt(startPos, startLoc)
	node.Left = left
	node.BinaryOperator = op
	node.Right = right
	if logical {
		return this.finishNode(node, NODE_LOGICAL_EXPRESSION), nil
	}
	return this.finishNode(node, NODE_BINARY_EXPRESSION), nil
}

func (this *Parser) parseMaybeUnary(refDestructuringErrors *DestructuringErrors, sawUnary bool, incDec bool, forInit string) (*Node, error) {
	startPos, startLoc := this.start, this.startLoc
	var expr *Node
	var err error

	if this.isContextual("await") && this.canAwait() {
		expr, err = this.parseAwait(forInit)
		if err != nil {
			return nil, err
		}
		sawUnary = true
	} else if this.Type.prefix {
		node, update := this.startNode(), this.Type.identifier == TOKEN_INCDEC
		if uop, ok := this.Value.([]byte); ok {
			node.UnaryOperator = UnaryOperator(uop)
		} else {
			panic("this.Value was not []byte as expected")
		}

		node.Prefix = true
		this.next(false)
		maybeUnary, err := this.parseMaybeUnary(nil, true, update, forInit)
		if err != nil {
			return nil, err
		}

		node.Argument = maybeUnary
		_, err = this.checkExpressionErrors(refDestructuringErrors, true)

		if err != nil {
			return nil, err
		}

		if update {
			err := this.checkLValSimple(node.Argument, 0, struct {
				check bool
				hash  map[string]bool
			}{check: false, hash: map[string]bool{}})
			if err != nil {
				return nil, err
			}
		} else if this.Strict && node.UnaryOperator == UNARY_DELETE && isLocalVariableAccess(node.Argument) {
			return nil, this.raiseRecoverable(node.Start, "Deleting local variable in strict mode")
		} else if node.UnaryOperator == UNARY_DELETE && isPrivateFieldAccess(node.Argument) {
			return nil, this.raiseRecoverable(node.Start, "Private fields can not be deleted")
		} else {
			sawUnary = true
		}

		if update {
			expr = this.finishNode(node, NODE_UPDATE_EXPRESSION)
		} else {
			expr = this.finishNode(node, NODE_UNARY_EXPRESSION)
		}

	} else if !sawUnary && this.Type.identifier == TOKEN_PRIVATEID {
		if len(forInit) != 0 || len(this.PrivateNameStack) == 0 && this.options.CheckPrivateFields {
			return nil, this.unexpected(`len(forInit) != 0 || len(this.PrivateNameStack) == 0 && this.options.CheckPrivateFields`, &this.pos)
		}
		expr, err = this.parsePrivateIdent()
		if err != nil {
			return nil, err
		}
		// only could be private fields in 'in', such as #x in obj
		if this.Type.identifier != TOKEN_IN {
			return nil, this.unexpected("`only could be private fields in 'in', such as #x in obj` what?", &this.pos)
		}
	} else {
		expr, err = this.parseExprSubscripts(refDestructuringErrors, forInit)
		if err != nil {
			return nil, err
		}
		hasExprError, _ := this.checkExpressionErrors(refDestructuringErrors, false)
		if hasExprError {
			return expr, nil
		}

		for this.Type.postfix && !this.canInsertSemicolon() {
			node := this.startNodeAt(startPos, startLoc)
			if val, ok := this.Value.([]byte); ok {
				node.UpdateOperator = UpdateOperator(val)
			} else {
				panic("We expected []byte")
			}
			node.Prefix = false
			node.Argument = expr
			err := this.checkLValSimple(expr, 0, struct {
				check bool
				hash  map[string]bool
			}{check: false, hash: map[string]bool{}})

			if err != nil {
				return nil, err
			}
			this.next(false)
			expr = this.finishNode(node, NODE_UPDATE_EXPRESSION)
		}
	}

	if !incDec && this.eat(TOKEN_STARSTAR) {
		if sawUnary {

			return nil, this.unexpected("we saw unary, which is wrong?", &this.LastTokStart)
		} else {
			unary, err := this.parseMaybeUnary(nil, false, false, forInit)
			if err != nil {
				return nil, err
			}

			binOp, errBinop := this.buildBinary(startPos, startLoc, expr, unary, EXPONENTIATION, false)
			if errBinop != nil {
				return nil, errBinop
			}
			return binOp, nil
		}
	} else {
		return expr, nil
	}
}

func (this *Parser) parseExprOps(forInit string, refDestructuringErrors *DestructuringErrors) (*Node, error) {
	startPos, startLoc := this.start, this.startLoc
	expr, err := this.parseMaybeUnary(refDestructuringErrors, false, false, forInit)
	if err != nil {
		return nil, err
	}
	exprErrors, _ := this.checkExpressionErrors(refDestructuringErrors, false)

	if exprErrors {
		return expr, nil
	}
	if expr.Start == startPos && expr.Type == NODE_ARROW_FUNCTION_EXPRESSION {
		return expr, nil
	}
	expr, err = this.parseExprOp(expr, startPos, startLoc, -1, forInit)
	if err != nil {
		return nil, err
	}
	return expr, nil

}

func (this *Parser) parseExprOp(left *Node, leftStartPos int, leftStartLoc *Location, minPrec int, forInit string) (*Node, error) {
	if this.Type.binop != nil && (len(forInit) == 0 || this.Type.identifier != TOKEN_IN) {
		prec := this.Type.binop.prec
		if this.Type.binop.prec > minPrec {
			logical := this.Type.identifier == TOKEN_LOGICALOR || this.Type.identifier == TOKEN_LOGICALAND
			coalesce := this.Type.identifier == TOKEN_COALESCE
			if coalesce {
				// Handle the precedence of `tt.coalesce` as equal to the range of logical expressions.
				// In other words, `node.right` shouldn't contain logical expressions in order to check the mixed error.
				prec = tokenTypes[TOKEN_LOGICALAND].binop.prec
			}
			if op, ok := this.Value.([]byte); ok {
				this.next(false)
				startPos, startLoc := this.start, this.startLoc
				unary, err := this.parseMaybeUnary(nil, false, false, forInit)
				if err != nil {
					return nil, err
				}
				right, err := this.parseExprOp(unary, startPos, startLoc, prec, forInit)

				if err != nil {
					return nil, err
				}
				node, err := this.buildBinary(leftStartPos, leftStartLoc, left, right, BinaryOperator(op), logical || coalesce)
				if err != nil {
					return nil, err
				}
				if (logical && this.Type.identifier == TOKEN_COALESCE) || (coalesce && (this.Type.identifier == TOKEN_LOGICALOR || this.Type.identifier == TOKEN_LOGICALAND)) {
					return nil, this.raiseRecoverable(this.start, "Logical expressions and coalesce expressions cannot be mixed. Wrap either by parentheses")
				}
				expr, err := this.parseExprOp(node, leftStartPos, leftStartLoc, minPrec, forInit)
				if err != nil {
					return nil, err
				}
				return expr, nil
			} else {
				panic("Node had invalid operator as Value, expected []byte")
			}

		}
	}
	return left, nil
}

func isPrivateFieldAccess(node *Node) bool {
	return node.Type == NODE_MEMBER_EXPRESSION && node.Property.Type == NODE_PRIVATE_IDENTIFIER ||
		node.Type == NODE_CHAIN_EXPRESSION && isPrivateFieldAccess(node.Expression) ||
		node.Type == NODE_PARENTHESIZED_EXPRESSION && isPrivateFieldAccess(node.Expression)

}

func (this *Parser) parseYield(forInit string) (*Node, error) {
	if this.YieldPos == 0 {
		this.YieldPos = this.start
	}

	node := this.startNode()
	this.next(false)
	if this.Type.identifier == TOKEN_SEMI || this.canInsertSemicolon() || (this.Type.identifier != TOKEN_STAR && !this.Type.startsExpr) {
		node.Delegate = false
		node.Argument = nil
	} else {
		node.Delegate = this.eat(TOKEN_STAR)
		maybeAssign, err := this.parseMaybeAssign(forInit, nil, nil)
		if err != nil {
			return nil, err
		}
		node.Argument = maybeAssign
	}
	return this.finishNode(node, NODE_YIELD_EXPRESSION), nil
}

func (this *Parser) parseTemplate(opts struct{ isTagged bool }) (*Node, error) {
	node := this.startNode()
	this.next(false)
	node.Expressions = []*Node{}
	curElt, err := this.parseTemplateElement(opts)

	if err != nil {
		return nil, err
	}

	node.Quasis = []*Node{curElt}
	for !curElt.Tail {
		if this.Type.identifier == TOKEN_EOF {
			return nil, this.raise(this.pos, "Unterminated template literal")
		}
		err := this.expect(TOKEN_DOLLARBRACEL)
		if err != nil {
			return nil, err
		}
		n, err := this.parseExpression("", nil)

		if err != nil {
			return nil, err
		}
		node.Expressions = append(node.Expressions, n)
		err = this.expect(TOKEN_BRACER)
		if err != nil {
			return nil, err
		}
		curElt, err = this.parseTemplateElement(opts)
		if err != nil {
			return nil, err
		}
		node.Quasis = append(node.Quasis, curElt)
	}
	this.next(false)
	return this.finishNode(node, NODE_TEMPLATE_LITERAL), nil
}

func (this *Parser) parseTemplateElement(opts struct{ isTagged bool }) (*Node, error) {
	elem := this.startNode()
	if this.Type.identifier == TOKEN_INVALIDTEMPLATE {
		if !opts.isTagged {
			return nil, this.raiseRecoverable(this.start, "Bad escape sequence in untagged template literal")
		}

		elem.Value = struct {
			raw    string
			cooked string
		}{
			raw:    strings.ReplaceAll(string(this.Value.([]byte)), "\r\n", "\n"),
			cooked: "",
		}
	} else {
		elem.Value = struct {
			raw    string
			cooked string
		}{
			raw:    strings.ReplaceAll(string(this.input[this.start:this.End]), "\r\n", "\n"),
			cooked: string(this.Value.([]byte)),
		}
	}
	this.next(false)
	elem.Tail = this.Type.identifier == TOKEN_BACKQUOTE
	return this.finishNode(elem, NODE_TEMPLATE_ELEMENT), nil
}

func (this *Parser) parseSubscriptAsyncArrow(startPos int, startLoc *Location, exprList []*Node, forInit string) (*Node, error) {
	node := this.startNodeAt(startPos, startLoc)
	arrowExpression, err := this.parseArrowExpression(node, exprList, true, forInit)

	return arrowExpression, err
}

func (this *Parser) parseExprAtom(refDestructuringErrors *DestructuringErrors, forInit string, forNew bool) (*Node, error) {
	// If a division operator appears in an expression position, the
	// tokenizer got confused, and we force it to read a regexp instead.
	if this.Type.identifier == TOKEN_SLASH {
		err := this.readRegexp()
		if err != nil {
			return nil, err
		}
	}

	_, canBeArrow := this.PotentialArrowAt == this.start, this.PotentialArrowAt == this.start
	switch this.Type.identifier {
	case TOKEN_SUPER:
		if !this.allowSuper() {
			return nil, this.raise(this.start, "'super' keyword outside a method")
		}

		node := this.startNode()
		this.next(false)
		if this.Type.identifier == TOKEN_PARENL && !this.allowDirectSuper() {
			return nil, this.raise(node.Start, "super() call outside constructor of a subclass")
		}

		// The `super` keyword can appear at below:
		// SuperProperty:
		//     super [ Expression ]
		//     super . IdentifierName
		// SuperCall:
		//     super ( Arguments )

		if this.Type.identifier != TOKEN_DOT && this.Type.identifier != TOKEN_BRACKETL && this.Type.identifier != TOKEN_PARENL {
			return nil, this.unexpected(`this.Type.identifier != TOKEN_DOT && this.Type.identifier != TOKEN_BRACKETL && this.Type.identifier != TOKEN_PARENL`, nil)
		}

		return this.finishNode(node, NODE_SUPER), nil

	case TOKEN_THIS:
		node := this.startNode()
		this.next(false)
		return this.finishNode(node, NODE_THIS_EXPRESSION), nil

	case TOKEN_NAME:
		startPos, startLoc, containsEsc := this.start, this.startLoc, this.ContainsEsc

		id, err := this.parseIdent(false)
		if err != nil {
			return nil, err
		}
		if this.getEcmaVersion() >= 8 && !containsEsc && id.Name == "async" && !this.canInsertSemicolon() && this.eat(TOKEN_FUNCTION) {
			this.overrideContext(TokenContexts[FUNCTION_EXPRESSION])
			fun, err := this.parseFunction(this.startNodeAt(startPos, startLoc), 0, false, true, forInit)
			return fun, err
		}

		if canBeArrow && !this.canInsertSemicolon() {
			if this.eat(TOKEN_ARROW) {
				arrowExpr, err := this.parseArrowExpression(this.startNodeAt(startPos, startLoc), []*Node{id}, false, forInit)
				return arrowExpr, err
			}

			if this.getEcmaVersion() >= 8 && id.Name == "async" && this.Type.identifier == TOKEN_NAME && !containsEsc &&
				(!this.PotentialArrowInForAwait || this.Value != "of" || this.ContainsEsc) {
				id, err = this.parseIdent(false)
				if err != nil {
					return nil, err
				}

				if this.canInsertSemicolon() || !this.eat(TOKEN_ARROW) {
					return nil, this.unexpected(`if this.canInsertSemicolon() || !this.eat(TOKEN_ARROW)`, nil)
				}
				arrowExpr, err := this.parseArrowExpression(this.startNodeAt(startPos, startLoc), []*Node{id}, true, forInit)
				return arrowExpr, err
			}
		}
		return id, nil

	case TOKEN_REGEXP:
		panic("TOKEN_REGEXP not implemented")
		/*
			value := this.Value
			node = this.parseLiteral(value.value)
			node.regex = RegExpState{Pattern: value.pattern, flags: value.flags}
			return node
		*/

	case TOKEN_NUM, TOKEN_STRING:
		{
			literal, err := this.parseLiteral(this.Value)
			return literal, err
		}

	case TOKEN_NULL, TOKEN_TRUE, TOKEN_FALSE:
		node := this.startNode()
		if this.Type.identifier == TOKEN_NULL {
			node.Value = nil
		} else {
			node.Value = this.Type.identifier == TOKEN_TRUE
		}

		node.Raw = this.Type.keyword
		this.next(false)
		return this.finishNode(node, NODE_LITERAL), nil

	case TOKEN_PARENL:
		expr, err := this.parseParenAndDistinguishExpression(canBeArrow, forInit)
		if err != nil {
			return nil, err
		}

		start := this.start
		if refDestructuringErrors != nil {
			if refDestructuringErrors.parenthesizedAssign < 0 && !this.isSimpleAssignTarget(expr) {
				refDestructuringErrors.parenthesizedAssign = start
			}

			if refDestructuringErrors.parenthesizedBind < 0 {
				refDestructuringErrors.parenthesizedBind = start
			}

		}
		return expr, nil

	case TOKEN_BRACKETL:
		node := this.startNode()
		this.next(false)

		exprList, err := this.parseExprList(TOKEN_BRACKETR, true, true, refDestructuringErrors)

		if err != nil {
			return nil, err
		}

		node.Elements = exprList
		return this.finishNode(node, NODE_ARRAY_EXPRESSION), nil

	case TOKEN_BRACEL:
		this.overrideContext(TokenContexts[BRACKET_EXPRESSION])
		obj, err := this.parseObj(false, refDestructuringErrors)
		return obj, err

	case TOKEN_FUNCTION:
		node := this.startNode()
		this.next(false)
		fun, err := this.parseFunction(node, 0, false, false, "")
		return fun, err

	case TOKEN_CLASS:
		node := this.startNode()
		class, err := this.parseClass(node, false)
		return class, err

	case TOKEN_NEW:
		new, err := this.parseNew()
		return new, err

	case TOKEN_BACKQUOTE:
		tmpl, err := this.parseTemplate(struct{ isTagged bool }{isTagged: false})
		return tmpl, err

	case TOKEN_IMPORT:
		if this.getEcmaVersion() >= 11 {
			exprImport, err := this.parseExprImport(forNew)
			return exprImport, err
		} else {
			return nil, this.unexpected("Ecma version is too old", nil)
		}

	default:
		return nil, this.parseExprAtomDefault()

	}
}

func (p *Parser) parseExprAtomDefault() error {
	return p.unexpected("parseExprAtomDefault()", nil)
}

func (this *Parser) shouldParseAsyncArrow() bool {
	return !this.canInsertSemicolon() && this.eat(TOKEN_ARROW)
}

func (this *Parser) parseExprList(close Token, allowTrailingComma bool, allowEmpty bool, refDestructuringErrors *DestructuringErrors) ([]*Node, error) {
	elts, first := []*Node{}, true

	for !this.eat(close) {
		if !first {
			if err := this.expect(TOKEN_COMMA); err != nil {
				return nil, err
			}

			if allowTrailingComma && this.afterTrailingComma(close, false) {
				break
			}
		} else {
			first = false
		}

		var elt *Node
		if allowEmpty && this.Type.identifier == TOKEN_COMMA {
			elt = nil
		} else if this.Type.identifier == TOKEN_ELLIPSIS {
			spreadElement, err := this.parseSpread(refDestructuringErrors)

			if err != nil {
				return nil, err
			}

			if refDestructuringErrors != nil && this.Type.identifier == TOKEN_COMMA && refDestructuringErrors.trailingComma < 0 {
				refDestructuringErrors.trailingComma = this.start
			}
			elt = spreadElement

		} else {
			maybeAssign, err := this.parseMaybeAssign("", refDestructuringErrors, nil)
			if err != nil {
				return nil, err
			}
			elt = maybeAssign
		}
		elts = append(elts, elt)
	}
	return elts, nil
}

func (this *Parser) parseIdent(liberal bool) (*Node, error) {
	node, err := this.parseIdentNode()
	if err != nil {
		return nil, err
	}
	this.next(liberal)
	this.finishNode(node, NODE_IDENTIFIER)
	if !liberal {
		err := this.checkUnreserved(struct {
			start int
			end   int
			name  string
		}{start: node.Start, end: node.End, name: node.Name})

		if err != nil {
			return nil, err
		}

		if node.Name == "await" && !(this.AwaitIdentPos != 0) {
			this.AwaitIdentPos = node.Start
		}

	}
	return node, nil
}

func (this *Parser) parseIdentNode() (*Node, error) {
	node := this.startNode()
	if this.Type.identifier == TOKEN_NAME {
		if val, ok := this.Value.(string); ok {
			node.Name = val
		} else {
			panic("Theres a situation with node having a wrong type of .Value")
		}

	} else if len(this.Type.keyword) != 0 {
		node.Name = this.Type.keyword

		if (node.Name == "class" || node.Name == "function") &&
			(this.LastTokEnd != this.LastTokStart+1 || this.input[this.LastTokStart] != 46) {
			this.Context = this.Context[:len(this.Context)-1]
		}
		this.Type = tokenTypes[TOKEN_NAME]
	} else {
		return nil, this.unexpected("Keyword was not present, we want it", nil)
	}
	return node, nil
}

func (this *Parser) checkUnreserved(opts struct {
	start int
	end   int
	name  string
}) error {
	if this.inGenerator() && opts.name == "yield" {
		return this.raiseRecoverable(opts.start, "Cannot use 'yield' as identifier inside a generator")
	}

	if this.inAsync() && opts.name == "await" {
		return this.raiseRecoverable(opts.start, "Cannot use 'await' as identifier inside an async function")
	}
	curScope := this.currentThisScope()
	if !(curScope != nil && curScope.Flags&SCOPE_VAR == SCOPE_VAR) && opts.name == "arguments" {
		return this.raiseRecoverable(opts.start, "Cannot use 'arguments' in class field initializer")
	}

	if this.InClassStaticBlock && (opts.name == "arguments" || opts.name == "await") {
		return this.raise(opts.start, `Cannot use ${name} in class static initialization block`)
	}
	if this.Keywords.Match([]byte(opts.name)) {
		return this.raise(opts.start, "Unexpected keyword "+opts.name)
	}

	if this.getEcmaVersion() < 6 && strings.Index(string(this.input[opts.start:opts.end]), "\\") != -1 {
		return nil
	}
	var re *regexp.Regexp
	if this.Strict {
		re = this.ReservedWordsStrict
	} else {
		re = this.ReservedWords
	}

	if re.Match([]byte(opts.name)) {
		if !this.inAsync() && opts.name == "await" {
			return this.raiseRecoverable(opts.start, "Cannot use keyword 'await' outside an async function")
		}
		return this.raiseRecoverable(opts.start, "The keyword "+opts.name+" is reserved")
	}
	return nil
}

func (this *Parser) parseExprImport(forNew bool) (*Node, error) {
	node := this.startNode()

	// Consume `import` as an identifier for `import.meta`.
	// Because `this.parseIdent(true)` doesn't check escape sequences, it needs the check of `this.containsEsc`.
	if this.ContainsEsc {
		return nil, this.raiseRecoverable(this.start, "Escape sequence in keyword import")
	}
	this.next(false)

	if this.Type.identifier == TOKEN_PARENL && !forNew {
		dynImport, err := this.parseDynamicImport(node)
		return dynImport, err
	} else if this.Type.identifier == TOKEN_DOT {
		var loc *Location

		if node.Location != nil && node.Location.Start != nil {
			loc = node.Location.Start
		}
		meta := this.startNodeAt(node.Start, loc)
		meta.Name = "import"
		node.Meta = this.finishNode(meta, NODE_IDENTIFIER)
		importMeta, err := this.parseImportMeta(node)
		return importMeta, err
	} else {
		return nil, this.unexpected("", nil)
	}
}

func (this *Parser) parseImportMeta(node *Node) (*Node, error) {
	this.next(false) // skip `.`

	containsEsc := this.ContainsEsc
	ident, err := this.parseIdent(true)

	if err != nil {
		return nil, err
	}
	node.Property = ident

	if node.Property.Name != "meta" {
		return nil, this.raiseRecoverable(node.Property.Start, "The only valid meta property for import is 'import.meta'")
	}

	if containsEsc {
		return nil, this.raiseRecoverable(node.Start, "'import.meta' must not contain escaped characters")
	}

	if this.options.SourceType != "module" && !this.options.AllowImportExportEverywhere {
		return nil, this.raiseRecoverable(node.Start, "Cannot use 'import.meta' outside a module")
	}

	return this.finishNode(node, NODE_META_PROPERTY), nil
}

func (this *Parser) parseDynamicImport(node *Node) (*Node, error) {
	this.next(false)

	source, err := this.parseMaybeAssign("", nil, nil)
	if err != nil {
		return nil, err
	}
	node.Source = source

	if this.getEcmaVersion() >= 16 {
		if !this.eat(TOKEN_PARENR) {
			err := this.expect(TOKEN_COMMA)
			if err != nil {
				return nil, err
			}

			if !this.afterTrailingComma(TOKEN_PARENR, false) {
				opts, err := this.parseMaybeAssign("", nil, nil)
				if err != nil {
					return nil, err
				}
				node.Options = opts
				if !this.eat(TOKEN_PARENR) {
					err := this.expect(TOKEN_COMMA)
					if err != nil {
						return nil, err
					}
					if !this.afterTrailingComma(TOKEN_PARENR, false) {
						this.unexpected("trailing commas", nil)
					}
				}
			} else {
				node.Options = nil
			}
		} else {
			node.Options = nil
		}
	} else {
		// Verify ending.
		if !this.eat(TOKEN_PARENR) {
			errorPos := this.start
			if this.eat(TOKEN_COMMA) && this.eat(TOKEN_PARENR) {
				return nil, this.raiseRecoverable(errorPos, "Trailing comma is not allowed in import()")
			} else {
				return nil, this.unexpected("", &errorPos)
			}
		}
	}

	return this.finishNode(node, NODE_IMPORT_EXPRESSION), nil
}

func (this *Parser) parseLiteral(value any) (*Node, error) {
	node := this.startNode()
	node.Value = value

	node.Raw = string(this.input[this.start:this.End])
	if node.Raw[len(node.Raw)-1] == 110 { // big int stuff, maybe some day....
		node.Bigint = strings.ReplaceAll(node.Raw[:len(node.Raw)-1], "_", "")
		// node.bigint = node.raw.slice(0, -1).replace(/_/g, "")
	}
	this.next(false)
	return this.finishNode(node, NODE_LITERAL), nil
}

func (this *Parser) parsePrivateIdent() (*Node, error) {
	node := this.startNode()
	if this.Type.identifier == TOKEN_PRIVATEID {
		if val, ok := this.Value.(string); ok {
			node.Name = val
		} else {
			panic("In parsePrivateIdent() this.Value was not string as expected")
		}
	} else {
		return nil, this.unexpected("", &this.pos)
	}
	this.next(false)
	this.finishNode(node, NODE_PRIVATE_IDENTIFIER)

	if this.options.CheckPrivateFields {
		if len(this.PrivateNameStack) == 0 {
			this.raise(node.Start, "Private field #"+node.Name+" must be declared in an enclosing class")
		} else {
			this.PrivateNameStack[len(this.PrivateNameStack)-1].Used = append(this.PrivateNameStack[len(this.PrivateNameStack)-1].Used, node)
		}
	}

	return node, nil
}

func (this *Parser) parseParenAndDistinguishExpression(canBeArrow bool, forInit string) (*Node, error) {
	startPos, startLoc, allowTrailingComma := this.start, this.startLoc, this.getEcmaVersion() >= 8
	var val *Node
	if this.getEcmaVersion() >= 6 {
		this.next(false)

		innerStartPos, innerStartLoc := this.start, this.startLoc
		exprList, first, lastIsComma := []*Node{}, true, false
		refDestructuringErrors, oldYieldPos, oldAwaitPos, spreadStart := NewDestructuringErrors(), this.YieldPos, this.AwaitPos, 0
		this.YieldPos = 0
		this.AwaitPos = 0
		// Do not save awaitIdentPos to allow checking awaits nested in parameters
		for this.Type.identifier != TOKEN_PARENR {
			if first {
				first = false
			} else {
				err := this.expect(TOKEN_COMMA)
				if err != nil {
					return nil, err
				}
			}

			if allowTrailingComma && this.afterTrailingComma(TOKEN_PARENR, true) {
				lastIsComma = true
				break
			} else if this.Type.identifier == TOKEN_ELLIPSIS {
				spreadStart = this.start
				restBinding, err := this.parseRestBinding()
				if err != nil {
					return nil, err
				}

				parenItem, err := this.parseParenItem(restBinding)

				if err != nil {
					return nil, err
				}

				exprList = append(exprList, parenItem)

				if this.Type.identifier == TOKEN_COMMA {
					return nil, this.raiseRecoverable(
						this.start,
						"Comma is not permitted after the rest element",
					)
				}
				break
			} else {
				maybeAssign, err := this.parseMaybeAssign("", refDestructuringErrors, &struct {
					call func(p *Parser, l *Node, s int, sl *Location) (*Node, error)
				}{call: func(p *Parser, l *Node, s int, sl *Location) (*Node, error) {
					res, err := p.parseParenItem(l)
					return res, err
				}}) // Horrible :S

				if err != nil {
					return nil, err
				}
				exprList = append(exprList, maybeAssign)
			}
		}
		innerEndPos, innerEndLoc := this.LastTokEnd, this.LastTokEndLoc
		err := this.expect(TOKEN_PARENR)

		if err != nil {
			return nil, err
		}

		if canBeArrow && this.shouldParseArrow(exprList) && this.eat(TOKEN_ARROW) {
			err := this.checkPatternErrors(refDestructuringErrors, false)
			if err != nil {
				return nil, err
			}

			err = this.checkYieldAwaitInDefaultParams()
			if err != nil {
				return nil, err
			}

			this.YieldPos = oldYieldPos
			this.AwaitPos = oldAwaitPos
			parenArrowList, err := this.parseParenArrowList(startPos, startLoc, exprList, forInit)
			return parenArrowList, err
		}

		if len(exprList) == 0 || lastIsComma {
			return nil, this.unexpected("hanging comma", &this.LastTokStart)
		}

		if spreadStart != 0 {
			return nil, this.unexpected("", &spreadStart)
		}
		_, err = this.checkExpressionErrors(refDestructuringErrors, true)

		if err != nil {
			return nil, err
		}

		if oldYieldPos != 0 {
			this.YieldPos = oldYieldPos
		}

		if oldAwaitPos != 0 {
			this.AwaitPos = oldAwaitPos
		}

		if len(exprList) > 1 {
			val = this.startNodeAt(innerStartPos, innerStartLoc)
			val.Expressions = exprList
			this.finishNodeAt(val, NODE_SEQUENCE_EXPRESSION, innerEndPos, innerEndLoc)
		} else {
			val = exprList[0]
		}
	} else {
		parenExpr, err := this.parseParenExpression()

		if err != nil {
			return nil, err
		}
		val = parenExpr
	}

	if this.options.PreserveParens {
		par := this.startNodeAt(startPos, startLoc)
		par.Expression = val
		return this.finishNode(par, NODE_PARENTHESIZED_EXPRESSION), nil
	} else {
		return val, nil
	}
}

func (this *Parser) parseParenArrowList(startPos int, startLoc *Location, exprList []*Node, forInit string) (*Node, error) {
	arrExpr, err := this.parseArrowExpression(this.startNodeAt(startPos, startLoc), exprList, false, forInit)
	return arrExpr, err
}

func (this *Parser) shouldParseArrow(_ []*Node) bool {
	return !this.canInsertSemicolon()
}

func (this *Parser) parseParenItem(item *Node) (*Node, error) {
	return item, nil
}

func (this *Parser) parseParenExpression() (*Node, error) {
	err := this.expect(TOKEN_PARENL)
	if err != nil {
		return nil, err
	}
	val, errParse := this.parseExpression("", nil)
	if errParse != nil {
		return nil, err
	}
	err = this.expect(TOKEN_PARENR)

	if err != nil {
		return nil, err
	}
	return val, nil
}

func (this *Parser) parseNew() (*Node, error) {
	if this.ContainsEsc {
		return nil, this.raiseRecoverable(this.start, "Escape sequence in keyword new")
	}
	node := this.startNode()
	this.next(false)
	if this.getEcmaVersion() >= 6 && this.Type.identifier == TOKEN_DOT {

		var startLoc *Location

		if node.Location != nil {
			startLoc = node.Location.Start
		}
		meta := this.startNodeAt(node.Start, startLoc)
		meta.Name = "new"
		node.Meta = this.finishNode(meta, NODE_IDENTIFIER)
		this.next(false)
		containsEsc := this.ContainsEsc
		id, err := this.parseIdent(true)
		if err != nil {
			return nil, err
		}
		node.Property = id
		if node.Property.Name != "target" {
			return nil, this.raiseRecoverable(node.Property.Start, "The only valid meta property for new is 'new.target'")
		}

		if containsEsc {
			return nil, this.raiseRecoverable(node.Start, "'new.target' must not contain escaped characters")
		}

		if !this.allowNewDotTarget() {
			return nil, this.raiseRecoverable(node.Start, "'new.target' can only be used in functions and class static block")
		}

		return this.finishNode(node, NODE_META_PROPERTY), nil
	}
	startPos, startLoc := this.start, this.startLoc
	exprAtom, err := this.parseExprAtom(nil, "", true)
	if err != nil {
		return nil, err
	}

	subscript, errSubcript := this.parseSubscript(exprAtom, startPos, startLoc, true, false, false, "")

	if errSubcript != nil {
		return nil, errSubcript
	}
	node.Callee = subscript
	if this.eat(TOKEN_PARENL) {
		exprList, err := this.parseExprList(TOKEN_PARENR, this.getEcmaVersion() >= 8, false, nil)
		if err != nil {
			return nil, err
		}

		node.Arguments = exprList
	} else {
		node.Arguments = []*Node{}
	}
	return this.finishNode(node, NODE_NEW_EXPRESSION), nil
}

func (this *Parser) parseArrowExpression(node *Node, params []*Node, isAsync bool, forInit string) (*Node, error) {
	oldYieldPos, oldAwaitPos, oldAwaitIdentPos := this.YieldPos, this.AwaitPos, this.AwaitIdentPos

	this.enterScope(functionFlags(isAsync, false) | SCOPE_ARROW)
	this.initFunction(node)

	node.IsAsync = isAsync

	this.YieldPos = 0
	this.AwaitPos = 0
	this.AwaitIdentPos = 0

	listParams, err := this.toAssignableList(params, true)

	if err != nil {
		return nil, err
	}
	node.Params = listParams
	err = this.parseFunctionBody(node, true, false, forInit)
	if err != nil {
		return nil, err
	}

	this.YieldPos = oldYieldPos
	this.AwaitPos = oldAwaitPos
	this.AwaitIdentPos = oldAwaitIdentPos

	return this.finishNode(node, NODE_ARROW_FUNCTION_EXPRESSION), nil
}

func (this *Parser) parseFunctionBody(node *Node, isArrowFunction bool, isMethod bool, forInit string) error {
	isExpression := isArrowFunction && this.Type.identifier != TOKEN_BRACEL
	oldStrict, useStrict := this.Strict, false
	if isExpression {
		maybeAssign, err := this.parseMaybeAssign(forInit, nil, nil)
		if err != nil {
			return err
		}
		node.BodyNode = maybeAssign
		node.IsExpression = true
		err = this.checkParams(node, false)
		if err != nil {
			return err
		}
	} else {
		nonSimple := this.getEcmaVersion() >= 7 && !this.isSimpleParamList(node.Params)
		if !oldStrict || nonSimple {
			useStrict = this.strictDirective(this.End)
			// If this is a strict mode function, verify that argument names
			// are not repeated, and it does not try to bind the words `eval`
			// or `arguments`.
			if useStrict && nonSimple {
				return this.raiseRecoverable(node.Start, "Illegal 'use strict' directive in function with non-simple parameter list")
			}

		}
		// Start a new scope with regard to labels and the `inFunction`
		// flag (restore them to their old value afterwards).
		oldLabels := this.Labels
		this.Labels = []Label{}
		if useStrict {
			this.Strict = true
		}

		// Add the params to varDeclaredNames to ensure that an error is thrown
		// if a let/const declaration in the function clashes with one of the params.
		err := this.checkParams(node, !oldStrict && !useStrict && !isArrowFunction && !isMethod && this.isSimpleParamList(node.Params))

		if err != nil {
			return err
		}
		// Ensure the function name isn't a forbidden identifier in strict mode, e.g. 'eval'
		if this.Strict && node.Identifier != nil {
			err := this.checkLValSimple(node.Identifier, BIND_OUTSIDE, struct {
				check bool
				hash  map[string]bool
			}{})

			if err != nil {
				return err
			}
		}
		block, err := this.parseBlock(false, nil, useStrict && !oldStrict)
		if err != nil {
			return err
		}
		node.BodyNode = block
		node.IsExpression = false
		this.adaptDirectivePrologue(node.BodyNode.Body)
		this.Labels = oldLabels
	}
	this.exitScope()
	return nil
}

func (this *Parser) isSimpleParamList(params []*Node) bool {
	for _, param := range params {
		if param.Type != NODE_IDENTIFIER {
			return false
		}
	}
	return true
}

func (this *Parser) checkParams(node *Node, allowDuplicates bool) error {
	// nameHash = Object.create(null), let's see if I got this right....
	for _, param := range node.Params {
		if allowDuplicates {
			err := this.checkLValInnerPattern(param, BIND_VAR, struct {
				check bool
				hash  map[string]bool
			}{})
			if err != nil {
				return err
			}
		} else {
			err := this.checkLValInnerPattern(param, BIND_VAR, struct {
				check bool
				hash  map[string]bool
			}{check: true, hash: map[string]bool{}})
			if err != nil {
				return err
			}
		}

	}
	return nil
}

func (this *Parser) initFunction(node *Node) {
	node.Identifier = nil
	if this.getEcmaVersion() >= 6 {
		node.IsGenerator = false
		node.IsExpression = false
	}

	if this.getEcmaVersion() >= 8 {
		node.IsAsync = false
	}
}

func (this *Parser) parseMethod(isGenerator bool, isAsync bool, allowDirectSuper bool) (*Node, error) {
	node, oldYieldPos, oldAwaitPos, oldAwaitIdentPos := this.startNode(), this.YieldPos, this.AwaitPos, this.AwaitIdentPos

	this.initFunction(node)
	node.IsGenerator = isGenerator

	node.IsAsync = isAsync

	this.YieldPos = 0
	this.AwaitPos = 0
	this.AwaitIdentPos = 0

	flags := functionFlags(isAsync, node.IsGenerator) | SCOPE_SUPER

	if allowDirectSuper {
		this.enterScope(flags | SCOPE_DIRECT_SUPER)
	} else {
		this.enterScope(flags)
	}

	err := this.expect(TOKEN_PARENL)
	if err != nil {
		return nil, err
	}

	bindingList, errBindingList := this.parseBindingList(TOKEN_PARENR, false, this.getEcmaVersion() >= 8, false)
	if errBindingList != nil {
		return nil, errBindingList
	}
	node.Params = bindingList
	err = this.checkYieldAwaitInDefaultParams()
	if err != nil {
		return nil, err
	}

	err = this.parseFunctionBody(node, false, true, "")

	if err != nil {
		return nil, err
	}

	this.YieldPos = oldYieldPos
	this.AwaitPos = oldAwaitPos
	this.AwaitIdentPos = oldAwaitIdentPos
	return this.finishNode(node, NODE_FUNCTION_EXPRESSION), nil
}

func (this *Parser) parseObj(isPattern bool, refDestructuringErrors *DestructuringErrors) (*Node, error) {
	node, first, propHash := this.startNode(), true, &PropertyHash{proto: false, m: map[string]map[Kind]bool{}}
	node.Properties = []*Node{}
	this.next(false)
	for !this.eat(TOKEN_BRACER) {
		if !first {
			err := this.expect(TOKEN_COMMA)
			if err != nil {
				return nil, err
			}
			if this.getEcmaVersion() >= 5 && this.afterTrailingComma(TOKEN_BRACER, false) {
				break
			}
		} else {
			first = false
		}
		prop, err := this.parseProperty(isPattern, refDestructuringErrors)
		if err != nil {
			return nil, err
		}
		if !isPattern {
			err := this.checkPropClash(prop, propHash, refDestructuringErrors)
			if err != nil {
				return nil, err
			}
		}
		node.Properties = append(node.Properties, prop)
	}

	if isPattern {
		return this.finishNode(node, NODE_OBJECT_PATTERN), nil
	}
	return this.finishNode(node, NODE_OBJECT_EXPRESSION), nil
}

func (this *Parser) parseProperty(isPattern bool, refDestructuringErrors *DestructuringErrors) (*Node, error) {
	prop := this.startNode()
	isGenerator, isAsync, startPos := false, false, 0
	var startLoc *Location

	if this.getEcmaVersion() >= 9 && this.eat(TOKEN_ELLIPSIS) {
		if isPattern {
			ident, err := this.parseIdent(false)
			if err != nil {
				return nil, err
			}

			prop.Argument = ident
			if this.Type.identifier == TOKEN_COMMA {
				return nil, this.raiseRecoverable(this.start, "Comma is not permitted after the rest element")
			}
			return this.finishNode(prop, NODE_REST_ELEMENT), nil
		}
		// Parse argument.
		maybeAssign, err := this.parseMaybeAssign("", refDestructuringErrors, nil)
		if err != nil {
			return nil, err
		}
		prop.Argument = maybeAssign
		// To disallow trailing comma via `this.toAssignable()`.
		if this.Type.identifier == TOKEN_COMMA && refDestructuringErrors != nil && refDestructuringErrors.trailingComma < 0 {
			refDestructuringErrors.trailingComma = this.start
		}
		// Finish
		return this.finishNode(prop, NODE_SPREAD_ELEMENT), nil
	}
	if this.getEcmaVersion() >= 6 {
		prop.IsMethod = false
		prop.Shorthand = false
		if isPattern || refDestructuringErrors != nil {
			startPos = this.start
			startLoc = this.startLoc
		}
		if !isPattern {
			isGenerator = this.eat(TOKEN_STAR)
		}

	}
	containsEsc := this.ContainsEsc
	_, err := this.parsePropertyName(prop)

	if err != nil {
		return nil, err
	}

	if !isPattern && !containsEsc && this.getEcmaVersion() >= 8 && !isGenerator && this.isAsyncProp(prop) {
		isAsync = true
		isGenerator = this.getEcmaVersion() >= 9 && this.eat(TOKEN_STAR)
		_, err := this.parsePropertyName(prop)

		if err != nil {
			return nil, err
		}
	} else {
		isAsync = false
	}
	err = this.parsePropertyValue(prop, isPattern, isGenerator, isAsync, startPos, startLoc, refDestructuringErrors, containsEsc)
	if err != nil {
		return nil, err
	}
	return this.finishNode(prop, NODE_PROPERTY), nil
}

func (this *Parser) parsePropertyValue(prop *Node, isPattern bool, isGenerator bool, isAsync bool, startPos int, startLoc *Location, refDestructuringErrors *DestructuringErrors, containsEsc bool) error {
	if (isGenerator || isAsync) && this.Type.identifier == TOKEN_COLON {
		return this.unexpected("", nil)
	}

	if this.eat(TOKEN_COLON) {
		prop.Kind = KIND_PROPERTY_INIT
		if isPattern {
			val, err := this.parseMaybeDefault(this.start, this.startLoc, nil)
			if err != nil {
				return err
			}
			prop.Value = val
		} else {
			val, err := this.parseMaybeAssign("", refDestructuringErrors, nil)
			if err != nil {
				return err
			}
			prop.Value = val
		}
	} else if this.getEcmaVersion() >= 6 && this.Type.identifier == TOKEN_PARENL {
		if isPattern {
			return this.unexpected("", nil)
		}
		method, err := this.parseMethod(isGenerator, isAsync, false)
		if err != nil {
			return err
		}
		prop.IsMethod = true
		prop.Kind = KIND_PROPERTY_INIT
		prop.Value = method
	} else if !isPattern && !containsEsc &&
		this.getEcmaVersion() >= 5 && !prop.Computed && prop.Key.Type == NODE_IDENTIFIER &&
		(prop.Key.Name == "get" || prop.Key.Name == "set") &&
		(this.Type.identifier != TOKEN_COMMA && this.Type.identifier != TOKEN_BRACER && this.Type.identifier != TOKEN_EQ) {
		if isGenerator || isAsync {
			return this.unexpected("", nil)
		}
		err := this.parseGetterSetter(prop)
		if err != nil {
			return err
		}
	} else if this.getEcmaVersion() >= 6 && !prop.Computed && prop.Key.Type == NODE_IDENTIFIER {
		if isGenerator || isAsync {
			return this.unexpected("", nil)
		}
		err := this.checkUnreserved(struct {
			start int
			end   int
			name  string
		}{start: prop.Start, end: prop.End, name: prop.Name})
		if err != nil {
			return err
		}
		if prop.Key.Name == "await" && !(this.AwaitIdentPos != 0) {
			this.AwaitIdentPos = startPos
		}

		if isPattern {
			c, err := this.copyNode(prop.Key)
			if err != nil {
				return err
			}
			val, err := this.parseMaybeDefault(startPos, startLoc, c)
			if err != nil {
				return err
			}
			prop.Value = val
		} else if this.Type.identifier == TOKEN_EQ && refDestructuringErrors != nil {
			if refDestructuringErrors.shorthandAssign < 0 {
				refDestructuringErrors.shorthandAssign = this.start
			}
			c, err := this.copyNode(prop.Key)
			if err != nil {
				return err
			}
			val, err := this.parseMaybeDefault(startPos, startLoc, c)
			if err != nil {
				return err
			}
			prop.Value = val
		} else {
			c, err := this.copyNode(prop.Key)
			if err != nil {
				return err
			}
			prop.Value = c
		}
		prop.Kind = KIND_PROPERTY_INIT
		prop.Shorthand = true
	} else {
		return this.unexpected("", nil)
	}
	return nil
}

func (this *Parser) parseGetterSetter(prop *Node) error {
	kind := KIND_NOT_INITIALIZED

	switch prop.Key.Name {
	case "set":
		kind = KIND_PROPERTY_SET
	case "get":
		kind = KIND_PROPERTY_GET
	}

	this.parsePropertyName(prop)
	method, err := this.parseMethod(false, false, false)
	if err != nil {
		return err
	}
	prop.Value = method
	prop.Kind = kind
	paramCount := 0

	if prop.Kind == KIND_PROPERTY_GET {
		paramCount = 1
	}

	if val, ok := prop.Value.(*Node); ok {
		if len(val.Params) != paramCount {
			start := val.Start
			if prop.Kind == KIND_PROPERTY_GET {
				return this.raiseRecoverable(start, "getter should have no params")
			} else {
				return this.raiseRecoverable(start, "setter should have exactly one param")
			}
		} else {
			if prop.Kind == KIND_PROPERTY_SET && val.Params[0].Type == NODE_REST_ELEMENT {
				return this.raiseRecoverable(val.Params[0].Start, "Setter cannot use rest params")
			}
		}
	} else {
		panic("prop.Value was not *Node as we expected, we are in parseGetterSetter")
	}
	return nil
}

func (this *Parser) isAsyncProp(prop *Node) bool {
	return !prop.Computed && prop.Key.Type == NODE_IDENTIFIER && prop.Key.Name == "async" &&
		(this.Type.identifier == TOKEN_NAME || this.Type.identifier == TOKEN_NUM || this.Type.identifier == TOKEN_STRING || this.Type.identifier == TOKEN_BRACKETL || len(this.Type.keyword) != 0 || (this.getEcmaVersion() >= 9 && this.Type.identifier == TOKEN_STAR)) &&
		!lineBreak.Match(this.input[this.LastTokEnd:this.start])
}

func (this *Parser) parsePropertyName(prop *Node) (*Node, error) {
	if this.getEcmaVersion() >= 6 {
		if this.eat(TOKEN_BRACKETL) {
			prop.Computed = true
			maybeAssign, err := this.parseMaybeAssign("", nil, nil)
			if err != nil {
				return nil, err
			}
			prop.Key = maybeAssign
			err = this.expect(TOKEN_BRACKETR)

			if err != nil {
				return nil, err
			}
			return prop.Key, nil
		} else {
			prop.Computed = false
		}
	}
	if this.Type.identifier == TOKEN_NUM || this.Type.identifier == TOKEN_STRING {
		exprAtom, err := this.parseExprAtom(nil, "", false)
		if err != nil {
			return nil, err
		}
		prop.Key = exprAtom
		return prop.Key, nil
	} else {
		ident, err := this.parseIdent(this.options.AllowReserved != ALLOW_RESERVED_NEVER)
		if err != nil {
			return nil, err
		}
		prop.Key = ident
		return prop.Key, nil
	}
}
