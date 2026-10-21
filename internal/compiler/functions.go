package compiler

import (
	"github.com/alvinjaison/PyGo/internal/ast"
	"github.com/alvinjaison/PyGo/internal/bytecode"
)

// compileFuncDef compiles a `def` statement: it builds a nested Code object,
// wraps it in a function via MAKE_FUNCTION, applies decorators, and binds the
// result to the function's name.
func (c *compiler) compileFuncDef(n *ast.FunctionDef) {
	c.compileFunctionObject(n.Name, n.Args, n.Body)

	// Apply decorators (innermost first — they were parsed outermost-first).
	for i := len(n.Decorators) - 1; i >= 0; i-- {
		c.compileExpr(n.Decorators[i])
		c.emit(bytecode.ROT_TWO, 0)
		c.emit(bytecode.CALL_FUNCTION, 1)
	}
	c.storeName(n.Name)
}

// compileFunctionObject builds the nested Code object and emits the
// instructions to create a function value on the stack (defaults, closure,
// MAKE_FUNCTION).
func (c *compiler) compileFunctionObject(name string, args *ast.Arguments, body []ast.Stmt) {
	// Determine names bound in enclosing function scopes for free-var analysis.
	enclosing := c.enclosingBindings()

	sub := &compiler{
		parent:   c,
		kind:     scopeFunction,
		name:     name,
		filename: c.filename,
		symbols:  buildFunctionSymtable(args, body, enclosing),
	}
	sub.flags |= bytecode.FlagOptimized | bytecode.FlagNewLocals

	// Register parameters as locals, in order, so their slot indices are
	// assigned first (the VM binds arguments by slot order).
	if args != nil {
		for _, a := range args.Args {
			sub.varIndex(a.Arg)
		}
		sub.argCount = len(args.Args)
		if args.Vararg != nil {
			sub.varIndex(args.Vararg.Arg)
			sub.flags |= bytecode.FlagVarargs
		}
		for _, a := range args.KwonlyArgs {
			sub.varIndex(a.Arg)
		}
		sub.kwOnlyArgs = len(args.KwonlyArgs)
		if args.Kwarg != nil {
			sub.varIndex(args.Kwarg.Arg)
			sub.flags |= bytecode.FlagVarkeywords
		}
	}

	// Determine cell variables (locals captured by nested functions).
	for cap := range sub.symbols.capturedByChild {
		if k, ok := sub.symbols.kindOf(cap); ok && k == symLocal {
			sub.symbols.kinds[cap] = symCell
			sub.cellvars = append(sub.cellvars, cap)
		}
	}

	// Determine free variables (names resolved from enclosing scopes).
	for name, kind := range sub.symbols.kinds {
		if kind == symFree {
			sub.freevars = append(sub.freevars, name)
		}
	}

	sub.compileBlock(body)
	// Implicit return None.
	sub.emit(bytecode.LOAD_CONST, sub.constIndex(nil))
	sub.emit(bytecode.RETURN_VALUE, 0)

	if sub.err != nil && c.err == nil {
		c.err = sub.err
	}
	code := sub.assemble()

	// Emit default values (as a tuple) if present.
	var flags uint32
	if args != nil && len(args.Defaults) > 0 {
		for _, d := range args.Defaults {
			c.compileExpr(d)
		}
		c.emit(bytecode.BUILD_TUPLE, uint32(len(args.Defaults)))
		flags |= uint32(bytecode.MakeFuncHasDefaults)
	}

	// Emit closure cells if the function has free variables.
	if len(sub.freevars) > 0 {
		for _, fv := range sub.freevars {
			c.loadClosureCell(fv)
		}
		c.emit(bytecode.BUILD_TUPLE, uint32(len(sub.freevars)))
		flags |= uint32(bytecode.MakeFuncHasClosure)
	}

	c.emit(bytecode.LOAD_CONST, c.constIndex(code))
	c.emit(bytecode.LOAD_CONST, c.constIndex(name))
	c.emit(bytecode.MAKE_FUNCTION, flags)
}

// loadClosureCell pushes the cell for a free variable, either from this
// scope's cells or its own free variables.
func (c *compiler) loadClosureCell(name string) {
	if idx, ok := c.cellIndex(name); ok {
		c.emit(bytecode.LOAD_CLOSURE, idx)
		return
	}
	if idx, ok := c.freeIndex(name); ok {
		c.emit(bytecode.LOAD_CLOSURE, idx)
		return
	}
	// Fallback: the name should have been classified; emit from names.
	c.emit(bytecode.LOAD_CLOSURE, c.nameIndex(name))
}

// enclosingBindings returns the set of names bound in this compiler's scope
// and its ancestors (used for free-variable resolution of nested functions).
func (c *compiler) enclosingBindings() map[string]bool {
	bindings := map[string]bool{}
	for cur := c; cur != nil; cur = cur.parent {
		if cur.kind == scopeModule {
			continue // module globals are not closure-captured
		}
		for name, kind := range cur.symbols.kinds {
			if kind == symLocal || kind == symCell || kind == symFree {
				bindings[name] = true
			}
		}
	}
	return bindings
}

// compileClassDef compiles a `class` statement.
func (c *compiler) compileClassDef(n *ast.ClassDef) {
	// Build the class body as a function-like code object that populates a
	// namespace, then call __build_class__ to create the type.
	c.emit(bytecode.LOAD_BUILD_CLASS, 0)

	sub := &compiler{
		parent:   c,
		kind:     scopeClass,
		name:     n.Name,
		filename: c.filename,
		symbols:  newSymtable(),
	}
	sub.flags |= bytecode.FlagNewLocals
	for _, s := range n.Body {
		collectAssigned(s, sub.symbols)
	}
	for name := range sub.symbols.assigned {
		sub.symbols.kinds[name] = symGlobal // class-body names use STORE_NAME
	}
	sub.compileBlock(n.Body)
	sub.emit(bytecode.LOAD_CONST, sub.constIndex(nil))
	sub.emit(bytecode.RETURN_VALUE, 0)
	if sub.err != nil && c.err == nil {
		c.err = sub.err
	}
	classCode := sub.assemble()

	c.emit(bytecode.LOAD_CONST, c.constIndex(classCode))
	c.emit(bytecode.LOAD_CONST, c.constIndex(n.Name))
	c.emit(bytecode.MAKE_FUNCTION, 0)
	c.emit(bytecode.LOAD_CONST, c.constIndex(n.Name))

	// Bases.
	for _, base := range n.Bases {
		c.compileExpr(base)
	}
	// __build_class__(func, name, *bases)
	c.emit(bytecode.CALL_FUNCTION, uint32(2+len(n.Bases)))

	// Apply decorators.
	for i := len(n.Decorators) - 1; i >= 0; i-- {
		c.compileExpr(n.Decorators[i])
		c.emit(bytecode.ROT_TWO, 0)
		c.emit(bytecode.CALL_FUNCTION, 1)
	}
	c.storeName(n.Name)
}
