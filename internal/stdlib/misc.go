package stdlib

import (
	"math/rand"
	"strings"

	"github.com/pygo/pygo/internal/object"
)

// buildSys constructs a minimal `sys` module.
func buildSys(name string) *object.ModuleObject {
	mod, set := newModule(name)
	set("version", object.StrFrom("3.x (PyGo)"))
	set("platform", object.StrFrom("pygo"))
	set("maxsize", object.IntFrom(1<<63-1))
	set("argv", object.NewList([]object.Object{object.StrFrom("")}))
	set("path", object.NewList(nil))
	return mod
}

// buildString constructs the `string` module (constants only for now).
func buildString(name string) *object.ModuleObject {
	mod, set := newModule(name)
	set("ascii_lowercase", object.StrFrom("abcdefghijklmnopqrstuvwxyz"))
	set("ascii_uppercase", object.StrFrom("ABCDEFGHIJKLMNOPQRSTUVWXYZ"))
	set("ascii_letters", object.StrFrom("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"))
	set("digits", object.StrFrom("0123456789"))
	set("hexdigits", object.StrFrom("0123456789abcdefABCDEF"))
	set("punctuation", object.StrFrom("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"))
	set("whitespace", object.StrFrom(" \t\n\r\x0b\x0c"))
	return mod
}

// buildRandom constructs a small `random` module.
func buildRandom(name string) *object.ModuleObject {
	mod, set := newModule(name)
	rng := rand.New(rand.NewSource(1)) // deterministic default; seed() reseeds

	set("seed", fn("random.seed", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		if len(args) > 0 {
			if n, ok := args[0].(*object.IntObject); ok {
				v, _ := n.Int64()
				rng = rand.New(rand.NewSource(v))
			}
		}
		return object.None, nil
	}))
	set("random", fn("random.random", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
		return object.FloatFrom(rng.Float64()), nil
	}))
	set("randint", fn("random.randint", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		a, err := argInt(args, 0)
		if err != nil {
			return nil, err
		}
		b, err := argInt(args, 1)
		if err != nil {
			return nil, err
		}
		if b < a {
			return nil, object.NewValueError("empty range for randint")
		}
		return object.IntFrom(a + rng.Int63n(b-a+1)), nil
	}))
	set("choice", fn("random.choice", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		seq, ok := args[0].(*object.ListObject)
		if !ok {
			return nil, object.NewTypeError("choice() requires a list")
		}
		if len(seq.Items) == 0 {
			return nil, object.NewException(object.TypeIndexError, "cannot choose from an empty sequence")
		}
		return seq.Items[rng.Intn(len(seq.Items))], nil
	}))
	return mod
}

// ensure strings import is used (shared helpers may reference it later)
var _ = strings.TrimSpace
