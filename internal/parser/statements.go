package parser

import (
	"github.com/pygo/pygo/internal/ast"
	"github.com/pygo/pygo/internal/lexer"
)

// parseStatement parses one statement (simple or compound).
func (p *Parser) parseStatement() ast.Stmt {
	switch p.cur.Type {
	case lexer.IF:
		return p.parseIf()
	case lexer.WHILE:
		return p.parseWhile()
	case lexer.FOR:
		return p.parseFor()
	case lexer.DEF:
		return p.parseFuncDef(nil)
	case lexer.CLASS:
		return p.parseClassDef(nil)
	case lexer.RETURN:
		return p.parseReturn()
	case lexer.PASS:
		pos := p.pos()
		p.advance()
		p.consumeNewline()
		return ast.SetPos(&ast.Pass{}, pos)
	case lexer.BREAK:
		pos := p.pos()
		p.advance()
		p.consumeNewline()
		return ast.SetPos(&ast.Break{}, pos)
	case lexer.CONTINUE:
		pos := p.pos()
		p.advance()
		p.consumeNewline()
		return ast.SetPos(&ast.Continue{}, pos)
	case lexer.RAISE:
		return p.parseRaise()
	case lexer.TRY:
		return p.parseTry()
	case lexer.WITH:
		return p.parseWith()
	case lexer.IMPORT:
		return p.parseImport()
	case lexer.FROM:
		return p.parseFromImport()
	case lexer.GLOBAL:
		return p.parseGlobal()
	case lexer.NONLOCAL:
		return p.parseNonlocal()
	case lexer.DEL:
		return p.parseDel()
	case lexer.ASSERT:
		return p.parseAssert()
	case lexer.AT:
		return p.parseDecorated()
	default:
		return p.parseSimpleStatement()
	}
}

// parseBlock parses an indented suite of statements following a colon.
//
//	':' NEWLINE INDENT statement+ DEDENT
//
// It also supports a single simple statement on the same line:
//
//	':' simple_statement NEWLINE
func (p *Parser) parseBlock() []ast.Stmt {
	p.expect(lexer.COLON)

	// Single-line suite: `if x: return 1`
	if p.cur.Type != lexer.NEWLINE {
		var body []ast.Stmt
		body = append(body, p.parseSimpleStatement())
		return body
	}

	p.expect(lexer.NEWLINE)
	p.expect(lexer.INDENT)

	var body []ast.Stmt
	for p.cur.Type != lexer.DEDENT && p.cur.Type != lexer.EOF {
		if p.cur.Type == lexer.NEWLINE {
			p.advance()
			continue
		}
		stmt := p.parseStatement()
		if stmt != nil {
			body = append(body, stmt)
		}
		if len(p.errors) > 0 {
			return body
		}
	}
	p.expect(lexer.DEDENT)
	return body
}

// parseSimpleStatement parses assignment, augmented assignment, annotated
// assignment, or a bare expression statement.
func (p *Parser) parseSimpleStatement() ast.Stmt {
	pos := p.pos()
	first := p.parseExprListNoTrailing()

	switch {
	case p.check(lexer.EQUAL):
		// Assignment (possibly chained: a = b = value). Each side may be a
		// bare tuple (a, b = 1, 2).
		targets := []ast.Expr{first}
		for p.accept(lexer.EQUAL) {
			next := p.parseExprListNoTrailing()
			targets = append(targets, next)
		}
		value := targets[len(targets)-1]
		targets = targets[:len(targets)-1]
		setStore(targets)
		p.consumeNewline()
		return ast.SetPos(&ast.Assign{Targets: targets, Value: value}, pos)

	case isAugAssignOp(p.cur.Type):
		op := augAssignOperator(p.cur.Type)
		p.advance()
		value := p.parseExpr()
		setStoreOne(first)
		p.consumeNewline()
		return ast.SetPos(&ast.AugAssign{Target: first, Op: op, Value: value}, pos)

	case p.check(lexer.COLON):
		// Annotated assignment: target: annotation [= value]
		p.advance()
		annotation := p.parseExpr()
		var value ast.Expr
		if p.accept(lexer.EQUAL) {
			value = p.parseExpr()
		}
		_, simple := first.(*ast.Name)
		setStoreOne(first)
		p.consumeNewline()
		return ast.SetPos(&ast.AnnAssign{
			Target: first, Annotation: annotation, Value: value, Simple: simple,
		}, pos)

	default:
		p.consumeNewline()
		return ast.SetPos(&ast.ExprStmt{Value: first}, pos)
	}
}

func (p *Parser) parseIf() ast.Stmt {
	pos := p.pos()
	p.expect(lexer.IF)
	test := p.parseExpr()
	body := p.parseBlock()

	var orelse []ast.Stmt
	switch p.cur.Type {
	case lexer.ELIF:
		// Represent elif as a nested If in orelse.
		orelse = []ast.Stmt{p.parseElif()}
	case lexer.ELSE:
		p.advance()
		orelse = p.parseBlock()
	}
	return ast.SetPos(&ast.If{Test: test, Body: body, Orelse: orelse}, pos)
}

func (p *Parser) parseElif() ast.Stmt {
	pos := p.pos()
	p.expect(lexer.ELIF)
	test := p.parseExpr()
	body := p.parseBlock()

	var orelse []ast.Stmt
	switch p.cur.Type {
	case lexer.ELIF:
		orelse = []ast.Stmt{p.parseElif()}
	case lexer.ELSE:
		p.advance()
		orelse = p.parseBlock()
	}
	return ast.SetPos(&ast.If{Test: test, Body: body, Orelse: orelse}, pos)
}

func (p *Parser) parseWhile() ast.Stmt {
	pos := p.pos()
	p.expect(lexer.WHILE)
	test := p.parseExpr()
	body := p.parseBlock()
	var orelse []ast.Stmt
	if p.accept(lexer.ELSE) {
		orelse = p.parseBlock()
	}
	return ast.SetPos(&ast.While{Test: test, Body: body, Orelse: orelse}, pos)
}

func (p *Parser) parseFor() ast.Stmt {
	pos := p.pos()
	p.expect(lexer.FOR)
	target := p.parseTargetList()
	setStoreOne(target)
	p.expect(lexer.IN)
	iter := p.parseExpr()
	body := p.parseBlock()
	var orelse []ast.Stmt
	if p.accept(lexer.ELSE) {
		orelse = p.parseBlock()
	}
	return ast.SetPos(&ast.For{Target: target, Iter: iter, Body: body, Orelse: orelse}, pos)
}

// parseTargetList parses a for-loop target, which may be a tuple without parens.
func (p *Parser) parseTargetList() ast.Expr {
	pos := p.pos()
	first := p.parseAtomTrailer()
	if !p.check(lexer.COMMA) {
		return first
	}
	elts := []ast.Expr{first}
	for p.accept(lexer.COMMA) {
		if p.check(lexer.IN) {
			break
		}
		elts = append(elts, p.parseAtomTrailer())
	}
	return ast.SetPos(&ast.Tuple{Elts: elts, Ctx: ast.Store}, pos)
}

func (p *Parser) parseReturn() ast.Stmt {
	pos := p.pos()
	p.expect(lexer.RETURN)
	var value ast.Expr
	if !p.check(lexer.NEWLINE) && !p.check(lexer.EOF) && !p.check(lexer.SEMICOLON) {
		value = p.parseExprList()
	}
	p.consumeNewline()
	return ast.SetPos(&ast.Return{Value: value}, pos)
}

func (p *Parser) parseRaise() ast.Stmt {
	pos := p.pos()
	p.expect(lexer.RAISE)
	var exc, cause ast.Expr
	if !p.check(lexer.NEWLINE) && !p.check(lexer.EOF) && !p.check(lexer.SEMICOLON) {
		exc = p.parseExpr()
		if p.accept(lexer.FROM) {
			cause = p.parseExpr()
		}
	}
	p.consumeNewline()
	return ast.SetPos(&ast.Raise{Exc: exc, Cause: cause}, pos)
}

func (p *Parser) parseTry() ast.Stmt {
	pos := p.pos()
	p.expect(lexer.TRY)
	body := p.parseBlock()

	var handlers []*ast.ExceptHandler
	for p.check(lexer.EXCEPT) {
		hpos := p.pos()
		p.advance()
		var typ ast.Expr
		var name string
		if !p.check(lexer.COLON) {
			typ = p.parseExpr()
			if p.accept(lexer.AS) {
				name = p.expect(lexer.NAME).Value
			}
		}
		hbody := p.parseBlock()
		h := &ast.ExceptHandler{Type: typ, Name: name, Body: hbody}
		ast.SetPos(h, hpos)
		handlers = append(handlers, h)
	}

	var orelse, finalbody []ast.Stmt
	if p.accept(lexer.ELSE) {
		orelse = p.parseBlock()
	}
	if p.accept(lexer.FINALLY) {
		finalbody = p.parseBlock()
	}
	return ast.SetPos(&ast.Try{
		Body: body, Handlers: handlers, Orelse: orelse, Finalbody: finalbody,
	}, pos)
}

func (p *Parser) parseWith() ast.Stmt {
	pos := p.pos()
	p.expect(lexer.WITH)
	var items []*ast.WithItem
	for {
		ctx := p.parseExpr()
		var optVar ast.Expr
		if p.accept(lexer.AS) {
			optVar = p.parseAtomTrailer()
			setStoreOne(optVar)
		}
		items = append(items, &ast.WithItem{ContextExpr: ctx, OptionalVar: optVar})
		if !p.accept(lexer.COMMA) {
			break
		}
	}
	body := p.parseBlock()
	return ast.SetPos(&ast.With{Items: items, Body: body}, pos)
}

func (p *Parser) parseImport() ast.Stmt {
	pos := p.pos()
	p.expect(lexer.IMPORT)
	var names []*ast.Alias
	for {
		name := p.parseDottedName()
		alias := &ast.Alias{Name: name}
		if p.accept(lexer.AS) {
			alias.AsName = p.expect(lexer.NAME).Value
		}
		names = append(names, alias)
		if !p.accept(lexer.COMMA) {
			break
		}
	}
	p.consumeNewline()
	return ast.SetPos(&ast.Import{Names: names}, pos)
}

func (p *Parser) parseFromImport() ast.Stmt {
	pos := p.pos()
	p.expect(lexer.FROM)
	level := 0
	for p.check(lexer.DOT) {
		level++
		p.advance()
	}
	module := ""
	if p.check(lexer.NAME) {
		module = p.parseDottedName()
	}
	p.expect(lexer.IMPORT)

	var names []*ast.Alias
	if p.accept(lexer.STAR) {
		names = append(names, &ast.Alias{Name: "*"})
	} else {
		wrapped := p.accept(lexer.LPAREN)
		for {
			n := p.expect(lexer.NAME).Value
			alias := &ast.Alias{Name: n}
			if p.accept(lexer.AS) {
				alias.AsName = p.expect(lexer.NAME).Value
			}
			names = append(names, alias)
			if !p.accept(lexer.COMMA) {
				break
			}
			if wrapped && p.check(lexer.RPAREN) {
				break
			}
		}
		if wrapped {
			p.expect(lexer.RPAREN)
		}
	}
	p.consumeNewline()
	return ast.SetPos(&ast.ImportFrom{Module: module, Names: names, Level: level}, pos)
}

func (p *Parser) parseDottedName() string {
	name := p.expect(lexer.NAME).Value
	for p.check(lexer.DOT) {
		p.advance()
		name += "." + p.expect(lexer.NAME).Value
	}
	return name
}

func (p *Parser) parseGlobal() ast.Stmt {
	pos := p.pos()
	p.expect(lexer.GLOBAL)
	names := []string{p.expect(lexer.NAME).Value}
	for p.accept(lexer.COMMA) {
		names = append(names, p.expect(lexer.NAME).Value)
	}
	p.consumeNewline()
	return ast.SetPos(&ast.Global{Names: names}, pos)
}

func (p *Parser) parseNonlocal() ast.Stmt {
	pos := p.pos()
	p.expect(lexer.NONLOCAL)
	names := []string{p.expect(lexer.NAME).Value}
	for p.accept(lexer.COMMA) {
		names = append(names, p.expect(lexer.NAME).Value)
	}
	p.consumeNewline()
	return ast.SetPos(&ast.Nonlocal{Names: names}, pos)
}

func (p *Parser) parseDel() ast.Stmt {
	pos := p.pos()
	p.expect(lexer.DEL)
	targets := []ast.Expr{p.parseAtomTrailer()}
	setStoreOneCtx(targets[0], ast.Del)
	for p.accept(lexer.COMMA) {
		t := p.parseAtomTrailer()
		setStoreOneCtx(t, ast.Del)
		targets = append(targets, t)
	}
	p.consumeNewline()
	return ast.SetPos(&ast.Delete{Targets: targets}, pos)
}

func (p *Parser) parseAssert() ast.Stmt {
	pos := p.pos()
	p.expect(lexer.ASSERT)
	test := p.parseExpr()
	var msg ast.Expr
	if p.accept(lexer.COMMA) {
		msg = p.parseExpr()
	}
	p.consumeNewline()
	return ast.SetPos(&ast.Assert{Test: test, Msg: msg}, pos)
}

// parseDecorated parses one or more decorators followed by a def or class.
func (p *Parser) parseDecorated() ast.Stmt {
	var decorators []ast.Expr
	for p.check(lexer.AT) {
		p.advance()
		decorators = append(decorators, p.parseExpr())
		p.consumeNewline()
	}
	switch p.cur.Type {
	case lexer.DEF:
		return p.parseFuncDef(decorators)
	case lexer.CLASS:
		return p.parseClassDef(decorators)
	default:
		p.errorf("expected 'def' or 'class' after decorator, got %s", p.cur.Type)
		return nil
	}
}

func (p *Parser) parseFuncDef(decorators []ast.Expr) ast.Stmt {
	pos := p.pos()
	p.expect(lexer.DEF)
	name := p.expect(lexer.NAME).Value
	p.expect(lexer.LPAREN)
	args := p.parseParameters()
	p.expect(lexer.RPAREN)

	var returns ast.Expr
	if p.accept(lexer.RARROW) {
		returns = p.parseExpr()
	}
	body := p.parseBlock()
	return ast.SetPos(&ast.FunctionDef{
		Name: name, Args: args, Body: body, Decorators: decorators, Returns: returns,
	}, pos)
}

func (p *Parser) parseClassDef(decorators []ast.Expr) ast.Stmt {
	pos := p.pos()
	p.expect(lexer.CLASS)
	name := p.expect(lexer.NAME).Value

	var bases []ast.Expr
	var keywords []*ast.Keyword
	if p.accept(lexer.LPAREN) {
		for !p.check(lexer.RPAREN) {
			if p.check(lexer.NAME) && p.peek(1).Type == lexer.EQUAL {
				kw := p.expect(lexer.NAME).Value
				p.expect(lexer.EQUAL)
				val := p.parseExpr()
				keywords = append(keywords, &ast.Keyword{Arg: kw, Value: val})
			} else {
				bases = append(bases, p.parseExpr())
			}
			if !p.accept(lexer.COMMA) {
				break
			}
		}
		p.expect(lexer.RPAREN)
	}
	body := p.parseBlock()
	return ast.SetPos(&ast.ClassDef{
		Name: name, Bases: bases, Keywords: keywords, Body: body, Decorators: decorators,
	}, pos)
}

// parseParameters parses a function parameter list.
// Supports positional params with defaults, *args, and **kwargs.
func (p *Parser) parseParameters() *ast.Arguments {
	args := &ast.Arguments{}
	seenStar := false

	for !p.check(lexer.RPAREN) && p.cur.Type != lexer.EOF {
		switch {
		case p.accept(lexer.DOUBLESTAR):
			name := p.expect(lexer.NAME).Value
			args.Kwarg = ast.NewArg(name, nil, p.pos())

		case p.accept(lexer.STAR):
			seenStar = true
			if p.check(lexer.NAME) {
				name := p.expect(lexer.NAME).Value
				args.Vararg = ast.NewArg(name, nil, p.pos())
			}
			// bare * introduces keyword-only args that follow

		default:
			apos := p.pos()
			name := p.expect(lexer.NAME).Value
			var annotation ast.Expr
			if p.accept(lexer.COLON) {
				annotation = p.parseExpr()
			}
			arg := ast.NewArg(name, annotation, apos)

			var def ast.Expr
			if p.accept(lexer.EQUAL) {
				def = p.parseExpr()
			}

			if seenStar {
				args.KwonlyArgs = append(args.KwonlyArgs, arg)
				args.KwDefaults = append(args.KwDefaults, def)
			} else {
				args.Args = append(args.Args, arg)
				if def != nil {
					args.Defaults = append(args.Defaults, def)
				}
			}
		}

		if !p.accept(lexer.COMMA) {
			break
		}
	}
	return args
}

// ============================================================
// Assignment-target context helpers
// ============================================================

func setStore(targets []ast.Expr) {
	for _, t := range targets {
		setStoreOne(t)
	}
}

func setStoreOne(e ast.Expr) { setStoreOneCtx(e, ast.Store) }

func setStoreOneCtx(e ast.Expr, ctx ast.ExprContext) {
	switch n := e.(type) {
	case *ast.Name:
		n.Ctx = ctx
	case *ast.Attribute:
		n.Ctx = ctx
	case *ast.Subscript:
		n.Ctx = ctx
	case *ast.Tuple:
		n.Ctx = ctx
		for _, elt := range n.Elts {
			setStoreOneCtx(elt, ctx)
		}
	case *ast.List:
		n.Ctx = ctx
		for _, elt := range n.Elts {
			setStoreOneCtx(elt, ctx)
		}
	case *ast.Starred:
		n.Ctx = ctx
		setStoreOneCtx(n.Value, ctx)
	}
}

func isAugAssignOp(tt lexer.TokenType) bool {
	switch tt {
	case lexer.PLUSEQUAL, lexer.MINUSEQUAL, lexer.STAREQUAL, lexer.SLASHEQUAL,
		lexer.DOUBLESLASHEQUAL, lexer.PERCENTEQUAL, lexer.DOUBLESTAREQUAL,
		lexer.AMPEREQUAL, lexer.PIPEEQUAL, lexer.CARETEQUAL,
		lexer.LSHIFTEQUAL, lexer.RSHIFTEQUAL, lexer.ATEQUAL:
		return true
	}
	return false
}

func augAssignOperator(tt lexer.TokenType) ast.Operator {
	switch tt {
	case lexer.PLUSEQUAL:
		return ast.Add
	case lexer.MINUSEQUAL:
		return ast.Sub
	case lexer.STAREQUAL:
		return ast.Mult
	case lexer.SLASHEQUAL:
		return ast.Div
	case lexer.DOUBLESLASHEQUAL:
		return ast.FloorDiv
	case lexer.PERCENTEQUAL:
		return ast.Mod
	case lexer.DOUBLESTAREQUAL:
		return ast.Pow
	case lexer.AMPEREQUAL:
		return ast.BitAnd
	case lexer.PIPEEQUAL:
		return ast.BitOr
	case lexer.CARETEQUAL:
		return ast.BitXor
	case lexer.LSHIFTEQUAL:
		return ast.LShift
	case lexer.RSHIFTEQUAL:
		return ast.RShift
	case lexer.ATEQUAL:
		return ast.MatMult
	}
	return ast.Add
}
