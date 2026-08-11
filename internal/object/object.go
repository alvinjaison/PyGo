// Package object implements the PyGo Python object model.
//
// Every Python value is represented as a Go value implementing the Object
// interface. Additional capabilities (callable, iterable, hashable, etc.) are
// expressed as optional Go interfaces detected via type assertions at runtime.
//
// See docs/object-model.md for the complete design rationale.
package object

import "fmt"

// Object is the root interface for all Python values.
// Every value that can exist on the Python evaluation stack implements Object.
type Object interface {
	// Type returns the Python type object for this value.
	// Equivalent to type(obj) in Python.
	Type() *TypeObject

	// String returns the human-readable string representation.
	// Used by str() and print(). Equivalent to Python's __str__.
	String() string

	// Repr returns the developer string representation.
	// Used by repr(). Equivalent to Python's __repr__.
	Repr() string
}

// ============================================================
// Optional protocol interfaces
// ============================================================

// Callable objects can be invoked with arguments.
type Callable interface {
	Object
	Call(args []Object, kwargs map[string]Object) (Object, error)
}

// Iterable objects can produce an iterator.
type Iterable interface {
	Object
	Iter() (Iterator, error)
}

// Iterator objects produce successive values.
// Returns (nil, StopIteration) when exhausted.
type Iterator interface {
	Object
	Next() (Object, error)
}

// Hashable objects can be used as dict keys or set members.
// Objects that compare equal must return the same hash.
type Hashable interface {
	Object
	Hash() (int64, error)
}

// Comparable supports rich comparison (beyond identity equality).
type Comparable interface {
	Object
	RichCompare(other Object, op CompareOp) (Object, error)
}

// LenProvider supports len().
type LenProvider interface {
	Object
	Len() int
}

// Subscriptable supports obj[key], obj[key] = v, del obj[key].
type Subscriptable interface {
	Object
	GetItem(key Object) (Object, error)
	SetItem(key Object, value Object) error
	DelItem(key Object) error
}

// AttributeHolder supports attribute get/set/delete.
type AttributeHolder interface {
	Object
	GetAttr(name string) (Object, error)
	SetAttr(name string, value Object) error
	DelAttr(name string) error
}

// TruthTester supports bool() conversion via __bool__.
type TruthTester interface {
	Object
	IsTrue() (bool, error)
}

// Container supports the `in` operator via __contains__.
type Container interface {
	Object
	Contains(item Object) (bool, error)
}

// CompareOp is the operator for rich comparison.
type CompareOp int

const (
	OpLt CompareOp = iota
	OpLtE
	OpEq
	OpNotEq
	OpGt
	OpGtE
)

func (c CompareOp) String() string {
	return [...]string{"<", "<=", "==", "!=", ">", ">="}[c]
}

// ============================================================
// Error sentinels
// ============================================================

// ErrStopIteration is returned by Iterator.Next() when the iterator is
// exhausted. It is NOT a Go error in the traditional sense — callers must
// check for it explicitly.
var ErrStopIteration = fmt.Errorf("StopIteration")

// IsStopIteration reports whether err signals iterator exhaustion.
func IsStopIteration(err error) bool {
	return err == ErrStopIteration
}

// ============================================================
// Shared singleton values
// ============================================================

// None is the singleton Python None value.
var None Object = &NoneObject{}

// NotImplemented is the singleton Python NotImplemented value, returned
// by operator methods when an operation is not implemented for the given type.
var NotImplemented Object = &notImplementedObject{}

// IsNone reports whether obj is Python None.
func IsNone(obj Object) bool { _, ok := obj.(*NoneObject); return ok }

// IsNotImplemented reports whether obj is Python NotImplemented.
func IsNotImplemented(obj Object) bool { _, ok := obj.(*notImplementedObject); return ok }

// ============================================================
// Truthiness
// ============================================================

// IsTruthy returns the Python truth value of obj.
// All objects are truthy except: False, None, 0, 0.0, "", [], (), {}, set()
func IsTruthy(obj Object) (bool, error) {
	if t, ok := obj.(TruthTester); ok {
		return t.IsTrue()
	}
	if l, ok := obj.(LenProvider); ok {
		return l.Len() != 0, nil
	}
	// Default: all other objects are truthy.
	return true, nil
}
