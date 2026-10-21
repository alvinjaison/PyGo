# PyGo — Project Vision

## What is PyGo?

PyGo is a Python 3 runtime implemented entirely in Go. It allows Go applications to execute Python code without requiring CPython as a runtime dependency.

PyGo is not a CPython wrapper, not a cgo binding generator, and not a CPython extension system. It is an independent interpreter and runtime: lexer, parser, compiler, bytecode virtual machine, object model, and standard library — all written in Go.

## Why PyGo?

Several scenarios benefit from an embeddable, CPython-free Python runtime:

- **Scriptable Go applications** — expose Go APIs to end-user Python scripts without shipping or installing CPython.
- **Plugin systems** — allow plugins written in Python to run inside a Go host with controlled access to host APIs.
- **Isolated execution** — run untrusted Python snippets with well-defined capability boundaries, impossible with a full CPython install.
- **Portable deployment** — ship a single Go binary that includes both the application and a Python execution engine.
- **Testing infrastructure** — drive Go services from Python test scripts without a separate CPython install in CI.

## The Key Distinction

```
Existing gopy / cffi approaches:

    Python script
        ↓
    CPython (required at runtime)
        ↓
    C bindings / extension modules
        ↓
    Go code


PyGo:

    Python script
        ↓
    PyGo lexer → parser → compiler → bytecode
        ↓
    PyGo virtual machine (pure Go)
        ↓
    PyGo object model (pure Go)
        ↓
    Go host application
```

CPython is never in the picture. PyGo is the runtime.

## Two First-Class Modes

### 1. Embedded Library Mode

A Go application imports PyGo as a library:

```go
import "github.com/alvinjaison/PyGo/pygo"

rt := pygo.New()
rt.RegisterFunction("greet", func(name string) string {
    return "Hello, " + name
})
err := rt.RunString(`print(greet("world"))`)
```

### 2. Standalone CLI Mode

A compiled binary runs Python scripts directly:

```bash
pygo script.py
pygo -c "print(10 + 20)"
pygo            # interactive REPL
```

The CLI is a thin wrapper around the same library. There is no separate interpreter for CLI mode.

## Goals

1. Execute Python 3 source code without CPython.
2. Embed cleanly into any Go application as a regular library dependency.
3. Allow Go applications to expose functions, types, and objects to Python scripts.
4. Provide a standalone CLI and REPL using the same runtime.
5. Measure Python 3 compatibility precisely rather than claiming full compatibility.
6. Grow compatibility incrementally through a well-defined roadmap.
7. Be a serious, maintainable open-source project, not a prototype.

## Non-Goals

1. **CPython ABI compatibility.** PyGo does not aim to load `.so` CPython extension modules.
2. **Full PyPI ecosystem support.** Packages with C extensions cannot run on PyGo without a pure-Python fallback.
3. **Complete CPython standard library.** A curated subset will be implemented in pure Go; the rest is out of scope initially.
4. **Drop-in CPython replacement.** PyGo is not a general-purpose replacement for every CPython use case.
5. **Python 2 support.** Python 3 only.
6. **Guaranteed CPython behavioral parity.** Edge cases in CPython may differ; documented divergences are acceptable.
7. **JIT compilation.** Not in scope for the initial runtime; the architecture should not preclude it later.
8. **Secure sandbox by default.** Security boundaries require deliberate host configuration; PyGo does not guarantee isolation out of the box.

## Design Principles

1. PyGo must not depend on CPython for runtime execution.
2. PyGo must be embeddable as a normal Go library — `go get` and `import`.
3. CLI mode must use the same runtime as embedded mode; no duplicate interpreter.
4. Public APIs must be small, stable, and idiomatic Go.
5. Python compatibility is measured, not claimed.
6. Go interoperability is a first-class feature, not an afterthought.
7. Avoid premature optimization; establish correctness first.
8. Do not attempt to implement the entire Python ecosystem at once.
9. Keep the core runtime independent of any application-specific logic.
10. Design for open-source contributors from day one.
11. Prefer standard Go mechanisms; avoid `unsafe` without strong justification.
12. Maintain clear boundaries between lexer, parser, compiler, VM, runtime, and host integration.

## Relationship to Existing Projects

| Project | Approach | Requires CPython? |
|---------|----------|-------------------|
| gopy | Generates CPython extension modules from Go | Yes |
| go-python3 | cgo bindings to libpython3 | Yes |
| gpython | Python 3 interpreter in Go (subset) | No |
| **PyGo** | Full Python 3 runtime in Go, embedding-first | **No** |

PyGo is most similar in spirit to gpython but targets a broader compatibility scope, a first-class embedding API, and a production-quality architecture from the start.

## Success Criteria

PyGo will be considered successful when:

- A Go application can embed PyGo with zero CPython dependency.
- A meaningful subset of Python 3 programs run correctly and verifiably.
- Go ↔ Python interoperability works for the common Go type surface.
- A standalone `pygo` binary can run simple Python scripts and provide a REPL.
- The compatibility surface is precisely documented and tested against CPython as a reference.
- Contributors can work on isolated components without needing to understand the entire runtime.
