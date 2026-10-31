package object

import (
	"sort"

	"github.com/alvinjaison/PyGo/internal/bytecode"
)

// SortObjects sorts items in place the way list.sort and sorted do: a stable
// sort by Python's < ordering, optionally on key(item), optionally reversed.
// With reverse, equal elements keep their original relative order (CPython
// reverses, sorts, then reverses again to guarantee this).
func SortObjects(items []Object, key Object, reverse bool) error {
	keys := items
	if key != nil && !IsNone(key) {
		keys = make([]Object, len(items))
		for i, it := range items {
			k, err := Invoke(key, []Object{it})
			if err != nil {
				return err
			}
			keys[i] = k
		}
	}

	idx := make([]int, len(items))
	for i := range idx {
		idx[i] = i
	}
	if reverse {
		for i, j := 0, len(idx)-1; i < j; i, j = i+1, j-1 {
			idx[i], idx[j] = idx[j], idx[i]
		}
	}

	var sortErr error
	sort.SliceStable(idx, func(a, b int) bool {
		if sortErr != nil {
			return false
		}
		res, err := compareObjects(keys[idx[a]], keys[idx[b]], bytecode.CmpLt)
		if err != nil {
			sortErr = err
			return false
		}
		less, err := IsTruthy(res)
		if err != nil {
			sortErr = err
		}
		return less
	})
	if sortErr != nil {
		return sortErr
	}

	if reverse {
		for i, j := 0, len(idx)-1; i < j; i, j = i+1, j-1 {
			idx[i], idx[j] = idx[j], idx[i]
		}
	}
	sorted := make([]Object, len(items))
	for i, src := range idx {
		sorted[i] = items[src]
	}
	copy(items, sorted)
	return nil
}

// sortOptions extracts the key and reverse keyword arguments of sort/sorted.
func sortOptions(kwargs map[string]Object) (key Object, reverse bool, err error) {
	for name, val := range kwargs {
		switch name {
		case "key":
			key = val
		case "reverse":
			if reverse, err = IsTruthy(val); err != nil {
				return nil, false, err
			}
		default:
			return nil, false, NewTypeError("'%s' is an invalid keyword argument for sort()", name)
		}
	}
	return key, reverse, nil
}

// SortOptions is the exported form used by the sorted() builtin.
func SortOptions(kwargs map[string]Object) (Object, bool, error) { return sortOptions(kwargs) }
