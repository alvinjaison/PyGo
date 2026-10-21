package object

import (
	"fmt"
	"math/big"

	"github.com/alvinjaison/PyGo/internal/bytecode"
)

// binaryOp dispatches a binary operation on two Python objects.
// It follows Python's left-first, then reflected-right protocol.
func BinaryOp(left, right Object, op string) (Object, error) {
	return binaryOp(left, right, op)
}

func binaryOp(left, right Object, op string) (Object, error) {
	// User-defined classes: dispatch through dunder methods first so Python
	// __add__/__radd__/etc. take precedence over built-in coercion.
	if _, ok := left.(*InstanceObject); ok {
		if res, handled, err := tryBinaryDunder(left, right, op); handled || err != nil {
			if err != nil {
				return nil, err
			}
			return res, nil
		}
	} else if _, ok := right.(*InstanceObject); ok {
		if res, handled, err := tryBinaryDunder(left, right, op); handled || err != nil {
			if err != nil {
				return nil, err
			}
			return res, nil
		}
	}

	// Try left operand's slot first.
	if result, err, ok := trySlotOp(left, right, op, false); ok {
		if !IsNotImplemented(result) {
			return result, err
		}
	}
	// Try right operand's reflected slot.
	if result, err, ok := trySlotOp(left, right, op, true); ok {
		if !IsNotImplemented(result) {
			return result, err
		}
	}
	return nil, NewTypeError("unsupported operand type(s) for %s: '%s' and '%s'",
		op, left.Type().Name(), right.Type().Name())
}

// trySlotOp attempts a numeric slot operation.
func trySlotOp(left, right Object, op string, reflected bool) (Object, error, bool) {
	var a, b Object
	if reflected {
		a, b = right, left
	} else {
		a, b = left, right
	}

	slot := getNumericSlot(a.Type(), op, reflected)
	if slot == nil {
		return nil, nil, false
	}
	result, err := slot(a, b)
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

// unaryOp dispatches a unary operation.
func UnaryOp(val Object, op string) (Object, error) {
	return unaryOp(val, op)
}

func unaryOp(val Object, op string) (Object, error) {
	// User-defined classes: dispatch through __neg__/__pos__/__invert__.
	if res, handled, err := tryUnaryDunder(val, op); handled || err != nil {
		if err != nil {
			return nil, err
		}
		return res, nil
	}

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
		return val, nil
	case "~":
		if s.Invert != nil {
			return s.Invert(val)
		}
	}
	// Fallback: integer unary ops on IntObject directly.
	if n, ok := val.(*IntObject); ok {
		switch op {
		case "-":
			return IntFromBig(new(big.Int).Neg(n.Value)), nil
		case "+":
			return val, nil
		case "~":
			result := new(big.Int).Not(n.Value)
			return IntFromBig(result), nil
		}
	}
	return nil, fmt.Errorf("TypeError: bad operand type for unary %s: '%s'", op, val.Type().Name())
}

// compareObjects performs a Python comparison operation.
func CompareObjects(left, right Object, op bytecode.CompareOp) (Object, error) {
	return compareObjects(left, right, op)
}

func compareObjects(left, right Object, op bytecode.CompareOp) (Object, error) {
	// Identity comparisons.
	if op == bytecode.CmpIs {
		return BoolFrom(left == right), nil
	}
	if op == bytecode.CmpIsNot {
		return BoolFrom(left != right), nil
	}

	// Membership tests.
	if op == bytecode.CmpIn || op == bytecode.CmpNotIn {
		container, item := right, left
		found, err := containsItem(container, item)
		if err != nil {
			return nil, err
		}
		if op == bytecode.CmpNotIn {
			found = !found
		}
		return BoolFrom(found), nil
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

	// User-defined classes: dispatch through rich-comparison dunder methods
	// first so Python __eq__/__lt__/etc. take precedence.
	if _, lok := left.(*InstanceObject); lok {
		if res, handled, err := tryCompareDunder(left, right, objOp); handled || err != nil {
			if err != nil {
				return nil, err
			}
			return res, nil
		}
	} else if _, rok := right.(*InstanceObject); rok {
		if res, handled, err := tryCompareDunder(left, right, objOp); handled || err != nil {
			if err != nil {
				return nil, err
			}
			return res, nil
		}
	}

	// Try left's comparison: type slot first, then the Comparable interface.
	if slot := left.Type().slots.RichCmp; slot != nil {
		result, err := slot(left, right, objOp)
		if err != nil {
			return nil, err
		}
		if !IsNotImplemented(result) {
			return result, nil
		}
	}
	if c, ok := left.(Comparable); ok {
		result, err := c.RichCompare(right, objOp)
		if err != nil {
			return nil, err
		}
		if !IsNotImplemented(result) {
			return result, nil
		}
	}

	// Try right's reflected comparison.
	reflected := reflectCompareOp(objOp)
	if slot := right.Type().slots.RichCmp; slot != nil {
		result, err := slot(right, left, reflected)
		if err != nil {
			return nil, err
		}
		if !IsNotImplemented(result) {
			return result, nil
		}
	}
	if c, ok := right.(Comparable); ok {
		result, err := c.RichCompare(left, reflected)
		if err != nil {
			return nil, err
		}
		if !IsNotImplemented(result) {
			return result, nil
		}
	}

	// Fall back to identity comparison for == and !=.
	switch objOp {
	case OpEq:
		return BoolFrom(left == right), nil
	case OpNotEq:
		return BoolFrom(left != right), nil
	}

	return nil, fmt.Errorf("TypeError: '%s' not supported between instances of '%s' and '%s'",
		objOp, left.Type().Name(), right.Type().Name())
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

// GetIter returns an iterator for obj, or a TypeError if obj is not iterable.
//
// For user-defined instances, __iter__ is consulted first (it may perform
// setup and return self), and only then the __next__-based iterator protocol.
// Built-in iterators (which implement Iterator directly) are returned as-is.
func GetIter(obj Object) (Iterator, error) {
	if inst, ok := obj.(*InstanceObject); ok {
		if hasDunder(inst, "__iter__") || hasDunder(inst, "__next__") {
			return inst.Iter()
		}
		if hasDunder(inst, "__getitem__") {
			return newSeqIterator(inst), nil
		}
		return nil, fmt.Errorf("TypeError: '%s' object is not iterable", obj.Type().Name())
	}
	if it, ok := obj.(Iterator); ok {
		return it, nil
	}
	if it, ok := obj.(Iterable); ok {
		return it.Iter()
	}
	return nil, fmt.Errorf("TypeError: '%s' object is not iterable", obj.Type().Name())
}

// MakeFunctionFlags is a bitmask for the MAKE_FUNCTION instruction.
type MakeFunctionFlags = uint32

const (
	MakeFuncHasDefaults    MakeFunctionFlags = 1 << 0
	MakeFuncHasKwDefaults  MakeFunctionFlags = 1 << 1
	MakeFuncHasAnnotations MakeFunctionFlags = 1 << 2
	MakeFuncHasClosure     MakeFunctionFlags = 1 << 3
)

// containsItem implements the `in` test, consulting a user __contains__, the
// type's Contains slot, the Container interface, and finally iteration.
func containsItem(container, item Object) (bool, error) {
	// User-defined classes: prefer __contains__, else fall through to the
	// iteration protocol below (so classes with only __iter__/__getitem__ work).
	if inst, ok := container.(*InstanceObject); ok {
		if inst.HasContains() {
			return inst.Contains(item)
		}
	} else {
		if slot := container.Type().slots.Contains; slot != nil {
			return slot(container, item)
		}
		if c, ok := container.(Container); ok {
			return c.Contains(item)
		}
	}
	// Fall back to iterating and comparing by equality.
	it, err := GetIter(container)
	if err != nil {
		return false, fmt.Errorf("TypeError: argument of type '%s' is not iterable", container.Type().Name())
	}
	for {
		v, err := it.Next()
		if err == ErrStopIteration {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		eq, err := objectsEqual(v, item)
		if err != nil {
			return false, err
		}
		if eq {
			return true, nil
		}
	}
}
