package runtime

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// ============================================================
// Feature: pygo-post-mvp, Task 5.4 — Properties 28–30 (slicing)
// **Validates: Requirements 9.2, 9.3, 9.5, 9.6**
// ============================================================

// execSource runs source and returns its trimmed stdout.
func execSource(t *rapid.T, source string) string {
	t.Helper()
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	if err := r.ExecString(source, "<test>"); err != nil {
		t.Fatalf("execution failed: %v\nsource:\n%s", err, source)
	}
	return strings.TrimSpace(buf.String())
}

// pyIntList renders a Go int slice as a Python list literal.
func pyIntList(vals []int) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = strconv.Itoa(v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// pyIntTuple renders a Go int slice as a Python tuple literal, handling the
// empty and single-element forms that need special syntax.
func pyIntTuple(vals []int) string {
	switch len(vals) {
	case 0:
		return "()"
	case 1:
		return "(" + strconv.Itoa(vals[0]) + ",)"
	}
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = strconv.Itoa(v)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// drawInts draws a sequence of n integers.
func drawInts(t *rapid.T, n int) []int {
	vals := make([]int, n)
	for i := 0; i < n; i++ {
		vals[i] = rapid.IntRange(-100, 100).Draw(t, fmt.Sprintf("v%d", i))
	}
	return vals
}

// sliceSafeAlphabet keeps generated strings easy to embed in a Python literal
// while still covering multi-byte code points, so slicing is exercised on
// characters rather than bytes.
var sliceSafeAlphabet = []string{"a", "b", "z", "0", " ", "é", "ß", "日", "🙂"}

// drawPyString draws a string and returns both the Python literal and the
// expected value.
func drawPyString(t *rapid.T, n int) (literal, value string) {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString(rapid.SampledFrom(sliceSafeAlphabet).Draw(t, fmt.Sprintf("c%d", i)))
	}
	value = sb.String()
	return `"` + value + `"`, value
}

// ============================================================
// Property 28: Slice Reversal Round-Trip
// ============================================================
//
// For any sequence s, reversing twice SHALL yield the original sequence:
// s[::-1][::-1] == s. Verified for list, tuple and str.

func TestProperty28_SliceReversalRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 8).Draw(t, "n")
		vals := drawInts(t, n)
		listLit := pyIntList(vals)
		tupleLit := pyIntTuple(vals)
		strLit, strVal := drawPyString(t, n)

		source := fmt.Sprintf(`
xs = %s
ts = %s
ss = %s
print(xs[::-1][::-1] == xs)
print(ts[::-1][::-1] == ts)
print(ss[::-1][::-1] == ss)
print(xs[::-1][::-1])
print("<" + ss[::-1][::-1] + ">")
`, listLit, tupleLit, strLit)

		got := execSource(t, source)
		lines := strings.Split(got, "\n")
		if len(lines) != 5 {
			t.Fatalf("expected 5 lines, got %d: %q", len(lines), got)
		}
		for i, kind := range []string{"list", "tuple", "str"} {
			if lines[i] != "True" {
				t.Fatalf("%s reversal round-trip failed: got %q\nsource:\n%s",
					kind, lines[i], source)
			}
		}
		// Also confirm the round-tripped values, not just the equality result,
		// so a broken == cannot mask a broken slice.
		if lines[3] != listLit {
			t.Fatalf("list round-trip value: got %q, want %q", lines[3], listLit)
		}
		if wantStr := "<" + strVal + ">"; lines[4] != wantStr {
			t.Fatalf("str round-trip value: got %q, want %q", lines[4], wantStr)
		}
	})
}

// ============================================================
// Property 29: Slice Default Identity
// ============================================================
//
// For any sequence s, a slice that omits all bounds SHALL reproduce s:
// s[:] == s and s[::1] == s. The copy is equal but, for lists, not the
// same object.

func TestProperty29_SliceDefaultIdentity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 8).Draw(t, "n")
		vals := drawInts(t, n)
		listLit := pyIntList(vals)
		tupleLit := pyIntTuple(vals)
		strLit, strVal := drawPyString(t, n)

		source := fmt.Sprintf(`
xs = %s
ts = %s
ss = %s
print(xs[:] == xs)
print(xs[::1] == xs)
print(ts[:] == ts)
print(ts[::1] == ts)
print(ss[:] == ss)
print(ss[::1] == ss)
print(xs[:])
print("<" + ss[:] + ">")
`, listLit, tupleLit, strLit)

		got := execSource(t, source)
		lines := strings.Split(got, "\n")
		if len(lines) != 8 {
			t.Fatalf("expected 8 lines, got %d: %q", len(lines), got)
		}
		labels := []string{
			"list s[:]", "list s[::1]",
			"tuple s[:]", "tuple s[::1]",
			"str s[:]", "str s[::1]",
		}
		for i, label := range labels {
			if lines[i] != "True" {
				t.Fatalf("%s identity failed: got %q\nsource:\n%s", label, lines[i], source)
			}
		}
		if lines[6] != listLit {
			t.Fatalf("list copy value: got %q, want %q", lines[6], listLit)
		}
		if wantStr := "<" + strVal + ">"; lines[7] != wantStr {
			t.Fatalf("str copy value: got %q, want %q", lines[7], wantStr)
		}
	})
}

// ============================================================
// Property 30: Slice Index Clipping
// ============================================================
//
// Slice bounds are clamped rather than validated, so a slice SHALL never raise
// IndexError however far out of range its bounds are. The result is always no
// longer than the original, is drawn from the original's elements, and for
// step 1 is a contiguous run of the original.

func TestProperty30_SliceIndexClipping(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 6).Draw(t, "n")
		vals := drawInts(t, n)
		listLit := pyIntList(vals)

		start := rapid.IntRange(-25, 25).Draw(t, "start")
		stop := rapid.IntRange(-25, 25).Draw(t, "stop")
		// Step 0 is excluded: it is a ValueError by design, not an IndexError.
		step := rapid.SampledFrom([]int{1, 2, 3, -1, -2, -3}).Draw(t, "step")

		source := fmt.Sprintf(`
xs = %s
r = xs[%d:%d:%d]
print(len(r))
for v in r:
    print(v)
`, listLit, start, stop, step)

		// The central assertion: this must not raise.
		got := execSource(t, source)

		lines := strings.Split(got, "\n")
		length, err := strconv.Atoi(strings.TrimSpace(lines[0]))
		if err != nil {
			t.Fatalf("could not parse length from %q\nsource:\n%s", got, source)
		}
		if length > n {
			t.Fatalf("slice longer than source: len=%d, n=%d\nsource:\n%s", length, n, source)
		}

		result := make([]int, 0, length)
		for _, line := range lines[1:] {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			v, err := strconv.Atoi(line)
			if err != nil {
				t.Fatalf("could not parse element %q\nsource:\n%s", line, source)
			}
			result = append(result, v)
		}
		if len(result) != length {
			t.Fatalf("len() reported %d but %d elements were printed\nsource:\n%s",
				length, len(result), source)
		}

		// Every element must come from the source, respecting multiplicity.
		remaining := make(map[int]int, n)
		for _, v := range vals {
			remaining[v]++
		}
		for _, v := range result {
			if remaining[v] == 0 {
				t.Fatalf("slice produced element %d absent from source %v\nsource:\n%s",
					v, vals, source)
			}
			remaining[v]--
		}

		// For step 1 the result must be a contiguous run of the original.
		if step == 1 && length > 0 {
			if !containsRun(vals, result) {
				t.Fatalf("step-1 slice %v is not a contiguous run of %v\nsource:\n%s",
					result, vals, source)
			}
		}
	})
}

// containsRun reports whether run appears as a contiguous subsequence of vals.
func containsRun(vals, run []int) bool {
	if len(run) > len(vals) {
		return false
	}
	for offset := 0; offset+len(run) <= len(vals); offset++ {
		match := true
		for i, v := range run {
			if vals[offset+i] != v {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
