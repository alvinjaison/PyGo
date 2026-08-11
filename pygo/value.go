package pygo

// Value represents a Python value accessible from Go.
// It wraps a Python object and provides safe conversion to Go types.
//
// All conversion methods return (result, error). An error is returned when the
// conversion is not applicable (e.g., calling Int() on a Python str).
type Value interface {
	// Type returns the Python type name (e.g., "int", "str", "list").
	Type() string

	// String returns the Python str() representation of the value.
	String() string

	// Repr returns the Python repr() representation of the value.
	Repr() string

	// IsNone reports whether this value is Python None.
	IsNone() bool

	// Int converts the value to int64.
	// Returns an error if the value is not a Python int or bool.
	Int() (int64, error)

	// Float converts the value to float64.
	// Returns an error if the value is not a Python float or int.
	Float() (float64, error)

	// Bool converts the value to bool using Python's truth-value testing.
	// All values are convertible (equivalent to bool(v) in Python).
	Bool() bool

	// Str converts the value to string.
	// Returns an error if the value is not a Python str.
	Str() (string, error)

	// Bytes converts the value to []byte.
	// Returns an error if the value is not Python bytes.
	Bytes() ([]byte, error)

	// List converts the value to a []Value slice.
	// Returns an error if the value is not a Python list or tuple.
	List() ([]Value, error)

	// Dict converts the value to map[string]Value.
	// Returns an error if the value is not a Python dict,
	// or if any dict key is not a string.
	Dict() (map[string]Value, error)

	// Unwrap returns the underlying Go value if this Value wraps a registered
	// Go object (via Register or as the return value of a registered function).
	// Returns nil if the value is a pure Python object with no Go backing.
	Unwrap() interface{}
}
