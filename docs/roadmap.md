# PyGo — Phased Implementation Roadmap

## Effort Estimates

Two effort scales are used throughout:

- **Solo** — a single experienced Go developer working part-time (~10–15 hours/week).
- **Community** — a small active open-source team (3–6 contributors with overlapping availability).

These are realistic estimates, not optimistic ones. Python semantics contain many subtleties that become apparent only during implementation. The estimates account for rework.

---

## Phase 0 — Architecture and Foundation

**Goal:** No major implementation code. Establish everything needed to begin Phase 1 with confidence.

### Deliverables

- [x] Project vision and goals (`VISION.md`)
- [x] Architecture document (`docs/architecture.md`)
- [x] Python object model proposal (`docs/object-model.md`)
- [x] Go ↔ Python interoperability design (`docs/interop.md`)
- [x] Public API proposal (`docs/api.md`)
- [x] Python compatibility matrix (`docs/compatibility.md`)
- [x] Testing strategy (`docs/testing.md`)
- [x] CLI design (`docs/cli.md`)
- [x] Security considerations (`docs/security.md`)
- [x] Phased roadmap (`docs/roadmap.md`)
- [x] Contribution model (`docs/contributing.md`)
- [x] Go module scaffold (`go.mod`, directory structure)
- [x] Skeleton Go source files with package declarations

### What is explicitly NOT done in Phase 0

- No lexer implementation.
- No parser implementation.
- No VM implementation.
- No bytecode format locked in.

### Effort

Solo: **1–2 weeks** (this document represents completion of Phase 0).
Community: same — architecture is a single-voice activity.

---

## Phase 1 — Minimal Interpreter

**Goal:** Execute the two canonical programs:

```python
print("Hello, world!")
```

```python
x = 10
y = 20
print(x + y)
```

### Scope

**Lexer:**
- All keyword tokens
- Integer and float literals
- String literals (single/double-quoted, basic)
- Identifiers
- Operators: `+`, `-`, `*`, `/`, `=`
- Delimiters: `(`, `)`, `:`, `,`
- INDENT / DEDENT (even if not needed for Phase 1 programs)
- NEWLINE, EOF

**Parser:**
- Assignment statements
- Expression statements
- Function calls (simple positional only)
- Binary expressions: `+`, `-`, `*`, `/`
- Integer and string literals
- Name expressions

**Compiler:**
- `LOAD_CONST`, `LOAD_NAME`, `STORE_NAME`
- `BINARY_ADD`, `BINARY_SUB`, `BINARY_MUL`, `BINARY_TRUE_DIV`
- `CALL_FUNCTION` (positional args only)
- `POP_TOP`, `RETURN_VALUE`

**Object model:**
- `IntObject` (arbitrary precision)
- `FloatObject`
- `StrObject`
- `NoneObject` (singleton)
- `BuiltinFuncObject`

**Runtime:**
- `print()` built-in
- Global namespace
- `RunString()`

**Tests:**
- Lexer: integer literals, string literals, identifiers, arithmetic operators
- Parser: assignments, function calls, binary ops
- Compiler: bytecode snapshot for the two target programs
- VM: end-to-end execution of both target programs
- Output capture to verify `print` works correctly

### What is NOT in Phase 1

- `if`, `for`, `while`
- Functions (`def`)
- Lists, dicts, tuples
- Exceptions
- Modules / imports
- Closures
- Classes

### Effort

Solo: **3–5 weeks** (lexer + parser + compiler + VM + tests is substantial even for a small subset)
Community: **2–3 weeks** (lexer and parser can be developed in parallel)

### Success criterion

`go test ./...` passes. `pygo -c 'print(10 + 20)'` outputs `30`.

---

## Phase 2 — Core Language

**Goal:** A usable subset of Python 3 covering the features needed for non-trivial scripts.

### Scope

**Lexer additions:**
- All remaining operators and delimiters
- `True`, `False`, `None` keywords
- All keywords (`if`, `else`, `elif`, `for`, `while`, `def`, `return`, `break`, `continue`, `pass`, `import`, `from`, `global`, `nonlocal`, `del`, `raise`, `try`, `except`, `finally`, `with`, `class`, `yield`, `lambda`, `and`, `or`, `not`, `in`, `is`)
- Triple-quoted strings
- Raw strings (`r"..."`)
- f-strings (basic interpolation: `f"hello {name}"`)
- Hex, octal, binary integer literals
- Underscore in numeric literals

**Parser additions:**
- `if / elif / else`
- `while / else`
- `for / else`
- `break`, `continue`, `pass`
- `return` (with and without value)
- `del`
- All comparison operators (chained comparisons)
- Boolean operators (`and`, `or`, `not`)
- Unary operators
- `def` (function definition, positional args, defaults)
- `*args`, `**kwargs`
- `global`, `nonlocal`
- `raise`, `try/except/finally`
- `import`, `from ... import`
- Collection literals: `[...]`, `(...)`, `{...}` (dict), `{val, ...}` (set)
- Subscript: `a[i]`
- Attribute access: `a.b`
- Augmented assignment: `+=`, `-=`, etc.
- Ternary expression
- Lambda
- `assert`

**Compiler additions:**
- Complete opcode set for all the above constructs
- Symbol table (local/global/nonlocal/free variable analysis)
- Closure compilation (`LOAD_DEREF`, `STORE_DEREF`, `MAKE_CELL`)
- Exception table compilation
- `MAKE_FUNCTION` with defaults and closure

**Object model additions:**
- `BoolObject` (True, False singletons)
- `ListObject` with methods
- `TupleObject`
- `DictObject` (insertion-ordered)
- `SetObject`
- `FunctionObject`
- `BoundMethodObject`
- `CellObject`
- `ExceptionObject` + full built-in exception hierarchy
- `TracebackObject`
- Iterator types for list, tuple, dict, set, range

**Built-ins additions:**
- `len()`, `range()`, `type()`
- `int()`, `float()`, `str()`, `bool()` (type constructors)
- `abs()`, `round()`, `pow()`, `divmod()`
- `min()`, `max()`, `sum()`
- `repr()`
- `isinstance()` (basic — before full class support)
- `list()`, `tuple()`, `dict()`, `set()` (from iterables)
- `enumerate()`, `zip()`, `map()`, `filter()`
- `sorted()`, `reversed()`
- `any()`, `all()`
- `chr()`, `ord()`
- `hex()`, `bin()`, `oct()`

**String methods:** `.upper()`, `.lower()`, `.strip()`, `.split()`, `.join()`, `.startswith()`, `.endswith()`, `.replace()`, `.find()`, `.format()` (basic), `.encode()` (basic)

**VM additions:**
- All new opcodes
- Exception handling (block stack, exception table lookup)
- Generator support (`YIELD_VALUE`, suspended frames)

**Module system (basic):**
- `sys` module (subset: `argv`, `version`, `path`, `exit`)
- `math` module
- `json` module (basic)
- File-based module loading (`.py` files from `sys.path`)

**Public API:**
- `RunString`, `RunFile`, `Eval` working end-to-end
- `Set`, `Get`, `Delete`
- `*pygo.Error` with traceback

**CLI:**
- `pygo script.py`
- `pygo -c "source"`
- Error output with traceback
- Correct exit codes

### Effort

Solo: **3–5 months** (this is the largest phase — full language core)
Community: **6–10 weeks** (parser, compiler, VM, object model, stdlib can be worked in parallel)

### Success criterion

The following programs run correctly and match CPython output:

```python
# fibonacci
def fib(n):
    if n <= 1:
        return n
    return fib(n-1) + fib(n-2)

for i in range(10):
    print(fib(i))
```

```python
# closures
def make_counter(start=0):
    count = start
    def inc():
        nonlocal count
        count += 1
        return count
    return inc

c = make_counter(10)
print(c(), c(), c())  # 11 12 13
```

```python
# exceptions
try:
    x = int("bad")
except ValueError as e:
    print(f"Caught: {e}")
finally:
    print("done")
```

---

## Phase 3 — Object Model (Classes)

**Goal:** Python classes, instances, inheritance, operator overloading, iterators.

### Scope

- Class definition (`class Foo:`, `class Foo(Bar):`)
- `__init__`, `__str__`, `__repr__`
- Instance attribute access and mutation
- Single and multiple inheritance
- C3 MRO linearization
- `super()` (zero-argument form: Phase 3; `super(T, self)` form: Phase 3)
- Method resolution through MRO
- Operator overloading (`__add__`, `__eq__`, `__lt__`, etc.)
- `__hash__`
- `__bool__`, `__len__`
- `__getitem__`, `__setitem__`, `__delitem__` (container protocol)
- `__iter__`, `__next__` (iteration protocol)
- `__contains__` (`in` operator)
- `__call__` (callable instances)
- Class attributes vs instance attributes
- `@classmethod`, `@staticmethod`
- `property` (basic `@property` decorator — getter only)
- `property` getter + setter
- Descriptor protocol (`__get__`, `__set__`, `__delete__`) — basic
- Exception subclassing
- `dataclasses` (pure Python implementation, Phase 3b)

### Effort

Solo: **2–3 months**
Community: **4–6 weeks** (MRO, descriptors, and operator dispatch can be parallelized)

### Success criterion

```python
class Vector:
    def __init__(self, x, y):
        self.x = x
        self.y = y
    def __add__(self, other):
        return Vector(self.x + other.x, self.y + other.y)
    def __repr__(self):
        return f"Vector({self.x}, {self.y})"
    def __iter__(self):
        yield self.x
        yield self.y

v1 = Vector(1, 2)
v2 = Vector(3, 4)
print(v1 + v2)           # Vector(4, 6)
print(list(v1 + v2))     # [4, 6]
```

---

## Phase 4 — Go Interoperability

**Goal:** Full Go ↔ Python bridge for the common Go type surface.

### Scope

- `RegisterFunction` for all supported Go function signatures
- `Register` for Go structs and pointers
- `RegisterModule`
- `Call`, `CallByName`, `CallMethod`
- Full type conversion table (Go → Python, Python → Go)
- Error handling: Go `error` → Python exception
- Go error with `PyError` interface → typed Python exception
- Python exception → `*pygo.Error`
- Callbacks (Go function that calls Python callable)
- Variadic Go functions
- `interface{}` parameter and return handling
- Reflection type cache
- Panic recovery at all entry points
- `MustRegister`, `MustRegisterFunction`

### Not in Phase 4

- Code generation (Phase 8)
- `chan T` support
- `unsafe.Pointer`

### Effort

Solo: **4–6 weeks**
Community: **3–4 weeks**

### Success criterion

The embedding examples in `docs/api.md` all work correctly, including struct field mutation, method calls, error propagation, and callbacks.

---

## Phase 5 — Modules and Standard Library

**Goal:** A usable `import` system and a practical standard library subset.

### Scope

**Import system:**
- `sys.modules` cache
- Built-in module registry
- File-based module loading (`sys.path` search)
- Package directories (`__init__.py`)
- `import as`, `from ... import as`
- Circular import detection and error
- `__name__`, `__file__`, `__package__`, `__spec__`

**Standard library modules:**
- `sys` (complete subset: argv, path, modules, version, exit, stdin/stdout/stderr)
- `math` (all standard math functions)
- `os` (path operations: join, exists, isfile, isdir, getcwd, listdir; controlled)
- `os.path`
- `json` (dumps, loads, JSONDecodeError)
- `re` (basic patterns: match, search, findall, sub — note: Python regex vs Go regexp syntax differences)
- `time` (time, sleep, monotonic)
- `io` (StringIO, BytesIO)
- `collections` (deque, OrderedDict, defaultdict, Counter, namedtuple)
- `itertools` (chain, islice, product, permutations, combinations, repeat, cycle, count)
- `functools` (partial, reduce, wraps, lru_cache)
- `copy` (copy, deepcopy — shallow implementation acceptable initially)

**Pure-Python fallbacks:**
Some modules can ship as `.py` files that run on PyGo itself (bootstrapping). `functools.wraps` and `contextlib` are good candidates.

### Effort

Solo: **2–3 months** (module system + 12+ stdlib modules is substantial)
Community: **5–7 weeks** (each module is an independent work item)

### Success criterion

```python
import json
import math
import re
import os

data = json.loads('{"values": [1, 2, 3]}')
total = sum(data["values"])
print(f"Sum: {total}, sqrt: {math.sqrt(total):.2f}")

pattern = re.compile(r"\d+")
matches = pattern.findall("abc 123 def 456")
print(matches)  # ['123', '456']

print(os.path.join("/tmp", "file.txt"))
```

---

## Phase 6 — CLI and REPL

**Goal:** Polished standalone `pygo` binary with full REPL experience.

### Scope

- `pygo script.py` (using Phase 1–5 runtime)
- `pygo -c "source"`
- `pygo` interactive REPL
- Incomplete input detection (multi-line blocks)
- REPL expression result display (`repr()`)
- REPL error display (traceback, continue session)
- Line history (`~/.pygo_history`)
- `pygo --version`
- `pygo --help`
- `sys.argv` population
- Correct exit codes
- Traceback formatting matching CPython style
- `pygo disasm script.py` (diagnostic subcommand)
- `pygo ast script.py` (diagnostic subcommand)

### Effort

Solo: **3–4 weeks** (most runtime work already done; this is wiring)
Community: **2 weeks**

### Success criterion

The REPL session in `docs/cli.md §5` works correctly, including multi-line input, error recovery, and expression display.

---

## Phase 7 — Compatibility Expansion

**Goal:** Systematic expansion of Python 3 compatibility. Drive down the `# pygo-skip` count in `testdata/compat/`.

### Scope (ordered by impact)

- List/dict/set comprehensions
- Generator expressions
- `yield from`
- `send()` / `throw()` / `close()` on generators
- Context managers (`with` statement, `__enter__` / `__exit__`)
- `contextlib.contextmanager`
- Walrus operator (`:=`)
- Decorators with arguments
- `@classmethod`, `@staticmethod` (if not done in Phase 3)
- f-string full support (nested expressions, format spec, `!r`/`!s`/`!a`)
- `bytes` / `bytearray`
- `complex` numbers
- Extended slicing
- `str.format()` full spec
- `%`-formatting strings
- `__slots__`
- `__init_subclass__`
- `abc.ABC` and abstract methods
- Relative imports
- `__all__` for `from module import *`
- `match` / `case` (Python 3.10 structural pattern matching)
- Additional built-ins: `vars()`, `dir()`, `globals()`, `locals()`
- `eval()` / `exec()` (restricted mode blocks these)
- Positional-only parameters (`/`)
- Exception groups (Python 3.11+)

This phase is ongoing and parallel to Phase 8. New features are added based on community interest and real-world usage gaps.

### Effort

Solo: ongoing, **6–12 months** for meaningful coverage
Community: continuous — each feature is an independent contribution

---

## Phase 8 — Performance and Production Hardening

**Goal:** Make PyGo production-ready for performance-sensitive embedding use cases.

### Scope

**Performance:**
- Instruction count limit (cooperating cancellation at backedges)
- VM dispatch optimization (switch → computed dispatch, or direct threading if viable in Go)
- Small-integer cache validation and tuning
- String interning expansion
- Value stack pre-allocation / pooling
- Dict implementation optimization (flat compact dict like CPython 3.6+)
- Reflection type cache (already designed in Phase 4, validate effectiveness)
- Interop code generation tool (`pygo-gen`)
- Benchmark suite with regression tracking

**Correctness hardening:**
- Memory limit (best-effort, using `runtime.ReadMemStats`)
- Hard cancellation flag in VM (bypasses Python exception handling)
- Per-field access control on registered Go structs
- Module allowlist in restricted mode
- Comprehensive error message quality pass
- `weakref` module (Phase 8b)

**Concurrency:**
- Thread-safety audit of the runtime
- Explicit documentation of concurrent Runtime usage patterns
- Consider a thread-safe `Runtime` wrapper with a mutex for single-runtime multi-goroutine usage

**Tooling:**
- `pygo fmt` (formatter, very long-term)
- `pygo check` (syntax check only)
- Bytecode serialization (`.pyc`-equivalent for caching compiled scripts)
- Profiling support (instruction-level profiling hooks)

**Async/Await (Phase 8c — major):**
- Coroutine object type
- `async def`, `await` expression
- `async for`, `async with`
- Minimal event loop compatible with `asyncio` API
- Bridging Python coroutines to Go goroutines/channels

### Effort

Solo: **ongoing** — performance work is never truly done
Community: **ongoing** — each item is an independent contribution

---

## Milestone Summary

| Phase | Name | Key Deliverable | Solo Estimate | Community Estimate |
|-------|------|-----------------|--------------|-------------------|
| 0 | Architecture | This document set | 1–2 weeks | 1–2 weeks |
| 1 | Minimal Interpreter | `print("Hello")` works | 3–5 weeks | 2–3 weeks |
| 2 | Core Language | Functions, loops, exceptions, basic types | 3–5 months | 6–10 weeks |
| 3 | Object Model | Classes, inheritance, protocols | 2–3 months | 4–6 weeks |
| 4 | Go Interoperability | Full Go ↔ Python bridge | 4–6 weeks | 3–4 weeks |
| 5 | Modules + Stdlib | Import system + 12 modules | 2–3 months | 5–7 weeks |
| 6 | CLI + REPL | Polished standalone binary | 3–4 weeks | 2 weeks |
| 7 | Compatibility | Comprehensions, generators, context managers, etc. | 6–12 months | ongoing |
| 8 | Performance + Hardening | Production-ready | ongoing | ongoing |

**Realistic solo time to a useful Phase 2 PyGo:** 4–7 months of part-time work.
**Realistic community time to a useful Phase 2 PyGo:** 2–4 months with 4–6 active contributors.

---

## MVP Definition

The Minimum Viable Product is **Phase 2 complete**.

At MVP, PyGo can:
- Execute real Python 3 programs covering variables, arithmetic, strings, booleans, None, lists, tuples, dicts, sets, functions with closures, control flow, exceptions, basic iteration, and f-strings.
- Run as a standalone `pygo script.py` binary.
- Be embedded in a Go application with `RunString` and `Eval`.
- Report Python exceptions to the Go caller as structured `*pygo.Error` values.
- Load `.py` module files.
- Use `math`, `sys`, and `json` modules.

At MVP, PyGo cannot:
- Execute class-based Python programs.
- Use the full Go interop API (RegisterFunction / Register) — that is Phase 4.
- Use most standard library modules.
- Run a REPL (that is Phase 6, though a basic REPL may arrive earlier as a development convenience).

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Python semantics are subtler than anticipated (floor division, modulo sign, late-binding closures) | High | Use CPython as reference for every operation; compat tests catch drift |
| Phase 2 scope is too large for a solo developer to maintain momentum | High | Ship Phase 1 publicly to attract contributors before Phase 2 is complete |
| Generator suspension without goroutines is harder than expected | Medium | Prototype generator frame suspension in Phase 1 as a spike; don't leave it to Phase 2 |
| C3 MRO implementation bugs cause subtle class hierarchy failures | Medium | Implement from the specification with dedicated test cases against CPython |
| Reflection-based interop performance bottleneck discovered late | Low (MVP) | Benchmark in Phase 4; defer optimization to Phase 8 |
| `re` module: Python and Go use different regex flavors (lookahead, backreferences) | Medium | Document differences clearly; implement a compatibility shim for common patterns |
| Community interest does not materialize | Medium | Strong Phase 0–1 documentation is the best recruiting tool; make contribution easy |
| Bytecode format changes between phases break serialized cache | Low | Do not expose bytecode serialization API until Phase 8; keep it internal |
