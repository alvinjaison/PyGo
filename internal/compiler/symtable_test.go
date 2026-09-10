package compiler

import (
	"testing"

	"github.com/alvinjaison/pygo/internal/ast"
)

func TestAnalyzeModule_SimpleAssignment(t *testing.T) {
	// Simulates: x = 10
	module := ast.NewModule([]ast.Stmt{
		makeAssign("x"),
	}, ast.NewPos("test.py", 1, 1, 1, 6))

	st := analyzeModule(module)

	info := st.lookupName("x")
	if info == nil {
		t.Fatal("expected 'x' to be in symbol table")
	}
	if info.scope != scopeGlobal {
		t.Errorf("expected 'x' to be global at module level, got %d", info.scope)
	}
	if len(st.names) != 1 || st.names[0] != "x" {
		t.Errorf("expected Names=['x'], got %v", st.names)
	}
}

func TestAnalyzeModule_MultipleAssignments(t *testing.T) {
	// Simulates: x = 10; y = 20
	module := ast.NewModule([]ast.Stmt{
		makeAssign("x"),
		makeAssign("y"),
	}, ast.NewPos("test.py", 1, 1, 2, 6))

	st := analyzeModule(module)

	if len(st.names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(st.names))
	}
	for _, name := range []string{"x", "y"} {
		info := st.lookupName(name)
		if info == nil {
			t.Errorf("expected '%s' to be in symbol table", name)
		} else if info.scope != scopeGlobal {
			t.Errorf("expected '%s' to be global, got %d", name, info.scope)
		}
	}
}

func TestAnalyzeModule_FunctionDef(t *testing.T) {
	// Simulates: def foo(): pass
	// The function name 'foo' should be a global in the module scope.
	module := ast.NewModule([]ast.Stmt{
		makeFuncDef("foo", nil, nil),
	}, ast.NewPos("test.py", 1, 1, 1, 1))

	st := analyzeModule(module)

	info := st.lookupName("foo")
	if info == nil {
		t.Fatal("expected 'foo' to be in symbol table")
	}
	if info.scope != scopeGlobal {
		t.Errorf("expected 'foo' to be global at module level, got %d", info.scope)
	}
}

func TestAnalyzeFunction_LocalVariables(t *testing.T) {
	// Simulates: def foo(a, b): x = a + b
	funcDef := makeFuncDef("foo", []string{"a", "b"}, []ast.Stmt{
		makeAssign("x"),
	})

	parentST := newSymbolTable()
	st := analyzeFunction(funcDef, parentST)

	// Parameters should be local, in order.
	if len(st.varnames) != 3 {
		t.Fatalf("expected 3 varnames, got %d: %v", len(st.varnames), st.varnames)
	}
	if st.varnames[0] != "a" || st.varnames[1] != "b" || st.varnames[2] != "x" {
		t.Errorf("expected varnames=['a','b','x'], got %v", st.varnames)
	}

	// Check slots.
	for i, name := range []string{"a", "b", "x"} {
		info := st.lookupName(name)
		if info == nil {
			t.Fatalf("expected '%s' in symbol table", name)
		}
		if info.scope != scopeLocal {
			t.Errorf("expected '%s' to be local, got %d", name, info.scope)
		}
		if info.slot != i {
			t.Errorf("expected '%s' slot=%d, got %d", name, i, info.slot)
		}
	}
}

func TestAnalyzeFunction_GlobalDeclaration(t *testing.T) {
	// Simulates: def foo(): global x; x = 10
	funcDef := makeFuncDef("foo", nil, []ast.Stmt{
		&ast.Global{Names: []string{"x"}},
		makeAssign("x"),
	})

	parentST := newSymbolTable()
	st := analyzeFunction(funcDef, parentST)

	// x should be global, not local.
	info := st.lookupName("x")
	if info == nil {
		t.Fatal("expected 'x' in symbol table")
	}
	if info.scope != scopeGlobal {
		t.Errorf("expected 'x' to be global, got %d", info.scope)
	}

	// x should NOT be in varnames.
	for _, vn := range st.varnames {
		if vn == "x" {
			t.Error("'x' should not be in varnames when declared global")
		}
	}
}

func TestAnalyzeFunction_NestedFunctionScope(t *testing.T) {
	// Simulates:
	// def outer():
	//     x = 1
	//     def inner():
	//         y = 2
	//
	// inner's name 'inner' should be a local in outer.
	// inner should NOT cause 'y' to appear in outer's scope.
	innerFunc := makeFuncDef("inner", nil, []ast.Stmt{
		makeAssign("y"),
	})
	outerFunc := makeFuncDef("outer", nil, []ast.Stmt{
		makeAssign("x"),
		innerFunc,
	})

	parentST := newSymbolTable()
	st := analyzeFunction(outerFunc, parentST)

	// outer should have locals: x, inner
	if st.localSlot("x") == -1 {
		t.Error("expected 'x' to be local in outer")
	}
	if st.localSlot("inner") == -1 {
		t.Error("expected 'inner' to be local in outer")
	}
	// y should NOT be in outer's scope.
	if st.lookupName("y") != nil {
		t.Error("'y' from inner should not appear in outer's scope")
	}
}

func TestAnalyzeFunction_ForLoopTarget(t *testing.T) {
	// Simulates: def foo(): for i in range(10): pass
	funcDef := makeFuncDef("foo", nil, []ast.Stmt{
		&ast.For{
			Target: &ast.Name{Id: "i"},
			Iter:   &ast.Name{Id: "range"},
			Body:   []ast.Stmt{&ast.Pass{}},
		},
	})

	parentST := newSymbolTable()
	st := analyzeFunction(funcDef, parentST)

	info := st.lookupName("i")
	if info == nil {
		t.Fatal("expected 'i' in symbol table")
	}
	if info.scope != scopeLocal {
		t.Errorf("expected 'i' to be local, got %d", info.scope)
	}
}

func TestAnalyzeFunction_TupleUnpacking(t *testing.T) {
	// Simulates: def foo(): a, b = 1, 2
	funcDef := makeFuncDef("foo", nil, []ast.Stmt{
		&ast.Assign{
			Targets: []ast.Expr{
				&ast.Tuple{
					Elts: []ast.Expr{
						&ast.Name{Id: "a"},
						&ast.Name{Id: "b"},
					},
				},
			},
			Value: &ast.Constant{Value: int64(1)},
		},
	})

	parentST := newSymbolTable()
	st := analyzeFunction(funcDef, parentST)

	if st.localSlot("a") == -1 {
		t.Error("expected 'a' to be local")
	}
	if st.localSlot("b") == -1 {
		t.Error("expected 'b' to be local")
	}
}

func TestCompile_ReturnsValidCode(t *testing.T) {
	// Simulates: x = 10
	module := ast.NewModule([]ast.Stmt{
		makeAssign("x"),
	}, ast.NewPos("test.py", 1, 1, 1, 6))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code == nil {
		t.Fatal("expected non-nil Code")
	}
	if code.Name != "<module>" {
		t.Errorf("expected name '<module>', got %q", code.Name)
	}
	if code.Filename != "test.py" {
		t.Errorf("expected filename 'test.py', got %q", code.Filename)
	}
	// Should have at least a LOAD_CONST (None) and RETURN_VALUE.
	if len(code.Instructions) < 2 {
		t.Errorf("expected at least 2 instructions, got %d", len(code.Instructions))
	}
	// x should be in Names.
	found := false
	for _, n := range code.Names {
		if n == "x" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'x' in Names, got %v", code.Names)
	}
}

// Helper functions to construct AST nodes for testing.

func makeAssign(name string) *ast.Assign {
	return &ast.Assign{
		Targets: []ast.Expr{&ast.Name{Id: name, Ctx: ast.Store}},
		Value:   &ast.Constant{Value: int64(10)},
	}
}

func makeFuncDef(name string, params []string, body []ast.Stmt) *ast.FunctionDef {
	var args *ast.Arguments
	if params != nil {
		argList := make([]*ast.Arg, len(params))
		for i, p := range params {
			argList[i] = &ast.Arg{Arg: p}
		}
		args = &ast.Arguments{Args: argList}
	}
	if body == nil {
		body = []ast.Stmt{&ast.Pass{}}
	}
	return &ast.FunctionDef{
		Name: name,
		Args: args,
		Body: body,
	}
}
