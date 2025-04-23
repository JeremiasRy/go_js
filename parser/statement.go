package parser

import "errors"

func (this *Parser) parseTopLevel(node *Node) (*Node, error) {
	exports := map[string]*Node{}
	node.Body = []*Node{}
	for this.Type.identifier != TOKEN_EOF {
		stmt, err := this.parseStatement("", true, exports)
		if err != nil {
			return nil, err
		}
		node.Body = append(node.Body, stmt)
	}

	if this.InModule {
		for k, _ := range this.UndefinedExports { // let's just aggregate all of the undefined exports since now it'll just return at first
			return nil, this.raiseRecoverable(this.UndefinedExports[k].Start, "Export "+k+" not defined")
		}
	}

	err := this.adaptDirectivePrologue(node.Body)

	if err != nil {
		return nil, err
	}
	return this.finishNode(node, NODE_PROGRAM), nil
}

func (this *Parser) parseStatement(context string, topLevel bool, exports map[string]*Node) (*Node, error) {
	startType, node := this.Type, this.startNode()
	kind := DECLARATION_KIND_NOT_INITIALIZED

	if this.isLet(context) {
		startType = tokenTypes[TOKEN_VAR]
		kind = LET
	}

	switch startType.identifier {
	case TOKEN_BREAK, TOKEN_CONTINUE:
		breakContinueStatement, err := this.parseBreakContinueStatement(node, startType.keyword)
		if err != nil {
			return nil, err
		}
		return breakContinueStatement, nil

	case TOKEN_DEBUGGER:
		debuggerStatement, err := this.parseDebuggerStatement(node)
		if err != nil {
			return nil, err
		}
		return debuggerStatement, nil

	case TOKEN_DO:
		doStatement, err := this.parseDoStatement(node)
		if err != nil {
			return nil, err
		}
		return doStatement, nil

	case TOKEN_FOR:
		forStatement, err := this.parseForStatement(node)
		if err != nil {
			return nil, err
		}
		return forStatement, nil

	case TOKEN_FUNCTION:
		if len(context) != 0 && (this.Strict || context != "if" && context != "label" && this.getEcmaVersion() >= 6) {
			return nil, this.unexpected(nil)
		}
		functionStatement, err := this.parseFunctionStatement(node, false, len(context) == 0)

		if err != nil {
			return nil, err
		}

		return functionStatement, nil

	case TOKEN_CLASS:
		if len(context) != 0 {
			return nil, this.unexpected(nil)
		}
		classStatement, err := this.parseClass(node, true)

		if err != nil {
			return nil, err
		}
		return classStatement, nil

	case TOKEN_IF:
		ifStatement, err := this.parseIfStatement(node)
		if err != nil {
			return nil, err
		}
		return ifStatement, nil

	case TOKEN_RETURN:
		returnStatement, err := this.parseReturnStatement(node)
		if err != nil {
			return nil, err
		}
		return returnStatement, nil

	case TOKEN_SWITCH:
		switchStatement, err := this.parseSwitchStatement(node)
		if err != nil {
			return nil, err
		}
		return switchStatement, nil

	case TOKEN_THROW:
		throwStatement, err := this.parseThrowStatement(node)
		if err != nil {
			return nil, err
		}
		return throwStatement, nil

	case TOKEN_TRY:
		tryStatement, err := this.parseTryStatement(node)
		if err != nil {
			return nil, err
		}
		return tryStatement, nil

	case TOKEN_CONST, TOKEN_VAR:
		if kind == DECLARATION_KIND_NOT_INITIALIZED {
			if k, ok := this.Value.(DeclarationKind); ok {
				kind = k
			} else {
				panic("We were expectin a DeclarationKind from node.Value, didn't happen so we are now here.")
			}
		}

		if len(context) != 0 && kind != VAR {
			return nil, this.unexpected(nil)
		}

		varStatement, err := this.parseVarStatement(node, kind)

		if err != nil {
			return nil, err
		}

		return varStatement, nil

	case TOKEN_WHILE:
		whileStatement, err := this.parseWhileStatement(node)

		if err != nil {
			return nil, err
		}

		return whileStatement, err

	case TOKEN_WITH:
		withStatement, err := this.parseWithStatement(node)

		if err != nil {
			return nil, err
		}

		return withStatement, err

	case TOKEN_BRACEL:
		block, err := this.parseBlock(true, node, false)

		if err != nil {
			return nil, err
		}

		return block, nil

	case TOKEN_SEMI:
		emptyStatement, err := this.parseEmptyStatement(node)

		if err != nil {
			return nil, err
		}
		return emptyStatement, nil

	case TOKEN_EXPORT, TOKEN_IMPORT:
		if this.getEcmaVersion() > 10 && startType.identifier == TOKEN_IMPORT {
			skip := skipWhiteSpace.Find((this.input[this.pos:]))

			next := this.pos + len(skip)
			nextCh := this.input[next]

			if nextCh == '(' || nextCh == '.' {
				expression, err := this.parseExpression("", nil)

				if err != nil {
					return nil, err
				}

				expressionStatement, err := this.parseExpressionStatement(node, expression)

				if err != nil {
					return nil, err
				}

				return expressionStatement, nil
			}
		}

		if this.options.AllowImportExportEverywhere {
			if !topLevel {
				return nil, this.raise(this.start, "'import' and 'export' may only appear at the top level")
			}

			if this.InModule {
				return nil, this.raise(this.start, "'import' and 'export' may appear only with 'sourceType: module'")
			}
		}

		if startType.identifier == TOKEN_IMPORT {
			importStatement, err := this.parseImport(node)
			if err != nil {
				return nil, err
			}
			return importStatement, nil
		}

		if startType.identifier == TOKEN_EXPORT {
			exportStatement, err := this.parseExport(node)
			if err != nil {
				return nil, err
			}
			return exportStatement, nil
		}

	default:
		if this.isAsyncFunction() {
			if len(context) != 0 {
				return nil, this.unexpected(nil)
			}
			this.next(false)

			functionStatement, err := this.parseFunctionStatement(node, true, len(context) == 0)
			if err != nil {
				return nil, err
			}

			return functionStatement, nil
		}

		maybeName := this.Value
		expr, err := this.parseExpression("", nil)

		if err != nil {
			return nil, err
		}

		if startType.identifier == TOKEN_NAME && expr.Type == NODE_IDENTIFIER && this.eat(TOKEN_COLON) {

			if name, ok := maybeName.(string); ok {
				labeledStatement, err := this.parseLabeledStatement(node, name, expr, context)
				if err != nil {
					return nil, err
				}
				return labeledStatement, nil
			} else {
				panic("We expected node.Value to be a string, it wasn't so we ended up here")
			}

		}

		expressionStatement, err := this.parseExpressionStatement(node, expr)

		if err != nil {
			return nil, err
		}

		return expressionStatement, nil
	}

	return nil, errors.New("Unreachable... or was it?")
}

func (this *Parser) parseLabeledStatement(node *Node, name string, expr *Node, context string) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) isAsyncFunction() bool {
	if this.getEcmaVersion() < 8 || !this.isContextual("async") {
		return false
	}

	skip := skipWhiteSpace.Find(this.input[this.pos:])
	next := this.pos + len(skip)
	after := rune(this.input[next+8])

	return !lineBreak.Match(this.input[this.pos:next]) &&
		string(this.input[next:next+8]) == "function" &&
		(next+8 == len(this.input) ||
			!(IsIdentifierChar(after, false) /*|| after > 0xd7ff && after < 0xdc00*/))
}

func (this *Parser) parseExport(node *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseImport(node *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseExpressionStatement(node *Node, expression *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseEmptyStatement(node *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseWithStatement(node *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseWhileStatement(node *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseVarStatement(node *Node, kind DeclarationKind) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseTryStatement(node *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseThrowStatement(node *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseSwitchStatement(node *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseReturnStatement(node *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) isLet(context string) bool {
	if this.getEcmaVersion() < 6 || !this.isContextual("let") {
		return false
	}

	skip := skipWhiteSpace.Find(this.input[this.pos:])
	next := this.pos + len(skip)

	nextCh := rune(this.input[next])
	// For ambiguous cases, determine if a LexicalDeclaration (or only a
	// Statement) is allowed here. If context is not empty then only a Statement
	// is allowed. However, `let [` is an explicit negative lookahead for
	// ExpressionStatement, so special-case it first.
	if nextCh == '[' || nextCh == '\\' {
		return true
	}
	if len(context) != 0 {
		return false
	}

	if nextCh == 123 /* || nextCh > 0xd7ff && nextCh < 0xdc00 */ {
		return true // '{', astral
	}
	if IsIdentifierStart(nextCh, true) {
		pos := next + 1
		nextCh = rune(this.input[pos])
		for IsIdentifierChar(nextCh, true) {
			pos = pos + 1
		}
		if nextCh == 92 /*|| nextCh > 0xd7ff && nextCh < 0xdc00*/ {
			return true

		}
		ident := this.input[next:pos]
		if !keywordRelationalOperator.Match(ident) {
			return true
		}
	}
	return false
}

func (this *Parser) parseIfStatement(node *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseFunctionStatement(node *Node, false bool, b bool) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseForStatement(node *Node) (*Node, error) {
	this.next(false)
	awaitAt := -1

	if this.getEcmaVersion() >= 9 && this.canAwait() && this.eatContextual("await") {
		awaitAt = this.LastTokStart
	}

	this.Labels = append(this.Labels, Label{Kind: "loop", Name: ""})

	this.enterScope(0)
	err := this.expect(TOKEN_PARENL)

	if err != nil {
		return nil, err
	}

	if this.Type.identifier == TOKEN_SEMI {
		if awaitAt > -1 {
			return nil, this.unexpected(&awaitAt)
		}
		forStatement, err := this.parseFor(node, nil)
		if err != nil {
			return nil, err
		}
		return forStatement, nil
	}

	isLet := this.isLet("")
	if this.Type.identifier == TOKEN_VAR || this.Type.identifier == TOKEN_CONST || isLet {
		init := this.startNode()

		kind := DECLARATION_KIND_NOT_INITIALIZED
		if isLet {
			kind = LET
		} else {
			if k, ok := this.Value.(DeclarationKind); ok {
				kind = k
			} else {
				panic("parser.Value was snot declarationKind as we expected")
			}
		}
		this.next(false)
		varExpresssion, err := this.parseVar(init, true, kind)

		if err != nil {
			return nil, err
		}

		this.finishNode(init, NODE_VARIABLE_DECLARATION)

		if (this.Type.identifier == TOKEN_IN || (this.getEcmaVersion() >= 6 && this.isContextual("of"))) && len(init.Declarations) == 1 {
			if this.getEcmaVersion() >= 9 {
				if this.Type.identifier == TOKEN_IN {
					if awaitAt > -1 {
						return nil, this.unexpected(&awaitAt)
					}
				} else {
					node.Await = awaitAt > -1
				}
			}
			forIn, err := this.parseForIn(node, init)

			if err != nil {
				return nil, err
			}
			return forIn, nil
		}
		if awaitAt > -1 {
			return nil, this.unexpected(&awaitAt)
		}

		forStatement, err := this.parseFor(node, init)
		if err != nil {
			return nil, err
		}
		return forStatement, nil
	}

	startsWithLet, isForOf, containsEsc, refDestructuringErrors, initPos := this.isContextual("let"), false, this.ContainsEsc, NewDestructuringErrors(), this.start
	var init *Node

	if awaitAt > -1 {
		exprSubscripts, err := this.parseExprSubscripts(refDestructuringErrors, "await")
		if err != nil {
			return nil, err
		}

		init = exprSubscripts
	} else {
		expr, err := this.parseExpression("true", refDestructuringErrors)
		if err != nil {
			return nil, err
		}

		init = expr
	}

	isForOf = this.getEcmaVersion() >= 6 && this.isContextual("of")
	if this.Type.identifier == TOKEN_IN || isForOf {
		if awaitAt > -1 { // implies `ecmaVersion >= 9` (see declaration of awaitAt)
			if this.Type.identifier == TOKEN_IN {
				return nil, this.unexpected(&awaitAt)
			}
			node.Await = true
		} else if isForOf && this.getEcmaVersion() >= 8 {
			if init.Start == initPos && !containsEsc && init.Type == NODE_IDENTIFIER && init.Name == "async" {
				return nil, this.unexpected(nil)
			} else if this.getEcmaVersion() >= 9 {
				node.Await = false
			}
		}
		if startsWithLet && isForOf {
			return nil, this.raise(init.Start, "The left-hand side of a for-of loop may not start with 'let'.")
		}
		_, err := this.toAssignable(init, false, refDestructuringErrors)
		if err != nil {
			return nil, err
		}
		err = this.checkLValPattern(init, 0, struct {
			check bool
			hash  map[string]bool
		}{check: false, hash: map[string]bool{}})

		if err != nil {
			return nil, err
		}
		forIn, err := this.parseForIn(node, init)

		if err != nil {
			return nil, err
		}

		return forIn, nil
	} else {
		_, err := this.checkExpressionErrors(refDestructuringErrors, true)

		if err != nil {
			return nil, err
		}
	}
	if awaitAt > -1 {
		return nil, this.unexpected(&awaitAt)
	}

	forStatement, err := this.parseFor(node, init)

	if err != nil {
		return nil, err
	}
	return forStatement, nil
}

func (this *Parser) parseFor(node *Node, init *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseForIn(node *Node, init *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseVar(init *Node, true bool, kind DeclarationKind) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) eatContextual(s string) bool {
	panic("unimplemented")
}

func (this *Parser) parseDoStatement(node *Node) (*Node, error) {
	this.next(false)
	this.Labels = append(this.Labels, Label{Kind: "loop", Name: ""})
	doStatement, err := this.parseStatement("do", false, map[string]*Node{})
	if err != nil {
		return nil, err
	}
	node.BodyNode = doStatement
	this.Labels = this.Labels[:len(this.Labels)-1]
	err = this.expect(TOKEN_WHILE)

	if err != nil {
		return nil, err
	}

	testParenExpression, err := this.parseParenExpression()

	if err != nil {
		return nil, err
	}

	node.Test = testParenExpression

	if this.getEcmaVersion() >= 6 {
		this.eat(TOKEN_SEMI)
	} else {
		this.semicolon()
	}

	return this.finishNode(node, NODE_DO_WHILE_STATEMENT), nil
}

func (this *Parser) parseDebuggerStatement(node *Node) (*Node, error) {
	this.next(false)
	this.semicolon()
	return this.finishNode(node, NODE_DEBUGGER_STATEMENT), nil
}

func (this *Parser) parseBreakContinueStatement(node *Node, keyword string) (*Node, error) {
	isBreak := keyword == "break"
	this.next(false)
	if this.eat(TOKEN_SEMI) || this.insertSemicolon() {
		node.Label = nil
	} else if this.Type.identifier != TOKEN_NAME {
		return nil, this.unexpected(nil)
	} else {
		ident, err := this.parseIdent(false)

		if err != nil {
			return nil, err
		}
		node.Label = ident
		this.semicolon()
	}

	// Verify that there is an actual destination to break or
	// continue to.
	i := 0
	for i < len(this.Labels) {
		lab := this.Labels[i]
		if node.Label == nil || lab.Name == node.Label.Name {
			if len(lab.Kind) != 0 && isBreak || lab.Kind == "loop" {
				break
			}
			if node.Label != nil && isBreak {
				break
			}
		}
	}

	if i == len(this.Labels) {
		return nil, this.raise(node.Start, "Unsyntactic "+keyword)
	}

	if isBreak {
		return this.finishNode(node, NODE_BREAK_STATEMENT), nil
	}

	return this.finishNode(node, NODE_CONTINUE_STATEMENT), nil
}

func (this *Parser) semicolon() {
	panic("unimplemented")
}

func (this *Parser) insertSemicolon() bool {
	panic("unimplemented")
}

func (this *Parser) parseBlock(createNewLexicalScope bool, node *Node, exitStrict bool) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) adaptDirectivePrologue(param []*Node) error {
	panic("unimplemented")
}

func (p *Parser) parseFunction(node *Node, statement Flags, allowExpressionBody bool, isAsync bool, forInit string) (*Node, error) {
	panic("unimplemented")
}

func (p *Parser) parseClass(node *Node, isStatement bool) (*Node, error) {
	panic("unimplemented")
}
