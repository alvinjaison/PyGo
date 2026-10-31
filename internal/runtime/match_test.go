package runtime

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alvinjaison/PyGo/internal/object"
)

func matchExpect(t *testing.T, src, want string) {
	t.Helper()
	var buf bytes.Buffer
	rt := New(Config{Stdout: &buf, MaxInstructions: 5_000_000})
	if err := rt.RunString(context.Background(), src, "<match-test>"); err != nil {
		t.Fatalf("error: %v\nsource:\n%s", err, src)
	}
	if got := buf.String(); got != want {
		t.Errorf("source:\n%s\n got: %q\nwant: %q", src, got, want)
	}
}

// matchError runs src and requires an error whose message contains substr.
func matchError(t *testing.T, src, substr string) {
	t.Helper()
	rt := New(Config{Stdout: &bytes.Buffer{}, MaxInstructions: 5_000_000})
	err := rt.RunString(context.Background(), src, "<match-test>")
	if err == nil {
		t.Fatalf("expected an error containing %q, got none\nsource:\n%s", substr, src)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Errorf("error %q does not contain %q\nsource:\n%s", err.Error(), substr, src)
	}
}

func TestMatchLiteralsAndOr(t *testing.T) {
	matchExpect(t, `def f(x):
    match x:
        case 0:
            return "zero"
        case 1 | 2 | 3:
            return "small"
        case -1:
            return "neg"
        case -2.5:
            return "negfloat"
        case "hi":
            return "str"
        case b"ab":
            return "bytes"
        case None:
            return "none"
        case _:
            return "other"
print([f(v) for v in [0, 2, -1, -2.5, "hi", b"ab", None, 99]])
`, "['zero', 'small', 'neg', 'negfloat', 'str', 'bytes', 'none', 'other']\n")
}

func TestMatchSingletonsUseIdentityAndValuesUseEquality(t *testing.T) {
	// True == 1, so a value pattern 1 matches True (as in CPython), while the
	// singleton pattern True matches by identity and so rejects 1.
	matchExpect(t, `def value(x):
    match x:
        case 1:
            return "one"
        case _:
            return "other"
def singleton(x):
    match x:
        case True:
            return "true"
        case False:
            return "false"
        case _:
            return "other"
print(value(1), value(True), singleton(True), singleton(False), singleton(1), singleton(0))
`, "one one true false other other\n")
}

func TestMatchFallsThroughWhenNoCaseMatches(t *testing.T) {
	matchExpect(t, `def f(x):
    match x:
        case 1:
            return "one"
    return "fell through"
print(f(1), f(2))
`, "one fell through\n")
}

func TestMatchSequencePatterns(t *testing.T) {
	matchExpect(t, `def f(x):
    match x:
        case []:
            return "empty"
        case [a]:
            return "one " + str(a)
        case [a, b]:
            return "two " + str(a + b)
        case [first, *rest]:
            return "many " + str(first) + " " + str(len(rest))
        case _:
            return "other"
print(f([]), f([1]), f((1, 2)), f([1, 2, 3, 4]), f("ab"), f(b"ab"), f({1: 2}), f(5))
`, "empty one 1 two 3 many 1 3 other other other other\n")
}

func TestMatchStarCapturesMiddle(t *testing.T) {
	matchExpect(t, `def f(x):
    match x:
        case [first, *mid, last]:
            return (first, mid, last)
        case _:
            return None
print(f([1, 2, 3, 4]), f([1, 2]), f([1]))
`, "(1, [2, 3], 4) (1, [], 2) None\n")
}

func TestMatchMappingPatterns(t *testing.T) {
	matchExpect(t, `def f(x):
    match x:
        case {"kind": "circle", "r": r}:
            return "circle " + str(r)
        case {"kind": "rect", "w": w, "h": h, **rest}:
            return "rect " + str(w * h) + " " + str(rest)
        case {}:
            return "dict"
        case _:
            return "not a dict"
print(f({"kind": "circle", "r": 2}), f({"kind": "rect", "w": 2, "h": 3, "z": 1}), f({"a": 1}), f([1]))
`, "circle 2 rect 6 {'z': 1} dict not a dict\n")
}

func TestMatchMappingDoesNotInventDefaults(t *testing.T) {
	matchExpect(t, `from collections import defaultdict
dd = defaultdict(list)
dd["a"] = 1
match dd:
    case {"missing": v}:
        print("bad")
    case {"a": v}:
        print("a =", v)
print(len(dd))
`, "a = 1\n1\n")
}

func TestMatchClassPatterns(t *testing.T) {
	matchExpect(t, `class Point:
    __match_args__ = ("x", "y")
    def __init__(self, x, y):
        self.x = x
        self.y = y
def f(p):
    match p:
        case Point(0, 0):
            return "origin"
        case Point(x, 0):
            return "x-axis " + str(x)
        case Point(x=0, y=y):
            return "y-axis " + str(y)
        case Point(x, y):
            return "point " + str(x) + "," + str(y)
        case _:
            return "not a point"
print(f(Point(0, 0)), f(Point(3, 0)), f(Point(0, 4)), f(Point(1, 2)), f(5))
`, "origin x-axis 3 y-axis 4 point 1,2 not a point\n")
}

func TestMatchBuiltinTypePatterns(t *testing.T) {
	matchExpect(t, `def f(x):
    match x:
        case bool():
            return "bool"
        case int(n):
            return "int " + str(n)
        case float() as v:
            return "float " + str(v)
        case str() as s:
            return "str " + s
        case list():
            return "list"
        case dict():
            return "dict"
        case _:
            return "?"
print([f(v) for v in [True, 5, 2.5, "s", [1], {}, None]])
`, "['bool', 'int 5', 'float 2.5', 'str s', 'list', 'dict', '?']\n")
}

func TestMatchNamedTuple(t *testing.T) {
	matchExpect(t, `from collections import namedtuple
P = namedtuple("P", "x y")
def f(v):
    match v:
        case P(0, 0):
            return "origin"
        case P(x=x, y=0):
            return "x-axis " + str(x)
        case P(a, b):
            return "p " + str(a) + " " + str(b)
        case _:
            return "other"
print(f(P(0, 0)), f(P(5, 0)), f(P(1, 2)), f((1, 2)))
`, "origin x-axis 5 p 1 2 other\n")
}

func TestMatchDottedValueAndGuard(t *testing.T) {
	matchExpect(t, `class Color:
    RED = 1
    GREEN = 2
def f(c):
    match c:
        case Color.RED:
            return "red"
        case Color.GREEN:
            return "green"
        case n if n < 0:
            return "negative"
        case n:
            return "other " + str(n)
print(f(1), f(2), f(-5), f(9))
`, "red green negative other 9\n")
}

func TestMatchAsAndOrBindings(t *testing.T) {
	matchExpect(t, `def f(v):
    match v:
        case ("a", x) | ("b", x):
            return x
        case [1, 2] as whole:
            return whole
        case (3 | 4) as n:
            return n
        case _:
            return None
print(f(("a", 1)), f(("b", 2)), f([1, 2]), f(4), f(("c", 3)))
`, "1 2 [1, 2] 4 None\n")
}

func TestMatchSoftKeywordsStayUsableAsNames(t *testing.T) {
	matchExpect(t, `match = 5
case = match + 1
print(match, case)
def f(match):
    return match * 2
print(f(4))
d = {"match": 1}
print(d["match"])
`, "5 6\n8\n1\n")
}

func TestMatchTupleSubjectAndOpenSequence(t *testing.T) {
	matchExpect(t, `a = 1
b = 2
match a, b:
    case 1, y:
        print("y =", y)
`, "y = 2\n")
}

func TestMatchSubjectEvaluatedOnce(t *testing.T) {
	matchExpect(t, `calls = []
def subj():
    calls.append(1)
    return 3
match subj():
    case 1: pass
    case 2: pass
    case 3: pass
print(len(calls))
`, "1\n")
}

func TestMatchKeepsStackBalancedInLoops(t *testing.T) {
	// break/continue/return inside case bodies, and partial failures deep in
	// nested patterns, must not leak values onto the stack.
	matchExpect(t, `out = []
for item in [1, "skip", 2, "stop", 3]:
    match item:
        case "skip":
            continue
        case "stop":
            break
        case int() as n:
            out.append(n)
print(out)

def early(items):
    for it in items:
        match it:
            case [x, y] if x == y:
                return "pair " + str(x)
    return "none"
print(early([[1, 2], [3, 3]]), early([]))

def deep(v):
    match v:
        case [[1, 2], [3, 4]]:
            return "A"
        case [[1, 2], [3, _]]:
            return "B"
        case [[1, _], *_]:
            return "C"
        case _:
            return "D"
res = []
for _ in range(50):
    res = [deep([[1, 2], [3, 4]]), deep([[1, 2], [3, 9]]), deep([[1, 9], 5]), deep([[2, 2]])]
print(res)
`, "[1, 2]\npair 3 none\n['A', 'B', 'C', 'D']\n")
}

func TestMatchInsideGeneratorsAndClosures(t *testing.T) {
	matchExpect(t, `def gen(xs):
    for x in xs:
        match x:
            case [a, b]:
                yield a + b
            case n:
                yield n
print(list(gen([[1, 2], 5, [3, 4]])))

def mk():
    total = 0
    def add(v):
        nonlocal total
        match v:
            case [a, b]:
                total += a + b
            case n:
                total += n
        return total
    return add
ad = mk()
ad([1, 2])
print(ad(10))
`, "[3, 5, 7]\n13\n")
}

func TestMatchExceptionInBodyUnwindsCleanly(t *testing.T) {
	matchExpect(t, `for i in range(3):
    try:
        match i:
            case 1:
                raise KeyError(i)
            case n:
                print("n", n)
    except KeyError:
        print("caught")
`, "n 0\ncaught\nn 2\n")
}

func TestMatchMissingAttributeMeansNoMatch(t *testing.T) {
	matchExpect(t, `class P:
    __match_args__ = ("x",)
match P():
    case P(1):
        print("matched")
    case P(x=2):
        print("matched")
    case _:
        print("no match")
`, "no match\n")
}

func TestMatchCompileErrors(t *testing.T) {
	matchError(t, "match 1:\n    case x:\n        pass\n    case 2:\n        pass\n", "irrefutable")
	matchError(t, "match [1, 2]:\n    case [x, x]:\n        pass\n", "multiple assignments to name 'x'")
	matchError(t, "match 1:\n    case [a] | b:\n        pass\n", "different names")
	matchError(t, "match 1:\n    case *a:\n        pass\n", "star pattern")
	matchError(t, "match 1:\n    case [*a, *b]:\n        pass\n", "multiple starred")
	matchError(t, "match 1:\n    case {x: 1}:\n        pass\n", "mapping pattern keys")
}

func TestMatchRuntimeErrors(t *testing.T) {
	matchError(t, "class A: pass\nmatch A():\n    case A(1):\n        pass\n",
		"accepts 0 positional sub-patterns")
	matchError(t, "class A:\n    __match_args__ = ('a',)\nmatch A():\n    case A(1, 2):\n        pass\n",
		"accepts 1 positional sub-patterns")
	matchError(t, "x = 3\nmatch 5:\n    case x():\n        pass\n", "must be a class")
}

func TestStarUnpackingAssignment(t *testing.T) {
	matchExpect(t, `a, *b = [1, 2, 3]
*c, d = (1, 2, 3)
e, *f, g = "hello"
h, *i = [7]
print(a, b, c, d, e, f, g, h, i)
`, "1 [2, 3] [1, 2] 3 h ['e', 'l', 'l'] o 7 []\n")
	matchError(t, "a, *b, c = [1]\n", "not enough values to unpack")
}

func TestBuiltinTypeNamesWorkWithIsinstance(t *testing.T) {
	matchExpect(t, `print(isinstance(1, int), isinstance("a", str), isinstance([1], list))
print(isinstance(True, int), isinstance(1.5, float), isinstance({}, dict))
print(isinstance(1, (str, int)), isinstance(1, (str, list)))
print(type(1) == int, type("a") == int, type([]) == list)
print(int, dict)
print(dict(a=1, b=2), dict([(1, 2)]), len(set([1, 1, 2])), len(set()))
`, "True True True\nTrue True True\nTrue False\nTrue False True\n<class 'int'> <class 'dict'>\n{'a': 1, 'b': 2} {1: 2} 2 0\n")
}

func TestUnaryOperatorsOnFloatsAndBools(t *testing.T) {
	matchExpect(t, `x = 1.5
print(-x, +x, -1.5, -True, +True, ~True)
`, "-1.5 1.5 -1.5 -1 1 -2\n")
	matchError(t, "print(-'x')\n", "bad operand type for unary -")
}

func TestNonlocalWriteOnlyClosure(t *testing.T) {
	// The nested function only assigns the variable; the enclosing function
	// must still keep it in a cell.
	matchExpect(t, `def mk():
    total = 0
    def add(v):
        nonlocal total
        total += v
        return total
    return add
ad = mk()
ad(1)
print(ad(10))
`, "11\n")
}

func TestTracebackAndSyntaxErrorLineNumbers(t *testing.T) {
	// Runtime errors must report the line that raised, not the first line.
	rt := New(Config{Stdout: &bytes.Buffer{}})
	err := rt.RunString(context.Background(), "x = 1\ny = 2\n\nz = 3\nprint(undefined_name)\n", "<lines>")
	var exc *object.ExceptionObject
	if !errors.As(err, &exc) || exc.Traceback == nil || len(exc.Traceback.Frames) == 0 {
		t.Fatalf("expected a NameError with a traceback, got: %v", err)
	}
	if got := exc.Traceback.Frames[len(exc.Traceback.Frames)-1].Line; got != 5 {
		t.Errorf("traceback should report line 5, got %d", got)
	}

	// A function's frame reports the line of the failing statement, not the
	// line of its first statement.
	rt = New(Config{Stdout: &bytes.Buffer{}})
	err = rt.RunString(context.Background(), "def f():\n    a = 1\n    b = 2\n    return a / 0\nf()\n", "<lines>")
	if !errors.As(err, &exc) || exc.Traceback == nil {
		t.Fatalf("expected a ZeroDivisionError with a traceback, got: %v", err)
	}
	var lines []int
	for _, fr := range exc.Traceback.Frames {
		lines = append(lines, fr.Line)
	}
	if len(lines) != 2 || lines[0] != 5 || lines[1] != 4 {
		t.Errorf("traceback lines = %v, want [5 4]", lines)
	}

	// Syntax errors must report the physical line, not a doubled count.
	rt = New(Config{Stdout: &bytes.Buffer{}})
	err = rt.RunString(context.Background(), "x = 1\ny = 2\nz = 3\nw = 5 +\n", "<lines>")
	if err == nil {
		t.Fatal("expected SyntaxError")
	}
	if !strings.Contains(err.Error(), ":4:") {
		t.Errorf("syntax error should report line 4, got: %v", err)
	}
}
