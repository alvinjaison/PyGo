// Package parser implements a recursive-descent parser for Python 3 source.
//
// It consumes the token stream produced by internal/lexer and produces an
// ast.Module. Expression parsing uses Pratt (top-down operator precedence)
// parsing to handle Python's precedence table cleanly.
//
// The parser is the second stage of the PyGo pipeline:
//
//	tokens → Parser → ast.Module → Compiler
package parser

import (
	"fmt"

	"github.com/pygo/pygo/internal/ast"
	"github.com/pygo/pygo/internal/lexer"
)

// Error represents a syntax error with source position.
type Error struct {
	Filename string
	Line     int
	Col      int
	Message  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.Filename, e.Line, e.Col, e.Message)
}

// Parser holds the state of a single parse.
type Parser struct {
	filename string
	lex      *lexer.Lexer

	// A small ring of lookahead tokens. cur is the current token;
	// peeked holds tokens read ahead via peek().
	cur    lexer.Token
	peeked []lexer.Token

	errors []*Error
}

// New creates a parser for the given source.
func New(filename, source string) *Parser {
	p := &Parser{
		filename: filename,
		lex:      lexer.New(filename, source),
	}
	p.cur = p.lex.Next()
	return p
}

// Parse parses a complete module and returns it.
// If syntax errors were encountered, the first error is returned.
func (p *Parser) Parse() (*ast.Module, error) {
	mod := &ast.Module{}
	pos := p.pos()

	// Skip leading blank lines.
	for p.cur.Type == lexer.NEWLINE {
		p.advance()
	}

	for p.cur.Type != lexer.EOF {
		if p.cur.Type == lexer.NEWLINE {
			p.advance()
			continue
		}
		stmt := p.parseStatement()
		if stmt != nil {
			mod.Body = append(mod.Body, stmt)
		}
		if len(p.errors) > 0 {
			// Stop at the first error for now (Phase 1).
			return nil, p.errors[0]
		}
	}

	ast.SetPos(mod, pos)
	return mod, nil
}

// ParseExpression parses a single expression (used by Eval).
func (p *Parser) ParseExpression() (ast.Expr, error) {
	for p.cur.Type == lexer.NEWLINE {
		p.advance()
	}
	expr := p.parseExpr()
	if len(p.errors) > 0 {
		return nil, p.errors[0]
	}
	return expr, nil
}

// ============================================================
// Token stream helpers
// ============================================================

func (p *Parser) advance() lexer.Token {
	prev := p.cur
	if len(p.peeked) > 0 {
		p.cur = p.peeked[0]
		p.peeked = p.peeked[1:]
	} else {
		p.cur = p.lex.Next()
	}
	return prev
}

// peek returns the token n positions ahead (peek(0) == cur).
func (p *Parser) peek(n int) lexer.Token {
	if n == 0 {
		return p.cur
	}
	for len(p.peeked) < n {
		p.peeked = append(p.peeked, p.lex.Next())
	}
	return p.peeked[n-1]
}

func (p *Parser) check(tt lexer.TokenType) bool {
	return p.cur.Type == tt
}

func (p *Parser) accept(tt lexer.TokenType) bool {
	if p.cur.Type == tt {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) expect(tt lexer.TokenType) lexer.Token {
	if p.cur.Type == tt {
		return p.advance()
	}
	p.errorf("expected %s, got %s", tt, p.cur.Type)
	return p.cur
}

func (p *Parser) errorf(format string, args ...interface{}) {
	p.errors = append(p.errors, &Error{
		Filename: p.filename,
		Line:     p.cur.Line,
		Col:      p.cur.Col,
		Message:  fmt.Sprintf(format, args...),
	})
}

func (p *Parser) pos() ast.Position {
	return ast.NewPos(p.filename, p.cur.Line, p.cur.Col, p.cur.EndLine, p.cur.EndCol)
}

// consumeNewline consumes a statement-terminating NEWLINE (or EOF / DEDENT).
func (p *Parser) consumeNewline() {
	switch p.cur.Type {
	case lexer.NEWLINE:
		p.advance()
	case lexer.SEMICOLON:
		p.advance()
	case lexer.EOF, lexer.DEDENT:
		// acceptable terminators
	default:
		p.errorf("expected newline, got %s", p.cur.Type)
	}
}
