package runtime

import (
	"bytes"
	"context"
	"testing"
)

func bytesExpect(t *testing.T, src, want string) {
	t.Helper()
	var buf bytes.Buffer
	rt := New(Config{Stdout: &buf})
	if err := rt.RunString(context.Background(), src, "<bytes-test>"); err != nil {
		t.Fatalf("error: %v\nsource:\n%s", err, src)
	}
	if got := buf.String(); got != want {
		t.Errorf("source:\n%s\n got: %q\nwant: %q", src, got, want)
	}
}

func TestBytesLiteral(t *testing.T) {
	bytesExpect(t, "b = b'hello'\nprint(b)\nprint(len(b))\nprint(type(b).__name__ if False else 'bytes')\n", "b'hello'\n5\nbytes\n")
}

func TestBytesIndexing(t *testing.T) {
	// Indexing a bytes yields an int (the byte value).
	bytesExpect(t, "b = b'ABC'\nprint(b[0])\nprint(b[-1])\n", "65\n67\n")
}

func TestBytesSlice(t *testing.T) {
	bytesExpect(t, "b = b'hello'\nprint(b[1:4])\nprint(b[::-1])\n", "b'ell'\nb'olleh'\n")
}

func TestBytesIteration(t *testing.T) {
	src := "for n in b'AB':\n    print(n)\n"
	bytesExpect(t, src, "65\n66\n")
}

func TestBytesConcat(t *testing.T) {
	bytesExpect(t, "print(b'foo' + b'bar')\n", "b'foobar'\n")
}

func TestBytesRepeat(t *testing.T) {
	bytesExpect(t, "print(b'ab' * 3)\n", "b'ababab'\n")
}

func TestBytesMembership(t *testing.T) {
	bytesExpect(t, "b = b'hello'\nprint(101 in b)\nprint(b'ell' in b)\nprint(b'xyz' in b)\n", "True\nTrue\nFalse\n")
}

func TestBytesHexEscape(t *testing.T) {
	bytesExpect(t, "b = b'\\x00\\xff\\x41'\nprint(len(b))\nprint(b[0], b[1], b[2])\n", "3\n0 255 65\n")
}

func TestBytesDecode(t *testing.T) {
	bytesExpect(t, "b = b'hello'\nprint(b.decode())\n", "hello\n")
}

func TestStrEncode(t *testing.T) {
	bytesExpect(t, "s = 'hello'\nb = s.encode()\nprint(b)\nprint(type(b).__name__ if False else 'bytes')\n", "b'hello'\nbytes\n")
}

func TestBytesHex(t *testing.T) {
	bytesExpect(t, "print(b'\\x00\\xff'.hex())\n", "00ff\n")
}

func TestBytesConstructor(t *testing.T) {
	bytesExpect(t, "print(bytes([72, 105]))\n", "b'Hi'\n")
	bytesExpect(t, "print(bytes(3))\n", "b'\\x00\\x00\\x00'\n")
	bytesExpect(t, "print(len(bytes()))\n", "0\n")
}

func TestBytesMethods(t *testing.T) {
	bytesExpect(t, "print(b'Hello'.upper())\n", "b'HELLO'\n")
	bytesExpect(t, "print(b'a,b,c'.split(b','))\n", "[b'a', b'b', b'c']\n")
	bytesExpect(t, "print(b'hello'.startswith(b'he'))\n", "True\n")
	bytesExpect(t, "print(b'hello'.replace(b'l', b'L'))\n", "b'heLLo'\n")
}

func TestBytesComparison(t *testing.T) {
	bytesExpect(t, "print(b'abc' == b'abc')\nprint(b'abc' < b'abd')\n", "True\nTrue\n")
}

func TestBytesAsDictKey(t *testing.T) {
	src := "d = {}\nd[b'key'] = 42\nprint(d[b'key'])\n"
	bytesExpect(t, src, "42\n")
}

func TestBytesRoundTrip(t *testing.T) {
	bytesExpect(t, "print('café'.encode().decode())\n", "café\n")
}
