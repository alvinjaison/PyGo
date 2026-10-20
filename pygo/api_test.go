package pygo_test

import (
	"testing"

	"github.com/pygo/pygo/pygo"
)

func TestPublicRunString(t *testing.T) {
	rt := pygo.New()
	if err := rt.RunString(`x = 2 + 3`); err != nil {
		t.Fatal(err)
	}
	v, err := rt.Get("x")
	if err != nil {
		t.Fatal(err)
	}
	n, _ := v.Int()
	if n != 5 {
		t.Errorf("got %d want 5", n)
	}
}

func TestPublicEval(t *testing.T) {
	rt := pygo.New()
	v, err := rt.Eval("10 * 10 + 1")
	if err != nil {
		t.Fatal(err)
	}
	n, _ := v.Int()
	if n != 101 {
		t.Errorf("got %d want 101", n)
	}
}

func TestPublicSetGet(t *testing.T) {
	rt := pygo.New()
	if err := rt.Set("config", map[string]interface{}{"port": 8080}); err != nil {
		t.Fatal(err)
	}
	v, err := rt.Eval(`config["port"]`)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := v.Int()
	if n != 8080 {
		t.Errorf("got %d want 8080", n)
	}
}

func TestPublicRegisterFunction(t *testing.T) {
	rt := pygo.New()
	rt.MustRegisterFunction("add", func(a, b int) int { return a + b })
	v, err := rt.Eval("add(6, 7)")
	if err != nil {
		t.Fatal(err)
	}
	n, _ := v.Int()
	if n != 13 {
		t.Errorf("got %d want 13", n)
	}
}

func TestPublicRegisterFunctionError(t *testing.T) {
	rt := pygo.New()
	rt.MustRegisterFunction("boom", func() error {
		return &myErr{}
	})
	err := rt.RunString(`boom()`)
	if !pygo.IsError(err, "RuntimeError") {
		t.Errorf("expected RuntimeError, got %v", err)
	}
}

type myErr struct{}

func (myErr) Error() string { return "kaboom" }

func TestPublicCallByName(t *testing.T) {
	rt := pygo.New()
	if err := rt.RunString("def square(n):\n    return n * n\n"); err != nil {
		t.Fatal(err)
	}
	v, err := rt.CallByName("square", 9)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := v.Int()
	if n != 81 {
		t.Errorf("got %d want 81", n)
	}
}

func TestPublicRegisterStruct(t *testing.T) {
	type Config struct {
		Debug bool
		Port  int
	}
	rt := pygo.New()
	rt.MustRegister("cfg", &Config{Debug: true, Port: 8080})
	v, err := rt.Eval("cfg.Port")
	if err != nil {
		t.Fatal(err)
	}
	n, _ := v.Int()
	if n != 8080 {
		t.Errorf("got %d want 8080", n)
	}
}

func TestPublicException(t *testing.T) {
	rt := pygo.New()
	err := rt.RunString(`raise ValueError("bad")`)
	if !pygo.IsError(err, "ValueError") {
		t.Errorf("expected ValueError, got %v", err)
	}
}

func TestPublicListResult(t *testing.T) {
	rt := pygo.New()
	v, err := rt.Eval("[x * x for x in range(4)]")
	if err != nil {
		t.Fatal(err)
	}
	items, err := v.List()
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{0, 1, 4, 9}
	if len(items) != 4 {
		t.Fatalf("got %d items", len(items))
	}
	for i, it := range items {
		n, _ := it.Int()
		if n != want[i] {
			t.Errorf("item %d: got %d want %d", i, n, want[i])
		}
	}
}
