package compiler

import (
	"testing"

	"github.com/alvinjaison/pygo/internal/ast"
	"github.com/alvinjaison/pygo/internal/bytecode"
)

// =============================================================================
// Constant Pool Deduplication Tests
// =============================================================================

// TestConstantPoolDeduplication_SameInt verifies that repeated integer constants
// are stored only once in the constant pool.
func TestConstantPoolDeduplication_SameInt(t *testing.T) {
	// x = 42
	// y = 42
	pos := ast.NewPos("test.py", 1, 1, 2, 5)
	module := ast.NewModule([]ast.Stmt{
		&ast.Assign{
			Targets: []ast.Expr{&ast.Name{Id: "x", Ctx: ast.Store}},
			Value:   &ast.Constant{Value: int64(42)},
		},
		&ast.Assign{
			Targets: []ast.Expr{&ast.Name{Id: "y", Ctx: ast.Store}},
			Value:   &ast.Constant{Value: int64(42)},
		},
	}, pos)

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count how many times 42 appears in the constant pool.
	count := 0
	for _, c := range code.Constants {
		if v, ok := c.(int64); ok && v == 42 {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected constant 42 to appear once in pool, got %d", count)
	}
}

// TestConstantPoolDeduplication_SameString verifies that repeated string
// constants share a single pool entry.
func TestConstantPoolDeduplication_SameString(t *testing.T) {
	// a = "hello"
	// b = "hello"
	pos := ast.NewPos("test.py", 1, 1, 2, 5)
	module := ast.NewModule([]ast.Stmt{
		&ast.Assign{
			Targets: []ast.Expr{&ast.Name{Id: "a", Ctx: ast.Store}},
			Value:   &ast.Constant{Value: "hello"},
		},
		&ast.Assign{
			Targets: []ast.Expr{&ast.Name{Id: "b", Ctx: ast.Store}},
			Value:   &ast.Constant{Value: "hello"},
		},
	}, pos)

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count := 0
	for _, c := range code.Constants {
		if v, ok := c.(string); ok && v == "hello" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected constant \"hello\" to appear once in pool, got %d", count)
	}
}

// TestConstantPoolDeduplication_DifferentTypes verifies that different types
// with the same "value" are NOT deduplicated (e.g., int64(1) vs float64(1.0)).
func TestConstantPoolDeduplication_DifferentTypes(t *testing.T) {
	// x = 1
	// y = 1.0
	pos := ast.NewPos("test.py", 1, 1, 2, 5)
	module := ast.NewModule([]ast.Stmt{
		&ast.Assign{
			Targets: []ast.Expr{&ast.Name{Id: "x", Ctx: ast.Store}},
			Value:   &ast.Constant{Value: int64(1)},
		},
		&ast.Assign{
			Targets: []ast.Expr{&ast.Name{Id: "y", Ctx: ast.Store}},
			Value:   &ast.Constant{Value: float64(1.0)},
		},
	}, pos)

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both should be in the pool since they have different types.
	hasInt := false
	hasFloat := false
	for _, c := range code.Constants {
		if v, ok := c.(int64); ok && v == 1 {
			hasInt = true
		}
		if v, ok := c.(float64); ok && v == 1.0 {
			hasFloat = true
		}
	}
	if !hasInt {
		t.Error("expected int64(1) in constant pool")
	}
	if !hasFloat {
		t.Error("expected float64(1.0) in constant pool")
	}
}

// TestConstantPoolDeduplication_NoneDedup verifies that multiple references to
// None share a single constant pool entry.
func TestConstantPoolDeduplication_NoneDedup(t *testing.T) {
	// x = None
	// y = None
	pos := ast.NewPos("test.py", 1, 1, 2, 5)
	module := ast.NewModule([]ast.Stmt{
		&ast.Assign{
			Targets: []ast.Expr{&ast.Name{Id: "x", Ctx: ast.Store}},
			Value:   &ast.Constant{Value: nil},
		},
		&ast.Assign{
			Targets: []ast.Expr{&ast.Name{Id: "y", Ctx: ast.Store}},
			Value:   &ast.Constant{Value: nil},
		},
	}, pos)

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both LOAD_CONST instructions should reference the same constant index.
	var loadConstArgs []uint32
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.LOAD_CONST {
			loadConstArgs = append(loadConstArgs, instr.Arg)
		}
	}
	// The two explicit None assignments should use the same index.
	// (There's also an implicit None at the end for RETURN_VALUE.)
	if len(loadConstArgs) < 2 {
		t.Fatalf("expected at least 2 LOAD_CONST, got %d", len(loadConstArgs))
	}
	if loadConstArgs[0] != loadConstArgs[1] {
		t.Errorf("expected same constant index for both None loads, got %d and %d",
			loadConstArgs[0], loadConstArgs[1])
	}
}

// TestConstantPoolDeduplication_SharedIndex verifies that two references to the
// same integer constant emit LOAD_CONST instructions with the same operand index.
func TestConstantPoolDeduplication_SharedIndex(t *testing.T) {
	// x = 1
	// y = 1
	pos := ast.NewPos("test.py", 1, 1, 2, 5)
	module := ast.NewModule([]ast.Stmt{
		&ast.Assign{
			Targets: []ast.Expr{&ast.Name{Id: "x", Ctx: ast.Store}},
			Value:   &ast.Constant{Value: int64(1)},
		},
		&ast.Assign{
			Targets: []ast.Expr{&ast.Name{Id: "y", Ctx: ast.Store}},
			Value:   &ast.Constant{Value: int64(1)},
		},
	}, pos)

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Gather the LOAD_CONST arg for the two assignments (skip the implicit None at end).
	var loadConstArgs []uint32
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.LOAD_CONST {
			loadConstArgs = append(loadConstArgs, instr.Arg)
		}
	}
	if len(loadConstArgs) < 2 {
		t.Fatalf("expected at least 2 LOAD_CONST, got %d", len(loadConstArgs))
	}
	// The first two should reference the same constant pool index.
	if loadConstArgs[0] != loadConstArgs[1] {
		t.Errorf("expected both LOAD_CONST for value 1 to share index, got %d and %d",
			loadConstArgs[0], loadConstArgs[1])
	}
}

// =============================================================================
// Local vs Global Variable Classification Tests
// =============================================================================

// TestLocalVsGlobal_ModuleLevel verifies that module-level variables use
// STORE_NAME/LOAD_NAME (not STORE_FAST/LOAD_FAST which is for functions).
func TestLocalVsGlobal_ModuleLevel(t *testing.T) {
	// x = 10
	// y = x
	pos := ast.NewPos("test.py", 1, 1, 2, 5)
	module := ast.NewModule([]ast.Stmt{
		&ast.Assign{
			Targets: []ast.Expr{&ast.Name{Id: "x", Ctx: ast.Store}},
			Value:   &ast.Constant{Value: int64(10)},
		},
		&ast.Assign{
			Targets: []ast.Expr{&ast.Name{Id: "y", Ctx: ast.Store}},
			Value:   &ast.Name{Id: "x"},
		},
	}, pos)

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Module-level should use STORE_NAME and LOAD_NAME.
	hasStoreName := false
	hasLoadName := false
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.STORE_NAME {
			hasStoreName = true
		}
		if instr.Op == bytecode.LOAD_NAME {
			hasLoadName = true
		}
	}
	if !hasStoreName {
		t.Error("expected STORE_NAME for module-level variable assignment")
	}
	if !hasLoadName {
		t.Error("expected LOAD_NAME for module-level variable load")
	}

	// Should NOT use STORE_FAST or LOAD_FAST at module level.
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.STORE_FAST || instr.Op == bytecode.LOAD_FAST {
			t.Error("did not expect STORE_FAST/LOAD_FAST at module level")
			break
		}
	}
}

// TestLocalVsGlobal_FunctionLocals verifies that assigned variables inside a
// function use LOAD_FAST/STORE_FAST (local), not LOAD_GLOBAL/STORE_GLOBAL.
func TestLocalVsGlobal_FunctionLocals(t *testing.T) {
	// def foo():
	//     x = 10
	//     return x
	pos := ast.NewPos("test.py", 1, 1, 3, 1)
	module := ast.NewModule([]ast.Stmt{
		&ast.FunctionDef{
			Name: "foo",
			Args: &ast.Arguments{},
			Body: []ast.Stmt{
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "x", Ctx: ast.Store}},
					Value:   &ast.Constant{Value: int64(10)},
				},
				&ast.Return{Value: &ast.Name{Id: "x"}},
			},
		},
	}, pos)

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get the nested function code.
	var funcCode *bytecode.Code
	for _, c := range code.Constants {
		if nc, ok := c.(*bytecode.Code); ok {
			funcCode = nc
			break
		}
	}
	if funcCode == nil {
		t.Fatal("expected nested Code object for function")
	}

	// Verify STORE_FAST is used for x (not STORE_NAME or STORE_GLOBAL).
	hasStoreFast := false
	hasLoadFast := false
	for _, instr := range funcCode.Instructions {
		if instr.Op == bytecode.STORE_FAST {
			hasStoreFast = true
		}
		if instr.Op == bytecode.LOAD_FAST {
			hasLoadFast = true
		}
	}
	if !hasStoreFast {
		t.Error("expected STORE_FAST for local variable x in function")
	}
	if !hasLoadFast {
		t.Error("expected LOAD_FAST for local variable x in function")
	}
}

// TestLocalVsGlobal_GlobalDeclaration verifies that a variable declared global
// inside a function uses LOAD_GLOBAL/STORE_GLOBAL.
func TestLocalVsGlobal_GlobalDeclaration(t *testing.T) {
	// def foo():
	//     global g
	//     g = 99
	//     return g
	pos := ast.NewPos("test.py", 1, 1, 4, 1)
	module := ast.NewModule([]ast.Stmt{
		&ast.FunctionDef{
			Name: "foo",
			Args: &ast.Arguments{},
			Body: []ast.Stmt{
				&ast.Global{Names: []string{"g"}},
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "g", Ctx: ast.Store}},
					Value:   &ast.Constant{Value: int64(99)},
				},
				&ast.Return{Value: &ast.Name{Id: "g"}},
			},
		},
	}, pos)

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get the nested function code.
	var funcCode *bytecode.Code
	for _, c := range code.Constants {
		if nc, ok := c.(*bytecode.Code); ok {
			funcCode = nc
			break
		}
	}
	if funcCode == nil {
		t.Fatal("expected nested Code object for function")
	}

	// Verify STORE_GLOBAL and LOAD_GLOBAL are used for g.
	hasStoreGlobal := false
	hasLoadGlobal := false
	for _, instr := range funcCode.Instructions {
		if instr.Op == bytecode.STORE_GLOBAL {
			hasStoreGlobal = true
		}
		if instr.Op == bytecode.LOAD_GLOBAL {
			hasLoadGlobal = true
		}
	}
	if !hasStoreGlobal {
		t.Error("expected STORE_GLOBAL for variable declared global")
	}
	if !hasLoadGlobal {
		t.Error("expected LOAD_GLOBAL for variable declared global")
	}

	// Verify STORE_FAST/LOAD_FAST are NOT used for g.
	for _, instr := range funcCode.Instructions {
		if instr.Op == bytecode.STORE_FAST || instr.Op == bytecode.LOAD_FAST {
			t.Error("did not expect STORE_FAST/LOAD_FAST for global variable g")
			break
		}
	}
}

// TestLocalVsGlobal_MixedScope verifies that a function with both local
// variables and global accesses uses the correct load/store instructions.
func TestLocalVsGlobal_MixedScope(t *testing.T) {
	// def foo(a):
	//     global g
	//     b = a + g
	//     g = b
	//     return b
	pos := ast.NewPos("test.py", 1, 1, 5, 1)
	module := ast.NewModule([]ast.Stmt{
		&ast.FunctionDef{
			Name: "foo",
			Args: &ast.Arguments{
				Args: []*ast.Arg{{Arg: "a"}},
			},
			Body: []ast.Stmt{
				&ast.Global{Names: []string{"g"}},
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "b", Ctx: ast.Store}},
					Value: &ast.BinOp{
						Left:  &ast.Name{Id: "a"},
						Op:    ast.Add,
						Right: &ast.Name{Id: "g"},
					},
				},
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "g", Ctx: ast.Store}},
					Value:   &ast.Name{Id: "b"},
				},
				&ast.Return{Value: &ast.Name{Id: "b"}},
			},
		},
	}, pos)

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var funcCode *bytecode.Code
	for _, c := range code.Constants {
		if nc, ok := c.(*bytecode.Code); ok {
			funcCode = nc
			break
		}
	}
	if funcCode == nil {
		t.Fatal("expected nested Code object")
	}

	// Verify: a and b should use LOAD_FAST/STORE_FAST
	// g should use LOAD_GLOBAL/STORE_GLOBAL
	hasLoadFast := false
	hasStoreFast := false
	hasLoadGlobal := false
	hasStoreGlobal := false
	for _, instr := range funcCode.Instructions {
		switch instr.Op {
		case bytecode.LOAD_FAST:
			hasLoadFast = true
		case bytecode.STORE_FAST:
			hasStoreFast = true
		case bytecode.LOAD_GLOBAL:
			hasLoadGlobal = true
		case bytecode.STORE_GLOBAL:
			hasStoreGlobal = true
		}
	}
	if !hasLoadFast {
		t.Error("expected LOAD_FAST for local variables (a, b)")
	}
	if !hasStoreFast {
		t.Error("expected STORE_FAST for local variable b")
	}
	if !hasLoadGlobal {
		t.Error("expected LOAD_GLOBAL for global variable g")
	}
	if !hasStoreGlobal {
		t.Error("expected STORE_GLOBAL for global variable g")
	}

	// Verify Varnames contains "a" and "b" but not "g".
	foundA := false
	foundB := false
	foundG := false
	for _, v := range funcCode.Varnames {
		switch v {
		case "a":
			foundA = true
		case "b":
			foundB = true
		case "g":
			foundG = true
		}
	}
	if !foundA || !foundB {
		t.Errorf("expected Varnames to contain 'a' and 'b', got %v", funcCode.Varnames)
	}
	if foundG {
		t.Error("expected Varnames NOT to contain 'g' (it's global)")
	}
}

// =============================================================================
// Function Compilation Produces Nested Code Objects Tests
// =============================================================================

// TestFunctionCompilation_NestedCodeObject verifies that compiling a function
// definition produces a *bytecode.Code in the module's constant pool with
// correct metadata.
func TestFunctionCompilation_NestedCodeObject(t *testing.T) {
	// def greet(name, greeting):
	//     msg = greeting + name
	//     return msg
	pos := ast.NewPos("test.py", 1, 1, 3, 1)
	module := ast.NewModule([]ast.Stmt{
		&ast.FunctionDef{
			Name: "greet",
			Args: &ast.Arguments{
				Args: []*ast.Arg{
					{Arg: "name"},
					{Arg: "greeting"},
				},
			},
			Body: []ast.Stmt{
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "msg", Ctx: ast.Store}},
					Value: &ast.BinOp{
						Left:  &ast.Name{Id: "greeting"},
						Op:    ast.Add,
						Right: &ast.Name{Id: "name"},
					},
				},
				&ast.Return{Value: &ast.Name{Id: "msg"}},
			},
		},
	}, pos)

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the nested Code object.
	var funcCode *bytecode.Code
	for _, c := range code.Constants {
		if nc, ok := c.(*bytecode.Code); ok {
			funcCode = nc
			break
		}
	}
	if funcCode == nil {
		t.Fatal("expected nested *bytecode.Code in module constants")
	}

	// Verify Name.
	if funcCode.Name != "greet" {
		t.Errorf("expected function code Name='greet', got %q", funcCode.Name)
	}

	// Verify ArgCount.
	if funcCode.ArgCount != 2 {
		t.Errorf("expected ArgCount=2, got %d", funcCode.ArgCount)
	}
}

// TestFunctionCompilation_VarnamesAndFlags verifies that function code objects
// have correct Varnames (parameters + locals) and FlagOptimized | FlagNewLocals.
func TestFunctionCompilation_VarnamesAndFlags(t *testing.T) {
	// def compute(x, y):
	//     result = x + y
	//     return result
	pos := ast.NewPos("test.py", 1, 1, 3, 1)
	module := ast.NewModule([]ast.Stmt{
		&ast.FunctionDef{
			Name: "compute",
			Args: &ast.Arguments{
				Args: []*ast.Arg{
					{Arg: "x"},
					{Arg: "y"},
				},
			},
			Body: []ast.Stmt{
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "result", Ctx: ast.Store}},
					Value: &ast.BinOp{
						Left:  &ast.Name{Id: "x"},
						Op:    ast.Add,
						Right: &ast.Name{Id: "y"},
					},
				},
				&ast.Return{Value: &ast.Name{Id: "result"}},
			},
		},
	}, pos)

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var funcCode *bytecode.Code
	for _, c := range code.Constants {
		if nc, ok := c.(*bytecode.Code); ok {
			funcCode = nc
			break
		}
	}
	if funcCode == nil {
		t.Fatal("expected nested Code object")
	}

	// Verify Varnames contains parameters first, then locals.
	if len(funcCode.Varnames) < 2 {
		t.Fatalf("expected at least 2 Varnames entries, got %d", len(funcCode.Varnames))
	}
	if funcCode.Varnames[0] != "x" || funcCode.Varnames[1] != "y" {
		t.Errorf("expected Varnames[0:2]=['x','y'], got %v", funcCode.Varnames[:2])
	}
	// "result" should also be in Varnames.
	foundResult := false
	for _, v := range funcCode.Varnames {
		if v == "result" {
			foundResult = true
		}
	}
	if !foundResult {
		t.Errorf("expected 'result' in Varnames, got %v", funcCode.Varnames)
	}

	// Verify flags.
	if !funcCode.HasFlag(bytecode.FlagOptimized) {
		t.Error("expected FlagOptimized on function code")
	}
	if !funcCode.HasFlag(bytecode.FlagNewLocals) {
		t.Error("expected FlagNewLocals on function code")
	}
}

// =============================================================================
// Jump Offset Patching Tests
// =============================================================================

// TestJumpPatching_IfElse_PopJumpIfFalse verifies that POP_JUMP_IF_FALSE in an
// if/else points to a valid instruction offset (the start of the else branch).
func TestJumpPatching_IfElse_PopJumpIfFalse(t *testing.T) {
	// if x:
	//     a = 1
	// else:
	//     a = 2
	pos := ast.NewPos("test.py", 1, 1, 4, 5)
	module := ast.NewModule([]ast.Stmt{
		&ast.If{
			Test: &ast.Name{Id: "x"},
			Body: []ast.Stmt{
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "a", Ctx: ast.Store}},
					Value:   &ast.Constant{Value: int64(1)},
				},
			},
			Orelse: []ast.Stmt{
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "a", Ctx: ast.Store}},
					Value:   &ast.Constant{Value: int64(2)},
				},
			},
		},
	}, pos)

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find POP_JUMP_IF_FALSE and verify its target is within bounds.
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.POP_JUMP_IF_FALSE {
			target := int(instr.Arg)
			if target <= 0 || target >= len(code.Instructions) {
				t.Errorf("POP_JUMP_IF_FALSE target %d is out of bounds [0, %d)",
					target, len(code.Instructions))
			}
			// The target should be after the body instructions + JUMP_FORWARD.
			// It should NOT point to instruction index 0.
			if target <= 1 {
				t.Errorf("POP_JUMP_IF_FALSE target %d seems too early", target)
			}
			break
		}
	}
}

// TestJumpPatching_While_BackEdge verifies that JUMP_ABSOLUTE in a while loop
// points back to the loop start (where the test condition is evaluated).
func TestJumpPatching_While_BackEdge(t *testing.T) {
	// while x:
	//     x = x - 1
	pos := ast.NewPos("test.py", 1, 1, 2, 5)
	module := ast.NewModule([]ast.Stmt{
		&ast.While{
			Test: &ast.Name{Id: "x"},
			Body: []ast.Stmt{
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "x", Ctx: ast.Store}},
					Value: &ast.BinOp{
						Left:  &ast.Name{Id: "x"},
						Op:    ast.Sub,
						Right: &ast.Constant{Value: int64(1)},
					},
				},
			},
		},
	}, pos)

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The loop structure should be:
	// [0]: LOAD_NAME x       (loop start)
	// [1]: POP_JUMP_IF_FALSE (exit)
	// ...body...
	// [N]: JUMP_ABSOLUTE 0   (back-edge to loop start)
	//
	// Find the back-edge JUMP_ABSOLUTE.
	loopStart := 0 // while loop starts at instruction 0
	foundBackEdge := false
	for i, instr := range code.Instructions {
		if instr.Op == bytecode.JUMP_ABSOLUTE && int(instr.Arg) == loopStart && i > loopStart {
			foundBackEdge = true
			break
		}
	}
	if !foundBackEdge {
		t.Error("expected JUMP_ABSOLUTE back-edge targeting loop start (offset 0)")
	}
}

// TestJumpPatching_JumpForward_SkipsElseBranch verifies that JUMP_FORWARD in
// an if/else correctly skips the else branch.
func TestJumpPatching_JumpForward_SkipsElseBranch(t *testing.T) {
	// if x:
	//     a = 1
	// else:
	//     a = 2
	pos := ast.NewPos("test.py", 1, 1, 4, 5)
	module := ast.NewModule([]ast.Stmt{
		&ast.If{
			Test: &ast.Name{Id: "x"},
			Body: []ast.Stmt{
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "a", Ctx: ast.Store}},
					Value:   &ast.Constant{Value: int64(1)},
				},
			},
			Orelse: []ast.Stmt{
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "a", Ctx: ast.Store}},
					Value:   &ast.Constant{Value: int64(2)},
				},
			},
		},
	}, pos)

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the JUMP_FORWARD instruction.
	jumpFwdIdx := -1
	for i, instr := range code.Instructions {
		if instr.Op == bytecode.JUMP_FORWARD {
			jumpFwdIdx = i
			break
		}
	}
	if jumpFwdIdx == -1 {
		t.Fatal("expected JUMP_FORWARD in if/else bytecode")
	}

	// JUMP_FORWARD's arg is the number of instructions to skip.
	// The target = jumpFwdIdx + 1 + arg, and should be within bounds.
	delta := int(code.Instructions[jumpFwdIdx].Arg)
	target := jumpFwdIdx + 1 + delta
	if target < jumpFwdIdx || target > len(code.Instructions) {
		t.Errorf("JUMP_FORWARD target %d (idx=%d, delta=%d) is invalid, total instructions=%d",
			target, jumpFwdIdx, delta, len(code.Instructions))
	}

	// The target should be past the else branch.
	// The else branch is at least 2 instructions (LOAD_CONST + STORE_NAME).
	if delta < 2 {
		t.Errorf("JUMP_FORWARD delta %d seems too small to skip else branch", delta)
	}
}

// TestJumpPatching_WhileBreak_JumpsAfterLoop verifies that break inside a while
// loop jumps to an offset after the entire loop (including else clause if any).
func TestJumpPatching_WhileBreak_JumpsAfterLoop(t *testing.T) {
	// while x:
	//     if y:
	//         break
	//     x = x - 1
	pos := ast.NewPos("test.py", 1, 1, 4, 5)
	module := ast.NewModule([]ast.Stmt{
		&ast.While{
			Test: &ast.Name{Id: "x"},
			Body: []ast.Stmt{
				&ast.If{
					Test: &ast.Name{Id: "y"},
					Body: []ast.Stmt{&ast.Break{}},
				},
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "x", Ctx: ast.Store}},
					Value: &ast.BinOp{
						Left:  &ast.Name{Id: "x"},
						Op:    ast.Sub,
						Right: &ast.Constant{Value: int64(1)},
					},
				},
			},
		},
	}, pos)

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the back-edge (JUMP_ABSOLUTE that targets an earlier offset).
	backEdgeIdx := -1
	for i, instr := range code.Instructions {
		if instr.Op == bytecode.JUMP_ABSOLUTE && int(instr.Arg) < i {
			backEdgeIdx = i
		}
	}
	if backEdgeIdx == -1 {
		t.Fatal("expected a back-edge JUMP_ABSOLUTE")
	}

	// Find the break JUMP_ABSOLUTE (targets an offset after back-edge).
	breakFound := false
	for i, instr := range code.Instructions {
		if instr.Op == bytecode.JUMP_ABSOLUTE && i != backEdgeIdx && int(instr.Arg) > backEdgeIdx {
			breakFound = true
			// Verify the target is within bounds.
			target := int(instr.Arg)
			if target > len(code.Instructions) {
				t.Errorf("break target %d exceeds instruction count %d",
					target, len(code.Instructions))
			}
			break
		}
	}
	if !breakFound {
		t.Error("expected break's JUMP_ABSOLUTE to target after the loop back-edge")
	}
}

// =============================================================================
// Line Table Accuracy Tests
// =============================================================================

// TestLineTable_MultiLineStatements verifies that the line table correctly maps
// instruction offsets to source lines.
func TestLineTable_MultiLineStatements(t *testing.T) {
	// Line 1: a = 10
	// Line 2: b = 20
	// Line 3: c = a + b
	pos1 := ast.NewPos("test.py", 1, 1, 1, 6)
	pos2 := ast.NewPos("test.py", 2, 1, 2, 6)
	pos3 := ast.NewPos("test.py", 3, 1, 3, 9)

	module := ast.NewModule([]ast.Stmt{
		ast.NewAssign(
			[]ast.Expr{ast.NewName("a", ast.Store, pos1)},
			ast.NewConstant(int64(10), "", pos1),
			pos1,
		),
		ast.NewAssign(
			[]ast.Expr{ast.NewName("b", ast.Store, pos2)},
			ast.NewConstant(int64(20), "", pos2),
			pos2,
		),
		ast.NewAssign(
			[]ast.Expr{ast.NewName("c", ast.Store, pos3)},
			ast.NewBinOp(
				ast.NewName("a", ast.Load, pos3),
				ast.Add,
				ast.NewName("b", ast.Load, pos3),
				pos3,
			),
			pos3,
		),
	}, ast.NewPos("test.py", 1, 1, 3, 9))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Line table should have entries.
	if len(code.LineTable) == 0 {
		t.Fatal("expected non-empty LineTable")
	}

	// Line table should have pairs (bytecode_delta, line_delta).
	if len(code.LineTable)%2 != 0 {
		t.Fatalf("expected even number of LineTable entries, got %d", len(code.LineTable))
	}

	// Use LineForOffset to verify source lines for known instruction offsets.
	// Instruction 0 should be on the first line (1).
	line0 := code.LineForOffset(0)
	if line0 != 1 {
		t.Errorf("expected line 1 at offset 0, got %d", line0)
	}

	// The first statement (a = 10) emits 2 instructions: LOAD_CONST + STORE_NAME.
	// So offset 2 should be on line 2.
	line2 := code.LineForOffset(2)
	if line2 != 2 {
		t.Errorf("expected line 2 at offset 2, got %d", line2)
	}
}

// TestLineTable_FunctionBody verifies line table accuracy in a function body.
func TestLineTable_FunctionBody(t *testing.T) {
	// Line 1: def foo():
	// Line 2:     x = 1
	// Line 3:     y = 2
	// Line 4:     return x + y
	pos1 := ast.NewPos("test.py", 1, 1, 4, 1)
	pos2 := ast.NewPos("test.py", 2, 5, 2, 10)
	pos3 := ast.NewPos("test.py", 3, 5, 3, 10)
	pos4 := ast.NewPos("test.py", 4, 5, 4, 16)

	module := ast.NewModule([]ast.Stmt{
		ast.NewFunctionDef("foo", &ast.Arguments{}, []ast.Stmt{
			ast.NewAssign(
				[]ast.Expr{ast.NewName("x", ast.Store, pos2)},
				ast.NewConstant(int64(1), "", pos2),
				pos2,
			),
			ast.NewAssign(
				[]ast.Expr{ast.NewName("y", ast.Store, pos3)},
				ast.NewConstant(int64(2), "", pos3),
				pos3,
			),
			ast.NewReturn(
				ast.NewBinOp(
					ast.NewName("x", ast.Load, pos4),
					ast.Add,
					ast.NewName("y", ast.Load, pos4),
					pos4,
				),
				pos4,
			),
		}, nil, nil, pos1),
	}, ast.NewPos("test.py", 1, 1, 4, 16))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get the function's code object.
	var funcCode *bytecode.Code
	for _, c := range code.Constants {
		if nc, ok := c.(*bytecode.Code); ok {
			funcCode = nc
			break
		}
	}
	if funcCode == nil {
		t.Fatal("expected nested Code object")
	}

	// The function code should have line table entries tracking line changes.
	if len(funcCode.LineTable) == 0 {
		t.Error("expected non-empty LineTable in function code")
	}
	if len(funcCode.LineTable)%2 != 0 {
		t.Errorf("expected even number of LineTable entries, got %d", len(funcCode.LineTable))
	}
}

// TestLineTable_LineDeltas verifies that line table deltas are correct
// by summing them and checking they produce the expected line progression.
func TestLineTable_LineDeltas(t *testing.T) {
	// Line 1: x = 1
	// Line 3: y = 2  (skipping line 2)
	// Line 4: z = 3
	pos1 := ast.NewPos("test.py", 1, 1, 1, 5)
	pos3 := ast.NewPos("test.py", 3, 1, 3, 5)
	pos4 := ast.NewPos("test.py", 4, 1, 4, 5)

	module := ast.NewModule([]ast.Stmt{
		ast.NewAssign(
			[]ast.Expr{ast.NewName("x", ast.Store, pos1)},
			ast.NewConstant(int64(1), "", pos1),
			pos1,
		),
		ast.NewAssign(
			[]ast.Expr{ast.NewName("y", ast.Store, pos3)},
			ast.NewConstant(int64(2), "", pos3),
			pos3,
		),
		ast.NewAssign(
			[]ast.Expr{ast.NewName("z", ast.Store, pos4)},
			ast.NewConstant(int64(3), "", pos4),
			pos4,
		),
	}, ast.NewPos("test.py", 1, 1, 4, 5))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Line table should exist.
	if len(code.LineTable) == 0 {
		t.Fatal("expected non-empty LineTable")
	}

	// Verify that the sum of all line deltas from FirstLine reaches the last line.
	totalLineDelta := 0
	for i := 1; i < len(code.LineTable); i += 2 {
		totalLineDelta += code.LineTable[i]
	}
	// Starting from line 1, we should reach at least line 4 (delta of 3).
	if totalLineDelta < 3 {
		t.Errorf("expected total line delta >= 3 (from line 1 to line 4), got %d", totalLineDelta)
	}

	// Verify LineForOffset works for a later offset.
	// The last statement (z = 3) should be on line 4.
	// Each assignment is 2 instructions (LOAD_CONST + STORE_NAME).
	// Offset 4 = start of third statement.
	line := code.LineForOffset(4)
	if line != 4 {
		t.Errorf("expected line 4 at offset 4, got %d", line)
	}
}

// =============================================================================
// Nonlocal Statement Tests
// =============================================================================

// TestNonlocal_ValidDeclaration verifies that a valid nonlocal declaration
// compiles successfully when the name exists in an enclosing scope.
func TestNonlocal_ValidDeclaration(t *testing.T) {
	// def outer():
	//     x = 10
	//     def inner():
	//         nonlocal x
	//         x = 20
	//     return inner
	pos := ast.NewPos("test.py", 1, 1, 6, 1)
	module := ast.NewModule([]ast.Stmt{
		&ast.FunctionDef{
			Name: "outer",
			Args: &ast.Arguments{},
			Body: []ast.Stmt{
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "x", Ctx: ast.Store}},
					Value:   &ast.Constant{Value: int64(10)},
				},
				&ast.FunctionDef{
					Name: "inner",
					Args: &ast.Arguments{},
					Body: []ast.Stmt{
						ast.NewNonlocal([]string{"x"}, pos),
						&ast.Assign{
							Targets: []ast.Expr{&ast.Name{Id: "x", Ctx: ast.Store}},
							Value:   &ast.Constant{Value: int64(20)},
						},
					},
				},
				&ast.Return{Value: &ast.Name{Id: "inner"}},
			},
		},
	}, pos)

	_, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error compiling valid nonlocal: %v", err)
	}
}

// TestNonlocal_InvalidDeclaration verifies that using nonlocal for a name
// not defined in any enclosing scope raises a SyntaxError.
func TestNonlocal_InvalidDeclaration(t *testing.T) {
	// def outer():
	//     def inner():
	//         nonlocal missing
	//         missing = 1
	pos := ast.NewPos("test.py", 1, 1, 4, 1)
	module := ast.NewModule([]ast.Stmt{
		&ast.FunctionDef{
			Name: "outer",
			Args: &ast.Arguments{},
			Body: []ast.Stmt{
				&ast.FunctionDef{
					Name: "inner",
					Args: &ast.Arguments{},
					Body: []ast.Stmt{
						ast.NewNonlocal([]string{"missing"}, pos),
						&ast.Assign{
							Targets: []ast.Expr{&ast.Name{Id: "missing", Ctx: ast.Store}},
							Value:   &ast.Constant{Value: int64(1)},
						},
					},
				},
			},
		},
	}, pos)

	_, err := Compile(module)
	if err == nil {
		t.Fatal("expected error for nonlocal with no enclosing binding, got nil")
	}
	expected := "SyntaxError: no binding for nonlocal 'missing' found"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

// TestNonlocal_MultipleNames verifies that nonlocal with multiple comma-separated
// names is handled correctly by the compiler.
func TestNonlocal_MultipleNames(t *testing.T) {
	// def outer():
	//     a = 1
	//     b = 2
	//     def inner():
	//         nonlocal a, b
	//         a = 10
	//         b = 20
	pos := ast.NewPos("test.py", 1, 1, 7, 1)
	module := ast.NewModule([]ast.Stmt{
		&ast.FunctionDef{
			Name: "outer",
			Args: &ast.Arguments{},
			Body: []ast.Stmt{
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "a", Ctx: ast.Store}},
					Value:   &ast.Constant{Value: int64(1)},
				},
				&ast.Assign{
					Targets: []ast.Expr{&ast.Name{Id: "b", Ctx: ast.Store}},
					Value:   &ast.Constant{Value: int64(2)},
				},
				&ast.FunctionDef{
					Name: "inner",
					Args: &ast.Arguments{},
					Body: []ast.Stmt{
						ast.NewNonlocal([]string{"a", "b"}, pos),
						&ast.Assign{
							Targets: []ast.Expr{&ast.Name{Id: "a", Ctx: ast.Store}},
							Value:   &ast.Constant{Value: int64(10)},
						},
						&ast.Assign{
							Targets: []ast.Expr{&ast.Name{Id: "b", Ctx: ast.Store}},
							Value:   &ast.Constant{Value: int64(20)},
						},
					},
				},
			},
		},
	}, pos)

	_, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error compiling multiple nonlocal names: %v", err)
	}
}
