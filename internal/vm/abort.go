package vm

import (
	"context"
	"runtime"

	"github.com/alvinjaison/PyGo/internal/object"
)

// abortError represents a hard, non-catchable termination of execution: an
// instruction/memory limit was exceeded, or the context was cancelled. Unlike
// a Python exception, an abort cannot be intercepted by `except` handlers or
// suppressed by a context manager's __exit__ — it unwinds straight out of the
// VM so a host can reliably stop untrusted or runaway scripts.
type abortError struct {
	kind string // "instructions", "memory", "cancel", "deadline"
	msg  string
	// cause is the underlying Go error for cancellation (ctx.Err()), if any.
	cause error
}

func (a *abortError) Error() string { return a.msg }

// checkAbort returns the active abort condition for the VM, if any. It is
// called once per instruction. It is intentionally cheap: the cancellation
// check is a non-blocking channel read and the memory check is throttled.
func (v *VM) checkAbort() *abortError {
	if v.aborted != nil {
		return v.aborted
	}

	// Cancellation / deadline.
	select {
	case <-v.ctx.Done():
		err := v.ctx.Err()
		kind := "cancel"
		msg := "execution cancelled"
		if err == context.DeadlineExceeded {
			kind = "deadline"
			msg = "execution time limit exceeded"
		}
		v.aborted = &abortError{kind: kind, msg: msg, cause: err}
		return v.aborted
	default:
	}

	// Instruction limit.
	if v.maxInstr > 0 && v.instrCount > v.maxInstr {
		v.aborted = &abortError{
			kind: "instructions",
			msg:  "instruction limit exceeded",
		}
		return v.aborted
	}

	// Memory limit (best-effort, polled every memCheckEvery instructions).
	if v.maxMemory > 0 && v.instrCount%v.memCheckEvery == 0 {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		if int64(m.HeapAlloc) > v.maxMemory {
			v.aborted = &abortError{
				kind: "memory",
				msg:  "memory limit exceeded",
			}
			return v.aborted
		}
	}

	return nil
}

// abortToError converts an abort into the error returned from Run. Cancellation
// aborts return the underlying context error (so callers can use errors.Is);
// limit aborts return a sentinel *object.ExceptionObject-free error value.
func abortToError(a *abortError) error {
	if a.cause != nil {
		return a.cause
	}
	return &LimitError{Kind: a.kind, Message: a.msg}
}

// LimitError is returned by Run when a resource limit is exceeded. It is a Go
// error, not a Python exception, so it cannot be caught by Python code.
type LimitError struct {
	Kind    string // "instructions" or "memory"
	Message string
}

func (e *LimitError) Error() string { return e.Message }

// isAbort reports whether err is a hard-abort error (limit or cancellation)
// that must bypass Python exception handling.
func isAbort(err error) bool {
	switch err.(type) {
	case *abortError, *LimitError:
		return true
	}
	return err == context.Canceled || err == context.DeadlineExceeded
}

// ensure object import is retained for future abort helpers
var _ = object.None
