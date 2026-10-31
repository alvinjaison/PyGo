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
  (with methods), `tuple`, `dict`, `set`; comprehensions; lazy generator
  expressions; generators (`yield`) with `send()`, `throw()`, and `close()`.
- Classes: single and multiple inheritance (C3 MRO), the full dunder protocol
  (arithmetic, comparison, container, iteration, `__call__`, `__bool__`,
  `__hash__`, `__str__`/`__repr__`), `@property`/`@classmethod`/`@staticmethod`,
  and `super()` (zero-arg and explicit).
- Structural pattern matching: the `match` statement with literal, capture,
  wildcard, value (`Color.RED`), `|`, `as`, sequence (`[a, *rest]`), mapping
  (`{"k": v, **rest}`) and class (`Point(x, y=0)`) patterns, guards, and
  `__match_args__`. Built-in types (`int()`, `str() as s`, `dict()`) and
  `collections.namedtuple` classes work as class patterns. `match` and `case`
  stay ordinary identifiers everywhere else.
- Star-unpacking assignment: `first, *rest = items`, `*init, last = items`.
- `dict(...)` and `set(...)` constructors.
- Builtins: `object` (so `class A(object):` works), `issubclass`, `callable`,
  lazy `zip`/`map`/`filter`, `divmod`, `pow` (with modulus), `bin`/`oct`/`hex`,
  `id`, `delattr`, and `slice`.
- `set` equality and subset/superset comparisons (`==`, `<=`, `<`, `>=`, `>`).
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

### Fixed

- `finally` now runs when an exception propagates out of a `try`, and on
  `return`, `break`, and `continue`. Previously it only ran when an `except`
  clause had handled the exception.
- A `try` statement's `else` clause no longer runs after an exception was
  handled.
- `with` now calls `__exit__` when the body exits via `return`, `break`, or
  `continue`.
- `break` out of a `for` loop no longer leaks the loop iterator on the value
  stack. Enough of them used to overflow it and crash the VM.
- `lambda` can now close over variables of an enclosing function.
- Floats print like CPython: `3.0`, `10000000000.0`, `1e+16`, `1e-05`.
  Whole-number floats used to print without the `.0`.
- Tracebacks report the line that raised. They used to show only the first
  statement's line of each function, because the line table was never attached
  to the compiled code.
- Syntax errors report the correct physical line. The lexer counted every
  newline twice, so a failure on line 4 was reported as line 7. A statement's
  final `NEWLINE` token is also attributed to its own line, not the next one.
- `nonlocal x` now works when the nested function only assigns `x` (the classic
  counter closure). The enclosing function did not keep `x` in a cell, so the
  call failed with "free variable referenced before assignment".
- `int`, `str`, `float`, `bool`, `list`, `tuple` and `bytes` are now usable as
  types: `isinstance(1, int)` and `type(x) == int` work. Previously
  `isinstance` rejected them with a `TypeError`.
- Unary `-` works on `float` and `bool`, unary `+` rejects non-numbers, and
  bad operands raise a Python `TypeError` instead of a bare Go error.
- `any()` and `all()` short-circuit, so they work on infinite generators.
- Errors are classified accurately at the public API: a script that fails to
  parse is a `SyntaxError`, a recovered interpreter panic is an `InternalError`,
  and a missing file is returned as the underlying I/O error. Previously all of
  these were reported as `SyntaxError`.

### Not yet implemented

`async`/`await`, `bytearray`, custom descriptors,
metaclasses, and most of the remaining standard library.
