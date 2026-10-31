package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func genExpect(t *testing.T, src, want string) {
	t.Helper()
	var buf bytes.Buffer
	// A generous instruction cap turns an accidental infinite loop into a
	// test failure instead of a hang.
	rt := New(Config{Stdout: &buf, MaxInstructions: 5_000_000})
	if err := rt.RunString(context.Background(), src, "<gen-test>"); err != nil {
		t.Fatalf("error: %v\nsource:\n%s", err, src)
	}
	if got := buf.String(); got != want {
		t.Errorf("source:\n%s\n got: %q\nwant: %q", src, got, want)
	}
}

const natGen = `def nat():
    n = 0
    while True:
        yield n
        n += 1
`

func TestLazyGeneratorExpression(t *testing.T) {
	genExpect(t, natGen+`g = (x * 2 for x in nat())
print(next(g), next(g), next(g))
`, "0 2 4\n")
}

func TestLazyGenexpShortCircuitsAny(t *testing.T) {
	// any() must stop at the first match even on an infinite source.
	genExpect(t, natGen+`print(any(x > 5 for x in nat()))
`, "True\n")
}

func TestLazyGenexpShortCircuitsAll(t *testing.T) {
	genExpect(t, natGen+`print(all(x < 3 for x in nat()))
`, "False\n")
}

func TestGenexpWithFilterAndNesting(t *testing.T) {
	genExpect(t, `g = (x * y for x in range(1, 4) for y in range(1, 4) if x != y)
print(list(g))
`, "[2, 3, 2, 6, 3, 6]\n")
}

func TestGenexpCapturesEnclosingLocal(t *testing.T) {
	genExpect(t, `def scaled(k):
    return list(x * k for x in range(4))
print(scaled(10))
`, "[0, 10, 20, 30]\n")
}

func TestGenexpIsSingleUse(t *testing.T) {
	genExpect(t, `g = (x for x in range(3))
print(list(g))
print(list(g))
`, "[0, 1, 2]\n[]\n")
}

func TestLambdaCapturesEnclosingLocal(t *testing.T) {
	genExpect(t, `def make(k):
    return lambda x: x * k
print(make(3)(4))
adders = [(lambda n: (lambda x: x + n))(i) for i in range(3)]
print([f(10) for f in adders])
`, "12\n[10, 11, 12]\n")
}

func TestGeneratorSend(t *testing.T) {
	genExpect(t, `def acc():
    total = 0
    while True:
        x = yield total
        total += x

g = acc()
print(next(g))
print(g.send(5))
print(g.send(10))
`, "0\n5\n15\n")
}

func TestGeneratorSendToFreshGenerator(t *testing.T) {
	var out strings.Builder
	rt := New(Config{Stdout: &out})
	src := `def g():
    yield 1
x = g()
try:
    x.send(42)
except TypeError:
    print("TypeError")
print(x.send(None))
`
	if err := rt.RunString(context.Background(), src, "<t>"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "TypeError\n1\n" {
		t.Errorf("got %q", out.String())
	}
}

func TestGeneratorThrowCaught(t *testing.T) {
	// The generator handles the exception and keeps going.
	genExpect(t, `def g():
    try:
        yield 1
    except ValueError:
        yield "recovered"
    yield "done"

x = g()
print(next(x))
print(x.throw(ValueError("boom")))
print(next(x))
`, "1\nrecovered\ndone\n")
}

func TestGeneratorThrowUncaught(t *testing.T) {
	genExpect(t, `def g():
    yield 1
    yield 2

x = g()
next(x)
try:
    x.throw(KeyError("k"))
except KeyError:
    print("propagated")
print(list(x))
`, "propagated\n[]\n")
}

func TestGeneratorClose(t *testing.T) {
	// close() raises GeneratorExit inside, so finally blocks run.
	genExpect(t, `def g():
    try:
        yield 1
        yield 2
    finally:
        print("cleanup")

x = g()
print(next(x))
x.close()
print(list(x))
`, "1\ncleanup\n[]\n")
}

func TestGeneratorCloseIgnoredExitIsError(t *testing.T) {
	genExpect(t, `def g():
    while True:
        try:
            yield 1
        except GeneratorExit:
            pass

x = g()
next(x)
try:
    x.close()
except RuntimeError:
    print("RuntimeError")
`, "RuntimeError\n")
}

func TestGeneratorCloseUnstarted(t *testing.T) {
	genExpect(t, `def g():
    yield 1
x = g()
x.close()
print(list(x))
`, "[]\n")
}
