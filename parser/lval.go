package parser

// send 0 for bindingType if not used in acorn function calls
func (this *Parser) checkLValSimple(expr *Node, bindingType Flags, checkClashes struct {
	check bool
	hash  map[string]bool
}) error {
	isBind := bindingType != BIND_NONE

	switch expr.Type {
	case NODE_IDENTIFIER:
		if this.Strict && this.ReservedWordsStrictBind.Match([]byte(expr.Name)) {
			msg := ""
			if isBind {
				msg += "Binding "
			} else {
				msg += "Assigning to "
			}

			msg += expr.Name
			return this.raiseRecoverable(expr.Start, msg+" in strict mode")
		}

		if isBind {
			if bindingType == BIND_LEXICAL && expr.Name == "let" {
				return this.raiseRecoverable(expr.Start, "let is disallowed as a lexically bound name")
			}

			if checkClashes.check {
				if _, has := checkClashes.hash[expr.Name]; has {
					return this.raiseRecoverable(expr.Start, "Argument name clash")
				}

				checkClashes.hash[expr.Name] = true
			}
			if bindingType != BIND_OUTSIDE {
				return this.declareName(expr.Name, bindingType, expr.Start)
			}
		}

	case NODE_CHAIN_EXPRESSION:
		return this.raiseRecoverable(expr.Start, "Optional chaining cannot appear in left-hand side")

	case NODE_MEMBER_EXPRESSION:
		if isBind {
			return this.raiseRecoverable(expr.Start, "Binding member expression")
		}

	case NODE_PARENTHESIZED_EXPRESSION:
		if isBind {
			return this.raiseRecoverable(expr.Start, "Binding parenthesized expression")
		}
		return this.checkLValSimple(expr.Expression, bindingType, checkClashes)

	default:
		msg := ""
		if isBind {
			msg += "Binding"
		} else {
			msg += "Assignin to"
		}

		this.raise(expr.Start, msg+" rvalue")
	}
	return nil
}

func (this *Parser) checkLValPattern(expr *Node, bindingType Flags, checkClashes struct {
	check bool
	hash  map[string]bool
}) error {
	switch expr.Type {
	case NODE_OBJECT_PATTERN:
		for _, prop := range expr.Properties {
			return this.checkLValInnerPattern(prop, bindingType, checkClashes)
		}

	case NODE_ARRAY_PATTERN:
		for _, elem := range expr.Elements {
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
	switch expr.Type {
	case NODE_PROPERTY:
		// AssignmentProperty has type === "Property"
		if expr, ok := expr.Value.(*Node); ok {
			return this.checkLValInnerPattern(expr.Value.(*Node), bindingType, checkClashes)
		}

		return this.raise(this.pos, "Expression had invalid Value")

	case NODE_ASSIGNMENT_PATTERN:
		return this.checkLValPattern(expr.Left, bindingType, checkClashes)

	case NODE_REST_ELEMENT:
		return this.checkLValPattern(expr.Argument, bindingType, checkClashes)
	}

	return this.checkLValPattern(expr, bindingType, checkClashes)
}

func (this *Parser) parseSpread(refDestructuringErrors *DestructuringErrors) (*Node, error) {
	node := this.startNode()
	this.next(false)
	maybeAsssign, err := this.parseMaybeAssign("", refDestructuringErrors)
	if err != nil {
		return nil, err
	}
	node.Argument = maybeAsssign
	return this.finishNode(node, NODE_SPREAD_ELEMENT), nil
}
