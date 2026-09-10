package runtime

import (
	"bytes"
	"strings"
	"testing"
)

// runJSON executes source with the json module available and returns trimmed
// stdout, failing the test on error.
func runJSON(t *testing.T, source string) string {
	t.Helper()
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	if err := r.ExecString(source, "<test>"); err != nil {
		t.Fatalf("ExecString failed: %v\nsource:\n%s", err, source)
	}
	return strings.TrimSpace(buf.String())
}

func TestJSONImport(t *testing.T) {
	// The module must import without error even when nothing is called yet.
	// This is the exact shape of the Emily "Dashboard Validation" script's
	// first line: import json,sys
	runJSON(t, "import json, sys")
}

func TestJSONLoadsScalars(t *testing.T) {
	cases := []struct{ src, want string }{
		{`print(json.loads("42"))`, "42"},
		{`print(json.loads("-7"))`, "-7"},
		{`print(json.loads("3.5"))`, "3.5"},
		{`print(json.loads('"hello"'))`, "hello"},
		{`print(json.loads("true"))`, "True"},
		{`print(json.loads("false"))`, "False"},
		{`print(json.loads("null"))`, "None"},
	}
	for _, c := range cases {
		got := runJSON(t, "import json\n"+c.src)
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

func TestJSONLoadsBigIntStaysInt(t *testing.T) {
	// A value beyond int64 must remain an int, not degrade to a float.
	got := runJSON(t, `import json
print(json.loads("123456789012345678901234567890"))`)
	if got != "123456789012345678901234567890" {
		t.Errorf("big int round-trip = %q", got)
	}
}

func TestJSONLoadsObjectAndAccess(t *testing.T) {
	got := runJSON(t, `import json
data = json.loads('{"values": [1, 2, 3], "name": "svc"}')
print(data["name"])
print(data["values"][1])
print(len(data["values"]))`)
	want := "svc\n2\n3"
	if got != want {
		t.Errorf("object access = %q, want %q", got, want)
	}
}

func TestJSONLoadsNestedManualSum(t *testing.T) {
	// Iterate a parsed array and total it without relying on the sum() builtin,
	// which PyGo has not implemented yet.
	got := runJSON(t, `import json
data = json.loads('{"values": [1, 2, 3]}')
total = 0
for n in data["values"]:
    total = total + n
print(total)`)
	if got != "6" {
		t.Errorf("nested manual sum = %q, want 6", got)
	}
}

func TestJSONDumpsCompact(t *testing.T) {
	got := runJSON(t, `import json
print(json.dumps({"a": 1, "b": [True, None, "x"]}))`)
	want := `{"a": 1, "b": [true, null, "x"]}`
	if got != want {
		t.Errorf("dumps = %q, want %q", got, want)
	}
}

func TestJSONDumpsSortKeys(t *testing.T) {
	got := runJSON(t, `import json
print(json.dumps({"b": 2, "a": 1, "c": 3}, sort_keys=True))`)
	want := `{"a": 1, "b": 2, "c": 3}`
	if got != want {
		t.Errorf("dumps sort_keys = %q, want %q", got, want)
	}
}

func TestJSONDumpsIndent(t *testing.T) {
	got := runJSON(t, `import json
print(json.dumps({"a": 1}, indent=2))`)
	want := "{\n  \"a\": 1\n}"
	if got != want {
		t.Errorf("dumps indent = %q, want %q", got, want)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	got := runJSON(t, `import json
original = '{"nums": [1, 2, 3], "flag": true}'
parsed = json.loads(original)
print(json.dumps(parsed, sort_keys=True))`)
	want := `{"flag": true, "nums": [1, 2, 3]}`
	if got != want {
		t.Errorf("round trip = %q, want %q", got, want)
	}
}

func TestJSONLoadsDecodeErrorIsValueError(t *testing.T) {
	// Malformed input must raise, and be catchable as ValueError (CPython makes
	// JSONDecodeError a ValueError subclass).
	got := runJSON(t, `import json
try:
    json.loads("{not valid}")
    print("no error")
except ValueError:
    print("caught ValueError")`)
	if got != "caught ValueError" {
		t.Errorf("decode error handling = %q", got)
	}
}

func TestJSONDumpsNonSerializableRaises(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`import json
json.dumps(json)`, "<test>")
	if err == nil {
		t.Fatal("expected TypeError dumping a non-serializable object, got nil")
	}
	if !strings.Contains(err.Error(), "not JSON serializable") {
		t.Errorf("error = %v, want 'not JSON serializable'", err)
	}
}

func TestJSONLoadsRejectsTrailingData(t *testing.T) {
	var buf bytes.Buffer
	r := New(WithOutput(&buf))
	err := r.ExecString(`import json
json.loads('{"a": 1} extra')`, "<test>")
	if err == nil {
		t.Fatal("expected error on trailing data, got nil")
	}
}
