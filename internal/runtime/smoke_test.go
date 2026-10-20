package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func run(t *testing.T, src string) string {
	t.Helper()
	var buf bytes.Buffer
	rt := New(Config{Stdout: &buf})
	if err := rt.RunString(context.Background(), src, "<test>"); err != nil {
		t.Fatalf("RunString error: %v\nsource:\n%s", err, src)
	}
	return buf.String()
}

func expect(t *testing.T, src, want string) {
	t.Helper()
	got := run(t, src)
	if got != want {
		t.Errorf("source:\n%s\n got: %q\nwant: %q", src, got, want)
	}
}

func TestHelloWorld(t *testing.T) {
	expect(t, `print("Hello, world!")`+"\n", "Hello, world!\n")
}

func TestArithmetic(t *testing.T) {
	expect(t, "x = 10\ny = 20\nprint(x + y)\n", "30\n")
}

func TestOperators(t *testing.T) {
	expect(t, "print(2 ** 10)\n", "1024\n")
	expect(t, "print(17 // 5, 17 % 5)\n", "3 2\n")
	expect(t, "print(-7 % 3)\n", "2\n")
	expect(t, "print(1 < 2 < 3)\n", "True\n")
	expect(t, "print(1 < 2 > 5)\n", "False\n")
}

func TestIfElse(t *testing.T) {
	src := "x = 5\nif x > 3:\n    print('big')\nelif x > 1:\n    print('mid')\nelse:\n    print('small')\n"
	expect(t, src, "big\n")
}

func TestWhileLoop(t *testing.T) {
	src := "i = 0\nwhile i < 3:\n    print(i)\n    i += 1\n"
	expect(t, src, "0\n1\n2\n")
}

func TestForLoop(t *testing.T) {
	src := "for i in range(3):\n    print(i)\n"
	expect(t, src, "0\n1\n2\n")
}

func TestFunction(t *testing.T) {
	src := "def add(a, b):\n    return a + b\nprint(add(3, 4))\n"
	expect(t, src, "7\n")
}

func TestClosure(t *testing.T) {
	src := `def make_adder(n):
    def adder(x):
        return x + n
    return adder
add5 = make_adder(5)
print(add5(3))
`
	expect(t, src, "8\n")
}

func TestRecursion(t *testing.T) {
	src := `def fib(n):
    if n <= 1:
        return n
    return fib(n-1) + fib(n-2)
print(fib(10))
`
	expect(t, src, "55\n")
}

func TestListsAndIndex(t *testing.T) {
	src := "xs = [1, 2, 3]\nprint(xs[0], xs[-1], len(xs))\n"
	expect(t, src, "1 3 3\n")
}

func TestDict(t *testing.T) {
	src := "d = {'a': 1, 'b': 2}\nprint(d['a'] + d['b'])\n"
	expect(t, src, "3\n")
}

func TestExceptions(t *testing.T) {
	src := `try:
    x = 1 / 0
except ZeroDivisionError:
    print("caught")
finally:
    print("done")
`
	expect(t, src, "caught\ndone\n")
}

func TestClass(t *testing.T) {
	src := `class Counter:
    def __init__(self, start):
        self.n = start
    def inc(self):
        self.n += 1
        return self.n
c = Counter(10)
print(c.inc())
print(c.inc())
`
	expect(t, src, "11\n12\n")
}

func TestListComprehension(t *testing.T) {
	src := "print([x * 2 for x in range(4)])\n"
	expect(t, src, "[0, 2, 4, 6]\n")
}

func TestFString(t *testing.T) {
	// Basic string concatenation (f-strings handled later).
	src := "name = 'world'\nprint('Hello, ' + name)\n"
	expect(t, src, "Hello, world\n")
}

func TestBuiltins(t *testing.T) {
	expect(t, "print(sum([1, 2, 3, 4]))\n", "10\n")
	expect(t, "print(max(3, 7, 2))\n", "7\n")
	expect(t, "print(min([5, 2, 8]))\n", "2\n")
	expect(t, "print(abs(-5))\n", "5\n")
}

func TestEval(t *testing.T) {
	rt := New(Config{})
	v, err := rt.Eval(context.Background(), "2 + 3 * 4")
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != "14" {
		t.Errorf("got %q want 14", v.String())
	}
}

func TestMultilineProgram(t *testing.T) {
	src := `
total = 0
for i in range(1, 101):
    total += i
print(total)
`
	expect(t, src, "5050\n")
}

func TestStringMethodsBasic(t *testing.T) {
	// Will drive later string-method work; for now ensure concatenation chains.
	got := run(t, "print('a' + 'b' + 'c')\n")
	if !strings.Contains(got, "abc") {
		t.Errorf("got %q", got)
	}
}

func TestGeneratorBasic(t *testing.T) {
	src := `def gen():
    yield 1
    yield 2
    yield 3
for x in gen():
    print(x)
`
	expect(t, src, "1\n2\n3\n")
}

func TestGeneratorWithLoop(t *testing.T) {
	src := `def squares(n):
    i = 0
    while i < n:
        yield i * i
        i += 1
for s in squares(5):
    print(s, end=" ")
print()
`
	expect(t, src, "0 1 4 9 16 \n")
}

func TestGeneratorInList(t *testing.T) {
	src := `def gen():
    yield 10
    yield 20
    yield 30
print(list(gen()))
print(sum(gen()))
`
	expect(t, src, "[10, 20, 30]\n60\n")
}

func TestGeneratorState(t *testing.T) {
	// Generator preserves local state across yields.
	src := `def counter():
    total = 0
    for i in range(4):
        total += i
        yield total
print(list(counter()))
`
	expect(t, src, "[0, 1, 3, 6]\n")
}

func TestFStringInterpolation(t *testing.T) {
	expect(t, "name = 'world'\nprint(f'Hello, {name}!')\n", "Hello, world!\n")
}

func TestFStringExpr(t *testing.T) {
	expect(t, "x = 6\ny = 7\nprint(f'{x} * {y} = {x * y}')\n", "6 * 7 = 42\n")
}

func TestFStringMethodCall(t *testing.T) {
	expect(t, "items = [1, 2, 3]\nprint(f'count: {len(items)}')\n", "count: 3\n")
}

func TestFStringLiteralBraces(t *testing.T) {
	expect(t, "print(f'{{literal}} {1 + 1}')\n", "{literal} 2\n")
}

func TestStringEscapes(t *testing.T) {
	expect(t, `print("line1\nline2")`+"\n", "line1\nline2\n")
	expect(t, `print("tab\there")`+"\n", "tab\there\n")
}

func TestTripleQuoted(t *testing.T) {
	src := "s = \"\"\"a\nb\nc\"\"\"\nprint(s)\n"
	expect(t, src, "a\nb\nc\n")
}

func TestStringMethods(t *testing.T) {
	expect(t, "print('hello'.upper())\n", "HELLO\n")
	expect(t, "print('HELLO'.lower())\n", "hello\n")
	expect(t, "print('  hi  '.strip())\n", "hi\n")
	expect(t, "print('a,b,c'.split(','))\n", "['a', 'b', 'c']\n")
	expect(t, "print('-'.join(['a', 'b', 'c']))\n", "a-b-c\n")
	expect(t, "print('hello world'.replace('world', 'there'))\n", "hello there\n")
	expect(t, "print('hello'.startswith('he'))\n", "True\n")
	expect(t, "print('hello'.endswith('lo'))\n", "True\n")
	expect(t, "print('hello'.find('ll'))\n", "2\n")
	expect(t, "print('abcabc'.count('a'))\n", "2\n")
	expect(t, "print('123'.isdigit())\n", "True\n")
}

func TestStringIndexSlice(t *testing.T) {
	expect(t, "s = 'hello'\nprint(s[0], s[-1])\n", "h o\n")
	expect(t, "s = 'hello'\nprint(s[1:4])\n", "ell\n")
	expect(t, "s = 'hello'\nprint(s[::-1])\n", "olleh\n")
}

func TestStringIteration(t *testing.T) {
	src := "for c in 'abc':\n    print(c)\n"
	expect(t, src, "a\nb\nc\n")
}

func TestStringFormat(t *testing.T) {
	expect(t, "print('{} and {}'.format('a', 'b'))\n", "a and b\n")
	expect(t, "print('{0} {1} {0}'.format('x', 'y'))\n", "x y x\n")
}

func TestStringMembership(t *testing.T) {
	expect(t, "print('ell' in 'hello')\n", "True\n")
	expect(t, "print('xyz' in 'hello')\n", "False\n")
}

func TestListSlice(t *testing.T) {
	expect(t, "xs = [0, 1, 2, 3, 4]\nprint(xs[1:3])\n", "[1, 2]\n")
	expect(t, "xs = [0, 1, 2, 3, 4]\nprint(xs[::-1])\n", "[4, 3, 2, 1, 0]\n")
	expect(t, "xs = [0, 1, 2, 3, 4]\nprint(xs[::2])\n", "[0, 2, 4]\n")
}

func TestDictComprehensionAndMembership(t *testing.T) {
	expect(t, "d = {'a': 1, 'b': 2}\nprint('a' in d)\nprint('z' in d)\n", "True\nFalse\n")
}

func TestNestedDataStructures(t *testing.T) {
	src := `data = {'nums': [1, 2, 3], 'name': 'test'}
print(data['name'], sum(data['nums']))
`
	expect(t, src, "test 6\n")
}

func TestGeneratorExpression(t *testing.T) {
	expect(t, "print(sum(x * x for x in range(5)))\n", "30\n")
	expect(t, "words = ['a', 'b', 'c']\nprint(' '.join(w.upper() for w in words))\n", "A B C\n")
}

func TestSetComprehension(t *testing.T) {
	// Set repr is sorted deterministically in this implementation.
	expect(t, "print({x % 3 for x in range(6)})\n", "{0, 1, 2}\n")
}

func TestDictComprehension(t *testing.T) {
	expect(t, "print({x: x * x for x in range(4)})\n", "{0: 0, 1: 1, 2: 4, 3: 9}\n")
}
