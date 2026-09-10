package object

import (
	"fmt"
	"math/big"
	"testing"

	"pgregory.net/rapid"
)

// ============================================================
// Property 14: Hash Consistency
// ============================================================
// **Validates: Requirements 6.7**
//
// Objects that compare equal MUST produce the same hash value.
// Mutable collections (list, dict) MUST raise TypeError when Hash() is attempted.

func TestProperty14_HashConsistency_EqualInts(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.Int64Range(-10000, 10000).Draw(t, "n")
		a := IntFrom(n)
		b := IntFrom(n)

		hashA, errA := a.Hash()
		hashB, errB := b.Hash()
		if errA != nil {
			t.Fatalf("unexpected error hashing int a: %v", errA)
		}
		if errB != nil {
			t.Fatalf("unexpected error hashing int b: %v", errB)
		}
		if hashA != hashB {
			t.Fatalf("hash(%d) = %d != hash(%d) = %d", n, hashA, n, hashB)
		}
	})
}

func TestProperty14_HashConsistency_EqualFloats(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		f := rapid.Float64Range(-10000, 10000).Draw(t, "f")
		a := FloatFrom(f)
		b := FloatFrom(f)

		hashA, errA := a.Hash()
		hashB, errB := b.Hash()
		if errA != nil {
			t.Fatalf("unexpected error hashing float a: %v", errA)
		}
		if errB != nil {
			t.Fatalf("unexpected error hashing float b: %v", errB)
		}
		if hashA != hashB {
			t.Fatalf("hash(%v) = %d != hash(%v) = %d", f, hashA, f, hashB)
		}
	})
}

func TestProperty14_HashConsistency_IntFloatCrossType(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate integers that fit exactly in float64
		n := rapid.Int64Range(-1000, 1000).Draw(t, "n")
		intObj := IntFrom(n)
		floatObj := FloatFrom(float64(n))

		// Verify they compare equal
		result, err := intObj.RichCompare(floatObj, OpEq)
		if err != nil {
			t.Fatalf("comparison error: %v", err)
		}
		eq, err := IsTruthy(result)
		if err != nil {
			t.Fatalf("truthiness error: %v", err)
		}
		if !eq {
			t.Skipf("int(%d) != float(%v), skipping hash check", n, float64(n))
		}

		// If equal, hashes must match
		hashInt, errI := intObj.Hash()
		hashFloat, errF := floatObj.Hash()
		if errI != nil {
			t.Fatalf("unexpected error hashing int: %v", errI)
		}
		if errF != nil {
			t.Fatalf("unexpected error hashing float: %v", errF)
		}
		if hashInt != hashFloat {
			t.Fatalf("hash(int(%d)) = %d != hash(float(%v)) = %d", n, hashInt, float64(n), hashFloat)
		}
	})
}

func TestProperty14_HashConsistency_EqualStrings(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		a := StrFrom(s)
		b := StrFrom(s)

		hashA, errA := a.Hash()
		hashB, errB := b.Hash()
		if errA != nil {
			t.Fatalf("unexpected error hashing str a: %v", errA)
		}
		if errB != nil {
			t.Fatalf("unexpected error hashing str b: %v", errB)
		}
		if hashA != hashB {
			t.Fatalf("hash(%q) = %d != hash(%q) = %d", s, hashA, s, hashB)
		}
	})
}

func TestProperty14_HashConsistency_EqualTuples(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a tuple of ints (all hashable)
		length := rapid.IntRange(0, 10).Draw(t, "length")
		items := make([]Object, length)
		for i := 0; i < length; i++ {
			items[i] = IntFrom(rapid.Int64Range(-100, 100).Draw(t, fmt.Sprintf("item_%d", i)))
		}

		a := NewTuple(items)
		b := NewTuple(items)

		hashA, errA := a.Hash()
		hashB, errB := b.Hash()
		if errA != nil {
			t.Fatalf("unexpected error hashing tuple a: %v", errA)
		}
		if errB != nil {
			t.Fatalf("unexpected error hashing tuple b: %v", errB)
		}
		if hashA != hashB {
			t.Fatalf("hash(%s) = %d != hash(%s) = %d", a.Repr(), hashA, b.Repr(), hashB)
		}
	})
}

func TestProperty14_HashConsistency_BoolIntEquality(t *testing.T) {
	// True == 1 and False == 0, so hash(True) must == hash(1), hash(False) must == hash(0)
	hashTrue, _ := PyTrue.Hash()
	hashOne, _ := IntFrom(1).Hash()
	if hashTrue != hashOne {
		t.Fatalf("hash(True) = %d != hash(1) = %d", hashTrue, hashOne)
	}

	hashFalse, _ := PyFalse.Hash()
	hashZero, _ := IntFrom(0).Hash()
	if hashFalse != hashZero {
		t.Fatalf("hash(False) = %d != hash(0) = %d", hashFalse, hashZero)
	}
}

func TestProperty14_HashConsistency_MutableCollectionsUnhashable(t *testing.T) {
	// ListObject should not implement Hashable
	list := NewList([]Object{IntFrom(1), IntFrom(2)})
	if _, ok := interface{}(list).(Hashable); ok {
		t.Fatal("ListObject should not implement Hashable interface")
	}

	// DictObject should not implement Hashable
	dict := NewDict()
	_ = dict.SetItem(StrFrom("a"), IntFrom(1))
	if _, ok := interface{}(dict).(Hashable); ok {
		t.Fatal("DictObject should not implement Hashable interface")
	}

	// Verify that hashObject helper returns TypeError for list
	_, err := hashObject(list)
	if err == nil {
		t.Fatal("expected TypeError when hashing a list")
	}

	// Verify that hashObject helper returns TypeError for dict
	_, err = hashObject(dict)
	if err == nil {
		t.Fatal("expected TypeError when hashing a dict")
	}
}

// ============================================================
// Property 15: Dict Insertion Order Preservation
// ============================================================
// **Validates: Requirements 6.3**
//
// Inserting keys into a DictObject and then iterating must yield
// keys in the exact order they were first inserted. Overwriting
// a key's value must NOT change its position in the iteration order.

func TestProperty15_DictInsertionOrderPreservation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random sequence of unique string keys
		numKeys := rapid.IntRange(1, 50).Draw(t, "numKeys")
		keys := make([]string, numKeys)
		seen := make(map[string]bool)
		for i := 0; i < numKeys; i++ {
			for {
				k := rapid.String().Draw(t, fmt.Sprintf("key_%d", i))
				if !seen[k] {
					keys[i] = k
					seen[k] = true
					break
				}
			}
		}

		// Insert them into a DictObject
		d := NewDict()
		for i, k := range keys {
			err := d.SetItem(StrFrom(k), IntFrom(int64(i)))
			if err != nil {
				t.Fatalf("SetItem error for key %q: %v", k, err)
			}
		}

		// Verify iteration order matches insertion order via Keys()
		gotKeys := d.Keys()
		if len(gotKeys) != numKeys {
			t.Fatalf("expected %d keys, got %d", numKeys, len(gotKeys))
		}
		for i, got := range gotKeys {
			gotStr, ok := got.(*StrObject)
			if !ok {
				t.Fatalf("key at index %d is not a string", i)
			}
			if gotStr.Value != keys[i] {
				t.Fatalf("key order mismatch at index %d: expected %q, got %q", i, keys[i], gotStr.Value)
			}
		}

		// Verify iteration order matches insertion order via Iter()
		iter, err := d.Iter()
		if err != nil {
			t.Fatalf("Iter() error: %v", err)
		}
		for i := 0; i < numKeys; i++ {
			v, err := iter.Next()
			if err != nil {
				t.Fatalf("unexpected error at iteration %d: %v", i, err)
			}
			vStr, ok := v.(*StrObject)
			if !ok {
				t.Fatalf("iter key at index %d is not a string", i)
			}
			if vStr.Value != keys[i] {
				t.Fatalf("iter order mismatch at index %d: expected %q, got %q", i, keys[i], vStr.Value)
			}
		}
		// Iterator should be exhausted
		_, err = iter.Next()
		if !IsStopIteration(err) {
			t.Fatal("expected StopIteration after iterating all keys")
		}
	})
}

func TestProperty15_DictOverwritePreservesOrder(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate unique keys
		numKeys := rapid.IntRange(2, 30).Draw(t, "numKeys")
		keys := make([]string, numKeys)
		seen := make(map[string]bool)
		for i := 0; i < numKeys; i++ {
			for {
				k := rapid.String().Draw(t, fmt.Sprintf("key_%d", i))
				if !seen[k] {
					keys[i] = k
					seen[k] = true
					break
				}
			}
		}

		// Insert all keys
		d := NewDict()
		for i, k := range keys {
			err := d.SetItem(StrFrom(k), IntFrom(int64(i)))
			if err != nil {
				t.Fatalf("SetItem error: %v", err)
			}
		}

		// Overwrite a random key with a new value
		overwriteIdx := rapid.IntRange(0, numKeys-1).Draw(t, "overwriteIdx")
		err := d.SetItem(StrFrom(keys[overwriteIdx]), IntFrom(999))
		if err != nil {
			t.Fatalf("overwrite SetItem error: %v", err)
		}

		// Verify the dict still has the same number of keys
		if d.Len() != numKeys {
			t.Fatalf("expected len %d after overwrite, got %d", numKeys, d.Len())
		}

		// Verify iteration order is unchanged (overwrite does not move keys)
		gotKeys := d.Keys()
		for i, got := range gotKeys {
			gotStr, ok := got.(*StrObject)
			if !ok {
				t.Fatalf("key at index %d is not a string", i)
			}
			if gotStr.Value != keys[i] {
				t.Fatalf("overwrite changed order at index %d: expected %q, got %q", i, keys[i], gotStr.Value)
			}
		}

		// Verify the overwritten key has the new value
		val, err := d.GetItem(StrFrom(keys[overwriteIdx]))
		if err != nil {
			t.Fatalf("GetItem error after overwrite: %v", err)
		}
		intVal, ok := val.(*IntObject)
		if !ok {
			t.Fatal("expected IntObject value")
		}
		v, _ := intVal.Int64()
		if v != 999 {
			t.Fatalf("expected value 999 after overwrite, got %d", v)
		}
	})
}

// ============================================================
// Property 2: Integer Arbitrary Precision
// ============================================================
// **Validates: Requirements 16.7, 6.8**
//
// Generate large integers (beyond int64 range), perform arithmetic
// (add, sub, mul), and verify the result matches math/big computation.

func TestProperty2_IntegerArbitraryPrecision_Add(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate big integers beyond int64 range by combining large parts
		aHi := rapid.Int64().Draw(t, "aHi")
		aLo := rapid.Int64().Draw(t, "aLo")
		bHi := rapid.Int64().Draw(t, "bHi")
		bLo := rapid.Int64().Draw(t, "bLo")

		// Construct large numbers: aHi * 2^64 + aLo
		shift := new(big.Int).Lsh(big.NewInt(1), 64)
		aBig := new(big.Int).Mul(big.NewInt(aHi), shift)
		aBig.Add(aBig, big.NewInt(aLo))

		bBig := new(big.Int).Mul(big.NewInt(bHi), shift)
		bBig.Add(bBig, big.NewInt(bLo))

		// Create PyGo IntObjects
		aObj := IntFromBig(aBig)
		bObj := IntFromBig(bBig)

		// Perform addition via operator dispatch
		result, err := BinaryOp(aObj, bObj, "+")
		if err != nil {
			t.Fatalf("BinaryOp(+) error: %v", err)
		}

		resultInt, ok := result.(*IntObject)
		if !ok {
			t.Fatalf("expected IntObject result, got %T", result)
		}

		// Verify against math/big
		expected := new(big.Int).Add(aBig, bBig)
		if resultInt.Value.Cmp(expected) != 0 {
			t.Fatalf("Add: got %s, want %s", resultInt.Value.String(), expected.String())
		}
	})
}

func TestProperty2_IntegerArbitraryPrecision_Sub(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		aHi := rapid.Int64().Draw(t, "aHi")
		aLo := rapid.Int64().Draw(t, "aLo")
		bHi := rapid.Int64().Draw(t, "bHi")
		bLo := rapid.Int64().Draw(t, "bLo")

		shift := new(big.Int).Lsh(big.NewInt(1), 64)
		aBig := new(big.Int).Mul(big.NewInt(aHi), shift)
		aBig.Add(aBig, big.NewInt(aLo))

		bBig := new(big.Int).Mul(big.NewInt(bHi), shift)
		bBig.Add(bBig, big.NewInt(bLo))

		aObj := IntFromBig(aBig)
		bObj := IntFromBig(bBig)

		result, err := BinaryOp(aObj, bObj, "-")
		if err != nil {
			t.Fatalf("BinaryOp(-) error: %v", err)
		}

		resultInt, ok := result.(*IntObject)
		if !ok {
			t.Fatalf("expected IntObject result, got %T", result)
		}

		expected := new(big.Int).Sub(aBig, bBig)
		if resultInt.Value.Cmp(expected) != 0 {
			t.Fatalf("Sub: got %s, want %s", resultInt.Value.String(), expected.String())
		}
	})
}

func TestProperty2_IntegerArbitraryPrecision_Mul(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		aHi := rapid.Int64().Draw(t, "aHi")
		aLo := rapid.Int64().Draw(t, "aLo")
		bHi := rapid.Int64().Draw(t, "bHi")
		bLo := rapid.Int64().Draw(t, "bLo")

		shift := new(big.Int).Lsh(big.NewInt(1), 64)
		aBig := new(big.Int).Mul(big.NewInt(aHi), shift)
		aBig.Add(aBig, big.NewInt(aLo))

		bBig := new(big.Int).Mul(big.NewInt(bHi), shift)
		bBig.Add(bBig, big.NewInt(bLo))

		aObj := IntFromBig(aBig)
		bObj := IntFromBig(bBig)

		result, err := BinaryOp(aObj, bObj, "*")
		if err != nil {
			t.Fatalf("BinaryOp(*) error: %v", err)
		}

		resultInt, ok := result.(*IntObject)
		if !ok {
			t.Fatalf("expected IntObject result, got %T", result)
		}

		expected := new(big.Int).Mul(aBig, bBig)
		if resultInt.Value.Cmp(expected) != 0 {
			t.Fatalf("Mul: got %s, want %s", resultInt.Value.String(), expected.String())
		}
	})
}

// ============================================================
// Property 4: String Code-Point Semantics
// ============================================================
// **Validates: Requirements 17.1, 6.8**
//
// Generate Unicode strings (including multi-byte chars like emoji, CJK),
// verify Len() returns rune count (not byte count), Index(i) returns the
// correct i-th code point as a string, and iterating via Iter() yields
// individual code points matching []rune(s).

func TestProperty4_StringCodePointSemantics_Len(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		obj := StrFrom(s)

		gotLen := obj.Len()
		expectedLen := len([]rune(s))

		if gotLen != expectedLen {
			t.Fatalf("Len(%q): got %d (rune count expected %d, byte count %d)",
				s, gotLen, expectedLen, len(s))
		}
	})
}

func TestProperty4_StringCodePointSemantics_Index(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate non-empty strings to allow valid indexing
		s := rapid.StringOf(rapid.Rune()).Filter(func(s string) bool {
			return len(s) > 0
		}).Draw(t, "s")

		obj := StrFrom(s)
		runes := []rune(s)

		// Pick a random valid index
		idx := rapid.IntRange(0, len(runes)-1).Draw(t, "idx")

		result, err := obj.Index(idx)
		if err != nil {
			t.Fatalf("Index(%d) error: %v", idx, err)
		}

		expected := string(runes[idx])
		if result.Value != expected {
			t.Fatalf("Index(%d) of %q: got %q, want %q", idx, s, result.Value, expected)
		}
	})
}

func TestProperty4_StringCodePointSemantics_Iter(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		obj := StrFrom(s)
		runes := []rune(s)

		iter, err := obj.Iter()
		if err != nil {
			t.Fatalf("Iter() error: %v", err)
		}

		for i, expectedRune := range runes {
			val, err := iter.Next()
			if err != nil {
				t.Fatalf("Next() at index %d error: %v", i, err)
			}
			strVal, ok := val.(*StrObject)
			if !ok {
				t.Fatalf("Next() at index %d returned %T, expected *StrObject", i, val)
			}
			expectedStr := string(expectedRune)
			if strVal.Value != expectedStr {
				t.Fatalf("Iter at index %d of %q: got %q, want %q", i, s, strVal.Value, expectedStr)
			}
		}

		// Iterator should be exhausted
		_, err = iter.Next()
		if !IsStopIteration(err) {
			t.Fatalf("expected StopIteration after iterating all runes, got: %v", err)
		}
	})
}

// ============================================================
// Property 5: Truthiness Consistency
// ============================================================
// **Validates: Requirements 16.6**
//
// All falsy values (False, None, IntFrom(0), FloatFrom(0.0), StrFrom(""),
// NewList([]Object{}), NewTuple([]Object{}), NewDict()) must return false
// from IsTruthy. Random non-zero ints, non-empty strings, and non-empty
// collections must return true.

func TestProperty5_TruthinessConsistency_FalsyValues(t *testing.T) {
	falsyValues := []Object{
		PyFalse,
		None,
		IntFrom(0),
		FloatFrom(0.0),
		StrFrom(""),
		NewList([]Object{}),
		NewTuple([]Object{}),
		NewDict(),
	}

	for _, obj := range falsyValues {
		truthy, err := IsTruthy(obj)
		if err != nil {
			t.Fatalf("IsTruthy(%s) error: %v", obj.Repr(), err)
		}
		if truthy {
			t.Fatalf("IsTruthy(%s) = true, expected false", obj.Repr())
		}
	}
}

func TestProperty5_TruthinessConsistency_TruthyInts(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate non-zero integers
		n := rapid.Int64().Filter(func(n int64) bool { return n != 0 }).Draw(t, "n")
		obj := IntFrom(n)

		truthy, err := IsTruthy(obj)
		if err != nil {
			t.Fatalf("IsTruthy(int(%d)) error: %v", n, err)
		}
		if !truthy {
			t.Fatalf("IsTruthy(int(%d)) = false, expected true", n)
		}
	})
}

func TestProperty5_TruthinessConsistency_TruthyStrings(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate non-empty strings
		s := rapid.StringOf(rapid.Rune()).Filter(func(s string) bool {
			return len(s) > 0
		}).Draw(t, "s")
		obj := StrFrom(s)

		truthy, err := IsTruthy(obj)
		if err != nil {
			t.Fatalf("IsTruthy(str(%q)) error: %v", s, err)
		}
		if !truthy {
			t.Fatalf("IsTruthy(str(%q)) = false, expected true", s)
		}
	})
}

func TestProperty5_TruthinessConsistency_TruthyCollections(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate non-empty lists
		listLen := rapid.IntRange(1, 10).Draw(t, "listLen")
		items := make([]Object, listLen)
		for i := range items {
			items[i] = IntFrom(rapid.Int64Range(-100, 100).Draw(t, fmt.Sprintf("item_%d", i)))
		}

		list := NewList(items)
		truthy, err := IsTruthy(list)
		if err != nil {
			t.Fatalf("IsTruthy(list) error: %v", err)
		}
		if !truthy {
			t.Fatalf("IsTruthy(non-empty list) = false, expected true")
		}

		// Non-empty tuple
		tuple := NewTuple(items)
		truthy, err = IsTruthy(tuple)
		if err != nil {
			t.Fatalf("IsTruthy(tuple) error: %v", err)
		}
		if !truthy {
			t.Fatalf("IsTruthy(non-empty tuple) = false, expected true")
		}

		// Non-empty dict
		dict := NewDict()
		_ = dict.SetItem(StrFrom("key"), IntFrom(1))
		truthy, err = IsTruthy(dict)
		if err != nil {
			t.Fatalf("IsTruthy(dict) error: %v", err)
		}
		if !truthy {
			t.Fatalf("IsTruthy(non-empty dict) = false, expected true")
		}
	})
}

func TestProperty5_TruthinessConsistency_TruthyFloats(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate non-zero floats
		f := rapid.Float64().Filter(func(f float64) bool { return f != 0.0 }).Draw(t, "f")
		obj := FloatFrom(f)

		truthy, err := IsTruthy(obj)
		if err != nil {
			t.Fatalf("IsTruthy(float(%v)) error: %v", f, err)
		}
		if !truthy {
			t.Fatalf("IsTruthy(float(%v)) = false, expected true", f)
		}
	})
}

func TestProperty5_TruthinessConsistency_TrueIsTruthy(t *testing.T) {
	truthy, err := IsTruthy(PyTrue)
	if err != nil {
		t.Fatalf("IsTruthy(True) error: %v", err)
	}
	if !truthy {
		t.Fatal("IsTruthy(True) = false, expected true")
	}
}
