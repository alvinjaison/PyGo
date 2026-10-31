package object

import (
	"fmt"
	"math"
)

// RangeObject is Python's lazy, immutable range(start, stop, step). It stores
// three int64s however long the range is, so range(10**9) costs nothing until
// it is iterated.
type RangeObject struct {
	Start, Stop, Step int64
}

// NewRange constructs a range. step must be non-zero.
func NewRange(start, stop, step int64) *RangeObject {
	return &RangeObject{Start: start, Stop: stop, Step: step}
}

// length returns the number of items, computed in unsigned arithmetic so that
// spans wider than int64 do not overflow.
func (r *RangeObject) length() uint64 {
	switch {
	case r.Step > 0 && r.Start < r.Stop:
		return (uint64(r.Stop)-uint64(r.Start)-1)/uint64(r.Step) + 1
	case r.Step < 0 && r.Start > r.Stop:
		return (uint64(r.Start)-uint64(r.Stop)-1)/(uint64(0)-uint64(r.Step)) + 1
	}
	return 0
}

func (r *RangeObject) Type() *TypeObject { return RangeType }
func (r *RangeObject) Len() int {
	n := r.length()
	if n > math.MaxInt {
		return math.MaxInt
	}
	return int(n)
}
func (r *RangeObject) IsTrue() (bool, error) { return r.length() > 0, nil }

func (r *RangeObject) String() string {
	if r.Step == 1 {
		return fmt.Sprintf("range(%d, %d)", r.Start, r.Stop)
	}
	return fmt.Sprintf("range(%d, %d, %d)", r.Start, r.Stop, r.Step)
}
func (r *RangeObject) Repr() string { return r.String() }

// at returns the i-th element (0 <= i < length).
func (r *RangeObject) at(i uint64) int64 { return r.Start + int64(i)*r.Step }

// Iter returns a lazy iterator over the range.
func (r *RangeObject) Iter() (Iterator, error) {
	return &rangeIterator{cur: r.Start, step: r.Step, remaining: r.length()}, nil
}

type rangeIterator struct {
	cur       int64
	step      int64
	remaining uint64
}

func (it *rangeIterator) Type() *TypeObject { return ObjectType }
func (it *rangeIterator) String() string    { return "<range_iterator object>" }
func (it *rangeIterator) Repr() string      { return it.String() }
func (it *rangeIterator) Iter() (Iterator, error) {
	return it, nil
}
func (it *rangeIterator) Next() (Object, error) {
	if it.remaining == 0 {
		return nil, ErrStopIteration
	}
	v := it.cur
	it.cur += it.step
	it.remaining--
	return IntFrom(v), nil
}

// GetItem supports r[i] and r[a:b:c] (a slice yields another range).
func (r *RangeObject) GetItem(key Object) (Object, error) {
	switch k := key.(type) {
	case *IntObject:
		idx, ok := k.Int64()
		n := r.length()
		if !ok {
			return nil, NewIndexError("range object index out of range")
		}
		if idx < 0 {
			if uint64(-idx) > n || idx == math.MinInt64 {
				return nil, NewIndexError("range object index out of range")
			}
			return IntFrom(r.at(n - uint64(-idx))), nil
		}
		if uint64(idx) >= n {
			return nil, NewIndexError("range object index out of range")
		}
		return IntFrom(r.at(uint64(idx))), nil
	case *SliceObject:
		start, stop, step, err := k.Resolve(r.Len())
		if err != nil {
			return nil, err
		}
		return NewRange(
			r.Start+int64(start)*r.Step,
			r.Start+int64(stop)*r.Step,
			r.Step*int64(step)), nil
	}
	return nil, NewTypeError("range indices must be integers or slices, not %s", key.Type().Name())
}

// Contains implements `x in r` in constant time for integers.
func (r *RangeObject) Contains(item Object) (bool, error) {
	var x int64
	switch v := item.(type) {
	case *IntObject:
		n, ok := v.Int64()
		if !ok {
			return false, nil
		}
		x = n
	case *BoolObject:
		if v.Value {
			x = 1
		}
	case *FloatObject:
		if v.Value != math.Trunc(v.Value) || math.Abs(v.Value) >= 1<<63 {
			return false, nil
		}
		x = int64(v.Value)
	default:
		return false, nil
	}
	if r.Step > 0 {
		if x < r.Start || x >= r.Stop {
			return false, nil
		}
		return (uint64(x)-uint64(r.Start))%uint64(r.Step) == 0, nil
	}
	if x > r.Start || x <= r.Stop {
		return false, nil
	}
	return (uint64(r.Start)-uint64(x))%(uint64(0)-uint64(r.Step)) == 0, nil
}

// RichCompare: two ranges are equal when they produce the same sequence.
func (r *RangeObject) RichCompare(other Object, op CompareOp) (Object, error) {
	o, ok := other.(*RangeObject)
	if !ok || (op != OpEq && op != OpNotEq) {
		return NotImplemented, nil
	}
	n := r.length()
	eq := n == o.length() && (n == 0 || (r.Start == o.Start && (n == 1 || r.Step == o.Step)))
	return BoolFrom(eq == (op == OpEq)), nil
}

// GetAttr exposes start, stop and step.
func (r *RangeObject) GetAttr(name string) (Object, error) {
	switch name {
	case "start":
		return IntFrom(r.Start), nil
	case "stop":
		return IntFrom(r.Stop), nil
	case "step":
		return IntFrom(r.Step), nil
	}
	return nil, NewAttributeError("range", name)
}
func (r *RangeObject) SetAttr(name string, _ Object) error {
	return NewAttributeError("range", name)
}
func (r *RangeObject) DelAttr(name string) error { return NewAttributeError("range", name) }
