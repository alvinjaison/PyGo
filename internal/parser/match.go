package parser

import (
	"github.com/alvinjaison/PyGo/internal/ast"
	"github.com/alvinjaison/PyGo/internal/lexer"
)

// Structural pattern matching (PEP 634).
//
// "match" and "case" are soft keywords: they lex as NAME tokens and keep their
// ordinary meaning everywhere except where a match statement can start.

// looksLikeMatch reports whether the current NAME token "match" begins a match
// statement rather than an expression such as `match(x)` or `match = 1`.
//
// A match statement is a logical line of the form `match <subject>:` followed
// by an indented block whose first token is the soft keyword "case".
func (p *Parser) looksLikeMatch() bool {
	if p.cur.Type != lexer.NAME || p.cur.Value != "match" {
		return false
	}
	// Find the end of the logical line (the lexer already joins continuation
	// lines and bracketed expressions).
	i := 1
	for {
		t := p.peek(i).Type
		if t == lexer.NEWLINE {
			break
		}
		if t == lexer.EOF || t == lexer.SEMICOLON {
			return false
		}
		i++
	}
	// The subject must be non-empty and the line must end with ':'.
	if i < 3 || p.peek(i-1).Type != lexer.COLON {
		return false
	}
	if p.peek(i+1).Type != lexer.INDENT {
		return false
	}
	next := p.peek(i + 2)
	return next.Type == lexer.NAME && next.Value == "case"
}

func (p *Parser) isCaseKeyword() bool {
	return p.cur.Type == lexer.NAME && p.cur.Value == "case"
}

// parseMatch parses a match statement. The current token is "match".
func (p *Parser) parseMatch() ast.Stmt {
	pos := p.pos()
	p.advance() // match
	subject := p.parseExprList()
	p.expect(lexer.COLON)
	p.expect(lexer.NEWLINE)
	p.expect(lexer.INDENT)

	var cases []*ast.MatchCase
	for len(p.errors) == 0 && p.cur.Type != lexer.DEDENT && p.cur.Type != lexer.EOF {
		if p.cur.Type == lexer.NEWLINE {
			p.advance()
			continue
		}
		if !p.isCaseKeyword() {
			p.errorf("expected 'case', got %s", p.cur.Type)
			break
		}
		cases = append(cases, p.parseCase())
	}
	if len(p.errors) > 0 {
		return ast.SetPos(&ast.Match{Subject: subject, Cases: cases}, pos)
	}
	if len(cases) == 0 {
		p.errorf("match statement must have at least one case")
	}
	p.expect(lexer.DEDENT)
	return ast.SetPos(&ast.Match{Subject: subject, Cases: cases}, pos)
}

func (p *Parser) parseCase() *ast.MatchCase {
	pos := p.pos()
	p.advance() // case
	pat := p.parsePatterns()
	var guard ast.Expr
	if p.accept(lexer.IF) {
		guard = p.parseExpr()
	}
	body := p.parseBlock()
	return ast.SetPos(&ast.MatchCase{Pattern: pat, Guard: guard, Body: body}, pos)
}

// parsePatterns parses the top-level pattern of a case clause, which may be an
// unparenthesised sequence: `case a, b:` or `case first, *rest:`.
func (p *Parser) parsePatterns() ast.Pattern {
	pos := p.pos()
	first := p.parseMaybeStarPattern()
	if !p.check(lexer.COMMA) {
		p.rejectStar(first)
		return first
	}
	items := []ast.Pattern{first}
	for p.accept(lexer.COMMA) {
		if p.check(lexer.COLON) || p.check(lexer.IF) {
			break
		}
		items = append(items, p.parseMaybeStarPattern())
		if len(p.errors) > 0 {
			break
		}
	}
	p.checkSingleStar(items)
	return ast.SetPos(&ast.MatchSequence{Patterns: items}, pos)
}

func (p *Parser) rejectStar(pat ast.Pattern) {
	if _, ok := pat.(*ast.MatchStar); ok {
		p.errorf("star pattern is only allowed inside a sequence pattern")
	}
}

func (p *Parser) checkSingleStar(items []ast.Pattern) {
	stars := 0
	for _, it := range items {
		if _, ok := it.(*ast.MatchStar); ok {
			stars++
		}
	}
	if stars > 1 {
		p.errorf("multiple starred names in sequence pattern")
	}
}

// parseMaybeStarPattern parses `*name` or an as-pattern.
func (p *Parser) parseMaybeStarPattern() ast.Pattern {
	if p.check(lexer.STAR) {
		pos := p.pos()
		p.advance()
		name := p.expect(lexer.NAME).Value
		if name == "_" {
			name = ""
		}
		return ast.SetPos(&ast.MatchStar{Name: name}, pos)
	}
	return p.parseAsPattern()
}

// parseAsPattern parses `or_pattern ['as' NAME]`.
func (p *Parser) parseAsPattern() ast.Pattern {
	pos := p.pos()
	pat := p.parseOrPattern()
	if p.accept(lexer.AS) {
		name := p.expect(lexer.NAME).Value
		if name == "_" {
			p.errorf("cannot use '_' as a target")
		}
		return ast.SetPos(&ast.MatchAs{Pattern: pat, Name: name}, pos)
	}
	return pat
}

// parseOrPattern parses `closed_pattern ('|' closed_pattern)*`.
func (p *Parser) parseOrPattern() ast.Pattern {
	pos := p.pos()
	first := p.parseClosedPattern()
	if !p.check(lexer.PIPE) {
		return first
	}
	alts := []ast.Pattern{first}
	for p.accept(lexer.PIPE) {
		alts = append(alts, p.parseClosedPattern())
		if len(p.errors) > 0 {
			break
		}
	}
	return ast.SetPos(&ast.MatchOr{Patterns: alts}, pos)
}

func (p *Parser) parseClosedPattern() ast.Pattern {
	pos := p.pos()
	switch p.cur.Type {
	case lexer.INTEGER, lexer.FLOAT, lexer.STRING, lexer.BYTES:
		return ast.SetPos(&ast.MatchValue{Value: p.parseAtom()}, pos)

	case lexer.MINUS:
		return ast.SetPos(&ast.MatchValue{Value: p.parseSignedNumber()}, pos)

	case lexer.NONE:
		p.advance()
		return ast.SetPos(&ast.MatchSingleton{Value: nil}, pos)
	case lexer.TRUE:
		p.advance()
		return ast.SetPos(&ast.MatchSingleton{Value: true}, pos)
	case lexer.FALSE:
		p.advance()
		return ast.SetPos(&ast.MatchSingleton{Value: false}, pos)

	case lexer.NAME:
		return p.parseNamePattern()

	case lexer.LPAREN:
		return p.parseParenPattern()

	case lexer.LBRACKET:
		p.advance()
		items := p.parsePatternList(lexer.RBRACKET)
		return ast.SetPos(&ast.MatchSequence{Patterns: items}, pos)

	case lexer.LBRACE:
		return p.parseMappingPattern()
	}
	p.errorf("invalid pattern, got %s", p.cur.Type)
	p.advance()
	return ast.SetPos(&ast.MatchAs{}, pos)
}

// parseSignedNumber parses `-NUMBER`.
func (p *Parser) parseSignedNumber() ast.Expr {
	pos := p.pos()
	p.expect(lexer.MINUS)
	if !p.check(lexer.INTEGER) && !p.check(lexer.FLOAT) {
		p.errorf("expected a number after '-' in pattern, got %s", p.cur.Type)
		return ast.SetPos(&ast.Constant{Value: nil}, pos)
	}
	operand := p.parseAtom()
	return ast.SetPos(&ast.UnaryOp{Op: ast.USub, Operand: operand}, pos)
}

// parseNamePattern handles a capture (x), wildcard (_), value (a.b) or class
// pattern (Name(...), a.b.Name(...)).
func (p *Parser) parseNamePattern() ast.Pattern {
	pos := p.pos()
	tok := p.advance()

	if !p.check(lexer.DOT) && !p.check(lexer.LPAREN) {
		if tok.Value == "_" {
			return ast.SetPos(&ast.MatchAs{}, pos)
		}
		return ast.SetPos(&ast.MatchAs{Name: tok.Value}, pos)
	}

	var expr ast.Expr = ast.SetPos(&ast.Name{Id: tok.Value, Ctx: ast.Load}, pos)
	dotted := false
	for p.accept(lexer.DOT) {
		attr := p.expect(lexer.NAME).Value
		expr = ast.SetPos(&ast.Attribute{Value: expr, Attr: attr, Ctx: ast.Load}, pos)
		dotted = true
		if len(p.errors) > 0 {
			break
		}
	}
	if p.check(lexer.LPAREN) {
		return p.parseClassPattern(expr, pos)
	}
	if !dotted {
		p.errorf("invalid pattern")
	}
	return ast.SetPos(&ast.MatchValue{Value: expr}, pos)
}

// parseClassPattern parses the argument list of `Cls(p1, p2, attr=p3)`.
func (p *Parser) parseClassPattern(cls ast.Expr, pos ast.Position) ast.Pattern {
	p.expect(lexer.LPAREN)
	node := &ast.MatchClass{Cls: cls}
	for len(p.errors) == 0 && !p.check(lexer.RPAREN) {
		if p.check(lexer.NAME) && p.peek(1).Type == lexer.EQUAL {
			attr := p.advance().Value
			p.advance() // =
			node.KwdAttrs = append(node.KwdAttrs, attr)
			node.KwdPatterns = append(node.KwdPatterns, p.parseAsPattern())
		} else {
			if len(node.KwdAttrs) > 0 {
				p.errorf("positional patterns follow keyword patterns")
				break
			}
			node.Patterns = append(node.Patterns, p.parseAsPattern())
		}
		if !p.accept(lexer.COMMA) {
			break
		}
	}
	p.expect(lexer.RPAREN)
	return ast.SetPos(node, pos)
}

// parseParenPattern parses `(pattern)` (a group) or a parenthesised sequence.
func (p *Parser) parseParenPattern() ast.Pattern {
	pos := p.pos()
	p.expect(lexer.LPAREN)
	if p.accept(lexer.RPAREN) {
		return ast.SetPos(&ast.MatchSequence{}, pos)
	}
	first := p.parseMaybeStarPattern()
	if p.check(lexer.COMMA) {
		items := []ast.Pattern{first}
		for p.accept(lexer.COMMA) {
			if p.check(lexer.RPAREN) {
				break
			}
			items = append(items, p.parseMaybeStarPattern())
			if len(p.errors) > 0 {
				break
			}
		}
		p.expect(lexer.RPAREN)
		p.checkSingleStar(items)
		return ast.SetPos(&ast.MatchSequence{Patterns: items}, pos)
	}
	p.expect(lexer.RPAREN)
	p.rejectStar(first)
	return first // a parenthesised group is just the inner pattern
}

// parsePatternList parses comma-separated maybe-star patterns up to closer
// (which is consumed).
func (p *Parser) parsePatternList(closer lexer.TokenType) []ast.Pattern {
	var items []ast.Pattern
	for len(p.errors) == 0 && !p.check(closer) {
		items = append(items, p.parseMaybeStarPattern())
		if !p.accept(lexer.COMMA) {
			break
		}
	}
	p.expect(closer)
	p.checkSingleStar(items)
	return items
}

// parseMappingPattern parses `{key: pattern, ..., **rest}`.
func (p *Parser) parseMappingPattern() ast.Pattern {
	pos := p.pos()
	p.expect(lexer.LBRACE)
	node := &ast.MatchMapping{}
	for len(p.errors) == 0 && !p.check(lexer.RBRACE) {
		if p.accept(lexer.DOUBLESTAR) {
			node.Rest = p.expect(lexer.NAME).Value
			p.accept(lexer.COMMA)
			break
		}
		key := p.parseMappingKey()
		p.expect(lexer.COLON)
		node.Keys = append(node.Keys, key)
		node.Patterns = append(node.Patterns, p.parseAsPattern())
		if !p.accept(lexer.COMMA) {
			break
		}
	}
	p.expect(lexer.RBRACE)
	return ast.SetPos(node, pos)
}

// parseMappingKey parses a literal or dotted-name key of a mapping pattern.
func (p *Parser) parseMappingKey() ast.Expr {
	pos := p.pos()
	switch p.cur.Type {
	case lexer.INTEGER, lexer.FLOAT, lexer.STRING, lexer.BYTES,
		lexer.NONE, lexer.TRUE, lexer.FALSE:
		return p.parseAtom()
	case lexer.MINUS:
		return p.parseSignedNumber()
	case lexer.NAME:
		var expr ast.Expr = ast.SetPos(&ast.Name{Id: p.advance().Value, Ctx: ast.Load}, pos)
		if !p.check(lexer.DOT) {
			p.errorf("mapping pattern keys must be literals or dotted names")
			return expr
		}
		for p.accept(lexer.DOT) {
			attr := p.expect(lexer.NAME).Value
			expr = ast.SetPos(&ast.Attribute{Value: expr, Attr: attr, Ctx: ast.Load}, pos)
			if len(p.errors) > 0 {
				break
			}
		}
		return expr
	}
	p.errorf("invalid mapping pattern key, got %s", p.cur.Type)
	p.advance()
	return ast.SetPos(&ast.Constant{Value: nil}, pos)
}
