package runtime

import (
	"bytes"
	"strings"
	"testing"
)

// TestDisassemble exercises the diagnostics path end-to-end.
func TestDisassemble(t *testing.T) {
	src := "x = 1 + 2\nprint(x)\n"
	rt := New(Config{})
	var buf bytes.Buffer
	if err := rt.Disassemble(src, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"LOAD_CONST", "BINARY_ADD", "STORE_NAME", "CALL_FUNCTION"} {
		if !strings.Contains(out, want) {
			t.Errorf("disassembly missing %q:\n%s", want, out)
		}
	}
}
