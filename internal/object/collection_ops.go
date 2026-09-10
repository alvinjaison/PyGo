package object

import "fmt"

// ============================================================
// List operator slots
// ============================================================

// listAdd implements list.__add__ (concatenation)
func listAdd(a, b Object) (Object, error) {
	al, ok := a.(*ListObject)
	if !ok {
		return NotImplemented, nil
	}
	bl, ok := b.(*ListObject)
	if !ok {
		return NotImplemented, nil
	}
	items := make([]Object, len(al.Items)+len(bl.Items))
	copy(items, al.Items)
	copy(items[len(al.Items):], bl.Items)
	return &ListObject{Items: items}, nil
}

// listMul implements list.__mul__ (repetition)
func listMul(a, b Object) (Object, error) {
	al, ok := a.(*ListObject)
	if !ok {
		return NotImplemented, nil
	}
	n, err := toRepeatCount(b)
	if err == errNotNumeric {
		return NotImplemented, nil
	}
	if err != nil {
		return nil, err
	}
	if n <= 0 {
		return NewList(nil), nil
	}
	items := make([]Object, 0, len(al.Items)*n)
	for i := 0; i < n; i++ {
		items = append(items, al.Items...)
	}
	return &ListObject{Items: items}, nil
}

// listRMul implements list.__rmul__ (n * list)
func listRMul(a, b Object) (Object, error) {
	return listMul(a, b)
}

// ============================================================
// Tuple operator slots
// ============================================================

// tupleAdd implements tuple.__add__ (concatenation)
func tupleAdd(a, b Object) (Object, error) {
	at, ok := a.(*TupleObject)
	if !ok {
		return NotImplemented, nil
	}
	bt, ok := b.(*TupleObject)
	if !ok {
		return NotImplemented, nil
	}
	items := make([]Object, len(at.Items)+len(bt.Items))
	copy(items, at.Items)
	copy(items[len(at.Items):], bt.Items)
	return &TupleObject{Items: items}, nil
}

// tupleMul implements tuple.__mul__ (repetition)
func tupleMul(a, b Object) (Object, error) {
	at, ok := a.(*TupleObject)
	if !ok {
		return NotImplemented, nil
	}
	n, err := toRepeatCount(b)
	if err == errNotNumeric {
		return NotImplemented, nil
	}
	if err != nil {
		return nil, err
	}
	if n <= 0 {
		return NewTuple(nil), nil
	}
	items := make([]Object, 0, len(at.Items)*n)
	for i := 0; i < n; i++ {
		items = append(items, at.Items...)
	}
	return &TupleObject{Items: items}, nil
}

// tupleRMul implements tuple.__rmul__ (n * tuple)
func tupleRMul(a, b Object) (Object, error) {
	return tupleMul(a, b)
}

// ============================================================
// Helper
// ============================================================

// toRepeatCount extracts an integer repeat count from an Object.
// Returns (0, nil) with a special sentinel if the type is not numeric,
// so the caller can return NotImplemented.
func toRepeatCount(b Object) (int, error) {
	switch bi := b.(type) {
	case *IntObject:
		v, fits := bi.Int64()
		if !fits {
			return 0, fmt.Errorf("OverflowError: cannot fit into int")
		}
		return int(v), nil
	case *BoolObject:
		if bi.Value {
			return 1, nil
		}
		return 0, nil
	}
	return 0, errNotNumeric
}

// errNotNumeric is a sentinel used by toRepeatCount.
var errNotNumeric = fmt.Errorf("not numeric")

// registerCollectionSlots registers operator slots on collection types.
// Called from registerOperatorSlots in ops.go.
func registerCollectionSlots() {
	// List operators
	ListType.slots.Add = listAdd
	ListType.slots.Mul = listMul
	ListType.slots.RMul = listRMul

	// Tuple operators
	TupleType.slots.Add = tupleAdd
	TupleType.slots.Mul = tupleMul
	TupleType.slots.RMul = tupleRMul
}
