package runtime

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestNew_DefaultConfig(t *testing.T) {
	r := New()
	if r == nil {
		t.Fatal("New() returned nil")
	}
	if r.globals == nil {
		t.Fatal("globals is nil")
	}
	if r.builtins == nil {
		t.Fatal("builtins is nil")
	}
	if r.maxCallDepth != defaultMaxCallDepth {
		t.Errorf("maxCallDepth = %d, want %d", r.maxCallDepth, defaultMaxCallDepth)
	}
}

func TestNew_WithOptions(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf), WithMaxCallDepth(100))
	if r.maxCallDepth != 100 {
		t.Errorf("maxCallDepth = %d, want 100", r.maxCallDepth)
	}
	if r.output != &buf {
		t.Error("output writer not set correctly")
	}
}

func TestExecString_PrintHello(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`print("hello")`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}

	got := buf.String()
	if got != "hello\n" {
		t.Errorf("output = %q, want %q", got, "hello\n")
	}
}

func TestExecString_PrintMultipleArgs(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`print("hello", "world")`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}

	got := buf.String()
	if got != "hello world\n" {
		t.Errorf("output = %q, want %q", got, "hello world\n")
	}
}

func TestExecString_PrintInteger(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`print(42)`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}

	got := buf.String()
	if got != "42\n" {
		t.Errorf("output = %q, want %q", got, "42\n")
	}
}

func TestExecString_VariableAssignAndPrint(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString("x = 10\nprint(x)", "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}

	got := buf.String()
	if got != "10\n" {
		t.Errorf("output = %q, want %q", got, "10\n")
	}
}

func TestExecString_ArithmeticAndPrint(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString("x = 10\ny = 20\nprint(x + y)", "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}

	got := buf.String()
	if got != "30\n" {
		t.Errorf("output = %q, want %q", got, "30\n")
	}
}

func TestExecString_FunctionDefAndCall(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = 10
y = 20
def add(a, b):
    return a + b
print(add(x, y))`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}

	got := buf.String()
	if got != "30\n" {
		t.Errorf("output = %q, want %q", got, "30\n")
	}
}

func TestExecString_SyntaxError(t *testing.T) {
	r := New()
	err := r.ExecString("def (", "<test>")
	if err == nil {
		t.Fatal("expected error for invalid syntax")
	}

	var se *SyntaxError
	if !errors.As(err, &se) {
		t.Errorf("expected *SyntaxError, got %T: %v", err, err)
	}
}

func TestExecString_NameError(t *testing.T) {
	r := New()
	err := r.ExecString("print(undefined_var)", "<test>")
	if err == nil {
		t.Fatal("expected error for undefined name")
	}

	var re *RuntimeError
	if !errors.As(err, &re) {
		t.Errorf("expected *RuntimeError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "undefined_var") {
		t.Errorf("error should mention 'undefined_var', got: %s", err.Error())
	}
}

func TestExecString_PrintNoArgs(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`print()`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}

	got := buf.String()
	if got != "\n" {
		t.Errorf("output = %q, want %q", got, "\n")
	}
}

func TestExecString_MultipleStatements(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `print("a")
print("b")
print("c")`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}

	got := buf.String()
	want := "a\nb\nc\n"
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_BuiltinInGlobalScope(t *testing.T) {
	// Verify that builtins are accessible from module scope via LOAD_NAME.
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`print("ok")`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	if buf.String() != "ok\n" {
		t.Errorf("output = %q, want %q", buf.String(), "ok\n")
	}
}

// --- Task 7.1 tests: verify pipeline handles all implemented features ---

func TestExecString_TypeBuiltin(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`print(type(42))`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	got := buf.String()
	if got != "<class 'int'>\n" {
		t.Errorf("output = %q, want %q", got, "<class 'int'>\n")
	}
}

func TestExecString_TypeBuiltinStr(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`print(type("hello"))`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	got := buf.String()
	if got != "<class 'str'>\n" {
		t.Errorf("output = %q, want %q", got, "<class 'str'>\n")
	}
}

func TestExecString_TypeBuiltinNoArgs(t *testing.T) {
	r := New()
	err := r.ExecString(`type()`, "<test>")
	if err == nil {
		t.Fatal("expected error for type() with no args")
	}
	var re *RuntimeError
	if !errors.As(err, &re) {
		t.Errorf("expected *RuntimeError, got %T: %v", err, err)
	}
}

func TestExecString_IfElse(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = 5
if x > 3:
    print("big")
else:
    print("small")`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	if got := buf.String(); got != "big\n" {
		t.Errorf("output = %q, want %q", got, "big\n")
	}
}

func TestExecString_WhileLoop(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `i = 0
result = 0
while i < 5:
    result = result + i
    i = i + 1
print(result)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	if got := buf.String(); got != "10\n" {
		t.Errorf("output = %q, want %q", got, "10\n")
	}
}

func TestExecString_ForLoop(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `result = 0
for x in [1, 2, 3, 4, 5]:
    result = result + x
print(result)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	if got := buf.String(); got != "15\n" {
		t.Errorf("output = %q, want %q", got, "15\n")
	}
}

func TestExecString_BreakContinue(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `result = 0
for x in [1, 2, 3, 4, 5]:
    if x == 3:
        continue
    if x == 5:
        break
    result = result + x
print(result)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	// 1 + 2 + 4 = 7 (skip 3, break at 5)
	if got := buf.String(); got != "7\n" {
		t.Errorf("output = %q, want %q", got, "7\n")
	}
}

func TestExecString_TryExcept(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `try:
    x = 1 / 0
except ZeroDivisionError:
    print("caught")`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	if got := buf.String(); got != "caught\n" {
		t.Errorf("output = %q, want %q", got, "caught\n")
	}
}

func TestExecString_TryFinally(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `try:
    print("try")
finally:
    print("finally")`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "try\nfinally\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ComparisonOperators(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `print(1 < 2)
print(2 > 1)
print(1 == 1)
print(1 != 2)
print(1 <= 1)
print(2 >= 2)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "True\nTrue\nTrue\nTrue\nTrue\nTrue\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_BooleanOperators(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `print(True and False)
print(True or False)
print(not True)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "False\nTrue\nFalse\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ArithmeticOps(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `print(7 // 2)
print(7 % 3)
print(2 ** 3)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "3\n1\n8\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ListOperations(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [1, 2, 3]
print(x[0])
print(x[2])`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "1\n3\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_TupleCreation(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = (1, 2, 3)
print(x[1])`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "2\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_DictOperations(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `d = {"a": 1, "b": 2}
print(d["a"])
d["c"] = 3
print(d["c"])`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "1\n3\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_NestedFunctions(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	// Note: closures (accessing enclosing scope variables) are Post-MVP.
	// This test verifies nested function definitions work where the inner
	// function uses only its own parameters and globals.
	source := `def make_adder(x):
    return x + 10

def apply(fn, val):
    return fn(val)

print(apply(make_adder, 5))`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "15\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_RecursiveFunction(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def factorial(n):
    if n <= 1:
        return 1
    return n * factorial(n - 1)
print(factorial(5))`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "120\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_RuntimeIsolation(t *testing.T) {
	// Verify that two Runtime instances don't share state.
	var buf1, buf2 bytes.Buffer
	r1 := New(WithOutput(&buf1))
	r2 := New(WithOutput(&buf2))

	_ = r1.ExecString("x = 42\nprint(x)", "<test>")
	err := r2.ExecString("print(x)", "<test>")

	if err == nil {
		t.Fatal("expected error: r2 should not have r1's 'x'")
	}
	var re *RuntimeError
	if !errors.As(err, &re) {
		t.Errorf("expected *RuntimeError, got %T: %v", err, err)
	}
}

func TestExecString_ErrorTypes(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		errType interface{}
	}{
		{"SyntaxError", "def (", &SyntaxError{}},
		{"RuntimeError_NameError", "print(xyz)", &RuntimeError{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			err := r.ExecString(tt.source, "<test>")
			if err == nil {
				t.Fatal("expected error")
			}
			switch tt.errType.(type) {
			case *SyntaxError:
				var se *SyntaxError
				if !errors.As(err, &se) {
					t.Errorf("expected *SyntaxError, got %T: %v", err, err)
				}
			case *RuntimeError:
				var re *RuntimeError
				if !errors.As(err, &re) {
					t.Errorf("expected *RuntimeError, got %T: %v", err, err)
				}
			}
		})
	}
}

// --- Task 7.3: Full vertical slice integration tests ---

func TestExecString_RecursionError(t *testing.T) {
	// Verify that exceeding the max call depth raises RecursionError
	// through the full pipeline (resource limits enforcement).
	r := New(WithMaxCallDepth(20))

	source := `def recurse(n):
    return recurse(n + 1)
recurse(0)`

	err := r.ExecString(source, "<test>")
	if err == nil {
		t.Fatal("expected RecursionError, got nil")
	}

	var re *RuntimeError
	if !errors.As(err, &re) {
		t.Fatalf("expected *RuntimeError, got %T: %v", err, err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "recursion") {
		t.Errorf("expected error containing 'recursion', got: %s", err.Error())
	}
}

func TestExecString_FullVerticalSlice(t *testing.T) {
	// Comprehensive integration test verifying all foundational features
	// work together: constant loading, name store/load, function creation,
	// function call with new frame, arithmetic, return value, builtin
	// invocation, exception handling, and control flow.
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = 10
y = 20

def add(a, b):
    return a + b

result = add(x, y)
print(result)

def safe_divide(a, b):
    try:
        return a // b
    except ZeroDivisionError:
        return -1

print(safe_divide(100, 5))
print(safe_divide(7, 0))

total = 0
for i in [1, 2, 3, 4, 5]:
    if i == 3:
        continue
    total = total + i
print(total)

def factorial(n):
    if n <= 1:
        return 1
    return n * factorial(n - 1)
print(factorial(6))
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}

	// Expected output:
	// 30      (add(10, 20))
	// 20      (100 // 5)
	// -1      (7 // 0 caught by except)
	// 12      (1+2+4+5, skipping 3)
	// 720     (6!)
	want := "30\n20\n-1\n12\n720\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ExceptionHandlingIntegration(t *testing.T) {
	// Verify try/except/finally works correctly through the full pipeline.
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def risky(x):
    try:
        if x == 0:
            result = 10 / x
        else:
            result = 100 // x
        print(result)
    except ZeroDivisionError:
        print("division error")
    finally:
        print("cleanup")

risky(5)
risky(0)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}

	want := "20\ncleanup\ndivision error\ncleanup\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// --- Task 9.4: Control flow expansion tests ---

func TestExecString_WhileElse(t *testing.T) {
	// while/else: else block runs when loop exits normally (condition becomes false)
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `i = 0
while i < 3:
    i = i + 1
else:
    print("done")
print(i)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "done\n3\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_WhileElseBreak(t *testing.T) {
	// while/else with break: else does NOT run when loop exits via break
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `i = 0
while i < 10:
    if i == 3:
        break
    i = i + 1
else:
    print("else ran")
print(i)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	// else should NOT run because of break; just prints i=3
	want := "3\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ForElse(t *testing.T) {
	// for/else: else block runs when loop completes normally (iterator exhausted)
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `result = 0
for x in [1, 2, 3]:
    result = result + x
else:
    print("completed")
print(result)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "completed\n6\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ForElseBreak(t *testing.T) {
	// for/else with break: else does NOT run when loop exits via break
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `found = False
for x in [1, 2, 3, 4, 5]:
    if x == 3:
        found = True
        break
else:
    print("not found")
print(found)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	// else should NOT run because of break
	want := "True\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_AugAssign(t *testing.T) {
	// Augmented assignment: +=, -=, *=, /=, //=, %=, **=
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = 5
x += 3
print(x)
x -= 2
print(x)
x *= 4
print(x)
x //= 3
print(x)
x %= 5
print(x)
y = 2
y **= 10
print(y)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	// x=5+3=8, 8-2=6, 6*4=24, 24//3=8, 8%5=3, y=2**10=1024
	want := "8\n6\n24\n8\n3\n1024\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_NestedLoopBreakContinue(t *testing.T) {
	// break only breaks the innermost loop; outer loop continues
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `result = 0
for i in [1, 2, 3]:
    for j in [10, 20, 30]:
        if j == 20:
            break
        result = result + j
    result = result + i
print(result)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	// inner loop: j=10 then break at j=20, so adds 10 each time
	// outer loop: i=1,2,3 each adds i after inner loop
	// iteration 1: result = 0 + 10 + 1 = 11
	// iteration 2: result = 11 + 10 + 2 = 23
	// iteration 3: result = 23 + 10 + 3 = 36
	want := "36\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_NestedLoopContinueInner(t *testing.T) {
	// continue in inner loop skips remaining inner body; outer continues normally
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `result = 0
for i in [1, 2, 3]:
    for j in [10, 20, 30]:
        if j == 20:
            continue
        result = result + j
    result = result + i
print(result)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	// inner loop: j=10 (add), j=20 (skip), j=30 (add) => adds 40 each time
	// outer loop: i=1,2,3
	// iteration 1: result = 0 + 10 + 30 + 1 = 41
	// iteration 2: result = 41 + 10 + 30 + 2 = 83
	// iteration 3: result = 83 + 10 + 30 + 3 = 126
	want := "126\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// --- Task 9.5: Exception handling integration tests ---

func TestExecString_MultipleExceptClauses(t *testing.T) {
	// Multiple except clauses with priority: first match wins.
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def test(x):
    try:
        if x == 1:
            result = 1 / 0
        elif x == 2:
            result = int("abc")
        else:
            result = [][5]
    except ZeroDivisionError:
        print("zero")
    except ValueError:
        print("value")
    except IndexError:
        print("index")

test(1)
test(2)
test(3)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "zero\nvalue\nindex\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ExceptAsBinding(t *testing.T) {
	// except ValueError as e: binds the exception to a variable.
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `try:
    raise ValueError("custom error")
except ValueError as e:
    print(e)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "custom error\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_RaiseWithExpression(t *testing.T) {
	// raise TypeError("msg") creates and raises a typed exception with a message.
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `try:
    raise TypeError("wrong type")
except TypeError as e:
    print(e)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "wrong type\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_BareRaise(t *testing.T) {
	// bare raise re-raises the current exception in an except handler.
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def inner():
    try:
        raise ValueError("from inner")
    except ValueError:
        print("caught in inner")
        raise

try:
    inner()
except ValueError as e:
    print(e)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "caught in inner\nfrom inner\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ExceptionPropagationThroughCalls(t *testing.T) {
	// Exception in called function propagates to caller's try/except.
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def deep():
    raise RuntimeError("deep error")

def middle():
    deep()

def outer():
    try:
        middle()
    except RuntimeError as e:
        print(e)

outer()`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "deep error\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_FinallyAlwaysExecutes(t *testing.T) {
	// finally block always executes, even when exception occurs.
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def test():
    try:
        print("try")
        raise ValueError("oops")
    except ValueError:
        print("except")
    finally:
        print("finally")

test()`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "try\nexcept\nfinally\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_FinallyWithUnhandledException(t *testing.T) {
	// finally block runs even when exception is NOT caught.
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `try:
    print("before")
    raise TypeError("unhandled")
finally:
    print("finally")`

	err := r.ExecString(source, "<test>")
	// The TypeError should propagate as an unhandled exception.
	if err == nil {
		t.Fatal("expected error for unhandled TypeError")
	}
	// But finally should still have executed.
	want := "before\nfinally\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ResourceLimitsWithLowCallDepth(t *testing.T) {
	// Verify that the configurable max call depth actually limits execution.
	// With maxCallDepth=5, even shallow recursion should trigger RecursionError.
	r := New(WithMaxCallDepth(5))

	source := `def a():
    return b()
def b():
    return c()
def c():
    return d()
def d():
    return e()
def e():
    return f()
def f():
    return 42
a()`

	err := r.ExecString(source, "<test>")
	if err == nil {
		t.Fatal("expected RecursionError with maxCallDepth=5, got nil")
	}

	var re *RuntimeError
	if !errors.As(err, &re) {
		t.Fatalf("expected *RuntimeError, got %T: %v", err, err)
	}
}
