package compiler

import (
	"github.com/alvinjaison/PyGo/internal/ast"
	"github.com/alvinjaison/PyGo/internal/bytecode"
)

// Structural pattern matching.
//
// Code generation keeps the subject on the value stack for the whole match
// statement. For each case the compiler duplicates it and runs the pattern on
// the copy:
//
//	<subject>
//	case 1:  DUP_TOP; <pattern>; [<guard>]; POP_TOP; <body>; JUMP end
//	         <cleanup stubs>
//	case 2:  ...
//	no match: POP_TOP
//	end:
//
// A pattern is compiled with its value on top of the stack. On success it
// consumes that value and binds captures. On failure it jumps to the case's
// cleanup stubs. Because a failure can happen with extra values (unpacked
// elements, mapping, ...) still on the stack, every failure jump records the
// stack depth it was taken at, and the stubs pop down to the case's base.
//
// Depths are counted in values above the saved subject.

// failJump is a conditional jump taken when a pattern does not match.
type failJump struct {
	idx   int // index of the jump instruction to patch
	depth int // values above the subject on the stack when it is taken
}

type failList struct{ jumps []failJump }

func (fl *failList) add(idx, depth int) {
	fl.jumps = append(fl.jumps, failJump{idx: idx, depth: depth})
}

// resolveFails emits cleanup stubs that pop every failure down to depth base.
// It returns the jumps that must be patched to the code that follows a failed
// match (the next alternative or the next case).
func (c *compiler) resolveFails(fl *failList, base int) []int {
	var pending []int
	done := make(map[int]bool)
	for _, fj := range fl.jumps {
		if fj.depth == base {
			pending = append(pending, fj.idx)
			continue
		}
		if done[fj.depth] {
			continue
		}
		done[fj.depth] = true
		for _, other := range fl.jumps {
			if other.depth == fj.depth {
				c.patchJump(other.idx) // lands on the stub emitted below
			}
		}
		for i := base; i < fj.depth; i++ {
			c.emit(bytecode.POP_TOP, 0)
		}
		pending = append(pending, c.emitJump(bytecode.JUMP_ABSOLUTE))
	}
	return pending
}

// noteDepth records a stack depth reached by pattern code so the frame's value
// stack is sized large enough (the linear estimate ignores control flow).
func (c *compiler) noteDepth(d int) {
	if d > c.stackExtra {
		c.stackExtra = d
	}
}

func (c *compiler) compileMatch(n *ast.Match) {
	c.compileExpr(n.Subject)

	var endJumps []int
	for i, cs := range n.Cases {
		if c.err != nil {
			return
		}
		c.trackLine(cs.Pos().Line)
		c.patternBindings(cs.Pattern)
		if irrefutable(cs.Pattern) && cs.Guard == nil && i < len(n.Cases)-1 {
			c.fail(cs.Pos().Line, "irrefutable pattern makes the remaining case blocks unreachable")
			return
		}

		fails := &failList{}
		c.emit(bytecode.DUP_TOP, 0)
		c.compilePattern(cs.Pattern, 1, fails)

		if cs.Guard != nil {
			c.compileExpr(cs.Guard)
			fails.add(c.emitJump(bytecode.POP_JUMP_IF_FALSE), 0)
		}

		// The pattern matched: drop the subject, then run the body with a
		// balanced stack so break/continue/return need no special handling.
		c.emit(bytecode.POP_TOP, 0)
		c.compileBlock(cs.Body)
		endJumps = append(endJumps, c.emitJump(bytecode.JUMP_ABSOLUTE))

		for _, j := range c.resolveFails(fails, 0) {
			c.patchJump(j) // falls through to the next case
		}
	}

	// No case matched.
	c.emit(bytecode.POP_TOP, 0)
	for _, j := range endJumps {
		c.patchJump(j)
	}
}

// compilePattern emits code that tests the value on top of the stack against
// p. depth is the number of values above the subject, including that value.
func (c *compiler) compilePattern(p ast.Pattern, depth int, fails *failList) {
	if c.err != nil {
		return
	}
	c.noteDepth(depth + 2)

	switch n := p.(type) {
	case *ast.MatchAs:
		if n.Pattern != nil {
			c.emit(bytecode.DUP_TOP, 0)
			c.compilePattern(n.Pattern, depth+1, fails)
		}
		if n.Name == "" {
			c.emit(bytecode.POP_TOP, 0)
		} else {
			c.storeName(n.Name)
		}

	case *ast.MatchValue:
		c.compileExpr(n.Value)
		c.emit(bytecode.COMPARE_OP, uint32(bytecode.CmpEq))
		fails.add(c.emitJump(bytecode.POP_JUMP_IF_FALSE), depth-1)

	case *ast.MatchSingleton:
		c.emit(bytecode.LOAD_CONST, c.constIndex(n.Value))
		c.emit(bytecode.COMPARE_OP, uint32(bytecode.CmpIs))
		fails.add(c.emitJump(bytecode.POP_JUMP_IF_FALSE), depth-1)

	case *ast.MatchOr:
		c.compileOrPattern(n, depth, fails)

	case *ast.MatchSequence:
		c.compileSequencePattern(n, depth, fails)

	case *ast.MatchMapping:
		c.compileMappingPattern(n, depth, fails)

	case *ast.MatchClass:
		c.compileClassPattern(n, depth, fails)

	case *ast.MatchStar:
		c.fail(n.Pos().Line, "star pattern is only allowed inside a sequence pattern")

	default:
		c.fail(p.Pos().Line, "unsupported pattern %T", p)
	}
}

// compileOrPattern tries each alternative in turn. Every alternative but the
// last runs on a copy of the value so the next one can start from the
// original.
func (c *compiler) compileOrPattern(n *ast.MatchOr, depth int, fails *failList) {
	var okJumps []int
	for i, alt := range n.Patterns {
		if i == len(n.Patterns)-1 {
			c.compilePattern(alt, depth, fails)
			break
		}
		if irrefutable(alt) {
			c.fail(alt.Pos().Line, "alternative pattern makes the remaining alternatives unreachable")
			return
		}
		c.emit(bytecode.DUP_TOP, 0)
		local := &failList{}
		c.compilePattern(alt, depth+1, local)
		// Matched: the copy is consumed; discard the original too.
		c.emit(bytecode.POP_TOP, 0)
		okJumps = append(okJumps, c.emitJump(bytecode.JUMP_ABSOLUTE))
		for _, j := range c.resolveFails(local, depth) {
			c.patchJump(j) // the next alternative begins here
		}
	}
	for _, j := range okJumps {
		c.patchJump(j)
	}
}

func (c *compiler) compileSequencePattern(n *ast.MatchSequence, depth int, fails *failList) {
	star := -1
	for i, sp := range n.Patterns {
		if _, ok := sp.(*ast.MatchStar); ok {
			star = i
		}
	}
	total := len(n.Patterns)
	fixed := total
	if star >= 0 {
		fixed--
	}

	c.emit(bytecode.MATCH_SEQ, 0)
	fails.add(c.emitJump(bytecode.POP_JUMP_IF_FALSE), depth)

	lenArg := uint32(fixed) << 1
	if star >= 0 {
		lenArg |= 1
	}
	c.emit(bytecode.MATCH_SEQ_LEN, lenArg)
	fails.add(c.emitJump(bytecode.POP_JUMP_IF_FALSE), depth)

	if star < 0 {
		c.emit(bytecode.UNPACK_SEQUENCE, uint32(total))
	} else {
		before, after := star, total-star-1
		if before > bytecode.UnpackBeforeMask || after > bytecode.UnpackBeforeMask {
			c.fail(n.Pos().Line, "too many elements in sequence pattern")
			return
		}
		c.emit(bytecode.UNPACK_EX, uint32(before)|uint32(after)<<bytecode.UnpackAfterShift)
	}
	c.noteDepth(depth + total + 2)

	// The elements are now on the stack, the first one on top.
	for i, sp := range n.Patterns {
		if ms, ok := sp.(*ast.MatchStar); ok {
			if ms.Name == "" {
				c.emit(bytecode.POP_TOP, 0)
			} else {
				c.storeName(ms.Name)
			}
			continue
		}
		c.compilePattern(sp, depth-1+(total-i), fails)
	}
}

func (c *compiler) compileMappingPattern(n *ast.MatchMapping, depth int, fails *failList) {
	c.emit(bytecode.MATCH_MAPPING, 0)
	fails.add(c.emitJump(bytecode.POP_JUMP_IF_FALSE), depth)

	for i, key := range n.Keys {
		c.compileExpr(key)
		c.emit(bytecode.MATCH_KEY, 0)
		// Missing key: the mapping alone remains. Present: its value is on top.
		fails.add(c.emitJump(bytecode.POP_JUMP_IF_FALSE), depth)
		c.compilePattern(n.Patterns[i], depth+1, fails)
	}

	if n.Rest != "" {
		for _, key := range n.Keys {
			c.compileExpr(key)
		}
		c.emit(bytecode.BUILD_TUPLE, uint32(len(n.Keys)))
		c.emit(bytecode.MATCH_REST, 0)
		c.storeName(n.Rest)
	}
	c.emit(bytecode.POP_TOP, 0) // the mapping itself
}

func (c *compiler) compileClassPattern(n *ast.MatchClass, depth int, fails *failList) {
	c.compileExpr(n.Cls)
	for _, attr := range n.KwdAttrs {
		c.emit(bytecode.LOAD_CONST, c.constIndex(attr))
	}
	c.emit(bytecode.BUILD_TUPLE, uint32(len(n.KwdAttrs)))
	c.emit(bytecode.MATCH_CLASS, uint32(len(n.Patterns)))
	// MATCH_CLASS consumes the subject, so a failure is one value shallower.
	fails.add(c.emitJump(bytecode.POP_JUMP_IF_FALSE), depth-1)

	total := len(n.Patterns) + len(n.KwdPatterns)
	if total == 0 {
		c.emit(bytecode.POP_TOP, 0) // the (empty) attribute tuple
		return
	}
	c.emit(bytecode.UNPACK_SEQUENCE, uint32(total))
	c.noteDepth(depth + total + 2)

	subs := make([]ast.Pattern, 0, total)
	subs = append(subs, n.Patterns...)
	subs = append(subs, n.KwdPatterns...)
	for i, sp := range subs {
		c.compilePattern(sp, depth-1+(total-i), fails)
	}
}

// irrefutable reports whether a pattern always matches.
func irrefutable(p ast.Pattern) bool {
	switch n := p.(type) {
	case *ast.MatchAs:
		return n.Pattern == nil || irrefutable(n.Pattern)
	case *ast.MatchOr:
		for _, alt := range n.Patterns {
			if irrefutable(alt) {
				return true
			}
		}
	}
	return false
}

// patternBindings returns the names a pattern binds, in order, and reports
// patterns that bind a name twice or whose alternatives bind different names.
func (c *compiler) patternBindings(p ast.Pattern) []string {
	switch n := p.(type) {
	case *ast.MatchAs:
		var names []string
		if n.Pattern != nil {
			names = c.patternBindings(n.Pattern)
		}
		if n.Name != "" {
			names = c.addBinding(names, n.Name, n.Pos().Line)
		}
		return names

	case *ast.MatchStar:
		if n.Name != "" {
			return []string{n.Name}
		}

	case *ast.MatchSequence:
		return c.bindingsOf(n.Patterns, "", n.Pos().Line)

	case *ast.MatchMapping:
		return c.bindingsOf(n.Patterns, n.Rest, n.Pos().Line)

	case *ast.MatchClass:
		all := append(append([]ast.Pattern(nil), n.Patterns...), n.KwdPatterns...)
		return c.bindingsOf(all, "", n.Pos().Line)

	case *ast.MatchOr:
		first := c.patternBindings(n.Patterns[0])
		want := make(map[string]bool, len(first))
		for _, name := range first {
			want[name] = true
		}
		for _, alt := range n.Patterns[1:] {
			got := c.patternBindings(alt)
			same := len(got) == len(first)
			for _, name := range got {
				if !want[name] {
					same = false
				}
			}
			if !same {
				c.fail(alt.Pos().Line, "alternative patterns bind different names")
				return first
			}
		}
		return first
	}
	return nil
}

func (c *compiler) bindingsOf(pats []ast.Pattern, rest string, line int) []string {
	var names []string
	for _, sp := range pats {
		for _, name := range c.patternBindings(sp) {
			names = c.addBinding(names, name, line)
		}
	}
	if rest != "" {
		names = c.addBinding(names, rest, line)
	}
	return names
}

func (c *compiler) addBinding(names []string, name string, line int) []string {
	for _, existing := range names {
		if existing == name {
			c.fail(line, "multiple assignments to name '%s' in pattern", name)
			return names
		}
	}
	return append(names, name)
}
