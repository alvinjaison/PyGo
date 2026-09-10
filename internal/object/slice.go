package object

import "fmt"

// SliceType is the type of slice objects.
var SliceType = newTypeObject("slice", []*TypeObject{ObjectType}, TypeSlots{})

// SliceObject represents a Python slice(start, stop, step).
// Start, Stop, and Step may be None or IntObject.
type SliceObject struct {
	Start Object // None or *IntObject
	Stop  Object // None or *IntObject
	Step  Object // None or *IntObject
}

// NewSlice creates a new SliceObject from start, stop, step values.
func NewSlice(start, stop, step Object) *SliceObject {
	return &SliceObject{Start: start, Stop: stop, Step: step}
}

func (s *SliceObject) Type() *TypeObject { return SliceType }

func (s *SliceObject) String() string {
	return fmt.Sprintf("slice(%s, %s, %s)",
		reprOrNone(s.Start), reprOrNone(s.Stop), reprOrNone(s.Step))
}

func (s *SliceObject) Repr() string { return s.String() }

// reprOrNone returns the repr of an object, used for slice printing.
func reprOrNone(obj Object) string {
	if IsNone(obj) {
		return "None"
	}
	return obj.Repr()
}

// Indices computes concrete (start, stop, step) indices for a sequence of the
// given length. It applies defaults for None values, resolves negative indices,
// and clamps to valid bounds. This matches Python's slice.indices(length).
//
// Returns ValueError if step is zero.
func (s *SliceObject) Indices(length int) (start, stop, step int, err error) {
	// 1. Resolve step (default 1).
	step, err = sliceIndex(s.Step, 1)
	if err != nil {
		return 0, 0, 0, err
	}
	if step == 0 {
		return 0, 0, 0, NewValueError("slice step cannot be zero")
	}

	// 2. Resolve start.
	if IsNone(s.Start) {
		if step < 0 {
			start = length - 1
		} else {
			start = 0
		}
	} else {
		start, err = sliceIndex(s.Start, 0)
		if err != nil {
			return 0, 0, 0, err
		}
		if start < 0 {
			start += length
		}
		// Clamp start.
		if step < 0 {
			if start < -1 {
				start = -1
			} else if start >= length {
				start = length - 1
			}
		} else {
			if start < 0 {
				start = 0
			} else if start >= length {
				start = length
			}
		}
	}

	// 3. Resolve stop.
	if IsNone(s.Stop) {
		if step < 0 {
			stop = -1
		} else {
			stop = length
		}
	} else {
		stop, err = sliceIndex(s.Stop, 0)
		if err != nil {
			return 0, 0, 0, err
		}
		if stop < 0 {
			stop += length
		}
		// Clamp stop.
		if step < 0 {
			if stop < -1 {
				stop = -1
			} else if stop >= length {
				stop = length - 1
			}
		} else {
			if stop < 0 {
				stop = 0
			} else if stop >= length {
				stop = length
			}
		}
	}

	return start, stop, step, nil
}

// sliceIndex extracts an int from an Object that should be an int or None.
// If the object is None, it returns the provided default value.
func sliceIndex(obj Object, defaultVal int) (int, error) {
	if IsNone(obj) {
		return defaultVal, nil
	}
	intObj, ok := obj.(*IntObject)
	if !ok {
		// Check for bool (which is a subtype of int in Python).
		if b, ok := obj.(*BoolObject); ok {
			if b.Value {
				return 1, nil
			}
			return 0, nil
		}
		return 0, NewTypeError("slice indices must be integers or None or have an __index__ method")
	}
	val, fits := intObj.Int64()
	if !fits {
		// For very large ints, clamp to max/min int.
		// Python does this transparently — very large positive → MaxInt,
		// very large negative → MinInt. The caller clamps further.
		if intObj.Value.Sign() > 0 {
			return int(^uint(0) >> 1), nil // max int
		}
		return -int(^uint(0)>>1) - 1, nil // min int
	}
	return int(val), nil
}

// SliceLen returns the number of elements selected by a slice with the given
// start, stop, step (all already resolved by Indices).
func SliceLen(start, stop, step int) int {
	if step > 0 {
		if stop <= start {
			return 0
		}
		return (stop - start + step - 1) / step
	}
	// step < 0
	if start <= stop {
		return 0
	}
	return (start - stop - step - 1) / (-step)
}
