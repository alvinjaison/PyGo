package compiler

import (
	"github.com/alvinjaison/PyGo/internal/ast"
	"github.com/alvinjaison/PyGo/internal/bytecode"
)

func (c *compiler) compileStmt(s ast.Stmt) {
	if c.err != nil {
		return
	}
	c.trackLine(s.Pos().Line)

	switch n := s.(type) {
	case *ast.ExprStmt:
		c.compileExpr(n.Value)
		c.emit(bytecode.POP_TOP, 0)

	case *ast.Assign:
		c.compileExpr(n.Value)
		// For multiple targets, duplicate the value for each.
		for i := range n.Targets {
			if i < len(n.Targets)-1 {
				c.emit(bytecode.DUP_TOP, 0)
			}
			c.compileAssignTarget(n.Targets[i])
		}

	case *ast.AugAssign:
		c.compileAugAssign(n)

	case *ast.AnnAssign:
		if n.Value != nil {
			c.compileExpr(n.Value)
			c.compileAssignTarget(n.Target)
		}

	case *ast.If:
		c.compileIf(n)

	case *ast.While:
		c.compileWhile(n)

	case *ast.For:
		c.compileFor(n)

	case *ast.FunctionDef:
		c.compileFuncDef(n)

	case *ast.ClassDef:
		c.compileClassDef(n)

	case *ast.Return:
		if n.Value != nil {
			c.compileExpr(n.Value)
		} else {
			c.emit(bytecode.LOAD_CONST, c.constIndex(nil))
		}
		c.emit(bytecode.RETURN_VALUE, 0)

	case *ast.Pass:
		// no-op

	case *ast.Break:
		if len(c.loops) == 0 {
			c.fail(n.Pos().Line, "'break' outside loop")
			return
		}
		j := c.emitJump(bytecode.JUMP_ABSOLUTE)
		lc := &c.loops[len(c.loops)-1]
		lc.breakJumps = append(lc.breakJumps, j)

	case *ast.Continue:
		if len(c.loops) == 0 {
			c.fail(n.Pos().Line, "'continue' outside loop")
			return
		}
		lc := c.loops[len(c.loops)-1]
		c.emit(bytecode.JUMP_ABSOLUTE, uint32(lc.continueTarget))

	case *ast.Raise:
		c.compileRaise(n)

	case *ast.Try:
		c.compileTry(n)

	case *ast.Global, *ast.Nonlocal:
		// Handled by the symbol table; no code emitted.

	case *ast.Delete:
		for _, t := range n.Targets {
			c.compileDelete(t)
		}

	case *ast.Assert:
		c.compileAssert(n)

	case *ast.Import:
		c.compileImport(n)

	case *ast.ImportFrom:
		c.compileImportFrom(n)

	case *ast.With:
		c.compileWith(n)

	default:
		c.fail(s.Pos().Line, "unsupported statement %T", s)
	}
}

func (c *compiler) compileBlock(block []ast.Stmt) {
	for _, s := range block {
		c.compileStmt(s)
		if c.err != nil {
			return
		}
	}
}

func (c *compiler) compileIf(n *ast.If) {
	c.compileExpr(n.Test)
	elseJump := c.emitJump(bytecode.POP_JUMP_IF_FALSE)
	c.compileBlock(n.Body)

	if len(n.Orelse) > 0 {
		endJump := c.emitJump(bytecode.JUMP_ABSOLUTE)
		c.patchJump(elseJump)
		c.compileBlock(n.Orelse)
		c.patchJump(endJump)
	} else {
		c.patchJump(elseJump)
	}
}

func (c *compiler) compileWhile(n *ast.While) {
	loopStart := c.here()
	c.compileExpr(n.Test)
	exitJump := c.emitJump(bytecode.POP_JUMP_IF_FALSE)

	c.loops = append(c.loops, loopContext{continueTarget: loopStart})
	c.compileBlock(n.Body)
	lc := c.loops[len(c.loops)-1]
	c.loops = c.loops[:len(c.loops)-1]

	c.emit(bytecode.JUMP_ABSOLUTE, uint32(loopStart))
	c.patchJump(exitJump)

	if len(n.Orelse) > 0 {
		c.compileBlock(n.Orelse)
	}
	for _, j := range lc.breakJumps {
		c.patchJump(j)
	}
}

func (c *compiler) compileFor(n *ast.For) {
	c.compileExpr(n.Iter)
	c.emit(bytecode.GET_ITER, 0)

	loopStart := c.here()
	forIter := c.emitJump(bytecode.FOR_ITER) // jumps past loop on exhaustion
	c.compileAssignTarget(n.Target)

	c.loops = append(c.loops, loopContext{continueTarget: loopStart})
	c.compileBlock(n.Body)
	lc := c.loops[len(c.loops)-1]
	c.loops = c.loops[:len(c.loops)-1]

	c.emit(bytecode.JUMP_ABSOLUTE, uint32(loopStart))

	// FOR_ITER's arg is a relative forward offset in this codebase's VM.
	c.instrs[forIter].Arg = uint32(c.here() - forIter - 1)

	if len(n.Orelse) > 0 {
		c.compileBlock(n.Orelse)
	}
	for _, j := range lc.breakJumps {
		c.patchJump(j)
	}
}

func (c *compiler) compileRaise(n *ast.Raise) {
	switch {
	case n.Exc == nil:
		c.emit(bytecode.RAISE_VARARGS, 0)
	case n.Cause == nil:
		c.compileExpr(n.Exc)
		c.emit(bytecode.RAISE_VARARGS, 1)
	default:
		c.compileExpr(n.Exc)
		c.compileExpr(n.Cause)
		c.emit(bytecode.RAISE_VARARGS, 2)
	}
}

func (c *compiler) compileTry(n *ast.Try) {
	// Simplified try/except model for Phase 2:
	//   SETUP_FINALLY <handler>
	//   <body>
	//   POP_BLOCK
	//   JUMP <else-or-end>
	// handler:
	//   <for each except: match type, bind name, run body>
	//   (re-raise if no handler matched)
	// end:
	//   <finally>
	setup := c.emitJump(bytecode.SETUP_FINALLY)
	c.compileBlock(n.Body)
	c.emit(bytecode.POP_BLOCK, 0)
	successJump := c.emitJump(bytecode.JUMP_ABSOLUTE)

	// Handler region begins here.
	c.instrs[setup].Arg = uint32(c.here())

	var endJumps []int
	for _, h := range n.Handlers {
		// Stack top holds the raised exception (pushed by the VM).
		var nextHandler int = -1
		if h.Type != nil {
			c.emit(bytecode.DUP_TOP, 0)
			c.compileExpr(h.Type)
			c.emit(bytecode.COMPARE_OP, uint32(bytecode.CmpExcMatch))
			nextHandler = c.emitJump(bytecode.POP_JUMP_IF_FALSE)
		}

		// Matched: optionally bind the exception, then run the handler body.
		if h.Name != "" {
			c.storeName(h.Name)
		} else {
			c.emit(bytecode.POP_TOP, 0)
		}
		c.emit(bytecode.POP_EXCEPT, 0)
		c.compileBlock(h.Body)
		endJumps = append(endJumps, c.emitJump(bytecode.JUMP_ABSOLUTE))

		if nextHandler >= 0 {
			c.patchJump(nextHandler)
		}
	}
	// No handler matched: re-raise.
	c.emit(bytecode.RERAISE, 0)

	c.patchJump(successJump)
	for _, j := range endJumps {
		c.patchJump(j)
	}

	if len(n.Orelse) > 0 {
		c.compileBlock(n.Orelse)
	}
	if len(n.Finalbody) > 0 {
		c.compileBlock(n.Finalbody)
	}
}

func (c *compiler) compileAssert(n *ast.Assert) {
	c.compileExpr(n.Test)
	ok := c.emitJump(bytecode.POP_JUMP_IF_TRUE)
	// Build AssertionError and raise.
	c.loadName("AssertionError")
	if n.Msg != nil {
		c.compileExpr(n.Msg)
		c.emit(bytecode.CALL_FUNCTION, 1)
	} else {
		c.emit(bytecode.CALL_FUNCTION, 0)
	}
	c.emit(bytecode.RAISE_VARARGS, 1)
	c.patchJump(ok)
}

func (c *compiler) compileImport(n *ast.Import) {
	for _, a := range n.Names {
		c.emit(bytecode.IMPORT_NAME, c.nameIndex(a.Name))
		bind := a.AsName
		if bind == "" {
			bind = topName(a.Name)
		}
		c.storeName(bind)
	}
}

func (c *compiler) compileImportFrom(n *ast.ImportFrom) {
	c.emit(bytecode.IMPORT_NAME, c.nameIndex(n.Module))
	for _, a := range n.Names {
		c.emit(bytecode.IMPORT_FROM, c.nameIndex(a.Name))
		bind := a.AsName
		if bind == "" {
			bind = a.Name
		}
		c.storeName(bind)
	}
	c.emit(bytecode.POP_TOP, 0) // pop the module left by IMPORT_NAME
}

func (c *compiler) compileWith(n *ast.With) {
	c.compileWithItems(n.Items, n.Body)
}

// compileWithItems compiles a (possibly multi-item) with statement with
// exception-safe cleanup. Multiple context managers nest left-to-right:
//
//	with A as a, B as b: BODY   ≡   with A as a: (with B as b: BODY)
//
// Each item emits:
//
//	<eval ctx>
//	SETUP_WITH <cleanup>     # calls __enter__, pushes with-block, pushes result
//	<bind result to target, or POP_TOP>
//	... body ...
//	POP_BLOCK
//	WITH_CLEANUP             # normal path: __exit__(None, None, None)
//	JUMP end
//	cleanup/end:             # exception suppression resumes here (set by unwind)
func (c *compiler) compileWithItems(items []*ast.WithItem, body []ast.Stmt) {
	if len(items) == 0 {
		c.compileBlock(body)
		return
	}
	item := items[0]

	c.compileExpr(item.ContextExpr)
	setup := c.emitJump(bytecode.SETUP_WITH) // operand patched to the end offset

	if item.OptionalVar != nil {
		c.compileAssignTarget(item.OptionalVar)
	} else {
		c.emit(bytecode.POP_TOP, 0)
	}

	// Nested remaining items (or the body for the last item).
	c.compileWithItems(items[1:], body)

	// Normal completion path.
	c.emit(bytecode.POP_BLOCK, 0)
	c.emit(bytecode.WITH_CLEANUP, 0)

	// The with-block's handler/resume point is here (after cleanup). On the
	// exception-suppressed path, unwind jumps here directly.
	c.instrs[setup].Arg = uint32(c.here())
}

// ============================================================
// Assignment targets
// ============================================================

func (c *compiler) compileAssignTarget(target ast.Expr) {
	switch t := target.(type) {
	case *ast.Name:
		c.storeName(t.Id)
	case *ast.Attribute:
		c.compileExpr(t.Value)
		c.emit(bytecode.STORE_ATTR, c.nameIndex(t.Attr))
	case *ast.Subscript:
		c.compileExpr(t.Value)
		c.compileSliceIndex(t.Slice)
		c.emit(bytecode.STORE_SUBSCR, 0)
	case *ast.Tuple:
		c.compileUnpack(t.Elts)
	case *ast.List:
		c.compileUnpack(t.Elts)
	default:
		c.fail(target.Pos().Line, "cannot assign to %T", target)
	}
}

func (c *compiler) compileUnpack(elts []ast.Expr) {
	c.emit(bytecode.UNPACK_SEQUENCE, uint32(len(elts)))
	for _, elt := range elts {
		c.compileAssignTarget(elt)
	}
}

func (c *compiler) compileDelete(target ast.Expr) {
	switch t := target.(type) {
	case *ast.Name:
		c.deleteName(t.Id)
	case *ast.Attribute:
		c.compileExpr(t.Value)
		c.emit(bytecode.DELETE_ATTR, c.nameIndex(t.Attr))
	case *ast.Subscript:
		c.compileExpr(t.Value)
		c.compileSliceIndex(t.Slice)
		c.emit(bytecode.DELETE_SUBSCR, 0)
	default:
		c.fail(target.Pos().Line, "cannot delete %T", target)
	}
}

func (c *compiler) compileAugAssign(n *ast.AugAssign) {
	// Load current value, apply op, store back.
	switch t := n.Target.(type) {
	case *ast.Name:
		c.loadName(t.Id)
		c.compileExpr(n.Value)
		c.emitBinaryOp(n.Op)
		c.storeName(t.Id)
	case *ast.Attribute:
		c.compileExpr(t.Value)
		c.emit(bytecode.DUP_TOP, 0)
		c.emit(bytecode.LOAD_ATTR, c.nameIndex(t.Attr))
		c.compileExpr(n.Value)
		c.emitBinaryOp(n.Op)
		c.emit(bytecode.ROT_TWO, 0)
		c.emit(bytecode.STORE_ATTR, c.nameIndex(t.Attr))
	case *ast.Subscript:
		c.compileExpr(t.Value)
		c.compileSliceIndex(t.Slice)
		// stack: container, key
		c.emit(bytecode.DUP_TOP_TWO, 0)
		c.emit(bytecode.BINARY_SUBSCR, 0)
		c.compileExpr(n.Value)
		c.emitBinaryOp(n.Op)
		// stack: container, key, newval
		c.emit(bytecode.ROT_THREE, 0)
		c.emit(bytecode.STORE_SUBSCR, 0)
	default:
		c.fail(n.Pos().Line, "cannot augment-assign to %T", n.Target)
	}
}

// ============================================================
// Name load/store with scope awareness
// ============================================================

func (c *compiler) loadName(name string) {
	kind, ok := c.symbols.kindOf(name)
	if !ok {
		// Unknown: treat as global/builtin.
		c.emit(bytecode.LOAD_GLOBAL, c.nameIndex(name))
		return
	}
	switch kind {
	case symLocal:
		c.emit(bytecode.LOAD_FAST, c.varIndex(name))
	case symGlobal:
		if c.kind == scopeModule {
			c.emit(bytecode.LOAD_NAME, c.nameIndex(name))
		} else {
			c.emit(bytecode.LOAD_GLOBAL, c.nameIndex(name))
		}
	case symCell:
		idx, _ := c.cellIndex(name)
		c.emit(bytecode.LOAD_DEREF, idx)
	case symFree:
		idx, _ := c.freeIndex(name)
		c.emit(bytecode.LOAD_DEREF, idx)
	}
}

func (c *compiler) storeName(name string) {
	kind, ok := c.symbols.kindOf(name)
	if !ok {
		if c.kind == scopeModule {
			c.emit(bytecode.STORE_NAME, c.nameIndex(name))
		} else {
			c.emit(bytecode.STORE_FAST, c.varIndex(name))
		}
		return
	}
	switch kind {
	case symLocal:
		c.emit(bytecode.STORE_FAST, c.varIndex(name))
	case symGlobal:
		if c.kind == scopeModule {
			c.emit(bytecode.STORE_NAME, c.nameIndex(name))
		} else {
			c.emit(bytecode.STORE_GLOBAL, c.nameIndex(name))
		}
	case symCell:
		idx, _ := c.cellIndex(name)
		c.emit(bytecode.STORE_DEREF, idx)
	case symFree:
		idx, _ := c.freeIndex(name)
		c.emit(bytecode.STORE_DEREF, idx)
	}
}

func (c *compiler) deleteName(name string) {
	kind, ok := c.symbols.kindOf(name)
	if !ok || c.kind == scopeModule {
		c.emit(bytecode.DELETE_NAME, c.nameIndex(name))
		return
	}
	switch kind {
	case symLocal:
		c.emit(bytecode.DELETE_FAST, c.varIndex(name))
	case symGlobal:
		c.emit(bytecode.DELETE_GLOBAL, c.nameIndex(name))
	default:
		c.emit(bytecode.DELETE_NAME, c.nameIndex(name))
	}
}

// ============================================================
// Line tracking
// ============================================================

func (c *compiler) trackLine(line int) {
	if line == 0 || line == c.lastLine {
		return
	}
	off := len(c.instrs)
	c.lineTable = append(c.lineTable, off-c.lastOff, line-c.lastLine)
	c.lastOff = off
	c.lastLine = line
	if c.firstLine == 0 {
		c.firstLine = line
	}
}
