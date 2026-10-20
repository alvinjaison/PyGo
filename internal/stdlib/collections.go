package stdlib

import (
	"github.com/pygo/pygo/internal/object"
)

// buildCollections constructs the `collections` module: Counter, defaultdict,
// OrderedDict, deque, and namedtuple.
func buildCollections(name string) *object.ModuleObject {
	mod, set := newModule(name)

	set("Counter", fn("collections.Counter", counterConstructor))
	set("defaultdict", fn("collections.defaultdict", defaultDictConstructor))
	set("OrderedDict", fn("collections.OrderedDict", orderedDictConstructor))
	set("deque", fn("collections.deque", dequeConstructor))
	set("namedtuple", fn("collections.namedtuple", namedTupleConstructor))

	return mod
}

// ============================================================
// Counter
// ============================================================

// counterConstructor builds a Counter from an iterable (counting occurrences)
// or a mapping (copying counts).
func counterConstructor(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	c := &CounterObject{counts: object.NewDict()}
	if len(args) == 0 {
		return c, nil
	}
	switch src := args[0].(type) {
	case *object.DictObject:
		for _, k := range src.Keys() {
			v, _ := src.GetItem(k)
			_ = c.counts.SetItem(k, v)
		}
	default:
		items, err := drain(args[0])
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			c.increment(it, 1)
		}
	}
	return c, nil
}

// CounterObject is a dict subtype that counts hashable items. Missing keys
// read as 0 rather than raising KeyError.
type CounterObject struct {
	counts *object.DictObject
}

func (c *CounterObject) Type() *object.TypeObject { return object.DictType }
func (c *CounterObject) String() string           { return "Counter(" + c.counts.Repr() + ")" }
func (c *CounterObject) Repr() string              { return c.String() }
func (c *CounterObject) Len() int                  { return c.counts.Len() }
func (c *CounterObject) IsTrue() (bool, error)     { return c.counts.Len() > 0, nil }

func (c *CounterObject) increment(key object.Object, by int64) {
	cur := int64(0)
	if v, err := c.counts.GetItem(key); err == nil {
		if n, ok := v.(*object.IntObject); ok {
			cur, _ = n.Int64()
		}
	}
	_ = c.counts.SetItem(key, object.IntFrom(cur+by))
}

// GetItem returns the count for a key, defaulting to 0.
func (c *CounterObject) GetItem(key object.Object) (object.Object, error) {
	if v, err := c.counts.GetItem(key); err == nil {
		return v, nil
	}
	return object.IntFrom(0), nil
}

func (c *CounterObject) SetItem(key, value object.Object) error {
	return c.counts.SetItem(key, value)
}

func (c *CounterObject) DelItem(key object.Object) error {
	return c.counts.DelItem(key)
}

func (c *CounterObject) Iter() (object.Iterator, error) { return c.counts.Iter() }

func (c *CounterObject) Contains(item object.Object) (bool, error) {
	return c.counts.Contains(item)
}

// GetAttr exposes Counter methods: most_common, elements, get, keys, values,
// items, update.
func (c *CounterObject) GetAttr(name string) (object.Object, error) {
	switch name {
	case "most_common":
		return fn("Counter.most_common", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			return c.mostCommon(args)
		}), nil
	case "elements":
		return fn("Counter.elements", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			var out []object.Object
			for _, k := range c.counts.Keys() {
				v, _ := c.counts.GetItem(k)
				n, _ := v.(*object.IntObject).Int64()
				for i := int64(0); i < n; i++ {
					out = append(out, k)
				}
			}
			return object.NewList(out), nil
		}), nil
	case "get":
		return fn("Counter.get", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			return c.GetItem(args[0])
		}), nil
	case "keys":
		return fn("Counter.keys", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return object.NewList(c.counts.Keys()), nil
		}), nil
	case "values":
		return fn("Counter.values", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return object.NewList(c.counts.Values()), nil
		}), nil
	case "items":
		return fn("Counter.items", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return dictItems(c.counts), nil
		}), nil
	case "update":
		return fn("Counter.update", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			items, err := drain(args[0])
			if err != nil {
				return nil, err
			}
			for _, it := range items {
				c.increment(it, 1)
			}
			return object.None, nil
		}), nil
	}
	return nil, object.NewAttributeError("Counter", name)
}

func (c *CounterObject) SetAttr(string, object.Object) error {
	return object.NewTypeError("cannot set attributes on Counter")
}
func (c *CounterObject) DelAttr(string) error {
	return object.NewTypeError("cannot delete attributes on Counter")
}

// mostCommon returns the (n) most common (element, count) pairs, descending.
func (c *CounterObject) mostCommon(args []object.Object) (object.Object, error) {
	type pair struct {
		key   object.Object
		count int64
	}
	var pairs []pair
	for _, k := range c.counts.Keys() {
		v, _ := c.counts.GetItem(k)
		n, _ := v.(*object.IntObject).Int64()
		pairs = append(pairs, pair{k, n})
	}
	// Stable descending sort by count (insertion-order preserved for ties).
	for i := 1; i < len(pairs); i++ {
		for j := i; j > 0 && pairs[j].count > pairs[j-1].count; j-- {
			pairs[j], pairs[j-1] = pairs[j-1], pairs[j]
		}
	}
	limit := len(pairs)
	if len(args) > 0 {
		if n, ok := args[0].(*object.IntObject); ok {
			nv, _ := n.Int64()
			if int(nv) < limit {
				limit = int(nv)
			}
		}
	}
	out := make([]object.Object, limit)
	for i := 0; i < limit; i++ {
		out[i] = object.NewTuple([]object.Object{pairs[i].key, object.IntFrom(pairs[i].count)})
	}
	return object.NewList(out), nil
}

// dictItems returns a list of (key, value) tuples for a dict.
func dictItems(d *object.DictObject) object.Object {
	keys := d.Keys()
	out := make([]object.Object, len(keys))
	for i, k := range keys {
		v, _ := d.GetItem(k)
		out[i] = object.NewTuple([]object.Object{k, v})
	}
	return object.NewList(out)
}
