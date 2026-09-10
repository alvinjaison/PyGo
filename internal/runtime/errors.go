package runtime

import "fmt"

// SyntaxError wraps a parse-time error with source context.
type SyntaxError struct {
	Filename string
	Err      error
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("SyntaxError in %s: %s", e.Filename, e.Err.Error())
}

func (e *SyntaxError) Unwrap() error {
	return e.Err
}

// CompileError wraps a compilation error with source context.
type CompileError struct {
	Filename string
	Err      error
}

func (e *CompileError) Error() string {
	return fmt.Sprintf("CompileError in %s: %s", e.Filename, e.Err.Error())
}

func (e *CompileError) Unwrap() error {
	return e.Err
}

// RuntimeError wraps a VM execution error (unhandled exception or system error).
type RuntimeError struct {
	Err error
}

func (e *RuntimeError) Error() string {
	return e.Err.Error()
}

func (e *RuntimeError) Unwrap() error {
	return e.Err
}
