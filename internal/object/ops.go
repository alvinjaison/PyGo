package object

import (
	"fmt"
	"math"
	"math/big"

	"github.com/alvinjaison/pygo/internal/bytecode"
)

// ============================================================
// Binary operator dispatch
// ============================================================

// BinaryOp dispatches a binary operation on two Python objects.
// It follows Python's operator dispatch protocol:
// 1. If type(right) is a strict subtype of type(left), try right's reflected slot first
// 2. Try left operand's slot
// 3. If NotImplemented, try right operand's reflected slot
// 4. If both return NotImplemented, raise TypeError
func BinaryOp(left, right Object, op string) (Object, error) {
	leftType := left.Type()
	rightType := right.Type()

	// Step 1: If right is a strict subtype of left, try right's reflected first.
	if rightType != leftType && rightType.IsSubtype(leftType) {
		if result, err, ok := trySlotOp(left, right, op, true); ok {
			if !IsNotImplemented(result) {
				return result, err
			}
		}
	}

	// Step 2: Try left's slot.
	if result, err, ok := trySlotOp(left, right, op, false); ok {
		if !IsNotImplemented(result) {
			return result, err
		}
	}

	// Step 3: Try right's reflected slot (unless already tried above).
	if !(rightType != leftType && rightType.IsSubtype(leftType)) {
		if result, err, ok := trySlotOp(left, right, op, true); ok {
			if !IsNotImplemented(result) {
				return result, err
			}
		}
	}

	// Step 4: TypeError.
	return nil, fmt.Errorf("TypeError: unsupported operand type(s) for %s: '%s' and '%s'",
		op, leftType.Name(), rightType.Name())
}

// trySlotOp attempts a numeric slot operation.
func trySlotOp(left, right Object, op string, reflected bool) (Object, error, bool) {
	var owner Object
	if reflected {
		owner = right
	} else {
		owner = left
	}

	slot := getNumericSlot(owner.Type(), op, reflected)
	if slot == nil {
		return nil, nil, false
	}
	if reflected {
		result, err := slot(right, left)
		return result, err, true
	}
	result, err := slot(left, right)
	return result, err, true
}

// getNumericSlot returns the appropriate slot function for an operation.
func getNumericSlot(typ *TypeObject, op string, reflected bool) func(Object, Object) (Object, error) {
	s := &typ.slots
	if !reflected {
		switch op {
		case "+":
			return s.Add
		case "-":
			return s.Sub
		case "*":
			return s.Mul
		case "/":
			return s.Div
		case "//":
			return s.FloorDiv
		case "%":
			return s.Mod
		case "**":
			return s.Pow
		case "&":
			return s.BitAnd
		case "|":
			return s.BitOr
		case "^":
			return s.BitXor
		case "<<":
			return s.LShift
		case ">>":
			return s.RShift
		}
	} else {
		switch op {
		case "+":
			return s.RAdd
		case "-":
			return s.RSub
		case "*":
			return s.RMul
		case "/":
			return s.RDiv
		case "//":
			return s.RFloorDiv
		case "%":
			return s.RMod
		case "**":
			return s.RPow
		}
	}
	return nil
}

// ============================================================
// Unary operator dispatch
// ============================================================

// UnaryOp dispatches a unary operation on a Python object.
func UnaryOp(val Object, op string) (Object, error) {
	s := val.Type().Slots()
	switch op {
	case "-":
		if s.Neg != nil {
			return s.Neg(val)
		}
	case "+":
		if s.Pos != nil {
			return s.Pos(val)
		}
	case "~":
		if s.Invert != nil {
			return s.Invert(val)
		}
	}
	return nil, fmt.Errorf("TypeError: bad operand type for unary %s: '%s'", op, val.Type().Name())
}

// ============================================================
// Comparison dispatch
// ============================================================

// CompareObjects performs a Python comparison operation.
func CompareObjects(left, right Object, op bytecode.CompareOp) (Object, error) {
	// Identity comparisons.
	if op == bytecode.CmpIs {
		return BoolFrom(left == right), nil
	}
	if op == bytecode.CmpIsNot {
		return BoolFrom(left != right), nil
	}

	// Exception matching for except clauses.
	// left is the raised exception, right is the exception type to match against.
	if op == bytecode.CmpExcMatch {
		exc, isExc := left.(*ExceptionObject)
		if !isExc {
			return BoolFrom(false), nil
		}
		excType, isType := right.(*TypeObject)
		if !isType {
			return nil, fmt.Errorf("TypeError: catching classes that do not inherit from BaseException is not allowed")
		}
		// Check if the exception's type is the target type or a subtype of it.
		return BoolFrom(exc.ExcType.IsSubtype(excType)), nil
	}

	// Membership tests.
	if op == bytecode.CmpIn || op == bytecode.CmpNotIn {
		container, item := right, left
		if c, ok := container.(Container); ok {
			found, err := c.Contains(item)
			if err != nil {
				return nil, err
			}
			if op == bytecode.CmpNotIn {
				found = !found
			}
			return BoolFrom(found), nil
		}
		return nil, fmt.Errorf("TypeError: argument of type '%s' is not iterable",
			container.Type().Name())
	}

	// Map bytecode.CompareOp to object.CompareOp.
	var objOp CompareOp
	switch op {
	case bytecode.CmpLt:
		objOp = OpLt
	case bytecode.CmpLtE:
		objOp = OpLtE
	case bytecode.CmpEq:
		objOp = OpEq
	case bytecode.CmpNotEq:
		objOp = OpNotEq
	case bytecode.CmpGt:
		objOp = OpGt
	case bytecode.CmpGtE:
		objOp = OpGtE
	default:
		return nil, fmt.Errorf("unknown compare op %d", op)
	}

	return RichCompareObjects(left, right, objOp)
}

// RichCompareObjects applies the rich comparison protocol to two objects:
// try the left operand's slot, then the right operand's reflected slot, then
// fall back to identity for == and != (any two objects can be compared for
// equality). Ordering operators with no implementation raise TypeError.
//
// Collection types call this to compare their elements, so it is exported
// separately from CompareObjects, which additionally handles the operators that
// are not part of the protocol (is, in, exception matching).
func RichCompareObjects(left, right Object, op CompareOp) (Object, error) {
	// Try left's RichCompare.
	if c, ok := left.(Comparable); ok {
		result, err := c.RichCompare(right, op)
		if err != nil {
			return nil, err
		}
		if !IsNotImplemented(result) {
			return result, nil
		}
	}

	// Try right's reflected RichCompare.
	if c, ok := right.(Comparable); ok {
		reflected := reflectCompareOp(op)
		result, err := c.RichCompare(left, reflected)
		if err != nil {
			return nil, err
		}
		if !IsNotImplemented(result) {
			return result, nil
		}
	}

	// Fall back to identity comparison for == and !=.
	switch op {
	case OpEq:
		return BoolFrom(left == right), nil
	case OpNotEq:
		return BoolFrom(left != right), nil
	}

	return nil, fmt.Errorf("TypeError: '%s' not supported between instances of '%s' and '%s'",
		op, left.Type().Name(), right.Type().Name())
}

func reflectCompareOp(op CompareOp) CompareOp {
	switch op {
	case OpLt:
		return OpGt
	case OpLtE:
		return OpGtE
	case OpGt:
		return OpLt
	case OpGtE:
		return OpLtE
	default:
		return op
	}
}

// ============================================================
// Iterator helper
// ============================================================

// GetIter returns an iterator for obj, or a TypeError if obj is not iterable.
func GetIter(obj Object) (Iterator, error) {
	if it, ok := obj.(Iterator); ok {
		return it, nil
	}
	if it, ok := obj.(Iterable); ok {
		return it.Iter()
	}
	return nil, fmt.Errorf("TypeError: '%s' object is not iterable", obj.Type().Name())
}

// ============================================================
// MakeFunction flags
// ============================================================

// MakeFunctionFlags is a bitmask for the MAKE_FUNCTION instruction.
type MakeFunctionFlags = uint32

const (
	MakeFuncHasDefaults    MakeFunctionFlags = 1 << 0
	MakeFuncHasKwDefaults  MakeFunctionFlags = 1 << 1
	MakeFuncHasAnnotations MakeFunctionFlags = 1 << 2
	MakeFuncHasClosure     MakeFunctionFlags = 1 << 3
)

// ============================================================
// Int arithmetic slot implementations
// ============================================================

// intAdd implements int.__add__
func intAdd(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *IntObject:
		return IntFromBig(new(big.Int).Add(ai.Value, bi.Value)), nil
	case *BoolObject:
		bv := boolToInt(bi)
		return IntFromBig(new(big.Int).Add(ai.Value, bv)), nil
	case *FloatObject:
		// Promote int to float
		af := intToFloat(ai)
		return FloatFrom(af + bi.Value), nil
	}
	return NotImplemented, nil
}

// intSub implements int.__sub__
func intSub(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *IntObject:
		return IntFromBig(new(big.Int).Sub(ai.Value, bi.Value)), nil
	case *BoolObject:
		bv := boolToInt(bi)
		return IntFromBig(new(big.Int).Sub(ai.Value, bv)), nil
	case *FloatObject:
		af := intToFloat(ai)
		return FloatFrom(af - bi.Value), nil
	}
	return NotImplemented, nil
}

// intMul implements int.__mul__
func intMul(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *IntObject:
		return IntFromBig(new(big.Int).Mul(ai.Value, bi.Value)), nil
	case *BoolObject:
		bv := boolToInt(bi)
		return IntFromBig(new(big.Int).Mul(ai.Value, bv)), nil
	case *FloatObject:
		af := intToFloat(ai)
		return FloatFrom(af * bi.Value), nil
	}
	return NotImplemented, nil
}

// intTrueDiv implements int.__truediv__ (always returns float)
func intTrueDiv(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *IntObject:
		if bi.Value.Sign() == 0 {
			return nil, fmt.Errorf("ZeroDivisionError: division by zero")
		}
		af := intToFloat(ai)
		bf := intToFloat(bi)
		return FloatFrom(af / bf), nil
	case *BoolObject:
		bv := boolToInt(bi)
		if bv.Sign() == 0 {
			return nil, fmt.Errorf("ZeroDivisionError: division by zero")
		}
		af := intToFloat(ai)
		bf, _ := new(big.Float).SetInt(bv).Float64()
		return FloatFrom(af / bf), nil
	case *FloatObject:
		if bi.Value == 0 {
			return nil, fmt.Errorf("ZeroDivisionError: float division by zero")
		}
		af := intToFloat(ai)
		return FloatFrom(af / bi.Value), nil
	}
	return NotImplemented, nil
}

// intFloorDiv implements int.__floordiv__
// Floor division rounds toward negative infinity.
func intFloorDiv(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *IntObject:
		if bi.Value.Sign() == 0 {
			return nil, fmt.Errorf("ZeroDivisionError: integer division or modulo by zero")
		}
		// big.Int Div rounds toward zero; we need floor division (toward -inf).
		result := floorDivBig(ai.Value, bi.Value)
		return IntFromBig(result), nil
	case *BoolObject:
		bv := boolToInt(bi)
		if bv.Sign() == 0 {
			return nil, fmt.Errorf("ZeroDivisionError: integer division or modulo by zero")
		}
		result := floorDivBig(ai.Value, bv)
		return IntFromBig(result), nil
	case *FloatObject:
		if bi.Value == 0 {
			return nil, fmt.Errorf("ZeroDivisionError: float floor division by zero")
		}
		af := intToFloat(ai)
		return FloatFrom(math.Floor(af / bi.Value)), nil
	}
	return NotImplemented, nil
}

// intMod implements int.__mod__
// Modulo result sign matches the divisor.
func intMod(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *IntObject:
		if bi.Value.Sign() == 0 {
			return nil, fmt.Errorf("ZeroDivisionError: integer division or modulo by zero")
		}
		result := floorModBig(ai.Value, bi.Value)
		return IntFromBig(result), nil
	case *BoolObject:
		bv := boolToInt(bi)
		if bv.Sign() == 0 {
			return nil, fmt.Errorf("ZeroDivisionError: integer division or modulo by zero")
		}
		result := floorModBig(ai.Value, bv)
		return IntFromBig(result), nil
	case *FloatObject:
		if bi.Value == 0 {
			return nil, fmt.Errorf("ZeroDivisionError: float modulo")
		}
		af := intToFloat(ai)
		return FloatFrom(floatMod(af, bi.Value)), nil
	}
	return NotImplemented, nil
}

// intPow implements int.__pow__
func intPow(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *IntObject:
		if bi.Value.Sign() < 0 {
			// Negative exponent: return float
			af := intToFloat(ai)
			if af == 0 {
				return nil, fmt.Errorf("ZeroDivisionError: 0.0 cannot be raised to a negative power")
			}
			bf := intToFloat(bi)
			return FloatFrom(math.Pow(af, bf)), nil
		}
		// Non-negative integer exponent
		result := new(big.Int).Exp(ai.Value, bi.Value, nil)
		return IntFromBig(result), nil
	case *BoolObject:
		bv := boolToInt(bi)
		result := new(big.Int).Exp(ai.Value, bv, nil)
		return IntFromBig(result), nil
	case *FloatObject:
		af := intToFloat(ai)
		if af == 0 && bi.Value < 0 {
			return nil, fmt.Errorf("ZeroDivisionError: 0.0 cannot be raised to a negative power")
		}
		return FloatFrom(math.Pow(af, bi.Value)), nil
	}
	return NotImplemented, nil
}

// intRAdd implements int.__radd__
func intRAdd(a, b Object) (Object, error) {
	return intAdd(a, b)
}

// intRSub implements int.__rsub__ (a is the right operand)
func intRSub(a, b Object) (Object, error) {
	// a.__rsub__(b) means b - a
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *IntObject:
		return IntFromBig(new(big.Int).Sub(bi.Value, ai.Value)), nil
	case *FloatObject:
		af := intToFloat(ai)
		return FloatFrom(bi.Value - af), nil
	}
	return NotImplemented, nil
}

// intRMul implements int.__rmul__
func intRMul(a, b Object) (Object, error) {
	return intMul(a, b)
}

// intRDiv implements int.__rtruediv__ (a is right, b is left: b / a)
func intRDiv(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	if ai.Value.Sign() == 0 {
		return nil, fmt.Errorf("ZeroDivisionError: division by zero")
	}
	switch bi := b.(type) {
	case *FloatObject:
		af := intToFloat(ai)
		return FloatFrom(bi.Value / af), nil
	}
	return NotImplemented, nil
}

// intRFloorDiv implements int.__rfloordiv__ (b // a)
func intRFloorDiv(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	if ai.Value.Sign() == 0 {
		return nil, fmt.Errorf("ZeroDivisionError: integer division or modulo by zero")
	}
	switch bi := b.(type) {
	case *FloatObject:
		af := intToFloat(ai)
		return FloatFrom(math.Floor(bi.Value / af)), nil
	}
	return NotImplemented, nil
}

// intRMod implements int.__rmod__ (b % a)
func intRMod(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	if ai.Value.Sign() == 0 {
		return nil, fmt.Errorf("ZeroDivisionError: integer division or modulo by zero")
	}
	switch bi := b.(type) {
	case *FloatObject:
		af := intToFloat(ai)
		return FloatFrom(floatMod(bi.Value, af)), nil
	}
	return NotImplemented, nil
}

// intRPow implements int.__rpow__ (b ** a)
func intRPow(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *FloatObject:
		af := intToFloat(ai)
		if bi.Value == 0 && af < 0 {
			return nil, fmt.Errorf("ZeroDivisionError: 0.0 cannot be raised to a negative power")
		}
		return FloatFrom(math.Pow(bi.Value, af)), nil
	}
	return NotImplemented, nil
}

// ============================================================
// Float arithmetic slot implementations
// ============================================================

// floatAdd implements float.__add__
func floatAdd(a, b Object) (Object, error) {
	af, ok := a.(*FloatObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *FloatObject:
		return FloatFrom(af.Value + bi.Value), nil
	case *IntObject:
		bf := intToFloat(bi)
		return FloatFrom(af.Value + bf), nil
	case *BoolObject:
		bf := boolToFloat(bi)
		return FloatFrom(af.Value + bf), nil
	}
	return NotImplemented, nil
}

// floatSub implements float.__sub__
func floatSub(a, b Object) (Object, error) {
	af, ok := a.(*FloatObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *FloatObject:
		return FloatFrom(af.Value - bi.Value), nil
	case *IntObject:
		bf := intToFloat(bi)
		return FloatFrom(af.Value - bf), nil
	case *BoolObject:
		bf := boolToFloat(bi)
		return FloatFrom(af.Value - bf), nil
	}
	return NotImplemented, nil
}

// floatMulOp implements float.__mul__
func floatMulOp(a, b Object) (Object, error) {
	af, ok := a.(*FloatObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *FloatObject:
		return FloatFrom(af.Value * bi.Value), nil
	case *IntObject:
		bf := intToFloat(bi)
		return FloatFrom(af.Value * bf), nil
	case *BoolObject:
		bf := boolToFloat(bi)
		return FloatFrom(af.Value * bf), nil
	}
	return NotImplemented, nil
}

// floatTrueDiv implements float.__truediv__
func floatTrueDiv(a, b Object) (Object, error) {
	af, ok := a.(*FloatObject)
	if !ok {
		return NotImplemented, nil
	}
	var bval float64
	switch bi := b.(type) {
	case *FloatObject:
		bval = bi.Value
	case *IntObject:
		bval = intToFloat(bi)
	case *BoolObject:
		bval = boolToFloat(bi)
	default:
		return NotImplemented, nil
	}
	if bval == 0 {
		return nil, fmt.Errorf("ZeroDivisionError: float division by zero")
	}
	return FloatFrom(af.Value / bval), nil
}

// floatFloorDivOp implements float.__floordiv__
func floatFloorDivOp(a, b Object) (Object, error) {
	af, ok := a.(*FloatObject)
	if !ok {
		return NotImplemented, nil
	}
	var bval float64
	switch bi := b.(type) {
	case *FloatObject:
		bval = bi.Value
	case *IntObject:
		bval = intToFloat(bi)
	case *BoolObject:
		bval = boolToFloat(bi)
	default:
		return NotImplemented, nil
	}
	if bval == 0 {
		return nil, fmt.Errorf("ZeroDivisionError: float floor division by zero")
	}
	return FloatFrom(math.Floor(af.Value / bval)), nil
}

// floatModOp implements float.__mod__
func floatModOp(a, b Object) (Object, error) {
	af, ok := a.(*FloatObject)
	if !ok {
		return NotImplemented, nil
	}
	var bval float64
	switch bi := b.(type) {
	case *FloatObject:
		bval = bi.Value
	case *IntObject:
		bval = intToFloat(bi)
	case *BoolObject:
		bval = boolToFloat(bi)
	default:
		return NotImplemented, nil
	}
	if bval == 0 {
		return nil, fmt.Errorf("ZeroDivisionError: float modulo")
	}
	return FloatFrom(floatMod(af.Value, bval)), nil
}

// floatPowOp implements float.__pow__
func floatPowOp(a, b Object) (Object, error) {
	af, ok := a.(*FloatObject)
	if !ok {
		return NotImplemented, nil
	}
	var bval float64
	switch bi := b.(type) {
	case *FloatObject:
		bval = bi.Value
	case *IntObject:
		bval = intToFloat(bi)
	case *BoolObject:
		bval = boolToFloat(bi)
	default:
		return NotImplemented, nil
	}
	if af.Value == 0 && bval < 0 {
		return nil, fmt.Errorf("ZeroDivisionError: 0.0 cannot be raised to a negative power")
	}
	return FloatFrom(math.Pow(af.Value, bval)), nil
}

// floatRAdd implements float.__radd__
func floatRAdd(a, b Object) (Object, error) {
	return floatAdd(a, b)
}

// floatRSub implements float.__rsub__ (b - a)
func floatRSub(a, b Object) (Object, error) {
	af, ok := a.(*FloatObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *IntObject:
		bf := intToFloat(bi)
		return FloatFrom(bf - af.Value), nil
	case *BoolObject:
		bf := boolToFloat(bi)
		return FloatFrom(bf - af.Value), nil
	}
	return NotImplemented, nil
}

// floatRMul implements float.__rmul__
func floatRMul(a, b Object) (Object, error) {
	return floatMulOp(a, b)
}

// floatRDiv implements float.__rtruediv__ (b / a)
func floatRDiv(a, b Object) (Object, error) {
	af, ok := a.(*FloatObject)
	if !ok {
		return NotImplemented, nil
	}
	if af.Value == 0 {
		return nil, fmt.Errorf("ZeroDivisionError: float division by zero")
	}
	switch bi := b.(type) {
	case *IntObject:
		bf := intToFloat(bi)
		return FloatFrom(bf / af.Value), nil
	case *BoolObject:
		bf := boolToFloat(bi)
		return FloatFrom(bf / af.Value), nil
	}
	return NotImplemented, nil
}

// floatRFloorDiv implements float.__rfloordiv__ (b // a)
func floatRFloorDiv(a, b Object) (Object, error) {
	af, ok := a.(*FloatObject)
	if !ok {
		return NotImplemented, nil
	}
	if af.Value == 0 {
		return nil, fmt.Errorf("ZeroDivisionError: float floor division by zero")
	}
	switch bi := b.(type) {
	case *IntObject:
		bf := intToFloat(bi)
		return FloatFrom(math.Floor(bf / af.Value)), nil
	case *BoolObject:
		bf := boolToFloat(bi)
		return FloatFrom(math.Floor(bf / af.Value)), nil
	}
	return NotImplemented, nil
}

// floatRMod implements float.__rmod__ (b % a)
func floatRMod(a, b Object) (Object, error) {
	af, ok := a.(*FloatObject)
	if !ok {
		return NotImplemented, nil
	}
	if af.Value == 0 {
		return nil, fmt.Errorf("ZeroDivisionError: float modulo")
	}
	switch bi := b.(type) {
	case *IntObject:
		bf := intToFloat(bi)
		return FloatFrom(floatMod(bf, af.Value)), nil
	case *BoolObject:
		bf := boolToFloat(bi)
		return FloatFrom(floatMod(bf, af.Value)), nil
	}
	return NotImplemented, nil
}

// floatRPow implements float.__rpow__ (b ** a)
func floatRPow(a, b Object) (Object, error) {
	af, ok := a.(*FloatObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *IntObject:
		bf := intToFloat(bi)
		if bf == 0 && af.Value < 0 {
			return nil, fmt.Errorf("ZeroDivisionError: 0.0 cannot be raised to a negative power")
		}
		return FloatFrom(math.Pow(bf, af.Value)), nil
	case *BoolObject:
		bf := boolToFloat(bi)
		if bf == 0 && af.Value < 0 {
			return nil, fmt.Errorf("ZeroDivisionError: 0.0 cannot be raised to a negative power")
		}
		return FloatFrom(math.Pow(bf, af.Value)), nil
	}
	return NotImplemented, nil
}

// ============================================================
// Bitwise operator slot implementations (IntObject only)
// ============================================================

// intBitAnd implements int.__and__
func intBitAnd(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *IntObject:
		return IntFromBig(new(big.Int).And(ai.Value, bi.Value)), nil
	case *BoolObject:
		bv := boolToInt(bi)
		return IntFromBig(new(big.Int).And(ai.Value, bv)), nil
	}
	return NotImplemented, nil
}

// intBitOr implements int.__or__
func intBitOr(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *IntObject:
		return IntFromBig(new(big.Int).Or(ai.Value, bi.Value)), nil
	case *BoolObject:
		bv := boolToInt(bi)
		return IntFromBig(new(big.Int).Or(ai.Value, bv)), nil
	}
	return NotImplemented, nil
}

// intBitXor implements int.__xor__
func intBitXor(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	switch bi := b.(type) {
	case *IntObject:
		return IntFromBig(new(big.Int).Xor(ai.Value, bi.Value)), nil
	case *BoolObject:
		bv := boolToInt(bi)
		return IntFromBig(new(big.Int).Xor(ai.Value, bv)), nil
	}
	return NotImplemented, nil
}

// intLShift implements int.__lshift__
func intLShift(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	shift, err := toShiftAmount(b)
	if err != nil {
		return nil, err
	}
	if shift < 0 {
		return nil, fmt.Errorf("ValueError: negative shift count")
	}
	result := new(big.Int).Lsh(ai.Value, uint(shift))
	return IntFromBig(result), nil
}

// intRShift implements int.__rshift__
func intRShift(a, b Object) (Object, error) {
	ai, ok := a.(*IntObject)
	if !ok {
		return NotImplemented, nil
	}
	shift, err := toShiftAmount(b)
	if err != nil {
		return nil, err
	}
	if shift < 0 {
		return nil, fmt.Errorf("ValueError: negative shift count")
	}
	result := new(big.Int).Rsh(ai.Value, uint(shift))
	return IntFromBig(result), nil
}

// toShiftAmount extracts a non-negative shift amount from an Object.
func toShiftAmount(b Object) (int64, error) {
	switch bi := b.(type) {
	case *IntObject:
		if !bi.Value.IsInt64() {
			return 0, fmt.Errorf("OverflowError: shift count too large")
		}
		return bi.Value.Int64(), nil
	case *BoolObject:
		if bi.Value {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("TypeError: unsupported operand type(s) for shift: '%s'",
			b.Type().Name())
	}
}

// ============================================================
// Unary operator slot implementations
// ============================================================

// intNeg implements int.__neg__
func intNeg(self Object) (Object, error) {
	ai, ok := self.(*IntObject)
	if !ok {
		return nil, fmt.Errorf("TypeError: bad operand type for unary -: '%s'", self.Type().Name())
	}
	return IntFromBig(new(big.Int).Neg(ai.Value)), nil
}

// intPos implements int.__pos__
func intPos(self Object) (Object, error) {
	if _, ok := self.(*IntObject); !ok {
		return nil, fmt.Errorf("TypeError: bad operand type for unary +: '%s'", self.Type().Name())
	}
	return self, nil
}

// intInvert implements int.__invert__
func intInvert(self Object) (Object, error) {
	ai, ok := self.(*IntObject)
	if !ok {
		return nil, fmt.Errorf("TypeError: bad operand type for unary ~: '%s'", self.Type().Name())
	}
	// ~x == -(x+1) for integers
	result := new(big.Int).Not(ai.Value)
	return IntFromBig(result), nil
}

// floatNeg implements float.__neg__
func floatNeg(self Object) (Object, error) {
	af, ok := self.(*FloatObject)
	if !ok {
		return nil, fmt.Errorf("TypeError: bad operand type for unary -: '%s'", self.Type().Name())
	}
	return FloatFrom(-af.Value), nil
}

// floatPos implements float.__pos__
func floatPos(self Object) (Object, error) {
	if _, ok := self.(*FloatObject); !ok {
		return nil, fmt.Errorf("TypeError: bad operand type for unary +: '%s'", self.Type().Name())
	}
	return self, nil
}

// ============================================================
// Helper functions
// ============================================================

// boolToInt converts a BoolObject to a *big.Int (0 or 1).
func boolToInt(b *BoolObject) *big.Int {
	if b.Value {
		return big.NewInt(1)
	}
	return big.NewInt(0)
}

// boolToFloat converts a BoolObject to float64.
func boolToFloat(b *BoolObject) float64 {
	if b.Value {
		return 1.0
	}
	return 0.0
}

// intToFloat converts an IntObject to float64.
// Note: large integers may lose precision.
func intToFloat(i *IntObject) float64 {
	f, _ := new(big.Float).SetInt(i.Value).Float64()
	return f
}

// floorDivBig performs floor division on big.Int values.
// Python's // rounds toward negative infinity, unlike Go's Quo which truncates toward zero.
func floorDivBig(a, b *big.Int) *big.Int {
	quo := new(big.Int)
	r := new(big.Int)
	quo.QuoRem(a, b, r)
	// If remainder != 0 and signs of a and b differ, floor = quo - 1
	if r.Sign() != 0 && (a.Sign() < 0) != (b.Sign() < 0) {
		quo.Sub(quo, big.NewInt(1))
	}
	return quo
}

// floorModBig performs Python's modulo on big.Int values.
// Result sign matches the divisor.
func floorModBig(a, b *big.Int) *big.Int {
	// r = a - floor(a/b) * b
	quo := new(big.Int)
	r := new(big.Int)
	quo.QuoRem(a, b, r)
	// If remainder != 0 and signs differ, adjust
	if r.Sign() != 0 && (a.Sign() < 0) != (b.Sign() < 0) {
		r.Add(r, b)
	}
	return r
}

// floatMod implements Python's float modulo semantics.
// Result sign matches the divisor.
func floatMod(a, b float64) float64 {
	r := math.Mod(a, b)
	// If r != 0 and has different sign than b, adjust
	if r != 0 && ((r < 0) != (b < 0)) {
		r += b
	}
	return r
}

// ============================================================
// Slot registration
// ============================================================

// registerOperatorSlots registers arithmetic and bitwise operator slots
// on IntType and FloatType. Called from types.go init after types are created.
func registerOperatorSlots() {
	// Register IntType slots
	IntType.slots.Add = intAdd
	IntType.slots.Sub = intSub
	IntType.slots.Mul = intMul
	IntType.slots.Div = intTrueDiv
	IntType.slots.FloorDiv = intFloorDiv
	IntType.slots.Mod = intMod
	IntType.slots.Pow = intPow
	IntType.slots.Neg = intNeg
	IntType.slots.Pos = intPos
	IntType.slots.Invert = intInvert
	IntType.slots.BitAnd = intBitAnd
	IntType.slots.BitOr = intBitOr
	IntType.slots.BitXor = intBitXor
	IntType.slots.LShift = intLShift
	IntType.slots.RShift = intRShift
	IntType.slots.RAdd = intRAdd
	IntType.slots.RSub = intRSub
	IntType.slots.RMul = intRMul
	IntType.slots.RDiv = intRDiv
	IntType.slots.RFloorDiv = intRFloorDiv
	IntType.slots.RMod = intRMod
	IntType.slots.RPow = intRPow

	// Register FloatType slots
	FloatType.slots.Add = floatAdd
	FloatType.slots.Sub = floatSub
	FloatType.slots.Mul = floatMulOp
	FloatType.slots.Div = floatTrueDiv
	FloatType.slots.FloorDiv = floatFloorDivOp
	FloatType.slots.Mod = floatModOp
	FloatType.slots.Pow = floatPowOp
	FloatType.slots.Neg = floatNeg
	FloatType.slots.Pos = floatPos
	FloatType.slots.RAdd = floatRAdd
	FloatType.slots.RSub = floatRSub
	FloatType.slots.RMul = floatRMul
	FloatType.slots.RDiv = floatRDiv
	FloatType.slots.RFloorDiv = floatRFloorDiv
	FloatType.slots.RMod = floatRMod
	FloatType.slots.RPow = floatRPow

	// Register StrType slots
	registerStringSlots()

	// Register collection type slots (list, tuple)
	registerCollectionSlots()
}
