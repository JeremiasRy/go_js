package parser

import (
	"regexp"
	"strings"
)

// EXPRESION PARSING
func (this *Parser) checkPropClash(prop *Node, propHash *PropertyHash, refDestructuringErrors *DestructuringErrors) error {
	if this.getEcmaVersion() >= 9 && prop.Type == NODE_SPREAD_ELEMENT {
		return nil
	}

	if this.getEcmaVersion() >= 6 && (prop.Computed || (prop.Method != nil && *prop.Method) || (prop.Shorthand != nil && *prop.Shorthand)) {
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

	kind := prop.PropertyKind

	if this.getEcmaVersion() >= 6 {
		if name == "__proto__" && *kind == INIT {
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
		if *kind == INIT {
			redefinition = this.Strict && other[INIT] || other[GET] || other[SET]
		} else {
			redefinition = other[INIT] || other[*kind]
		}
		if redefinition {
			this.raiseRecoverable(key.Start, "Redefinition of property")
		}
	} else {
		newInfo := map[PropertyKind]bool{
			INIT: false,
			GET:  false,
			SET:  false,
		}
		newInfo[*kind] = true
		propHash.m[name] = newInfo
	}

	return nil
}

func (this *Parser) parseExpression(forInit string, refDestructuringErrors *DestructuringErrors) (*Node, error) {
	startPos, startLoc := this.start, this.startLoc
	expr, err := this.parseMaybeAssign(forInit, refDestructuringErrors)

	if err != nil {
		return nil, err
	}
	if this.Type.identifier == TOKEN_COMMA {
		node := this.startNodeAt(startPos, startLoc)
		node.Expressions = []*Node{expr}

		for this.eat(TOKEN_COMMA) {
			maybeAssign, err := this.parseMaybeAssign(forInit, refDestructuringErrors)
			if err != nil {
				return nil, err
			}
			node.Expressions = append(node.Expressions, maybeAssign)
		}

		return this.finishNode(node, NODE_SEQUENCE_EXPRESSION), nil
	}
	return expr, nil
}

func (this *Parser) parseMaybeAssign(forInit string, refDestructuringErrors *DestructuringErrors) (*Node, error) {
	if this.isContextual("yield") {
		if this.inGeneratorContext() {
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

	/* what ??
	 if afterLeftParse {
		left = afterLeftParse.call(this, left, startPos, startLoc)
	 }
	*/

	if this.Type.isAssign {
		node := this.startNodeAt(startPos, startLoc)
		node.AssignmentOperator = this.Value.(*AssignmentOperator)
		if this.Type.identifier == TOKEN_EQ {
			left = this.toAssignable(left, false, refDestructuringErrors)
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
		node.Rigth, err = this.parseMaybeAssign(forInit, refDestructuringErrors)

		if err != nil {
			return nil, err
		}

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
		maybeAssign, err := this.parseMaybeAssign("", nil)
		if err != nil {
			return nil, err
		}
		node.Consequent = maybeAssign

		errExpect := this.expect(TOKEN_COLON)
		if errExpect != nil {
			return nil, err
		}

		maybeAssignElse, errElse := this.parseMaybeAssign(forInit, nil)
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
			ident, err := this.parseIdent(this.options.AllowReserved)
			if err != nil {
				return nil, err
			}
			node.Property = ident
		}
		node.Computed = !computed
		if optionalSupported {
			node.Optional = optional
		}
		base = this.finishNode(node, NODE_MEMBER_EXPRESSION)
	} else if !noCalls && this.eat(TOKEN_PARENL) {
		refDestructuringErrors, oldYieldPos, oldAwaitPos, oldAwaitIdentPos := NewDestructuringErrors(), this.YieldPos, this.AwaitPos, this.AwaitIdentPos
		this.YieldPos = 0
		this.AwaitPos = 0
		this.AwaitIdentPos = 0
		exprList, err := this.parseExprList(TOKEN_PARENL, this.getEcmaVersion() >= 8, false, refDestructuringErrors)

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
	panic("unimplemented")
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
	node.BinaryOperator = &op
	node.Rigth = right
	if logical {
		return this.finishNode(node, NODE_LOGICAL_EXPRESSION), nil
	}
	return this.finishNode(node, NODE_BINARY_EXPRESSION), nil
}

func (this *Parser) parseMaybeUnary(refDestructuringErrors *DestructuringErrors, sawUnary bool, incDec bool, forInit string) (*Node, error) {
	startPos, startLoc := this.start, this.startLoc
	var expr *Node
	var err error

	if this.isContextual("await") && this.CanAwait {
		expr, err = this.parseAwait(forInit)
		sawUnary = true
	} else if this.Type.prefix {
		node, update := this.startNode(), this.Type.identifier == TOKEN_INCDEC
		node.UnaryOperator = this.Value.(*UnaryOperator)
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
		} else if this.Strict && *node.UnaryOperator == UNARY_DELETE && isLocalVariableAccess(node.Argument) {
			return nil, this.raiseRecoverable(node.Start, "Deleting local variable in strict mode")
		} else if *node.UnaryOperator == UNARY_DELETE && isPrivateFieldAccess(node.Argument) {
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
			return nil, this.unexpected(&this.pos)
		}
		expr, err = this.parsePrivateIdent()
		if err != nil {
			return nil, err
		}
		// only could be private fields in 'in', such as #x in obj
		if this.Type.identifier != TOKEN_IN {
			return nil, this.unexpected(&this.pos)
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
			node.UpdateOperator = this.Value.(*UpdateOperator)
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

			return nil, this.unexpected(&this.LastTokStart)
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
	} else {
		expr, err := this.parseExprOp(expr, startPos, startLoc, -1, forInit)
		return expr, err
	}
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
			if op, ok := this.Value.(BinaryOperator); ok {
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
				node, err := this.buildBinary(leftStartPos, leftStartLoc, left, right, op, logical || coalesce)
				if err != nil {
					return nil, err
				}
				if (logical && this.Type.identifier == TOKEN_COALESCE) || (coalesce && (this.Type.identifier == TOKEN_LOGICALOR || this.Type.identifier == TOKEN_LOGICALAND)) {
					return nil, this.raiseRecoverable(this.start, "Logical expressions and coalesce expressions cannot be mixed. Wrap either by parentheses")
				}
				expr, err := this.parseExprOp(node, leftStartPos, leftStartLoc, minPrec, forInit)
				return expr, err
			} else {
				panic("Node had invalid operator as Value, expected BinaryOperator")
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
		maybeAssign, err := this.parseMaybeAssign(forInit, nil)
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
			raw:    strings.ReplaceAll(this.Value.(string), "\r\n", "\n"),
			cooked: "",
		}
	} else {
		elem.Value = struct {
			raw    string
			cooked string
		}{
			raw:    strings.ReplaceAll(string(this.input[this.start:this.End]), "\r\n", "\n"),
			cooked: this.Value.(string),
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
		if !this.AllowSuper {
			return nil, this.raise(this.start, "'super' keyword outside a method")
		}

		node := this.startNode()
		this.next(false)
		if this.Type.identifier == TOKEN_PARENL && !this.AllowDirectSuper {
			return nil, this.raise(node.Start, "super() call outside constructor of a subclass")
		}

		// The `super` keyword can appear at below:
		// SuperProperty:
		//     super [ Expression ]
		//     super . IdentifierName
		// SuperCall:
		//     super ( Arguments )

		if this.Type.identifier != TOKEN_DOT && this.Type.identifier != TOKEN_BRACKETL && this.Type.identifier != TOKEN_PARENL {
			return nil, this.unexpected(nil)
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
					return nil, this.unexpected(nil)
				}
				arrowExpr, err := this.parseArrowExpression(this.startNodeAt(startPos, startLoc), []*Node{id}, true, forInit)
				return arrowExpr, err
			}
		}
		return id, nil
		/*
		   Skipped for now...

		     case TOKEN_REGEXP:
		       value := this.Value
		       node = this.parseLiteral(value.value)
		       node.regex = {pattern: value.pattern, flags: value.flags}
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
			return nil, this.unexpected(nil)
		}

	default:
		return nil, this.parseExprAtomDefault()

	}
}

func (p *Parser) parseExprAtomDefault() error {
	return p.unexpected(nil)
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
			maybeAssign, err := this.parseMaybeAssign("", refDestructuringErrors)
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

	this.next(!liberal)
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
		return nil, this.unexpected(nil)
	}
	return node, nil
}

func (this *Parser) checkUnreserved(opts struct {
	start int
	end   int
	name  string
}) error {
	if this.InGenerator && opts.name == "yield" {
		return this.raiseRecoverable(opts.start, "Cannot use 'yield' as identifier inside a generator")
	}

	if this.InAsync && opts.name == "await" {
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
		if !this.InAsync && opts.name == "await" {
			return this.raiseRecoverable(opts.start, "Cannot use keyword 'await' outside an async function")
		}

		return this.raiseRecoverable(opts.start, `The keyword '${name}' is reserved`)
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

		if node.Loc != nil && node.Loc.Start != nil {
			loc = node.Loc.Start
		}
		meta := this.startNodeAt(node.Start, loc)
		meta.Name = "import"
		node.Meta = this.finishNode(meta, NODE_IDENTIFIER)
		importMeta, err := this.parseImportMeta(node)
		return importMeta, err
	} else {
		return nil, this.unexpected(nil)
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
	this.next(false) // skip `(`

	// Parse node.source.
	source, err := this.parseMaybeAssign("", nil)
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
				opts, err := this.parseMaybeAssign("", nil)
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
						this.unexpected(nil)
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
				return nil, this.unexpected(&errorPos)
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
		// node.bigint = node.raw.slice(0, -1).replace(/_/g, "")
	}
	this.next(false)
	return this.finishNode(node, NODE_LITERAL), nil
}
