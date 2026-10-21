package compiler

import (
	"github.com/alvinjaison/PyGo/internal/ast"
	"github.com/alvinjaison/PyGo/internal/bytecode"
)

func (c *compiler) compileExpr(e ast.Expr) {
	if c.err != nil {
		return
	}
	switch n := e.(type) {
	case *ast.Constant:
		c.emit(bytecode.LOAD_CONST, c.constIndex(n.Value))

	case *ast.Name:
		c.loadName(n.Id)

	case *ast.BinOp:
		c.compileExpr(n.Left)
		c.compileExpr(n.Right)
		c.emitBinaryOp(n.Op)

	case *ast.UnaryOp:
		c.compileExpr(n.Operand)
		switch n.Op {
		case ast.USub:
			c.emit(bytecode.UNARY_NEGATIVE, 0)
		case ast.UAdd:
			c.emit(bytecode.UNARY_POSITIVE, 0)
		case ast.Invert:
			c.emit(bytecode.UNARY_INVERT, 0)
		case ast.Not:
			c.emit(bytecode.UNARY_NOT, 0)
		}

	case *ast.BoolOp:
		c.compileBoolOp(n)

	case *ast.Compare:
		c.compileCompare(n)

	case *ast.IfExp:
		c.compileExpr(n.Test)
		elseJump := c.emitJump(bytecode.POP_JUMP_IF_FALSE)
		c.compileExpr(n.Body)
		endJump := c.emitJump(bytecode.JUMP_ABSOLUTE)
		c.patchJump(elseJump)
		c.compileExpr(n.Orelse)
		c.patchJump(endJump)

	case *ast.Call:
		c.compileCall(n)

	case *ast.Attribute:
		c.compileExpr(n.Value)
		c.emit(bytecode.LOAD_ATTR, c.nameIndex(n.Attr))

	case *ast.Subscript:
		c.compileExpr(n.Value)
		c.compileSliceIndex(n.Slice)
		c.emit(bytecode.BINARY_SUBSCR, 0)

	case *ast.List:
		for _, elt := range n.Elts {
			c.compileExpr(elt)
		}
		c.emit(bytecode.BUILD_LIST, uint32(len(n.Elts)))

	case *ast.Tuple:
		for _, elt := range n.Elts {
			c.compileExpr(elt)
		}
		c.emit(bytecode.BUILD_TUPLE, uint32(len(n.Elts)))

	case *ast.Set:
		for _, elt := range n.Elts {
			c.compileExpr(elt)
		}
		c.emit(bytecode.BUILD_SET, uint32(len(n.Elts)))

	case *ast.Dict:
		for i := range n.Keys {
			c.compileExpr(n.Keys[i])
			c.compileExpr(n.Values[i])
		}
		c.emit(bytecode.BUILD_MAP, uint32(len(n.Keys)))

	case *ast.Lambda:
		c.compileLambda(n)

	case *ast.Starred:
		c.compileExpr(n.Value)

	case *ast.Slice:
		c.compileSliceObject(n)

	case *ast.ListComp:
		c.compileListComp(n)

	case *ast.GeneratorExp:
		// Eager approximation: build a list and let the consumer iterate it.
		// (A fully lazy generator expression is a Phase 7 refinement.)
		c.compileListComp(&ast.ListComp{Elt: n.Elt, Generators: n.Generators})

	case *ast.SetComp:
		c.emit(bytecode.BUILD_SET, 0)
		c.compileComprehensionLoops(n.Generators, 0, func() {
			c.compileExpr(n.Elt)
			c.emit(bytecode.SET_ADD, 0)
		})

	case *ast.DictComp:
		c.emit(bytecode.BUILD_MAP, 0)
		c.compileComprehensionLoops(n.Generators, 0, func() {
			c.compileExpr(n.Key)
			c.compileExpr(n.Value)
			c.emit(bytecode.MAP_ADD, 0)
		})

	case *ast.Yield:
		if n.Value != nil {
			c.compileExpr(n.Value)
		} else {
			c.emit(bytecode.LOAD_CONST, c.constIndex(nil))
		}
		c.emit(bytecode.YIELD_VALUE, 0)
		c.flags |= bytecode.FlagGenerator

	default:
		c.fail(e.Pos().Line, "unsupported expression %T", e)
	}
}

func (c *compiler) compileBoolOp(n *ast.BoolOp) {
	// Short-circuit: `and` jumps out on first falsey; `or` on first truthy.
	var jumps []int
	for i, v := range n.Values {
		c.compileExpr(v)
		if i < len(n.Values)-1 {
			if n.Op == ast.And {
				jumps = append(jumps, c.emitJump(bytecode.JUMP_IF_FALSE_OR_POP))
			} else {
				jumps = append(jumps, c.emitJump(bytecode.JUMP_IF_TRUE_OR_POP))
			}
		}
	}
	for _, j := range jumps {
		c.patchJump(j)
	}
}

func (c *compiler) compileCompare(n *ast.Compare) {
	// Chained comparisons: a < b < c  →  (a < b) and (b < c), each operand
	// evaluated once. For Phase 2 we implement the simple (non-short-circuit)
	// form for single comparisons, and a correct chained form for multiples.
	c.compileExpr(n.Left)
	if len(n.Ops) == 1 {
		c.compileExpr(n.Comparators[0])
		c.emit(bytecode.COMPARE_OP, uint32(compareOpCode(n.Ops[0])))
		return
	}

	var exitJumps []int
	for i, op := range n.Ops {
		c.compileExpr(n.Comparators[i])
		if i < len(n.Ops)-1 {
			// Keep the right operand for the next comparison.
			c.emit(bytecode.DUP_TOP, 0)
			c.emit(bytecode.ROT_THREE, 0)
			c.emit(bytecode.COMPARE_OP, uint32(compareOpCode(op)))
			exitJumps = append(exitJumps, c.emitJump(bytecode.JUMP_IF_FALSE_OR_POP))
		} else {
			c.emit(bytecode.COMPARE_OP, uint32(compareOpCode(op)))
		}
	}
	if len(exitJumps) > 0 {
		endJump := c.emitJump(bytecode.JUMP_ABSOLUTE)
		for _, j := range exitJumps {
			c.patchJump(j)
		}
		// Clean up the leftover duplicated operand.
		c.emit(bytecode.ROT_TWO, 0)
		c.emit(bytecode.POP_TOP, 0)
		c.patchJump(endJump)
	}
}

func (c *compiler) compileCall(n *ast.Call) {
	c.compileExpr(n.Func)
	for _, a := range n.Args {
		c.compileExpr(a)
	}
	if len(n.Keywords) == 0 {
		c.emit(bytecode.CALL_FUNCTION, uint32(len(n.Args)))
		return
	}
	// Keyword arguments: push values and a tuple of keyword names.
	var kwNames []interface{}
	for _, kw := range n.Keywords {
		c.compileExpr(kw.Value)
		kwNames = append(kwNames, kw.Arg)
	}
	c.emit(bytecode.LOAD_CONST, c.constIndex(kwTuple(kwNames)))
	c.emit(bytecode.CALL_FUNCTION_KW, uint32(len(n.Args)+len(n.Keywords)))
}

// kwTuple wraps keyword names as a constant the VM can recover.
type kwNameTuple struct{ names []string }

func kwTuple(names []interface{}) interface{} {
	s := make([]string, len(names))
	for i, n := range names {
		s[i] = n.(string)
	}
	return &kwNameTuple{names: s}
}

// KwNames extracts keyword names from a kw-tuple constant (used by the VM).
func KwNames(v interface{}) ([]string, bool) {
	if t, ok := v.(*kwNameTuple); ok {
		return t.names, true
	}
	return nil, false
}

func (c *compiler) compileLambda(n *ast.Lambda) {
	body := []ast.Stmt{ast.SetPos(&ast.Return{Value: n.Body}, n.Pos())}
	c.compileFunctionObject("<lambda>", n.Args, body)
}

func (c *compiler) compileSliceIndex(slice ast.Expr) {
	if sl, ok := slice.(*ast.Slice); ok {
		c.compileSliceObject(sl)
		return
	}
	c.compileExpr(slice)
}

func (c *compiler) compileSliceObject(sl *ast.Slice) {
	if sl.Lower != nil {
		c.compileExpr(sl.Lower)
	} else {
		c.emit(bytecode.LOAD_CONST, c.constIndex(nil))
	}
	if sl.Upper != nil {
		c.compileExpr(sl.Upper)
	} else {
		c.emit(bytecode.LOAD_CONST, c.constIndex(nil))
	}
	if sl.Step != nil {
		c.compileExpr(sl.Step)
	} else {
		c.emit(bytecode.LOAD_CONST, c.constIndex(nil))
	}
	c.emit(bytecode.BUILD_SLICE, 3)
}

// compileListComp compiles a list comprehension by desugaring into an
// accumulator loop inline in the current scope (Phase 2 approach).
func (c *compiler) compileListComp(n *ast.ListComp) {
	c.emit(bytecode.BUILD_LIST, 0)
	c.compileComprehensionLoops(n.Generators, 0, func() {
		c.compileExpr(n.Elt)
		c.emit(bytecode.LIST_APPEND, 0)
	})
}

func (c *compiler) compileComprehensionLoops(gens []*ast.Comprehension, i int, emitBody func()) {
	if i >= len(gens) {
		emitBody()
		return
	}
	g := gens[i]
	c.compileExpr(g.Iter)
	c.emit(bytecode.GET_ITER, 0)
	loopStart := c.here()
	forIter := c.emitJump(bytecode.FOR_ITER)
	c.compileAssignTarget(g.Target)

	// Conditions.
	var condJumps []int
	for _, cond := range g.Ifs {
		c.compileExpr(cond)
		condJumps = append(condJumps, c.emitJump(bytecode.POP_JUMP_IF_FALSE))
	}

	c.compileComprehensionLoops(gens, i+1, emitBody)

	for _, j := range condJumps {
		c.patchJumpTo(j, loopStart)
	}
	c.emit(bytecode.JUMP_ABSOLUTE, uint32(loopStart))
	c.instrs[forIter].Arg = uint32(c.here() - forIter - 1)
}

// ============================================================
// Operator opcode mapping
// ============================================================

func (c *compiler) emitBinaryOp(op ast.Operator) {
	switch op {
	case ast.Add:
		c.emit(bytecode.BINARY_ADD, 0)
	case ast.Sub:
		c.emit(bytecode.BINARY_SUBTRACT, 0)
	case ast.Mult:
		c.emit(bytecode.BINARY_MULTIPLY, 0)
	case ast.Div:
		c.emit(bytecode.BINARY_TRUE_DIVIDE, 0)
	case ast.FloorDiv:
		c.emit(bytecode.BINARY_FLOOR_DIVIDE, 0)
	case ast.Mod:
		c.emit(bytecode.BINARY_MODULO, 0)
	case ast.Pow:
		c.emit(bytecode.BINARY_POWER, 0)
	case ast.BitAnd:
		c.emit(bytecode.BINARY_AND, 0)
	case ast.BitOr:
		c.emit(bytecode.BINARY_OR, 0)
	case ast.BitXor:
		c.emit(bytecode.BINARY_XOR, 0)
	case ast.LShift:
		c.emit(bytecode.BINARY_LSHIFT, 0)
	case ast.RShift:
		c.emit(bytecode.BINARY_RSHIFT, 0)
	default:
		c.emit(bytecode.BINARY_ADD, 0)
	}
}

func compareOpCode(op ast.CmpOp) bytecode.CompareOp {
	switch op {
	case ast.Lt:
		return bytecode.CmpLt
	case ast.LtE:
		return bytecode.CmpLtE
	case ast.Eq:
		return bytecode.CmpEq
	case ast.NotEq:
		return bytecode.CmpNotEq
	case ast.Gt:
		return bytecode.CmpGt
	case ast.GtE:
		return bytecode.CmpGtE
	case ast.In:
		return bytecode.CmpIn
	case ast.NotIn:
		return bytecode.CmpNotIn
	case ast.Is:
		return bytecode.CmpIs
	case ast.IsNot:
		return bytecode.CmpIsNot
	}
	return bytecode.CmpEq
}
