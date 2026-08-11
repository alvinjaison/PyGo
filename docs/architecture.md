# PyGo — Architecture

## Table of Contents

1. [Execution Model Decision](#1-execution-model-decision)
2. [Pipeline Overview](#2-pipeline-overview)
3. [Component Breakdown](#3-component-breakdown)
4. [Component Dependency Graph](#4-component-dependency-graph)
5. [Execution Flow — Step by Step](#5-execution-flow--step-by-step)
6. [Runtime Lifecycle](#6-runtime-lifecycle)
7. [Package Layout](#7-package-layout)
8. [Cross-Cutting Concerns](#8-cross-cutting-concerns)
9. [Risks and Mitigations](#9-risks-and-mitigations)

---

## 1. Execution Model Decision

### Option A — Tree-Walking Interpreter

Parse Python source into an AST, then recursively evaluate AST nodes directly.

**Advantages**
- Simplest possible implementation.
- Very short path from "nothing" to "something works."
- Easy to debug: execution traces directly mirror the source structure.
- No compiler phase needed initially.

**Disadvantages**
- Poor performance ceiling. Every re-execution of a loop body re-walks the same tree.
- Difficult to optimize later — optimizations require rewriting the evaluator rather than transforming an intermediate representation.
- Exception handling, generators, and closures become increasingly awkward to implement at the AST level.
- Generators require either CPS transformation or goroutine-per-generator (both have serious costs).
- Almost always replaced by a bytecode VM in production interpreters.

### Option B — Bytecode + Virtual Machine

Compile the AST to a flat sequence of instructions (bytecode), then execute those instructions on a register- or stack-based virtual machine.

**Advantages**
- Clear separation between the compiler (correctness) and VM (performance).
- Generators, coroutines, closures, and exception tables are natural to represent in bytecode.
- VM dispatch loop is the single hot path — targeted for optimization without touching the compiler.
- Enables future serialization of compiled bytecode (`.pyc`-equivalent).
- Creates a well-defined intermediate representation that tools (debuggers, profilers, optimizers) can target.
- Consistent with how CPython, PyPy, and Lua are implemented.

**Disadvantages**
- More initial scaffolding than a tree-walker.
- Compiler correctness is a separate concern from VM correctness.
- Bytecode format must be versioned once stabilized.

### Recommendation: Bytecode + Stack-Based VM

PyGo uses a **bytecode compiler and stack-based virtual machine**, matching CPython's fundamental architecture.

The choice of stack-based (rather than register-based like Lua 5) is deliberate:
- CPython is stack-based, making future compatibility testing easier.
- Stack VMs are slightly simpler to implement correctly.
- Register VMs have better throughput at the cost of more complex instruction encoding — an optimization that belongs in Phase 8, not Phase 1.

The architecture is designed so the VM dispatch loop can be replaced or optimized (e.g., threaded dispatch, computed gotos via assembly, eventually JIT) without changing the compiler, object model, or runtime.

A tree-walking interpreter may be used as an **internal prototype** during early development to validate the parser and AST, but it will not be part of the public execution path.

---

## 2. Pipeline Overview

```
Python source (string or file)
         │
         ▼
    ┌─────────┐
    │  Lexer  │   internal/lexer
    └─────────┘
         │  []Token
         ▼
    ┌─────────┐
    │ Parser  │   internal/parser
    └─────────┘
         │  AST
         ▼
    ┌──────────┐
    │ Compiler │   internal/compiler
    └──────────┘
         │  *Code (bytecode + metadata)
         ▼
    ┌────────────────────┐
    │  Virtual Machine   │   internal/vm
    │  (execution frame) │
    └────────────────────┘
         │  PyObject results / exceptions
         ▼
    ┌───────────────────────────┐
    │  Python Object Model      │   internal/object
    │  Runtime support          │   internal/runtime
    │  Go ↔ Python Interop      │   internal/interop
    │  Module / Import System   │   internal/importer
    │  Standard Library         │   internal/stdlib
    └───────────────────────────┘
         │
         ▼
    Host Go application  /  pygo CLI
```

Each stage is independently testable. Inputs and outputs between stages are well-typed Go interfaces — no stage reaches into another stage's internals.

---

## 3. Component Breakdown

### 3.1 Lexer (`internal/lexer`)

**Responsibility:** Convert raw Python source text into a flat stream of tokens.

**Key design points:**
- Handles all Python 3 token types: keywords, identifiers, operators, literals, delimiters, newlines, indentation (INDENT / DEDENT), comments (discarded), string prefixes (`r`, `b`, `f`, `rb`).
- Indentation tracking produces synthetic `INDENT` and `DEDENT` tokens, eliminating whitespace complexity from the parser.
- f-string lexing is complex; initial implementation may treat f-strings as opaque string literals and handle interpolation in the compiler.
- Tracks source position (file, line, column) on every token for error reporting and traceback generation.
- Produces tokens as a stream (iterator-style) rather than materializing the entire token slice up front — allows the parser to drive consumption.

**Public surface to the parser:**
```go
type Lexer interface {
    Next() Token
    Peek() Token
}
```

**Does not:** parse, evaluate, or make semantic decisions.

---

### 3.2 Parser (`internal/parser`)

**Responsibility:** Consume the token stream and produce an Abstract Syntax Tree (AST).

**Key design points:**
- Recursive descent parser. This is the standard choice for hand-written parsers: readable, debuggable, good error recovery.
- Operator precedence handled with Pratt parsing (top-down operator precedence) for expressions. This cleanly handles Python's complex precedence table without explicit grammar tables.
- Produces a strongly-typed AST (concrete Go structs per node type), not a generic tree of `interface{}`.
- Error recovery: on a syntax error, the parser should attempt to continue and collect multiple errors rather than stopping at the first one. This improves the development experience.
- Source positions are attached to every AST node (for tracebacks and error messages).

**AST node categories:**
- Module, FunctionDef, AsyncFunctionDef, ClassDef
- Return, Delete, Assign, AugAssign, AnnAssign
- For, While, If, With, AsyncWith
- Raise, Try, ExceptHandler
- Import, ImportFrom
- Expressions: BoolOp, BinOp, UnaryOp, IfExp, Lambda, Dict, Set, ListComp, SetComp, DictComp, GeneratorExp
- Await, Yield, YieldFrom
- Compare, Call, Subscript, Starred, Name, List, Tuple, Constant
- Comprehensions: comprehension node

This closely mirrors the CPython `ast` module's node set, which simplifies compatibility testing and reference documentation.

---

### 3.3 AST (`internal/ast`)

**Responsibility:** Define all AST node types and visitor infrastructure.

This is a pure data-definition package. It does not parse or evaluate.

**Key types:**
```go
type Node interface {
    nodeType() string
    Pos() Position
}

type Stmt interface {
    Node
    stmtNode()
}

type Expr interface {
    Node
    exprNode()
}
```

A `Visitor` interface supports AST traversal for tooling (linters, formatters, future static analysis):
```go
type Visitor interface {
    Visit(node Node) Visitor
}

func Walk(v Visitor, node Node)
```

---

### 3.4 Compiler (`internal/compiler`)

**Responsibility:** Transform the AST into bytecode (`*Code` objects).

**Key design points:**
- Produces a `Code` object per function/class/module scope.
- Each `Code` object contains:
  - instruction array (`[]Instruction`)
  - constant pool
  - local variable names
  - free variable names (for closures)
  - cell variable names
  - source-line-number table (maps bytecode offset → source line)
  - argument count, flags
- Scoped symbol table analysis happens before bytecode generation:
  - Determines which names are local, enclosing (free), global, or built-in.
  - Assigns local variable indices.
  - Detects closure variables.
- Exception handling tables: `try/except/finally` blocks are compiled to exception table entries, not inline conditional branches.
- Comprehensions and generators compile to nested `Code` objects.

**Does not:** optimize. A separate optimization pass can be layered between the compiler and VM without changing either.

---

### 3.5 Bytecode (`internal/bytecode`)

**Responsibility:** Define the instruction set — opcodes, operands, encoding.

This is a pure definition package (no execution logic).

**Instruction format:**
```
Each instruction: opcode (uint8) + optional operand (uint32)
Variable-width encoding for compactness, fixed-width for simplicity of VM dispatch.
```

The initial design uses fixed-width 2-byte instructions (opcode + 1-byte operand), with extended operands for large constants (matching CPython's `EXTENDED_ARG` pattern). This keeps the instruction array indexable and avoids alignment issues.

**Opcode categories:**
- Load / Store: `LOAD_CONST`, `LOAD_NAME`, `LOAD_FAST`, `STORE_NAME`, `STORE_FAST`, `LOAD_GLOBAL`, `STORE_GLOBAL`
- Stack manipulation: `POP_TOP`, `DUP_TOP`, `ROT_TWO`, `ROT_THREE`
- Operators: `BINARY_ADD`, `BINARY_SUB`, `BINARY_MUL`, `BINARY_TRUE_DIV`, `BINARY_FLOOR_DIV`, `BINARY_MOD`, `BINARY_POW`, `BINARY_AND`, `BINARY_OR`, `BINARY_XOR`, `BINARY_LSHIFT`, `BINARY_RSHIFT`
- Unary: `UNARY_NEG`, `UNARY_NOT`, `UNARY_INVERT`, `UNARY_PLUS`
- Comparison: `COMPARE_OP` (with comparison kind as operand)
- Control flow: `JUMP_ABSOLUTE`, `JUMP_FORWARD`, `JUMP_IF_TRUE_OR_POP`, `JUMP_IF_FALSE_OR_POP`, `POP_JUMP_IF_TRUE`, `POP_JUMP_IF_FALSE`
- Function calls: `CALL_FUNCTION`, `CALL_FUNCTION_KW`, `CALL_FUNCTION_EX`, `MAKE_FUNCTION`, `RETURN_VALUE`
- Collections: `BUILD_LIST`, `BUILD_TUPLE`, `BUILD_SET`, `BUILD_MAP`, `BUILD_SLICE`
- Attributes / subscripts: `LOAD_ATTR`, `STORE_ATTR`, `DELETE_ATTR`, `BINARY_SUBSCR`, `STORE_SUBSCR`
- Iteration: `GET_ITER`, `FOR_ITER`
- Exceptions: `SETUP_FINALLY`, `POP_BLOCK`, `RAISE_VARARGS`, `RERAISE`
- Coroutines / generators: `YIELD_VALUE`, `YIELD_FROM`, `GET_AWAITABLE`, `GET_AITER`, `GET_ANEXT`
- Classes: `LOAD_BUILD_CLASS`, `BUILD_CLASS`
- Imports: `IMPORT_NAME`, `IMPORT_FROM`, `IMPORT_STAR`

The opcode set is versioned. Bytecode produced by one version of PyGo is not guaranteed to be executable by a different version — this is an internal implementation detail.

---

### 3.6 Virtual Machine (`internal/vm`)

**Responsibility:** Execute `Code` objects and manage the call stack.

**Key design points:**
- Stack-based execution. Each frame has a value stack, block stack (for exception handlers and loops), and local variable array.
- Frame structure:
  ```go
  type Frame struct {
      Code        *bytecode.Code
      Locals      []object.Object  // indexed by local variable slot
      Stack        []object.Object // value stack
      PC           int             // program counter
      BlockStack   []Block         // exception/loop blocks
      Back         *Frame          // caller frame (for tracebacks)
      Globals      object.Dict
      Builtins     object.Dict
      Closure      []*object.Cell  // free variables
  }
  ```
- Main dispatch loop: `for { switch opcode { case ...: } }`. Simple, debuggable, correct.
- Exception handling: when an exception is raised, the VM walks the block stack looking for an active exception handler. If none, the exception propagates up the Go call stack as a Go error value.
- Generator support: a generator's frame is suspended (PC saved, stack captured) and resumed on `next()` calls. Each generator owns its `Frame`. No goroutine is used per generator — generators are stackful coroutines implemented by suspending the frame.
- The VM does not do I/O, file access, or any host interaction directly — all of that goes through the runtime/stdlib layer.

**VM ↔ Go boundary:**
When a Python call resolves to a Go-registered function, the VM invokes the interop layer rather than entering a new bytecode frame.

---

### 3.7 Python Object Model (`internal/object`)

See `docs/object-model.md` for the full design. Summary:

- Every Python value is represented as a Go interface `object.Object`.
- Core types: `Int`, `Float`, `Str`, `Bool`, `NoneType`, `List`, `Tuple`, `Dict`, `Set`, `Function`, `BoundMethod`, `Type`, `Instance`, `Module`, `Exception`, `Iterator`, `Generator`, `GoObject`.
- Type objects implement the Python type protocol: `__getattribute__`, `__setattr__`, `__call__`, `__iter__`, `__next__`, etc., as Go method dispatch.
- The `Type` object itself is an `object.Object` — types are first-class Python values.

---

### 3.8 Runtime (`internal/runtime`)

**Responsibility:** The central coordinator that connects all components for a single execution context.

**Key design points:**
- Owns the global namespace, built-in namespace, and module registry for one `Runtime` instance.
- Manages the lifecycle of execution: `RunString`, `RunFile`, `RunCode`.
- Initializes built-in functions (`print`, `len`, `range`, `type`, etc.) and built-in types (`int`, `str`, `list`, etc.).
- Provides the `context.Context` hook for cancellation and timeout.
- Exposes the Go ↔ Python registration surface (used by the public `pygo` package).
- Multiple `Runtime` instances must be safe to use concurrently from different goroutines (each owning its own state). A single `Runtime` instance should be protected by the caller or documented as single-threaded.

---

### 3.9 Go ↔ Python Interoperability Layer (`internal/interop`)

See `docs/interop.md` for the full design. Summary:

- Converts Go values to `object.Object` and back.
- Wraps Go functions as callable `object.Object` values.
- Wraps Go structs/pointers as Python objects whose attributes map to Go fields and methods.
- Uses `reflect` (not `unsafe`) to inspect and invoke Go values dynamically.
- Defines explicit rules for which Go types are supported, which are rejected, and which require explicit conversion hints.

---

### 3.10 Standard Library (`internal/stdlib`)

**Responsibility:** Implement a curated subset of the Python standard library in pure Go.

Initial modules: `builtins`, `math`, `sys`, `os` (restricted), `json`, `re` (subset), `time`, `io`, `collections`, `itertools`, `functools`.

Each module is registered with the import system as a built-in module. Pure-Go modules can be supplemented by `.py` files shipping alongside PyGo for modules where a Go implementation is impractical.

---

### 3.11 Module / Import System (`internal/importer`)

**Responsibility:** Resolve and load Python modules.

Import resolution order:
1. `sys.modules` cache (already-imported modules).
2. Built-in Go modules (registered by the runtime).
3. `.py` files on `sys.path`.
4. Package directories (with `__init__.py`).

Circular import detection via in-progress module tracking.

No PyPI or package manager integration in the initial implementation.

---

### 3.12 Public Embedded API (`pygo/`)

See `docs/api.md` for the full design.

The `pygo` package is the **only** package consumers import. All `internal/` packages are private. This boundary is enforced by Go's `internal` package rules.

---

### 3.13 CLI (`cmd/pygo/`)

See `docs/cli.md` for the full design. The CLI is a thin `main` package that creates a `pygo.Runtime`, configures it, and calls `RunFile`, `RunString`, or `RunREPL`.

---

## 4. Component Dependency Graph

```
cmd/pygo
    └── pygo (public API)
            └── internal/runtime
                    ├── internal/vm
                    │       ├── internal/bytecode
                    │       └── internal/object
                    ├── internal/compiler
                    │       ├── internal/ast
                    │       └── internal/bytecode
                    ├── internal/parser
                    │       └── internal/ast
                    ├── internal/lexer
                    ├── internal/interop
                    │       └── internal/object
                    ├── internal/importer
                    │       └── internal/object
                    └── internal/stdlib
                            └── internal/object
```

**Rules:**
- No internal package imports `pygo` or `cmd/pygo`.
- `internal/vm` does not import `internal/compiler` or `internal/parser`.
- `internal/compiler` does not import `internal/vm`.
- `internal/object` has no dependencies on other internal packages — it is the foundation layer.
- `internal/lexer` has no dependencies on other internal packages.

These rules ensure that components can be developed, tested, and replaced independently.

---

## 5. Execution Flow — Step by Step

### Embedded mode: `rt.RunString(source)`

```
1.  pygo.Runtime.RunString(source)
         ↓
2.  internal/runtime: compile(source)
         ↓
3.  internal/lexer.New(source) → token stream
         ↓
4.  internal/parser.Parse(tokens) → ast.Module
         ↓
5.  internal/compiler.Compile(ast.Module) → *bytecode.Code
         ↓
6.  internal/runtime: execute(code)
         ↓
7.  internal/vm.NewFrame(code, globals, builtins)
         ↓
8.  internal/vm.Run(frame) → dispatch loop
         ↓
9.  On LOAD_NAME "print":
        → look up in locals → globals → builtins
        → found: built-in print function (object.Builtin)
         ↓
10. On CALL_FUNCTION:
        → pop args from stack
        → if callee is object.Builtin → invoke Go function
        → if callee is object.Function → create new Frame, recurse
        → if callee is interop.GoFunction → invoke interop layer
         ↓
11. Return value pushed onto stack (or None)
         ↓
12. Frame exits → runtime returns to caller
         ↓
13. pygo.Runtime.RunString returns nil error (or *pygo.Error)
```

### Exception propagation:

```
Python: raise ValueError("bad input")
         ↓
VM: RAISE_VARARGS opcode
         ↓
vm.raise(exc) → walk block stack for active except handler
         ↓
If handler found: push exception onto stack, jump to handler PC
         ↓
If no handler in current frame: propagate to caller frame
         ↓
If no handler anywhere: wrap as *pygo.Error, return to Go caller
```

---

## 6. Runtime Lifecycle

```
pygo.New()
    │
    ├── create Runtime struct
    ├── initialize object model (type objects, built-in types)
    ├── initialize built-in namespace (print, len, range, type, …)
    ├── initialize sys module
    ├── initialize module registry
    └── return *Runtime

runtime.RegisterFunction(name, fn)
    └── interop: wrap fn as GoFunction object
        └── store in runtime globals

runtime.RunString(source)
    ├── lexer → parser → compiler → *Code
    ├── create top-level Frame
    ├── vm.Run(frame)
    └── return (Value, error)

runtime.Close()  [optional, future]
    └── release any resources, finalize modules
```

---

## 7. Package Layout

```
github.com/pygo/pygo
├── go.mod
├── go.sum
│
├── pygo/                          ← public API (consumers import this)
│   ├── pygo.go                    ← Runtime type, New(), options
│   ├── error.go                   ← PyError type (Python exceptions as Go errors)
│   ├── value.go                   ← Value interface (Go-visible Python values)
│   └── options.go                 ← RuntimeOption functional options
│
├── internal/
│   ├── lexer/
│   │   ├── lexer.go
│   │   ├── token.go
│   │   └── lexer_test.go
│   │
│   ├── parser/
│   │   ├── parser.go
│   │   └── parser_test.go
│   │
│   ├── ast/
│   │   ├── ast.go                 ← all node type definitions
│   │   ├── walk.go                ← Walk / Visitor
│   │   └── print.go               ← debug printer
│   │
│   ├── compiler/
│   │   ├── compiler.go
│   │   ├── symtable.go            ← symbol table / scope analysis
│   │   └── compiler_test.go
│   │
│   ├── bytecode/
│   │   ├── opcode.go              ← opcode constants
│   │   ├── code.go                ← Code object definition
│   │   └── disasm.go              ← disassembler (for debugging)
│   │
│   ├── vm/
│   │   ├── vm.go                  ← main dispatch loop
│   │   ├── frame.go               ← Frame struct
│   │   ├── block.go               ← Block (exception/loop) stack
│   │   └── vm_test.go
│   │
│   ├── object/
│   │   ├── object.go              ← Object interface
│   │   ├── types.go               ← type constants / type registry
│   │   ├── int.go
│   │   ├── float.go
│   │   ├── str.go
│   │   ├── bool.go
│   │   ├── none.go
│   │   ├── list.go
│   │   ├── tuple.go
│   │   ├── dict.go
│   │   ├── set.go
│   │   ├── function.go
│   │   ├── class.go
│   │   ├── instance.go
│   │   ├── module.go
│   │   ├── exception.go
│   │   ├── iterator.go
│   │   ├── generator.go
│   │   ├── goobject.go            ← wrapper for Go values
│   │   └── ops.go                 ← arithmetic / comparison dispatch
│   │
│   ├── runtime/
│   │   ├── runtime.go             ← Runtime internals
│   │   ├── builtins.go            ← built-in functions
│   │   ├── eval.go                ← RunString / RunFile / RunCode
│   │   └── runtime_test.go
│   │
│   ├── interop/
│   │   ├── interop.go             ← Go→Python conversion
│   │   ├── gofunc.go              ← Go function wrapping
│   │   ├── goobject.go            ← Go struct wrapping
│   │   ├── convert.go             ← type conversion rules
│   │   └── interop_test.go
│   │
│   ├── importer/
│   │   ├── importer.go            ← import resolution
│   │   ├── loader.go              ← file loader
│   │   └── importer_test.go
│   │
│   └── stdlib/
│       ├── stdlib.go              ← module registry
│       ├── builtins/
│       ├── math/
│       ├── sys/
│       ├── os/
│       ├── json/
│       ├── re/
│       ├── time/
│       ├── io/
│       ├── collections/
│       ├── itertools/
│       └── functools/
│
├── cmd/
│   └── pygo/
│       ├── main.go
│       ├── run.go                 ← script execution
│       ├── repl.go                ← REPL
│       └── flags.go
│
├── examples/
│   ├── embedding/
│   │   └── main.go
│   ├── functions/
│   │   └── main.go
│   └── objects/
│       └── main.go
│
├── testdata/
│   ├── lexer/
│   ├── parser/
│   ├── compiler/
│   ├── vm/
│   └── compat/                    ← Python scripts tested against CPython
│
└── docs/
    ├── architecture.md            ← this document
    ├── object-model.md
    ├── interop.md
    ├── api.md
    ├── compatibility.md
    ├── testing.md
    ├── cli.md
    ├── security.md
    ├── roadmap.md
    └── contributing.md
```

---

## 8. Cross-Cutting Concerns

### Error Reporting

All errors carry source position (file, line, column). Python-level exceptions produce a `*pygo.Error` at the Go boundary containing:
- exception type name
- message
- traceback (list of frames: file, function, line)

### Cancellation

The VM checks `context.Context.Done()` at loop backedges and function calls. A cancelled context causes the VM to raise a `KeyboardInterrupt`-equivalent Python exception that propagates normally through Python exception handling.

### Logging / Diagnostics

Internal packages do not log. The public API provides an optional `DiagnosticsWriter` for debug output (e.g., bytecode disassembly, parse trees).

### Concurrency Model

- Each `Runtime` instance owns its own state and is safe to use from a single goroutine.
- Multiple `Runtime` instances may be created and used concurrently (each is independent).
- Shared Go objects exposed to multiple runtimes require external synchronization — PyGo does not impose a GIL.
- Python-level threading (`threading` module) is not in scope for MVP.

### Memory Management

PyGo relies entirely on the Go garbage collector. Python `object.Object` values are Go heap objects; the Go GC handles their collection. Reference cycles between Python objects are handled by the Go GC's cycle detection. No reference counting is implemented. This is the correct initial choice — custom GC is a major undertaking with minimal benefit until performance profiling justifies it.

---

## 9. Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| Python's operator semantics are more complex than anticipated | High | Medium | Implement incrementally; use CPython as reference; maintain failing test cases |
| Closure and free-variable handling is subtle | High | High | Invest in compiler symbol table tests before VM tests |
| Generator suspension without goroutines is complex | Medium | High | Prototype early; generator frame state must be explicitly captured |
| f-string lexing edge cases | Medium | Low | Treat as opaque initially; add full f-string support in Phase 2 |
| Reflection-based interop is slow | Low (MVP) | Medium | Acceptable for MVP; add code-gen or type caching in Phase 8 |
| Go GC pauses visible in latency-sensitive embeddings | Low (MVP) | Low | Document; mitigate with object pooling in Phase 8 |
| Bytecode format instability | Medium | Low | Mark bytecode as internal; do not expose serialization API until stable |
| Metaclass semantics are very complex | Low (MVP) | High | Defer to Phase 3+; document as not supported |
| `async/await` requires significant VM changes | Low (MVP) | High | Design frame suspension model to support it; defer implementation |
