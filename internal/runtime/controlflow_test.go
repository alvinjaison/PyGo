package runtime

import (
	"bytes"
	"context"
	"testing"
)

func cfExpect(t *testing.T, src, want string) {
	t.Helper()
	var buf bytes.Buffer
	rt := New(Config{Stdout: &buf, MaxInstructions: 5_000_000})
	if err := rt.RunString(context.Background(), src, "<cf-test>"); err != nil {
		t.Fatalf("error: %v\nsource:\n%s", err, src)
	}
	if got := buf.String(); got != want {
		t.Errorf("source:\n%s\n got: %q\nwant: %q", src, got, want)
	}
}

func TestFinallyRunsWhenExceptionPropagates(t *testing.T) {
	cfExpect(t, `def f():
    try:
        raise ValueError("x")
    finally:
        print("cleanup")
try:
    f()
except ValueError:
    print("caught")
`, "cleanup\ncaught\n")
}

func TestFinallyRunsOnReturnAndKeepsValue(t *testing.T) {
	cfExpect(t, `def f():
    try:
        return 1
    finally:
        print("cleanup")
print(f())
`, "cleanup\n1\n")
}

func TestNestedFinallyOrderOnReturn(t *testing.T) {
	// Inner finally runs before outer finally.
	cfExpect(t, `def f():
    try:
        try:
            return "r"
        finally:
            print("inner")
    finally:
        print("outer")
print(f())
`, "inner\nouter\nr\n")
}

func TestFinallyRunsPerIterationOnBreak(t *testing.T) {
	cfExpect(t, `for i in range(3):
    try:
        if i == 1:
            break
        print("body", i)
    finally:
        print("fin", i)
print("end")
`, "body 0\nfin 0\nfin 1\nend\n")
}

func TestFinallyRunsOnContinue(t *testing.T) {
	cfExpect(t, `for i in range(3):
    try:
        if i == 1:
            continue
        print("body", i)
    finally:
        print("fin", i)
`, "body 0\nfin 0\nfin 1\nbody 2\nfin 2\n")
}

func TestWhileBreakInsideTryFinally(t *testing.T) {
	cfExpect(t, `i = 0
while True:
    try:
        i += 1
        if i == 3:
            break
    finally:
        print("fin", i)
print("done", i)
`, "fin 1\nfin 2\nfin 3\ndone 3\n")
}

func TestElseRunsOnlyWithoutException(t *testing.T) {
	cfExpect(t, `try:
    raise ValueError("x")
except ValueError:
    print("handled")
else:
    print("WRONG")
try:
    pass
except ValueError:
    print("WRONG")
else:
    print("else ran")
`, "handled\nelse ran\n")
}

func TestFinallyRunsWhenHandlerRaises(t *testing.T) {
	cfExpect(t, `def f():
    try:
        raise ValueError("a")
    except ValueError:
        raise KeyError("b")
    finally:
        print("fin")
try:
    f()
except KeyError:
    print("KeyError escaped")
`, "fin\nKeyError escaped\n")
}

func TestFinallyRunsWhenElseRaises(t *testing.T) {
	cfExpect(t, `def f():
    try:
        pass
    except ValueError:
        pass
    else:
        raise KeyError("else")
    finally:
        print("fin")
try:
    f()
except KeyError:
    print("caught")
`, "fin\ncaught\n")
}

func TestFinallyBodyContainingTry(t *testing.T) {
	// A try statement inside a finally body that runs during a return must not
	// corrupt the enclosing unwind bookkeeping.
	cfExpect(t, `def f():
    try:
        try:
            return "v"
        finally:
            try:
                print("inner-fin")
            except Exception:
                pass
    finally:
        print("outer-fin")
print(f())
`, "inner-fin\nouter-fin\nv\n")
}

func TestWithExitRunsOnReturnStatement(t *testing.T) {
	cfExpect(t, `class CM:
    def __enter__(self):
        return self
    def __exit__(self, a, b, c):
        print("exit")
        return False
def f():
    with CM():
        return 7
print(f())
`, "exit\n7\n")
}

func TestReturnFromWithInsideLoop(t *testing.T) {
	cfExpect(t, `class CM:
    def __init__(self, n):
        self.n = n
    def __enter__(self):
        return self
    def __exit__(self, a, b, c):
        print("exit", self.n)
        return False
def f():
    for i in range(5):
        with CM(i):
            if i == 1:
                return i
print(f())
`, "exit 0\nexit 1\n1\n")
}

func TestBreakOutOfWithRunsExit(t *testing.T) {
	cfExpect(t, `class CM:
    def __enter__(self):
        return self
    def __exit__(self, a, b, c):
        print("exit")
        return False
for i in range(3):
    with CM():
        break
print("after")
`, "exit\nafter\n")
}

func TestWithAndFinallyTogetherOnReturn(t *testing.T) {
	// Exit order is innermost-first: with's __exit__, then finally.
	cfExpect(t, `class CM:
    def __enter__(self):
        return self
    def __exit__(self, a, b, c):
        print("exit")
        return False
def f():
    try:
        with CM():
            return "x"
    finally:
        print("fin")
print(f())
`, "exit\nfin\nx\n")
}

func TestBreakDoesNotLeakIteratorOnStack(t *testing.T) {
	// Regression: `break` out of a for loop used to leave the iterator on the
	// value stack, so enough of them overflowed it and crashed the VM.
	cfExpect(t, `n = 0
for i in range(1000):
    for j in range(5):
        n += 1
        if j == 1:
            break
print(n)
`, "2000\n")
}

func TestBreakLeakInFunctionWithManyCalls(t *testing.T) {
	cfExpect(t, `def first_even(xs):
    for x in xs:
        if x % 2 == 0:
            break
    return x
total = 0
for _ in range(500):
    total += first_even([1, 3, 4, 5])
print(total)
`, "2000\n")
}

func TestForElseSkippedOnBreak(t *testing.T) {
	cfExpect(t, `for i in range(3):
    if i == 1:
        break
else:
    print("WRONG")
for i in range(2):
    pass
else:
    print("else ran")
`, "else ran\n")
}

func TestExceptionInsideLoopStillCleansUp(t *testing.T) {
	cfExpect(t, `out = []
for i in range(3):
    try:
        if i == 1:
            raise ValueError("skip")
        out.append(i)
    except ValueError:
        out.append("e")
    finally:
        out.append("f")
print(out)
`, "[0, 'f', 'e', 'f', 2, 'f']\n")
}
