package runtime

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCompat runs all Python scripts in testdata/compat/ under PyGo and
// compares stdout against the corresponding .expected file.
//
// This is the compatibility testing infrastructure for measuring Python 3
// compatibility against CPython as a test oracle. Each .py file tests one
// category of features, and the .expected output is generated from CPython
// or hand-written based on Python semantics.
//
// Scripts can be skipped by placing "# pygo-skip: <reason>" as the first line.
func TestCompat(t *testing.T) {
	// Locate testdata/compat/ relative to the project root.
	// From internal/runtime/, we go up two directories.
	compatDir := filepath.Join("..", "..", "testdata", "compat")

	entries, err := os.ReadDir(compatDir)
	if err != nil {
		t.Fatalf("failed to read compat directory %s: %v", compatDir, err)
	}

	var pyFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".py") {
			pyFiles = append(pyFiles, e.Name())
		}
	}

	if len(pyFiles) == 0 {
		t.Fatal("no .py files found in testdata/compat/")
	}

	for _, pyFile := range pyFiles {
		name := strings.TrimSuffix(pyFile, ".py")
		t.Run(name, func(t *testing.T) {
			pyPath := filepath.Join(compatDir, pyFile)
			expectedPath := filepath.Join(compatDir, name+".expected")

			// Read the Python source.
			source, err := os.ReadFile(pyPath)
			if err != nil {
				t.Fatalf("failed to read %s: %v", pyPath, err)
			}
			sourceStr := strings.ReplaceAll(string(source), "\r\n", "\n")

			// Check for pygo-skip marker on the first line.
			firstLine := strings.SplitN(sourceStr, "\n", 2)[0]
			if strings.HasPrefix(firstLine, "# pygo-skip:") {
				reason := strings.TrimPrefix(firstLine, "# pygo-skip:")
				reason = strings.TrimSpace(reason)
				t.Skipf("skipped: %s", reason)
			}

			// Read the expected output (normalize line endings for cross-platform).
			expectedBytes, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("failed to read expected output %s: %v", expectedPath, err)
			}
			expected := strings.ReplaceAll(string(expectedBytes), "\r\n", "\n")

			// Execute under PyGo.
			var buf bytes.Buffer
			r := New(WithOutput(&buf))
			execErr := r.ExecString(sourceStr, pyFile)
			if execErr != nil {
				t.Fatalf("execution error in %s:\n%v", pyFile, execErr)
			}

			// Compare output.
			got := buf.String()
			if got != expected {
				t.Errorf("output mismatch for %s\n\n--- got ---\n%s\n--- expected ---\n%s\n--- diff ---\n%s",
					pyFile, got, expected, lineDiff(got, expected))
			}
		})
	}
}

// lineDiff produces a simple line-by-line diff showing where outputs diverge.
func lineDiff(got, expected string) string {
	gotLines := strings.Split(got, "\n")
	expLines := strings.Split(expected, "\n")

	var diff strings.Builder
	maxLines := len(gotLines)
	if len(expLines) > maxLines {
		maxLines = len(expLines)
	}

	diffCount := 0
	for i := 0; i < maxLines; i++ {
		var g, e string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(expLines) {
			e = expLines[i]
		}
		if g != e {
			diffCount++
			if diffCount > 10 {
				diff.WriteString("  ... (more differences omitted)\n")
				break
			}
			diff.WriteString("  line ")
			diff.WriteString(itoa(i + 1))
			diff.WriteString(":\n")
			diff.WriteString("    got:      ")
			diff.WriteString(reprStr(g))
			diff.WriteString("\n")
			diff.WriteString("    expected: ")
			diff.WriteString(reprStr(e))
			diff.WriteString("\n")
		}
	}
	if diffCount == 0 {
		diff.WriteString("  (no visible line differences — possible trailing whitespace/newline issue)\n")
	}
	return diff.String()
}

// itoa converts an integer to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// reprStr returns a Go-syntax quoted string for clear diff output.
func reprStr(s string) string {
	return "\"" + strings.ReplaceAll(
		strings.ReplaceAll(
			strings.ReplaceAll(s, "\\", "\\\\"),
			"\"", "\\\""),
		"\n", "\\n") + "\""
}
