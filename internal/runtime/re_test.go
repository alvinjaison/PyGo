package runtime

import (
	"bytes"
	"context"
	"testing"
)

func reExpect(t *testing.T, src, want string) {
	t.Helper()
	var buf bytes.Buffer
	rt := New(Config{Stdout: &buf})
	if err := rt.RunString(context.Background(), src, "<re-test>"); err != nil {
		t.Fatalf("error: %v\nsource:\n%s", err, src)
	}
	if got := buf.String(); got != want {
		t.Errorf("source:\n%s\n got: %q\nwant: %q", src, got, want)
	}
}

func TestReFindall(t *testing.T) {
	reExpect(t, `import re
print(re.findall(r'\d+', 'abc 123 def 456'))
`, "['123', '456']\n")
}

func TestReMatchGroup(t *testing.T) {
	reExpect(t, `import re
m = re.match(r'(\w+)@(\w+)', 'user@host')
print(m.group(0))
print(m.group(1))
print(m.group(2))
`, "user@host\nuser\nhost\n")
}

func TestReSearch(t *testing.T) {
	reExpect(t, `import re
m = re.search(r'\d+', 'abc 42 xyz')
print(m.group())
print(m.start(), m.end())
`, "42\n4 6\n")
}

func TestReMatchAnchored(t *testing.T) {
	// re.match only matches at the start.
	reExpect(t, `import re
m = re.match(r'\d+', 'abc 123')
print(m is None)
`, "True\n")
}

func TestReSub(t *testing.T) {
	reExpect(t, `import re
print(re.sub(r'\d+', '#', 'a1b22c333'))
`, "a#b#c#\n")
}

func TestReSubBackref(t *testing.T) {
	reExpect(t, `import re
print(re.sub(r'(\w+)@(\w+)', r'\2.\1', 'user@host'))
`, "host.user\n")
}

func TestReSplit(t *testing.T) {
	reExpect(t, `import re
print(re.split(r'\s*,\s*', 'a, b ,c,  d'))
`, "['a', 'b', 'c', 'd']\n")
}

func TestReCompile(t *testing.T) {
	reExpect(t, `import re
p = re.compile(r'[a-z]+')
print(p.findall('abc123def'))
print(p.match('hello').group())
`, "['abc', 'def']\n"+"hello\n")
}

func TestReGroups(t *testing.T) {
	reExpect(t, `import re
m = re.match(r'(\d+)-(\d+)-(\d+)', '2024-01-15')
print(m.groups())
`, "('2024', '01', '15')\n")
}

func TestReFindallGroups(t *testing.T) {
	reExpect(t, `import re
print(re.findall(r'(\w)(\d)', 'a1b2c3'))
`, "[('a', '1'), ('b', '2'), ('c', '3')]\n")
}
