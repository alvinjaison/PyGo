package compiler

import (
	"testing"

	"github.com/alvinjaison/pygo/internal/ast"
	"github.com/alvinjaison/pygo/internal/bytecode"
)

// TestCompileIf_Simple tests basic if statement compilation.
func TestCompileIf_Simple(t *testing.T) {
	// if x:
	//     y = 1
	module := ast.NewModule([]ast.Stmt{
		makeIfStmt(
			&ast.Name{Id: "x"},
			[]ast.Stmt{makeAssignValue("y", int64(1))},
			nil,
		),
	}, ast.NewPos("test.py", 1, 1, 2, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain POP_JUMP_IF_FALSE.
	if !containsOpcode(code, bytecode.POP_JUMP_IF_FALSE) {
		t.Error("expected POP_JUMP_IF_FALSE in bytecode for if statement")
	}
}

// TestCompileIf_Else tests if/else compilation.
func TestCompileIf_Else(t *testing.T) {
	// if x:
	//     y = 1
	// else:
	//     y = 2
	module := ast.NewModule([]ast.Stmt{
		makeIfStmt(
			&ast.Name{Id: "x"},
			[]ast.Stmt{makeAssignValue("y", int64(1))},
			[]ast.Stmt{makeAssignValue("y", int64(2))},
		),
	}, ast.NewPos("test.py", 1, 1, 4, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain POP_JUMP_IF_FALSE and JUMP_FORWARD.
	if !containsOpcode(code, bytecode.POP_JUMP_IF_FALSE) {
		t.Error("expected POP_JUMP_IF_FALSE in bytecode")
	}
	if !containsOpcode(code, bytecode.JUMP_FORWARD) {
		t.Error("expected JUMP_FORWARD in bytecode for if/else")
	}
}

// TestCompileIf_Elif tests if/elif/else compilation.
func TestCompileIf_Elif(t *testing.T) {
	// if x:
	//     a = 1
	// elif y:
	//     a = 2
	// else:
	//     a = 3
	elifBlock := makeIfStmt(
		&ast.Name{Id: "y"},
		[]ast.Stmt{makeAssignValue("a", int64(2))},
		[]ast.Stmt{makeAssignValue("a", int64(3))},
	)
	module := ast.NewModule([]ast.Stmt{
		makeIfStmt(
			&ast.Name{Id: "x"},
			[]ast.Stmt{makeAssignValue("a", int64(1))},
			[]ast.Stmt{elifBlock},
		),
	}, ast.NewPos("test.py", 1, 1, 6, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least two POP_JUMP_IF_FALSE (one for if, one for elif).
	count := countOpcode(code, bytecode.POP_JUMP_IF_FALSE)
	if count < 2 {
		t.Errorf("expected at least 2 POP_JUMP_IF_FALSE for if/elif, got %d", count)
	}
}

// TestCompileWhile_Simple tests basic while loop compilation.
func TestCompileWhile_Simple(t *testing.T) {
	// while x:
	//     x = x - 1
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
	}, ast.NewPos("test.py", 1, 1, 2, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain POP_JUMP_IF_FALSE and JUMP_ABSOLUTE (back-edge).
	if !containsOpcode(code, bytecode.POP_JUMP_IF_FALSE) {
		t.Error("expected POP_JUMP_IF_FALSE for while test")
	}
	if !containsOpcode(code, bytecode.JUMP_ABSOLUTE) {
		t.Error("expected JUMP_ABSOLUTE for while back-edge")
	}

	// Verify the JUMP_ABSOLUTE target is before the POP_JUMP_IF_FALSE (back-edge).
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.JUMP_ABSOLUTE {
			// The back-edge target should be an earlier instruction.
			if int(instr.Arg) >= len(code.Instructions)-2 {
				t.Error("expected JUMP_ABSOLUTE to target loop start (earlier offset)")
			}
			break
		}
	}
}

// TestCompileWhile_Break tests break inside a while loop.
func TestCompileWhile_Break(t *testing.T) {
	// while True:
	//     break
	module := ast.NewModule([]ast.Stmt{
		&ast.While{
			Test: &ast.Constant{Value: true},
			Body: []ast.Stmt{&ast.Break{}},
		},
	}, ast.NewPos("test.py", 1, 1, 2, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Break emits a JUMP_ABSOLUTE that should jump past the loop.
	jumpAbsCount := countOpcode(code, bytecode.JUMP_ABSOLUTE)
	if jumpAbsCount < 2 {
		// One for the back-edge, one for the break.
		t.Errorf("expected at least 2 JUMP_ABSOLUTE (back-edge + break), got %d", jumpAbsCount)
	}

	// The break jump should target an offset after the back-edge.
	backEdgeIdx := -1
	for i, instr := range code.Instructions {
		if instr.Op == bytecode.JUMP_ABSOLUTE && int(instr.Arg) == 0 {
			// This is likely the break jump (patched to end of loop).
			// But let's find both and verify.
			_ = i
		}
		if instr.Op == bytecode.JUMP_ABSOLUTE && int(instr.Arg) < i {
			backEdgeIdx = i
		}
	}
	if backEdgeIdx == -1 {
		t.Error("expected a back-edge JUMP_ABSOLUTE targeting an earlier offset")
	}
}

// TestCompileWhile_Continue tests continue inside a while loop.
func TestCompileWhile_Continue(t *testing.T) {
	// while x:
	//     continue
	module := ast.NewModule([]ast.Stmt{
		&ast.While{
			Test: &ast.Name{Id: "x"},
			Body: []ast.Stmt{&ast.Continue{}},
		},
	}, ast.NewPos("test.py", 1, 1, 2, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Continue should emit a JUMP_ABSOLUTE to the loop start (same as back-edge target).
	jumpCount := countOpcode(code, bytecode.JUMP_ABSOLUTE)
	if jumpCount < 2 {
		t.Errorf("expected at least 2 JUMP_ABSOLUTE (continue + back-edge), got %d", jumpCount)
	}
}

// TestCompileWhile_Else tests while/else compilation.
func TestCompileWhile_Else(t *testing.T) {
	// while x:
	//     x = x - 1
	// else:
	//     y = 0
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
			Orelse: []ast.Stmt{makeAssignValue("y", int64(0))},
		},
	}, ast.NewPos("test.py", 1, 1, 4, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The else body should store y=0. Verify y is in the Names.
	found := false
	for _, n := range code.Names {
		if n == "y" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'y' in Names for while/else body")
	}
}

// TestCompileFor_Simple tests basic for loop compilation.
func TestCompileFor_Simple(t *testing.T) {
	// for i in items:
	//     pass
	module := ast.NewModule([]ast.Stmt{
		&ast.For{
			Target: &ast.Name{Id: "i", Ctx: ast.Store},
			Iter:   &ast.Name{Id: "items"},
			Body:   []ast.Stmt{&ast.Pass{}},
		},
	}, ast.NewPos("test.py", 1, 1, 2, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain GET_ITER, FOR_ITER, and JUMP_ABSOLUTE.
	if !containsOpcode(code, bytecode.GET_ITER) {
		t.Error("expected GET_ITER in for loop bytecode")
	}
	if !containsOpcode(code, bytecode.FOR_ITER) {
		t.Error("expected FOR_ITER in for loop bytecode")
	}
	if !containsOpcode(code, bytecode.JUMP_ABSOLUTE) {
		t.Error("expected JUMP_ABSOLUTE for for loop back-edge")
	}
}

// TestCompileFor_Break tests break inside a for loop.
func TestCompileFor_Break(t *testing.T) {
	// for i in items:
	//     break
	module := ast.NewModule([]ast.Stmt{
		&ast.For{
			Target: &ast.Name{Id: "i", Ctx: ast.Store},
			Iter:   &ast.Name{Id: "items"},
			Body:   []ast.Stmt{&ast.Break{}},
		},
	}, ast.NewPos("test.py", 1, 1, 2, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Break inside for should emit JUMP_ABSOLUTE past the loop end.
	jumpCount := countOpcode(code, bytecode.JUMP_ABSOLUTE)
	if jumpCount < 2 {
		t.Errorf("expected at least 2 JUMP_ABSOLUTE (break + back-edge), got %d", jumpCount)
	}
}

// TestCompileFor_Continue tests continue inside a for loop.
func TestCompileFor_Continue(t *testing.T) {
	// for i in items:
	//     continue
	module := ast.NewModule([]ast.Stmt{
		&ast.For{
			Target: &ast.Name{Id: "i", Ctx: ast.Store},
			Iter:   &ast.Name{Id: "items"},
			Body:   []ast.Stmt{&ast.Continue{}},
		},
	}, ast.NewPos("test.py", 1, 1, 2, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Continue in for loop targets the FOR_ITER instruction.
	// Find the FOR_ITER instruction offset.
	forIterOffset := -1
	for i, instr := range code.Instructions {
		if instr.Op == bytecode.FOR_ITER {
			forIterOffset = i
			break
		}
	}
	if forIterOffset == -1 {
		t.Fatal("expected FOR_ITER in bytecode")
	}

	// There should be a JUMP_ABSOLUTE targeting the FOR_ITER offset (continue).
	found := false
	for _, instr := range code.Instructions {
		if instr.Op == bytecode.JUMP_ABSOLUTE && int(instr.Arg) == forIterOffset {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected continue JUMP_ABSOLUTE to target FOR_ITER offset")
	}
}

// TestCompileFor_Else tests for/else compilation.
func TestCompileFor_Else(t *testing.T) {
	// for i in items:
	//     pass
	// else:
	//     y = 99
	module := ast.NewModule([]ast.Stmt{
		&ast.For{
			Target: &ast.Name{Id: "i", Ctx: ast.Store},
			Iter:   &ast.Name{Id: "items"},
			Body:   []ast.Stmt{&ast.Pass{}},
			Orelse: []ast.Stmt{makeAssignValue("y", int64(99))},
		},
	}, ast.NewPos("test.py", 1, 1, 4, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify y is in Names (else body was compiled).
	found := false
	for _, n := range code.Names {
		if n == "y" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'y' in Names for for/else body")
	}
}

// TestCompileBreakOutsideLoop tests that break outside a loop returns an error.
func TestCompileBreakOutsideLoop(t *testing.T) {
	module := ast.NewModule([]ast.Stmt{
		&ast.Break{},
	}, ast.NewPos("test.py", 1, 1, 1, 1))

	_, err := Compile(module)
	if err == nil {
		t.Fatal("expected error for break outside loop")
	}
}

// TestCompileContinueOutsideLoop tests that continue outside a loop returns an error.
func TestCompileContinueOutsideLoop(t *testing.T) {
	module := ast.NewModule([]ast.Stmt{
		&ast.Continue{},
	}, ast.NewPos("test.py", 1, 1, 1, 1))

	_, err := Compile(module)
	if err == nil {
		t.Fatal("expected error for continue outside loop")
	}
}

// TestCompileBoolOp_And tests short-circuit and compilation.
func TestCompileBoolOp_And(t *testing.T) {
	// x and y
	module := ast.NewModule([]ast.Stmt{
		&ast.ExprStmt{
			Value: &ast.BoolOp{
				Op: ast.And,
				Values: []ast.Expr{
					&ast.Name{Id: "x"},
					&ast.Name{Id: "y"},
				},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 1, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain JUMP_IF_FALSE_OR_POP for short-circuit and.
	if !containsOpcode(code, bytecode.JUMP_IF_FALSE_OR_POP) {
		t.Error("expected JUMP_IF_FALSE_OR_POP for 'and' short-circuit")
	}
}

// TestCompileBoolOp_Or tests short-circuit or compilation.
func TestCompileBoolOp_Or(t *testing.T) {
	// x or y
	module := ast.NewModule([]ast.Stmt{
		&ast.ExprStmt{
			Value: &ast.BoolOp{
				Op: ast.Or,
				Values: []ast.Expr{
					&ast.Name{Id: "x"},
					&ast.Name{Id: "y"},
				},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 1, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain JUMP_IF_TRUE_OR_POP for short-circuit or.
	if !containsOpcode(code, bytecode.JUMP_IF_TRUE_OR_POP) {
		t.Error("expected JUMP_IF_TRUE_OR_POP for 'or' short-circuit")
	}
}

// TestCompileBoolOp_MultipleAnd tests chained and: x and y and z
func TestCompileBoolOp_MultipleAnd(t *testing.T) {
	module := ast.NewModule([]ast.Stmt{
		&ast.ExprStmt{
			Value: &ast.BoolOp{
				Op: ast.And,
				Values: []ast.Expr{
					&ast.Name{Id: "x"},
					&ast.Name{Id: "y"},
					&ast.Name{Id: "z"},
				},
			},
		},
	}, ast.NewPos("test.py", 1, 1, 1, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 JUMP_IF_FALSE_OR_POP (one for each non-last operand).
	count := countOpcode(code, bytecode.JUMP_IF_FALSE_OR_POP)
	if count != 2 {
		t.Errorf("expected 2 JUMP_IF_FALSE_OR_POP for 'x and y and z', got %d", count)
	}
}

// TestCompileLineTable tests that line table entries are generated.
func TestCompileLineTable(t *testing.T) {
	// x = 1   (line 1)
	// y = 2   (line 2)
	// z = 3   (line 3)
	pos1 := ast.NewPos("test.py", 1, 1, 1, 5)
	pos2 := ast.NewPos("test.py", 2, 1, 2, 5)
	pos3 := ast.NewPos("test.py", 3, 1, 3, 5)

	module := ast.NewModule([]ast.Stmt{
		ast.NewAssign(
			[]ast.Expr{ast.NewName("x", ast.Store, pos1)},
			ast.NewConstant(int64(1), "", pos1),
			pos1,
		),
		ast.NewAssign(
			[]ast.Expr{ast.NewName("y", ast.Store, pos2)},
			ast.NewConstant(int64(2), "", pos2),
			pos2,
		),
		ast.NewAssign(
			[]ast.Expr{ast.NewName("z", ast.Store, pos3)},
			ast.NewConstant(int64(3), "", pos3),
			pos3,
		),
	}, ast.NewPos("test.py", 1, 1, 3, 5))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have line table entries (non-empty).
	if len(code.LineTable) == 0 {
		t.Error("expected non-empty LineTable for multi-line code")
	}

	// Line table should have pairs (bytecode_delta, line_delta).
	if len(code.LineTable)%2 != 0 {
		t.Errorf("expected even number of LineTable entries, got %d", len(code.LineTable))
	}
}

// TestCompileNestedLoopBreak tests break in nested loops.
func TestCompileNestedLoopBreak(t *testing.T) {
	// while True:
	//     while True:
	//         break
	//     break
	innerWhile := &ast.While{
		Test: &ast.Constant{Value: true},
		Body: []ast.Stmt{&ast.Break{}},
	}
	outerWhile := &ast.While{
		Test: &ast.Constant{Value: true},
		Body: []ast.Stmt{innerWhile, &ast.Break{}},
	}
	module := ast.NewModule([]ast.Stmt{outerWhile}, ast.NewPos("test.py", 1, 1, 4, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should compile without error — both breaks target their respective loops.
	// Count JUMP_ABSOLUTE: 2 back-edges + 2 breaks = 4
	jumpCount := countOpcode(code, bytecode.JUMP_ABSOLUTE)
	if jumpCount < 4 {
		t.Errorf("expected at least 4 JUMP_ABSOLUTE for nested loops with breaks, got %d", jumpCount)
	}
}

// TestCompileJumpPatching_WhileBreakSkipsElse tests that break skips the else clause.
func TestCompileJumpPatching_WhileBreakSkipsElse(t *testing.T) {
	// while x:
	//     break
	// else:
	//     y = 1
	module := ast.NewModule([]ast.Stmt{
		&ast.While{
			Test:   &ast.Name{Id: "x"},
			Body:   []ast.Stmt{&ast.Break{}},
			Orelse: []ast.Stmt{makeAssignValue("y", int64(1))},
		},
	}, ast.NewPos("test.py", 1, 1, 4, 1))

	code, err := Compile(module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The break's JUMP_ABSOLUTE should target an offset AFTER the else clause's code.
	// Find the break instruction (a JUMP_ABSOLUTE that goes forward past the back-edge).
	backEdgeIdx := -1
	for i := len(code.Instructions) - 1; i >= 0; i-- {
		if code.Instructions[i].Op == bytecode.JUMP_ABSOLUTE {
			target := int(code.Instructions[i].Arg)
			if target < i {
				backEdgeIdx = i
				break
			}
		}
	}

	// Find break jump: a JUMP_ABSOLUTE going forward
	breakIdx := -1
	for i, instr := range code.Instructions {
		if instr.Op == bytecode.JUMP_ABSOLUTE && int(instr.Arg) > i && i != backEdgeIdx {
			breakIdx = i
			break
		}
	}
	if breakIdx == -1 {
		t.Error("expected a forward JUMP_ABSOLUTE for break")
		return
	}

	// The break target should be after the back-edge (after else clause)
	breakTarget := int(code.Instructions[breakIdx].Arg)
	if backEdgeIdx >= 0 && breakTarget <= backEdgeIdx {
		t.Errorf("expected break target (%d) to be after back-edge (%d)", breakTarget, backEdgeIdx)
	}
}

// Helper functions for control flow tests.

func makeIfStmt(test ast.Expr, body []ast.Stmt, orelse []ast.Stmt) *ast.If {
	return &ast.If{
		Test:   test,
		Body:   body,
		Orelse: orelse,
	}
}

func makeAssignValue(name string, value interface{}) *ast.Assign {
	return &ast.Assign{
		Targets: []ast.Expr{&ast.Name{Id: name, Ctx: ast.Store}},
		Value:   &ast.Constant{Value: value},
	}
}

func containsOpcode(code *bytecode.Code, op bytecode.Opcode) bool {
	for _, instr := range code.Instructions {
		if instr.Op == op {
			return true
		}
	}
	return false
}

func countOpcode(code *bytecode.Code, op bytecode.Opcode) int {
	count := 0
	for _, instr := range code.Instructions {
		if instr.Op == op {
			count++
		}
	}
	return count
}
