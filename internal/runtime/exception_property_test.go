package runtime

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// ============================================================
// Property 8: Exception Propagation Preserves Type and Message
// ============================================================
// **Validates: Requirements 11.1, 11.2**
//
// When a Python exception is raised and not caught within Python, the
// Runtime returns a structured error to the Go caller containing the
// exception type name and message. The error implements Go's error interface.
//
// For any combination of built-in exception type and ASCII message string:
// 1. ExecString returns a non-nil error
// 2. The error message contains the exception type name
// 3. The error message contains the original message string

func TestProperty8_ExceptionPropagationPreservesTypeAndMessage(t *testing.T) {
	// Built-in exception types from the hierarchy that can be raised with a message.
	exceptionTypes := []string{
		"TypeError",
		"ValueError",
		"NameError",
		"IndexError",
		"KeyError",
		"ZeroDivisionError",
		"RuntimeError",
	}

	rapid.Check(t, func(t *rapid.T) {
		// Generate a random exception type from the built-in hierarchy.
		typeIdx := rapid.IntRange(0, len(exceptionTypes)-1).Draw(t, "typeIdx")
		excType := exceptionTypes[typeIdx]

		// Generate a random message using safe ASCII printable characters.
		// Avoid backslashes, quotes, and newlines that could break the Python string literal.
		msg := rapid.StringMatching(`[a-zA-Z0-9 _\-\.,!?:;()]{1,50}`).Draw(t, "msg")

		// Construct Python source that raises the exception.
		source := fmt.Sprintf(`raise %s("%s")`, excType, msg)

		// Execute via ExecString.
		r := New()
		err := r.ExecString(source, "<test>")

		// 1. The returned error must be non-nil.
		if err == nil {
			t.Fatalf("expected error for `%s`, got nil", source)
		}

		// 2. The error implements Go's error interface (guaranteed by type system,
		//    but verify the Error() method returns a non-empty string).
		errMsg := err.Error()
		if errMsg == "" {
			t.Fatalf("error.Error() returned empty string for `%s`", source)
		}

		// 3. The error message contains the exception type name.
		if !strings.Contains(errMsg, excType) {
			t.Fatalf("error message does not contain exception type %q\n  source: %s\n  error:  %s",
				excType, source, errMsg)
		}

		// 4. The error message contains the original message string.
		if !strings.Contains(errMsg, msg) {
			t.Fatalf("error message does not contain message %q\n  source: %s\n  error:  %s",
				msg, source, errMsg)
		}

		// 5. The error is a *RuntimeError (verifying structured error type).
		var re *RuntimeError
		if !isRuntimeError(err, &re) {
			t.Fatalf("expected *RuntimeError, got %T: %v", err, err)
		}
	})
}

// isRuntimeError checks if err can be unwrapped to a *RuntimeError.
func isRuntimeError(err error, target **RuntimeError) bool {
	for err != nil {
		if re, ok := err.(*RuntimeError); ok {
			*target = re
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}
