package runtime

import (
	"bytes"
	"strings"
	"testing"
)

// ============================================================
// Call-site argument unpacking and **kwargs ordering — Task 2.3
// Validates: Requirements 4.5, 4.6, 4.7, 4.8, 4.9
//
// These cover the two halves of *args/**kwargs that the existing
// kwargs suites did not exercise:
//   - `f(*iterable)` / `f(**mapping)` at the call site, which the
//     compiler lowers to BUILD_TUPLE_UNPACK_WITH_CALL /
//     BUILD_MAP_UNPACK / CALL_FUNCTION_EX.
//   - the ordering guarantee on a collected **kwargs dict.
// ============================================================

// run executes src and returns its stdout, failing the test on error.
func runOK(t *testing.T, src string) string {
	t.Helper()
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	if err := r.ExecString(src, "<test>"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return strings.TrimSpace(buf.String())
}

// runErr executes src expecting failure and returns the error message.
func runErr(t *testing.T, src string) string {
	t.Helper()
	r := New()
	err := r.ExecString(src, "<test>")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	return err.Error()
}

// --- Requirement 4.9: *iterable unpacking at the call site ---

func TestCallUnpack_StarList(t *testing.T) {
	got := runOK(t, `
def g(a, b, c):
    print(a, b, c)
g(*[1, 2, 3])
`)
	if got != "1 2 3" {
		t.Errorf("got %q, want %q", got, "1 2 3")
	}
}

func TestCallUnpack_StarTuple(t *testing.T) {
	got := runOK(t, `
def g(a, b):
    print(a, b)
g(*(7, 8))
`)
	if got != "7 8" {
		t.Errorf("got %q, want %q", got, "7 8")
	}
}

func TestCallUnpack_StarRange(t *testing.T) {
	got := runOK(t, `
def g(a, b, c):
    print(a, b, c)
g(*range(3))
`)
	if got != "0 1 2" {
		t.Errorf("got %q, want %q", got, "0 1 2")
	}
}

func TestCallUnpack_PositionalThenStar(t *testing.T) {
	got := runOK(t, `
def g(a, b, c, d):
    print(a, b, c, d)
g(1, 2, *[3, 4])
`)
	if got != "1 2 3 4" {
		t.Errorf("got %q, want %q", got, "1 2 3 4")
	}
}

func TestCallUnpack_MultipleStarSegments(t *testing.T) {
	got := runOK(t, `
def g(a, b, c, d, e):
    print(a, b, c, d, e)
g(*[1, 2], 3, *[4, 5])
`)
	if got != "1 2 3 4 5" {
		t.Errorf("got %q, want %q", got, "1 2 3 4 5")
	}
}

func TestCallUnpack_StarIntoStarArgs(t *testing.T) {
	got := runOK(t, `
def g(a, *args):
    print(a, args)
g(*[1, 2, 3])
`)
	if got != "1 (2, 3)" {
		t.Errorf("got %q, want %q", got, "1 (2, 3)")
	}
}

func TestCallUnpack_StarEmptyIterable(t *testing.T) {
	got := runOK(t, `
def g(*args):
    print(args)
g(*[])
`)
	if got != "()" {
		t.Errorf("got %q, want %q", got, "()")
	}
}

func TestCallUnpack_StarNonIterable_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `
def g(a):
    pass
g(*5)
`)
	if !strings.Contains(msg, "argument after * must be an iterable") {
		t.Errorf("expected 'argument after * must be an iterable', got: %s", msg)
	}
}

// --- Requirement 4.8: **mapping unpacking at the call site ---

func TestCallUnpack_DoubleStarDict(t *testing.T) {
	got := runOK(t, `
def g(a, b, c):
    print(a, b, c)
g(**{"a": 1, "b": 2, "c": 3})
`)
	if got != "1 2 3" {
		t.Errorf("got %q, want %q", got, "1 2 3")
	}
}

func TestCallUnpack_PositionalAndDoubleStar(t *testing.T) {
	got := runOK(t, `
def g(a, b, c):
    print(a, b, c)
g(1, **{"b": 2, "c": 3})
`)
	if got != "1 2 3" {
		t.Errorf("got %q, want %q", got, "1 2 3")
	}
}

func TestCallUnpack_StarAndDoubleStarTogether(t *testing.T) {
	got := runOK(t, `
def g(a, b, c):
    print(a, b, c)
g(*[1, 2], **{"c": 3})
`)
	if got != "1 2 3" {
		t.Errorf("got %q, want %q", got, "1 2 3")
	}
}

func TestCallUnpack_KeywordAndDoubleStarMerged(t *testing.T) {
	got := runOK(t, `
def g(a, b, c):
    print(a, b, c)
g(a=1, **{"b": 2, "c": 3})
`)
	if got != "1 2 3" {
		t.Errorf("got %q, want %q", got, "1 2 3")
	}
}

func TestCallUnpack_DoubleStarIntoStarStarKwargs(t *testing.T) {
	got := runOK(t, `
def g(a, **kwargs):
    print(a)
    print(kwargs)
g(**{"a": 1, "x": 2, "y": 3})
`)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), got)
	}
	if lines[0] != "1" {
		t.Errorf("a: got %q, want %q", lines[0], "1")
	}
	// The mapping's own order is preserved into **kwargs.
	if lines[1] != "{'x': 2, 'y': 3}" {
		t.Errorf("kwargs: got %q, want %q", lines[1], "{'x': 2, 'y': 3}")
	}
}

func TestCallUnpack_DoubleStarEmptyDict(t *testing.T) {
	got := runOK(t, `
def g(**kwargs):
    print(kwargs)
g(**{})
`)
	if got != "{}" {
		t.Errorf("got %q, want %q", got, "{}")
	}
}

func TestCallUnpack_DoubleStarDuplicateKey_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `
def g(a):
    pass
g(**{"a": 1}, **{"a": 2})
`)
	if !strings.Contains(msg, "multiple values for keyword argument") {
		t.Errorf("expected 'multiple values for keyword argument', got: %s", msg)
	}
	if !strings.Contains(msg, "'a'") {
		t.Errorf("expected error to mention 'a', got: %s", msg)
	}
}

func TestCallUnpack_DoubleStarNonMapping_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `
def g(a):
    pass
g(**[1, 2])
`)
	if !strings.Contains(msg, "argument after ** must be a mapping") {
		t.Errorf("expected 'argument after ** must be a mapping', got: %s", msg)
	}
}

func TestCallUnpack_DoubleStarNonStringKey_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `
def g(**kwargs):
    pass
g(**{1: 2})
`)
	if !strings.Contains(msg, "keywords must be strings") {
		t.Errorf("expected 'keywords must be strings', got: %s", msg)
	}
}

func TestCallUnpack_DoubleStarUnexpectedKeyword_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `
def g(a):
    pass
g(**{"a": 1, "nope": 2})
`)
	if !strings.Contains(msg, "unexpected keyword argument") {
		t.Errorf("expected 'unexpected keyword argument', got: %s", msg)
	}
	if !strings.Contains(msg, "'nope'") {
		t.Errorf("expected error to mention 'nope', got: %s", msg)
	}
}

func TestCallUnpack_DoubleStarDuplicatesPositional_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `
def g(a, b):
    pass
g(1, **{"a": 2, "b": 3})
`)
	if !strings.Contains(msg, "multiple values for argument") {
		t.Errorf("expected 'multiple values for argument', got: %s", msg)
	}
}

func TestCallUnpack_StarIntoBuiltin(t *testing.T) {
	got := runOK(t, `print(*[1, 2, 3])`)
	if got != "1 2 3" {
		t.Errorf("got %q, want %q", got, "1 2 3")
	}
}

// --- Requirement 4.5: **kwargs preserves call-site order ---

func TestKwargs_CollectionPreservesCallOrder(t *testing.T) {
	// Go randomises map iteration order, so a **kwargs dict built from a map
	// would print differently between runs. Repeat to catch any regression.
	const want = "{'z': 1, 'y': 2, 'x': 3, 'w': 4, 'v': 5}"
	for i := 0; i < 20; i++ {
		got := runOK(t, `
def f(**kwargs):
    print(kwargs)
f(z=1, y=2, x=3, w=4, v=5)
`)
		if got != want {
			t.Fatalf("iteration %d: got %q, want %q", i, got, want)
		}
	}
}

func TestKwargs_CollectionOrderSkipsNamedParams(t *testing.T) {
	const want = "{'d': 4, 'c': 3, 'b': 2}"
	for i := 0; i < 20; i++ {
		got := runOK(t, `
def f(a, **kwargs):
    print(kwargs)
f(d=4, a=1, c=3, b=2)
`)
		if got != want {
			t.Fatalf("iteration %d: got %q, want %q", i, got, want)
		}
	}
}

func TestKwargs_IterationOrderMatchesCallOrder(t *testing.T) {
	got := runOK(t, `
def f(**kwargs):
    for k in kwargs:
        print(k, kwargs[k])
f(third=3, first=1, second=2)
`)
	want := "third 3\nfirst 1\nsecond 2"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
