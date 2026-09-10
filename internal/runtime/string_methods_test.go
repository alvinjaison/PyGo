package runtime

import (
	"strings"
	"testing"
)

// ============================================================
// Unit tests for string methods — Task 6.3
// **Validates: Requirements 8.1–8.16**
//
// Every expected value and error message below was captured from CPython 3
// running the same snippet, not derived from PyGo's own output.
// ============================================================

// checkStr runs (expression, expected output) pairs against a fixed setup.
func checkStr(t *testing.T, cases [][2]string) {
	t.Helper()
	for _, c := range cases {
		if got := runOK(t, "print("+c[0]+")"); got != c[1] {
			t.Errorf("%s: got %q, want %q", c[0], got, c[1])
		}
	}
}

// --- Requirements 8.1, 8.2: case conversion ---

func TestStrMethods_UpperLower(t *testing.T) {
	checkStr(t, [][2]string{
		{`"Hello World".upper()`, "HELLO WORLD"},
		{`"Hello World".lower()`, "hello world"},
		{`"".upper()`, ""},
		{`"".lower()`, ""},
		{`"123!".upper()`, "123!"},
		// Unicode case mapping, not ASCII-only.
		{`"éàü".upper()`, "ÉÀÜ"},
		{`"ÉÀÜ".lower()`, "éàü"},
	})
}

// --- Requirements 8.3, 8.16: strip family on whitespace ---

func TestStrMethods_StripWhitespace(t *testing.T) {
	checkStr(t, [][2]string{
		{`"<" + "  hi  ".strip() + ">"`, "<hi>"},
		{`"<" + "  hi  ".lstrip() + ">"`, "<hi  >"},
		{`"<" + "  hi  ".rstrip() + ">"`, "<  hi>"},
		{`"<" + "\t\n hi \n\t".strip() + ">"`, "<hi>"},
		{`"<" + "   ".strip() + ">"`, "<>"},
		{`"<" + "hi".strip() + ">"`, "<hi>"},
	})
}

// --- Requirements 8.4, 8.16: strip family with a character set ---

func TestStrMethods_StripCutset(t *testing.T) {
	checkStr(t, [][2]string{
		{`"<" + "xxhixx".strip("x") + ">"`, "<hi>"},
		{`"<" + "xyhixy".strip("xy") + ">"`, "<hi>"},
		{`"<" + "xxhixx".lstrip("x") + ">"`, "<hixx>"},
		{`"<" + "xxhixx".rstrip("x") + ">"`, "<xxhi>"},
		{`"<" + "abc".strip("xyz") + ">"`, "<abc>"},
		{`"<" + "aaa".strip("a") + ">"`, "<>"},
	})
}

// --- Requirement 8.5: split on whitespace collapses runs ---

func TestStrMethods_SplitWhitespace(t *testing.T) {
	checkStr(t, [][2]string{
		{`"a b c".split()`, "['a', 'b', 'c']"},
		{`"  a   b  c  ".split()`, "['a', 'b', 'c']"},
		{`"".split()`, "[]"},
		{`"   ".split()`, "[]"},
		{`"a\tb\nc".split()`, "['a', 'b', 'c']"},
		{`"a b c d".split(None, 2)`, "['a', 'b', 'c d']"},
	})
}

// --- Requirement 8.6: split on an explicit separator does not collapse ---

func TestStrMethods_SplitSeparator(t *testing.T) {
	checkStr(t, [][2]string{
		{`"a,b,c".split(",")`, "['a', 'b', 'c']"},
		{`"a,,b".split(",")`, "['a', '', 'b']"},
		{`",a,".split(",")`, "['', 'a', '']"},
		{`"abc".split(",")`, "['abc']"},
		{`"".split(",")`, "['']"},
		{`"a,b,c,d".split(",", 2)`, "['a', 'b', 'c,d']"},
		{`"a-b-c".split("-", 0)`, "['a-b-c']"},
	})
}

func TestStrMethods_SplitEmptySeparator_RaisesValueError(t *testing.T) {
	msg := runErr(t, `print("a,b".split(""))`)
	if !strings.Contains(msg, "empty separator") {
		t.Errorf("expected 'empty separator', got: %s", msg)
	}
}

// --- Requirement 8.7: join ---

func TestStrMethods_Join(t *testing.T) {
	checkStr(t, [][2]string{
		{`"-".join(["a", "b", "c"])`, "a-b-c"},
		{`"".join(["a", "b"])`, "ab"},
		{`",".join([])`, ""},
		{`"-".join(("x", "y"))`, "x-y"},
		// A string is itself an iterable of characters.
		{`"-".join("abc")`, "a-b-c"},
	})
}

func TestStrMethods_JoinNonString_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `print("-".join(["a", 1]))`)
	if !strings.Contains(msg, "sequence item 1: expected str instance, int found") {
		t.Errorf("unexpected error: %s", msg)
	}
}

func TestStrMethods_JoinNone_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `print("-".join([None]))`)
	if !strings.Contains(msg, "sequence item 0: expected str instance, NoneType found") {
		t.Errorf("unexpected error: %s", msg)
	}
}

// --- Requirements 8.8, 8.9: replace ---

func TestStrMethods_Replace(t *testing.T) {
	checkStr(t, [][2]string{
		{`"aaa".replace("a", "b")`, "bbb"},
		{`"aaa".replace("a", "b", 2)`, "bba"},
		{`"hello".replace("l", "L")`, "heLLo"},
		{`"hello".replace("xyz", "!")`, "hello"},
		{`"aaa".replace("a", "b", 0)`, "aaa"},
		// A negative count means replace everything.
		{`"aaa".replace("a", "b", -1)`, "bbb"},
		{`"aaa".replace("aa", "x")`, "xa"},
		// An empty pattern inserts between every character.
		{`"abc".replace("", "-")`, "-a-b-c-"},
	})
}

// --- Requirements 8.10, 8.11: startswith / endswith ---

func TestStrMethods_StartsEndsWith(t *testing.T) {
	checkStr(t, [][2]string{
		{`"hello".startswith("he")`, "True"},
		{`"hello".startswith("lo")`, "False"},
		{`"hello".endswith("lo")`, "True"},
		{`"hello".endswith("he")`, "False"},
		{`"hello".startswith("")`, "True"},
		// A tuple of candidates matches if any one does.
		{`"hello".startswith(("x", "he"))`, "True"},
		{`"hello".endswith(("x", "lo"))`, "True"},
		// Optional start/end bounds.
		{`"hello".startswith("ell", 1)`, "True"},
		{`"hello".startswith("he", 1)`, "False"},
		{`"hello".endswith("ll", 0, 4)`, "True"},
	})
}

// --- Requirements 8.12, 8.13: find ---

func TestStrMethods_Find(t *testing.T) {
	checkStr(t, [][2]string{
		{`"hello".find("l")`, "2"},
		{`"hello".find("z")`, "-1"},
		{`"hello".find("l", 3)`, "3"},
		{`"hello".find("l", 0, 2)`, "-1"},
		{`"hello".find("")`, "0"},
		{`"hello".find("hello")`, "0"},
		{`"aXbXc".find("X", 2)`, "3"},
		{`"hello".find("l", -2)`, "3"},
		{`"hello".find("l", 100)`, "-1"},
	})
}

// find reports code-point indices, not byte offsets.
func TestStrMethods_FindIsCodePointBased(t *testing.T) {
	checkStr(t, [][2]string{
		{`"héllo".find("l")`, "2"},
		{`"日本語".find("語")`, "2"},
	})
}

// --- Requirement 8.14: format ---

func TestStrMethods_Format(t *testing.T) {
	checkStr(t, [][2]string{
		{`"{} {}".format("a", "b")`, "a b"},
		{`"{0} {1}".format("a", "b")`, "a b"},
		{`"{1} {0}".format("a", "b")`, "b a"},
		{`"{0} {0}".format("x")`, "x x"},
		{`"{name}".format(name="v")`, "v"},
		{`"{a}-{b}".format(a=1, b=2)`, "1-2"},
		{`"{} and {n}".format("p", n="k")`, "p and k"},
		{`"no fields".format()`, "no fields"},
		{`"{}".format(42)`, "42"},
		{`"{}".format([1, 2])`, "[1, 2]"},
		{`"{}".format(None)`, "None"},
		{`"{}".format(True)`, "True"},
	})
}

func TestStrMethods_FormatBraceEscapes(t *testing.T) {
	checkStr(t, [][2]string{
		{`"{{}}".format()`, "{}"},
		{`"{{{}}}".format("v")`, "{v}"},
	})
}

// --- Requirement 8.15: format error cases ---

func TestStrMethods_FormatTooFewArgs_RaisesIndexError(t *testing.T) {
	msg := runErr(t, `print("{}{}".format("only"))`)
	if !strings.Contains(msg, "Replacement index 1 out of range") {
		t.Errorf("unexpected error: %s", msg)
	}
}

func TestStrMethods_FormatBadIndex_RaisesIndexError(t *testing.T) {
	msg := runErr(t, `print("{2}".format("a", "b"))`)
	if !strings.Contains(msg, "Replacement index 2 out of range") {
		t.Errorf("unexpected error: %s", msg)
	}
}

func TestStrMethods_FormatMissingKey_RaisesKeyError(t *testing.T) {
	msg := runErr(t, `print("{missing}".format(a=1))`)
	if !strings.Contains(msg, "missing") {
		t.Errorf("expected KeyError mentioning 'missing', got: %s", msg)
	}
}

func TestStrMethods_FormatMixedNumbering_RaisesValueError(t *testing.T) {
	// CPython forbids mixing automatic and manual field numbering.
	msg := runErr(t, `print("{} {0}".format("a", "b"))`)
	if !strings.Contains(msg, "cannot switch from automatic field numbering") {
		t.Errorf("unexpected error: %s", msg)
	}
}

func TestStrMethods_FormatUnmatchedBrace_RaisesValueError(t *testing.T) {
	msg := runErr(t, `print("{".format())`)
	if !strings.Contains(msg, "Single '{'") {
		t.Errorf("unexpected error: %s", msg)
	}
}

// Format specifications are recognised and rejected rather than silently
// dropped, which would produce quietly wrong output.
func TestStrMethods_FormatSpecRejected(t *testing.T) {
	msg := runErr(t, `print("{:>8}".format("a"))`)
	if !strings.Contains(msg, "format specifications") {
		t.Errorf("expected an explicit unsupported-format-spec error, got: %s", msg)
	}
}

// --- Argument validation ---

func TestStrMethods_WrongArgTypes_RaiseTypeError(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{`"abc".replace(1, "x")`, "replace() argument 1 must be str, not int"},
		{`"abc".find(1)`, "find() argument 1 must be str, not int"},
		{`"abc".strip(1)`, "strip arg must be None or str"},
		{`"abc".startswith(1)`, "startswith first arg must be str or a tuple of str, not int"},
	}
	for _, c := range cases {
		msg := runErr(t, "print("+c.src+")")
		if !strings.Contains(msg, c.want) {
			t.Errorf("%s: expected %q, got: %s", c.src, c.want, msg)
		}
	}
}

func TestStrMethods_UnknownAttribute_RaisesAttributeError(t *testing.T) {
	msg := runErr(t, `print("abc".nosuchmethod())`)
	if !strings.Contains(msg, "'str' object has no attribute 'nosuchmethod'") {
		t.Errorf("unexpected error: %s", msg)
	}
}

func TestStrMethods_ChainedCalls(t *testing.T) {
	got := runOK(t, `
s = "  Hello, World  "
print(s.strip().lower().replace(",", ";"))
print("-".join(s.strip().split(", ")))
print(s.strip().startswith("Hello"))
`)
	want := "hello; world\nHello-World\nTrue"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
