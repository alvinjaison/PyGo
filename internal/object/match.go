package object

// Hooks for structural pattern matching and isinstance on class-like objects
// that are not plain *TypeObject values.

// InstanceTester is implemented by class-like objects (for example the
// factory returned by collections.namedtuple) that decide for themselves
// whether a value is one of their instances.
type InstanceTester interface {
	Object
	IsInstance(obj Object) bool
}

// MatchArgser is implemented by class-like objects that provide the
// equivalent of __match_args__ for positional class patterns.
type MatchArgser interface {
	MatchArgs() []string
}

// MatchSequencer marks sequence types (beyond list and tuple) that a sequence
// pattern such as `case [a, b]:` may match. Implementers must also provide
// Len() and be iterable.
type MatchSequencer interface {
	IsMatchSequence()
}

// AsClass resolves an object used as a class to its *TypeObject. It accepts a
// real type, and the built-in constructor callables (int, str, list, ...) that
// stand for one.
func AsClass(o Object) (*TypeObject, bool) {
	switch c := o.(type) {
	case *TypeObject:
		return c, true
	case *BuiltinFuncObject:
		if c.Class != nil {
			return c.Class, true
		}
	}
	return nil, false
}

// IsInstanceOfClass reports whether obj is an instance of cls, where cls is a
// type, a built-in constructor standing for a type, or an InstanceTester.
// ok is false when cls is not usable as a class at all.
func IsInstanceOfClass(obj, cls Object) (isInstance, ok bool) {
	if t, found := AsClass(cls); found {
		return obj.Type().IsSubtype(t), true
	}
	if it, found := cls.(InstanceTester); found {
		return it.IsInstance(obj), true
	}
	return false, false
}
