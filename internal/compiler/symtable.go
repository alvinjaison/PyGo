package compiler

import "github.com/pygo/pygo/internal/ast"

// symbolKind classifies how a name is bound in a scope.
type symbolKind int

const (
	symFree    symbolKind = iota // referenced, bound in an enclosing function scope
	symLocal                     // assigned in this scope
	symGlobal                    // explicitly global, or module-level
	symCell                      // local here and captured by a nested function
)

// symtable holds name classification for a single scope.
type symtable struct {
	kinds       map[string]symbolKind
	assigned    map[string]bool // names assigned in this scope
	declGlobal  map[string]bool
	declNonlocal map[string]bool
	referenced  map[string]bool
	// names captured by nested functions (become cells here)
	capturedByChild map[string]bool
}

func newSymtable() *symtable {
	return &symtable{
		kinds:           map[string]symbolKind{},
		assigned:        map[string]bool{},
		declGlobal:      map[string]bool{},
		declNonlocal:    map[string]bool{},
		referenced:      map[string]bool{},
		capturedByChild: map[string]bool{},
	}
}

// buildSymtable builds a module-level symbol table.
// At module scope, all assigned names are globals.
func buildSymtable(mod *ast.Module) *symtable {
	st := newSymtable()
	for _, s := range mod.Body {
		collectAssigned(s, st)
	}
	for name := range st.assigned {
		st.kinds[name] = symGlobal
	}
	return st
}

// buildFunctionSymtable builds a symbol table for a function scope.
// enclosing lists the set of names bound in enclosing *function* scopes
// (used to determine free variables).
func buildFunctionSymtable(args *ast.Arguments, body []ast.Stmt, enclosing map[string]bool) *symtable {
	st := newSymtable()

	// Parameters are locals.
	if args != nil {
		for _, a := range args.Args {
			st.assigned[a.Arg] = true
		}
		for _, a := range args.KwonlyArgs {
			st.assigned[a.Arg] = true
		}
		if args.Vararg != nil {
			st.assigned[args.Vararg.Arg] = true
		}
		if args.Kwarg != nil {
			st.assigned[args.Kwarg.Arg] = true
		}
	}

	for _, s := range body {
		collectAssigned(s, st)
		collectReferenced(s, st)
		collectDeclarations(s, st)
	}

	// Classify each name.
	for name := range st.assigned {
		switch {
		case st.declGlobal[name]:
			st.kinds[name] = symGlobal
		case st.declNonlocal[name]:
			st.kinds[name] = symFree
		default:
			st.kinds[name] = symLocal
		}
	}

	// Referenced-but-not-assigned names: free if bound in an enclosing
	// function scope, otherwise global.
	for name := range st.referenced {
		if _, ok := st.kinds[name]; ok {
			continue
		}
		if st.declGlobal[name] {
			st.kinds[name] = symGlobal
		} else if enclosing[name] {
			st.kinds[name] = symFree
		} else {
			st.kinds[name] = symGlobal
		}
	}

	return st
}

func (st *symtable) kindOf(name string) (symbolKind, bool) {
	k, ok := st.kinds[name]
	return k, ok
}

// collectAssigned records names that are assignment targets in a statement.
func collectAssigned(s ast.Stmt, st *symtable) {
	switch n := s.(type) {
	case *ast.Assign:
		for _, t := range n.Targets {
			collectTargets(t, st)
		}
	case *ast.AugAssign:
		collectTargets(n.Target, st)
	case *ast.AnnAssign:
		if n.Value != nil {
			collectTargets(n.Target, st)
		}
	case *ast.For:
		collectTargets(n.Target, st)
		collectAssignedBlock(n.Body, st)
		collectAssignedBlock(n.Orelse, st)
	case *ast.While:
		collectAssignedBlock(n.Body, st)
		collectAssignedBlock(n.Orelse, st)
	case *ast.If:
		collectAssignedBlock(n.Body, st)
		collectAssignedBlock(n.Orelse, st)
	case *ast.With:
		for _, item := range n.Items {
			if item.OptionalVar != nil {
				collectTargets(item.OptionalVar, st)
			}
		}
		collectAssignedBlock(n.Body, st)
	case *ast.Try:
		collectAssignedBlock(n.Body, st)
		for _, h := range n.Handlers {
			if h.Name != "" {
				st.assigned[h.Name] = true
			}
			collectAssignedBlock(h.Body, st)
		}
		collectAssignedBlock(n.Orelse, st)
		collectAssignedBlock(n.Finalbody, st)
	case *ast.FunctionDef:
		st.assigned[n.Name] = true
	case *ast.ClassDef:
		st.assigned[n.Name] = true
	case *ast.Import:
		for _, a := range n.Names {
			name := a.AsName
			if name == "" {
				name = topName(a.Name)
			}
			st.assigned[name] = true
		}
	case *ast.ImportFrom:
		for _, a := range n.Names {
			name := a.AsName
			if name == "" {
				name = a.Name
			}
			if name != "*" {
				st.assigned[name] = true
			}
		}
	}
}

func collectAssignedBlock(block []ast.Stmt, st *symtable) {
	for _, s := range block {
		collectAssigned(s, st)
	}
}

func collectTargets(e ast.Expr, st *symtable) {
	switch n := e.(type) {
	case *ast.Name:
		st.assigned[n.Id] = true
	case *ast.Tuple:
		for _, elt := range n.Elts {
			collectTargets(elt, st)
		}
	case *ast.List:
		for _, elt := range n.Elts {
			collectTargets(elt, st)
		}
	case *ast.Starred:
		collectTargets(n.Value, st)
	}
}

// collectReferenced records all Name loads in a statement subtree.
func collectReferenced(s ast.Stmt, st *symtable) {
	ast.Walk(refVisitor{st}, s)
}

type refVisitor struct{ st *symtable }

func (v refVisitor) Visit(node ast.Node) ast.Visitor {
	// Do not descend into nested function/class scopes for referenced names;
	// their free variables are handled when those scopes are compiled. But we
	// DO record names they reference as captured, below.
	switch n := node.(type) {
	case *ast.Name:
		if n.Ctx == ast.Load {
			v.st.referenced[n.Id] = true
		}
	case *ast.FunctionDef:
		// Record free names of the nested function as captured by this scope.
		markCaptured(n.Args, n.Body, v.st)
		return nil
	case *ast.Lambda:
		return nil
	}
	return v
}

// markCaptured scans a nested function body for names it references that are
// not bound locally within it, recording them as captured by the parent.
func markCaptured(args *ast.Arguments, body []ast.Stmt, parent *symtable) {
	inner := newSymtable()
	if args != nil {
		for _, a := range args.Args {
			inner.assigned[a.Arg] = true
		}
		if args.Vararg != nil {
			inner.assigned[args.Vararg.Arg] = true
		}
		if args.Kwarg != nil {
			inner.assigned[args.Kwarg.Arg] = true
		}
	}
	for _, s := range body {
		collectAssigned(s, inner)
	}
	for _, s := range body {
		ast.Walk(capVisitor{inner: inner, parent: parent}, s)
	}
}

type capVisitor struct {
	inner  *symtable
	parent *symtable
}

func (v capVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.Name:
		if n.Ctx == ast.Load && !v.inner.assigned[n.Id] {
			// Referenced in nested function but not bound there: if bound in
			// the parent, the parent must make it a cell.
			if v.parent.assigned[n.Id] {
				v.parent.capturedByChild[n.Id] = true
			}
		}
	}
	return v
}

func collectDeclarations(s ast.Stmt, st *symtable) {
	switch n := s.(type) {
	case *ast.Global:
		for _, name := range n.Names {
			st.declGlobal[name] = true
		}
	case *ast.Nonlocal:
		for _, name := range n.Names {
			st.declNonlocal[name] = true
		}
	case *ast.If:
		collectDeclBlock(n.Body, st)
		collectDeclBlock(n.Orelse, st)
	case *ast.For:
		collectDeclBlock(n.Body, st)
		collectDeclBlock(n.Orelse, st)
	case *ast.While:
		collectDeclBlock(n.Body, st)
		collectDeclBlock(n.Orelse, st)
	case *ast.Try:
		collectDeclBlock(n.Body, st)
		for _, h := range n.Handlers {
			collectDeclBlock(h.Body, st)
		}
		collectDeclBlock(n.Orelse, st)
		collectDeclBlock(n.Finalbody, st)
	case *ast.With:
		collectDeclBlock(n.Body, st)
	}
}

func collectDeclBlock(block []ast.Stmt, st *symtable) {
	for _, s := range block {
		collectDeclarations(s, st)
	}
}

func topName(dotted string) string {
	for i := 0; i < len(dotted); i++ {
		if dotted[i] == '.' {
			return dotted[:i]
		}
	}
	return dotted
}
