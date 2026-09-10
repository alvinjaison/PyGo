package lexer

import (
	"strings"
	"testing"
)

// collectTokens tokenizes source and returns all tokens (including EOF).
func collectTokens(source string) []Token {
	return All("<test>", source)
}

// collectTokenTypes returns just the token types from source.
func collectTokenTypes(source string) []TokenType {
	tokens := collectTokens(source)
	types := make([]TokenType, len(tokens))
	for i, t := range tokens {
		types[i] = t.Type
	}
	return types
}

func TestIdentifiers(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"x", "x"},
		{"_private", "_private"},
		{"__dunder__", "__dunder__"},
		{"camelCase", "camelCase"},
		{"var123", "var123"},
		{"_", "_"},
		{"abc_def", "abc_def"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens := collectTokens(tt.input)
			// Expect: NAME, NEWLINE, EOF (or NAME, EOF for single-token inputs)
			found := false
			for _, tok := range tokens {
				if tok.Type == NAME {
					if tok.Value != tt.want {
						t.Errorf("got value %q, want %q", tok.Value, tt.want)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no NAME token found in %v", tokens)
			}
		})
	}
}

func TestKeywords(t *testing.T) {
	tests := []struct {
		input string
		want  TokenType
	}{
		{"def", DEF},
		{"return", RETURN},
		{"if", IF},
		{"elif", ELIF},
		{"else", ELSE},
		{"while", WHILE},
		{"for", FOR},
		{"in", IN},
		{"break", BREAK},
		{"continue", CONTINUE},
		{"pass", PASS},
		{"and", AND},
		{"or", OR},
		{"not", NOT},
		{"True", TRUE},
		{"False", FALSE},
		{"None", NONE},
		{"import", IMPORT},
		{"from", FROM},
		{"as", AS},
		{"try", TRY},
		{"except", EXCEPT},
		{"finally", FINALLY},
		{"raise", RAISE},
		{"del", DEL},
		{"global", GLOBAL},
		{"class", CLASS},
		{"is", IS},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens := collectTokens(tt.input)
			if len(tokens) == 0 {
				t.Fatal("no tokens")
			}
			// Find the keyword token (skip NEWLINE/EOF)
			found := false
			for _, tok := range tokens {
				if tok.Type == tt.want {
					if tok.Value != tt.input {
						t.Errorf("got value %q, want %q", tok.Value, tt.input)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected token type %v not found; got tokens: %v", tt.want, tokens)
			}
		})
	}
}

func TestKeywordVsIdentifier(t *testing.T) {
	// Keywords should not match if they're substrings of identifiers
	tests := []struct {
		input    string
		wantType TokenType
	}{
		{"define", NAME},       // not DEF
		{"return_value", NAME}, // not RETURN
		{"iffy", NAME},        // not IF
		{"for_loop", NAME},    // not FOR
		{"True", TRUE},        // keyword
		{"Trueish", NAME},     // not TRUE
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens := collectTokens(tt.input)
			found := false
			for _, tok := range tokens {
				if tok.Type == tt.wantType && tok.Value == tt.input {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %v token with value %q; got: %v", tt.wantType, tt.input, tokens)
			}
		})
	}
}

func TestOperators(t *testing.T) {
	tests := []struct {
		input string
		want  TokenType
		value string
	}{
		{"+", PLUS, "+"},
		{"-", MINUS, "-"},
		{"*", STAR, "*"},
		{"/", SLASH, "/"},
		{"//", DOUBLESLASH, "//"},
		{"%", PERCENT, "%"},
		{"**", DOUBLESTAR, "**"},
		{"==", EQEQUAL, "=="},
		{"!=", NOTEQUAL, "!="},
		{"<", LESS, "<"},
		{">", GREATER, ">"},
		{"<=", LESSEQUAL, "<="},
		{">=", GREATEREQUAL, ">="},
		{"=", EQUAL, "="},
		{"+=", PLUSEQUAL, "+="},
		{"-=", MINUSEQUAL, "-="},
		{"*=", STAREQUAL, "*="},
		{"/=", SLASHEQUAL, "/="},
		{"//=", DOUBLESLASHEQUAL, "//="},
		{"%=", PERCENTEQUAL, "%="},
		{"**=", DOUBLESTAREQUAL, "**="},
		{"&", AMPER, "&"},
		{"|", PIPE, "|"},
		{"^", CARET, "^"},
		{"~", TILDE, "~"},
		{"<<", LSHIFT, "<<"},
		{">>", RSHIFT, ">>"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens := collectTokens(tt.input)
			found := false
			for _, tok := range tokens {
				if tok.Type == tt.want {
					if tok.Value != tt.value {
						t.Errorf("got value %q, want %q", tok.Value, tt.value)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected token type %v; got: %v", tt.want, tokens)
			}
		})
	}
}

func TestDelimiters(t *testing.T) {
	tests := []struct {
		input string
		want  TokenType
		value string
	}{
		{",", COMMA, ","},
		{".", DOT, "."},
		{":", COLON, ":"},
		{";", SEMICOLON, ";"},
		{"(", LPAREN, "("},
		{")", RPAREN, ")"},
		{"[", LBRACKET, "["},
		{"]", RBRACKET, "]"},
		{"{", LBRACE, "{"},
		{"}", RBRACE, "}"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens := collectTokens(tt.input)
			found := false
			for _, tok := range tokens {
				if tok.Type == tt.want {
					if tok.Value != tt.value {
						t.Errorf("got value %q, want %q", tok.Value, tt.value)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected token type %v; got: %v", tt.want, tokens)
			}
		})
	}
}

func TestComments(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		noType TokenType // this type should NOT appear
	}{
		{"comment only line", "# this is a comment", EOF}, // should just get EOF
		{"comment after code", "x = 1 # inline comment", EOF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := collectTokens(tt.input)
			// Comments should never produce a token
			for _, tok := range tokens {
				if tok.Value == "# this is a comment" || tok.Value == "# inline comment" {
					t.Errorf("comment text appeared as token value: %v", tok)
				}
			}
		})
	}

	// Verify that code before a comment is still tokenized
	t.Run("code before comment is tokenized", func(t *testing.T) {
		tokens := collectTokens("x = 1 # inline comment")
		foundName := false
		foundEqual := false
		foundInt := false
		for _, tok := range tokens {
			switch tok.Type {
			case NAME:
				if tok.Value == "x" {
					foundName = true
				}
			case EQUAL:
				foundEqual = true
			case INTEGER:
				if tok.Value == "1" {
					foundInt = true
				}
			}
		}
		if !foundName || !foundEqual || !foundInt {
			t.Errorf("expected x, =, 1 tokens; name=%v equal=%v int=%v; tokens: %v",
				foundName, foundEqual, foundInt, tokens)
		}
	})
}

func TestMultiCharOperatorPrecedence(t *testing.T) {
	// Ensure multi-char operators are preferred over single-char
	tests := []struct {
		input string
		want  []TokenType
	}{
		{"**=", []TokenType{DOUBLESTAREQUAL}},
		{"//=", []TokenType{DOUBLESLASHEQUAL}},
		{"<<=", []TokenType{LSHIFTEQUAL}},
		{">>=", []TokenType{RSHIFTEQUAL}},
		{"**", []TokenType{DOUBLESTAR}},
		{"//", []TokenType{DOUBLESLASH}},
		{"==", []TokenType{EQEQUAL}},
		{"!=", []TokenType{NOTEQUAL}},
		{"<=", []TokenType{LESSEQUAL}},
		{">=", []TokenType{GREATEREQUAL}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens := collectTokens(tt.input)
			idx := 0
			for _, tok := range tokens {
				if tok.Type == NEWLINE || tok.Type == EOF {
					continue
				}
				if idx >= len(tt.want) {
					t.Errorf("too many non-structural tokens")
					break
				}
				if tok.Type != tt.want[idx] {
					t.Errorf("token %d: got %v, want %v", idx, tok.Type, tt.want[idx])
				}
				idx++
			}
			if idx < len(tt.want) {
				t.Errorf("only found %d tokens, want %d", idx, len(tt.want))
			}
		})
	}
}

func TestExpressionTokenization(t *testing.T) {
	// Test a full expression
	tokens := collectTokens("x + y * 2")
	expected := []struct {
		tt  TokenType
		val string
	}{
		{NAME, "x"},
		{PLUS, "+"},
		{NAME, "y"},
		{STAR, "*"},
		{INTEGER, "2"},
	}

	idx := 0
	for _, tok := range tokens {
		if tok.Type == NEWLINE || tok.Type == EOF {
			continue
		}
		if idx >= len(expected) {
			t.Fatalf("unexpected extra token: %v", tok)
		}
		if tok.Type != expected[idx].tt || tok.Value != expected[idx].val {
			t.Errorf("token %d: got (%v, %q), want (%v, %q)",
				idx, tok.Type, tok.Value, expected[idx].tt, expected[idx].val)
		}
		idx++
	}
	if idx < len(expected) {
		t.Errorf("only got %d tokens, want %d", idx, len(expected))
	}
}

func TestFunctionDefTokenization(t *testing.T) {
	source := "def add(a, b):"
	tokens := collectTokens(source)
	expected := []struct {
		tt  TokenType
		val string
	}{
		{DEF, "def"},
		{NAME, "add"},
		{LPAREN, "("},
		{NAME, "a"},
		{COMMA, ","},
		{NAME, "b"},
		{RPAREN, ")"},
		{COLON, ":"},
	}

	idx := 0
	for _, tok := range tokens {
		if tok.Type == NEWLINE || tok.Type == EOF {
			continue
		}
		if idx >= len(expected) {
			t.Fatalf("unexpected extra token: %v", tok)
		}
		if tok.Type != expected[idx].tt || tok.Value != expected[idx].val {
			t.Errorf("token %d: got (%v, %q), want (%v, %q)",
				idx, tok.Type, tok.Value, expected[idx].tt, expected[idx].val)
		}
		idx++
	}
	if idx < len(expected) {
		t.Errorf("only got %d tokens, want %d", idx, len(expected))
	}
}

func TestPositionTracking(t *testing.T) {
	// First token should be at line 1, column 1
	tokens := collectTokens("x = 1")
	if len(tokens) == 0 {
		t.Fatal("no tokens")
	}

	// Find the first meaningful token
	var firstTok Token
	for _, tok := range tokens {
		if tok.Type != EOF && tok.Type != NEWLINE {
			firstTok = tok
			break
		}
	}

	if firstTok.Pos.Line != 1 {
		t.Errorf("first token line: got %d, want 1", firstTok.Pos.Line)
	}
	if firstTok.Pos.Column != 1 {
		t.Errorf("first token column: got %d, want 1", firstTok.Pos.Column)
	}
	if firstTok.Pos.Filename != "<test>" {
		t.Errorf("filename: got %q, want %q", firstTok.Pos.Filename, "<test>")
	}

	// Test that '=' is at column 3 (after "x ")
	var equalTok Token
	for _, tok := range tokens {
		if tok.Type == EQUAL {
			equalTok = tok
			break
		}
	}
	if equalTok.Pos.Column != 3 {
		t.Errorf("'=' column: got %d, want 3", equalTok.Pos.Column)
	}

	// Test that '1' is at column 5 (after "x = ")
	var intTok Token
	for _, tok := range tokens {
		if tok.Type == INTEGER {
			intTok = tok
			break
		}
	}
	if intTok.Pos.Column != 5 {
		t.Errorf("'1' column: got %d, want 5", intTok.Pos.Column)
	}
}

func TestEmptySource(t *testing.T) {
	tokens := collectTokens("")
	if len(tokens) == 0 {
		t.Fatal("expected at least EOF")
	}
	if tokens[len(tokens)-1].Type != EOF {
		t.Errorf("last token should be EOF, got %v", tokens[len(tokens)-1].Type)
	}
}

func TestPeek(t *testing.T) {
	l := New("<test>", "x + y")
	// Peek should not consume
	tok1 := l.Peek()
	tok2 := l.Peek()
	if tok1.Type != tok2.Type || tok1.Value != tok2.Value {
		t.Errorf("Peek not idempotent: %v vs %v", tok1, tok2)
	}
	// Next should return the same token
	tok3 := l.Next()
	if tok1.Type != tok3.Type || tok1.Value != tok3.Value {
		t.Errorf("Peek/Next mismatch: Peek=%v, Next=%v", tok1, tok3)
	}
}

func TestAugmentedAssignmentOperators(t *testing.T) {
	tests := []struct {
		input string
		want  TokenType
	}{
		{"+=", PLUSEQUAL},
		{"-=", MINUSEQUAL},
		{"*=", STAREQUAL},
		{"/=", SLASHEQUAL},
		{"//=", DOUBLESLASHEQUAL},
		{"%=", PERCENTEQUAL},
		{"**=", DOUBLESTAREQUAL},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens := collectTokens("x " + tt.input + " 1")
			found := false
			for _, tok := range tokens {
				if tok.Type == tt.want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %v in 'x %s 1'; got: %v", tt.want, tt.input, tokens)
			}
		})
	}
}

func TestBitwiseOperators(t *testing.T) {
	tests := []struct {
		input string
		want  TokenType
	}{
		{"&", AMPER},
		{"|", PIPE},
		{"^", CARET},
		{"~", TILDE},
		{"<<", LSHIFT},
		{">>", RSHIFT},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens := collectTokens(tt.input)
			found := false
			for _, tok := range tokens {
				if tok.Type == tt.want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %v; got: %v", tt.want, tokens)
			}
		})
	}
}


// --- INDENT/DEDENT and NEWLINE tests ---

func TestBasicIndentDedent(t *testing.T) {
	source := "if x:\n    y = 1\nz = 2\n"
	types := collectTokenTypes(source)

	// Expected: IF NAME COLON NEWLINE INDENT NAME EQUAL INTEGER NEWLINE DEDENT NAME EQUAL INTEGER NEWLINE EOF
	expected := []TokenType{
		IF, NAME, COLON, NEWLINE,
		INDENT, NAME, EQUAL, INTEGER, NEWLINE,
		DEDENT, NAME, EQUAL, INTEGER, NEWLINE,
		EOF,
	}

	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}

func TestMultipleDedents(t *testing.T) {
	// Go back multiple indent levels at once: should emit multiple DEDENT tokens.
	source := "if x:\n    if y:\n        z = 1\na = 2\n"
	types := collectTokenTypes(source)

	// Expected:
	// IF NAME COLON NEWLINE
	// INDENT IF NAME COLON NEWLINE
	// INDENT NAME EQUAL INTEGER NEWLINE
	// DEDENT DEDENT NAME EQUAL INTEGER NEWLINE
	// EOF
	expected := []TokenType{
		IF, NAME, COLON, NEWLINE,
		INDENT, IF, NAME, COLON, NEWLINE,
		INDENT, NAME, EQUAL, INTEGER, NEWLINE,
		DEDENT, DEDENT, NAME, EQUAL, INTEGER, NEWLINE,
		EOF,
	}

	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}

func TestEOFDedents(t *testing.T) {
	// EOF should emit DEDENT tokens for all open indent levels.
	source := "if x:\n    if y:\n        z = 1"
	types := collectTokenTypes(source)

	// Expected:
	// IF NAME COLON NEWLINE
	// INDENT IF NAME COLON NEWLINE
	// INDENT NAME EQUAL INTEGER
	// DEDENT DEDENT EOF
	expected := []TokenType{
		IF, NAME, COLON, NEWLINE,
		INDENT, IF, NAME, COLON, NEWLINE,
		INDENT, NAME, EQUAL, INTEGER,
		DEDENT, DEDENT, EOF,
	}

	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}

func TestNewlineSuppressedInParens(t *testing.T) {
	// Newlines inside parentheses should not produce NEWLINE tokens.
	source := "x = (1 +\n2)\n"
	types := collectTokenTypes(source)

	// Expected: NAME EQUAL LPAREN INTEGER PLUS INTEGER RPAREN NEWLINE EOF
	expected := []TokenType{
		NAME, EQUAL, LPAREN, INTEGER, PLUS,
		INTEGER, RPAREN, NEWLINE,
		EOF,
	}

	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}

func TestNewlineSuppressedInBrackets(t *testing.T) {
	// Newlines inside square brackets should not produce NEWLINE tokens.
	source := "x = [1,\n2,\n3]\n"
	types := collectTokenTypes(source)

	// Expected: NAME EQUAL LBRACKET INTEGER COMMA INTEGER COMMA INTEGER RBRACKET NEWLINE EOF
	expected := []TokenType{
		NAME, EQUAL, LBRACKET, INTEGER, COMMA,
		INTEGER, COMMA,
		INTEGER, RBRACKET, NEWLINE,
		EOF,
	}

	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}

func TestNewlineSuppressedInBraces(t *testing.T) {
	// Newlines inside braces should not produce NEWLINE tokens.
	source := "x = {\n'a': 1,\n'b': 2}\n"
	types := collectTokenTypes(source)

	// Expected: NAME EQUAL LBRACE STRING COLON INTEGER COMMA STRING COLON INTEGER RBRACE NEWLINE EOF
	expected := []TokenType{
		NAME, EQUAL, LBRACE,
		STRING, COLON, INTEGER, COMMA,
		STRING, COLON, INTEGER, RBRACE, NEWLINE,
		EOF,
	}

	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}

func TestExplicitLineContinuation(t *testing.T) {
	// Backslash-newline should not produce NEWLINE or affect indentation.
	source := "x = 1 +\\\n    2\n"
	types := collectTokenTypes(source)

	// Expected: NAME EQUAL INTEGER PLUS INTEGER NEWLINE EOF
	// The "    2" is NOT an indent because it's a continuation.
	expected := []TokenType{
		NAME, EQUAL, INTEGER, PLUS, INTEGER, NEWLINE,
		EOF,
	}

	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}

func TestBlankLinesIgnored(t *testing.T) {
	// Blank lines between statements should not generate NEWLINE or INDENT/DEDENT.
	source := "x = 1\n\n\ny = 2\n"
	types := collectTokenTypes(source)

	// Expected: NAME EQUAL INTEGER NEWLINE NAME EQUAL INTEGER NEWLINE EOF
	expected := []TokenType{
		NAME, EQUAL, INTEGER, NEWLINE,
		NAME, EQUAL, INTEGER, NEWLINE,
		EOF,
	}

	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}

func TestCommentOnlyLinesIgnored(t *testing.T) {
	// Comment-only lines should not generate NEWLINE or affect indentation.
	source := "x = 1\n# this is a comment\ny = 2\n"
	types := collectTokenTypes(source)

	// Expected: NAME EQUAL INTEGER NEWLINE NAME EQUAL INTEGER NEWLINE EOF
	expected := []TokenType{
		NAME, EQUAL, INTEGER, NEWLINE,
		NAME, EQUAL, INTEGER, NEWLINE,
		EOF,
	}

	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}

func TestBlankLineInsideIndentedBlock(t *testing.T) {
	// A blank line inside an indented block should not trigger DEDENT.
	source := "if x:\n    a = 1\n\n    b = 2\n"
	types := collectTokenTypes(source)

	// Expected: IF NAME COLON NEWLINE INDENT NAME EQUAL INTEGER NEWLINE NAME EQUAL INTEGER NEWLINE DEDENT EOF
	expected := []TokenType{
		IF, NAME, COLON, NEWLINE,
		INDENT, NAME, EQUAL, INTEGER, NEWLINE,
		NAME, EQUAL, INTEGER, NEWLINE,
		DEDENT, EOF,
	}

	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}

func TestTabSpaceMixingDetected(t *testing.T) {
	// Mixing tabs and spaces in indentation should report an error.
	source := "if x:\n \ty = 1\n"
	l := New("<test>", source)
	for {
		tok := l.Next()
		if tok.Type == EOF {
			break
		}
	}
	errors := l.Errors()
	if len(errors) == 0 {
		t.Fatal("expected error for mixed tabs and spaces, got none")
	}
	found := false
	for _, e := range errors {
		if e.Message == "inconsistent use of tabs and spaces in indentation" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'inconsistent use of tabs and spaces in indentation' error, got: %v", errors)
	}
}

func TestDedentBeforeContent(t *testing.T) {
	// DEDENT tokens must appear before the content on the dedented line.
	source := "if x:\n    y = 1\nz = 2\n"
	tokens := collectTokens(source)

	// Find the DEDENT token index and the 'z' NAME token index.
	dedentIdx := -1
	zIdx := -1
	for i, tok := range tokens {
		if tok.Type == DEDENT && dedentIdx == -1 {
			dedentIdx = i
		}
		if tok.Type == NAME && tok.Value == "z" {
			zIdx = i
		}
	}

	if dedentIdx == -1 {
		t.Fatal("no DEDENT token found")
	}
	if zIdx == -1 {
		t.Fatal("no 'z' NAME token found")
	}
	if dedentIdx >= zIdx {
		t.Errorf("DEDENT (index %d) should come before 'z' (index %d)", dedentIdx, zIdx)
	}
}

func TestNestedFunctionIndentation(t *testing.T) {
	// Test a realistic Python function with nested indentation.
	source := "def foo():\n    if True:\n        return 1\n    return 0\n"
	types := collectTokenTypes(source)

	expected := []TokenType{
		DEF, NAME, LPAREN, RPAREN, COLON, NEWLINE,
		INDENT, IF, TRUE, COLON, NEWLINE,
		INDENT, RETURN, INTEGER, NEWLINE,
		DEDENT, RETURN, INTEGER, NEWLINE,
		DEDENT, EOF,
	}

	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}

// --- Tests for literal tokenization (integers, floats, strings) ---

func TestIntegerLiterals(t *testing.T) {
	tests := []struct {
		input string
		value string
	}{
		// Decimal
		{"42", "42"},
		{"0", "0"},
		{"1_000_000", "1_000_000"},
		{"100_000", "100_000"},
		// Hex
		{"0xFF", "0xFF"},
		{"0XFF", "0XFF"},
		{"0xff", "0xff"},
		{"0xFF_FF", "0xFF_FF"},
		{"0x0", "0x0"},
		// Octal
		{"0o77", "0o77"},
		{"0O77", "0O77"},
		{"0o7_7", "0o7_7"},
		// Binary
		{"0b1010", "0b1010"},
		{"0B1010", "0B1010"},
		{"0b1010_0101", "0b1010_0101"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens := collectTokens(tt.input)
			found := false
			for _, tok := range tokens {
				if tok.Type == INTEGER {
					if tok.Value != tt.value {
						t.Errorf("got value %q, want %q", tok.Value, tt.value)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no INTEGER token found in %v", tokens)
			}
		})
	}
}

func TestFloatLiterals(t *testing.T) {
	tests := []struct {
		input string
		value string
	}{
		{"3.14", "3.14"},
		{"1e10", "1e10"},
		{"1E10", "1E10"},
		{"1.5e-3", "1.5e-3"},
		{"1.5e+3", "1.5e+3"},
		{".5", ".5"},
		{"1.", "1."},
		{"1_000.5e-3", "1_000.5e-3"},
		{"1_000.5", "1_000.5"},
		{"0.0", "0.0"},
		{"1e100", "1e100"},
		{"2.5E10", "2.5E10"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens := collectTokens(tt.input)
			found := false
			for _, tok := range tokens {
				if tok.Type == FLOAT {
					if tok.Value != tt.value {
						t.Errorf("got value %q, want %q", tok.Value, tt.value)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no FLOAT token found; got: %v", tokens)
			}
		})
	}
}

func TestStringLiteralsSingleQuoted(t *testing.T) {
	tests := []struct {
		name  string
		input string
		value string
	}{
		{"single quoted", `'hello'`, "hello"},
		{"double quoted", `"hello"`, "hello"},
		{"empty single", `''`, ""},
		{"empty double", `""`, ""},
		{"with spaces", `"hello world"`, "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := collectTokens(tt.input)
			found := false
			for _, tok := range tokens {
				if tok.Type == STRING {
					if tok.Value != tt.value {
						t.Errorf("got value %q, want %q", tok.Value, tt.value)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no STRING token found in %v", tokens)
			}
		})
	}
}

func TestTripleQuotedStrings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		value string
	}{
		{"triple double", `"""hello"""`, "hello"},
		{"triple single", `'''hello'''`, "hello"},
		{"triple multiline", "\"\"\"line1\nline2\"\"\"", "line1\nline2"},
		{"triple with single quotes inside", `"""it's ok"""`, "it's ok"},
		{"triple empty", `""""""`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := collectTokens(tt.input)
			found := false
			for _, tok := range tokens {
				if tok.Type == STRING {
					if tok.Value != tt.value {
						t.Errorf("got value %q, want %q", tok.Value, tt.value)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no STRING token found; got: %v", tokens)
			}
		})
	}
}

func TestEscapeSequences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		value string
	}{
		{"newline", `"\n"`, "\n"},
		{"tab", `"\t"`, "\t"},
		{"backslash", `"\\"`, "\\"},
		{"single quote in double", `"\'"`, "'"},
		{"double quote escape", `"\""`, "\""},
		{"carriage return", `"\r"`, "\r"},
		{"null", `"\0"`, "\x00"},
		{"bell", `"\a"`, "\a"},
		{"backspace", `"\b"`, "\b"},
		{"form feed", `"\f"`, "\f"},
		{"vertical tab", `"\v"`, "\v"},
		{"hex escape", `"\x41"`, "A"},
		{"hex escape lower", `"\x61"`, "a"},
		{"unicode 4 digit", `"\u0041"`, "A"},
		{"unicode 4 digit euro", `"\u20ac"`, "€"},
		{"unicode 8 digit", `"\U00000041"`, "A"},
		{"unicode 8 digit emoji", `"\U0001f600"`, "\U0001f600"},
		{"unknown escape", `"\q"`, "\\q"},
		{"multiple escapes", `"\n\t"`, "\n\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := collectTokens(tt.input)
			found := false
			for _, tok := range tokens {
				if tok.Type == STRING {
					if tok.Value != tt.value {
						t.Errorf("got value %q, want %q", tok.Value, tt.value)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no STRING token found; got: %v", tokens)
			}
		})
	}
}

func TestRawStrings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		value string
	}{
		{"raw single", `r"hello\n"`, `hello\n`},
		{"raw double", `r'hello\t'`, `hello\t`},
		{"raw uppercase", `R"hello\n"`, `hello\n`},
		{"raw with backslash", `r"C:\Users\test"`, `C:\Users\test`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := collectTokens(tt.input)
			found := false
			for _, tok := range tokens {
				if tok.Type == STRING {
					if tok.Value != tt.value {
						t.Errorf("got value %q, want %q", tok.Value, tt.value)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no STRING token found; got: %v", tokens)
			}
		})
	}
}

func TestBytesLiterals(t *testing.T) {
	tests := []struct {
		name  string
		input string
		value string
		typ   TokenType
	}{
		{"bytes single", `b"hello"`, "hello", BYTES},
		{"bytes double", `b'hello'`, "hello", BYTES},
		{"bytes uppercase", `B"hello"`, "hello", BYTES},
		{"raw bytes", `rb"hello\n"`, `hello\n`, BYTES},
		{"bytes raw", `br"hello\n"`, `hello\n`, BYTES},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := collectTokens(tt.input)
			found := false
			for _, tok := range tokens {
				if tok.Type == tt.typ {
					if tok.Value != tt.value {
						t.Errorf("got value %q, want %q", tok.Value, tt.value)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no %v token found; got: %v", tt.typ, tokens)
			}
		})
	}
}

func TestUnterminatedStrings(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unterminated single", `"hello`},
		{"unterminated single quote", `'hello`},
		{"unterminated with newline", "\"hello\n"},
		{"unterminated triple", `"""hello`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New("<test>", tt.input)
			// Consume all tokens
			for {
				tok := l.Next()
				if tok.Type == EOF {
					break
				}
			}
			// Should have errors
			if len(l.Errors()) == 0 {
				t.Error("expected SyntaxError for unterminated string, got none")
			}
			// Check error message contains SyntaxError
			foundSyntaxErr := false
			for _, err := range l.Errors() {
				if strings.Contains(err.Message, "SyntaxError") && strings.Contains(err.Message, "unterminated") {
					foundSyntaxErr = true
					break
				}
			}
			if !foundSyntaxErr {
				t.Errorf("expected SyntaxError about unterminated string; got errors: %v", l.Errors())
			}
		})
	}
}

func TestUnterminatedStringPosition(t *testing.T) {
	// Verify that unterminated string error reports the correct position
	l := New("<test>", `x = "hello`)
	for {
		tok := l.Next()
		if tok.Type == EOF {
			break
		}
	}
	errors := l.Errors()
	if len(errors) == 0 {
		t.Fatal("expected error for unterminated string")
	}
	// The string starts at column 5 (after "x = ")
	if errors[0].Col != 5 {
		t.Errorf("error column: got %d, want 5", errors[0].Col)
	}
	if errors[0].Line != 1 {
		t.Errorf("error line: got %d, want 1", errors[0].Line)
	}
}

func TestFStringPrefix(t *testing.T) {
	// f-strings should be recognized as STRING tokens (f-string expression parsing is Phase 2)
	tokens := collectTokens(`f"hello {name}"`)
	found := false
	for _, tok := range tokens {
		if tok.Type == STRING {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected STRING token for f-string; got: %v", tokens)
	}
}

func TestNumberInExpression(t *testing.T) {
	// Verify numbers work correctly in expressions
	tokens := collectTokens("x = 0xFF + 3.14")
	expected := []struct {
		tt  TokenType
		val string
	}{
		{NAME, "x"},
		{EQUAL, "="},
		{INTEGER, "0xFF"},
		{PLUS, "+"},
		{FLOAT, "3.14"},
	}

	idx := 0
	for _, tok := range tokens {
		if tok.Type == NEWLINE || tok.Type == EOF {
			continue
		}
		if idx >= len(expected) {
			t.Fatalf("unexpected extra token: %v", tok)
		}
		if tok.Type != expected[idx].tt || tok.Value != expected[idx].val {
			t.Errorf("token %d: got (%v, %q), want (%v, %q)",
				idx, tok.Type, tok.Value, expected[idx].tt, expected[idx].val)
		}
		idx++
	}
}

func TestDotAsOperatorNotFloat(t *testing.T) {
	// A dot that's not followed by a digit should be DOT, not FLOAT
	tokens := collectTokens("x.y")
	expected := []struct {
		tt  TokenType
		val string
	}{
		{NAME, "x"},
		{DOT, "."},
		{NAME, "y"},
	}

	idx := 0
	for _, tok := range tokens {
		if tok.Type == NEWLINE || tok.Type == EOF {
			continue
		}
		if idx >= len(expected) {
			t.Fatalf("unexpected extra token: %v", tok)
		}
		if tok.Type != expected[idx].tt || tok.Value != expected[idx].val {
			t.Errorf("token %d: got (%v, %q), want (%v, %q)",
				idx, tok.Type, tok.Value, expected[idx].tt, expected[idx].val)
		}
		idx++
	}
}

// --- Additional edge case tests for CRLF, multi-line positions, error cases ---

func TestCRLFLineEndings(t *testing.T) {
	// CRLF should be handled the same as LF for line separation.
	source := "x = 1\r\ny = 2\r\n"
	types := collectTokenTypes(source)

	expected := []TokenType{
		NAME, EQUAL, INTEGER, NEWLINE,
		NAME, EQUAL, INTEGER, NEWLINE,
		EOF,
	}

	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}

func TestCRLFIndentation(t *testing.T) {
	// INDENT/DEDENT should work with CRLF line endings.
	source := "if x:\r\n    y = 1\r\nz = 2\r\n"
	types := collectTokenTypes(source)

	expected := []TokenType{
		IF, NAME, COLON, NEWLINE,
		INDENT, NAME, EQUAL, INTEGER, NEWLINE,
		DEDENT, NAME, EQUAL, INTEGER, NEWLINE,
		EOF,
	}

	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}

func TestMultiLinePositionTracking(t *testing.T) {
	// Verify positions are tracked correctly across multiple lines.
	source := "x = 1\ny = 2\nz = 3\n"
	tokens := collectTokens(source)

	// 'y' should be on line 2, column 1.
	var yTok Token
	for _, tok := range tokens {
		if tok.Type == NAME && tok.Value == "y" {
			yTok = tok
			break
		}
	}
	if yTok.Pos.Line != 2 {
		t.Errorf("'y' line: got %d, want 2", yTok.Pos.Line)
	}
	if yTok.Pos.Column != 1 {
		t.Errorf("'y' column: got %d, want 1", yTok.Pos.Column)
	}

	// 'z' should be on line 3, column 1.
	var zTok Token
	for _, tok := range tokens {
		if tok.Type == NAME && tok.Value == "z" {
			zTok = tok
			break
		}
	}
	if zTok.Pos.Line != 3 {
		t.Errorf("'z' line: got %d, want 3", zTok.Pos.Line)
	}
	if zTok.Pos.Column != 1 {
		t.Errorf("'z' column: got %d, want 1", zTok.Pos.Column)
	}
}

func TestUnindentMismatchError(t *testing.T) {
	// Dedent to a level that doesn't match any outer indentation level.
	source := "if x:\n        y = 1\n   z = 2\n"
	l := New("<test>", source)
	for {
		tok := l.Next()
		if tok.Type == EOF {
			break
		}
	}
	errors := l.Errors()
	if len(errors) == 0 {
		t.Fatal("expected error for unindent mismatch, got none")
	}
	found := false
	for _, e := range errors {
		if strings.Contains(e.Message, "unindent does not match any outer indentation level") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'unindent does not match any outer indentation level' error; got: %v", errors)
	}
}

func TestNestedBracketLineContinuation(t *testing.T) {
	// Nested brackets should all suppress NEWLINE.
	source := "x = ([\n1,\n2\n])\n"
	types := collectTokenTypes(source)

	// No NEWLINE inside ([...])
	expected := []TokenType{
		NAME, EQUAL, LPAREN, LBRACKET,
		INTEGER, COMMA,
		INTEGER,
		RBRACKET, RPAREN, NEWLINE,
		EOF,
	}

	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}

func TestCRLFExplicitLineContinuation(t *testing.T) {
	// Backslash followed by CRLF should be treated as line continuation.
	source := "x = 1 +\\\r\n    2\r\n"
	types := collectTokenTypes(source)

	expected := []TokenType{
		NAME, EQUAL, INTEGER, PLUS, INTEGER, NEWLINE,
		EOF,
	}

	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}

func TestPositionAfterCRLF(t *testing.T) {
	// Positions should be correct after CRLF line endings.
	source := "x = 1\r\ny = 2\r\n"
	tokens := collectTokens(source)

	var yTok Token
	for _, tok := range tokens {
		if tok.Type == NAME && tok.Value == "y" {
			yTok = tok
			break
		}
	}
	if yTok.Pos.Line != 2 {
		t.Errorf("'y' line after CRLF: got %d, want 2", yTok.Pos.Line)
	}
	if yTok.Pos.Column != 1 {
		t.Errorf("'y' column after CRLF: got %d, want 1", yTok.Pos.Column)
	}
}

func TestErrorPositionOnSecondLine(t *testing.T) {
	// Error positions should report the correct line for errors on line 2+.
	source := "x = 1\ny = \"unterminated\n"
	l := New("<test>", source)
	for {
		tok := l.Next()
		if tok.Type == EOF {
			break
		}
	}
	errors := l.Errors()
	if len(errors) == 0 {
		t.Fatal("expected error for unterminated string on line 2")
	}
	if errors[0].Line != 2 {
		t.Errorf("error line: got %d, want 2", errors[0].Line)
	}
	if errors[0].Col != 5 {
		t.Errorf("error column: got %d, want 5", errors[0].Col)
	}
}

func TestSemicolonDelimiter(t *testing.T) {
	// Semicolons should be tokenized as SEMICOLON.
	tokens := collectTokens("x = 1; y = 2")
	found := false
	for _, tok := range tokens {
		if tok.Type == SEMICOLON {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected SEMICOLON token; got: %v", tokens)
	}
}

func TestIndentWithTabsOnly(t *testing.T) {
	// Tabs-only indentation should work without errors.
	source := "if x:\n\ty = 1\nz = 2\n"
	l := New("<test>", source)
	for {
		tok := l.Next()
		if tok.Type == EOF {
			break
		}
	}
	if len(l.Errors()) != 0 {
		t.Errorf("expected no errors for tabs-only indentation; got: %v", l.Errors())
	}

	// Also verify correct token types.
	types := collectTokenTypes(source)
	expected := []TokenType{
		IF, NAME, COLON, NEWLINE,
		INDENT, NAME, EQUAL, INTEGER, NEWLINE,
		DEDENT, NAME, EQUAL, INTEGER, NEWLINE,
		EOF,
	}
	if len(types) != len(expected) {
		t.Fatalf("got %d tokens, want %d\ntokens: %v", len(types), len(expected), types)
	}
	for i, tt := range expected {
		if types[i] != tt {
			t.Errorf("token %d: got %v, want %v", i, types[i], tt)
		}
	}
}
