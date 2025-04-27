package parser

import (
	"errors"
	"slices"
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

	this.adaptDirectivePrologue(node.Body)
	return this.finishNode(node, NODE_PROGRAM), nil
}

func (this *Parser) parseStatement(context string, topLevel bool, exports map[string]*Node) (*Node, error) {
	//this.printState()
	startType, node := this.Type, this.startNode()
	kind := KIND_NOT_INITIALIZED

	if this.isLet(context) {
		startType = tokenTypes[TOKEN_VAR]
		kind = KIND_DECLARATION_LET
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
			return nil, this.unexpected("", nil)
		}
		functionStatement, err := this.parseFunctionStatement(node, false, len(context) == 0)

		if err != nil {
			return nil, err
		}

		return functionStatement, nil

	case TOKEN_CLASS:
		if len(context) != 0 {
			return nil, this.unexpected("Cant parse class in context", nil)
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
		if kind == KIND_NOT_INITIALIZED {
			if k, ok := this.Value.(Kind); ok {
				kind = k
			} else {
				panic("We were expectin a DeclarationKind from node.Value, didn't happen so we are now here.")
			}
		}

		if len(context) != 0 && kind != KIND_DECLARATION_VAR {
			return nil, this.unexpected("Expected empty context, and not KIND_DECLARATION_VAR", nil)
		}

		varStatement, err := this.parseVarStatement(node, kind, false)

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
				return nil, this.unexpected("Expected empty context", nil)
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
	after := rune(-1)
	if next+8 < len(this.input) {
		after = rune(this.input[next+8])
	}

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
				return nil, this.unexpected("Can't have STRING as current type", nil)
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

				this.checkLocalExport(spec.Local)

				if spec.Local.Type == NODE_LITERAL {
					return nil, this.raise(spec.Local.Start, "A string literal cannot be used as an exported binding without `from`.")
				}
			}

			node.Source = nil
			if this.getEcmaVersion() >= 16 {
				node.Attributes = []*Node{}
			}
		}
		err = this.semicolon()
		if err != nil {
			return nil, err
		}
	}
	return this.finishNode(node, NODE_EXPORT_NAMED_DECLARATION), nil
}

func (this *Parser) checkLocalExport(opts *Node) {
	if slices.Index(this.ScopeStack[0].Lexical, opts.Name) == -1 &&
		slices.Index(this.ScopeStack[0].Var, opts.Name) == -1 {
		this.UndefinedExports[opts.Name] = opts
	}
}

func (this *Parser) parseExportSpecifiers(exports map[string]*Node) ([]*Node, error) {
	nodes, first := []*Node{}, true
	// export { x, y as z } [from '...']
	err := this.expect(TOKEN_BRACEL)

	if err != nil {
		return nil, err
	}
	for !this.eat(TOKEN_BRACER) {
		if !first {
			err := this.expect(TOKEN_COMMA)
			if err != nil {
				return nil, err
			}
			if this.afterTrailingComma(TOKEN_BRACER, false) {
				break
			}
		} else {
			first = false
		}
		exportSpecifier, err := this.parseExportSpecifier(exports)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, exportSpecifier)

	}
	return nodes, nil
}

func (this *Parser) parseExportSpecifier(exports map[string]*Node) (*Node, error) {
	node := this.startNode()
	moduleExportName, err := this.parseModuleExportName()
	if err != nil {
		return nil, err
	}
	node.Local = moduleExportName

	if this.eatContextual("as") {
		moduleExportName, err := this.parseModuleExportName()
		if err != nil {
			return nil, err
		}

		node.Exported = moduleExportName
	} else {
		node.Exported = node.Local
	}
	this.checkExport(
		exports,
		struct {
			s string
			n *Node
		}{
			n: node.Exported,
		},
		node.Exported.Start,
	)

	return this.finishNode(node, NODE_EXPORT_SPECIFIER), nil
}

func (this *Parser) parseModuleExportName() (*Node, error) {
	if this.getEcmaVersion() >= 13 && this.Type.identifier == TOKEN_STRING {
		stringLiteral, err := this.parseLiteral(this.Value)

		if err != nil {
			return nil, err
		}

		if val, ok := stringLiteral.Value.(string); ok {
			if loneSurrogate.Match([]byte(val)) {
				return nil, this.raise(stringLiteral.Start, "An export name cannot include a lone surrogate.")
			}
			return stringLiteral, nil
		}
	}
	ident, err := this.parseIdent(true)
	if err != nil {
		return nil, err
	}
	return ident, nil
}

func (this *Parser) parseWithClause() ([]*Node, error) {
	nodes := []*Node{}
	if !this.eat(TOKEN_WITH) {
		return nodes, nil
	}

	err := this.expect(TOKEN_BRACEL)
	if err != nil {
		return nil, err
	}
	attributeKeys := map[string]struct{}{}
	first := true
	for !this.eat(TOKEN_BRACER) {
		if !first {
			err := this.expect(TOKEN_COMMA)
			if err != nil {
				return nil, err
			}
			if this.afterTrailingComma(TOKEN_BRACER, false) {
				break
			}
		} else {
			first = false
		}

		attr, err := this.parseImportAttribute()
		if err != nil {
			return nil, err
		}
		keyName := ""

		if attr.Key.Type == NODE_IDENTIFIER {
			keyName = attr.Key.Name
		} else {
			if val, ok := attr.Key.Value.(string); ok {
				keyName = val
			}
		}

		if _, found := attributeKeys[keyName]; found {
			return nil, this.raiseRecoverable(attr.Key.Start, "Duplicate attribute key '"+keyName+"'")
		}
		attributeKeys[keyName] = struct{}{}
		nodes = append(nodes, attr)
	}
	return nodes, nil
}

func (this *Parser) parseImportAttribute() (*Node, error) {
	node := this.startNode()

	if this.Type.identifier == TOKEN_STRING {
		exprAtom, err := this.parseExprAtom(nil, "", false)

		if err != nil {
			return nil, err
		}
		node.Key = exprAtom
	} else {
		ident, err := this.parseIdent(this.options.AllowReserved) // questions to be answered: this.parseIdent(this.options.allowReserved !== "never"), we'll figure it out :)

		if err != nil {
			return nil, err
		}
		node.Key = ident
	}
	err := this.expect(TOKEN_COLON)
	if err != nil {
		return nil, err
	}

	if this.Type.identifier != TOKEN_STRING {
		return nil, this.unexpected("Expected TOKEN_STRING", nil)
	}

	exprAtom, err := this.parseExprAtom(nil, "", false)

	if err != nil {
		return nil, err
	}

	node.Value = exprAtom
	return this.finishNode(node, NODE_IMPORT_ATTRIBUTE), nil
}

func (this *Parser) checkVariableExport(exports map[string]*Node, declarations []*Node) error {
	if exports == nil {
		return errors.New("exports was defined, i guess we don't want that?")
	}

	for _, decl := range declarations {
		err := this.checkPatternExport(exports, decl.Id)

		if err != nil {
			return err
		}
	}
	return nil
}

func (this *Parser) checkPatternExport(exports map[string]*Node, pat *Node) error {
	t := pat.Type

	switch t {
	case NODE_IDENTIFIER:
		this.checkExport(exports, struct {
			s string
			n *Node
		}{n: pat}, pat.Start)
	case NODE_OBJECT_PATTERN:
		for _, prop := range pat.Properties {
			err := this.checkPatternExport(exports, prop)
			if err != nil {
				return err
			}
		}
	case NODE_ARRAY_PATTERN:
		for _, elt := range pat.Elements {
			if elt != nil {
				err := this.checkPatternExport(exports, elt)
				if err != nil {
					return err
				}
			}
		}
	case NODE_PROPERTY:
		if val, ok := pat.Value.(*Node); ok {
			err := this.checkPatternExport(exports, val)
			if err != nil {
				return err
			}
		} else {
			panic("we were expecting *Node from node.Value")
		}
	case NODE_ASSIGNMENT_PATTERN:

		err := this.checkPatternExport(exports, pat.Left)
		if err != nil {
			return err
		}
	case NODE_REST_ELEMENT:
		err := this.checkPatternExport(exports, pat.Argument)
		if err != nil {
			return err
		}
	}
	return nil
}

func (this *Parser) parseExportDeclaration(node *Node) (*Node, error) {
	stmt, err := this.parseStatement("", false, nil)
	if err != nil {
		return nil, err
	}
	return stmt, nil
}

func (this *Parser) shouldParseExportStatement() bool {
	return this.Type.keyword == "var" ||
		this.Type.keyword == "const" ||
		this.Type.keyword == "class" ||
		this.Type.keyword == "function" ||
		this.isLet("") ||
		this.isAsyncFunction()
}

func (this *Parser) checkExport(exports map[string]*Node, val struct {
	s string
	n *Node
}, start int) error {
	if exports == nil {
		return nil
	}

	name := ""

	if len(val.s) == 0 {
		name = val.n.Name
	} else {
		name = val.s
	}

	if _, found := exports[name]; found {
		return this.raiseRecoverable(start, "Duplicate export '"+name+"'")
	}

	return nil
}

func (this *Parser) parseExportAllDeclaration(node *Node, exports map[string]*Node) (*Node, error) {
	if this.getEcmaVersion() >= 11 {
		if this.eatContextual("as") {
			moduleExportName, err := this.parseModuleExportName()
			if err != nil {
				return nil, err
			}

			node.Exported = moduleExportName

			this.checkExport(exports, struct {
				s string
				n *Node
			}{n: node.Exported}, this.LastTokStart)
		} else {
			node.Exported = nil
		}
	}
	err := this.expectContextual("from")

	if err != nil {
		return nil, err
	}

	if this.Type.identifier != TOKEN_STRING {
		return nil, this.unexpected("Expected TOKEN_STRING", nil)
	}
	exprAtom, err := this.parseExprAtom(nil, "", false)
	if err != nil {
		return nil, err
	}
	node.Source = exprAtom
	if this.getEcmaVersion() >= 16 {
		attr, err := this.parseWithClause()
		if err != nil {
			return nil, err
		}
		node.Attributes = attr
	}

	err = this.semicolon()
	if err != nil {
		return nil, err
	}
	return this.finishNode(node, NODE_EXPORT_ALL_DECLARATION), nil
}

func (this *Parser) expectContextual(name string) error {
	if !this.eatContextual(name) {
		return this.unexpected("Expected context from context stack", nil)
	}
	return nil
}

func (this *Parser) parseImport(node *Node) (*Node, error) {
	this.next(false)

	// import '...'
	if this.Type.identifier == TOKEN_STRING {
		node.Specifiers = []*Node{}
		exprAtom, err := this.parseExprAtom(nil, "", false)
		if err != nil {
			return nil, err
		}
		node.Source = exprAtom
	} else {
		importSpecifiers, err := this.parseImportSpecifiers()
		if err != nil {
			return nil, err
		}

		node.Specifiers = importSpecifiers
		err = this.expectContextual("from")
		if err != nil {
			return nil, err
		}
		if this.Type.identifier == TOKEN_STRING {
			exprAtom, err := this.parseExprAtom(nil, "", false)
			if err != nil {
				return nil, err
			}
			node.Source = exprAtom
		} else {
			return nil, this.unexpected("Expected TOKEN_STRING", nil)
		}
	}
	if this.getEcmaVersion() >= 16 {
		withClause, err := this.parseWithClause()
		if err != nil {
			return nil, err
		}
		node.Attributes = withClause
	}
	err := this.semicolon()
	if err != nil {
		return nil, err
	}
	return this.finishNode(node, NODE_IMPORT_DECLARATION), nil
}

func (this *Parser) parseImportSpecifiers() ([]*Node, error) {
	nodes, first := []*Node{}, true
	if this.Type.identifier == TOKEN_NAME {
		importDefaultSpecifier, err := this.parseImportDefaultSpecifier()
		if err != nil {
			return nil, err
		}

		nodes = append(nodes, importDefaultSpecifier)
		if !this.eat(TOKEN_COMMA) {
			return nodes, nil
		}
	}
	if this.Type.identifier == TOKEN_STAR {
		importNamespaceSpecifier, err := this.parseImportNamespaceSpecifier()
		if err != nil {
			return nil, err
		}

		nodes = append(nodes, importNamespaceSpecifier)
		return nodes, nil
	}
	err := this.expect(TOKEN_BRACEL)
	if err != nil {
		return nil, err
	}

	for !this.eat(TOKEN_BRACER) {
		if !first {
			err := this.expect(TOKEN_COMMA)
			if err != nil {
				return nil, err
			}
			if this.afterTrailingComma(TOKEN_BRACER, false) {
				break
			}
		} else {
			first = false
		}
		importSpecifier, err := this.parseImportSpecifier()
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, importSpecifier)
	}
	return nodes, nil
}

func (this *Parser) parseImportSpecifier() (*Node, error) {
	node := this.startNode()
	moduleExportName, err := this.parseModuleExportName()
	if err != nil {
		return nil, err
	}
	node.Imported = moduleExportName
	if this.eatContextual("as") {
		ident, err := this.parseIdent(false)
		if err != nil {
			return nil, err
		}
		node.Local = ident
	} else {
		err := this.checkUnreserved(struct {
			start int
			end   int
			name  string
		}{start: node.Imported.Start, end: node.Imported.End, name: node.Imported.Name})
		if err != nil {
			return nil, err
		}

		node.Local = node.Imported
	}
	err = this.checkLValSimple(node.Local, BIND_LEXICAL, struct {
		check bool
		hash  map[string]bool
	}{check: false})

	if err != nil {
		return nil, err
	}

	return this.finishNode(node, NODE_IMPORT_SPECIFIER), nil
}

func (this *Parser) parseImportNamespaceSpecifier() (*Node, error) {
	node := this.startNode()
	this.next(false)
	err := this.expectContextual("as")

	if err != nil {
		return nil, err
	}

	ident, err := this.parseIdent(false)

	if err != nil {
		return nil, err
	}
	node.Local = ident
	err = this.checkLValSimple(node.Local, BIND_LEXICAL, struct {
		check bool
		hash  map[string]bool
	}{check: false})

	if err != nil {
		return nil, err
	}
	return this.finishNode(node, NODE_IMPORT_NAMESPACE_SPECIFIER), nil
}

func (this *Parser) parseImportDefaultSpecifier() (*Node, error) {
	// import defaultObj, { x, y as z } from '...'
	node := this.startNode()
	ident, err := this.parseIdent(false)
	if err != nil {
		return nil, err
	}
	node.Local = ident
	err = this.checkLValSimple(node.Local, BIND_LEXICAL, struct {
		check bool
		hash  map[string]bool
	}{check: false})

	if err != nil {
		return nil, err
	}

	return this.finishNode(node, NODE_IMPORT_DEFAULT_SPECIFIER), nil
}

func (this *Parser) parseExpressionStatement(node *Node, expr *Node) (*Node, error) {
	node.Expression = expr
	err := this.semicolon()
	if err != nil {
		return nil, err
	}
	return this.finishNode(node, NODE_EXPRESSION_STATEMENT), nil
}

func (this *Parser) parseEmptyStatement(node *Node) (*Node, error) {
	this.next(false)
	return this.finishNode(node, NODE_EMPTY_STATEMENT), nil
}

func (this *Parser) parseWithStatement(node *Node) (*Node, error) {
	if this.Strict {
		this.raise(this.start, "'with' in strict mode")
	}
	this.next(false)
	parenthesizedExpr, err := this.parseParenExpression()
	if err != nil {
		return nil, err
	}
	node.Object = parenthesizedExpr
	stmt, err := this.parseStatement("with", false, nil)
	node.BodyNode = stmt
	return this.finishNode(node, NODE_WITH_STATEMENT), nil
}

func (this *Parser) parseWhileStatement(node *Node) (*Node, error) {
	this.next(false)
	parenthesizedExpression, err := this.parseParenExpression()
	if err != nil {
		return nil, err
	}
	node.Test = parenthesizedExpression
	this.Labels = append(this.Labels, Label{Kind: "loop"})

	stmt, err := this.parseStatement("while", false, nil)
	if err != nil {
		return nil, err
	}

	node.BodyNode = stmt
	this.Labels = this.Labels[:len(this.Labels)-1]

	return this.finishNode(node, NODE_WHILE_STATEMENT), nil
}

func (this *Parser) parseVarStatement(node *Node, kind Kind, allowMissingInitializer bool) (*Node, error) {
	this.next(false)
	_, err := this.parseVar(node, false, kind, allowMissingInitializer)
	if err != nil {
		return nil, err
	}
	err = this.semicolon()
	if err != nil {
		return nil, err
	}
	return this.finishNode(node, NODE_VARIABLE_DECLARATION), nil
}

func (this *Parser) parseTryStatement(node *Node) (*Node, error) {
	this.next(false)

	blok, err := this.parseBlock(false, nil, false)
	if err != nil {
		return nil, err
	}
	node.Block = blok
	node.Handler = nil
	if this.Type.identifier == TOKEN_CATCH {
		clause := this.startNode()
		this.next(false)
		if this.eat(TOKEN_PARENL) {
			catchClauseParam, err := this.parseCatchClauseParam()
			if err != nil {
				return nil, err
			}
			clause.Param = catchClauseParam
		} else {
			if this.getEcmaVersion() < 10 {
				return nil, this.unexpected("Wrong ecma version", nil)
			}
			clause.Param = nil
			this.enterScope(0)
		}
		block, err := this.parseBlock(false, nil, false)
		if err != nil {
			return nil, err
		}

		clause.BodyNode = block
		this.exitScope()
		node.Handler = this.finishNode(clause, NODE_CATCH_CLAUSE)
	}
	if this.eat(TOKEN_FINALLY) {
		block, err := this.parseBlock(false, nil, false)
		if err != nil {
			return nil, err
		}
		node.Finalizer = block
	} else {
		node.Finalizer = nil
	}

	if node.Handler == nil && node.Finalizer == nil {
		return nil, this.raise(node.Start, "Missing catch or finally clause")
	}

	return this.finishNode(node, NODE_TRY_STATEMENT), nil
}

func (this *Parser) parseCatchClauseParam() (*Node, error) {
	param, err := this.parseBindingAtom()
	if err != nil {
		return nil, err
	}

	simple := param.Type == NODE_IDENTIFIER
	if simple {
		this.enterScope(SCOPE_SIMPLE_CATCH)
	} else {
		this.enterScope(0)
	}

	if simple {
		err := this.checkLValPattern(param, BIND_SIMPLE_CATCH, struct {
			check bool
			hash  map[string]bool
		}{check: false})
		if err != nil {
			return nil, err
		}
	} else {
		err := this.checkLValPattern(param, BIND_LEXICAL, struct {
			check bool
			hash  map[string]bool
		}{check: false})
		if err != nil {
			return nil, err
		}
	}
	err = this.expect(TOKEN_PARENR)

	if err != nil {
		return nil, err
	}

	return param, nil
}

func (this *Parser) parseThrowStatement(node *Node) (*Node, error) {
	this.next(false)
	if lineBreak.Match(this.input[this.LastTokEnd:this.start]) {
		return nil, this.raise(this.LastTokEnd, "Illegal newline after throw")
	}
	expr, err := this.parseExpression("", nil)
	if err != nil {
		return nil, err
	}
	node.Argument = expr
	err = this.semicolon()
	if err != nil {
		return nil, err
	}
	return this.finishNode(node, NODE_THROW_STATEMENT), nil
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
				return nil, this.unexpected("cur cant be nil", nil)
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
		err = this.semicolon()
		if err != nil {
			return nil, err
		}
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
			nextCh = rune(this.input[pos])
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
		function, err := this.parseFunction(node, FUNC_STATEMENT, false, isAsync, "")
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
			return nil, this.unexpected("Something about having an await clause and semicolon", &awaitAt)
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

		kind := KIND_NOT_INITIALIZED
		if isLet {
			kind = KIND_DECLARATION_LET
		} else {
			if k, ok := this.Value.(Kind); ok {
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
						return nil, this.unexpected("", &awaitAt)
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
			return nil, this.unexpected("", &awaitAt)
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
				return nil, this.unexpected("", &awaitAt)
			}
			node.Await = true
		} else if isForOf && this.getEcmaVersion() >= 8 {
			if init.Start == initPos && !containsEsc && init.Type == NODE_IDENTIFIER && init.Name == "async" {
				return nil, this.unexpected("", nil)
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
		return nil, this.unexpected("", &awaitAt)
	}

	forStatement, err := this.parseFor(node, init)

	if err != nil {
		return nil, err
	}
	return forStatement, nil
}

func (this *Parser) parseFor(node *Node, init *Node) (*Node, error) {
	node.Init = init
	err := this.expect(TOKEN_SEMI)

	if err != nil {
		return nil, err
	}

	if this.Type.identifier == TOKEN_SEMI {
		node.Test = nil
	} else {
		expr, err := this.parseExpression("", nil)

		if err != nil {
			return nil, err
		}

		node.Test = expr
	}

	err = this.expect(TOKEN_SEMI)

	if err != nil {
		return nil, err
	}

	if this.Type.identifier == TOKEN_PARENR {
		node.Update = nil
	} else {
		expr, err := this.parseExpression("", nil)

		if err != nil {
			return nil, err
		}
		node.Update = expr
	}

	err = this.expect(TOKEN_PARENR)
	if err != nil {
		return nil, err
	}
	stmt, err := this.parseStatement("for", false, nil)

	if err != nil {
		return nil, err
	}

	node.BodyNode = stmt
	this.exitScope()
	this.Labels = this.Labels[:len(this.Labels)-1]
	return this.finishNode(node, NODE_FOR_STATEMENT), nil
}

func (this *Parser) parseForIn(node *Node, init *Node) (*Node, error) {
	isForIn := this.Type.identifier == TOKEN_IN
	this.next(false)

	if init.Type == NODE_VARIABLE_DECLARATION && init.Declarations[0].Init != nil && (!isForIn || this.getEcmaVersion() < 8 || this.Strict || init.Kind != KIND_DECLARATION_VAR || init.Declarations[0].Id.Type != NODE_IDENTIFIER) {
		return nil, this.raise(init.Start, `for-in or for-of loop variable declaration may not have an initializer`)
	}
	node.Left = init
	if isForIn {
		expr, err := this.parseExpression("", nil)
		if err != nil {
			return nil, err
		}
		node.Rigth = expr
	} else {
		maybeAssign, err := this.parseMaybeAssign("", nil, nil)
		if err != nil {
			return nil, err
		}
		node.Rigth = maybeAssign
	}

	err := this.expect(TOKEN_PARENR)

	if err != nil {
		return nil, err
	}
	stmt, err := this.parseStatement("for", false, nil)
	if err != nil {
		return nil, err
	}
	node.BodyNode = stmt
	this.exitScope()
	this.Labels = this.Labels[:len(this.Labels)-1]

	if isForIn {
		return this.finishNode(node, NODE_FOR_IN_STATEMENT), nil
	}
	return this.finishNode(node, NODE_FOR_OF_STATEMENT), nil

}

func (this *Parser) parseVar(node *Node, isFor bool, kind Kind, allowMissingInitializer bool) (*Node, error) {
	node.Declarations = []*Node{}
	node.Kind = kind
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
			declInit, err := this.parseMaybeAssign(forInit, nil, nil)
			if err != nil {
				return nil, err
			}
			decl.Init = declInit
		} else if !allowMissingInitializer && kind == KIND_DECLARATION_CONST && !(this.Type.identifier == TOKEN_IN || (this.getEcmaVersion() >= 6 && this.isContextual("of"))) {
			return nil, this.unexpected("Missing initializer in for..of loop", nil)
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

func (this *Parser) parseVarId(decl *Node, kind Kind) error {
	declarationIdentifier, err := this.parseBindingAtom()
	if err != nil {
		return err
	}
	decl.Id = declarationIdentifier
	if kind == KIND_DECLARATION_VAR {
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

func (this *Parser) eatContextual(name string) bool {
	if !this.isContextual(name) {
		return false
	}
	this.next(false)
	return true
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
		err := this.semicolon()
		if err != nil {
			return nil, err
		}
	}

	return this.finishNode(node, NODE_DO_WHILE_STATEMENT), nil
}

func (this *Parser) parseDebuggerStatement(node *Node) (*Node, error) {
	this.next(false)
	err := this.semicolon()
	if err != nil {
		return nil, err
	}
	return this.finishNode(node, NODE_DEBUGGER_STATEMENT), nil
}

func (this *Parser) parseBreakContinueStatement(node *Node, keyword string) (*Node, error) {
	isBreak := keyword == "break"
	this.next(false)
	if this.eat(TOKEN_SEMI) || this.insertSemicolon() {
		node.Label = nil
	} else if this.Type.identifier != TOKEN_NAME {
		return nil, this.unexpected("", nil)
	} else {
		ident, err := this.parseIdent(false)

		if err != nil {
			return nil, err
		}
		node.Label = ident
		err = this.semicolon()
		if err != nil {
			return nil, err
		}
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

func (this *Parser) parseBlock(createNewLexicalScope bool, node *Node, exitStrict bool) (*Node, error) {
	if node == nil {
		node = this.startNode()
	}
	node.Body = []*Node{}
	err := this.expect(TOKEN_BRACEL)
	if err != nil {
		return nil, err
	}
	if createNewLexicalScope {
		this.enterScope(0)
	}
	for this.Type.identifier != TOKEN_BRACER {
		stmt, err := this.parseStatement("", false, nil)
		if err != nil {
			return nil, err
		}
		node.Body = append(node.Body, stmt)
	}
	if exitStrict {
		this.Strict = false
	}
	this.next(false)

	if createNewLexicalScope {
		this.exitScope()
	}
	return this.finishNode(node, NODE_BLOCK_STATEMENT), nil
}

func (this *Parser) adaptDirectivePrologue(statements []*Node) {
	for i := 0; i < len(statements) && this.isDirectiveCandidate(statements[i]); {
		statements[i].Directive = statements[i].Expression.Raw[1 : len(statements[i].Expression.Raw)-2]
		i++
	}
}

func (this *Parser) isDirectiveCandidate(statement *Node) bool {
	literalAndString := false

	if statement.Expression != nil && statement.Expression.Type == NODE_LITERAL {
		_, ok := statement.Expression.Value.(string)
		literalAndString = ok
	}
	return this.getEcmaVersion() >= 5 && statement.Type == NODE_EXPRESSION_STATEMENT && literalAndString && /* Reject parenthesized strings.*/ (this.input[statement.Start] == '"' || this.input[statement.Start] == '\'')
}

func (this *Parser) parseFunction(node *Node, statement Flags, allowExpressionBody bool, isAsync bool, forInit string) (*Node, error) {
	this.initFunction(node)
	if this.getEcmaVersion() >= 9 || this.getEcmaVersion() >= 6 && !isAsync {
		if this.Type.identifier == TOKEN_STAR && (statement&FUNC_HANGING_STATEMENT == FUNC_HANGING_STATEMENT) {
			return nil, this.unexpected("Token was star and FUNC_HANGING_STATEMENT flag was set", nil)
		}

		node.IsGenerator = this.eat(TOKEN_STAR)
	}
	if this.getEcmaVersion() >= 8 {
		node.IsAsync = isAsync
	}

	if statement&FUNC_STATEMENT == FUNC_STATEMENT {

		if statement&FUNC_NULLABLE_ID == FUNC_NULLABLE_ID && this.Type.identifier != TOKEN_NAME {
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

func (this *Parser) parseClass(node *Node, isStatement bool) (*Node, error) {
	this.next(false)

	// ecma-262 14.6 Class Definitions
	// A class definition is always strict mode code.
	oldStrict := this.Strict
	this.Strict = true

	err := this.parseClassId(node, isStatement)
	if err != nil {
		return nil, err
	}
	err = this.parseClassSuper(node)
	if err != nil {
		return nil, err
	}
	privateNameMap, err := this.enterClassBody()
	if err != nil {
		return nil, err
	}
	classBody := this.startNode()
	hadConstructor := false
	classBody.Body = []*Node{}
	err = this.expect(TOKEN_BRACEL)
	if err != nil {
		return nil, err
	}
	for this.Type.identifier != TOKEN_BRACER {
		element, err := this.parseClassElement(node.SuperClass != nil)
		if err != nil {
			return nil, err
		}
		if element != nil {
			classBody.Body = append(classBody.Body, element)
			if element.Type == NODE_METHOD_DEFINITION && element.Kind == KIND_CONSTRUCTOR {
				if hadConstructor {
					return nil, this.raiseRecoverable(element.Start, "Duplicate constructor in the same class")
				}
				hadConstructor = true
			} else if element.Key != nil && element.Key.Type == NODE_PRIVATE_IDENTIFIER && isPrivateNameConflicted(privateNameMap, element) {
				return nil, this.raiseRecoverable(element.Key.Start, "Identifier #"+element.Key.Name+"has already been declared")
			}
		}
	}
	this.Strict = oldStrict
	this.next(false)
	node.BodyNode = this.finishNode(classBody, NODE_CLASS_BODY)
	err = this.exitClassBody()

	if err != nil {
		return nil, err
	}

	if isStatement {
		return this.finishNode(node, NODE_CLASS_DECLARATION), nil
	}
	return this.finishNode(node, NODE_CLASS_EXPRESSION), nil
}

func (this *Parser) exitClassBody() error {
	privateNameTop := this.PrivateNameStack[len(this.PrivateNameStack)-1]
	this.PrivateNameStack = this.PrivateNameStack[:len(this.PrivateNameStack)-1]

	if !this.options.CheckPrivateFields {
		return nil
	}
	stackLength := len(this.PrivateNameStack)

	var parent *PrivateName

	if stackLength != 0 {
		parent = this.PrivateNameStack[len(this.PrivateNameStack)-1]
	}

	for _, id := range privateNameTop.Used {
		if _, found := privateNameTop.Declared[id.Name]; found {
			if parent != nil {
				parent.Used = append(parent.Used, id)
			} else {
				return this.raiseRecoverable(id.Start, "Private field #"+id.Name+" must be declared in an enclosing class")
			}
		}
	}
	return nil
}

func isPrivateNameConflicted(privateNameMap map[string]string, element *Node) bool {
	name := element.Key.Name
	curr := privateNameMap[name]

	next := "true"
	if element.Type == NODE_METHOD_DEFINITION && (element.Kind == KIND_PROPERTY_GET || element.Kind == KIND_PROPERTY_SET) {
		if element.IsStatic {
			next = "s" + kindStringMap[element.Kind]
		} else {
			next = "i" + kindStringMap[element.Kind]
		}
	}

	// `class { get #a(){}; static set #a(_){} }` is also conflict.
	if curr == "iget" && next == "iset" || curr == "iset" && next == "iget" || curr == "sget" && next == "sset" || curr == "sset" && next == "sget" {
		privateNameMap[name] = "true"
		return false
	} else if len(curr) == 0 {
		privateNameMap[name] = next
		return false
	} else {
		return true
	}
}

func (this *Parser) parseClassElement(constructorAllowsSuper bool) (*Node, error) {
	if this.eat(TOKEN_SEMI) {
		return nil, nil
	}

	ecmaVersion := this.getEcmaVersion()
	node := this.startNode()
	keyName, isGenerator, isAsync, kind, isStatic := "", false, false, KIND_PROPERTY_METHOD, false

	if this.eatContextual("static") {
		// Parse static init block
		if ecmaVersion >= 13 && this.eat(TOKEN_BRACEL) {
			_, err := this.parseClassStaticBlock(node)
			if err != nil {
				return nil, err
			}
			return node, nil
		}

		if this.isClassElementNameStart() || this.Type.identifier == TOKEN_STAR {
			isStatic = true
		} else {
			keyName = "static"
		}
	}
	node.IsStatic = isStatic

	if len(keyName) == 0 && ecmaVersion >= 8 && this.eatContextual("async") {
		if (this.isClassElementNameStart() || this.Type.identifier == TOKEN_STAR) && !this.canInsertSemicolon() {
			isAsync = true
		} else {
			keyName = "async"
		}
	}

	if len(keyName) == 0 && (ecmaVersion >= 9 || !isAsync) && this.eat(TOKEN_STAR) {
		isGenerator = true
	}

	if len(keyName) == 0 && !isAsync && !isGenerator {
		lastValue := this.Value
		if this.eatContextual("get") || this.eatContextual("set") {
			if this.isClassElementNameStart() {
				if k, ok := lastValue.(Kind); ok {
					kind = k
				} else {
					panic("We were expecting this.Value to be kind, it wasn't")
				}
			} else {
				if str, ok := lastValue.(string); ok {
					keyName = str
				} else {
					panic("We were expecting this.Value to be string, it wasn't")
				}
			}
		}
	}

	// Parse element name
	if len(keyName) != 0 {
		// 'async', 'get', 'set', or 'static' were not a keyword contextually.
		// The last token is any of those. Make it the element name.
		node.Computed = false
		node.Key = this.startNodeAt(this.LastTokStart, this.LastTokStartLoc)
		node.Key.Name = keyName
		this.finishNode(node.Key, NODE_IDENTIFIER)
	} else {
		err := this.parseClassElementName(node)
		if err != nil {
			return nil, err
		}
	}
	// Parse element value
	if ecmaVersion < 13 || this.Type.identifier == TOKEN_PARENL || kind != KIND_PROPERTY_METHOD || isGenerator || isAsync {
		isConstructor := !node.IsStatic && checkKeyName(node, "constructor")
		allowsDirectSuper := isConstructor && constructorAllowsSuper
		// Couldn't move this check into the 'parseClassMethod' method for backward compatibility.
		if isConstructor && kind != KIND_PROPERTY_METHOD {
			return nil, this.raise(node.Key.Start, "Constructor can't have get/set modifier")
		}

		if isConstructor {
			node.Kind = KIND_CONSTRUCTOR
		} else {
			node.Kind = kind
		}
		_, err := this.parseClassMethod(node, isGenerator, isAsync, allowsDirectSuper)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := this.parseClassField(node)
		if err != nil {
			return nil, err
		}
	}

	return node, nil
}

func (this *Parser) parseClassStaticBlock(node *Node) (*Node, error) {
	node.Body = []*Node{}

	oldLabels := this.Labels
	this.Labels = []Label{}
	this.enterScope(SCOPE_CLASS_STATIC_BLOCK | SCOPE_SUPER)
	for this.Type.identifier != TOKEN_BRACER {
		stmt, err := this.parseStatement("", false, nil)
		if err != nil {
			return nil, err
		}
		node.Body = append(node.Body, stmt)
	}
	this.next(false)
	this.exitScope()
	this.Labels = oldLabels

	return this.finishNode(node, NODE_STATIC_BLOCK), nil
}

func (this *Parser) parseClassField(field *Node) (*Node, error) {
	if checkKeyName(field, "constructor") {
		return nil, this.raise(field.Key.Start, "Classes can't have a field named 'constructor'")
	} else if field.IsStatic && checkKeyName(field, "prototype") {
		return nil, this.raise(field.Key.Start, "Classes can't have a static field named 'prototype'")
	}

	if this.eat(TOKEN_EQ) {
		// To raise SyntaxError if 'arguments' exists in the initializer.
		this.enterScope(SCOPE_CLASS_FIELD_INIT | SCOPE_SUPER)
		maybeAssign, err := this.parseMaybeAssign("", nil, nil)
		if err != nil {
			return nil, err
		}
		field.Value = maybeAssign
		this.exitScope()
	} else {
		field.Value = nil
	}
	this.semicolon()

	return this.finishNode(field, NODE_PROPERTY_DEFINITION), nil
}

func (this *Parser) parseClassMethod(method *Node, isGenerator bool, isAsync bool, allowsDirectSuper bool) (*Node, error) {
	// Check key and flags
	key := method.Key
	if method.Kind == KIND_CONSTRUCTOR {
		if isGenerator {
			return nil, this.raise(key.Start, "Constructor can't be a generator")
		}
		if isAsync {
			return nil, this.raise(key.Start, "Constructor can't be an async method")
		}
	} else if method.IsStatic && checkKeyName(method, "prototype") {
		return nil, this.raise(key.Start, "Classes may not have a static property named prototype")
	}

	// Parse value
	value, err := this.parseMethod(isGenerator, isAsync, allowsDirectSuper)
	if err != nil {
		return nil, err
	}
	method.Value = value

	// Check value
	if method.Kind == KIND_PROPERTY_GET && len(value.Params) != 0 {
		return nil, this.raiseRecoverable(value.Start, "getter should have no params")
	}

	if method.Kind == KIND_PROPERTY_SET && len(value.Params) != 1 {
		return nil, this.raiseRecoverable(value.Start, "setter should have exactly one param")
	}

	if method.Kind == KIND_PROPERTY_SET && value.Params[0].Type == NODE_REST_ELEMENT {
		return nil, this.raiseRecoverable(value.Params[0].Start, "Setter cannot use rest params")
	}

	return this.finishNode(method, NODE_METHOD_DEFINITION), nil
}

func checkKeyName(node *Node, name string) bool {
	computed, key := node.Computed, node.Key
	return !computed && (key.Type == NODE_IDENTIFIER && key.Name == name || key.Type == NODE_LITERAL && key.Value == name)
}

func (this *Parser) parseClassElementName(element *Node) error {
	if this.Type.identifier == TOKEN_PRIVATEID {
		if val, ok := this.Value.(string); ok && val == "constructor" {
			return this.raise(this.start, "Classes can't have an element named '#constructor'")
		}
		element.Computed = false
		privateId, err := this.parsePrivateIdent()
		if err != nil {
			return err
		}
		element.Key = privateId
	} else {
		_, err := this.parsePropertyName(element)
		if err != nil {
			return err
		}
	}
	return nil
}

func (this *Parser) isClassElementNameStart() bool {
	t := this.Type.identifier
	return t == TOKEN_NAME || t == TOKEN_PRIVATEID || t == TOKEN_NUM || t == TOKEN_STRING || t == TOKEN_BRACKETL || len(this.Type.keyword) != 0
}

func (this *Parser) enterClassBody() (map[string]string, error) {
	element := &PrivateName{Declared: map[string]string{}, Used: []*Node{}}
	this.PrivateNameStack = append(this.PrivateNameStack, element)
	return element.Declared, nil
}

func (this *Parser) parseClassSuper(node *Node) error {

	if this.eat(TOKEN_EXTENDS) {
		expr, err := this.parseExprSubscripts(nil, "")
		if err != nil {
			return err
		}
		node.SuperClass = expr
	} else {
		node.SuperClass = nil
	}
	return nil
}

func (this *Parser) parseClassId(node *Node, isStatement bool) error {
	if this.Type.identifier == TOKEN_NAME {

		id, err := this.parseIdent(false)
		if err != nil {
			return err
		}
		node.Id = id
		if isStatement {
			err := this.checkLValSimple(node.Id, BIND_LEXICAL, struct {
				check bool
				hash  map[string]bool
			}{check: false})
			if err != nil {
				return err
			}
		} else {
			if isStatement {
				return this.unexpected("cant be in a statement", nil)
			}

			node.Id = nil
		}
		return nil
	}
	return nil
}
