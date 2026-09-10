package runtime

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Module and Import System Tests ---

func TestImportSys(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`import sys
print(sys.version)`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "PyGo") {
		t.Errorf("expected sys.version to contain 'PyGo', got %q", got)
	}
}

func TestImportSysArgv(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`import sys
print(type(sys.argv))`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "list") {
		t.Errorf("expected sys.argv to be a list, got %q", got)
	}
}

func TestImportSysPath(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`import sys
print(type(sys.path))`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "list") {
		t.Errorf("expected sys.path to be a list, got %q", got)
	}
}

func TestImportSysAsAlias(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`import sys as system
print(system.version)`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "PyGo") {
		t.Errorf("expected system.version to contain 'PyGo', got %q", got)
	}
}

func TestFromSysImportVersion(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`from sys import version
print(version)`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "PyGo") {
		t.Errorf("expected version to contain 'PyGo', got %q", got)
	}
}

func TestFromSysImportMultiple(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`from sys import version, argv
print(version)
print(type(argv))`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "PyGo") {
		t.Errorf("expected output to contain 'PyGo', got %q", got)
	}
	if !strings.Contains(got, "list") {
		t.Errorf("expected output to contain 'list', got %q", got)
	}
}

func TestFromSysImportNameAsAlias(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`from sys import version as ver
print(ver)`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "PyGo") {
		t.Errorf("expected ver to contain 'PyGo', got %q", got)
	}
}

func TestImportModuleNotFound(t *testing.T) {
	r := New()
	err := r.ExecString(`import nonexistent_module`, "<test>")
	if err == nil {
		t.Fatal("expected ModuleNotFoundError, got nil")
	}
	if !strings.Contains(err.Error(), "No module named") {
		t.Errorf("expected 'No module named' in error, got: %s", err.Error())
	}
}

func TestFromImportNameError(t *testing.T) {
	r := New()
	err := r.ExecString(`from sys import nonexistent_attr`, "<test>")
	if err == nil {
		t.Fatal("expected ImportError, got nil")
	}
	if !strings.Contains(err.Error(), "cannot import name") {
		t.Errorf("expected 'cannot import name' in error, got: %s", err.Error())
	}
}

func TestModuleCache(t *testing.T) {
	// Importing the same module twice should return the cached version.
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`import sys
import sys
print(sys.version)`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "PyGo") {
		t.Errorf("expected sys.version to contain 'PyGo', got %q", got)
	}
}

func TestModuleCachePreventsReExecution(t *testing.T) {
	// Module caching must prevent re-execution of module code.
	// The module prints a side-effect line; importing it twice should only
	// produce that output once.
	tmpDir := t.TempDir()
	modContent := `counter = 0
counter = counter + 1
`
	err := os.WriteFile(filepath.Join(tmpDir, "counter_mod.py"), []byte(modContent), 0644)
	if err != nil {
		t.Fatalf("failed to write temp module: %v", err)
	}

	var buf bytes.Buffer
	r := New(WithOutput(&buf), WithSearchPath([]string{tmpDir}))

	execErr := r.ExecString(`import counter_mod
first = counter_mod.counter
import counter_mod
second = counter_mod.counter
print(first)
print(second)`, "<test>")
	if execErr != nil {
		t.Fatalf("ExecString failed: %v", execErr)
	}

	// Both imports return the same cached module, so counter stays at 1.
	got := buf.String()
	want := "1\n1\n"
	if got != want {
		t.Errorf("output = %q, want %q (module should not re-execute)", got, want)
	}
}

func TestImportSysExit(t *testing.T) {
	r := New()
	err := r.ExecString(`import sys
sys.exit(0)`, "<test>")
	if err == nil {
		t.Fatal("expected SystemExit, got nil")
	}
	if !strings.Contains(err.Error(), "SystemExit") {
		t.Errorf("expected 'SystemExit' in error, got: %s", err.Error())
	}
}

func TestImportResolutionOrder(t *testing.T) {
	// Verifies resolution order: cache > builtin > file.
	// If a file named "sys.py" exists on the search path, the builtin "sys"
	// module should still take priority (builtins before file search).
	tmpDir := t.TempDir()
	// Create a file-based sys.py that would set a different version.
	modContent := `version = "fake-sys"
`
	err := os.WriteFile(filepath.Join(tmpDir, "sys.py"), []byte(modContent), 0644)
	if err != nil {
		t.Fatalf("failed to write temp module: %v", err)
	}

	var buf bytes.Buffer
	r := New(WithOutput(&buf), WithSearchPath([]string{tmpDir}))

	execErr := r.ExecString(`import sys
print(sys.version)`, "<test>")
	if execErr != nil {
		t.Fatalf("ExecString failed: %v", execErr)
	}

	// The builtin sys module should win over the file-based sys.py.
	got := buf.String()
	if !strings.Contains(got, "PyGo") {
		t.Errorf("expected builtin sys.version containing 'PyGo', got %q (file module took priority over builtin)", got)
	}
}

func TestDunderNameIsMain(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	err := r.ExecString(`print(__name__)`, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "__main__" {
		t.Errorf("__name__ = %q, want %q", got, "__main__")
	}
}

func TestImportFileModule(t *testing.T) {
	// Create a temporary directory with a .py module file.
	tmpDir := t.TempDir()
	modContent := `x = 42
greeting = "hello from module"
`
	err := os.WriteFile(filepath.Join(tmpDir, "mymod.py"), []byte(modContent), 0644)
	if err != nil {
		t.Fatalf("failed to write temp module: %v", err)
	}

	var buf bytes.Buffer
	r := New(WithOutput(&buf), WithSearchPath([]string{tmpDir}))

	execErr := r.ExecString(`import mymod
print(mymod.x)
print(mymod.greeting)`, "<test>")
	if execErr != nil {
		t.Fatalf("ExecString failed: %v", execErr)
	}

	got := buf.String()
	want := "42\nhello from module\n"
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestFromFileModuleImport(t *testing.T) {
	// Create a temporary directory with a .py module file.
	tmpDir := t.TempDir()
	modContent := `value = 99
`
	err := os.WriteFile(filepath.Join(tmpDir, "helper.py"), []byte(modContent), 0644)
	if err != nil {
		t.Fatalf("failed to write temp module: %v", err)
	}

	var buf bytes.Buffer
	r := New(WithOutput(&buf), WithSearchPath([]string{tmpDir}))

	execErr := r.ExecString(`from helper import value
print(value)`, "<test>")
	if execErr != nil {
		t.Fatalf("ExecString failed: %v", execErr)
	}

	got := strings.TrimSpace(buf.String())
	if got != "99" {
		t.Errorf("output = %q, want %q", got, "99")
	}
}
