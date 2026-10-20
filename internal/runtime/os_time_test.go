package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runOut(t *testing.T, cfg Config, src string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cfg.Stdout = &buf
	rt := New(cfg)
	err := rt.RunString(context.Background(), src, "<t>")
	return buf.String(), err
}

func TestOSPathJoin(t *testing.T) {
	out, err := runOut(t, Config{}, "import os\nprint(os.path.join('a', 'b', 'c.txt'))\n")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("a", "b", "c.txt") + "\n"
	if out != want {
		t.Errorf("got %q want %q", out, want)
	}
}

func TestOSPathPredicates(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(f, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "import os\n" +
		"print(os.path.exists(" + pyStr(f) + "))\n" +
		"print(os.path.isfile(" + pyStr(f) + "))\n" +
		"print(os.path.isdir(" + pyStr(dir) + "))\n" +
		"print(os.path.basename(" + pyStr(f) + "))\n"
	out, err := runOut(t, Config{}, src)
	if err != nil {
		t.Fatal(err)
	}
	want := "True\nTrue\nTrue\nfile.txt\n"
	if out != want {
		t.Errorf("got %q want %q", out, want)
	}
}

func TestOSPathSplitext(t *testing.T) {
	out, err := runOut(t, Config{}, "import os\nprint(os.path.splitext('archive.tar.gz'))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "('archive.tar', '.gz')\n" {
		t.Errorf("got %q", out)
	}
}

func TestOSGetenv(t *testing.T) {
	t.Setenv("PYGO_TEST_VAR", "hello")
	out, err := runOut(t, Config{}, "import os\nprint(os.getenv('PYGO_TEST_VAR'))\nprint(os.getenv('PYGO_MISSING', 'default'))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello\ndefault\n" {
		t.Errorf("got %q", out)
	}
}

func TestOSListdir(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), nil, 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.txt"), nil, 0o644)
	out, err := runOut(t, Config{}, "import os\nprint(sorted(os.listdir("+pyStr(dir)+")))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "['a.txt', 'b.txt']\n" {
		t.Errorf("got %q", out)
	}
}

func TestOSRestricted(t *testing.T) {
	// In restricted mode, importing os must fail but safe modules still work.
	_, err := runOut(t, Config{Restricted: true}, "import os\n")
	if err == nil {
		t.Fatal("expected ImportError for os in restricted mode")
	}
	if !strings.Contains(err.Error(), "restricted") {
		t.Errorf("unexpected error: %v", err)
	}
	// math should still import in restricted mode.
	out, err := runOut(t, Config{Restricted: true}, "import math\nprint(math.floor(2.9))\n")
	if err != nil {
		t.Fatalf("math should import in restricted mode: %v", err)
	}
	if out != "2\n" {
		t.Errorf("got %q", out)
	}
}

func TestTimeMonotonic(t *testing.T) {
	// monotonic() returns a float; a simple sanity check via arithmetic.
	out, err := runOut(t, Config{}, "import time\nt = time.monotonic()\nprint(t >= 0.0)\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "True\n" {
		t.Errorf("got %q", out)
	}
}

func TestTimeSleep(t *testing.T) {
	// A zero-length sleep should return None without error.
	out, err := runOut(t, Config{}, "import time\ntime.sleep(0)\nprint('ok')\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "ok\n" {
		t.Errorf("got %q", out)
	}
}

// pyStr renders a Go string as a Python single-quoted literal for embedding
// into test source. Backslashes are escaped so Windows-style paths survive.
func pyStr(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return "'" + s + "'"
}
