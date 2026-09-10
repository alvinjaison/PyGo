package object

import (
	"testing"
)

// ============================================================
// ListObject Tests
// ============================================================

func TestListBasicOps(t *testing.T) {
	l := NewList([]Object{IntFrom(1), IntFrom(2), IntFrom(3)})

	if l.Len() != 3 {
		t.Fatalf("expected len 3, got %d", l.Len())
	}

	// GetItem
	v, err := l.GetItem(IntFrom(0))
	if err != nil {
		t.Fatal(err)
	}
	if v.(*IntObject).Value.Int64() != 1 {
		t.Fatalf("expected 1, got %s", v.String())
	}

	// Negative indexing
	v, err = l.GetItem(IntFrom(-1))
	if err != nil {
		t.Fatal(err)
	}
	if v.(*IntObject).Value.Int64() != 3 {
		t.Fatalf("expected 3, got %s", v.String())
	}

	// SetItem
	err = l.SetItem(IntFrom(1), IntFrom(99))
	if err != nil {
		t.Fatal(err)
	}
	v, _ = l.GetItem(IntFrom(1))
	if v.(*IntObject).Value.Int64() != 99 {
		t.Fatalf("expected 99, got %s", v.String())
	}

	// DelItem
	err = l.DelItem(IntFrom(1))
	if err != nil {
		t.Fatal(err)
	}
	if l.Len() != 2 {
		t.Fatalf("expected len 2 after del, got %d", l.Len())
	}

	// Append
	l.Append(IntFrom(42))
	if l.Len() != 3 {
		t.Fatalf("expected len 3 after append, got %d", l.Len())
	}
}

func TestListContains(t *testing.T) {
	l := NewList([]Object{IntFrom(10), IntFrom(20), IntFrom(30)})

	found, err := l.Contains(IntFrom(20))
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected Contains(20) to be true")
	}

	found, err = l.Contains(IntFrom(99))
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("expected Contains(99) to be false")
	}
}

func TestListIter(t *testing.T) {
	l := NewList([]Object{IntFrom(1), IntFrom(2), IntFrom(3)})
	iter, err := l.Iter()
	if err != nil {
		t.Fatal(err)
	}

	var results []int64
	for {
		v, err := iter.Next()
		if err != nil {
			if IsStopIteration(err) {
				break
			}
			t.Fatal(err)
		}
		n, _ := v.(*IntObject).Int64()
		results = append(results, n)
	}

	if len(results) != 3 || results[0] != 1 || results[1] != 2 || results[2] != 3 {
		t.Fatalf("unexpected iteration results: %v", results)
	}
}

func TestListIndexOutOfRange(t *testing.T) {
	l := NewList([]Object{IntFrom(1)})
	_, err := l.GetItem(IntFrom(5))
	if err == nil {
		t.Fatal("expected IndexError")
	}
}

func TestListIsTrue(t *testing.T) {
	empty := NewList([]Object{})
	b, _ := empty.IsTrue()
	if b {
		t.Fatal("empty list should be falsy")
	}

	nonEmpty := NewList([]Object{IntFrom(1)})
	b, _ = nonEmpty.IsTrue()
	if !b {
		t.Fatal("non-empty list should be truthy")
	}
}

// ============================================================
// TupleObject Tests
// ============================================================

func TestTupleImmutability(t *testing.T) {
	tup := NewTuple([]Object{IntFrom(1), IntFrom(2)})

	err := tup.SetItem(IntFrom(0), IntFrom(99))
	if err == nil {
		t.Fatal("expected TypeError on tuple SetItem")
	}

	err = tup.DelItem(IntFrom(0))
	if err == nil {
		t.Fatal("expected TypeError on tuple DelItem")
	}
}

func TestTupleContains(t *testing.T) {
	tup := NewTuple([]Object{IntFrom(1), IntFrom(2), IntFrom(3)})

	found, err := tup.Contains(IntFrom(2))
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected Contains(2) to be true")
	}

	found, err = tup.Contains(IntFrom(99))
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("expected Contains(99) to be false")
	}
}

func TestTupleHash(t *testing.T) {
	tup1 := NewTuple([]Object{IntFrom(1), IntFrom(2)})
	tup2 := NewTuple([]Object{IntFrom(1), IntFrom(2)})

	h1, err := tup1.Hash()
	if err != nil {
		t.Fatal(err)
	}
	h2, err := tup2.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("equal tuples should have equal hashes: %d != %d", h1, h2)
	}
}

func TestTupleHashUnhashableElement(t *testing.T) {
	// A tuple containing a list should not be hashable.
	tup := NewTuple([]Object{NewList([]Object{IntFrom(1)})})
	_, err := tup.Hash()
	if err == nil {
		t.Fatal("expected TypeError for tuple with unhashable element")
	}
}

func TestTupleIter(t *testing.T) {
	tup := NewTuple([]Object{IntFrom(10), IntFrom(20)})
	iter, err := tup.Iter()
	if err != nil {
		t.Fatal(err)
	}

	v, err := iter.Next()
	if err != nil {
		t.Fatal(err)
	}
	if v.(*IntObject).Value.Int64() != 10 {
		t.Fatalf("expected 10, got %s", v.String())
	}

	v, err = iter.Next()
	if err != nil {
		t.Fatal(err)
	}
	if v.(*IntObject).Value.Int64() != 20 {
		t.Fatalf("expected 20, got %s", v.String())
	}

	_, err = iter.Next()
	if !IsStopIteration(err) {
		t.Fatal("expected StopIteration")
	}
}

func TestTupleReprSingleElement(t *testing.T) {
	tup := NewTuple([]Object{IntFrom(1)})
	if tup.Repr() != "(1,)" {
		t.Fatalf("expected (1,), got %s", tup.Repr())
	}
}

func TestEmptyTupleSingleton(t *testing.T) {
	t1 := NewTuple([]Object{})
	t2 := NewTuple([]Object{})
	if t1 != t2 {
		t.Fatal("empty tuples should be the same singleton")
	}
}

// ============================================================
// DictObject Tests
// ============================================================

func TestDictSetAndGet(t *testing.T) {
	d := NewDict()
	err := d.SetItem(StrFrom("hello"), IntFrom(42))
	if err != nil {
		t.Fatal(err)
	}

	v, err := d.GetItem(StrFrom("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if v.(*IntObject).Value.Int64() != 42 {
		t.Fatalf("expected 42, got %s", v.String())
	}
}

func TestDictOverwrite(t *testing.T) {
	d := NewDict()
	d.SetItem(StrFrom("key"), IntFrom(1))
	d.SetItem(StrFrom("key"), IntFrom(2))

	v, _ := d.GetItem(StrFrom("key"))
	if v.(*IntObject).Value.Int64() != 2 {
		t.Fatalf("expected 2 after overwrite, got %s", v.String())
	}

	if d.Len() != 1 {
		t.Fatalf("expected len 1 after overwrite, got %d", d.Len())
	}
}

func TestDictKeyError(t *testing.T) {
	d := NewDict()
	_, err := d.GetItem(StrFrom("missing"))
	if err == nil {
		t.Fatal("expected KeyError")
	}
}

func TestDictDeleteTombstone(t *testing.T) {
	d := NewDict()
	d.SetItem(StrFrom("a"), IntFrom(1))
	d.SetItem(StrFrom("b"), IntFrom(2))
	d.SetItem(StrFrom("c"), IntFrom(3))

	err := d.DelItem(StrFrom("b"))
	if err != nil {
		t.Fatal(err)
	}

	if d.Len() != 2 {
		t.Fatalf("expected len 2 after del, got %d", d.Len())
	}

	// "b" should no longer be found.
	_, err = d.GetItem(StrFrom("b"))
	if err == nil {
		t.Fatal("expected KeyError for deleted key")
	}

	// "a" and "c" should still be accessible.
	v, _ := d.GetItem(StrFrom("a"))
	if v.(*IntObject).Value.Int64() != 1 {
		t.Fatal("expected a=1")
	}
	v, _ = d.GetItem(StrFrom("c"))
	if v.(*IntObject).Value.Int64() != 3 {
		t.Fatal("expected c=3")
	}
}

func TestDictInsertionOrder(t *testing.T) {
	d := NewDict()
	d.SetItem(StrFrom("z"), IntFrom(1))
	d.SetItem(StrFrom("a"), IntFrom(2))
	d.SetItem(StrFrom("m"), IntFrom(3))

	keys := d.Keys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	expected := []string{"z", "a", "m"}
	for i, k := range keys {
		if k.(*StrObject).Value != expected[i] {
			t.Fatalf("expected key %s at index %d, got %s", expected[i], i, k.String())
		}
	}
}

func TestDictInsertionOrderAfterDelete(t *testing.T) {
	d := NewDict()
	d.SetItem(StrFrom("first"), IntFrom(1))
	d.SetItem(StrFrom("second"), IntFrom(2))
	d.SetItem(StrFrom("third"), IntFrom(3))

	// Delete middle entry.
	d.DelItem(StrFrom("second"))

	keys := d.Keys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[0].(*StrObject).Value != "first" {
		t.Fatal("expected 'first' as first key")
	}
	if keys[1].(*StrObject).Value != "third" {
		t.Fatal("expected 'third' as second key")
	}
}

func TestDictContains(t *testing.T) {
	d := NewDict()
	d.SetItem(StrFrom("key"), IntFrom(1))

	found, err := d.Contains(StrFrom("key"))
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected Contains('key') to be true")
	}

	found, err = d.Contains(StrFrom("nope"))
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("expected Contains('nope') to be false")
	}
}

func TestDictIter(t *testing.T) {
	d := NewDict()
	d.SetItem(StrFrom("a"), IntFrom(1))
	d.SetItem(StrFrom("b"), IntFrom(2))
	d.SetItem(StrFrom("c"), IntFrom(3))

	iter, err := d.Iter()
	if err != nil {
		t.Fatal(err)
	}

	var keys []string
	for {
		v, err := iter.Next()
		if err != nil {
			if IsStopIteration(err) {
				break
			}
			t.Fatal(err)
		}
		keys = append(keys, v.(*StrObject).Value)
	}

	expected := []string{"a", "b", "c"}
	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(keys))
	}
	for i, k := range keys {
		if k != expected[i] {
			t.Fatalf("expected key %s at %d, got %s", expected[i], i, k)
		}
	}
}

func TestDictIterSkipsTombstones(t *testing.T) {
	d := NewDict()
	d.SetItem(StrFrom("a"), IntFrom(1))
	d.SetItem(StrFrom("b"), IntFrom(2))
	d.SetItem(StrFrom("c"), IntFrom(3))

	d.DelItem(StrFrom("b"))

	iter, err := d.Iter()
	if err != nil {
		t.Fatal(err)
	}

	var keys []string
	for {
		v, err := iter.Next()
		if err != nil {
			if IsStopIteration(err) {
				break
			}
			t.Fatal(err)
		}
		keys = append(keys, v.(*StrObject).Value)
	}

	expected := []string{"a", "c"}
	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(keys))
	}
	for i, k := range keys {
		if k != expected[i] {
			t.Fatalf("expected key %s at %d, got %s", expected[i], i, k)
		}
	}
}

func TestDictCompaction(t *testing.T) {
	// Insert several items, delete more than half to trigger compaction.
	d := NewDict()
	for i := 0; i < 10; i++ {
		d.SetItem(IntFrom(int64(i)), IntFrom(int64(i*10)))
	}
	// Delete 7 of 10 entries to trigger compaction (70% tombstones > 50%).
	for i := 0; i < 7; i++ {
		err := d.DelItem(IntFrom(int64(i)))
		if err != nil {
			t.Fatal(err)
		}
	}

	if d.Len() != 3 {
		t.Fatalf("expected 3 remaining entries, got %d", d.Len())
	}

	// Verify remaining keys.
	for i := 7; i < 10; i++ {
		v, err := d.GetItem(IntFrom(int64(i)))
		if err != nil {
			t.Fatalf("key %d not found after compaction", i)
		}
		n, _ := v.(*IntObject).Int64()
		if n != int64(i*10) {
			t.Fatalf("expected %d, got %d", i*10, n)
		}
	}
}

func TestDictDeleteNonExistent(t *testing.T) {
	d := NewDict()
	d.SetItem(StrFrom("a"), IntFrom(1))

	err := d.DelItem(StrFrom("missing"))
	if err == nil {
		t.Fatal("expected KeyError for deleting non-existent key")
	}
}

func TestDictUnhashableKey(t *testing.T) {
	d := NewDict()
	// Lists are not hashable.
	err := d.SetItem(NewList([]Object{IntFrom(1)}), IntFrom(1))
	if err == nil {
		t.Fatal("expected TypeError for unhashable key")
	}
}

func TestDictIsTrue(t *testing.T) {
	empty := NewDict()
	b, _ := empty.IsTrue()
	if b {
		t.Fatal("empty dict should be falsy")
	}

	d := NewDict()
	d.SetItem(StrFrom("k"), IntFrom(1))
	b, _ = d.IsTrue()
	if !b {
		t.Fatal("non-empty dict should be truthy")
	}
}

func TestDictReprSkipsTombstones(t *testing.T) {
	d := NewDict()
	d.SetItem(StrFrom("a"), IntFrom(1))
	d.SetItem(StrFrom("b"), IntFrom(2))
	d.SetItem(StrFrom("c"), IntFrom(3))
	d.DelItem(StrFrom("b"))

	repr := d.Repr()
	if repr != "{'a': 1, 'c': 3}" {
		t.Fatalf("unexpected repr: %s", repr)
	}
}
