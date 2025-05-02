package parser

func (this *Parser) toAssignable(node *Node, isBinding bool, refDestructuringErrors *DestructuringErrors) (*Node, error) {
	if this.getEcmaVersion() >= 6 && node != nil {
		switch node.type_ {
		case NODE_IDENTIFIER:
			if this.inAsync() && node.name == "await" {
				return nil, this.raise(node.start, "Cannot use 'await' as identifier inside an async function")
			}

		case NODE_OBJECT_PATTERN, NODE_ARRAY_PATTERN, NODE_ASSIGNMENT_PATTERN, NODE_REST_ELEMENT:
		case NODE_OBJECT_EXPRESSION:
			node.type_ = NODE_OBJECT_PATTERN
			if refDestructuringErrors != nil {
				if err := this.checkPatternErrors(refDestructuringErrors, true); err != nil {
					return nil, err
				}
			}

			for _, prop := range node.properties {
				_, err := this.toAssignable(prop, isBinding, nil)

				if err != nil {
					return nil, err
				}
				if prop.type_ == NODE_REST_ELEMENT &&
					(prop.argument.type_ == NODE_ARRAY_PATTERN || prop.argument.type_ == NODE_OBJECT_PATTERN) {
					return nil, this.raise(prop.alternate.start, "Unexpected token")
				}
			}

		case NODE_PROPERTY:
			// AssignmentProperty has type == "Property"
			if node.kind != KIND_PROPERTY_INIT {
				return nil, this.raise(node.key.start, "Object pattern can't contain getter or setter")
			}

			if val, ok := node.value.(*Node); ok {
				_, err := this.toAssignable(val, isBinding, nil)
				if err != nil {
					return nil, err
				}
			} else {
				panic("In toAsignable() we were expecting node.Value to be *Node")
			}

		case NODE_ARRAY_EXPRESSION:
			node.type_ = NODE_ARRAY_PATTERN
			if refDestructuringErrors != nil {
				err := this.checkPatternErrors(refDestructuringErrors, true)

				if err != nil {
					return nil, err
				}
			}
			_, err := this.toAssignableList(node.elements, isBinding)
			if err != nil {
				return nil, err
			}

		case NODE_SPREAD_ELEMENT:
			node.type_ = NODE_REST_ELEMENT
			_, err := this.toAssignable(node.argument, isBinding, nil)
			if err != nil {
				return nil, err
			}
			if node.argument.type_ == NODE_ASSIGNMENT_PATTERN {
				return nil, this.raise(node.argument.start, "Rest elements cannot have a default value")
			}

		case NODE_ASSIGNMENT_EXPRESSION:
			if node.assignmentOperator != ASSIGN {
				return nil, this.raise(node.left.end, "Only '=' operator can be used for specifying default value.")
			}
			node.type_ = NODE_ASSIGNMENT_PATTERN
			node.assignmentOperator = ""
			_, err := this.toAssignable(node.left, isBinding, nil)

			if err != nil {
				return nil, err
			}

		case NODE_PARENTHESIZED_EXPRESSION:
			_, err := this.toAssignable(node.expression, isBinding, refDestructuringErrors)
			if err != nil {
				return nil, err
			}

		case NODE_CHAIN_EXPRESSION:
			return nil, this.raiseRecoverable(node.start, "Optional chaining cannot appear in left-hand side")

		case NODE_MEMBER_EXPRESSION:
			if !isBinding {
				break
			}
			fallthrough

		default:
			return nil, this.raise(node.start, "Assigning to rvalue")
		}
	} else if refDestructuringErrors != nil {
		err := this.checkPatternErrors(refDestructuringErrors, true)
		if err != nil {
			return nil, err
		}
	}
	return node, nil
}

func (this *Parser) toAssignableList(exprList []*Node, isBinding bool) ([]*Node, error) {
	end := len(exprList)

	for i := 0; i < end; i++ {
		elt := exprList[i]
		if elt != nil {
			_, err := this.toAssignable(elt, isBinding, nil)
			if err != nil {
				return nil, err
			}
		}
	}

	if end != 0 {
		last := exprList[end-1]
		if this.getEcmaVersion() == 6 && isBinding && last != nil && last.type_ == NODE_REST_ELEMENT && last.argument.type_ != NODE_IDENTIFIER {
			return nil, this.unexpected(`if this.getEcmaVersion() == 6 && isBinding && last != nil && last.Type == NODE_REST_ELEMENT && last.Argument.Type != NODE_IDENTIFIER`, &last.argument.start)
		}

	}
	return exprList, nil
}

// send 0 for bindingType if not used in acorn function calls
func (this *Parser) checkLValSimple(expr *Node, bindingType Flags, checkClashes struct {
	check bool
	hash  map[string]bool
}) error {
	isBind := bindingType != BIND_NONE

	switch expr.type_ {
	case NODE_IDENTIFIER:
		if this.Strict && this.ReservedWordsStrictBind.Match([]byte(expr.name)) {
			msg := ""
			if isBind {
				msg += "Binding "
			} else {
				msg += "Assigning to "
			}

			msg += expr.name
			return this.raiseRecoverable(expr.start, msg+" in strict mode")
		}

		if isBind {
			if bindingType == BIND_LEXICAL && expr.name == "let" {
				return this.raiseRecoverable(expr.start, "let is disallowed as a lexically bound name")
			}

			if checkClashes.check {
				if _, has := checkClashes.hash[expr.name]; has {
					return this.raiseRecoverable(expr.start, "Argument name clash")
				}

				checkClashes.hash[expr.name] = true
			}
			if bindingType != BIND_OUTSIDE {
				return this.declareName(expr.name, bindingType, expr.start)
			}
		}

	case NODE_CHAIN_EXPRESSION:
		return this.raiseRecoverable(expr.start, "Optional chaining cannot appear in left-hand side")

	case NODE_MEMBER_EXPRESSION:
		if isBind {
			return this.raiseRecoverable(expr.start, "Binding member expression")
		}

	case NODE_PARENTHESIZED_EXPRESSION:
		if isBind {
			return this.raiseRecoverable(expr.start, "Binding parenthesized expression")
		}
		return this.checkLValSimple(expr.expression, bindingType, checkClashes)

	default:
		msg := ""
		if isBind {
			msg += "Binding"
		} else {
			msg += "Assignin to"
		}

		this.raise(expr.start, msg+" rvalue")
	}
	return nil
}

func (this *Parser) checkLValPattern(expr *Node, bindingType Flags, checkClashes struct {
	check bool
	hash  map[string]bool
}) error {
	switch expr.type_ {
	case NODE_OBJECT_PATTERN:
		for _, prop := range expr.properties {
			return this.checkLValInnerPattern(prop, bindingType, checkClashes)
		}

	case NODE_ARRAY_PATTERN:
		for _, elem := range expr.elements {
			if elem != nil {
				return this.checkLValInnerPattern(elem, bindingType, checkClashes)
			}
		}
	}

	return this.checkLValSimple(expr, bindingType, checkClashes)
}

func (this *Parser) checkLValInnerPattern(expr *Node, bindingType Flags, checkClashes struct {
	check bool
	hash  map[string]bool
}) error {
	switch expr.type_ {
	case NODE_PROPERTY:
		// AssignmentProperty has type === "Property"
		if expr, ok := expr.value.(*Node); ok {
			return this.checkLValInnerPattern(expr.value.(*Node), bindingType, checkClashes)
		}

		return this.raise(this.pos, "Expression had invalid Value")

	case NODE_ASSIGNMENT_PATTERN:
		return this.checkLValPattern(expr.left, bindingType, checkClashes)

	case NODE_REST_ELEMENT:
		return this.checkLValPattern(expr.argument, bindingType, checkClashes)
	}

	return this.checkLValPattern(expr, bindingType, checkClashes)
}

func (this *Parser) parseSpread(refDestructuringErrors *DestructuringErrors) (*Node, error) {
	node := this.startNode()
	this.next(false)
	maybeAsssign, err := this.parseMaybeAssign("", refDestructuringErrors, nil)
	if err != nil {
		return nil, err
	}
	node.argument = maybeAsssign
	return this.finishNode(node, NODE_SPREAD_ELEMENT), nil
}

func (this *Parser) parseRestBinding() (*Node, error) {
	node := this.startNode()
	this.next(false)

	// RestElement inside of a function parameter must be an identifier
	if this.getEcmaVersion() == 6 && this.Type.identifier != TOKEN_NAME {
		return nil, this.unexpected("", nil)
	}

	bindingAtom, err := this.parseBindingAtom()

	if err != nil {
		return nil, err
	}

	node.argument = bindingAtom

	return this.finishNode(node, NODE_REST_ELEMENT), nil
}

func (this *Parser) parseBindingAtom() (*Node, error) {
	if this.getEcmaVersion() >= 6 {
		switch this.Type.identifier {
		case TOKEN_BRACKETL:
			node := this.startNode()
			this.next(false)
			elements, err := this.parseBindingList(TOKEN_BRACKETR, true, true, false)

			if err != nil {
				return nil, err
			}
			node.elements = elements
			return this.finishNode(node, NODE_ARRAY_PATTERN), nil

		case TOKEN_BRACEL:
			obj, err := this.parseObj(true, nil)
			return obj, err
		}
	}
	identifier, err := this.parseIdent(false)
	return identifier, err
}

func (this *Parser) parseMaybeDefault(startPos int, startLoc *Location, left *Node) (*Node, error) {
	if left == nil {
		bindingAtom, err := this.parseBindingAtom()
		if err != nil {
			return nil, err
		}
		left = bindingAtom
	}

	if this.getEcmaVersion() < 6 || !this.eat(TOKEN_EQ) {
		return left, nil
	}
	node := this.startNodeAt(startPos, startLoc)
	node.left = left
	maybeAssign, err := this.parseMaybeAssign("", nil, nil)
	if err != nil {
		return nil, err
	}
	node.rigth = maybeAssign
	return this.finishNode(node, NODE_ASSIGNMENT_PATTERN), nil
}

func (this *Parser) parseBindingList(close Token, allowEmpty bool, allowTrailingComma bool, allowModifiers bool) ([]*Node, error) {
	elts, first := []*Node{}, true
	for !this.eat(close) {
		if first {
			first = false
		} else {
			err := this.expect(TOKEN_COMMA)
			if err != nil {
				return nil, err
			}
		}
		if allowEmpty && this.Type.identifier == TOKEN_COMMA {
			elts = append(elts, nil)

		} else if allowTrailingComma && this.afterTrailingComma(close, false) {
			break
		} else if this.Type.identifier == TOKEN_ELLIPSIS {
			rest, err := this.parseRestBinding()
			if err != nil {
				return nil, err
			}
			bindingListItem := this.parseBindingListItem(rest)

			elts = append(elts, bindingListItem)
			if this.Type.identifier == TOKEN_COMMA {
				return nil, this.raiseRecoverable(this.start, "Comma is not permitted after the rest element")
			}
			err = this.expect(close)
			if err != nil {
				return nil, err
			}
			break
		} else {
			assignableListItem, err := this.parseAssignableListItem(allowModifiers)
			if err != nil {
				return nil, err
			}
			elts = append(elts, assignableListItem)
		}
	}
	return elts, nil
}

func (this *Parser) parseAssignableListItem(allowModifiers bool) (*Node, error) {
	elem, err := this.parseMaybeDefault(this.start, this.startLoc, nil)
	if err != nil {
		return nil, err
	}
	this.parseBindingListItem(elem)
	return elem, nil
}

func (this *Parser) parseBindingListItem(param *Node) *Node {
	return param
}
