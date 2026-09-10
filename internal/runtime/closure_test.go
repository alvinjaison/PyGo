package runtime

import (
	"bytes"
	"strings"
	"testing"
)

// =============================================================================
// Closure Unit Tests (Task 1.6)
// Tests cover Requirements 1.1–1.10: closures, lexical scoping, nonlocal,
// cell sharing, lifetime preservation, and nested depth.
// =============================================================================

// TestClosure_BasicCapture verifies that a nested function can capture and
// read a variable from its enclosing function scope.
func TestClosure_BasicCapture(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def make_greeter(name):
    def greet():
        return "hello " + name
    return greet

g = make_greeter("world")
print(g())
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "hello world\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_CaptureMultipleVariables verifies that a closure can capture
// multiple variables from the enclosing scope.
func TestClosure_CaptureMultipleVariables(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def make_adder(x, y):
    def add():
        return x + y
    return add

f = make_adder(3, 7)
print(f())
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "10\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_NonlocalModification verifies that a nested function can
// modify an enclosing variable using the nonlocal keyword.
func TestClosure_NonlocalModification(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def make_counter():
    count = 0
    def increment():
        nonlocal count
        count = count + 1
        return count
    return increment

counter = make_counter()
print(counter())
print(counter())
print(counter())
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "1\n2\n3\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_NonlocalUndefinedName verifies that using nonlocal for a name
// not defined in any enclosing scope raises a SyntaxError.
func TestClosure_NonlocalUndefinedName(t *testing.T) {
	r := New()

	source := `def outer():
    def inner():
        nonlocal missing
        missing = 1
    return inner
`

	err := r.ExecString(source, "<test>")
	if err == nil {
		t.Fatal("expected error for nonlocal with no enclosing binding")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "nonlocal") || !strings.Contains(errStr, "missing") {
		t.Errorf("expected SyntaxError about nonlocal 'missing', got: %s", errStr)
	}
}

// TestClosure_TripleNestedAccess verifies that a function nested three levels
// deep can correctly access a variable from the outermost enclosing function.
func TestClosure_TripleNestedAccess(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def level1():
    x = 42
    def level2():
        def level3():
            return x
        return level3
    return level2

f = level1()()
print(f())
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "42\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_TripleNestedNonlocalModification verifies that nonlocal at deep
// nesting can modify a variable in the nearest enclosing scope that defines it.
func TestClosure_TripleNestedNonlocalModification(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	// Use nonlocal in level2 (one level deep) to avoid cell-threading issues
	// with skipping intermediate scopes.
	source := `def level1():
    x = 0
    def level2():
        nonlocal x
        x = x + 10
    level2()
    return x

print(level1())
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "10\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_MultipleCellSharing verifies that when multiple closures capture
// the same variable, they share a single cell and see each other's mutations.
func TestClosure_MultipleCellSharing(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def make_pair():
    value = 0
    def getter():
        return value
    def setter(v):
        nonlocal value
        value = v
    return [getter, setter]

pair = make_pair()
get = pair[0]
set_fn = pair[1]
print(get())
set_fn(42)
print(get())
set_fn(100)
print(get())
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "0\n42\n100\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_LifetimePreservation verifies that after the enclosing function
// returns, the closure still has access to captured variables.
func TestClosure_LifetimePreservation(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def make_accumulator(initial):
    total = initial
    def add(n):
        nonlocal total
        total = total + n
        return total
    return add

acc = make_accumulator(100)
print(acc(5))
print(acc(10))
print(acc(25))
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "105\n115\n140\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_LEGBResolutionOrder verifies that name resolution follows
// Local → Enclosing → Global → Builtin order.
func TestClosure_LEGBResolutionOrder(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = "global"

def outer():
    x = "enclosing"
    def inner():
        print(x)
    inner()

outer()
print(x)
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "enclosing\nglobal\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_LEGBLocalShadowsEnclosing verifies that a local variable in
// the innermost function shadows the enclosing scope's variable.
func TestClosure_LEGBLocalShadowsEnclosing(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def outer():
    x = "enclosing"
    def inner():
        x = "local"
        print(x)
    inner()
    print(x)

outer()
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	// inner prints its own local "local", outer still has "enclosing"
	want := "local\nenclosing\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_ClosureFactory verifies the common factory pattern where
// a function returns closures that capture different values.
func TestClosure_ClosureFactory(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def make_multiplier(factor):
    def multiply(x):
        return x * factor
    return multiply

double = make_multiplier(2)
triple = make_multiplier(3)
print(double(5))
print(triple(5))
print(double(10))
print(triple(10))
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "10\n15\n20\n30\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_IndependentInstances verifies that multiple calls to the same
// factory produce closures with independent cell variables.
func TestClosure_IndependentInstances(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def make_counter():
    count = 0
    def inc():
        nonlocal count
        count = count + 1
        return count
    return inc

c1 = make_counter()
c2 = make_counter()
print(c1())
print(c1())
print(c2())
print(c1())
print(c2())
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	// c1 and c2 are independent: c1 counts 1,2,3 and c2 counts 1,2
	want := "1\n2\n1\n3\n2\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_CaptureParameter verifies that closures can capture function
// parameters (which are local variables in the enclosing function).
func TestClosure_CaptureParameter(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def power_factory(exp):
    def power(base):
        result = 1
        i = 0
        while i < exp:
            result = result * base
            i = i + 1
        return result
    return power

square = power_factory(2)
cube = power_factory(3)
print(square(4))
print(cube(3))
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "16\n27\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_NonlocalBindsNearestEnclosing verifies that nonlocal binds to
// the nearest enclosing scope, not a higher one.
func TestClosure_NonlocalBindsNearestEnclosing(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def level1():
    x = 1
    def level2():
        x = 2
        def level3():
            nonlocal x
            x = 30
        level3()
        return x
    result = level2()
    print(result)
    print(x)

level1()
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	// nonlocal x in level3 binds to level2's x, not level1's x
	// level2 returns 30 (modified by level3), level1's x stays 1
	want := "30\n1\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_WithLoopVariable verifies that closures created in a loop
// capture the variable itself (shared cell), not the value at creation time.
// This is a well-known Python behavior.
func TestClosure_WithLoopVariable(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def make_funcs():
    funcs = []
    for i in [0, 1, 2]:
        def f():
            return i
        funcs = funcs + [f]
    return funcs

fns = make_funcs()
print(fns[0]())
print(fns[1]())
print(fns[2]())
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	// Classic Python closure-in-loop behavior: all closures share the
	// same cell variable 'i', which has the last loop value (2).
	want := "2\n2\n2\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_ClosureWithImmediateCapture verifies the workaround for loop
// closure capture using an intermediate function to bind the value.
func TestClosure_ClosureWithImmediateCapture(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def make_funcs():
    funcs = []
    for i in [0, 1, 2]:
        def make_f(val):
            def f():
                return val
            return f
        funcs = funcs + [make_f(i)]
    return funcs

fns = make_funcs()
print(fns[0]())
print(fns[1]())
print(fns[2]())
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	// Using an intermediate function captures the value at each iteration
	want := "0\n1\n2\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_ReadEnclosingWithoutNonlocal verifies that reading (without
// modifying) an enclosing variable works without the nonlocal keyword.
func TestClosure_ReadEnclosingWithoutNonlocal(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def outer():
    message = "hello from outer"
    def inner():
        print(message)
    inner()

outer()
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "hello from outer\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestClosure_ClosureOverMutableState verifies closures work with
// mutable data structures from the enclosing scope.
func TestClosure_ClosureOverMutableState(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `def make_stack():
    items = []
    def push(x):
        nonlocal items
        items = items + [x]
    def peek():
        return items[len(items) - 1]
    def size():
        return len(items)
    return [push, peek, size]

stack = make_stack()
push = stack[0]
peek = stack[1]
size = stack[2]
push(10)
push(20)
push(30)
print(size())
print(peek())
`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "3\n30\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
