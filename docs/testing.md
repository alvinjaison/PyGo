# PyGo — Testing Strategy

## Table of Contents

1. [Philosophy](#1-philosophy)
2. [Test Layer Architecture](#2-test-layer-architecture)
3. [Lexer Tests](#3-lexer-tests)
4. [Parser Tests](#4-parser-tests)
5. [Compiler Tests](#5-compiler-tests)
6. [VM Tests](#6-vm-tests)
7. [Runtime / Integration Tests](#7-runtime--integration-tests)
8. [Compatibility Tests](#8-compatibility-tests)
9. [Go Interoperability Tests](#9-go-interoperability-tests)
10. [CLI Tests](#10-cli-tests)
11. [Regression Tests](#11-regression-tests)
12. [Benchmarks](#12-benchmarks)
13. [Fuzz Tests](#13-fuzz-tests)
14. [Test Infrastructure](#14-test-infrastructure)
15. [CI Pipeline](#15-ci-pipeline)

---

## 1. Philosophy

- Every layer is tested independently. A test that fails in the VM must not require debugging the parser to understand the failure.
- Test inputs and expected outputs are explicit. No "it compiled without panicking" is a passing test — the actual output must be verified.
- CPython is used as a **reference for expected outputs**, not as a dependency at test time. Test scripts that can be run against both PyGo and CPython provide the highest confidence.
- Tests are written before or alongside the feature. No feature is considered done until it has test coverage.
- Regression tests are created whenever a bug is fixed. The bug's reproducer becomes a permanent test.
- Performance is tested with benchmarks, not assertions. Benchmark regressions are tracked separately from correctness regressions.

---

## 2. Test Layer Architecture

```
testdata/compat/           ← Python scripts runnable against CPython + PyGo
    basic/
    functions/
    classes/
    exceptions/
    comprehensions/
    generators/
    imports/
    stdlib/

internal/lexer/
    lexer_test.go          ← unit tests: source string → []Token

internal/parser/
    parser_test.go         ← unit tests: source string → AST snapshot

internal/ast/
    walk_test.go           ← visitor traversal tests

internal/compiler/
    compiler_test.go       ← unit tests: source string → bytecode snapshot

internal/bytecode/
    disasm_test.go         ← disassembler output tests

internal/vm/
    vm_test.go             ← unit tests: *Code → result/exception

internal/object/
    int_test.go
    str_test.go
    list_test.go
    dict_test.go
    ...

internal/runtime/
    runtime_test.go        ← integration: RunString → result

internal/interop/
    interop_test.go        ← Go↔Python conversion tests

internal/importer/
    importer_test.go       ← module resolution tests

pygo/
    api_test.go            ← public API contract tests

cmd/pygo/
    cli_test.go            ← CLI subcommand tests (exec + stdout capture)
```

---

## 3. Lexer Tests

**What is tested:** Given a Python source string, the lexer produces the expected token sequence.

**Format:** Table-driven Go tests. Each test case has a `source` string and a `want []Token` slice.

```go
func TestLexer(t *testing.T) {
    tests := []struct {
        name   string
        source string
        want   []Token
    }{
        {
            name:   "integer_literal",
            source: "42",
            want: []Token{
                {Type: INTEGER, Value: "42", Line: 1, Col: 1},
                {Type: NEWLINE},
                {Type: EOF},
            },
        },
        {
            name:   "indentation",
            source: "if True:\n    pass\n",
            want: []Token{
                {Type: NAME, Value: "if"},
                {Type: NAME, Value: "True"},
                {Type: COLON},
                {Type: NEWLINE},
                {Type: INDENT},
                {Type: NAME, Value: "pass"},
                {Type: NEWLINE},
                {Type: DEDENT},
                {Type: EOF},
            },
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := lexAll(tt.source)
            assertTokensEqual(t, tt.want, got)
        })
    }
}
```

**Coverage targets:**
- All keyword tokens
- All operator tokens
- All delimiter tokens
- Integer literals: decimal, hex, octal, binary, underscored
- Float literals: simple, scientific, underscored
- String literals: single, double, triple-quoted, prefixes (r, b, f, rb)
- INDENT / DEDENT sequences (various depths, mixed tabs and spaces — error case)
- Logical line continuation (`\`)
- Comments (discarded)
- Unicode identifiers
- f-string boundary tokens

**Edge cases that must be tested:**
- Empty source
- Source with only whitespace
- Unterminated string literals (error)
- Mismatched indentation (error)
- Deeply nested INDENT/DEDENT sequences
- Operator at end of file (no trailing newline)

---

## 4. Parser Tests

**What is tested:** Given a Python source string, the parser produces the expected AST.

**Two test approaches:**

### 4a. AST snapshot tests

For complex structures, the AST is serialized to a stable text format and compared against a stored golden file:

```go
func TestParser_FunctionDef(t *testing.T) {
    source := `def add(a, b):\n    return a + b\n`
    tree := mustParse(t, source)
    got := ast.Sprint(tree)
    golden := loadGolden(t, "testdata/parser/function_def.txt")
    assertGolden(t, golden, got)
}
```

Golden files are in `testdata/parser/*.txt`. They are updated with `go test -update`.

### 4b. Structural assertions

For simpler cases, assert specific properties of the AST directly:

```go
func TestParser_BinaryOp(t *testing.T) {
    tree := mustParseExpr(t, "1 + 2")
    binop, ok := tree.(*ast.BinOp)
    require.True(t, ok)
    assert.Equal(t, ast.Add, binop.Op)
    assertInt(t, 1, binop.Left)
    assertInt(t, 2, binop.Right)
}
```

**Coverage targets:**
- All statement types
- All expression types
- All operator precedence combinations
- Function definitions with all argument forms
- Class definitions with and without bases
- Import forms
- Try/except/else/finally
- With statement
- Comprehensions (list, dict, set, generator)
- Nested scopes
- f-strings
- Decorator syntax
- `match` statement (when implemented)

**Error recovery tests:**
- Syntax errors produce useful messages with correct line/column
- Multiple syntax errors are reported together where possible

---

## 5. Compiler Tests

**What is tested:** Given a Python source string (or AST), the compiler produces the expected bytecode.

**Format:** Bytecode snapshot tests using the disassembler.

```go
func TestCompiler_SimpleAssignment(t *testing.T) {
    code := mustCompile(t, "x = 1")
    got := bytecode.Disassemble(code)
    assertGolden(t, "testdata/compiler/simple_assignment.txt", got)
}
```

Example golden file content:
```
LOAD_CONST   0 (1)
STORE_NAME   0 (x)
LOAD_CONST   1 (None)
RETURN_VALUE
```

**Coverage targets:**
- Variable assignment (local, global, nonlocal)
- All binary and unary operators
- Function definition (creates nested Code object)
- Default arguments
- `*args`, `**kwargs`
- Closure variable capture (LOAD_DEREF, STORE_DEREF)
- `if/elif/else` (correct jump offsets)
- `while` with `break`, `continue`
- `for` loop with `break`, `continue`, `else`
- `try/except/finally` (exception table entries)
- Class definition
- Import statements
- Comprehensions (nested Code objects)
- `yield` in functions (generator flag on Code)

**Symbol table tests:**
The compiler's symbol table analysis is tested separately:

```go
func TestSymTable_LocalVsGlobal(t *testing.T) {
    syms := mustAnalyze(t, `
def f():
    x = 1
    return x
`)
    f := syms.Lookup("f")
    assert.Equal(t, SymGlobal, f.Scope)
    x := f.Children[0].Lookup("x")
    assert.Equal(t, SymLocal, x.Scope)
}
```

---

## 6. VM Tests

**What is tested:** Given a `*bytecode.Code` object (or Python source compiled to code), the VM produces the expected result or exception.

```go
func TestVM_ArithmeticAdd(t *testing.T) {
    result := mustRun(t, "1 + 2")
    assertInt(t, 3, result)
}

func TestVM_DivisionByZero(t *testing.T) {
    _, err := run("1 / 0")
    assertPyError(t, "ZeroDivisionError", err)
}

func TestVM_ClosureCapture(t *testing.T) {
    result := mustRun(t, `
def make_adder(n):
    def adder(x):
        return x + n
    return adder
add5 = make_adder(5)
add5(3)
`)
    assertInt(t, 8, result)
}
```

**Coverage targets:** Every opcode must have at least one direct test. Every opcode interaction (e.g., exception raised during BINARY_ADD, generator suspended at YIELD_VALUE) must be tested.

---

## 7. Runtime / Integration Tests

**What is tested:** `RunString` → correct program behavior, including side effects (output capture).

Output capture pattern:
```go
func TestRuntime_Print(t *testing.T) {
    rt := pygo.New()
    output := captureStdout(t, func() {
        err := rt.RunString(`print("hello")`)
        require.NoError(t, err)
    })
    assert.Equal(t, "hello\n", output)
}
```

**Coverage targets:**
- End-to-end execution of representative Python programs
- Variable persistence across multiple `RunString` calls on the same runtime
- Exception propagation to Go caller
- `Set` / `Get` round-trips
- Module loading
- Multiple concurrent runtimes (one goroutine each)

---

## 8. Compatibility Tests

These are the most important tests for measuring Python compliance.

### Structure

`testdata/compat/` contains Python `.py` files. Each file has a corresponding `.expected` file containing the expected stdout output (generated by CPython).

```
testdata/compat/
    basic/
        hello.py
        hello.expected
        arithmetic.py
        arithmetic.expected
    functions/
        defaults.py
        defaults.expected
        closures.py
        closures.expected
    ...
```

### Running against CPython (generate expected outputs)

```bash
make compat-generate  # runs all .py files through python3 and writes .expected files
```

This is run by contributors with CPython installed. The `.expected` files are committed to the repository.

### Running against PyGo (CI)

```bash
make compat-test  # runs all .py files through pygo and diffs against .expected
```

CPython is not required in CI. Only the committed `.expected` files are needed.

### Test runner

```go
func TestCompat(t *testing.T) {
    entries, _ := filepath.Glob("../../testdata/compat/**/*.py")
    for _, pyFile := range entries {
        pyFile := pyFile
        t.Run(pyFile, func(t *testing.T) {
            expectedFile := strings.TrimSuffix(pyFile, ".py") + ".expected"
            expected, err := os.ReadFile(expectedFile)
            require.NoError(t, err, "missing .expected file")

            got := runPyGo(t, pyFile)
            assert.Equal(t, string(expected), got)
        })
    }
}
```

### Skip markers

Some tests are known to fail on PyGo features not yet implemented. Add a `# pygo-skip: <reason>` comment at the top of the `.py` file to mark it as expected-to-fail:

```python
# pygo-skip: generators not yet implemented
def gen():
    yield 1
    yield 2

for x in gen():
    print(x)
```

Skipped tests are tracked and reported separately. The goal is to shrink the skip list over time.

---

## 9. Go Interoperability Tests

**What is tested:** The `internal/interop` package and the registration API.

```go
func TestInterop_FunctionCall(t *testing.T) {
    rt := pygo.New()
    rt.MustRegisterFunction("add", func(a, b int) int { return a + b })

    result, err := rt.Eval("add(3, 4)")
    require.NoError(t, err)
    n, _ := result.Int()
    assert.Equal(t, int64(7), n)
}

func TestInterop_StructFields(t *testing.T) {
    type Point struct{ X, Y int }
    rt := pygo.New()
    rt.MustRegister("p", &Point{X: 3, Y: 4})

    result, _ := rt.Eval("p.X + p.Y")
    n, _ := result.Int()
    assert.Equal(t, int64(7), n)
}

func TestInterop_GoErrorToPythonException(t *testing.T) {
    rt := pygo.New()
    rt.MustRegisterFunction("fail", func() error {
        return errors.New("something went wrong")
    })

    err := rt.RunString("fail()")
    assert.True(t, pygo.IsError(err, "RuntimeError"))
    assert.Contains(t, err.Error(), "something went wrong")
}

func TestInterop_TypeConversion(t *testing.T) {
    tests := []struct {
        name     string
        goValue  interface{}
        pyExpr   string  // expression using the registered value
        want     interface{}
    }{
        {"bool_true",   true,    "type(v).__name__", "bool"},
        {"int",         42,      "v + 1",            int64(43)},
        {"float",       3.14,    "v > 3.0",          true},
        {"string",      "hello", "v.upper()",        "HELLO"},
        {"slice_int",   []int{1,2,3}, "len(v)",      int64(3)},
        {"map_str_int", map[string]int{"a":1}, "v['a']", int64(1)},
    }
    // ...
}
```

---

## 10. CLI Tests

**What is tested:** The `pygo` binary's behavior (script execution, `-c` flag, exit codes, error output).

These are subprocess-based tests that compile and run the binary:

```go
func TestCLI_RunScript(t *testing.T) {
    binary := buildBinary(t)  // go build → temp binary
    out, err := exec.Command(binary, "testdata/hello.py").Output()
    require.NoError(t, err)
    assert.Equal(t, "Hello, world!\n", string(out))
}

func TestCLI_CFlag(t *testing.T) {
    binary := buildBinary(t)
    out, _ := exec.Command(binary, "-c", "print(1+1)").Output()
    assert.Equal(t, "2\n", string(out))
}

func TestCLI_SyntaxErrorExitCode(t *testing.T) {
    binary := buildBinary(t)
    cmd := exec.Command(binary, "testdata/syntax_error.py")
    err := cmd.Run()
    var exitErr *exec.ExitError
    require.True(t, errors.As(err, &exitErr))
    assert.Equal(t, 1, exitErr.ExitCode())
}
```

---

## 11. Regression Tests

Every bug fix must be accompanied by a regression test. The test goes in the most appropriate layer (lexer, parser, compiler, VM, or runtime).

Naming convention: `TestRegression_IssueNNN_description`.

```go
func TestRegression_Issue42_NegativeModulo(t *testing.T) {
    // Python modulo: -7 % 3 == 2 (sign follows divisor)
    // This was incorrectly returning -1.
    result := mustRun(t, "-7 % 3")
    assertInt(t, 2, result)
}
```

---

## 12. Benchmarks

Benchmarks use Go's standard `testing.B` interface. They are in `*_bench_test.go` files (or `*_test.go` files with `Benchmark` prefix).

### Micro-benchmarks (per component)

```go
// Lexer throughput
func BenchmarkLexer_1000Lines(b *testing.B) { ... }

// Parser throughput
func BenchmarkParser_FunctionDef(b *testing.B) { ... }

// VM: tight arithmetic loop
func BenchmarkVM_Loop(b *testing.B) {
    code := mustCompile(nil, `
total = 0
for i in range(1000000):
    total += i
`)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        rt := pygo.New()
        rt.RunString(...)
    }
}
```

### Macro-benchmarks

Benchmark programs representative of real workloads:
- Fibonacci (recursive and iterative)
- Quicksort on a list of integers
- String processing (split, join, format)
- Dictionary operations
- Function call overhead (many small calls)

Benchmarks are not run in CI by default. They are run manually or on a dedicated performance-tracking workflow.

### Performance baseline

Establish a baseline in Phase 2 and track regressions from Phase 3 onward. The initial baseline should be documented in a `BENCHMARKS.md` file.

---

## 13. Fuzz Tests

Go 1.18+ native fuzzing is used for the lexer and parser, which are the primary attack surfaces for malformed input.

```go
// internal/lexer/fuzz_test.go
func FuzzLexer(f *testing.F) {
    f.Add("x = 1\n")
    f.Add("def f():\n    pass\n")
    f.Fuzz(func(t *testing.T, src string) {
        // Must not panic. Errors are acceptable; panics are not.
        defer func() {
            if r := recover(); r != nil {
                t.Fatalf("lexer panicked on input %q: %v", src, r)
            }
        }()
        l := lexer.New(src)
        for {
            tok := l.Next()
            if tok.Type == lexer.EOF {
                break
            }
        }
    })
}

// internal/parser/fuzz_test.go
func FuzzParser(f *testing.F) {
    f.Add("x = 1\n")
    f.Fuzz(func(t *testing.T, src string) {
        defer func() { recover() }()
        parser.Parse(lexer.New(src))
    })
}
```

Fuzz tests are run as part of a scheduled CI job (not every PR, due to runtime cost). Interesting fuzz corpus entries are committed to `testdata/fuzz/`.

---

## 14. Test Infrastructure

### Helper packages

`internal/testutil/` provides shared test helpers:

```go
package testutil

// MustCompile compiles source or fails the test.
func MustCompile(t *testing.T, source string) *bytecode.Code

// MustRun compiles and runs source, returning the last expression's value.
func MustRun(t *testing.T, source string) object.Object

// AssertPyError asserts that err is a *pygo.Error of the given type.
func AssertPyError(t *testing.T, typeName string, err error)

// AssertInt asserts that obj is an IntObject with the given value.
func AssertInt(t *testing.T, expected int64, obj object.Object)

// CaptureStdout captures stdout during the execution of fn.
func CaptureStdout(t *testing.T, fn func()) string
```

### Golden file management

```bash
go test ./... -update  # regenerate all golden files
```

Golden files are committed. Unexpected changes to golden files in CI are a test failure.

### Test data organization

- `testdata/compat/` — Python scripts with `.expected` files
- `testdata/fuzz/` — fuzz corpus inputs
- `testdata/lexer/` — lexer-specific test inputs
- `testdata/parser/` — parser golden files
- `testdata/compiler/` — compiler golden files

---

## 15. CI Pipeline

```yaml
# Every PR:
- go vet ./...
- go build ./...
- go test ./... -count=1 -race    # with race detector
- go test ./internal/lexer/... -fuzz=FuzzLexer -fuzztime=10s  # short fuzz run

# Nightly:
- go test ./... -fuzz=... -fuzztime=5m   # longer fuzz run
- go test -bench=. -benchmem ./...      # benchmark run (results stored)
- compat-test                            # cross-reference against stored .expected files

# On tag (release):
- go build -ldflags="-s -w" ./cmd/pygo
- Release binary artifacts
```

**Race detector:** All tests run with `-race`. The Go runtime's race detector will catch concurrent access violations in the interop layer and runtime. This is important because the interop layer uses reflection and the runtime state is accessed from user-provided callbacks.

**Coverage:** Track coverage per package. The goal is not 100% coverage (which can be gamed), but meaningful coverage of all correctness-critical paths. Coverage reports are generated but not used as a pass/fail gate.
