package object

// This file implements dunder-method dispatch for user-defined classes.
// Built-in types use Go type slots and interfaces; user classes store their
// special methods (__add__, __eq__, __len__, …) as functions in the type's
// MRO dict. These helpers look those methods up and invoke them through the
// global invoke hook (the VM), so instances participate in operators, the
// container protocol, truthiness, str()/repr(), and calling.

// lookupDunder returns an instance's special method bound to self, or nil if
// the method is not defined anywhere in the type's MRO. Special methods are
// looked up on the type, not the instance, matching Python semantics.
func lookupDunder(obj Object, name string) Object {
	inst, ok := obj.(*InstanceObject)
	if !ok {
		return nil
	}
	m := lookupInMRO(inst.cls, name)
	if m == nil {
		return nil
	}
	// Bind plain functions to self; already-callable objects pass through.
	if fn, ok := m.(*FunctionObject); ok {
		return &BoundMethodObject{Self: inst, Function: fn}
	}
	return m
}

// hasDunder reports whether an instance defines the named special method.
func hasDunder(obj Object, name string) bool {
	return lookupDunder(obj, name) != nil
}

// Dunder returns the instance's named special method, bound to self, and
// whether it exists. Exported for use by builtins (e.g. abs → __abs__).
func (o *InstanceObject) Dunder(name string) (Object, bool) {
	m := lookupDunder(o, name)
	return m, m != nil
}

// callDunder invokes a bound special method with the given arguments.
func callDunder(method Object, args ...Object) (Object, error) {
	return Invoke(method, args)
}

// binaryDunderNames maps an operator token to its forward and reflected
// special-method names.
var binaryDunderNames = map[string][2]string{
	"+":  {"__add__", "__radd__"},
	"-":  {"__sub__", "__rsub__"},
	"*":  {"__mul__", "__rmul__"},
	"/":  {"__truediv__", "__rtruediv__"},
	"//": {"__floordiv__", "__rfloordiv__"},
	"%":  {"__mod__", "__rmod__"},
	"**": {"__pow__", "__rpow__"},
	"@":  {"__matmul__", "__rmatmul__"},
	"&":  {"__and__", "__rand__"},
	"|":  {"__or__", "__ror__"},
	"^":  {"__xor__", "__rxor__"},
	"<<": {"__lshift__", "__rlshift__"},
	">>": {"__rshift__", "__rrshift__"},
}

// tryBinaryDunder attempts operator dispatch via user-defined dunder methods,
// following Python's left-then-reflected protocol. It returns (result, handled,
// err): handled is false when neither operand defines the method, so the caller
// can fall through to its default error.
func tryBinaryDunder(left, right Object, op string) (Object, bool, error) {
	names, ok := binaryDunderNames[op]
	if !ok {
		return nil, false, nil
	}
	fwd, rev := names[0], names[1]

	// Forward: left.__op__(right).
	if m := lookupDunder(left, fwd); m != nil {
		res, err := callDunder(m, right)
		if err != nil {
			return nil, true, err
		}
		if !IsNotImplemented(res) {
			return res, true, nil
		}
	}
	// Reflected: right.__rop__(left).
	if m := lookupDunder(right, rev); m != nil {
		res, err := callDunder(m, left)
		if err != nil {
			return nil, true, err
		}
		if !IsNotImplemented(res) {
			return res, true, nil
		}
	}
	return nil, false, nil
}

// unaryDunderNames maps a unary operator token to its special-method name.
var unaryDunderNames = map[string]string{
	"-": "__neg__",
	"+": "__pos__",
	"~": "__invert__",
}

// tryUnaryDunder attempts unary operator dispatch via a dunder method.
func tryUnaryDunder(val Object, op string) (Object, bool, error) {
	name, ok := unaryDunderNames[op]
	if !ok {
		return nil, false, nil
	}
	if m := lookupDunder(val, name); m != nil {
		res, err := callDunder(m)
		return res, true, err
	}
	return nil, false, nil
}

// compareDunderNames maps a comparison operator to its special-method name and
// the reflected operator's name (for the right operand's fallback).
var compareDunderNames = map[CompareOp][2]string{
	OpLt:    {"__lt__", "__gt__"},
	OpLtE:   {"__le__", "__ge__"},
	OpEq:    {"__eq__", "__eq__"},
	OpNotEq: {"__ne__", "__ne__"},
	OpGt:    {"__gt__", "__lt__"},
	OpGtE:   {"__ge__", "__le__"},
}

// tryCompareDunder attempts rich-comparison dispatch via dunder methods.
func tryCompareDunder(left, right Object, op CompareOp) (Object, bool, error) {
	names, ok := compareDunderNames[op]
	if !ok {
		return nil, false, nil
	}
	if m := lookupDunder(left, names[0]); m != nil {
		res, err := callDunder(m, right)
		if err != nil {
			return nil, true, err
		}
		if !IsNotImplemented(res) {
			return res, true, nil
		}
	}
	if m := lookupDunder(right, names[1]); m != nil {
		res, err := callDunder(m, left)
		if err != nil {
			return nil, true, err
		}
		if !IsNotImplemented(res) {
			return res, true, nil
		}
	}
	return nil, false, nil
}
