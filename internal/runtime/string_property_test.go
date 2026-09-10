package runtime

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// ============================================================
// Feature: pygo-post-mvp, Task 6.2 — Properties 23–27 (string methods)
// **Validates: Requirements 8.1–8.13, 8.16**
// ============================================================

// strAlphabet covers ASCII letters, digits, whitespace and multi-byte code
// points so the methods are exercised on characters rather than bytes. Every
// character is safe to embed directly in a double-quoted Python literal.
var strAlphabet = []string{
	"a", "b", "Z", "Q", "0", "7", " ", "\t", "-", ",", "é", "Ü", "日", "🙂",
}

// drawStr draws a string of up to max characters.
func drawStr(t *rapid.T, label string, max int) string {
	n := rapid.IntRange(0, max).Draw(t, label+"_len")
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString(rapid.SampledFrom(strAlphabet).Draw(t, fmt.Sprintf("%s_%d", label, i)))
	}
	return sb.String()
}

// runProp executes source and returns its output split into lines. Unlike the
// helpers elsewhere it does not trim, so trailing empty lines survive.
func runProp(t *rapid.T, source string) []string {
	t.Helper()
	var buf strings.Builder
	r := New(WithOutput(&buf))
	if err := r.ExecString(source, "<test>"); err != nil {
		t.Fatalf("execution failed: %v\nsource:\n%s", err, source)
	}
	out := buf.String()
	out = strings.TrimSuffix(out, "\n")
	return strings.Split(out, "\n")
}

// ============================================================
// Property 23: String split/join Round-Trip
// ============================================================
//
// For any separator sep and any list of strings none of which contain sep,
// sep.join(parts).split(sep) SHALL reproduce parts.

func TestProperty23_SplitJoinRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sep := rapid.SampledFrom([]string{",", "-", "::", "|", "日"}).Draw(t, "sep")

		n := rapid.IntRange(1, 5).Draw(t, "n")
		parts := make([]string, n)
		for i := 0; i < n; i++ {
			// Draw from an alphabet that excludes the separator so the
			// round-trip is well defined.
			var sb strings.Builder
			length := rapid.IntRange(0, 4).Draw(t, fmt.Sprintf("plen%d", i))
			for j := 0; j < length; j++ {
				c := rapid.SampledFrom([]string{"a", "b", "Z", "0", "é", "🙂"}).
					Draw(t, fmt.Sprintf("p%d_%d", i, j))
				sb.WriteString(c)
			}
			parts[i] = sb.String()
		}

		literals := make([]string, n)
		for i, p := range parts {
			literals[i] = `"` + p + `"`
		}

		source := fmt.Sprintf(`
parts = [%s]
sep = "%s"
joined = sep.join(parts)
back = joined.split(sep)
print(back == parts)
print(len(back))
`, strings.Join(literals, ", "), sep)

		lines := runProp(t, source)
		if lines[0] != "True" {
			t.Fatalf("split/join round-trip failed for sep=%q parts=%q\nsource:\n%s",
				sep, parts, source)
		}
		if lines[1] != fmt.Sprintf("%d", n) {
			t.Fatalf("expected %d parts back, got %s\nsource:\n%s", n, lines[1], source)
		}
	})
}

// ============================================================
// Property 24: String Case Conversion Idempotence
// ============================================================
//
// upper() and lower() SHALL be idempotent: s.upper().upper() == s.upper() and
// s.lower().lower() == s.lower().

func TestProperty24_CaseConversionIdempotence(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := drawStr(t, "s", 8)
		source := fmt.Sprintf(`
s = "%s"
print(s.upper().upper() == s.upper())
print(s.lower().lower() == s.lower())
print(len(s.upper()) >= len(s))
print(len(s.lower()) >= len(s))
`, s)

		lines := runProp(t, source)
		labels := []string{
			"upper idempotence", "lower idempotence",
			"upper length", "lower length",
		}
		for i, label := range labels {
			if lines[i] != "True" {
				t.Fatalf("%s failed for %q\nsource:\n%s", label, s, source)
			}
		}
	})
}

// ============================================================
// Property 25: String Strip Invariant
// ============================================================
//
// strip() SHALL be idempotent and SHALL never lengthen the string. Stripping
// both ends SHALL equal lstrip followed by rstrip.

func TestProperty25_StripInvariant(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := drawStr(t, "s", 8)
		useCutset := rapid.Bool().Draw(t, "useCutset")

		arg := ""
		if useCutset {
			cutset := drawStr(t, "cut", 3)
			arg = `"` + cutset + `"`
		}

		source := fmt.Sprintf(`
s = "%s"
stripped = s.strip(%s)
print(stripped.strip(%s) == stripped)
print(len(stripped) <= len(s))
print(s.lstrip(%s).rstrip(%s) == stripped)
print(len(s.lstrip(%s)) <= len(s))
print(len(s.rstrip(%s)) <= len(s))
`, s, arg, arg, arg, arg, arg, arg)

		lines := runProp(t, source)
		labels := []string{
			"strip idempotence",
			"strip never lengthens",
			"lstrip+rstrip equals strip",
			"lstrip never lengthens",
			"rstrip never lengthens",
		}
		for i, label := range labels {
			if lines[i] != "True" {
				t.Fatalf("%s failed for %q arg=%s\nsource:\n%s", label, s, arg, source)
			}
		}
	})
}

// ============================================================
// Property 26: String Prefix/Suffix Consistency
// ============================================================
//
// For any strings a and b, (a+b).startswith(a) and (a+b).endswith(b) SHALL both
// hold, and every string SHALL start and end with the empty string and itself.

func TestProperty26_PrefixSuffixConsistency(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := drawStr(t, "a", 5)
		b := drawStr(t, "b", 5)

		source := fmt.Sprintf(`
a = "%s"
b = "%s"
c = a + b
print(c.startswith(a))
print(c.endswith(b))
print(c.startswith(""))
print(c.endswith(""))
print(c.startswith(c))
print(c.endswith(c))
`, a, b)

		lines := runProp(t, source)
		labels := []string{
			"(a+b).startswith(a)", "(a+b).endswith(b)",
			"startswith empty", "endswith empty",
			"startswith self", "endswith self",
		}
		for i, label := range labels {
			if lines[i] != "True" {
				t.Fatalf("%s failed for a=%q b=%q\nsource:\n%s", label, a, b, source)
			}
		}
	})
}

// ============================================================
// Property 27: String find Consistency
// ============================================================
//
// find(sub) SHALL return -1 exactly when sub is absent, and otherwise return an
// index at which sub actually occurs. The result SHALL agree with the `in`
// operator, and indices SHALL be code-point based so that slicing at the
// returned index recovers sub.

func TestProperty27_FindConsistency(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := drawStr(t, "s", 8)
		sub := drawStr(t, "sub", 3)

		source := fmt.Sprintf(`
s = "%s"
sub = "%s"
i = s.find(sub)
print(i)
print(sub in s)
if i >= 0:
    print(s[i:i + len(sub)] == sub)
else:
    print("skipped")
`, s, sub)

		lines := runProp(t, source)

		idx := lines[0]
		contained := lines[1]

		found := idx != "-1"
		if found != (contained == "True") {
			t.Fatalf("find/in disagree for s=%q sub=%q: find=%s in=%s\nsource:\n%s",
				s, sub, idx, contained, source)
		}
		if found {
			// The returned index must be a code-point index, so slicing there
			// has to reproduce sub.
			if lines[2] != "True" {
				t.Fatalf("find index is not a code-point index for s=%q sub=%q (find=%s)\nsource:\n%s",
					s, sub, idx, source)
			}
		}
	})
}
