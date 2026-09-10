package object

import "fmt"

// ============================================================
// RangeObject
// ============================================================

// RangeType is the type of range objects.
var RangeType *TypeObject

// RangeIteratorType is the type of range iterators.
var RangeIteratorType *TypeObject

func init() {
	RangeType = newTypeObject("range", []*TypeObject{ObjectType}, TypeSlots{})
	RangeIteratorType = newTypeObject("range_iterator", []*TypeObject{ObjectType}, TypeSlots{})
}

// RangeObject represents Python's range(start, stop, step).
type RangeObject struct {
	Start int64
	Stop  int64
	Step  int64
}

// NewRange creates a new RangeObject. Step must not be zero.
func NewRange(start, stop, step int64) (*RangeObject, error) {
	if step == 0 {
		return nil, fmt.Errorf("ValueError: range() arg 3 must not be zero")
	}
	return &RangeObject{Start: start, Stop: stop, Step: step}, nil
}

func (r *RangeObject) Type() *TypeObject { return RangeType }
func (r *RangeObject) String() string {
	if r.Step == 1 {
		return fmt.Sprintf("range(%d, %d)", r.Start, r.Stop)
	}
	return fmt.Sprintf("range(%d, %d, %d)", r.Start, r.Stop, r.Step)
}
func (r *RangeObject) Repr() string { return r.String() }

// Len returns the number of items in the range.
func (r *RangeObject) Len() int {
	return rangeLen(r.Start, r.Stop, r.Step)
}

// IsTrue returns true if the range is non-empty.
func (r *RangeObject) IsTrue() (bool, error) {
	return r.Len() > 0, nil
}

// Iter returns a range iterator.
func (r *RangeObject) Iter() (Iterator, error) {
	return &rangeIterator{current: r.Start, stop: r.Stop, step: r.Step}, nil
}

// rangeLen computes the length of range(start, stop, step).
func rangeLen(start, stop, step int64) int {
	if step > 0 {
		if start >= stop {
			return 0
		}
		return int((stop - start - 1) / step) + 1
	}
	// step < 0
	if start <= stop {
		return 0
	}
	return int((start - stop - 1) / (-step)) + 1
}

// ============================================================
// rangeIterator
// ============================================================

type rangeIterator struct {
	current int64
	stop    int64
	step    int64
}

func (it *rangeIterator) Type() *TypeObject { return RangeIteratorType }
func (it *rangeIterator) String() string    { return "<range_iterator>" }
func (it *rangeIterator) Repr() string      { return it.String() }

func (it *rangeIterator) Next() (Object, error) {
	if it.step > 0 {
		if it.current >= it.stop {
			return nil, ErrStopIteration
		}
	} else {
		if it.current <= it.stop {
			return nil, ErrStopIteration
		}
	}
	val := it.current
	it.current += it.step
	return IntFrom(val), nil
}
