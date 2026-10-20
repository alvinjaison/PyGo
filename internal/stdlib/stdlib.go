// Package stdlib provides pure-Go implementations of a curated subset of the
// Python standard library. Each module is built as an *object.ModuleObject and
// registered with the import system by name.
//
// Modules here must not depend on CPython. They are implemented directly in Go
// using the object model.
package stdlib

import "github.com/pygo/pygo/internal/object"

// Builder constructs a module object. It receives the module's name so the
// returned module reports the correct __name__.
type Builder func(name string) *object.ModuleObject

// registry maps module names to their Go builders.
var registry = map[string]Builder{
	"math":      buildMath,
	"json":      buildJSON,
	"re":        buildRe,
	"random":    buildRandom,
	"sys":       buildSys,
	"string":    buildString,
	"functools": buildFunctools,
	"itertools": buildItertools,
	"os":        buildOS,
	"time":      buildTime,
}

// osCapable lists modules that interact with the host OS (filesystem,
// environment, processes). The import system blocks these when a runtime is
// created in restricted mode.
var osCapable = map[string]bool{
	"os": true,
}

// IsOSCapable reports whether importing the named module grants access to host
// OS capabilities and should therefore be blocked in restricted mode.
func IsOSCapable(name string) bool {
	return osCapable[name]
}

// Names returns the list of available built-in module names.
func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}

// Has reports whether a built-in module with the given name exists.
func Has(name string) bool {
	_, ok := registry[name]
	return ok
}

// Build constructs the named built-in module, or returns nil if unknown.
func Build(name string) *object.ModuleObject {
	b, ok := registry[name]
	if !ok {
		return nil
	}
	return b(name)
}

// ---- helpers shared by module builders ----

// newModule creates a module and provides a helper to register attributes.
func newModule(name string) (*object.ModuleObject, func(string, object.Object)) {
	mod := object.NewModule(name)
	set := func(attr string, val object.Object) {
		_ = mod.SetAttr(attr, val)
	}
	return mod, set
}

// fn wraps a Go function as a named builtin callable.
func fn(name string, f func([]object.Object, map[string]object.Object) (object.Object, error)) *object.BuiltinFuncObject {
	return object.NewBuiltin(name, f)
}

// argFloat coerces the i-th argument to a float64.
func argFloat(args []object.Object, i int) (float64, error) {
	if i >= len(args) {
		return 0, object.NewTypeError("missing required argument")
	}
	switch v := args[i].(type) {
	case *object.FloatObject:
		return v.Value, nil
	case *object.IntObject:
		f, _ := v.Int64()
		return float64(f), nil
	case *object.BoolObject:
		if v.Value {
			return 1, nil
		}
		return 0, nil
	}
	return 0, object.NewTypeError("expected a number, got '%s'", args[i].Type().Name())
}

// strListToObject converts a slice of Go strings into a Python list.
func strListToObject(parts []string) object.Object {
	items := make([]object.Object, len(parts))
	for i, p := range parts {
		items[i] = object.StrFrom(p)
	}
	return object.NewList(items)
}

// argInt coerces the i-th argument to an int64.
func argInt(args []object.Object, i int) (int64, error) {
	if i >= len(args) {
		return 0, object.NewTypeError("missing required argument")
	}
	if n, ok := args[i].(*object.IntObject); ok {
		v, _ := n.Int64()
		return v, nil
	}
	return 0, object.NewTypeError("expected an int, got '%s'", args[i].Type().Name())
}
