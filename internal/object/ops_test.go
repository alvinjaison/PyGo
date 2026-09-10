package object

import (
	"math/big"
	"testing"

	"github.com/alvinjaison/pygo/internal/bytecode"
)

// ============================================================
// BinaryOp dispatch tests
// ============================================================

func TestBinaryOp_IntAdd(t *testing.T) {
	result, err := BinaryOp(IntFrom(10), IntFrom(20), "+")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, 30)
}

func TestBinaryOp_IntSub(t *testing.T) {
	result, err := BinaryOp(IntFrom(30), IntFrom(7), "-")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, 23)
}

func TestBinaryOp_IntMul(t *testing.T) {
	result, err := BinaryOp(IntFrom(6), IntFrom(7), "*")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, 42)
}

func TestBinaryOp_IntTrueDiv(t *testing.T) {
	result, err := BinaryOp(IntFrom(7), IntFrom(2), "/")
	if err != nil {
		t.Fatal(err)
	}
	assertFloatResult(t, result, 3.5)
}

func TestBinaryOp_IntFloorDiv(t *testing.T) {
	result, err := BinaryOp(IntFrom(7), IntFrom(2), "//")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, 3)
}

func TestBinaryOp_IntFloorDiv_NegativeInfinity(t *testing.T) {
	// -7 // 2 = -4 (rounds toward -inf, not toward zero)
	result, err := BinaryOp(IntFrom(-7), IntFrom(2), "//")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, -4)
}

func TestBinaryOp_IntMod_SignMatchesDivisor(t *testing.T) {
	// -7 % 3 = 2 (sign matches divisor, which is positive)
	result, err := BinaryOp(IntFrom(-7), IntFrom(3), "%")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, 2)

	// 7 % -3 = -2 (sign matches divisor, which is negative)
	result, err = BinaryOp(IntFrom(7), IntFrom(-3), "%")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, -2)
}

func TestBinaryOp_IntPow(t *testing.T) {
	result, err := BinaryOp(IntFrom(2), IntFrom(10), "**")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, 1024)
}

func TestBinaryOp_IntPow_NegativeExponent(t *testing.T) {
	// 2 ** -1 = 0.5 (returns float)
	result, err := BinaryOp(IntFrom(2), IntFrom(-1), "**")
	if err != nil {
		t.Fatal(err)
	}
	assertFloatResult(t, result, 0.5)
}

func TestBinaryOp_DivisionByZero(t *testing.T) {
	_, err := BinaryOp(IntFrom(1), IntFrom(0), "/")
	if err == nil {
		t.Fatal("expected ZeroDivisionError")
	}

	_, err = BinaryOp(IntFrom(1), IntFrom(0), "//")
	if err == nil {
		t.Fatal("expected ZeroDivisionError")
	}

	_, err = BinaryOp(IntFrom(1), IntFrom(0), "%")
	if err == nil {
		t.Fatal("expected ZeroDivisionError")
	}
}

// ============================================================
// Mixed int/float promotion tests
// ============================================================

func TestBinaryOp_MixedIntFloat_Add(t *testing.T) {
	result, err := BinaryOp(IntFrom(3), FloatFrom(1.5), "+")
	if err != nil {
		t.Fatal(err)
	}
	assertFloatResult(t, result, 4.5)
}

func TestBinaryOp_MixedFloatInt_Add(t *testing.T) {
	result, err := BinaryOp(FloatFrom(1.5), IntFrom(3), "+")
	if err != nil {
		t.Fatal(err)
	}
	assertFloatResult(t, result, 4.5)
}

// ============================================================
// Float arithmetic tests
// ============================================================

func TestBinaryOp_FloatAdd(t *testing.T) {
	result, err := BinaryOp(FloatFrom(1.5), FloatFrom(2.5), "+")
	if err != nil {
		t.Fatal(err)
	}
	assertFloatResult(t, result, 4.0)
}

func TestBinaryOp_FloatFloorDiv(t *testing.T) {
	result, err := BinaryOp(FloatFrom(7.5), FloatFrom(2.0), "//")
	if err != nil {
		t.Fatal(err)
	}
	assertFloatResult(t, result, 3.0)
}

func TestBinaryOp_FloatMod(t *testing.T) {
	// -7.0 % 3.0 = 2.0 (sign matches divisor)
	result, err := BinaryOp(FloatFrom(-7.0), FloatFrom(3.0), "%")
	if err != nil {
		t.Fatal(err)
	}
	assertFloatResult(t, result, 2.0)
}

// ============================================================
// Bitwise operator tests
// ============================================================

func TestBinaryOp_IntBitAnd(t *testing.T) {
	result, err := BinaryOp(IntFrom(0b1100), IntFrom(0b1010), "&")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, 0b1000)
}

func TestBinaryOp_IntBitOr(t *testing.T) {
	result, err := BinaryOp(IntFrom(0b1100), IntFrom(0b1010), "|")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, 0b1110)
}

func TestBinaryOp_IntBitXor(t *testing.T) {
	result, err := BinaryOp(IntFrom(0b1100), IntFrom(0b1010), "^")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, 0b0110)
}

func TestBinaryOp_IntLShift(t *testing.T) {
	result, err := BinaryOp(IntFrom(1), IntFrom(3), "<<")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, 8)
}

func TestBinaryOp_IntRShift(t *testing.T) {
	result, err := BinaryOp(IntFrom(8), IntFrom(2), ">>")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, 2)
}

func TestBinaryOp_BitwiseOnFloat_TypeError(t *testing.T) {
	_, err := BinaryOp(IntFrom(5), FloatFrom(2.0), "&")
	if err == nil {
		t.Fatal("expected TypeError for bitwise on float")
	}

	_, err = BinaryOp(FloatFrom(2.0), IntFrom(5), "&")
	if err == nil {
		t.Fatal("expected TypeError for bitwise on float")
	}
}

// ============================================================
// Unary operator tests
// ============================================================

func TestUnaryOp_IntNeg(t *testing.T) {
	result, err := UnaryOp(IntFrom(5), "-")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, -5)
}

func TestUnaryOp_IntPos(t *testing.T) {
	result, err := UnaryOp(IntFrom(5), "+")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, 5)
}

func TestUnaryOp_IntInvert(t *testing.T) {
	// ~5 == -6
	result, err := UnaryOp(IntFrom(5), "~")
	if err != nil {
		t.Fatal(err)
	}
	assertIntResult(t, result, -6)
}

func TestUnaryOp_FloatNeg(t *testing.T) {
	result, err := UnaryOp(FloatFrom(3.14), "-")
	if err != nil {
		t.Fatal(err)
	}
	assertFloatResult(t, result, -3.14)
}

func TestUnaryOp_FloatPos(t *testing.T) {
	result, err := UnaryOp(FloatFrom(3.14), "+")
	if err != nil {
		t.Fatal(err)
	}
	assertFloatResult(t, result, 3.14)
}

func TestUnaryOp_FloatInvert_TypeError(t *testing.T) {
	_, err := UnaryOp(FloatFrom(3.14), "~")
	if err == nil {
		t.Fatal("expected TypeError for ~float")
	}
}

// ============================================================
// Comparison tests
// ============================================================

func TestCompareObjects_IntLt(t *testing.T) {
	result, err := CompareObjects(IntFrom(3), IntFrom(5), bytecode.CmpLt)
	if err != nil {
		t.Fatal(err)
	}
	assertBoolResult(t, result, true)
}

func TestCompareObjects_IntEq(t *testing.T) {
	result, err := CompareObjects(IntFrom(5), IntFrom(5), bytecode.CmpEq)
	if err != nil {
		t.Fatal(err)
	}
	assertBoolResult(t, result, true)
}

func TestCompareObjects_MixedIntFloat(t *testing.T) {
	result, err := CompareObjects(IntFrom(5), FloatFrom(5.0), bytecode.CmpEq)
	if err != nil {
		t.Fatal(err)
	}
	assertBoolResult(t, result, true)
}

func TestCompareObjects_StrLt(t *testing.T) {
	result, err := CompareObjects(StrFrom("abc"), StrFrom("abd"), bytecode.CmpLt)
	if err != nil {
		t.Fatal(err)
	}
	assertBoolResult(t, result, true)
}

func TestCompareObjects_Is(t *testing.T) {
	obj := IntFrom(5)
	result, err := CompareObjects(obj, obj, bytecode.CmpIs)
	if err != nil {
		t.Fatal(err)
	}
	assertBoolResult(t, result, true)
}

func TestCompareObjects_IsNot(t *testing.T) {
	result, err := CompareObjects(IntFrom(5), IntFrom(5), bytecode.CmpIsNot)
	if err != nil {
		t.Fatal(err)
	}
	// Two different IntFrom(5) calls outside cache range still equal via value,
	// but 5 is in the cache so they are the same object.
	assertBoolResult(t, result, false)
}

// ============================================================
// Dispatch protocol tests: TypeError on unsupported types
// ============================================================

func TestBinaryOp_TypeError_StrInt(t *testing.T) {
	_, err := BinaryOp(StrFrom("hello"), IntFrom(5), "+")
	if err == nil {
		t.Fatal("expected TypeError for str + int")
	}
}

func TestBinaryOp_TypeError_NoneAdd(t *testing.T) {
	_, err := BinaryOp(None, IntFrom(5), "+")
	if err == nil {
		t.Fatal("expected TypeError for None + int")
	}
}

// ============================================================
// Floor division invariant: n == (n//d)*d + (n%d)
// ============================================================

func TestFloorDivModInvariant(t *testing.T) {
	cases := [][2]int64{
		{7, 3}, {-7, 3}, {7, -3}, {-7, -3},
		{10, 5}, {-10, 5}, {0, 3}, {1, 1},
	}
	for _, tc := range cases {
		n, d := tc[0], tc[1]
		quotient, err := BinaryOp(IntFrom(n), IntFrom(d), "//")
		if err != nil {
			t.Fatalf("n=%d, d=%d: %v", n, d, err)
		}
		remainder, err := BinaryOp(IntFrom(n), IntFrom(d), "%")
		if err != nil {
			t.Fatalf("n=%d, d=%d: %v", n, d, err)
		}

		// n == q*d + r
		qd, err := BinaryOp(quotient, IntFrom(d), "*")
		if err != nil {
			t.Fatalf("n=%d, d=%d: %v", n, d, err)
		}
		reconstructed, err := BinaryOp(qd, remainder, "+")
		if err != nil {
			t.Fatalf("n=%d, d=%d: %v", n, d, err)
		}

		rInt := reconstructed.(*IntObject)
		if rInt.Value.Cmp(big.NewInt(n)) != 0 {
			t.Fatalf("invariant failed: n=%d, d=%d, q=%s, r=%s, q*d+r=%s",
				n, d, quotient.String(), remainder.String(), rInt.Value.String())
		}
	}
}

// ============================================================
// Helper assertions
// ============================================================

func assertIntResult(t *testing.T, result Object, expected int64) {
	t.Helper()
	intObj, ok := result.(*IntObject)
	if !ok {
		t.Fatalf("expected IntObject, got %T (%s)", result, result.String())
	}
	if intObj.Value.Int64() != expected {
		t.Fatalf("expected %d, got %s", expected, intObj.Value.String())
	}
}

func assertFloatResult(t *testing.T, result Object, expected float64) {
	t.Helper()
	floatObj, ok := result.(*FloatObject)
	if !ok {
		t.Fatalf("expected FloatObject, got %T (%s)", result, result.String())
	}
	if floatObj.Value != expected {
		t.Fatalf("expected %f, got %f", expected, floatObj.Value)
	}
}

func assertBoolResult(t *testing.T, result Object, expected bool) {
	t.Helper()
	boolObj, ok := result.(*BoolObject)
	if !ok {
		t.Fatalf("expected BoolObject, got %T (%s)", result, result.String())
	}
	if boolObj.Value != expected {
		t.Fatalf("expected %v, got %v", expected, boolObj.Value)
	}
}
