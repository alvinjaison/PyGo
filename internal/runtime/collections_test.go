package runtime

import (
	"bytes"
	"testing"
)

// ============================================================
// List operations tests
// ============================================================

func TestExecString_ListAppend(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [1, 2, 3]
x.append(4)
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[1, 2, 3, 4]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ListPop(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [1, 2, 3, 4]
v = x.pop()
print(v)
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "4\n[1, 2, 3]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ListPopIndex(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [10, 20, 30]
v = x.pop(0)
print(v)
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "10\n[20, 30]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ListClear(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [1, 2, 3]
x.clear()
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ListExtend(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [1, 2]
x.extend([3, 4, 5])
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[1, 2, 3, 4, 5]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ListSort(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [3, 1, 4, 1, 5]
x.sort()
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[1, 1, 3, 4, 5]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ListReverse(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [1, 2, 3, 4]
x.reverse()
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[4, 3, 2, 1]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ListInsert(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [1, 2, 3]
x.insert(1, 99)
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[1, 99, 2, 3]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ListRemove(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [1, 2, 3, 2, 1]
x.remove(2)
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[1, 3, 2, 1]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ListIndex(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [10, 20, 30, 40]
print(x.index(30))`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "2\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ListCount(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [1, 2, 2, 3, 2]
print(x.count(2))`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "3\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ListCopy(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [1, 2, 3]
y = x.copy()
x.append(4)
print(y)
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[1, 2, 3]\n[1, 2, 3, 4]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// ============================================================
// List operator tests (+, *)
// ============================================================

func TestExecString_ListConcatenation(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [1, 2] + [3, 4]
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[1, 2, 3, 4]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ListRepetition(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [1, 2] * 3
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[1, 2, 1, 2, 1, 2]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_ListRepetitionRMul(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = 2 * [5, 6]
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[5, 6, 5, 6]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// ============================================================
// Tuple operator tests (+, *)
// ============================================================

func TestExecString_TupleConcatenation(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = (1, 2) + (3, 4)
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "(1, 2, 3, 4)\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_TupleRepetition(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = (1, 2) * 2
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "(1, 2, 1, 2)\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_TupleRepetitionRMul(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = 3 * (7,)
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "(7, 7, 7)\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// ============================================================
// Dict method tests
// ============================================================

func TestExecString_DictKeys(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `d = {"a": 1, "b": 2}
k = d.keys()
print(k)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "['a', 'b']\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_DictValues(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `d = {"x": 10, "y": 20}
v = d.values()
print(v)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[10, 20]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_DictItems(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `d = {"a": 1, "b": 2}
items = d.items()
print(items)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[('a', 1), ('b', 2)]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_DictGet(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `d = {"a": 1, "b": 2}
print(d.get("a"))
print(d.get("z"))
print(d.get("z", 99))`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "1\nNone\n99\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_DictPop(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `d = {"a": 1, "b": 2, "c": 3}
v = d.pop("b")
print(v)
print(d)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "2\n{'a': 1, 'c': 3}\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_DictClear(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `d = {"a": 1, "b": 2}
d.clear()
print(d)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "{}\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_DictSetdefault(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `d = {"a": 1}
v1 = d.setdefault("a", 99)
v2 = d.setdefault("b", 42)
print(v1)
print(v2)
print(d)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "1\n42\n{'a': 1, 'b': 42}\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_DictCopy(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `d = {"a": 1, "b": 2}
d2 = d.copy()
d["c"] = 3
print(d2)
print(d)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "{'a': 1, 'b': 2}\n{'a': 1, 'b': 2, 'c': 3}\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// ============================================================
// Dict contains (in operator)
// ============================================================

func TestExecString_DictContains(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `d = {"a": 1, "b": 2}
print("a" in d)
print("z" in d)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "True\nFalse\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// ============================================================
// List contains (in operator)
// ============================================================

func TestExecString_ListContains(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [1, 2, 3]
print(2 in x)
print(5 in x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "True\nFalse\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// ============================================================
// Delete operations (del)
// ============================================================

func TestExecString_DelListIndex(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `x = [1, 2, 3, 4]
del x[1]
print(x)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "[1, 3, 4]\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestExecString_DelDictKey(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `d = {"a": 1, "b": 2, "c": 3}
del d["b"]
print(d)`

	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}
	want := "{'a': 1, 'c': 3}\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
