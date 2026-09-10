package compiler

import (
	"fmt"

	"github.com/alvinjaison/pygo/internal/ast"
)

// nameScope classifies how a name is stored/loaded.
type nameScope int

const (
	// scopeLocal means the name is a local variable (assigned in a function).
	// Accessed via LOAD_FAST/STORE_FAST using an index into Varnames.
	scopeLocal nameScope = iota

	// scopeGlobal means the name is a global variable (module-level or explicitly declared global).
	// Accessed via LOAD_GLOBAL/STORE_GLOBAL or LOAD_NAME/STORE_NAME at module level.
	scopeGlobal

	// scopeBuiltin means the name is a builtin (not assigned anywhere in scope).
	// Resolved at runtime via the builtins dict.
	scopeBuiltin

	// scopeCell means the variable is captured by a nested function.
	// Accessed via LOAD_DEREF/STORE_DEREF using an index into Cellvars.
	scopeCell

	// scopeFree means the variable comes from an enclosing scope.
	// Accessed via LOAD_DEREF/STORE_DEREF using an index into Freevars.
	scopeFree
)

// symbolInfo holds classification and slot information for a single name.
type symbolInfo struct {
	scope nameScope
	slot  int // index into Varnames for locals, Cellvars for cell, Freevars for free; -1 for globals/builtins
}

// symbolTable holds the name classification for one scope.
type symbolTable struct {
	// symbols maps name → classification info.
	symbols map[string]*symbolInfo

	// varnames is the ordered list of local variable names.
	// The index is used as the operand for LOAD_FAST/STORE_FAST.
	varnames []string

	// names is the ordered list of global/attribute names.
	// The index is used as the operand for LOAD_NAME/STORE_NAME/LOAD_GLOBAL/STORE_GLOBAL.
	names []string

	// cellvars is the ordered list of cell variable names (variables captured by nested functions).
	// The index is used as the operand for LOAD_CLOSURE/LOAD_DEREF/STORE_DEREF.
	cellvars []string

	// freevars is the ordered list of free variable names (variables from enclosing scopes).
	// The index is used as the operand for LOAD_DEREF/STORE_DEREF (offset by len(cellvars)).
	freevars []string

	// globals is the set of names explicitly declared global.
	globals map[string]bool

	// assigned is the set of names assigned in this scope.
	assigned map[string]bool

	// nonlocals is the set of names explicitly declared nonlocal.
	nonlocals map[string]bool

	// children holds symbol tables for nested function scopes (in definition order).
	children []*symbolTable

	// parent points to the enclosing scope's symbol table (nil for module scope).
	parent *symbolTable

	// hasYield is true if this function scope contains a yield or yield from expression.
	hasYield bool

	// referenced is the set of names referenced (loaded) in this scope but not necessarily assigned.
	// Used during multi-pass analysis to detect free variables.
	referenced map[string]bool
}

// newSymbolTable creates an empty symbol table.
func newSymbolTable() *symbolTable {
	return &symbolTable{
		symbols:    make(map[string]*symbolInfo),
		globals:    make(map[string]bool),
		assigned:   make(map[string]bool),
		nonlocals:  make(map[string]bool),
		referenced: make(map[string]bool),
	}
}

// lookupName returns the symbol info for a name, or nil if not found.
func (st *symbolTable) lookupName(name string) *symbolInfo {
	return st.symbols[name]
}

// localSlot returns the Varnames index for a local variable name.
// Returns -1 if the name is not a local.
func (st *symbolTable) localSlot(name string) int {
	if info, ok := st.symbols[name]; ok && info.scope == scopeLocal {
		return info.slot
	}
	return -1
}

// cellSlot returns the Cellvars index for a cell variable name.
// Returns -1 if the name is not a cell variable.
func (st *symbolTable) cellSlot(name string) int {
	if info, ok := st.symbols[name]; ok && info.scope == scopeCell {
		return info.slot
	}
	return -1
}

// freeSlot returns the Freevars index for a free variable name.
// Returns -1 if the name is not a free variable.
func (st *symbolTable) freeSlot(name string) int {
	if info, ok := st.symbols[name]; ok && info.scope == scopeFree {
		return info.slot
	}
	return -1
}

// nameIndex returns the Names index for a global/attribute name, adding it if needed.
func (st *symbolTable) nameIndex(name string) int {
	for i, n := range st.names {
		if n == name {
			return i
		}
	}
	idx := len(st.names)
	st.names = append(st.names, name)
	return idx
}

// analyzeModule builds a symbol table for module-level code.
// At module level, all names are treated as global (stored in Names).
func analyzeModule(module *ast.Module) *symbolTable {
	st := newSymbolTable()

	// Walk the module body to collect assigned names and global declarations.
	collector := &nameCollector{
		assigned: st.assigned,
		globals:  st.globals,
	}
	for _, stmt := range module.Body {
		collector.collectStmt(stmt)
	}

	// At module level, all names are global — stored in the Names pool.
	// Use assignOrder to ensure deterministic slot allocation.
	for _, name := range collector.assignOrder {
		idx := st.nameIndex(name)
		st.symbols[name] = &symbolInfo{scope: scopeGlobal, slot: idx}
	}

	// Build child symbol tables for nested functions (needed for closure analysis).
	for _, stmt := range module.Body {
		if funcDef, ok := stmt.(*ast.FunctionDef); ok {
			child := analyzeFunctionPass1(funcDef, st)
			st.children = append(st.children, child)
		}
	}

	// Run cross-scope analysis passes on all children.
	for _, child := range st.children {
		resolveClosures(child)
	}

	return st
}

// analyzeFunction builds a symbol table for a function scope.
// Names assigned within the function are local (unless declared global).
// Parameters are always local.
// This is the main entry point that runs all analysis passes.
func analyzeFunction(funcDef *ast.FunctionDef, parent *symbolTable) *symbolTable {
	st := analyzeFunctionPass1(funcDef, parent)

	// Register this child in the parent's children list if not already there.
	// (analyzeFunction is called by the compiler, so we connect the tree here.)
	found := false
	for _, c := range parent.children {
		if c == st {
			found = true
			break
		}
	}
	if !found {
		parent.children = append(parent.children, st)
	}

	// Run multi-pass closure analysis from this scope downward.
	resolveClosures(st)

	return st
}

// analyzeFunctionPass1 performs Pass 1: collect assignments, globals, nonlocals,
// parameters, and references. It also recursively builds child symbol tables
// for nested functions.
func analyzeFunctionPass1(funcDef *ast.FunctionDef, parent *symbolTable) *symbolTable {
	st := newSymbolTable()
	st.parent = parent

	// Collect global declarations, nonlocal declarations, and assignments in the function body.
	collector := &nameCollector{
		assigned:  st.assigned,
		globals:   st.globals,
		nonlocals: st.nonlocals,
	}
	for _, stmt := range funcDef.Body {
		collector.collectStmt(stmt)
	}

	// Collect name references (names loaded but not necessarily assigned).
	refCollector := &refCollector{referenced: st.referenced}
	for _, stmt := range funcDef.Body {
		refCollector.collectStmt(stmt)
	}

	// Pass 4: Detect yield/yield from in this function body.
	st.hasYield = detectYield(funcDef.Body)

	// Parameters are always local.
	if funcDef.Args != nil {
		for _, arg := range funcDef.Args.PosonlyArgs {
			st.assigned[arg.Arg] = true
		}
		for _, arg := range funcDef.Args.Args {
			st.assigned[arg.Arg] = true
		}
		if funcDef.Args.Vararg != nil {
			st.assigned[funcDef.Args.Vararg.Arg] = true
		}
		for _, arg := range funcDef.Args.KwonlyArgs {
			st.assigned[arg.Arg] = true
		}
		if funcDef.Args.Kwarg != nil {
			st.assigned[funcDef.Args.Kwarg.Arg] = true
		}
	}

	// Classify names:
	// - Explicitly declared global → global scope
	// - Assigned (and not declared global/nonlocal) → local scope
	// - Referenced but not assigned → will be resolved in Pass 2/3

	// First, assign local slots for all non-global, non-nonlocal assigned names.
	// Parameters come first in varnames, in definition order.
	if funcDef.Args != nil {
		for _, arg := range funcDef.Args.PosonlyArgs {
			if !st.globals[arg.Arg] && !st.nonlocals[arg.Arg] {
				st.addLocal(arg.Arg)
			}
		}
		for _, arg := range funcDef.Args.Args {
			if !st.globals[arg.Arg] && !st.nonlocals[arg.Arg] {
				st.addLocal(arg.Arg)
			}
		}
		if funcDef.Args.Vararg != nil && !st.globals[funcDef.Args.Vararg.Arg] && !st.nonlocals[funcDef.Args.Vararg.Arg] {
			st.addLocal(funcDef.Args.Vararg.Arg)
		}
		for _, arg := range funcDef.Args.KwonlyArgs {
			if !st.globals[arg.Arg] && !st.nonlocals[arg.Arg] {
				st.addLocal(arg.Arg)
			}
		}
		if funcDef.Args.Kwarg != nil && !st.globals[funcDef.Args.Kwarg.Arg] && !st.nonlocals[funcDef.Args.Kwarg.Arg] {
			st.addLocal(funcDef.Args.Kwarg.Arg)
		}
	}

	// Then add remaining assigned non-global, non-nonlocal names as locals.
	// Use assignOrder to ensure deterministic slot allocation.
	for _, name := range collector.assignOrder {
		if st.globals[name] || st.nonlocals[name] {
			continue
		}
		if _, exists := st.symbols[name]; exists {
			continue // already added (parameter)
		}
		st.addLocal(name)
	}

	// Register explicitly global names.
	// Use globalOrder to ensure deterministic slot allocation.
	for _, name := range collector.globalOrder {
		if _, exists := st.symbols[name]; !exists {
			idx := st.nameIndex(name)
			st.symbols[name] = &symbolInfo{scope: scopeGlobal, slot: idx}
		}
	}

	// Recursively build child symbol tables for nested functions.
	for _, stmt := range funcDef.Body {
		buildChildScopes(st, stmt)
	}

	return st
}

// buildChildScopes recursively walks statements to find nested FunctionDef nodes
// and builds their symbol tables as children of the given parent scope.
func buildChildScopes(parent *symbolTable, stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.FunctionDef:
		child := analyzeFunctionPass1(s, parent)
		parent.children = append(parent.children, child)

	case *ast.For:
		for _, bodyStmt := range s.Body {
			buildChildScopes(parent, bodyStmt)
		}
		for _, elseStmt := range s.Orelse {
			buildChildScopes(parent, elseStmt)
		}

	case *ast.While:
		for _, bodyStmt := range s.Body {
			buildChildScopes(parent, bodyStmt)
		}
		for _, elseStmt := range s.Orelse {
			buildChildScopes(parent, elseStmt)
		}

	case *ast.If:
		for _, bodyStmt := range s.Body {
			buildChildScopes(parent, bodyStmt)
		}
		for _, elseStmt := range s.Orelse {
			buildChildScopes(parent, elseStmt)
		}

	case *ast.Try:
		for _, bodyStmt := range s.Body {
			buildChildScopes(parent, bodyStmt)
		}
		for _, handler := range s.Handlers {
			for _, handlerStmt := range handler.Body {
				buildChildScopes(parent, handlerStmt)
			}
		}
		for _, elseStmt := range s.Orelse {
			buildChildScopes(parent, elseStmt)
		}
		for _, finallyStmt := range s.Finalbody {
			buildChildScopes(parent, finallyStmt)
		}

	case *ast.With:
		for _, bodyStmt := range s.Body {
			buildChildScopes(parent, bodyStmt)
		}
	}
}

// resolveClosures implements Pass 2 and Pass 3 of the multi-pass analysis.
// It resolves free/cell variables across nested scopes.
func resolveClosures(st *symbolTable) {
	// Pass 3: Handle nonlocal declarations.
	// Validate that each nonlocal name exists in an enclosing function scope.
	for name := range st.nonlocals {
		enclosing := findEnclosingDef(st.parent, name)
		if enclosing == nil {
			// This will be caught during compilation; for now mark as free
			// so compilation can raise SyntaxError.
			// We still add it so the error path can detect it.
		}
		// Mark as free in this scope.
		st.addFreevar(name)
		// Mark as cell in the enclosing scope (if found).
		if enclosing != nil {
			enclosing.addCellvar(name)
		}
	}

	// Pass 2: Detect free variables — names referenced in this scope
	// that are not local, global, or builtin, and exist in an enclosing scope.
	for name := range st.referenced {
		// Skip if already classified (local, global, nonlocal/free, or cell).
		if _, exists := st.symbols[name]; exists {
			continue
		}
		if st.globals[name] || st.nonlocals[name] {
			continue
		}
		if st.assigned[name] {
			continue
		}
		// Look up in enclosing scopes.
		enclosing := findEnclosingDef(st.parent, name)
		if enclosing != nil {
			// This name is a free variable in this scope.
			st.addFreevar(name)
			// Mark it as a cell variable in the enclosing scope.
			enclosing.addCellvar(name)
			// Also propagate through intermediate scopes: any scope between
			// enclosing and this scope must also thread the cell/free link.
			propagateFreevar(st.parent, enclosing, name)
		}
		// If not found in any enclosing scope, it remains unresolved
		// (will be a global/builtin lookup at runtime).
	}

	// Recursively resolve closures for all child scopes.
	for _, child := range st.children {
		resolveClosures(child)
	}
}

// propagateFreevar ensures that intermediate scopes between a child and the
// defining enclosing scope also carry the free/cell link for the given name.
// This supports arbitrarily deep nesting: if scope A defines x, and scope C
// (nested inside B, which is nested inside A) references x, then B must also
// have x as a free variable (so it can pass it through).
func propagateFreevar(scope *symbolTable, definingScope *symbolTable, name string) {
	if scope == nil || scope == definingScope {
		return
	}
	// If this intermediate scope doesn't already have the name as cell or free,
	// add it as free so it can be threaded through.
	if info := scope.symbols[name]; info != nil {
		if info.scope == scopeCell || info.scope == scopeFree {
			return // already handled
		}
		if info.scope == scopeLocal {
			// The name is local here, so the defining scope must be this one.
			// This shouldn't happen in propagation since definingScope != scope.
			return
		}
	}
	// Add as free variable in the intermediate scope.
	scope.addFreevar(name)
	// Continue propagating up.
	propagateFreevar(scope.parent, definingScope, name)
}

// findEnclosingDef walks up the parent chain looking for a scope where the
// given name is assigned (local) or declared as a cell variable.
// Returns the symbol table where the name is defined, or nil if not found.
// Stops at module scope (does not consider module-level as enclosing for closures).
func findEnclosingDef(scope *symbolTable, name string) *symbolTable {
	for scope != nil {
		// Module scope is not a valid enclosing scope for closures.
		if scope.parent == nil {
			return nil
		}
		// Check if the name is assigned in this scope (and not declared global).
		if scope.assigned[name] && !scope.globals[name] {
			return scope
		}
		// Check if already marked as cell (from a previous resolution).
		if info, ok := scope.symbols[name]; ok && info.scope == scopeCell {
			return scope
		}
		scope = scope.parent
	}
	return nil
}

// addCellvar promotes a local variable to a cell variable.
// If the name was previously a plain local, it is reclassified as cell.
// The name stays in varnames so that parameter binding in buildFrame still works
// (buildFrame binds arguments by position into frame.Locals). All subsequent
// access in generated code uses LOAD_DEREF/STORE_DEREF via the cell.
func (st *symbolTable) addCellvar(name string) {
	if info, exists := st.symbols[name]; exists {
		if info.scope == scopeCell {
			return // already a cell
		}
		// Reclassify from local to cell. Keep varnames intact for parameter binding.
	}
	// Check if already in cellvars list.
	for _, cv := range st.cellvars {
		if cv == name {
			return
		}
	}
	slot := len(st.cellvars)
	st.cellvars = append(st.cellvars, name)
	st.symbols[name] = &symbolInfo{scope: scopeCell, slot: slot}
}

// addFreevar adds a name as a free variable in this scope.
func (st *symbolTable) addFreevar(name string) {
	if info, exists := st.symbols[name]; exists {
		if info.scope == scopeFree {
			return // already a free variable
		}
	}
	// Check if already in freevars list.
	for _, fv := range st.freevars {
		if fv == name {
			return
		}
	}
	slot := len(st.freevars)
	st.freevars = append(st.freevars, name)
	st.symbols[name] = &symbolInfo{scope: scopeFree, slot: slot}
}

// removeFromVarnames removes a name from the varnames list and re-indexes
// remaining locals. This is used when promoting a local to a cell variable.
func (st *symbolTable) removeFromVarnames(name string) {
	idx := -1
	for i, v := range st.varnames {
		if v == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}
	st.varnames = append(st.varnames[:idx], st.varnames[idx+1:]...)
	// Re-index remaining locals.
	for i := idx; i < len(st.varnames); i++ {
		if info, ok := st.symbols[st.varnames[i]]; ok && info.scope == scopeLocal {
			info.slot = i
		}
	}
}

// ValidateNonlocals checks that all nonlocal declarations refer to names
// that exist in an enclosing function scope. Returns an error if any
// nonlocal name cannot be resolved.
func (st *symbolTable) ValidateNonlocals() error {
	for name := range st.nonlocals {
		enclosing := findEnclosingDef(st.parent, name)
		if enclosing == nil {
			return fmt.Errorf("SyntaxError: no binding for nonlocal '%s' found", name)
		}
	}
	// Recursively validate children.
	for _, child := range st.children {
		if err := child.ValidateNonlocals(); err != nil {
			return err
		}
	}
	return nil
}

// detectYield checks whether a function body contains yield or yield from
// expressions. It does NOT recurse into nested function definitions.
func detectYield(body []ast.Stmt) bool {
	detector := &yieldDetector{}
	for _, stmt := range body {
		if detector.detectInStmt(stmt) {
			return true
		}
	}
	return false
}

// yieldDetector walks statements and expressions looking for Yield/YieldFrom
// without recursing into nested FunctionDef/Lambda scopes.
type yieldDetector struct{}

func (d *yieldDetector) detectInStmt(stmt ast.Stmt) bool {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		return d.detectInExpr(s.Value)
	case *ast.Return:
		if s.Value != nil {
			return d.detectInExpr(s.Value)
		}
	case *ast.Assign:
		for _, t := range s.Targets {
			if d.detectInExpr(t) {
				return true
			}
		}
		return d.detectInExpr(s.Value)
	case *ast.AugAssign:
		return d.detectInExpr(s.Target) || d.detectInExpr(s.Value)
	case *ast.AnnAssign:
		if s.Value != nil && d.detectInExpr(s.Value) {
			return true
		}
	case *ast.For:
		if d.detectInExpr(s.Iter) {
			return true
		}
		for _, bodyStmt := range s.Body {
			if d.detectInStmt(bodyStmt) {
				return true
			}
		}
		for _, elseStmt := range s.Orelse {
			if d.detectInStmt(elseStmt) {
				return true
			}
		}
	case *ast.While:
		if d.detectInExpr(s.Test) {
			return true
		}
		for _, bodyStmt := range s.Body {
			if d.detectInStmt(bodyStmt) {
				return true
			}
		}
		for _, elseStmt := range s.Orelse {
			if d.detectInStmt(elseStmt) {
				return true
			}
		}
	case *ast.If:
		if d.detectInExpr(s.Test) {
			return true
		}
		for _, bodyStmt := range s.Body {
			if d.detectInStmt(bodyStmt) {
				return true
			}
		}
		for _, elseStmt := range s.Orelse {
			if d.detectInStmt(elseStmt) {
				return true
			}
		}
	case *ast.With:
		for _, item := range s.Items {
			if d.detectInExpr(item.ContextExpr) {
				return true
			}
		}
		for _, bodyStmt := range s.Body {
			if d.detectInStmt(bodyStmt) {
				return true
			}
		}
	case *ast.Try:
		for _, bodyStmt := range s.Body {
			if d.detectInStmt(bodyStmt) {
				return true
			}
		}
		for _, handler := range s.Handlers {
			for _, handlerStmt := range handler.Body {
				if d.detectInStmt(handlerStmt) {
					return true
				}
			}
		}
		for _, elseStmt := range s.Orelse {
			if d.detectInStmt(elseStmt) {
				return true
			}
		}
		for _, finallyStmt := range s.Finalbody {
			if d.detectInStmt(finallyStmt) {
				return true
			}
		}
	case *ast.Raise:
		if s.Exc != nil && d.detectInExpr(s.Exc) {
			return true
		}
	case *ast.Assert:
		if d.detectInExpr(s.Test) {
			return true
		}
		if s.Msg != nil && d.detectInExpr(s.Msg) {
			return true
		}
	case *ast.FunctionDef:
		// Do NOT recurse into nested functions — they have their own scope.
	case *ast.ClassDef:
		// Do NOT recurse into class bodies.
	}
	return false
}

func (d *yieldDetector) detectInExpr(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	switch e := expr.(type) {
	case *ast.Yield:
		return true
	case *ast.YieldFrom:
		return true
	case *ast.BinOp:
		return d.detectInExpr(e.Left) || d.detectInExpr(e.Right)
	case *ast.UnaryOp:
		return d.detectInExpr(e.Operand)
	case *ast.BoolOp:
		for _, v := range e.Values {
			if d.detectInExpr(v) {
				return true
			}
		}
	case *ast.Compare:
		if d.detectInExpr(e.Left) {
			return true
		}
		for _, c := range e.Comparators {
			if d.detectInExpr(c) {
				return true
			}
		}
	case *ast.Call:
		if d.detectInExpr(e.Func) {
			return true
		}
		for _, arg := range e.Args {
			if d.detectInExpr(arg) {
				return true
			}
		}
		for _, kw := range e.Keywords {
			if d.detectInExpr(kw.Value) {
				return true
			}
		}
	case *ast.IfExp:
		return d.detectInExpr(e.Test) || d.detectInExpr(e.Body) || d.detectInExpr(e.Orelse)
	case *ast.Attribute:
		return d.detectInExpr(e.Value)
	case *ast.Subscript:
		return d.detectInExpr(e.Value) || d.detectInExpr(e.Slice)
	case *ast.Starred:
		return d.detectInExpr(e.Value)
	case *ast.Tuple:
		for _, elt := range e.Elts {
			if d.detectInExpr(elt) {
				return true
			}
		}
	case *ast.List:
		for _, elt := range e.Elts {
			if d.detectInExpr(elt) {
				return true
			}
		}
	case *ast.Dict:
		for i, k := range e.Keys {
			if k != nil && d.detectInExpr(k) {
				return true
			}
			if d.detectInExpr(e.Values[i]) {
				return true
			}
		}
	case *ast.Set:
		for _, elt := range e.Elts {
			if d.detectInExpr(elt) {
				return true
			}
		}
	case *ast.Lambda:
		// Do NOT recurse into lambda — it's a separate scope.
	case *ast.ListComp, *ast.SetComp, *ast.DictComp, *ast.GeneratorExp:
		// Comprehensions have their own scope — don't recurse.
	case *ast.NamedExpr:
		return d.detectInExpr(e.Value)
	case *ast.Slice:
		if e.Lower != nil && d.detectInExpr(e.Lower) {
			return true
		}
		if e.Upper != nil && d.detectInExpr(e.Upper) {
			return true
		}
		if e.Step != nil && d.detectInExpr(e.Step) {
			return true
		}
	}
	return false
}

// refCollector collects all Name references (loads) in a function body
// without recursing into nested function/class/lambda/comprehension definitions.
type refCollector struct {
	referenced map[string]bool
}

func (rc *refCollector) collectStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		rc.collectExpr(s.Value)
	case *ast.Return:
		if s.Value != nil {
			rc.collectExpr(s.Value)
		}
	case *ast.Assign:
		// Don't collect targets as references — they're stores.
		rc.collectExpr(s.Value)
	case *ast.AugAssign:
		// The target is both loaded and stored; collect the load.
		rc.collectExpr(s.Target)
		rc.collectExpr(s.Value)
	case *ast.AnnAssign:
		if s.Value != nil {
			rc.collectExpr(s.Value)
		}
	case *ast.For:
		rc.collectExpr(s.Iter)
		for _, bodyStmt := range s.Body {
			rc.collectStmt(bodyStmt)
		}
		for _, elseStmt := range s.Orelse {
			rc.collectStmt(elseStmt)
		}
	case *ast.While:
		rc.collectExpr(s.Test)
		for _, bodyStmt := range s.Body {
			rc.collectStmt(bodyStmt)
		}
		for _, elseStmt := range s.Orelse {
			rc.collectStmt(elseStmt)
		}
	case *ast.If:
		rc.collectExpr(s.Test)
		for _, bodyStmt := range s.Body {
			rc.collectStmt(bodyStmt)
		}
		for _, elseStmt := range s.Orelse {
			rc.collectStmt(elseStmt)
		}
	case *ast.With:
		for _, item := range s.Items {
			rc.collectExpr(item.ContextExpr)
		}
		for _, bodyStmt := range s.Body {
			rc.collectStmt(bodyStmt)
		}
	case *ast.Try:
		for _, bodyStmt := range s.Body {
			rc.collectStmt(bodyStmt)
		}
		for _, handler := range s.Handlers {
			for _, handlerStmt := range handler.Body {
				rc.collectStmt(handlerStmt)
			}
		}
		for _, elseStmt := range s.Orelse {
			rc.collectStmt(elseStmt)
		}
		for _, finallyStmt := range s.Finalbody {
			rc.collectStmt(finallyStmt)
		}
	case *ast.Raise:
		if s.Exc != nil {
			rc.collectExpr(s.Exc)
		}
		if s.Cause != nil {
			rc.collectExpr(s.Cause)
		}
	case *ast.Assert:
		rc.collectExpr(s.Test)
		if s.Msg != nil {
			rc.collectExpr(s.Msg)
		}
	case *ast.FunctionDef:
		// Do NOT recurse into nested functions.
		// But do collect references from decorator expressions and default args.
		for _, dec := range s.Decorators {
			rc.collectExpr(dec)
		}
		if s.Args != nil {
			for _, d := range s.Args.Defaults {
				rc.collectExpr(d)
			}
			for _, d := range s.Args.KwDefaults {
				if d != nil {
					rc.collectExpr(d)
				}
			}
		}
	case *ast.ClassDef:
		// Do NOT recurse into class bodies.
		for _, dec := range s.Decorators {
			rc.collectExpr(dec)
		}
		for _, base := range s.Bases {
			rc.collectExpr(base)
		}
	case *ast.Delete:
		for _, target := range s.Targets {
			rc.collectExpr(target)
		}
	case *ast.Global, *ast.Nonlocal, *ast.Pass, *ast.Break, *ast.Continue:
		// No references to collect.
	case *ast.Import, *ast.ImportFrom:
		// No references to collect.
	}
}

func (rc *refCollector) collectExpr(expr ast.Expr) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.Name:
		rc.referenced[e.Id] = true
	case *ast.BinOp:
		rc.collectExpr(e.Left)
		rc.collectExpr(e.Right)
	case *ast.UnaryOp:
		rc.collectExpr(e.Operand)
	case *ast.BoolOp:
		for _, v := range e.Values {
			rc.collectExpr(v)
		}
	case *ast.Compare:
		rc.collectExpr(e.Left)
		for _, c := range e.Comparators {
			rc.collectExpr(c)
		}
	case *ast.Call:
		rc.collectExpr(e.Func)
		for _, arg := range e.Args {
			rc.collectExpr(arg)
		}
		for _, kw := range e.Keywords {
			rc.collectExpr(kw.Value)
		}
	case *ast.IfExp:
		rc.collectExpr(e.Test)
		rc.collectExpr(e.Body)
		rc.collectExpr(e.Orelse)
	case *ast.Attribute:
		rc.collectExpr(e.Value)
	case *ast.Subscript:
		rc.collectExpr(e.Value)
		rc.collectExpr(e.Slice)
	case *ast.Starred:
		rc.collectExpr(e.Value)
	case *ast.Tuple:
		for _, elt := range e.Elts {
			rc.collectExpr(elt)
		}
	case *ast.List:
		for _, elt := range e.Elts {
			rc.collectExpr(elt)
		}
	case *ast.Dict:
		for i, k := range e.Keys {
			if k != nil {
				rc.collectExpr(k)
			}
			rc.collectExpr(e.Values[i])
		}
	case *ast.Set:
		for _, elt := range e.Elts {
			rc.collectExpr(elt)
		}
	case *ast.Yield:
		if e.Value != nil {
			rc.collectExpr(e.Value)
		}
	case *ast.YieldFrom:
		rc.collectExpr(e.Value)
	case *ast.Lambda:
		// Do NOT recurse into lambda — it's a separate scope.
	case *ast.ListComp, *ast.SetComp, *ast.DictComp, *ast.GeneratorExp:
		// Comprehensions have their own scope — don't recurse.
	case *ast.NamedExpr:
		rc.collectExpr(e.Value)
	case *ast.Slice:
		if e.Lower != nil {
			rc.collectExpr(e.Lower)
		}
		if e.Upper != nil {
			rc.collectExpr(e.Upper)
		}
		if e.Step != nil {
			rc.collectExpr(e.Step)
		}
	case *ast.Constant:
		// No references.
	case *ast.Await:
		rc.collectExpr(e.Value)
	}
}

// addLocal adds a name as a local variable and assigns it a slot.
func (st *symbolTable) addLocal(name string) {
	if _, exists := st.symbols[name]; exists {
		return // already added
	}
	slot := len(st.varnames)
	st.varnames = append(st.varnames, name)
	st.symbols[name] = &symbolInfo{scope: scopeLocal, slot: slot}
}

// nameCollector walks statements to collect assigned names and global declarations.
// It does NOT recurse into nested function/class definitions (those are separate scopes).
type nameCollector struct {
	assigned  map[string]bool
	globals   map[string]bool
	nonlocals map[string]bool // NEW: nonlocal declarations
	// assignOrder tracks the order in which names are first assigned,
	// ensuring deterministic iteration for slot allocation.
	assignOrder []string
	// globalOrder tracks the order in which names are first declared global,
	// ensuring deterministic iteration for global slot allocation.
	globalOrder []string
	// nonlocalOrder tracks the order in which names are first declared nonlocal.
	nonlocalOrder []string
}

// recordAssign records a name as assigned, maintaining first-seen order.
func (nc *nameCollector) recordAssign(name string) {
	if !nc.assigned[name] {
		nc.assigned[name] = true
		nc.assignOrder = append(nc.assignOrder, name)
	}
}

// collectStmt processes a statement for name collection.
func (nc *nameCollector) collectStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.Assign:
		for _, target := range s.Targets {
			nc.collectTarget(target)
		}

	case *ast.AugAssign:
		nc.collectTarget(s.Target)

	case *ast.AnnAssign:
		if s.Value != nil {
			nc.collectTarget(s.Target)
		}

	case *ast.Global:
		for _, name := range s.Names {
			if !nc.globals[name] {
				nc.globals[name] = true
				nc.globalOrder = append(nc.globalOrder, name)
			}
		}

	case *ast.Nonlocal:
		if nc.nonlocals != nil {
			for _, name := range s.Names {
				if !nc.nonlocals[name] {
					nc.nonlocals[name] = true
					nc.nonlocalOrder = append(nc.nonlocalOrder, name)
				}
			}
		}

	case *ast.FunctionDef:
		// The function name itself is assigned in the enclosing scope.
		nc.recordAssign(s.Name)
		// Do NOT recurse into the function body — it's a separate scope.

	case *ast.ClassDef:
		// The class name is assigned in the enclosing scope.
		nc.recordAssign(s.Name)
		// Do NOT recurse into the class body — it's a separate scope.

	case *ast.For:
		nc.collectTarget(s.Target)
		for _, bodyStmt := range s.Body {
			nc.collectStmt(bodyStmt)
		}
		for _, elseStmt := range s.Orelse {
			nc.collectStmt(elseStmt)
		}

	case *ast.While:
		for _, bodyStmt := range s.Body {
			nc.collectStmt(bodyStmt)
		}
		for _, elseStmt := range s.Orelse {
			nc.collectStmt(elseStmt)
		}

	case *ast.If:
		for _, bodyStmt := range s.Body {
			nc.collectStmt(bodyStmt)
		}
		for _, elseStmt := range s.Orelse {
			nc.collectStmt(elseStmt)
		}

	case *ast.Try:
		for _, bodyStmt := range s.Body {
			nc.collectStmt(bodyStmt)
		}
		for _, handler := range s.Handlers {
			if handler.Name != "" {
				nc.recordAssign(handler.Name)
			}
			for _, handlerStmt := range handler.Body {
				nc.collectStmt(handlerStmt)
			}
		}
		for _, elseStmt := range s.Orelse {
			nc.collectStmt(elseStmt)
		}
		for _, finallyStmt := range s.Finalbody {
			nc.collectStmt(finallyStmt)
		}

	case *ast.With:
		for _, item := range s.Items {
			if item.OptionalVar != nil {
				nc.collectTarget(item.OptionalVar)
			}
		}
		for _, bodyStmt := range s.Body {
			nc.collectStmt(bodyStmt)
		}

	case *ast.Import:
		for _, alias := range s.Names {
			name := alias.AsName
			if name == "" {
				name = alias.Name
			}
			nc.recordAssign(name)
		}

	case *ast.ImportFrom:
		for _, alias := range s.Names {
			name := alias.AsName
			if name == "" {
				name = alias.Name
			}
			nc.recordAssign(name)
		}

	case *ast.Delete:
		// del doesn't create a binding — skip.

	case *ast.ExprStmt, *ast.Return, *ast.Raise, *ast.Pass, *ast.Break, *ast.Continue, *ast.Assert:
		// These don't create bindings.
	}
}

// collectTarget extracts assigned names from an assignment target expression.
func (nc *nameCollector) collectTarget(expr ast.Expr) {
	switch t := expr.(type) {
	case *ast.Name:
		nc.recordAssign(t.Id)
	case *ast.Tuple:
		for _, elt := range t.Elts {
			nc.collectTarget(elt)
		}
	case *ast.List:
		for _, elt := range t.Elts {
			nc.collectTarget(elt)
		}
	case *ast.Starred:
		nc.collectTarget(t.Value)
	case *ast.Attribute, *ast.Subscript:
		// Attribute/subscript assignments don't bind a name in the local scope.
	}
}
