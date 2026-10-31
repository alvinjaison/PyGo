package runtime

import (
	"bytes"
	"context"
	"testing"
)

// Benchmarks for the interpreter's main workload shapes. Each runs a complete
// script on a fresh runtime, sized so execution dominates parse/compile time.
//
//	go test -run '^$' -bench . -benchmem ./internal/runtime
//
// Compare two runs with benchstat (golang.org/x/perf/cmd/benchstat).

func benchScript(b *testing.B, src string) {
	b.Helper()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rt := New(Config{Stdout: &bytes.Buffer{}})
		if err := rt.RunString(context.Background(), src, "<bench>"); err != nil {
			b.Fatalf("script failed: %v", err)
		}
	}
}

// Function calls and recursion.
func BenchmarkFibRecursive(b *testing.B) {
	benchScript(b, `def fib(n):
    if n < 2:
        return n
    return fib(n - 1) + fib(n - 2)
fib(22)
`)
}

// Tight integer loop: LOAD/STORE, BINARY_ADD, COMPARE, jumps.
func BenchmarkIntLoop(b *testing.B) {
	benchScript(b, `total = 0
i = 0
while i < 100000:
    total = total + i * 2
    i = i + 1
`)
}

// The same loop inside a function, where locals are fast variables.
func BenchmarkIntLoopLocals(b *testing.B) {
	benchScript(b, `def run():
    total = 0
    for i in range(100000):
        total += i * 2
    return total
run()
`)
}

func BenchmarkFloatMath(b *testing.B) {
	benchScript(b, `def run():
    x = 0.0
    for i in range(50000):
        x = x * 0.5 + i / 3.0
    return x
run()
`)
}

func BenchmarkListAppendAndIndex(b *testing.B) {
	benchScript(b, `def run():
    xs = []
    for i in range(30000):
        xs.append(i)
    total = 0
    for i in range(30000):
        total += xs[i]
    return total
run()
`)
}

func BenchmarkListComprehension(b *testing.B) {
	benchScript(b, `def run():
    return sum([x * x for x in range(30000) if x % 3 == 0])
run()
`)
}

func BenchmarkDictOps(b *testing.B) {
	benchScript(b, `def run():
    d = {}
    for i in range(20000):
        d[i] = i * 2
    total = 0
    for i in range(20000):
        total += d[i]
    return total
run()
`)
}

func BenchmarkStringBuild(b *testing.B) {
	benchScript(b, `def run():
    parts = []
    for i in range(10000):
        parts.append("item" + str(i))
    return len(",".join(parts))
run()
`)
}

func BenchmarkMethodCalls(b *testing.B) {
	benchScript(b, `class Counter:
    def __init__(self):
        self.n = 0
    def inc(self, k):
        self.n += k
        return self.n
def run():
    c = Counter()
    for i in range(20000):
        c.inc(i)
    return c.n
run()
`)
}

func BenchmarkGenerator(b *testing.B) {
	benchScript(b, `def gen(n):
    for i in range(n):
        yield i * 2
def run():
    total = 0
    for v in gen(20000):
        total += v
    return total
run()
`)
}

func BenchmarkMatchStatement(b *testing.B) {
	benchScript(b, `def classify(x):
    match x:
        case 0:
            return 0
        case [a, b]:
            return a + b
        case {"k": v}:
            return v
        case int() as n:
            return n
        case _:
            return -1
def run():
    total = 0
    for i in range(5000):
        total += classify(i) + classify([i, 1]) + classify({"k": i})
    return total
run()
`)
}

func BenchmarkTryExcept(b *testing.B) {
	benchScript(b, `def run():
    n = 0
    for i in range(10000):
        try:
            if i % 2:
                raise ValueError("x")
            n += 1
        except ValueError:
            n += 2
    return n
run()
`)
}
