package parser

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/pygo/pygo/internal/ast"
	"github.com/pygo/pygo/internal/bytecode"
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

// parseExprListNoTrailing parses a comma-separated expression list used on the
// left of an assignment or as an expression statement. It returns a Tuple if
// commas are present (supporting `a, b = ...` unpacking and `a, b` tuples).
func (p *Parser) parseExprListNoTrailing() ast.Expr {
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
			argExpr := p.parseExpr()
			// A bare generator expression as a sole argument: f(x for x in it)
			if p.check(lexer.FOR) {
				gpos := argExpr.Pos()
				gens := p.parseComprehensionClauses()
				argExpr = ast.SetPos(&ast.GeneratorExp{Elt: argExpr, Generators: gens}, gpos)
			}
			args = append(args, argExpr)
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
	case lexer.BYTES:
		// Adjacent bytes literals concatenate; carry the raw bytes through the
		// constant pool as a []byte so the VM builds a BytesObject.
		var raw []byte
		for p.check(lexer.BYTES) {
			raw = append(raw, []byte(p.advance().Value)...)
		}
		return ast.SetPos(&ast.Constant{Value: bytesConstant(raw)}, pos)
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

func bytesConstant(b []byte) *bytecode.BytesConstant { return &bytecode.BytesConstant{Data: b} }

// parseStringConcat handles implicit adjacent string-literal concatenation,
// including f-strings. Adjacent literals are joined; if any piece is an
// f-string, the result is a concatenation of literal text and str(expr) calls.
func (p *Parser) parseStringConcat() ast.Expr {
	pos := p.pos()

	var parts []ast.Expr // each part is a str-valued expression to concatenate
	addLiteral := func(s string) {
		if s == "" {
			return
		}
		parts = append(parts, ast.SetPos(&ast.Constant{Value: s}, pos))
	}

	for p.check(lexer.STRING) {
		tok := p.advance()
		if tok.IsFString() {
			fparts, err := p.parseFStringParts(tok.Value, pos)
			if err != nil {
				p.errors = append(p.errors, err)
				return ast.SetPos(&ast.Constant{Value: ""}, pos)
			}
			parts = append(parts, fparts...)
		} else {
			addLiteral(tok.Value)
		}
	}

	switch len(parts) {
	case 0:
		return ast.SetPos(&ast.Constant{Value: ""}, pos)
	case 1:
		// A lone literal part is a plain constant; a lone expr part needs no +.
		return parts[0]
	}
	// Fold parts into a left-associative + chain.
	acc := parts[0]
	for _, part := range parts[1:] {
		acc = ast.SetPos(&ast.BinOp{Left: acc, Op: ast.Add, Right: part}, pos)
	}
	return acc
}

// parseFStringParts splits an f-string's content into literal and expression
// parts. Each {expr} becomes str(expr); {{ and }} are literal braces.
// Format specs and conversions (e.g. {x:.2f}, {x!r}) are not yet supported;
// the expression up to the closing brace is evaluated and stringified.
func (p *Parser) parseFStringParts(content string, pos ast.Position) ([]ast.Expr, *Error) {
	var parts []ast.Expr
	var lit strings.Builder
	flushLit := func() {
		if lit.Len() > 0 {
			parts = append(parts, ast.SetPos(&ast.Constant{Value: lit.String()}, pos))
			lit.Reset()
		}
	}

	runes := []rune(content)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch c {
		case '{':
			if i+1 < len(runes) && runes[i+1] == '{' {
				lit.WriteRune('{')
				i++
				continue
			}
			// Read until the matching closing brace (respecting nesting).
			flushLit()
			depth := 1
			j := i + 1
			var exprSrc strings.Builder
			for j < len(runes) && depth > 0 {
				switch runes[j] {
				case '{':
					depth++
				case '}':
					depth--
					if depth == 0 {
						j++
						goto done
					}
				}
				exprSrc.WriteRune(runes[j])
				j++
			}
		done:
			raw := exprSrc.String()
			// Strip an optional conversion/format spec (!r, :spec) — evaluated
			// as plain str() for now.
			if idx := splitFormatSpec(raw); idx >= 0 {
				raw = raw[:idx]
			}
			sub := New(p.filename, raw)
			expr, err := sub.ParseExpression()
			if err != nil {
				return nil, &Error{Filename: p.filename, Line: pos.Line, Col: pos.Col,
					Message: "invalid f-string expression: " + err.Error()}
			}
			// Wrap in str(...) so the result concatenates as text.
			call := ast.SetPos(&ast.Call{
				Func: ast.SetPos(&ast.Name{Id: "str", Ctx: ast.Load}, pos),
				Args: []ast.Expr{expr},
			}, pos)
			parts = append(parts, call)
			i = j - 1
		case '}':
			if i+1 < len(runes) && runes[i+1] == '}' {
				lit.WriteRune('}')
				i++
				continue
			}
			lit.WriteRune('}')
		default:
			lit.WriteRune(c)
		}
	}
	flushLit()
	if len(parts) == 0 {
		parts = append(parts, ast.SetPos(&ast.Constant{Value: ""}, pos))
	}
	return parts, nil
}

// splitFormatSpec returns the index where a conversion (!) or format spec (:)
// begins in an f-string field, or -1 if there is none. It ignores characters
// inside brackets/parens so that dict/call syntax is not mistaken for a spec.
func splitFormatSpec(s string) int {
	depth := 0
	for i, c := range s {
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case '!':
			// "!=" is a comparison, not a conversion.
			if depth == 0 && (i+1 >= len(s) || s[i+1] != '=') {
				return i
			}
		case ':':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
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
