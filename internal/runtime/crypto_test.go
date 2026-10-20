package runtime

import (
	"bytes"
	"context"
	"testing"
)

func cryptoExpect(t *testing.T, src, want string) {
	t.Helper()
	var buf bytes.Buffer
	rt := New(Config{Stdout: &buf})
	if err := rt.RunString(context.Background(), src, "<crypto-test>"); err != nil {
		t.Fatalf("error: %v\nsource:\n%s", err, src)
	}
	if got := buf.String(); got != want {
		t.Errorf("source:\n%s\n got: %q\nwant: %q", src, got, want)
	}
}

func TestHashlibSHA256(t *testing.T) {
	// Known SHA-256 of "hello".
	src := `import hashlib
h = hashlib.sha256(b"hello")
print(h.hexdigest())
`
	cryptoExpect(t, src, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824\n")
}

func TestHashlibMD5(t *testing.T) {
	// Known MD5 of the empty string.
	src := `import hashlib
print(hashlib.md5(b"").hexdigest())
`
	cryptoExpect(t, src, "d41d8cd98f00b204e9800998ecf8427e\n")
}

func TestHashlibSHA1(t *testing.T) {
	// Known SHA-1 of "abc".
	src := `import hashlib
print(hashlib.sha1(b"abc").hexdigest())
`
	cryptoExpect(t, src, "a9993e364706816aba3e25717850c26c9cd0d89d\n")
}

func TestHashlibUpdate(t *testing.T) {
	// update() accumulates: hashing "hello" in one go vs in pieces.
	src := `import hashlib
h = hashlib.sha256()
h.update(b"hel")
h.update(b"lo")
print(h.hexdigest())
`
	cryptoExpect(t, src, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824\n")
}

func TestHashlibName(t *testing.T) {
	src := `import hashlib
h = hashlib.sha256()
print(h.name)
print(h.digest_size)
`
	cryptoExpect(t, src, "sha256\n32\n")
}

func TestHashlibNew(t *testing.T) {
	src := `import hashlib
h = hashlib.new("md5", b"")
print(h.hexdigest())
`
	cryptoExpect(t, src, "d41d8cd98f00b204e9800998ecf8427e\n")
}

func TestHashlibDigestIsBytes(t *testing.T) {
	src := `import hashlib
d = hashlib.md5(b"").digest()
print(len(d))
`
	cryptoExpect(t, src, "16\n")
}

func TestBase64Encode(t *testing.T) {
	src := `import base64
print(base64.b64encode(b"hello"))
`
	cryptoExpect(t, src, "b'aGVsbG8='\n")
}

func TestBase64Decode(t *testing.T) {
	src := `import base64
print(base64.b64decode(b"aGVsbG8="))
`
	cryptoExpect(t, src, "b'hello'\n")
}

func TestBase64RoundTrip(t *testing.T) {
	src := `import base64
original = b"PyGo rules!"
encoded = base64.b64encode(original)
decoded = base64.b64decode(encoded)
print(decoded == original)
print(decoded.decode())
`
	cryptoExpect(t, src, "True\nPyGo rules!\n")
}

func TestBase64URLSafe(t *testing.T) {
	src := `import base64
# Bytes that produce + and / in standard base64 become - and _ url-safe.
data = bytes([251, 255, 191])
print(base64.urlsafe_b64encode(data))
`
	cryptoExpect(t, src, "b'-_-_'\n")
}

func TestBase64Hex16(t *testing.T) {
	src := `import base64
print(base64.b16encode(b"AB"))
print(base64.b16decode(b"4142"))
`
	cryptoExpect(t, src, "b'4142'\nb'AB'\n")
}

func TestBase64FromString(t *testing.T) {
	// decode accepts a str as well as bytes.
	src := `import base64
print(base64.b64decode("aGVsbG8=").decode())
`
	cryptoExpect(t, src, "hello\n")
}

func TestHashlibBase64Combo(t *testing.T) {
	// A realistic combination: base64-encode a sha256 digest.
	src := `import hashlib
import base64
digest = hashlib.sha256(b"data").digest()
print(len(base64.b64encode(digest)))
`
	cryptoExpect(t, src, "44\n")
}
