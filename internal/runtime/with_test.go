package runtime

import (
	"bytes"
	"context"
	"testing"
)

func withExpect(t *testing.T, src, want string) {
	t.Helper()
	var buf bytes.Buffer
	rt := New(Config{Stdout: &buf})
	if err := rt.RunString(context.Background(), src, "<with-test>"); err != nil {
		t.Fatalf("error: %v\nsource:\n%s", err, src)
	}
	if got := buf.String(); got != want {
		t.Errorf("source:\n%s\n got: %q\nwant: %q", src, got, want)
	}
}

func TestWithBasic(t *testing.T) {
	src := `class CM:
    def __enter__(self):
        print("enter")
        return self
    def __exit__(self, t, v, tb):
        print("exit")
        return False

with CM():
    print("body")
`
	withExpect(t, src, "enter\nbody\nexit\n")
}

func TestWithAs(t *testing.T) {
	src := `class Resource:
    def __init__(self, name):
        self.name = name
    def __enter__(self):
        return self.name
    def __exit__(self, t, v, tb):
        return False

with Resource("db") as r:
    print(r)
`
	withExpect(t, src, "db\n")
}

func TestWithExitRunsOnException(t *testing.T) {
	// __exit__ must run even when the body raises, and the exception
	// propagates when __exit__ returns a falsey value.
	src := `class CM:
    def __enter__(self):
        return self
    def __exit__(self, t, v, tb):
        print("cleanup")
        return False

try:
    with CM():
        print("before")
        raise ValueError("boom")
        print("after")
except ValueError as e:
    print("caught:", e)
`
	withExpect(t, src, "before\ncleanup\ncaught: boom\n")
}

func TestWithExitSuppressesException(t *testing.T) {
	// When __exit__ returns True, the exception is suppressed.
	src := `class Suppress:
    def __enter__(self):
        return self
    def __exit__(self, t, v, tb):
        print("suppressing")
        return True

with Suppress():
    print("before")
    raise ValueError("boom")
    print("after")
print("continued")
`
	withExpect(t, src, "before\nsuppressing\ncontinued\n")
}

func TestWithMultiple(t *testing.T) {
	src := `class CM:
    def __init__(self, n):
        self.n = n
    def __enter__(self):
        print("enter", self.n)
        return self.n
    def __exit__(self, t, v, tb):
        print("exit", self.n)
        return False

with CM("A") as a, CM("B") as b:
    print("body", a, b)
`
	// Entering left-to-right, exiting right-to-left.
	withExpect(t, src, "enter A\nenter B\nbody A B\nexit B\nexit A\n")
}

func TestWithExitRunsOnReturn(t *testing.T) {
	src := `class CM:
    def __enter__(self):
        return self
    def __exit__(self, t, v, tb):
        print("exit")
        return False

def f():
    with CM():
        print("inside")

f()
print("done")
`
	withExpect(t, src, "inside\nexit\ndone\n")
}

func TestExceptionBuiltins(t *testing.T) {
	// Newly-exposed Exception-derived types are catchable by `except Exception`.
	src := `for exc in [OSError, IOError, ImportError]:
    try:
        raise exc("x")
    except Exception as e:
        print("caught")
`
	withExpect(t, src, "caught\ncaught\ncaught\n")
}

func TestKeyboardInterruptNotException(t *testing.T) {
	// KeyboardInterrupt derives from BaseException, not Exception.
	src := `try:
    try:
        raise KeyboardInterrupt()
    except Exception:
        print("wrongly caught")
except BaseException:
    print("caught as BaseException")
`
	withExpect(t, src, "caught as BaseException\n")
}
