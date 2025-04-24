package parser

import (
	"errors"
	"strings"
)

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
			exportStatement, err := this.parseExport(node, exports)
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

	return nil, errors.New("unreachable... or was it?")
}

func (this *Parser) parseLabeledStatement(node *Node, maybeName string, expr *Node, context string) (*Node, error) {
	for _, label := range this.Labels {
		if label.Name == maybeName {
			return nil, this.raise(expr.Start, "Label '"+maybeName+"' is already declared")
		}
	}

	kind := ""
	if this.Type.isLoop {
		kind = "loop"
	} else {
		if this.Type.identifier == TOKEN_SWITCH {
			kind = "switch"
		}
	}
	for i := len(this.Labels) - 1; i >= 0; i-- {
		if this.Labels[i].StatementStart == node.Start {
			this.Labels[i].StatementStart = this.start
			this.Labels[i].Kind = kind
		} else {
			break
		}
	}

	this.Labels = append(this.Labels, Label{Name: maybeName, Kind: kind, StatementStart: this.start})

	if len(context) != 0 && !strings.Contains(context, "label") {
		context += "label"
	}

	statement, err := this.parseStatement(context, false, map[string]*Node{})
	if err != nil {
		return nil, err
	}

	node.BodyNode = statement
	this.Labels = this.Labels[:len(this.Labels)-1]
	node.Label = expr
	return this.finishNode(node, NODE_LABELED_STATEMENT), nil
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

func (this *Parser) parseExport(node *Node, exports map[string]*Node) (*Node, error) {
	this.next(false)

	if this.eat(TOKEN_STAR) {
		exportAllDeclaration, err := this.parseExportAllDeclaration(node, exports)
		if err != nil {
			return nil, err
		}
		return exportAllDeclaration, nil
	}

	if this.eat(TOKEN_DEFAULT) {
		err := this.checkExport(exports, struct {
			s string
			n *Node
		}{s: "default"}, this.LastTokStart)
		if err != nil {
			return nil, err
		}

		return this.finishNode(node, NODE_EXPORT_DEFAULT_DECLARATION), nil
	}

	if this.shouldParseExportStatement() {
		decl, err := this.parseExportDeclaration(node)

		if err != nil {
			return nil, err
		}

		node.Declaration = decl

		if node.Declaration.Type == NODE_VARIABLE_DECLARATION {
			err := this.checkVariableExport(exports, node.Declaration.Declarations)
			if err != nil {
				return nil, err
			}
		} else {
			err := this.checkExport(exports, struct {
				s string
				n *Node
			}{n: node.Declaration.Id}, node.Declaration.Id.Start)
			if err != nil {
				return nil, err
			}
		}

		node.Specifiers = []*Node{}
		node.Source = nil

		if this.getEcmaVersion() >= 16 {
			node.Attributes = []*Node{}
		}
	} else {
		node.Declaration = nil
		specifiers, err := this.parseExportSpecifiers(exports)

		if err != nil {
			return nil, err
		}

		node.Specifiers = specifiers

		if this.eatContextual("from") {
			if this.Type.identifier == TOKEN_STRING {
				return nil, this.unexpected(nil)
			}
			exprAtom, err := this.parseExprAtom(nil, "", false)

			if err != nil {
				return nil, err
			}

			node.Source = exprAtom
			if this.getEcmaVersion() >= 16 {
				withClause, err := this.parseWithClause()

				if err != nil {
					return nil, err
				}
				node.Attributes = withClause
			}
		} else {

			for _, spec := range node.Specifiers {
				err := this.checkUnreserved(struct {
					start int
					end   int
					name  string
				}{start: spec.Local.Start, end: spec.Local.End, name: spec.Local.Name})

				if err != nil {
					return nil, err
				}

				err = this.checkLocalExport(struct {
					start int
					end   int
					name  string
				}{start: spec.Local.Start, end: spec.Local.End, name: spec.Local.Name})

				if err != nil {
					return nil, err
				}

				if spec.Local.Type == NODE_LITERAL {
					return nil, this.raise(spec.Local.Start, "A string literal cannot be used as an exported binding without `from`.")
				}
			}

			node.Source = nil
			if this.getEcmaVersion() >= 16 {
				node.Attributes = []*Node{}
			}
		}
		this.semicolon()
	}
	return this.finishNode(node, NODE_EXPORT_NAMED_DECLARATION), nil
}

func (this *Parser) checkLocalExport(opts struct {
	start int
	end   int
	name  string
}) error {
	panic("unimplemented")
}

func (this *Parser) parseExportSpecifiers(exports map[string]*Node) ([]*Node, error) {
	panic("unimplemented")
}

func (this *Parser) parseWithClause() ([]*Node, error) {
	panic("unimplemented")
}

func (this *Parser) checkVariableExport(exports map[string]*Node, declarations []*Node) error {
	panic("unimplemented")
}

func (this *Parser) parseExportDeclaration(node *Node) (*Node, error) {
	panic("unimplemented")
}

func (this *Parser) shouldParseExportStatement() bool {
	panic("unimplemented")
}

func (this *Parser) checkExport(exports map[string]*Node, val struct {
	s string
	n *Node
}, start int) error {
	panic("unimplemented")
}

func (this *Parser) parseExportAllDeclaration(node *Node, exports map[string]*Node) (*Node, error) {
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
	this.next(false)

	expr, err := this.parseParenExpression()
	if err != nil {
		return nil, err
	}
	node.Discriminant = expr
	node.Cases = []*Node{}
	err = this.expect(TOKEN_BRACEL)
	if err != nil {
		return nil, err
	}
	this.Labels = append(this.Labels, Label{Kind: "switch"})
	this.enterScope(0)

	// Statements under must be grouped (by label) in SwitchCase
	// nodes. `cur` is used to keep the node that we are currently
	// adding statements to.

	var cur *Node
	sawDefault := false
	for this.Type.identifier != TOKEN_BRACER {
		if this.Type.identifier == TOKEN_CASE || this.Type.identifier == TOKEN_DEFAULT {
			isCase := this.Type.identifier == TOKEN_CASE
			if cur != nil {
				return this.finishNode(cur, NODE_SWITCH_CASE), nil
			}
			cur = this.startNode()
			node.Cases = append(node.Cases, cur)

			cur.ConsequentSlice = []*Node{}
			this.next(false)
			if isCase {
				test, err := this.parseExpression("", nil)
				if err != nil {
					return nil, err
				}
				cur.Test = test

			} else {
				if sawDefault {
					return nil, this.raiseRecoverable(this.LastTokStart, "Multiple default clauses")
				}
				sawDefault = true
				cur.Test = nil
			}
			err = this.expect(TOKEN_COLON)
			if err != nil {
				return nil, err
			}
		} else {
			if cur == nil {
				return nil, this.unexpected(nil)
			}
			stmt, err := this.parseStatement("", false, nil)

			if err != nil {
				return nil, err
			}

			cur.ConsequentSlice = append(cur.ConsequentSlice, stmt)

		}
	}
	this.exitScope()
	if cur != nil {
		return this.finishNode(cur, NODE_SWITCH_CASE), nil
	}
	this.next(false)
	this.Labels = this.Labels[:len(this.Labels)-1]
	return this.finishNode(node, NODE_SWITCH_STATEMENT), nil
}

func (this *Parser) parseReturnStatement(node *Node) (*Node, error) {
	if !this.inFunction() && !this.options.AllowReturnOutsideFunction {
		return nil, this.raise(this.start, "'return' outside of function")
	}

	this.next(false)

	// In `return` (and `break`/`continue`), the keywords with
	// optional arguments, we eagerly look for a semicolon or the
	// possibility to insert one.

	if this.eat(TOKEN_SEMI) || this.insertSemicolon() {
		node.Argument = nil
	} else {
		expr, err := this.parseExpression("", nil)
		if err != nil {
			return nil, err
		}
		node.Argument = expr
		this.semicolon()
	}
	return this.finishNode(node, NODE_RETURN_STATEMENT), nil
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
	this.next(false)
	test, err := this.parseParenExpression()
	if err != nil {
		return nil, err
	}

	node.Test = test
	// allow function declarations in branches, but only in non-strict mode
	statement, err := this.parseStatement("if", false, nil)
	if err != nil {
		return nil, err
	}
	node.Consequent = statement

	if this.eat(TOKEN_ELSE) {
		alternate, err := this.parseStatement("if", false, nil)
		if err != nil {
			return nil, err
		}
		node.Alternate = alternate
	}

	return this.finishNode(node, NODE_IF_STATEMENT), nil
}

const (
	FUNC_STATEMENT         = 1
	FUNC_HANGING_STATEMENT = 2
	FUNC_NULLABLE_ID       = 4
)

func (this *Parser) parseFunctionStatement(node *Node, isAsync bool, declarationPosition bool) (*Node, error) {
	this.next(false)
	if declarationPosition {
		function, err := this.parseFunction(node, FUNC_STATEMENT|0, false, isAsync, "")
		if err != nil {
			return nil, err
		}
		return function, nil
	} else {
		function, err := this.parseFunction(node, FUNC_STATEMENT|FUNC_HANGING_STATEMENT, false, isAsync, "")
		if err != nil {
			return nil, err
		}
		return function, nil
	}
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
		_, err := this.parseVar(init, true, kind, false)

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

	isForOf = this.getEcmaVersion() >= 6
	if this.Type.identifier == TOKEN_IN || (isForOf && this.isContextual("of")) {
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

func (this *Parser) parseVar(node *Node, isFor bool, kind DeclarationKind, allowMissingInitializer bool) (*Node, error) {
	node.Declarations = []*Node{}
	node.DeclarationKind = kind
	for {
		decl := this.startNode()
		err := this.parseVarId(decl, kind)

		if err != nil {
			return nil, err
		}

		if this.eat(TOKEN_EQ) {
			forInit := ""
			if isFor {
				forInit = "isFor"
			}
			declInit, err := this.parseMaybeAssign(forInit, nil)
			if err != nil {
				return nil, err
			}
			decl.Init = declInit
		} else if !allowMissingInitializer && kind == CONST && !(this.Type.identifier == TOKEN_IN || (this.getEcmaVersion() >= 6 && this.isContextual("of"))) {
			return nil, this.unexpected(nil)
		} else if !allowMissingInitializer && decl.Id.Type != NODE_IDENTIFIER && !(isFor && (this.Type.identifier == TOKEN_IN || this.isContextual("of"))) {
			return nil, this.raise(this.LastTokEnd, "Complex binding patterns require an initialization value")
		} else {
			decl.Init = nil
		}
		node.Declarations = append(node.Declarations, this.finishNode(decl, NODE_VARIABLE_DECLARATOR))
		if !this.eat(TOKEN_COMMA) {
			break
		}
	}
	return node, nil
}

func (this *Parser) parseVarId(decl *Node, kind DeclarationKind) error {
	declarationIdentifier, err := this.parseBindingAtom()
	if err != nil {
		return err
	}
	decl.Id = declarationIdentifier
	if kind == VAR {
		err := this.checkLValPattern(decl.Id, BIND_VAR, struct {
			check bool
			hash  map[string]bool
		}{check: false})

		if err != nil {
			return err
		}
	} else {
		err := this.checkLValPattern(decl.Id, BIND_LEXICAL, struct {
			check bool
			hash  map[string]bool
		}{check: false})

		if err != nil {
			return err
		}
	}
	return nil
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

func (this *Parser) parseFunction(node *Node, statement Flags, allowExpressionBody bool, isAsync bool, forInit string) (*Node, error) {
	this.initFunction(node)
	if this.getEcmaVersion() >= 9 || this.getEcmaVersion() >= 6 && !isAsync {
		if this.Type.identifier == TOKEN_STAR && (statement&FUNC_HANGING_STATEMENT == FUNC_HANGING_STATEMENT) {
			return nil, this.unexpected(nil)
		}

		node.IsGenerator = this.eat(TOKEN_STAR)
	}
	if this.getEcmaVersion() >= 8 {
		node.IsAsync = isAsync
	}

	if statement&FUNC_STATEMENT == FUNC_STATEMENT {
		if statement&FUNC_NULLABLE_ID == FUNC_NULLABLE_ID && this.Type.identifier == TOKEN_NAME {
			node.Id = nil
		} else {
			identifier, err := this.parseIdent(false)
			if err != nil {
				return nil, err
			}
			node.Id = identifier
		}
	}
	if node.Id != nil && !(statement&FUNC_HANGING_STATEMENT == FUNC_HANGING_STATEMENT) {
		// If it is a regular function declaration in sloppy mode, then it is
		// subject to Annex B semantics (BIND_FUNCTION). Otherwise, the binding
		// mode depends on properties of the current scope (see
		// treatFunctionsAsVar).

		if this.Strict || node.IsGenerator || node.IsAsync {
			if this.treatFunctionsAsVar() {
				err := this.checkLValSimple(node.Id, BIND_VAR, struct {
					check bool
					hash  map[string]bool
				}{check: false})
				if err != nil {
					return nil, err
				}
			} else {
				err := this.checkLValSimple(node.Id, BIND_LEXICAL, struct {
					check bool
					hash  map[string]bool
				}{check: false})
				if err != nil {
					return nil, err
				}
			}
			err := this.checkLValSimple(node.Id, BIND_FUNCTION, struct {
				check bool
				hash  map[string]bool
			}{check: false})
			if err != nil {
				return nil, err
			}
		}
	}

	oldYieldPos, oldAwaitPos, oldAwaitIdentPos := this.YieldPos, this.AwaitPos, this.AwaitIdentPos
	this.YieldPos = 0
	this.AwaitPos = 0
	this.AwaitIdentPos = 0
	this.enterScope(functionFlags(node.IsAsync, node.IsGenerator))

	if !(statement&FUNC_STATEMENT == FUNC_STATEMENT) {
		if this.Type.identifier == TOKEN_NAME {
			ident, err := this.parseIdent(false)
			if err != nil {
				return nil, err
			}
			node.Id = ident
		} else {
			node.Id = nil
		}
	}

	err := this.parseFunctionParams(node)
	if err != nil {
		return nil, err
	}
	err = this.parseFunctionBody(node, allowExpressionBody, false, forInit)

	if err != nil {
		return nil, err
	}

	this.YieldPos = oldYieldPos
	this.AwaitPos = oldAwaitPos
	this.AwaitIdentPos = oldAwaitIdentPos

	if statement&FUNC_STATEMENT == FUNC_STATEMENT {
		return this.finishNode(node, NODE_FUNCTION_DECLARATION), nil
	}
	return this.finishNode(node, NODE_FUNCTION_EXPRESSION), nil

}

func (this *Parser) parseFunctionParams(node *Node) error {
	err := this.expect(TOKEN_PARENL)
	if err != nil {
		return err
	}
	bindingList, err := this.parseBindingList(TOKEN_PARENR, false, this.getEcmaVersion() >= 8, false)
	if err != nil {
		return err
	}

	node.Params = bindingList
	err = this.checkYieldAwaitInDefaultParams()
	if err != nil {
		return err
	}
	return nil
}

func (p *Parser) parseClass(node *Node, isStatement bool) (*Node, error) {
	panic("unimplemented")
}
