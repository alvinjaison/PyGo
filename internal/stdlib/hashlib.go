package stdlib

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"

	"github.com/pygo/pygo/internal/object"
)

// hashFactory maps an algorithm name to a Go hash constructor and digest size.
type hashFactory struct {
	name    string
	newHash func() hash.Hash
}

var hashAlgorithms = map[string]hashFactory{
	"md5":    {"md5", md5.New},
	"sha1":   {"sha1", sha1.New},
	"sha224": {"sha224", sha256.New224},
	"sha256": {"sha256", sha256.New},
	"sha384": {"sha384", sha512.New384},
	"sha512": {"sha512", sha512.New},
}

// buildHashlib constructs the `hashlib` module.
func buildHashlib(name string) *object.ModuleObject {
	mod, set := newModule(name)

	for algoName, factory := range hashAlgorithms {
		f := factory // capture
		set(f.name, fn("hashlib."+algoName, func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			return newHashObject(f, args)
		}))
	}

	// hashlib.new(name[, data]).
	set("new", fn("hashlib.new", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		algo, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		factory, ok := hashAlgorithms[algo]
		if !ok {
			return nil, object.NewValueError("unsupported hash type %s", algo)
		}
		return newHashObject(factory, args[1:])
	}))

	// algorithms_available / algorithms_guaranteed as a set-like list.
	names := make([]object.Object, 0, len(hashAlgorithms))
	for n := range hashAlgorithms {
		names = append(names, object.StrFrom(n))
	}
	set("algorithms_guaranteed", object.NewList(names))
	set("algorithms_available", object.NewList(names))

	return mod
}

// newHashObject creates a hash object, optionally seeded with initial data.
func newHashObject(factory hashFactory, args []object.Object) (object.Object, error) {
	h := &HashObject{name: factory.name, h: factory.newHash()}
	if len(args) > 0 {
		data, err := bytesLike(args, 0)
		if err != nil {
			return nil, err
		}
		h.h.Write(data)
	}
	return h, nil
}

// HashObject wraps a Go hash.Hash as a Python hashlib hash object.
type HashObject struct {
	name string
	h    hash.Hash
}

func (o *HashObject) Type() *object.TypeObject { return object.ObjectType }
func (o *HashObject) String() string           { return "<" + o.name + " hash object>" }
func (o *HashObject) Repr() string             { return o.String() }

func (o *HashObject) GetAttr(name string) (object.Object, error) {
	switch name {
	case "name":
		return object.StrFrom(o.name), nil
	case "digest_size":
		return object.IntFrom(int64(o.h.Size())), nil
	case "block_size":
		return object.IntFrom(int64(o.h.BlockSize())), nil
	case "update":
		return fn(o.name+".update", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			data, err := bytesLike(args, 0)
			if err != nil {
				return nil, err
			}
			o.h.Write(data)
			return object.None, nil
		}), nil
	case "digest":
		return fn(o.name+".digest", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return object.NewBytes(o.h.Sum(nil)), nil
		}), nil
	case "hexdigest":
		return fn(o.name+".hexdigest", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return object.StrFrom(hex.EncodeToString(o.h.Sum(nil))), nil
		}), nil
	case "copy":
		// Go's hash.Hash does not expose generic cloning, so copy() is not
		// supported. Documented as a divergence.
		return fn(o.name+".copy", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return nil, object.NewException(object.TypeNotImplementedError, "hash.copy() is not supported")
		}), nil
	}
	return nil, object.NewAttributeError(o.name, name)
}

func (o *HashObject) SetAttr(string, object.Object) error {
	return object.NewTypeError("hash object attributes are read-only")
}
func (o *HashObject) DelAttr(string) error {
	return object.NewTypeError("hash object attributes are read-only")
}
