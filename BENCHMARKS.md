# Benchmarks

The benchmark suite lives in `internal/runtime/bench_test.go`. Each benchmark
runs a complete Python script on a fresh runtime, so the numbers include
parsing and compilation, but each script is sized so execution dominates.

```sh
go test -run '^$' -bench . -benchmem -count=6 ./internal/runtime | tee new.txt
```

Compare two runs with [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat):

```sh
benchstat old.txt new.txt
```

Benchmarks are not run in CI; timings on shared runners are too noisy to gate
on. Run them before and after any change to the VM, compiler or object model,
and include the comparison in the pull request.

| Benchmark | What it exercises |
|---|---|
| `FibRecursive` | Python function calls and recursion |
| `IntLoop` | Module-level `while` loop (global name lookups) |
| `IntLoopLocals` | The same loop in a function (fast locals) |
| `FloatMath` | Float arithmetic |
| `ListAppendAndIndex` | `list.append` and indexing |
| `ListComprehension` | Comprehension with a filter |
| `DictOps` | Dict insert and lookup with int keys |
| `StringBuild` | `str()` conversion, concatenation and `join` |
| `MethodCalls` | Method calls and attribute updates on an instance |
| `Generator` | Generator iteration |
| `MatchStatement` | `match` with literal, sequence, mapping and class patterns |
| `TryExcept` | Raising and catching exceptions |

## Baseline

Apple M2 Pro, Go 1.25.1, macOS. Medians of three runs. "Before" is commit
`32995ae`; "after" is the VM optimisation work that followed it.

| Benchmark | Before | After | Speedup | Allocs/op before | Allocs/op after |
|---|---:|---:|---:|---:|---:|
| `FibRecursive` | 23.5 ms | 9.4 ms | 2.5× | 315,778 | 57,660 |
| `IntLoop` | 50.1 ms | 22.6 ms | 2.2× | 1,499,059 | 399,777 |
| `IntLoopLocals` | 35.2 ms | 10.9 ms | 3.2× | 1,099,120 | 299,802 |
| `FloatMath` | 15.2 ms | 6.4 ms | 2.4× | 499,563 | 299,959 |
| `ListAppendAndIndex` | 16.4 ms | 8.3 ms | 2.0× | 418,809 | 209,729 |
| `ListComprehension` | 11.6 ms | 4.5 ms | 2.6× | 249,562 | 49,974 |
| `DictOps` | 14.9 ms | 6.7 ms | 2.2× | 298,582 | 79,754 |
| `StringBuild` | 8.6 ms | 4.2 ms | 2.0× | 179,737 | 90,005 |
| `MethodCalls` | 54.2 ms | 8.3 ms | 6.5× | 1,319,825 | 120,103 |
| `Generator` | 8.8 ms | 3.9 ms | 2.3× | 219,183 | 59,870 |
| `MatchStatement` | 20.1 ms | 7.0 ms | 2.9× | 248,371 | 84,642 |
| `TryExcept` | 8.6 ms | 2.7 ms | 3.2× | 174,087 | 39,813 |

Geometric-mean speedup: **2.67×**.

## What changed

- **Unboxed integers.** Ints that fit in 64 bits are stored inline instead of
  as a `*big.Int`, so creating one is a single small allocation, and
  arithmetic, comparison, hashing and `str()` skip `math/big` entirely.
  Overflow falls back to arbitrary precision.
- **Fast paths** for `int`/`float` arithmetic and `int`/`float`/`str`
  comparison, ahead of the general slot and dunder dispatch.
- **Frame pooling.** Frames of finished calls are recycled, and a frame's
  locals and value stack share one allocation.
- **Allocation-free name lookup.** Globals, builtins, class attributes and
  instance attributes are looked up by Go string without building a
  `StrObject` or allocating a `KeyError` on a miss. String hashes are cached.
- **Leaner dicts.** The hash index chains entries instead of holding a slice
  per hash.
- **Lazy `range`.** `range()` returns a range object instead of materialising
  a list.
- **Cheaper limit checks.** The context is polled every 256 instructions; the
  instruction limit is still exact.

## Where the time goes now

With GC disabled, most remaining time is allocation of short-lived ints and
floats. The next steps are caching attribute/method lookups per call site,
avoiding a bound-method object for `obj.method(...)` calls, and a
register-style calling convention that avoids copying arguments.
