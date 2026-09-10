package compiler

import (
	"fmt"
	"testing"

	"github.com/alvinjaison/pygo/internal/bytecode"
	"github.com/alvinjaison/pygo/internal/parser"
	"pgregory.net/rapid"
)

// ============================================================
// Property 1: Compilation Determinism
// ============================================================
// **Validates: Requirements 4.5**
//
// Compiling the same Python source N times within the same process MUST
// produce identical instruction sequences, constant pools, line tables,
// Names, and Varnames arrays.

// genValidPythonSource generates valid Python source strings using
// combinatorial construction from a set of templates and building blocks.
func genValidPythonSource(t *rapid.T) string {
	// Categories of valid Python programs the compiler supports
	generators := []func(*rapid.T) string{
		genAssignment,
		genMultiAssignment,
		genArithmeticExpr,
		genFunctionDef,
		genIfStatement,
		genWhileLoop,
		genForLoop,
		genNestedStatements,
		genComparisonExpr,
		genBooleanExpr,
		genUnaryExpr,
		genCollectionLiteral,
		genFunctionCall,
	}

	idx := rapid.IntRange(0, len(generators)-1).Draw(t, "genIdx")
	return generators[idx](t)
}

func genVarName(t *rapid.T, label string) string {
	names := []string{"x", "y", "z", "a", "b", "c", "n", "m", "val", "result", "tmp", "foo", "bar"}
	idx := rapid.IntRange(0, len(names)-1).Draw(t, label)
	return names[idx]
}

func genIntLiteral(t *rapid.T, label string) string {
	n := rapid.IntRange(-1000, 1000).Draw(t, label)
	return fmt.Sprintf("%d", n)
}

func genAssignment(t *rapid.T) string {
	name := genVarName(t, "varName")
	val := genIntLiteral(t, "intVal")
	return fmt.Sprintf("%s = %s", name, val)
}

func genMultiAssignment(t *rapid.T) string {
	n := rapid.IntRange(2, 4).Draw(t, "numAssigns")
	lines := make([]string, n)
	for i := 0; i < n; i++ {
		name := genVarName(t, fmt.Sprintf("var_%d", i))
		val := genIntLiteral(t, fmt.Sprintf("val_%d", i))
		lines[i] = fmt.Sprintf("%s = %s", name, val)
	}
	result := lines[0]
	for i := 1; i < len(lines); i++ {
		result += "\n" + lines[i]
	}
	return result
}

func genArithmeticExpr(t *rapid.T) string {
	left := genIntLiteral(t, "left")
	right := genIntLiteral(t, "right")
	ops := []string{"+", "-", "*"}
	opIdx := rapid.IntRange(0, len(ops)-1).Draw(t, "op")
	name := genVarName(t, "varName")
	return fmt.Sprintf("%s = %s %s %s", name, left, ops[opIdx], right)
}

func genFunctionDef(t *rapid.T) string {
	funcName := genVarName(t, "funcName")
	paramCount := rapid.IntRange(0, 3).Draw(t, "paramCount")
	params := ""
	for i := 0; i < paramCount; i++ {
		if i > 0 {
			params += ", "
		}
		params += genVarName(t, fmt.Sprintf("param_%d", i))
	}

	// Simple function body
	bodies := []string{
		"    return 0",
		"    x = 1\n    return x",
		"    return a + 1",
		"    pass",
	}
	bodyIdx := rapid.IntRange(0, len(bodies)-1).Draw(t, "bodyIdx")

	return fmt.Sprintf("def %s(%s):\n%s", funcName, params, bodies[bodyIdx])
}

func genIfStatement(t *rapid.T) string {
	name := genVarName(t, "varName")
	templates := []string{
		fmt.Sprintf("if %s:\n    y = 1", name),
		fmt.Sprintf("if %s:\n    y = 1\nelse:\n    y = 2", name),
		fmt.Sprintf("if %s:\n    y = 1\n    z = 2\nelse:\n    y = 3", name),
	}
	idx := rapid.IntRange(0, len(templates)-1).Draw(t, "templateIdx")
	return templates[idx]
}

func genWhileLoop(t *rapid.T) string {
	templates := []string{
		"x = 5\nwhile x:\n    x = x - 1",
		"n = 3\nwhile n:\n    n = n - 1",
		"i = 10\nwhile i:\n    i = i - 1\n    x = i",
	}
	idx := rapid.IntRange(0, len(templates)-1).Draw(t, "templateIdx")
	return templates[idx]
}

func genForLoop(t *rapid.T) string {
	templates := []string{
		"x = [1, 2, 3]\nfor i in x:\n    y = i",
		"for i in [1, 2]:\n    x = i",
		"items = [10, 20]\nfor item in items:\n    result = item + 1",
	}
	idx := rapid.IntRange(0, len(templates)-1).Draw(t, "templateIdx")
	return templates[idx]
}

func genNestedStatements(t *rapid.T) string {
	templates := []string{
		"x = 1\ny = 2\nz = x + y",
		"a = 10\nb = 20\nc = a + b\nd = c - a",
		"x = 5\nif x:\n    y = x + 1\nz = 0",
	}
	idx := rapid.IntRange(0, len(templates)-1).Draw(t, "templateIdx")
	return templates[idx]
}

func genComparisonExpr(t *rapid.T) string {
	left := genIntLiteral(t, "left")
	right := genIntLiteral(t, "right")
	ops := []string{"==", "!=", "<", ">", "<=", ">="}
	opIdx := rapid.IntRange(0, len(ops)-1).Draw(t, "op")
	name := genVarName(t, "varName")
	return fmt.Sprintf("%s = %s %s %s", name, left, ops[opIdx], right)
}

func genBooleanExpr(t *rapid.T) string {
	templates := []string{
		"x = True and False",
		"x = True or False",
		"x = 1 and 2",
		"x = 0 or 5",
	}
	idx := rapid.IntRange(0, len(templates)-1).Draw(t, "templateIdx")
	return templates[idx]
}

func genUnaryExpr(t *rapid.T) string {
	val := genIntLiteral(t, "val")
	ops := []string{"-", "+", "not "}
	opIdx := rapid.IntRange(0, len(ops)-1).Draw(t, "op")
	name := genVarName(t, "varName")
	return fmt.Sprintf("%s = %s%s", name, ops[opIdx], val)
}

func genCollectionLiteral(t *rapid.T) string {
	templates := []string{
		"x = [1, 2, 3]",
		"x = (1, 2, 3)",
		`x = {"a": 1, "b": 2}`,
		"x = []",
		"x = ()",
	}
	idx := rapid.IntRange(0, len(templates)-1).Draw(t, "templateIdx")
	return templates[idx]
}

func genFunctionCall(t *rapid.T) string {
	templates := []string{
		"x = 1\nprint(x)",
		"print(42)",
		"print(1, 2, 3)",
		"def f(a):\n    return a\nx = f(5)",
	}
	idx := rapid.IntRange(0, len(templates)-1).Draw(t, "templateIdx")
	return templates[idx]
}

// codeEqual checks whether two Code objects are identical in terms of
// instruction sequences, constant pools, line tables, Names, and Varnames.
func codeEqual(a, b *bytecode.Code) (bool, string) {
	// Compare instruction sequences
	if len(a.Instructions) != len(b.Instructions) {
		return false, fmt.Sprintf("instruction count differs: %d vs %d", len(a.Instructions), len(b.Instructions))
	}
	for i, instrA := range a.Instructions {
		instrB := b.Instructions[i]
		if instrA.Op != instrB.Op || instrA.Arg != instrB.Arg {
			return false, fmt.Sprintf("instruction[%d] differs: {%v, %d} vs {%v, %d}",
				i, instrA.Op, instrA.Arg, instrB.Op, instrB.Arg)
		}
	}

	// Compare constant pools
	if len(a.Constants) != len(b.Constants) {
		return false, fmt.Sprintf("constant pool size differs: %d vs %d", len(a.Constants), len(b.Constants))
	}
	for i, constA := range a.Constants {
		constB := b.Constants[i]
		if !constantEqual(constA, constB) {
			return false, fmt.Sprintf("constant[%d] differs: %v vs %v", i, constA, constB)
		}
	}

	// Compare line tables
	if len(a.LineTable) != len(b.LineTable) {
		return false, fmt.Sprintf("line table size differs: %d vs %d", len(a.LineTable), len(b.LineTable))
	}
	for i, lineA := range a.LineTable {
		lineB := b.LineTable[i]
		if lineA != lineB {
			return false, fmt.Sprintf("line table[%d] differs: %d vs %d", i, lineA, lineB)
		}
	}

	// Compare Names
	if len(a.Names) != len(b.Names) {
		return false, fmt.Sprintf("Names size differs: %d vs %d", len(a.Names), len(b.Names))
	}
	for i, nameA := range a.Names {
		nameB := b.Names[i]
		if nameA != nameB {
			return false, fmt.Sprintf("Names[%d] differs: %q vs %q", i, nameA, nameB)
		}
	}

	// Compare Varnames
	if len(a.Varnames) != len(b.Varnames) {
		return false, fmt.Sprintf("Varnames size differs: %d vs %d", len(a.Varnames), len(b.Varnames))
	}
	for i, vnA := range a.Varnames {
		vnB := b.Varnames[i]
		if vnA != vnB {
			return false, fmt.Sprintf("Varnames[%d] differs: %q vs %q", i, vnA, vnB)
		}
	}

	return true, ""
}

// constantEqual compares two constant pool entries for equality.
// Handles nested *bytecode.Code objects recursively.
func constantEqual(a, b interface{}) bool {
	// Handle nested Code objects
	codeA, aIsCode := a.(*bytecode.Code)
	codeB, bIsCode := b.(*bytecode.Code)
	if aIsCode && bIsCode {
		eq, _ := codeEqual(codeA, codeB)
		return eq
	}
	if aIsCode != bIsCode {
		return false
	}

	// Compare by formatted string for other types (int64, float64, string, bool, nil)
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func TestProperty1_CompilationDeterminism(t *testing.T) {
	const N = 3 // Number of compilations to compare

	rapid.Check(t, func(t *rapid.T) {
		source := genValidPythonSource(t)

		// Parse the source into an AST
		module, err := parser.Parse(source, "<test>")
		if err != nil {
			// If parsing fails, skip this input — the generator produced
			// something the parser doesn't handle. This is acceptable for
			// property testing since we're testing compilation determinism,
			// not parser coverage.
			t.Skip("parse error (not a compiler determinism issue)")
		}

		// Compile N times and collect results
		codes := make([]*bytecode.Code, N)
		for i := 0; i < N; i++ {
			// Re-parse each time to ensure independent AST objects
			mod, parseErr := parser.Parse(source, "<test>")
			if parseErr != nil {
				t.Fatalf("parse failed on iteration %d (source parsed successfully before): %v", i, parseErr)
			}
			_ = module // original parse was just a validity check

			code, compileErr := Compile(mod)
			if compileErr != nil {
				t.Fatalf("compilation failed on iteration %d: %v\nsource: %s", i, compileErr, source)
			}
			codes[i] = code
		}

		// Assert all compilation results are identical to the first
		for i := 1; i < N; i++ {
			equal, reason := codeEqual(codes[0], codes[i])
			if !equal {
				t.Fatalf("compilation %d differs from compilation 0: %s\nsource: %s", i, reason, source)
			}
		}
	})
}
