package object

import (
	"math"
	"strconv"

	"github.com/alvinjaison/PyGo/internal/bytecode"
)

// Fast paths for the overwhelmingly common operand types. They only handle
// exact *IntObject / *FloatObject / *StrObject operands (never user instances
// or subclasses) and decline, returning ok=false, whenever the general path
// is needed: overflow, division by zero, big integers, mixed exotic types.

const maxExactFloatInt = 1 << 53

// fastBinary computes left op right for plain ints and floats.
func fastBinary(left, right Object, op string) (Object, bool) {
	switch x := left.(type) {
	case *IntObject:
		switch y := right.(type) {
		case *IntObject:
			if x.isSmall && y.isSmall {
				return fastIntOp(x.small, y.small, op)
			}
		case *FloatObject:
			if x.isSmall {
				return fastFloatOp(float64(x.small), y.Value, op)
			}
		}
	case *FloatObject:
		switch y := right.(type) {
		case *FloatObject:
			return fastFloatOp(x.Value, y.Value, op)
		case *IntObject:
			if y.isSmall {
				return fastFloatOp(x.Value, float64(y.small), op)
			}
		}
	}
	return nil, false
}

func fastIntOp(a, b int64, op string) (Object, bool) {
	switch op {
	case "+":
		s := a + b
		if (a^s)&(b^s) < 0 {
			return nil, false // overflow
		}
		return IntFrom(s), true
	case "-":
		s := a - b
		if (a^b)&(a^s) < 0 {
			return nil, false
		}
		return IntFrom(s), true
	case "*":
		if a == 0 || b == 0 {
			return IntFrom(0), true
		}
		p := a * b
		if p/b != a || (a == -1 && b == math.MinInt64) || (b == -1 && a == math.MinInt64) {
			return nil, false
		}
		return IntFrom(p), true
	case "//", "%":
		if b == 0 || (a == math.MinInt64 && b == -1) {
			return nil, false
		}
		q, r := a/b, a%b
		if r != 0 && (r < 0) != (b < 0) { // Python floors toward -inf
			q--
			r += b
		}
		if op == "//" {
			return IntFrom(q), true
		}
		return IntFrom(r), true
	case "/":
		if b == 0 || a > maxExactFloatInt || a < -maxExactFloatInt ||
			b > maxExactFloatInt || b < -maxExactFloatInt {
			return nil, false
		}
		return FloatFrom(float64(a) / float64(b)), true
	case "&":
		return IntFrom(a & b), true
	case "|":
		return IntFrom(a | b), true
	case "^":
		return IntFrom(a ^ b), true
	}
	return nil, false
}

func fastFloatOp(a, b float64, op string) (Object, bool) {
	switch op {
	case "+":
		return FloatFrom(a + b), true
	case "-":
		return FloatFrom(a - b), true
	case "*":
		return FloatFrom(a * b), true
	case "/":
		if b == 0 {
			return nil, false
		}
		return FloatFrom(a / b), true
	}
	return nil, false
}

// fastCompare evaluates a rich comparison for plain ints, floats and strs.
func fastCompare(left, right Object, op bytecode.CompareOp) (Object, bool) {
	switch x := left.(type) {
	case *IntObject:
		switch y := right.(type) {
		case *IntObject:
			if x.isSmall && y.isSmall {
				return cmpOrdered(op, x.small < y.small, x.small == y.small)
			}
		case *FloatObject:
			if x.isSmall && x.small <= maxExactFloatInt && x.small >= -maxExactFloatInt {
				f := float64(x.small)
				return cmpFloat(op, f, y.Value)
			}
		}
	case *FloatObject:
		switch y := right.(type) {
		case *FloatObject:
			return cmpFloat(op, x.Value, y.Value)
		case *IntObject:
			if y.isSmall && y.small <= maxExactFloatInt && y.small >= -maxExactFloatInt {
				return cmpFloat(op, x.Value, float64(y.small))
			}
		}
	case *StrObject:
		if y, ok := right.(*StrObject); ok {
			return cmpOrdered(op, x.Value < y.Value, x.Value == y.Value)
		}
	}
	return nil, false
}

// cmpOrdered maps (less, equal) onto the six comparison operators.
func cmpOrdered(op bytecode.CompareOp, less, equal bool) (Object, bool) {
	switch op {
	case bytecode.CmpLt:
		return BoolFrom(less), true
	case bytecode.CmpLtE:
		return BoolFrom(less || equal), true
	case bytecode.CmpEq:
		return BoolFrom(equal), true
	case bytecode.CmpNotEq:
		return BoolFrom(!equal), true
	case bytecode.CmpGt:
		return BoolFrom(!less && !equal), true
	case bytecode.CmpGtE:
		return BoolFrom(!less), true
	}
	return nil, false
}

// cmpFloat compares floats with IEEE semantics (NaN is unordered).
func cmpFloat(op bytecode.CompareOp, a, b float64) (Object, bool) {
	switch op {
	case bytecode.CmpLt:
		return BoolFrom(a < b), true
	case bytecode.CmpLtE:
		return BoolFrom(a <= b), true
	case bytecode.CmpEq:
		return BoolFrom(a == b), true
	case bytecode.CmpNotEq:
		return BoolFrom(a != b), true
	case bytecode.CmpGt:
		return BoolFrom(a > b), true
	case bytecode.CmpGtE:
		return BoolFrom(a >= b), true
	}
	return nil, false
}

// intString formats an int64 without going through big.Int.
func intString(n int64) string { return strconv.FormatInt(n, 10) }
