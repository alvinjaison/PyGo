package object

import "fmt"

// ============================================================
// ListObject
// ============================================================

// ListObject represents a Python list (mutable ordered sequence).
type ListObject struct {
	Items []Object
}

func NewList(items []Object) *ListObject {
	return &ListObject{Items: append([]Object{}, items...)}
}

func (l *ListObject) Type() *TypeObject { return ListType }
func (l *ListObject) String() string    { return reprCollection("[", "]", l.Items) }
func (l *ListObject) Repr() string      { return l.String() }
func (l *ListObject) Len() int          { return len(l.Items) }
func (l *ListObject) IsTrue() (bool, error) { return len(l.Items) > 0, nil }

func (l *ListObject) GetItem(key Object) (Object, error) {
	idx, err := resolveIndex(key, len(l.Items))
	if err != nil {
		return nil, err
	}
	return l.Items[idx], nil
}

func (l *ListObject) SetItem(key Object, value Object) error {
	idx, err := resolveIndex(key, len(l.Items))
	if err != nil {
		return err
	}
	l.Items[idx] = value
	return nil
}

func (l *ListObject) DelItem(key Object) error {
	idx, err := resolveIndex(key, len(l.Items))
	if err != nil {
		return err
	}
	l.Items = append(l.Items[:idx], l.Items[idx+1:]...)
	return nil
}

func (l *ListObject) Append(item Object) {
	l.Items = append(l.Items, item)
}

// Iter returns a list iterator.
func (l *ListObject) Iter() (Iterator, error) {
	return &listIterator{list: l, index: 0}, nil
}

// Contains implements the `in` operator.
func (l *ListObject) Contains(item Object) (bool, error) {
	for _, v := range l.Items {
		eq, err := objectsEqual(v, item)
		if err != nil {
			return false, err
		}
		if eq {
			return true, nil
		}
	}
	return false, nil
}

type listIterator struct {
	list  *ListObject
	index int
}

func (it *listIterator) Type() *TypeObject { return ObjectType } // TODO: listiterator type
func (it *listIterator) String() string    { return "<list_iterator>" }
func (it *listIterator) Repr() string      { return it.String() }
func (it *listIterator) Next() (Object, error) {
	if it.index >= len(it.list.Items) {
		return nil, ErrStopIteration
	}
	v := it.list.Items[it.index]
	it.index++
	return v, nil
}

// ============================================================
// TupleObject
// ============================================================

// TupleObject represents a Python tuple (immutable ordered sequence).
type TupleObject struct {
	Items []Object
}

// emptyTuple is a singleton for the empty tuple.
var emptyTuple = &TupleObject{Items: []Object{}}

func NewTuple(items []Object) *TupleObject {
	if len(items) == 0 {
		return emptyTuple
	}
	return &TupleObject{Items: append([]Object{}, items...)}
}

func (t *TupleObject) Type() *TypeObject { return TupleType }
func (t *TupleObject) String() string {
	if len(t.Items) == 1 {
		return "(" + t.Items[0].Repr() + ",)"
	}
	return reprCollection("(", ")", t.Items)
}
func (t *TupleObject) Repr() string { return t.String() }
func (t *TupleObject) Len() int     { return len(t.Items) }
func (t *TupleObject) IsTrue() (bool, error) { return len(t.Items) > 0, nil }

func (t *TupleObject) GetItem(key Object) (Object, error) {
	idx, err := resolveIndex(key, len(t.Items))
	if err != nil {
		return nil, err
	}
	return t.Items[idx], nil
}

// Tuples are read-only, so SetItem and DelItem always error.
func (t *TupleObject) SetItem(_ Object, _ Object) error {
	return fmt.Errorf("TypeError: 'tuple' object does not support item assignment")
}
func (t *TupleObject) DelItem(_ Object) error {
	return fmt.Errorf("TypeError: 'tuple' object does not support item deletion")
}

func (t *TupleObject) Iter() (Iterator, error) {
	return &tupleIterator{tuple: t, index: 0}, nil
}

func (t *TupleObject) Hash() (int64, error) {
	// Tuple hash: combination of element hashes.
	// Algorithm mirrors CPython's tuplehash.
	var h int64 = 0x345678
	mult := int64(1000003)
	for _, item := range t.Items {
		hsh, ok := item.(Hashable)
		if !ok {
			return 0, fmt.Errorf("TypeError: unhashable type: '%s'", item.Type().Name())
		}
		itemHash, err := hsh.Hash()
		if err != nil {
			return 0, err
		}
		h = (h ^ itemHash) * mult
		mult++
	}
	h += int64(len(t.Items) + 82520) // CPython suffix
	if h == -1 {
		h = -2
	}
	return h, nil
}

type tupleIterator struct {
	tuple *TupleObject
	index int
}

func (it *tupleIterator) Type() *TypeObject { return ObjectType }
func (it *tupleIterator) String() string    { return "<tuple_iterator>" }
func (it *tupleIterator) Repr() string      { return it.String() }
func (it *tupleIterator) Next() (Object, error) {
	if it.index >= len(it.tuple.Items) {
		return nil, ErrStopIteration
	}
	v := it.tuple.Items[it.index]
	it.index++
	return v, nil
}

// ============================================================
// DictObject
// ============================================================

// DictObject represents a Python dict (insertion-ordered mutable mapping).
//
// Implementation: parallel slice of entries (for order) + map of hash → []int
// (for O(1) lookup). This is a simplified version of CPython's compact dict.
type DictObject struct {
	entries []dictEntry
	index   map[int64][]int // hash → list of entry indices
}

type dictEntry struct {
	key   Object
	value Object
	hash  int64
}

func NewDict() *DictObject {
	return &DictObject{
		index: make(map[int64][]int),
	}
}

func (d *DictObject) Type() *TypeObject { return DictType }
func (d *DictObject) String() string {
	var sb []byte
	sb = append(sb, '{')
	for i, e := range d.entries {
		if i > 0 {
			sb = append(sb, ',', ' ')
		}
		sb = append(sb, []byte(e.key.Repr())...)
		sb = append(sb, ':', ' ')
		sb = append(sb, []byte(e.value.Repr())...)
	}
	sb = append(sb, '}')
	return string(sb)
}
func (d *DictObject) Repr() string { return d.String() }
func (d *DictObject) Len() int     { return len(d.entries) }
func (d *DictObject) IsTrue() (bool, error) { return len(d.entries) > 0, nil }

// GetItem retrieves the value for a key, or returns a KeyError.
func (d *DictObject) GetItem(key Object) (Object, error) {
	h, err := hashObject(key)
	if err != nil {
		return nil, err
	}
	for _, idx := range d.index[h] {
		eq, err := objectsEqual(d.entries[idx].key, key)
		if err != nil {
			return nil, err
		}
		if eq {
			return d.entries[idx].value, nil
		}
	}
	return nil, fmt.Errorf("KeyError: %s", key.Repr())
}

// SetItem sets or updates a key-value pair.
func (d *DictObject) SetItem(key Object, value Object) error {
	h, err := hashObject(key)
	if err != nil {
		return err
	}
	// Check if key already exists.
	for _, idx := range d.index[h] {
		eq, err := objectsEqual(d.entries[idx].key, key)
		if err != nil {
			return err
		}
		if eq {
			d.entries[idx].value = value
			return nil
		}
	}
	// New key: append to entries and add to index.
	newIdx := len(d.entries)
	d.entries = append(d.entries, dictEntry{key: key, value: value, hash: h})
	d.index[h] = append(d.index[h], newIdx)
	return nil
}

// DelItem removes a key-value pair.
func (d *DictObject) DelItem(key Object) error {
	h, err := hashObject(key)
	if err != nil {
		return err
	}
	indices := d.index[h]
	for i, idx := range indices {
		eq, err := objectsEqual(d.entries[idx].key, key)
		if err != nil {
			return err
		}
		if eq {
			// Remove from entries (maintain relative order).
			d.entries = append(d.entries[:idx], d.entries[idx+1:]...)
			// Remove from index.
			d.index[h] = append(indices[:i], indices[i+1:]...)
			// Rebuild index for entries after the removed one (indices shifted).
			d.rebuildIndex()
			return nil
		}
	}
	return fmt.Errorf("KeyError: %s", key.Repr())
}

// rebuildIndex rebuilds the hash → index map after a deletion.
func (d *DictObject) rebuildIndex() {
	d.index = make(map[int64][]int, len(d.entries))
	for i, e := range d.entries {
		d.index[e.hash] = append(d.index[e.hash], i)
	}
}

// Keys returns all keys in insertion order.
func (d *DictObject) Keys() []Object {
	keys := make([]Object, len(d.entries))
	for i, e := range d.entries {
		keys[i] = e.key
	}
	return keys
}

// Values returns all values in insertion order.
func (d *DictObject) Values() []Object {
	vals := make([]Object, len(d.entries))
	for i, e := range d.entries {
		vals[i] = e.value
	}
	return vals
}

// Contains implements the `in` operator (key membership test).
func (d *DictObject) Contains(key Object) (bool, error) {
	_, err := d.GetItem(key)
	if err != nil {
		// KeyError means not found — not an error for `in`.
		return false, nil
	}
	return true, nil
}

// Iter returns an iterator over the dict's keys (matching Python dict iteration).
func (d *DictObject) Iter() (Iterator, error) {
	keys := d.Keys()
	return NewList(keys).Iter()
}

// ============================================================
// Helpers
// ============================================================

// hashObject returns the hash of obj, or an error if unhashable.
func hashObject(obj Object) (int64, error) {
	h, ok := obj.(Hashable)
	if !ok {
		return 0, fmt.Errorf("TypeError: unhashable type: '%s'", obj.Type().Name())
	}
	return h.Hash()
}

// objectsEqual returns whether a and b compare equal using Python semantics.
// TODO(Phase 2): Use the full RichCompare protocol.
func objectsEqual(a, b Object) (bool, error) {
	if a == b {
		return true, nil
	}
	if c, ok := a.(Comparable); ok {
		result, err := c.RichCompare(b, OpEq)
		if err != nil {
			return false, err
		}
		if !IsNotImplemented(result) {
			return IsTruthy(result)
		}
	}
	if c, ok := b.(Comparable); ok {
		result, err := c.RichCompare(a, OpEq)
		if err != nil {
			return false, err
		}
		if !IsNotImplemented(result) {
			return IsTruthy(result)
		}
	}
	return false, nil
}

// resolveIndex converts a Python index (possibly negative) to a slice index.
func resolveIndex(key Object, length int) (int, error) {
	intKey, ok := key.(*IntObject)
	if !ok {
		return 0, fmt.Errorf("TypeError: indices must be integers, not '%s'", key.Type().Name())
	}
	idx64, fits := intKey.Int64()
	if !fits {
		return 0, fmt.Errorf("IndexError: index out of range")
	}
	idx := int(idx64)
	if idx < 0 {
		idx += length
	}
	if idx < 0 || idx >= length {
		return 0, fmt.Errorf("IndexError: index out of range")
	}
	return idx, nil
}

// reprCollection formats a sequence as "prefix item, item suffix".
func reprCollection(open, close string, items []Object) string {
	var sb []byte
	sb = append(sb, []byte(open)...)
	for i, item := range items {
		if i > 0 {
			sb = append(sb, ',', ' ')
		}
		sb = append(sb, []byte(item.Repr())...)
	}
	sb = append(sb, []byte(close)...)
	return string(sb)
}
