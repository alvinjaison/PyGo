package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pygo/pygo/internal/object"
)

// writeModule writes a .py file into dir and returns dir.
func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runInDir(t *testing.T, dir, src string) string {
	t.Helper()
	var buf bytes.Buffer
	rt := New(Config{Stdout: &buf, SearchPath: []string{dir}})
	if err := rt.RunString(context.Background(), src, "<test>"); err != nil {
		t.Fatalf("error: %v", err)
	}
	return buf.String()
}

func TestImportUserModule(t *testing.T) {
	dir := writeTemp(t, "mymod.py", "VALUE = 42\n\ndef double(x):\n    return x * 2\n")
	out := runInDir(t, dir, "import mymod\nprint(mymod.VALUE)\nprint(mymod.double(21))\n")
	if out != "42\n42\n" {
		t.Errorf("got %q", out)
	}
}

func TestFromUserModule(t *testing.T) {
	dir := writeTemp(t, "util.py", "def add(a, b):\n    return a + b\n\nNAME = 'util'\n")
	out := runInDir(t, dir, "from util import add, NAME\nprint(add(2, 3))\nprint(NAME)\n")
	if out != "5\nutil\n" {
		t.Errorf("got %q", out)
	}
}

func TestModuleCaching(t *testing.T) {
	// Importing the same module twice should execute its body only once.
	dir := writeTemp(t, "once.py", "print('loading once')\nX = 1\n")
	out := runInDir(t, dir, "import once\nimport once\nprint(once.X)\n")
	if out != "loading once\n1\n" {
		t.Errorf("module body ran more than once or X wrong: %q", out)
	}
}

func TestHostRegisteredModuleImport(t *testing.T) {
	var buf bytes.Buffer
	rt := New(Config{Stdout: &buf})

	mod := object.NewModule("hostmod")
	_ = mod.SetAttr("version", object.StrFrom("1.0"))
	_ = mod.SetAttr("answer", object.IntFrom(42))
	rt.RegisterModule(mod)

	src := "import hostmod\nprint(hostmod.version, hostmod.answer)\n"
	if err := rt.RunString(context.Background(), src, "<t>"); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "1.0 42\n" {
		t.Errorf("got %q", got)
	}
}
