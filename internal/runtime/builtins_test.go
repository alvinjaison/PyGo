package runtime

import (
	"bytes"
	"strings"
	"testing"
)

// ============================================================
// len() tests
// ============================================================

func TestBuiltinLen(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"list", `print(len([1, 2, 3]))`, "3\n"},
		{"empty_list", `print(len([]))`, "0\n"},
		{"string", `print(len("hello"))`, "5\n"},
		{"empty_string", `print(len(""))`, "0\n"},
		{"tuple", `print(len((1, 2, 3, 4)))`, "4\n"},
		{"dict", `print(len({"a": 1, "b": 2}))`, "2\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := New(WithOutput(&buf))
			err := r.ExecString(tt.source, "<test>")
			if err != nil {
				t.Fatalf("ExecString failed: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuiltinLen_TypeError(t *testing.T) {
	r := New()
	err := r.ExecString(`len(42)`, "<test>")
	if err == nil {
		t.Fatal("expected TypeError for len(42)")
	}
	if !strings.Contains(err.Error(), "no len()") {
		t.Errorf("expected 'no len()' in error, got: %s", err.Error())
	}
}

// ============================================================
// range() tests
// ============================================================

func TestBuiltinRange(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"range_stop", `
result = 0
for i in range(5):
    result = result + i
print(result)`, "10\n"},
		{"range_start_stop", `
result = 0
for i in range(2, 5):
    result = result + i
print(result)`, "9\n"},
		{"range_start_stop_step", `
result = 0
for i in range(0, 10, 2):
    result = result + i
print(result)`, "20\n"},
		{"range_negative_step", `
result = 0
for i in range(5, 0, -1):
    result = result + i
print(result)`, "15\n"},
		{"range_empty", `
result = 0
for i in range(0):
    result = result + 1
print(result)`, "0\n"},
		{"range_repr", `print(range(10))`, "range(0, 10)\n"},
		{"range_repr_step", `print(range(1, 10, 2))`, "range(1, 10, 2)\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := New(WithOutput(&buf))
			err := r.ExecString(tt.source, "<test>")
			if err != nil {
				t.Fatalf("ExecString failed: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuiltinRange_ZeroStep(t *testing.T) {
	r := New()
	err := r.ExecString(`range(1, 10, 0)`, "<test>")
	if err == nil {
		t.Fatal("expected ValueError for range with step=0")
	}
	if !strings.Contains(err.Error(), "must not be zero") {
		t.Errorf("expected 'must not be zero' in error, got: %s", err.Error())
	}
}

// ============================================================
// int() tests
// ============================================================

func TestBuiltinInt(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"no_args", `print(int())`, "0\n"},
		{"from_int", `print(int(42))`, "42\n"},
		{"from_float", `print(int(3.7))`, "3\n"},
		{"from_neg_float", `print(int(-3.7))`, "-3\n"},
		{"from_str", `print(int("123"))`, "123\n"},
		{"from_str_neg", `print(int("-42"))`, "-42\n"},
		{"from_str_spaces", `print(int("  99  "))`, "99\n"},
		{"from_bool_true", `print(int(True))`, "1\n"},
		{"from_bool_false", `print(int(False))`, "0\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := New(WithOutput(&buf))
			err := r.ExecString(tt.source, "<test>")
			if err != nil {
				t.Fatalf("ExecString failed: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuiltinInt_ValueError(t *testing.T) {
	r := New()
	err := r.ExecString(`int("abc")`, "<test>")
	if err == nil {
		t.Fatal("expected ValueError for int('abc')")
	}
	if !strings.Contains(err.Error(), "invalid literal") {
		t.Errorf("expected 'invalid literal' in error, got: %s", err.Error())
	}
}

// ============================================================
// float() tests
// ============================================================

func TestBuiltinFloat(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"no_args", `print(float())`, "0\n"},
		{"from_int", `print(float(42))`, "42\n"},
		{"from_float", `print(float(3.14))`, "3.14\n"},
		{"from_str", `print(float("3.14"))`, "3.14\n"},
		{"from_bool_true", `print(float(True))`, "1\n"},
		{"from_bool_false", `print(float(False))`, "0\n"},
		{"from_str_inf", `print(float("inf"))`, "+Inf\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := New(WithOutput(&buf))
			err := r.ExecString(tt.source, "<test>")
			if err != nil {
				t.Fatalf("ExecString failed: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuiltinFloat_ValueError(t *testing.T) {
	r := New()
	err := r.ExecString(`float("xyz")`, "<test>")
	if err == nil {
		t.Fatal("expected ValueError for float('xyz')")
	}
	if !strings.Contains(err.Error(), "could not convert") {
		t.Errorf("expected 'could not convert' in error, got: %s", err.Error())
	}
}

// ============================================================
// str() tests
// ============================================================

func TestBuiltinStr(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"no_args", `print(str())`, "\n"},
		{"from_int", `print(str(42))`, "42\n"},
		{"from_float", `print(str(3.14))`, "3.14\n"},
		{"from_bool", `print(str(True))`, "True\n"},
		{"from_none", `print(str(None))`, "None\n"},
		{"from_str", `print(str("hello"))`, "hello\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := New(WithOutput(&buf))
			err := r.ExecString(tt.source, "<test>")
			if err != nil {
				t.Fatalf("ExecString failed: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

// ============================================================
// bool() tests
// ============================================================

func TestBuiltinBool(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"no_args", `print(bool())`, "False\n"},
		{"zero", `print(bool(0))`, "False\n"},
		{"nonzero", `print(bool(42))`, "True\n"},
		{"empty_str", `print(bool(""))`, "False\n"},
		{"nonempty_str", `print(bool("hi"))`, "True\n"},
		{"empty_list", `print(bool([]))`, "False\n"},
		{"nonempty_list", `print(bool([1]))`, "True\n"},
		{"none", `print(bool(None))`, "False\n"},
		{"true", `print(bool(True))`, "True\n"},
		{"false", `print(bool(False))`, "False\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := New(WithOutput(&buf))
			err := r.ExecString(tt.source, "<test>")
			if err != nil {
				t.Fatalf("ExecString failed: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

// ============================================================
// isinstance() tests
// ============================================================

func TestBuiltinIsinstance(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"int_is_int", `print(isinstance(42, int))`, "True\n"},
		{"str_is_str", `print(isinstance("hi", str))`, "True\n"},
		{"int_not_str", `print(isinstance(42, str))`, "False\n"},
		{"bool_is_int", `print(isinstance(True, int))`, "True\n"},
		{"list_is_list", `print(isinstance([1, 2], list))`, "True\n"},
		{"dict_is_dict", `print(isinstance({"a": 1}, dict))`, "True\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := New(WithOutput(&buf))
			err := r.ExecString(tt.source, "<test>")
			if err != nil {
				t.Fatalf("ExecString failed: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

// ============================================================
// repr() tests
// ============================================================

func TestBuiltinRepr(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"int", `print(repr(42))`, "42\n"},
		{"float", `print(repr(3.14))`, "3.14\n"},
		{"bool", `print(repr(True))`, "True\n"},
		{"none", `print(repr(None))`, "None\n"},
		{"string", `print(repr("hello"))`, "'hello'\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := New(WithOutput(&buf))
			err := r.ExecString(tt.source, "<test>")
			if err != nil {
				t.Fatalf("ExecString failed: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

// ============================================================
// abs() tests
// ============================================================

func TestBuiltinAbs(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"positive_int", `print(abs(5))`, "5\n"},
		{"negative_int", `print(abs(-5))`, "5\n"},
		{"zero", `print(abs(0))`, "0\n"},
		{"positive_float", `print(abs(3.14))`, "3.14\n"},
		{"negative_float", `print(abs(-3.14))`, "3.14\n"},
		{"bool_true", `print(abs(True))`, "1\n"},
		{"bool_false", `print(abs(False))`, "0\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := New(WithOutput(&buf))
			err := r.ExecString(tt.source, "<test>")
			if err != nil {
				t.Fatalf("ExecString failed: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuiltinAbs_TypeError(t *testing.T) {
	r := New()
	err := r.ExecString(`abs("hello")`, "<test>")
	if err == nil {
		t.Fatal("expected TypeError for abs('hello')")
	}
	if !strings.Contains(err.Error(), "bad operand type") {
		t.Errorf("expected 'bad operand type' in error, got: %s", err.Error())
	}
}

// ============================================================
// min() / max() tests
// ============================================================

func TestBuiltinMin(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"two_args", `print(min(3, 1))`, "1\n"},
		{"three_args", `print(min(3, 1, 2))`, "1\n"},
		{"list_arg", `print(min([5, 2, 8, 1]))`, "1\n"},
		{"negative", `print(min(-1, -5, 3))`, "-5\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := New(WithOutput(&buf))
			err := r.ExecString(tt.source, "<test>")
			if err != nil {
				t.Fatalf("ExecString failed: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuiltinMax(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"two_args", `print(max(3, 1))`, "3\n"},
		{"three_args", `print(max(3, 1, 2))`, "3\n"},
		{"list_arg", `print(max([5, 2, 8, 1]))`, "8\n"},
		{"negative", `print(max(-1, -5, 3))`, "3\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := New(WithOutput(&buf))
			err := r.ExecString(tt.source, "<test>")
			if err != nil {
				t.Fatalf("ExecString failed: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuiltinMin_EmptySequence(t *testing.T) {
	r := New()
	err := r.ExecString(`min([])`, "<test>")
	if err == nil {
		t.Fatal("expected ValueError for min([])")
	}
	if !strings.Contains(err.Error(), "empty sequence") {
		t.Errorf("expected 'empty sequence' in error, got: %s", err.Error())
	}
}

// ============================================================
// Integration test: combining builtins
// ============================================================

func TestBuiltins_Integration(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))

	source := `
# Test range with len
r = range(10)
print(len(r))

# Test type conversions
print(int("42") + 8)
print(float(3) + 0.14)

# Test abs with min/max
nums = [-5, -2, 3, 7, -1]
print(abs(min(nums)))
print(max(nums))

# Test bool conversions
print(bool(0))
print(bool(1))

# Test str
print(str(100))

# Test repr
print(repr("hi"))
`
	err := r.ExecString(source, "<test>")
	if err != nil {
		t.Fatalf("ExecString failed: %v", err)
	}

	want := "10\n50\n3.14\n5\n7\nFalse\nTrue\n100\n'hi'\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
