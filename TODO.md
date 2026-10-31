# TODO

Known gaps, roughly in priority order. Tick items off as they land and move
the user-facing note to `CHANGELOG.md`.

## Builtins parity
- [x] `object`, `issubclass`, `callable`, `zip`, `map`, `filter`, `divmod`,
      `pow`, `slice`, `bin`, `hex`, `oct`, `id`, `delattr`, `set` comparison
- [ ] `frozenset` (needs its own object type), `complex`
- [ ] `format`
- [ ] `dir`, `vars`, `globals`, `locals`
- [ ] `input`, `open` (need a file object and restricted-mode handling)
- [ ] Set operators (`|`, `&`, `-`, `^`) and set methods: check coverage

## Correctness found during benchmarking
- [ ] Remaining `fmt.Errorf("TypeError: ...")` sites should build real
      exceptions directly (they are converted at the VM boundary today)
- [ ] `**kwargs` key order: the call machinery passes a Go map, so order is
      not the caller's order (CPython preserves it)

## Lexer and diagnostics
- [ ] Token positions are end positions; report start line/column
- [ ] Compile errors should include the file name and column
- [ ] `ZeroDivisionError` message wording (`division by zero`)

## Language
- [ ] `async` / `await`
- [ ] `bytearray`
- [ ] Custom descriptors (`__get__` / `__set__`) and metaclasses
- [ ] Complex-number literals (also in `match` patterns)
- [ ] `match`: sequence/mapping patterns for user subclasses of `list`/`dict`
- [ ] `str.format` keyword fields
- [ ] Generator return value via `StopIteration.value`
- [ ] Comprehension targets leak into the enclosing scope
- [ ] `except ... as e` should delete `e` afterwards

## Standard library
- [ ] `io`, `csv`, `struct`
- [ ] `hashlib` `copy()`
- [ ] `isinstance` against stdlib factory types is unreliable

## Performance
- [x] Benchmark suite (`BENCHMARKS.md`)
- [x] First optimisation pass: unboxed ints, fast paths, frame pooling,
      allocation-free lookups, lazy `range` (2.7× geomean)
- [ ] Per-call-site attribute/method caches; skip bound-method allocation for
      `obj.method(...)` (needs a LOAD_METHOD/CALL_METHOD pair)
- [ ] Avoid allocating a builtin wrapper for every `list.append` etc. access
- [ ] Run benchmarks on a schedule and track regressions

## Docs
- [ ] Align `docs/cli.md` and `docs/architecture.md` with the implementation
      (readline, `internal/interop`, `go.sum`)
