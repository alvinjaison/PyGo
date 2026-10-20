// Package compiler transforms an AST into bytecode (a *bytecode.Code object).
//
// It is the third stage of the PyGo pipeline:
//
//	ast.Module → Compiler → *bytecode.Code → VM
//
// Compilation is scope-aware: a symbol table pass (symtable.go) classifies
// each name as local, global, free (closure), or cell before code generation,
// so the code generator can emit the right LOAD_/STORE_ opcode.
package compiler

import (
	"fmt"

	"github.com/pygo/pygo/internal/ast"
	"github.com/pygo/pygo/internal/bytecode"
)

// Error is a compile-time error (e.g., an unsupported construct).
type Error struct {
	Message string
	Line    int
}

func (e *Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s", e.Line, e.Message)
	}
	return e.Message
}

// scopeKind identifies the kind of scope a compiler unit represents.
type scopeKind int

const (
	scopeModule scopeKind = iota
	scopeFunction
	scopeClass
	scopeComprehension
)

// compiler is a single compilation unit (one Code object under construction).
type compiler struct {
	parent *compiler
	kind   scopeKind

	name     string
	filename string

	instrs    []bytecode.Instruction
	constants []interface{}
	names     []string
	varnames  []string
	cellvars  []string
	freevars  []string

	symbols *symtable

	argCount   int
	kwOnlyArgs int
	flags      bytecode.CodeFlag

	firstLine int
	lineTable []int
	lastLine  int
	lastOff   int

	// loop stack for break/continue targets
	loops []loopContext

	err error
}

type loopContext struct {
	continueTarget int   // jump target for `continue`
	breakJumps     []int // patch locations for `break`
}

// Compile compiles a module AST into a top-level Code object.
func Compile(mod *ast.Module, filename string) (*bytecode.Code, error) {
	c := &compiler{
		kind:      scopeModule,
		name:      "<module>",
		filename:  filename,
		symbols:   buildSymtable(mod),
		firstLine: 1,
	}
	for _, stmt := range mod.Body {
		c.compileStmt(stmt)
		if c.err != nil {
			return nil, c.err
		}
	}
	// Module implicitly returns None.
	c.emit(bytecode.LOAD_CONST, c.constIndex(nil))
	c.emit(bytecode.RETURN_VALUE, 0)
	return c.assemble(), c.err
}

// CompileExpression compiles a single expression into a Code object whose
// execution leaves the expression's value on the stack and returns it.
func CompileExpression(expr ast.Expr, filename string) (*bytecode.Code, error) {
	c := &compiler{
		kind:      scopeModule,
		name:      "<eval>",
		filename:  filename,
		symbols:   newSymtable(),
		firstLine: 1,
	}
	c.compileExpr(expr)
	c.emit(bytecode.RETURN_VALUE, 0)
	return c.assemble(), c.err
}

// ============================================================
// Emit helpers
// ============================================================

func (c *compiler) emit(op bytecode.Opcode, arg uint32) int {
	c.instrs = append(c.instrs, bytecode.Instruction{Op: op, Arg: arg})
	return len(c.instrs) - 1
}

// emitJump emits a jump with a placeholder target, returning its index for
// later patching.
func (c *compiler) emitJump(op bytecode.Opcode) int {
	return c.emit(op, 0)
}

// patchJump sets the target of a previously emitted jump to the current offset.
func (c *compiler) patchJump(idx int) {
	c.instrs[idx].Arg = uint32(len(c.instrs))
}

// patchJumpTo sets the target of a jump to a specific offset.
func (c *compiler) patchJumpTo(idx, target int) {
	c.instrs[idx].Arg = uint32(target)
}

func (c *compiler) here() int { return len(c.instrs) }

func (c *compiler) fail(line int, format string, args ...interface{}) {
	if c.err == nil {
		c.err = &Error{Message: fmt.Sprintf(format, args...), Line: line}
	}
}

// ============================================================
// Pools
// ============================================================

func (c *compiler) constIndex(v interface{}) uint32 {
	for i, existing := range c.constants {
		if constEqual(existing, v) {
			return uint32(i)
		}
	}
	c.constants = append(c.constants, v)
	return uint32(len(c.constants) - 1)
}

func constEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	// Only merge identical primitive constants; never merge nested Code.
	switch av := a.(type) {
	case int64:
		bv, ok := b.(int64)
		return ok && av == bv
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case *bytecode.BytesConstant:
		bv, ok := b.(*bytecode.BytesConstant)
		return ok && bytesEqual(av.Data, bv.Data)
	}
	return false
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (c *compiler) nameIndex(name string) uint32 {
	for i, n := range c.names {
		if n == name {
			return uint32(i)
		}
	}
	c.names = append(c.names, name)
	return uint32(len(c.names) - 1)
}

func (c *compiler) varIndex(name string) uint32 {
	for i, n := range c.varnames {
		if n == name {
			return uint32(i)
		}
	}
	c.varnames = append(c.varnames, name)
	return uint32(len(c.varnames) - 1)
}

func (c *compiler) cellIndex(name string) (uint32, bool) {
	for i, n := range c.cellvars {
		if n == name {
			return uint32(i), true
		}
	}
	return 0, false
}

func (c *compiler) freeIndex(name string) (uint32, bool) {
	for i, n := range c.freevars {
		if n == name {
			return uint32(len(c.cellvars) + i), true
		}
	}
	return 0, false
}

// ============================================================
// Assembly
// ============================================================

func (c *compiler) assemble() *bytecode.Code {
	stackSize := estimateStackSize(c.instrs)
	return &bytecode.Code{
		Name:           c.name,
		Filename:       c.filename,
		FirstLine:      c.firstLine,
		Instructions:   c.instrs,
		Constants:      c.constants,
		Names:          c.names,
		Varnames:       c.varnames,
		Freevars:       c.freevars,
		Cellvars:       c.cellvars,
		ArgCount:       c.argCount,
		KwOnlyArgCount: c.kwOnlyArgs,
		Flags:          c.flags,
		StackSize:      stackSize,
	}
}

// estimateStackSize computes a conservative upper bound on stack depth by
// simulating net stack effect. For Phase 1/2 we use a generous heuristic:
// the number of instructions is always a safe upper bound on how many pushes
// can be live, but we compute a tighter value via per-opcode effect.
func estimateStackSize(instrs []bytecode.Instruction) int {
	depth, max := 0, 0
	for _, in := range instrs {
		depth += stackEffect(in)
		if depth < 0 {
			depth = 0
		}
		if depth > max {
			max = depth
		}
	}
	// Add headroom for call argument marshaling.
	return max + 8
}
