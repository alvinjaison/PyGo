package parser

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/pygo/pygo/internal/ast"
	"github.com/pygo/pygo/internal/lexer"
)

// Precedence levels, lowest to highest.
const (
	precNone = iota
	precTernary // x if c else y
	precOr      // or
	precAnd     // and
	precNot     // not x
	precCompare // < > == != <= >= in not-in is is-not
	precBitOr   // |
	precBitXor  // ^
	precBitAnd  // &
	precShift   // << >>
	precAdd     // + -
	precMul     // * / // % @
	precUnary   // -x +x ~x
	precPow     // **
	precCall    // f() x[] x.attr
)

// parseExprList parses a comma-separated expression list, producing a Tuple
// if more than one element is present. Used for return values and assignments.
func (p *Parser) parseExprList() ast.Expr {
	pos := p.pos()
	first := p.parseExpr()
	if !p.check(lexer.COMMA) {
		return first
	}
	elts := []ast.Expr{first}
	for p.accept(lexer.COMMA) {
		if p.isExprEnd() {
			break
		}
		elts = append(elts, p.parseExpr())
	}
	return ast.SetPos(&ast.Tuple{Elts: elts, Ctx: ast.Load}, pos)
}

func (p *Parser) isExprEnd() bool {
	switch p.cur.Type {
	case lexer.NEWLINE, lexer.EOF, lexer.SEMICOLON, lexer.RPAREN,
		lexer.RBRACKET, lexer.RBRACE, lexer.COLON, lexer.EQUAL:
		return true
	}
	return false
}

// parseExpr parses a full expression, including ternary conditionals.
func (p *Parser) parseExpr() ast.Expr {
	return p.parseTernary()
}

func (p *Parser) parseTernary() ast.Expr {
	pos := p.pos()
	body := p.parseBinary(precOr)
	if p.check(lexer.IF) {
		p.advance()
		test := p.parseBinary(precOr)
		p.expect(lexer.ELSE)
		orelse := p.parseExpr()
		return ast.SetPos(&ast.IfExp{Test: test, Body: body, Orelse: orelse}, pos)
	}
	return body
}

// parseBinary parses binary operators using precedence climbing.
func (p *Parser) parseBinary(minPrec int) ast.Expr {
	left := p.parseUnary()

	for {
		prec, rightAssoc := binaryPrecedence(p.cur.Type)
		if prec < minPrec || prec == precNone {
			break
		}

		// Boolean operators.
		if p.cur.Type == lexer.AND || p.cur.Type == lexer.OR {
			left = p.parseBoolOp(left, prec)
			continue
		}

		// Comparison chain (handles a < b < c and `is`, `in`, `not in`, `is not`).
		if isComparisonStart(p) {
			left = p.parseComparison(left)
			continue
		}

		pos := left.Pos()
		op := p.cur.Type
		p.advance()

		nextMin := prec + 1
		if rightAssoc {
			nextMin = prec
		}
		right := p.parseBinaryFromUnary(nextMin)

		left = ast.SetPos(&ast.BinOp{
			Left: left, Op: binaryOperator(op), Right: right,
		}, pos)
	}
	return left
}

// parseBinaryFromUnary is like parseBinary but begins at a unary expression,
// used for the right-hand side of a binary operator.
func (p *Parser) parseBinaryFromUnary(minPrec int) ast.Expr {
	left := p.parseUnary()
	for {
		prec, rightAssoc := binaryPrecedence(p.cur.Type)
		if prec < minPrec || prec == precNone {
			break
		}
		if p.cur.Type == lexer.AND || p.cur.Type == lexer.OR {
			left = p.parseBoolOp(left, prec)
			continue
		}
		if isComparisonStart(p) {
			left = p.parseComparison(left)
			continue
		}
		pos := left.Pos()
		op := p.cur.Type
		p.advance()
		nextMin := prec + 1
		if rightAssoc {
			nextMin = prec
		}
		right := p.parseBinaryFromUnary(nextMin)
		left = ast.SetPos(&ast.BinOp{Left: left, Op: binaryOperator(op), Right: right}, pos)
	}
	return left
}

func (p *Parser) parseBoolOp(left ast.Expr, prec int) ast.Expr {
	pos := left.Pos()
	opTok := p.cur.Type
	var op ast.BoolOperator
	if opTok == lexer.AND {
		op = ast.And
	} else {
		op = ast.Or
	}
	values := []ast.Expr{left}
	for p.cur.Type == opTok {
		p.advance()
		values = append(values, p.parseBinaryFromUnary(prec+1))
	}
	return ast.SetPos(&ast.BoolOp{Op: op, Values: values}, pos)
}

func (p *Parser) parseComparison(left ast.Expr) ast.Expr {
	pos := left.Pos()
	var ops []ast.CmpOp
	var comparators []ast.Expr

	for isComparisonStart(p) {
		op := p.parseCompareOp()
		ops = append(ops, op)
		comparators = append(comparators, p.parseBinaryFromUnary(precBitOr))
	}
	return ast.SetPos(&ast.Compare{Left: left, Ops: ops, Comparators: comparators}, pos)
}

func (p *Parser) parseCompareOp() ast.CmpOp {
	switch p.cur.Type {
	case lexer.LESS:
		p.advance()
		return ast.Lt
	case lexer.GREATER:
		p.advance()
		return ast.Gt
	case lexer.LESSEQUAL:
		p.advance()
		return ast.LtE
	case lexer.GREATEREQUAL:
		p.advance()
		return ast.GtE
	case lexer.EQEQUAL:
		p.advance()
		return ast.Eq
	case lexer.NOTEQUAL:
		p.advance()
		return ast.NotEq
	case lexer.IN:
		p.advance()
		return ast.In
	case lexer.IS:
		p.advance()
		if p.accept(lexer.NOT) {
			return ast.IsNot
		}
		return ast.Is
	case lexer.NOT:
		// `not in`
		p.advance()
		p.expect(lexer.IN)
		return ast.NotIn
	}
	p.errorf("expected comparison operator, got %s", p.cur.Type)
	return ast.Eq
}

func isComparisonStart(p *Parser) bool {
	switch p.cur.Type {
	case lexer.LESS, lexer.GREATER, lexer.LESSEQUAL, lexer.GREATEREQUAL,
		lexer.EQEQUAL, lexer.NOTEQUAL, lexer.IN, lexer.IS:
		return true
	case lexer.NOT:
		// `not in` — only a comparison if followed by `in`
		return p.peek(1).Type == lexer.IN
	}
	return false
}

// parseUnary parses unary operators and `not`.
func (p *Parser) parseUnary() ast.Expr {
	pos := p.pos()
	switch p.cur.Type {
	case lexer.MINUS:
		p.advance()
		return ast.SetPos(&ast.UnaryOp{Op: ast.USub, Operand: p.parseUnary()}, pos)
	case lexer.PLUS:
		p.advance()
		return ast.SetPos(&ast.UnaryOp{Op: ast.UAdd, Operand: p.parseUnary()}, pos)
	case lexer.TILDE:
		p.advance()
		return ast.SetPos(&ast.UnaryOp{Op: ast.Invert, Operand: p.parseUnary()}, pos)
	case lexer.NOT:
		p.advance()
		return ast.SetPos(&ast.UnaryOp{Op: ast.Not, Operand: p.parseUnary()}, pos)
	}
	return p.parsePower()
}

// parsePower handles the ** operator, which is right-associative and binds
// more tightly than unary on its left but less than unary on its right.
func (p *Parser) parsePower() ast.Expr {
	pos := p.pos()
	base := p.parseAtomTrailer()
	if p.check(lexer.DOUBLESTAR) {
		p.advance()
		exp := p.parseUnary() // right side allows unary: 2 ** -1
		return ast.SetPos(&ast.BinOp{Left: base, Op: ast.Pow, Right: exp}, pos)
	}
	return base
}

// parseAtomTrailer parses an atom followed by call/subscript/attribute trailers.
func (p *Parser) parseAtomTrailer() ast.Expr {
	expr := p.parseAtom()
	for {
		switch p.cur.Type {
		case lexer.LPAREN:
			expr = p.parseCall(expr)
		case lexer.LBRACKET:
			expr = p.parseSubscript(expr)
		case lexer.DOT:
			pos := expr.Pos()
			p.advance()
			attr := p.expect(lexer.NAME).Value
			expr = ast.SetPos(&ast.Attribute{Value: expr, Attr: attr, Ctx: ast.Load}, pos)
		default:
			return expr
		}
	}
}

func (p *Parser) parseCall(fn ast.Expr) ast.Expr {
	pos := fn.Pos()
	p.expect(lexer.LPAREN)
	var args []ast.Expr
	var keywords []*ast.Keyword

	for !p.check(lexer.RPAREN) && p.cur.Type != lexer.EOF {
		switch {
		case p.accept(lexer.DOUBLESTAR):
			keywords = append(keywords, &ast.Keyword{Arg: "", Value: p.parseExpr()})
		case p.accept(lexer.STAR):
			spos := p.pos()
			args = append(args, ast.SetPos(&ast.Starred{Value: p.parseExpr(), Ctx: ast.Load}, spos))
		case p.check(lexer.NAME) && p.peek(1).Type == lexer.EQUAL:
			kw := p.expect(lexer.NAME).Value
			p.expect(lexer.EQUAL)
			keywords = append(keywords, &ast.Keyword{Arg: kw, Value: p.parseExpr()})
		default:
			args = append(args, p.parseExpr())
		}
		if !p.accept(lexer.COMMA) {
			break
		}
	}
	p.expect(lexer.RPAREN)
	return ast.SetPos(&ast.Call{Func: fn, Args: args, Keywords: keywords}, pos)
}

func (p *Parser) parseSubscript(value ast.Expr) ast.Expr {
	pos := value.Pos()
	p.expect(lexer.LBRACKET)
	sl := p.parseSliceOrExpr()
	p.expect(lexer.RBRACKET)
	return ast.SetPos(&ast.Subscript{Value: value, Slice: sl, Ctx: ast.Load}, pos)
}

// parseSliceOrExpr parses the inside of [], which may be an expression or a slice.
func (p *Parser) parseSliceOrExpr() ast.Expr {
	pos := p.pos()

	var lower ast.Expr
	if !p.check(lexer.COLON) {
		lower = p.parseExpr()
		if !p.check(lexer.COLON) {
			return lower // plain index
		}
	}
	// It's a slice.
	p.expect(lexer.COLON)
	var upper, step ast.Expr
	if !p.check(lexer.COLON) && !p.check(lexer.RBRACKET) {
		upper = p.parseExpr()
	}
	if p.accept(lexer.COLON) {
		if !p.check(lexer.RBRACKET) {
			step = p.parseExpr()
		}
	}
	return ast.SetPos(&ast.Slice{Lower: lower, Upper: upper, Step: step}, pos)
}

// parseAtom parses the most basic expression units.
func (p *Parser) parseAtom() ast.Expr {
	pos := p.pos()
	switch p.cur.Type {
	case lexer.INTEGER:
		tok := p.advance()
		return ast.SetPos(&ast.Constant{Value: parseIntLiteral(tok.Value)}, pos)
	case lexer.FLOAT:
		tok := p.advance()
		f, _ := strconv.ParseFloat(strings.ReplaceAll(tok.Value, "_", ""), 64)
		return ast.SetPos(&ast.Constant{Value: f}, pos)
	case lexer.STRING:
		return p.parseStringConcat()
	case lexer.NAME:
		tok := p.advance()
		return ast.SetPos(&ast.Name{Id: tok.Value, Ctx: ast.Load}, pos)
	case lexer.TRUE:
		p.advance()
		return ast.SetPos(&ast.Constant{Value: true}, pos)
	case lexer.FALSE:
		p.advance()
		return ast.SetPos(&ast.Constant{Value: false}, pos)
	case lexer.NONE:
		p.advance()
		return ast.SetPos(&ast.Constant{Value: nil}, pos)
	case lexer.LPAREN:
		return p.parseParenOrTuple()
	case lexer.LBRACKET:
		return p.parseListOrComp()
	case lexer.LBRACE:
		return p.parseDictOrSet()
	case lexer.LAMBDA:
		return p.parseLambda()
	case lexer.YIELD:
		return p.parseYield()
	}
	p.errorf("unexpected token %s", p.cur.Type)
	p.advance()
	return ast.SetPos(&ast.Constant{Value: nil}, pos)
}

// parseStringConcat handles implicit adjacent string literal concatenation.
func (p *Parser) parseStringConcat() ast.Expr {
	pos := p.pos()
	var sb strings.Builder
	for p.check(lexer.STRING) {
		sb.WriteString(p.advance().Value)
	}
	return ast.SetPos(&ast.Constant{Value: sb.String()}, pos)
}

func (p *Parser) parseParenOrTuple() ast.Expr {
	pos := p.pos()
	p.expect(lexer.LPAREN)

	if p.accept(lexer.RPAREN) {
		return ast.SetPos(&ast.Tuple{Elts: nil, Ctx: ast.Load}, pos)
	}

	first := p.parseExpr()

	// Generator expression: (x for ...)
	if p.check(lexer.FOR) {
		gens := p.parseComprehensionClauses()
		p.expect(lexer.RPAREN)
		return ast.SetPos(&ast.GeneratorExp{Elt: first, Generators: gens}, pos)
	}

	if !p.check(lexer.COMMA) {
		p.expect(lexer.RPAREN)
		return first // parenthesized expression
	}

	elts := []ast.Expr{first}
	for p.accept(lexer.COMMA) {
		if p.check(lexer.RPAREN) {
			break
		}
		elts = append(elts, p.parseExpr())
	}
	p.expect(lexer.RPAREN)
	return ast.SetPos(&ast.Tuple{Elts: elts, Ctx: ast.Load}, pos)
}

func (p *Parser) parseListOrComp() ast.Expr {
	pos := p.pos()
	p.expect(lexer.LBRACKET)

	if p.accept(lexer.RBRACKET) {
		return ast.SetPos(&ast.List{Elts: nil, Ctx: ast.Load}, pos)
	}

	first := p.parseExpr()
	if p.check(lexer.FOR) {
		gens := p.parseComprehensionClauses()
		p.expect(lexer.RBRACKET)
		return ast.SetPos(&ast.ListComp{Elt: first, Generators: gens}, pos)
	}

	elts := []ast.Expr{first}
	for p.accept(lexer.COMMA) {
		if p.check(lexer.RBRACKET) {
			break
		}
		elts = append(elts, p.parseExpr())
	}
	p.expect(lexer.RBRACKET)
	return ast.SetPos(&ast.List{Elts: elts, Ctx: ast.Load}, pos)
}

func (p *Parser) parseDictOrSet() ast.Expr {
	pos := p.pos()
	p.expect(lexer.LBRACE)

	if p.accept(lexer.RBRACE) {
		return ast.SetPos(&ast.Dict{}, pos) // empty dict
	}

	first := p.parseExpr()

	if p.accept(lexer.COLON) {
		// Dict
		firstVal := p.parseExpr()
		if p.check(lexer.FOR) {
			gens := p.parseComprehensionClauses()
			p.expect(lexer.RBRACE)
			return ast.SetPos(&ast.DictComp{Key: first, Value: firstVal, Generators: gens}, pos)
		}
		keys := []ast.Expr{first}
		values := []ast.Expr{firstVal}
		for p.accept(lexer.COMMA) {
			if p.check(lexer.RBRACE) {
				break
			}
			keys = append(keys, p.parseExpr())
			p.expect(lexer.COLON)
			values = append(values, p.parseExpr())
		}
		p.expect(lexer.RBRACE)
		return ast.SetPos(&ast.Dict{Keys: keys, Values: values}, pos)
	}

	// Set
	if p.check(lexer.FOR) {
		gens := p.parseComprehensionClauses()
		p.expect(lexer.RBRACE)
		return ast.SetPos(&ast.SetComp{Elt: first, Generators: gens}, pos)
	}
	elts := []ast.Expr{first}
	for p.accept(lexer.COMMA) {
		if p.check(lexer.RBRACE) {
			break
		}
		elts = append(elts, p.parseExpr())
	}
	p.expect(lexer.RBRACE)
	return ast.SetPos(&ast.Set{Elts: elts}, pos)
}

func (p *Parser) parseComprehensionClauses() []*ast.Comprehension {
	var gens []*ast.Comprehension
	for p.check(lexer.FOR) {
		p.advance()
		target := p.parseTargetList()
		setStoreOne(target)
		p.expect(lexer.IN)
		iter := p.parseBinary(precOr)
		comp := &ast.Comprehension{Target: target, Iter: iter}
		for p.check(lexer.IF) {
			p.advance()
			comp.Ifs = append(comp.Ifs, p.parseBinary(precOr))
		}
		gens = append(gens, comp)
	}
	return gens
}

func (p *Parser) parseLambda() ast.Expr {
	pos := p.pos()
	p.expect(lexer.LAMBDA)
	args := &ast.Arguments{}
	for !p.check(lexer.COLON) && p.cur.Type != lexer.EOF {
		apos := p.pos()
		name := p.expect(lexer.NAME).Value
		arg := ast.NewArg(name, nil, apos)
		var def ast.Expr
		if p.accept(lexer.EQUAL) {
			def = p.parseExpr()
		}
		args.Args = append(args.Args, arg)
		if def != nil {
			args.Defaults = append(args.Defaults, def)
		}
		if !p.accept(lexer.COMMA) {
			break
		}
	}
	p.expect(lexer.COLON)
	body := p.parseExpr()
	return ast.SetPos(&ast.Lambda{Args: args, Body: body}, pos)
}

func (p *Parser) parseYield() ast.Expr {
	pos := p.pos()
	p.expect(lexer.YIELD)
	if p.accept(lexer.FROM) {
		return ast.SetPos(&ast.YieldFrom{Value: p.parseExpr()}, pos)
	}
	var value ast.Expr
	if !p.isExprEnd() {
		value = p.parseExprList()
	}
	return ast.SetPos(&ast.Yield{Value: value}, pos)
}

// ============================================================
// Precedence and operator mapping
// ============================================================

func binaryPrecedence(tt lexer.TokenType) (prec int, rightAssoc bool) {
	switch tt {
	case lexer.OR:
		return precOr, false
	case lexer.AND:
		return precAnd, false
	case lexer.LESS, lexer.GREATER, lexer.LESSEQUAL, lexer.GREATEREQUAL,
		lexer.EQEQUAL, lexer.NOTEQUAL, lexer.IN, lexer.IS, lexer.NOT:
		return precCompare, false
	case lexer.PIPE:
		return precBitOr, false
	case lexer.CARET:
		return precBitXor, false
	case lexer.AMPER:
		return precBitAnd, false
	case lexer.LSHIFT, lexer.RSHIFT:
		return precShift, false
	case lexer.PLUS, lexer.MINUS:
		return precAdd, false
	case lexer.STAR, lexer.SLASH, lexer.DOUBLESLASH, lexer.PERCENT, lexer.AT:
		return precMul, false
	}
	return precNone, false
}

func binaryOperator(tt lexer.TokenType) ast.Operator {
	switch tt {
	case lexer.PLUS:
		return ast.Add
	case lexer.MINUS:
		return ast.Sub
	case lexer.STAR:
		return ast.Mult
	case lexer.SLASH:
		return ast.Div
	case lexer.DOUBLESLASH:
		return ast.FloorDiv
	case lexer.PERCENT:
		return ast.Mod
	case lexer.AT:
		return ast.MatMult
	case lexer.PIPE:
		return ast.BitOr
	case lexer.CARET:
		return ast.BitXor
	case lexer.AMPER:
		return ast.BitAnd
	case lexer.LSHIFT:
		return ast.LShift
	case lexer.RSHIFT:
		return ast.RShift
	}
	return ast.Add
}

// parseIntLiteral parses a Python integer literal, returning int64 if it fits,
// otherwise *big.Int.
func parseIntLiteral(s string) interface{} {
	s = strings.ReplaceAll(s, "_", "")
	base := 10
	switch {
	case strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X"):
		base, s = 16, s[2:]
	case strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0O"):
		base, s = 8, s[2:]
	case strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B"):
		base, s = 2, s[2:]
	}
	if n, err := strconv.ParseInt(s, base, 64); err == nil {
		return n
	}
	bi := new(big.Int)
	if _, ok := bi.SetString(s, base); ok {
		return bi
	}
	return int64(0)
}
