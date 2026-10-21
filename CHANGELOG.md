# Changelog

All notable changes to PyGo are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). PyGo has not yet cut
a tagged release; everything below is on the `main` branch (`Unreleased`).

## [Unreleased]

### Language

- Core interpreter: variables, arbitrary-precision `int`, `float`, `bool`,
  `None`, all arithmetic/bitwise/comparison operators, chained comparisons,
  boolean short-circuiting.
- Control flow: `if`/`elif`/`else`, `while`, `for`, `break`/`continue`,
  `pass`, ternary expressions, `assert`.
- Functions: defaults, `*args`, `**kwargs`, closures (`nonlocal`), lambdas,
  recursion, decorators.
- Data types: `str` (with ~20 methods, slicing, f-strings), `bytes`, `list`
  (with methods), `tuple`, `dict`, `set`; comprehensions and generator
  expressions; generators (`yield`).
- Classes: single and multiple inheritance (C3 MRO), the full dunder protocol
  (arithmetic, comparison, container, iteration, `__call__`, `__bool__`,
  `__hash__`, `__str__`/`__repr__`), `@property`/`@classmethod`/`@staticmethod`,
  and `super()` (zero-arg and explicit).
- Exceptions: `try`/`except`/`finally`, `raise`, custom exception classes,
  built-in exception hierarchy, and exception-safe `with` statements.

### Standard library

- `math`, `json`, `re`, `random`, `sys`, `string`, `functools`, `itertools`,
  `os` (+ `os.path`), `time`, `collections` (Counter, defaultdict,
  OrderedDict, deque, namedtuple), `datetime`, `hashlib`, and `base64`.
- Import system: `import`, `from … import`, `import … as`, module caching,
  `.py` modules on the search path, and host-registered modules.

### Embedding (Go API)

- `RunString`, `RunFile`, `Eval`, `Set`, `Get`, `Delete`.
- `RegisterFunction`, `Register` (structs/methods/fields), `RegisterModule`.
- `Call`, `CallByName`, `CallMethod`; Go `error` ↔ Python exception mapping.
- Go ↔ Python value conversion, including `[]byte` ↔ `bytes`.

### CLI

- `pygo script.py`, `pygo -c "…"`, and an interactive REPL.

### Hardening

- Enforceable execution limits: instruction count, best-effort memory,
  recursion depth, and `context`-based timeout/cancellation. Instruction,
  memory, and cancellation limits are uncatchable aborts.
- `WithRestrictedBuiltins()` for reduced-capability execution; `os` and other
  OS-capable modules are blocked in restricted mode.

### Not yet implemented

`async`/`await`, the `match` statement, lazy generator expressions, generator
`.send()`/`.throw()`, `bytearray`, custom descriptors, metaclasses, and most
of the remaining standard library.
