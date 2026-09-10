package object

import (
	"fmt"
	"sort"
)

// ============================================================
// List methods (accessible via LOAD_ATTR)
// ============================================================

// GetAttr implements AttributeHolder for ListObject.
func (l *ListObject) GetAttr(name string) (Object, error) {
	switch name {
	case "append":
		return listBoundMethod(l, "append", listMethodAppend), nil
	case "pop":
		return listBoundMethod(l, "pop", listMethodPop), nil
	case "clear":
		return listBoundMethod(l, "clear", listMethodClear), nil
	case "extend":
		return listBoundMethod(l, "extend", listMethodExtend), nil
	case "sort":
		return listBoundMethod(l, "sort", listMethodSort), nil
	case "reverse":
		return listBoundMethod(l, "reverse", listMethodReverse), nil
	case "insert":
		return listBoundMethod(l, "insert", listMethodInsert), nil
	case "remove":
		return listBoundMethod(l, "remove", listMethodRemove), nil
	case "index":
		return listBoundMethod(l, "index", listMethodIndex), nil
	case "count":
		return listBoundMethod(l, "count", listMethodCount), nil
	case "copy":
		return listBoundMethod(l, "copy", listMethodCopy), nil
	default:
		return nil, NewAttributeError("list", name)
	}
}

// SetAttr returns an error since lists don't support attribute assignment.
func (l *ListObject) SetAttr(name string, value Object) error {
	return NewAttributeError("list", name)
}

// DelAttr returns an error since lists don't support attribute deletion.
func (l *ListObject) DelAttr(name string) error {
	return NewAttributeError("list", name)
}

// listBoundMethod creates a bound method for a list instance.
func listBoundMethod(l *ListObject, name string, fn func(*ListObject, []Object, map[string]Object) (Object, error)) *BuiltinFuncObject {
	return &BuiltinFuncObject{
		Name: name,
		Fn: func(args []Object, kwargs map[string]Object) (Object, error) {
			return fn(l, args, kwargs)
		},
	}
}

func listMethodAppend(l *ListObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: append() takes exactly one argument (%d given)", len(args))
	}
	l.Items = append(l.Items, args[0])
	return None, nil
}

func listMethodPop(l *ListObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(l.Items) == 0 {
		return nil, fmt.Errorf("IndexError: pop from empty list")
	}
	idx := len(l.Items) - 1
	if len(args) == 1 {
		i, err := resolveIndex(args[0], len(l.Items))
		if err != nil {
			return nil, err
		}
		idx = i
	} else if len(args) > 1 {
		return nil, fmt.Errorf("TypeError: pop() takes at most 1 argument (%d given)", len(args))
	}
	item := l.Items[idx]
	l.Items = append(l.Items[:idx], l.Items[idx+1:]...)
	return item, nil
}

func listMethodClear(l *ListObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("TypeError: clear() takes no arguments (%d given)", len(args))
	}
	l.Items = l.Items[:0]
	return None, nil
}

func listMethodExtend(l *ListObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: extend() takes exactly one argument (%d given)", len(args))
	}
	it, err := GetIter(args[0])
	if err != nil {
		return nil, err
	}
	for {
		item, err := it.Next()
		if IsStopIteration(err) {
			break
		}
		if err != nil {
			return nil, err
		}
		l.Items = append(l.Items, item)
	}
	return None, nil
}

func listMethodSort(l *ListObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("TypeError: sort() takes no positional arguments")
	}
	// Simple sort: compare elements using < operator.
	var sortErr error
	sort.SliceStable(l.Items, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		a, b := l.Items[i], l.Items[j]
		result, err := compareForSort(a, b)
		if err != nil {
			sortErr = err
			return false
		}
		return result
	})
	if sortErr != nil {
		return nil, sortErr
	}
	return None, nil
}

func listMethodReverse(l *ListObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("TypeError: reverse() takes no arguments (%d given)", len(args))
	}
	for i, j := 0, len(l.Items)-1; i < j; i, j = i+1, j-1 {
		l.Items[i], l.Items[j] = l.Items[j], l.Items[i]
	}
	return None, nil
}

func listMethodInsert(l *ListObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("TypeError: insert() takes exactly 2 arguments (%d given)", len(args))
	}
	intKey, ok := args[0].(*IntObject)
	if !ok {
		return nil, fmt.Errorf("TypeError: integer argument expected, got '%s'", args[0].Type().Name())
	}
	idx64, _ := intKey.Int64()
	idx := int(idx64)
	if idx < 0 {
		idx += len(l.Items)
		if idx < 0 {
			idx = 0
		}
	}
	if idx > len(l.Items) {
		idx = len(l.Items)
	}
	l.Items = append(l.Items, nil)
	copy(l.Items[idx+1:], l.Items[idx:])
	l.Items[idx] = args[1]
	return None, nil
}

func listMethodRemove(l *ListObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: remove() takes exactly one argument (%d given)", len(args))
	}
	for i, item := range l.Items {
		eq, err := objectsEqual(item, args[0])
		if err != nil {
			return nil, err
		}
		if eq {
			l.Items = append(l.Items[:i], l.Items[i+1:]...)
			return None, nil
		}
	}
	return nil, fmt.Errorf("ValueError: list.remove(x): x not in list")
}

func listMethodIndex(l *ListObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("TypeError: index() takes 1 to 3 arguments (%d given)", len(args))
	}
	start := 0
	end := len(l.Items)
	if len(args) >= 2 {
		s, ok := args[1].(*IntObject)
		if !ok {
			return nil, fmt.Errorf("TypeError: integer argument expected")
		}
		sv, _ := s.Int64()
		start = int(sv)
		if start < 0 {
			start += len(l.Items)
			if start < 0 {
				start = 0
			}
		}
	}
	if len(args) == 3 {
		e, ok := args[2].(*IntObject)
		if !ok {
			return nil, fmt.Errorf("TypeError: integer argument expected")
		}
		ev, _ := e.Int64()
		end = int(ev)
		if end < 0 {
			end += len(l.Items)
		}
	}
	for i := start; i < end && i < len(l.Items); i++ {
		eq, err := objectsEqual(l.Items[i], args[0])
		if err != nil {
			return nil, err
		}
		if eq {
			return IntFrom(int64(i)), nil
		}
	}
	return nil, fmt.Errorf("ValueError: %s is not in list", args[0].Repr())
}

func listMethodCount(l *ListObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: count() takes exactly one argument (%d given)", len(args))
	}
	count := 0
	for _, item := range l.Items {
		eq, err := objectsEqual(item, args[0])
		if err != nil {
			return nil, err
		}
		if eq {
			count++
		}
	}
	return IntFrom(int64(count)), nil
}

func listMethodCopy(l *ListObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("TypeError: copy() takes no arguments (%d given)", len(args))
	}
	return NewList(l.Items), nil
}

// ============================================================
// Tuple methods (minimal - tuples are immutable)
// ============================================================

// GetAttr implements AttributeHolder for TupleObject.
func (t *TupleObject) GetAttr(name string) (Object, error) {
	switch name {
	case "count":
		return tupleBoundMethod(t, "count", tupleMethodCount), nil
	case "index":
		return tupleBoundMethod(t, "index", tupleMethodIndex), nil
	default:
		return nil, NewAttributeError("tuple", name)
	}
}

// SetAttr returns an error since tuples don't support attribute assignment.
func (t *TupleObject) SetAttr(name string, value Object) error {
	return NewAttributeError("tuple", name)
}

// DelAttr returns an error since tuples don't support attribute deletion.
func (t *TupleObject) DelAttr(name string) error {
	return NewAttributeError("tuple", name)
}

func tupleBoundMethod(t *TupleObject, name string, fn func(*TupleObject, []Object, map[string]Object) (Object, error)) *BuiltinFuncObject {
	return &BuiltinFuncObject{
		Name: name,
		Fn: func(args []Object, kwargs map[string]Object) (Object, error) {
			return fn(t, args, kwargs)
		},
	}
}

func tupleMethodCount(t *TupleObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: count() takes exactly one argument (%d given)", len(args))
	}
	count := 0
	for _, item := range t.Items {
		eq, err := objectsEqual(item, args[0])
		if err != nil {
			return nil, err
		}
		if eq {
			count++
		}
	}
	return IntFrom(int64(count)), nil
}

func tupleMethodIndex(t *TupleObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("TypeError: index() takes 1 to 3 arguments (%d given)", len(args))
	}
	start := 0
	end := len(t.Items)
	if len(args) >= 2 {
		s, ok := args[1].(*IntObject)
		if !ok {
			return nil, fmt.Errorf("TypeError: integer argument expected")
		}
		sv, _ := s.Int64()
		start = int(sv)
		if start < 0 {
			start += len(t.Items)
			if start < 0 {
				start = 0
			}
		}
	}
	if len(args) == 3 {
		e, ok := args[2].(*IntObject)
		if !ok {
			return nil, fmt.Errorf("TypeError: integer argument expected")
		}
		ev, _ := e.Int64()
		end = int(ev)
		if end < 0 {
			end += len(t.Items)
		}
	}
	for i := start; i < end && i < len(t.Items); i++ {
		eq, err := objectsEqual(t.Items[i], args[0])
		if err != nil {
			return nil, err
		}
		if eq {
			return IntFrom(int64(i)), nil
		}
	}
	return nil, fmt.Errorf("ValueError: tuple.index(x): x not in tuple")
}

// ============================================================
// Dict methods (accessible via LOAD_ATTR)
// ============================================================

// GetAttr implements AttributeHolder for DictObject.
func (d *DictObject) GetAttr(name string) (Object, error) {
	switch name {
	case "keys":
		return dictBoundMethod(d, "keys", dictMethodKeys), nil
	case "values":
		return dictBoundMethod(d, "values", dictMethodValues), nil
	case "items":
		return dictBoundMethod(d, "items", dictMethodItems), nil
	case "get":
		return dictBoundMethod(d, "get", dictMethodGet), nil
	case "pop":
		return dictBoundMethod(d, "pop", dictMethodPop), nil
	case "update":
		return dictBoundMethod(d, "update", dictMethodUpdate), nil
	case "clear":
		return dictBoundMethod(d, "clear", dictMethodClear), nil
	case "setdefault":
		return dictBoundMethod(d, "setdefault", dictMethodSetdefault), nil
	case "copy":
		return dictBoundMethod(d, "copy", dictMethodCopy), nil
	default:
		return nil, NewAttributeError("dict", name)
	}
}

// SetAttr returns an error since dicts don't support attribute assignment.
func (d *DictObject) SetAttr(name string, value Object) error {
	return NewAttributeError("dict", name)
}

// DelAttr returns an error since dicts don't support attribute deletion.
func (d *DictObject) DelAttr(name string) error {
	return NewAttributeError("dict", name)
}

func dictBoundMethod(d *DictObject, name string, fn func(*DictObject, []Object, map[string]Object) (Object, error)) *BuiltinFuncObject {
	return &BuiltinFuncObject{
		Name: name,
		Fn: func(args []Object, kwargs map[string]Object) (Object, error) {
			return fn(d, args, kwargs)
		},
	}
}

func dictMethodKeys(d *DictObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("TypeError: keys() takes no arguments (%d given)", len(args))
	}
	return NewList(d.Keys()), nil
}

func dictMethodValues(d *DictObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("TypeError: values() takes no arguments (%d given)", len(args))
	}
	return NewList(d.Values()), nil
}

func dictMethodItems(d *DictObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("TypeError: items() takes no arguments (%d given)", len(args))
	}
	items := make([]Object, 0, d.size)
	for _, e := range d.entries {
		if e.deleted {
			continue
		}
		items = append(items, NewTuple([]Object{e.key, e.value}))
	}
	return NewList(items), nil
}

func dictMethodGet(d *DictObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("TypeError: get() takes 1 to 2 arguments (%d given)", len(args))
	}
	val, err := d.GetItem(args[0])
	if err != nil {
		// Key not found - return default
		if len(args) == 2 {
			return args[1], nil
		}
		return None, nil
	}
	return val, nil
}

func dictMethodPop(d *DictObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("TypeError: pop() takes 1 to 2 arguments (%d given)", len(args))
	}
	val, err := d.GetItem(args[0])
	if err != nil {
		// Key not found
		if len(args) == 2 {
			return args[1], nil
		}
		return nil, fmt.Errorf("KeyError: %s", args[0].Repr())
	}
	// Remove the key
	_ = d.DelItem(args[0])
	return val, nil
}

func dictMethodUpdate(d *DictObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("TypeError: update() takes at most 1 positional argument (%d given)", len(args))
	}
	if len(args) == 1 {
		other, ok := args[0].(*DictObject)
		if !ok {
			return nil, fmt.Errorf("TypeError: cannot convert '%s' to dict", args[0].Type().Name())
		}
		for _, e := range other.entries {
			if e.deleted {
				continue
			}
			if err := d.SetItem(e.key, e.value); err != nil {
				return nil, err
			}
		}
	}
	// Apply kwargs
	for k, v := range kwargs {
		if err := d.SetItem(StrFrom(k), v); err != nil {
			return nil, err
		}
	}
	return None, nil
}

func dictMethodClear(d *DictObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("TypeError: clear() takes no arguments (%d given)", len(args))
	}
	d.entries = d.entries[:0]
	d.indices = make(map[int64][]int)
	d.size = 0
	return None, nil
}

func dictMethodSetdefault(d *DictObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("TypeError: setdefault() takes 1 to 2 arguments (%d given)", len(args))
	}
	val, err := d.GetItem(args[0])
	if err == nil {
		return val, nil
	}
	// Key not found, set it
	defaultVal := Object(None)
	if len(args) == 2 {
		defaultVal = args[1]
	}
	if err := d.SetItem(args[0], defaultVal); err != nil {
		return nil, err
	}
	return defaultVal, nil
}

func dictMethodCopy(d *DictObject, args []Object, kwargs map[string]Object) (Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("TypeError: copy() takes no arguments (%d given)", len(args))
	}
	newDict := NewDict()
	for _, e := range d.entries {
		if e.deleted {
			continue
		}
		if err := newDict.SetItem(e.key, e.value); err != nil {
			return nil, err
		}
	}
	return newDict, nil
}

// ============================================================
// Comparison helper for sort
// ============================================================

// compareForSort returns true if a < b using Python comparison.
func compareForSort(a, b Object) (bool, error) {
	if c, ok := a.(Comparable); ok {
		result, err := c.RichCompare(b, OpLt)
		if err != nil {
			return false, err
		}
		if !IsNotImplemented(result) {
			return IsTruthy(result)
		}
	}
	return false, fmt.Errorf("TypeError: '<' not supported between instances of '%s' and '%s'",
		a.Type().Name(), b.Type().Name())
}
