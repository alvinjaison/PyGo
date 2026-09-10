package object

import (
	"fmt"
	"unicode/utf8"
)

// ============================================================
// Slice operations for sequences (list, tuple, string)
// ============================================================

// SliceGetList returns a new list containing the elements selected by the slice.
func SliceGetList(l *ListObject, s *SliceObject) (*ListObject, error) {
	start, stop, step, err := s.Indices(len(l.Items))
	if err != nil {
		return nil, err
	}
	items := collectSliceItems(l.Items, start, stop, step)
	return &ListObject{Items: items}, nil
}

// SliceSetList replaces the elements selected by the slice with those from the
// iterable value. When step == 1, the replacement may be a different length.
// When step != 1, the replacement must have exactly the same number of elements
// as the slice selects, or a ValueError is raised.
func SliceSetList(l *ListObject, s *SliceObject, value Object) error {
	start, stop, step, err := s.Indices(len(l.Items))
	if err != nil {
		return err
	}

	// Convert value to a slice of objects.
	replacement, err := iterableToSlice(value)
	if err != nil {
		return NewTypeError("must assign iterable to extended slice")
	}

	// Only step == 1 is a "simple" assignment that may change the list length.
	// Every other step (including -1) is an extended slice, which Python
	// requires to be replaced element-for-element.
	if step == 1 {
		// Simple slice assignment: can grow or shrink the list.
		if start < 0 {
			start = 0
		}
		if stop > len(l.Items) {
			stop = len(l.Items)
		}
		if start > stop {
			start = stop
		}

		// Build new items slice.
		newItems := make([]Object, 0, start+len(replacement)+(len(l.Items)-stop))
		newItems = append(newItems, l.Items[:start]...)
		newItems = append(newItems, replacement...)
		newItems = append(newItems, l.Items[stop:]...)
		l.Items = newItems
	} else {
		// Extended slice assignment (step != 1): replacement must be exactly as
		// long as the slice selects, since the elements are overwritten in place.
		sliceLen := SliceLen(start, stop, step)
		if len(replacement) != sliceLen {
			return NewValueError(
				"attempt to assign sequence of size %d to extended slice of size %d",
				len(replacement), sliceLen)
		}
		idx := 0
		for i := start; ; i += step {
			if step > 0 && i >= stop {
				break
			}
			if step < 0 && i <= stop {
				break
			}
			l.Items[i] = replacement[idx]
			idx++
		}
	}
	return nil
}

// SliceDelList removes the elements selected by the slice from the list.
func SliceDelList(l *ListObject, s *SliceObject) error {
	start, stop, step, err := s.Indices(len(l.Items))
	if err != nil {
		return err
	}

	if step == 1 {
		// Simple contiguous deletion.
		if start < 0 {
			start = 0
		}
		if stop > len(l.Items) {
			stop = len(l.Items)
		}
		if start >= stop {
			return nil
		}
		l.Items = append(l.Items[:start], l.Items[stop:]...)
		return nil
	}

	// Extended slice deletion: collect the indices to remove, then rebuild.
	indices := collectSliceIndices(start, stop, step)
	if len(indices) == 0 {
		return nil
	}

	// Sort indices in ascending order for stable removal.
	sortInts(indices)

	// Remove in reverse order to preserve index validity.
	for i := len(indices) - 1; i >= 0; i-- {
		idx := indices[i]
		l.Items = append(l.Items[:idx], l.Items[idx+1:]...)
	}
	return nil
}

// SliceGetTuple returns a new tuple containing the elements selected by the slice.
func SliceGetTuple(t *TupleObject, s *SliceObject) (*TupleObject, error) {
	start, stop, step, err := s.Indices(len(t.Items))
	if err != nil {
		return nil, err
	}
	items := collectSliceItems(t.Items, start, stop, step)
	return NewTuple(items), nil
}

// SliceGetStr returns a new string containing the characters (runes) selected
// by the slice.
func SliceGetStr(str *StrObject, s *SliceObject) (*StrObject, error) {
	runes := []rune(str.Value)
	length := len(runes)
	start, stop, step, err := s.Indices(length)
	if err != nil {
		return nil, err
	}

	result := collectSliceRunes(runes, start, stop, step)
	return StrFrom(string(result)), nil
}

// ============================================================
// Helper: collect elements from a slice of Objects
// ============================================================

// collectSliceItems returns elements from items[start:stop:step].
func collectSliceItems(items []Object, start, stop, step int) []Object {
	n := SliceLen(start, stop, step)
	if n <= 0 {
		return []Object{}
	}
	result := make([]Object, 0, n)
	for i := start; ; i += step {
		if step > 0 && i >= stop {
			break
		}
		if step < 0 && i <= stop {
			break
		}
		result = append(result, items[i])
	}
	return result
}

// collectSliceRunes returns runes from runes[start:stop:step].
func collectSliceRunes(runes []rune, start, stop, step int) []rune {
	n := SliceLen(start, stop, step)
	if n <= 0 {
		return []rune{}
	}
	result := make([]rune, 0, n)
	for i := start; ; i += step {
		if step > 0 && i >= stop {
			break
		}
		if step < 0 && i <= stop {
			break
		}
		result = append(result, runes[i])
	}
	return result
}

// collectSliceIndices returns the concrete indices selected by a slice.
func collectSliceIndices(start, stop, step int) []int {
	n := SliceLen(start, stop, step)
	if n <= 0 {
		return nil
	}
	indices := make([]int, 0, n)
	for i := start; ; i += step {
		if step > 0 && i >= stop {
			break
		}
		if step < 0 && i <= stop {
			break
		}
		indices = append(indices, i)
	}
	return indices
}

// iterableToSlice converts an Object to a slice of Objects by iterating it.
func iterableToSlice(obj Object) ([]Object, error) {
	// Fast path for list and tuple.
	switch v := obj.(type) {
	case *ListObject:
		return append([]Object{}, v.Items...), nil
	case *TupleObject:
		return append([]Object{}, v.Items...), nil
	}

	// Try the iterator protocol.
	iter, ok := obj.(Iterable)
	if !ok {
		return nil, fmt.Errorf("TypeError: object is not iterable")
	}
	it, err := iter.Iter()
	if err != nil {
		return nil, err
	}
	var result []Object
	for {
		val, err := it.Next()
		if err != nil {
			if IsStopIteration(err) {
				break
			}
			return nil, err
		}
		result = append(result, val)
	}
	return result, nil
}

// sortInts sorts a slice of ints in ascending order (insertion sort for small slices).
func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		key := a[i]
		j := i - 1
		for j >= 0 && a[j] > key {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = key
	}
}

// ============================================================
// StrObject slice helpers for Unicode correctness
// ============================================================

// strRuneCount returns the number of Unicode code points in a string.
// Equivalent to utf8.RuneCountInString but exposed for clarity.
func strRuneCount(s string) int {
	return utf8.RuneCountInString(s)
}
