// Package parser transforms Python 3 source into a typed AST using
// recursive descent with Pratt operator-precedence parsing for expressions.
package parser

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/alvinjaison/pygo/internal/ast"
	"github.com/alvinjaison/pygo/internal/lexer"
)

// Precedence levels for Pratt parsing (lowest to highest).
const (
	precNone       int = iota
	precOr             // or
	precAnd            // and
	precNot            // not (prefix)
	precComparison     // == != < > <= >= in not-in is is-not
	precBitOr          // |
	precBitXor         // ^
	precBitAnd         // &
	precShift          // << >>
	precAddSub         // + -
	precMulDiv         // * / // % @
	precUnary          // unary - + ~
	precPower          // **
	precPostfix        // calls, subscripts, attributes
	precAtom           // atoms
)

// SyntaxError represents a parsing error with source location.
type SyntaxError struct {
	Filename string
	Line     int
	Col      int
	Msg      string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("%s:%d:%d: SyntaxError: %s", e.Filename, e.Line, e.Col, e.Msg)
}

// parser holds state during parsing.
type parser struct {
	lex      *lexer.Lexer
	filename string
	cur      lexer.Token
	prev     lexer.Token
}

// Parse parses Python source and returns an AST Module.
func Parse(source, filename string) (*ast.Module, error) {
	p := &parser{
		lex:      lexer.New(filename, source),
		filename: filename,
	}
	p.advance() // prime the first token
	return p.parseModule()
}

func (p *parser) advance() {
	p.prev = p.cur
	p.cur = p.lex.Next()
}

func (p *parser) peek() lexer.Token {
	return p.lex.Peek()
}

func (p *parser) at(tt lexer.TokenType) bool {
	return p.cur.Type == tt
}

func (p *parser) match(tt lexer.TokenType) bool {
	if p.cur.Type == tt {
		p.advance()
		return true
	}
	return false
}

func (p *parser) expect(tt lexer.TokenType) (lexer.Token, error) {
	if p.cur.Type == tt {
		tok := p.cur
		p.advance()
		return tok, nil
	}
	return lexer.Token{}, p.errorf("expected %s, got %s", tt, p.cur.Type)
}

func (p *parser) errorf(format string, args ...interface{}) error {
	return &SyntaxError{
		Filename: p.filename,
		Line:     p.cur.Pos.Line,
		Col:      p.cur.Pos.Column,
		Msg:      fmt.Sprintf(format, args...),
	}
}

func (p *parser) pos() ast.Position {
	return ast.NewPos(p.filename, p.cur.Pos.Line, p.cur.Pos.Column, 0, 0)
}

func (p *parser) tokPos(tok lexer.Token) ast.Position {
	return ast.NewPos(p.filename, tok.Pos.Line, tok.Pos.Column, 0, 0)
}

// skipNewlines consumes any NEWLINE tokens.
func (p *parser) skipNewlines() {
	for p.cur.Type == lexer.NEWLINE {
		p.advance()
	}
}

// parseModule parses a complete module (sequence of statements).
func (p *parser) parseModule() (*ast.Module, error) {
	modPos := p.pos()
	var body []ast.Stmt

	p.skipNewlines()
	for p.cur.Type != lexer.EOF {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		body = append(body, stmt)
		p.skipNewlines()
	}
	return ast.NewModule(body, modPos), nil
}

// parseStatement dispatches to compound or simple statement parsing.
func (p *parser) parseStatement() (ast.Stmt, error) {
	switch p.cur.Type {
	case lexer.AT:
		return p.parseDecorated()
	case lexer.DEF:
		return p.parseFunctionDef()
	case lexer.IF:
		return p.parseIfStmt()
	case lexer.WHILE:
		return p.parseWhileStmt()
	case lexer.FOR:
		return p.parseForStmt()
	case lexer.TRY:
		return p.parseTryStmt()
	default:
		return p.parseSimpleStmt()
	}
}

// parseSimpleStmt parses simple (non-compound) statements.
func (p *parser) parseSimpleStmt() (ast.Stmt, error) {
	var stmt ast.Stmt
	var err error

	switch p.cur.Type {
	case lexer.RETURN:
		stmt, err = p.parseReturnStmt()
	case lexer.PASS:
		stmt, err = p.parsePassStmt()
	case lexer.BREAK:
		stmt, err = p.parseBreakStmt()
	case lexer.CONTINUE:
		stmt, err = p.parseContinueStmt()
	case lexer.RAISE:
		stmt, err = p.parseRaiseStmt()
	case lexer.IMPORT:
		stmt, err = p.parseImportStmt()
	case lexer.FROM:
		stmt, err = p.parseFromImportStmt()
	case lexer.DEL:
		stmt, err = p.parseDelStmt()
	case lexer.GLOBAL:
		stmt, err = p.parseGlobalStmt()
	case lexer.NONLOCAL:
		stmt, err = p.parseNonlocalStmt()
	default:
		stmt, err = p.parseExprOrAssignStmt()
	}
	if err != nil {
		return nil, err
	}
	p.consumeStmtEnd()
	return stmt, nil
}

// parseExprOrAssignStmt parses expression statements, assignments, and augmented assignments.
func (p *parser) parseExprOrAssignStmt() (ast.Stmt, error) {
	pos := p.pos()
	expr, err := p.parseExprList()
	if err != nil {
		return nil, err
	}

	// Check for augmented assignment: target op= value
	if op, ok := p.augAssignOp(); ok {
		p.advance()
		value, err := p.parseExprList()
		if err != nil {
			return nil, err
		}
		return ast.NewAugAssign(expr, op, value, pos), nil
	}

	// Check for assignment: target = value (possibly chained: a = b = value)
	if p.at(lexer.EQUAL) {
		targets := []ast.Expr{expr}
		for p.at(lexer.EQUAL) {
			p.advance()
			next, err := p.parseExprList()
			if err != nil {
				return nil, err
			}
			targets = append(targets, next)
		}
		// The last expression is the value; everything before it is a target
		value := targets[len(targets)-1]
		targets = targets[:len(targets)-1]
		return ast.NewAssign(targets, value, pos), nil
	}

	return ast.NewExprStmt(expr, pos), nil
}

// augAssignOp returns the operator and true if the current token is an augmented assignment operator.
func (p *parser) augAssignOp() (ast.Operator, bool) {
	switch p.cur.Type {
	case lexer.PLUSEQUAL:
		return ast.Add, true
	case lexer.MINUSEQUAL:
		return ast.Sub, true
	case lexer.STAREQUAL:
		return ast.Mult, true
	case lexer.SLASHEQUAL:
		return ast.Div, true
	case lexer.DOUBLESLASHEQUAL:
		return ast.FloorDiv, true
	case lexer.PERCENTEQUAL:
		return ast.Mod, true
	case lexer.DOUBLESTAREQUAL:
		return ast.Pow, true
	}
	return 0, false
}

// parseDecorated parses one or more decorator lines followed by the def (or
// class) they decorate:
//
//	'@' expression NEWLINE ... (funcdef | classdef)
//
// The decorators are kept in source order (topmost first); the compiler applies
// them bottom-up.
func (p *parser) parseDecorated() (ast.Stmt, error) {
	var decorators []ast.Expr

	for p.at(lexer.AT) {
		p.advance() // consume '@'

		// The decorator is an arbitrary expression, which covers plain names,
		// dotted paths (@a.b.c) and calls (@dec(arg)).
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		decorators = append(decorators, expr)

		if _, err := p.expect(lexer.NEWLINE); err != nil {
			return nil, err
		}
		p.skipNewlines()
	}

	switch p.cur.Type {
	case lexer.DEF:
		return p.parseFunctionDefWithDecorators(decorators)
	case lexer.CLASS:
		// Class statements are not parsed yet (see spec task 9). The AST and the
		// compiler already carry decorators for ClassDef, so decorated classes
		// will work as soon as class parsing lands.
		return nil, p.errorf("class definitions are not supported yet")
	default:
		return nil, p.errorf("expected 'def' or 'class' after decorator")
	}
}

// parseFunctionDef parses: def name(params): body
func (p *parser) parseFunctionDef() (ast.Stmt, error) {
	return p.parseFunctionDefWithDecorators(nil)
}

// parseFunctionDefWithDecorators parses a def, attaching any decorators that
// preceded it.
func (p *parser) parseFunctionDefWithDecorators(decorators []ast.Expr) (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'def'

	if !p.at(lexer.NAME) {
		return nil, p.errorf("expected function name after 'def'")
	}
	name := p.cur.Value
	p.advance()

	if _, err := p.expect(lexer.LPAREN); err != nil {
		return nil, err
	}
	args, err := p.parseParameters()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.RPAREN); err != nil {
		return nil, err
	}

	if _, err := p.expect(lexer.COLON); err != nil {
		return nil, err
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	return ast.NewFunctionDef(name, args, body, decorators, nil, pos), nil
}

// parseParameters parses function parameter list including *args, **kwargs,
// keyword-only parameters (after * or *args), and bare * separator.
//
// Grammar:
//   params: param (',' param)* [',' ['*' [param] (',' param)* [',' '**' param] | '**' param]]
//         | '*' [param] (',' param)* [',' '**' param]
//         | '**' param
func (p *parser) parseParameters() (*ast.Arguments, error) {
	args := &ast.Arguments{}
	if p.at(lexer.RPAREN) {
		return args, nil
	}

	seenStar := false    // after * or *args
	seenDefault := false // after first positional default

	for {
		if p.at(lexer.RPAREN) {
			break
		}

		// Handle **kwargs
		if p.at(lexer.DOUBLESTAR) {
			p.advance()
			if !p.at(lexer.NAME) {
				return nil, p.errorf("expected parameter name after '**'")
			}
			paramPos := p.pos()
			paramName := p.cur.Value
			p.advance()
			args.Kwarg = ast.NewArg(paramName, nil, paramPos)
			// **kwargs must be last; allow trailing comma
			if p.match(lexer.COMMA) {
				// Allow trailing comma before )
			}
			break
		}

		// Handle *args or bare *
		if p.at(lexer.STAR) {
			if seenStar {
				return nil, p.errorf("duplicate '*' in parameter list")
			}
			p.advance()
			seenStar = true

			if p.at(lexer.NAME) {
				// *args
				paramPos := p.pos()
				paramName := p.cur.Value
				p.advance()
				args.Vararg = ast.NewArg(paramName, nil, paramPos)
			}
			// If no NAME follows, it's bare * (keyword-only separator)

			if !p.match(lexer.COMMA) {
				break
			}
			if p.at(lexer.RPAREN) {
				break
			}
			continue
		}

		// Regular parameter name
		if !p.at(lexer.NAME) {
			return nil, p.errorf("expected parameter name")
		}
		paramPos := p.pos()
		paramName := p.cur.Value
		p.advance()

		arg := ast.NewArg(paramName, nil, paramPos)

		if seenStar {
			// After * or *args: keyword-only parameter
			args.KwonlyArgs = append(args.KwonlyArgs, arg)
			if p.match(lexer.EQUAL) {
				def, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				args.KwDefaults = append(args.KwDefaults, def)
			} else {
				// No default: append nil placeholder (required kw-only param)
				args.KwDefaults = append(args.KwDefaults, nil)
			}
		} else {
			// Regular positional parameter
			args.Args = append(args.Args, arg)
			if p.match(lexer.EQUAL) {
				def, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				args.Defaults = append(args.Defaults, def)
				seenDefault = true
			} else if seenDefault {
				return nil, p.errorf("non-default argument follows default argument")
			}
		}

		if !p.match(lexer.COMMA) {
			break
		}
		// Allow trailing comma before )
		if p.at(lexer.RPAREN) {
			break
		}
	}
	return args, nil
}

// parseBlock parses either:
//   - An indented block: NEWLINE INDENT stmts DEDENT
//   - A simple suite: stmt (';' stmt)* NEWLINE (body on same line as colon)
func (p *parser) parseBlock() ([]ast.Stmt, error) {
	// Simple suite: if the next token is NOT a NEWLINE, the body is on the same line.
	// Example: "def f(x): return x * 2"
	if !p.at(lexer.NEWLINE) {
		return p.parseSimpleSuite()
	}

	if _, err := p.expect(lexer.NEWLINE); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.INDENT); err != nil {
		return nil, err
	}

	var body []ast.Stmt
	for !p.at(lexer.DEDENT) && !p.at(lexer.EOF) {
		p.skipNewlines()
		if p.at(lexer.DEDENT) || p.at(lexer.EOF) {
			break
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		body = append(body, stmt)
		p.skipNewlines()
	}

	if _, err := p.expect(lexer.DEDENT); err != nil {
		return nil, err
	}
	return body, nil
}

// parseSimpleSuite parses one or more simple statements on the same line,
// separated by semicolons, terminated by NEWLINE or EOF.
// Example: "return x" or "x = 1; y = 2"
func (p *parser) parseSimpleSuite() ([]ast.Stmt, error) {
	var stmts []ast.Stmt
	for {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, stmt)

		// Check for semicolon to continue parsing more statements.
		if !p.match(lexer.SEMICOLON) {
			break
		}
		// If after the semicolon we hit NEWLINE/EOF, stop.
		if p.at(lexer.NEWLINE) || p.at(lexer.EOF) {
			break
		}
	}
	// Consume the trailing NEWLINE if present.
	p.match(lexer.NEWLINE)
	return stmts, nil
}

// parseReturnStmt parses: return [expr_list]
func (p *parser) parseReturnStmt() (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'return'

	// Check if there's a value to return
	if p.at(lexer.NEWLINE) || p.at(lexer.EOF) || p.at(lexer.SEMICOLON) || p.at(lexer.DEDENT) {
		return ast.NewReturn(nil, pos), nil
	}

	value, err := p.parseExprList()
	if err != nil {
		return nil, err
	}
	return ast.NewReturn(value, pos), nil
}

// parseIfStmt parses: if test: body [elif test: body]* [else: body]
func (p *parser) parseIfStmt() (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'if'

	test, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.COLON); err != nil {
		return nil, err
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	var orelse []ast.Stmt
	p.skipNewlines()
	if p.at(lexer.ELIF) {
		// elif becomes a nested If in Orelse
		elifStmt, err := p.parseElifStmt()
		if err != nil {
			return nil, err
		}
		orelse = []ast.Stmt{elifStmt}
	} else if p.at(lexer.ELSE) {
		p.advance() // consume 'else'
		if _, err := p.expect(lexer.COLON); err != nil {
			return nil, err
		}
		orelse, err = p.parseBlock()
		if err != nil {
			return nil, err
		}
	}

	return ast.NewIf(test, body, orelse, pos), nil
}

// parseElifStmt parses: elif test: body [elif ...]* [else: body]
func (p *parser) parseElifStmt() (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'elif'

	test, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.COLON); err != nil {
		return nil, err
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	var orelse []ast.Stmt
	p.skipNewlines()
	if p.at(lexer.ELIF) {
		elifStmt, err := p.parseElifStmt()
		if err != nil {
			return nil, err
		}
		orelse = []ast.Stmt{elifStmt}
	} else if p.at(lexer.ELSE) {
		p.advance() // consume 'else'
		if _, err := p.expect(lexer.COLON); err != nil {
			return nil, err
		}
		orelse, err = p.parseBlock()
		if err != nil {
			return nil, err
		}
	}

	return ast.NewIf(test, body, orelse, pos), nil
}

// parseWhileStmt parses: while test: body [else: body]
func (p *parser) parseWhileStmt() (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'while'

	test, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.COLON); err != nil {
		return nil, err
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	var orelse []ast.Stmt
	p.skipNewlines()
	if p.at(lexer.ELSE) {
		p.advance() // consume 'else'
		if _, err := p.expect(lexer.COLON); err != nil {
			return nil, err
		}
		orelse, err = p.parseBlock()
		if err != nil {
			return nil, err
		}
	}

	return ast.NewWhile(test, body, orelse, pos), nil
}

// parseForStmt parses: for target in iter: body [else: body]
func (p *parser) parseForStmt() (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'for'

	target, err := p.parseTargetList()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.IN); err != nil {
		return nil, err
	}
	iter, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.COLON); err != nil {
		return nil, err
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	var orelse []ast.Stmt
	p.skipNewlines()
	if p.at(lexer.ELSE) {
		p.advance() // consume 'else'
		if _, err := p.expect(lexer.COLON); err != nil {
			return nil, err
		}
		orelse, err = p.parseBlock()
		if err != nil {
			return nil, err
		}
	}

	return ast.NewFor(target, iter, body, orelse, pos), nil
}

// parseTargetList parses assignment targets for 'for' loops.
// It parses simple names, starred targets (*x), or tuple targets (a, b)
// stopping before 'in'.
func (p *parser) parseTargetList() (ast.Expr, error) {
	pos := p.pos()
	first, err := p.parsePrimaryTargetOrStarred()
	if err != nil {
		return nil, err
	}

	// Check for tuple target: a, b, c
	if p.at(lexer.COMMA) {
		elts := []ast.Expr{first}
		for p.match(lexer.COMMA) {
			if p.at(lexer.IN) {
				break
			}
			elt, err := p.parsePrimaryTargetOrStarred()
			if err != nil {
				return nil, err
			}
			elts = append(elts, elt)
		}
		return ast.NewTuple(elts, ast.Store, pos), nil
	}
	return first, nil
}

// parsePrimaryTargetOrStarred parses a single target or a starred target (*name).
func (p *parser) parsePrimaryTargetOrStarred() (ast.Expr, error) {
	if p.at(lexer.STAR) {
		pos := p.pos()
		p.advance()
		target, err := p.parsePrimaryTarget()
		if err != nil {
			return nil, err
		}
		return ast.NewStarred(target, ast.Store, pos), nil
	}
	return p.parsePrimaryTarget()
}

// parsePrimaryTarget parses a single assignment target (name, attribute, or subscript).
func (p *parser) parsePrimaryTarget() (ast.Expr, error) {
	pos := p.pos()
	if !p.at(lexer.NAME) && !p.at(lexer.LPAREN) {
		return nil, p.errorf("expected target name")
	}

	if p.at(lexer.LPAREN) {
		// Parenthesized target list
		p.advance()
		target, err := p.parseTargetList()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.RPAREN); err != nil {
			return nil, err
		}
		return target, nil
	}

	// Parse name and any attribute/subscript postfix
	name := p.cur.Value
	p.advance()
	var expr ast.Expr = ast.NewName(name, ast.Store, pos)

	for {
		switch p.cur.Type {
		case lexer.DOT:
			attrPos := p.pos()
			p.advance()
			if !p.at(lexer.NAME) {
				return nil, p.errorf("expected attribute name after '.'")
			}
			attr := p.cur.Value
			p.advance()
			expr = ast.NewAttribute(expr, attr, ast.Store, attrPos)
		case lexer.LBRACKET:
			subPos := p.pos()
			p.advance()
			slice, err := p.parseSubscriptSlice()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.RBRACKET); err != nil {
				return nil, err
			}
			expr = ast.NewSubscript(expr, slice, ast.Store, subPos)
		default:
			return expr, nil
		}
	}
}

// parseTryStmt parses: try: body except [type [as name]]: body [else: body] [finally: body]
func (p *parser) parseTryStmt() (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'try'

	if _, err := p.expect(lexer.COLON); err != nil {
		return nil, err
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	var handlers []*ast.ExceptHandler
	var orelse []ast.Stmt
	var finalbody []ast.Stmt

	p.skipNewlines()
	for p.at(lexer.EXCEPT) {
		handler, err := p.parseExceptHandler()
		if err != nil {
			return nil, err
		}
		handlers = append(handlers, handler)
		p.skipNewlines()
	}

	if p.at(lexer.ELSE) {
		p.advance() // consume 'else'
		if _, err := p.expect(lexer.COLON); err != nil {
			return nil, err
		}
		orelse, err = p.parseBlock()
		if err != nil {
			return nil, err
		}
		p.skipNewlines()
	}

	if p.at(lexer.FINALLY) {
		p.advance() // consume 'finally'
		if _, err := p.expect(lexer.COLON); err != nil {
			return nil, err
		}
		finalbody, err = p.parseBlock()
		if err != nil {
			return nil, err
		}
	}

	return ast.NewTry(body, handlers, orelse, finalbody, pos), nil
}

// parseExceptHandler parses: except [type [as name]]: body
func (p *parser) parseExceptHandler() (*ast.ExceptHandler, error) {
	pos := p.pos()
	p.advance() // consume 'except'

	var typ ast.Expr
	var name string

	// Check for bare except:
	if !p.at(lexer.COLON) {
		var err error
		typ, err = p.parseExpr()
		if err != nil {
			return nil, err
		}
		// Check for 'as name'
		if p.match(lexer.AS) {
			if !p.at(lexer.NAME) {
				return nil, p.errorf("expected name after 'as'")
			}
			name = p.cur.Value
			p.advance()
		}
	}

	if _, err := p.expect(lexer.COLON); err != nil {
		return nil, err
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	return ast.NewExceptHandler(typ, name, body, pos), nil
}

// parsePassStmt parses: pass
func (p *parser) parsePassStmt() (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'pass'
	return ast.NewPass(pos), nil
}

// parseBreakStmt parses: break
func (p *parser) parseBreakStmt() (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'break'
	return ast.NewBreak(pos), nil
}

// parseContinueStmt parses: continue
func (p *parser) parseContinueStmt() (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'continue'
	return ast.NewContinue(pos), nil
}

// parseRaiseStmt parses: raise [expr [from expr]]
func (p *parser) parseRaiseStmt() (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'raise'

	// Bare raise
	if p.at(lexer.NEWLINE) || p.at(lexer.EOF) || p.at(lexer.SEMICOLON) || p.at(lexer.DEDENT) {
		return ast.NewRaise(nil, nil, pos), nil
	}

	exc, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	var cause ast.Expr
	if p.match(lexer.FROM) {
		cause, err = p.parseExpr()
		if err != nil {
			return nil, err
		}
	}

	return ast.NewRaise(exc, cause, pos), nil
}

// parseImportStmt parses: import module [as alias], ...
func (p *parser) parseImportStmt() (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'import'

	var names []*ast.Alias
	for {
		alias, err := p.parseDottedName()
		if err != nil {
			return nil, err
		}
		if p.match(lexer.AS) {
			if !p.at(lexer.NAME) {
				return nil, p.errorf("expected name after 'as'")
			}
			alias.AsName = p.cur.Value
			p.advance()
		}
		names = append(names, alias)
		if !p.match(lexer.COMMA) {
			break
		}
	}
	return ast.NewImport(names, pos), nil
}

// parseDottedName parses a dotted module name like "os.path".
func (p *parser) parseDottedName() (*ast.Alias, error) {
	if !p.at(lexer.NAME) {
		return nil, p.errorf("expected module name")
	}
	name := p.cur.Value
	p.advance()
	for p.match(lexer.DOT) {
		if !p.at(lexer.NAME) {
			return nil, p.errorf("expected name after '.'")
		}
		name += "." + p.cur.Value
		p.advance()
	}
	return &ast.Alias{Name: name}, nil
}

// parseFromImportStmt parses: from module import name [as alias], ...
func (p *parser) parseFromImportStmt() (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'from'

	// Count leading dots for relative imports
	level := 0
	for p.at(lexer.DOT) {
		level++
		p.advance()
	}

	// Module name (may be absent for relative-only: from . import x)
	var module string
	if p.at(lexer.NAME) {
		alias, err := p.parseDottedName()
		if err != nil {
			return nil, err
		}
		module = alias.Name
	}

	if _, err := p.expect(lexer.IMPORT); err != nil {
		return nil, err
	}

	var names []*ast.Alias

	// from module import *
	if p.at(lexer.STAR) {
		p.advance()
		names = append(names, &ast.Alias{Name: "*"})
	} else {
		for {
			if !p.at(lexer.NAME) {
				return nil, p.errorf("expected name to import")
			}
			alias := &ast.Alias{Name: p.cur.Value}
			p.advance()
			if p.match(lexer.AS) {
				if !p.at(lexer.NAME) {
					return nil, p.errorf("expected name after 'as'")
				}
				alias.AsName = p.cur.Value
				p.advance()
			}
			names = append(names, alias)
			if !p.match(lexer.COMMA) {
				break
			}
		}
	}

	return ast.NewImportFrom(module, names, level, pos), nil
}

// parseDelStmt parses: del target, target, ...
func (p *parser) parseDelStmt() (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'del'

	var targets []ast.Expr
	for {
		target, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
		if !p.match(lexer.COMMA) {
			break
		}
	}
	return ast.NewDelete(targets, pos), nil
}

// parseGlobalStmt parses: global name, name, ...
func (p *parser) parseGlobalStmt() (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'global'

	var names []string
	for {
		if !p.at(lexer.NAME) {
			return nil, p.errorf("expected name in global statement")
		}
		names = append(names, p.cur.Value)
		p.advance()
		if !p.match(lexer.COMMA) {
			break
		}
	}
	return ast.NewGlobal(names, pos), nil
}

// parseNonlocalStmt parses: nonlocal name, name, ...
func (p *parser) parseNonlocalStmt() (ast.Stmt, error) {
	pos := p.pos()
	p.advance() // consume 'nonlocal'

	var names []string
	for {
		if !p.at(lexer.NAME) {
			return nil, p.errorf("expected name in nonlocal statement")
		}
		names = append(names, p.cur.Value)
		p.advance()
		if !p.match(lexer.COMMA) {
			break
		}
	}
	return ast.NewNonlocal(names, pos), nil
}

func (p *parser) consumeStmtEnd() {
	if p.cur.Type == lexer.NEWLINE || p.cur.Type == lexer.SEMICOLON {
		p.advance()
	}
}

// parseStarExprOrExpr parses either a starred expression (*expr) or a regular
// expression. This is used in contexts where starred assignment targets are
// allowed (expression lists that may become assignment targets).
func (p *parser) parseStarExprOrExpr() (ast.Expr, error) {
	if p.at(lexer.STAR) {
		pos := p.pos()
		p.advance()
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return ast.NewStarred(val, ast.Store, pos), nil
	}
	return p.parseExpr()
}

// parseExprList parses a comma-separated expression list, producing a Tuple
// node when multiple expressions are found. This handles Python's implicit
// tuple creation: `return a, b`, `x = 1, 2, 3`, `a, b = expr`.
// It also handles starred expressions (*x) for starred assignment targets.
// The stopTokens set controls which tokens prevent consuming a comma
// (to avoid eating commas that belong to an enclosing construct).
func (p *parser) parseExprList() (ast.Expr, error) {
	first, err := p.parseStarExprOrExpr()
	if err != nil {
		return nil, err
	}

	// If no comma follows, it's a single expression.
	if !p.at(lexer.COMMA) {
		return first, nil
	}

	// Comma-separated → build a Tuple.
	pos := first.Pos()
	elts := []ast.Expr{first}
	for p.match(lexer.COMMA) {
		// Allow trailing comma: stop if we hit a statement terminator or
		// assignment operator.
		if p.at(lexer.NEWLINE) || p.at(lexer.EOF) || p.at(lexer.SEMICOLON) ||
			p.at(lexer.COLON) || p.at(lexer.RPAREN) || p.at(lexer.RBRACKET) ||
			p.at(lexer.RBRACE) || p.at(lexer.EQUAL) || p.at(lexer.DEDENT) {
			break
		}
		// Stop if an augmented assignment follows.
		if _, ok := p.augAssignOp(); ok {
			break
		}
		elt, err := p.parseStarExprOrExpr()
		if err != nil {
			return nil, err
		}
		elts = append(elts, elt)
	}

	// Single element with trailing comma is still a 1-tuple: (x,) → Tuple{x}
	return ast.NewTuple(elts, ast.Load, pos), nil
}

// parseExpr is the entry point for expression parsing.
// It handles the lowest precedence: ternary if-else.
func (p *parser) parseExpr() (ast.Expr, error) {
	expr, err := p.parseTernary()
	if err != nil {
		return nil, err
	}
	return expr, nil
}

// parseTernary handles: expr if cond else expr
func (p *parser) parseTernary() (ast.Expr, error) {
	body, err := p.parseOrExpr()
	if err != nil {
		return nil, err
	}

	if !p.at(lexer.IF) {
		return body, nil
	}

	pos := p.pos()
	p.advance() // consume 'if'
	test, err := p.parseOrExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.ELSE); err != nil {
		return nil, err
	}
	orelse, err := p.parseExpr() // right-associative
	if err != nil {
		return nil, err
	}
	return ast.NewIfExp(test, body, orelse, pos), nil
}

// parseOrExpr handles: expr or expr or ...
func (p *parser) parseOrExpr() (ast.Expr, error) {
	left, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}

	if !p.at(lexer.OR) {
		return left, nil
	}

	pos := left.Pos()
	values := []ast.Expr{left}
	for p.match(lexer.OR) {
		right, err := p.parseAndExpr()
		if err != nil {
			return nil, err
		}
		values = append(values, right)
	}
	return ast.NewBoolOp(ast.Or, values, pos), nil
}

// parseAndExpr handles: expr and expr and ...
func (p *parser) parseAndExpr() (ast.Expr, error) {
	left, err := p.parseNotExpr()
	if err != nil {
		return nil, err
	}

	if !p.at(lexer.AND) {
		return left, nil
	}

	pos := left.Pos()
	values := []ast.Expr{left}
	for p.match(lexer.AND) {
		right, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}
		values = append(values, right)
	}
	return ast.NewBoolOp(ast.And, values, pos), nil
}

// parseNotExpr handles: not expr
func (p *parser) parseNotExpr() (ast.Expr, error) {
	if p.at(lexer.NOT) {
		pos := p.pos()
		p.advance()
		operand, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}
		return ast.NewUnaryOp(ast.Not, operand, pos), nil
	}
	return p.parseComparison()
}

// parseComparison handles chained comparisons: a < b < c
func (p *parser) parseComparison() (ast.Expr, error) {
	left, err := p.parseBitOr()
	if err != nil {
		return nil, err
	}

	if !p.isComparisonOp() {
		return left, nil
	}

	pos := left.Pos()
	var ops []ast.CmpOp
	var comparators []ast.Expr

	for p.isComparisonOp() {
		op, err := p.parseCompOp()
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
		right, err := p.parseBitOr()
		if err != nil {
			return nil, err
		}
		comparators = append(comparators, right)
	}
	return ast.NewCompare(left, ops, comparators, pos), nil
}

func (p *parser) isComparisonOp() bool {
	switch p.cur.Type {
	case lexer.EQEQUAL, lexer.NOTEQUAL, lexer.LESS, lexer.GREATER,
		lexer.LESSEQUAL, lexer.GREATEREQUAL, lexer.IN, lexer.IS:
		return true
	case lexer.NOT:
		// "not in"
		return p.peek().Type == lexer.IN
	}
	return false
}

func (p *parser) parseCompOp() (ast.CmpOp, error) {
	switch p.cur.Type {
	case lexer.EQEQUAL:
		p.advance()
		return ast.Eq, nil
	case lexer.NOTEQUAL:
		p.advance()
		return ast.NotEq, nil
	case lexer.LESS:
		p.advance()
		return ast.Lt, nil
	case lexer.GREATER:
		p.advance()
		return ast.Gt, nil
	case lexer.LESSEQUAL:
		p.advance()
		return ast.LtE, nil
	case lexer.GREATEREQUAL:
		p.advance()
		return ast.GtE, nil
	case lexer.IN:
		p.advance()
		return ast.In, nil
	case lexer.IS:
		p.advance()
		if p.match(lexer.NOT) {
			return ast.IsNot, nil
		}
		return ast.Is, nil
	case lexer.NOT:
		// "not in"
		p.advance()
		if _, err := p.expect(lexer.IN); err != nil {
			return 0, err
		}
		return ast.NotIn, nil
	}
	return 0, p.errorf("expected comparison operator")
}

// parseBitOr handles: expr | expr
func (p *parser) parseBitOr() (ast.Expr, error) {
	left, err := p.parseBitXor()
	if err != nil {
		return nil, err
	}
	for p.at(lexer.PIPE) {
		pos := p.pos()
		p.advance()
		right, err := p.parseBitXor()
		if err != nil {
			return nil, err
		}
		left = ast.NewBinOp(left, ast.BitOr, right, pos)
	}
	return left, nil
}

// parseBitXor handles: expr ^ expr
func (p *parser) parseBitXor() (ast.Expr, error) {
	left, err := p.parseBitAnd()
	if err != nil {
		return nil, err
	}
	for p.at(lexer.CARET) {
		pos := p.pos()
		p.advance()
		right, err := p.parseBitAnd()
		if err != nil {
			return nil, err
		}
		left = ast.NewBinOp(left, ast.BitXor, right, pos)
	}
	return left, nil
}

// parseBitAnd handles: expr & expr
func (p *parser) parseBitAnd() (ast.Expr, error) {
	left, err := p.parseShift()
	if err != nil {
		return nil, err
	}
	for p.at(lexer.AMPER) {
		pos := p.pos()
		p.advance()
		right, err := p.parseShift()
		if err != nil {
			return nil, err
		}
		left = ast.NewBinOp(left, ast.BitAnd, right, pos)
	}
	return left, nil
}

// parseShift handles: expr << expr, expr >> expr
func (p *parser) parseShift() (ast.Expr, error) {
	left, err := p.parseAddSub()
	if err != nil {
		return nil, err
	}
	for p.at(lexer.LSHIFT) || p.at(lexer.RSHIFT) {
		pos := p.pos()
		var op ast.Operator
		if p.cur.Type == lexer.LSHIFT {
			op = ast.LShift
		} else {
			op = ast.RShift
		}
		p.advance()
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		left = ast.NewBinOp(left, op, right, pos)
	}
	return left, nil
}

// parseAddSub handles: expr + expr, expr - expr
func (p *parser) parseAddSub() (ast.Expr, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return nil, err
	}
	for p.at(lexer.PLUS) || p.at(lexer.MINUS) {
		pos := p.pos()
		var op ast.Operator
		if p.cur.Type == lexer.PLUS {
			op = ast.Add
		} else {
			op = ast.Sub
		}
		p.advance()
		right, err := p.parseMulDiv()
		if err != nil {
			return nil, err
		}
		left = ast.NewBinOp(left, op, right, pos)
	}
	return left, nil
}

// parseMulDiv handles: * / // % @
func (p *parser) parseMulDiv() (ast.Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.isMulDivOp() {
		pos := p.pos()
		op := p.mulDivOp()
		p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = ast.NewBinOp(left, op, right, pos)
	}
	return left, nil
}

func (p *parser) isMulDivOp() bool {
	switch p.cur.Type {
	case lexer.STAR, lexer.SLASH, lexer.DOUBLESLASH, lexer.PERCENT, lexer.AT:
		return true
	}
	return false
}

func (p *parser) mulDivOp() ast.Operator {
	switch p.cur.Type {
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
	}
	return ast.Mult // unreachable
}

// parseUnary handles: -x, +x, ~x
func (p *parser) parseUnary() (ast.Expr, error) {
	switch p.cur.Type {
	case lexer.MINUS:
		pos := p.pos()
		p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return ast.NewUnaryOp(ast.USub, operand, pos), nil
	case lexer.PLUS:
		pos := p.pos()
		p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return ast.NewUnaryOp(ast.UAdd, operand, pos), nil
	case lexer.TILDE:
		pos := p.pos()
		p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return ast.NewUnaryOp(ast.Invert, operand, pos), nil
	}
	return p.parsePower()
}

// parsePower handles: base ** exponent (right-associative)
func (p *parser) parsePower() (ast.Expr, error) {
	base, err := p.parsePostfix()
	if err != nil {
		return nil, err
	}
	if p.at(lexer.DOUBLESTAR) {
		pos := p.pos()
		p.advance()
		// Right-associative: parse exponent at unary level
		exp, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return ast.NewBinOp(base, ast.Pow, exp, pos), nil
	}
	return base, nil
}

// parsePostfix handles calls f(args), subscripts obj[key], attributes obj.attr
func (p *parser) parsePostfix() (ast.Expr, error) {
	expr, err := p.parseAtom()
	if err != nil {
		return nil, err
	}

	for {
		switch p.cur.Type {
		case lexer.LPAREN:
			// Function call
			pos := p.pos()
			p.advance() // consume '('
			args, keywords, err := p.parseCallArgList()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.RPAREN); err != nil {
				return nil, err
			}
			expr = ast.NewCall(expr, args, keywords, pos)

		case lexer.DOT:
			// Attribute access
			pos := p.pos()
			p.advance() // consume '.'
			if !p.at(lexer.NAME) {
				return nil, p.errorf("expected attribute name after '.'")
			}
			attr := p.cur.Value
			p.advance()
			expr = ast.NewAttribute(expr, attr, ast.Load, pos)

		case lexer.LBRACKET:
			// Subscript access (may be index or slice)
			pos := p.pos()
			p.advance() // consume '['
			slice, err := p.parseSubscriptSlice()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.RBRACKET); err != nil {
				return nil, err
			}
			expr = ast.NewSubscript(expr, slice, ast.Load, pos)

		default:
			return expr, nil
		}
	}
}

// parseSubscriptSlice parses the content inside [...] which can be either a
// plain index expression or a slice notation (start:stop:step). Returns either
// the index expression or a Slice AST node.
func (p *parser) parseSubscriptSlice() (ast.Expr, error) {
	pos := p.pos()

	// Parse optional lower bound (may be omitted, e.g. [:5])
	var lower ast.Expr
	if !p.at(lexer.COLON) && !p.at(lexer.RBRACKET) {
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		// If there's no colon following, this is a plain index
		if !p.at(lexer.COLON) {
			return expr, nil
		}
		lower = expr
	}

	// We have a colon → this is a slice
	if _, err := p.expect(lexer.COLON); err != nil {
		return nil, err
	}

	// Parse optional upper bound (omitted if followed by ] or :)
	var upper ast.Expr
	if !p.at(lexer.RBRACKET) && !p.at(lexer.COLON) {
		var err error
		upper, err = p.parseExpr()
		if err != nil {
			return nil, err
		}
	}

	// Parse optional step (preceded by second colon)
	var step ast.Expr
	if p.match(lexer.COLON) {
		if !p.at(lexer.RBRACKET) {
			var err error
			step, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
		}
	}

	return ast.NewSlice(lower, upper, step, pos), nil
}

// parseArgList parses comma-separated expressions for function calls.
func (p *parser) parseArgList() ([]ast.Expr, error) {
	if p.at(lexer.RPAREN) {
		return nil, nil
	}
	var args []ast.Expr
	for {
		arg, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if !p.match(lexer.COMMA) {
			break
		}
		// Allow trailing comma before )
		if p.at(lexer.RPAREN) {
			break
		}
	}
	return args, nil
}

// parseCallArgList parses function call arguments including keyword args,
// *args, and **kwargs. Returns positional args and keyword args separately.
func (p *parser) parseCallArgList() ([]ast.Expr, []*ast.Keyword, error) {
	if p.at(lexer.RPAREN) {
		return nil, nil, nil
	}
	var args []ast.Expr
	var keywords []*ast.Keyword

	for {
		if p.at(lexer.RPAREN) {
			break
		}

		if p.at(lexer.DOUBLESTAR) {
			// **expr → keyword unpacking (Keyword with Arg="")
			p.advance()
			val, err := p.parseExpr()
			if err != nil {
				return nil, nil, err
			}
			keywords = append(keywords, &ast.Keyword{Arg: "", Value: val})
		} else if p.at(lexer.STAR) {
			// *expr → starred positional arg
			pos := p.pos()
			p.advance()
			val, err := p.parseExpr()
			if err != nil {
				return nil, nil, err
			}
			args = append(args, ast.NewStarred(val, ast.Load, pos))
		} else if p.at(lexer.NAME) && p.peek().Type == lexer.EQUAL {
			// name=expr → keyword argument
			name := p.cur.Value
			p.advance() // consume name
			p.advance() // consume '='
			val, err := p.parseExpr()
			if err != nil {
				return nil, nil, err
			}
			keywords = append(keywords, &ast.Keyword{Arg: name, Value: val})
		} else {
			// Regular positional argument
			if len(keywords) > 0 {
				return nil, nil, p.errorf("positional argument follows keyword argument")
			}
			arg, err := p.parseExpr()
			if err != nil {
				return nil, nil, err
			}
			args = append(args, arg)
		}

		if !p.match(lexer.COMMA) {
			break
		}
	}
	return args, keywords, nil
}

// parseAtom parses atomic expressions: names, literals, collections, parens.
func (p *parser) parseAtom() (ast.Expr, error) {
	switch p.cur.Type {
	case lexer.NAME:
		return p.parseName()
	case lexer.INTEGER:
		return p.parseInteger()
	case lexer.FLOAT:
		return p.parseFloat()
	case lexer.STRING:
		return p.parseString()
	case lexer.TRUE:
		pos := p.pos()
		p.advance()
		return ast.NewConstant(true, "", pos), nil
	case lexer.FALSE:
		pos := p.pos()
		p.advance()
		return ast.NewConstant(false, "", pos), nil
	case lexer.NONE:
		pos := p.pos()
		p.advance()
		return ast.NewConstant(nil, "", pos), nil
	case lexer.LPAREN:
		return p.parseParenExpr()
	case lexer.LBRACKET:
		return p.parseListLiteral()
	case lexer.LBRACE:
		return p.parseDictOrSet()
	}
	return nil, p.errorf("unexpected token %s", p.cur.Type)
}

func (p *parser) parseName() (ast.Expr, error) {
	pos := p.pos()
	name := p.cur.Value
	p.advance()
	return ast.NewName(name, ast.Load, pos), nil
}

func (p *parser) parseInteger() (ast.Expr, error) {
	pos := p.pos()
	raw := p.cur.Value
	p.advance()

	// Remove underscores (Python allows 1_000_000)
	raw = strings.ReplaceAll(raw, "_", "")

	var val *big.Int
	var ok bool

	switch {
	case strings.HasPrefix(raw, "0x") || strings.HasPrefix(raw, "0X"):
		val, ok = new(big.Int).SetString(raw[2:], 16)
	case strings.HasPrefix(raw, "0o") || strings.HasPrefix(raw, "0O"):
		val, ok = new(big.Int).SetString(raw[2:], 8)
	case strings.HasPrefix(raw, "0b") || strings.HasPrefix(raw, "0B"):
		val, ok = new(big.Int).SetString(raw[2:], 2)
	default:
		val, ok = new(big.Int).SetString(raw, 10)
	}

	if !ok {
		return nil, &SyntaxError{
			Filename: p.filename, Line: pos.Line, Col: pos.Col,
			Msg: fmt.Sprintf("invalid integer literal: %s", p.prev.Value),
		}
	}

	// Store as int64 if it fits, otherwise *big.Int
	if val.IsInt64() {
		return ast.NewConstant(val.Int64(), "", pos), nil
	}
	return ast.NewConstant(val, "", pos), nil
}

func (p *parser) parseFloat() (ast.Expr, error) {
	pos := p.pos()
	raw := p.cur.Value
	p.advance()

	raw = strings.ReplaceAll(raw, "_", "")
	val, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, &SyntaxError{
			Filename: p.filename, Line: pos.Line, Col: pos.Col,
			Msg: fmt.Sprintf("invalid float literal: %s", raw),
		}
	}
	return ast.NewConstant(val, "", pos), nil
}

func (p *parser) parseString() (ast.Expr, error) {
	pos := p.pos()
	// Python allows implicit string concatenation: "a" "b" == "ab"
	var sb strings.Builder
	for p.at(lexer.STRING) {
		sb.WriteString(p.cur.Value)
		p.advance()
	}
	return ast.NewConstant(sb.String(), "", pos), nil
}

// parseParenExpr parses parenthesized expressions and tuples.
func (p *parser) parseParenExpr() (ast.Expr, error) {
	pos := p.pos()
	p.advance() // consume '('

	// Empty tuple
	if p.at(lexer.RPAREN) {
		p.advance()
		return ast.NewTuple(nil, ast.Load, pos), nil
	}

	first, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	// Generator expression: (elt for target in iter ...)
	if p.at(lexer.FOR) || p.atAsyncFor() {
		generators, err := p.parseComprehensionClauses()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.RPAREN); err != nil {
			return nil, err
		}
		return ast.NewGeneratorExp(first, generators, pos), nil
	}

	// Check if it's a tuple (has comma)
	if p.match(lexer.COMMA) {
		elts := []ast.Expr{first}
		for !p.at(lexer.RPAREN) && !p.at(lexer.EOF) {
			elt, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			elts = append(elts, elt)
			if !p.match(lexer.COMMA) {
				break
			}
		}
		if _, err := p.expect(lexer.RPAREN); err != nil {
			return nil, err
		}
		return ast.NewTuple(elts, ast.Load, pos), nil
	}

	// Single parenthesized expression
	if _, err := p.expect(lexer.RPAREN); err != nil {
		return nil, err
	}
	return first, nil
}

// parseListLiteral parses a list display: either a comprehension
// [elt for target in iter ...] or a plain list [elem, elem, ...].
func (p *parser) parseListLiteral() (ast.Expr, error) {
	pos := p.pos()
	p.advance() // consume '['

	// Empty list.
	if p.at(lexer.RBRACKET) {
		p.advance()
		return ast.NewList(nil, ast.Load, pos), nil
	}

	first, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	// List comprehension: the first element is followed by a 'for' clause.
	if p.at(lexer.FOR) || p.atAsyncFor() {
		generators, err := p.parseComprehensionClauses()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.RBRACKET); err != nil {
			return nil, err
		}
		return ast.NewListComp(first, generators, pos), nil
	}

	// Plain list display.
	elts := []ast.Expr{first}
	if p.match(lexer.COMMA) {
		for !p.at(lexer.RBRACKET) && !p.at(lexer.EOF) {
			elt, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			elts = append(elts, elt)
			if !p.match(lexer.COMMA) {
				break
			}
		}
	}
	if _, err := p.expect(lexer.RBRACKET); err != nil {
		return nil, err
	}
	return ast.NewList(elts, ast.Load, pos), nil
}

// atAsyncFor reports whether the parser is positioned at an 'async for' clause.
func (p *parser) atAsyncFor() bool {
	return p.at(lexer.ASYNC) && p.peek().Type == lexer.FOR
}

// parseComprehensionClauses parses one or more for/if clauses that follow the
// element of a comprehension or generator expression:
//
//	for target in iter [if cond ...] [for target in iter ...]
//
// The parser must be positioned at the first 'for' (or 'async for').
func (p *parser) parseComprehensionClauses() ([]*ast.Comprehension, error) {
	var generators []*ast.Comprehension
	for p.at(lexer.FOR) || p.atAsyncFor() {
		isAsync := false
		if p.at(lexer.ASYNC) {
			isAsync = true
			p.advance() // consume 'async'
		}
		p.advance() // consume 'for'

		target, err := p.parseTargetList()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.IN); err != nil {
			return nil, err
		}
		// The iterable and filter conditions are or-expressions in the Python
		// grammar (not full conditional expressions), so a trailing ternary
		// would need parentheses and cannot swallow following for/if clauses.
		iter, err := p.parseOrExpr()
		if err != nil {
			return nil, err
		}

		var ifs []ast.Expr
		for p.at(lexer.IF) {
			p.advance() // consume 'if'
			cond, err := p.parseOrExpr()
			if err != nil {
				return nil, err
			}
			ifs = append(ifs, cond)
		}

		generators = append(generators, ast.NewComprehension(target, iter, ifs, isAsync))
	}
	return generators, nil
}

// parseDictOrSet parses {k: v, ...} or {elem, ...}
func (p *parser) parseDictOrSet() (ast.Expr, error) {
	pos := p.pos()
	p.advance() // consume '{'

	// Empty dict
	if p.at(lexer.RBRACE) {
		p.advance()
		return ast.NewDict(nil, nil, pos), nil
	}

	// Parse first element to determine dict vs set
	first, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	// Dict: first element followed by ':'
	if p.match(lexer.COLON) {
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		// Dict comprehension: {key: value for target in iter ...}
		if p.at(lexer.FOR) || p.atAsyncFor() {
			generators, err := p.parseComprehensionClauses()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.RBRACE); err != nil {
				return nil, err
			}
			return ast.NewDictComp(first, val, generators, pos), nil
		}
		return p.parseDictRest(first, val, pos)
	}

	// Set comprehension: {elt for target in iter ...}
	if p.at(lexer.FOR) || p.atAsyncFor() {
		generators, err := p.parseComprehensionClauses()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.RBRACE); err != nil {
			return nil, err
		}
		return ast.NewSetComp(first, generators, pos), nil
	}

	// Plain set: first element followed by ',' or '}'
	return p.parseSetRest(first, pos)
}

func (p *parser) parseDictRest(firstKey, firstVal ast.Expr, pos ast.Position) (ast.Expr, error) {
	keys := []ast.Expr{firstKey}
	values := []ast.Expr{firstVal}

	for p.match(lexer.COMMA) {
		if p.at(lexer.RBRACE) {
			break // trailing comma
		}
		key, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.COLON); err != nil {
			return nil, err
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
		values = append(values, val)
	}
	if _, err := p.expect(lexer.RBRACE); err != nil {
		return nil, err
	}
	return ast.NewDict(keys, values, pos), nil
}

func (p *parser) parseSetRest(first ast.Expr, pos ast.Position) (ast.Expr, error) {
	elts := []ast.Expr{first}
	for p.match(lexer.COMMA) {
		if p.at(lexer.RBRACE) {
			break // trailing comma
		}
		elt, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		elts = append(elts, elt)
	}
	if _, err := p.expect(lexer.RBRACE); err != nil {
		return nil, err
	}
	return ast.NewSet(elts, pos), nil
}
