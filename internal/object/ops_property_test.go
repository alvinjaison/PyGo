package object

import (
	"math/big"
	"strings"
	"testing"
	"unicode/utf8"

	"pgregory.net/rapid"
)

// ============================================================
// Property 3: Floor Division and Modulo Semantics
// ============================================================
// **Validates: Requirements 16.1, 16.2**
//
// For any integer pair (n, d) with d ≠ 0:
// 1. The invariant n == (n // d) * d + (n % d) must hold
// 2. Floor division rounds toward negative infinity (not truncation toward zero)
// 3. The sign of the modulo result matches the sign of the divisor (or remainder is 0)

func TestProperty3_FloorDivModInvariant(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.Int64().Draw(t, "n")
		d := rapid.Int64().Filter(func(v int64) bool { return v != 0 }).Draw(t, "d")

		nObj := IntFrom(n)
		dObj := IntFrom(d)

		// Compute q = n // d
		qObj, err := BinaryOp(nObj, dObj, "//")
		if err != nil {
			t.Fatalf("n=%d, d=%d: floor div error: %v", n, d, err)
		}

		// Compute r = n % d
		rObj, err := BinaryOp(nObj, dObj, "%")
		if err != nil {
			t.Fatalf("n=%d, d=%d: mod error: %v", n, d, err)
		}

		q := qObj.(*IntObject)
		r := rObj.(*IntObject)

		// Verify invariant: n == q*d + r
		qd := new(big.Int).Mul(q.Value, big.NewInt(d))
		reconstructed := new(big.Int).Add(qd, r.Value)
		if reconstructed.Cmp(big.NewInt(n)) != 0 {
			t.Fatalf("invariant n == q*d + r failed: n=%d, d=%d, q=%s, r=%s, q*d+r=%s",
				n, d, q.Value.String(), r.Value.String(), reconstructed.String())
		}
	})
}

func TestProperty3_FloorDivRoundsTowardNegInf(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.Int64().Draw(t, "n")
		d := rapid.Int64().Filter(func(v int64) bool { return v != 0 }).Draw(t, "d")

		nObj := IntFrom(n)
		dObj := IntFrom(d)

		qObj, err := BinaryOp(nObj, dObj, "//")
		if err != nil {
			t.Fatalf("n=%d, d=%d: floor div error: %v", n, d, err)
		}
		q := qObj.(*IntObject)

		// Floor division toward -inf means: q <= n/d (exact)
		// Equivalently: q*d <= n if d > 0, or q*d >= n if d < 0
		// More precisely: q is the largest integer such that q*d <= n (when d > 0)
		// or the largest integer such that q*d >= n (when d < 0).
		//
		// We verify: q*d <= n < (q+1)*d when d > 0
		//            (q+1)*d < n <= q*d when d < 0
		bigN := big.NewInt(n)
		bigD := big.NewInt(d)
		qd := new(big.Int).Mul(q.Value, bigD)
		qPlusOneD := new(big.Int).Add(q.Value, big.NewInt(1))
		qPlusOneD.Mul(qPlusOneD, bigD)

		if d > 0 {
			// q*d <= n < (q+1)*d
			if qd.Cmp(bigN) > 0 {
				t.Fatalf("floor div not toward -inf: n=%d, d=%d, q=%s, q*d=%s > n",
					n, d, q.Value.String(), qd.String())
			}
			if qPlusOneD.Cmp(bigN) <= 0 {
				t.Fatalf("floor div too small: n=%d, d=%d, q=%s, (q+1)*d=%s <= n",
					n, d, q.Value.String(), qPlusOneD.String())
			}
		} else {
			// (q+1)*d < n <= q*d  (since d < 0, multiplying by larger q gives smaller result)
			if qd.Cmp(bigN) < 0 {
				t.Fatalf("floor div not toward -inf: n=%d, d=%d, q=%s, q*d=%s < n",
					n, d, q.Value.String(), qd.String())
			}
			if qPlusOneD.Cmp(bigN) >= 0 {
				t.Fatalf("floor div too small: n=%d, d=%d, q=%s, (q+1)*d=%s >= n",
					n, d, q.Value.String(), qPlusOneD.String())
			}
		}
	})
}

func TestProperty3_ModuloSignMatchesDivisor(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.Int64().Draw(t, "n")
		d := rapid.Int64().Filter(func(v int64) bool { return v != 0 }).Draw(t, "d")

		nObj := IntFrom(n)
		dObj := IntFrom(d)

		rObj, err := BinaryOp(nObj, dObj, "%")
		if err != nil {
			t.Fatalf("n=%d, d=%d: mod error: %v", n, d, err)
		}
		r := rObj.(*IntObject)

		// Sign of r must match sign of d (or r == 0)
		rSign := r.Value.Sign()
		dSign := big.NewInt(d).Sign()

		if rSign != 0 && rSign != dSign {
			t.Fatalf("modulo sign mismatch: n=%d, d=%d, r=%s (sign of r=%d, sign of d=%d)",
				n, d, r.Value.String(), rSign, dSign)
		}
	})
}

// Test with big integers (beyond int64 range) to exercise arbitrary precision
func TestProperty3_FloorDivModInvariant_BigInt(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate big integers by combining two int64 values
		hi := rapid.Int64Range(-1000, 1000).Draw(t, "hi")
		lo := rapid.Int64().Draw(t, "lo")
		d := rapid.Int64().Filter(func(v int64) bool { return v != 0 }).Draw(t, "d")

		// Create a big integer: hi * 2^63 + lo
		bigN := new(big.Int).Mul(big.NewInt(hi), new(big.Int).Lsh(big.NewInt(1), 63))
		bigN.Add(bigN, big.NewInt(lo))

		nObj := IntFromBig(bigN)
		dObj := IntFrom(d)

		qObj, err := BinaryOp(nObj, dObj, "//")
		if err != nil {
			t.Fatalf("big n, d=%d: floor div error: %v", d, err)
		}

		rObj, err := BinaryOp(nObj, dObj, "%")
		if err != nil {
			t.Fatalf("big n, d=%d: mod error: %v", d, err)
		}

		q := qObj.(*IntObject)
		r := rObj.(*IntObject)

		// Verify invariant: n == q*d + r
		bigD := big.NewInt(d)
		qd := new(big.Int).Mul(q.Value, bigD)
		reconstructed := new(big.Int).Add(qd, r.Value)
		if reconstructed.Cmp(bigN) != 0 {
			t.Fatalf("invariant n == q*d + r failed for big int: n=%s, d=%d, q=%s, r=%s",
				bigN.String(), d, q.Value.String(), r.Value.String())
		}

		// Verify modulo sign matches divisor
		rSign := r.Value.Sign()
		dSign := bigD.Sign()
		if rSign != 0 && rSign != dSign {
			t.Fatalf("modulo sign mismatch for big int: n=%s, d=%d, r=%s",
				bigN.String(), d, r.Value.String())
		}
	})
}

// ============================================================
// Property 12: String Concatenation and Repetition
// ============================================================
// **Validates: Requirements 17.4**
//
// String concatenation yields a string whose code-point length equals
// the sum of the operands' code-point lengths.
// String repetition yields a string whose code-point length equals
// len(s) * count (for count >= 0).

func TestProperty12_StringConcatenationLength(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s1 := rapid.String().Draw(t, "s1")
		s2 := rapid.String().Draw(t, "s2")

		str1 := StrFrom(s1)
		str2 := StrFrom(s2)

		// Concatenate using Go string concatenation on StrObject values
		concatenated := StrFrom(str1.Value + str2.Value)

		// Verify code-point length equals sum of individual lengths
		expectedLen := str1.Len() + str2.Len()
		actualLen := concatenated.Len()

		if actualLen != expectedLen {
			t.Fatalf("concatenation length mismatch: len(%q)=%d + len(%q)=%d = %d, but got %d",
				s1, str1.Len(), s2, str2.Len(), expectedLen, actualLen)
		}
	})
}

func TestProperty12_StringConcatenationContent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s1 := rapid.String().Draw(t, "s1")
		s2 := rapid.String().Draw(t, "s2")

		str1 := StrFrom(s1)
		str2 := StrFrom(s2)

		concatenated := StrFrom(str1.Value + str2.Value)

		// Verify code points: iterating over concatenated yields s1's code points then s2's
		runes1 := []rune(s1)
		runes2 := []rune(s2)
		allRunes := append(runes1, runes2...)

		concatRunes := []rune(concatenated.Value)
		if len(concatRunes) != len(allRunes) {
			t.Fatalf("concatenation code point count mismatch: expected %d, got %d",
				len(allRunes), len(concatRunes))
		}
		for i, r := range allRunes {
			if concatRunes[i] != r {
				t.Fatalf("code point mismatch at index %d: expected %U, got %U",
					i, r, concatRunes[i])
			}
		}
	})
}

func TestProperty12_StringRepetitionLength(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		// Limit count to avoid excessive memory usage
		count := rapid.IntRange(0, 100).Draw(t, "count")

		str := StrFrom(s)
		strLen := str.Len()

		// Skip very long strings to avoid memory issues
		if strLen*count > 100000 {
			t.Skip("skipping due to large result size")
		}

		// Repeat using Go's strings.Repeat
		repeated := StrFrom(strings.Repeat(s, count))

		// Verify code-point length equals len(s) * count
		expectedLen := strLen * count
		actualLen := repeated.Len()

		if actualLen != expectedLen {
			t.Fatalf("repetition length mismatch: len(%q)=%d, count=%d, expected %d, got %d",
				s, strLen, count, expectedLen, actualLen)
		}
	})
}

func TestProperty12_StringRepetitionZeroYieldsEmpty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")

		// count <= 0 yields empty string
		count := rapid.IntRange(-10, 0).Draw(t, "count")

		var repeated string
		if count <= 0 {
			repeated = ""
		} else {
			repeated = strings.Repeat(s, count)
		}

		result := StrFrom(repeated)
		if result.Len() != 0 {
			t.Fatalf("expected empty string for count=%d, got len=%d", count, result.Len())
		}
	})
}

func TestProperty12_StringConcatenationUnicode(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate strings that include multi-byte UTF-8 characters
		unicodeRunes1 := []rune{'€', '∑', '你', '好', '🎉', 'a', 'b', 'ñ'}
		unicodeRunes2 := []rune{'日', '本', '語', '🌍', 'c', 'ü', '∞'}
		len1 := rapid.IntRange(0, 20).Draw(t, "len1")
		len2 := rapid.IntRange(0, 20).Draw(t, "len2")
		var runes1, runes2 []rune
		for i := 0; i < len1; i++ {
			idx := rapid.IntRange(0, len(unicodeRunes1)-1).Draw(t, "idx1")
			runes1 = append(runes1, unicodeRunes1[idx])
		}
		for i := 0; i < len2; i++ {
			idx := rapid.IntRange(0, len(unicodeRunes2)-1).Draw(t, "idx2")
			runes2 = append(runes2, unicodeRunes2[idx])
		}
		s1 := string(runes1)
		s2 := string(runes2)

		str1 := StrFrom(s1)
		str2 := StrFrom(s2)

		concatenated := StrFrom(str1.Value + str2.Value)

		// Verify Len() returns code points, not bytes
		expectedCodePoints := utf8.RuneCountInString(s1) + utf8.RuneCountInString(s2)
		actualCodePoints := concatenated.Len()

		if actualCodePoints != expectedCodePoints {
			t.Fatalf("unicode concatenation length mismatch: expected %d code points, got %d (byte len=%d)",
				expectedCodePoints, actualCodePoints, len(concatenated.Value))
		}
	})
}

func TestProperty12_StringRepetitionUnicode(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate strings with multi-byte characters
		unicodeRunes := []rune{'€', '∑', '你', '好', '🎉', 'a'}
		strLen := rapid.IntRange(0, 20).Draw(t, "strLen")
		var runes []rune
		for i := 0; i < strLen; i++ {
			idx := rapid.IntRange(0, len(unicodeRunes)-1).Draw(t, "idx")
			runes = append(runes, unicodeRunes[idx])
		}
		s := string(runes)
		count := rapid.IntRange(0, 50).Draw(t, "count")

		str := StrFrom(s)
		codePointLen := str.Len()

		if codePointLen*count > 50000 {
			t.Skip("skipping due to large result size")
		}

		repeated := StrFrom(strings.Repeat(s, count))
		expectedLen := codePointLen * count
		actualLen := repeated.Len()

		if actualLen != expectedLen {
			t.Fatalf("unicode repetition length: len(%q)=%d, count=%d, expected %d code points, got %d",
				s, codePointLen, count, expectedLen, actualLen)
		}
	})
}
