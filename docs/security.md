# PyGo — Security Considerations

## Table of Contents

1. [Security Posture Statement](#1-security-posture-statement)
2. [Threat Model](#2-threat-model)
3. [The Capability Boundary](#3-the-capability-boundary)
4. [Default Configuration Security](#4-default-configuration-security)
5. [Restricted Runtime Mode](#5-restricted-runtime-mode)
6. [Execution Limits](#6-execution-limits)
7. [Cancellation and Timeout](#7-cancellation-and-timeout)
8. [Go ↔ Python Boundary Safety](#8-go--python-boundary-safety)
9. [Hash Flooding](#9-hash-flooding)
10. [Resource Exhaustion](#10-resource-exhaustion)
11. [Code Injection](#11-code-injection)
12. [Known Limitations](#12-known-limitations)
13. [Future Sandboxing](#13-future-sandboxing)
14. [Responsible Disclosure](#14-responsible-disclosure)

---

## 1. Security Posture Statement

**PyGo is not a secure sandbox in its initial release.**

This must be stated clearly and prominently in the documentation.

PyGo is designed to be an embeddable Python runtime for **trusted scripts** — scripts written or controlled by the application developer or trusted operators. It is not designed to safely execute arbitrary untrusted code from unknown sources without additional controls.

However, PyGo's architecture is deliberately designed so that security controls can be layered in over time. The key property is: **Python scripts can only interact with host capabilities that the host deliberately exposes.** This is a meaningful security property even without a full sandbox.

---

## 2. Threat Model

### In scope (things PyGo should defend against in later phases)

- **Runaway execution:** infinite loops or deep recursion consuming CPU and goroutine stack until the process crashes or becomes unresponsive.
- **Memory exhaustion:** Python programs allocating unbounded objects.
- **Unexpected host API access:** Python code reaching Go APIs that the host did not intend to expose.
- **Hash flooding:** crafted input producing worst-case dict/set performance.
- **Panic propagation:** a bug in PyGo's runtime causing an unrecovered Go panic that crashes the host application.

### Out of scope initially (not claimed as defended)

- Privilege escalation within the host process.
- Hardware-level side-channel attacks (Spectre, etc.).
- Timing attacks on cryptographic operations.
- Denial of service from a fully trusted script (the host is assumed to control the scripts it runs).

### Attacker model (when using restricted mode)

When a host enables restricted execution, the attacker is:
- Able to provide arbitrary Python source code to be executed.
- Unable to modify the PyGo runtime, the Go host binary, or the OS.
- Attempting to access host resources (filesystem, network, environment variables, other processes) or exhaust host resources.

---

## 3. The Capability Boundary

The most important security property of PyGo is:

> A Python script can only interact with host capabilities that the host explicitly registers via `RegisterFunction`, `Register`, or `RegisterModule`.

**What this means in practice:**

By default, a PyGo runtime does not expose:
- `open()` / file I/O
- `subprocess` / process execution
- `socket` / network access
- `os.environ` / environment variables
- `os.system` / `os.execv`
- Any Go-internal state of the host application

**The host grants capabilities.** The host is the security policy enforcer.

Example of a tightly scoped runtime:
```go
rt := pygo.New(pygo.WithRestrictedBuiltins())
// Only exposes: print, len, range, type, basic math, string ops
// Does NOT expose: open, input, __import__ (beyond pre-loaded modules)
rt.MustRegisterFunction("query", myDBQueryFunc)  // explicitly grant DB access
rt.RunString(userScript)
```

**What this does NOT protect against:**

If the host calls `pygo.New()` with default settings and loads full stdlib, the script has filesystem access, environment access, etc. The host is responsible for choosing the appropriate configuration.

---

## 4. Default Configuration Security

The **default `pygo.New()`** configuration:
- Enables standard built-ins including `print`, `len`, `range`, `type`, conversions.
- Enables `math`, `sys`, `json`, `re`, `time` modules.
- **Enables `os` module (subset)** — this means filesystem access is available by default.
- **Enables `open()`** — file reading and writing are available.

This default is appropriate for trusted scripts. It is not appropriate for untrusted script execution.

The documented recommendation for any untrusted input is to use `WithRestrictedBuiltins()` and to only register the specific capabilities the script needs.

---

## 5. Restricted Runtime Mode

```go
rt := pygo.New(pygo.WithRestrictedBuiltins())
```

In restricted mode:
- `open()` is not available.
- `input()` is not available.
- `os` module is not available.
- `subprocess` module is not available.
- `socket` module is not available.
- `__import__` is limited to pre-registered modules only.
- `eval()` and `exec()` are not available.
- `compile()` is not available.

Available in restricted mode:
- `print()` (to stdout only — not to arbitrary file handles)
- `len()`, `range()`, `type()`, `isinstance()`
- `int()`, `float()`, `str()`, `bool()`, `list()`, `tuple()`, `dict()`, `set()`
- `abs()`, `round()`, `min()`, `max()`, `sum()`, `sorted()`, `reversed()`
- `enumerate()`, `zip()`, `map()`, `filter()`, `any()`, `all()`
- `math` module
- `json` module (pure data processing)
- `re` module (string processing)
- String methods

This provides a useful set of capabilities for data processing scripts without exposing system access.

**Future:** The restricted set should be fully customizable via an allow-list rather than a fixed preset.

---

## 6. Execution Limits

### Call depth limit

Recursion is bounded by the `WithMaxCallDepth` option (default: 500). Exceeding this raises `RecursionError`.

This is enforced by the VM's frame creation logic. It prevents stack overflow from translating into a Go goroutine stack overflow.

### Instruction count limit (Phase 2+)

```go
rt := pygo.New(pygo.WithMaxInstructions(1_000_000))
```

The VM counts executed instructions. When the limit is exceeded, a `RuntimeError` is raised. This is the primary defense against infinite loops in untrusted code.

### Memory limit (Phase 4+, best-effort)

```go
rt := pygo.New(pygo.WithMaxMemory(64 * 1024 * 1024)) // 64 MB
```

Object allocation in the VM can be tracked. When approximate usage exceeds the limit, a `MemoryError` is raised. This is best-effort because Go's GC does not expose per-allocator memory accounting. Implementation using `runtime.ReadMemStats` and polling.

---

## 7. Cancellation and Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
err := rt.RunStringContext(ctx, untrustedScript)
```

The VM checks `ctx.Done()` at:
- Loop backedges (`for`, `while`)
- Function call sites
- `yield` points (generators)

When the context is cancelled, a `KeyboardInterrupt`-equivalent exception is raised in Python. Python code can catch `KeyboardInterrupt`. If it does not re-raise it or if it handles it without ending execution, the VM will not be stopped by the context alone.

**Design consideration:** Whether `KeyboardInterrupt` should be catchable in restricted mode is a policy decision. For maximum safety, restricted mode should treat a cancelled context as a forced termination that bypasses Python exception handling. This requires a VM-level "abort" flag separate from the Python exception mechanism. This is a Phase 4+ hardening.

---

## 8. Go ↔ Python Boundary Safety

### Type safety

The interop layer validates all type conversions at the boundary. A Python script cannot cause a type assertion panic in Go by passing the wrong type — all conversions return errors rather than panicking.

### Panic recovery

All entry points from Go into Python execution (`RunString`, `RunFile`, `Eval`, `Call`) recover from unexpected Go panics:

```go
func (r *Runtime) RunString(source string) (retErr error) {
    defer func() {
        if rec := recover(); rec != nil {
            retErr = fmt.Errorf("internal error: %v\n%s", rec, debug.Stack())
        }
    }()
    // ...
}
```

This prevents a PyGo bug from crashing the host application. The host receives an error instead. This does not prevent all harm (e.g., memory corruption from unsafe code is out of scope), but it protects against logic panics.

### Callback safety

When Go calls Python (via `runtime.Call`), the same panic recovery applies. A Python exception raised in a callback is converted to a `*pygo.Error` Go error value — it does not propagate as a Go panic.

### Reflect safety

The interop layer uses `reflect`. Reflection panics (calling a method with wrong argument count, nil pointer dereference in reflect) are caught by the panic recovery layer.

---

## 9. Hash Flooding

Python dicts and sets are vulnerable to hash-flooding attacks when hash values are predictable: an attacker can craft inputs that all hash to the same bucket, degrading O(1) operations to O(n).

PyGo mitigates this by:
- Using a random per-runtime seed for string hash computation (enabled by default).
- The seed is generated at `New()` time from `crypto/rand`.
- The seed can be fixed for tests via `WithHashSeed(seed)` — this option should never be used in production.

Integer and float hashing cannot be randomized (because `hash(1) == hash(1.0)` must hold), so integer-key flooding remains theoretically possible. This is acceptable for typical use cases.

---

## 10. Resource Exhaustion

### CPU

Mitigated by: context timeout, instruction count limit (Phase 2+).

### Memory

Mitigated by: memory limit (Phase 4+, best-effort). The Go GC will eventually reclaim objects, but a fast-allocating Python loop can exhaust memory before the GC collects. Go's `debug.SetMemoryLimit` (Go 1.19+) can be set at the process level as an additional control.

### Goroutines

PyGo does not create goroutines on behalf of Python scripts (no goroutine per generator, no goroutine per thread). Goroutines are only created by explicit Go code in the host. This prevents a Python script from spawning unbounded goroutines.

### Open file descriptors

In restricted mode, `open()` is not available, so Python cannot exhaust file descriptors. In unrestricted mode, the host is responsible.

### Stack depth

Mitigated by the call depth limit.

---

## 11. Code Injection

### Python in Python (`eval`, `exec`, `compile`)

These built-ins allow Python code to compile and execute arbitrary strings as Python code. In restricted mode, these are disabled.

In unrestricted mode (trusted scripts), they are available and may be useful (e.g., for scripting frameworks that generate Python dynamically).

### Go string injection

When a Go application passes a string to `RunString` that was constructed from user input:

```go
// DANGEROUS:
rt.RunString("x = " + userInput)

// SAFE — pass as a variable, not inline code:
rt.Set("x", userInput)
rt.RunString(`print(x.upper())`)
```

This is a usage pattern issue, not a PyGo vulnerability. The documentation must warn against string-interpolating user input into Python source.

---

## 12. Known Limitations

1. **Not a sandbox.** PyGo does not provide OS-level isolation. A Python script can call any registered Go function, and registered functions can do anything Go can do.

2. **`__del__` timing.** Finalizers run non-deterministically. A Python script that relies on `__del__` for resource cleanup may leave resources open longer than expected.

3. **Memory limit is best-effort.** There is no hard memory limit in the initial implementation. The Go GC does not provide per-allocation accounting.

4. **Cancellation is cooperative.** A tight loop that does not execute a backedge instruction will not be cancellable until the next backedge. This means very tight inner loops may not respond immediately to cancellation.

5. **No inter-runtime isolation.** Two `Runtime` instances in the same process share the same Go heap. A Python script that exhausts memory in one runtime affects all runtimes in the same process.

6. **Reflection is not access-controlled.** A Go struct registered via `Register` exposes all exported fields and methods. There is no per-field access control in the initial implementation.

---

## 13. Future Sandboxing

These are post-MVP security hardening items:

- **Hard cancellation:** A VM-level abort flag that forces execution to stop regardless of Python exception handling. This would prevent a `try: while True: pass` from blocking cancellation.

- **Per-field access control:** Allow hosts to specify which fields and methods of a registered struct are accessible from Python.

- **Module allowlist:** In restricted mode, restrict imports to an explicit allowlist (not just "no OS module," but "only these specific modules").

- **Resource accounting:** Per-runtime memory and CPU accounting using Go runtime instrumentation.

- **OS-level isolation:** For the highest security, PyGo can be combined with OS-level sandboxing (e.g., running the Go binary in a container, seccomp-bpf filter, or WebAssembly). This is out of scope for PyGo itself but should be documented as the recommended approach for hostile input.

- **Wasm target:** Compiling PyGo to WebAssembly (via `GOARCH=wasm`) would provide strong OS-level isolation for browser and cloud-function use cases.

---

## 14. Responsible Disclosure

Security vulnerabilities in PyGo should be reported to the project maintainers via private disclosure before public disclosure. The project will maintain a `SECURITY.md` file at the repository root with contact information and the disclosure process.

Vulnerabilities in the interpreter itself (panic-causing inputs, memory safety issues) are treated as high priority. Vulnerabilities that only manifest in unrestricted mode against untrusted input are treated as medium priority, with documentation improvements as part of the fix.
