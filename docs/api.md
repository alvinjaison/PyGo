# PyGo — Public Go API

## Table of Contents

1. [Design Philosophy](#1-design-philosophy)
2. [Package Structure](#2-package-structure)
3. [Runtime Type](#3-runtime-type)
4. [Construction and Options](#4-construction-and-options)
5. [Execution Methods](#5-execution-methods)
6. [Variable Access](#6-variable-access)
7. [Registration Methods](#7-registration-methods)
8. [Calling Python from Go](#8-calling-python-from-go)
9. [Error Types](#9-error-types)
10. [Value Interface](#10-value-interface)
11. [Cancellation and Limits](#11-cancellation-and-limits)
12. [Diagnostics](#12-diagnostics)
13. [Complete API Surface (Reference)](#13-complete-api-surface-reference)
14. [Usage Examples](#14-usage-examples)

---

## 1. Design Philosophy

- **Small surface.** The public API exposes only what consumers need. All implementation details are in `internal/`.
- **Idiomatic Go.** No Python-isms in the Go API. Functions return `error`, not exceptions. Constructors use functional options.
- **Stable.** The API is designed to remain backward-compatible across minor versions. Breaking changes require a major version bump.
- **No magic.** The API does not use `interface{}` where a concrete type can be used. Where `interface{}` is unavoidable (generic value passing), the behavior is clearly documented.
- **Embeddable.** Creating a runtime is a single function call. There are no global singletons.

---

## 2. Package Structure

```
github.com/pygo/pygo         ← import this package
```

Consumers never import `internal/` packages directly. Go's module system enforces this.

```go
import "github.com/pygo/pygo"

rt := pygo.New()
```

---

## 3. Runtime Type

```go
// Runtime is a single Python execution context.
// Each Runtime has its own globals, module state, and object state.
// Multiple Runtime instances can coexist safely in the same process.
//
// A Runtime is not safe for concurrent use by multiple goroutines.
// If concurrent Python execution is needed, create separate Runtime instances.
type Runtime struct {
    // unexported fields
}
```

**Isolation guarantee:** Two `Runtime` instances do not share any mutable state. Registering a Go object in one runtime does not affect another. Module state is per-runtime.

**Thread safety:** A single `Runtime` must not be used from multiple goroutines simultaneously. This is consistent with CPython's GIL behavior and simplifies the implementation significantly. If concurrent execution is needed, create multiple runtimes. A future version may offer explicit locking or a thread-safe wrapper.

---

## 4. Construction and Options

```go
// New creates and initializes a new Runtime with the default configuration.
// It initializes the built-in namespace, standard type objects, and core modules.
func New(opts ...Option) *Runtime

// Option is a functional option for configuring a Runtime.
type Option func(*runtimeConfig)

// WithStdlib controls whether the standard library modules are pre-loaded.
// Default: true.
func WithStdlib(enabled bool) Option

// WithSearchPath sets the Python module search path (equivalent to sys.path).
// Default: current working directory.
func WithSearchPath(paths []string) Option

// WithMaxCallDepth sets the maximum call stack depth.
// Default: 500. Exceeding this raises RecursionError.
func WithMaxCallDepth(depth int) Option

// WithDiagnostics sets a writer for diagnostic output (bytecode disassembly,
// AST dumps, etc.). If nil, diagnostics are disabled. Default: nil.
func WithDiagnostics(w io.Writer) Option

// WithHashSeed sets a fixed seed for hash randomization.
// Only use this in tests that require deterministic hash ordering.
// Default: random seed chosen at runtime creation.
func WithHashSeed(seed uint64) Option

// WithRestrictedBuiltins creates a runtime where I/O-capable built-ins
// (open, print-to-file, input) and OS-level modules are disabled.
// Useful for sandboxed script execution.
func WithRestrictedBuiltins() Option
```

---

## 5. Execution Methods

```go
// RunString compiles and executes a Python source string.
// The source is executed in the runtime's global namespace.
// Returns a non-nil error if compilation fails or a Python exception is raised.
func (r *Runtime) RunString(source string) error

// RunStringContext is like RunString but respects the provided context
// for cancellation. If ctx is cancelled while Python code is running,
// execution is interrupted and an error is returned.
func (r *Runtime) RunStringContext(ctx context.Context, source string) error

// RunFile reads and executes a Python source file.
// The file path is used in tracebacks.
func (r *Runtime) RunFile(path string) error

// RunFileContext is like RunFile but respects the provided context.
func (r *Runtime) RunFileContext(ctx context.Context, path string) error

// Eval compiles and evaluates a Python expression, returning the result.
// The expression must be a single expression (not a statement or block).
// The result is the Python value as a Value interface.
func (r *Runtime) Eval(expr string) (Value, error)

// EvalContext is like Eval but respects the provided context.
func (r *Runtime) EvalContext(ctx context.Context, expr string) (Value, error)
```

**`RunString` vs `Eval`:** `RunString` executes statements (assignments, loops, function definitions, print calls — anything Python can do). It returns no value (Python modules do not have a "return value"). `Eval` evaluates a single expression and returns the result as a `Value`.

---

## 6. Variable Access

```go
// Set sets a variable in the runtime's global namespace.
// value is converted from Go to Python using standard type conversion rules.
// Returns an error if value cannot be converted.
func (r *Runtime) Set(name string, value interface{}) error

// Get retrieves a variable from the runtime's global namespace.
// Returns the value as a Value interface, or an error if the name is not defined.
func (r *Runtime) Get(name string) (Value, error)

// Delete removes a variable from the runtime's global namespace.
// No error if the variable does not exist.
func (r *Runtime) Delete(name string)
```

**Example:**
```go
rt := pygo.New()
rt.Set("config", map[string]interface{}{"debug": true, "port": 8080})
rt.RunString(`print(config["port"])`)
result, _ := rt.Get("config")
```

---

## 7. Registration Methods

```go
// RegisterFunction exposes a Go function to Python under the given name
// in the runtime's global namespace.
//
// fn must be a function. Accepted parameter and return types are documented
// in docs/interop.md. Returns an error if fn is not a function or contains
// unsupported types.
func (r *Runtime) RegisterFunction(name string, fn interface{}) error

// Register exposes a Go value (struct pointer, primitive, etc.) to Python
// under the given name in the runtime's global namespace.
// The value's exported methods and fields become Python attributes.
func (r *Runtime) Register(name string, value interface{}) error

// RegisterModule creates a Python module and populates it with the provided
// attributes. The module is importable by Python code as `import name`.
//
// attrs maps Python attribute names to Go values, converted by standard rules.
func (r *Runtime) RegisterModule(name string, attrs map[string]interface{}) error

// MustRegisterFunction is like RegisterFunction but panics on error.
// Useful for init() registration where an error means a programming mistake.
func (r *Runtime) MustRegisterFunction(name string, fn interface{})

// MustRegister is like Register but panics on error.
func (r *Runtime) MustRegister(name string, value interface{})
```

---

## 8. Calling Python from Go

```go
// Call invokes a Python callable (function, class, built-in) with positional arguments.
// args are converted from Go to Python.
// The return value is the Python result as a Value.
func (r *Runtime) Call(callable Value, args ...interface{}) (Value, error)

// CallByName looks up a name in the global namespace and calls it.
// Equivalent to Get(name) followed by Call(value, args...).
func (r *Runtime) CallByName(name string, args ...interface{}) (Value, error)

// CallMethod looks up an attribute on obj and calls it.
// Equivalent to accessing obj.method and calling it.
func (r *Runtime) CallMethod(obj Value, method string, args ...interface{}) (Value, error)
```

**Example:**
```go
rt := pygo.New()
rt.RunString(`
def greet(name):
    return "Hello, " + name
`)

result, err := rt.CallByName("greet", "world")
if err != nil {
    log.Fatal(err)
}
fmt.Println(result.String()) // Hello, world
```

---

## 9. Error Types

```go
// Error represents a Python exception raised during execution.
// It implements the error interface.
type Error struct {
    // Type is the Python exception class name (e.g., "ValueError", "TypeError").
    Type string

    // Message is the exception's string representation.
    Message string

    // Traceback contains the Python call stack at the point the exception was raised.
    Traceback []TracebackEntry
}

func (e *Error) Error() string // returns "TypeName: Message"

// IsError reports whether err is a *Error with the given Python exception type.
// Useful for checking specific exception types without type asserting.
//
// Example: pygo.IsError(err, "ValueError")
func IsError(err error, typeName string) bool

// TracebackEntry represents one frame in a Python traceback.
type TracebackEntry struct {
    Filename string
    FuncName string
    Line     int
}
```

**Pattern for handling specific exceptions:**

```go
err := rt.RunString(`
raise ValueError("bad input")
`)
if pygo.IsError(err, "ValueError") {
    log.Println("Python ValueError:", err)
}
```

**Compile errors** (syntax errors) are also returned as `*Error` with type `"SyntaxError"`.

---

## 10. Value Interface

```go
// Value represents a Python value accessible from Go.
// It provides type inspection and conversion to Go types.
type Value interface {
    // Type returns the Python type name (e.g., "int", "str", "list").
    Type() string

    // String returns the string representation (equivalent to str(v) in Python).
    String() string

    // IsNone reports whether this value is Python None.
    IsNone() bool

    // Int converts the value to int64.
    // Returns an error if the value is not a Python int or bool.
    Int() (int64, error)

    // Float converts the value to float64.
    // Returns an error if the value is not a Python float or int.
    Float() (float64, error)

    // Bool converts the value to bool using Python's truth-value testing.
    // All values are convertible (equivalent to bool(v) in Python).
    Bool() bool

    // Str converts the value to string.
    // Returns an error if the value is not a Python str.
    Str() (string, error)

    // Bytes converts the value to []byte.
    // Returns an error if the value is not a Python bytes.
    Bytes() ([]byte, error)

    // List converts the value to []Value.
    // Returns an error if the value is not a Python list or tuple.
    List() ([]Value, error)

    // Dict converts the value to map[string]Value.
    // Returns an error if the value is not a Python dict or if keys are not strings.
    Dict() (map[string]Value, error)

    // Unwrap returns the underlying Go value if this Value wraps a Go object
    // (registered via Register or RegisterFunction's return).
    // Returns nil if the value is a pure Python value with no Go backing.
    Unwrap() interface{}
}
```

**Example:**
```go
result, _ := rt.Eval("[1, 2, 3]")
items, _ := result.List()
for _, item := range items {
    n, _ := item.Int()
    fmt.Println(n)
}
```

---

## 11. Cancellation and Limits

```go
// The Runtime respects context.Context for cancellation.
// Cancellation is checked at:
//   - loop backedges (for/while loops)
//   - function calls
//   - yield points
//
// When a context is cancelled, Python execution is interrupted with
// a KeyboardInterrupt-equivalent exception. Python code can catch this
// with `except KeyboardInterrupt`, but the runtime will forcibly terminate
// after a short grace period (configurable).

rt.RunStringContext(ctx, source)

// WithExecutionTimeout is a convenience wrapper for context timeout:
func WithExecutionTimeout(d time.Duration) Option
```

**Execution limits (Phase 2+):**
```go
// WithMaxInstructions sets a limit on the number of bytecode instructions
// that can be executed per RunString/RunFile call.
// Exceeding this raises a RuntimeError.
func WithMaxInstructions(n int64) Option

// WithMaxMemory sets a soft limit on Python heap usage.
// Exceeding this raises a MemoryError.
// Note: this is a best-effort limit; Go's GC does not provide hard limits.
func WithMaxMemory(bytes int64) Option
```

---

## 12. Diagnostics

```go
// Disassemble compiles the source and writes the bytecode disassembly
// to the provided writer. Useful for debugging compiler output.
func (r *Runtime) Disassemble(source string, w io.Writer) error

// DumpAST parses the source and writes a text representation of the AST
// to the provided writer. Useful for debugging parser output.
func (r *Runtime) DumpAST(source string, w io.Writer) error
```

These methods are intended for development and debugging. They should not be called in production hot paths.

---

## 13. Complete API Surface (Reference)

```go
package pygo

// Construction
func New(opts ...Option) *Runtime
func WithStdlib(enabled bool) Option
func WithSearchPath(paths []string) Option
func WithMaxCallDepth(depth int) Option
func WithDiagnostics(w io.Writer) Option
func WithHashSeed(seed uint64) Option
func WithRestrictedBuiltins() Option
func WithExecutionTimeout(d time.Duration) Option

// Execution
func (r *Runtime) RunString(source string) error
func (r *Runtime) RunStringContext(ctx context.Context, source string) error
func (r *Runtime) RunFile(path string) error
func (r *Runtime) RunFileContext(ctx context.Context, path string) error
func (r *Runtime) Eval(expr string) (Value, error)
func (r *Runtime) EvalContext(ctx context.Context, expr string) (Value, error)

// Variable access
func (r *Runtime) Set(name string, value interface{}) error
func (r *Runtime) Get(name string) (Value, error)
func (r *Runtime) Delete(name string)

// Registration
func (r *Runtime) RegisterFunction(name string, fn interface{}) error
func (r *Runtime) Register(name string, value interface{}) error
func (r *Runtime) RegisterModule(name string, attrs map[string]interface{}) error
func (r *Runtime) MustRegisterFunction(name string, fn interface{})
func (r *Runtime) MustRegister(name string, value interface{})

// Calling Python from Go
func (r *Runtime) Call(callable Value, args ...interface{}) (Value, error)
func (r *Runtime) CallByName(name string, args ...interface{}) (Value, error)
func (r *Runtime) CallMethod(obj Value, method string, args ...interface{}) (Value, error)

// Diagnostics
func (r *Runtime) Disassemble(source string, w io.Writer) error
func (r *Runtime) DumpAST(source string, w io.Writer) error

// Error types
type Error struct { Type, Message string; Traceback []TracebackEntry }
func (e *Error) Error() string
func IsError(err error, typeName string) bool
type TracebackEntry struct { Filename, FuncName string; Line int }

// Value interface
type Value interface { ... }  // see §10
```

Total exported symbols: ~30. This is intentionally small.

---

## 14. Usage Examples

### Hello World

```go
rt := pygo.New()
if err := rt.RunString(`print("Hello, world!")`); err != nil {
    log.Fatal(err)
}
```

### Exposing a Go function

```go
rt := pygo.New()
rt.MustRegisterFunction("multiply", func(a, b int) int {
    return a * b
})
rt.RunString(`print(multiply(6, 7))`)  // 42
```

### Retrieving a result

```go
rt := pygo.New()
rt.RunString(`result = sum(range(1, 101))`)
v, _ := rt.Get("result")
n, _ := v.Int()
fmt.Println(n) // 5050
```

### Exposing a Go struct

```go
type Config struct {
    Debug bool
    Port  int
}

rt := pygo.New()
rt.MustRegister("config", &Config{Debug: true, Port: 8080})
rt.RunString(`
if config.Debug:
    print(f"Server running on port {config.Port}")
`)
```

### Handling Python exceptions

```go
rt := pygo.New()
err := rt.RunString(`
x = int("not a number")
`)
if pygo.IsError(err, "ValueError") {
    fmt.Println("Got expected ValueError:", err)
}
```

### Cancellation

```go
rt := pygo.New()
ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
defer cancel()

err := rt.RunStringContext(ctx, `
while True:
    pass
`)
if err != nil {
    fmt.Println("Execution cancelled:", err)
}
```

### Calling Python from Go

```go
rt := pygo.New()
rt.RunString(`
def process(items):
    return [x * 2 for x in items]
`)

result, _ := rt.CallByName("process", []interface{}{1, 2, 3, 4, 5})
items, _ := result.List()
for _, item := range items {
    n, _ := item.Int()
    fmt.Print(n, " ")
}
// Output: 2 4 6 8 10
```

### Module registration

```go
rt := pygo.New()
rt.RegisterModule("myapp", map[string]interface{}{
    "version": "1.0.0",
    "debug":   false,
    "connect": func(host string, port int) error {
        // ...
        return nil
    },
})

rt.RunString(`
import myapp
myapp.connect("localhost", 5432)
print(myapp.version)
`)
```
