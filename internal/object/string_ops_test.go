package object

import (
	"testing"

	"github.com/alvinjaison/pygo/internal/bytecode"
)

// ============================================================
// String concatenation (+) tests
// ============================================================

func TestStringOps_Add(t *testing.T) {
	result, err := BinaryOp(StrFrom("hello"), StrFrom(" world"), "+")
	if err != nil {
		t.Fatal(err)
	}
	s, ok := result.(*StrObject)
	if !ok {
		t.Fatalf("expected StrObject, got %T", result)
	}
	if s.Value != "hello world" {
		t.Fatalf("expected 'hello world', got %q", s.Value)
	}
}

func TestStringOps_Add_EmptyStrings(t *testing.T) {
	result, err := BinaryOp(StrFrom(""), StrFrom("abc"), "+")
	if err != nil {
		t.Fatal(err)
	}
	s := result.(*StrObject)
	if s.Value != "abc" {
		t.Fatalf("expected 'abc', got %q", s.Value)
	}

	result, err = BinaryOp(StrFrom("abc"), StrFrom(""), "+")
	if err != nil {
		t.Fatal(err)
	}
	s = result.(*StrObject)
	if s.Value != "abc" {
		t.Fatalf("expected 'abc', got %q", s.Value)
	}
}

func TestStringOps_Add_Unicode(t *testing.T) {
	result, err := BinaryOp(StrFrom("hello "), StrFrom("世界"), "+")
	if err != nil {
		t.Fatal(err)
	}
	s := result.(*StrObject)
	if s.Value != "hello 世界" {
		t.Fatalf("expected 'hello 世界', got %q", s.Value)
	}
}

func TestStringOps_Add_TypeError(t *testing.T) {
	_, err := BinaryOp(StrFrom("hello"), IntFrom(5), "+")
	if err == nil {
		t.Fatal("expected TypeError for str + int")
	}
}

// ============================================================
// String repetition (*) tests
// ============================================================

func TestStringOps_Mul(t *testing.T) {
	result, err := BinaryOp(StrFrom("abc"), IntFrom(3), "*")
	if err != nil {
		t.Fatal(err)
	}
	s := result.(*StrObject)
	if s.Value != "abcabcabc" {
		t.Fatalf("expected 'abcabcabc', got %q", s.Value)
	}
}

func TestStringOps_Mul_ZeroYieldsEmpty(t *testing.T) {
	result, err := BinaryOp(StrFrom("abc"), IntFrom(0), "*")
	if err != nil {
		t.Fatal(err)
	}
	s := result.(*StrObject)
	if s.Value != "" {
		t.Fatalf("expected '', got %q", s.Value)
	}
}

func TestStringOps_Mul_NegativeYieldsEmpty(t *testing.T) {
	result, err := BinaryOp(StrFrom("abc"), IntFrom(-5), "*")
	if err != nil {
		t.Fatal(err)
	}
	s := result.(*StrObject)
	if s.Value != "" {
		t.Fatalf("expected '', got %q", s.Value)
	}
}

func TestStringOps_Mul_One(t *testing.T) {
	result, err := BinaryOp(StrFrom("abc"), IntFrom(1), "*")
	if err != nil {
		t.Fatal(err)
	}
	s := result.(*StrObject)
	if s.Value != "abc" {
		t.Fatalf("expected 'abc', got %q", s.Value)
	}
}

func TestStringOps_RMul(t *testing.T) {
	// 3 * "abc" should also work via reflected mul
	result, err := BinaryOp(IntFrom(3), StrFrom("abc"), "*")
	if err != nil {
		t.Fatal(err)
	}
	s := result.(*StrObject)
	if s.Value != "abcabcabc" {
		t.Fatalf("expected 'abcabcabc', got %q", s.Value)
	}
}

func TestStringOps_RMul_Zero(t *testing.T) {
	result, err := BinaryOp(IntFrom(0), StrFrom("abc"), "*")
	if err != nil {
		t.Fatal(err)
	}
	s := result.(*StrObject)
	if s.Value != "" {
		t.Fatalf("expected '', got %q", s.Value)
	}
}

// ============================================================
// String indexing tests
// ============================================================

func TestStringOps_GetItem_Basic(t *testing.T) {
	s := StrFrom("hello")
	result, err := s.GetItem(IntFrom(0))
	if err != nil {
		t.Fatal(err)
	}
	r := result.(*StrObject)
	if r.Value != "h" {
		t.Fatalf("expected 'h', got %q", r.Value)
	}
}

func TestStringOps_GetItem_LastChar(t *testing.T) {
	s := StrFrom("hello")
	result, err := s.GetItem(IntFrom(4))
	if err != nil {
		t.Fatal(err)
	}
	r := result.(*StrObject)
	if r.Value != "o" {
		t.Fatalf("expected 'o', got %q", r.Value)
	}
}

func TestStringOps_GetItem_NegativeIndex(t *testing.T) {
	s := StrFrom("hello")
	result, err := s.GetItem(IntFrom(-1))
	if err != nil {
		t.Fatal(err)
	}
	r := result.(*StrObject)
	if r.Value != "o" {
		t.Fatalf("expected 'o', got %q", r.Value)
	}
}

func TestStringOps_GetItem_NegativeIndex2(t *testing.T) {
	s := StrFrom("hello")
	result, err := s.GetItem(IntFrom(-5))
	if err != nil {
		t.Fatal(err)
	}
	r := result.(*StrObject)
	if r.Value != "h" {
		t.Fatalf("expected 'h', got %q", r.Value)
	}
}

func TestStringOps_GetItem_Unicode(t *testing.T) {
	// "€" is 3 bytes in UTF-8, but 1 code point
	s := StrFrom("a€b")
	result, err := s.GetItem(IntFrom(1))
	if err != nil {
		t.Fatal(err)
	}
	r := result.(*StrObject)
	if r.Value != "€" {
		t.Fatalf("expected '€', got %q", r.Value)
	}
}

func TestStringOps_GetItem_IndexError(t *testing.T) {
	s := StrFrom("hello")
	_, err := s.GetItem(IntFrom(5))
	if err == nil {
		t.Fatal("expected IndexError for out-of-range index")
	}
}

func TestStringOps_GetItem_NegativeIndexError(t *testing.T) {
	s := StrFrom("hello")
	_, err := s.GetItem(IntFrom(-6))
	if err == nil {
		t.Fatal("expected IndexError for out-of-range negative index")
	}
}

// ============================================================
// String comparison tests
// ============================================================

func TestStringOps_Compare_Eq(t *testing.T) {
	result, err := CompareObjects(StrFrom("abc"), StrFrom("abc"), bytecode.CmpEq)
	if err != nil {
		t.Fatal(err)
	}
	assertBoolResult(t, result, true)
}

func TestStringOps_Compare_NotEq(t *testing.T) {
	result, err := CompareObjects(StrFrom("abc"), StrFrom("abd"), bytecode.CmpNotEq)
	if err != nil {
		t.Fatal(err)
	}
	assertBoolResult(t, result, true)
}

func TestStringOps_Compare_Lt(t *testing.T) {
	result, err := CompareObjects(StrFrom("abc"), StrFrom("abd"), bytecode.CmpLt)
	if err != nil {
		t.Fatal(err)
	}
	assertBoolResult(t, result, true)
}

func TestStringOps_Compare_GtE(t *testing.T) {
	result, err := CompareObjects(StrFrom("b"), StrFrom("a"), bytecode.CmpGtE)
	if err != nil {
		t.Fatal(err)
	}
	assertBoolResult(t, result, true)
}

func TestStringOps_Compare_UnicodeOrdering(t *testing.T) {
	// "b" > "a" in Unicode
	result, err := CompareObjects(StrFrom("b"), StrFrom("a"), bytecode.CmpGt)
	if err != nil {
		t.Fatal(err)
	}
	assertBoolResult(t, result, true)
}

// ============================================================
// String Contains protocol tests
// ============================================================

func TestStringOps_Contains(t *testing.T) {
	s := StrFrom("hello world")
	found, err := s.Contains(StrFrom("world"))
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected 'world' to be found in 'hello world'")
	}
}

func TestStringOps_Contains_NotFound(t *testing.T) {
	s := StrFrom("hello world")
	found, err := s.Contains(StrFrom("xyz"))
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("expected 'xyz' to not be found in 'hello world'")
	}
}

func TestStringOps_Contains_EmptySubstring(t *testing.T) {
	s := StrFrom("hello")
	found, err := s.Contains(StrFrom(""))
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected empty string to be found in any string")
	}
}

func TestStringOps_Contains_TypeError(t *testing.T) {
	s := StrFrom("hello")
	_, err := s.Contains(IntFrom(5))
	if err == nil {
		t.Fatal("expected TypeError for non-string 'in' operand")
	}
}

func TestStringOps_Contains_ViaCompareObjects(t *testing.T) {
	// "lo" in "hello" via CompareObjects dispatch
	result, err := CompareObjects(StrFrom("lo"), StrFrom("hello"), bytecode.CmpIn)
	if err != nil {
		t.Fatal(err)
	}
	assertBoolResult(t, result, true)
}

func TestStringOps_NotContains_ViaCompareObjects(t *testing.T) {
	// "xyz" not in "hello" via CompareObjects dispatch
	result, err := CompareObjects(StrFrom("xyz"), StrFrom("hello"), bytecode.CmpNotIn)
	if err != nil {
		t.Fatal(err)
	}
	assertBoolResult(t, result, true)
}

// ============================================================
// String Len tests
// ============================================================

func TestStringOps_Len_ASCII(t *testing.T) {
	s := StrFrom("hello")
	if s.Len() != 5 {
		t.Fatalf("expected len 5, got %d", s.Len())
	}
}

func TestStringOps_Len_Unicode(t *testing.T) {
	// "€" is 1 code point (3 bytes UTF-8)
	s := StrFrom("€")
	if s.Len() != 1 {
		t.Fatalf("expected len 1, got %d", s.Len())
	}
}

func TestStringOps_Len_MultiCodePoint(t *testing.T) {
	// "hello世界" = 7 code points
	s := StrFrom("hello世界")
	if s.Len() != 7 {
		t.Fatalf("expected len 7, got %d", s.Len())
	}
}

func TestStringOps_Len_Empty(t *testing.T) {
	s := StrFrom("")
	if s.Len() != 0 {
		t.Fatalf("expected len 0, got %d", s.Len())
	}
}

// ============================================================
// String Hash tests
// ============================================================

func TestStringOps_Hash_Consistent(t *testing.T) {
	s1 := StrFrom("hello")
	s2 := StrFrom("hello")
	h1, err := s1.Hash()
	if err != nil {
		t.Fatal(err)
	}
	h2, err := s2.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("expected same hash for same string, got %d and %d", h1, h2)
	}
}

func TestStringOps_Hash_DifferentStrings(t *testing.T) {
	h1, _ := StrFrom("hello").Hash()
	h2, _ := StrFrom("world").Hash()
	if h1 == h2 {
		t.Fatal("expected different hashes for different strings (unlikely collision)")
	}
}

// ============================================================
// String immutability tests
// ============================================================

func TestStringOps_SetItem_Immutable(t *testing.T) {
	s := StrFrom("hello")
	err := s.SetItem(IntFrom(0), StrFrom("H"))
	if err == nil {
		t.Fatal("expected TypeError for string item assignment")
	}
}

func TestStringOps_DelItem_Immutable(t *testing.T) {
	s := StrFrom("hello")
	err := s.DelItem(IntFrom(0))
	if err == nil {
		t.Fatal("expected TypeError for string item deletion")
	}
}
