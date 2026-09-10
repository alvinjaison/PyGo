package compiler

import (
	"testing"

	"github.com/alvinjaison/pygo/internal/ast"
	"github.com/alvinjaison/pygo/internal/bytecode"
)

// TestCompile_FunctionDef verifies that compiling a function definition
// produces a nested Code object and emits MAKE_FUNCTION.
func TestCompile_FunctionDef(t *testing.T) {
	// Simulates:
	// def add(a, b):
	//     return a + b
	module := ast.NewModule([]ast.Stmt{
		&ast.FunctionDef{
			Name: "add",
			Args: &ast.Arguments{
				Args: []*ast.Arg{
					{Arg: "a"},
					{Arg: "b"},
				},
			},
			Body: []ast.Stmt{
				&ast.Return{
					Value: &ast.BinOp{
						Left:  &ast.Name{Id: "a"},
						Op:    ast.Add,
						Right: &ast.Name{Id: "b"},
					},
				},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 3, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Check that a nested Code object exists in the constants.
	var nestedCode *bytecode.Code
	for _, c := range code.Constants {
		if nc, ok := c.(*bytecode.Code); ok {
			nestedCode = nc
			break
		}
	}
	if nestedCode == nil {
		t.Fatal("expected nested Code object in constants for function definition")
	}
	if nestedCode.Name != "add" {
		t.Errorf("expected nested code name 'add', got %q", nestedCode.Name)
	}
	if nestedCode.ArgCount != 2 {
		t.Errorf("expected ArgCount=2, got %d", nestedCode.ArgCount)
	}

	// Check that the nested code has FlagOptimized | FlagNewLocals.
	if !nestedCode.HasFlag(bytecode.FlagOptimized) {
		t.Error("expected FlagOptimized on function code")
	}
	if !nestedCode.HasFlag(bytecode.FlagNewLocals) {
		t.Error("expected FlagNewLocals on function code")
	}

	// Check that MAKE_FUNCTION is emitted in the module code.
	found := false
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.MAKE_FUNCTION {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected MAKE_FUNCTION instruction in module code")
	}

	// Check that the function name "add" is stored.
	foundStore := false
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.STORE_NAME {
			foundStore = true
			break
		}
	}
	if !foundStore {
		t.Error("expected STORE_NAME for function name 'add'")
	}
}

// TestCompile_FunctionCall verifies CALL_FUNCTION emission.
func TestCompile_FunctionCall(t *testing.T) {
	// Simulates: add(x, y)
	module := ast.NewModule([]ast.Stmt{
		&ast.ExprStmt{
			Value: &ast.Call{
				Func: &ast.Name{Id: "add"},
				Args: []ast.Expr{
					&ast.Name{Id: "x"},
					&ast.Name{Id: "y"},
				},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 1, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Find CALL_FUNCTION with arg=2.
	found := false
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.CALL_FUNCTION && instr.Arg == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected CALL_FUNCTION with arg=2")
	}
}

// TestCompile_ReturnValue verifies RETURN_VALUE with a value.
func TestCompile_ReturnValue(t *testing.T) {
	// Simulates:
	// def foo():
	//     return 42
	module := ast.NewModule([]ast.Stmt{
		&ast.FunctionDef{
			Name: "foo",
			Args: &ast.Arguments{},
			Body: []ast.Stmt{
				&ast.Return{
					Value: &ast.Constant{Value: int64(42)},
				},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 2, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Get the nested function code.
	var nestedCode *bytecode.Code
	for _, c := range code.Constants {
		if nc, ok := c.(*bytecode.Code); ok {
			nestedCode = nc
			break
		}
	}
	if nestedCode == nil {
		t.Fatal("expected nested Code object")
	}

	// Should have LOAD_CONST(42) followed by RETURN_VALUE.
	foundReturn := false
	for i, instr := range nestedCode.Instructions {
		if instr.Op == bytecode.RETURN_VALUE && i > 0 {
			prev := nestedCode.Instructions[i-1]
			if prev.Op == bytecode.LOAD_CONST {
				// Verify the constant is 42.
				constIdx := prev.Arg
				if int(constIdx) < len(nestedCode.Constants) {
					if val, ok := nestedCode.Constants[constIdx].(int64); ok && val == 42 {
						foundReturn = true
					}
				}
			}
		}
	}
	if !foundReturn {
		t.Error("expected LOAD_CONST(42) + RETURN_VALUE in function code")
	}
}

// TestCompile_ImplicitReturn verifies implicit None return at end of function.
func TestCompile_ImplicitReturn(t *testing.T) {
	// Simulates:
	// def foo():
	//     pass
	module := ast.NewModule([]ast.Stmt{
		&ast.FunctionDef{
			Name: "foo",
			Args: &ast.Arguments{},
			Body: []ast.Stmt{
				&ast.Pass{},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 2, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Get the nested function code.
	var nestedCode *bytecode.Code
	for _, c := range code.Constants {
		if nc, ok := c.(*bytecode.Code); ok {
			nestedCode = nc
			break
		}
	}
	if nestedCode == nil {
		t.Fatal("expected nested Code object")
	}

	// The last two instructions should be LOAD_CONST(None) + RETURN_VALUE.
	instrs := nestedCode.Instructions
	if len(instrs) < 2 {
		t.Fatalf("expected at least 2 instructions, got %d", len(instrs))
	}
	last := instrs[len(instrs)-1]
	secondLast := instrs[len(instrs)-2]
	if last.Op != bytecode.RETURN_VALUE {
		t.Errorf("expected last instruction to be RETURN_VALUE, got %s", last.Op)
	}
	if secondLast.Op != bytecode.LOAD_CONST {
		t.Errorf("expected second-to-last instruction to be LOAD_CONST, got %s", secondLast.Op)
	}
	// Verify the constant is nil (None).
	constIdx := secondLast.Arg
	if int(constIdx) < len(nestedCode.Constants) {
		if nestedCode.Constants[constIdx] != nil {
			t.Errorf("expected None constant for implicit return, got %v", nestedCode.Constants[constIdx])
		}
	}
}

// TestCompile_Raise verifies RAISE_VARARGS emission.
func TestCompile_Raise(t *testing.T) {
	// Simulates: raise ValueError("msg")
	module := ast.NewModule([]ast.Stmt{
		&ast.Raise{
			Exc: &ast.Call{
				Func: &ast.Name{Id: "ValueError"},
				Args: []ast.Expr{
					&ast.Constant{Value: "msg"},
				},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 1, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Should have RAISE_VARARGS with arg=1.
	found := false
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.RAISE_VARARGS && instr.Arg == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected RAISE_VARARGS with arg=1")
	}
}

// TestCompile_BareRaise verifies bare raise (re-raise) emission.
func TestCompile_BareRaise(t *testing.T) {
	// Simulates: raise (bare raise within except handler)
	module := ast.NewModule([]ast.Stmt{
		&ast.Raise{},
	}, ast.NewPos("test.py", 1, 1, 1, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Should have RAISE_VARARGS with arg=0 (re-raise).
	found := false
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.RAISE_VARARGS && instr.Arg == 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected RAISE_VARARGS with arg=0 for bare raise")
	}
}

// TestCompile_TryExcept verifies try/except compilation.
func TestCompile_TryExcept(t *testing.T) {
	// Simulates:
	// try:
	//     x = 1
	// except:
	//     x = 2
	module := ast.NewModule([]ast.Stmt{
		&ast.Try{
			Body: []ast.Stmt{
				makeAssign("x"),
			},
			Handlers: []*ast.ExceptHandler{
				{
					Body: []ast.Stmt{
						&ast.Assign{
							Targets: []ast.Expr{&ast.Name{Id: "x", Ctx: ast.Store}},
							Value:   &ast.Constant{Value: int64(2)},
						},
					},
				},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 4, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Should have SETUP_FINALLY, POP_BLOCK, and POP_EXCEPT.
	hasSetup := false
	hasPopBlock := false
	hasPopExcept := false
	for _, instr := range code.Instructions {
		switch instr.Op {
		case bytecode.SETUP_FINALLY:
			hasSetup = true
		case bytecode.POP_BLOCK:
			hasPopBlock = true
		case bytecode.POP_EXCEPT:
			hasPopExcept = true
		}
	}
	if !hasSetup {
		t.Error("expected SETUP_FINALLY instruction")
	}
	if !hasPopBlock {
		t.Error("expected POP_BLOCK instruction")
	}
	if !hasPopExcept {
		t.Error("expected POP_EXCEPT instruction")
	}
}

// TestCompile_TryExceptAs verifies except-as binding.
func TestCompile_TryExceptAs(t *testing.T) {
	// Simulates:
	// try:
	//     x = 1
	// except ValueError as e:
	//     pass
	module := ast.NewModule([]ast.Stmt{
		&ast.Try{
			Body: []ast.Stmt{
				makeAssign("x"),
			},
			Handlers: []*ast.ExceptHandler{
				{
					Type: &ast.Name{Id: "ValueError"},
					Name: "e",
					Body: []ast.Stmt{&ast.Pass{}},
				},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 4, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Check that 'e' is in the Names (since it's stored at module level).
	foundE := false
	for _, n := range code.Names {
		if n == "e" {
			foundE = true
			break
		}
	}
	if !foundE {
		t.Error("expected 'e' in Names for except-as binding")
	}

	// Should have a STORE_NAME for 'e'.
	hasStoreE := false
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.STORE_NAME {
			if int(instr.Arg) < len(code.Names) && code.Names[instr.Arg] == "e" {
				hasStoreE = true
				break
			}
		}
	}
	if !hasStoreE {
		t.Error("expected STORE_NAME for 'e' (except-as binding)")
	}
}

// TestCompile_TryFinally verifies try/finally compilation.
func TestCompile_TryFinally(t *testing.T) {
	// Simulates:
	// try:
	//     x = 1
	// finally:
	//     y = 2
	module := ast.NewModule([]ast.Stmt{
		&ast.Try{
			Body: []ast.Stmt{
				makeAssign("x"),
			},
			Finalbody: []ast.Stmt{
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "y", Ctx: ast.Store}},
					Value:   &ast.Constant{Value: int64(2)},
				},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 4, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Should have SETUP_FINALLY and POP_BLOCK.
	hasSetup := false
	hasPopBlock := false
	for _, instr := range code.Instructions {
		switch instr.Op {
		case bytecode.SETUP_FINALLY:
			hasSetup = true
		case bytecode.POP_BLOCK:
			hasPopBlock = true
		}
	}
	if !hasSetup {
		t.Error("expected SETUP_FINALLY for finally block")
	}
	if !hasPopBlock {
		t.Error("expected POP_BLOCK for finally block")
	}

	// 'y' should be in Names.
	foundY := false
	for _, n := range code.Names {
		if n == "y" {
			foundY = true
			break
		}
	}
	if !foundY {
		t.Error("expected 'y' in Names for finally body")
	}
}

// TestCompile_UnpackSequence verifies UNPACK_SEQUENCE emission for tuple unpacking.
func TestCompile_UnpackSequence(t *testing.T) {
	// Simulates: a, b = 1, 2
	module := ast.NewModule([]ast.Stmt{
		&ast.Assign{
			Targets: []ast.Expr{
				&ast.Tuple{
					Elts: []ast.Expr{
						&ast.Name{Id: "a", Ctx: ast.Store},
						&ast.Name{Id: "b", Ctx: ast.Store},
					},
				},
			},
			Value: &ast.Tuple{
				Elts: []ast.Expr{
					&ast.Constant{Value: int64(1)},
					&ast.Constant{Value: int64(2)},
				},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 1, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Should have UNPACK_SEQUENCE with arg=2.
	found := false
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.UNPACK_SEQUENCE && instr.Arg == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected UNPACK_SEQUENCE with arg=2")
	}
}

// TestCompile_UnsupportedStmt verifies error reporting for unsupported AST nodes.
func TestCompile_UnsupportedStmt(t *testing.T) {
	// Use a ClassDef since it's not supported in the current compiler.
	module := ast.NewModule([]ast.Stmt{
		&ast.ClassDef{
			Name: "Foo",
			Body: []ast.Stmt{&ast.Pass{}},
		},
	}, ast.NewPos("test.py", 1, 1, 2, 1))

	_, err := Compile(module)
	if err == nil {
		t.Fatal("expected error for unsupported statement type")
	}
	// Error should mention "unsupported".
	if !containsString(err.Error(), "unsupported") {
		t.Errorf("expected error to mention 'unsupported', got: %s", err.Error())
	}
}

// TestCompile_FunctionWithNestedCode verifies that the function code object
// uses LOAD_FAST for local parameter access.
func TestCompile_FunctionWithNestedCode(t *testing.T) {
	// Simulates:
	// def foo(x):
	//     return x
	module := ast.NewModule([]ast.Stmt{
		&ast.FunctionDef{
			Name: "foo",
			Args: &ast.Arguments{
				Args: []*ast.Arg{{Arg: "x"}},
			},
			Body: []ast.Stmt{
				&ast.Return{
					Value: &ast.Name{Id: "x"},
				},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 2, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Get the nested function code.
	var nestedCode *bytecode.Code
	for _, c := range code.Constants {
		if nc, ok := c.(*bytecode.Code); ok {
			nestedCode = nc
			break
		}
	}
	if nestedCode == nil {
		t.Fatal("expected nested Code object")
	}

	// The function should use LOAD_FAST for parameter 'x'.
	foundLoadFast := false
	for _, instr := range nestedCode.Instructions {
		if instr.Op == bytecode.LOAD_FAST && instr.Arg == 0 {
			foundLoadFast = true
			break
		}
	}
	if !foundLoadFast {
		t.Error("expected LOAD_FAST(0) for parameter 'x' in function code")
	}

	// Varnames should contain 'x'.
	if len(nestedCode.Varnames) == 0 || nestedCode.Varnames[0] != "x" {
		t.Errorf("expected Varnames=['x'], got %v", nestedCode.Varnames)
	}
}

// TestCompile_TryExceptFinally verifies combined try/except/finally.
func TestCompile_TryExceptFinally(t *testing.T) {
	// Simulates:
	// try:
	//     x = 1
	// except:
	//     x = 2
	// finally:
	//     y = 3
	module := ast.NewModule([]ast.Stmt{
		&ast.Try{
			Body: []ast.Stmt{
				makeAssign("x"),
			},
			Handlers: []*ast.ExceptHandler{
				{
					Body: []ast.Stmt{
						&ast.Assign{
							Targets: []ast.Expr{&ast.Name{Id: "x", Ctx: ast.Store}},
							Value:   &ast.Constant{Value: int64(2)},
						},
					},
				},
			},
			Finalbody: []ast.Stmt{
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "y", Ctx: ast.Store}},
					Value:   &ast.Constant{Value: int64(3)},
				},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 6, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Should have two SETUP_FINALLY (one for finally, one for except).
	setupCount := 0
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.SETUP_FINALLY {
			setupCount++
		}
	}
	if setupCount != 2 {
		t.Errorf("expected 2 SETUP_FINALLY instructions, got %d", setupCount)
	}
}

// TestCompile_FunctionCallNoArgs verifies CALL_FUNCTION with 0 args.
func TestCompile_FunctionCallNoArgs(t *testing.T) {
	// Simulates: foo()
	module := ast.NewModule([]ast.Stmt{
		&ast.ExprStmt{
			Value: &ast.Call{
				Func: &ast.Name{Id: "foo"},
				Args: []ast.Expr{},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 1, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Find CALL_FUNCTION with arg=0.
	found := false
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.CALL_FUNCTION && instr.Arg == 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected CALL_FUNCTION with arg=0")
	}
}

// containsString checks if a string contains a substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestCompile_SwapAtomicity_BuildTupleBeforeUnpack verifies that `a, b = b, a`
// emits BUILD_TUPLE before UNPACK_SEQUENCE, ensuring the RHS is fully evaluated
// before any stores happen (Requirement 10.9).
func TestCompile_SwapAtomicity_BuildTupleBeforeUnpack(t *testing.T) {
	// Simulates: a, b = b, a
	module := ast.NewModule([]ast.Stmt{
		&ast.Assign{
			Targets: []ast.Expr{
				&ast.Tuple{
					Elts: []ast.Expr{
						&ast.Name{Id: "a", Ctx: ast.Store},
						&ast.Name{Id: "b", Ctx: ast.Store},
					},
				},
			},
			Value: &ast.Tuple{
				Elts: []ast.Expr{
					&ast.Name{Id: "b"},
					&ast.Name{Id: "a"},
				},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 1, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Verify instruction ordering: BUILD_TUPLE(2) must appear before UNPACK_SEQUENCE(2).
	buildIdx := -1
	unpackIdx := -1
	for i, instr := range code.Instructions {
		if instr.Op == bytecode.BUILD_TUPLE && instr.Arg == 2 && buildIdx == -1 {
			buildIdx = i
		}
		if instr.Op == bytecode.UNPACK_SEQUENCE && instr.Arg == 2 && unpackIdx == -1 {
			unpackIdx = i
		}
	}

	if buildIdx == -1 {
		t.Fatal("expected BUILD_TUPLE(2) instruction for RHS evaluation")
	}
	if unpackIdx == -1 {
		t.Fatal("expected UNPACK_SEQUENCE(2) instruction for LHS unpacking")
	}
	if buildIdx >= unpackIdx {
		t.Errorf("BUILD_TUPLE (index %d) must come before UNPACK_SEQUENCE (index %d) for swap atomicity",
			buildIdx, unpackIdx)
	}
}
