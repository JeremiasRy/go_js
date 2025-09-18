package parser

func (p *Parser) toAssignable(node *Node, isBinding bool, refDestructuringErrors *DestructuringErrors) (*Node, error) {
	if p.getEcmaVersion() >= 6 && node != nil {
		switch node.Type {
		case NODE_IDENTIFIER:
			if p.inAsync() && node.Name == "await" {
				return nil, p.raise(node.Start, "Cannot use 'await' as identifier inside an async function")
			}

		case NODE_OBJECT_PATTERN, NODE_ARRAY_PATTERN, NODE_ASSIGNMENT_PATTERN, NODE_REST_ELEMENT:
		case NODE_OBJECT_EXPRESSION:
			node.Type = NODE_OBJECT_PATTERN
			if refDestructuringErrors != nil {
				if err := p.checkPatternErrors(refDestructuringErrors, true); err != nil {
					return nil, err
				}
			}

			for _, prop := range node.Properties {
				_, err := p.toAssignable(prop, isBinding, nil)

				if err != nil {
					return nil, err
				}
				if prop.Type == NODE_REST_ELEMENT &&
					(prop.Argument.Type == NODE_ARRAY_PATTERN || prop.Argument.Type == NODE_OBJECT_PATTERN) {
					return nil, p.raise(prop.Alternate.Start, "Unexpected token")
				}
			}

		case NODE_PROPERTY:
			// AssignmentProperty has type == "Property"
			if node.Kind != KIND_PROPERTY_INIT {
				return nil, p.raise(node.Key.Start, "Object pattern can't contain getter or setter")
			}

			if val, ok := node.Value.(*Node); ok {
				_, err := p.toAssignable(val, isBinding, nil)
				if err != nil {
					return nil, err
				}
			} else {
				panic("In toAsignable() we were expecting node.Value to be *Node")
			}

		case NODE_ARRAY_EXPRESSION:
			node.Type = NODE_ARRAY_PATTERN
			if refDestructuringErrors != nil {
				err := p.checkPatternErrors(refDestructuringErrors, true)

				if err != nil {
					return nil, err
				}
			}
			_, err := p.toAssignableList(node.Elements, isBinding)
			if err != nil {
				return nil, err
			}

		case NODE_SPREAD_ELEMENT:
			node.Type = NODE_REST_ELEMENT
			_, err := p.toAssignable(node.Argument, isBinding, nil)
			if err != nil {
				return nil, err
			}
			if node.Argument.Type == NODE_ASSIGNMENT_PATTERN {
				return nil, p.raise(node.Argument.Start, "Rest elements cannot have a default value")
			}

		case NODE_ASSIGNMENT_EXPRESSION:
			if node.AssignmentOperator != ASSIGN {
				return nil, p.raise(node.Left.End, "Only '=' operator can be used for specifying default value.")
			}
			node.Type = NODE_ASSIGNMENT_PATTERN
			node.AssignmentOperator = ""
			_, err := p.toAssignable(node.Left, isBinding, nil)

			if err != nil {
				return nil, err
			}

		case NODE_PARENTHESIZED_EXPRESSION:
			_, err := p.toAssignable(node.Expression, isBinding, refDestructuringErrors)
			if err != nil {
				return nil, err
			}

		case NODE_CHAIN_EXPRESSION:
			return nil, p.raiseRecoverable(node.Start, "Optional chaining cannot appear in left-hand side")

		case NODE_MEMBER_EXPRESSION:
			if !isBinding {
				break
			}
			fallthrough

		default:
			return nil, p.raise(node.Start, "Assigning to rvalue")
		}
	} else if refDestructuringErrors != nil {
		err := p.checkPatternErrors(refDestructuringErrors, true)
		if err != nil {
			return nil, err
		}
	}
	return node, nil
}

func (p *Parser) toAssignableList(exprList []*Node, isBinding bool) ([]*Node, error) {
	end := len(exprList)

	for i := 0; i < end; i++ {
		elt := exprList[i]
		if elt != nil {
			_, err := p.toAssignable(elt, isBinding, nil)
			if err != nil {
				return nil, err
			}
		}
	}

	if end != 0 {
		last := exprList[end-1]
		if p.getEcmaVersion() == 6 && isBinding && last != nil && last.Type == NODE_REST_ELEMENT && last.Argument.Type != NODE_IDENTIFIER {
			return nil, p.unexpected(`if p.getEcmaVersion() == 6 && isBinding && last != nil && last.Type == NODE_REST_ELEMENT && last.Argument.Type != NODE_IDENTIFIER`, &last.Argument.Start)
		}

	}
	return exprList, nil
}

// send 0 for bindingType if not used in acorn function calls
func (p *Parser) checkLValSimple(expr *Node, bindingType Flags, checkClashes struct {
	check bool
	hash  map[string]bool
}) error {
	isBind := bindingType != BIND_NONE

	switch expr.Type {
	case NODE_IDENTIFIER:
		if p.Strict && p.ReservedWordsStrictBind.Match([]byte(expr.Name)) {
			msg := ""
			if isBind {
				msg += "Binding "
			} else {
				msg += "Assigning to "
			}

			msg += expr.Name
			return p.raiseRecoverable(expr.Start, msg+" in strict mode")
		}

		if isBind {
			if bindingType == BIND_LEXICAL && expr.Name == "let" {
				return p.raiseRecoverable(expr.Start, "let is disallowed as a lexically bound name")
			}

			if checkClashes.check {
				if _, has := checkClashes.hash[expr.Name]; has {
					return p.raiseRecoverable(expr.Start, "Argument name clash")
				}

				checkClashes.hash[expr.Name] = true
			}
			if bindingType != BIND_OUTSIDE {
				return p.declareName(expr.Name, bindingType, expr.Start)
			}
		}

	case NODE_CHAIN_EXPRESSION:
		return p.raiseRecoverable(expr.Start, "Optional chaining cannot appear in left-hand side")

	case NODE_MEMBER_EXPRESSION:
		if isBind {
			return p.raiseRecoverable(expr.Start, "Binding member expression")
		}

	case NODE_PARENTHESIZED_EXPRESSION:
		if isBind {
			return p.raiseRecoverable(expr.Start, "Binding parenthesized expression")
		}
		return p.checkLValSimple(expr.Expression, bindingType, checkClashes)

	default:
		msg := ""
		if isBind {
			msg += "Binding"
		} else {
			msg += "Assignin to"
		}

		return p.raise(expr.Start, msg+" rvalue")
	}
	return nil
}

func (p *Parser) checkLValPattern(expr *Node, bindingType Flags, checkClashes struct {
	check bool
	hash  map[string]bool
}) error {
	switch expr.Type {
	case NODE_OBJECT_PATTERN:
		for _, prop := range expr.Properties {
			return p.checkLValInnerPattern(prop, bindingType, checkClashes)
		}

	case NODE_ARRAY_PATTERN:
		for _, elem := range expr.Elements {
			if elem != nil {
				return p.checkLValInnerPattern(elem, bindingType, checkClashes)
			}
		}
	}
	return p.checkLValSimple(expr, bindingType, checkClashes)
}

func (p *Parser) checkLValInnerPattern(expr *Node, bindingType Flags, checkClashes struct {
	check bool
	hash  map[string]bool
}) error {
	switch expr.Type {
	case NODE_PROPERTY:
		// AssignmentProperty has type === "Property"
		if expr, ok := expr.Value.(*Node); ok {
			return p.checkLValInnerPattern(expr, bindingType, checkClashes)
		}
		return p.raise(p.pos, "Expression had invalid Value")

	case NODE_ASSIGNMENT_PATTERN:
		return p.checkLValPattern(expr.Left, bindingType, checkClashes)

	case NODE_REST_ELEMENT:
		return p.checkLValPattern(expr.Argument, bindingType, checkClashes)
	}

	return p.checkLValPattern(expr, bindingType, checkClashes)
}

func (p *Parser) parseSpread(refDestructuringErrors *DestructuringErrors) (*Node, error) {
	node := p.startNode()
	p.next(false)
	maybeAsssign, err := p.parseMaybeAssign("", refDestructuringErrors, nil)
	if err != nil {
		return nil, err
	}
	node.Argument = maybeAsssign
	return p.finishNode(node, NODE_SPREAD_ELEMENT), nil
}

func (p *Parser) parseRestBinding() (*Node, error) {
	node := p.startNode()
	p.next(false)

	// RestElement inside of a function parameter must be an identifier
	if p.getEcmaVersion() == 6 && p.Type.identifier != TOKEN_NAME {
		return nil, p.unexpected("", nil)
	}

	bindingAtom, err := p.parseBindingAtom()

	if err != nil {
		return nil, err
	}

	node.Argument = bindingAtom

	return p.finishNode(node, NODE_REST_ELEMENT), nil
}

func (p *Parser) parseBindingAtom() (*Node, error) {
	if p.getEcmaVersion() >= 6 {
		switch p.Type.identifier {
		case TOKEN_BRACKETL:
			node := p.startNode()
			p.next(false)
			elements, err := p.parseBindingList(TOKEN_BRACKETR, true, true, false)

			if err != nil {
				return nil, err
			}
			node.Elements = elements
			return p.finishNode(node, NODE_ARRAY_PATTERN), nil

		case TOKEN_BRACEL:
			obj, err := p.parseObj(true, nil)
			return obj, err
		}
	}
	identifier, err := p.parseIdent(false)
	return identifier, err
}

func (p *Parser) parseMaybeDefault(startPos int, startLoc *Location, left *Node) (*Node, error) {
	if left == nil {
		bindingAtom, err := p.parseBindingAtom()
		if err != nil {
			return nil, err
		}
		left = bindingAtom
	}

	if p.getEcmaVersion() < 6 || !p.eat(TOKEN_EQ) {
		return left, nil
	}
	node := p.startNodeAt(startPos, startLoc)
	node.Left = left
	maybeAssign, err := p.parseMaybeAssign("", nil, nil)
	if err != nil {
		return nil, err
	}
	node.Right = maybeAssign
	return p.finishNode(node, NODE_ASSIGNMENT_PATTERN), nil
}

func (p *Parser) parseBindingList(close Token, allowEmpty bool, allowTrailingComma bool, allowModifiers bool) ([]*Node, error) {
	elts, first := []*Node{}, true
	for !p.eat(close) {
		if first {
			first = false
		} else {
			err := p.expect(TOKEN_COMMA)
			if err != nil {
				return nil, err
			}
		}
		if allowEmpty && p.Type.identifier == TOKEN_COMMA {
			elts = append(elts, nil)

		} else if allowTrailingComma && p.afterTrailingComma(close, false) {
			break
		} else if p.Type.identifier == TOKEN_ELLIPSIS {
			rest, err := p.parseRestBinding()
			if err != nil {
				return nil, err
			}
			bindingListItem := p.parseBindingListItem(rest)

			elts = append(elts, bindingListItem)
			if p.Type.identifier == TOKEN_COMMA {
				return nil, p.raiseRecoverable(p.start, "Comma is not permitted after the rest element")
			}
			err = p.expect(close)
			if err != nil {
				return nil, err
			}
			break
		} else {
			assignableListItem, err := p.parseAssignableListItem(allowModifiers)
			if err != nil {
				return nil, err
			}
			elts = append(elts, assignableListItem)
		}
	}
	return elts, nil
}

func (p *Parser) parseAssignableListItem(allowModifiers bool) (*Node, error) {
	elem, err := p.parseMaybeDefault(p.start, p.startLoc, nil)
	if err != nil {
		return nil, err
	}
	p.parseBindingListItem(elem)
	return elem, nil
}

func (p *Parser) parseBindingListItem(param *Node) *Node {
	return param
}
