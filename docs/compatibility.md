# PyGo — Python 3 Compatibility Matrix

## Overview

PyGo targets **Python 3 language compatibility**, not CPython implementation compatibility.

This means:
- Language semantics and behavior are the goal.
- CPython internal APIs, the C extension ABI, and implementation-specific behavior are not goals.
- Divergences from CPython are acceptable when documented.
- Compatibility is measured by tests, not claimed by assertion.

This document categorizes every significant Python 3 language and runtime feature by implementation phase. Each feature's status is one of:

| Status | Meaning |
|--------|---------|
| **MVP** | Required for Phase 1–2. No serious use case without it. |
| **Post-MVP** | Phase 3–4. Important but not blocking initial usefulness. |
| **Future** | Phase 5–7. Valuable but complex or low priority. |
| **Complex** | Phase 7+. Technically challenging; requires substantial design work. |
| **Not Planned** | Out of scope for foreseeable future. |

---

## 1. Types and Values

| Feature | Status | Notes |
|---------|--------|-------|
| `int` (arbitrary precision) | MVP | Use `*big.Int` internally |
| `int` small-integer cache | MVP | Cache -5..256 for performance |
| `float` (IEEE 754 double) | MVP | |
| `bool` (`True`, `False`) | MVP | Subtype of `int` |
| `None` | MVP | Singleton |
| `str` (Unicode, immutable) | MVP | Go `string` (UTF-8) |
| `str` indexing → Unicode codepoints | MVP | Not bytes |
| `bytes` (immutable byte sequence) | Post-MVP | |
| `bytearray` (mutable bytes) | Future | |
| `complex` | Future | |
| `memoryview` | Not Planned | Requires buffer protocol |
| `Ellipsis` (`...`) | Future | Needed for numpy-style slicing |

---

## 2. Variables and Names

| Feature | Status | Notes |
|---------|--------|-------|
| Variable assignment (`x = 1`) | MVP | |
| Multiple assignment (`a, b = 1, 2`) | MVP | |
| Augmented assignment (`x += 1`) | MVP | |
| Annotated assignment (`x: int = 1`) | Post-MVP | Annotation stored but not enforced |
| Walrus operator (`:=`) | Post-MVP | Python 3.8+ |
| `global` statement | MVP | |
| `nonlocal` statement | MVP | Required for closures |
| `del` statement | MVP | |
| Name lookup (local → enclosing → global → builtin) | MVP | LEGB rule |

---

## 3. Operators

| Feature | Status | Notes |
|---------|--------|-------|
| Arithmetic (`+`, `-`, `*`, `/`, `//`, `%`, `**`) | MVP | |
| Bitwise (`&`, `\|`, `^`, `~`, `<<`, `>>`) | MVP | |
| Comparison (`==`, `!=`, `<`, `>`, `<=`, `>=`) | MVP | |
| Chained comparisons (`1 < x < 10`) | MVP | |
| Identity (`is`, `is not`) | MVP | |
| Membership (`in`, `not in`) | MVP | |
| Boolean (`and`, `or`, `not`) | MVP | Short-circuit evaluation |
| Matrix multiply (`@`) | Future | For numpy-compatible libraries |
| `+` / `*` on sequences | MVP | `str`, `list`, `tuple` |
| Unary (`-`, `+`, `~`) | MVP | |
| `//` floor division semantics | MVP | Must match Python (toward −∞) |
| `%` modulo semantics | MVP | Must match Python (sign follows divisor) |

---

## 4. Collections

| Feature | Status | Notes |
|---------|--------|-------|
| `list` creation, indexing, slicing | MVP | |
| `list` mutation (append, insert, remove, etc.) | MVP | |
| `tuple` creation, indexing, slicing | MVP | |
| `tuple` unpacking | MVP | |
| `dict` creation, access, mutation | MVP | |
| `dict` insertion-order preservation (3.7+) | MVP | |
| `dict` merge operators (`\|`, `\|=`) (3.9+) | Future | |
| `set` creation, operations | Post-MVP | |
| `frozenset` | Post-MVP | |
| Negative indexing | MVP | |
| Extended slicing (`a[::2]`) | Post-MVP | |
| Slice object | Post-MVP | |
| `len()`, `min()`, `max()`, `sum()` | MVP | |
| `sorted()`, `reversed()` | Post-MVP | |
| `zip()`, `enumerate()`, `map()`, `filter()` | Post-MVP | |
| `range()` | MVP | |
| Nested collection literals | MVP | |

---

## 5. Control Flow

| Feature | Status | Notes |
|---------|--------|-------|
| `if` / `elif` / `else` | MVP | |
| `while` / `else` | MVP | |
| `for` / `else` | MVP | |
| `break`, `continue` | MVP | |
| `pass` | MVP | |
| `for` over iterables | MVP | |
| `for` over `range` | MVP | |
| Ternary expression (`x if c else y`) | MVP | |
| `match` / `case` (structural pattern matching, 3.10+) | Complex | Non-trivial compiler work |

---

## 6. Functions

| Feature | Status | Notes |
|---------|--------|-------|
| Function definition (`def`) | MVP | |
| Positional arguments | MVP | |
| Default argument values | MVP | |
| `*args` (variadic positional) | MVP | |
| `**kwargs` (variadic keyword) | MVP | |
| Keyword-only arguments (after `*`) | Post-MVP | |
| Positional-only arguments (`/` separator, 3.8+) | Future | |
| Return value | MVP | |
| `return` without value (→ `None`) | MVP | |
| Nested functions | MVP | |
| Closures (`nonlocal`) | MVP | |
| Lambda expressions | Post-MVP | |
| Annotations (type hints) | Post-MVP | Stored, not enforced |
| Recursive functions | MVP | |
| Mutual recursion | MVP | |
| First-class functions | MVP | |

---

## 7. Exceptions

| Feature | Status | Notes |
|---------|--------|-------|
| `raise` statement | MVP | |
| `raise` without args (re-raise) | MVP | |
| `try` / `except` / `else` / `finally` | MVP | |
| Multiple `except` clauses | MVP | |
| `except ExcType as e` | MVP | |
| Exception chaining (`raise X from Y`) | Post-MVP | |
| Exception groups (3.11+, `ExceptionGroup`) | Complex | New in Python 3.11 |
| Built-in exception hierarchy | MVP | See object-model.md |
| Custom exception classes | Post-MVP | Requires class support |
| `sys.exc_info()` | Post-MVP | |
| Traceback objects | MVP | Needed for error reporting |

---

## 8. Classes

| Feature | Status | Notes |
|---------|--------|-------|
| Class definition (`class`) | Post-MVP | Phase 3 |
| Instance creation (`MyClass()`) | Post-MVP | |
| Instance attributes | Post-MVP | |
| Instance methods | Post-MVP | |
| `self` parameter | Post-MVP | |
| `__init__` | Post-MVP | |
| `__str__`, `__repr__` | Post-MVP | |
| Single inheritance | Post-MVP | |
| Multiple inheritance | Post-MVP | Requires C3 MRO |
| `super()` | Post-MVP | |
| Class attributes | Post-MVP | |
| Class methods (`@classmethod`) | Future | |
| Static methods (`@staticmethod`) | Future | |
| `property` descriptor | Future | |
| `__slots__` | Future | Memory optimization |
| `__del__` | Future | Non-deterministic with Go GC |
| Operator overloading (`__add__`, etc.) | Post-MVP | |
| Comparison overloading (`__eq__`, etc.) | Post-MVP | |
| `__hash__` | Post-MVP | |
| `__bool__` | Post-MVP | |
| `__len__`, `__getitem__` | Post-MVP | |
| `__iter__`, `__next__` | Post-MVP | |
| `__enter__`, `__exit__` | Future | Context managers |
| `__getattr__`, `__setattr__` | Future | |
| `__get__`, `__set__` (descriptors) | Complex | |
| Metaclasses | Complex | |
| `__init_subclass__` | Complex | |
| `__class_getitem__` | Complex | For generics |
| `dataclasses` module | Future | Pure Python possible |
| Abstract base classes (`abc`) | Complex | |

---

## 9. Imports and Modules

| Feature | Status | Notes |
|---------|--------|-------|
| `import module` | MVP | |
| `from module import name` | MVP | |
| `from module import *` | Post-MVP | |
| `import module as alias` | Post-MVP | |
| `from module import name as alias` | Post-MVP | |
| Relative imports (`from . import`) | Future | |
| `__init__.py` packages | Future | |
| `sys.modules` cache | MVP | |
| Module `__name__`, `__file__` | MVP | |
| Circular import detection | Post-MVP | |
| Dynamic import (`importlib`) | Complex | |
| Namespace packages (PEP 420) | Not Planned | |

---

## 10. Built-in Functions

| Function | Status | Notes |
|----------|--------|-------|
| `print()` | MVP | |
| `len()` | MVP | |
| `range()` | MVP | |
| `type()` | MVP | |
| `isinstance()` | Post-MVP | Requires class support |
| `issubclass()` | Post-MVP | |
| `int()`, `float()`, `str()`, `bool()` | MVP | Type constructors / conversions |
| `list()`, `tuple()`, `dict()`, `set()` | Post-MVP | |
| `input()` | Post-MVP | CLI/REPL use |
| `abs()`, `round()`, `divmod()`, `pow()` | MVP | |
| `min()`, `max()`, `sum()` | MVP | |
| `sorted()` | Post-MVP | |
| `reversed()` | Post-MVP | |
| `enumerate()` | Post-MVP | |
| `zip()` | Post-MVP | |
| `map()`, `filter()` | Post-MVP | |
| `any()`, `all()` | Post-MVP | |
| `id()` | Post-MVP | |
| `hash()` | Post-MVP | |
| `repr()` | MVP | |
| `iter()`, `next()` | Post-MVP | |
| `callable()` | Post-MVP | |
| `hasattr()`, `getattr()`, `setattr()`, `delattr()` | Post-MVP | |
| `vars()`, `dir()` | Future | |
| `open()` | Future | File I/O |
| `exec()`, `eval()` | Future | Dynamic execution |
| `compile()` | Future | |
| `globals()`, `locals()` | Future | |
| `super()` | Post-MVP | |
| `object()` | Post-MVP | |
| `staticmethod()`, `classmethod()` | Future | |
| `property()` | Future | |
| `chr()`, `ord()` | Post-MVP | |
| `hex()`, `oct()`, `bin()` | Post-MVP | |
| `format()` | Post-MVP | |
| `bytes()`, `bytearray()` | Future | |
| `memoryview()` | Not Planned | |
| `frozenset()` | Future | |
| `slice()` | Future | |
| `NotImplemented` | Post-MVP | For operator protocols |
| `__import__()` | Future | |

---

## 11. String Features

| Feature | Status | Notes |
|---------|--------|-------|
| String literals (single, double, triple-quoted) | MVP | |
| String concatenation, repetition | MVP | |
| String methods (`.upper()`, `.lower()`, `.split()`, etc.) | MVP | ~40 methods |
| f-strings (basic `f"hello {name}"`) | MVP | |
| f-strings (nested expressions) | Post-MVP | Complex lexer work |
| f-strings (conversion flags `!r`, `!s`, `!a`) | Post-MVP | |
| f-strings (format spec `:.2f`) | Post-MVP | |
| Raw strings (`r"..."`) | MVP | |
| Byte strings (`b"..."`) | Post-MVP | |
| String formatting (`.format()`) | Post-MVP | |
| `%` formatting | Future | Legacy but common |
| `str.encode()` / `bytes.decode()` | Future | Encoding support |
| Unicode normalization | Not Planned | |

---

## 12. Iterators and Generators

| Feature | Status | Notes |
|---------|--------|-------|
| `for` loop over iterables | MVP | |
| Custom `__iter__` / `__next__` | Post-MVP | |
| `StopIteration` | MVP | |
| Generator functions (`yield`) | Post-MVP | Phase 2+ |
| Generator expressions | Post-MVP | |
| `yield from` (delegation) | Future | |
| `send()` to generators | Future | |
| `throw()` / `close()` on generators | Future | |
| `iter()` built-in | Post-MVP | |
| `next()` built-in | Post-MVP | |

---

## 13. Comprehensions

| Feature | Status | Notes |
|---------|--------|-------|
| List comprehensions | Post-MVP | |
| Dict comprehensions | Post-MVP | |
| Set comprehensions | Post-MVP | |
| Generator expressions | Post-MVP | |
| Nested comprehensions | Post-MVP | |
| Multiple `for` clauses in comprehension | Post-MVP | |
| `if` filter in comprehension | Post-MVP | |
| Walrus in comprehension | Complex | |

---

## 14. Decorators

| Feature | Status | Notes |
|---------|--------|-------|
| Function decorators (`@decorator`) | Post-MVP | |
| Decorator with arguments (`@decorator(arg)`) | Post-MVP | |
| Class decorators | Future | |
| Stacked decorators | Post-MVP | |
| `functools.wraps` | Future | |

---

## 15. Context Managers

| Feature | Status | Notes |
|---------|--------|-------|
| `with` statement | Future | Phase 3+ |
| `__enter__` / `__exit__` protocol | Future | |
| Multiple context managers in one `with` | Future | |
| `contextlib.contextmanager` | Future | |

---

## 16. Closures

| Feature | Status | Notes |
|---------|--------|-------|
| Closure variable capture | MVP | |
| `nonlocal` binding | MVP | |
| Mutable closure variables | MVP | |
| Closure over loop variable | MVP | Must match Python semantics (late binding) |
| Nested closures | Post-MVP | |

---

## 17. Async / Await

| Feature | Status | Notes |
|---------|--------|-------|
| `async def` | Complex | Phase 6+ |
| `await` expression | Complex | |
| `async for` | Complex | |
| `async with` | Complex | |
| `asyncio` event loop | Complex | |
| Coroutine protocol | Complex | |

---

## 18. Descriptors and Properties

| Feature | Status | Notes |
|---------|--------|-------|
| `property` | Future | |
| `classmethod` | Future | |
| `staticmethod` | Future | |
| Custom descriptors (`__get__`, `__set__`, `__delete__`) | Complex | |

---

## 19. Metaclasses

| Feature | Status | Notes |
|---------|--------|-------|
| Default metaclass (`type`) | Post-MVP | |
| Custom metaclass (`metaclass=Meta`) | Complex | |
| `__init_subclass__` | Complex | |
| `__prepare__` | Complex | |
| `abc.ABCMeta` | Complex | |

---

## 20. Standard Library Modules

| Module | Status | Implementation |
|--------|--------|---------------|
| `builtins` | MVP | Go |
| `sys` (subset) | MVP | Go |
| `math` | MVP | Go (wraps `math` package) |
| `os` (subset: path, environ, getcwd) | Future | Go |
| `os.path` | Future | Go |
| `pathlib` | Future | Go |
| `json` | Post-MVP | Go (wraps `encoding/json`) |
| `re` (subset) | Future | Go (wraps `regexp`) — syntax differences apply |
| `time` (subset) | Future | Go |
| `datetime` | Future | Go |
| `io` (subset) | Future | Go |
| `sys` (full) | Future | Go |
| `collections` (deque, OrderedDict, Counter) | Future | Go / Pure Python |
| `itertools` | Future | Go |
| `functools` (partial, reduce, wraps) | Future | Go / Pure Python |
| `typing` (subset) | Future | Pure Python stubs |
| `dataclasses` | Future | Depends on class support |
| `abc` | Complex | |
| `threading` | Complex | Would need Go goroutine model |
| `multiprocessing` | Not Planned | |
| `asyncio` | Complex | |
| `subprocess` | Future | Controlled host permission |
| `socket` | Future | Controlled host permission |
| `http` | Not Planned | |
| `xml` / `html` | Not Planned | |
| `csv` | Future | Pure Python possible |
| `hashlib` | Future | Go (`crypto`) |
| `base64` | Future | Go (`encoding/base64`) |
| `struct` (binary packing) | Future | |
| `ctypes` | Not Planned | Requires CPython C API |
| `cffi` | Not Planned | Requires CPython C API |
| `importlib` | Complex | |

---

## 21. Divergences from CPython (Documented)

These are intentional divergences that are acceptable for PyGo:

| Area | CPython behavior | PyGo behavior |
|------|-----------------|--------------|
| `id(x)` | Memory address | Opaque unique integer per object |
| Hash randomization | Enabled by default (PYTHONHASHSEED) | Enabled; seed set at runtime creation |
| `__del__` timing | Called when reference count hits 0 | Called at GC collection time (non-deterministic) |
| `sys.getrefcount()` | Reference count | Not implemented (returns 0 or absent) |
| Integer representation | CPython `int` object with internal limb array | `*big.Int` |
| `float` `repr()` | Shortest round-trip representation | Go `strconv.FormatFloat` (matches for most values) |
| C extension modules | Supported via CPython ABI | Not supported |
| `__slots__` | Memory optimization | Functional but no memory benefit (future) |
| Recursion limit | Default 1000 (`sys.setrecursionlimit`) | Default 500, configurable |
| `threading` module | OS threads with GIL | Not implemented in MVP |
| `ctypes` / `cffi` | Supported | Not planned |
| String interning | Implementation detail | More aggressive interning acceptable |

---

## 22. Compatibility Testing Strategy

Every feature in the MVP and Post-MVP column must have:

1. A Go unit test in the corresponding `internal/` package.
2. A Python script in `testdata/compat/` that can be run against both PyGo and CPython to verify identical behavior.
3. A note in this document if behavior intentionally differs from CPython.

The `testdata/compat/` directory is the source of truth for cross-reference testing. CPython is used only as a **reference for tests**, not as a runtime dependency.

See `docs/testing.md` for the complete testing strategy.
