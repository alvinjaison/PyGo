package object

import (
	"sort"
	"strings"
)

// ============================================================
// SetObject
// ============================================================

// SetObject represents a Python set (mutable, unordered, unique elements).
//
// Elements are stored in a hash-indexed bucket map keyed by the element's
// Python hash, with collision chaining by Python equality.
type SetObject struct {
	buckets map[int64][]Object
	count   int
}

// NewSet creates an empty set.
func NewSet() *SetObject {
	return &SetObject{buckets: map[int64][]Object{}}
}

// NewSetFrom builds a set from the given items, de-duplicating by equality.
func NewSetFrom(items []Object) (*SetObject, error) {
	s := NewSet()
	for _, it := range items {
		if err := s.Add(it); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *SetObject) Type() *TypeObject     { return SetType }
func (s *SetObject) Len() int              { return s.count }
func (s *SetObject) IsTrue() (bool, error) { return s.count > 0, nil }

func (s *SetObject) String() string { return s.Repr() }

func (s *SetObject) Repr() string {
	if s.count == 0 {
		return "set()"
	}
	var parts []string
	for _, bucket := range s.buckets {
		for _, el := range bucket {
			parts = append(parts, el.Repr())
		}
	}
	sort.Strings(parts) // deterministic repr for readability/tests
	return "{" + strings.Join(parts, ", ") + "}"
}

// Add inserts an element if not already present.
func (s *SetObject) Add(item Object) error {
	h, err := hashObject(item)
	if err != nil {
		return err
	}
	for _, existing := range s.buckets[h] {
		eq, err := objectsEqual(existing, item)
		if err != nil {
			return err
		}
		if eq {
			return nil
		}
	}
	s.buckets[h] = append(s.buckets[h], item)
	s.count++
	return nil
}

// Contains reports membership.
func (s *SetObject) Contains(item Object) (bool, error) {
	h, err := hashObject(item)
	if err != nil {
		return false, err
	}
	for _, existing := range s.buckets[h] {
		eq, err := objectsEqual(existing, item)
		if err != nil {
			return false, err
		}
		if eq {
			return true, nil
		}
	}
	return false, nil
}

// Items returns all elements (unordered).
func (s *SetObject) Items() []Object {
	items := make([]Object, 0, s.count)
	for _, bucket := range s.buckets {
		items = append(items, bucket...)
	}
	return items
}

// isSubsetOf reports whether every element of s is in other.
func (s *SetObject) isSubsetOf(other *SetObject) (bool, error) {
	if s.count > other.count {
		return false, nil
	}
	for _, el := range s.Items() {
		ok, err := other.Contains(el)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// RichCompare implements ==, != and the subset/superset orderings between
// sets. Comparing with a non-set yields NotImplemented.
func (s *SetObject) RichCompare(other Object, op CompareOp) (Object, error) {
	o, ok := other.(*SetObject)
	if !ok {
		return NotImplemented, nil
	}
	var res bool
	var err error
	switch op {
	case OpEq, OpNotEq:
		if s.count != o.count {
			res = false
		} else {
			res, err = s.isSubsetOf(o)
		}
		if op == OpNotEq {
			res = !res
		}
	case OpLtE:
		res, err = s.isSubsetOf(o)
	case OpLt:
		if s.count < o.count {
			res, err = s.isSubsetOf(o)
		}
	case OpGtE:
		res, err = o.isSubsetOf(s)
	case OpGt:
		if o.count < s.count {
			res, err = o.isSubsetOf(s)
		}
	default:
		return NotImplemented, nil
	}
	if err != nil {
		return nil, err
	}
	return BoolFrom(res), nil
}

// Iter returns an iterator over the set's elements.
func (s *SetObject) Iter() (Iterator, error) {
	return NewList(s.Items()).Iter()
}

// ============================================================
// SliceObject
// ============================================================

// SliceObject represents a Python slice (start:stop:step).
type SliceObject struct {
	Start Object
	Stop  Object
	Step  Object
}

// NewSlice creates a slice object.
func NewSlice(start, stop, step Object) *SliceObject {
	return &SliceObject{Start: start, Stop: stop, Step: step}
}

func (s *SliceObject) Type() *TypeObject { return SliceType }
func (s *SliceObject) String() string    { return s.Repr() }
func (s *SliceObject) Repr() string {
	return "slice(" + s.Start.Repr() + ", " + s.Stop.Repr() + ", " + s.Step.Repr() + ")"
}

// Resolve computes concrete (start, stop, step) indices for a sequence of the
// given length, following Python's slice semantics.
func (s *SliceObject) Resolve(length int) (start, stop, step int, err error) {
	step = 1
	if !IsNone(s.Step) {
		st, ok := s.Step.(*IntObject)
		if !ok {
			return 0, 0, 0, NewTypeError("slice indices must be integers or None")
		}
		step64, _ := st.Int64()
		step = int(step64)
		if step == 0 {
			return 0, 0, 0, NewValueError("slice step cannot be zero")
		}
	}

	// Defaults depend on step direction.
	if step > 0 {
		start, stop = 0, length
	} else {
		start, stop = length-1, -1 // -1 means "before index 0"
	}

	if !IsNone(s.Start) {
		start = clampIndex(s.Start, length, step > 0)
	}
	if !IsNone(s.Stop) {
		stop = clampIndex(s.Stop, length, step > 0)
	}
	return start, stop, step, nil
}

func clampIndex(o Object, length int, forward bool) int {
	n, ok := o.(*IntObject)
	if !ok {
		return 0
	}
	v64, _ := n.Int64()
	i := int(v64)
	if i < 0 {
		i += length
	}
	if forward {
		if i < 0 {
			i = 0
		}
		if i > length {
			i = length
		}
	} else {
		if i < -1 {
			i = -1
		}
		if i > length-1 {
			i = length - 1
		}
	}
	return i
}
