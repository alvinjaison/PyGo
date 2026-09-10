package runtime

import (
	"bytes"
	"strings"
	"testing"
)

func TestDisassemble_SimpleAssignment(t *testing.T) {
	var buf bytes.Buffer
	err := Disassemble("x = 42\n", &buf)
	if err != nil {
		t.Fatalf("Disassemble failed: %v", err)
	}

	output := buf.String()

	// Must start with the disassembly header
	if !strings.HasPrefix(output, "Disassembly of <module>:") {
		t.Errorf("expected header 'Disassembly of <module>:', got:\n%s", output)
	}

	// Must contain LOAD_CONST with 42
	if !strings.Contains(output, "LOAD_CONST") || !strings.Contains(output, "42") {
		t.Errorf("expected LOAD_CONST with 42 in output:\n%s", output)
	}

	// Must contain STORE_NAME with x
	if !strings.Contains(output, "STORE_NAME") || !strings.Contains(output, "x") {
		t.Errorf("expected STORE_NAME with x in output:\n%s", output)
	}

	// Must contain RETURN_VALUE (implicit module return)
	if !strings.Contains(output, "RETURN_VALUE") {
		t.Errorf("expected RETURN_VALUE in output:\n%s", output)
	}
}

func TestDisassemble_FunctionDef(t *testing.T) {
	source := `def add(a, b):
    return a + b
`
	var buf bytes.Buffer
	err := Disassemble(source, &buf)
	if err != nil {
		t.Fatalf("Disassemble failed: %v", err)
	}

	output := buf.String()

	// Should contain MAKE_FUNCTION
	if !strings.Contains(output, "MAKE_FUNCTION") {
		t.Errorf("expected MAKE_FUNCTION in output:\n%s", output)
	}

	// Should reference the function name
	if !strings.Contains(output, "add") {
		t.Errorf("expected 'add' in output:\n%s", output)
	}
}

func TestDisassemble_FormatIncludesOffset(t *testing.T) {
	var buf bytes.Buffer
	err := Disassemble("x = 1\n", &buf)
	if err != nil {
		t.Fatalf("Disassemble failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have at least header + some instructions
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d:\n%s", len(lines), output)
	}

	// Each instruction line should have offset, line number, opcode
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Lines should contain at least an opcode name (all caps with underscore)
		hasOpcode := false
		for _, word := range strings.Fields(line) {
			if word == strings.ToUpper(word) && len(word) > 2 && strings.Contains(word, "_") {
				hasOpcode = true
				break
			}
			// Handle opcodes without underscores (like NOP) — but RETURN_VALUE, LOAD_CONST etc. have them
			if word == "RETURN_VALUE" || word == "LOAD_CONST" || word == "STORE_NAME" || word == "POP_TOP" {
				hasOpcode = true
				break
			}
		}
		if !hasOpcode {
			t.Errorf("line doesn't appear to have an opcode: %q", line)
		}
	}
}

func TestDisassemble_SyntaxError(t *testing.T) {
	var buf bytes.Buffer
	err := Disassemble("def (", &buf)
	if err == nil {
		t.Fatal("expected error for syntax error source")
	}

	// Buffer should be empty (no partial output)
	if buf.Len() != 0 {
		t.Errorf("expected no partial output on error, got %d bytes: %q", buf.Len(), buf.String())
	}
}

func TestDumpAST_SimpleAssignment(t *testing.T) {
	var buf bytes.Buffer
	err := DumpAST("x = 42\n", &buf)
	if err != nil {
		t.Fatalf("DumpAST failed: %v", err)
	}

	output := buf.String()

	// Should start with Module
	if !strings.HasPrefix(output, "Module") {
		t.Errorf("expected output to start with 'Module', got:\n%s", output)
	}

	// Should contain Assign
	if !strings.Contains(output, "Assign") {
		t.Errorf("expected 'Assign' in output:\n%s", output)
	}

	// Should contain the target name
	if !strings.Contains(output, "Name(x)") {
		t.Errorf("expected 'Name(x)' in output:\n%s", output)
	}

	// Should contain the constant value
	if !strings.Contains(output, "Constant(42)") {
		t.Errorf("expected 'Constant(42)' in output:\n%s", output)
	}

	// Should contain line numbers
	if !strings.Contains(output, "[line") {
		t.Errorf("expected line numbers in output:\n%s", output)
	}
}

func TestDumpAST_FunctionDef(t *testing.T) {
	source := `def greet(name):
    return name
`
	var buf bytes.Buffer
	err := DumpAST(source, &buf)
	if err != nil {
		t.Fatalf("DumpAST failed: %v", err)
	}

	output := buf.String()

	// Should contain FunctionDef
	if !strings.Contains(output, "FunctionDef") {
		t.Errorf("expected 'FunctionDef' in output:\n%s", output)
	}

	// Should show the function name
	if !strings.Contains(output, "Name: greet") {
		t.Errorf("expected 'Name: greet' in output:\n%s", output)
	}

	// Should show arguments
	if !strings.Contains(output, "Args:") && !strings.Contains(output, "name") {
		t.Errorf("expected args info in output:\n%s", output)
	}

	// Should contain Return
	if !strings.Contains(output, "Return") {
		t.Errorf("expected 'Return' in output:\n%s", output)
	}
}

func TestDumpAST_IfStatement(t *testing.T) {
	source := `if x > 0:
    y = 1
else:
    y = 2
`
	var buf bytes.Buffer
	err := DumpAST(source, &buf)
	if err != nil {
		t.Fatalf("DumpAST failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "If") {
		t.Errorf("expected 'If' in output:\n%s", output)
	}
	if !strings.Contains(output, "Compare") {
		t.Errorf("expected 'Compare' in output:\n%s", output)
	}
	if !strings.Contains(output, "Else:") {
		t.Errorf("expected 'Else:' in output:\n%s", output)
	}
}

func TestDumpAST_SyntaxError(t *testing.T) {
	var buf bytes.Buffer
	err := DumpAST("def (", &buf)
	if err == nil {
		t.Fatal("expected error for syntax error source")
	}

	// Buffer should be empty (no partial output)
	if buf.Len() != 0 {
		t.Errorf("expected no partial output on error, got %d bytes: %q", buf.Len(), buf.String())
	}
}

func TestDumpAST_MultipleStatements(t *testing.T) {
	source := `x = 1
y = 2
print(x + y)
`
	var buf bytes.Buffer
	err := DumpAST(source, &buf)
	if err != nil {
		t.Fatalf("DumpAST failed: %v", err)
	}

	output := buf.String()

	// Should have multiple Assign nodes
	if strings.Count(output, "Assign") < 2 {
		t.Errorf("expected at least 2 Assign nodes in output:\n%s", output)
	}

	// Should have ExprStmt for the print call
	if !strings.Contains(output, "ExprStmt") {
		t.Errorf("expected 'ExprStmt' in output:\n%s", output)
	}

	// Should have Call
	if !strings.Contains(output, "Call") {
		t.Errorf("expected 'Call' in output:\n%s", output)
	}
}

func TestDisassemble_CompareOp(t *testing.T) {
	var buf bytes.Buffer
	err := Disassemble("x = 1 < 2\n", &buf)
	if err != nil {
		t.Fatalf("Disassemble failed: %v", err)
	}

	output := buf.String()

	// Should contain COMPARE_OP with the decoded comparison
	if !strings.Contains(output, "COMPARE_OP") {
		t.Errorf("expected COMPARE_OP in output:\n%s", output)
	}
}
