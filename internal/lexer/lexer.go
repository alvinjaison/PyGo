package lexer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Lexer tokenizes Python 3 source text.
//
// Usage:
//
//	l := lexer.New("<stdin>", source)
//	for {
//	    tok := l.Next()
//	    if tok.Type == EOF { break }
//	}
type Lexer struct {
	filename string
	src      []rune // source as rune slice for O(1) indexing
	pos      int    // current read position (rune index)
	line     int    // current 1-indexed line number
	col      int    // current 1-indexed column number

	// Indentation stack. Each entry is the column width of one active indent
	// level. The stack starts with a single entry of 0 (the base level).
	indentStack []int

	// pending holds tokens that have been generated but not yet consumed.
	// DEDENT tokens may be generated multiple times for a single dedent event.
	pending []Token

	// parenDepth tracks open parentheses, brackets, and braces.
	// When > 0, NEWLINE tokens are suppressed (implicit line continuation).
	parenDepth int

	// atLineStart is true when the lexer is at the beginning of a logical line
	// and needs to emit INDENT/DEDENT tokens before the first real token.
	atLineStart bool

	// done is true after the EOF token has been emitted.
	done bool

	// lastWasNewline records whether the most recently produced non-pending
	// token was a NEWLINE, so emitEOF can decide whether to synthesize one.
	lastWasNewline bool

	errors []LexError
}

// LexError represents a lexical error with source position.
type LexError struct {
	Line    int
	Col     int
	Message string
}

func (e LexError) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Line, e.Col, e.Message)
}

// New creates a new Lexer for the given source text.
// filename is used in error messages and token positions.
func New(filename, source string) *Lexer {
	return &Lexer{
		filename:    filename,
		src:         []rune(source),
		pos:         0,
		line:        1,
		col:         1,
		indentStack: []int{0},
		atLineStart: true,
	}
}

// Errors returns any lexical errors encountered so far.
func (l *Lexer) Errors() []LexError {
	return l.errors
}

// Next returns the next token from the source.
// After EOF is returned, all subsequent calls also return EOF.
func (l *Lexer) Next() Token {
	var tok Token
	if len(l.pending) > 0 {
		tok = l.pending[0]
		l.pending = l.pending[1:]
	} else {
		tok = l.scan()
	}
	// Track whether we just produced a NEWLINE (ignoring the synthetic tokens
	// that may follow) so emitEOF can avoid a duplicate terminator.
	switch tok.Type {
	case NEWLINE:
		l.lastWasNewline = true
	case INDENT, DEDENT, EOF:
		// leave lastWasNewline unchanged
	default:
		l.lastWasNewline = false
	}
	return tok
}

// Peek returns the next token without consuming it.
func (l *Lexer) Peek() Token {
	tok := l.Next()
	l.pending = append([]Token{tok}, l.pending...)
	return tok
}

// scan reads the next token from the source stream.
// It is the core of the lexer.
func (l *Lexer) scan() Token {
	if l.done {
		return l.makeToken(EOF, "")
	}

	// Handle indentation at the start of each logical line.
	if l.atLineStart && l.parenDepth == 0 {
		return l.scanIndent()
	}

	// Skip whitespace (not at line start — indentation is handled above).
	l.skipWhitespace()

	if l.pos >= len(l.src) {
		return l.emitEOF()
	}

	ch := l.src[l.pos]

	// Comments: skip to end of line.
	if ch == '#' {
		l.skipComment()
		return l.scan()
	}

	// Newlines.
	if ch == '\n' || ch == '\r' {
		return l.scanNewline()
	}

	// Explicit line continuation.
	if ch == '\\' {
		if l.pos+1 < len(l.src) && (l.src[l.pos+1] == '\n' || l.src[l.pos+1] == '\r') {
			l.advance() // consume backslash
			l.advance() // consume newline
			return l.scan()
		}
		l.addError("unexpected backslash")
		l.advance()
		return l.scan()
	}

	// String literals (including prefixed: r, b, f, rb, br, etc.).
	if isStringStart(l.src, l.pos) {
		return l.scanString()
	}

	// Numeric literals.
	if isDigit(ch) || (ch == '.' && l.pos+1 < len(l.src) && isDigit(l.src[l.pos+1])) {
		return l.scanNumber()
	}

	// Identifiers and keywords.
	if isIdentStart(ch) {
		return l.scanIdentOrKeyword()
	}

	// Operators and delimiters.
	return l.scanOperator()
}

// scanIndent handles INDENT and DEDENT at the start of a logical line.
// It consumes leading whitespace and computes the new indentation level.
func (l *Lexer) scanIndent() Token {
	l.atLineStart = false

	// Count leading spaces/tabs.
	indent := 0
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
		if l.src[l.pos] == '\t' {
			// Tabs expand to the next multiple of 8, matching Python's tokenize.
			indent = (indent/8 + 1) * 8
		} else {
			indent++
		}
		l.advance()
	}

	// Skip blank lines and comment-only lines.
	if l.pos < len(l.src) && (l.src[l.pos] == '\n' || l.src[l.pos] == '\r' || l.src[l.pos] == '#') {
		return l.scan()
	}

	current := l.indentStack[len(l.indentStack)-1]

	switch {
	case indent > current:
		l.indentStack = append(l.indentStack, indent)
		return l.makeToken(INDENT, "")

	case indent < current:
		// Pop indent levels, emitting one DEDENT per level. Return the first
		// DEDENT now and queue the rest so ordering is preserved (DEDENTs must
		// precede the next real token).
		var dedents int
		for len(l.indentStack) > 1 && l.indentStack[len(l.indentStack)-1] > indent {
			l.indentStack = l.indentStack[:len(l.indentStack)-1]
			dedents++
		}
		if l.indentStack[len(l.indentStack)-1] != indent {
			l.addError("unindent does not match any outer indentation level")
		}
		// The current line still needs to be tokenized after the DEDENTs, so
		// re-enter indent handling is unnecessary; just emit DEDENT tokens and
		// let the normal scan continue from the current position.
		for i := 1; i < dedents; i++ {
			l.pending = append(l.pending, l.makeToken(DEDENT, ""))
		}
		return l.makeToken(DEDENT, "")

	default:
		// Same indent level — no INDENT/DEDENT needed.
		return l.scan()
	}
}

// scanNewline handles physical newlines and emits a logical NEWLINE token
// when we are not inside parentheses/brackets/braces.
func (l *Lexer) scanNewline() Token {
	start := l.pos
	if l.src[l.pos] == '\r' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '\n' {
		l.advance() // CR
	}
	l.advance() // LF or lone CR
	l.line++
	l.col = 1

	if l.parenDepth > 0 {
		// Inside brackets: suppress NEWLINE, continue scanning.
		return l.scan()
	}

	l.atLineStart = true
	_ = start
	return l.makeToken(NEWLINE, "\n")
}

// emitEOF emits the closing token sequence at end of input: a final NEWLINE
// (if the last logical line was not already terminated), one DEDENT per open
// indentation level, and finally EOF.
func (l *Lexer) emitEOF() Token {
	l.done = true

	var seq []Token

	// Terminate the final logical line if needed.
	if !l.lastWasNewline {
		seq = append(seq, l.makeToken(NEWLINE, "\n"))
	}
	// Close all open indentation levels.
	for len(l.indentStack) > 1 {
		l.indentStack = l.indentStack[:len(l.indentStack)-1]
		seq = append(seq, l.makeToken(DEDENT, ""))
	}
	seq = append(seq, l.makeToken(EOF, ""))

	first := seq[0]
	l.pending = append(l.pending, seq[1:]...)
	return first
}

// scanString scans a string literal (all flavors: raw, bytes, f-string,
// single/double quoted, triple-quoted).
func (l *Lexer) scanString() Token {
	startLine, startCol := l.line, l.col

	// Read the optional prefix (letters before the quote): r, b, f, u and
	// two-letter combinations.
	var prefix strings.Builder
	for l.pos < len(l.src) && l.src[l.pos] != '"' && l.src[l.pos] != '\'' {
		prefix.WriteRune(unicode.ToLower(l.src[l.pos]))
		l.advance()
	}
	pfx := prefix.String()
	raw := strings.Contains(pfx, "r")
	isBytes := strings.Contains(pfx, "b")

	quote := l.src[l.pos]

	// Detect triple-quoted strings.
	triple := false
	if l.pos+2 < len(l.src) && l.src[l.pos+1] == quote && l.src[l.pos+2] == quote {
		triple = true
		l.advance()
		l.advance()
		l.advance()
	} else {
		l.advance()
	}

	var sb strings.Builder
	for l.pos < len(l.src) {
		ch := l.src[l.pos]

		// Closing delimiter?
		if ch == quote {
			if !triple {
				l.advance()
				break
			}
			if l.pos+2 < len(l.src) && l.src[l.pos+1] == quote && l.src[l.pos+2] == quote {
				l.advance()
				l.advance()
				l.advance()
				break
			}
		}

		// Escape sequences (unless a raw string).
		if ch == '\\' && !raw && l.pos+1 < len(l.src) {
			l.advance()
			sb.WriteString(decodeEscape(l.src[l.pos]))
			l.advance()
			continue
		}

		// A non-triple string must not contain a bare newline.
		if ch == '\n' && !triple {
			l.addError("EOL while scanning string literal")
			break
		}

		sb.WriteRune(ch)
		l.advance()
	}

	tt := STRING
	if isBytes {
		tt = BYTES
	}
	tok := Token{
		Type:      tt,
		Value:     sb.String(),
		Line:      startLine,
		Col:       startCol,
		StrPrefix: pfx,
	}
	return tok
}

// decodeEscape returns the replacement string for a backslash escape.
func decodeEscape(c rune) string {
	switch c {
	case 'n':
		return "\n"
	case 't':
		return "\t"
	case 'r':
		return "\r"
	case '\\':
		return "\\"
	case '\'':
		return "'"
	case '"':
		return "\""
	case '0':
		return "\x00"
	case 'a':
		return "\a"
	case 'b':
		return "\b"
	case 'f':
		return "\f"
	case 'v':
		return "\v"
	case '\n':
		return "" // line continuation inside a string
	}
	return "\\" + string(c)
}

// scanNumber scans an integer or float literal.
func (l *Lexer) scanNumber() Token {
	// TODO(Phase 1): Full number scanning including:
	//   - Decimal integers with underscores (1_000_000)
	//   - Hex (0xFF), octal (0o77), binary (0b1010)
	//   - Floats (3.14, 1e10, 1.5e-3)
	//   - Complex numbers (3+4j)
	//
	// Placeholder: scan a simple decimal integer or float.
	start := l.pos
	isFloat := false

	for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
		l.advance()
	}
	if l.pos < len(l.src) && l.src[l.pos] == '.' {
		isFloat = true
		l.advance()
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			l.advance()
		}
	}
	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		isFloat = true
		l.advance()
		if l.pos < len(l.src) && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
			l.advance()
		}
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			l.advance()
		}
	}

	val := string(l.src[start:l.pos])
	if isFloat {
		return l.makeToken(FLOAT, val)
	}
	return l.makeToken(INTEGER, val)
}

// scanIdentOrKeyword scans an identifier or keyword.
func (l *Lexer) scanIdentOrKeyword() Token {
	start := l.pos
	for l.pos < len(l.src) && isIdentContinue(l.src[l.pos]) {
		l.advance()
	}
	value := string(l.src[start:l.pos])
	tt := LookupKeyword(value)
	return l.makeToken(tt, value)
}

// scanOperator scans a single- or multi-character operator or delimiter.
func (l *Lexer) scanOperator() Token {
	// TODO(Phase 1): Implement full operator scanning.
	// Placeholder for the most common operators.
	ch := l.src[l.pos]
	l.advance()

	switch ch {
	case '+':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeToken(PLUSEQUAL, "+=")
		}
		return l.makeToken(PLUS, "+")
	case '-':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeToken(MINUSEQUAL, "-=")
		}
		if l.pos < len(l.src) && l.src[l.pos] == '>' {
			l.advance()
			return l.makeToken(RARROW, "->")
		}
		return l.makeToken(MINUS, "-")
	case '*':
		if l.pos < len(l.src) && l.src[l.pos] == '*' {
			l.advance()
			if l.pos < len(l.src) && l.src[l.pos] == '=' {
				l.advance()
				return l.makeToken(DOUBLESTAREQUAL, "**=")
			}
			return l.makeToken(DOUBLESTAR, "**")
		}
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeToken(STAREQUAL, "*=")
		}
		return l.makeToken(STAR, "*")
	case '/':
		if l.pos < len(l.src) && l.src[l.pos] == '/' {
			l.advance()
			if l.pos < len(l.src) && l.src[l.pos] == '=' {
				l.advance()
				return l.makeToken(DOUBLESLASHEQUAL, "//=")
			}
			return l.makeToken(DOUBLESLASH, "//")
		}
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeToken(SLASHEQUAL, "/=")
		}
		return l.makeToken(SLASH, "/")
	case '%':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeToken(PERCENTEQUAL, "%=")
		}
		return l.makeToken(PERCENT, "%")
	case '=':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeToken(EQEQUAL, "==")
		}
		return l.makeToken(EQUAL, "=")
	case '!':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeToken(NOTEQUAL, "!=")
		}
		l.addError("invalid character '!'")
		return l.makeToken(ILLEGAL, "!")
	case '<':
		if l.pos < len(l.src) && l.src[l.pos] == '<' {
			l.advance()
			if l.pos < len(l.src) && l.src[l.pos] == '=' {
				l.advance()
				return l.makeToken(LSHIFTEQUAL, "<<=")
			}
			return l.makeToken(LSHIFT, "<<")
		}
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeToken(LESSEQUAL, "<=")
		}
		return l.makeToken(LESS, "<")
	case '>':
		if l.pos < len(l.src) && l.src[l.pos] == '>' {
			l.advance()
			if l.pos < len(l.src) && l.src[l.pos] == '=' {
				l.advance()
				return l.makeToken(RSHIFTEQUAL, ">>=")
			}
			return l.makeToken(RSHIFT, ">>")
		}
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeToken(GREATEREQUAL, ">=")
		}
		return l.makeToken(GREATER, ">")
	case ':':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeToken(WALRUS, ":=")
		}
		return l.makeToken(COLON, ":")
	case '(':
		l.parenDepth++
		return l.makeToken(LPAREN, "(")
	case ')':
		if l.parenDepth > 0 {
			l.parenDepth--
		}
		return l.makeToken(RPAREN, ")")
	case '[':
		l.parenDepth++
		return l.makeToken(LBRACKET, "[")
	case ']':
		if l.parenDepth > 0 {
			l.parenDepth--
		}
		return l.makeToken(RBRACKET, "]")
	case '{':
		l.parenDepth++
		return l.makeToken(LBRACE, "{")
	case '}':
		if l.parenDepth > 0 {
			l.parenDepth--
		}
		return l.makeToken(RBRACE, "}")
	case ',':
		return l.makeToken(COMMA, ",")
	case '.':
		if l.pos+1 < len(l.src) && l.src[l.pos] == '.' && l.src[l.pos+1] == '.' {
			l.advance()
			l.advance()
			return l.makeToken(ELLIPSIS, "...")
		}
		return l.makeToken(DOT, ".")
	case '&':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeToken(AMPEREQUAL, "&=")
		}
		return l.makeToken(AMPER, "&")
	case '|':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeToken(PIPEEQUAL, "|=")
		}
		return l.makeToken(PIPE, "|")
	case '^':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeToken(CARETEQUAL, "^=")
		}
		return l.makeToken(CARET, "^")
	case '~':
		return l.makeToken(TILDE, "~")
	case '@':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeToken(ATEQUAL, "@=")
		}
		return l.makeToken(AT, "@")
	case ';':
		return l.makeToken(SEMICOLON, ";")
	}

	l.addError(fmt.Sprintf("unexpected character %q", ch))
	return l.makeToken(ILLEGAL, string(ch))
}

// --- helpers ---

// advance moves the position forward by one rune and updates line/col.
func (l *Lexer) advance() {
	if l.pos >= len(l.src) {
		return
	}
	if l.src[l.pos] == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	l.pos++
}

// skipWhitespace skips spaces and tabs (but not newlines — those are tokens).
func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
		l.advance()
	}
}

// skipComment skips from '#' to end of line.
func (l *Lexer) skipComment() {
	for l.pos < len(l.src) && l.src[l.pos] != '\n' && l.src[l.pos] != '\r' {
		l.advance()
	}
}

// makeToken constructs a token at the current source position.
func (l *Lexer) makeToken(tt TokenType, value string) Token {
	return Token{
		Type:  tt,
		Value: value,
		Line:  l.line,
		Col:   l.col,
	}
}

// addError records a lexical error at the current position.
func (l *Lexer) addError(msg string) {
	l.errors = append(l.errors, LexError{Line: l.line, Col: l.col, Message: msg})
}

// isStringStart reports whether the rune(s) at pos begin a string literal.
// Handles prefixes: r, b, f, u, rb, br, fr, rf (case-insensitive).
func isStringStart(src []rune, pos int) bool {
	if pos >= len(src) {
		return false
	}
	ch := unicode.ToLower(src[pos])
	if ch == '"' || ch == '\'' {
		return true
	}
	if ch == 'r' || ch == 'b' || ch == 'f' || ch == 'u' {
		next := pos + 1
		if next < len(src) {
			n := unicode.ToLower(src[next])
			if n == '"' || n == '\'' {
				return true
			}
			if (ch == 'r' && (n == 'b' || n == 'f')) || ((ch == 'b' || ch == 'f') && n == 'r') {
				if next+1 < len(src) {
					nn := unicode.ToLower(src[next+1])
					return nn == '"' || nn == '\''
				}
			}
		}
	}
	return false
}

func isDigit(r rune) bool        { return r >= '0' && r <= '9' }
func isIdentStart(r rune) bool   { return r == '_' || unicode.IsLetter(r) }
func isIdentContinue(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// All returns all tokens from the source as a slice.
// Useful for testing. Stops at EOF.
func All(filename, source string) []Token {
	l := New(filename, source)
	var tokens []Token
	for {
		tok := l.Next()
		tokens = append(tokens, tok)
		if tok.Type == EOF {
			break
		}
	}
	return tokens
}

// ensure utf8 import is used
var _ = utf8.RuneLen
