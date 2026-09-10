package pygo_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alvinjaison/pygo/pygo"
)

func TestNew_Defaults(t *testing.T) {
	rt := pygo.New()
	if rt == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_WithOptions(t *testing.T) {
	rt := pygo.New(
		pygo.WithMaxCallDepth(100),
		pygo.WithSearchPath([]string{"/tmp"}),
	)
	if rt == nil {
		t.Fatal("New(WithMaxCallDepth, WithSearchPath) returned nil")
	}
}

func TestRunString_Print(t *testing.T) {
	rt := pygo.New()
	err := rt.RunString(`x = 1 + 2`)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}
}

func TestRunString_SyntaxError(t *testing.T) {
	rt := pygo.New()
	err := rt.RunString(`def`)
	if err == nil {
		t.Fatal("expected error for invalid syntax, got nil")
	}
	if !pygo.IsError(err, "SyntaxError") {
		t.Fatalf("expected SyntaxError, got: %v", err)
	}
}

func TestRunString_RuntimeError(t *testing.T) {
	rt := pygo.New()
	err := rt.RunString(`raise ValueError("bad input")`)
	if err == nil {
		t.Fatal("expected error for raise, got nil")
	}
	if !pygo.IsError(err, "ValueError") {
		t.Fatalf("expected ValueError, got: %v", err)
	}
	pyErr := pygo.AsError(err)
	if pyErr == nil {
		t.Fatal("AsError returned nil")
	}
	if pyErr.Message != "bad input" {
		t.Fatalf("expected message 'bad input', got %q", pyErr.Message)
	}
}

func TestSet_Get_RoundTrip(t *testing.T) {
	rt := pygo.New()

	// Set an integer
	if err := rt.Set("x", 42); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get it back
	val, err := rt.Get("x")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val.Type() != "int" {
		t.Fatalf("expected type 'int', got %q", val.Type())
	}
	n, err := val.Int()
	if err != nil {
		t.Fatalf("Int() failed: %v", err)
	}
	if n != 42 {
		t.Fatalf("expected 42, got %d", n)
	}
}

func TestSet_Get_String(t *testing.T) {
	rt := pygo.New()

	if err := rt.Set("name", "hello"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := rt.Get("name")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	s, err := val.Str()
	if err != nil {
		t.Fatalf("Str() failed: %v", err)
	}
	if s != "hello" {
		t.Fatalf("expected 'hello', got %q", s)
	}
}

func TestSet_Get_Float(t *testing.T) {
	rt := pygo.New()

	if err := rt.Set("pi", 3.14); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := rt.Get("pi")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	f, err := val.Float()
	if err != nil {
		t.Fatalf("Float() failed: %v", err)
	}
	if f != 3.14 {
		t.Fatalf("expected 3.14, got %f", f)
	}
}

func TestSet_Get_Bool(t *testing.T) {
	rt := pygo.New()

	if err := rt.Set("flag", true); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := rt.Get("flag")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !val.Bool() {
		t.Fatal("expected Bool() to return true")
	}
}

func TestGet_Undefined(t *testing.T) {
	rt := pygo.New()
	_, err := rt.Get("undefined_var")
	if err == nil {
		t.Fatal("expected error for undefined variable, got nil")
	}
	if !pygo.IsError(err, "NameError") {
		t.Fatalf("expected NameError, got: %v", err)
	}
}

func TestRegisterFunction_CallFromPython(t *testing.T) {
	rt := pygo.New()

	// Register a Go function
	err := rt.RegisterFunction("add", func(a, b int64) int64 {
		return a + b
	})
	if err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}

	// Call it from Python
	err = rt.RunString(`result = add(3, 4)`)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	// Verify result
	val, err := rt.Get("result")
	if err != nil {
		t.Fatalf("Get result failed: %v", err)
	}
	n, err := val.Int()
	if err != nil {
		t.Fatalf("Int() failed: %v", err)
	}
	if n != 7 {
		t.Fatalf("expected 7, got %d", n)
	}
}

func TestRegisterFunction_StringReturn(t *testing.T) {
	rt := pygo.New()

	err := rt.RegisterFunction("greet", func(name string) string {
		return "Hello, " + name + "!"
	})
	if err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}

	err = rt.RunString(`msg = greet("World")`)
	if err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	val, err := rt.Get("msg")
	if err != nil {
		t.Fatalf("Get msg failed: %v", err)
	}
	s, err := val.Str()
	if err != nil {
		t.Fatalf("Str() failed: %v", err)
	}
	if s != "Hello, World!" {
		t.Fatalf("expected 'Hello, World!', got %q", s)
	}
}

func TestRegisterFunction_InvalidArg(t *testing.T) {
	rt := pygo.New()

	err := rt.RegisterFunction("noop", nil)
	if err == nil {
		t.Fatal("expected error for nil function, got nil")
	}
}

func TestEval(t *testing.T) {
	rt := pygo.New()

	if err := rt.Set("x", 10); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := rt.Eval("x + 5")
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	n, err := val.Int()
	if err != nil {
		t.Fatalf("Int() failed: %v", err)
	}
	if n != 15 {
		t.Fatalf("expected 15, got %d", n)
	}
}

func TestRunFile(t *testing.T) {
	// Create a temporary Python file.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.py")
	content := []byte("result = 100 + 200\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	rt := pygo.New()
	err := rt.RunFile(path)
	if err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	val, err := rt.Get("result")
	if err != nil {
		t.Fatalf("Get result failed: %v", err)
	}
	n, err := val.Int()
	if err != nil {
		t.Fatalf("Int() failed: %v", err)
	}
	if n != 300 {
		t.Fatalf("expected 300, got %d", n)
	}
}

func TestRunFile_NotFound(t *testing.T) {
	rt := pygo.New()
	err := rt.RunFile("/nonexistent/file.py")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestValue_IsNone(t *testing.T) {
	rt := pygo.New()
	if err := rt.RunString(`x = None`); err != nil {
		t.Fatalf("RunString failed: %v", err)
	}
	val, err := rt.Get("x")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !val.IsNone() {
		t.Fatal("expected IsNone() to return true")
	}
}

func TestValue_BoolTruthiness(t *testing.T) {
	rt := pygo.New()

	// Empty string is falsy
	if err := rt.Set("empty", ""); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	val, err := rt.Get("empty")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val.Bool() {
		t.Fatal("expected empty string to be falsy")
	}

	// Non-empty string is truthy
	if err := rt.Set("nonempty", "hi"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	val, err = rt.Get("nonempty")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !val.Bool() {
		t.Fatal("expected non-empty string to be truthy")
	}
}

func TestError_Format(t *testing.T) {
	pyErr := &pygo.Error{
		Type:    "ValueError",
		Message: "something went wrong",
		Traceback: []pygo.TracebackEntry{
			{Filename: "test.py", FuncName: "<module>", Line: 1},
		},
	}
	formatted := pyErr.Format()
	if formatted == "" {
		t.Fatal("Format() returned empty string")
	}
	errStr := pyErr.Error()
	if errStr != "ValueError: something went wrong" {
		t.Fatalf("unexpected Error(): %q", errStr)
	}
}

func TestDelete(t *testing.T) {
	rt := pygo.New()
	if err := rt.Set("x", 10); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	rt.Delete("x")
	_, err := rt.Get("x")
	if err == nil {
		t.Fatal("expected error after Delete, got nil")
	}
}
