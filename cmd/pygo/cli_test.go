package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// testBinary holds the path to the compiled pygo binary used in integration tests.
var testBinary string

func TestMain(m *testing.M) {
	// Build the binary into a temp directory.
	tmp, err := os.MkdirTemp("", "pygo-cli-test-*")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)

	bin := filepath.Join(tmp, "pygo")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic("failed to build pygo binary: " + err.Error() + "\nOutput: " + string(out))
	}

	testBinary = bin
	os.Exit(m.Run())
}

// TestCLI_ScriptExecution verifies that pygo executes a .py file and produces
// correct stdout output.
func TestCLI_ScriptExecution(t *testing.T) {
	script := writeTemp(t, "hello.py", `print("hello world")`)

	cmd := exec.Command(testBinary, script)
	stdout, stderr, exitCode := runCmd(t, cmd)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", exitCode, stderr)
	}
	if strings.TrimSpace(stdout) != "hello world" {
		t.Errorf("expected stdout %q, got %q", "hello world", stdout)
	}
}

// TestCLI_CommandFlag verifies that pygo -c "source" executes inline Python.
func TestCLI_CommandFlag(t *testing.T) {
	cmd := exec.Command(testBinary, "-c", `print(42)`)
	stdout, stderr, exitCode := runCmd(t, cmd)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", exitCode, stderr)
	}
	if strings.TrimSpace(stdout) != "42" {
		t.Errorf("expected stdout %q, got %q", "42\n", stdout)
	}
}

// TestCLI_MissingFile verifies that pygo reports an error and exits with code 2
// when given a non-existent file.
func TestCLI_MissingFile(t *testing.T) {
	cmd := exec.Command(testBinary, "/nonexistent_file_xyz.py")
	_, stderr, exitCode := runCmd(t, cmd)

	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}
	if !strings.Contains(stderr, "No such file or directory") {
		t.Errorf("expected stderr to contain 'No such file or directory', got: %q", stderr)
	}
}

// TestCLI_PythonException verifies that a Python exception produces a traceback
// on stderr and exits with code 1.
func TestCLI_PythonException(t *testing.T) {
	script := writeTemp(t, "raises.py", `raise ValueError("test error")`)

	cmd := exec.Command(testBinary, script)
	_, stderr, exitCode := runCmd(t, cmd)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d\nstderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stderr, "Traceback") {
		t.Errorf("expected stderr to contain 'Traceback', got: %q", stderr)
	}
	if !strings.Contains(stderr, "ValueError") {
		t.Errorf("expected stderr to contain 'ValueError', got: %q", stderr)
	}
	if !strings.Contains(stderr, "test error") {
		t.Errorf("expected stderr to contain 'test error', got: %q", stderr)
	}
}

// TestCLI_SysArgv verifies that sys.argv is populated correctly with the script
// name and trailing arguments.
func TestCLI_SysArgv(t *testing.T) {
	script := writeTemp(t, "argv_test.py", `
import sys
for arg in sys.argv:
    print(arg)
`)

	cmd := exec.Command(testBinary, script, "arg1", "arg2")
	stdout, stderr, exitCode := runCmd(t, cmd)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", exitCode, stderr)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines of output (script + 2 args), got %d: %v", len(lines), lines)
	}
	// First element should be the script path
	if lines[0] != script {
		t.Errorf("sys.argv[0] expected %q, got %q", script, lines[0])
	}
	if lines[1] != "arg1" {
		t.Errorf("sys.argv[1] expected %q, got %q", "arg1", lines[1])
	}
	if lines[2] != "arg2" {
		t.Errorf("sys.argv[2] expected %q, got %q", "arg2", lines[2])
	}
}

// TestCLI_VersionFlag verifies that pygo --version prints a version containing "PyGo".
func TestCLI_VersionFlag(t *testing.T) {
	cmd := exec.Command(testBinary, "--version")
	stdout, _, exitCode := runCmd(t, cmd)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "PyGo") {
		t.Errorf("expected version output to contain 'PyGo', got: %q", stdout)
	}
}

// --- helpers ---

// writeTemp creates a temporary file with the given content and returns its path.
// The file is automatically cleaned up when the test completes.
func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

// runCmd executes a command and returns stdout, stderr, and exit code.
func runCmd(t *testing.T, cmd *exec.Cmd) (stdout, stderr string, exitCode int) {
	t.Helper()

	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run command: %v", err)
		}
	}

	return stdout, stderr, exitCode
}
