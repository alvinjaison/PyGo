package parser

import (
	"errors"
	"testing"

	"github.com/alvinjaison/pygo/internal/ast"
)

func TestParseIntegerLiteral(t *testing.T) {
	mod, err := Parse("42", "<test>")
	if err != nil {
		t.Fatal(err)
	}
	if len(mod.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(mod.Body))
	}
	stmt, ok := mod.Body[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("expected ExprStmt, got %T", mod.Body[0])
	}
	c, ok := stmt.Value.(*ast.Constant)
	if !ok {
		t.Fatalf("expected Constant, got %T", stmt.Value)
	}
	if c.Value != int64(42) {
		t.Fatalf("expected 42, got %v", c.Value)
	}
}

func TestParseArithmetic(t *testing.T) {
	mod, err := Parse("2 + 3 * 4", "<test>")
	if err != nil {
		t.Fatal(err)
	}
	stmt := mod.Body[0].(*ast.ExprStmt)
	// Should be BinOp(2, Add, BinOp(3, Mult, 4))
	binop, ok := stmt.Value.(*ast.BinOp)
	if !ok {
		t.Fatalf("expected BinOp, got %T", stmt.Value)
	}
	if binop.Op != ast.Add {
		t.Fatalf("expected Add, got %s", binop.Op)
	}
	right, ok := binop.Right.(*ast.BinOp)
	if !ok {
		t.Fatalf("expected right to be BinOp, got %T", binop.Right)
	}
	if right.Op != ast.Mult {
		t.Fatalf("expected Mult, got %s", right.Op)
	}
}

func TestParsePower(t *testing.T) {
	// 2 ** 3 ** 2 should be right-associative: 2 ** (3 ** 2)
	mod, err := Parse("2 ** 3 ** 2", "<test>")
	if err != nil {
		t.Fatal(err)
	}
	stmt := mod.Body[0].(*ast.ExprStmt)
	binop := stmt.Value.(*ast.BinOp)
	if binop.Op != ast.Pow {
		t.Fatalf("expected Pow, got %s", binop.Op)
	}
	// Right should also be Pow (right-associative)
	right, ok := binop.Right.(*ast.BinOp)
	if !ok {
		t.Fatalf("expected right to be BinOp, got %T", binop.Right)
	}
	if right.Op != ast.Pow {
		t.Fatalf("expected right Pow, got %s", right.Op)
	}
}

func TestParseUnary(t *testing.T) {
	mod, err := Parse("-x", "<test>")
	if err != nil {
		t.Fatal(err)
	}
	stmt := mod.Body[0].(*ast.ExprStmt)
	unary, ok := stmt.Value.(*ast.UnaryOp)
	if !ok {
		t.Fatalf("expected UnaryOp, got %T", stmt.Value)
	}
	if unary.Op != ast.USub {
		t.Fatalf("expected USub, got %d", unary.Op)
	}
}

func TestParseBoolOp(t *testing.T) {
	mod, err := Parse("a and b or c", "<test>")
	if err != nil {
		t.Fatal(err)
	}
	stmt := mod.Body[0].(*ast.ExprStmt)
	// Should be BoolOp(Or, [BoolOp(And, [a, b]), c])
	boolop, ok := stmt.Value.(*ast.BoolOp)
	if !ok {
		t.Fatalf("expected BoolOp, got %T", stmt.Value)
	}
	if boolop.Op != ast.Or {
		t.Fatalf("expected Or, got %d", boolop.Op)
	}
}

func TestParseComparison(t *testing.T) {
	mod, err := Parse("a < b < c", "<test>")
	if err != nil {
		t.Fatal(err)
	}
	stmt := mod.Body[0].(*ast.ExprStmt)
	cmp, ok := stmt.Value.(*ast.Compare)
	if !ok {
		t.Fatalf("expected Compare, got %T", stmt.Value)
	}
	if len(cmp.Ops) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(cmp.Ops))
	}
	if cmp.Ops[0] != ast.Lt || cmp.Ops[1] != ast.Lt {
		t.Fatalf("expected Lt, Lt, got %v", cmp.Ops)
	}
}

func TestParseFunctionCall(t *testing.T) {
	mod, err := Parse("f(1, 2, 3)", "<test>")
	if err != nil {
		t.Fatal(err)
	}
	stmt := mod.Body[0].(*ast.ExprStmt)
	call, ok := stmt.Value.(*ast.Call)
	if !ok {
		t.Fatalf("expected Call, got %T", stmt.Value)
	}
	if len(call.Args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(call.Args))
	}
}

func TestParseAttributeAccess(t *testing.T) {
	mod, err := Parse("obj.attr.method", "<test>")
	if err != nil {
		t.Fatal(err)
	}
	stmt := mod.Body[0].(*ast.ExprStmt)
	attr, ok := stmt.Value.(*ast.Attribute)
	if !ok {
		t.Fatalf("expected Attribute, got %T", stmt.Value)
	}
	if attr.Attr != "method" {
		t.Fatalf("expected 'method', got %s", attr.Attr)
	}
}

func TestParseSubscript(t *testing.T) {
	mod, err := Parse("x[0]", "<test>")
	if err != nil {
		t.Fatal(err)
	}
	stmt := mod.Body[0].(*ast.ExprStmt)
	sub, ok := stmt.Value.(*ast.Subscript)
	if !ok {
		t.Fatalf("expected Subscript, got %T", stmt.Value)
	}
	_ = sub
}

func TestParseListLiteral(t *testing.T) {
	mod, err := Parse("[1, 2, 3]", "<test>")
	if err != nil {
		t.Fatal(err)
	}
	stmt := mod.Body[0].(*ast.ExprStmt)
	list, ok := stmt.Value.(*ast.List)
	if !ok {
		t.Fatalf("expected List, got %T", stmt.Value)
	}
	if len(list.Elts) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(list.Elts))
	}
}

func TestParseDictLiteral(t *testing.T) {
	mod, err := Parse(`{"a": 1, "b": 2}`, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	stmt := mod.Body[0].(*ast.ExprStmt)
	dict, ok := stmt.Value.(*ast.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", stmt.Value)
	}
	if len(dict.Keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(dict.Keys))
	}
}

func TestParseTupleLiteral(t *testing.T) {
	mod, err := Parse("(1, 2, 3)", "<test>")
	if err != nil {
		t.Fatal(err)
	}
	stmt := mod.Body[0].(*ast.ExprStmt)
	tup, ok := stmt.Value.(*ast.Tuple)
	if !ok {
		t.Fatalf("expected Tuple, got %T", stmt.Value)
	}
	if len(tup.Elts) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(tup.Elts))
	}
}

func TestParseTernary(t *testing.T) {
	mod, err := Parse("x if True else y", "<test>")
	if err != nil {
		t.Fatal(err)
	}
	stmt := mod.Body[0].(*ast.ExprStmt)
	ifexp, ok := stmt.Value.(*ast.IfExp)
	if !ok {
		t.Fatalf("expected IfExp, got %T", stmt.Value)
	}
	_ = ifexp
}

func TestParseNotIn(t *testing.T) {
	mod, err := Parse("x not in y", "<test>")
	if err != nil {
		t.Fatal(err)
	}
	stmt := mod.Body[0].(*ast.ExprStmt)
	cmp, ok := stmt.Value.(*ast.Compare)
	if !ok {
		t.Fatalf("expected Compare, got %T", stmt.Value)
	}
	if cmp.Ops[0] != ast.NotIn {
		t.Fatalf("expected NotIn, got %v", cmp.Ops[0])
	}
}

func TestParseIsNot(t *testing.T) {
	mod, err := Parse("x is not None", "<test>")
	if err != nil {
		t.Fatal(err)
	}
	stmt := mod.Body[0].(*ast.ExprStmt)
	cmp, ok := stmt.Value.(*ast.Compare)
	if !ok {
		t.Fatalf("expected Compare, got %T", stmt.Value)
	}
	if cmp.Ops[0] != ast.IsNot {
		t.Fatalf("expected IsNot, got %v", cmp.Ops[0])
	}
}

// ============================================================
// Statement-level tests (Task 3.4)
// ============================================================

// --- Function Definition ---

func TestParseFunctionDef(t *testing.T) {
	src := "def add(a, b):\n    return a + b\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	if len(mod.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(mod.Body))
	}
	fn, ok := mod.Body[0].(*ast.FunctionDef)
	if !ok {
		t.Fatalf("expected FunctionDef, got %T", mod.Body[0])
	}
	if fn.Name != "add" {
		t.Fatalf("expected function name 'add', got %q", fn.Name)
	}
	if len(fn.Args.Args) != 2 {
		t.Fatalf("expected 2 params, got %d", len(fn.Args.Args))
	}
	if fn.Args.Args[0].Arg != "a" || fn.Args.Args[1].Arg != "b" {
		t.Fatalf("expected params (a, b), got (%s, %s)", fn.Args.Args[0].Arg, fn.Args.Args[1].Arg)
	}
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 body stmt, got %d", len(fn.Body))
	}
	ret, ok := fn.Body[0].(*ast.Return)
	if !ok {
		t.Fatalf("expected Return in body, got %T", fn.Body[0])
	}
	if ret.Value == nil {
		t.Fatal("expected return value, got nil")
	}
}

func TestParseFunctionDefWithDefaults(t *testing.T) {
	src := "def greet(name, greeting=\"hello\"):\n    return greeting\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	fn := mod.Body[0].(*ast.FunctionDef)
	if len(fn.Args.Args) != 2 {
		t.Fatalf("expected 2 params, got %d", len(fn.Args.Args))
	}
	if len(fn.Args.Defaults) != 1 {
		t.Fatalf("expected 1 default, got %d", len(fn.Args.Defaults))
	}
	def, ok := fn.Args.Defaults[0].(*ast.Constant)
	if !ok {
		t.Fatalf("expected Constant default, got %T", fn.Args.Defaults[0])
	}
	if def.Value != "hello" {
		t.Fatalf("expected default value \"hello\", got %v", def.Value)
	}
}

// --- If/Elif/Else ---

func TestParseIfStmt(t *testing.T) {
	src := "if x > 0:\n    y = 1\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	ifStmt, ok := mod.Body[0].(*ast.If)
	if !ok {
		t.Fatalf("expected If, got %T", mod.Body[0])
	}
	// Test should be a Compare node
	_, ok = ifStmt.Test.(*ast.Compare)
	if !ok {
		t.Fatalf("expected Compare test, got %T", ifStmt.Test)
	}
	if len(ifStmt.Body) != 1 {
		t.Fatalf("expected 1 body stmt, got %d", len(ifStmt.Body))
	}
	if len(ifStmt.Orelse) != 0 {
		t.Fatalf("expected no orelse, got %d", len(ifStmt.Orelse))
	}
}

func TestParseIfElifElse(t *testing.T) {
	src := "if x > 0:\n    y = 1\nelif x == 0:\n    y = 0\nelse:\n    y = -1\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	ifStmt, ok := mod.Body[0].(*ast.If)
	if !ok {
		t.Fatalf("expected If, got %T", mod.Body[0])
	}
	if len(ifStmt.Body) != 1 {
		t.Fatalf("expected 1 body stmt, got %d", len(ifStmt.Body))
	}
	// Orelse should contain a single nested If (the elif)
	if len(ifStmt.Orelse) != 1 {
		t.Fatalf("expected 1 orelse stmt (elif), got %d", len(ifStmt.Orelse))
	}
	elif, ok := ifStmt.Orelse[0].(*ast.If)
	if !ok {
		t.Fatalf("expected elif to be If, got %T", ifStmt.Orelse[0])
	}
	// The elif should have an else body
	if len(elif.Orelse) != 1 {
		t.Fatalf("expected elif to have 1 orelse (else body), got %d", len(elif.Orelse))
	}
	// The else body should be an Assign
	_, ok = elif.Orelse[0].(*ast.Assign)
	if !ok {
		t.Fatalf("expected else body to be Assign, got %T", elif.Orelse[0])
	}
}

// --- While loop ---

func TestParseWhileStmt(t *testing.T) {
	src := "while x > 0:\n    x = x - 1\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	whileStmt, ok := mod.Body[0].(*ast.While)
	if !ok {
		t.Fatalf("expected While, got %T", mod.Body[0])
	}
	_, ok = whileStmt.Test.(*ast.Compare)
	if !ok {
		t.Fatalf("expected Compare test, got %T", whileStmt.Test)
	}
	if len(whileStmt.Body) != 1 {
		t.Fatalf("expected 1 body stmt, got %d", len(whileStmt.Body))
	}
}

// --- For loop ---

func TestParseForStmt(t *testing.T) {
	src := "for i in items:\n    print(i)\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	forStmt, ok := mod.Body[0].(*ast.For)
	if !ok {
		t.Fatalf("expected For, got %T", mod.Body[0])
	}
	target, ok := forStmt.Target.(*ast.Name)
	if !ok {
		t.Fatalf("expected Name target, got %T", forStmt.Target)
	}
	if target.Id != "i" {
		t.Fatalf("expected target 'i', got %q", target.Id)
	}
	iter, ok := forStmt.Iter.(*ast.Name)
	if !ok {
		t.Fatalf("expected Name iter, got %T", forStmt.Iter)
	}
	if iter.Id != "items" {
		t.Fatalf("expected iter 'items', got %q", iter.Id)
	}
	if len(forStmt.Body) != 1 {
		t.Fatalf("expected 1 body stmt, got %d", len(forStmt.Body))
	}
}

// --- Try/Except/Finally ---

func TestParseTryExcept(t *testing.T) {
	src := "try:\n    x = 1\nexcept ValueError:\n    x = 0\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	tryStmt, ok := mod.Body[0].(*ast.Try)
	if !ok {
		t.Fatalf("expected Try, got %T", mod.Body[0])
	}
	if len(tryStmt.Body) != 1 {
		t.Fatalf("expected 1 body stmt, got %d", len(tryStmt.Body))
	}
	if len(tryStmt.Handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(tryStmt.Handlers))
	}
	handler := tryStmt.Handlers[0]
	typeName, ok := handler.Type.(*ast.Name)
	if !ok {
		t.Fatalf("expected handler type Name, got %T", handler.Type)
	}
	if typeName.Id != "ValueError" {
		t.Fatalf("expected 'ValueError', got %q", typeName.Id)
	}
	if len(handler.Body) != 1 {
		t.Fatalf("expected 1 handler body stmt, got %d", len(handler.Body))
	}
}

func TestParseTryExceptAs(t *testing.T) {
	src := "try:\n    x = 1\nexcept ValueError as e:\n    print(e)\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	tryStmt := mod.Body[0].(*ast.Try)
	handler := tryStmt.Handlers[0]
	if handler.Name != "e" {
		t.Fatalf("expected handler name 'e', got %q", handler.Name)
	}
}

func TestParseTryExceptElseFinally(t *testing.T) {
	src := "try:\n    x = 1\nexcept TypeError:\n    x = 0\nelse:\n    x = 2\nfinally:\n    cleanup()\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	tryStmt := mod.Body[0].(*ast.Try)
	if len(tryStmt.Handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(tryStmt.Handlers))
	}
	if len(tryStmt.Orelse) != 1 {
		t.Fatalf("expected 1 else stmt, got %d", len(tryStmt.Orelse))
	}
	if len(tryStmt.Finalbody) != 1 {
		t.Fatalf("expected 1 finally stmt, got %d", len(tryStmt.Finalbody))
	}
}

func TestParseTryFinally(t *testing.T) {
	src := "try:\n    x = 1\nfinally:\n    cleanup()\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	tryStmt := mod.Body[0].(*ast.Try)
	if len(tryStmt.Handlers) != 0 {
		t.Fatalf("expected 0 handlers, got %d", len(tryStmt.Handlers))
	}
	if len(tryStmt.Finalbody) != 1 {
		t.Fatalf("expected 1 finally stmt, got %d", len(tryStmt.Finalbody))
	}
}

// --- Assignment ---

func TestParseAssignment(t *testing.T) {
	src := "x = 42\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	assign, ok := mod.Body[0].(*ast.Assign)
	if !ok {
		t.Fatalf("expected Assign, got %T", mod.Body[0])
	}
	if len(assign.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(assign.Targets))
	}
	target, ok := assign.Targets[0].(*ast.Name)
	if !ok {
		t.Fatalf("expected Name target, got %T", assign.Targets[0])
	}
	if target.Id != "x" {
		t.Fatalf("expected target 'x', got %q", target.Id)
	}
}

func TestParseChainedAssignment(t *testing.T) {
	src := "a = b = 1\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	assign, ok := mod.Body[0].(*ast.Assign)
	if !ok {
		t.Fatalf("expected Assign, got %T", mod.Body[0])
	}
	if len(assign.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(assign.Targets))
	}
	a, ok := assign.Targets[0].(*ast.Name)
	if !ok {
		t.Fatalf("expected first target Name, got %T", assign.Targets[0])
	}
	if a.Id != "a" {
		t.Fatalf("expected first target 'a', got %q", a.Id)
	}
	b, ok := assign.Targets[1].(*ast.Name)
	if !ok {
		t.Fatalf("expected second target Name, got %T", assign.Targets[1])
	}
	if b.Id != "b" {
		t.Fatalf("expected second target 'b', got %q", b.Id)
	}
	val, ok := assign.Value.(*ast.Constant)
	if !ok {
		t.Fatalf("expected Constant value, got %T", assign.Value)
	}
	if val.Value != int64(1) {
		t.Fatalf("expected value 1, got %v", val.Value)
	}
}

// --- Augmented Assignment ---

func TestParseAugAssign(t *testing.T) {
	src := "x += 1\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	aug, ok := mod.Body[0].(*ast.AugAssign)
	if !ok {
		t.Fatalf("expected AugAssign, got %T", mod.Body[0])
	}
	if aug.Op != ast.Add {
		t.Fatalf("expected Add op, got %s", aug.Op)
	}
	target, ok := aug.Target.(*ast.Name)
	if !ok {
		t.Fatalf("expected Name target, got %T", aug.Target)
	}
	if target.Id != "x" {
		t.Fatalf("expected target 'x', got %q", target.Id)
	}
	val, ok := aug.Value.(*ast.Constant)
	if !ok {
		t.Fatalf("expected Constant value, got %T", aug.Value)
	}
	if val.Value != int64(1) {
		t.Fatalf("expected value 1, got %v", val.Value)
	}
}

func TestParseAugAssignVariants(t *testing.T) {
	tests := []struct {
		src string
		op  ast.Operator
	}{
		{"x -= 1\n", ast.Sub},
		{"x *= 2\n", ast.Mult},
		{"x /= 3\n", ast.Div},
		{"x //= 4\n", ast.FloorDiv},
		{"x %= 5\n", ast.Mod},
		{"x **= 6\n", ast.Pow},
	}
	for _, tt := range tests {
		mod, err := Parse(tt.src, "<test>")
		if err != nil {
			t.Fatalf("parsing %q: %v", tt.src, err)
		}
		aug, ok := mod.Body[0].(*ast.AugAssign)
		if !ok {
			t.Fatalf("parsing %q: expected AugAssign, got %T", tt.src, mod.Body[0])
		}
		if aug.Op != tt.op {
			t.Fatalf("parsing %q: expected op %s, got %s", tt.src, tt.op, aug.Op)
		}
	}
}

// --- Import ---

func TestParseImport(t *testing.T) {
	src := "import os\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	imp, ok := mod.Body[0].(*ast.Import)
	if !ok {
		t.Fatalf("expected Import, got %T", mod.Body[0])
	}
	if len(imp.Names) != 1 {
		t.Fatalf("expected 1 name, got %d", len(imp.Names))
	}
	if imp.Names[0].Name != "os" {
		t.Fatalf("expected 'os', got %q", imp.Names[0].Name)
	}
	if imp.Names[0].AsName != "" {
		t.Fatalf("expected no alias, got %q", imp.Names[0].AsName)
	}
}

func TestParseImportAs(t *testing.T) {
	src := "import numpy as np\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	imp := mod.Body[0].(*ast.Import)
	if imp.Names[0].Name != "numpy" {
		t.Fatalf("expected 'numpy', got %q", imp.Names[0].Name)
	}
	if imp.Names[0].AsName != "np" {
		t.Fatalf("expected alias 'np', got %q", imp.Names[0].AsName)
	}
}

func TestParseFromImport(t *testing.T) {
	src := "from os import path\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	imp, ok := mod.Body[0].(*ast.ImportFrom)
	if !ok {
		t.Fatalf("expected ImportFrom, got %T", mod.Body[0])
	}
	if imp.Module != "os" {
		t.Fatalf("expected module 'os', got %q", imp.Module)
	}
	if len(imp.Names) != 1 {
		t.Fatalf("expected 1 name, got %d", len(imp.Names))
	}
	if imp.Names[0].Name != "path" {
		t.Fatalf("expected 'path', got %q", imp.Names[0].Name)
	}
}

func TestParseFromImportMultiple(t *testing.T) {
	src := "from os import path, getcwd\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	imp := mod.Body[0].(*ast.ImportFrom)
	if len(imp.Names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(imp.Names))
	}
	if imp.Names[0].Name != "path" || imp.Names[1].Name != "getcwd" {
		t.Fatalf("expected (path, getcwd), got (%s, %s)", imp.Names[0].Name, imp.Names[1].Name)
	}
}

// --- Del, Global, Pass, Break, Continue, Return, Raise ---

func TestParseDelStmt(t *testing.T) {
	src := "del x\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	del, ok := mod.Body[0].(*ast.Delete)
	if !ok {
		t.Fatalf("expected Delete, got %T", mod.Body[0])
	}
	if len(del.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(del.Targets))
	}
	name, ok := del.Targets[0].(*ast.Name)
	if !ok {
		t.Fatalf("expected Name target, got %T", del.Targets[0])
	}
	if name.Id != "x" {
		t.Fatalf("expected 'x', got %q", name.Id)
	}
}

func TestParseGlobalStmt(t *testing.T) {
	src := "global x, y\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	glob, ok := mod.Body[0].(*ast.Global)
	if !ok {
		t.Fatalf("expected Global, got %T", mod.Body[0])
	}
	if len(glob.Names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(glob.Names))
	}
	if glob.Names[0] != "x" || glob.Names[1] != "y" {
		t.Fatalf("expected [x, y], got %v", glob.Names)
	}
}

func TestParseNonlocalStmt(t *testing.T) {
	src := "nonlocal x, y\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	nl, ok := mod.Body[0].(*ast.Nonlocal)
	if !ok {
		t.Fatalf("expected Nonlocal, got %T", mod.Body[0])
	}
	if len(nl.Names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(nl.Names))
	}
	if nl.Names[0] != "x" || nl.Names[1] != "y" {
		t.Fatalf("expected [x, y], got %v", nl.Names)
	}
}

func TestParseNonlocalSingleName(t *testing.T) {
	src := "nonlocal count\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	nl, ok := mod.Body[0].(*ast.Nonlocal)
	if !ok {
		t.Fatalf("expected Nonlocal, got %T", mod.Body[0])
	}
	if len(nl.Names) != 1 || nl.Names[0] != "count" {
		t.Fatalf("expected [count], got %v", nl.Names)
	}
}

func TestParsePassStmt(t *testing.T) {
	src := "pass\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	_, ok := mod.Body[0].(*ast.Pass)
	if !ok {
		t.Fatalf("expected Pass, got %T", mod.Body[0])
	}
}

func TestParseBreakContinue(t *testing.T) {
	src := "while True:\n    break\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	whileStmt := mod.Body[0].(*ast.While)
	_, ok := whileStmt.Body[0].(*ast.Break)
	if !ok {
		t.Fatalf("expected Break, got %T", whileStmt.Body[0])
	}

	src2 := "while True:\n    continue\n"
	mod2, err := Parse(src2, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	whileStmt2 := mod2.Body[0].(*ast.While)
	_, ok = whileStmt2.Body[0].(*ast.Continue)
	if !ok {
		t.Fatalf("expected Continue, got %T", whileStmt2.Body[0])
	}
}

func TestParseReturnBare(t *testing.T) {
	src := "def f():\n    return\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	fn := mod.Body[0].(*ast.FunctionDef)
	ret, ok := fn.Body[0].(*ast.Return)
	if !ok {
		t.Fatalf("expected Return, got %T", fn.Body[0])
	}
	if ret.Value != nil {
		t.Fatalf("expected bare return (nil value), got %v", ret.Value)
	}
}

func TestParseRaiseStmt(t *testing.T) {
	src := "raise ValueError\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	raise, ok := mod.Body[0].(*ast.Raise)
	if !ok {
		t.Fatalf("expected Raise, got %T", mod.Body[0])
	}
	exc, ok := raise.Exc.(*ast.Name)
	if !ok {
		t.Fatalf("expected Name exc, got %T", raise.Exc)
	}
	if exc.Id != "ValueError" {
		t.Fatalf("expected 'ValueError', got %q", exc.Id)
	}
}

func TestParseRaiseBare(t *testing.T) {
	// bare raise (re-raise) inside except block
	src := "try:\n    x = 1\nexcept:\n    raise\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	tryStmt := mod.Body[0].(*ast.Try)
	raise, ok := tryStmt.Handlers[0].Body[0].(*ast.Raise)
	if !ok {
		t.Fatalf("expected Raise, got %T", tryStmt.Handlers[0].Body[0])
	}
	if raise.Exc != nil {
		t.Fatalf("expected bare raise (nil exc), got %v", raise.Exc)
	}
}

// --- SyntaxError reporting ---

func TestParseSyntaxError(t *testing.T) {
	src := "def\n"
	_, err := Parse(src, "test.py")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	synErr := &SyntaxError{}
	if !errors.As(err, &synErr) {
		t.Fatalf("expected *SyntaxError, got %T: %v", err, err)
	}
	if synErr.Filename != "test.py" {
		t.Fatalf("expected filename 'test.py', got %q", synErr.Filename)
	}
	if synErr.Line != 1 {
		t.Fatalf("expected line 1, got %d", synErr.Line)
	}
}

func TestParseSyntaxErrorPosition(t *testing.T) {
	// Missing colon after if condition
	src := "if x > 0\n    y = 1\n"
	_, err := Parse(src, "test.py")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	synErr := &SyntaxError{}
	if !errors.As(err, &synErr) {
		t.Fatalf("expected *SyntaxError, got %T: %v", err, err)
	}
	if synErr.Line < 1 {
		t.Fatalf("expected positive line number, got %d", synErr.Line)
	}
	if synErr.Col < 1 {
		t.Fatalf("expected positive column, got %d", synErr.Col)
	}
}

// --- Operator precedence (verification of expression in statement context) ---

func TestParseOperatorPrecedenceInAssignment(t *testing.T) {
	// 2 + 3 * 4 should produce BinOp(2, Add, BinOp(3, Mult, 4))
	src := "result = 2 + 3 * 4\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	assign := mod.Body[0].(*ast.Assign)
	binop, ok := assign.Value.(*ast.BinOp)
	if !ok {
		t.Fatalf("expected BinOp value, got %T", assign.Value)
	}
	if binop.Op != ast.Add {
		t.Fatalf("expected outer Add, got %s", binop.Op)
	}
	right, ok := binop.Right.(*ast.BinOp)
	if !ok {
		t.Fatalf("expected right BinOp, got %T", binop.Right)
	}
	if right.Op != ast.Mult {
		t.Fatalf("expected inner Mult, got %s", right.Op)
	}
}

// --- Collection literals in statements ---

func TestParseListLiteralAssignment(t *testing.T) {
	src := "items = [1, 2, 3]\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	assign := mod.Body[0].(*ast.Assign)
	list, ok := assign.Value.(*ast.List)
	if !ok {
		t.Fatalf("expected List, got %T", assign.Value)
	}
	if len(list.Elts) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(list.Elts))
	}
}

func TestParseDictLiteralAssignment(t *testing.T) {
	src := "d = {\"a\": 1, \"b\": 2}\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	assign := mod.Body[0].(*ast.Assign)
	dict, ok := assign.Value.(*ast.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", assign.Value)
	}
	if len(dict.Keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(dict.Keys))
	}
}

func TestParseTupleLiteralAssignment(t *testing.T) {
	src := "t = (1, 2, 3)\n"
	mod, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	assign := mod.Body[0].(*ast.Assign)
	tup, ok := assign.Value.(*ast.Tuple)
	if !ok {
		t.Fatalf("expected Tuple, got %T", assign.Value)
	}
	if len(tup.Elts) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(tup.Elts))
	}
}
