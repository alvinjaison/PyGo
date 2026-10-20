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
