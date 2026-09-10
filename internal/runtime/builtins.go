package runtime

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/alvinjaison/pygo/internal/object"
)

// builtinLen implements Python's len(obj).
// Delegates to the LenProvider interface.
func (r *Runtime) builtinLen(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("len() takes exactly one argument (%d given)", len(args))
	}
	lp, ok := args[0].(object.LenProvider)
	if !ok {
		return nil, object.NewTypeError("object of type '%s' has no len()", args[0].Type().Name())
	}
	return object.IntFrom(int64(lp.Len())), nil
}

// builtinRange implements Python's range(stop) / range(start, stop) / range(start, stop, step).
func (r *Runtime) builtinRange(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	var start, stop, step int64

	switch len(args) {
	case 1:
		s, err := toInt64(args[0], "range")
		if err != nil {
			return nil, err
		}
		start, stop, step = 0, s, 1
	case 2:
		s, err := toInt64(args[0], "range")
		if err != nil {
			return nil, err
		}
		e, err := toInt64(args[1], "range")
		if err != nil {
			return nil, err
		}
		start, stop, step = s, e, 1
	case 3:
		s, err := toInt64(args[0], "range")
		if err != nil {
			return nil, err
		}
		e, err := toInt64(args[1], "range")
		if err != nil {
			return nil, err
		}
		st, err := toInt64(args[2], "range")
		if err != nil {
			return nil, err
		}
		start, stop, step = s, e, st
	default:
		return nil, object.NewTypeError("range expected at most 3 arguments, got %d", len(args))
	}

	rng, err := object.NewRange(start, stop, step)
	if err != nil {
		return nil, object.NewValueError("%s", err.Error())
	}
	return rng, nil
}

// builtinInt implements Python's int(x) type conversion.
func (r *Runtime) builtinInt(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if len(args) == 0 {
		return object.IntFrom(0), nil
	}
	if len(args) != 1 {
		return nil, object.NewTypeError("int() takes at most 1 argument (%d given)", len(args))
	}

	arg := args[0]
	switch v := arg.(type) {
	case *object.IntObject:
		return v, nil
	case *object.BoolObject:
		if v.Value {
			return object.IntFrom(1), nil
		}
		return object.IntFrom(0), nil
	case *object.FloatObject:
		// Truncate toward zero (like Python's int(float))
		if math.IsInf(v.Value, 0) || math.IsNaN(v.Value) {
			return nil, object.NewValueError("cannot convert float %s to integer", v.String())
		}
		truncated := math.Trunc(v.Value)
		// Check if it fits in int64
		if truncated >= math.MinInt64 && truncated <= math.MaxInt64 {
			return object.IntFrom(int64(truncated)), nil
		}
		// Use big.Float for large values
		bf := new(big.Float).SetFloat64(truncated)
		bi, _ := bf.Int(nil)
		return object.IntFromBig(bi), nil
	case *object.StrObject:
		s := strings.TrimSpace(v.Value)
		// Try parsing as integer
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, object.NewValueError("invalid literal for int() with base 10: %s", v.Repr())
		}
		return object.IntFromBig(n), nil
	default:
		return nil, object.NewTypeError("int() argument must be a string or a number, not '%s'", arg.Type().Name())
	}
}

// builtinFloat implements Python's float(x) type conversion.
func (r *Runtime) builtinFloat(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if len(args) == 0 {
		return object.FloatFrom(0.0), nil
	}
	if len(args) != 1 {
		return nil, object.NewTypeError("float() takes at most 1 argument (%d given)", len(args))
	}

	arg := args[0]
	switch v := arg.(type) {
	case *object.FloatObject:
		return v, nil
	case *object.IntObject:
		f, _ := new(big.Float).SetInt(v.Value).Float64()
		return object.FloatFrom(f), nil
	case *object.BoolObject:
		if v.Value {
			return object.FloatFrom(1.0), nil
		}
		return object.FloatFrom(0.0), nil
	case *object.StrObject:
		s := strings.TrimSpace(v.Value)
		// Handle special cases
		lower := strings.ToLower(s)
		switch lower {
		case "inf", "+inf", "infinity", "+infinity":
			return object.FloatFrom(math.Inf(1)), nil
		case "-inf", "-infinity":
			return object.FloatFrom(math.Inf(-1)), nil
		case "nan":
			return object.FloatFrom(math.NaN()), nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, object.NewValueError("could not convert string to float: %s", v.Repr())
		}
		return object.FloatFrom(f), nil
	default:
		return nil, object.NewTypeError("float() argument must be a string or a number, not '%s'", arg.Type().Name())
	}
}

// builtinStr implements Python's str(x) type conversion.
func (r *Runtime) builtinStr(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if len(args) == 0 {
		return object.StrFrom(""), nil
	}
	if len(args) != 1 {
		return nil, object.NewTypeError("str() takes at most 1 argument (%d given)", len(args))
	}
	return object.StrFrom(args[0].String()), nil
}

// builtinBool implements Python's bool(x) type conversion.
func (r *Runtime) builtinBool(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if len(args) == 0 {
		return object.PyFalse, nil
	}
	if len(args) != 1 {
		return nil, object.NewTypeError("bool() takes at most 1 argument (%d given)", len(args))
	}
	truth, err := object.IsTruthy(args[0])
	if err != nil {
		return nil, err
	}
	return object.BoolFrom(truth), nil
}

// builtinIsinstance implements Python's isinstance(obj, classinfo).
func (r *Runtime) builtinIsinstance(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, object.NewTypeError("isinstance() takes exactly 2 arguments (%d given)", len(args))
	}

	obj := args[0]
	classinfo := args[1]

	// classinfo can be a type or a tuple of types
	switch ci := classinfo.(type) {
	case *object.TypeObject:
		return object.BoolFrom(obj.Type().IsSubtype(ci)), nil
	case *object.TupleObject:
		for _, item := range ci.Items {
			typ := resolveToType(item)
			if typ == nil {
				return nil, object.NewTypeError("isinstance() arg 2 must be a type or tuple of types")
			}
			if obj.Type().IsSubtype(typ) {
				return object.PyTrue, nil
			}
		}
		return object.PyFalse, nil
	default:
		// Try to resolve a builtin function to its corresponding type
		typ := resolveToType(classinfo)
		if typ != nil {
			return object.BoolFrom(obj.Type().IsSubtype(typ)), nil
		}
		return nil, object.NewTypeError("isinstance() arg 2 must be a type or tuple of types")
	}
}

// resolveToType tries to convert an Object to a *TypeObject.
// Handles both direct TypeObjects and BuiltinFuncObjects named after types.
func resolveToType(obj object.Object) *object.TypeObject {
	if t, ok := obj.(*object.TypeObject); ok {
		return t
	}
	// Map builtin conversion functions to their type objects
	if bf, ok := obj.(*object.BuiltinFuncObject); ok {
		switch bf.Name {
		case "int":
			return object.IntType
		case "float":
			return object.FloatType
		case "str":
			return object.StrType
		case "bool":
			return object.BoolType
		}
	}
	return nil
}

// builtinRepr implements Python's repr(obj).
func (r *Runtime) builtinRepr(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("repr() takes exactly one argument (%d given)", len(args))
	}
	return object.StrFrom(args[0].Repr()), nil
}

// builtinAbs implements Python's abs(x).
func (r *Runtime) builtinAbs(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("abs() takes exactly one argument (%d given)", len(args))
	}

	arg := args[0]
	switch v := arg.(type) {
	case *object.IntObject:
		if v.Value.Sign() >= 0 {
			return v, nil
		}
		return object.IntFromBig(new(big.Int).Abs(v.Value)), nil
	case *object.FloatObject:
		return object.FloatFrom(math.Abs(v.Value)), nil
	case *object.BoolObject:
		// bool is a subtype of int: abs(True) == 1, abs(False) == 0
		if v.Value {
			return object.IntFrom(1), nil
		}
		return object.IntFrom(0), nil
	default:
		return nil, object.NewTypeError("bad operand type for abs(): '%s'", arg.Type().Name())
	}
}

// builtinMin implements Python's min(*args).
func (r *Runtime) builtinMin(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	items, err := resolveMinMaxArgs("min", args)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, object.NewValueError("min() arg is an empty sequence")
	}

	result := items[0]
	for _, item := range items[1:] {
		lt, err := isLessThan(item, result)
		if err != nil {
			return nil, err
		}
		if lt {
			result = item
		}
	}
	return result, nil
}

// builtinMax implements Python's max(*args).
func (r *Runtime) builtinMax(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	items, err := resolveMinMaxArgs("max", args)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, object.NewValueError("max() arg is an empty sequence")
	}

	result := items[0]
	for _, item := range items[1:] {
		gt, err := isGreaterThan(item, result)
		if err != nil {
			return nil, err
		}
		if gt {
			result = item
		}
	}
	return result, nil
}

// ============================================================
// Helper functions
// ============================================================

// toInt64 extracts an int64 from an Object for range() arguments.
func toInt64(obj object.Object, funcName string) (int64, error) {
	switch v := obj.(type) {
	case *object.IntObject:
		val, ok := v.Int64()
		if !ok {
			return 0, object.NewValueError("%s() argument too large", funcName)
		}
		return val, nil
	case *object.BoolObject:
		if v.Value {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, object.NewTypeError("'%s' object cannot be interpreted as an integer", obj.Type().Name())
	}
}

// resolveMinMaxArgs handles both min(a, b, c) and min([a, b, c]) forms.
func resolveMinMaxArgs(funcName string, args []object.Object) ([]object.Object, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("TypeError: %s expected at least 1 argument, got 0", funcName)
	}

	if len(args) == 1 {
		// Single arg: must be iterable
		iter, err := object.GetIter(args[0])
		if err != nil {
			return nil, err
		}
		var items []object.Object
		for {
			item, err := iter.Next()
			if err != nil {
				if object.IsStopIteration(err) {
					break
				}
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	}

	// Multiple args: compare them directly
	return args, nil
}

// isLessThan returns true if a < b using RichCompare.
func isLessThan(a, b object.Object) (bool, error) {
	if c, ok := a.(object.Comparable); ok {
		result, err := c.RichCompare(b, object.OpLt)
		if err != nil {
			return false, err
		}
		if !object.IsNotImplemented(result) {
			return object.IsTruthy(result)
		}
	}
	if c, ok := b.(object.Comparable); ok {
		result, err := c.RichCompare(a, object.OpGt)
		if err != nil {
			return false, err
		}
		if !object.IsNotImplemented(result) {
			return object.IsTruthy(result)
		}
	}
	return false, fmt.Errorf("TypeError: '<' not supported between instances of '%s' and '%s'",
		a.Type().Name(), b.Type().Name())
}

// isGreaterThan returns true if a > b using RichCompare.
func isGreaterThan(a, b object.Object) (bool, error) {
	if c, ok := a.(object.Comparable); ok {
		result, err := c.RichCompare(b, object.OpGt)
		if err != nil {
			return false, err
		}
		if !object.IsNotImplemented(result) {
			return object.IsTruthy(result)
		}
	}
	if c, ok := b.(object.Comparable); ok {
		result, err := c.RichCompare(a, object.OpLt)
		if err != nil {
			return false, err
		}
		if !object.IsNotImplemented(result) {
			return object.IsTruthy(result)
		}
	}
	return false, fmt.Errorf("TypeError: '>' not supported between instances of '%s' and '%s'",
		a.Type().Name(), b.Type().Name())
}
