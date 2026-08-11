# PyGo — CLI and REPL Design

## Table of Contents

1. [Design Principles](#1-design-principles)
2. [Command Structure](#2-command-structure)
3. [Script Execution](#3-script-execution)
4. [Inline Execution (`-c`)](#4-inline-execution--c)
5. [Interactive REPL](#5-interactive-repl)
6. [Error Output Format](#6-error-output-format)
7. [Exit Codes](#7-exit-codes)
8. [Implementation Architecture](#8-implementation-architecture)
9. [Package Layout](#9-package-layout)
10. [Future Commands](#10-future-commands)

---

## 1. Design Principles

- The CLI is a **thin wrapper** around the `pygo` library. Zero interpreter logic lives in the CLI package.
- The REPL uses the exact same `pygo.Runtime` as embedded mode — there is no separate interpreter for interactive use.
- The CLI user experience should feel familiar to Python developers: error messages, traceback format, and prompt style should mirror CPython's where practical.
- The binary is statically compiled — one binary, no shared library dependencies, no CPython installation required.

---

## 2. Command Structure

```
pygo [flags] [script.py [args...]]
pygo -c "source"
pygo [flags]          (no script → REPL)
```

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-v`, `--version` | — | Print PyGo version and exit |
| `--verbose` | off | Enable verbose/diagnostic output |
| `--path PATH` | `.` | Prepend to Python module search path |
| `--max-depth N` | 500 | Maximum call stack depth |
| `--timeout DURATION` | 0 (none) | Execution timeout (e.g., `30s`, `5m`) |
| `--no-stdlib` | off | Disable standard library modules |

---

## 3. Script Execution

```bash
pygo script.py
pygo script.py arg1 arg2      # args passed as sys.argv
```

Behavior:
1. Open and read `script.py`. If the file does not exist, print an error to stderr and exit 2.
2. Create a `pygo.Runtime` with the script's directory prepended to the search path.
3. Set `sys.argv = [script.py, arg1, arg2, ...]`.
4. Set `__name__ = "__main__"` in the script's globals.
5. `RunFileContext(ctx, script.py)`.
6. If an unhandled exception occurs, print the traceback to stderr and exit 1.
7. If execution completes normally, exit 0.

### `sys.argv` handling

```go
rt.Set("__name__", "__main__")
argv := make([]interface{}, len(os.Args)-1)
for i, a := range os.Args[1:] {
    argv[i] = a
}
rt.RunString(`import sys`)
rt.Set("__argv__", argv)
rt.RunString(`sys.argv = [__argv__[0]] + list(__argv__[1:])`)
```

The cleaner approach is to set `sys.argv` directly in the runtime before execution rather than via Python source.

---

## 4. Inline Execution (`-c`)

```bash
pygo -c "print(1 + 1)"
pygo -c "import math; print(math.pi)"
```

Behavior:
1. Create a runtime.
2. Set `sys.argv = ["-c"]`.
3. `RunString(source)`.
4. Handle errors as in script execution.

The `-c` flag takes exactly one argument (the entire source string, as a single shell argument). Multi-line programs can be passed using shell quoting or heredocs:

```bash
pygo -c "
for i in range(10):
    print(i)
"
```

---

## 5. Interactive REPL

```bash
pygo
```

```
PyGo 0.1.0 (Go 1.22) on darwin
Type "exit()" or Ctrl-D to quit.
>>> x = 10
>>> x * 2
20
>>> def greet(name):
...     return "Hello, " + name
...
>>> greet("world")
'Hello, world'
>>>
```

### REPL behavior

- **Prompt:** `>>> ` for a new statement, `... ` for continuation lines.
- **Incomplete input detection:** The REPL distinguishes between a syntax error (bad input) and an incomplete statement (more input expected). An incomplete statement (e.g., after `def f():`) shows `... ` and waits for more input.
- **Expression results:** If a statement is a single expression and its result is not `None`, print `repr(result)` to stdout — matching CPython's REPL behavior.
- **Exception display:** Unhandled exceptions in REPL mode print the traceback but do not terminate the session.
- **Persistence:** The REPL maintains a single `Runtime` instance for the entire session. Variables, functions, and imports defined in earlier entries remain available.
- **Exit:** `exit()`, `quit()`, or Ctrl-D (EOF) exit cleanly.
- **History:** Line history is supported via the `readline`-compatible library (`github.com/chzyer/readline`). History is stored in `~/.pygo_history`.

### Incomplete input detection

The REPL feeds input to the parser in a "test for completeness" mode. The parser returns one of:

```go
type ParseResult int
const (
    ParseOK         ParseResult = iota // full valid statement
    ParseIncomplete                    // needs more input (open block, continuation)
    ParseError                         // syntax error
)
```

The parser must not consume the token stream when returning `ParseIncomplete`. The REPL accumulates lines until `ParseOK` or `ParseError`.

Incomplete detection triggers on:
- Open `def`, `class`, `if`, `for`, `while`, `try`, `with` blocks (line ending with `:`)
- Open parentheses, brackets, or braces (implicit line continuation)
- Explicit line continuation (`\` at end of line)

### REPL startup

On startup, the REPL:
1. Creates a runtime with full stdlib enabled.
2. Imports `sys` and sets `sys.ps1 = ">>> "`, `sys.ps2 = "... "`.
3. Prints the version banner.
4. Enters the read-eval-print loop.

### Multiline input example

```
>>> def fib(n):
...     if n <= 1:
...         return n
...     return fib(n-1) + fib(n-2)
...
>>> fib(10)
55
```

The `...` prompt is shown after the `:` and until the user enters a blank line to terminate the block — matching CPython's behavior.

### Error display in REPL

```
>>> 1 / 0
Traceback (most recent call last):
  File "<stdin>", line 1, in <module>
ZeroDivisionError: division by zero
>>>
```

The session continues after the error.

---

## 6. Error Output Format

PyGo's error output format mirrors CPython closely:

### Traceback format

```
Traceback (most recent call last):
  File "script.py", line 5, in <module>
  File "script.py", line 2, in f
  File "script.py", line 8, in g
RuntimeError: something went wrong
```

For the REPL:
```
Traceback (most recent call last):
  File "<stdin>", line 1, in <module>
ZeroDivisionError: division by zero
```

For `-c` execution:
```
Traceback (most recent call last):
  File "<string>", line 1, in <module>
NameError: name 'x' is not defined
```

### Syntax errors

```
  File "script.py", line 3
    def f(
         ^
SyntaxError: unexpected EOF while parsing
```

Syntax errors include a caret pointing at the error location.

---

## 7. Exit Codes

| Condition | Exit Code |
|-----------|-----------|
| Normal completion | 0 |
| Python exception (unhandled) | 1 |
| `SystemExit(n)` | `n` |
| `SystemExit(0)` or `SystemExit()` | 0 |
| File not found / IO error | 2 |
| Invalid flag / usage error | 2 |
| Timeout / cancellation | 1 |

These match CPython's exit code conventions.

---

## 8. Implementation Architecture

```
cmd/pygo/
├── main.go        ← entry point; parses flags; dispatches to run/repl/eval
├── flags.go       ← flag definitions and parsing
├── run.go         ← script execution and -c mode
├── repl.go        ← interactive REPL loop
└── errors.go      ← traceback formatting for CLI output
```

### `main.go`

```go
func main() {
    cfg := flags.Parse(os.Args[1:])
    switch cfg.Mode {
    case flags.ModeScript:
        os.Exit(runScript(cfg))
    case flags.ModeCommand:
        os.Exit(runCommand(cfg))
    case flags.ModeREPL:
        os.Exit(runREPL(cfg))
    }
}
```

### `run.go`

```go
func runScript(cfg *flags.Config) int {
    rt := buildRuntime(cfg)
    ctx := buildContext(cfg)
    if err := rt.RunFileContext(ctx, cfg.Script); err != nil {
        printError(err, cfg.Script)
        return exitCodeForError(err)
    }
    return 0
}
```

### `repl.go`

```go
func runREPL(cfg *flags.Config) int {
    rt := buildRuntime(cfg)
    rl := readline.New(...)
    printBanner()

    var buf strings.Builder
    prompt := ">>> "
    for {
        line, err := rl.Readline()
        if err == io.EOF { break }

        buf.WriteString(line + "\n")
        source := buf.String()

        switch parseResult(source) {
        case ParseIncomplete:
            prompt = "... "
            continue
        case ParseOK, ParseError:
            prompt = ">>> "
            buf.Reset()
        }

        result, err := rt.EvalOrRun(source)
        if err != nil {
            printError(err, "<stdin>")
        } else if result != nil && !result.IsNone() {
            fmt.Println(result.Repr())
        }
    }
    return 0
}
```

### `buildRuntime`

```go
func buildRuntime(cfg *flags.Config) *pygo.Runtime {
    opts := []pygo.Option{
        pygo.WithSearchPath(cfg.SearchPath),
        pygo.WithMaxCallDepth(cfg.MaxDepth),
    }
    if cfg.NoStdlib {
        opts = append(opts, pygo.WithStdlib(false))
    }
    if cfg.Timeout > 0 {
        opts = append(opts, pygo.WithExecutionTimeout(cfg.Timeout))
    }
    return pygo.New(opts...)
}
```

---

## 9. Package Layout

The `cmd/pygo/` package has no logic beyond wiring the `pygo` library to stdin/stdout/stderr and the filesystem. Every behavior decision (execution, exception handling, module loading) is in the library.

This guarantees the invariant: **the CLI and embedded mode are identical in behavior**.

---

## 10. Future Commands

Phase 6+ may add subcommands:

```bash
pygo run script.py          # explicit run subcommand
pygo compile script.py      # compile to bytecode file (for caching/inspection)
pygo disasm script.py       # dump bytecode disassembly
pygo ast script.py          # dump AST
pygo check script.py        # syntax check only (no execution)
pygo fmt script.py          # code formatter (very long-term)
```

These subcommands all use the same library. The `pygo` root command with no subcommand defaults to REPL for backward compatibility.

### Version output

```bash
$ pygo --version
PyGo 0.1.0
  Go runtime:   go1.22
  Python compat: 3.x (subset — see compatibility matrix)
  Build:        darwin/arm64
```
