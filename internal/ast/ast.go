// Package ast defines the Abstract Syntax Tree node types for Python 3 source.
//
// Every construct that can appear in a Python program is represented as a
// concrete Go struct implementing either the Stmt or Expr interface, both of
// which embed Node.
//
// The node set closely mirrors the CPython ast module (see the Python Language
// Reference §Abstract Syntax Trees), which makes compatibility testing against
// CPython straightforward.
//
// Node structure:
//
//	Node
//	├── Stmt    (statements)
//	│   ├── FunctionDef, AsyncFunctionDef
//	│   ├── ClassDef
//	│   ├── Return, Delete
//	│   ├── Assign, AugAssign, AnnAssign
//	│   ├── For, AsyncFor, While, If, With, AsyncWith
//	│   ├── Raise, Try, ExceptHandler
//	│   ├── Import, ImportFrom
//	│   ├── Global, Nonlocal
//	│   ├── Expr (expression used as statement)
//	│   ├── Pass, Break, Continue
//	│   └── Assert
//	│
//	└── Expr    (expressions)
//	    ├── BoolOp, BinOp, UnaryOp
//	    ├── IfExp, Lambda
//	    ├── Dict, Set, List, Tuple
//	    ├── ListComp, SetComp, DictComp, GeneratorExp
//	    ├── Await, Yield, YieldFrom
//	    ├── Compare, Call
//	    ├── Attribute, Subscript, Starred
//	    ├── Name, Constant
//	    └── NamedExpr (walrus :=)
package ast

import "fmt"

// Position holds source location information attached to every AST node.
type Position struct {
	Filename  string
	Line      int // 1-indexed
	Col       int // 1-indexed
	EndLine   int
	EndCol    int
}

func (p Position) String() string {
	return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Col)
}

// Node is the root interface for all AST nodes.
type Node interface {
	Pos() Position
	nodeMarker() // unexported to prevent external implementations
}

// Stmt is the interface for all statement nodes.
type Stmt interface {
	Node
	stmtMarker()
}

// Expr is the interface for all expression nodes.
type Expr interface {
	Node
	exprMarker()
}

// baseNode provides source position for all nodes.
type baseNode struct{ pos Position }

func (b baseNode) Pos() Position   { return b.pos }
func (b baseNode) nodeMarker()     {}

// baseStmt embeds baseNode and marks a node as a statement.
type baseStmt struct{ baseNode }

func (baseStmt) stmtMarker() {}

// baseExpr embeds baseNode and marks a node as an expression.
type baseExpr struct{ baseNode }

func (baseExpr) exprMarker() {}

// NewPos constructs a Position value.
func NewPos(filename string, line, col, endLine, endCol int) Position {
	return Position{Filename: filename, Line: line, Col: col, EndLine: endLine, EndCol: endCol}
}

// ============================================================
// Top-level
// ============================================================

// Module is the root node for a parsed Python module.
type Module struct {
	Body []Stmt
	pos  Position
}

func (m *Module) Pos() Position { return m.pos }
func (m *Module) nodeMarker()   {}

// NewModule constructs a Module node.
func NewModule(body []Stmt, pos Position) *Module {
	return &Module{Body: body, pos: pos}
}

// ============================================================
// Statements
// ============================================================

// FunctionDef represents a def statement.
//
//	def name(args) -> returns:
//	    body
type FunctionDef struct {
	baseStmt
	Name       string
	Args       *Arguments
	Body       []Stmt
	Decorators []Expr
	Returns    Expr // optional return type annotation
	IsAsync    bool // true for async def
}

// ClassDef represents a class statement.
//
//	class Name(bases, keywords):
//	    body
type ClassDef struct {
	baseStmt
	Name       string
	Bases      []Expr
	Keywords   []*Keyword
	Body       []Stmt
	Decorators []Expr
}

// Return represents a return statement.
type Return struct {
	baseStmt
	Value Expr // nil if bare return
}

// Delete represents a del statement.
type Delete struct {
	baseStmt
	Targets []Expr
}

// Assign represents a simple assignment: targets = value
type Assign struct {
	baseStmt
	Targets []Expr
	Value   Expr
}

// AugAssign represents an augmented assignment: target op= value
type AugAssign struct {
	baseStmt
	Target Expr
	Op     Operator
	Value  Expr
}

// AnnAssign represents an annotated assignment: target: annotation = value
type AnnAssign struct {
	baseStmt
	Target     Expr
	Annotation Expr
	Value      Expr // nil if no value
	Simple     bool // true when target is a simple name (not subscript/attr)
}

// For represents a for loop.
type For struct {
	baseStmt
	Target  Expr
	Iter    Expr
	Body    []Stmt
	Orelse  []Stmt // for/else clause
	IsAsync bool
}

// While represents a while loop.
type While struct {
	baseStmt
	Test   Expr
	Body   []Stmt
	Orelse []Stmt
}

// If represents an if/elif/else statement.
type If struct {
	baseStmt
	Test   Expr
	Body   []Stmt
	Orelse []Stmt // elif chain or else body
}

// With represents a with statement.
type With struct {
	baseStmt
	Items   []*WithItem
	Body    []Stmt
	IsAsync bool
}

// WithItem is one context manager in a with statement.
type WithItem struct {
	ContextExpr Expr
	OptionalVar Expr // the `as` target, or nil
}

// Raise represents a raise statement.
type Raise struct {
	baseStmt
	Exc   Expr // the exception to raise, or nil (bare raise)
	Cause Expr // the cause (from clause), or nil
}

// Try represents a try statement.
type Try struct {
	baseStmt
	Body      []Stmt
	Handlers  []*ExceptHandler
	Orelse    []Stmt // try/else
	Finalbody []Stmt // finally
}

// ExceptHandler is one except clause.
type ExceptHandler struct {
	baseStmt
	Type Expr   // the exception type, or nil (bare except)
	Name string // the as-name, or ""
	Body []Stmt
}

// Import represents an import statement.
type Import struct {
	baseStmt
	Names []*Alias
}

// ImportFrom represents a from ... import statement.
type ImportFrom struct {
	baseStmt
	Module string // module name, "" for relative-only imports
	Names  []*Alias
	Level  int // number of dots for relative import
}

// Alias is one name in an import statement.
type Alias struct {
	Name   string // original name
	AsName string // alias, or "" if no 'as' clause
}

// Global represents a global statement.
type Global struct {
	baseStmt
	Names []string
}

// Nonlocal represents a nonlocal statement.
type Nonlocal struct {
	baseStmt
	Names []string
}

// ExprStmt represents an expression used as a statement.
type ExprStmt struct {
	baseStmt
	Value Expr
}

// Pass represents a pass statement.
type Pass struct{ baseStmt }

// Break represents a break statement.
type Break struct{ baseStmt }

// Continue represents a continue statement.
type Continue struct{ baseStmt }

// Assert represents an assert statement.
type Assert struct {
	baseStmt
	Test Expr
	Msg  Expr // optional assertion message
}

// ============================================================
// Expressions
// ============================================================

// BoolOp represents boolean operations: a and b, a or b
type BoolOp struct {
	baseExpr
	Op     BoolOperator
	Values []Expr // two or more operands
}

// BinOp represents binary arithmetic/bitwise operations: a + b
type BinOp struct {
	baseExpr
	Left  Expr
	Op    Operator
	Right Expr
}

// UnaryOp represents unary operations: -x, not x, ~x
type UnaryOp struct {
	baseExpr
	Op      UnaryOperator
	Operand Expr
}

// IfExp represents a ternary conditional: body if test else orelse
type IfExp struct {
	baseExpr
	Test   Expr
	Body   Expr // value when test is true
	Orelse Expr // value when test is false
}

// Lambda represents a lambda expression.
type Lambda struct {
	baseExpr
	Args *Arguments
	Body Expr
}

// NamedExpr represents a walrus operator expression: name := value
type NamedExpr struct {
	baseExpr
	Target *Name
	Value  Expr
}

// Dict represents a dict literal: {k: v, ...}
type Dict struct {
	baseExpr
	Keys   []Expr // parallel with Values; nil key means **unpacking
	Values []Expr
}

// Set represents a set literal: {a, b, c}
type Set struct {
	baseExpr
	Elts []Expr
}

// ListComp represents a list comprehension: [elt for ... if ...]
type ListComp struct {
	baseExpr
	Elt        Expr
	Generators []*Comprehension
}

// SetComp represents a set comprehension: {elt for ... if ...}
type SetComp struct {
	baseExpr
	Elt        Expr
	Generators []*Comprehension
}

// DictComp represents a dict comprehension: {k: v for ... if ...}
type DictComp struct {
	baseExpr
	Key        Expr
	Value      Expr
	Generators []*Comprehension
}

// GeneratorExp represents a generator expression: (elt for ... if ...)
type GeneratorExp struct {
	baseExpr
	Elt        Expr
	Generators []*Comprehension
}

// Comprehension is a single for/if clause in a comprehension.
type Comprehension struct {
	Target Expr
	Iter   Expr
	Ifs    []Expr
	IsAsync bool
}

// Await represents an await expression.
type Await struct {
	baseExpr
	Value Expr
}

// Yield represents a yield expression.
type Yield struct {
	baseExpr
	Value Expr // nil for bare yield
}

// YieldFrom represents yield from expression.
type YieldFrom struct {
	baseExpr
	Value Expr
}

// Compare represents a comparison: a < b > c
type Compare struct {
	baseExpr
	Left        Expr
	Ops         []CmpOp
	Comparators []Expr
}

// Call represents a function call: func(args, *args, **kwargs)
type Call struct {
	baseExpr
	Func     Expr
	Args     []Expr
	Keywords []*Keyword
}

// Keyword is a keyword argument in a function call or class definition.
type Keyword struct {
	Arg   string // keyword name, or "" for **unpacking
	Value Expr
}

// Attribute represents attribute access: value.attr
type Attribute struct {
	baseExpr
	Value Expr
	Attr  string
	Ctx   ExprContext
}

// Subscript represents subscript access: value[slice]
type Subscript struct {
	baseExpr
	Value Expr
	Slice Expr
	Ctx   ExprContext
}

// Starred represents a starred expression: *x in assignments or calls
type Starred struct {
	baseExpr
	Value Expr
	Ctx   ExprContext
}

// Name represents an identifier.
type Name struct {
	baseExpr
	Id  string
	Ctx ExprContext
}

// List represents a list literal or list assignment target.
type List struct {
	baseExpr
	Elts []Expr
	Ctx  ExprContext
}

// Tuple represents a tuple literal or tuple assignment target.
type Tuple struct {
	baseExpr
	Elts []Expr
	Ctx  ExprContext
}

// Constant represents a literal constant value.
type Constant struct {
	baseExpr
	Value interface{} // int64, float64, complex128, string, bool, nil (for None)
	Kind  string      // "u" for unicode string prefix (rare), "" otherwise
}

// Slice represents a slice: lower:upper:step
type Slice struct {
	baseExpr
	Lower Expr
	Upper Expr
	Step  Expr
}

// ============================================================
// Function arguments
// ============================================================

// Arguments holds the full argument specification for a function definition.
type Arguments struct {
	PosonlyArgs []*Arg   // positional-only args (before /)
	Args        []*Arg   // regular positional args
	Vararg      *Arg     // *args, or nil
	KwonlyArgs  []*Arg   // keyword-only args (after *)
	KwDefaults  []Expr   // defaults for kw-only args (parallel; nil = no default)
	Kwarg       *Arg     // **kwargs, or nil
	Defaults    []Expr   // defaults for positional args (right-aligned)
}

// Arg is a single function parameter.
type Arg struct {
	Arg        string
	Annotation Expr // type annotation, or nil
	pos        Position
}

func (a *Arg) Pos() Position { return a.pos }

// ============================================================
// Operator enumerations
// ============================================================

// Operator enumerates binary operators.
type Operator int

const (
	Add Operator = iota
	Sub
	Mult
	MatMult
	Div
	Mod
	Pow
	LShift
	RShift
	BitOr
	BitXor
	BitAnd
	FloorDiv
)

func (o Operator) String() string {
	return [...]string{"Add", "Sub", "Mult", "MatMult", "Div", "Mod", "Pow",
		"LShift", "RShift", "BitOr", "BitXor", "BitAnd", "FloorDiv"}[o]
}

// UnaryOperator enumerates unary operators.
type UnaryOperator int

const (
	Invert UnaryOperator = iota
	Not
	UAdd
	USub
)

// BoolOperator enumerates boolean operators.
type BoolOperator int

const (
	And BoolOperator = iota
	Or
)

// CmpOp enumerates comparison operators.
type CmpOp int

const (
	Eq CmpOp = iota
	NotEq
	Lt
	LtE
	Gt
	GtE
	Is
	IsNot
	In
	NotIn
)

func (c CmpOp) String() string {
	return [...]string{"==", "!=", "<", "<=", ">", ">=", "is", "is not", "in", "not in"}[c]
}

// ExprContext indicates how an expression is used (load, store, delete).
type ExprContext int

const (
	Load ExprContext = iota
	Store
	Del
)

// ============================================================
// Position attachment helper
// ============================================================

// posSetter is implemented by all node types that embed baseNode
// (directly or via baseStmt / baseExpr). It allows external packages,
// such as the parser, to attach source positions to nodes they construct
// with struct literals.
type posSetter interface {
	setPos(Position)
}

func (b *baseNode) setPos(p Position) { b.pos = p }

// SetPos attaches a source position to a node and returns the node.
// It is used by the parser, which constructs nodes with struct literals
// (exported fields) and then records their source location.
//
// If node does not support position attachment, SetPos is a no-op.
func SetPos[T Node](node T, p Position) T {
	if s, ok := any(node).(posSetter); ok {
		s.setPos(p)
	}
	return node
}

// NewArg constructs an Arg with a position (the pos field is unexported).
func NewArg(name string, annotation Expr, p Position) *Arg {
	return &Arg{Arg: name, Annotation: annotation, pos: p}
}

