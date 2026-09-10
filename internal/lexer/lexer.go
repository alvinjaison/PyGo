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
	for len(l.pending) > 0 {
		tok := l.pending[0]
		l.pending = l.pending[1:]
		return tok
	}
	return l.scan()
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
			// Consume newline: handle \r\n (CRLF), \r (CR), or \n (LF).
			if l.src[l.pos] == '\r' {
				l.pos++
				if l.pos < len(l.src) && l.src[l.pos] == '\n' {
					l.pos++
				}
			} else {
				l.pos++
			}
			l.line++
			l.col = 1
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

	// Count leading spaces/tabs and detect mixing.
	indent := 0
	hasTabs := false
	hasSpaces := false
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
		if l.src[l.pos] == '\t' {
			hasTabs = true
			// Tabs expand to the next multiple of 8, matching Python's tokenize.
			indent = (indent/8 + 1) * 8
		} else {
			hasSpaces = true
			indent++
		}
		l.advance()
	}

	// Skip blank lines and comment-only lines: consume the rest of the line
	// and restart indentation processing on the next line without emitting
	// any NEWLINE token.
	if l.pos < len(l.src) && (l.src[l.pos] == '\n' || l.src[l.pos] == '\r' || l.src[l.pos] == '#') {
		// Skip comment text if present.
		if l.pos < len(l.src) && l.src[l.pos] == '#' {
			l.skipComment()
		}
		// Consume the newline character(s).
		if l.pos < len(l.src) && (l.src[l.pos] == '\r' || l.src[l.pos] == '\n') {
			if l.src[l.pos] == '\r' {
				l.pos++
				if l.pos < len(l.src) && l.src[l.pos] == '\n' {
					l.pos++
				}
			} else {
				l.pos++
			}
			l.line++
			l.col = 1
		}
		// Restart indentation processing on the next line.
		l.atLineStart = true
		return l.scan()
	}

	// Report SyntaxError for inconsistent indentation (tab/space mixing)
	// only when the indentation is non-zero and both tabs and spaces are used.
	if hasTabs && hasSpaces && indent > 0 {
		l.addError("inconsistent use of tabs and spaces in indentation")
	}

	// If we reached EOF after consuming whitespace, don't compare indentation.
	if l.pos >= len(l.src) {
		return l.emitEOF()
	}

	current := l.indentStack[len(l.indentStack)-1]

	switch {
	case indent > current:
		l.indentStack = append(l.indentStack, indent)
		return l.makeToken(INDENT, "")

	case indent < current:
		// Pop indent levels and queue DEDENT tokens.
		var dedents []Token
		for len(l.indentStack) > 1 && l.indentStack[len(l.indentStack)-1] > indent {
			l.indentStack = l.indentStack[:len(l.indentStack)-1]
			dedents = append(dedents, l.makeToken(DEDENT, ""))
		}
		if l.indentStack[len(l.indentStack)-1] != indent {
			l.addError("unindent does not match any outer indentation level")
		}
		if len(dedents) == 0 {
			return l.scan()
		}
		// Return the first DEDENT; queue the rest in pending.
		if len(dedents) > 1 {
			l.pending = append(l.pending, dedents[1:]...)
		}
		return dedents[0]

	default:
		// Same indent level — no INDENT/DEDENT needed.
		return l.scan()
	}
}

// scanNewline handles physical newlines and emits a logical NEWLINE token
// when we are not inside parentheses/brackets/braces.
func (l *Lexer) scanNewline() Token {
	startLine, startCol := l.line, l.col
	if l.src[l.pos] == '\r' {
		l.pos++ // skip CR without triggering advance() side effects
		if l.pos < len(l.src) && l.src[l.pos] == '\n' {
			l.pos++ // skip LF in CRLF
		}
	} else {
		l.pos++ // skip LF
	}
	l.line++
	l.col = 1

	if l.parenDepth > 0 {
		// Inside brackets: suppress NEWLINE, continue scanning.
		return l.scan()
	}

	l.atLineStart = true
	return l.makeTokenAt(NEWLINE, "\n", startLine, startCol)
}

// emitEOF emits the EOF token and any pending DEDENTs to close open blocks.
func (l *Lexer) emitEOF() Token {
	if len(l.indentStack) > 1 {
		// Emit all remaining DEDENT tokens before EOF.
		var dedents []Token
		for len(l.indentStack) > 1 {
			l.indentStack = l.indentStack[:len(l.indentStack)-1]
			dedents = append(dedents, l.makeToken(DEDENT, ""))
		}
		// Queue remaining DEDENTs and EOF in pending.
		if len(dedents) > 1 {
			l.pending = append(l.pending, dedents[1:]...)
		}
		l.pending = append(l.pending, l.makeToken(EOF, ""))
		l.done = true
		return dedents[0]
	}
	l.done = true
	return l.makeToken(EOF, "")
}

// scanString scans a string literal (all flavors: raw, bytes, f-string,
// single/double quoted, triple-quoted).
func (l *Lexer) scanString() Token {
	startLine, startCol := l.line, l.col

	// Consume prefix characters (r, b, f, u, rb, br, fr, rf).
	isRaw := false
	isBytes := false
	for l.pos < len(l.src) {
		ch := unicode.ToLower(l.src[l.pos])
		if ch == 'r' {
			isRaw = true
			l.advance()
		} else if ch == 'b' {
			isBytes = true
			l.advance()
		} else if ch == 'f' || ch == 'u' {
			l.advance()
		} else {
			break
		}
	}

	// Determine quote character and whether this is triple-quoted.
	quote := l.src[l.pos]
	l.advance()

	triple := false
	if l.pos+1 < len(l.src) && l.src[l.pos] == quote && l.src[l.pos+1] == quote {
		triple = true
		l.advance()
		l.advance()
	}

	var sb strings.Builder

	if triple {
		// Triple-quoted string: scan until closing triple-quote.
		for {
			if l.pos >= len(l.src) {
				l.errors = append(l.errors, LexError{
					Line:    startLine,
					Col:     startCol,
					Message: "SyntaxError: unterminated triple-quoted string literal",
				})
				tokType := STRING
				if isBytes {
					tokType = BYTES
				}
				return l.makeTokenAt(tokType, sb.String(), startLine, startCol)
			}
			// Check for closing triple-quote.
			if l.src[l.pos] == quote && l.pos+2 < len(l.src) && l.src[l.pos+1] == quote && l.src[l.pos+2] == quote {
				l.advance()
				l.advance()
				l.advance()
				break
			}
			if l.src[l.pos] == '\\' && !isRaw {
				esc := l.scanEscapeSequence()
				sb.WriteString(esc)
			} else {
				sb.WriteRune(l.src[l.pos])
				l.advance()
			}
		}
	} else {
		// Single-quoted string: scan until closing quote or end of line.
		for l.pos < len(l.src) && l.src[l.pos] != quote {
			if l.src[l.pos] == '\n' || l.src[l.pos] == '\r' {
				l.errors = append(l.errors, LexError{
					Line:    startLine,
					Col:     startCol,
					Message: "SyntaxError: unterminated string literal",
				})
				tokType := STRING
				if isBytes {
					tokType = BYTES
				}
				return l.makeTokenAt(tokType, sb.String(), startLine, startCol)
			}
			if l.src[l.pos] == '\\' && !isRaw {
				esc := l.scanEscapeSequence()
				sb.WriteString(esc)
			} else {
				sb.WriteRune(l.src[l.pos])
				l.advance()
			}
		}
		if l.pos >= len(l.src) {
			l.errors = append(l.errors, LexError{
				Line:    startLine,
				Col:     startCol,
				Message: "SyntaxError: unterminated string literal",
			})
			tokType := STRING
			if isBytes {
				tokType = BYTES
			}
			return l.makeTokenAt(tokType, sb.String(), startLine, startCol)
		}
		l.advance() // consume closing quote
	}

	tokType := STRING
	if isBytes {
		tokType = BYTES
	}
	return l.makeTokenAt(tokType, sb.String(), startLine, startCol)
}

// scanEscapeSequence processes an escape sequence starting at the backslash.
// The lexer position should be at the backslash character.
// Returns the decoded escape sequence as a string.
func (l *Lexer) scanEscapeSequence() string {
	l.advance() // consume the backslash
	if l.pos >= len(l.src) {
		return "\\"
	}

	ch := l.src[l.pos]
	l.advance()

	switch ch {
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
	case 'a':
		return "\a"
	case 'b':
		return "\b"
	case 'f':
		return "\f"
	case 'v':
		return "\v"
	case '0':
		return "\x00"
	case 'x':
		// \xHH — exactly 2 hex digits
		if l.pos+1 < len(l.src) && isHexDigit(l.src[l.pos]) && isHexDigit(l.src[l.pos+1]) {
			val := hexVal(l.src[l.pos])*16 + hexVal(l.src[l.pos+1])
			l.advance()
			l.advance()
			return string(rune(val))
		}
		return "\\x"
	case 'u':
		// \uHHHH — exactly 4 hex digits
		if l.pos+3 < len(l.src) && isHexDigit(l.src[l.pos]) && isHexDigit(l.src[l.pos+1]) && isHexDigit(l.src[l.pos+2]) && isHexDigit(l.src[l.pos+3]) {
			val := hexVal(l.src[l.pos])<<12 | hexVal(l.src[l.pos+1])<<8 | hexVal(l.src[l.pos+2])<<4 | hexVal(l.src[l.pos+3])
			l.advance()
			l.advance()
			l.advance()
			l.advance()
			return string(rune(val))
		}
		return "\\u"
	case 'U':
		// \UHHHHHHHH — exactly 8 hex digits
		if l.pos+7 < len(l.src) {
			allHex := true
			for i := 0; i < 8; i++ {
				if !isHexDigit(l.src[l.pos+i]) {
					allHex = false
					break
				}
			}
			if allHex {
				var val rune
				for i := 0; i < 8; i++ {
					val = val<<4 | rune(hexVal(l.src[l.pos]))
					l.advance()
				}
				return string(val)
			}
		}
		return "\\U"
	case '\n':
		// Escaped newline: line continuation inside string (not raw)
		return ""
	default:
		// Unknown escape: keep the backslash and character as-is (Python behavior)
		return "\\" + string(ch)
	}
}

// isHexDigit reports whether a rune is a hexadecimal digit.
func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

// hexVal returns the numeric value of a hex digit.
func hexVal(r rune) rune {
	switch {
	case r >= '0' && r <= '9':
		return r - '0'
	case r >= 'a' && r <= 'f':
		return r - 'a' + 10
	case r >= 'A' && r <= 'F':
		return r - 'A' + 10
	default:
		return 0
	}
}

// scanNumber scans an integer or float literal.
// Supports: decimal, hex (0x/0X), octal (0o/0O), binary (0b/0B),
// underscore separators, float with decimal/exponent notation.
func (l *Lexer) scanNumber() Token {
	startLine, startCol := l.line, l.col
	start := l.pos

	// Check for leading dot (e.g., .5)
	if l.src[l.pos] == '.' {
		return l.scanFloatFromDot(start, startLine, startCol)
	}

	// Check for 0x, 0o, 0b prefixes
	if l.src[l.pos] == '0' && l.pos+1 < len(l.src) {
		next := unicode.ToLower(l.src[l.pos+1])
		switch next {
		case 'x':
			return l.scanHex(start, startLine, startCol)
		case 'o':
			return l.scanOctal(start, startLine, startCol)
		case 'b':
			return l.scanBinary(start, startLine, startCol)
		}
	}

	// Decimal integer or float.
	l.scanDecimalDigits()

	// Check for float indicators.
	if l.pos < len(l.src) && l.src[l.pos] == '.' {
		l.advance()
		l.scanDecimalDigits()
		// Check for exponent
		if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
			l.scanExponent()
		}
		val := string(l.src[start:l.pos])
		return l.makeTokenAt(FLOAT, val, startLine, startCol)
	}

	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		l.scanExponent()
		val := string(l.src[start:l.pos])
		return l.makeTokenAt(FLOAT, val, startLine, startCol)
	}

	val := string(l.src[start:l.pos])
	return l.makeTokenAt(INTEGER, val, startLine, startCol)
}

// scanFloatFromDot scans a float that starts with a dot (e.g., .5, .5e10).
func (l *Lexer) scanFloatFromDot(start int, startLine, startCol int) Token {
	l.advance() // consume the dot
	l.scanDecimalDigits()
	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		l.scanExponent()
	}
	val := string(l.src[start:l.pos])
	return l.makeTokenAt(FLOAT, val, startLine, startCol)
}

// scanHex scans a hexadecimal integer (0x or 0X prefix).
func (l *Lexer) scanHex(start int, startLine, startCol int) Token {
	l.advance() // consume '0'
	l.advance() // consume 'x' or 'X'
	for l.pos < len(l.src) && (isHexDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
		l.advance()
	}
	val := string(l.src[start:l.pos])
	return l.makeTokenAt(INTEGER, val, startLine, startCol)
}

// scanOctal scans an octal integer (0o or 0O prefix).
func (l *Lexer) scanOctal(start int, startLine, startCol int) Token {
	l.advance() // consume '0'
	l.advance() // consume 'o' or 'O'
	for l.pos < len(l.src) && (isOctalDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
		l.advance()
	}
	val := string(l.src[start:l.pos])
	return l.makeTokenAt(INTEGER, val, startLine, startCol)
}

// scanBinary scans a binary integer (0b or 0B prefix).
func (l *Lexer) scanBinary(start int, startLine, startCol int) Token {
	l.advance() // consume '0'
	l.advance() // consume 'b' or 'B'
	for l.pos < len(l.src) && (l.src[l.pos] == '0' || l.src[l.pos] == '1' || l.src[l.pos] == '_') {
		l.advance()
	}
	val := string(l.src[start:l.pos])
	return l.makeTokenAt(INTEGER, val, startLine, startCol)
}

// scanDecimalDigits consumes decimal digits and underscore separators.
func (l *Lexer) scanDecimalDigits() {
	for l.pos < len(l.src) && (isDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
		l.advance()
	}
}

// scanExponent consumes an exponent part (e/E followed by optional sign and digits).
func (l *Lexer) scanExponent() {
	l.advance() // consume 'e' or 'E'
	if l.pos < len(l.src) && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
		l.advance()
	}
	l.scanDecimalDigits()
}

// isOctalDigit reports whether a rune is an octal digit.
func isOctalDigit(r rune) bool {
	return r >= '0' && r <= '7'
}

// scanIdentOrKeyword scans an identifier or keyword.
func (l *Lexer) scanIdentOrKeyword() Token {
	startLine, startCol := l.line, l.col
	start := l.pos
	for l.pos < len(l.src) && isIdentContinue(l.src[l.pos]) {
		l.advance()
	}
	value := string(l.src[start:l.pos])
	tt := LookupKeyword(value)
	return l.makeTokenAt(tt, value, startLine, startCol)
}

// scanOperator scans a single- or multi-character operator or delimiter.
func (l *Lexer) scanOperator() Token {
	startLine, startCol := l.line, l.col
	ch := l.src[l.pos]
	l.advance()

	switch ch {
	case '+':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeTokenAt(PLUSEQUAL, "+=", startLine, startCol)
		}
		return l.makeTokenAt(PLUS, "+", startLine, startCol)
	case '-':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeTokenAt(MINUSEQUAL, "-=", startLine, startCol)
		}
		if l.pos < len(l.src) && l.src[l.pos] == '>' {
			l.advance()
			return l.makeTokenAt(RARROW, "->", startLine, startCol)
		}
		return l.makeTokenAt(MINUS, "-", startLine, startCol)
	case '*':
		if l.pos < len(l.src) && l.src[l.pos] == '*' {
			l.advance()
			if l.pos < len(l.src) && l.src[l.pos] == '=' {
				l.advance()
				return l.makeTokenAt(DOUBLESTAREQUAL, "**=", startLine, startCol)
			}
			return l.makeTokenAt(DOUBLESTAR, "**", startLine, startCol)
		}
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeTokenAt(STAREQUAL, "*=", startLine, startCol)
		}
		return l.makeTokenAt(STAR, "*", startLine, startCol)
	case '/':
		if l.pos < len(l.src) && l.src[l.pos] == '/' {
			l.advance()
			if l.pos < len(l.src) && l.src[l.pos] == '=' {
				l.advance()
				return l.makeTokenAt(DOUBLESLASHEQUAL, "//=", startLine, startCol)
			}
			return l.makeTokenAt(DOUBLESLASH, "//", startLine, startCol)
		}
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeTokenAt(SLASHEQUAL, "/=", startLine, startCol)
		}
		return l.makeTokenAt(SLASH, "/", startLine, startCol)
	case '%':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeTokenAt(PERCENTEQUAL, "%=", startLine, startCol)
		}
		return l.makeTokenAt(PERCENT, "%", startLine, startCol)
	case '=':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeTokenAt(EQEQUAL, "==", startLine, startCol)
		}
		return l.makeTokenAt(EQUAL, "=", startLine, startCol)
	case '!':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeTokenAt(NOTEQUAL, "!=", startLine, startCol)
		}
		l.addError("invalid character '!'")
		return l.makeTokenAt(ILLEGAL, "!", startLine, startCol)
	case '<':
		if l.pos < len(l.src) && l.src[l.pos] == '<' {
			l.advance()
			if l.pos < len(l.src) && l.src[l.pos] == '=' {
				l.advance()
				return l.makeTokenAt(LSHIFTEQUAL, "<<=", startLine, startCol)
			}
			return l.makeTokenAt(LSHIFT, "<<", startLine, startCol)
		}
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeTokenAt(LESSEQUAL, "<=", startLine, startCol)
		}
		return l.makeTokenAt(LESS, "<", startLine, startCol)
	case '>':
		if l.pos < len(l.src) && l.src[l.pos] == '>' {
			l.advance()
			if l.pos < len(l.src) && l.src[l.pos] == '=' {
				l.advance()
				return l.makeTokenAt(RSHIFTEQUAL, ">>=", startLine, startCol)
			}
			return l.makeTokenAt(RSHIFT, ">>", startLine, startCol)
		}
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeTokenAt(GREATEREQUAL, ">=", startLine, startCol)
		}
		return l.makeTokenAt(GREATER, ">", startLine, startCol)
	case ':':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeTokenAt(WALRUS, ":=", startLine, startCol)
		}
		return l.makeTokenAt(COLON, ":", startLine, startCol)
	case '(':
		l.parenDepth++
		return l.makeTokenAt(LPAREN, "(", startLine, startCol)
	case ')':
		if l.parenDepth > 0 {
			l.parenDepth--
		}
		return l.makeTokenAt(RPAREN, ")", startLine, startCol)
	case '[':
		l.parenDepth++
		return l.makeTokenAt(LBRACKET, "[", startLine, startCol)
	case ']':
		if l.parenDepth > 0 {
			l.parenDepth--
		}
		return l.makeTokenAt(RBRACKET, "]", startLine, startCol)
	case '{':
		l.parenDepth++
		return l.makeTokenAt(LBRACE, "{", startLine, startCol)
	case '}':
		if l.parenDepth > 0 {
			l.parenDepth--
		}
		return l.makeTokenAt(RBRACE, "}", startLine, startCol)
	case ',':
		return l.makeTokenAt(COMMA, ",", startLine, startCol)
	case '.':
		if l.pos+1 < len(l.src) && l.src[l.pos] == '.' && l.src[l.pos+1] == '.' {
			l.advance()
			l.advance()
			return l.makeTokenAt(ELLIPSIS, "...", startLine, startCol)
		}
		return l.makeTokenAt(DOT, ".", startLine, startCol)
	case '&':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeTokenAt(AMPEREQUAL, "&=", startLine, startCol)
		}
		return l.makeTokenAt(AMPER, "&", startLine, startCol)
	case '|':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeTokenAt(PIPEEQUAL, "|=", startLine, startCol)
		}
		return l.makeTokenAt(PIPE, "|", startLine, startCol)
	case '^':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeTokenAt(CARETEQUAL, "^=", startLine, startCol)
		}
		return l.makeTokenAt(CARET, "^", startLine, startCol)
	case '~':
		return l.makeTokenAt(TILDE, "~", startLine, startCol)
	case '@':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return l.makeTokenAt(ATEQUAL, "@=", startLine, startCol)
		}
		return l.makeTokenAt(AT, "@", startLine, startCol)
	case ';':
		return l.makeTokenAt(SEMICOLON, ";", startLine, startCol)
	}

	l.addError(fmt.Sprintf("unexpected character %q", ch))
	return l.makeTokenAt(ILLEGAL, string(ch), startLine, startCol)
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
		Pos: Position{
			Filename: l.filename,
			Line:     l.line,
			Column:   l.col,
		},
	}
}

// makeTokenAt constructs a token at a specific source position.
func (l *Lexer) makeTokenAt(tt TokenType, value string, line, col int) Token {
	return Token{
		Type:  tt,
		Value: value,
		Pos: Position{
			Filename: l.filename,
			Line:     line,
			Column:   col,
		},
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
