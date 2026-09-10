package vm_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alvinjaison/pygo/internal/compiler"
	"github.com/alvinjaison/pygo/internal/object"
	"github.com/alvinjaison/pygo/internal/parser"
	"github.com/alvinjaison/pygo/internal/runtime"
	"github.com/alvinjaison/pygo/internal/vm"
	"pgregory.net/rapid"
)

// ============================================================
// Property 6: Short-Circuit Evaluation
// ============================================================
// **Validates: Requirements 16.4**
//
// Python `and` and `or` operators use short-circuit evaluation:
// - `False and X` → X is never evaluated
// - `True or X` → X is never evaluated
// Side effects of unevaluated operands must NOT occur.

func TestPropertyShortCircuitEvaluation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Choose between "and" short-circuit or "or" short-circuit
		useAnd := rapid.Bool().Draw(t, "useAnd")

		// Generate a variable name for the side-effect tracker
		trackerVars := []string{"side_effect_flag", "tracker", "marker", "flag"}
		trackerIdx := rapid.IntRange(0, len(trackerVars)-1).Draw(t, "trackerIdx")
		tracker := trackerVars[trackerIdx]

		// Generate a result variable name
		resultVars := []string{"x", "y", "result", "val"}
		resultIdx := rapid.IntRange(0, len(resultVars)-1).Draw(t, "resultIdx")
		resultVar := resultVars[resultIdx]

		// Ensure tracker and result var are different
		if tracker == resultVar {
			resultVar = "out"
		}

		var source string
		if useAnd {
			// `False and side_effect()` — the right side should NOT be evaluated.
			// We use a function that sets a global flag to detect side effects.
			source = fmt.Sprintf(`%s = 0
def cause_side_effect():
    global %s
    %s = 1
    return True
%s = False and cause_side_effect()
`, tracker, tracker, tracker, resultVar)
		} else {
			// `True or side_effect()` — the right side should NOT be evaluated.
			source = fmt.Sprintf(`%s = 0
def cause_side_effect():
    global %s
    %s = 1
    return True
%s = True or cause_side_effect()
`, tracker, tracker, tracker, resultVar)
		}

		// Execute the source
		var buf bytes.Buffer
		r := runtime.New(runtime.WithOutput(&buf))
		err := r.ExecString(source, "<test>")
		if err != nil {
			t.Fatalf("execution failed: %v\nsource:\n%s", err, source)
		}

		// Verify the side-effect tracker was NOT modified (remains 0)
		checkSource := fmt.Sprintf("print(%s)", tracker)
		err = r.ExecString(checkSource, "<check>")
		if err != nil {
			t.Fatalf("check execution failed: %v", err)
		}

		output := buf.String()
		if strings.TrimSpace(output) != "0" {
			if useAnd {
				t.Fatalf("short-circuit 'and' failed: side effect occurred (tracker=%s)\nsource:\n%s",
					strings.TrimSpace(output), source)
			} else {
				t.Fatalf("short-circuit 'or' failed: side effect occurred (tracker=%s)\nsource:\n%s",
					strings.TrimSpace(output), source)
			}
		}
	})
}

// ============================================================
// Property 10: Recursion Depth Enforcement
// ============================================================
// **Validates: Requirements 5.4, 5.5**
//
// Recursive functions that call themselves beyond the configured max depth
// must raise RecursionError. The Go process must NOT crash with a stack
// overflow — the error must be caught cleanly.

func TestPropertyRecursionDepthEnforcement(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a small max call depth (10–50)
		maxDepth := rapid.IntRange(10, 50).Draw(t, "maxDepth")

		// Generate a function name
		funcNames := []string{"recurse", "loop_fn", "dive", "deep", "repeat_fn"}
		fnIdx := rapid.IntRange(0, len(funcNames)-1).Draw(t, "fnIdx")
		funcName := funcNames[fnIdx]

		// Create a recursive function that exceeds maxDepth
		source := fmt.Sprintf(`def %s(n):
    return %s(n + 1)
%s(0)
`, funcName, funcName, funcName)

		// Execute with a restricted max call depth
		var buf bytes.Buffer
		r := runtime.New(
			runtime.WithOutput(&buf),
			runtime.WithMaxCallDepth(maxDepth),
		)

		err := r.ExecString(source, "<test>")

		// Must get an error (not a crash)
		if err == nil {
			t.Fatalf("expected RecursionError for maxDepth=%d, got nil\nsource:\n%s",
				maxDepth, source)
		}

		// The error message should indicate recursion
		errMsg := strings.ToLower(err.Error())
		if !strings.Contains(errMsg, "recursion") {
			t.Fatalf("expected error containing 'recursion', got: %s\nmaxDepth=%d\nsource:\n%s",
				err.Error(), maxDepth, source)
		}
	})
}

// ============================================================
// Property 11: Cancellation Responsiveness
// ============================================================
// **Validates: Requirements 13.3**
//
// When a context is cancelled, an infinite loop must terminate within
// a bounded time. The VM checks context cancellation at every instruction
// (Phase 1), so termination should be near-immediate.

func TestPropertyCancellationResponsiveness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a small timeout duration (5ms to 100ms)
		timeoutMs := rapid.IntRange(5, 100).Draw(t, "timeoutMs")
		timeout := time.Duration(timeoutMs) * time.Millisecond

		// The maximum allowed wall-clock time for the VM to respond
		// to cancellation (generous bound to avoid flaky tests)
		maxResponseTime := 5 * time.Second

		// Use an infinite loop as the source
		source := "while True:\n    pass\n"

		// Compile the source to bytecode
		module, parseErr := parser.Parse(source, "<test>")
		if parseErr != nil {
			t.Fatalf("parse failed: %v", parseErr)
		}
		code, compileErr := compiler.Compile(module)
		if compileErr != nil {
			t.Fatalf("compile failed: %v", compileErr)
		}

		// Create a context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// Create globals and builtins dicts for the frame
		globals := object.NewDict()
		builtins := object.NewDict()

		// Create the VM with the cancellable context
		machine := vm.New(ctx, 500)

		// Create and execute the frame
		frame := vm.NewFrame(code, globals, builtins, nil)

		start := time.Now()
		_, execErr := machine.Run(frame)
		elapsed := time.Since(start)

		// Must get an error (context cancelled or deadline exceeded)
		if execErr == nil {
			t.Fatalf("expected cancellation error, got nil (timeout=%v)", timeout)
		}

		// Verify we terminated within bounded time
		if elapsed > maxResponseTime {
			t.Fatalf("cancellation took too long: %v (limit: %v, timeout: %v)",
				elapsed, maxResponseTime, timeout)
		}

		// The error should indicate cancellation (context.DeadlineExceeded or context.Canceled)
		errMsg := execErr.Error()
		if !strings.Contains(errMsg, "context") &&
			!strings.Contains(errMsg, "canceled") &&
			!strings.Contains(errMsg, "deadline") {
			t.Fatalf("expected cancellation-related error, got: %s", errMsg)
		}
	})
}
