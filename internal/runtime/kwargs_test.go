package runtime

import (
	"bytes"
	"strings"
	"testing"
)

// ============================================================
// Unit tests for CALL_FUNCTION_KW (Task 2.2)
// Validates: Requirements 4.2, 4.3, 4.4
// ============================================================

// --- Requirement 4.2: Keyword arguments matched to parameter names ---

func TestKwargs_BasicKeywordCall(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def greet(name, greeting):
    print(greeting, name)
greet(name="Alice", greeting="Hello")
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "Hello Alice" {
		t.Errorf("got %q, want %q", got, "Hello Alice")
	}
}

func TestKwargs_MixedPositionalAndKeyword(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def add(a, b, c):
    print(a + b + c)
add(1, c=3, b=2)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "6" {
		t.Errorf("got %q, want %q", got, "6")
	}
}

func TestKwargs_KeywordWithDefaults(t *testing.T) {
	// Note: default argument compilation is not yet implemented.
	// This test verifies that when all params are provided via positional + keyword,
	// the call works correctly.
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, b, c):
    print(a, b, c)
f(1, c=30, b=10)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "1 10 30" {
		t.Errorf("got %q, want %q", got, "1 10 30")
	}
}

func TestKwargs_AllKeyword(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(x, y, z):
    print(x, y, z)
f(z=3, x=1, y=2)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "1 2 3" {
		t.Errorf("got %q, want %q", got, "1 2 3")
	}
}

func TestKwargs_SingleKeyword(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def identity(val):
    return val
result = identity(val=42)
print(result)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "42" {
		t.Errorf("got %q, want %q", got, "42")
	}
}

// --- Requirement 4.3: TypeError for unexpected keyword argument ---

func TestKwargs_UnexpectedKeyword_RaisesTypeError(t *testing.T) {
	r := New()
	err := r.ExecString(`
def f(a, b):
    pass
f(1, 2, c=3)
`, "<test>")
	if err == nil {
		t.Fatal("expected TypeError for unexpected keyword argument, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "unexpected keyword argument") {
		t.Errorf("expected error about unexpected keyword, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "'c'") {
		t.Errorf("expected error to mention 'c', got: %s", errMsg)
	}
}

func TestKwargs_UnexpectedKeyword_NoArgs(t *testing.T) {
	r := New()
	err := r.ExecString(`
def f():
    pass
f(x=1)
`, "<test>")
	if err == nil {
		t.Fatal("expected TypeError for unexpected keyword argument, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected keyword argument") {
		t.Errorf("expected error about unexpected keyword, got: %s", err.Error())
	}
}

// --- Requirement 4.4: TypeError for duplicate argument ---

func TestKwargs_DuplicatePositionalAndKeyword_RaisesTypeError(t *testing.T) {
	r := New()
	err := r.ExecString(`
def f(a, b):
    pass
f(1, a=2)
`, "<test>")
	if err == nil {
		t.Fatal("expected TypeError for duplicate argument, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "multiple values for argument") {
		t.Errorf("expected error about multiple values, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "'a'") {
		t.Errorf("expected error to mention 'a', got: %s", errMsg)
	}
}

func TestKwargs_DuplicateMiddleParam(t *testing.T) {
	r := New()
	err := r.ExecString(`
def f(a, b, c):
    pass
f(1, 2, b=3)
`, "<test>")
	if err == nil {
		t.Fatal("expected TypeError for duplicate argument, got nil")
	}
	if !strings.Contains(err.Error(), "multiple values for argument") {
		t.Errorf("expected error about multiple values, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "'b'") {
		t.Errorf("expected error to mention 'b', got: %s", err.Error())
	}
}

// --- Additional edge cases ---

func TestKwargs_BuiltinWithKeyword(t *testing.T) {
	// print() accepts keyword arg 'end'
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`print("hello", end="!\n")`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if got != "hello!\n" {
		t.Errorf("got %q, want %q", got, "hello!\n")
	}
}

func TestKwargs_KeywordOverridesDefault(t *testing.T) {
	// Verify keyword args correctly bind to the second parameter.
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def f(a, b):
    print(a, b)
f(1, b=20)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "1 20" {
		t.Errorf("got %q, want %q", got, "1 20")
	}
}

func TestKwargs_TooManyPositional_RaisesTypeError(t *testing.T) {
	r := New()
	err := r.ExecString(`
def f(a, b):
    pass
f(1, 2, 3)
`, "<test>")
	if err == nil {
		t.Fatal("expected TypeError for too many positional args, got nil")
	}
	if !strings.Contains(err.Error(), "takes") {
		t.Errorf("expected error about takes N args, got: %s", err.Error())
	}
}

func TestKwargs_MissingRequiredArg_RaisesTypeError(t *testing.T) {
	r := New()
	err := r.ExecString(`
def f(a, b, c):
    pass
f(c=3)
`, "<test>")
	if err == nil {
		t.Fatal("expected TypeError for missing required argument, got nil")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("expected error about missing argument, got: %s", err.Error())
	}
}

func TestKwargs_ReturnValue(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def multiply(x, y):
    return x * y
result = multiply(x=6, y=7)
print(result)
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "42" {
		t.Errorf("got %q, want %q", got, "42")
	}
}

func TestKwargs_StringKeywordArgs(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`
def concat(first, second, sep):
    print(first + sep + second)
concat("hello", sep="-", second="world")
`, "<test>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "hello-world" {
		t.Errorf("got %q, want %q", got, "hello-world")
	}
}
