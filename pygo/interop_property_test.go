package pygo

import (
	"fmt"
	"math"
	"testing"

	"pgregory.net/rapid"
)

// pythonKeywords is the set of Python reserved words that cannot be used as identifiers.
var pythonKeywords = map[string]bool{
	"False": true, "None": true, "True": true, "and": true, "as": true,
	"assert": true, "async": true, "await": true, "break": true, "class": true,
	"continue": true, "def": true, "del": true, "elif": true, "else": true,
	"except": true, "finally": true, "for": true, "from": true, "global": true,
	"if": true, "import": true, "in": true, "is": true, "lambda": true,
	"nonlocal": true, "not": true, "or": true, "pass": true, "raise": true,
	"return": true, "try": true, "while": true, "with": true, "yield": true,
}

// genPythonIdentifier generates a valid Python identifier that is not a keyword.
func genPythonIdentifier(t *rapid.T, label string) string {
	return rapid.StringMatching(`[a-z][a-z0-9_]{2,9}`).
		Filter(func(s string) bool { return !pythonKeywords[s] }).
		Draw(t, label)
}

// ============================================================
// Property 7: Value Conversion Round-Trip
// ============================================================
// **Validates: Requirements 7.1, 7.3, 7.4, 8.1**
//
// Generate Go primitives (bool, int64, float64, string), Set into runtime,
// Get back, verify original value preserved. Also test RegisterFunction
// with correctly-typed args produces same result as direct Go call.

func TestProperty7_ValueConversionRoundTrip_Bool(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		b := rapid.Bool().Draw(t, "b")

		rt := New()
		err := rt.Set("x", b)
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}

		val, err := rt.Get("x")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}

		got := val.Bool()
		if got != b {
			t.Fatalf("Bool round-trip: Set(%v), Got(%v)", b, got)
		}
	})
}

func TestProperty7_ValueConversionRoundTrip_Int64(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.Int64().Draw(t, "n")

		rt := New()
		err := rt.Set("x", n)
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}

		val, err := rt.Get("x")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}

		got, err := val.Int()
		if err != nil {
			t.Fatalf("Int() error: %v", err)
		}
		if got != n {
			t.Fatalf("Int64 round-trip: Set(%d), Got(%d)", n, got)
		}
	})
}

func TestProperty7_ValueConversionRoundTrip_Float64(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate finite floats (NaN and Inf have special comparison semantics)
		f := rapid.Float64().Filter(func(f float64) bool {
			return !math.IsNaN(f) && !math.IsInf(f, 0)
		}).Draw(t, "f")

		rt := New()
		err := rt.Set("x", f)
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}

		val, err := rt.Get("x")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}

		got, err := val.Float()
		if err != nil {
			t.Fatalf("Float() error: %v", err)
		}
		if got != f {
			t.Fatalf("Float64 round-trip: Set(%v), Got(%v)", f, got)
		}
	})
}

func TestProperty7_ValueConversionRoundTrip_String(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")

		rt := New()
		err := rt.Set("x", s)
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}

		val, err := rt.Get("x")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}

		got, err := val.Str()
		if err != nil {
			t.Fatalf("Str() error: %v", err)
		}
		if got != s {
			t.Fatalf("String round-trip: Set(%q), Got(%q)", s, got)
		}
	})
}

func TestProperty7_RegisterFunction_MatchesDirectCall(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.Int64Range(-1000000, 1000000).Draw(t, "a")
		b := rapid.Int64Range(-1000000, 1000000).Draw(t, "b")

		// Direct Go call
		adder := func(x, y int64) int64 { return x + y }
		expected := adder(a, b)

		// Register and call via Python
		rt := New()
		err := rt.RegisterFunction("adder", adder)
		if err != nil {
			t.Fatalf("RegisterFunction error: %v", err)
		}

		err = rt.Set("a", a)
		if err != nil {
			t.Fatalf("Set a error: %v", err)
		}
		err = rt.Set("b", b)
		if err != nil {
			t.Fatalf("Set b error: %v", err)
		}

		// Call the registered function from Python
		err = rt.RunString("result = adder(a, b)")
		if err != nil {
			t.Fatalf("RunString error: %v", err)
		}

		val, err := rt.Get("result")
		if err != nil {
			t.Fatalf("Get result error: %v", err)
		}

		got, err := val.Int()
		if err != nil {
			t.Fatalf("Int() error: %v", err)
		}
		if got != expected {
			t.Fatalf("RegisterFunction round-trip: adder(%d, %d) = %d (Go), got %d (Python)",
				a, b, expected, got)
		}
	})
}

// ============================================================
// Property 9: Runtime Isolation
// ============================================================
// **Validates: Requirements 13.1**
//
// Create two Runtime instances, mutate globals in one (Set, RegisterFunction),
// verify the other is unaffected.

func TestProperty9_RuntimeIsolation_SetDoesNotLeak(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random variable name (valid Python identifier)
		name := genPythonIdentifier(t, "name")
		value := rapid.Int64().Draw(t, "value")

		rt1 := New()
		rt2 := New()

		// Set variable in rt1
		err := rt1.Set(name, value)
		if err != nil {
			t.Fatalf("rt1.Set error: %v", err)
		}

		// Verify rt1 has the variable
		val, err := rt1.Get(name)
		if err != nil {
			t.Fatalf("rt1.Get error: %v", err)
		}
		got, err := val.Int()
		if err != nil {
			t.Fatalf("rt1 Int() error: %v", err)
		}
		if got != value {
			t.Fatalf("rt1 has wrong value: expected %d, got %d", value, got)
		}

		// Verify rt2 does NOT have the variable (should return NameError)
		_, err = rt2.Get(name)
		if err == nil {
			t.Fatalf("rt2.Get(%q) should have returned error (variable should not exist in rt2)", name)
		}
		if !IsError(err, "NameError") {
			t.Fatalf("rt2.Get(%q) expected NameError, got: %v", name, err)
		}
	})
}

func TestProperty9_RuntimeIsolation_RegisterFunctionDoesNotLeak(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random function name (valid Python identifier)
		name := genPythonIdentifier(t, "name")
		returnVal := rapid.Int64Range(-1000, 1000).Draw(t, "returnVal")

		rt1 := New()
		rt2 := New()

		// Register a function in rt1
		fn := func() int64 { return returnVal }
		err := rt1.RegisterFunction(name, fn)
		if err != nil {
			t.Fatalf("rt1.RegisterFunction error: %v", err)
		}

		// Verify rt1 can call the function
		err = rt1.RunString(fmt.Sprintf("__result = %s()", name))
		if err != nil {
			t.Fatalf("rt1.RunString error: %v", err)
		}
		val, err := rt1.Get("__result")
		if err != nil {
			t.Fatalf("rt1.Get __result error: %v", err)
		}
		got, err := val.Int()
		if err != nil {
			t.Fatalf("rt1 Int() error: %v", err)
		}
		if got != returnVal {
			t.Fatalf("rt1 function returned %d, expected %d", got, returnVal)
		}

		// Verify rt2 does NOT have the function (should return NameError)
		_, err = rt2.Get(name)
		if err == nil {
			t.Fatalf("rt2.Get(%q) should have returned error (function should not exist in rt2)", name)
		}
		if !IsError(err, "NameError") {
			t.Fatalf("rt2.Get(%q) expected NameError, got: %v", name, err)
		}
	})
}

func TestProperty9_RuntimeIsolation_MutationDoesNotAffectOther(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate multiple random variables
		numVars := rapid.IntRange(1, 10).Draw(t, "numVars")

		rt1 := New()
		rt2 := New()

		names := make([]string, numVars)
		seen := make(map[string]bool)
		for i := 0; i < numVars; i++ {
			for {
				name := genPythonIdentifier(t, fmt.Sprintf("name_%d", i))
				if !seen[name] {
					names[i] = name
					seen[name] = true
					break
				}
			}
		}

		// Set all variables in rt1
		for i, name := range names {
			err := rt1.Set(name, int64(i*100))
			if err != nil {
				t.Fatalf("rt1.Set(%q) error: %v", name, err)
			}
		}

		// Verify NONE of them exist in rt2
		for _, name := range names {
			_, err := rt2.Get(name)
			if err == nil {
				t.Fatalf("rt2.Get(%q) should have failed — variable leaked from rt1", name)
			}
			if !IsError(err, "NameError") {
				t.Fatalf("rt2.Get(%q) expected NameError, got: %v", name, err)
			}
		}
	})
}
