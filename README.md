# PyGo

A Python 3 runtime implemented entirely in Go.

PyGo lets Go applications execute Python code without requiring a CPython installation. It implements its own lexer, parser, compiler, bytecode virtual machine, and object model — all in pure Go.

```go
rt := pygo.New()
rt.MustRegisterFunction("greet", func(name string) string {
    return "Hello, " + name
})
rt.RunString(`print(greet("world"))`)  // Hello, world
```

---

## Why PyGo?

**Existing approaches** (gopy, go-python3) wrap CPython using cgo. Your Go binary still requires a Python installation at runtime.

**PyGo** is the runtime:

```
Python source
    ↓
PyGo lexer → parser → compiler → bytecode
    ↓
PyGo virtual machine (pure Go)
    ↓
Go host application
```

No CPython. No cgo. One binary.

Useful for:
- **Scriptable applications** — expose Go APIs to user-written Python scripts
- **Plugin systems** — sandboxed Python plugins with explicit capability grants
- **Portable deployment** — ship a single binary that includes a Python engine
- **Testing infrastructure** — drive Go services from Python scripts without a CPython install in CI

---

## Status

**Phase 0 — Architecture complete.** The design documents, object model, public API, and skeleton implementation are in place. Phase 1 (minimal interpreter) is the next milestone.

This project is in early development. No Python programs execute yet. Contributions are welcome — see [docs/contributing.md](docs/contributing.md).

| Phase | Goal | Status |
|-------|------|--------|
| 0 | Architecture and design | ✅ Complete |
| 1 | `print("Hello, world!")` works | 🔨 Next |
| 2 | Core language (functions, loops, exceptions) | Planned |
| 3 | Classes and object model | Planned |
| 4 | Go ↔ Python interoperability | Planned |
| 5 | Modules and standard library | Planned |
| 6 | CLI and REPL | Planned |
| 7 | Compatibility expansion | Planned |
| 8 | Performance and production hardening | Planned |

See [docs/roadmap.md](docs/roadmap.md) for detailed scope and effort estimates.

---

## Installation

```bash
go get github.com/pygo/pygo
```

Requires Go 1.22+. No other dependencies at runtime.

---

## Quick Start

### Embedded library

```go
package main

import (
    "fmt"
    "log"

    "github.com/pygo/pygo/pygo"
)

func main() {
    rt := pygo.New()

    // Execute Python source
    if err := rt.RunString(`
x = 10
y = 20
print(x + y)
`); err != nil {
        log.Fatal(err)
    }

    // Retrieve a value
    result, _ := rt.Get("x")
    n, _ := result.Int()
    fmt.Println(n) // 10
}
```

### Exposing Go functions

```go
rt := pygo.New()
rt.MustRegisterFunction("add", func(a, b int) int {
    return a + b
})
rt.RunString(`print(add(3, 4))`)  // 7
```

### Exposing Go structs

```go
type Config struct {
    Debug bool
    Port  int
}

rt := pygo.New()
rt.MustRegister("config", &Config{Debug: true, Port: 8080})
rt.RunString(`
if config.Debug:
    print(f"Listening on port {config.Port}")
`)
```

### Handling Python exceptions

```go
err := rt.RunString(`raise ValueError("bad input")`)
if pygo.IsError(err, "ValueError") {
    fmt.Println("caught:", err)
}
```

### Cancellation

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

err := rt.RunStringContext(ctx, `
while True:
    pass  # infinite loop — cancelled after 5s
`)
```

### Standalone CLI (Phase 6)

```bash
pygo script.py
pygo -c "print(10 + 20)"
pygo                        # interactive REPL
```

---

## Project Layout

```
github.com/pygo/pygo
│
├── pygo/                   ← Public API (import this)
│   ├── pygo.go             ← Runtime type, New(), options
│   ├── error.go            ← Error type, IsError()
│   └── value.go            ← Value interface
│
├── internal/               ← Implementation (not importable externally)
│   ├── lexer/              ← Python source → token stream
│   ├── parser/             ← Token stream → AST
│   ├── ast/                ← AST node type definitions
│   ├── compiler/           ← AST → bytecode
│   ├── bytecode/           ← Opcode definitions, Code object
│   ├── vm/                 ← Bytecode execution engine
│   ├── object/             ← Python object model (int, str, list, dict, …)
│   ├── runtime/            ← Execution coordinator, built-ins
│   ├── interop/            ← Go ↔ Python type conversion
│   ├── importer/           ← Module resolution and loading
│   └── stdlib/             ← Standard library modules
│
├── cmd/pygo/               ← Standalone CLI binary
├── examples/               ← Embedding examples
├── testdata/               ← Test inputs and compat scripts
└── docs/                   ← Design documents
```

The `internal/` packages are the only things that ever import each other.
Consumers only ever import `github.com/pygo/pygo/pygo`.

---

## Public API

```go
// Construction
rt := pygo.New(opts...)

// Execution
rt.RunString(source string) error
rt.RunStringContext(ctx, source) error
rt.RunFile(path string) error
rt.Eval(expr string) (Value, error)

// Variable access
rt.Set(name string, value interface{}) error
rt.Get(name string) (Value, error)
rt.Delete(name string)

// Registration
rt.RegisterFunction(name string, fn interface{}) error
rt.Register(name string, value interface{}) error
rt.RegisterModule(name string, attrs map[string]interface{}) error

// Calling Python from Go
rt.Call(callable Value, args ...interface{}) (Value, error)
rt.CallByName(name string, args ...interface{}) (Value, error)

// Error handling
pygo.IsError(err error, typeName string) bool  // e.g. IsError(err, "ValueError")
pygo.AsError(err error) *pygo.Error
```

Full API documentation: [docs/api.md](docs/api.md)

---

## Python Compatibility

PyGo targets **Python 3 language compatibility**. It does not aim for CPython ABI compatibility or support for C extension modules.

Current compatibility status: **pre-implementation** (Phase 0).

The target compatibility surface for the MVP (Phase 2):

| Feature | Target |
|---------|--------|
| Variables, arithmetic, strings, booleans, None | ✅ MVP |
| Lists, tuples, dicts, sets | ✅ MVP |
| Control flow (if, for, while, break, continue) | ✅ MVP |
| Functions, closures, *args, **kwargs | ✅ MVP |
| Exceptions (try/except/finally) | ✅ MVP |
| f-strings (basic) | ✅ MVP |
| Classes and inheritance | Phase 3 |
| Generators | Phase 2+ |
| Comprehensions | Phase 2+ |
| async/await | Phase 6+ |
| Standard library (math, json, re, os subset) | Phase 5 |

Full matrix: [docs/compatibility.md](docs/compatibility.md)

**CPython is used as a reference for tests, not as a runtime dependency.**

---

## Architecture

PyGo uses a bytecode compiler and stack-based virtual machine — the same fundamental architecture as CPython, Lua, and most production interpreters.

```
Python source
    ↓  internal/lexer
Token stream
    ↓  internal/parser
AST
    ↓  internal/compiler
Bytecode (Code object)
    ↓  internal/vm
Result / Exception
```

Each stage is independently testable. No stage reaches into another stage's internals.

Full design: [docs/architecture.md](docs/architecture.md)

---

## Go ↔ Python Interoperability

Go functions, structs, and values are exposed to Python through an explicit registration API backed by `reflect`. Python scripts can only reach Go APIs that the host deliberately registers.

Supported Go types (Go → Python):

| Go type | Python type |
|---------|-------------|
| `bool` | `bool` |
| `int`, `int64`, etc. | `int` |
| `float64` | `float` |
| `string` | `str` |
| `[]byte` | `bytes` |
| `[]T` | `list` |
| `map[K]V` | `dict` |
| `struct` / `*struct` | object with attributes |
| `func(...)` | callable |
| `nil` / `error` (non-nil) | `None` / exception |

Full design: [docs/interop.md](docs/interop.md)

---

## Security

PyGo is not a secure sandbox by default. However, Python scripts can only access Go capabilities that the host explicitly registers.

For reduced-capability execution:

```go
rt := pygo.New(pygo.WithRestrictedBuiltins())
// No open(), no os module, no exec/eval
// Only safe data-processing built-ins
rt.MustRegisterFunction("query", myQueryFunc) // explicit grant
rt.RunString(userScript)
```

Full security model: [docs/security.md](docs/security.md)

---

## Contributing

PyGo is designed for open-source contribution. Components are independently workable — you do not need to understand the entire runtime to contribute.

**Good entry points by area:**

| Area | Prerequisites | Good for |
|------|--------------|---------|
| Standard library modules | Go | Anyone familiar with the Python module |
| Lexer / Parser | Go | People who enjoy language parsing |
| Object model types | Go | People who know Python's data model |
| VM opcodes | Go, stack VMs | Systems programming interest |
| Tests / compat scripts | Python, Go | QA-minded contributors |
| Documentation | Writing | Anyone |

See [docs/contributing.md](docs/contributing.md) for the full guide, dependency map, and getting-started checklist.

---

## Documentation Index

| Document | Contents |
|----------|----------|
| [VISION.md](VISION.md) | Project vision, goals, non-goals |
| [docs/architecture.md](docs/architecture.md) | Full architecture with execution flow |
| [docs/object-model.md](docs/object-model.md) | Python object model design |
| [docs/interop.md](docs/interop.md) | Go ↔ Python interoperability |
| [docs/api.md](docs/api.md) | Public Go API reference |
| [docs/compatibility.md](docs/compatibility.md) | Python 3 feature compatibility matrix |
| [docs/testing.md](docs/testing.md) | Testing strategy and infrastructure |
| [docs/cli.md](docs/cli.md) | CLI and REPL design |
| [docs/security.md](docs/security.md) | Security model and considerations |
| [docs/roadmap.md](docs/roadmap.md) | Phased roadmap with effort estimates |
| [docs/contributing.md](docs/contributing.md) | Contribution guide |

---

## License

MIT License. See [LICENSE](LICENSE) for details.

---

## Comparison with Related Projects

| Project | Requires CPython | Embedding-first | Pure Go |
|---------|-----------------|-----------------|---------|
| [gopy](https://github.com/go-python/gopy) | Yes | No | No |
| [go-python3](https://github.com/DataDog/go-python3) | Yes | Yes | No (cgo) |
| [gpython](https://github.com/go-python/gpython) | No | Partial | Yes |
| **PyGo** | **No** | **Yes** | **Yes** |

PyGo is most similar to gpython in spirit, but is designed from the start as an embedding-first library with a production-quality architecture, a comprehensive design document set, and a clear compatibility roadmap.
