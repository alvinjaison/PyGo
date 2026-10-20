package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestInstructionLimit(t *testing.T) {
	rt := New(Config{MaxInstructions: 10000})
	// An unbounded loop must be stopped by the instruction limit.
	err := rt.RunString(context.Background(), "while True:\n    pass\n", "<t>")
	if err == nil {
		t.Fatal("expected instruction limit to abort the loop")
	}
	if !strings.Contains(err.Error(), "instruction limit") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInstructionLimitNotCatchable(t *testing.T) {
	// A hard abort must NOT be interceptable by a bare `except`.
	rt := New(Config{MaxInstructions: 10000})
	src := `count = 0
try:
    while True:
        count += 1
except:
    print("swallowed")
print("after")
`
	err := rt.RunString(context.Background(), src, "<t>")
	if err == nil {
		t.Fatal("expected abort to propagate past the except clause")
	}
	if !strings.Contains(err.Error(), "instruction limit") {
		t.Errorf("abort was altered or swallowed: %v", err)
	}
}

func TestInstructionLimitAllowsNormalProgram(t *testing.T) {
	// A program well under the limit runs to completion.
	rt := New(Config{MaxInstructions: 1000000})
	err := rt.RunString(context.Background(), "total = sum(range(100))\n", "<t>")
	if err != nil {
		t.Fatalf("normal program should not hit the limit: %v", err)
	}
	if v, ok := rt.Get("total"); !ok || v.String() != "4950" {
		t.Errorf("expected total=4950, got %v", v)
	}
}

func TestTimeoutAbortsInfiniteLoop(t *testing.T) {
	rt := New(Config{})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := rt.RunString(ctx, "while True:\n    pass\n", "<t>")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout to abort the loop")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestCancellationAbortsLoop(t *testing.T) {
	rt := New(Config{})
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel shortly after starting.
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	err := rt.RunString(ctx, "while True:\n    pass\n", "<t>")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected Canceled, got: %v", err)
	}
}

func TestCancellationNotCatchable(t *testing.T) {
	rt := New(Config{})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	src := `try:
    while True:
        pass
except:
    pass
`
	err := rt.RunString(ctx, src, "<t>")
	if err == nil {
		t.Fatal("cancellation must not be swallowed by except")
	}
}

func TestRecursionLimit(t *testing.T) {
	rt := New(Config{MaxCallDepth: 50})
	src := `def f(n):
    return f(n + 1)
f(0)
`
	err := rt.RunString(context.Background(), src, "<t>")
	if err == nil {
		t.Fatal("expected RecursionError")
	}
	if !strings.Contains(err.Error(), "recursion") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRecursionLimitIsCatchable(t *testing.T) {
	// RecursionError is an ordinary Python exception and should be catchable.
	rt := New(Config{MaxCallDepth: 50})
	var out strings.Builder
	rt = New(Config{MaxCallDepth: 50, Stdout: &out})
	src := `def f(n):
    return f(n + 1)
try:
    f(0)
except RecursionError:
    print("caught recursion")
`
	if err := rt.RunString(context.Background(), src, "<t>"); err != nil {
		t.Fatalf("RecursionError should be catchable: %v", err)
	}
	if !strings.Contains(out.String(), "caught recursion") {
		t.Errorf("expected recursion to be caught, got: %q", out.String())
	}
}

func TestWithExitNotRunOnHardAbort(t *testing.T) {
	// A hard abort bypasses __exit__ too (it is not a normal unwind).
	rt := New(Config{MaxInstructions: 20000})
	src := `class CM:
    def __enter__(self):
        return self
    def __exit__(self, t, v, tb):
        print("exit ran")
        return False

with CM():
    while True:
        pass
`
	var out strings.Builder
	rt = New(Config{MaxInstructions: 20000, Stdout: &out})
	err := rt.RunString(context.Background(), src, "<t>")
	if err == nil {
		t.Fatal("expected abort")
	}
	if strings.Contains(out.String(), "exit ran") {
		t.Errorf("__exit__ must not run on a hard abort")
	}
}
