// Package lexer converts Python 3 source text into a stream of tokens.
// It handles all Python lexical elements: keywords, identifiers, literals,
// operators, delimiters, and synthetic INDENT/DEDENT tokens for indentation.
//
// The lexer is the first stage of the PyGo execution pipeline:
//
//	source text → Lexer → []Token → Parser
package lexer

import "fmt"

// TokenType identifies the category of a token.
type TokenType int

const (
	// Structural
	ILLEGAL TokenType = iota
	EOF
	NEWLINE   // logical line ending
	INDENT    // increased indentation (synthetic)
	DEDENT    // decreased indentation (synthetic)
	SEMICOLON // explicit statement separator

	// Literals
	INTEGER // 42, 0xFF, 0o77, 0b1010, 1_000_000
	FLOAT   // 3.14, 1e10, 1.5e-3
	COMPLEX // 3+4j
	STRING  // "hello", 'world', """triple""", r"raw", b"bytes", f"fstring"
	BYTES   // b"..." treated separately for clarity

	// Identifiers and keywords
	NAME // any identifier that is not a keyword

	// Keywords (alphabetical)
	AND
	AS
	ASSERT
	ASYNC
	AWAIT
	BREAK
	CLASS
	CONTINUE
	DEF
	DEL
	ELIF
	ELSE
	EXCEPT
	FALSE
	FINALLY
	FOR
	FROM
	GLOBAL
	IF
	IMPORT
	IN
	IS
	LAMBDA
	MATCH  // Python 3.10+ soft keyword
	NONE
	NONLOCAL
	NOT
	OR
	PASS
	RAISE
	RETURN
	TRUE
	TRY
	TYPE   // Python 3.12+ soft keyword
	WHILE
	WITH
	YIELD

	// Operators
	PLUS        // +
	MINUS       // -
	STAR        // *
	DOUBLESTAR  // **
	SLASH       // /
	DOUBLESLASH // //
	PERCENT     // %
	AT          // @ (matrix multiply / decorator)
	AMPER       // &
	PIPE        // |
	CARET       // ^
	TILDE       // ~
	LSHIFT      // <<
	RSHIFT      // >>

	// Augmented assignment operators
	PLUSEQUAL        // +=
	MINUSEQUAL       // -=
	STAREQUAL        // *=
	DOUBLESTAREQUAL  // **=
	SLASHEQUAL       // /=
	DOUBLESLASHEQUAL // //=
	PERCENTEQUAL     // %=
	ATEQUAL          // @=
	AMPEREQUAL       // &=
	PIPEEQUAL        // |=
	CARETEQUAL       // ^=
	LSHIFTEQUAL      // <<=
	RSHIFTEQUAL      // >>=

	// Comparison operators
	EQEQUAL    // ==
	NOTEQUAL   // !=
	LESS       // <
	GREATER    // >
	LESSEQUAL  // <=
	GREATEREQUAL // >=

	// Assignment
	EQUAL    // =
	WALRUS   // :=  (Python 3.8+)
	RARROW   // ->  (return type annotation)

	// Delimiters
	LPAREN    // (
	RPAREN    // )
	LBRACKET  // [
	RBRACKET  // ]
	LBRACE    // {
	RBRACE    // }
	COMMA     // ,
	COLON     // :
	DOT       // .
	ELLIPSIS  // ...
	BACKSLASH // \ (line continuation — consumed by lexer, not emitted)
)

// Token is a single lexical element of Python source.
type Token struct {
	Type    TokenType
	Value   string // raw source text of the token
	Line    int    // 1-indexed source line
	Col     int    // 1-indexed column of the first character
	EndLine int    // 1-indexed line of the last character
	EndCol  int    // 1-indexed column after the last character
}

// String returns a human-readable representation of the token, useful in
// error messages and debug output.
func (t Token) String() string {
	return fmt.Sprintf("Token(%s, %q, %d:%d)", t.Type, t.Value, t.Line, t.Col)
}

// IsKeyword reports whether the token is a Python keyword.
func (t Token) IsKeyword() bool {
	return t.Type >= AND && t.Type <= YIELD
}

// String returns the canonical name of the token type.
func (tt TokenType) String() string {
	if name, ok := tokenTypeNames[tt]; ok {
		return name
	}
	return fmt.Sprintf("TokenType(%d)", int(tt))
}

// keywords maps Python keyword strings to their token types.
// Soft keywords (match, type, case) are handled contextually by the parser.
var keywords = map[string]TokenType{
	"and":      AND,
	"as":       AS,
	"assert":   ASSERT,
	"async":    ASYNC,
	"await":    AWAIT,
	"break":    BREAK,
	"class":    CLASS,
	"continue": CONTINUE,
	"def":      DEF,
	"del":      DEL,
	"elif":     ELIF,
	"else":     ELSE,
	"except":   EXCEPT,
	"False":    FALSE,
	"finally":  FINALLY,
	"for":      FOR,
	"from":     FROM,
	"global":   GLOBAL,
	"if":       IF,
	"import":   IMPORT,
	"in":       IN,
	"is":       IS,
	"lambda":   LAMBDA,
	"None":     NONE,
	"nonlocal": NONLOCAL,
	"not":      NOT,
	"or":       OR,
	"pass":     PASS,
	"raise":    RAISE,
	"return":   RETURN,
	"True":     TRUE,
	"try":      TRY,
	"while":    WHILE,
	"with":     WITH,
	"yield":    YIELD,
}

// LookupKeyword returns the token type for a keyword string.
// If the string is not a keyword, it returns NAME.
func LookupKeyword(ident string) TokenType {
	if tt, ok := keywords[ident]; ok {
		return tt
	}
	return NAME
}

var tokenTypeNames = map[TokenType]string{
	ILLEGAL: "ILLEGAL", EOF: "EOF", NEWLINE: "NEWLINE",
	INDENT: "INDENT", DEDENT: "DEDENT", SEMICOLON: "SEMICOLON",
	INTEGER: "INTEGER", FLOAT: "FLOAT", COMPLEX: "COMPLEX",
	STRING: "STRING", BYTES: "BYTES", NAME: "NAME",
	AND: "and", AS: "as", ASSERT: "assert", ASYNC: "async",
	AWAIT: "await", BREAK: "break", CLASS: "class", CONTINUE: "continue",
	DEF: "def", DEL: "del", ELIF: "elif", ELSE: "else",
	EXCEPT: "except", FALSE: "False", FINALLY: "finally", FOR: "for",
	FROM: "from", GLOBAL: "global", IF: "if", IMPORT: "import",
	IN: "in", IS: "is", LAMBDA: "lambda", NONE: "None",
	NONLOCAL: "nonlocal", NOT: "not", OR: "or", PASS: "pass",
	RAISE: "raise", RETURN: "return", TRUE: "True", TRY: "try",
	WHILE: "while", WITH: "with", YIELD: "yield",
	PLUS: "+", MINUS: "-", STAR: "*", DOUBLESTAR: "**",
	SLASH: "/", DOUBLESLASH: "//", PERCENT: "%", AT: "@",
	AMPER: "&", PIPE: "|", CARET: "^", TILDE: "~",
	LSHIFT: "<<", RSHIFT: ">>",
	PLUSEQUAL: "+=", MINUSEQUAL: "-=", STAREQUAL: "*=",
	DOUBLESTAREQUAL: "**=", SLASHEQUAL: "/=", DOUBLESLASHEQUAL: "//=",
	PERCENTEQUAL: "%=", ATEQUAL: "@=", AMPEREQUAL: "&=",
	PIPEEQUAL: "|=", CARETEQUAL: "^=", LSHIFTEQUAL: "<<=", RSHIFTEQUAL: ">>=",
	EQEQUAL: "==", NOTEQUAL: "!=", LESS: "<", GREATER: ">",
	LESSEQUAL: "<=", GREATEREQUAL: ">=",
	EQUAL: "=", WALRUS: ":=", RARROW: "->",
	LPAREN: "(", RPAREN: ")", LBRACKET: "[", RBRACKET: "]",
	LBRACE: "{", RBRACE: "}", COMMA: ",", COLON: ":",
	DOT: ".", ELLIPSIS: "...",
}
