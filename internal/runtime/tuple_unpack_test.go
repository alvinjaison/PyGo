package runtime

import (
	"bytes"
	"strings"
	"testing"
)

// =============================================================================
// Tuple Unpacking Unit Tests (Task 4.1)
// Tests cover Requirements 10.1, 10.2, 10.4, 10.8, 10.10: tuple return,
// UNPACK_SEQUENCE, nested unpacking, for-loop unpacking, tuple literals.
// =============================================================================

// TestTupleUnpack_ReturnMultipleValues verifies that comma-separated return
// expressions compile as a tuple (Requirement 10.1).
func TestTupleUnpack_ReturnMultipleValues(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def swap(a, b):
    return b, a

result = swap(1, 2)
print(result)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "(2, 1)\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_ReturnTriple verifies returning three values as a tuple.
func TestTupleUnpack_ReturnTriple(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def triple():
    return 1, 2, 3

r = triple()
print(r)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "(1, 2, 3)\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_BasicAssignment verifies basic tuple unpacking in assignment
// (Requirement 10.2).
func TestTupleUnpack_BasicAssignment(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a, b = 1, 2
print(a)
print(b)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "1\n2\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_ThreeTargets verifies unpacking three values.
func TestTupleUnpack_ThreeTargets(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a, b, c = 10, 20, 30
print(a)
print(b)
print(c)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "10\n20\n30\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_ReturnAndUnpack verifies unpacking a function's multiple
// return values into separate variables.
func TestTupleUnpack_ReturnAndUnpack(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def divide(a, b):
    return a // b, a % b

q, r = divide(17, 5)
print(q)
print(r)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "3\n2\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_NestedUnpacking verifies nested unpacking targets
// (Requirement 10.4): a, (b, c) = [1, [2, 3]]
func TestTupleUnpack_NestedUnpacking(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a, (b, c) = [1, [2, 3]]
print(a)
print(b)
print(c)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "1\n2\n3\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_ForLoopUnpacking verifies for-loop tuple unpacking
// (Requirement 10.8).
func TestTupleUnpack_ForLoopUnpacking(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `pairs = [(1, 10), (2, 20), (3, 30)]
for a, b in pairs:
    print(a + b)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "11\n22\n33\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_TupleLiteralNoParens verifies that comma-separated
// expressions without parentheses create a tuple (Requirement 10.10).
func TestTupleUnpack_TupleLiteralNoParens(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = 1, 2, 3
print(x)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "(1, 2, 3)\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_SwapValues verifies swap via tuple unpacking
// (Requirement 10.9).
func TestTupleUnpack_SwapValues(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a = 1
b = 2
a, b = b, a
print(a)
print(b)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "2\n1\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_ValueErrorMismatch verifies that unpacking with wrong
// count raises ValueError (Requirement 10.3).
func TestTupleUnpack_ValueErrorMismatch(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a, b, c = 1, 2
`

	err := r.ExecString(source, "<test>")
	if err == nil {
		t.Fatal("expected ValueError for unpacking mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "not enough values to unpack") {
		t.Errorf("expected 'not enough values to unpack' error, got: %v", err)
	}
}

// TestTupleUnpack_UnpackFromList verifies unpacking from a list.
func TestTupleUnpack_UnpackFromList(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a, b, c = [10, 20, 30]
print(a)
print(b)
print(c)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "10\n20\n30\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_ForLoopThreeTargets verifies for-loop unpacking with three targets.
func TestTupleUnpack_ForLoopThreeTargets(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `triples = [(1, 2, 3), (4, 5, 6)]
for a, b, c in triples:
    print(a + b + c)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "6\n15\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_TrailingComma verifies a single element with trailing comma
// is treated as a 1-tuple.
func TestTupleUnpack_TrailingComma(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = 42,
print(x)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "(42,)\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// =============================================================================
// Swap Atomicity Tests (Task 4.3)
// Tests verify Requirement 10.9: RHS is fully evaluated before LHS stores.
// =============================================================================

// TestTupleUnpack_SwapTripleRotation verifies that a, b, c = b, c, a
// performs a correct triple rotation — all three RHS values are captured
// before any stores happen.
func TestTupleUnpack_SwapTripleRotation(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a = 1
b = 2
c = 3
a, b, c = b, c, a
print(a)
print(b)
print(c)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	// After rotation: a=2, b=3, c=1
	want := "2\n3\n1\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_SwapWithExpressions verifies atomicity when RHS contains
// expressions that reference the same variables being assigned to.
func TestTupleUnpack_SwapWithExpressions(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a = 5
b = 10
a, b = a + b, a - b
print(a)
print(b)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	// a+b=15, a-b=-5 (both computed from original a=5, b=10)
	want := "15\n-5\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_SwapSameVariable verifies that a, a = 1, 2 assigns
// left-to-right, so a ends up as 2.
func TestTupleUnpack_SwapSameVariable(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a, a = 1, 2
print(a)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	// UNPACK_SEQUENCE pushes in reverse, stores happen left-to-right:
	// a gets 1, then a gets 2 → final value is 2
	want := "2\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_FibonacciStep verifies the classic Fibonacci swap pattern
// a, b = b, a + b where both RHS values use the original values of a and b.
func TestTupleUnpack_FibonacciStep(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a = 1
b = 1
a, b = b, a + b
print(a)
print(b)
a, b = b, a + b
print(a)
print(b)
a, b = b, a + b
print(a)
print(b)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	// Step 1: a=1, b=2; Step 2: a=2, b=3; Step 3: a=3, b=5
	want := "1\n2\n2\n3\n3\n5\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// =============================================================================
// Starred Unpacking Tests (Task 4.5 — UNPACK_EX)
// Tests cover Requirements 10.5, 10.6, 10.7: starred unpacking, UNPACK_EX
// encoding, multiple-starred SyntaxError.
// =============================================================================

// TestTupleUnpack_StarredBasic verifies basic starred unpacking
// a, *b, c = [1, 2, 3, 4, 5] (Requirement 10.5).
func TestTupleUnpack_StarredBasic(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a, *b, c = [1, 2, 3, 4, 5]
print(a)
print(b)
print(c)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "1\n[2, 3, 4]\n5\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_StarredAtStart verifies starred target at the beginning
// *a, b = [1, 2, 3, 4] (Requirement 10.5).
func TestTupleUnpack_StarredAtStart(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `*a, b = [1, 2, 3, 4]
print(a)
print(b)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[1, 2, 3]\n4\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_StarredAtEnd verifies starred target at the end
// a, *b = [1, 2, 3, 4] (Requirement 10.5).
func TestTupleUnpack_StarredAtEnd(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a, *b = [1, 2, 3, 4]
print(a)
print(b)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "1\n[2, 3, 4]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_StarredEmptyList verifies starred target gets an empty list
// when there are exactly enough values for the non-starred targets
// a, *b, c = [1, 2] (Requirement 10.5).
func TestTupleUnpack_StarredEmptyList(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a, *b, c = [1, 2]
print(a)
print(b)
print(c)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "1\n[]\n2\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_StarredFromRange verifies starred unpacking from range()
// a, *b, c = range(5) (Requirement 10.5).
func TestTupleUnpack_StarredFromRange(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a, *b, c = range(5)
print(a)
print(b)
print(c)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "0\n[1, 2, 3]\n4\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_StarredFromTuple verifies starred unpacking from a tuple.
func TestTupleUnpack_StarredFromTuple(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a, *b, c = (10, 20, 30, 40)
print(a)
print(b)
print(c)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "10\n[20, 30]\n40\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_StarredSingleElement verifies starred gets exactly one
// element when there's one extra beyond the fixed targets.
func TestTupleUnpack_StarredSingleElement(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a, *b, c = [1, 2, 3]
print(a)
print(b)
print(c)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "1\n[2]\n3\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_StarredInsufficientValues verifies ValueError when there
// aren't enough values for the non-starred targets (Requirement 10.5).
func TestTupleUnpack_StarredInsufficientValues(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a, *b, c, d = [1]
`

	err := r.ExecString(source, "<test>")
	if err == nil {
		t.Fatal("expected ValueError for insufficient values with starred unpacking, got nil")
	}
	if !strings.Contains(err.Error(), "not enough values to unpack") {
		t.Errorf("expected 'not enough values to unpack' error, got: %v", err)
	}
}

// TestTupleUnpack_MultipleStarredSyntaxError verifies that multiple starred
// targets in one assignment raise SyntaxError (Requirement 10.7).
func TestTupleUnpack_MultipleStarredSyntaxError(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `*a, *b = [1, 2, 3]
`

	err := r.ExecString(source, "<test>")
	if err == nil {
		t.Fatal("expected SyntaxError for multiple starred expressions, got nil")
	}
	if !strings.Contains(err.Error(), "SyntaxError") || !strings.Contains(err.Error(), "multiple starred") {
		t.Errorf("expected SyntaxError about multiple starred expressions, got: %v", err)
	}
}

// TestTupleUnpack_ValueErrorTooManyValues verifies that unpacking with too
// many values (no starred) raises ValueError (Requirement 10.3).
func TestTupleUnpack_ValueErrorTooManyValues(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a, b = [1, 2, 3]
`

	err := r.ExecString(source, "<test>")
	if err == nil {
		t.Fatal("expected ValueError for too many values to unpack, got nil")
	}
	if !strings.Contains(err.Error(), "not enough values to unpack") &&
		!strings.Contains(err.Error(), "too many values to unpack") &&
		!strings.Contains(err.Error(), "ValueError") {
		t.Errorf("expected ValueError for mismatch, got: %v", err)
	}
}

// TestTupleUnpack_ForLoopNestedUnpacking verifies unpacking nested structures
// inside a for-loop (Requirements 10.4, 10.8).
func TestTupleUnpack_ForLoopNestedUnpacking(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `items = [(1, (2, 3)), (4, (5, 6))]
for a, (b, c) in items:
    print(a)
    print(b)
    print(c)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "1\n2\n3\n4\n5\n6\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_ReturnUnpackMultiple verifies unpacking multiple return
// values from a function into separate variables (Requirements 10.1, 10.2).
func TestTupleUnpack_ReturnUnpackMultiple(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def minmax(items):
    lo = items[0]
    hi = items[0]
    for x in items:
        if x < lo:
            lo = x
        if x > hi:
            hi = x
    return lo, hi

low, high = minmax([3, 1, 4, 1, 5, 9, 2, 6])
print(low)
print(high)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "1\n9\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_StarredOnlyTarget verifies starred as the only target
// *a, = [1, 2, 3] — starred gets all elements as a list.
func TestTupleUnpack_StarredOnlyTarget(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `*a, = [1, 2, 3]
print(a)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[1, 2, 3]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestTupleUnpack_DeepNested verifies deeply nested unpacking
// a, (b, (c, d)) = [1, [2, [3, 4]]] (Requirement 10.4).
func TestTupleUnpack_DeepNested(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `a, (b, (c, d)) = [1, [2, [3, 4]]]
print(a)
print(b)
print(c)
print(d)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "1\n2\n3\n4\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
