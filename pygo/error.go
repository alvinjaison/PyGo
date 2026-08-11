package pygo

import (
	"errors"
	"fmt"
	"strings"
)

// Error represents a Python exception raised during execution.
// It implements the Go error interface.
type Error struct {
	// Type is the Python exception class name (e.g., "ValueError", "TypeError").
	Type string

	// Message is the exception's string message.
	Message string

	// Traceback contains the Python call stack at the point the exception was raised.
	Traceback []TracebackEntry
}

// TracebackEntry represents one frame in a Python traceback.
type TracebackEntry struct {
	Filename string
	FuncName string
	Line     int
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Format returns the full CPython-style traceback string for display.
func (e *Error) Format() string {
	var sb strings.Builder
	if len(e.Traceback) > 0 {
		sb.WriteString("Traceback (most recent call last):\n")
		for _, f := range e.Traceback {
			sb.WriteString(fmt.Sprintf("  File %q, line %d, in %s\n",
				f.Filename, f.Line, f.FuncName))
		}
	}
	sb.WriteString(e.Type)
	if e.Message != "" {
		sb.WriteString(": ")
		sb.WriteString(e.Message)
	}
	sb.WriteString("\n")
	return sb.String()
}

// IsError reports whether err is a *Error with the given Python exception type name.
// The comparison is exact (e.g., "ValueError", not "Error").
//
// Example:
//
//	err := rt.RunString(`raise ValueError("bad input")`)
//	if pygo.IsError(err, "ValueError") {
//	    // handle ValueError
//	}
func IsError(err error, typeName string) bool {
	var pyErr *Error
	if errors.As(err, &pyErr) {
		return pyErr.Type == typeName
	}
	return false
}

// AsError returns the *Error from err, or nil if err is not a *Error.
func AsError(err error) *Error {
	var pyErr *Error
	if errors.As(err, &pyErr) {
		return pyErr
	}
	return nil
}
