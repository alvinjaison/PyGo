package stdlib

import "github.com/pygo/pygo/internal/object"

// ============================================================
// defaultdict
// ============================================================

// defaultDictConstructor builds a defaultdict with the given default factory.
func defaultDictConstructor(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	d := &DefaultDictObject{data: object.NewDict()}
	if len(args) > 0 && !object.IsNone(args[0]) {
		d.factory = args[0]
	}
	return d, nil
}

// DefaultDictObject is a dict that creates missing values via a factory.
type DefaultDictObject struct {
	data    *object.DictObject
	factory object.Object // a zero-arg callable, or nil
}

func (d *DefaultDictObject) Type() *object.TypeObject { return object.DictType }
func (d *DefaultDictObject) String() string           { return "defaultdict(" + d.data.Repr() + ")" }
func (d *DefaultDictObject) Repr() string              { return d.String() }
func (d *DefaultDictObject) Len() int                  { return d.data.Len() }
func (d *DefaultDictObject) IsTrue() (bool, error)     { return d.data.Len() > 0, nil }

// GetItem returns the value for key, creating it from the factory if absent.
func (d *DefaultDictObject) GetItem(key object.Object) (object.Object, error) {
	if v, err := d.data.GetItem(key); err == nil {
		return v, nil
	}
	if d.factory == nil {
		return nil, object.NewKeyError(key)
	}
	val, err := object.Invoke(d.factory, nil)
	if err != nil {
		return nil, err
	}
	if err := d.data.SetItem(key, val); err != nil {
		return nil, err
	}
	return val, nil
}

func (d *DefaultDictObject) SetItem(key, value object.Object) error {
	return d.data.SetItem(key, value)
}
func (d *DefaultDictObject) DelItem(key object.Object) error { return d.data.DelItem(key) }
func (d *DefaultDictObject) Iter() (object.Iterator, error)  { return d.data.Iter() }
func (d *DefaultDictObject) Contains(item object.Object) (bool, error) {
	return d.data.Contains(item)
}

func (d *DefaultDictObject) GetAttr(name string) (object.Object, error) {
	return dictLikeAttr(name, d.data, "defaultdict")
}
func (d *DefaultDictObject) SetAttr(string, object.Object) error {
	return object.NewTypeError("cannot set attributes on defaultdict")
}
func (d *DefaultDictObject) DelAttr(string) error {
	return object.NewTypeError("cannot delete attributes on defaultdict")
}

// ============================================================
// OrderedDict
// ============================================================

// orderedDictConstructor builds an OrderedDict (PyGo dicts are already
// insertion-ordered, so this wraps a plain dict with the same semantics).
func orderedDictConstructor(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	d := &OrderedDictObject{data: object.NewDict()}
	if len(args) > 0 {
		if src, ok := args[0].(*object.DictObject); ok {
			for _, k := range src.Keys() {
				v, _ := src.GetItem(k)
				_ = d.data.SetItem(k, v)
			}
		} else {
			// Accept an iterable of (key, value) pairs.
			items, err := drain(args[0])
			if err != nil {
				return nil, err
			}
			for _, it := range items {
				pair, err := drain(it)
				if err != nil || len(pair) != 2 {
					return nil, object.NewValueError("dictionary update sequence element is not a pair")
				}
				_ = d.data.SetItem(pair[0], pair[1])
			}
		}
	}
	return d, nil
}

// OrderedDictObject is an insertion-ordered mapping.
type OrderedDictObject struct {
	data *object.DictObject
}

func (d *OrderedDictObject) Type() *object.TypeObject { return object.DictType }
func (d *OrderedDictObject) String() string           { return "OrderedDict(" + d.data.Repr() + ")" }
func (d *OrderedDictObject) Repr() string              { return d.String() }
func (d *OrderedDictObject) Len() int                  { return d.data.Len() }
func (d *OrderedDictObject) IsTrue() (bool, error)     { return d.data.Len() > 0, nil }

func (d *OrderedDictObject) GetItem(key object.Object) (object.Object, error) {
	return d.data.GetItem(key)
}
func (d *OrderedDictObject) SetItem(key, value object.Object) error {
	return d.data.SetItem(key, value)
}
func (d *OrderedDictObject) DelItem(key object.Object) error { return d.data.DelItem(key) }
func (d *OrderedDictObject) Iter() (object.Iterator, error)  { return d.data.Iter() }
func (d *OrderedDictObject) Contains(item object.Object) (bool, error) {
	return d.data.Contains(item)
}
func (d *OrderedDictObject) GetAttr(name string) (object.Object, error) {
	return dictLikeAttr(name, d.data, "OrderedDict")
}
func (d *OrderedDictObject) SetAttr(string, object.Object) error {
	return object.NewTypeError("cannot set attributes on OrderedDict")
}
func (d *OrderedDictObject) DelAttr(string) error {
	return object.NewTypeError("cannot delete attributes on OrderedDict")
}

// dictLikeAttr provides the common mapping methods (keys/values/items/get)
// shared by Counter, defaultdict, and OrderedDict.
func dictLikeAttr(name string, data *object.DictObject, typeName string) (object.Object, error) {
	switch name {
	case "keys":
		return fn(typeName+".keys", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return object.NewList(data.Keys()), nil
		}), nil
	case "values":
		return fn(typeName+".values", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return object.NewList(data.Values()), nil
		}), nil
	case "items":
		return fn(typeName+".items", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return dictItems(data), nil
		}), nil
	case "get":
		return fn(typeName+".get", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			if v, err := data.GetItem(args[0]); err == nil {
				return v, nil
			}
			if len(args) > 1 {
				return args[1], nil
			}
			return object.None, nil
		}), nil
	}
	return nil, object.NewAttributeError(typeName, name)
}

// ============================================================
// deque
// ============================================================

// dequeConstructor builds a deque, optionally from an iterable.
func dequeConstructor(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	dq := &DequeObject{}
	if len(args) > 0 && !object.IsNone(args[0]) {
		items, err := drain(args[0])
		if err != nil {
			return nil, err
		}
		dq.items = items
	}
	return dq, nil
}

// DequeObject is a double-ended queue backed by a slice.
type DequeObject struct {
	items []object.Object
}

func (dq *DequeObject) Type() *object.TypeObject { return object.ListType }
func (dq *DequeObject) String() string {
	return "deque(" + object.NewList(dq.items).Repr() + ")"
}
func (dq *DequeObject) Repr() string          { return dq.String() }
func (dq *DequeObject) Len() int              { return len(dq.items) }
func (dq *DequeObject) IsTrue() (bool, error) { return len(dq.items) > 0, nil }

func (dq *DequeObject) Iter() (object.Iterator, error) {
	return object.NewList(dq.items).Iter()
}

func (dq *DequeObject) GetItem(key object.Object) (object.Object, error) {
	return object.NewList(dq.items).GetItem(key)
}

func (dq *DequeObject) Contains(item object.Object) (bool, error) {
	return object.NewList(dq.items).Contains(item)
}

func (dq *DequeObject) GetAttr(name string) (object.Object, error) {
	switch name {
	case "append":
		return fn("deque.append", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			dq.items = append(dq.items, args[0])
			return object.None, nil
		}), nil
	case "appendleft":
		return fn("deque.appendleft", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			dq.items = append([]object.Object{args[0]}, dq.items...)
			return object.None, nil
		}), nil
	case "pop":
		return fn("deque.pop", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			if len(dq.items) == 0 {
				return nil, object.NewException(object.TypeIndexError, "pop from an empty deque")
			}
			last := dq.items[len(dq.items)-1]
			dq.items = dq.items[:len(dq.items)-1]
			return last, nil
		}), nil
	case "popleft":
		return fn("deque.popleft", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			if len(dq.items) == 0 {
				return nil, object.NewException(object.TypeIndexError, "pop from an empty deque")
			}
			first := dq.items[0]
			dq.items = dq.items[1:]
			return first, nil
		}), nil
	case "extend":
		return fn("deque.extend", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			items, err := drain(args[0])
			if err != nil {
				return nil, err
			}
			dq.items = append(dq.items, items...)
			return object.None, nil
		}), nil
	case "clear":
		return fn("deque.clear", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			dq.items = nil
			return object.None, nil
		}), nil
	}
	return nil, object.NewAttributeError("deque", name)
}
func (dq *DequeObject) SetAttr(string, object.Object) error {
	return object.NewTypeError("cannot set attributes on deque")
}
func (dq *DequeObject) DelAttr(string) error {
	return object.NewTypeError("cannot delete attributes on deque")
}
