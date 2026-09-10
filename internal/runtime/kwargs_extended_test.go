package runtime

import (
	"bytes"
	"strings"
	"testing"
)

// ============================================================
// Unit tests for keyword arguments — Task 2.5
// Validates: Requirements 4.1–4.11
//
// These extend the existing kwargs_test.go (Task 2.2) with tests
// for *args, **kwargs collection, **dict unpacking, keyword-only
// parameters, and additional edge cases.
// ============================================================

// --- Requirement 4.5: **kwargs collection ---

func TestKwargs_StarStarKwargsCollectsUnmatched(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, **kwargs):
    print(a)
    print(kwargs)
f(1, x=10, y=20)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "1" {
		t.Errorf("first line: got %q, want %q", lines[0], "1")
	}
	// kwargs dict should contain x and y (order may vary).
	kw := lines[1]
	if !strings.Contains(kw, "'x': 10") {
		t.Errorf("expected kwargs to contain 'x': 10, got %q", kw)
	}
	if !strings.Contains(kw, "'y': 20") {
		t.Errorf("expected kwargs to contain 'y': 20, got %q", kw)
	}
}

func TestKwargs_StarStarKwargsEmpty(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, **kwargs):
    print(a)
    print(kwargs)
f(1)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "1" {
		t.Errorf("first line: got %q, want %q", lines[0], "1")
	}
	if lines[1] != "{}" {
		t.Errorf("expected empty kwargs dict, got %q", lines[1])
	}
}

func TestKwargs_StarStarKwargsOnly(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(**kwargs):
    print(kwargs)
f(a=1, b=2, c=3)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if !strings.Contains(got, "'a': 1") {
		t.Errorf("expected 'a': 1 in %q", got)
	}
	if !strings.Contains(got, "'b': 2") {
		t.Errorf("expected 'b': 2 in %q", got)
	}
	if !strings.Contains(got, "'c': 3") {
		t.Errorf("expected 'c': 3 in %q", got)
	}
}

// --- Requirement 4.6: *args collection ---

func TestKwargs_StarArgsCollectsExcessPositional(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, *args):
    print(a)
    print(args)
f(1, 2, 3, 4)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "1" {
		t.Errorf("first line: got %q, want %q", lines[0], "1")
	}
	if lines[1] != "(2, 3, 4)" {
		t.Errorf("second line: got %q, want %q", lines[1], "(2, 3, 4)")
	}
}

func TestKwargs_StarArgsEmpty(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, *args):
    print(a)
    print(args)
f(1)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "1" {
		t.Errorf("first line: got %q, want %q", lines[0], "1")
	}
	if lines[1] != "()" {
		t.Errorf("second line: got %q, want %q", lines[1], "()")
	}
}

func TestKwargs_StarArgsOnly(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(*args):
    print(args)
f(10, 20, 30)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "(10, 20, 30)" {
		t.Errorf("got %q, want %q", got, "(10, 20, 30)")
	}
}

// --- Requirement 4.7: *args and **kwargs together ---

func TestKwargs_StarArgsAndStarStarKwargsTogether(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, *args, **kwargs):
    print(a)
    print(args)
    print(kwargs)
f(1, 2, 3, x=10, y=20)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "1" {
		t.Errorf("a: got %q, want %q", lines[0], "1")
	}
	if lines[1] != "(2, 3)" {
		t.Errorf("args: got %q, want %q", lines[1], "(2, 3)")
	}
	// kwargs should have x and y.
	if !strings.Contains(lines[2], "'x': 10") {
		t.Errorf("expected kwargs to contain 'x': 10, got %q", lines[2])
	}
	if !strings.Contains(lines[2], "'y': 20") {
		t.Errorf("expected kwargs to contain 'y': 20, got %q", lines[2])
	}
}

func TestKwargs_StarArgsAndStarStarKwargsNoOverflow(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, *args, **kwargs):
    print(a)
    print(args)
    print(kwargs)
f(1)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "1" {
		t.Errorf("a: got %q, want %q", lines[0], "1")
	}
	if lines[1] != "()" {
		t.Errorf("args: got %q, want %q", lines[1], "()")
	}
	if lines[2] != "{}" {
		t.Errorf("kwargs: got %q, want %q", lines[2], "{}")
	}
}

func TestKwargs_PositionalOverflowAndKeywordOverflow_Separated(t *testing.T) {
	// Verify that positional overflow goes to *args and keyword overflow goes to **kwargs.
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, b, *args, **kwargs):
    print(a, b)
    print(args)
    print(kwargs)
f(1, 2, 3, 4, x=10)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "1 2" {
		t.Errorf("a, b: got %q, want %q", lines[0], "1 2")
	}
	if lines[1] != "(3, 4)" {
		t.Errorf("args: got %q, want %q", lines[1], "(3, 4)")
	}
	if !strings.Contains(lines[2], "'x': 10") {
		t.Errorf("kwargs: expected 'x': 10, got %q", lines[2])
	}
}

// --- Requirement 4.10: Keyword-only parameters ---

func TestKwargs_KeywordOnlyBasic(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, *, key):
    print(a, key)
f(1, key=42)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "1 42" {
		t.Errorf("got %q, want %q", got, "1 42")
	}
}

func TestKwargs_KeywordOnlyWithDefault(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, *, key=99):
    print(a, key)
f(1)
f(2, key=50)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "1 99" {
		t.Errorf("first call: got %q, want %q", lines[0], "1 99")
	}
	if lines[1] != "2 50" {
		t.Errorf("second call: got %q, want %q", lines[1], "2 50")
	}
}

func TestKwargs_KeywordOnlyAfterStarArgs(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, *args, key):
    print(a, args, key)
f(1, 2, 3, key=42)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "1 (2, 3) 42" {
		t.Errorf("got %q, want %q", got, "1 (2, 3) 42")
	}
}

func TestKwargs_KeywordOnlyAfterStarArgsWithDefault(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, *args, key=99):
    print(a, args, key)
f(1, 2, 3)
f(1, 2, key=42)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "1 (2, 3) 99" {
		t.Errorf("first call: got %q, want %q", lines[0], "1 (2, 3) 99")
	}
	if lines[1] != "1 (2,) 42" {
		t.Errorf("second call: got %q, want %q", lines[1], "1 (2,) 42")
	}
}

func TestKwargs_MultipleKeywordOnlyParams(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(*, a, b, c):
    print(a, b, c)
f(a=1, b=2, c=3)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "1 2 3" {
		t.Errorf("got %q, want %q", got, "1 2 3")
	}
}

// --- Requirement 4.11: TypeError for missing keyword-only argument ---

func TestKwargs_MissingKeywordOnly_RaisesTypeError(t *testing.T) {
	r := New()
	err := r.ExecString(`
def f(a, *, key):
    pass
f(1)
`, "<test>")
	if err == nil {
		t.Fatal("expected TypeError for missing keyword-only argument, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "missing") {
		t.Errorf("expected error about missing argument, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "keyword-only") {
		t.Errorf("expected error to mention 'keyword-only', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "'key'") {
		t.Errorf("expected error to mention 'key', got: %s", errMsg)
	}
}

func TestKwargs_KeywordOnlyCannotBePassedPositionally(t *testing.T) {
	r := New()
	// With bare *, keyword-only params cannot be passed positionally.
	err := r.ExecString(`
def f(a, *, key):
    pass
f(1, 2)
`, "<test>")
	if err == nil {
		t.Fatal("expected error when passing keyword-only param positionally, got nil")
	}
	// Should get either "takes N positional" or "missing keyword-only" error.
	errMsg := err.Error()
	if !strings.Contains(errMsg, "takes") && !strings.Contains(errMsg, "missing") {
		t.Errorf("expected error about too many positional or missing keyword-only, got: %s", errMsg)
	}
}

// --- Requirement 4.3: Unexpected keyword with **kwargs (should be collected, not error) ---

func TestKwargs_UnexpectedKeywordWithStarStarKwargs_NoError(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, **kwargs):
    print(a)
    print(kwargs)
f(1, unknown_key=42)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error (should be collected in **kwargs): %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}
	if !strings.Contains(lines[1], "'unknown_key': 42") {
		t.Errorf("expected kwargs to contain 'unknown_key': 42, got %q", lines[1])
	}
}

// --- Requirement 4.4: Duplicate argument detection with *args ---

func TestKwargs_DuplicateWithStarArgs_RaisesTypeError(t *testing.T) {
	r := New()
	// Even with *args, supplying a named param both positionally and via keyword is an error.
	err := r.ExecString(`
def f(a, *args):
    pass
f(1, a=2)
`, "<test>")
	if err == nil {
		t.Fatal("expected TypeError for duplicate argument, got nil")
	}
	if !strings.Contains(err.Error(), "multiple values for argument") {
		t.Errorf("expected error about multiple values, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "'a'") {
		t.Errorf("expected error to mention 'a', got: %s", err.Error())
	}
}

// --- Mixed *args, keyword-only, and **kwargs ---

func TestKwargs_FullSignature_ArgsKwOnlyKwargs(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, b, *args, key=0, **kwargs):
    print(a, b)
    print(args)
    print(key)
    print(kwargs)
f(1, 2, 3, 4, key=99, extra=42)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "1 2" {
		t.Errorf("a, b: got %q, want %q", lines[0], "1 2")
	}
	if lines[1] != "(3, 4)" {
		t.Errorf("args: got %q, want %q", lines[1], "(3, 4)")
	}
	if lines[2] != "99" {
		t.Errorf("key: got %q, want %q", lines[2], "99")
	}
	if !strings.Contains(lines[3], "'extra': 42") {
		t.Errorf("kwargs: expected 'extra': 42, got %q", lines[3])
	}
}

func TestKwargs_FullSignature_DefaultsUsed(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, b, *args, key=0, **kwargs):
    print(a, b)
    print(args)
    print(key)
    print(kwargs)
f(1, 2)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "1 2" {
		t.Errorf("a, b: got %q, want %q", lines[0], "1 2")
	}
	if lines[1] != "()" {
		t.Errorf("args: got %q, want %q", lines[1], "()")
	}
	if lines[2] != "0" {
		t.Errorf("key: got %q, want %q", lines[2], "0")
	}
	if lines[3] != "{}" {
		t.Errorf("kwargs: got %q, want %q", lines[3], "{}")
	}
}

// --- Keyword argument use cases with return values ---

func TestKwargs_FunctionWithArgsKwargsReturnsTuple(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(*args, **kwargs):
    print(len(args))
    print(len(kwargs))
f(1, 2, 3, a=4, b=5)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "3" {
		t.Errorf("len(args): got %q, want %q", lines[0], "3")
	}
	if lines[1] != "2" {
		t.Errorf("len(kwargs): got %q, want %q", lines[1], "2")
	}
}

// --- Keyword with positional defaults ---

func TestKwargs_PositionalDefaultWithKeywordOverride(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, b=10, c=20):
    print(a, b, c)
f(1, c=30)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "1 10 30" {
		t.Errorf("got %q, want %q", got, "1 10 30")
	}
}

func TestKwargs_AllDefaultsWithKeywordOverride(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a=1, b=2, c=3):
    print(a, b, c)
f(b=20)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "1 20 3" {
		t.Errorf("got %q, want %q", got, "1 20 3")
	}
}

// --- Nested calls with keywords ---

func TestKwargs_NestedFunctionCallsWithKeywords(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def add(a, b):
    return a + b
def mul(x, y):
    return x * y
result = mul(x=add(a=2, b=3), y=4)
print(result)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "20" {
		t.Errorf("got %q, want %q", got, "20")
	}
}

// --- Keyword-only params with bare * separator ---

func TestKwargs_BareStarSeparator(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, b, *, c, d):
    print(a, b, c, d)
f(1, 2, c=3, d=4)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "1 2 3 4" {
		t.Errorf("got %q, want %q", got, "1 2 3 4")
	}
}

func TestKwargs_BareStarTooManyPositional_RaisesTypeError(t *testing.T) {
	r := New()
	err := r.ExecString(`
def f(a, *, b):
    pass
f(1, 2)
`, "<test>")
	if err == nil {
		t.Fatal("expected TypeError for extra positional arg after bare *, got nil")
	}
	if !strings.Contains(err.Error(), "takes") || !strings.Contains(err.Error(), "positional") {
		t.Errorf("expected error about positional arguments, got: %s", err.Error())
	}
}

// --- *args with keyword passing to named param ---

func TestKwargs_StarArgsWithKeywordToNamed(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, b, *args):
    print(a, b, args)
f(1, b=2)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "1 2 ()" {
		t.Errorf("got %q, want %q", got, "1 2 ()")
	}
}

// --- Accessing *args and **kwargs in the function body ---

func TestKwargs_ArgsIndexAccess(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(*args):
    print(args[0], args[1], args[2])
f(10, 20, 30)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "10 20 30" {
		t.Errorf("got %q, want %q", got, "10 20 30")
	}
}

func TestKwargs_KwargsKeyAccess(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(**kwargs):
    print(kwargs["name"], kwargs["age"])
f(name="Alice", age=30)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "Alice 30" {
		t.Errorf("got %q, want %q", got, "Alice 30")
	}
}

// --- Keyword args with closures ---

func TestKwargs_ClosureWithKeywordArgs(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def make_adder(x):
    def adder(y, z=0):
        return x + y + z
    return adder
add5 = make_adder(5)
print(add5(3, z=10))
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "18" {
		t.Errorf("got %q, want %q", got, "18")
	}
}

// --- Edge case: no positional args, only keyword-only ---

func TestKwargs_OnlyKeywordOnlyParams(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(*, x, y):
    print(x + y)
f(x=3, y=7)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "10" {
		t.Errorf("got %q, want %q", got, "10")
	}
}

func TestKwargs_OnlyKeywordOnlyMissing_RaisesTypeError(t *testing.T) {
	r := New()
	err := r.ExecString(`
def f(*, x, y):
    pass
f(x=1)
`, "<test>")
	if err == nil {
		t.Fatal("expected TypeError for missing keyword-only 'y', got nil")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("expected error about missing argument, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "'y'") {
		t.Errorf("expected error to mention 'y', got: %s", err.Error())
	}
}

// --- kwargs iteration ---

func TestKwargs_IterateOverKwargs(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(**kwargs):
    for key in kwargs:
        print(key)
f(alpha=1, beta=2)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	// Order might vary; just check both keys are present.
	if !strings.Contains(got, "alpha") {
		t.Errorf("expected 'alpha' in output, got %q", got)
	}
	if !strings.Contains(got, "beta") {
		t.Errorf("expected 'beta' in output, got %q", got)
	}
}

// --- Keyword args with string values ---

func TestKwargs_StringValueKeywords(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def greet(name, greeting="Hi"):
    print(greeting + " " + name)
greet(greeting="Hello", name="World")
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "Hello World" {
		t.Errorf("got %q, want %q", got, "Hello World")
	}
}
