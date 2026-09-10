package runtime

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================
// Unit tests for decorators — Task 8.3
// **Validates: Requirements 5.1–5.6**
//
// Expected values were captured from CPython 3 running the same snippet.
// ============================================================

// --- Requirement 5.1: a single decorator on a function ---

func TestDecorator_Single(t *testing.T) {
	got := runOK(t, `
def twice(fn):
    def w(x):
        return fn(x) * 2
    return w

@twice
def add_one(x):
    return x + 1

print(add_one(5))
`)
	if got != "12" {
		t.Errorf("got %q, want %q", got, "12")
	}
}

// --- Requirements 5.2, 5.6: stacked decorators apply bottom-up ---

func TestDecorator_StackedOrder(t *testing.T) {
	got := runOK(t, `
order = []

def a(fn):
    order.append("a")
    def w():
        return "a(" + fn() + ")"
    return w

def b(fn):
    order.append("b")
    def w():
        return "b(" + fn() + ")"
    return w

def c(fn):
    order.append("c")
    def w():
        return "c(" + fn() + ")"
    return w

@a
@b
@c
def base():
    return "base"

print(order)
print(base())
`)
	// Called bottom-up, so c first; nested with a outermost.
	want := "['c', 'b', 'a']\na(b(c(base)))"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- Requirement 5.3: decorator expression may be a dotted name ---

func TestDecorator_DottedName(t *testing.T) {
	tmpDir := t.TempDir()
	modContent := `def shout(fn):
    def w(*args, **kwargs):
        return fn(*args, **kwargs).upper() + "!"
    return w
`
	if err := os.WriteFile(filepath.Join(tmpDir, "decmod.py"), []byte(modContent), 0644); err != nil {
		t.Fatalf("failed to write temp module: %v", err)
	}

	var buf bytes.Buffer
	r := New(WithOutput(&buf), WithSearchPath([]string{tmpDir}))
	err := r.ExecString(`import decmod

@decmod.shout
def greet():
    return "hey"

print(greet())`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	if got != "HEY!" {
		t.Errorf("got %q, want %q", got, "HEY!")
	}
}

// --- Requirement 5.4: parameterized decorator ---

func TestDecorator_Parameterized(t *testing.T) {
	got := runOK(t, `
def repeat(n):
    def deco(fn):
        def w():
            out = ""
            for i in range(n):
                out = out + fn()
            return out
        return w
    return deco

@repeat(3)
def hi():
    return "hi"

print(hi())
`)
	if got != "hihihi" {
		t.Errorf("got %q, want %q", got, "hihihi")
	}
}

// --- Requirement 5.5: a decorator may return any object ---

func TestDecorator_ReturnsNonFunction(t *testing.T) {
	got := runOK(t, `
def to_const(fn):
    return 42

@to_const
def whatever():
    return "unused"

print(whatever)
`)
	if got != "42" {
		t.Errorf("got %q, want %q", got, "42")
	}
}

func TestDecorator_Identity(t *testing.T) {
	got := runOK(t, `
def ident(fn):
    return fn

@ident
def square(x):
    return x * x

print(square(7))
`)
	if got != "49" {
		t.Errorf("got %q, want %q", got, "49")
	}
}

// --- Decorators interact correctly with the rest of the call machinery ---

func TestDecorator_PreservesArgsAndKwargs(t *testing.T) {
	got := runOK(t, `
def logged(fn):
    def w(*args, **kwargs):
        return (fn(*args, **kwargs), len(args), len(kwargs))
    return w

@logged
def combine(x, y=10, *rest, **kw):
    return x + y

print(combine(1))
print(combine(1, 2))
print(combine(1, 2, 3, z=4))
`)
	want := "(11, 1, 0)\n(3, 2, 0)\n(3, 3, 1)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDecorator_OnNestedFunction(t *testing.T) {
	got := runOK(t, `
def twice(fn):
    def w(x):
        return fn(x) * 2
    return w

def outer():
    @twice
    def inner(x):
        return x + 100
    return inner(1)

print(outer())
`)
	if got != "202" {
		t.Errorf("got %q, want %q", got, "202")
	}
}

func TestDecorator_ClosesOverEnclosingScope(t *testing.T) {
	got := runOK(t, `
def scale_by(k):
    def deco(fn):
        def w(x):
            return fn(x) * k
        return w
    return deco

def build(k):
    @scale_by(k)
    def f(x):
        return x + 1
    return f

print(build(10)(2))
print(build(3)(2))
`)
	want := "30\n9"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDecorator_DecoratedFunctionIsAnOrdinaryValue(t *testing.T) {
	got := runOK(t, `
def twice(fn):
    def w(x):
        return fn(x) * 2
    return w

@twice
def doubler(x):
    return x

fns = [doubler]
print(fns[0](21))
`)
	if got != "42" {
		t.Errorf("got %q, want %q", got, "42")
	}
}

// --- Error cases ---

func TestDecorator_NonCallable_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `
notcallable = 5

@notcallable
def f():
    pass
`)
	if !strings.Contains(msg, "'int' object is not callable") {
		t.Errorf("unexpected error: %s", msg)
	}
}

func TestDecorator_NonDefTarget_RaisesSyntaxError(t *testing.T) {
	msg := runErr(t, `
def d(fn):
    return fn

@d
x = 1
`)
	if !strings.Contains(msg, "expected 'def' or 'class' after decorator") {
		t.Errorf("unexpected error: %s", msg)
	}
}

// Decorated classes are parsed as far as the decorator list, but class
// statements themselves are not implemented yet (spec task 9). When class
// support lands this test should be replaced with a real class-decorator test;
// its failure is the signal to do so.
func TestDecorator_OnClass_NotYetSupported(t *testing.T) {
	msg := runErr(t, `
def d(cls):
    return cls

@d
class Foo:
    pass
`)
	if !strings.Contains(msg, "class definitions are not supported yet") {
		t.Errorf("expected the explicit class-not-supported error, got: %s", msg)
	}
}
