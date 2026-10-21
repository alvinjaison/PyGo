package stdlib

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"

	"github.com/alvinjaison/PyGo/internal/object"
)

// base32StdEncoding matches Python's base64.b32encode (standard RFC 4648).
var base32StdEncoding = base32.StdEncoding

// buildBase64 constructs the `base64` module.
//
// Following CPython, the encode functions take and return bytes; decode
// accepts bytes or str and returns bytes.
func buildBase64(name string) *object.ModuleObject {
	mod, set := newModule(name)

	set("b64encode", encodeFn("base64.b64encode", base64.StdEncoding))
	set("b64decode", decodeFn("base64.b64decode", base64.StdEncoding))
	set("urlsafe_b64encode", encodeFn("base64.urlsafe_b64encode", base64.URLEncoding))
	set("urlsafe_b64decode", decodeFn("base64.urlsafe_b64decode", base64.URLEncoding))
	set("standard_b64encode", encodeFn("base64.standard_b64encode", base64.StdEncoding))
	set("standard_b64decode", decodeFn("base64.standard_b64decode", base64.StdEncoding))

	// base16 (hex).
	set("b16encode", fn("base64.b16encode", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		data, err := bytesLike(args, 0)
		if err != nil {
			return nil, err
		}
		// Python b16encode is uppercase hex.
		out := make([]byte, hex.EncodedLen(len(data)))
		hex.Encode(out, data)
		return object.NewBytes([]byte(upperASCII(string(out)))), nil
	}))
	set("b16decode", fn("base64.b16decode", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		data, err := bytesLike(args, 0)
		if err != nil {
			return nil, err
		}
		decoded, derr := hex.DecodeString(string(data))
		if derr != nil {
			return nil, object.NewValueError("Non-base16 digit found")
		}
		return object.NewBytes(decoded), nil
	}))

	// base32.
	set("b32encode", encodeFn("base64.b32encode", base64Enc32{}))
	set("b32decode", decodeFn("base64.b32decode", base64Enc32{}))

	return mod
}

// encoder abstracts the base64/base32 encodings used here.
type encoder interface {
	EncodeToString(src []byte) string
	DecodeString(s string) ([]byte, error)
}

func encodeFn(name string, enc encoder) *object.BuiltinFuncObject {
	return fn(name, func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		data, err := bytesLike(args, 0)
		if err != nil {
			return nil, err
		}
		return object.NewBytes([]byte(enc.EncodeToString(data))), nil
	})
}

func decodeFn(name string, enc encoder) *object.BuiltinFuncObject {
	return fn(name, func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		data, err := bytesLike(args, 0)
		if err != nil {
			return nil, err
		}
		decoded, derr := enc.DecodeString(string(data))
		if derr != nil {
			return nil, object.NewValueError("Invalid base64-encoded string: %v", derr)
		}
		return object.NewBytes(decoded), nil
	})
}

// base64Enc32 adapts base32 to the encoder interface.
type base64Enc32 struct{}

func (base64Enc32) EncodeToString(src []byte) string {
	return base32StdEncoding.EncodeToString(src)
}
func (base64Enc32) DecodeString(s string) ([]byte, error) {
	return base32StdEncoding.DecodeString(s)
}

// bytesLike extracts a []byte from a bytes or str argument.
func bytesLike(args []object.Object, i int) ([]byte, error) {
	if i >= len(args) {
		return nil, object.NewTypeError("missing required argument")
	}
	switch v := args[i].(type) {
	case *object.BytesObject:
		return v.Value, nil
	case *object.StrObject:
		return []byte(v.Value), nil
	}
	return nil, object.NewTypeError("argument should be a bytes-like object or ASCII string, not '%s'",
		args[i].Type().Name())
}

func upperASCII(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 32
		}
	}
	return string(b)
}
