package object

// ============================================================
// Rich comparison for collection types
//
// Without these, lists, tuples and dicts fall through the comparison protocol
// to the identity fallback in RichCompareObjects, so `[1, 2] == [1, 2]` would
// report False and ordering would raise TypeError.
// ============================================================

// Element equality reuses objectsEqual from collections.go, which checks
// identity first (matching CPython's PyObject_RichCompareBool) before consulting
// the comparison slots. The identity check is what makes a list containing NaN
// compare equal to itself. Because dict lookup and `in` already route through
// that helper, adding the RichCompare methods below also fixes containment and
// key lookup for list and tuple values.

// compareSequences applies Python's lexicographic comparison to two sequences.
//
// Elements are compared pairwise. The first unequal pair decides the result of
// an ordering comparison. If neither sequence has an unequal element then one is
// a prefix of the other, and the lengths decide.
func compareSequences(a, b []Object, op CompareOp) (Object, error) {
	// Equality can only hold for equal lengths, so skip the element walk.
	if (op == OpEq || op == OpNotEq) && len(a) != len(b) {
		return BoolFrom(op == OpNotEq), nil
	}

	shorter := len(a)
	if len(b) < shorter {
		shorter = len(b)
	}

	for i := 0; i < shorter; i++ {
		eq, err := objectsEqual(a[i], b[i])
		if err != nil {
			return nil, err
		}
		if eq {
			continue
		}
		switch op {
		case OpEq:
			return PyFalse, nil
		case OpNotEq:
			return PyTrue, nil
		default:
			// Defer to the elements themselves for the ordering decision.
			return RichCompareObjects(a[i], b[i], op)
		}
	}

	switch op {
	case OpEq:
		return BoolFrom(len(a) == len(b)), nil
	case OpNotEq:
		return BoolFrom(len(a) != len(b)), nil
	case OpLt:
		return BoolFrom(len(a) < len(b)), nil
	case OpLtE:
		return BoolFrom(len(a) <= len(b)), nil
	case OpGt:
		return BoolFrom(len(a) > len(b)), nil
	case OpGtE:
		return BoolFrom(len(a) >= len(b)), nil
	default:
		return NotImplemented, nil
	}
}

// RichCompare implements Comparable for ListObject.
// Lists compare element-wise against other lists only; comparing a list to a
// tuple yields NotImplemented, so `[1] == (1,)` is False and `[1] < (1,)` is a
// TypeError, as in Python.
func (l *ListObject) RichCompare(other Object, op CompareOp) (Object, error) {
	o, ok := other.(*ListObject)
	if !ok {
		return NotImplemented, nil
	}
	if l == o && (op == OpEq || op == OpLtE || op == OpGtE) {
		return PyTrue, nil
	}
	return compareSequences(l.Items, o.Items, op)
}

// RichCompare implements Comparable for TupleObject.
func (t *TupleObject) RichCompare(other Object, op CompareOp) (Object, error) {
	o, ok := other.(*TupleObject)
	if !ok {
		return NotImplemented, nil
	}
	if t == o && (op == OpEq || op == OpLtE || op == OpGtE) {
		return PyTrue, nil
	}
	return compareSequences(t.Items, o.Items, op)
}

// RichCompare implements Comparable for DictObject.
// Dicts support only == and !=; Python defines no ordering for them, so the
// ordering operators return NotImplemented and end up as a TypeError.
func (d *DictObject) RichCompare(other Object, op CompareOp) (Object, error) {
	o, ok := other.(*DictObject)
	if !ok {
		return NotImplemented, nil
	}
	if op != OpEq && op != OpNotEq {
		return NotImplemented, nil
	}
	equal, err := dictsEqual(d, o)
	if err != nil {
		return nil, err
	}
	if op == OpNotEq {
		return BoolFrom(!equal), nil
	}
	return BoolFrom(equal), nil
}

// dictsEqual reports whether two dicts hold the same keys mapped to equal
// values. Insertion order does not affect equality.
func dictsEqual(a, b *DictObject) (bool, error) {
	if a == b {
		return true, nil
	}
	if a.Len() != b.Len() {
		return false, nil
	}
	for _, key := range a.Keys() {
		bVal, err := b.GetItem(key)
		if err != nil {
			// Key absent from b — the dicts differ. Keys held by a dict are
			// hashable by construction, so this cannot be a hashing failure.
			return false, nil
		}
		aVal, err := a.GetItem(key)
		if err != nil {
			return false, err
		}
		eq, err := objectsEqual(aVal, bVal)
		if err != nil {
			return false, err
		}
		if !eq {
			return false, nil
		}
	}
	return true, nil
}
