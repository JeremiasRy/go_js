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
	panic("unimplemented")
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
	panic("unimplemented")
}

func (this *Parser) parseIfStatement(node *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseFunctionStatement(node *Node, false bool, b bool) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseForStatement(node *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseDoStatement(node *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseDebuggerStatement(node *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseBreakContinueStatement(node *Node, keyword string) (*Node, error) {
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
