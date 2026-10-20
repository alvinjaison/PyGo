package runtime

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/pygo/pygo/internal/object"
)

// installBuiltins populates the runtime's builtin namespace with core
// functions and type constructors. In restricted mode, I/O-capable builtins
// are omitted.
func (r *Runtime) installBuiltins() {
	reg := func(name string, fn func([]object.Object, map[string]object.Object) (object.Object, error)) {
		_ = r.builtins.SetItem(object.StrFrom(name), object.NewBuiltin(name, fn))
	}

	reg("print", r.biPrint)
	reg("len", biLen)
	reg("range", biRange)
	reg("abs", biAbs)
	reg("min", biMin)
	reg("max", biMax)
	reg("sum", biSum)
	reg("repr", biRepr)
	reg("str", biStr)
	reg("int", biInt)
	reg("float", biFloat)
	reg("bool", biBool)
	reg("list", biList)
	reg("tuple", biTuple)
	reg("type", biType)
	reg("isinstance", biIsInstance)
	reg("chr", biChr)
	reg("ord", biOrd)
	reg("enumerate", biEnumerate)
	reg("__build_class__", r.biBuildClass)

	// Expose core exception types as builtins so `raise ValueError(...)` works.
	for _, t := range []*object.TypeObject{
		object.TypeValueError, object.TypeTypeError, object.TypeNameError,
		object.TypeAttributeError, object.TypeIndexError, object.TypeKeyError,
		object.TypeZeroDivisionError, object.TypeRuntimeError, object.TypeStopIteration,
		object.TypeAssertionError, object.TypeNotImplementedError, object.TypeOverflowError,
		object.ExceptionType, object.BaseExceptionType,
	} {
		_ = r.builtins.SetItem(object.StrFrom(t.Name()), t)
	}

	// Constants.
	_ = r.builtins.SetItem(object.StrFrom("True"), object.PyTrue)
	_ = r.builtins.SetItem(object.StrFrom("False"), object.PyFalse)
	_ = r.builtins.SetItem(object.StrFrom("None"), object.None)
}

// print(*args, sep=' ', end='\n')
func (r *Runtime) biPrint(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	sep := " "
	end := "\n"
	if v, ok := kwargs["sep"]; ok {
		if s, ok := v.(*object.StrObject); ok {
			sep = s.Value
		}
	}
	if v, ok := kwargs["end"]; ok {
		if s, ok := v.(*object.StrObject); ok {
			end = s.Value
		}
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = a.String()
	}
	fmt.Fprint(r.stdout, strings.Join(parts, sep)+end)
	return object.None, nil
}

func biLen(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("len() takes exactly one argument (%d given)", len(args))
	}
	if lp, ok := args[0].(object.LenProvider); ok {
		return object.IntFrom(int64(lp.Len())), nil
	}
	return nil, object.NewTypeError("object of type '%s' has no len()", args[0].Type().Name())
}

func biAbs(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("abs() takes exactly one argument")
	}
	switch n := args[0].(type) {
	case *object.IntObject:
		return object.IntFromBig(new(big.Int).Abs(n.Value)), nil
	case *object.FloatObject:
		if n.Value < 0 {
			return object.FloatFrom(-n.Value), nil
		}
		return n, nil
	case *object.BoolObject:
		if n.Value {
			return object.IntFrom(1), nil
		}
		return object.IntFrom(0), nil
	}
	return nil, object.NewTypeError("bad operand type for abs(): '%s'", args[0].Type().Name())
}

func biRepr(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("repr() takes exactly one argument")
	}
	return object.StrFrom(args[0].Repr()), nil
}

func biStr(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) == 0 {
		return object.StrFrom(""), nil
	}
	return object.StrFrom(args[0].String()), nil
}

func biInt(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) == 0 {
		return object.IntFrom(0), nil
	}
	switch v := args[0].(type) {
	case *object.IntObject:
		return v, nil
	case *object.BoolObject:
		if v.Value {
			return object.IntFrom(1), nil
		}
		return object.IntFrom(0), nil
	case *object.FloatObject:
		return object.IntFrom(int64(v.Value)), nil
	case *object.StrObject:
		s := strings.TrimSpace(v.Value)
		bi := new(big.Int)
		if _, ok := bi.SetString(s, 10); !ok {
			return nil, object.NewValueError("invalid literal for int() with base 10: '%s'", v.Value)
		}
		return object.IntFromBig(bi), nil
	}
	return nil, object.NewTypeError("int() argument must be a string or a number, not '%s'", args[0].Type().Name())
}

func biFloat(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) == 0 {
		return object.FloatFrom(0), nil
	}
	switch v := args[0].(type) {
	case *object.FloatObject:
		return v, nil
	case *object.IntObject:
		f := new(big.Float).SetInt(v.Value)
		out, _ := f.Float64()
		return object.FloatFrom(out), nil
	case *object.StrObject:
		var out float64
		if _, err := fmt.Sscanf(strings.TrimSpace(v.Value), "%g", &out); err != nil {
			return nil, object.NewValueError("could not convert string to float: '%s'", v.Value)
		}
		return object.FloatFrom(out), nil
	}
	return nil, object.NewTypeError("float() argument must be a string or a number")
}

func biBool(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) == 0 {
		return object.PyFalse, nil
	}
	t, err := object.IsTruthy(args[0])
	if err != nil {
		return nil, err
	}
	return object.BoolFrom(t), nil
}

func biList(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) == 0 {
		return object.NewList(nil), nil
	}
	items, err := iterateAll(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewList(items), nil
}

func biTuple(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) == 0 {
		return object.NewTuple(nil), nil
	}
	items, err := iterateAll(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewTuple(items), nil
}

func biType(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("type() takes 1 argument")
	}
	return args[0].Type(), nil
}

func biIsInstance(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, object.NewTypeError("isinstance expected 2 arguments")
	}
	obj := args[0]
	switch cls := args[1].(type) {
	case *object.TypeObject:
		return object.BoolFrom(obj.Type().IsSubtype(cls)), nil
	case *object.TupleObject:
		for _, item := range cls.Items {
			if t, ok := item.(*object.TypeObject); ok && obj.Type().IsSubtype(t) {
				return object.PyTrue, nil
			}
		}
		return object.PyFalse, nil
	}
	return nil, object.NewTypeError("isinstance() arg 2 must be a type or tuple of types")
}

func biChr(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	n, ok := args[0].(*object.IntObject)
	if !ok {
		return nil, object.NewTypeError("chr() requires an int")
	}
	code, _ := n.Int64()
	return object.StrFrom(string(rune(code))), nil
}

func biOrd(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	s, ok := args[0].(*object.StrObject)
	if !ok {
		return nil, object.NewTypeError("ord() requires a string")
	}
	runes := []rune(s.Value)
	if len(runes) != 1 {
		return nil, object.NewTypeError("ord() expected a character, but string of length %d found", len(runes))
	}
	return object.IntFrom(int64(runes[0])), nil
}

func biMin(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	return minMax(args, true)
}

func biMax(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	return minMax(args, false)
}

func minMax(args []object.Object, wantMin bool) (object.Object, error) {
	var items []object.Object
	if len(args) == 1 {
		var err error
		items, err = iterateAll(args[0])
		if err != nil {
			return nil, err
		}
	} else {
		items = args
	}
	if len(items) == 0 {
		return nil, object.NewValueError("min()/max() arg is an empty sequence")
	}
	best := items[0]
	for _, it := range items[1:] {
		op := object.OpLt
		if !wantMin {
			op = object.OpGt
		}
		res, err := object.CompareObjects(it, best, cmpToBytecode(op))
		if err != nil {
			return nil, err
		}
		if t, _ := object.IsTruthy(res); t {
			best = it
		}
	}
	return best, nil
}

func biSum(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) == 0 {
		return nil, object.NewTypeError("sum() takes at least 1 argument")
	}
	items, err := iterateAll(args[0])
	if err != nil {
		return nil, err
	}
	var acc object.Object = object.IntFrom(0)
	if len(args) > 1 {
		acc = args[1]
	}
	for _, it := range items {
		acc, err = object.BinaryOp(acc, it, "+")
		if err != nil {
			return nil, err
		}
	}
	return acc, nil
}

func biEnumerate(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) < 1 {
		return nil, object.NewTypeError("enumerate() takes at least 1 argument")
	}
	items, err := iterateAll(args[0])
	if err != nil {
		return nil, err
	}
	start := int64(0)
	if len(args) > 1 {
		if n, ok := args[1].(*object.IntObject); ok {
			start, _ = n.Int64()
		}
	}
	out := make([]object.Object, len(items))
	for i, it := range items {
		out[i] = object.NewTuple([]object.Object{object.IntFrom(start + int64(i)), it})
	}
	return object.NewList(out), nil
}

func biRange(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	var start, stop, step int64 = 0, 0, 1
	switch len(args) {
	case 1:
		stop = mustInt(args[0])
	case 2:
		start = mustInt(args[0])
		stop = mustInt(args[1])
	case 3:
		start = mustInt(args[0])
		stop = mustInt(args[1])
		step = mustInt(args[2])
	default:
		return nil, object.NewTypeError("range expected 1 to 3 arguments, got %d", len(args))
	}
	if step == 0 {
		return nil, object.NewValueError("range() arg 3 must not be zero")
	}
	var items []object.Object
	if step > 0 {
		for i := start; i < stop; i += step {
			items = append(items, object.IntFrom(i))
		}
	} else {
		for i := start; i > stop; i += step {
			items = append(items, object.IntFrom(i))
		}
	}
	return object.NewList(items), nil
}

func mustInt(o object.Object) int64 {
	if n, ok := o.(*object.IntObject); ok {
		v, _ := n.Int64()
		return v
	}
	return 0
}

// iterateAll drains an iterable into a slice.
func iterateAll(o object.Object) ([]object.Object, error) {
	it, err := object.GetIter(o)
	if err != nil {
		return nil, err
	}
	var items []object.Object
	for {
		v, err := it.Next()
		if object.IsStopIteration(err) {
			break
		}
		if err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, nil
}
