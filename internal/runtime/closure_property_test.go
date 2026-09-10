package runtime

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// ============================================================
// Feature: pygo-post-mvp, Property 1: Cell Variable Round-Trip
// ============================================================
// **Validates: Requirements 1.2, 1.3**
//
// For any value stored into a cell object via STORE_DEREF, subsequently
// executing LOAD_DEREF on the same cell SHALL produce that same value.
//
// We test this by creating a closure that captures a variable, stores a
// generated value via nonlocal assignment (STORE_DEREF), then reads it
// back (LOAD_DEREF) and verifies identity.

func TestProperty1_CellVariableRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random integer value to store in the cell
		val := rapid.IntRange(-1000, 1000).Draw(t, "val")

		// Python source that:
		// 1. Creates a cell variable in the enclosing scope
		// 2. Nested function writes to it via STORE_DEREF (nonlocal)
		// 3. Nested function reads it via LOAD_DEREF
		// 4. Verifies the read value matches the stored value
		source := fmt.Sprintf(`
def outer():
    x = 0
    def setter():
        nonlocal x
        x = %d
    def getter():
        return x
    setter()
    return getter()

result = outer()
print(result)
`, val)

		var buf bytes.Buffer
		r := New(WithOutput(&buf))
		err := r.ExecString(source, "<test>")
		if err != nil {
			t.Fatalf("execution failed: %v\nsource:\n%s", err, source)
		}

		output := strings.TrimSpace(buf.String())
		expected := fmt.Sprintf("%d", val)
		if output != expected {
			t.Fatalf("cell round-trip failed: got %q, want %q\nsource:\n%s", output, expected, source)
		}
	})
}

// ============================================================
// Feature: pygo-post-mvp, Property 2: LEGB Resolution Order
// ============================================================
// **Validates: Requirements 1.4**
//
// For any name that exists at multiple scope levels (Local, Enclosing,
// Global, Builtin), name resolution SHALL always return the value from
// the innermost scope where the name is defined, following
// Local -> Enclosing -> Global -> Builtin order.

func TestProperty2_LEGBResolutionOrder(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate distinct values for each scope level
		globalVal := rapid.IntRange(1000, 1999).Draw(t, "globalVal")
		enclosingVal := rapid.IntRange(2000, 2999).Draw(t, "enclosingVal")
		localVal := rapid.IntRange(3000, 3999).Draw(t, "localVal")

		// Choose which level to test: "local", "enclosing", or "global"
		levels := []string{"local", "enclosing", "global"}
		levelIdx := rapid.IntRange(0, len(levels)-1).Draw(t, "levelIdx")
		level := levels[levelIdx]

		var source string
		var expected int

		switch level {
		case "local":
			// Local shadows enclosing and global
			source = fmt.Sprintf(`
x = %d
def outer():
    x = %d
    def inner():
        x = %d
        return x
    return inner()
result = outer()
print(result)
`, globalVal, enclosingVal, localVal)
			expected = localVal

		case "enclosing":
			// Enclosing shadows global (no local assignment)
			source = fmt.Sprintf(`
x = %d
def outer():
    x = %d
    def inner():
        return x
    return inner()
result = outer()
print(result)
`, globalVal, enclosingVal)
			expected = enclosingVal

		case "global":
			// Global is used when no enclosing or local defines the name
			source = fmt.Sprintf(`
x = %d
def outer():
    def inner():
        global x
        return x
    return inner()
result = outer()
print(result)
`, globalVal)
			expected = globalVal
		}

		var buf bytes.Buffer
		r := New(WithOutput(&buf))
		err := r.ExecString(source, "<test>")
		if err != nil {
			t.Fatalf("execution failed for level=%s: %v\nsource:\n%s", level, err, source)
		}

		output := strings.TrimSpace(buf.String())
		expectedStr := fmt.Sprintf("%d", expected)
		if output != expectedStr {
			t.Fatalf("LEGB resolution failed for level=%s: got %q, want %q\nsource:\n%s",
				level, output, expectedStr, source)
		}
	})
}

// ============================================================
// Feature: pygo-post-mvp, Property 3: Cell Sharing Across Closures
// ============================================================
// **Validates: Requirements 1.5**
//
// For any value written to a shared cell variable by one closure, all
// other closures sharing that same cell SHALL observe the updated value
// on their next read.

func TestProperty3_CellSharingAcrossClosures(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate two values: initial and the mutated value
		initialVal := rapid.IntRange(0, 499).Draw(t, "initialVal")
		mutatedVal := rapid.IntRange(500, 999).Draw(t, "mutatedVal")

		// Python source:
		// - outer() creates a shared cell variable
		// - writer() mutates the cell
		// - reader() reads the cell
		// - After writer() modifies the cell, reader() should see the new value
		source := fmt.Sprintf(`
def outer():
    shared = %d
    def writer():
        nonlocal shared
        shared = %d
    def reader():
        return shared
    writer()
    return reader()

result = outer()
print(result)
`, initialVal, mutatedVal)

		var buf bytes.Buffer
		r := New(WithOutput(&buf))
		err := r.ExecString(source, "<test>")
		if err != nil {
			t.Fatalf("execution failed: %v\nsource:\n%s", err, source)
		}

		output := strings.TrimSpace(buf.String())
		expected := fmt.Sprintf("%d", mutatedVal)
		if output != expected {
			t.Fatalf("cell sharing failed: got %q, want %q (initial=%d, mutated=%d)\nsource:\n%s",
				output, expected, initialVal, mutatedVal, source)
		}
	})
}

// ============================================================
// Feature: pygo-post-mvp, Property 4: Closure Lifetime Preservation
// ============================================================
// **Validates: Requirements 1.6**
//
// For any closure returned from a function that has completed execution,
// calling that closure SHALL still be able to access and mutate the
// captured cell variables.

func TestProperty4_ClosureLifetimePreservation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate an initial value and an increment
		initialVal := rapid.IntRange(0, 100).Draw(t, "initialVal")
		increment := rapid.IntRange(1, 50).Draw(t, "increment")
		// Number of times to call the closure after the enclosing function returns
		numCalls := rapid.IntRange(1, 5).Draw(t, "numCalls")

		// Python source:
		// - make_counter() returns a closure that increments a captured variable
		// - After make_counter() returns, calling the closure repeatedly should
		//   still work and accumulate state
		source := fmt.Sprintf(`
def make_counter():
    count = %d
    def increment():
        nonlocal count
        count = count + %d
        return count
    return increment

counter = make_counter()
results = []
for i in range(%d):
    results.append(counter())
print(results[-1])
`, initialVal, increment, numCalls)

		var buf bytes.Buffer
		r := New(WithOutput(&buf))
		err := r.ExecString(source, "<test>")
		if err != nil {
			t.Fatalf("execution failed: %v\nsource:\n%s", err, source)
		}

		output := strings.TrimSpace(buf.String())
		// After numCalls, the value should be initialVal + increment * numCalls
		expected := fmt.Sprintf("%d", initialVal+increment*numCalls)
		if output != expected {
			t.Fatalf("closure lifetime failed: got %q, want %q (initial=%d, inc=%d, calls=%d)\nsource:\n%s",
				output, expected, initialVal, increment, numCalls, source)
		}
	})
}

// ============================================================
// Feature: pygo-post-mvp, Property 5: Nonlocal Binding to Nearest Enclosing Scope
// ============================================================
// **Validates: Requirements 1.7, 1.8**
//
// For any nonlocal declaration in a nested function, the compiler SHALL
// bind it to the nearest enclosing function scope that defines that name,
// and the resulting cell/free variable pair SHALL correctly share state.

func TestProperty5_NonlocalBindingToNearestEnclosingScope(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate values for two enclosing levels
		outerVal := rapid.IntRange(100, 199).Draw(t, "outerVal")
		middleVal := rapid.IntRange(200, 299).Draw(t, "middleVal")
		newVal := rapid.IntRange(300, 399).Draw(t, "newVal")

		// Python source:
		// - outer() defines x
		// - middle() also defines x (shadows outer's x)
		// - inner() uses nonlocal x -> should bind to middle's x (nearest)
		// - After inner() modifies x, middle's x should be changed
		// - outer's x should remain unchanged
		// We use two separate calls to verify each level independently.
		source := fmt.Sprintf(`
def outer():
    x = %d
    def middle():
        x = %d
        def inner():
            nonlocal x
            x = %d
        inner()
        return x
    middle_result = middle()
    print(x)
    print(middle_result)

outer()
`, outerVal, middleVal, newVal)

		var buf bytes.Buffer
		r := New(WithOutput(&buf))
		err := r.ExecString(source, "<test>")
		if err != nil {
			t.Fatalf("execution failed: %v\nsource:\n%s", err, source)
		}

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 output lines, got %d: %q\nsource:\n%s",
				len(lines), buf.String(), source)
		}

		// outer's x should remain unchanged (nonlocal binds to nearest = middle)
		expectedOuter := fmt.Sprintf("%d", outerVal)
		if strings.TrimSpace(lines[0]) != expectedOuter {
			t.Fatalf("outer x changed: got %q, want %q\nsource:\n%s",
				lines[0], expectedOuter, source)
		}

		// middle's x should be modified by inner's nonlocal assignment
		expectedMiddle := fmt.Sprintf("%d", newVal)
		if strings.TrimSpace(lines[1]) != expectedMiddle {
			t.Fatalf("middle x not modified: got %q, want %q\nsource:\n%s",
				lines[1], expectedMiddle, source)
		}
	})
}

// ============================================================
// Feature: pygo-post-mvp, Property 6: Closure Nesting Depth Independence
// ============================================================
// **Validates: Requirements 1.9, 1.10**
//
// For any closure nesting depth N (N >= 2), the compiler SHALL correctly
// thread cell/free variables through all intermediate scopes such that the
// innermost function can access variables from any enclosing level.

func TestProperty6_ClosureNestingDepthIndependence(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a nesting depth between 2 and 5
		depth := rapid.IntRange(2, 5).Draw(t, "depth")
		// Generate the value to capture at the outermost level
		capturedVal := rapid.IntRange(1, 1000).Draw(t, "capturedVal")

		// Build Python source dynamically:
		// def f0():
		//     x = <capturedVal>
		//     def f1():
		//         def f2():
		//             ...
		//                 def fN():
		//                     return x
		//                 return fN()
		//             ...
		//         return f2()
		//     return f1()
		// result = f0()
		// print(result)

		var sb strings.Builder
		indent := ""

		// Write outermost function with the captured variable
		sb.WriteString("def f0():\n")
		sb.WriteString("    x = ")
		sb.WriteString(fmt.Sprintf("%d", capturedVal))
		sb.WriteString("\n")
		indent = "    "

		// Write intermediate nested functions
		for i := 1; i <= depth; i++ {
			sb.WriteString(indent)
			sb.WriteString(fmt.Sprintf("def f%d():\n", i))
			indent += "    "
		}

		// Innermost function returns x
		sb.WriteString(indent)
		sb.WriteString("return x\n")

		// Unwind: each function calls the next and returns the result
		for i := depth; i >= 1; i-- {
			indent = indent[:len(indent)-4]
			sb.WriteString(indent)
			sb.WriteString(fmt.Sprintf("return f%d()\n", i))
		}

		// Call outermost and print result
		sb.WriteString("result = f0()\n")
		sb.WriteString("print(result)\n")

		source := sb.String()

		var buf bytes.Buffer
		r := New(WithOutput(&buf))
		err := r.ExecString(source, "<test>")
		if err != nil {
			t.Fatalf("execution failed for depth=%d: %v\nsource:\n%s", depth, err, source)
		}

		output := strings.TrimSpace(buf.String())
		expected := fmt.Sprintf("%d", capturedVal)
		if output != expected {
			t.Fatalf("nesting depth %d failed: got %q, want %q\nsource:\n%s",
				depth, output, expected, source)
		}
	})
}
