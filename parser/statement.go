package parser

import (
	"errors"
	"slices"
	"strings"
)

func (p *Parser) parseTopLevel(node *Node) (*Node, error) {
	exports := map[string]*Node{}
	node.Body = []*Node{}
	for p.Type.identifier != TOKEN_EOF {
		stmt, err := p.parseStatement("", true, exports)
		if err != nil {
			return nil, err
		}
		node.Body = append(node.Body, stmt)
	}

	if p.InModule {
		for k, _ := range p.UndefinedExports { // let's just aggregate all of the undefined exports since now it'll just return at first
			return nil, p.raiseRecoverable(p.UndefinedExports[k].Start, "Export "+k+" not defined")
		}
	}

	p.adaptDirectivePrologue(node.Body)
	return p.finishNode(node, NODE_PROGRAM), nil
}

func (p *Parser) parseStatement(context string, topLevel bool, exports map[string]*Node) (*Node, error) {
	//p.printState()
	startType, node := p.Type, p.startNode()
	kind := KIND_NOT_INITIALIZED

	if p.isLet(context) {
		startType = tokenTypes[TOKEN_VAR]
		kind = KIND_DECLARATION_LET
	}
	switch startType.identifier {
	case TOKEN_BREAK, TOKEN_CONTINUE:
		breakContinueStatement, err := p.parseBreakContinueStatement(node, startType.keyword)
		if err != nil {
			return nil, err
		}
		return breakContinueStatement, nil

	case TOKEN_DEBUGGER:
		debuggerStatement, err := p.parseDebuggerStatement(node)
		if err != nil {
			return nil, err
		}
		return debuggerStatement, nil

	case TOKEN_DO:
		doStatement, err := p.parseDoStatement(node)
		if err != nil {
			return nil, err
		}
		return doStatement, nil

	case TOKEN_FOR:
		forStatement, err := p.parseForStatement(node)
		if err != nil {
			return nil, err
		}
		return forStatement, nil

	case TOKEN_FUNCTION:
		if len(context) != 0 && (p.Strict || context != "if" && context != "label" && p.getEcmaVersion() >= 6) {
			return nil, p.unexpected("", nil)
		}
		functionStatement, err := p.parseFunctionStatement(node, false, len(context) == 0)

		if err != nil {
			return nil, err
		}

		return functionStatement, nil

	case TOKEN_CLASS:
		if len(context) != 0 {
			return nil, p.unexpected("Cant parse class in context", nil)
		}
		classStatement, err := p.parseClass(node, true)

		if err != nil {
			return nil, err
		}
		return classStatement, nil

	case TOKEN_IF:
		ifStatement, err := p.parseIfStatement(node)
		if err != nil {
			return nil, err
		}
		return ifStatement, nil

	case TOKEN_RETURN:
		returnStatement, err := p.parseReturnStatement(node)
		if err != nil {
			return nil, err
		}
		return returnStatement, nil

	case TOKEN_SWITCH:
		switchStatement, err := p.parseSwitchStatement(node)
		if err != nil {
			return nil, err
		}
		return switchStatement, nil

	case TOKEN_THROW:
		throwStatement, err := p.parseThrowStatement(node)
		if err != nil {
			return nil, err
		}
		return throwStatement, nil

	case TOKEN_TRY:
		tryStatement, err := p.parseTryStatement(node)
		if err != nil {
			return nil, err
		}
		return tryStatement, nil

	case TOKEN_CONST, TOKEN_VAR:

		if kind == KIND_NOT_INITIALIZED {
			if str, ok := p.Value.(string); ok {
				switch str {
				case "const":
					{
						kind = KIND_DECLARATION_CONST
					}
				case "var":
					{
						kind = KIND_DECLARATION_VAR
					}
				case "let":
					{
						kind = KIND_DECLARATION_LET
					}
				}
			} else {
				panic("We were expectin a Kind from node.Value, didn't happen so we are now here.")
			}
		}

		if len(context) != 0 && kind != KIND_DECLARATION_VAR {
			return nil, p.unexpected("Expected empty context, and not KIND_DECLARATION_VAR", nil)
		}

		varStatement, err := p.parseVarStatement(node, kind, false)

		if err != nil {
			return nil, err
		}

		return varStatement, nil

	case TOKEN_WHILE:
		whileStatement, err := p.parseWhileStatement(node)

		if err != nil {
			return nil, err
		}

		return whileStatement, err

	case TOKEN_WITH:
		withStatement, err := p.parseWithStatement(node)

		if err != nil {
			return nil, err
		}

		return withStatement, err

	case TOKEN_BRACEL:
		block, err := p.parseBlock(true, node, false)

		if err != nil {
			return nil, err
		}

		return block, nil

	case TOKEN_SEMI:
		emptyStatement, err := p.parseEmptyStatement(node)

		if err != nil {
			return nil, err
		}
		return emptyStatement, nil

	case TOKEN_EXPORT, TOKEN_IMPORT:
		if p.getEcmaVersion() > 10 && startType.identifier == TOKEN_IMPORT {
			skip := skipWhiteSpace.Find((p.input[p.pos:]))

			next := p.pos + len(skip)
			nextCh := p.input[next]

			if nextCh == '(' || nextCh == '.' {
				expression, err := p.parseExpression("", nil)

				if err != nil {
					return nil, err
				}

				expressionStatement, err := p.parseExpressionStatement(node, expression)

				if err != nil {
					return nil, err
				}

				return expressionStatement, nil
			}
		}

		if !p.options.AllowImportExportEverywhere {
			if !topLevel {
				return nil, p.raise(p.start, "'import' and 'export' may only appear at the top level")
			}

			if !p.InModule {
				return nil, p.raise(p.start, "'import' and 'export' may appear only with 'sourceType: module'")
			}
		}

		if startType.identifier == TOKEN_IMPORT {
			importStatement, err := p.parseImport(node)
			if err != nil {
				return nil, err
			}
			return importStatement, nil
		}

		if startType.identifier == TOKEN_EXPORT {
			exportStatement, err := p.parseExport(node, exports)
			if err != nil {
				return nil, err
			}
			return exportStatement, nil
		}

	default:
		if p.isAsyncFunction() {
			if len(context) != 0 {
				return nil, p.unexpected("Expected empty context", nil)
			}
			p.next(false)

			functionStatement, err := p.parseFunctionStatement(node, true, len(context) == 0)
			if err != nil {
				return nil, err
			}

			return functionStatement, nil
		}

		maybeName := p.Value
		expr, err := p.parseExpression("", nil)

		if err != nil {
			return nil, err
		}

		if startType.identifier == TOKEN_NAME && expr.Type == NODE_IDENTIFIER && p.eat(TOKEN_COLON) {

			if name, ok := maybeName.(string); ok {
				labeledStatement, err := p.parseLabeledStatement(node, name, expr, context)
				if err != nil {
					return nil, err
				}
				return labeledStatement, nil
			} else {
				panic("We expected node.Value to be a string, it wasn't so we ended up here")
			}

		}
		expressionStatement, err := p.parseExpressionStatement(node, expr)

		if err != nil {
			return nil, err
		}

		return expressionStatement, nil
	}

	return nil, errors.New("unreachable... or was it?")
}

func (p *Parser) parseLabeledStatement(node *Node, maybeName string, expr *Node, context string) (*Node, error) {
	for _, label := range p.Labels {
		if label.Name == maybeName {
			return nil, p.raise(expr.Start, "Label '"+maybeName+"' is already declared")
		}
	}

	kind := ""
	if p.Type.isLoop {
		kind = "loop"
	} else {
		if p.Type.identifier == TOKEN_SWITCH {
			kind = "switch"
		}
	}
	for i := len(p.Labels) - 1; i >= 0; i-- {
		if p.Labels[i].StatementStart == node.Start {
			p.Labels[i].StatementStart = p.start
			p.Labels[i].Kind = kind
		} else {
			break
		}
	}

	p.Labels = append(p.Labels, Label{Name: maybeName, Kind: kind, StatementStart: p.start})

	if len(context) != 0 && !strings.Contains(context, "label") {
		context += "label"
	}

	statement, err := p.parseStatement(context, false, map[string]*Node{})
	if err != nil {
		return nil, err
	}

	node.BodyNode = statement
	p.Labels = p.Labels[:len(p.Labels)-1]
	node.Label = expr
	return p.finishNode(node, NODE_LABELED_STATEMENT), nil
}

func (p *Parser) isAsyncFunction() bool {
	if p.getEcmaVersion() < 8 || !p.isContextual("async") {
		return false
	}

	skip := skipWhiteSpace.Find(p.input[p.pos:])
	next := p.pos + len(skip)
	after := rune(-1)
	if next+8 < len(p.input) {
		after = rune(p.input[next+8])
	}

	return !lineBreak.Match(p.input[p.pos:next]) &&
		string(p.input[next:next+8]) == "function" &&
		(next+8 == len(p.input) ||
			!(IsIdentifierChar(after, false) /*|| after > 0xd7ff && after < 0xdc00*/))
}

func (p *Parser) parseExport(node *Node, exports map[string]*Node) (*Node, error) {
	p.next(false)

	if p.eat(TOKEN_STAR) {
		exportAllDeclaration, err := p.parseExportAllDeclaration(node, exports)
		if err != nil {
			return nil, err
		}
		return exportAllDeclaration, nil
	}

	if p.eat(TOKEN_DEFAULT) {
		err := p.checkExport(exports, struct {
			s string
			n *Node
		}{s: "default"}, p.LastTokStart)
		if err != nil {
			return nil, err
		}

		return p.finishNode(node, NODE_EXPORT_DEFAULT_DECLARATION), nil
	}

	if p.shouldParseExportStatement() {
		decl, err := p.parseExportDeclaration(node)

		if err != nil {
			return nil, err
		}

		node.Declaration = decl

		if node.Declaration.Type == NODE_VARIABLE_DECLARATION {
			err := p.checkVariableExport(exports, node.Declaration.Declarations)
			if err != nil {
				return nil, err
			}
		} else {
			err := p.checkExport(exports, struct {
				s string
				n *Node
			}{n: node.Declaration.Identifier}, node.Declaration.Identifier.Start)
			if err != nil {
				return nil, err
			}
		}

		node.Specifiers = []*Node{}
		node.Source = nil

		if p.getEcmaVersion() >= 16 {
			node.Attributes = []*Node{}
		}
	} else {
		node.Declaration = nil
		specifiers, err := p.parseExportSpecifiers(exports)

		if err != nil {
			return nil, err
		}

		node.Specifiers = specifiers

		if p.eatContextual("from") {
			if p.Type.identifier == TOKEN_STRING {
				return nil, p.unexpected("Can't have STRING as current type", nil)
			}
			exprAtom, err := p.parseExprAtom(nil, "", false)

			if err != nil {
				return nil, err
			}

			node.Source = exprAtom
			if p.getEcmaVersion() >= 16 {
				withClause, err := p.parseWithClause()

				if err != nil {
					return nil, err
				}
				node.Attributes = withClause
			}
		} else {

			for _, spec := range node.Specifiers {
				err := p.checkUnreserved(struct {
					start int
					end   int
					name  string
				}{start: spec.Local.Start, end: spec.Local.End, name: spec.Local.Name})

				if err != nil {
					return nil, err
				}

				p.checkLocalExport(spec.Local)

				if spec.Local.Type == NODE_LITERAL {
					return nil, p.raise(spec.Local.Start, "A string literal cannot be used as an exported binding without `from`.")
				}
			}

			node.Source = nil
			if p.getEcmaVersion() >= 16 {
				node.Attributes = []*Node{}
			}
		}
		err = p.semicolon()
		if err != nil {
			return nil, err
		}
	}
	return p.finishNode(node, NODE_EXPORT_NAMED_DECLARATION), nil
}

func (p *Parser) checkLocalExport(opts *Node) {
	if slices.Index(p.ScopeStack[0].Lexical, opts.Name) == -1 &&
		slices.Index(p.ScopeStack[0].Var, opts.Name) == -1 {
		p.UndefinedExports[opts.Name] = opts
	}
}

func (p *Parser) parseExportSpecifiers(exports map[string]*Node) ([]*Node, error) {
	nodes, first := []*Node{}, true
	// export { x, y as z } [from '...']
	err := p.expect(TOKEN_BRACEL)

	if err != nil {
		return nil, err
	}
	for !p.eat(TOKEN_BRACER) {
		if !first {
			err := p.expect(TOKEN_COMMA)
			if err != nil {
				return nil, err
			}
			if p.afterTrailingComma(TOKEN_BRACER, false) {
				break
			}
		} else {
			first = false
		}
		exportSpecifier, err := p.parseExportSpecifier(exports)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, exportSpecifier)

	}
	return nodes, nil
}

func (p *Parser) parseExportSpecifier(exports map[string]*Node) (*Node, error) {
	node := p.startNode()
	moduleExportName, err := p.parseModuleExportName()
	if err != nil {
		return nil, err
	}
	node.Local = moduleExportName

	if p.eatContextual("as") {
		moduleExportName, err := p.parseModuleExportName()
		if err != nil {
			return nil, err
		}

		node.Exported = moduleExportName
	} else {
		node.Exported = node.Local
	}
	p.checkExport(
		exports,
		struct {
			s string
			n *Node
		}{
			n: node.Exported,
		},
		node.Exported.Start,
	)

	return p.finishNode(node, NODE_EXPORT_SPECIFIER), nil
}

func (p *Parser) parseModuleExportName() (*Node, error) {
	if p.getEcmaVersion() >= 13 && p.Type.identifier == TOKEN_STRING {
		stringLiteral, err := p.parseLiteral(p.Value)

		if err != nil {
			return nil, err
		}

		if val, ok := stringLiteral.Value.(string); ok {
			if loneSurrogate.Match([]byte(val)) {
				return nil, p.raise(stringLiteral.Start, "An export name cannot include a lone surrogate.")
			}
			return stringLiteral, nil
		}
	}
	ident, err := p.parseIdent(true)
	if err != nil {
		return nil, err
	}
	return ident, nil
}

func (p *Parser) parseWithClause() ([]*Node, error) {
	nodes := []*Node{}
	if !p.eat(TOKEN_WITH) {
		return nodes, nil
	}

	err := p.expect(TOKEN_BRACEL)
	if err != nil {
		return nil, err
	}
	attributeKeys := map[string]struct{}{}
	first := true
	for !p.eat(TOKEN_BRACER) {
		if !first {
			err := p.expect(TOKEN_COMMA)
			if err != nil {
				return nil, err
			}
			if p.afterTrailingComma(TOKEN_BRACER, false) {
				break
			}
		} else {
			first = false
		}

		attr, err := p.parseImportAttribute()
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
			return nil, p.raiseRecoverable(attr.Key.Start, "Duplicate attribute key '"+keyName+"'")
		}
		attributeKeys[keyName] = struct{}{}
		nodes = append(nodes, attr)
	}
	return nodes, nil
}

func (p *Parser) parseImportAttribute() (*Node, error) {
	node := p.startNode()

	if p.Type.identifier == TOKEN_STRING {
		exprAtom, err := p.parseExprAtom(nil, "", false)

		if err != nil {
			return nil, err
		}
		node.Key = exprAtom
	} else {
		ident, err := p.parseIdent(p.options.AllowReserved != ALLOW_RESERVED_NEVER)

		if err != nil {
			return nil, err
		}
		node.Key = ident
	}
	err := p.expect(TOKEN_COLON)
	if err != nil {
		return nil, err
	}

	if p.Type.identifier != TOKEN_STRING {
		return nil, p.unexpected("Expected TOKEN_STRING", nil)
	}

	exprAtom, err := p.parseExprAtom(nil, "", false)

	if err != nil {
		return nil, err
	}

	node.Value = exprAtom
	return p.finishNode(node, NODE_IMPORT_ATTRIBUTE), nil
}

func (p *Parser) checkVariableExport(exports map[string]*Node, declarations []*Node) error {
	if exports == nil {
		return errors.New("exports was defined, i guess we don't want that?")
	}

	for _, decl := range declarations {
		err := p.checkPatternExport(exports, decl.Identifier)

		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) checkPatternExport(exports map[string]*Node, pat *Node) error {
	t := pat.Type

	switch t {
	case NODE_IDENTIFIER:
		p.checkExport(exports, struct {
			s string
			n *Node
		}{n: pat}, pat.Start)
	case NODE_OBJECT_PATTERN:
		for _, prop := range pat.Properties {
			err := p.checkPatternExport(exports, prop)
			if err != nil {
				return err
			}
		}
	case NODE_ARRAY_PATTERN:
		for _, elt := range pat.Elements {
			if elt != nil {
				err := p.checkPatternExport(exports, elt)
				if err != nil {
					return err
				}
			}
		}
	case NODE_PROPERTY:
		if val, ok := pat.Value.(*Node); ok {
			err := p.checkPatternExport(exports, val)
			if err != nil {
				return err
			}
		} else {
			panic("we were expecting *Node from node.Value")
		}
	case NODE_ASSIGNMENT_PATTERN:

		err := p.checkPatternExport(exports, pat.Left)
		if err != nil {
			return err
		}
	case NODE_REST_ELEMENT:
		err := p.checkPatternExport(exports, pat.Argument)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) parseExportDeclaration(node *Node) (*Node, error) {
	stmt, err := p.parseStatement("", false, nil)
	if err != nil {
		return nil, err
	}
	return stmt, nil
}

func (p *Parser) shouldParseExportStatement() bool {
	return p.Type.keyword == "var" ||
		p.Type.keyword == "const" ||
		p.Type.keyword == "class" ||
		p.Type.keyword == "function" ||
		p.isLet("") ||
		p.isAsyncFunction()
}

func (p *Parser) checkExport(exports map[string]*Node, val struct {
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
		return p.raiseRecoverable(start, "Duplicate export '"+name+"'")
	}

	return nil
}

func (p *Parser) parseExportAllDeclaration(node *Node, exports map[string]*Node) (*Node, error) {
	if p.getEcmaVersion() >= 11 {
		if p.eatContextual("as") {
			moduleExportName, err := p.parseModuleExportName()
			if err != nil {
				return nil, err
			}

			node.Exported = moduleExportName

			p.checkExport(exports, struct {
				s string
				n *Node
			}{n: node.Exported}, p.LastTokStart)
		} else {
			node.Exported = nil
		}
	}
	err := p.expectContextual("from")

	if err != nil {
		return nil, err
	}

	if p.Type.identifier != TOKEN_STRING {
		return nil, p.unexpected("Expected TOKEN_STRING", nil)
	}
	exprAtom, err := p.parseExprAtom(nil, "", false)
	if err != nil {
		return nil, err
	}
	node.Source = exprAtom
	if p.getEcmaVersion() >= 16 {
		attr, err := p.parseWithClause()
		if err != nil {
			return nil, err
		}
		node.Attributes = attr
	}

	err = p.semicolon()
	if err != nil {
		return nil, err
	}
	return p.finishNode(node, NODE_EXPORT_ALL_DECLARATION), nil
}

func (p *Parser) expectContextual(name string) error {
	if !p.eatContextual(name) {
		return p.unexpected("Expected context from context stack", nil)
	}
	return nil
}

func (p *Parser) parseImport(node *Node) (*Node, error) {
	p.next(false)
	// import '...'
	if p.Type.identifier == TOKEN_STRING {
		node.Specifiers = []*Node{}
		exprAtom, err := p.parseExprAtom(nil, "", false)
		if err != nil {
			return nil, err
		}
		node.Source = exprAtom
	} else {
		importSpecifiers, err := p.parseImportSpecifiers()
		if err != nil {
			return nil, err
		}

		node.Specifiers = importSpecifiers
		err = p.expectContextual("from")
		if err != nil {
			return nil, err
		}
		if p.Type.identifier == TOKEN_STRING {
			exprAtom, err := p.parseExprAtom(nil, "", false)
			if err != nil {
				return nil, err
			}
			node.Source = exprAtom
		} else {
			return nil, p.unexpected("Expected TOKEN_STRING", nil)
		}
	}
	if p.getEcmaVersion() >= 16 {
		withClause, err := p.parseWithClause()
		if err != nil {
			return nil, err
		}
		node.Attributes = withClause
	}
	err := p.semicolon()
	if err != nil {
		return nil, err
	}
	return p.finishNode(node, NODE_IMPORT_DECLARATION), nil
}

func (p *Parser) parseImportSpecifiers() ([]*Node, error) {
	nodes, first := []*Node{}, true
	if p.Type.identifier == TOKEN_NAME {
		importDefaultSpecifier, err := p.parseImportDefaultSpecifier()
		if err != nil {
			return nil, err
		}

		nodes = append(nodes, importDefaultSpecifier)
		if !p.eat(TOKEN_COMMA) {
			return nodes, nil
		}
	}
	if p.Type.identifier == TOKEN_STAR {
		importNamespaceSpecifier, err := p.parseImportNamespaceSpecifier()
		if err != nil {
			return nil, err
		}

		nodes = append(nodes, importNamespaceSpecifier)
		return nodes, nil
	}
	err := p.expect(TOKEN_BRACEL)
	if err != nil {
		return nil, err
	}

	for !p.eat(TOKEN_BRACER) {
		if !first {
			err := p.expect(TOKEN_COMMA)
			if err != nil {
				return nil, err
			}
			if p.afterTrailingComma(TOKEN_BRACER, false) {
				break
			}
		} else {
			first = false
		}
		importSpecifier, err := p.parseImportSpecifier()
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, importSpecifier)
	}
	return nodes, nil
}

func (p *Parser) parseImportSpecifier() (*Node, error) {
	node := p.startNode()
	moduleExportName, err := p.parseModuleExportName()
	if err != nil {
		return nil, err
	}
	node.Imported = moduleExportName
	if p.eatContextual("as") {
		ident, err := p.parseIdent(false)
		if err != nil {
			return nil, err
		}
		node.Local = ident
	} else {
		err := p.checkUnreserved(struct {
			start int
			end   int
			name  string
		}{start: node.Imported.Start, end: node.Imported.End, name: node.Imported.Name})
		if err != nil {
			return nil, err
		}

		node.Local = node.Imported
	}
	err = p.checkLValSimple(node.Local, BIND_LEXICAL, struct {
		check bool
		hash  map[string]bool
	}{check: false})

	if err != nil {
		return nil, err
	}

	return p.finishNode(node, NODE_IMPORT_SPECIFIER), nil
}

func (p *Parser) parseImportNamespaceSpecifier() (*Node, error) {
	node := p.startNode()
	p.next(false)
	err := p.expectContextual("as")

	if err != nil {
		return nil, err
	}

	ident, err := p.parseIdent(false)

	if err != nil {
		return nil, err
	}
	node.Local = ident
	err = p.checkLValSimple(node.Local, BIND_LEXICAL, struct {
		check bool
		hash  map[string]bool
	}{check: false})

	if err != nil {
		return nil, err
	}
	return p.finishNode(node, NODE_IMPORT_NAMESPACE_SPECIFIER), nil
}

func (p *Parser) parseImportDefaultSpecifier() (*Node, error) {
	// import defaultObj, { x, y as z } from '...'
	node := p.startNode()
	ident, err := p.parseIdent(false)
	if err != nil {
		return nil, err
	}
	node.Local = ident
	err = p.checkLValSimple(node.Local, BIND_LEXICAL, struct {
		check bool
		hash  map[string]bool
	}{check: false})

	if err != nil {
		return nil, err
	}

	return p.finishNode(node, NODE_IMPORT_DEFAULT_SPECIFIER), nil
}

func (p *Parser) parseExpressionStatement(node *Node, expr *Node) (*Node, error) {
	node.Expression = expr
	err := p.semicolon()
	if err != nil {
		return nil, err
	}
	return p.finishNode(node, NODE_EXPRESSION_STATEMENT), nil
}

func (p *Parser) parseEmptyStatement(node *Node) (*Node, error) {
	p.next(false)
	return p.finishNode(node, NODE_EMPTY_STATEMENT), nil
}

func (p *Parser) parseWithStatement(node *Node) (*Node, error) {
	if p.Strict {
		p.raise(p.start, "'with' in strict mode")
	}
	p.next(false)
	parenthesizedExpr, err := p.parseParenExpression()
	if err != nil {
		return nil, err
	}
	node.Object = parenthesizedExpr
	stmt, err := p.parseStatement("with", false, nil)
	node.BodyNode = stmt
	return p.finishNode(node, NODE_WITH_STATEMENT), nil
}

func (p *Parser) parseWhileStatement(node *Node) (*Node, error) {
	p.next(false)
	parenthesizedExpression, err := p.parseParenExpression()
	if err != nil {
		return nil, err
	}
	node.Test = parenthesizedExpression
	p.Labels = append(p.Labels, Label{Kind: "loop"})

	stmt, err := p.parseStatement("while", false, nil)
	if err != nil {
		return nil, err
	}

	node.BodyNode = stmt
	p.Labels = p.Labels[:len(p.Labels)-1]

	return p.finishNode(node, NODE_WHILE_STATEMENT), nil
}

func (p *Parser) parseVarStatement(node *Node, kind Kind, allowMissingInitializer bool) (*Node, error) {
	p.next(false)
	_, err := p.parseVar(node, false, kind, allowMissingInitializer)
	if err != nil {
		return nil, err
	}
	err = p.semicolon()
	if err != nil {
		return nil, err
	}
	return p.finishNode(node, NODE_VARIABLE_DECLARATION), nil
}

func (p *Parser) parseTryStatement(node *Node) (*Node, error) {
	p.next(false)

	blok, err := p.parseBlock(false, nil, false)
	if err != nil {
		return nil, err
	}
	node.Block = blok
	node.Handler = nil
	if p.Type.identifier == TOKEN_CATCH {
		clause := p.startNode()
		p.next(false)
		if p.eat(TOKEN_PARENL) {
			catchClauseParam, err := p.parseCatchClauseParam()
			if err != nil {
				return nil, err
			}
			clause.Param = catchClauseParam
		} else {
			if p.getEcmaVersion() < 10 {
				return nil, p.unexpected("Wrong ecma version", nil)
			}
			clause.Param = nil
			p.enterScope(0)
		}
		block, err := p.parseBlock(false, nil, false)
		if err != nil {
			return nil, err
		}

		clause.BodyNode = block
		p.exitScope()
		node.Handler = p.finishNode(clause, NODE_CATCH_CLAUSE)
	}
	if p.eat(TOKEN_FINALLY) {
		block, err := p.parseBlock(false, nil, false)
		if err != nil {
			return nil, err
		}
		node.Finalizer = block
	} else {
		node.Finalizer = nil
	}

	if node.Handler == nil && node.Finalizer == nil {
		return nil, p.raise(node.Start, "Missing catch or finally clause")
	}

	return p.finishNode(node, NODE_TRY_STATEMENT), nil
}

func (p *Parser) parseCatchClauseParam() (*Node, error) {
	param, err := p.parseBindingAtom()
	if err != nil {
		return nil, err
	}

	simple := param.Type == NODE_IDENTIFIER
	if simple {
		p.enterScope(SCOPE_SIMPLE_CATCH)
	} else {
		p.enterScope(0)
	}

	if simple {
		err := p.checkLValPattern(param, BIND_SIMPLE_CATCH, struct {
			check bool
			hash  map[string]bool
		}{check: false})
		if err != nil {
			return nil, err
		}
	} else {
		err := p.checkLValPattern(param, BIND_LEXICAL, struct {
			check bool
			hash  map[string]bool
		}{check: false})
		if err != nil {
			return nil, err
		}
	}
	err = p.expect(TOKEN_PARENR)

	if err != nil {
		return nil, err
	}

	return param, nil
}

func (p *Parser) parseThrowStatement(node *Node) (*Node, error) {
	p.next(false)
	if lineBreak.Match(p.input[p.LastTokEnd:p.start]) {
		return nil, p.raise(p.LastTokEnd, "Illegal newline after throw")
	}
	expr, err := p.parseExpression("", nil)
	if err != nil {
		return nil, err
	}
	node.Argument = expr
	err = p.semicolon()
	if err != nil {
		return nil, err
	}
	return p.finishNode(node, NODE_THROW_STATEMENT), nil
}

func (p *Parser) parseSwitchStatement(node *Node) (*Node, error) {
	p.next(false)

	expr, err := p.parseParenExpression()
	if err != nil {
		return nil, err
	}
	node.Discriminant = expr
	node.Cases = []*Node{}
	err = p.expect(TOKEN_BRACEL)
	if err != nil {
		return nil, err
	}
	p.Labels = append(p.Labels, Label{Kind: "switch"})
	p.enterScope(0)

	// Statements under must be grouped (by label) in SwitchCase
	// nodes. `cur` is used to keep the node that we are currently
	// adding statements to.

	var cur *Node
	sawDefault := false
	for p.Type.identifier != TOKEN_BRACER {
		if p.Type.identifier == TOKEN_CASE || p.Type.identifier == TOKEN_DEFAULT {
			isCase := p.Type.identifier == TOKEN_CASE
			if cur != nil {
				p.finishNode(cur, NODE_SWITCH_CASE)
			}
			cur = p.startNode()
			node.Cases = append(node.Cases, cur)

			cur.ConsequentSlice = []*Node{}
			p.next(false)
			if isCase {
				test, err := p.parseExpression("", nil)
				if err != nil {
					return nil, err
				}
				cur.Test = test

			} else {
				if sawDefault {
					return nil, p.raiseRecoverable(p.LastTokStart, "Multiple default clauses")
				}
				sawDefault = true
				cur.Test = nil
			}
			err = p.expect(TOKEN_COLON)
			if err != nil {
				return nil, err
			}
		} else {
			if cur == nil {
				return nil, p.unexpected("cur cant be nil", nil)
			}
			stmt, err := p.parseStatement("", false, nil)
			if err != nil {
				return nil, err
			}

			cur.ConsequentSlice = append(cur.ConsequentSlice, stmt)
		}
		sawDefault = false
	}
	p.exitScope()
	if cur != nil {
		p.finishNode(cur, NODE_SWITCH_CASE)
	}
	p.next(false)
	p.Labels = p.Labels[:len(p.Labels)-1]
	return p.finishNode(node, NODE_SWITCH_STATEMENT), nil
}

func (p *Parser) parseReturnStatement(node *Node) (*Node, error) {
	if !p.inFunction() && !p.options.AllowReturnOutsideFunction {
		return nil, p.raise(p.start, "'return' outside of function")
	}

	p.next(false)

	// In `return` (and `break`/`continue`), the keywords with
	// optional arguments, we eagerly look for a semicolon or the
	// possibility to insert one.

	if p.eat(TOKEN_SEMI) || p.insertSemicolon() {
		node.Argument = nil
	} else {
		expr, err := p.parseExpression("", nil)
		if err != nil {
			return nil, err
		}
		node.Argument = expr
		err = p.semicolon()
		if err != nil {
			return nil, err
		}
	}
	return p.finishNode(node, NODE_RETURN_STATEMENT), nil
}

func (p *Parser) isLet(context string) bool {
	if p.getEcmaVersion() < 6 || !p.isContextual("let") {
		return false
	}

	skip := skipWhiteSpace.Find(p.input[p.pos:])
	next := p.pos + len(skip)

	nextCh := rune(p.input[next])
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
		nextCh = rune(p.input[pos])
		for IsIdentifierChar(nextCh, true) {
			pos = pos + 1
			nextCh = rune(p.input[pos])
		}
		if nextCh == 92 /*|| nextCh > 0xd7ff && nextCh < 0xdc00*/ {
			return true

		}
		ident := p.input[next:pos]
		if !keywordRelationalOperator.Match(ident) {
			return true
		}
	}
	return false
}

func (p *Parser) parseIfStatement(node *Node) (*Node, error) {
	p.next(false)
	test, err := p.parseParenExpression()
	if err != nil {
		return nil, err
	}

	node.Test = test
	// allow function declarations in branches, but only in non-strict mode
	statement, err := p.parseStatement("if", false, nil)
	if err != nil {
		return nil, err
	}
	node.Consequent = statement

	if p.eat(TOKEN_ELSE) {
		alternate, err := p.parseStatement("if", false, nil)
		if err != nil {
			return nil, err
		}
		node.Alternate = alternate
	}

	return p.finishNode(node, NODE_IF_STATEMENT), nil
}

const (
	FUNC_STATEMENT         = 1
	FUNC_HANGING_STATEMENT = 2
	FUNC_NULLABLE_ID       = 4
)

func (p *Parser) parseFunctionStatement(node *Node, isAsync bool, declarationPosition bool) (*Node, error) {
	p.next(false)

	if declarationPosition {
		function, err := p.parseFunction(node, FUNC_STATEMENT, false, isAsync, "")
		if err != nil {
			return nil, err
		}
		return function, nil
	} else {
		function, err := p.parseFunction(node, FUNC_STATEMENT|FUNC_HANGING_STATEMENT, false, isAsync, "")
		if err != nil {
			return nil, err
		}
		return function, nil
	}
}
func (p *Parser) parseForStatement(node *Node) (*Node, error) {
	p.next(false)
	awaitAt := -1

	if p.getEcmaVersion() >= 9 && p.canAwait() && p.eatContextual("await") {
		awaitAt = p.LastTokStart
	}

	p.Labels = append(p.Labels, Label{Kind: "loop", Name: ""})

	p.enterScope(0)
	err := p.expect(TOKEN_PARENL)

	if err != nil {
		return nil, err
	}

	if p.Type.identifier == TOKEN_SEMI {
		if awaitAt > -1 {
			return nil, p.unexpected("Something about having an await clause and semicolon", &awaitAt)
		}
		forStatement, err := p.parseFor(node, nil)
		if err != nil {
			return nil, err
		}
		return forStatement, nil
	}

	isLet := p.isLet("")
	if p.Type.identifier == TOKEN_VAR || p.Type.identifier == TOKEN_CONST || isLet {
		init := p.startNode()

		kind := KIND_NOT_INITIALIZED
		if isLet {
			kind = KIND_DECLARATION_LET
		} else {
			if str, ok := p.Value.(string); ok {
				if str == "var" {
					kind = KIND_DECLARATION_VAR
				} else if str == "const" {
					kind = KIND_DECLARATION_CONST
				}
			} else {
				panic("parser.Value was not declarationKind as we expected")
			}
		}
		p.next(false)
		_, err := p.parseVar(init, true, kind, false)

		if err != nil {
			return nil, err
		}

		p.finishNode(init, NODE_VARIABLE_DECLARATION)

		if (p.Type.identifier == TOKEN_IN || (p.getEcmaVersion() >= 6 && p.isContextual("of"))) && len(init.Declarations) == 1 {
			if p.getEcmaVersion() >= 9 {
				if p.Type.identifier == TOKEN_IN {
					if awaitAt > -1 {
						return nil, p.unexpected("", &awaitAt)
					}
				} else {
					node.Await = awaitAt > -1
				}
			}
			forIn, err := p.parseForIn(node, init)

			if err != nil {
				return nil, err
			}
			return forIn, nil
		}
		if awaitAt > -1 {
			return nil, p.unexpected("", &awaitAt)
		}

		forStatement, err := p.parseFor(node, init)
		if err != nil {
			return nil, err
		}
		return forStatement, nil
	}

	startsWithLet, isForOf, containsEsc, refDestructuringErrors, initPos := p.isContextual("let"), false, p.ContainsEsc, NewDestructuringErrors(), p.start
	var init *Node

	if awaitAt > -1 {
		exprSubscripts, err := p.parseExprSubscripts(refDestructuringErrors, "await")
		if err != nil {
			return nil, err
		}

		init = exprSubscripts
	} else {
		expr, err := p.parseExpression("true", refDestructuringErrors)
		if err != nil {
			return nil, err
		}

		init = expr
	}

	isForOf = p.getEcmaVersion() >= 6
	if p.Type.identifier == TOKEN_IN || (isForOf && p.isContextual("of")) {
		if awaitAt > -1 { // implies `ecmaVersion >= 9` (see declaration of awaitAt)
			if p.Type.identifier == TOKEN_IN {
				return nil, p.unexpected("", &awaitAt)
			}
			node.Await = true
		} else if isForOf && p.getEcmaVersion() >= 8 {
			if init.Start == initPos && !containsEsc && init.Type == NODE_IDENTIFIER && init.Name == "async" {
				return nil, p.unexpected("", nil)
			} else if p.getEcmaVersion() >= 9 {
				node.Await = false
			}
		}
		if startsWithLet && isForOf {
			return nil, p.raise(init.Start, "The left-hand side of a for-of loop may not start with 'let'.")
		}
		_, err := p.toAssignable(init, false, refDestructuringErrors)
		if err != nil {
			return nil, err
		}
		err = p.checkLValPattern(init, 0, struct {
			check bool
			hash  map[string]bool
		}{check: false, hash: map[string]bool{}})

		if err != nil {
			return nil, err
		}
		forIn, err := p.parseForIn(node, init)

		if err != nil {
			return nil, err
		}

		return forIn, nil
	} else {
		_, err := p.checkExpressionErrors(refDestructuringErrors, true)

		if err != nil {
			return nil, err
		}
	}
	if awaitAt > -1 {
		return nil, p.unexpected("", &awaitAt)
	}

	forStatement, err := p.parseFor(node, init)

	if err != nil {
		return nil, err
	}
	return forStatement, nil
}

func (p *Parser) parseFor(node *Node, init *Node) (*Node, error) {
	node.Initializer = init
	err := p.expect(TOKEN_SEMI)

	if err != nil {
		return nil, err
	}

	if p.Type.identifier == TOKEN_SEMI {
		node.Test = nil
	} else {
		expr, err := p.parseExpression("", nil)

		if err != nil {
			return nil, err
		}

		node.Test = expr
	}

	err = p.expect(TOKEN_SEMI)

	if err != nil {
		return nil, err
	}

	if p.Type.identifier == TOKEN_PARENR {
		node.Update = nil
	} else {
		expr, err := p.parseExpression("", nil)

		if err != nil {
			return nil, err
		}
		node.Update = expr
	}

	err = p.expect(TOKEN_PARENR)
	if err != nil {
		return nil, err
	}
	stmt, err := p.parseStatement("for", false, nil)

	if err != nil {
		return nil, err
	}

	node.BodyNode = stmt
	p.exitScope()
	p.Labels = p.Labels[:len(p.Labels)-1]
	return p.finishNode(node, NODE_FOR_STATEMENT), nil
}

func (p *Parser) parseForIn(node *Node, init *Node) (*Node, error) {
	isForIn := p.Type.identifier == TOKEN_IN
	p.next(false)

	if init.Type == NODE_VARIABLE_DECLARATION && init.Declarations[0].Initializer != nil && (!isForIn || p.getEcmaVersion() < 8 || p.Strict || init.Kind != KIND_DECLARATION_VAR || init.Declarations[0].Identifier.Type != NODE_IDENTIFIER) {
		return nil, p.raise(init.Start, `for-in or for-of loop variable declaration may not have an initializer`)
	}
	node.Left = init
	if isForIn {
		expr, err := p.parseExpression("", nil)
		if err != nil {
			return nil, err
		}
		node.Right = expr
	} else {
		maybeAssign, err := p.parseMaybeAssign("", nil, nil)
		if err != nil {
			return nil, err
		}
		node.Right = maybeAssign
	}

	err := p.expect(TOKEN_PARENR)

	if err != nil {
		return nil, err
	}
	stmt, err := p.parseStatement("for", false, nil)
	if err != nil {
		return nil, err
	}
	node.BodyNode = stmt
	p.exitScope()
	p.Labels = p.Labels[:len(p.Labels)-1]

	if isForIn {
		return p.finishNode(node, NODE_FOR_IN_STATEMENT), nil
	}
	return p.finishNode(node, NODE_FOR_OF_STATEMENT), nil

}

func (p *Parser) parseVar(node *Node, isFor bool, kind Kind, allowMissingInitializer bool) (*Node, error) {
	node.Declarations = []*Node{}
	node.Kind = kind
	for {
		decl := p.startNode()
		err := p.parseVarId(decl, kind)

		if err != nil {
			return nil, err
		}

		if p.eat(TOKEN_EQ) {
			forInit := ""
			if isFor {
				forInit = "isFor"
			}
			declInit, err := p.parseMaybeAssign(forInit, nil, nil)
			if err != nil {
				return nil, err
			}
			decl.Initializer = declInit
		} else if !allowMissingInitializer && kind == KIND_DECLARATION_CONST && !(p.Type.identifier == TOKEN_IN || (p.getEcmaVersion() >= 6 && p.isContextual("of"))) {
			return nil, p.unexpected("Missing initializer in for..of loop", nil)
		} else if !allowMissingInitializer && decl.Identifier.Type != NODE_IDENTIFIER && !(isFor && (p.Type.identifier == TOKEN_IN || p.isContextual("of"))) {
			return nil, p.raise(p.LastTokEnd, "Complex binding patterns require an initialization value")
		} else {
			decl.Initializer = nil
		}
		node.Declarations = append(node.Declarations, p.finishNode(decl, NODE_VARIABLE_DECLARATOR))
		if !p.eat(TOKEN_COMMA) {
			break
		}
	}
	return node, nil
}

func (p *Parser) parseVarId(decl *Node, kind Kind) error {
	declarationIdentifier, err := p.parseBindingAtom()
	if err != nil {
		return err
	}
	decl.Identifier = declarationIdentifier
	if kind == KIND_DECLARATION_VAR {
		err := p.checkLValPattern(decl.Identifier, BIND_VAR, struct {
			check bool
			hash  map[string]bool
		}{check: false})

		if err != nil {
			return err
		}
	} else {
		err := p.checkLValPattern(decl.Identifier, BIND_LEXICAL, struct {
			check bool
			hash  map[string]bool
		}{check: false})

		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) eatContextual(name string) bool {
	if !p.isContextual(name) {
		return false
	}
	p.next(false)
	return true
}

func (p *Parser) parseDoStatement(node *Node) (*Node, error) {
	p.next(false)
	p.Labels = append(p.Labels, Label{Kind: "loop", Name: ""})
	doStatement, err := p.parseStatement("do", false, map[string]*Node{})
	if err != nil {
		return nil, err
	}
	node.BodyNode = doStatement
	p.Labels = p.Labels[:len(p.Labels)-1]
	err = p.expect(TOKEN_WHILE)

	if err != nil {
		return nil, err
	}

	testParenExpression, err := p.parseParenExpression()

	if err != nil {
		return nil, err
	}

	node.Test = testParenExpression

	if p.getEcmaVersion() >= 6 {
		p.eat(TOKEN_SEMI)
	} else {
		err := p.semicolon()
		if err != nil {
			return nil, err
		}
	}

	return p.finishNode(node, NODE_DO_WHILE_STATEMENT), nil
}

func (p *Parser) parseDebuggerStatement(node *Node) (*Node, error) {
	p.next(false)
	err := p.semicolon()
	if err != nil {
		return nil, err
	}
	return p.finishNode(node, NODE_DEBUGGER_STATEMENT), nil
}

func (p *Parser) parseBreakContinueStatement(node *Node, keyword string) (*Node, error) {
	isBreak := keyword == "break"
	p.next(false)
	if p.eat(TOKEN_SEMI) || p.insertSemicolon() {
		node.Label = nil
	} else if p.Type.identifier != TOKEN_NAME {
		return nil, p.unexpected("", nil)
	} else {
		ident, err := p.parseIdent(false)

		if err != nil {
			return nil, err
		}
		node.Label = ident
		err = p.semicolon()
		if err != nil {
			return nil, err
		}
	}

	// Verify that there is an actual destination to break or
	// continue to.
	i := 0
	for i < len(p.Labels) {
		lab := p.Labels[i]
		if node.Label == nil || lab.Name == node.Label.Name {
			if len(lab.Kind) != 0 && isBreak || lab.Kind == "loop" {
				break
			}
			if node.Label != nil && isBreak {
				break
			}
		}
	}

	if i == len(p.Labels) {
		return nil, p.raise(node.Start, "Unsyntactic "+keyword)
	}

	if isBreak {
		return p.finishNode(node, NODE_BREAK_STATEMENT), nil
	}

	return p.finishNode(node, NODE_CONTINUE_STATEMENT), nil
}

func (p *Parser) parseBlock(createNewLexicalScope bool, node *Node, exitStrict bool) (*Node, error) {
	if node == nil {
		node = p.startNode()
	}
	node.Body = []*Node{}
	err := p.expect(TOKEN_BRACEL)
	if err != nil {
		return nil, err
	}
	if createNewLexicalScope {
		p.enterScope(0)
	}
	for p.Type.identifier != TOKEN_BRACER {
		stmt, err := p.parseStatement("", false, nil)
		if err != nil {
			return nil, err
		}
		node.Body = append(node.Body, stmt)
	}
	if exitStrict {
		p.Strict = false
	}
	p.next(false)

	if createNewLexicalScope {
		p.exitScope()
	}
	return p.finishNode(node, NODE_BLOCK_STATEMENT), nil
}

func (p *Parser) adaptDirectivePrologue(statements []*Node) {
	for i := 0; i < len(statements) && p.isDirectiveCandidate(statements[i]); {
		statements[i].Directive = statements[i].Expression.Raw[1 : len(statements[i].Expression.Raw)-2]
		i++
	}
}

func (p *Parser) isDirectiveCandidate(statement *Node) bool {
	literalAndString := false

	if statement.Expression != nil && statement.Expression.Type == NODE_LITERAL {
		_, ok := statement.Expression.Value.(string)
		literalAndString = ok
	}
	return p.getEcmaVersion() >= 5 && statement.Type == NODE_EXPRESSION_STATEMENT && literalAndString && /* Reject parenthesized strings.*/ (p.input[statement.Start] == '"' || p.input[statement.Start] == '\'')
}

func (p *Parser) parseFunction(node *Node, statement Flags, allowExpressionBody bool, isAsync bool, forInit string) (*Node, error) {
	p.initFunction(node)
	if p.getEcmaVersion() >= 9 || p.getEcmaVersion() >= 6 && !isAsync {
		if p.Type.identifier == TOKEN_STAR && (statement&FUNC_HANGING_STATEMENT == FUNC_HANGING_STATEMENT) {
			return nil, p.unexpected("Token was star and FUNC_HANGING_STATEMENT flag was set", nil)
		}

		node.IsGenerator = p.eat(TOKEN_STAR)
	}
	if p.getEcmaVersion() >= 8 {
		node.IsAsync = isAsync
	}

	if statement&FUNC_STATEMENT == FUNC_STATEMENT {

		if statement&FUNC_NULLABLE_ID == FUNC_NULLABLE_ID && p.Type.identifier != TOKEN_NAME {
			node.Identifier = nil
		} else {
			identifier, err := p.parseIdent(false)
			if err != nil {
				return nil, err
			}
			node.Identifier = identifier
		}
	}
	if node.Identifier != nil && !(statement&FUNC_HANGING_STATEMENT == FUNC_HANGING_STATEMENT) {
		// If it is a regular function declaration in sloppy mode, then it is
		// subject to Annex B semantics (BIND_FUNCTION). Otherwise, the binding
		// mode depends on properties of the current scope (see
		// treatFunctionsAsVar).

		if p.Strict || node.IsGenerator || node.IsAsync {
			if p.treatFunctionsAsVar() {
				err := p.checkLValSimple(node.Identifier, BIND_VAR, struct {
					check bool
					hash  map[string]bool
				}{check: false})
				if err != nil {
					return nil, err
				}
			} else {
				err := p.checkLValSimple(node.Identifier, BIND_LEXICAL, struct {
					check bool
					hash  map[string]bool
				}{check: false})
				if err != nil {
					return nil, err
				}
			}
		} else {
			err := p.checkLValSimple(node.Identifier, BIND_FUNCTION, struct {
				check bool
				hash  map[string]bool
			}{check: false})
			if err != nil {
				return nil, err
			}
		}
	}

	oldYieldPos, oldAwaitPos, oldAwaitIdentPos := p.YieldPos, p.AwaitPos, p.AwaitIdentPos
	p.YieldPos = 0
	p.AwaitPos = 0
	p.AwaitIdentPos = 0
	p.enterScope(functionFlags(node.IsAsync, node.IsGenerator))

	if statement&FUNC_STATEMENT != FUNC_STATEMENT {
		if p.Type.identifier == TOKEN_NAME {
			ident, err := p.parseIdent(false)
			if err != nil {
				return nil, err
			}
			node.Identifier = ident
		} else {
			node.Identifier = nil
		}
	}

	err := p.parseFunctionParams(node)
	if err != nil {
		return nil, err
	}
	err = p.parseFunctionBody(node, allowExpressionBody, false, forInit)

	if err != nil {
		return nil, err
	}

	p.YieldPos = oldYieldPos
	p.AwaitPos = oldAwaitPos
	p.AwaitIdentPos = oldAwaitIdentPos

	if statement&FUNC_STATEMENT == FUNC_STATEMENT {
		return p.finishNode(node, NODE_FUNCTION_DECLARATION), nil
	}
	return p.finishNode(node, NODE_FUNCTION_EXPRESSION), nil

}

func (p *Parser) parseFunctionParams(node *Node) error {
	err := p.expect(TOKEN_PARENL)
	if err != nil {
		return err
	}
	bindingList, err := p.parseBindingList(TOKEN_PARENR, false, p.getEcmaVersion() >= 8, false)
	if err != nil {
		return err
	}

	node.Params = bindingList
	err = p.checkYieldAwaitInDefaultParams()
	if err != nil {
		return err
	}
	return nil
}

func (p *Parser) parseClass(node *Node, isStatement bool) (*Node, error) {
	p.next(false)

	// ecma-262 14.6 Class Definitions
	// A class definition is always strict mode code.
	oldStrict := p.Strict
	p.Strict = true

	err := p.parseClassId(node, isStatement)
	if err != nil {
		return nil, err
	}
	err = p.parseClassSuper(node)
	if err != nil {
		return nil, err
	}
	privateNameMap, err := p.enterClassBody()
	if err != nil {
		return nil, err
	}
	classBody := p.startNode()
	hadConstructor := false
	classBody.Body = []*Node{}
	err = p.expect(TOKEN_BRACEL)
	if err != nil {
		return nil, err
	}
	for p.Type.identifier != TOKEN_BRACER {
		element, err := p.parseClassElement(node.SuperClass != nil)
		if err != nil {
			return nil, err
		}
		if element != nil {
			classBody.Body = append(classBody.Body, element)
			if element.Type == NODE_METHOD_DEFINITION && element.Kind == KIND_CONSTRUCTOR {
				if hadConstructor {
					return nil, p.raiseRecoverable(element.Start, "Duplicate constructor in the same class")
				}
				hadConstructor = true
			} else if element.Key != nil && element.Key.Type == NODE_PRIVATE_IDENTIFIER && isPrivateNameConflicted(privateNameMap, element) {
				return nil, p.raiseRecoverable(element.Key.Start, "Identifier #"+element.Key.Name+"has already been declared")
			}
		}
	}
	p.Strict = oldStrict
	p.next(false)
	node.BodyNode = p.finishNode(classBody, NODE_CLASS_BODY)
	err = p.exitClassBody()

	if err != nil {
		return nil, err
	}

	if isStatement {
		return p.finishNode(node, NODE_CLASS_DECLARATION), nil
	}
	return p.finishNode(node, NODE_CLASS_EXPRESSION), nil
}

func (p *Parser) exitClassBody() error {
	privateNameTop := p.PrivateNameStack[len(p.PrivateNameStack)-1]
	p.PrivateNameStack = p.PrivateNameStack[:len(p.PrivateNameStack)-1]

	if !p.options.CheckPrivateFields {
		return nil
	}
	stackLength := len(p.PrivateNameStack)

	var parent *PrivateName

	if stackLength != 0 {
		parent = p.PrivateNameStack[len(p.PrivateNameStack)-1]
	}

	for _, id := range privateNameTop.Used {
		if _, found := privateNameTop.Declared[id.Name]; !found {
			if parent != nil {
				parent.Used = append(parent.Used, id)
			} else {
				return p.raiseRecoverable(id.Start, "Private field #"+id.Name+" must be declared in an enclosing class")
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
			next = "s" + kindToString[element.Kind]
		} else {
			next = "i" + kindToString[element.Kind]
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

func (p *Parser) parseClassElement(constructorAllowsSuper bool) (*Node, error) {
	if p.eat(TOKEN_SEMI) {
		return nil, nil
	}

	ecmaVersion := p.getEcmaVersion()
	node := p.startNode()
	keyName, isGenerator, isAsync, kind, isStatic := "", false, false, KIND_PROPERTY_METHOD, false

	if p.eatContextual("static") {
		if ecmaVersion >= 13 && p.eat(TOKEN_BRACEL) {
			_, err := p.parseClassStaticBlock(node)
			if err != nil {
				return nil, err
			}
			return node, nil
		}

		if p.isClassElementNameStart() || p.Type.identifier == TOKEN_STAR {
			isStatic = true
		} else {
			keyName = "static"
		}
	}
	node.IsStatic = isStatic

	if len(keyName) == 0 && ecmaVersion >= 8 && p.eatContextual("async") {
		if (p.isClassElementNameStart() || p.Type.identifier == TOKEN_STAR) && !p.canInsertSemicolon() {
			isAsync = true
		} else {
			keyName = "async"
		}
	}

	if len(keyName) == 0 && (ecmaVersion >= 9 || !isAsync) && p.eat(TOKEN_STAR) {
		isGenerator = true
	}

	if len(keyName) == 0 && !isAsync && !isGenerator {
		lastValue := p.Value
		if p.eatContextual("get") || p.eatContextual("set") {
			if p.isClassElementNameStart() {
				if k, ok := lastValue.(Kind); ok {
					kind = k
				} else {
					panic("We were expecting p.Value to be kind, it wasn't")
				}
			} else {
				if str, ok := lastValue.(string); ok {
					keyName = str
				} else {
					panic("We were expecting p.Value to be string, it wasn't")
				}
			}
		}
	}

	// Parse element name
	if len(keyName) != 0 {
		// 'async', 'get', 'set', or 'static' were not a keyword contextually.
		// The last token is any of those. Make it the element name.
		node.Computed = false
		node.Key = p.startNodeAt(p.LastTokStart, p.LastTokStartLoc)
		node.Key.Name = keyName
		p.finishNode(node.Key, NODE_IDENTIFIER)
	} else {
		err := p.parseClassElementName(node)
		if err != nil {
			return nil, err
		}
	}
	// Parse element value
	if ecmaVersion < 13 || p.Type.identifier == TOKEN_PARENL || kind != KIND_PROPERTY_METHOD || isGenerator || isAsync {
		isConstructor := !node.IsStatic && checkKeyName(node, "constructor")
		allowsDirectSuper := isConstructor && constructorAllowsSuper
		// Couldn't move this check into the 'parseClassMethod' method for backward compatibility.
		if isConstructor && kind != KIND_PROPERTY_METHOD {
			return nil, p.raise(node.Key.Start, "Constructor can't have get/set modifier")
		}

		if isConstructor {
			node.Kind = KIND_CONSTRUCTOR
		} else {
			node.Kind = kind
		}
		_, err := p.parseClassMethod(node, isGenerator, isAsync, allowsDirectSuper)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := p.parseClassField(node)
		if err != nil {
			return nil, err
		}
	}

	return node, nil
}

func (p *Parser) parseClassStaticBlock(node *Node) (*Node, error) {
	node.Body = []*Node{}

	oldLabels := p.Labels
	p.Labels = []Label{}
	p.enterScope(SCOPE_CLASS_STATIC_BLOCK | SCOPE_SUPER)
	for p.Type.identifier != TOKEN_BRACER {
		stmt, err := p.parseStatement("", false, nil)
		if err != nil {
			return nil, err
		}
		node.Body = append(node.Body, stmt)
	}
	p.next(false)
	p.exitScope()
	p.Labels = oldLabels

	return p.finishNode(node, NODE_STATIC_BLOCK), nil
}

func (p *Parser) parseClassField(field *Node) (*Node, error) {
	if checkKeyName(field, "constructor") {
		return nil, p.raise(field.Key.Start, "Classes can't have a field named 'constructor'")
	} else if field.IsStatic && checkKeyName(field, "prototype") {
		return nil, p.raise(field.Key.Start, "Classes can't have a static field named 'prototype'")
	}

	if p.eat(TOKEN_EQ) {
		// To raise SyntaxError if 'arguments' exists in the initializer.
		p.enterScope(SCOPE_CLASS_FIELD_INIT | SCOPE_SUPER)
		maybeAssign, err := p.parseMaybeAssign("", nil, nil)
		if err != nil {
			return nil, err
		}
		field.Value = maybeAssign
		p.exitScope()
	} else {
		field.Value = nil
	}
	p.semicolon()

	return p.finishNode(field, NODE_PROPERTY_DEFINITION), nil
}

func (p *Parser) parseClassMethod(method *Node, isGenerator bool, isAsync bool, allowsDirectSuper bool) (*Node, error) {
	// Check key and flags
	key := method.Key
	if method.Kind == KIND_CONSTRUCTOR {
		if isGenerator {
			return nil, p.raise(key.Start, "Constructor can't be a generator")
		}
		if isAsync {
			return nil, p.raise(key.Start, "Constructor can't be an async method")
		}
	} else if method.IsStatic && checkKeyName(method, "prototype") {
		return nil, p.raise(key.Start, "Classes may not have a static property named prototype")
	}

	// Parse value
	value, err := p.parseMethod(isGenerator, isAsync, allowsDirectSuper)
	if err != nil {
		return nil, err
	}
	method.Value = value

	// Check value
	if method.Kind == KIND_PROPERTY_GET && len(value.Params) != 0 {
		return nil, p.raiseRecoverable(value.Start, "getter should have no params")
	}

	if method.Kind == KIND_PROPERTY_SET && len(value.Params) != 1 {
		return nil, p.raiseRecoverable(value.Start, "setter should have exactly one param")
	}

	if method.Kind == KIND_PROPERTY_SET && value.Params[0].Type == NODE_REST_ELEMENT {
		return nil, p.raiseRecoverable(value.Params[0].Start, "Setter cannot use rest params")
	}

	return p.finishNode(method, NODE_METHOD_DEFINITION), nil
}

func checkKeyName(node *Node, name string) bool {
	computed, key := node.Computed, node.Key
	return !computed && (key.Type == NODE_IDENTIFIER && key.Name == name || key.Type == NODE_LITERAL && key.Value == name)
}

func (p *Parser) parseClassElementName(element *Node) error {
	if p.Type.identifier == TOKEN_PRIVATEID {
		if val, ok := p.Value.(string); ok && val == "constructor" {
			return p.raise(p.start, "Classes can't have an element named '#constructor'")
		}
		element.Computed = false
		privateId, err := p.parsePrivateIdent()
		if err != nil {
			return err
		}
		element.Key = privateId
	} else {
		_, err := p.parsePropertyName(element)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) isClassElementNameStart() bool {
	t := p.Type.identifier
	return t == TOKEN_NAME || t == TOKEN_PRIVATEID || t == TOKEN_NUM || t == TOKEN_STRING || t == TOKEN_BRACKETL || len(p.Type.keyword) != 0
}

func (p *Parser) enterClassBody() (map[string]string, error) {
	element := &PrivateName{Declared: map[string]string{}, Used: []*Node{}}
	p.PrivateNameStack = append(p.PrivateNameStack, element)
	return element.Declared, nil
}

func (p *Parser) parseClassSuper(node *Node) error {

	if p.eat(TOKEN_EXTENDS) {
		expr, err := p.parseExprSubscripts(nil, "")
		if err != nil {
			return err
		}
		node.SuperClass = expr
	} else {
		node.SuperClass = nil
	}
	return nil
}

func (p *Parser) parseClassId(node *Node, isStatement bool) error {
	if p.Type.identifier == TOKEN_NAME {

		id, err := p.parseIdent(false)
		if err != nil {
			return err
		}
		node.Identifier = id
		if isStatement {
			err := p.checkLValSimple(node.Identifier, BIND_LEXICAL, struct {
				check bool
				hash  map[string]bool
			}{check: false})
			if err != nil {
				return err
			}
		} else {
			if isStatement {
				return p.unexpected("cant be in a statement", nil)
			}

			node.Identifier = nil
		}
		return nil
	}
	return nil
}
