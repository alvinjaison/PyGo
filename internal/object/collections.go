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

func (l *ListObject) Type() *TypeObject    { return ListType }
func (l *ListObject) String() string       { return reprCollection("[", "]", l.Items) }
func (l *ListObject) Repr() string         { return l.String() }
func (l *ListObject) Len() int             { return len(l.Items) }
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

// listIterator iterates over a list's elements.
type listIterator struct {
	list  *ListObject
	index int
}

func (it *listIterator) Type() *TypeObject { return ListIteratorType }
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

func (t *TupleObject) Type() *TypeObject    { return TupleType }
func (t *TupleObject) String() string {
	if len(t.Items) == 1 {
		return "(" + t.Items[0].Repr() + ",)"
	}
	return reprCollection("(", ")", t.Items)
}
func (t *TupleObject) Repr() string         { return t.String() }
func (t *TupleObject) Len() int             { return len(t.Items) }
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

// Contains implements the `in` operator for tuples.
func (t *TupleObject) Contains(item Object) (bool, error) {
	for _, v := range t.Items {
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

// tupleIterator iterates over a tuple's elements.
type tupleIterator struct {
	tuple *TupleObject
	index int
}

func (it *tupleIterator) Type() *TypeObject { return TupleIteratorType }
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
// Implementation uses a parallel slice of entries (for insertion-order iteration)
// and a hash → []int index map (for O(1) lookup with collision chains).
// Deletion uses tombstone markers to preserve iteration order without
// requiring full index rebuild on every delete.
type DictObject struct {
	entries []dictEntry
	indices map[int64][]int // hash → list of entry indices (collision chains)
	size    int             // number of live (non-deleted) entries
}

type dictEntry struct {
	key     Object
	value   Object
	hash    int64
	deleted bool // tombstone marker
}

func NewDict() *DictObject {
	return &DictObject{
		indices: make(map[int64][]int),
	}
}

func (d *DictObject) Type() *TypeObject { return DictType }
func (d *DictObject) String() string {
	var sb []byte
	sb = append(sb, '{')
	first := true
	for _, e := range d.entries {
		if e.deleted {
			continue
		}
		if !first {
			sb = append(sb, ',', ' ')
		}
		first = false
		sb = append(sb, []byte(e.key.Repr())...)
		sb = append(sb, ':', ' ')
		sb = append(sb, []byte(e.value.Repr())...)
	}
	sb = append(sb, '}')
	return string(sb)
}
func (d *DictObject) Repr() string         { return d.String() }
func (d *DictObject) Len() int             { return d.size }
func (d *DictObject) IsTrue() (bool, error) { return d.size > 0, nil }

// GetItem retrieves the value for a key, or returns a KeyError.
func (d *DictObject) GetItem(key Object) (Object, error) {
	h, err := hashObject(key)
	if err != nil {
		return nil, err
	}
	for _, idx := range d.indices[h] {
		entry := &d.entries[idx]
		if entry.deleted {
			continue
		}
		eq, err := objectsEqual(entry.key, key)
		if err != nil {
			return nil, err
		}
		if eq {
			return entry.value, nil
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
	for _, idx := range d.indices[h] {
		entry := &d.entries[idx]
		if entry.deleted {
			continue
		}
		eq, err := objectsEqual(entry.key, key)
		if err != nil {
			return err
		}
		if eq {
			// Replace value in-place (preserves insertion position).
			entry.value = value
			return nil
		}
	}
	// New key: append to entries and add to index.
	newIdx := len(d.entries)
	d.entries = append(d.entries, dictEntry{key: key, value: value, hash: h})
	d.indices[h] = append(d.indices[h], newIdx)
	d.size++
	return nil
}

// DelItem removes a key-value pair using tombstone deletion.
func (d *DictObject) DelItem(key Object) error {
	h, err := hashObject(key)
	if err != nil {
		return err
	}
	chain := d.indices[h]
	for i, idx := range chain {
		entry := &d.entries[idx]
		if entry.deleted {
			continue
		}
		eq, err := objectsEqual(entry.key, key)
		if err != nil {
			return err
		}
		if eq {
			// Tombstone the entry.
			entry.deleted = true
			entry.key = nil
			entry.value = nil
			d.size--
			// Remove from collision chain.
			d.indices[h] = append(chain[:i], chain[i+1:]...)
			if len(d.indices[h]) == 0 {
				delete(d.indices, h)
			}
			// Compact if tombstone ratio exceeds 50%.
			d.maybeCompact()
			return nil
		}
	}
	return fmt.Errorf("KeyError: %s", key.Repr())
}

// maybeCompact rebuilds entries and indices if tombstone ratio exceeds 50%.
func (d *DictObject) maybeCompact() {
	tombstones := len(d.entries) - d.size
	if d.size > 0 && tombstones > d.size {
		d.compact()
	}
}

// compact rebuilds the entries slice and indices map, removing tombstones.
func (d *DictObject) compact() {
	newEntries := make([]dictEntry, 0, d.size)
	newIndices := make(map[int64][]int, d.size)
	for _, e := range d.entries {
		if e.deleted {
			continue
		}
		newIdx := len(newEntries)
		newEntries = append(newEntries, e)
		newIndices[e.hash] = append(newIndices[e.hash], newIdx)
	}
	d.entries = newEntries
	d.indices = newIndices
}

// Keys returns all keys in insertion order.
func (d *DictObject) Keys() []Object {
	keys := make([]Object, 0, d.size)
	for _, e := range d.entries {
		if e.deleted {
			continue
		}
		keys = append(keys, e.key)
	}
	return keys
}

// Values returns all values in insertion order.
func (d *DictObject) Values() []Object {
	vals := make([]Object, 0, d.size)
	for _, e := range d.entries {
		if e.deleted {
			continue
		}
		vals = append(vals, e.value)
	}
	return vals
}

// DictItem is a single key/value pair returned by Items.
type DictItem struct {
	Key   Object
	Value Object
}

// Items returns all key/value pairs in insertion order. Unlike Keys and Values
// it pairs them, which json.dumps and similar callers need in a single pass.
func (d *DictObject) Items() []DictItem {
	items := make([]DictItem, 0, d.size)
	for _, e := range d.entries {
		if e.deleted {
			continue
		}
		items = append(items, DictItem{Key: e.key, Value: e.value})
	}
	return items
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

// Iter returns an iterator over the dict's keys in insertion order.
func (d *DictObject) Iter() (Iterator, error) {
	return &dictIterator{dict: d, index: 0}, nil
}

// dictIterator iterates over a dict's keys in insertion order, skipping tombstones.
type dictIterator struct {
	dict  *DictObject
	index int
}

func (it *dictIterator) Type() *TypeObject { return DictIteratorType }
func (it *dictIterator) String() string    { return "<dict_keyiterator>" }
func (it *dictIterator) Repr() string      { return it.String() }
func (it *dictIterator) Next() (Object, error) {
	for it.index < len(it.dict.entries) {
		entry := &it.dict.entries[it.index]
		it.index++
		if !entry.deleted {
			return entry.key, nil
		}
	}
	return nil, ErrStopIteration
}

// ============================================================
// Iterator type singletons
// ============================================================

var (
	// ListIteratorType is the type of list iterators.
	ListIteratorType *TypeObject

	// TupleIteratorType is the type of tuple iterators.
	TupleIteratorType *TypeObject

	// DictIteratorType is the type of dict key iterators.
	DictIteratorType *TypeObject
)

func init() {
	ListIteratorType = newTypeObject("list_iterator", []*TypeObject{ObjectType}, TypeSlots{})
	TupleIteratorType = newTypeObject("tuple_iterator", []*TypeObject{ObjectType}, TypeSlots{})
	DictIteratorType = newTypeObject("dict_keyiterator", []*TypeObject{ObjectType}, TypeSlots{})
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
