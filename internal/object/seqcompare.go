package object

import "github.com/alvinjaison/PyGo/internal/bytecode"

// Rich comparison for lists, tuples and dicts.

func toBytecodeOp(op CompareOp) bytecode.CompareOp {
	switch op {
	case OpLt:
		return bytecode.CmpLt
	case OpLtE:
		return bytecode.CmpLtE
	case OpEq:
		return bytecode.CmpEq
	case OpNotEq:
		return bytecode.CmpNotEq
	case OpGt:
		return bytecode.CmpGt
	}
	return bytecode.CmpGtE
}

// seqCompare compares two item slices lexicographically, as Python does for
// lists and tuples: the first differing pair decides; otherwise the shorter
// sequence is smaller.
func seqCompare(a, b []Object, op CompareOp) (Object, error) {
	if op == OpEq || op == OpNotEq {
		equal := len(a) == len(b)
		for i := 0; equal && i < len(a); i++ {
			eq, err := objectsEqual(a[i], b[i])
			if err != nil {
				return nil, err
			}
			equal = eq
		}
		return BoolFrom(equal == (op == OpEq)), nil
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		eq, err := objectsEqual(a[i], b[i])
		if err != nil {
			return nil, err
		}
		if !eq {
			return compareObjects(a[i], b[i], toBytecodeOp(op))
		}
	}
	return boolFromCmp(cmpInts(len(a), len(b)), op), nil
}

func cmpInts(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// RichCompare implements ==, != and ordering between lists.
func (l *ListObject) RichCompare(other Object, op CompareOp) (Object, error) {
	o, ok := other.(*ListObject)
	if !ok {
		return NotImplemented, nil
	}
	return seqCompare(l.Items, o.Items, op)
}

// RichCompare implements ==, != and ordering between tuples.
func (t *TupleObject) RichCompare(other Object, op CompareOp) (Object, error) {
	o, ok := other.(*TupleObject)
	if !ok {
		return NotImplemented, nil
	}
	return seqCompare(t.Items, o.Items, op)
}

// RichCompare implements == and != between dicts (order-insensitive).
func (d *DictObject) RichCompare(other Object, op CompareOp) (Object, error) {
	o, ok := other.(*DictObject)
	if !ok || (op != OpEq && op != OpNotEq) {
		return NotImplemented, nil
	}
	equal := len(d.entries) == len(o.entries)
	for i := 0; equal && i < len(d.entries); i++ {
		e := &d.entries[i]
		v, found, err := o.Lookup(e.key)
		if err != nil {
			return nil, err
		}
		if !found {
			equal = false
			break
		}
		if equal, err = objectsEqual(e.value, v); err != nil {
			return nil, err
		}
	}
	return BoolFrom(equal == (op == OpEq)), nil
}
