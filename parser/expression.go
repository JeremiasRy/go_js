package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// EXPRESSION PARSING

func (p *Parser) checkPropClash(prop *Node, propHash *PropertyHash, refDestructuringErrors *DestructuringErrors) error {
	if p.getEcmaVersion() >= 9 && prop.Type == NODE_SPREAD_ELEMENT {
		return nil
	}

	if p.getEcmaVersion() >= 6 && (prop.Computed || prop.IsMethod || prop.Shorthand) {
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

	if p.getEcmaVersion() >= 6 {
		if name == "__proto__" && kind == KIND_PROPERTY_INIT {
			if propHash.proto {
				if refDestructuringErrors != nil {
					if refDestructuringErrors.doubleProto < 0 {
						refDestructuringErrors.doubleProto = key.Start
					}
				} else {
					return p.raiseRecoverable(key.Start, "Redefinition of __proto__ property")
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
			redefinition = p.Strict && other[KIND_PROPERTY_INIT] || other[KIND_PROPERTY_GET] || other[KIND_PROPERTY_SET]
		} else {
			redefinition = other[KIND_PROPERTY_INIT] || other[kind]
		}
		if redefinition {
			p.raiseRecoverable(key.Start, "Redefinition of property")
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

func (p *Parser) parseExpression(forInit string, refDestructuringErrors *DestructuringErrors) (*Node, error) {
	startPos, startLoc := p.start, p.startLoc
	expr, err := p.parseMaybeAssign(forInit, refDestructuringErrors, nil)

	if err != nil {
		return nil, err
	}
	if p.Type.identifier == TOKEN_COMMA {
		node := p.startNodeAt(startPos, startLoc)
		node.Expressions = []*Node{expr}

		for p.eat(TOKEN_COMMA) {
			maybeAssign, err := p.parseMaybeAssign(forInit, refDestructuringErrors, nil)
			if err != nil {
				return nil, err
			}
			node.Expressions = append(node.Expressions, maybeAssign)
		}

		return p.finishNode(node, NODE_SEQUENCE_EXPRESSION), nil
	}
	return expr, nil
}

func (p *Parser) parseMaybeAssign(forInit string, refDestructuringErrors *DestructuringErrors, afterLeftParse *struct {
	call func(p *Parser, l *Node, s int, sl *Location) (*Node, error)
}) (*Node, error) {
	if p.isContextual("yield") {
		if p.inGenerator() {
			yield, err := p.parseYield(forInit)
			if err != nil {
				return nil, err
			}
			return yield, nil
		} else {
			// The tokenizer will assume an expression is allowed after
			// `yield`, but this isn't that kind of yield
			p.ExprAllowed = false
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

	startPos, startLoc := p.start, p.startLoc

	if p.Type.identifier == TOKEN_PARENL || p.Type.identifier == TOKEN_NAME {
		p.PotentialArrowAt = p.start
		p.PotentialArrowInForAwait = forInit == "await"
	}
	left, err := p.parseMaybeConditional(forInit, refDestructuringErrors)

	if err != nil {
		return nil, err
	}

	if afterLeftParse != nil {
		newLeft, err := afterLeftParse.call(p, left, startPos, startLoc)
		if err != nil {
			return nil, err
		}
		left = newLeft
	}

	if p.Type.isAssign {
		node := p.startNodeAt(startPos, startLoc)
		var op AssignmentOperator

		if byteSlice, ok := p.Value.([]byte); ok {
			op = AssignmentOperator(byteSlice)
		} else {
			return nil, fmt.Errorf("invalid p.Value expected []byte, got: %q", p.Value)
		}
		node.AssignmentOperator = op
		if p.Type.identifier == TOKEN_EQ {
			left, err = p.toAssignable(left, false, refDestructuringErrors)

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

		if p.Type.identifier == TOKEN_EQ {
			p.checkLValPattern(left, 0, struct {
				check bool
				hash  map[string]bool
			}{check: false, hash: map[string]bool{}})
		} else {
			p.checkLValSimple(left, 0, struct {
				check bool
				hash  map[string]bool
			}{check: false, hash: map[string]bool{}})
		}

		node.Left = left
		p.next(false)
		right, err := p.parseMaybeAssign(forInit, refDestructuringErrors, nil)

		if err != nil {
			return nil, err
		}
		node.Right = right

		if oldDoubleProto > -1 {
			refDestructuringErrors.doubleProto = oldDoubleProto

		}
		return p.finishNode(node, NODE_ASSIGNMENT_EXPRESSION), nil
	} else {
		if ownDestructuringErrors {
			_, err := p.checkExpressionErrors(refDestructuringErrors, true)
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

func (p *Parser) parseMaybeConditional(forInit string, refDestructuringErrors *DestructuringErrors) (*Node, error) {
	startPos, startLoc := p.start, p.startLoc
	expr, err := p.parseExprOps(forInit, refDestructuringErrors)

	if err != nil {
		return nil, err
	}

	exprError, _ := p.checkExpressionErrors(refDestructuringErrors, false)
	if exprError {
		return expr, nil
	}
	if p.eat(TOKEN_QUESTION) {
		node := p.startNodeAt(startPos, startLoc)
		node.Test = expr
		maybeAssign, err := p.parseMaybeAssign("", nil, nil)
		if err != nil {
			return nil, err
		}
		node.Consequent = maybeAssign

		errExpect := p.expect(TOKEN_COLON)
		if errExpect != nil {
			return nil, err
		}

		maybeAssignElse, errElse := p.parseMaybeAssign(forInit, nil, nil)
		if errElse != nil {
			return nil, errElse
		}
		node.Alternate = maybeAssignElse
		return p.finishNode(node, NODE_CONDITIONAL_EXPRESSION), nil
	}
	return expr, nil
}

func (p *Parser) checkExpressionErrors(refDestructuringErrors *DestructuringErrors, andThrow bool) (bool, error) {
	if refDestructuringErrors == nil {
		return false, nil
	}
	shorthandAssign, doubleProto := refDestructuringErrors.shorthandAssign, refDestructuringErrors.doubleProto
	if !andThrow {
		return shorthandAssign >= 0 || doubleProto >= 0, nil
	}

	if shorthandAssign >= 0 {
		return true, p.raise(shorthandAssign, "Shorthand property assignments are valid only in destructuring patterns")
	}

	if doubleProto >= 0 {
		return true, p.raiseRecoverable(doubleProto, "Redefinition of __proto__ property")
	}
	return false, nil
}

func (p *Parser) parseSubscripts(base *Node, startPos int, startLoc *Location, noCalls bool, forInit string) (*Node, error) {
	maybeAsyncArrow, optionalChained := p.getEcmaVersion() >= 8 && base.Type == NODE_IDENTIFIER && base.Name == "async" &&
		p.LastTokEnd == base.End && !p.canInsertSemicolon() && base.End-base.Start == 5 &&
		p.PotentialArrowAt == base.Start, false

	for {
		element, err := p.parseSubscript(base, startPos, startLoc, noCalls, maybeAsyncArrow, optionalChained, forInit)

		if err != nil {
			return nil, err
		}

		if element.Optional {
			optionalChained = true
		}

		if element == base || element.Type == NODE_ARROW_FUNCTION_EXPRESSION {
			if optionalChained {
				chainNode := p.startNodeAt(startPos, startLoc)
				chainNode.Expression = element
				element = p.finishNode(chainNode, NODE_CHAIN_EXPRESSION)
			}
			return element, nil
		}
		base = element
	}
}

func (p *Parser) parseSubscript(base *Node, startPos int, startLoc *Location, noCalls bool, maybeAsyncArrow bool, optionalChained bool, forInit string) (*Node, error) {
	optionalSupported := p.getEcmaVersion() >= 11
	optional := optionalSupported && p.eat(TOKEN_QUESTIONDOT)

	if noCalls && optional {
		return nil, p.raise(p.LastTokStart, "Optional chaining cannot appear in the callee of new expressions")
	}

	computed := p.eat(TOKEN_BRACKETL)

	if computed || optional && p.Type.identifier != TOKEN_PARENL && p.Type.identifier != TOKEN_BACKQUOTE || p.eat(TOKEN_DOT) {
		node := p.startNodeAt(startPos, startLoc)
		node.Object = base
		if computed {
			prop, err := p.parseExpression("", nil)
			if err != nil {
				return nil, err
			}
			node.Property = prop
			err = p.expect(TOKEN_BRACKETR)

			if err != nil {
				return nil, err
			}
		} else if p.Type.identifier == TOKEN_PRIVATEID && base.Type != NODE_SUPER {
			privIdent, err := p.parsePrivateIdent()
			if err != nil {
				return nil, err
			}
			node.Property = privIdent
		} else {
			ident, err := p.parseIdent(p.options.AllowReserved != ALLOW_RESERVED_NEVER)
			if err != nil {
				return nil, err
			}
			node.Property = ident
		}
		node.Computed = computed
		if optionalSupported {
			node.Optional = optional
		}
		base = p.finishNode(node, NODE_MEMBER_EXPRESSION)
	} else if !noCalls && p.eat(TOKEN_PARENL) {
		refDestructuringErrors, oldYieldPos, oldAwaitPos, oldAwaitIdentPos := NewDestructuringErrors(), p.YieldPos, p.AwaitPos, p.AwaitIdentPos
		p.YieldPos = 0
		p.AwaitPos = 0
		p.AwaitIdentPos = 0
		exprList, err := p.parseExprList(TOKEN_PARENR, p.getEcmaVersion() >= 8, false, refDestructuringErrors)

		if err != nil {
			return nil, err
		}

		if maybeAsyncArrow && !optional && p.shouldParseAsyncArrow() {
			p.checkPatternErrors(refDestructuringErrors, false)
			p.checkYieldAwaitInDefaultParams()
			if p.AwaitIdentPos > 0 {
				return nil, p.raise(p.AwaitIdentPos, "Cannot use 'await' as identifier inside an async function")
			}

			p.YieldPos = oldYieldPos
			p.AwaitPos = oldAwaitPos
			p.AwaitIdentPos = oldAwaitIdentPos
			asyncArr, err := p.parseSubscriptAsyncArrow(startPos, startLoc, exprList, forInit)
			return asyncArr, err
		}

		_, err = p.checkExpressionErrors(refDestructuringErrors, true)

		if err != nil {
			return nil, err
		}

		if oldYieldPos != 0 {
			p.YieldPos = oldYieldPos
		}

		if oldAwaitPos != 0 {
			p.AwaitPos = oldAwaitPos
		}

		if oldAwaitIdentPos != 0 {
			p.AwaitIdentPos = oldAwaitIdentPos
		}
		node := p.startNodeAt(startPos, startLoc)
		node.Callee = base
		node.Arguments = exprList
		if optionalSupported {
			node.Optional = optional
		}
		base = p.finishNode(node, NODE_CALL_EXPRESSION)
	} else if p.Type.identifier == TOKEN_BACKQUOTE {
		if optional || optionalChained {
			return nil, p.raise(p.start, "Optional chaining cannot appear in the tag of tagged template expressions")
		}
		node := p.startNodeAt(startPos, startLoc)
		node.Tag = base
		tmpl, err := p.parseTemplate(struct{ isTagged bool }{isTagged: true})
		if err != nil {
			return nil, err
		}
		node.Quasi = tmpl
		base = p.finishNode(node, NODE_TAGGED_TEMPLATE_EXPRESSION)
	}
	return base, nil
}

func isLocalVariableAccess(node *Node) bool {
	return node.Type == NODE_IDENTIFIER ||
		node.Type == NODE_PARENTHESIZED_EXPRESSION && isLocalVariableAccess(node.Expression)
}

func (p *Parser) parseAwait(forInit string) (*Node, error) {
	if !(p.AwaitPos != 0) {
		p.AwaitPos = p.start
	}

	node := p.startNode()
	p.next(false)
	maybeUnary, err := p.parseMaybeUnary(nil, true, false, forInit)
	if err != nil {
		return nil, err
	}
	node.Argument = maybeUnary
	return p.finishNode(node, NODE_AWAIT_EXPRESSION), nil
}

func (p *Parser) parseExprSubscripts(refDestructuringErrors *DestructuringErrors, forInit string) (*Node, error) {
	startPos, startLoc := p.start, p.startLoc
	expr, err := p.parseExprAtom(refDestructuringErrors, forInit, false)

	if err != nil {
		return nil, err
	}
	if expr.Type == NODE_ARROW_FUNCTION_EXPRESSION && string(p.input[p.LastTokStart:p.LastTokEnd]) != ")" {
		return expr, nil

	}
	result, err := p.parseSubscripts(expr, startPos, startLoc, false, forInit)
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

func (p *Parser) buildBinary(startPos int, startLoc *Location, left *Node, right *Node, op BinaryOperator, logical bool) (*Node, error) {
	if right.Type == NODE_PRIVATE_IDENTIFIER {
		return nil, p.raise(right.Start, "Private identifier can only be left side of binary expression")
	}
	node := p.startNodeAt(startPos, startLoc)
	node.Left = left
	node.BinaryOperator = op
	node.Right = right
	if logical {
		return p.finishNode(node, NODE_LOGICAL_EXPRESSION), nil
	}
	return p.finishNode(node, NODE_BINARY_EXPRESSION), nil
}

func (p *Parser) parseMaybeUnary(refDestructuringErrors *DestructuringErrors, sawUnary bool, incDec bool, forInit string) (*Node, error) {
	startPos, startLoc := p.start, p.startLoc
	var expr *Node
	var err error

	if p.isContextual("await") && p.canAwait() {
		expr, err = p.parseAwait(forInit)
		if err != nil {
			return nil, err
		}
		sawUnary = true
	} else if p.Type.prefix {
		node, update := p.startNode(), p.Type.identifier == TOKEN_INCDEC
		if uop, ok := p.Value.([]byte); ok {
			node.UnaryOperator = UnaryOperator(uop)
		} else {
			panic("p.Value was not []byte as expected")
		}

		node.Prefix = true
		p.next(false)
		maybeUnary, err := p.parseMaybeUnary(nil, true, update, forInit)
		if err != nil {
			return nil, err
		}

		node.Argument = maybeUnary
		_, err = p.checkExpressionErrors(refDestructuringErrors, true)

		if err != nil {
			return nil, err
		}

		if update {
			err := p.checkLValSimple(node.Argument, 0, struct {
				check bool
				hash  map[string]bool
			}{check: false, hash: map[string]bool{}})
			if err != nil {
				return nil, err
			}
		} else if p.Strict && node.UnaryOperator == UNARY_DELETE && isLocalVariableAccess(node.Argument) {
			return nil, p.raiseRecoverable(node.Start, "Deleting local variable in strict mode")
		} else if node.UnaryOperator == UNARY_DELETE && isPrivateFieldAccess(node.Argument) {
			return nil, p.raiseRecoverable(node.Start, "Private fields can not be deleted")
		} else {
			sawUnary = true
		}

		if update {
			expr = p.finishNode(node, NODE_UPDATE_EXPRESSION)
		} else {
			expr = p.finishNode(node, NODE_UNARY_EXPRESSION)
		}

	} else if !sawUnary && p.Type.identifier == TOKEN_PRIVATEID {
		if len(forInit) != 0 || len(p.PrivateNameStack) == 0 && p.options.CheckPrivateFields {
			return nil, p.unexpected(`len(forInit) != 0 || len(p.PrivateNameStack) == 0 && p.options.CheckPrivateFields`, &p.pos)
		}
		expr, err = p.parsePrivateIdent()
		if err != nil {
			return nil, err
		}
		// only could be private fields in 'in', such as #x in obj
		if p.Type.identifier != TOKEN_IN {
			return nil, p.unexpected("`only could be private fields in 'in', such as #x in obj` what?", &p.pos)
		}
	} else {
		expr, err = p.parseExprSubscripts(refDestructuringErrors, forInit)
		if err != nil {
			return nil, err
		}
		hasExprError, _ := p.checkExpressionErrors(refDestructuringErrors, false)
		if hasExprError {
			return expr, nil
		}

		for p.Type.postfix && !p.canInsertSemicolon() {
			node := p.startNodeAt(startPos, startLoc)
			if val, ok := p.Value.([]byte); ok {
				node.UpdateOperator = UpdateOperator(val)
			} else {
				panic("We expected []byte")
			}
			node.Prefix = false
			node.Argument = expr
			err := p.checkLValSimple(expr, 0, struct {
				check bool
				hash  map[string]bool
			}{check: false, hash: map[string]bool{}})

			if err != nil {
				return nil, err
			}
			p.next(false)
			expr = p.finishNode(node, NODE_UPDATE_EXPRESSION)
		}
	}

	if !incDec && p.eat(TOKEN_STARSTAR) {
		if sawUnary {

			return nil, p.unexpected("we saw unary, which is wrong?", &p.LastTokStart)
		} else {
			unary, err := p.parseMaybeUnary(nil, false, false, forInit)
			if err != nil {
				return nil, err
			}

			binOp, errBinop := p.buildBinary(startPos, startLoc, expr, unary, EXPONENTIATION, false)
			if errBinop != nil {
				return nil, errBinop
			}
			return binOp, nil
		}
	} else {
		return expr, nil
	}
}

func (p *Parser) parseExprOps(forInit string, refDestructuringErrors *DestructuringErrors) (*Node, error) {
	startPos, startLoc := p.start, p.startLoc
	expr, err := p.parseMaybeUnary(refDestructuringErrors, false, false, forInit)
	if err != nil {
		return nil, err
	}
	exprErrors, _ := p.checkExpressionErrors(refDestructuringErrors, false)

	if exprErrors {
		return expr, nil
	}
	if expr.Start == startPos && expr.Type == NODE_ARROW_FUNCTION_EXPRESSION {
		return expr, nil
	}
	expr, err = p.parseExprOp(expr, startPos, startLoc, -1, forInit)
	if err != nil {
		return nil, err
	}
	return expr, nil

}

func (p *Parser) parseExprOp(left *Node, leftStartPos int, leftStartLoc *Location, minPrec int, forInit string) (*Node, error) {
	if p.Type.binop != nil && (len(forInit) == 0 || p.Type.identifier != TOKEN_IN) {
		prec := p.Type.binop.prec
		if p.Type.binop.prec > minPrec {
			logical := p.Type.identifier == TOKEN_LOGICALOR || p.Type.identifier == TOKEN_LOGICALAND
			coalesce := p.Type.identifier == TOKEN_COALESCE
			if coalesce {
				// Handle the precedence of `tt.coalesce` as equal to the range of logical expressions.
				// In other words, `node.right` shouldn't contain logical expressions in order to check the mixed error.
				prec = tokenTypes[TOKEN_LOGICALAND].binop.prec
			}
			if op, ok := p.Value.([]byte); ok {
				p.next(false)
				startPos, startLoc := p.start, p.startLoc
				unary, err := p.parseMaybeUnary(nil, false, false, forInit)
				if err != nil {
					return nil, err
				}
				right, err := p.parseExprOp(unary, startPos, startLoc, prec, forInit)

				if err != nil {
					return nil, err
				}
				node, err := p.buildBinary(leftStartPos, leftStartLoc, left, right, BinaryOperator(op), logical || coalesce)
				if err != nil {
					return nil, err
				}
				if (logical && p.Type.identifier == TOKEN_COALESCE) || (coalesce && (p.Type.identifier == TOKEN_LOGICALOR || p.Type.identifier == TOKEN_LOGICALAND)) {
					return nil, p.raiseRecoverable(p.start, "Logical expressions and coalesce expressions cannot be mixed. Wrap either by parentheses")
				}
				expr, err := p.parseExprOp(node, leftStartPos, leftStartLoc, minPrec, forInit)
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

func (p *Parser) parseYield(forInit string) (*Node, error) {
	if p.YieldPos == 0 {
		p.YieldPos = p.start
	}

	node := p.startNode()
	p.next(false)
	if p.Type.identifier == TOKEN_SEMI || p.canInsertSemicolon() || (p.Type.identifier != TOKEN_STAR && !p.Type.startsExpr) {
		node.Delegate = false
		node.Argument = nil
	} else {
		node.Delegate = p.eat(TOKEN_STAR)
		maybeAssign, err := p.parseMaybeAssign(forInit, nil, nil)
		if err != nil {
			return nil, err
		}
		node.Argument = maybeAssign
	}
	return p.finishNode(node, NODE_YIELD_EXPRESSION), nil
}

func (p *Parser) parseTemplate(opts struct{ isTagged bool }) (*Node, error) {
	node := p.startNode()
	p.next(false)
	node.Expressions = []*Node{}
	curElt, err := p.parseTemplateElement(opts)

	if err != nil {
		return nil, err
	}

	node.Quasis = []*Node{curElt}
	for !curElt.Tail {
		if p.Type.identifier == TOKEN_EOF {
			return nil, p.raise(p.pos, "Unterminated template literal")
		}
		err := p.expect(TOKEN_DOLLARBRACEL)
		if err != nil {
			return nil, err
		}
		n, err := p.parseExpression("", nil)

		if err != nil {
			return nil, err
		}
		node.Expressions = append(node.Expressions, n)
		err = p.expect(TOKEN_BRACER)
		if err != nil {
			return nil, err
		}
		curElt, err = p.parseTemplateElement(opts)
		if err != nil {
			return nil, err
		}
		node.Quasis = append(node.Quasis, curElt)
	}
	p.next(false)
	return p.finishNode(node, NODE_TEMPLATE_LITERAL), nil
}

func (p *Parser) parseTemplateElement(opts struct{ isTagged bool }) (*Node, error) {
	elem := p.startNode()
	if p.Type.identifier == TOKEN_INVALIDTEMPLATE {
		if !opts.isTagged {
			return nil, p.raiseRecoverable(p.start, "Bad escape sequence in untagged template literal")
		}

		elem.Value = struct {
			raw    string
			cooked string
		}{
			raw:    strings.ReplaceAll(string(p.Value.([]byte)), "\r\n", "\n"),
			cooked: "",
		}
	} else {
		elem.Value = struct {
			raw    string
			cooked string
		}{
			raw:    strings.ReplaceAll(string(p.input[p.start:p.End]), "\r\n", "\n"),
			cooked: string(p.Value.([]byte)),
		}
	}
	p.next(false)
	elem.Tail = p.Type.identifier == TOKEN_BACKQUOTE
	return p.finishNode(elem, NODE_TEMPLATE_ELEMENT), nil
}

func (p *Parser) parseSubscriptAsyncArrow(startPos int, startLoc *Location, exprList []*Node, forInit string) (*Node, error) {
	node := p.startNodeAt(startPos, startLoc)
	arrowExpression, err := p.parseArrowExpression(node, exprList, true, forInit)

	return arrowExpression, err
}

func (p *Parser) parseExprAtom(refDestructuringErrors *DestructuringErrors, forInit string, forNew bool) (*Node, error) {
	// If a division operator appears in an expression position, the
	// tokenizer got confused, and we force it to read a regexp instead.
	if p.Type.identifier == TOKEN_SLASH {
		err := p.readRegexp()
		if err != nil {
			return nil, err
		}
	}

	_, canBeArrow := p.PotentialArrowAt == p.start, p.PotentialArrowAt == p.start
	switch p.Type.identifier {
	case TOKEN_SUPER:
		if !p.allowSuper() {
			return nil, p.raise(p.start, "'super' keyword outside a method")
		}

		node := p.startNode()
		p.next(false)
		if p.Type.identifier == TOKEN_PARENL && !p.allowDirectSuper() {
			return nil, p.raise(node.Start, "super() call outside constructor of a subclass")
		}

		// The `super` keyword can appear at below:
		// SuperProperty:
		//     super [ Expression ]
		//     super . IdentifierName
		// SuperCall:
		//     super ( Arguments )

		if p.Type.identifier != TOKEN_DOT && p.Type.identifier != TOKEN_BRACKETL && p.Type.identifier != TOKEN_PARENL {
			return nil, p.unexpected(`p.Type.identifier != TOKEN_DOT && p.Type.identifier != TOKEN_BRACKETL && p.Type.identifier != TOKEN_PARENL`, nil)
		}

		return p.finishNode(node, NODE_SUPER), nil

	case TOKEN_THIS:
		node := p.startNode()
		p.next(false)
		return p.finishNode(node, NODE_THIS_EXPRESSION), nil

	case TOKEN_NAME:
		startPos, startLoc, containsEsc := p.start, p.startLoc, p.ContainsEsc

		id, err := p.parseIdent(false)
		if err != nil {
			return nil, err
		}
		if p.getEcmaVersion() >= 8 && !containsEsc && id.Name == "async" && !p.canInsertSemicolon() && p.eat(TOKEN_FUNCTION) {
			p.overrideContext(TokenContexts[FUNCTION_EXPRESSION])
			fun, err := p.parseFunction(p.startNodeAt(startPos, startLoc), 0, false, true, forInit)
			return fun, err
		}

		if canBeArrow && !p.canInsertSemicolon() {
			if p.eat(TOKEN_ARROW) {
				arrowExpr, err := p.parseArrowExpression(p.startNodeAt(startPos, startLoc), []*Node{id}, false, forInit)
				return arrowExpr, err
			}

			if p.getEcmaVersion() >= 8 && id.Name == "async" && p.Type.identifier == TOKEN_NAME && !containsEsc &&
				(!p.PotentialArrowInForAwait || p.Value != "of" || p.ContainsEsc) {
				id, err = p.parseIdent(false)
				if err != nil {
					return nil, err
				}

				if p.canInsertSemicolon() || !p.eat(TOKEN_ARROW) {
					return nil, p.unexpected(`if p.canInsertSemicolon() || !p.eat(TOKEN_ARROW)`, nil)
				}
				arrowExpr, err := p.parseArrowExpression(p.startNodeAt(startPos, startLoc), []*Node{id}, true, forInit)
				return arrowExpr, err
			}
		}
		return id, nil

	case TOKEN_REGEXP:
		panic("TOKEN_REGEXP not implemented")
		/*
			value := p.Value
			node = p.parseLiteral(value.value)
			node.regex = RegExpState{Pattern: value.pattern, flags: value.flags}
			return node
		*/

	case TOKEN_NUM, TOKEN_STRING:
		{
			literal, err := p.parseLiteral(p.Value)
			return literal, err
		}

	case TOKEN_NULL, TOKEN_TRUE, TOKEN_FALSE:
		node := p.startNode()
		if p.Type.identifier == TOKEN_NULL {
			node.Value = nil
		} else {
			node.Value = p.Type.identifier == TOKEN_TRUE
		}

		node.Raw = p.Type.keyword
		p.next(false)
		return p.finishNode(node, NODE_LITERAL), nil

	case TOKEN_PARENL:
		expr, err := p.parseParenAndDistinguishExpression(canBeArrow, forInit)
		if err != nil {
			return nil, err
		}

		start := p.start
		if refDestructuringErrors != nil {
			if refDestructuringErrors.parenthesizedAssign < 0 && !p.isSimpleAssignTarget(expr) {
				refDestructuringErrors.parenthesizedAssign = start
			}

			if refDestructuringErrors.parenthesizedBind < 0 {
				refDestructuringErrors.parenthesizedBind = start
			}

		}
		return expr, nil

	case TOKEN_BRACKETL:
		node := p.startNode()
		p.next(false)

		exprList, err := p.parseExprList(TOKEN_BRACKETR, true, true, refDestructuringErrors)

		if err != nil {
			return nil, err
		}

		node.Elements = exprList
		return p.finishNode(node, NODE_ARRAY_EXPRESSION), nil

	case TOKEN_BRACEL:
		p.overrideContext(TokenContexts[BRACKET_EXPRESSION])
		obj, err := p.parseObj(false, refDestructuringErrors)
		return obj, err

	case TOKEN_FUNCTION:
		node := p.startNode()
		p.next(false)
		fun, err := p.parseFunction(node, 0, false, false, "")
		return fun, err

	case TOKEN_CLASS:
		node := p.startNode()
		class, err := p.parseClass(node, false)
		return class, err

	case TOKEN_NEW:
		new, err := p.parseNew()
		return new, err

	case TOKEN_BACKQUOTE:
		tmpl, err := p.parseTemplate(struct{ isTagged bool }{isTagged: false})
		return tmpl, err

	case TOKEN_IMPORT:
		if p.getEcmaVersion() >= 11 {
			exprImport, err := p.parseExprImport(forNew)
			return exprImport, err
		} else {
			return nil, p.unexpected("Ecma version is too old", nil)
		}

	default:
		return nil, p.parseExprAtomDefault()

	}
}

func (p *Parser) parseExprAtomDefault() error {
	return p.unexpected("parseExprAtomDefault()", nil)
}

func (p *Parser) shouldParseAsyncArrow() bool {
	return !p.canInsertSemicolon() && p.eat(TOKEN_ARROW)
}

func (p *Parser) parseExprList(close Token, allowTrailingComma bool, allowEmpty bool, refDestructuringErrors *DestructuringErrors) ([]*Node, error) {
	elts, first := []*Node{}, true

	for !p.eat(close) {
		if !first {
			if err := p.expect(TOKEN_COMMA); err != nil {
				return nil, err
			}

			if allowTrailingComma && p.afterTrailingComma(close, false) {
				break
			}
		} else {
			first = false
		}

		var elt *Node
		if allowEmpty && p.Type.identifier == TOKEN_COMMA {
			elt = nil
		} else if p.Type.identifier == TOKEN_ELLIPSIS {
			spreadElement, err := p.parseSpread(refDestructuringErrors)

			if err != nil {
				return nil, err
			}

			if refDestructuringErrors != nil && p.Type.identifier == TOKEN_COMMA && refDestructuringErrors.trailingComma < 0 {
				refDestructuringErrors.trailingComma = p.start
			}
			elt = spreadElement

		} else {
			maybeAssign, err := p.parseMaybeAssign("", refDestructuringErrors, nil)
			if err != nil {
				return nil, err
			}
			elt = maybeAssign
		}
		elts = append(elts, elt)
	}
	return elts, nil
}

func (p *Parser) parseIdent(liberal bool) (*Node, error) {
	node, err := p.parseIdentNode()
	if err != nil {
		return nil, err
	}
	p.next(liberal)
	p.finishNode(node, NODE_IDENTIFIER)
	if !liberal {
		err := p.checkUnreserved(struct {
			start int
			end   int
			name  string
		}{start: node.Start, end: node.End, name: node.Name})

		if err != nil {
			return nil, err
		}

		if node.Name == "await" && !(p.AwaitIdentPos != 0) {
			p.AwaitIdentPos = node.Start
		}

	}
	return node, nil
}

func (p *Parser) parseIdentNode() (*Node, error) {
	node := p.startNode()
	if p.Type.identifier == TOKEN_NAME {
		if val, ok := p.Value.(string); ok {
			node.Name = val
		} else {
			panic("Theres a situation with node having a wrong type of .Value")
		}

	} else if len(p.Type.keyword) != 0 {
		node.Name = p.Type.keyword

		if (node.Name == "class" || node.Name == "function") &&
			(p.LastTokEnd != p.LastTokStart+1 || p.input[p.LastTokStart] != 46) {
			p.Context = p.Context[:len(p.Context)-1]
		}
		p.Type = tokenTypes[TOKEN_NAME]
	} else {
		return nil, p.unexpected("Keyword was not present, we want it", nil)
	}
	return node, nil
}

func (p *Parser) checkUnreserved(opts struct {
	start int
	end   int
	name  string
}) error {
	if p.inGenerator() && opts.name == "yield" {
		return p.raiseRecoverable(opts.start, "Cannot use 'yield' as identifier inside a generator")
	}

	if p.inAsync() && opts.name == "await" {
		return p.raiseRecoverable(opts.start, "Cannot use 'await' as identifier inside an async function")
	}
	curScope := p.currentThisScope()
	if !(curScope != nil && curScope.Flags&SCOPE_VAR == SCOPE_VAR) && opts.name == "arguments" {
		return p.raiseRecoverable(opts.start, "Cannot use 'arguments' in class field initializer")
	}

	if p.InClassStaticBlock && (opts.name == "arguments" || opts.name == "await") {
		return p.raise(opts.start, `Cannot use ${name} in class static initialization block`)
	}
	if p.Keywords.Match([]byte(opts.name)) {
		return p.raise(opts.start, "Unexpected keyword "+opts.name)
	}

	if p.getEcmaVersion() < 6 && strings.Index(string(p.input[opts.start:opts.end]), "\\") != -1 {
		return nil
	}
	var re *regexp.Regexp
	if p.Strict {
		re = p.ReservedWordsStrict
	} else {
		re = p.ReservedWords
	}

	if re.Match([]byte(opts.name)) {
		if !p.inAsync() && opts.name == "await" {
			return p.raiseRecoverable(opts.start, "Cannot use keyword 'await' outside an async function")
		}
		return p.raiseRecoverable(opts.start, "The keyword "+opts.name+" is reserved")
	}
	return nil
}

func (p *Parser) parseExprImport(forNew bool) (*Node, error) {
	node := p.startNode()

	// Consume `import` as an identifier for `import.meta`.
	// Because `p.parseIdent(true)` doesn't check escape sequences, it needs the check of `p.containsEsc`.
	if p.ContainsEsc {
		return nil, p.raiseRecoverable(p.start, "Escape sequence in keyword import")
	}
	p.next(false)

	if p.Type.identifier == TOKEN_PARENL && !forNew {
		dynImport, err := p.parseDynamicImport(node)
		return dynImport, err
	} else if p.Type.identifier == TOKEN_DOT {
		var loc *Location

		if node.Location != nil && node.Location.Start != nil {
			loc = node.Location.Start
		}
		meta := p.startNodeAt(node.Start, loc)
		meta.Name = "import"
		node.Meta = p.finishNode(meta, NODE_IDENTIFIER)
		importMeta, err := p.parseImportMeta(node)
		return importMeta, err
	} else {
		return nil, p.unexpected("", nil)
	}
}

func (p *Parser) parseImportMeta(node *Node) (*Node, error) {
	p.next(false) // skip `.`

	containsEsc := p.ContainsEsc
	ident, err := p.parseIdent(true)

	if err != nil {
		return nil, err
	}
	node.Property = ident

	if node.Property.Name != "meta" {
		return nil, p.raiseRecoverable(node.Property.Start, "The only valid meta property for import is 'import.meta'")
	}

	if containsEsc {
		return nil, p.raiseRecoverable(node.Start, "'import.meta' must not contain escaped characters")
	}

	if p.options.SourceType != "module" && !p.options.AllowImportExportEverywhere {
		return nil, p.raiseRecoverable(node.Start, "Cannot use 'import.meta' outside a module")
	}

	return p.finishNode(node, NODE_META_PROPERTY), nil
}

func (p *Parser) parseDynamicImport(node *Node) (*Node, error) {
	p.next(false)

	source, err := p.parseMaybeAssign("", nil, nil)
	if err != nil {
		return nil, err
	}
	node.Source = source

	if p.getEcmaVersion() >= 16 {
		if !p.eat(TOKEN_PARENR) {
			err := p.expect(TOKEN_COMMA)
			if err != nil {
				return nil, err
			}

			if !p.afterTrailingComma(TOKEN_PARENR, false) {
				opts, err := p.parseMaybeAssign("", nil, nil)
				if err != nil {
					return nil, err
				}
				node.Options = opts
				if !p.eat(TOKEN_PARENR) {
					err := p.expect(TOKEN_COMMA)
					if err != nil {
						return nil, err
					}
					if !p.afterTrailingComma(TOKEN_PARENR, false) {
						p.unexpected("trailing commas", nil)
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
		if !p.eat(TOKEN_PARENR) {
			errorPos := p.start
			if p.eat(TOKEN_COMMA) && p.eat(TOKEN_PARENR) {
				return nil, p.raiseRecoverable(errorPos, "Trailing comma is not allowed in import()")
			} else {
				return nil, p.unexpected("", &errorPos)
			}
		}
	}

	return p.finishNode(node, NODE_IMPORT_EXPRESSION), nil
}

func (p *Parser) parseLiteral(value any) (*Node, error) {
	node := p.startNode()
	node.Value = value

	node.Raw = string(p.input[p.start:p.End])
	if node.Raw[len(node.Raw)-1] == 110 { // big int stuff, maybe some day....
		node.Bigint = strings.ReplaceAll(node.Raw[:len(node.Raw)-1], "_", "")
		// node.bigint = node.raw.slice(0, -1).replace(/_/g, "")
	}
	p.next(false)
	return p.finishNode(node, NODE_LITERAL), nil
}

func (p *Parser) parsePrivateIdent() (*Node, error) {
	node := p.startNode()
	if p.Type.identifier == TOKEN_PRIVATEID {
		if val, ok := p.Value.(string); ok {
			node.Name = val
		} else {
			panic("In parsePrivateIdent() p.Value was not string as expected")
		}
	} else {
		return nil, p.unexpected("", &p.pos)
	}
	p.next(false)
	p.finishNode(node, NODE_PRIVATE_IDENTIFIER)

	if p.options.CheckPrivateFields {
		if len(p.PrivateNameStack) == 0 {
			p.raise(node.Start, "Private field #"+node.Name+" must be declared in an enclosing class")
		} else {
			p.PrivateNameStack[len(p.PrivateNameStack)-1].Used = append(p.PrivateNameStack[len(p.PrivateNameStack)-1].Used, node)
		}
	}

	return node, nil
}

func (p *Parser) parseParenAndDistinguishExpression(canBeArrow bool, forInit string) (*Node, error) {
	startPos, startLoc, allowTrailingComma := p.start, p.startLoc, p.getEcmaVersion() >= 8
	var val *Node
	if p.getEcmaVersion() >= 6 {
		p.next(false)

		innerStartPos, innerStartLoc := p.start, p.startLoc
		exprList, first, lastIsComma := []*Node{}, true, false
		refDestructuringErrors, oldYieldPos, oldAwaitPos, spreadStart := NewDestructuringErrors(), p.YieldPos, p.AwaitPos, 0
		p.YieldPos = 0
		p.AwaitPos = 0
		// Do not save awaitIdentPos to allow checking awaits nested in parameters
		for p.Type.identifier != TOKEN_PARENR {
			if first {
				first = false
			} else {
				err := p.expect(TOKEN_COMMA)
				if err != nil {
					return nil, err
				}
			}

			if allowTrailingComma && p.afterTrailingComma(TOKEN_PARENR, true) {
				lastIsComma = true
				break
			} else if p.Type.identifier == TOKEN_ELLIPSIS {
				spreadStart = p.start
				restBinding, err := p.parseRestBinding()
				if err != nil {
					return nil, err
				}

				parenItem, err := p.parseParenItem(restBinding)

				if err != nil {
					return nil, err
				}

				exprList = append(exprList, parenItem)

				if p.Type.identifier == TOKEN_COMMA {
					return nil, p.raiseRecoverable(
						p.start,
						"Comma is not permitted after the rest element",
					)
				}
				break
			} else {
				maybeAssign, err := p.parseMaybeAssign("", refDestructuringErrors, &struct {
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
		innerEndPos, innerEndLoc := p.LastTokEnd, p.LastTokEndLoc
		err := p.expect(TOKEN_PARENR)

		if err != nil {
			return nil, err
		}

		if canBeArrow && p.shouldParseArrow(exprList) && p.eat(TOKEN_ARROW) {
			err := p.checkPatternErrors(refDestructuringErrors, false)
			if err != nil {
				return nil, err
			}

			err = p.checkYieldAwaitInDefaultParams()
			if err != nil {
				return nil, err
			}

			p.YieldPos = oldYieldPos
			p.AwaitPos = oldAwaitPos
			parenArrowList, err := p.parseParenArrowList(startPos, startLoc, exprList, forInit)
			return parenArrowList, err
		}

		if len(exprList) == 0 || lastIsComma {
			return nil, p.unexpected("hanging comma", &p.LastTokStart)
		}

		if spreadStart != 0 {
			return nil, p.unexpected("", &spreadStart)
		}
		_, err = p.checkExpressionErrors(refDestructuringErrors, true)

		if err != nil {
			return nil, err
		}

		if oldYieldPos != 0 {
			p.YieldPos = oldYieldPos
		}

		if oldAwaitPos != 0 {
			p.AwaitPos = oldAwaitPos
		}

		if len(exprList) > 1 {
			val = p.startNodeAt(innerStartPos, innerStartLoc)
			val.Expressions = exprList
			p.finishNodeAt(val, NODE_SEQUENCE_EXPRESSION, innerEndPos, innerEndLoc)
		} else {
			val = exprList[0]
		}
	} else {
		parenExpr, err := p.parseParenExpression()

		if err != nil {
			return nil, err
		}
		val = parenExpr
	}

	if p.options.PreserveParens {
		par := p.startNodeAt(startPos, startLoc)
		par.Expression = val
		return p.finishNode(par, NODE_PARENTHESIZED_EXPRESSION), nil
	} else {
		return val, nil
	}
}

func (p *Parser) parseParenArrowList(startPos int, startLoc *Location, exprList []*Node, forInit string) (*Node, error) {
	arrExpr, err := p.parseArrowExpression(p.startNodeAt(startPos, startLoc), exprList, false, forInit)
	return arrExpr, err
}

func (p *Parser) shouldParseArrow(_ []*Node) bool {
	return !p.canInsertSemicolon()
}

func (p *Parser) parseParenItem(item *Node) (*Node, error) {
	return item, nil
}

func (p *Parser) parseParenExpression() (*Node, error) {
	err := p.expect(TOKEN_PARENL)
	if err != nil {
		return nil, err
	}
	val, errParse := p.parseExpression("", nil)
	if errParse != nil {
		return nil, err
	}
	err = p.expect(TOKEN_PARENR)

	if err != nil {
		return nil, err
	}
	return val, nil
}

func (p *Parser) parseNew() (*Node, error) {
	if p.ContainsEsc {
		return nil, p.raiseRecoverable(p.start, "Escape sequence in keyword new")
	}
	node := p.startNode()
	p.next(false)
	if p.getEcmaVersion() >= 6 && p.Type.identifier == TOKEN_DOT {

		var startLoc *Location

		if node.Location != nil {
			startLoc = node.Location.Start
		}
		meta := p.startNodeAt(node.Start, startLoc)
		meta.Name = "new"
		node.Meta = p.finishNode(meta, NODE_IDENTIFIER)
		p.next(false)
		containsEsc := p.ContainsEsc
		id, err := p.parseIdent(true)
		if err != nil {
			return nil, err
		}
		node.Property = id
		if node.Property.Name != "target" {
			return nil, p.raiseRecoverable(node.Property.Start, "The only valid meta property for new is 'new.target'")
		}

		if containsEsc {
			return nil, p.raiseRecoverable(node.Start, "'new.target' must not contain escaped characters")
		}

		if !p.allowNewDotTarget() {
			return nil, p.raiseRecoverable(node.Start, "'new.target' can only be used in functions and class static block")
		}

		return p.finishNode(node, NODE_META_PROPERTY), nil
	}
	startPos, startLoc := p.start, p.startLoc
	exprAtom, err := p.parseExprAtom(nil, "", true)
	if err != nil {
		return nil, err
	}

	subscript, errSubcript := p.parseSubscript(exprAtom, startPos, startLoc, true, false, false, "")

	if errSubcript != nil {
		return nil, errSubcript
	}
	node.Callee = subscript
	if p.eat(TOKEN_PARENL) {
		exprList, err := p.parseExprList(TOKEN_PARENR, p.getEcmaVersion() >= 8, false, nil)
		if err != nil {
			return nil, err
		}

		node.Arguments = exprList
	} else {
		node.Arguments = []*Node{}
	}
	return p.finishNode(node, NODE_NEW_EXPRESSION), nil
}

func (p *Parser) parseArrowExpression(node *Node, params []*Node, isAsync bool, forInit string) (*Node, error) {
	oldYieldPos, oldAwaitPos, oldAwaitIdentPos := p.YieldPos, p.AwaitPos, p.AwaitIdentPos

	p.enterScope(functionFlags(isAsync, false) | SCOPE_ARROW)
	p.initFunction(node)

	node.IsAsync = isAsync

	p.YieldPos = 0
	p.AwaitPos = 0
	p.AwaitIdentPos = 0

	listParams, err := p.toAssignableList(params, true)

	if err != nil {
		return nil, err
	}
	node.Params = listParams
	err = p.parseFunctionBody(node, true, false, forInit)
	if err != nil {
		return nil, err
	}

	p.YieldPos = oldYieldPos
	p.AwaitPos = oldAwaitPos
	p.AwaitIdentPos = oldAwaitIdentPos

	return p.finishNode(node, NODE_ARROW_FUNCTION_EXPRESSION), nil
}

func (p *Parser) parseFunctionBody(node *Node, isArrowFunction bool, isMethod bool, forInit string) error {
	isExpression := isArrowFunction && p.Type.identifier != TOKEN_BRACEL
	oldStrict, useStrict := p.Strict, false
	if isExpression {
		maybeAssign, err := p.parseMaybeAssign(forInit, nil, nil)
		if err != nil {
			return err
		}
		node.BodyNode = maybeAssign
		node.IsExpression = true
		err = p.checkParams(node, false)
		if err != nil {
			return err
		}
	} else {
		nonSimple := p.getEcmaVersion() >= 7 && !p.isSimpleParamList(node.Params)
		if !oldStrict || nonSimple {
			useStrict = p.strictDirective(p.End)
			// If this is a strict mode function, verify that argument names
			// are not repeated, and it does not try to bind the words `eval`
			// or `arguments`.
			if useStrict && nonSimple {
				return p.raiseRecoverable(node.Start, "Illegal 'use strict' directive in function with non-simple parameter list")
			}

		}
		// Start a new scope with regard to labels and the `inFunction`
		// flag (restore them to their old value afterwards).
		oldLabels := p.Labels
		p.Labels = []Label{}
		if useStrict {
			p.Strict = true
		}

		// Add the params to varDeclaredNames to ensure that an error is thrown
		// if a let/const declaration in the function clashes with one of the params.
		err := p.checkParams(node, !oldStrict && !useStrict && !isArrowFunction && !isMethod && p.isSimpleParamList(node.Params))

		if err != nil {
			return err
		}
		// Ensure the function name isn't a forbidden identifier in strict mode, e.g. 'eval'
		if p.Strict && node.Identifier != nil {
			err := p.checkLValSimple(node.Identifier, BIND_OUTSIDE, struct {
				check bool
				hash  map[string]bool
			}{})

			if err != nil {
				return err
			}
		}
		block, err := p.parseBlock(false, nil, useStrict && !oldStrict)
		if err != nil {
			return err
		}
		node.BodyNode = block
		node.IsExpression = false
		p.adaptDirectivePrologue(node.BodyNode.Body)
		p.Labels = oldLabels
	}
	p.exitScope()
	return nil
}

func (p *Parser) isSimpleParamList(params []*Node) bool {
	for _, param := range params {
		if param.Type != NODE_IDENTIFIER {
			return false
		}
	}
	return true
}

func (p *Parser) checkParams(node *Node, allowDuplicates bool) error {
	// nameHash = Object.create(null), let's see if I got this right....
	for _, param := range node.Params {
		if allowDuplicates {
			err := p.checkLValInnerPattern(param, BIND_VAR, struct {
				check bool
				hash  map[string]bool
			}{})
			if err != nil {
				return err
			}
		} else {
			err := p.checkLValInnerPattern(param, BIND_VAR, struct {
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

func (p *Parser) initFunction(node *Node) {
	node.Identifier = nil
	if p.getEcmaVersion() >= 6 {
		node.IsGenerator = false
		node.IsExpression = false
	}

	if p.getEcmaVersion() >= 8 {
		node.IsAsync = false
	}
}

func (p *Parser) parseMethod(isGenerator bool, isAsync bool, allowDirectSuper bool) (*Node, error) {
	node, oldYieldPos, oldAwaitPos, oldAwaitIdentPos := p.startNode(), p.YieldPos, p.AwaitPos, p.AwaitIdentPos

	p.initFunction(node)
	node.IsGenerator = isGenerator

	node.IsAsync = isAsync

	p.YieldPos = 0
	p.AwaitPos = 0
	p.AwaitIdentPos = 0

	flags := functionFlags(isAsync, node.IsGenerator) | SCOPE_SUPER

	if allowDirectSuper {
		p.enterScope(flags | SCOPE_DIRECT_SUPER)
	} else {
		p.enterScope(flags)
	}

	err := p.expect(TOKEN_PARENL)
	if err != nil {
		return nil, err
	}

	bindingList, errBindingList := p.parseBindingList(TOKEN_PARENR, false, p.getEcmaVersion() >= 8, false)
	if errBindingList != nil {
		return nil, errBindingList
	}
	node.Params = bindingList
	err = p.checkYieldAwaitInDefaultParams()
	if err != nil {
		return nil, err
	}

	err = p.parseFunctionBody(node, false, true, "")

	if err != nil {
		return nil, err
	}

	p.YieldPos = oldYieldPos
	p.AwaitPos = oldAwaitPos
	p.AwaitIdentPos = oldAwaitIdentPos
	return p.finishNode(node, NODE_FUNCTION_EXPRESSION), nil
}

func (p *Parser) parseObj(isPattern bool, refDestructuringErrors *DestructuringErrors) (*Node, error) {
	node, first, propHash := p.startNode(), true, &PropertyHash{proto: false, m: map[string]map[Kind]bool{}}
	node.Properties = []*Node{}
	p.next(false)
	for !p.eat(TOKEN_BRACER) {
		if !first {
			err := p.expect(TOKEN_COMMA)
			if err != nil {
				return nil, err
			}
			if p.getEcmaVersion() >= 5 && p.afterTrailingComma(TOKEN_BRACER, false) {
				break
			}
		} else {
			first = false
		}
		prop, err := p.parseProperty(isPattern, refDestructuringErrors)
		if err != nil {
			return nil, err
		}
		if !isPattern {
			err := p.checkPropClash(prop, propHash, refDestructuringErrors)
			if err != nil {
				return nil, err
			}
		}
		node.Properties = append(node.Properties, prop)
	}

	if isPattern {
		return p.finishNode(node, NODE_OBJECT_PATTERN), nil
	}
	return p.finishNode(node, NODE_OBJECT_EXPRESSION), nil
}

func (p *Parser) parseProperty(isPattern bool, refDestructuringErrors *DestructuringErrors) (*Node, error) {
	prop := p.startNode()
	isGenerator, isAsync, startPos := false, false, 0
	var startLoc *Location

	if p.getEcmaVersion() >= 9 && p.eat(TOKEN_ELLIPSIS) {
		if isPattern {
			ident, err := p.parseIdent(false)
			if err != nil {
				return nil, err
			}

			prop.Argument = ident
			if p.Type.identifier == TOKEN_COMMA {
				return nil, p.raiseRecoverable(p.start, "Comma is not permitted after the rest element")
			}
			return p.finishNode(prop, NODE_REST_ELEMENT), nil
		}
		// Parse argument.
		maybeAssign, err := p.parseMaybeAssign("", refDestructuringErrors, nil)
		if err != nil {
			return nil, err
		}
		prop.Argument = maybeAssign
		// To disallow trailing comma via `p.toAssignable()`.
		if p.Type.identifier == TOKEN_COMMA && refDestructuringErrors != nil && refDestructuringErrors.trailingComma < 0 {
			refDestructuringErrors.trailingComma = p.start
		}
		// Finish
		return p.finishNode(prop, NODE_SPREAD_ELEMENT), nil
	}
	if p.getEcmaVersion() >= 6 {
		prop.IsMethod = false
		prop.Shorthand = false
		if isPattern || refDestructuringErrors != nil {
			startPos = p.start
			startLoc = p.startLoc
		}
		if !isPattern {
			isGenerator = p.eat(TOKEN_STAR)
		}

	}
	containsEsc := p.ContainsEsc
	_, err := p.parsePropertyName(prop)

	if err != nil {
		return nil, err
	}

	if !isPattern && !containsEsc && p.getEcmaVersion() >= 8 && !isGenerator && p.isAsyncProp(prop) {
		isAsync = true
		isGenerator = p.getEcmaVersion() >= 9 && p.eat(TOKEN_STAR)
		_, err := p.parsePropertyName(prop)

		if err != nil {
			return nil, err
		}
	} else {
		isAsync = false
	}
	err = p.parsePropertyValue(prop, isPattern, isGenerator, isAsync, startPos, startLoc, refDestructuringErrors, containsEsc)
	if err != nil {
		return nil, err
	}
	return p.finishNode(prop, NODE_PROPERTY), nil
}

func (p *Parser) parsePropertyValue(prop *Node, isPattern bool, isGenerator bool, isAsync bool, startPos int, startLoc *Location, refDestructuringErrors *DestructuringErrors, containsEsc bool) error {
	if (isGenerator || isAsync) && p.Type.identifier == TOKEN_COLON {
		return p.unexpected("", nil)
	}

	if p.eat(TOKEN_COLON) {
		prop.Kind = KIND_PROPERTY_INIT
		if isPattern {
			val, err := p.parseMaybeDefault(p.start, p.startLoc, nil)
			if err != nil {
				return err
			}
			prop.Value = val
		} else {
			val, err := p.parseMaybeAssign("", refDestructuringErrors, nil)
			if err != nil {
				return err
			}
			prop.Value = val
		}
	} else if p.getEcmaVersion() >= 6 && p.Type.identifier == TOKEN_PARENL {
		if isPattern {
			return p.unexpected("", nil)
		}
		method, err := p.parseMethod(isGenerator, isAsync, false)
		if err != nil {
			return err
		}
		prop.IsMethod = true
		prop.Kind = KIND_PROPERTY_INIT
		prop.Value = method
	} else if !isPattern && !containsEsc &&
		p.getEcmaVersion() >= 5 && !prop.Computed && prop.Key.Type == NODE_IDENTIFIER &&
		(prop.Key.Name == "get" || prop.Key.Name == "set") &&
		(p.Type.identifier != TOKEN_COMMA && p.Type.identifier != TOKEN_BRACER && p.Type.identifier != TOKEN_EQ) {
		if isGenerator || isAsync {
			return p.unexpected("", nil)
		}
		err := p.parseGetterSetter(prop)
		if err != nil {
			return err
		}
	} else if p.getEcmaVersion() >= 6 && !prop.Computed && prop.Key.Type == NODE_IDENTIFIER {
		if isGenerator || isAsync {
			return p.unexpected("", nil)
		}
		err := p.checkUnreserved(struct {
			start int
			end   int
			name  string
		}{start: prop.Start, end: prop.End, name: prop.Name})
		if err != nil {
			return err
		}
		if prop.Key.Name == "await" && !(p.AwaitIdentPos != 0) {
			p.AwaitIdentPos = startPos
		}

		if isPattern {
			c, err := p.copyNode(prop.Key)
			if err != nil {
				return err
			}
			val, err := p.parseMaybeDefault(startPos, startLoc, c)
			if err != nil {
				return err
			}
			prop.Value = val
		} else if p.Type.identifier == TOKEN_EQ && refDestructuringErrors != nil {
			if refDestructuringErrors.shorthandAssign < 0 {
				refDestructuringErrors.shorthandAssign = p.start
			}
			c, err := p.copyNode(prop.Key)
			if err != nil {
				return err
			}
			val, err := p.parseMaybeDefault(startPos, startLoc, c)
			if err != nil {
				return err
			}
			prop.Value = val
		} else {
			c, err := p.copyNode(prop.Key)
			if err != nil {
				return err
			}
			prop.Value = c
		}
		prop.Kind = KIND_PROPERTY_INIT
		prop.Shorthand = true
	} else {
		return p.unexpected("", nil)
	}
	return nil
}

func (p *Parser) parseGetterSetter(prop *Node) error {
	kind := KIND_NOT_INITIALIZED

	switch prop.Key.Name {
	case "set":
		kind = KIND_PROPERTY_SET
	case "get":
		kind = KIND_PROPERTY_GET
	}

	p.parsePropertyName(prop)
	method, err := p.parseMethod(false, false, false)
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
				return p.raiseRecoverable(start, "getter should have no params")
			} else {
				return p.raiseRecoverable(start, "setter should have exactly one param")
			}
		} else {
			if prop.Kind == KIND_PROPERTY_SET && val.Params[0].Type == NODE_REST_ELEMENT {
				return p.raiseRecoverable(val.Params[0].Start, "Setter cannot use rest params")
			}
		}
	} else {
		panic("prop.Value was not *Node as we expected, we are in parseGetterSetter")
	}
	return nil
}

func (p *Parser) isAsyncProp(prop *Node) bool {
	return !prop.Computed && prop.Key.Type == NODE_IDENTIFIER && prop.Key.Name == "async" &&
		(p.Type.identifier == TOKEN_NAME || p.Type.identifier == TOKEN_NUM || p.Type.identifier == TOKEN_STRING || p.Type.identifier == TOKEN_BRACKETL || len(p.Type.keyword) != 0 || (p.getEcmaVersion() >= 9 && p.Type.identifier == TOKEN_STAR)) &&
		!lineBreak.Match(p.input[p.LastTokEnd:p.start])
}

func (p *Parser) parsePropertyName(prop *Node) (*Node, error) {
	if p.getEcmaVersion() >= 6 {
		if p.eat(TOKEN_BRACKETL) {
			prop.Computed = true
			maybeAssign, err := p.parseMaybeAssign("", nil, nil)
			if err != nil {
				return nil, err
			}
			prop.Key = maybeAssign
			err = p.expect(TOKEN_BRACKETR)

			if err != nil {
				return nil, err
			}
			return prop.Key, nil
		} else {
			prop.Computed = false
		}
	}
	if p.Type.identifier == TOKEN_NUM || p.Type.identifier == TOKEN_STRING {
		exprAtom, err := p.parseExprAtom(nil, "", false)
		if err != nil {
			return nil, err
		}
		prop.Key = exprAtom
		return prop.Key, nil
	} else {
		ident, err := p.parseIdent(p.options.AllowReserved != ALLOW_RESERVED_NEVER)
		if err != nil {
			return nil, err
		}
		prop.Key = ident
		return prop.Key, nil
	}
}
