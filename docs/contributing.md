# PyGo — Contribution Model

## Table of Contents

1. [Welcome](#1-welcome)
2. [Contribution Areas](#2-contribution-areas)
3. [Dependency Map](#3-dependency-map)
4. [Good First Issues](#4-good-first-issues)
5. [Contribution Process](#5-contribution-process)
6. [Code Standards](#6-code-standards)
7. [Testing Requirements](#7-testing-requirements)
8. [Documentation Requirements](#8-documentation-requirements)
9. [Communication](#9-communication)
10. [Decision-Making](#10-decision-making)
11. [Getting Started Checklist](#11-getting-started-checklist)

---

## 1. Welcome

PyGo is designed to be worked on by contributors who each understand one part of the system well — not contributors who must understand the entire runtime to make a contribution.

The architecture is deliberately layered so that work on the lexer requires no knowledge of the VM, work on the standard library requires no knowledge of the compiler, and work on the CLI requires no knowledge of the object model.

If you are new to Python interpreter internals, that is fine. This document will tell you which area has the lowest prerequisite knowledge and how to start.

---

## 2. Contribution Areas

### A. Lexer (`internal/lexer`)

**What you work on:** Tokenizing Python source text.

**Prerequisites:** Familiarity with Go. No interpreter knowledge required. Helpful to read the Python 3 language reference on lexical analysis (§2).

**Independent from:** Parser, compiler, VM, object model, stdlib, interop.

**Representative tasks:**
- Implement a token type (e.g., `WALRUS_OP`)
- Fix edge case in string literal lexing (escape sequences, prefixes)
- Improve error messages for unterminated strings
- Improve INDENT/DEDENT handling
- Add fuzz test corpus entries
- Performance: reduce allocations in the hot path

---

### B. Parser (`internal/parser`)

**What you work on:** Converting a token stream into an AST.

**Prerequisites:** Familiarity with Go. Helpful to understand recursive descent parsing and Pratt parsing for expressions. No VM knowledge required.

**Independent from:** Compiler, VM, object model, stdlib, interop.

**Depends on:** Lexer (uses `Token` types from `internal/lexer`), AST node types from `internal/ast`.

**Representative tasks:**
- Add parsing for a new statement (e.g., `with`, `match`)
- Add parsing for a new expression type (e.g., walrus `:=`)
- Improve error recovery and error messages
- Add parser golden file tests
- Fix operator precedence edge cases

---

### C. AST (`internal/ast`)

**What you work on:** AST node type definitions and traversal utilities.

**Prerequisites:** Go. No runtime knowledge required.

**Independent from:** All other components.

**Representative tasks:**
- Add a new AST node type
- Implement AST printer improvements
- Add AST visitor for a static analysis pass

---

### D. Compiler (`internal/compiler`)

**What you work on:** Transforming the AST into bytecode. The symbol table analysis lives here too.

**Prerequisites:** Go. Familiarity with how stack-based VMs work (read `docs/architecture.md §3.4–3.5`). Helpful to understand how CPython compiles Python.

**Depends on:** AST node types, bytecode definitions.
**Independent from:** VM, object model, stdlib, interop.

**Representative tasks:**
- Compile a new statement type to bytecode
- Fix a jump offset calculation bug
- Implement or fix closure variable detection in the symbol table
- Add compiler snapshot tests for a new construct
- Improve constant folding (e.g., `1 + 2` → `3` at compile time)

---

### E. Bytecode (`internal/bytecode`)

**What you work on:** Opcode definitions and the disassembler.

**Prerequisites:** Go. No runtime knowledge required.

**Independent from:** All other components (pure definitions).

**Representative tasks:**
- Add a new opcode
- Improve the disassembler output format
- Add opcode documentation comments

---

### F. Virtual Machine (`internal/vm`)

**What you work on:** The bytecode execution engine.

**Prerequisites:** Go. Understanding of stack-based VM execution. Read `docs/architecture.md §3.6` and `docs/object-model.md`.

**Depends on:** Bytecode definitions, object model.
**Independent from:** Lexer, parser, compiler, stdlib (the VM calls through interfaces, not directly into stdlib), interop.

**Representative tasks:**
- Implement a new opcode
- Fix exception propagation through nested try/except blocks
- Implement generator frame suspension and resumption
- Fix an edge case in the block stack (e.g., `break` inside `try/finally`)
- Add VM-level instruction counting for cancellation
- Add VM tests for specific opcode interactions

---

### G. Object Model (`internal/object`)

**What you work on:** Python value types (int, str, list, dict, function, class, etc.) and the type protocol.

**Prerequisites:** Go. Read `docs/object-model.md` thoroughly. Understanding of Python's data model is very helpful.

**Independent from:** Lexer, parser, compiler, bytecode, VM (the VM calls into the object model through interfaces).

**Representative tasks:**
- Implement a new object type (e.g., `bytes`, `complex`, `frozenset`)
- Implement missing methods on an existing type (e.g., `str.format()`, `list.sort()`)
- Fix hash behavior (ensure `hash(1) == hash(1.0)`)
- Implement a protocol method (`__contains__`, `__reversed__`)
- Improve exception type hierarchy
- Add type-specific unit tests

---

### H. Runtime (`internal/runtime`)

**What you work on:** The coordinator that ties lexer → parser → compiler → VM together, initializes built-ins, and manages runtime state.

**Prerequisites:** Go. Understanding of the full pipeline. This is the most central package — changes here can affect everything.

**Depends on:** All internal packages.
**Independent from:** Public `pygo` package, CLI.

**Representative tasks:**
- Add a new built-in function
- Fix initialization order of built-in types
- Add context cancellation support
- Fix `sys.path` handling
- Implement the `exec()` built-in
- Add runtime-level tests

---

### I. Standard Library (`internal/stdlib`)

**What you work on:** Go implementations of Python standard library modules.

**Prerequisites:** Go. Understanding of the Python module you are implementing. Each module is independent.

**Independent from:** Lexer, parser, compiler, VM, interop.
**Depends on:** Object model (to create and return Python values).

**Representative tasks:**
- Implement a function in `math` (e.g., `math.gcd`)
- Add missing `json` features (e.g., `indent` parameter in `dumps`)
- Implement a `collections` type (e.g., `Counter`)
- Implement `itertools.chain`
- Implement `functools.partial`
- Add stdlib module tests

This is the most accessible area for contributors new to the project because:
- Each function is small and self-contained.
- There is a clear reference (the CPython docs).
- Tests are straightforward (input → expected output).
- No knowledge of the runtime internals is required.

---

### J. Go Interoperability (`internal/interop`)

**What you work on:** The Go ↔ Python type conversion and function/struct wrapping layer.

**Prerequisites:** Go reflection (`reflect` package). Read `docs/interop.md`.

**Independent from:** Lexer, parser, compiler, bytecode, VM.
**Depends on:** Object model.

**Representative tasks:**
- Add support for a new Go type conversion
- Fix a type conversion edge case (e.g., `uint64` range check)
- Add the reflection type cache
- Improve error messages for unsupported types
- Add interop tests for new type combinations

---

### K. Import System (`internal/importer`)

**What you work on:** Module resolution, loading, and caching.

**Prerequisites:** Go. Understanding of Python's import system.

**Independent from:** Lexer, parser, compiler, VM, object model.
**Depends on:** Runtime (to compile and execute module code).

**Representative tasks:**
- Implement `__init__.py` package loading
- Implement relative imports
- Fix circular import detection
- Add module cache invalidation
- Add importer tests

---

### L. Public API (`pygo/`)

**What you work on:** The consumer-facing Go package.

**Prerequisites:** Go API design. Read `docs/api.md`.

**Depends on:** Runtime.

**Representative tasks:**
- Implement a missing API method
- Improve error types
- Add/improve functional options
- Write usage examples
- API documentation improvements

---

### M. CLI and REPL (`cmd/pygo/`)

**What you work on:** The standalone binary.

**Prerequisites:** Go. Read `docs/cli.md`.

**Depends on:** Public API (`pygo/`).
**Independent from:** All internal packages.

**Representative tasks:**
- Improve REPL incomplete-input detection
- Improve traceback formatting
- Add `pygo disasm` subcommand
- Improve `--help` output
- Add CLI integration tests

---

### N. Testing Infrastructure

**What you work on:** Test helpers, compat test runner, golden file infrastructure.

**Prerequisites:** Go testing.

**Representative tasks:**
- Add compat test scripts to `testdata/compat/`
- Add fuzz corpus entries
- Improve test helper utilities
- Add benchmark programs

---

### O. Documentation

**What you work on:** Prose documentation.

**Prerequisites:** Python knowledge (for compatibility documentation). Writing skills.

**Representative tasks:**
- Update compatibility matrix as features are implemented
- Write tutorial examples in `examples/`
- Improve docstrings on public types and functions
- Document known divergences from CPython
- Write a "getting started" tutorial

---

## 3. Dependency Map

The following table shows which areas can be worked on independently:

```
         Lexer ─────────────────────────────────────────────► (standalone)
           │
         Parser ────────────────────────────────────────────► (depends on lexer + AST)
           │
         AST ───────────────────────────────────────────────► (standalone)
           │
         Compiler ──────────────────────────────────────────► (depends on AST + bytecode)
           │
         Bytecode ──────────────────────────────────────────► (standalone)
           │
         Object Model ──────────────────────────────────────► (standalone)
           │
         VM ─────────────────────────────────────────────────► (depends on bytecode + object model)
           │
         Runtime ────────────────────────────────────────────► (depends on all of the above)
           │
         ┌─────────────────┬──────────────────┐
         │                 │                  │
      Stdlib            Interop           Importer     (all depend on object model + runtime)
         │                 │                  │
         └─────────────────┴──────────────────┘
                                │
                           Public API
                                │
                              CLI
```

**Safe to work in parallel (no merge conflicts expected):**
- Lexer + AST + Bytecode definitions (all standalone)
- Different stdlib modules (each is a separate file/directory)
- Object model types (each type is a separate file)
- Parser + Compiler (different packages, different concerns)

**Requires coordination:**
- Changes to bytecode opcode layout (affects compiler + VM)
- Changes to core `object.Object` interface (affects everything)
- Changes to the public API (`pygo/`) (semver implications)
- Changes to `internal/runtime/runtime.go` (central coordinator)

---

## 4. Good First Issues

Issues tagged `good-first-issue` will be maintained in the GitHub issue tracker. Categories:

**Lexer / Parser (low risk, isolated):**
- Implement a missing string escape sequence
- Add a missing operator token
- Fix a parsing error message

**Object Model (medium risk, isolated):**
- Implement `str.zfill()`
- Implement `list.index()`
- Implement `dict.setdefault()`
- Implement `dict.update()` with keyword arguments

**Standard Library (low risk, isolated):**
- Implement `math.gcd()`
- Implement `math.lcm()`
- Implement `itertools.islice()`
- Implement `functools.reduce()`

**Tests (no risk):**
- Add a compat test for a specific Python feature
- Add fuzz corpus entries for the lexer
- Improve a test helper utility

**Documentation (no risk):**
- Fix a documentation error
- Improve an example
- Add a new example program

---

## 5. Contribution Process

1. **Open an issue** (or comment on an existing one) before starting substantial work. This avoids duplicate effort and ensures the design direction aligns with the project.

2. **Fork and branch.** Branch names: `feat/short-description`, `fix/issue-nnn-description`, `docs/topic`.

3. **Write tests first or alongside implementation.** PRs without tests for new functionality will not be merged.

4. **Run the full test suite** before submitting:
   ```bash
   go test ./... -race
   go vet ./...
   ```

5. **Open a pull request** against `main`. Use the PR template (to be provided). Include:
   - What this PR changes and why.
   - Which phase/roadmap item this addresses.
   - Test plan (what tests were added).
   - Notes on any CPython divergences.

6. **Code review.** All PRs require at least one review from a maintainer. For architectural changes, two reviews are required.

7. **Merge.** Maintainers merge after approval. Squash merge is preferred to keep a clean history.

---

## 6. Code Standards

- **Go version:** minimum Go 1.22 (uses `go/ast`, `go/types`, generics where appropriate).
- **Formatting:** `gofmt`. No exceptions. CI enforces this.
- **Linting:** `go vet` must pass. A golangci-lint configuration will be provided.
- **Error handling:** always return errors; do not silently ignore `err != nil`.
- **No `panic` in library code** (only in `Must*` convenience wrappers). The VM uses `recover()` at its top level.
- **No `unsafe`** unless there is a documented, reviewed justification.
- **Package docs:** every exported type, function, and method must have a Go doc comment.
- **No global mutable state** in library packages. All state lives in `Runtime` or per-execution objects.
- **Internal packages** must not import `pygo/` (the public package) — enforce the layering.

---

## 7. Testing Requirements

- Every new feature needs tests. See `docs/testing.md` for the required test layers per component.
- Every bug fix needs a regression test.
- Tests must pass with `-race`.
- Compat tests (in `testdata/compat/`) are required for new Python language features.
- Golden files are generated with `go test -update` and must be committed.

---

## 8. Documentation Requirements

- Public API changes must update `docs/api.md`.
- New Python features must update the compatibility matrix (`docs/compatibility.md`) — mark the feature with the phase it was implemented in and change the status from future/post-MVP to implemented.
- Architectural decisions must be documented (in the relevant `docs/` file or in a new ADR — Architecture Decision Record — in `docs/adr/`).
- Examples for significant features go in `examples/`.

---

## 9. Communication

- **GitHub Issues:** bug reports, feature requests, design discussions.
- **GitHub Discussions:** open-ended questions, RFCs for larger changes.
- **Pull Requests:** implementation work.

For large architectural changes (e.g., "rethink the bytecode format" or "change the object model interface"), open a GitHub Discussion with an RFC before opening a PR. This avoids large PRs being rejected because of design disagreement.

---

## 10. Decision-Making

- Day-to-day implementation decisions: made by the contributor in the PR, reviewed by maintainers.
- API surface changes: require maintainer consensus before merging.
- Architectural changes: require an RFC + Discussion with at least one week of comment period before implementation.
- Roadmap reprioritization: discussed in a GitHub Discussion; maintainers have final say.

---

## 11. Getting Started Checklist

```
[ ] Read VISION.md
[ ] Read docs/architecture.md
[ ] Read the doc for your chosen contribution area (e.g., docs/object-model.md for object model work)
[ ] Clone the repo: git clone https://github.com/alvinjaison/PyGo
[ ] Verify it builds: go build ./...
[ ] Verify tests pass: go test ./...
[ ] Pick a good-first-issue or open one proposing your contribution
[ ] Comment on the issue to claim it
[ ] Start coding
```

Welcome. Let's build something useful.
