package runtime

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// ============================================================
// Feature: pygo-post-mvp, Property 13: Keyword Argument Binding
// ============================================================
// **Validates: Requirements 4.2, 4.5**
//
// For any function with named parameters and a call with keyword arguments,
// the VM SHALL bind each keyword argument to the parameter with the matching
// name, and excess keywords (when **kwargs is declared) SHALL appear in the
// kwargs dict.

func TestProperty13_KeywordArgumentBinding(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate 2-4 named parameters
		nParams := rapid.IntRange(2, 4).Draw(t, "nParams")
		paramPool := []string{"a", "b", "c", "d"}
		params := paramPool[:nParams]

		// Generate a random integer value for each parameter
		vals := make([]int, nParams)
		for i := 0; i < nParams; i++ {
			vals[i] = rapid.IntRange(-100, 100).Draw(t, fmt.Sprintf("val_%s", params[i]))
		}

		// Decide whether to include **kwargs collection
		hasKwargs := rapid.Bool().Draw(t, "hasKwargs")

		// Optionally add extra keyword arguments (only valid if **kwargs is declared)
		var extraKeys []string
		var extraVals []int
		if hasKwargs {
			nExtra := rapid.IntRange(0, 2).Draw(t, "nExtra")
			extraPool := []string{"x", "y", "z"}
			for i := 0; i < nExtra; i++ {
				extraKeys = append(extraKeys, extraPool[i])
				extraVals = append(extraVals, rapid.IntRange(200, 300).Draw(t, fmt.Sprintf("extra_%s", extraPool[i])))
			}
		}

		// Decide how many args to pass positionally vs as keywords.
		// nPos can be 0..nParams (rest are keyword args)
		nPos := rapid.IntRange(0, nParams).Draw(t, "nPos")

		// Build function definition
		var sb strings.Builder
		sb.WriteString("def f(")
		sb.WriteString(strings.Join(params, ", "))
		if hasKwargs {
			sb.WriteString(", **kw")
		}
		sb.WriteString("):\n")

		// Print each named param's value
		for _, p := range params {
			sb.WriteString(fmt.Sprintf("    print(%s)\n", p))
		}
		if hasKwargs && len(extraKeys) > 0 {
			// Print each known extra key directly — we know exact keys
			for _, k := range extraKeys {
				sb.WriteString(fmt.Sprintf("    print(%q, kw[%q])\n", k, k))
			}
		}

		// Build the call
		sb.WriteString("f(")
		var callParts []string
		// Positional args
		for i := 0; i < nPos; i++ {
			callParts = append(callParts, fmt.Sprintf("%d", vals[i]))
		}
		// Keyword args for remaining named params
		for i := nPos; i < nParams; i++ {
			callParts = append(callParts, fmt.Sprintf("%s=%d", params[i], vals[i]))
		}
		// Extra kwargs
		for i, k := range extraKeys {
			callParts = append(callParts, fmt.Sprintf("%s=%d", k, extraVals[i]))
		}
		sb.WriteString(strings.Join(callParts, ", "))
		sb.WriteString(")\n")

		source := sb.String()

		// Compute expected output
		var expected []string
		for i := 0; i < nParams; i++ {
			expected = append(expected, fmt.Sprintf("%d", vals[i]))
		}
		if hasKwargs {
			for i, k := range extraKeys {
				expected = append(expected, fmt.Sprintf("%s %d", k, extraVals[i]))
			}
		}

		var buf bytes.Buffer
		r := New(WithOutput(&buf))
		err := r.ExecString(source, "<test>")
		if err != nil {
			t.Fatalf("execution failed: %v\nsource:\n%s", err, source)
		}

		output := strings.TrimSpace(buf.String())
		expectedStr := strings.Join(expected, "\n")
		if output != expectedStr {
			t.Fatalf("keyword binding failed:\ngot:  %q\nwant: %q\nsource:\n%s",
				output, expectedStr, source)
		}
	})
}

// ============================================================
// Feature: pygo-post-mvp, Property 14: Positional/Keyword Separation
// with *args and **kwargs
// ============================================================
// **Validates: Requirements 4.6, 4.7**
//
// For any function with both *args and **kwargs, excess positional arguments
// SHALL appear only in the args tuple and keyword arguments not matching
// declared parameters SHALL appear only in the kwargs dict, with no overlap.

func TestProperty14_PositionalKeywordSeparation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate 1-3 named parameters
		nParams := rapid.IntRange(1, 3).Draw(t, "nParams")
		paramPool := []string{"a", "b", "c"}
		params := paramPool[:nParams]

		// Values for named params
		namedVals := make([]int, nParams)
		for i := 0; i < nParams; i++ {
			namedVals[i] = rapid.IntRange(1, 50).Draw(t, fmt.Sprintf("named_%s", params[i]))
		}

		// Generate 0-3 extra positional args (overflow into *args)
		nExtraPos := rapid.IntRange(0, 3).Draw(t, "nExtraPos")
		extraPosVals := make([]int, nExtraPos)
		for i := 0; i < nExtraPos; i++ {
			extraPosVals[i] = rapid.IntRange(100, 199).Draw(t, fmt.Sprintf("extraPos_%d", i))
		}

		// Generate 0-3 extra keyword args (overflow into **kwargs)
		nExtraKw := rapid.IntRange(0, 3).Draw(t, "nExtraKw")
		extraKwPool := []string{"x", "y", "z"}
		extraKwKeys := extraKwPool[:nExtraKw]
		extraKwVals := make([]int, nExtraKw)
		for i := 0; i < nExtraKw; i++ {
			extraKwVals[i] = rapid.IntRange(200, 299).Draw(t, fmt.Sprintf("extraKw_%s", extraKwKeys[i]))
		}

		// Build function: def f(a, b, ..., *args, **kwargs):
		var sb strings.Builder
		sb.WriteString("def f(")
		sb.WriteString(strings.Join(params, ", "))
		sb.WriteString(", *args, **kwargs):\n")

		// Print named params
		for _, p := range params {
			sb.WriteString(fmt.Sprintf("    print(%s)\n", p))
		}
		// Print args tuple length and contents
		sb.WriteString("    print(len(args))\n")
		sb.WriteString("    for v in args:\n")
		sb.WriteString("        print(v)\n")
		// Print kwargs dict length and each known extra key directly
		sb.WriteString("    print(len(kwargs))\n")
		for _, k := range extraKwKeys {
			sb.WriteString(fmt.Sprintf("    print(%q, kwargs[%q])\n", k, k))
		}

		// Build the call: all named params positionally, then extras positionally,
		// then extra kw args
		sb.WriteString("f(")
		var callParts []string
		for i := 0; i < nParams; i++ {
			callParts = append(callParts, fmt.Sprintf("%d", namedVals[i]))
		}
		for i := 0; i < nExtraPos; i++ {
			callParts = append(callParts, fmt.Sprintf("%d", extraPosVals[i]))
		}
		for i := 0; i < nExtraKw; i++ {
			callParts = append(callParts, fmt.Sprintf("%s=%d", extraKwKeys[i], extraKwVals[i]))
		}
		sb.WriteString(strings.Join(callParts, ", "))
		sb.WriteString(")\n")

		source := sb.String()

		// Compute expected output
		var expected []string
		// Named param values
		for i := 0; i < nParams; i++ {
			expected = append(expected, fmt.Sprintf("%d", namedVals[i]))
		}
		// args tuple length and contents
		expected = append(expected, fmt.Sprintf("%d", nExtraPos))
		for i := 0; i < nExtraPos; i++ {
			expected = append(expected, fmt.Sprintf("%d", extraPosVals[i]))
		}
		// kwargs dict length and each key-value
		expected = append(expected, fmt.Sprintf("%d", nExtraKw))
		for i, k := range extraKwKeys {
			expected = append(expected, fmt.Sprintf("%s %d", k, extraKwVals[i]))
		}

		var buf bytes.Buffer
		r := New(WithOutput(&buf))
		err := r.ExecString(source, "<test>")
		if err != nil {
			t.Fatalf("execution failed: %v\nsource:\n%s", err, source)
		}

		output := strings.TrimSpace(buf.String())
		expectedStr := strings.Join(expected, "\n")
		if output != expectedStr {
			t.Fatalf("positional/keyword separation failed:\ngot:  %q\nwant: %q\nsource:\n%s",
				output, expectedStr, source)
		}
	})
}

// ============================================================
// Feature: pygo-post-mvp, Property 15: Duplicate Argument Detection
// ============================================================
// **Validates: Requirements 4.4**
//
// For any function call where the same parameter is supplied both
// positionally and as a keyword argument, the runtime SHALL raise TypeError.

func TestProperty15_DuplicateArgumentDetection(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate 2-4 named parameters
		nParams := rapid.IntRange(2, 4).Draw(t, "nParams")
		paramPool := []string{"a", "b", "c", "d"}
		params := paramPool[:nParams]

		// Choose which parameter to duplicate (must be within the positional range)
		// We supply nPos positional args, then also pass one of those as a keyword.
		nPos := rapid.IntRange(1, nParams).Draw(t, "nPos")
		// Pick a duplicate target index among the positionally-supplied params
		dupIdx := rapid.IntRange(0, nPos-1).Draw(t, "dupIdx")
		dupParam := params[dupIdx]

		// Build function definition
		var sb strings.Builder
		sb.WriteString("def f(")
		sb.WriteString(strings.Join(params, ", "))
		sb.WriteString("):\n")
		sb.WriteString("    pass\n")

		// Build the call: nPos positional args, then duplicate one as keyword
		sb.WriteString("f(")
		var callParts []string
		for i := 0; i < nPos; i++ {
			callParts = append(callParts, fmt.Sprintf("%d", i+1))
		}
		// Add the duplicate as a keyword arg
		callParts = append(callParts, fmt.Sprintf("%s=%d", dupParam, 999))
		// Fill remaining params as keyword args (so the call is otherwise valid)
		for i := nPos; i < nParams; i++ {
			if params[i] != dupParam {
				callParts = append(callParts, fmt.Sprintf("%s=%d", params[i], i+1))
			}
		}
		sb.WriteString(strings.Join(callParts, ", "))
		sb.WriteString(")\n")

		source := sb.String()

		r := New()
		err := r.ExecString(source, "<test>")
		if err == nil {
			t.Fatalf("expected TypeError for duplicate argument '%s', got nil\nsource:\n%s",
				dupParam, source)
		}

		errMsg := err.Error()
		if !strings.Contains(errMsg, "multiple values for argument") {
			t.Fatalf("expected 'multiple values for argument' error, got: %s\nsource:\n%s",
				errMsg, source)
		}
		if !strings.Contains(errMsg, fmt.Sprintf("'%s'", dupParam)) {
			t.Fatalf("expected error to mention '%s', got: %s\nsource:\n%s",
				dupParam, errMsg, source)
		}
	})
}
