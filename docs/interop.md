# PyGo — Go ↔ Python Interoperability

## Table of Contents

1. [Design Goals](#1-design-goals)
2. [Type Conversion: Go → Python](#2-type-conversion-go--python)
3. [Type Conversion: Python → Go](#3-type-conversion-python--go)
4. [Wrapping Go Functions](#4-wrapping-go-functions)
5. [Wrapping Go Structs and Methods](#5-wrapping-go-structs-and-methods)
6. [Unsupported and Rejected Types](#6-unsupported-and-rejected-types)
7. [Error Handling Across the Boundary](#7-error-handling-across-the-boundary)
8. [Callbacks: Python Calling Go Calling Python](#8-callbacks-python-calling-go-calling-python)
9. [Variadic Functions](#9-variadic-functions)
10. [Interfaces](#10-interfaces)
11. [Channels and Goroutines](#11-channels-and-goroutines)
12. [Reflection Strategy](#12-reflection-strategy)
13. [Registration API](#13-registration-api)
14. [Security Boundary](#14-security-boundary)
15. [Future: Code Generation](#15-future-code-generation)

---

## 1. Design Goals

1. A Go application can expose functions and objects to Python with minimal boilerplate.
2. Python code calls Go functions as if they were ordinary Python functions.
3. Python code accesses Go struct fields and calls Go methods as if accessing Python attributes.
4. Type conversion is automatic for the common Go type surface.
5. Conversion failures produce clear Python `TypeError` exceptions with actionable messages.
6. The interop layer does not use `unsafe`. All reflection uses the standard `reflect` package.
7. Go errors returned from Go functions are surfaced as Python exceptions.
8. The boundary is explicit: Python code can only reach Go APIs that the host deliberately registers.

---

## 2. Type Conversion: Go → Python

When a Go value crosses into Python (return value from a Go function, or a registered Go value), it is converted to a `object.Object` according to this table:

| Go Type | Python Type | Notes |
|---------|-------------|-------|
| `bool` | `bool` | |
| `int`, `int8`, `int16`, `int32`, `int64` | `int` | |
| `uint`, `uint8`, `uint16`, `uint32`, `uint64` | `int` | |
| `float32`, `float64` | `float` | `float32` is widened to `float64` |
| `string` | `str` | |
| `[]byte` | `bytes` | |
| `nil` (interface) | `None` | |
| `nil` (pointer) | `None` | |
| `error` (non-nil) | — | Raised as exception, not returned as value (see §7) |
| `[]T` | `list` | Elements converted recursively |
| `[N]T` | `list` | Fixed arrays treated as lists |
| `map[K]V` | `dict` | Keys and values converted recursively; key type must be hashable |
| `struct` / `*struct` | `GoObject` | Wrapped; attributes accessible via reflection |
| `interface{}` / `any` | Depends on underlying value | Unwrapped and converted based on concrete type |
| `func(...)` | callable `GoFunction` | Wrapped as callable (see §4) |
| `chan T` | Not supported initially | Rejected with clear error |

**Recursive conversion** applies to slices and maps. Deeply nested structures are supported. Circular Go data structures are not detected and will cause infinite recursion — document as a known limitation.

**Struct field exposure:** Only exported (capitalized) fields and methods of structs are accessible from Python. Unexported fields are silently inaccessible (no error, no `AttributeError` — they simply do not exist from Python's perspective). This is intentional: Go's visibility rules are respected.

---

## 3. Type Conversion: Python → Go

When a Python value is passed to a Go function parameter, PyGo must convert the Python object to the expected Go type:

| Python Type | Target Go Type | Conversion |
|-------------|---------------|------------|
| `int` | `int`, `int64`, `int32`, etc. | Range-checked; `OverflowError` if out of range |
| `int` | `float64` | Widened |
| `float` | `float64` | Direct |
| `float` | `float32` | Narrowed with precision loss warning in debug mode |
| `bool` | `bool` | Direct |
| `None` | pointer type | `nil` |
| `None` | `interface{}` | `nil` |
| `str` | `string` | Direct (UTF-8) |
| `bytes` | `[]byte` | Copy |
| `list` | `[]T` | Elements converted recursively; `TypeError` if element conversion fails |
| `tuple` | `[]T` | Same as list |
| `dict` | `map[K]V` | Keys and values converted; `TypeError` on mismatch |
| `GoObject` | Original Go type | Unwrap and type-assert; `TypeError` if type mismatch |
| Any Python object | `object.Object` | Pass through as Python object (for Go funcs that accept Python objects explicitly) |
| Any Python object | `interface{}` | Wrapped as Python object or converted to Go primitive if possible |

**Type mismatch:** If a Python value cannot be converted to the required Go type, a `TypeError` is raised with a message like:
```
TypeError: argument 1: cannot convert 'str' to int
```

**Implicit narrowing conversions** (e.g., Python `int` to Go `int32`) are performed silently for compatibility with natural Python usage. Range violations raise `OverflowError`.

---

## 4. Wrapping Go Functions

### Registration

```go
runtime.RegisterFunction("add", func(a, b int) int {
    return a + b
})
```

### How it works

At registration time, `reflect.TypeOf(fn)` inspects the function's type. PyGo generates a wrapper:

```go
type GoFunctionObject struct {
    Name    string
    fn      reflect.Value
    inTypes []reflect.Type
    outTypes []reflect.Type
    isVariadic bool
}

func (f *GoFunctionObject) Call(args []Object, kwargs map[string]Object) (Object, error) {
    if len(kwargs) > 0 {
        return nil, TypeError("Go functions do not accept keyword arguments")
    }
    if len(args) != len(f.inTypes) {
        return nil, TypeError("expected %d args, got %d", len(f.inTypes), len(args))
    }
    goArgs := make([]reflect.Value, len(args))
    for i, arg := range args {
        converted, err := pythonToGo(arg, f.inTypes[i])
        if err != nil {
            return nil, TypeError("argument %d: %v", i+1, err)
        }
        goArgs[i] = converted
    }
    results := f.fn.Call(goArgs)
    return handleResults(results, f.outTypes)
}
```

### Return value conventions

| Go function signature | Python return value |
|----------------------|---------------------|
| `func() ` | `None` |
| `func() T` | converted `T` |
| `func() (T, error)` | converted `T`; error raised as exception |
| `func() (T, U)` | `tuple` of (T, U) |
| `func() (T, U, error)` | `tuple` of (T, U); error raised as exception |

**The `error` return value is always the last return.** If non-nil, it is raised as a Python exception. It is not included in the Python tuple return.

This convention must be documented and consistently enforced.

### Keyword arguments

Go functions do not natively support keyword arguments. However, if a Go function accepts a `map[string]interface{}` or `map[string]object.Object` as its last argument, PyGo can forward `**kwargs` to it. This is an advanced feature for Phase 4+.

---

## 5. Wrapping Go Structs and Methods

### Registration

```go
type DeploymentService struct {
    Name string
}

func (d *DeploymentService) Deploy(app, env string) error {
    // ...
    return nil
}

runtime.Register("deployment", &DeploymentService{Name: "prod"})
```

### Python usage

```python
deployment.deploy("my-app", "production")
print(deployment.Name)  # or deployment.name with name normalization
```

### How it works

`Register` creates a `GoObject` wrapping the struct value. When Python accesses an attribute:

1. Check if the name matches an exported method (case-insensitive with snake_case normalization).
2. If method found: return a bound `GoMethodObject`.
3. Check if the name matches an exported field.
4. If field found: return the field value converted to Python.
5. Otherwise: `AttributeError`.

```go
type GoObject struct {
    value     reflect.Value
    typeCache *goTypeCache  // cached method/field index
    typeObj   *TypeObject
}
```

**Name normalization:** Python convention uses `snake_case`; Go uses `CamelCase`. PyGo will attempt the following resolution order:
1. Exact match (e.g., `Name` → `Name`).
2. CamelCase conversion (e.g., `deploy_application` → `DeployApplication`).
3. Lowercase match (e.g., `name` → `Name`).

This is documented behavior. Ambiguous cases (multiple Go fields matching the same Python name) raise a clear error at registration time.

### Field mutation

```python
deployment.Name = "staging"
```

`SetAttr` on a `GoObject` uses `reflect.Value.Set()`. This only works if:
- The field is exported.
- The struct was registered as a pointer (`*DeploymentService`, not `DeploymentService`).

Non-pointer structs are registered as copies — mutation has no effect. PyGo will warn (in debug mode) when a non-pointer struct is registered but field mutation is attempted.

### Methods returning error

```python
deployment.deploy("my-app", "production")  # raises if error != nil
```

Go methods returning `error` follow the same convention as functions: `error` is raised as a Python exception, not returned as a Python value.

---

## 6. Unsupported and Rejected Types

The following Go types are not supported and will raise an error at registration time:

| Go Type | Reason |
|---------|--------|
| `chan T` | Cannot safely bridge Go channels to Python iteration without goroutine semantics |
| `unsafe.Pointer` | Prohibited; use `unsafe` is not permitted |
| `uintptr` | Not meaningful as a Python value |
| Function types with unexported parameter types | Cannot convert without reflection access |
| Recursive struct types | Supported shallowly; circular references are not detected |
| `sync.Mutex`, `sync.RWMutex` | Not meaningful to Python; should be encapsulated |

These restrictions are enforced at `RegisterFunction` / `Register` call time, not lazily at runtime.

Future phases may add support for `chan T` with explicit async/channel semantics.

---

## 7. Error Handling Across the Boundary

### Go error → Python exception

When a Go function returns a non-nil `error`:
- The error is wrapped as a Python `RuntimeError` by default.
- If the error implements a `PyError` interface, the specified Python exception type is used:
  ```go
  type PyError interface {
      error
      PyExceptionType() string // e.g., "ValueError", "TypeError"
      PyMessage() string
  }
  ```
- The Go error message is preserved as the exception message.

### Python exception → Go error

When Python code raises an exception and the Go caller is in a `RunString`/`RunFile` call:
- The exception is caught at the top-level VM frame.
- A `*pygo.Error` is returned to the Go caller.
- `*pygo.Error` implements the `error` interface and contains:
  - Exception type name
  - Exception message
  - Full traceback

Go code calling into Python through a callback (see §8) follows the same pattern.

---

## 8. Callbacks: Python Calling Go Calling Python

A Go function registered with PyGo may itself call back into Python. For example:

```go
runtime.RegisterFunction("apply", func(fn object.Object, x int) (int, error) {
    result, err := runtime.Call(fn, []interface{}{x})
    if err != nil {
        return 0, err
    }
    return result.(int), nil
})
```

Python:
```python
def double(x):
    return x * 2

result = apply(double, 10)  # → 20
```

**This is safe** as long as:
- The runtime is not already concurrently executing Python on the same goroutine (no re-entrant locking needed; Go's stack is not limited).
- The `fn` Python object is not shared across goroutines.

The `runtime.Call(fn, args)` method handles the conversion and invocation. It returns `(object.Object, error)`.

Re-entrancy depth should be bounded by a configurable limit (default: 500 frames, matching CPython's `sys.getrecursionlimit`).

---

## 9. Variadic Functions

Go variadic functions (`func(args ...T)`) are supported:

```go
runtime.RegisterFunction("sum", func(values ...int) int {
    total := 0
    for _, v := range values {
        total += v
    }
    return total
})
```

Python:
```python
result = sum(1, 2, 3, 4)  # → 10
```

The interop layer detects `fn.Type().IsVariadic()` via reflection and handles argument slicing accordingly.

Python `*args` expansion is also supported when calling a Go variadic function:
```python
nums = [1, 2, 3]
result = sum(*nums)
```

---

## 10. Interfaces

Go interfaces are supported with the following behavior:

- A `nil` interface value converts to Python `None`.
- A non-nil interface value is unwrapped to its concrete type and converted.
- A Go function accepting an `interface{}` parameter can receive any Python value:
  - Python primitives are converted to their Go equivalents.
  - Python objects that cannot be cleanly converted are wrapped in an `object.Object` and passed through.

Registering a Go interface type (as opposed to a concrete struct) is supported:

```go
var svc MyService = &myServiceImpl{}
runtime.Register("svc", svc)
```

The concrete type's exported methods are reflected and exposed.

---

## 11. Channels and Goroutines

`chan T` is not supported in the initial implementation. Reasons:

1. Channel operations are blocking — suspending a Python execution on a channel receive would require either a goroutine per Python thread (not viable) or an async/await model (not yet implemented).
2. Sharing Python objects across goroutines is unsafe without a GIL or explicit synchronization.
3. Exposing goroutine creation to Python scripts creates unpredictable resource usage.

**Future (Phase 8+):** When async/await support is added to PyGo, channels can be bridged to Python async iterators.

For now, the recommended pattern is for Go code to do channel operations and pass results to Python via callbacks or return values.

---

## 12. Reflection Strategy

All interop uses `reflect`. No `unsafe` is used.

**Type cache:** Reflection on a given Go type is expensive on first use. PyGo caches the result of type inspection (methods, fields, their types and indices) in a `sync.Map` keyed by `reflect.Type`. Subsequent calls on the same type use the cache.

```go
var typeCache sync.Map // reflect.Type → *goTypeInfo

type goTypeInfo struct {
    methods []goMethodInfo
    fields  []goFieldInfo
}
```

This makes the first call to a method/field slightly expensive (reflection overhead), and subsequent calls fast (only `reflect.Value.Method(i).Call(...)` is needed, with the index looked up from the cache).

**Performance note:** Reflection-based dispatch is roughly 5–10× slower than direct Go function calls. For embedding use cases where PyGo executes Python logic that occasionally calls into Go, this is acceptable. For high-frequency Go call paths, code-generation (Phase 8) can produce direct dispatch wrappers.

---

## 13. Registration API

The full registration surface exposed by the public `pygo` package:

```go
// RegisterFunction exposes a Go function to Python under the given name.
// fn must be a function value. Panics if fn is not a function or the
// parameter/return types are not supported.
func (r *Runtime) RegisterFunction(name string, fn interface{}) error

// Register exposes a Go value (struct, pointer, or primitive) to Python
// under the given name.
func (r *Runtime) Register(name string, value interface{}) error

// RegisterModule creates a Python module populated with the given map
// of name → value. Values are converted using the standard Go→Python rules.
func (r *Runtime) RegisterModule(name string, attrs map[string]interface{}) error

// Call invokes a Python callable from Go with the given arguments.
// args are converted from Go to Python using standard rules.
// Returns the Python result converted back to Go (as interface{}).
func (r *Runtime) Call(callable interface{}, args []interface{}) (interface{}, error)
```

---

## 14. Security Boundary

**The interop layer is the security boundary.**

Python scripts can only reach Go functionality that the host deliberately registers. A Python script running inside PyGo cannot:
- Access the host process's environment variables (unless `os` module is enabled).
- Execute system commands (unless `subprocess` is registered or `os.system` is enabled).
- Read or write files (unless the `open` built-in or `os` module is enabled).
- Create network connections (unless a socket API is registered).
- Access other `Runtime` instances' state.

This is enforced by controlling which modules and built-ins are included at runtime initialization. A "restricted" runtime configuration can disable all I/O-capable built-ins and modules.

See `docs/security.md` for the full security model.

---

## 15. Future: Code Generation

In Phase 8+, the interop layer can be supplemented by a code-generation tool:

```bash
pygo-gen ./mypkg
```

This tool would:
1. Inspect Go packages using `go/ast` and `go/types`.
2. Generate static Go bridge code (no reflection at runtime).
3. Register the generated bridges with PyGo for zero-overhead interop.

This is strictly a performance optimization. The reflection-based layer is the primary API and must remain correct and complete. Code generation is opt-in and never required.
