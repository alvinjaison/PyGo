package runtime

import (
	"bytes"
	"strings"
	"testing"
)

// runComp executes source and returns trimmed stdout, failing on error.
func runComp(t *testing.T, source string) string {
	t.Helper()
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	if err := r.ExecString(source, "<test>"); err != nil {
		t.Fatalf("ExecString failed: %v\nsource:\n%s", err, source)
	}
	return strings.TrimSpace(buf.String())
}

// compError executes source expecting a failure, returning the error text.
func compError(t *testing.T, source string) string {
	t.Helper()
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(source, "<test>")
	if err == nil {
		t.Fatalf("expected an error, got success\nsource:\n%s", source)
	}
	return err.Error()
}

// --- list comprehensions ---

func TestListCompBasic(t *testing.T) {
	got := runComp(t, "print([x * x for x in [1, 2, 3, 4]])")
	if got != "[1, 4, 9, 16]" {
		t.Errorf("got %q", got)
	}
}

func TestListCompEmpty(t *testing.T) {
	got := runComp(t, "print([x for x in []])")
	if got != "[]" {
		t.Errorf("got %q", got)
	}
}

func TestListCompFilter(t *testing.T) {
	got := runComp(t, "print([x for x in range(10) if x % 2 == 0])")
	if got != "[0, 2, 4, 6, 8]" {
		t.Errorf("got %q", got)
	}
}

func TestListCompMultipleFilters(t *testing.T) {
	got := runComp(t, "print([x for x in range(20) if x % 2 == 0 if x % 3 == 0])")
	if got != "[0, 6, 12, 18]" {
		t.Errorf("got %q", got)
	}
}

func TestListCompNestedLoops(t *testing.T) {
	// Ordering must match CPython: outer loop varies slowest.
	got := runComp(t, "print([x + y for x in [1, 2] for y in [10, 20]])")
	if got != "[11, 21, 12, 22]" {
		t.Errorf("got %q", got)
	}
}

func TestListCompNestedLoopDependent(t *testing.T) {
	// Inner iterable depends on the outer loop variable.
	got := runComp(t, "print([y for x in range(3) for y in range(x)])")
	if got != "[0, 0, 1]" {
		t.Errorf("got %q", got)
	}
}

func TestListCompTupleTarget(t *testing.T) {
	got := runComp(t, "print([a + b for a, b in [(1, 2), (3, 4)]])")
	if got != "[3, 7]" {
		t.Errorf("got %q", got)
	}
}

func TestListCompClosureOverOuterVariable(t *testing.T) {
	// The comprehension runs in its own scope but must capture 'n' as a free
	// variable from the enclosing scope.
	got := runComp(t, "n = 100\nprint([x + n for x in [1, 2, 3]])")
	if got != "[101, 102, 103]" {
		t.Errorf("got %q", got)
	}
}

func TestListCompConditionalExpression(t *testing.T) {
	// A ternary as the element expression (distinct from an 'if' filter).
	got := runComp(t, "print(['even' if x % 2 == 0 else 'odd' for x in range(4)])")
	if got != "['even', 'odd', 'even', 'odd']" {
		t.Errorf("got %q", got)
	}
}

func TestListCompNestedComprehension(t *testing.T) {
	got := runComp(t, "print([[y for y in range(x)] for x in range(3)])")
	if got != "[[], [0], [0, 1]]" {
		t.Errorf("got %q", got)
	}
}

func TestListCompInsideFunction(t *testing.T) {
	// Comprehension inside a function, capturing a parameter.
	src := "def f(items, k):\n" +
		"    return [i * k for i in items]\n" +
		"print(f([1, 2, 3], 10))\n"
	got := runComp(t, src)
	if got != "[10, 20, 30]" {
		t.Errorf("got %q", got)
	}
}

func TestListCompLoopVarDoesNotLeak(t *testing.T) {
	// In Python 3 the comprehension loop variable does not leak into the
	// enclosing scope.
	src := "x = 'outer'\n" +
		"_ = [x for x in [1, 2, 3]]\n" +
		"print(x)\n"
	got := runComp(t, src)
	if got != "outer" {
		t.Errorf("comprehension leaked loop variable: got %q", got)
	}
}

// --- dict comprehensions ---

func TestDictCompBasic(t *testing.T) {
	got := runComp(t, "print({k: k * k for k in [1, 2, 3]})")
	if got != "{1: 1, 2: 4, 3: 9}" {
		t.Errorf("got %q", got)
	}
}

func TestDictCompFilter(t *testing.T) {
	got := runComp(t, "print({x: x for x in range(6) if x % 2 == 0})")
	if got != "{0: 0, 2: 2, 4: 4}" {
		t.Errorf("got %q", got)
	}
}

func TestDictCompTupleTarget(t *testing.T) {
	got := runComp(t, "print({k: v for k, v in [('a', 1), ('b', 2)]})")
	if got != "{'a': 1, 'b': 2}" {
		t.Errorf("got %q", got)
	}
}

func TestDictCompFromKeysAndValues(t *testing.T) {
	src := "keys = ['x', 'y', 'z']\n" +
		"vals = [1, 2, 3]\n" +
		"print({keys[i]: vals[i] for i in range(3)})\n"
	got := runComp(t, src)
	if got != "{'x': 1, 'y': 2, 'z': 3}" {
		t.Errorf("got %q", got)
	}
}

// --- deferred features report clear errors ---

func TestSetCompReportsClearError(t *testing.T) {
	msg := compError(t, "x = {v for v in [1, 2, 3]}")
	if !strings.Contains(msg, "set comprehensions are not supported yet") {
		t.Errorf("unclear error for set comprehension: %q", msg)
	}
}

func TestGeneratorExpReportsClearError(t *testing.T) {
	msg := compError(t, "x = (v for v in [1, 2, 3])")
	if !strings.Contains(msg, "generator expressions are not supported yet") {
		t.Errorf("unclear error for generator expression: %q", msg)
	}
}
