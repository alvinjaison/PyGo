package runtime

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// ============================================================
// Feature: pygo-post-mvp, Task 8.2 — Property 16
// **Validates: Requirements 5.1, 5.2, 5.6**
// ============================================================

// Property 16: Decorator Application Order
//
// For any stack of N decorators d0..dN-1 written top to bottom above a def:
//   - each decorator is *called* bottom-up, so dN-1 runs first and d0 last;
//   - the value bound to the name is d0(d1(...dN-1(func)...)), i.e. the topmost
//     decorator's return value is the final one.
//
// Both halves are observable: the build order is recorded as a side effect when
// each decorator runs, and the nesting is recorded in the wrapper's return value.

func TestProperty16_DecoratorApplicationOrder(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(t, "n")

		// Names are d0..dn-1 in source order, d0 being the topmost decorator.
		var src strings.Builder
		src.WriteString("built = []\n")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&src, `
def d%d(fn):
    built.append("d%d")

    def w():
        return "d%d(" + fn() + ")"
    return w
`, i, i, i)
		}

		for i := 0; i < n; i++ {
			fmt.Fprintf(&src, "@d%d\n", i)
		}
		src.WriteString("def target():\n    return \"T\"\n")
		src.WriteString("print(built)\n")
		src.WriteString("print(target())\n")

		var buf bytes.Buffer
		r := New(WithOutput(&buf))
		source := src.String()
		if err := r.ExecString(source, "<test>"); err != nil {
			t.Fatalf("execution failed: %v\nsource:\n%s", err, source)
		}

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
		}

		// Applied bottom-up: the decorator nearest the def runs first.
		wantBuild := make([]string, n)
		for i := 0; i < n; i++ {
			wantBuild[i] = fmt.Sprintf("'d%d'", n-1-i)
		}
		wantBuildStr := "[" + strings.Join(wantBuild, ", ") + "]"
		if lines[0] != wantBuildStr {
			t.Fatalf("decorator call order wrong:\ngot:  %s\nwant: %s\nsource:\n%s",
				lines[0], wantBuildStr, source)
		}

		// Nesting: d0 outermost, dN-1 innermost around the original function.
		want := "T"
		for i := n - 1; i >= 0; i-- {
			want = fmt.Sprintf("d%d(%s)", i, want)
		}
		if lines[1] != want {
			t.Fatalf("decorator nesting wrong:\ngot:  %s\nwant: %s\nsource:\n%s",
				lines[1], want, source)
		}
	})
}

// The topmost decorator decides the final bound value, even when it discards
// everything below it.
func TestProperty16_TopmostDecoratorWins(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 4).Draw(t, "n")
		sentinel := rapid.IntRange(-500, 500).Draw(t, "sentinel")

		var src strings.Builder
		fmt.Fprintf(&src, "def top(fn):\n    return %d\n", sentinel)
		for i := 0; i < n; i++ {
			fmt.Fprintf(&src, `
def d%d(fn):
    def w():
        return fn()
    return w
`, i)
		}
		src.WriteString("@top\n")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&src, "@d%d\n", i)
		}
		src.WriteString("def target():\n    return 0\n")
		src.WriteString("print(target)\n")

		var buf bytes.Buffer
		r := New(WithOutput(&buf))
		source := src.String()
		if err := r.ExecString(source, "<test>"); err != nil {
			t.Fatalf("execution failed: %v\nsource:\n%s", err, source)
		}

		got := strings.TrimSpace(buf.String())
		want := fmt.Sprintf("%d", sentinel)
		if got != want {
			t.Fatalf("topmost decorator's value not bound: got %q, want %q\nsource:\n%s",
				got, want, source)
		}
	})
}
