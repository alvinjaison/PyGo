// Package compiler transforms an AST into bytecode Code objects.
//
// The compilation process has two phases:
//  1. Symbol table analysis — classifies names as local/global/builtin and assigns variable slots.
//  2. Code emission — walks AST nodes and emits bytecode instructions.
//
// Each scope (module or function) gets its own symbol table and produces its own Code object.
package compiler

import (
	"fmt"

	"github.com/alvinjaison/pygo/internal/ast"
	"github.com/alvinjaison/pygo/internal/bytecode"
	"github.com/alvinjaison/pygo/internal/object"
)

// Compile transforms a parsed module AST into a bytecode Code object.
// The returned Code represents the module-level code. Nested function
// definitions produce nested Code objects stored in the Constants pool.
func Compile(module *ast.Module) (*bytecode.Code, error) {
	c := newCompiler(module.Pos().Filename)
	return c.compileModule(module)
}

// compiler holds state for the compilation of a single module.
type compiler struct {
	filename string
}

func newCompiler(filename string) *compiler {
	return &compiler{
		filename: filename,
	}
}

// compileModule compiles a top-level module into a Code object.
func (c *compiler) compileModule(module *ast.Module) (*bytecode.Code, error) {
	// Phase 1: Build symbol table for the module scope.
	symtab := analyzeModule(module)

	// Create a scope compiler for the module.
	sc := newScopeCompiler(c.filename, "<module>", symtab, scopeModule)
	sc.compiler = c

	// Phase 2: Code emission — compile each statement.
	for _, stmt := range module.Body {
		if err := sc.compileStmt(stmt); err != nil {
			return nil, err
		}
	}

	// Emit implicit return None at the end of the module.
	sc.emitLoadNone()
	sc.emit(bytecode.RETURN_VALUE, 0)

	return sc.finalize(module.Pos().Line), nil
}

// CompileFunction compiles a function definition into a nested Code object.
// This is called during code emission when a FunctionDef node is encountered.
func (c *compiler) compileFunction(funcDef *ast.FunctionDef, parentSymtab *symbolTable) (*bytecode.Code, error) {
	// Build symbol table for the function scope.
	symtab := analyzeFunction(funcDef, parentSymtab)

	// Validate that all nonlocal declarations can be resolved.
	if err := symtab.ValidateNonlocals(); err != nil {
		return nil, err
	}

	// Create a scope compiler for the function.
	sc := newScopeCompiler(c.filename, funcDef.Name, symtab, scopeFunction)
	sc.compiler = c

	// Compile the function body.
	for _, stmt := range funcDef.Body {
		if err := sc.compileStmt(stmt); err != nil {
			return nil, err
		}
	}

	// Emit implicit None return at end of function.
	sc.emitLoadNone()
	sc.emit(bytecode.RETURN_VALUE, 0)

	code := sc.finalize(funcDef.Pos().Line)
	code.ArgCount = countPosArgs(funcDef.Args)

	// Set keyword-only argument count.
	if funcDef.Args != nil {
		code.KwOnlyArgCount = len(funcDef.Args.KwonlyArgs)
		if funcDef.Args.Vararg != nil {
			code.Flags |= bytecode.FlagVarargs
		}
		if funcDef.Args.Kwarg != nil {
			code.Flags |= bytecode.FlagVarkeywords
		}
	}

	return code, nil
}

// countPosArgs returns the number of positional parameters in a function definition.
func countPosArgs(args *ast.Arguments) int {
	if args == nil {
		return 0
	}
	return len(args.PosonlyArgs) + len(args.Args)
}

// compileStmt compiles a single statement node.
func (sc *scopeCompiler) compileStmt(stmt ast.Stmt) error {
	// Record line table entry for this statement.
	sc.setLine(stmt.Pos().Line)

	switch s := stmt.(type) {
	case *ast.ExprStmt:
		// Compile the expression and discard its value.
		if err := sc.compileExpr(s.Value); err != nil {
			return err
		}
		sc.emit(bytecode.POP_TOP, 0)

	case *ast.Assign:
		// Compile the value expression.
		if err := sc.compileExpr(s.Value); err != nil {
			return err
		}
		// Store to each target. For multiple targets, duplicate the value.
		for i, target := range s.Targets {
			if i < len(s.Targets)-1 {
				sc.emit(bytecode.DUP_TOP, 0)
			}
			if err := sc.compileStore(target); err != nil {
				return err
			}
		}

	case *ast.AugAssign:
		// Load the current value, compile the RHS, apply the operator, store back.
		if err := sc.compileExpr(s.Target); err != nil {
			return err
		}
		if err := sc.compileExpr(s.Value); err != nil {
			return err
		}
		op, err := binOpToOpcode(s.Op)
		if err != nil {
			return err
		}
		sc.emit(op, 0)
		if err := sc.compileStore(s.Target); err != nil {
			return err
		}

	case *ast.Return:
		if s.Value != nil {
			if err := sc.compileExpr(s.Value); err != nil {
				return err
			}
		} else {
			sc.emitLoadNone()
		}
		sc.emit(bytecode.RETURN_VALUE, 0)

	case *ast.Pass:
		// No-op.

	case *ast.Break:
		loop := sc.currentLoop()
		if loop == nil {
			return fmt.Errorf("compiler: 'break' outside loop at %s", stmt.Pos())
		}
		// For for-loops, pop the iterator from the stack before breaking.
		if loop.isForLoop {
			sc.emit(bytecode.POP_TOP, 0)
		}
		// Emit a JUMP_ABSOLUTE with a placeholder target; will be patched when loop ends.
		idx := sc.emit(bytecode.JUMP_ABSOLUTE, 0)
		loop.breakJumps = append(loop.breakJumps, idx)

	case *ast.Continue:
		loop := sc.currentLoop()
		if loop == nil {
			return fmt.Errorf("compiler: 'continue' outside loop at %s", stmt.Pos())
		}
		// Jump back to the loop's continue target (top of loop test/iteration).
		sc.emit(bytecode.JUMP_ABSOLUTE, uint32(loop.continueTarget))

	case *ast.FunctionDef:
		// Compile the function body into a nested Code object.
		code, err := sc.compiler.compileFunction(s, sc.symtab)
		if err != nil {
			return err
		}

		// Decorator expressions are evaluated top to bottom, before the
		// function object exists, and pushed in that order. Each one is then
		// applied bottom-up once the function is on the stack, so the stack
		// unwinds in the right order without any extra shuffling.
		for _, dec := range s.Decorators {
			if err := sc.compileExpr(dec); err != nil {
				return err
			}
		}

		// Determine MAKE_FUNCTION flags.
		var makeFuncFlags uint32

		// Emit positional defaults tuple if any default values exist.
		// These are pushed FIRST (bottom of stack for MAKE_FUNCTION).
		if s.Args != nil && len(s.Args.Defaults) > 0 {
			for _, def := range s.Args.Defaults {
				if err := sc.compileExpr(def); err != nil {
					return err
				}
			}
			sc.emit(bytecode.BUILD_TUPLE, uint32(len(s.Args.Defaults)))
			makeFuncFlags |= uint32(object.MakeFuncHasDefaults)
		}

		// Emit keyword-only defaults dict if any kw-only params have defaults.
		if s.Args != nil && len(s.Args.KwDefaults) > 0 {
			hasKwDefaults := false
			for _, d := range s.Args.KwDefaults {
				if d != nil {
					hasKwDefaults = true
					break
				}
			}
			if hasKwDefaults {
				// Build a dict of {name: default} for keyword-only params with defaults.
				nEntries := 0
				for i, d := range s.Args.KwDefaults {
					if d != nil {
						// Push key (param name)
						keyIdx := sc.addConstant(s.Args.KwonlyArgs[i].Arg)
						sc.emit(bytecode.LOAD_CONST, uint32(keyIdx))
						// Push value (default expression)
						if err := sc.compileExpr(d); err != nil {
							return err
						}
						nEntries++
					}
				}
				sc.emit(bytecode.BUILD_MAP, uint32(nEntries))
				makeFuncFlags |= uint32(object.MakeFuncHasKwDefaults)
			}
		}

		// If the nested function has free variables, emit LOAD_CLOSURE for each
		// and build a closure tuple. The closure tuple must be on the stack
		// before the code object and name.
		if len(code.Freevars) > 0 {
			for _, freevarName := range code.Freevars {
				// Look up the freevar name in the enclosing (current) scope's
				// cellvars or freevars to find the correct LOAD_CLOSURE index.
				idx := sc.closureIndex(freevarName)
				sc.emit(bytecode.LOAD_CLOSURE, uint32(idx))
			}
			sc.emit(bytecode.BUILD_TUPLE, uint32(len(code.Freevars)))
			makeFuncFlags |= uint32(object.MakeFuncHasClosure)
		}

		// Load the code object and the function name, emit MAKE_FUNCTION.
		codeIdx := sc.addConstant(code)
		sc.emit(bytecode.LOAD_CONST, uint32(codeIdx))
		nameIdx := sc.addConstant(s.Name)
		sc.emit(bytecode.LOAD_CONST, uint32(nameIdx))
		sc.emit(bytecode.MAKE_FUNCTION, makeFuncFlags)

		// Apply the decorators bottom-up: the closest one to the def runs
		// first, and the topmost decorator's return value is what gets bound to
		// the name.
		for i := len(s.Decorators) - 1; i >= 0; i-- {
			sc.emit(bytecode.CALL_FUNCTION, 1)
		}

		// Store the (possibly decorated) function object with its name.
		if err := sc.compileStoreName(s.Name); err != nil {
			return err
		}

	case *ast.Delete:
		for _, target := range s.Targets {
			if err := sc.compileDelete(target); err != nil {
				return err
			}
		}

	case *ast.Global:
		// Global declarations are handled at symbol analysis time; no code emission needed.

	case *ast.Nonlocal:
		// Nonlocal declarations are handled at symbol analysis time; no code emission needed.

	case *ast.If:
		if err := sc.compileIf(s); err != nil {
			return err
		}

	case *ast.While:
		if err := sc.compileWhile(s); err != nil {
			return err
		}

	case *ast.For:
		if err := sc.compileFor(s); err != nil {
			return err
		}

	case *ast.Raise:
		if s.Exc != nil {
			if err := sc.compileExpr(s.Exc); err != nil {
				return err
			}
			sc.emit(bytecode.RAISE_VARARGS, 1)
		} else {
			sc.emit(bytecode.RAISE_VARARGS, 0)
		}

	case *ast.Import:
		// import module → IMPORT_NAME, store as alias or module name
		// import module as alias → IMPORT_NAME, store as alias
		for _, alias := range s.Names {
			// IMPORT_NAME takes the module name from the Names table.
			nameIdx := sc.symtab.nameIndex(alias.Name)
			sc.emit(bytecode.IMPORT_NAME, uint32(nameIdx))
			// Store the module under the alias (or its own name).
			storeName := alias.AsName
			if storeName == "" {
				storeName = alias.Name
			}
			if err := sc.compileStoreName(storeName); err != nil {
				return err
			}
		}

	case *ast.ImportFrom:
		// from module import name1, name2, ...
		// Emit IMPORT_NAME for the module, then IMPORT_FROM for each name.
		moduleNameIdx := sc.symtab.nameIndex(s.Module)
		sc.emit(bytecode.IMPORT_NAME, uint32(moduleNameIdx))
		for i, alias := range s.Names {
			// IMPORT_FROM pops nothing — it peeks at the module on TOS.
			attrNameIdx := sc.symtab.nameIndex(alias.Name)
			sc.emit(bytecode.IMPORT_FROM, uint32(attrNameIdx))
			// Store under alias or original name.
			storeName := alias.AsName
			if storeName == "" {
				storeName = alias.Name
			}
			if err := sc.compileStoreName(storeName); err != nil {
				return err
			}
			// If this is not the last import, the module stays on stack for the next IMPORT_FROM.
			_ = i
		}
		// Pop the module object from the stack.
		sc.emit(bytecode.POP_TOP, 0)

	case *ast.Try:
		if err := sc.compileTry(s); err != nil {
			return err
		}

	case *ast.Assert:
		// Compile test, emit jump if true, raise AssertionError if false.
		if err := sc.compileExpr(s.Test); err != nil {
			return err
		}
		jumpIdx := sc.emit(bytecode.POP_JUMP_IF_TRUE, 0)
		// Load AssertionError name.
		nameIdx := sc.symtab.nameIndex("AssertionError")
		sc.emit(bytecode.LOAD_GLOBAL, uint32(nameIdx))
		if s.Msg != nil {
			if err := sc.compileExpr(s.Msg); err != nil {
				return err
			}
			sc.emit(bytecode.CALL_FUNCTION, 1)
		} else {
			sc.emit(bytecode.CALL_FUNCTION, 0)
		}
		sc.emit(bytecode.RAISE_VARARGS, 1)
		sc.patchJump(jumpIdx)

	default:
		return fmt.Errorf("compiler: unsupported statement type %T at %s", stmt, stmt.Pos())
	}
	return nil
}

// compileExpr compiles an expression node, leaving its value on top of the stack.
func (sc *scopeCompiler) compileExpr(expr ast.Expr) error {
	switch e := expr.(type) {
	case *ast.Constant:
		sc.compileConstant(e)

	case *ast.Name:
		sc.compileLoadName(e.Id)

	case *ast.BinOp:
		if err := sc.compileExpr(e.Left); err != nil {
			return err
		}
		if err := sc.compileExpr(e.Right); err != nil {
			return err
		}
		op, err := binOpToOpcode(e.Op)
		if err != nil {
			return err
		}
		sc.emit(op, 0)

	case *ast.UnaryOp:
		if err := sc.compileExpr(e.Operand); err != nil {
			return err
		}
		op, err := unaryOpToOpcode(e.Op)
		if err != nil {
			return err
		}
		sc.emit(op, 0)

	case *ast.BoolOp:
		if err := sc.compileBoolOp(e); err != nil {
			return err
		}

	case *ast.Compare:
		if err := sc.compileCompare(e); err != nil {
			return err
		}

	case *ast.Call:
		if err := sc.compileCall(e); err != nil {
			return err
		}

	case *ast.Attribute:
		if err := sc.compileExpr(e.Value); err != nil {
			return err
		}
		nameIdx := sc.symtab.nameIndex(e.Attr)
		sc.emit(bytecode.LOAD_ATTR, uint32(nameIdx))

	case *ast.Subscript:
		if err := sc.compileExpr(e.Value); err != nil {
			return err
		}
		if err := sc.compileExpr(e.Slice); err != nil {
			return err
		}
		sc.emit(bytecode.BINARY_SUBSCR, 0)

	case *ast.List:
		for _, elt := range e.Elts {
			if err := sc.compileExpr(elt); err != nil {
				return err
			}
		}
		sc.emit(bytecode.BUILD_LIST, uint32(len(e.Elts)))

	case *ast.Tuple:
		for _, elt := range e.Elts {
			if err := sc.compileExpr(elt); err != nil {
				return err
			}
		}
		sc.emit(bytecode.BUILD_TUPLE, uint32(len(e.Elts)))

	case *ast.Dict:
		for i, key := range e.Keys {
			if err := sc.compileExpr(key); err != nil {
				return err
			}
			if err := sc.compileExpr(e.Values[i]); err != nil {
				return err
			}
		}
		sc.emit(bytecode.BUILD_MAP, uint32(len(e.Keys)))

	case *ast.IfExp:
		// Ternary: body if test else orelse
		if err := sc.compileExpr(e.Test); err != nil {
			return err
		}
		jumpFalse := sc.emit(bytecode.POP_JUMP_IF_FALSE, 0)
		if err := sc.compileExpr(e.Body); err != nil {
			return err
		}
		jumpEnd := sc.emit(bytecode.JUMP_FORWARD, 0)
		sc.patchJump(jumpFalse)
		if err := sc.compileExpr(e.Orelse); err != nil {
			return err
		}
		sc.patchJumpForward(jumpEnd)

	case *ast.Slice:
		// Emit start, stop, and optional step, then BUILD_SLICE.
		// Omitted components are compiled as None.
		if e.Lower != nil {
			if err := sc.compileExpr(e.Lower); err != nil {
				return err
			}
		} else {
			sc.emitLoadNone()
		}
		if e.Upper != nil {
			if err := sc.compileExpr(e.Upper); err != nil {
				return err
			}
		} else {
			sc.emitLoadNone()
		}
		if e.Step != nil {
			if err := sc.compileExpr(e.Step); err != nil {
				return err
			}
			sc.emit(bytecode.BUILD_SLICE, 3)
		} else {
			sc.emit(bytecode.BUILD_SLICE, 2)
		}

	default:
		return fmt.Errorf("compiler: unsupported expression type %T at %s", expr, expr.Pos())
	}
	return nil
}

// compileConstant emits LOAD_CONST for a literal value.
func (sc *scopeCompiler) compileConstant(c *ast.Constant) {
	var value interface{}
	switch v := c.Value.(type) {
	case int64:
		value = v
	case float64:
		value = v
	case string:
		value = v
	case bool:
		value = v
	case nil:
		value = nil
	default:
		// For any other type, store as-is.
		value = v
	}
	idx := sc.addConstant(value)
	sc.emit(bytecode.LOAD_CONST, uint32(idx))
}

// compileLoadName emits the appropriate load instruction for a name.
func (sc *scopeCompiler) compileLoadName(name string) {
	info := sc.symtab.lookupName(name)
	if info != nil {
		switch info.scope {
		case scopeLocal:
			sc.emit(bytecode.LOAD_FAST, uint32(info.slot))
			return
		case scopeGlobal:
			if sc.scopeType == scopeModule {
				sc.emit(bytecode.LOAD_NAME, uint32(info.slot))
			} else {
				sc.emit(bytecode.LOAD_GLOBAL, uint32(info.slot))
			}
			return
		case scopeCell:
			// Cell variables are accessed via LOAD_DEREF. The arg is the
			// cellvar slot index (into the Cells array in the frame).
			sc.emit(bytecode.LOAD_DEREF, uint32(info.slot))
			return
		case scopeFree:
			// Free variables are accessed via LOAD_DEREF. The arg is offset
			// by len(cellvars) to index into FreeVars in the frame.
			sc.emit(bytecode.LOAD_DEREF, uint32(len(sc.symtab.cellvars)+info.slot))
			return
		}
	}
	// Name not in symbol table — treat as global/builtin lookup.
	idx := sc.symtab.nameIndex(name)
	if sc.scopeType == scopeModule {
		sc.emit(bytecode.LOAD_NAME, uint32(idx))
	} else {
		sc.emit(bytecode.LOAD_GLOBAL, uint32(idx))
	}
}

// compileStore compiles a store operation for an assignment target.
func (sc *scopeCompiler) compileStore(target ast.Expr) error {
	switch t := target.(type) {
	case *ast.Name:
		return sc.compileStoreName(t.Id)

	case *ast.Attribute:
		// Stack has: value to store
		// Compile the object whose attribute we're setting.
		if err := sc.compileExpr(t.Value); err != nil {
			return err
		}
		nameIdx := sc.symtab.nameIndex(t.Attr)
		sc.emit(bytecode.STORE_ATTR, uint32(nameIdx))

	case *ast.Subscript:
		// Stack has: value to store
		// Compile the object and slice.
		if err := sc.compileExpr(t.Value); err != nil {
			return err
		}
		if err := sc.compileExpr(t.Slice); err != nil {
			return err
		}
		sc.emit(bytecode.STORE_SUBSCR, 0)

	case *ast.Tuple:
		// Check for starred unpacking: a, *b, c = expr
		starIdx := -1
		for i, elt := range t.Elts {
			if _, ok := elt.(*ast.Starred); ok {
				if starIdx != -1 {
					return fmt.Errorf("SyntaxError: multiple starred expressions in assignment at %s", target.Pos())
				}
				starIdx = i
			}
		}
		if starIdx >= 0 {
			// Starred unpacking: emit UNPACK_EX with before|after<<8
			before := starIdx
			after := len(t.Elts) - starIdx - 1
			arg := uint32(before) | uint32(after)<<8
			sc.emit(bytecode.UNPACK_EX, arg)
			for _, elt := range t.Elts {
				if starred, ok := elt.(*ast.Starred); ok {
					if err := sc.compileStore(starred.Value); err != nil {
						return err
					}
				} else {
					if err := sc.compileStore(elt); err != nil {
						return err
					}
				}
			}
		} else {
			// Tuple unpacking.
			sc.emit(bytecode.UNPACK_SEQUENCE, uint32(len(t.Elts)))
			for _, elt := range t.Elts {
				if err := sc.compileStore(elt); err != nil {
					return err
				}
			}
		}

	case *ast.List:
		// Check for starred unpacking: [a, *b, c] = expr
		starIdx := -1
		for i, elt := range t.Elts {
			if _, ok := elt.(*ast.Starred); ok {
				if starIdx != -1 {
					return fmt.Errorf("SyntaxError: multiple starred expressions in assignment at %s", target.Pos())
				}
				starIdx = i
			}
		}
		if starIdx >= 0 {
			// Starred unpacking: emit UNPACK_EX with before|after<<8
			before := starIdx
			after := len(t.Elts) - starIdx - 1
			arg := uint32(before) | uint32(after)<<8
			sc.emit(bytecode.UNPACK_EX, arg)
			for _, elt := range t.Elts {
				if starred, ok := elt.(*ast.Starred); ok {
					if err := sc.compileStore(starred.Value); err != nil {
						return err
					}
				} else {
					if err := sc.compileStore(elt); err != nil {
						return err
					}
				}
			}
		} else {
			// List unpacking (same as tuple for assignment targets).
			sc.emit(bytecode.UNPACK_SEQUENCE, uint32(len(t.Elts)))
			for _, elt := range t.Elts {
				if err := sc.compileStore(elt); err != nil {
					return err
				}
			}
		}

	default:
		return fmt.Errorf("compiler: unsupported assignment target %T at %s", target, target.Pos())
	}
	return nil
}

// compileStoreName emits the appropriate store instruction for a name.
func (sc *scopeCompiler) compileStoreName(name string) error {
	info := sc.symtab.lookupName(name)
	if info != nil {
		switch info.scope {
		case scopeLocal:
			sc.emit(bytecode.STORE_FAST, uint32(info.slot))
			return nil
		case scopeGlobal:
			if sc.scopeType == scopeModule {
				sc.emit(bytecode.STORE_NAME, uint32(info.slot))
			} else {
				sc.emit(bytecode.STORE_GLOBAL, uint32(info.slot))
			}
			return nil
		case scopeCell:
			// Cell variables are stored via STORE_DEREF. The arg is the
			// cellvar slot index.
			sc.emit(bytecode.STORE_DEREF, uint32(info.slot))
			return nil
		case scopeFree:
			// Free variables (nonlocal) are stored via STORE_DEREF.
			// The arg is offset by len(cellvars).
			sc.emit(bytecode.STORE_DEREF, uint32(len(sc.symtab.cellvars)+info.slot))
			return nil
		}
	}
	// Name not in symbol table — store as global.
	idx := sc.symtab.nameIndex(name)
	if sc.scopeType == scopeModule {
		sc.emit(bytecode.STORE_NAME, uint32(idx))
	} else {
		sc.emit(bytecode.STORE_GLOBAL, uint32(idx))
	}
	return nil
}

// compileDelete emits the appropriate delete instruction for a target.
func (sc *scopeCompiler) compileDelete(target ast.Expr) error {
	switch t := target.(type) {
	case *ast.Name:
		info := sc.symtab.lookupName(t.Id)
		if info != nil {
			switch info.scope {
			case scopeLocal:
				sc.emit(bytecode.DELETE_FAST, uint32(info.slot))
				return nil
			case scopeGlobal:
				if sc.scopeType == scopeModule {
					sc.emit(bytecode.DELETE_NAME, uint32(info.slot))
				} else {
					sc.emit(bytecode.DELETE_GLOBAL, uint32(info.slot))
				}
				return nil
			case scopeCell:
				sc.emit(bytecode.DELETE_DEREF, uint32(info.slot))
				return nil
			case scopeFree:
				sc.emit(bytecode.DELETE_DEREF, uint32(len(sc.symtab.cellvars)+info.slot))
				return nil
			}
		}
		idx := sc.symtab.nameIndex(t.Id)
		if sc.scopeType == scopeModule {
			sc.emit(bytecode.DELETE_NAME, uint32(idx))
		} else {
			sc.emit(bytecode.DELETE_GLOBAL, uint32(idx))
		}

	case *ast.Attribute:
		if err := sc.compileExpr(t.Value); err != nil {
			return err
		}
		nameIdx := sc.symtab.nameIndex(t.Attr)
		sc.emit(bytecode.DELETE_ATTR, uint32(nameIdx))

	case *ast.Subscript:
		if err := sc.compileExpr(t.Value); err != nil {
			return err
		}
		if err := sc.compileExpr(t.Slice); err != nil {
			return err
		}
		sc.emit(bytecode.DELETE_SUBSCR, 0)

	default:
		return fmt.Errorf("compiler: unsupported delete target %T at %s", target, target.Pos())
	}
	return nil
}

// compileBoolOp compiles and/or expressions with short-circuit semantics.
func (sc *scopeCompiler) compileBoolOp(e *ast.BoolOp) error {
	// Compile values with short-circuit jumps.
	var jumpOp bytecode.Opcode
	if e.Op == ast.And {
		jumpOp = bytecode.JUMP_IF_FALSE_OR_POP
	} else {
		jumpOp = bytecode.JUMP_IF_TRUE_OR_POP
	}

	// Compile all but the last value with conditional jumps.
	var jumps []int
	for i := 0; i < len(e.Values)-1; i++ {
		if err := sc.compileExpr(e.Values[i]); err != nil {
			return err
		}
		jumps = append(jumps, sc.emit(jumpOp, 0))
	}
	// Last value is compiled normally (no jump after it).
	if err := sc.compileExpr(e.Values[len(e.Values)-1]); err != nil {
		return err
	}
	// Patch all jumps to point to here (after the last value).
	for _, jmp := range jumps {
		sc.patchJump(jmp)
	}
	return nil
}

// compileCompare compiles comparison expressions.
// For simple comparisons (one operator), emit: left, right, COMPARE_OP.
// For chained comparisons (a < b < c), emit the chain with short-circuit logic.
func (sc *scopeCompiler) compileCompare(e *ast.Compare) error {
	if err := sc.compileExpr(e.Left); err != nil {
		return err
	}

	if len(e.Ops) == 1 {
		// Simple comparison.
		if err := sc.compileExpr(e.Comparators[0]); err != nil {
			return err
		}
		cmpOp := cmpOpToCompareOp(e.Ops[0])
		sc.emit(bytecode.COMPARE_OP, uint32(cmpOp))
		return nil
	}

	// Chained comparison: a < b < c → a < b and b < c
	// But we need short-circuit: if a < b is false, skip the rest.
	var jumps []int
	for i := 0; i < len(e.Ops)-1; i++ {
		if err := sc.compileExpr(e.Comparators[i]); err != nil {
			return err
		}
		// Duplicate the comparator for the next comparison.
		sc.emit(bytecode.DUP_TOP, 0)
		sc.emit(bytecode.ROT_THREE, 0)
		cmpOp := cmpOpToCompareOp(e.Ops[i])
		sc.emit(bytecode.COMPARE_OP, uint32(cmpOp))
		jumps = append(jumps, sc.emit(bytecode.JUMP_IF_FALSE_OR_POP, 0))
	}
	// Last comparison.
	if err := sc.compileExpr(e.Comparators[len(e.Comparators)-1]); err != nil {
		return err
	}
	cmpOp := cmpOpToCompareOp(e.Ops[len(e.Ops)-1])
	sc.emit(bytecode.COMPARE_OP, uint32(cmpOp))

	// Patch short-circuit jumps.
	// If any intermediate comparison was false, we jump here.
	// But we need to clean up the duplicated value first.
	// For simplicity in MVP, just patch to current position.
	endJump := sc.emit(bytecode.JUMP_FORWARD, 0)
	for _, jmp := range jumps {
		sc.patchJump(jmp)
	}
	// Clean up the duplicated comparator left on stack.
	sc.emit(bytecode.ROT_TWO, 0)
	sc.emit(bytecode.POP_TOP, 0)
	sc.patchJumpForward(endJump)

	return nil
}

// compileCall compiles a function call expression.
func (sc *scopeCompiler) compileCall(e *ast.Call) error {
	// Check if we need CALL_FUNCTION_EX (for *args or **kwargs unpacking).
	if sc.callNeedsUnpacking(e) {
		return sc.compileCallWithUnpacking(e)
	}

	// Compile the callable.
	if err := sc.compileExpr(e.Func); err != nil {
		return err
	}
	// Compile positional arguments.
	for _, arg := range e.Args {
		if err := sc.compileExpr(arg); err != nil {
			return err
		}
	}

	if len(e.Keywords) > 0 {
		// Compile keyword argument values.
		kwNames := make([]interface{}, 0, len(e.Keywords))
		for _, kw := range e.Keywords {
			if err := sc.compileExpr(kw.Value); err != nil {
				return err
			}
			kwNames = append(kwNames, kw.Arg)
		}
		// Push tuple of keyword names as a constant.
		kwTupleIdx := sc.addConstant(kwNames)
		sc.emit(bytecode.LOAD_CONST, uint32(kwTupleIdx))
		sc.emit(bytecode.CALL_FUNCTION_KW, uint32(len(e.Args)+len(e.Keywords)))
	} else {
		sc.emit(bytecode.CALL_FUNCTION, uint32(len(e.Args)))
	}
	return nil
}

// callNeedsUnpacking returns true if the call contains *args or **kwargs unpacking.
func (sc *scopeCompiler) callNeedsUnpacking(e *ast.Call) bool {
	// Check for *expr in positional args.
	for _, arg := range e.Args {
		if _, ok := arg.(*ast.Starred); ok {
			return true
		}
	}
	// Check for **expr in keywords (Keyword.Arg == "").
	for _, kw := range e.Keywords {
		if kw.Arg == "" {
			return true
		}
	}
	return false
}

// compileCallWithUnpacking compiles a call that uses *args or **kwargs unpacking.
// It uses CALL_FUNCTION_EX which expects (func, args_tuple, kwargs_dict) on the stack.
// CALL_FUNCTION_EX arg: 0 = no kwargs, 1 = has kwargs dict on TOS.
func (sc *scopeCompiler) compileCallWithUnpacking(e *ast.Call) error {
	// Compile the callable.
	if err := sc.compileExpr(e.Func); err != nil {
		return err
	}

	// Build the args tuple by combining positional args and *expr expansions.
	// Strategy: build individual tuples/lists for each segment, then use
	// BUILD_TUPLE_UNPACK_WITH_CALL to merge them.
	argSegments := 0
	var currentPositional []ast.Expr

	flushPositional := func() error {
		if len(currentPositional) > 0 {
			for _, arg := range currentPositional {
				if err := sc.compileExpr(arg); err != nil {
					return err
				}
			}
			sc.emit(bytecode.BUILD_TUPLE, uint32(len(currentPositional)))
			argSegments++
			currentPositional = nil
		}
		return nil
	}

	for _, arg := range e.Args {
		if starred, ok := arg.(*ast.Starred); ok {
			// Flush any accumulated positional args as a tuple.
			if err := flushPositional(); err != nil {
				return err
			}
			// Compile the starred expression (an iterable to be unpacked).
			if err := sc.compileExpr(starred.Value); err != nil {
				return err
			}
			argSegments++
		} else {
			currentPositional = append(currentPositional, arg)
		}
	}
	// Flush remaining positional args.
	if err := flushPositional(); err != nil {
		return err
	}

	if argSegments == 0 {
		// No positional args at all — push empty tuple.
		sc.emit(bytecode.BUILD_TUPLE, 0)
	} else if argSegments > 1 {
		// Merge multiple segments into one tuple.
		sc.emit(bytecode.BUILD_TUPLE_UNPACK_WITH_CALL, uint32(argSegments))
	}
	// If argSegments == 1, the single tuple is already on TOS.

	// Build the kwargs dict if there are keyword arguments.
	hasKwargs := len(e.Keywords) > 0
	if hasKwargs {
		kwSegments := 0
		var currentKw []*ast.Keyword

		flushKw := func() error {
			if len(currentKw) > 0 {
				// Build a dict from the accumulated keyword args.
				for _, kw := range currentKw {
					nameIdx := sc.addConstant(kw.Arg)
					sc.emit(bytecode.LOAD_CONST, uint32(nameIdx))
					if err := sc.compileExpr(kw.Value); err != nil {
						return err
					}
				}
				sc.emit(bytecode.BUILD_MAP, uint32(len(currentKw)))
				kwSegments++
				currentKw = nil
			}
			return nil
		}

		for _, kw := range e.Keywords {
			if kw.Arg == "" {
				// **expr — flush current keywords, then push the dict expression.
				if err := flushKw(); err != nil {
					return err
				}
				if err := sc.compileExpr(kw.Value); err != nil {
					return err
				}
				kwSegments++
			} else {
				currentKw = append(currentKw, kw)
			}
		}
		// Flush remaining keywords.
		if err := flushKw(); err != nil {
			return err
		}

		if kwSegments > 1 {
			// Merge multiple dicts.
			sc.emit(bytecode.BUILD_MAP_UNPACK, uint32(kwSegments))
		}
		// If kwSegments == 1, the single dict is already on TOS.

		sc.emit(bytecode.CALL_FUNCTION_EX, 1) // 1 = has kwargs
	} else {
		sc.emit(bytecode.CALL_FUNCTION_EX, 0) // 0 = no kwargs
	}

	return nil
}

// compileIf compiles an if/elif/else statement.
//
// Structure:
//
//	<test>
//	POP_JUMP_IF_FALSE  else_branch
//	<body>
//	JUMP_FORWARD       end
//	else_branch:
//	<orelse>            (may be another If node for elif)
//	end:
func (sc *scopeCompiler) compileIf(s *ast.If) error {
	// Compile the test expression.
	if err := sc.compileExpr(s.Test); err != nil {
		return err
	}
	// POP_JUMP_IF_FALSE to else/elif branch.
	jumpIdx := sc.emit(bytecode.POP_JUMP_IF_FALSE, 0)

	// Compile the body.
	for _, bodyStmt := range s.Body {
		if err := sc.compileStmt(bodyStmt); err != nil {
			return err
		}
	}

	if len(s.Orelse) > 0 {
		// Jump over the else/elif branch.
		jumpOverElse := sc.emit(bytecode.JUMP_FORWARD, 0)
		// Patch the false-branch jump target.
		sc.patchJump(jumpIdx)
		// Compile the else/elif body.
		for _, elseStmt := range s.Orelse {
			if err := sc.compileStmt(elseStmt); err != nil {
				return err
			}
		}
		sc.patchJumpForward(jumpOverElse)
	} else {
		sc.patchJump(jumpIdx)
	}
	return nil
}

// compileWhile compiles a while loop with break/continue support and optional else clause.
//
// Structure:
//
//	loop_start:
//	<test>
//	POP_JUMP_IF_FALSE  else_or_end
//	<body>
//	JUMP_ABSOLUTE      loop_start
//	else_or_end:
//	<orelse>           (runs if loop exits normally, not via break)
//	end:
//	(break jumps patched here)
func (sc *scopeCompiler) compileWhile(s *ast.While) error {
	// Record the loop start for continue and the back-edge.
	loopStart := len(sc.instructions)

	// Push a loop context for break/continue (while loop has no iterator on stack).
	sc.pushLoop(loopStart, false)

	// Compile the test expression.
	if err := sc.compileExpr(s.Test); err != nil {
		return err
	}

	// Jump out if test is false.
	jumpFalse := sc.emit(bytecode.POP_JUMP_IF_FALSE, 0)

	// Compile the loop body.
	for _, bodyStmt := range s.Body {
		if err := sc.compileStmt(bodyStmt); err != nil {
			return err
		}
	}

	// Back-edge: jump to loop start.
	sc.emit(bytecode.JUMP_ABSOLUTE, uint32(loopStart))

	// Patch the false-exit jump. This is where the else clause starts (if any).
	sc.patchJump(jumpFalse)

	// Compile else clause (runs when the loop exits normally without break).
	if len(s.Orelse) > 0 {
		for _, elseStmt := range s.Orelse {
			if err := sc.compileStmt(elseStmt); err != nil {
				return err
			}
		}
	}

	// Pop loop context and patch break jumps to here (after else clause).
	loop := sc.popLoop()
	for _, breakIdx := range loop.breakJumps {
		sc.instructions[breakIdx].Arg = uint32(len(sc.instructions))
	}

	return nil
}

// compileFor compiles a for loop with break/continue support and optional else clause.
//
// Structure:
//
//	<iter>
//	GET_ITER
//	loop_start:
//	FOR_ITER           else_or_end    (jumps when iterator exhausted)
//	<store target>
//	<body>
//	JUMP_ABSOLUTE      loop_start
//	else_or_end:
//	<orelse>           (runs if loop exits normally, not via break)
//	end:
//	(break jumps patched here)
func (sc *scopeCompiler) compileFor(s *ast.For) error {
	// Compile the iterable expression.
	if err := sc.compileExpr(s.Iter); err != nil {
		return err
	}

	// Get an iterator.
	sc.emit(bytecode.GET_ITER, 0)

	// The loop start is at the FOR_ITER instruction.
	loopStart := len(sc.instructions)

	// Push loop context. Continue target is the FOR_ITER. Mark as for-loop (iterator on stack).
	sc.pushLoop(loopStart, true)

	// FOR_ITER: get next value, or jump when exhausted.
	forIterIdx := sc.emit(bytecode.FOR_ITER, 0)

	// Store the iteration value to the target.
	if err := sc.compileStore(s.Target); err != nil {
		return err
	}

	// Compile the loop body.
	for _, bodyStmt := range s.Body {
		if err := sc.compileStmt(bodyStmt); err != nil {
			return err
		}
	}

	// Back-edge: jump to FOR_ITER.
	sc.emit(bytecode.JUMP_ABSOLUTE, uint32(loopStart))

	// Patch FOR_ITER to jump here when the iterator is exhausted.
	// FOR_ITER uses a forward delta (like JUMP_FORWARD).
	sc.patchJumpForward(forIterIdx)

	// Compile else clause (runs when the loop exits normally without break).
	if len(s.Orelse) > 0 {
		for _, elseStmt := range s.Orelse {
			if err := sc.compileStmt(elseStmt); err != nil {
				return err
			}
		}
	}

	// Pop loop context and patch break jumps to here (after else clause).
	loop := sc.popLoop()
	for _, breakIdx := range loop.breakJumps {
		sc.instructions[breakIdx].Arg = uint32(len(sc.instructions))
	}

	return nil
}

// compileTry compiles a try/except/else/finally statement.
//
// Structure:
//
//	SETUP_FINALLY  handler
//	<try body>
//	POP_BLOCK
//	<else body>
//	JUMP_FORWARD  end
//	handler:
//	<except handlers>
//	end:
//	<finally body>
func (sc *scopeCompiler) compileTry(s *ast.Try) error {
	hasFinally := len(s.Finalbody) > 0
	hasExcept := len(s.Handlers) > 0

	// If we have a finally block, wrap everything in SETUP_FINALLY for the finally handler.
	var finallySetup int
	if hasFinally {
		finallySetup = sc.emit(bytecode.SETUP_FINALLY, 0)
	}

	// If we have except handlers, set up a try block for them.
	var exceptSetup int
	if hasExcept {
		exceptSetup = sc.emit(bytecode.SETUP_FINALLY, 0)
	}

	// Compile the try body.
	for _, bodyStmt := range s.Body {
		if err := sc.compileStmt(bodyStmt); err != nil {
			return err
		}
	}

	// If we have except handlers, pop the except block on successful try body.
	if hasExcept {
		sc.emit(bytecode.POP_BLOCK, 0)
	}

	// Compile the else body (runs if no exception was raised).
	for _, elseStmt := range s.Orelse {
		if err := sc.compileStmt(elseStmt); err != nil {
			return err
		}
	}

	// Jump over the except handlers (the try body completed successfully).
	var jumpOverHandlers int
	if hasExcept {
		jumpOverHandlers = sc.emit(bytecode.JUMP_FORWARD, 0)
	}

	// Patch the except SETUP_FINALLY to point here (start of except handlers).
	if hasExcept {
		sc.patchJumpForward(exceptSetup)
	}

	// Compile except handlers.
	var handlerEndJumps []int
	for i, handler := range s.Handlers {
		if handler.Type != nil {
			// Typed except: check if the exception matches the type.
			// The VM pushes the exception onto the stack when entering a handler.
			// DUP the exception for the type check.
			sc.emit(bytecode.DUP_TOP, 0)
			if err := sc.compileExpr(handler.Type); err != nil {
				return err
			}
			sc.emit(bytecode.COMPARE_OP, uint32(bytecode.CmpExcMatch))
			// If the type doesn't match, jump to the next handler.
			var jumpNext int
			if i < len(s.Handlers)-1 {
				jumpNext = sc.emit(bytecode.POP_JUMP_IF_FALSE, 0)
			} else {
				// Last handler: if it doesn't match, re-raise.
				jumpNext = sc.emit(bytecode.POP_JUMP_IF_FALSE, 0)
			}

			// Handler matches — bind the exception if there's an as-name.
			if handler.Name != "" {
				// Store the exception in the named variable.
				if err := sc.compileStoreName(handler.Name); err != nil {
					return err
				}
			} else {
				// Discard the exception from the stack.
				sc.emit(bytecode.POP_TOP, 0)
			}

			sc.emit(bytecode.POP_EXCEPT, 0)

			// Compile the handler body.
			for _, handlerStmt := range handler.Body {
				if err := sc.compileStmt(handlerStmt); err != nil {
					return err
				}
			}

			// Jump to the end of all handlers.
			handlerEndJumps = append(handlerEndJumps, sc.emit(bytecode.JUMP_FORWARD, 0))

			// Patch the "doesn't match" jump to here for the next handler.
			sc.patchJump(jumpNext)
		} else {
			// Bare except (catches everything).
			if handler.Name != "" {
				// Store the exception in the named variable.
				if err := sc.compileStoreName(handler.Name); err != nil {
					return err
				}
			} else {
				// Discard the exception from the stack.
				sc.emit(bytecode.POP_TOP, 0)
			}

			sc.emit(bytecode.POP_EXCEPT, 0)

			// Compile the handler body.
			for _, handlerStmt := range handler.Body {
				if err := sc.compileStmt(handlerStmt); err != nil {
					return err
				}
			}

			// Jump to the end of all handlers.
			handlerEndJumps = append(handlerEndJumps, sc.emit(bytecode.JUMP_FORWARD, 0))
		}
	}

	// If we exhausted all typed handlers without a match, re-raise the exception.
	if hasExcept && len(s.Handlers) > 0 {
		lastHandler := s.Handlers[len(s.Handlers)-1]
		if lastHandler.Type != nil {
			// All typed handlers failed to match — re-raise.
			sc.emit(bytecode.RAISE_VARARGS, 0)
		}
	}

	// Patch the jump that skips over all handlers (successful try body path).
	if hasExcept {
		sc.patchJumpForward(jumpOverHandlers)
	}

	// Patch all handler-end jumps to point here.
	for _, jmp := range handlerEndJumps {
		sc.patchJumpForward(jmp)
	}

	// Pop the finally block if we set one up.
	if hasFinally {
		sc.emit(bytecode.POP_BLOCK, 0)
		// Push None on normal path — RERAISE at end will see None and continue.
		// On the exception path, the VM pushes the exception onto the stack.
		noneIdx := sc.addConstant(nil)
		sc.emit(bytecode.LOAD_CONST, uint32(noneIdx))
	}

	// Compile the finally body (always executes).
	if hasFinally {
		sc.patchJumpForward(finallySetup)
		for _, finallyStmt := range s.Finalbody {
			if err := sc.compileStmt(finallyStmt); err != nil {
				return err
			}
		}
		// RERAISE pops TOS: if exception (from exception path), re-raises it;
		// if None (from normal path), continues normally.
		sc.emit(bytecode.RERAISE, 0)
	}

	return nil
}

// binOpToOpcode maps an AST binary operator to a bytecode opcode.
func binOpToOpcode(op ast.Operator) (bytecode.Opcode, error) {
	switch op {
	case ast.Add:
		return bytecode.BINARY_ADD, nil
	case ast.Sub:
		return bytecode.BINARY_SUBTRACT, nil
	case ast.Mult:
		return bytecode.BINARY_MULTIPLY, nil
	case ast.Div:
		return bytecode.BINARY_TRUE_DIVIDE, nil
	case ast.FloorDiv:
		return bytecode.BINARY_FLOOR_DIVIDE, nil
	case ast.Mod:
		return bytecode.BINARY_MODULO, nil
	case ast.Pow:
		return bytecode.BINARY_POWER, nil
	case ast.LShift:
		return bytecode.BINARY_LSHIFT, nil
	case ast.RShift:
		return bytecode.BINARY_RSHIFT, nil
	case ast.BitOr:
		return bytecode.BINARY_OR, nil
	case ast.BitXor:
		return bytecode.BINARY_XOR, nil
	case ast.BitAnd:
		return bytecode.BINARY_AND, nil
	case ast.MatMult:
		return bytecode.BINARY_MATRIX_MULTIPLY, nil
	default:
		return 0, fmt.Errorf("compiler: unsupported binary operator %v", op)
	}
}

// unaryOpToOpcode maps an AST unary operator to a bytecode opcode.
func unaryOpToOpcode(op ast.UnaryOperator) (bytecode.Opcode, error) {
	switch op {
	case ast.USub:
		return bytecode.UNARY_NEGATIVE, nil
	case ast.UAdd:
		return bytecode.UNARY_POSITIVE, nil
	case ast.Not:
		return bytecode.UNARY_NOT, nil
	case ast.Invert:
		return bytecode.UNARY_INVERT, nil
	default:
		return 0, fmt.Errorf("compiler: unsupported unary operator %v", op)
	}
}

// cmpOpToCompareOp maps an AST comparison operator to a bytecode CompareOp.
func cmpOpToCompareOp(op ast.CmpOp) bytecode.CompareOp {
	switch op {
	case ast.Eq:
		return bytecode.CmpEq
	case ast.NotEq:
		return bytecode.CmpNotEq
	case ast.Lt:
		return bytecode.CmpLt
	case ast.LtE:
		return bytecode.CmpLtE
	case ast.Gt:
		return bytecode.CmpGt
	case ast.GtE:
		return bytecode.CmpGtE
	case ast.Is:
		return bytecode.CmpIs
	case ast.IsNot:
		return bytecode.CmpIsNot
	case ast.In:
		return bytecode.CmpIn
	case ast.NotIn:
		return bytecode.CmpNotIn
	default:
		return bytecode.CmpEq
	}
}
