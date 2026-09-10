package interop

import (
	"errors"
	"testing"

	"github.com/alvinjaison/pygo/internal/object"
)

func TestWrapGoFunction_AddInts(t *testing.T) {
	add := func(a, b int) int { return a + b }

	wrapped, err := WrapGoFunction("add", add)
	if err != nil {
		t.Fatalf("WrapGoFunction failed: %v", err)
	}

	result, err := wrapped.Call(
		[]object.Object{object.IntFrom(3), object.IntFrom(4)},
		nil,
	)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	intObj, ok := result.(*object.IntObject)
	if !ok {
		t.Fatalf("expected *object.IntObject, got %T", result)
	}
	if intObj.Value.Int64() != 7 {
		t.Errorf("expected 7, got %d", intObj.Value.Int64())
	}
}

func TestWrapGoFunction_ReturnsError(t *testing.T) {
	failing := func(x int) (string, error) {
		if x < 0 {
			return "", errors.New("negative value not allowed")
		}
		return "ok", nil
	}

	wrapped, err := WrapGoFunction("failing", failing)
	if err != nil {
		t.Fatalf("WrapGoFunction failed: %v", err)
	}

	// Test success case.
	result, err := wrapped.Call(
		[]object.Object{object.IntFrom(1)},
		nil,
	)
	if err != nil {
		t.Fatalf("Call failed unexpectedly: %v", err)
	}
	strObj, ok := result.(*object.StrObject)
	if !ok {
		t.Fatalf("expected *object.StrObject, got %T", result)
	}
	if strObj.Value != "ok" {
		t.Errorf("expected 'ok', got %q", strObj.Value)
	}

	// Test error case — should raise a Python exception.
	_, err = wrapped.Call(
		[]object.Object{object.IntFrom(-1)},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for negative input")
	}
	excObj, ok := err.(*object.ExceptionObject)
	if !ok {
		t.Fatalf("expected *object.ExceptionObject, got %T", err)
	}
	if excObj.ExcType != object.TypeRuntimeError {
		t.Errorf("expected RuntimeError, got %s", excObj.ExcType.Name())
	}
	if excObj.Message != "negative value not allowed" {
		t.Errorf("unexpected message: %s", excObj.Message)
	}
}

func TestWrapGoFunction_StringArgs(t *testing.T) {
	greet := func(name string) string {
		return "Hello, " + name + "!"
	}

	wrapped, err := WrapGoFunction("greet", greet)
	if err != nil {
		t.Fatalf("WrapGoFunction failed: %v", err)
	}

	result, err := wrapped.Call(
		[]object.Object{object.StrFrom("World")},
		nil,
	)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	strObj, ok := result.(*object.StrObject)
	if !ok {
		t.Fatalf("expected *object.StrObject, got %T", result)
	}
	if strObj.Value != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %q", strObj.Value)
	}
}

func TestWrapGoFunction_ArgCountMismatch(t *testing.T) {
	add := func(a, b int) int { return a + b }

	wrapped, err := WrapGoFunction("add", add)
	if err != nil {
		t.Fatalf("WrapGoFunction failed: %v", err)
	}

	// Too few args.
	_, err = wrapped.Call(
		[]object.Object{object.IntFrom(1)},
		nil,
	)
	if err == nil {
		t.Fatal("expected TypeError for arg count mismatch")
	}
	excObj, ok := err.(*object.ExceptionObject)
	if !ok {
		t.Fatalf("expected *object.ExceptionObject, got %T", err)
	}
	if excObj.ExcType != object.TypeTypeError {
		t.Errorf("expected TypeError, got %s", excObj.ExcType.Name())
	}

	// Too many args.
	_, err = wrapped.Call(
		[]object.Object{object.IntFrom(1), object.IntFrom(2), object.IntFrom(3)},
		nil,
	)
	if err == nil {
		t.Fatal("expected TypeError for too many args")
	}
	excObj, ok = err.(*object.ExceptionObject)
	if !ok {
		t.Fatalf("expected *object.ExceptionObject, got %T", err)
	}
	if excObj.ExcType != object.TypeTypeError {
		t.Errorf("expected TypeError, got %s", excObj.ExcType.Name())
	}
}

func TestWrapGoFunction_InvalidArgType(t *testing.T) {
	add := func(a, b int) int { return a + b }

	wrapped, err := WrapGoFunction("add", add)
	if err != nil {
		t.Fatalf("WrapGoFunction failed: %v", err)
	}

	// Pass a string where int is expected.
	_, err = wrapped.Call(
		[]object.Object{object.StrFrom("not_a_number"), object.IntFrom(2)},
		nil,
	)
	if err == nil {
		t.Fatal("expected TypeError for invalid arg type")
	}
	excObj, ok := err.(*object.ExceptionObject)
	if !ok {
		t.Fatalf("expected *object.ExceptionObject, got %T", err)
	}
	if excObj.ExcType != object.TypeTypeError {
		t.Errorf("expected TypeError, got %s", excObj.ExcType.Name())
	}
}

func TestWrapGoFunction_NonFunction(t *testing.T) {
	_, err := WrapGoFunction("notfunc", 42)
	if err == nil {
		t.Fatal("expected error for non-function argument")
	}
	if err.Error() != "WrapGoFunction: expected a function, got int" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestWrapGoFunction_NilFunc(t *testing.T) {
	_, err := WrapGoFunction("nilfn", nil)
	if err == nil {
		t.Fatal("expected error for nil argument")
	}
}

func TestWrapGoFunction_ErrorOnlyReturn(t *testing.T) {
	validate := func(s string) error {
		if s == "" {
			return errors.New("empty string")
		}
		return nil
	}

	wrapped, err := WrapGoFunction("validate", validate)
	if err != nil {
		t.Fatalf("WrapGoFunction failed: %v", err)
	}

	// Success case — returns None.
	result, err := wrapped.Call(
		[]object.Object{object.StrFrom("hello")},
		nil,
	)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if !object.IsNone(result) {
		t.Errorf("expected None, got %v", result)
	}

	// Error case.
	_, err = wrapped.Call(
		[]object.Object{object.StrFrom("")},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for empty string")
	}
	excObj, ok := err.(*object.ExceptionObject)
	if !ok {
		t.Fatalf("expected *object.ExceptionObject, got %T", err)
	}
	if excObj.Message != "empty string" {
		t.Errorf("unexpected message: %s", excObj.Message)
	}
}

func TestWrapGoFunction_NoReturnValue(t *testing.T) {
	called := false
	sideEffect := func() { called = true }

	wrapped, err := WrapGoFunction("sideEffect", sideEffect)
	if err != nil {
		t.Fatalf("WrapGoFunction failed: %v", err)
	}

	result, err := wrapped.Call([]object.Object{}, nil)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if !object.IsNone(result) {
		t.Errorf("expected None, got %v", result)
	}
	if !called {
		t.Error("expected side effect function to be called")
	}
}
