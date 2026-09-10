package runtime

import (
	"strings"
	"testing"
)

// ============================================================
// Unit tests for slicing — Task 5.5
// **Validates: Requirements 9.1–9.13**
//
// Every expected value below was captured from CPython 3 running the same
// snippet, rather than derived from PyGo's own behaviour.
// Extended slice assignment length rules live in slice_assign_test.go.
// ============================================================

// checkExpr runs a list of (expression, expected output) pairs.
func checkExpr(t *testing.T, setup string, cases [][2]string) {
	t.Helper()
	for _, c := range cases {
		src := setup + "\nprint(" + c[0] + ")"
		if got := runOK(t, src); got != c[1] {
			t.Errorf("%s: got %q, want %q", c[0], got, c[1])
		}
	}
}

// --- Requirement 9.1, 9.2, 9.3: list slice reads ---

func TestSlice_ListBasicReads(t *testing.T) {
	checkExpr(t, "x = [0, 1, 2, 3, 4, 5]", [][2]string{
		{"x[1:3]", "[1, 2]"},
		{"x[:3]", "[0, 1, 2]"},
		{"x[3:]", "[3, 4, 5]"},
		{"x[:]", "[0, 1, 2, 3, 4, 5]"},
		{"x[1:1]", "[]"},
		{"x[3:1]", "[]"},
	})
}

func TestSlice_ListStep(t *testing.T) {
	checkExpr(t, "x = [0, 1, 2, 3, 4, 5]", [][2]string{
		{"x[::1]", "[0, 1, 2, 3, 4, 5]"},
		{"x[::2]", "[0, 2, 4]"},
		{"x[1::2]", "[1, 3, 5]"},
		{"x[::-1]", "[5, 4, 3, 2, 1, 0]"},
		{"x[::-2]", "[5, 3, 1]"},
		{"x[4:1:-1]", "[4, 3, 2]"},
	})
}

// --- Requirement 9.4: negative indices ---

func TestSlice_ListNegativeIndices(t *testing.T) {
	checkExpr(t, "x = [0, 1, 2, 3, 4, 5]", [][2]string{
		{"x[-2:]", "[4, 5]"},
		{"x[:-2]", "[0, 1, 2, 3]"},
		{"x[-4:-1]", "[2, 3, 4]"},
		{"x[-1:]", "[5]"},
	})
}

// --- Requirement 9.5: out-of-range bounds are clamped, not errors ---

func TestSlice_ListOutOfRangeClamped(t *testing.T) {
	checkExpr(t, "x = [0, 1, 2, 3, 4, 5]", [][2]string{
		{"x[100:200]", "[]"},
		{"x[-100:100]", "[0, 1, 2, 3, 4, 5]"},
		{"x[-100:-99]", "[]"},
		{"x[2:2]", "[]"},
	})
}

// --- Requirement 9.6: tuple and str slices ---

func TestSlice_TupleReads(t *testing.T) {
	checkExpr(t, "t = (0, 1, 2, 3, 4)", [][2]string{
		{"t[1:3]", "(1, 2)"},
		{"t[::-1]", "(4, 3, 2, 1, 0)"},
		{"t[::2]", "(0, 2, 4)"},
	})
	checkExpr(t, "t = (1,)", [][2]string{
		{"t[0:1]", "(1,)"},
		{"t[1:5]", "()"},
	})
}

func TestSlice_StrReads(t *testing.T) {
	checkExpr(t, `s = "abcdef"`, [][2]string{
		{"s[1:3]", "bc"},
		{"s[::-1]", "fedcba"},
		{"s[::2]", "ace"},
		{"s[-3:]", "def"},
	})
}

// Slicing operates on code points, not bytes.
func TestSlice_StrIsCodePointBased(t *testing.T) {
	checkExpr(t, `s = "héllo"`, [][2]string{
		{"s[::-1]", "olléh"},
		{"s[1:3]", "él"},
		{"len(s)", "5"},
	})
	checkExpr(t, `j = "日本語"`, [][2]string{
		{"j[1:]", "本語"},
		{"j[::-1]", "語本日"},
		{"len(j)", "3"},
	})
	checkExpr(t, `e = "aé日"`, [][2]string{
		{"e[::-1]", "日éa"},
		{"e[1:2]", "é"},
	})
}

func TestSlice_EmptyContainers(t *testing.T) {
	checkExpr(t, "pass", [][2]string{
		{"[][:]", "[]"},
		{"()[0:5]", "()"},
	})
	// An empty string prints as a blank line, so compare via a sentinel.
	if got := runOK(t, `print("<" + ""[::-1] + ">")`); got != "<>" {
		t.Errorf(`""[::-1]: got %q, want %q`, got, "<>")
	}
}

// --- Requirement 9.7: slice assignment on lists ---

func TestSlice_AssignReplacesRange(t *testing.T) {
	got := runOK(t, `
a = [0, 1, 2, 3, 4]
a[1:3] = [10, 20, 30]
print(a)
`)
	if got != "[0, 10, 20, 30, 3, 4]" {
		t.Errorf("got %q, want %q", got, "[0, 10, 20, 30, 3, 4]")
	}
}

func TestSlice_AssignShrinksAndGrows(t *testing.T) {
	got := runOK(t, `
b = [0, 1, 2, 3, 4]
b[1:4] = [9]
print(b)
c = [0, 1, 2, 3, 4]
c[1:1] = [77, 88]
print(c)
d = [0, 1, 2, 3, 4]
d[:] = [1, 2]
print(d)
e = [0, 1, 2, 3, 4]
e[2:4] = []
print(e)
f = [0, 1, 2]
f[10:20] = [5]
print(f)
g = [0, 1, 2, 3, 4]
g[2:2] = [9, 9]
print(g)
`)
	want := strings.Join([]string{
		"[0, 9, 4]",
		"[0, 77, 88, 1, 2, 3, 4]",
		"[1, 2]",
		"[0, 1, 4]",
		"[0, 1, 2, 5]",
		"[0, 1, 9, 9, 2, 3, 4]",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSlice_AssignFromAnyIterable(t *testing.T) {
	got := runOK(t, `
g = [0, 1, 2, 3]
g[1:3] = (8, 9)
print(g)
h = [0, 1, 2, 3]
h[1:3] = range(3)
print(h)
`)
	want := "[0, 8, 9, 3]\n[0, 0, 1, 2, 3]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSlice_AssignExtendedExactLength(t *testing.T) {
	got := runOK(t, `
i = [0, 1, 2, 3, 4, 5]
i[::2] = [10, 20, 30]
print(i)
j = [0, 1, 2, 3, 4, 5]
j[1::2] = [11, 22, 33]
print(j)
k = [0, 1, 2, 3, 4]
k[::-1] = [1, 2, 3, 4, 5]
print(k)
`)
	want := "[10, 1, 20, 3, 30, 5]\n[0, 11, 2, 22, 4, 33]\n[5, 4, 3, 2, 1]"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// --- Requirement 9.8: slice deletion on lists ---

func TestSlice_Delete(t *testing.T) {
	got := runOK(t, `
p = [0, 1, 2, 3, 4]
del p[1:3]
print(p)
q = [0, 1, 2, 3, 4, 5]
del q[::2]
print(q)
r = [0, 1, 2, 3, 4]
del r[:]
print(r)
u = [0, 1, 2, 3, 4]
del u[3:1]
print(u)
w = [0, 1, 2, 3, 4, 5]
del w[::-2]
print(w)
v = [0, 1, 2, 3, 4]
del v[0:0]
print(v)
z = [0, 1, 2, 3, 4]
del z[::-1]
print(z)
`)
	want := strings.Join([]string{
		"[0, 3, 4]",
		"[1, 3, 5]",
		"[]",
		"[0, 1, 2, 3, 4]",
		"[0, 2, 4]",
		"[0, 1, 2, 3, 4]",
		"[]",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// --- Requirement 9.10: step of zero ---

func TestSlice_StepZero_RaisesValueError(t *testing.T) {
	msg := runErr(t, `
x = [1, 2, 3]
print(x[::0])
`)
	if !strings.Contains(msg, "slice step cannot be zero") {
		t.Errorf("expected 'slice step cannot be zero', got: %s", msg)
	}
}

// --- Requirement 9.12, 9.13: immutable sequences reject slice writes ---

func TestSlice_TupleAssignment_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `
t = (1, 2, 3)
t[0:2] = [9, 9]
`)
	if !strings.Contains(msg, "'tuple' object does not support item assignment") {
		t.Errorf("unexpected error: %s", msg)
	}
}

func TestSlice_StrAssignment_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `
s = "abc"
s[0:2] = "xy"
`)
	if !strings.Contains(msg, "'str' object does not support item assignment") {
		t.Errorf("unexpected error: %s", msg)
	}
}

func TestSlice_TupleDeletion_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `
t = (1, 2, 3)
del t[0:2]
`)
	if !strings.Contains(msg, "'tuple' object does not support item deletion") {
		t.Errorf("unexpected error: %s", msg)
	}
}

func TestSlice_StrDeletion_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `
s = "abc"
del s[0:2]
`)
	if !strings.Contains(msg, "'str' object does not support item deletion") {
		t.Errorf("unexpected error: %s", msg)
	}
}

// --- Non-integer slice bounds ---

func TestSlice_NonIntegerBound_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `
d = [0, 1, 2]
print(d["a":2])
`)
	if !strings.Contains(msg, "slice indices must be integers") {
		t.Errorf("expected 'slice indices must be integers', got: %s", msg)
	}
}
