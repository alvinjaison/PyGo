package runtime

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// ============================================================
// Feature: pygo-post-mvp, Property 31: Tuple Unpacking Completeness
// ============================================================
// **Validates: Requirements 10.2**
//
// For any tuple of N elements and N assignment targets, unpacking SHALL
// bind the i-th target to the i-th element for all i in [0, N).

func TestProperty31_TupleUnpackingCompleteness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate 2-6 integer values to unpack
		n := rapid.IntRange(2, 6).Draw(t, "n")
		vals := make([]int, n)
		for i := 0; i < n; i++ {
			vals[i] = rapid.IntRange(-500, 500).Draw(t, fmt.Sprintf("val_%d", i))
		}

		// Build variable names: a, b, c, d, e, f
		varNames := []string{"a", "b", "c", "d", "e", "f"}[:n]

		// Build source: a, b, c = (v0, v1, v2)
		var sb strings.Builder

		// Build the RHS tuple literal
		rhsParts := make([]string, n)
		for i := 0; i < n; i++ {
			rhsParts[i] = fmt.Sprintf("%d", vals[i])
		}

		sb.WriteString(strings.Join(varNames, ", "))
		sb.WriteString(" = ")
		sb.WriteString(strings.Join(rhsParts, ", "))
		sb.WriteString("\n")

		// Print each variable to verify correct assignment
		for _, v := range varNames {
			sb.WriteString(fmt.Sprintf("print(%s)\n", v))
		}

		source := sb.String()

		// Compute expected output
		expected := make([]string, n)
		for i := 0; i < n; i++ {
			expected[i] = fmt.Sprintf("%d", vals[i])
		}

		var buf bytes.Buffer
		r := New(WithOutput(&buf))
		err := r.ExecString(source, "<test>")
		if err != nil {
			t.Fatalf("execution failed: %v\nsource:\n%s", err, source)
		}

		output := strings.TrimSpace(buf.String())
		expectedStr := strings.Join(expected, "\n")
		if output != expectedStr {
			t.Fatalf("tuple unpacking completeness failed:\ngot:  %q\nwant: %q\nsource:\n%s",
				output, expectedStr, source)
		}
	})
}

// ============================================================
// Feature: pygo-post-mvp, Property 32: Starred Unpacking Correctness
// ============================================================
// **Validates: Requirements 10.5**
//
// For any sequence of length L and starred unpacking with B targets before
// the star and A targets after, the starred target SHALL receive a list of
// exactly L - B - A elements.

func TestProperty32_StarredUnpackingCorrectness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate the number of targets before and after the star.
		// At least one of before or after must be >= 1, otherwise the
		// LHS is just `*x` which is not valid Python — starred unpacking
		// requires a tuple context (e.g. `*x, = ...` or `a, *x = ...`).
		before := rapid.IntRange(0, 3).Draw(t, "before")
		after := rapid.IntRange(0, 3).Draw(t, "after")
		if before+after == 0 {
			// Force at least one companion target by adding one after
			after = 1
		}

		// Total elements must be at least before + after
		minLen := before + after
		totalLen := rapid.IntRange(minLen, minLen+5).Draw(t, "totalLen")

		// Generate integer values
		vals := make([]int, totalLen)
		for i := 0; i < totalLen; i++ {
			vals[i] = rapid.IntRange(-500, 500).Draw(t, fmt.Sprintf("val_%d", i))
		}

		// Build variable names for before-targets, starred, after-targets
		nameIdx := 0
		allNames := []string{"a", "b", "c", "d", "e", "f", "g", "h"}

		beforeNames := make([]string, before)
		for i := 0; i < before; i++ {
			beforeNames[i] = allNames[nameIdx]
			nameIdx++
		}

		starName := allNames[nameIdx]
		nameIdx++

		afterNames := make([]string, after)
		for i := 0; i < after; i++ {
			afterNames[i] = allNames[nameIdx]
			nameIdx++
		}

		// Build the LHS: a, b, *c, d, e
		var lhsParts []string
		lhsParts = append(lhsParts, beforeNames...)
		lhsParts = append(lhsParts, "*"+starName)
		lhsParts = append(lhsParts, afterNames...)

		// Build the RHS: [v0, v1, v2, ...]
		rhsParts := make([]string, totalLen)
		for i := 0; i < totalLen; i++ {
			rhsParts[i] = fmt.Sprintf("%d", vals[i])
		}

		var sb strings.Builder
		sb.WriteString(strings.Join(lhsParts, ", "))
		sb.WriteString(" = [")
		sb.WriteString(strings.Join(rhsParts, ", "))
		sb.WriteString("]\n")

		// Print before-targets
		for _, v := range beforeNames {
			sb.WriteString(fmt.Sprintf("print(%s)\n", v))
		}
		// Print the starred target (as repr to see it's a list)
		sb.WriteString(fmt.Sprintf("print(%s)\n", starName))
		// Print after-targets
		for _, v := range afterNames {
			sb.WriteString(fmt.Sprintf("print(%s)\n", v))
		}

		source := sb.String()

		// Compute expected output
		var expected []string

		// Before values
		for i := 0; i < before; i++ {
			expected = append(expected, fmt.Sprintf("%d", vals[i]))
		}

		// Starred values — a list of the middle portion
		starredVals := vals[before : totalLen-after]
		if len(starredVals) == 0 {
			expected = append(expected, "[]")
		} else {
			parts := make([]string, len(starredVals))
			for i, v := range starredVals {
				parts[i] = fmt.Sprintf("%d", v)
			}
			expected = append(expected, "["+strings.Join(parts, ", ")+"]")
		}

		// After values
		for i := totalLen - after; i < totalLen; i++ {
			expected = append(expected, fmt.Sprintf("%d", vals[i]))
		}

		var buf bytes.Buffer
		r := New(WithOutput(&buf))
		err := r.ExecString(source, "<test>")
		if err != nil {
			t.Fatalf("execution failed: %v\nsource:\n%s", err, source)
		}

		output := strings.TrimSpace(buf.String())
		expectedStr := strings.Join(expected, "\n")
		if output != expectedStr {
			t.Fatalf("starred unpacking failed:\ngot:  %q\nwant: %q\nsource:\n%s",
				output, expectedStr, source)
		}
	})
}

// ============================================================
// Feature: pygo-post-mvp, Property 33: Swap Atomicity via Tuple Unpacking
// ============================================================
// **Validates: Requirements 10.9**
//
// For any two values a and b, executing `a, b = b, a` SHALL result in
// the original value of b in a and the original value of a in b.

func TestProperty33_SwapAtomicityViaTupleUnpacking(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate two distinct integer values
		a := rapid.IntRange(-1000, 1000).Draw(t, "a")
		b := rapid.IntRange(-1000, 1000).Draw(t, "b")

		source := fmt.Sprintf(`a = %d
b = %d
a, b = b, a
print(a)
print(b)
`, a, b)

		// After swap: a should hold original b, b should hold original a
		var expected []string
		expected = append(expected, fmt.Sprintf("%d", b))
		expected = append(expected, fmt.Sprintf("%d", a))

		var buf bytes.Buffer
		r := New(WithOutput(&buf))
		err := r.ExecString(source, "<test>")
		if err != nil {
			t.Fatalf("execution failed: %v\nsource:\n%s", err, source)
		}

		output := strings.TrimSpace(buf.String())
		expectedStr := strings.Join(expected, "\n")
		if output != expectedStr {
			t.Fatalf("swap atomicity failed:\ngot:  %q\nwant: %q\nsource:\n%s",
				output, expectedStr, source)
		}
	})
}
