package runtime

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/alvinjaison/PyGo/internal/object"
)

// installBuiltins populates the runtime's builtin namespace with core
// functions and type constructors. In restricted mode, I/O-capable builtins
// are omitted.
func (r *Runtime) installBuiltins() {
	reg := func(name string, fn func([]object.Object, map[string]object.Object) (object.Object, error)) {
		_ = r.builtins.SetItem(object.StrFrom(name), object.NewBuiltin(name, fn))
	}

	// regType registers a type constructor. The resulting callable stands for
	// its type in isinstance() and class patterns.
	regType := func(name string, class *object.TypeObject, fn func([]object.Object, map[string]object.Object) (object.Object, error)) {
		b := object.NewBuiltin(name, fn)
		b.Class = class
		_ = r.builtins.SetItem(object.StrFrom(name), b)
	}
	regType("str", object.StrType, biStr)
	regType("int", object.IntType, biInt)
	regType("float", object.FloatType, biFloat)
	regType("bool", object.BoolType, biBool)
	regType("list", object.ListType, biList)
	regType("tuple", object.TupleType, biTuple)
	regType("bytes", object.BytesType, biBytes)
	regType("range", object.RangeType, biRange)
	regType("dict", object.DictType, biDict)
	regType("set", object.SetType, biSet)

	reg("print", r.biPrint)
	reg("len", biLen)
	reg("abs", biAbs)
	reg("round", biRound)
	reg("min", biMin)
	reg("max", biMax)
	reg("sum", biSum)
	reg("sorted", biSorted)
	reg("reversed", biReversed)
	reg("any", biAny)
	reg("all", biAll)
	reg("repr", biRepr)
	reg("type", biType)
	reg("isinstance", biIsInstance)
	reg("chr", biChr)
	reg("ord", biOrd)
	reg("enumerate", biEnumerate)
	reg("property", biProperty)
	reg("classmethod", biClassMethod)
	reg("staticmethod", biStaticMethod)
	reg("super", biSuper)
	reg("hash", biHash)
	reg("iter", biIter)
	reg("next", biNext)
	reg("getattr", biGetAttr)
	reg("setattr", biSetAttr)
	reg("hasattr", biHasAttr)
	reg("__build_class__", r.biBuildClass)

	// Expose core exception types as builtins so `raise ValueError(...)` works.
	for _, t := range []*object.TypeObject{
		object.TypeValueError, object.TypeTypeError, object.TypeNameError,
		object.TypeAttributeError, object.TypeIndexError, object.TypeKeyError,
		object.TypeZeroDivisionError, object.TypeRuntimeError, object.TypeStopIteration,
		object.TypeAssertionError, object.TypeNotImplementedError, object.TypeOverflowError,
		object.TypeOSError, object.TypeImportError, object.TypeModuleNotFoundError,
		object.TypeKeyboardInterrupt, object.TypeSystemExit, object.TypeUnboundLocalError,
		object.TypeRecursionError, object.TypeGeneratorExit,
		object.ExceptionType, object.BaseExceptionType,
	} {
		_ = r.builtins.SetItem(object.StrFrom(t.Name()), t)
	}
	// IOError is an alias for OSError in Python 3.
	_ = r.builtins.SetItem(object.StrFrom("IOError"), object.TypeOSError)

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
	// Instances only have a length if they define __len__.
	if inst, ok := args[0].(*object.InstanceObject); ok {
		if !inst.HasLen() {
			return nil, object.NewTypeError("object of type '%s' has no len()", args[0].Type().Name())
		}
		return object.IntFrom(int64(inst.Len())), nil
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
	// User-defined __abs__.
	if inst, ok := args[0].(*object.InstanceObject); ok {
		if m, found := inst.Dunder("__abs__"); found {
			return object.Invoke(m, nil)
		}
		return nil, object.NewTypeError("bad operand type for abs(): '%s'", args[0].Type().Name())
	}
	switch n := args[0].(type) {
	case *object.IntObject:
		return object.IntFromBig(new(big.Int).Abs(n.Big())), nil
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
		f := new(big.Float).SetInt(v.Big())
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

// biDict implements dict(), dict(mapping), dict(iterable_of_pairs) and
// dict(**kwargs).
func biDict(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if len(args) > 1 {
		return nil, object.NewTypeError("dict expected at most 1 argument, got %d", len(args))
	}
	d := object.NewDict()
	if len(args) == 1 {
		if src, ok := args[0].(*object.DictObject); ok {
			for _, k := range src.Keys() {
				v, _ := src.GetItem(k)
				if err := d.SetItem(k, v); err != nil {
					return nil, err
				}
			}
		} else {
			items, err := iterateAll(args[0])
			if err != nil {
				return nil, err
			}
			for _, item := range items {
				pair, err := iterateAll(item)
				if err != nil || len(pair) != 2 {
					return nil, object.NewValueError("dictionary update sequence element must have length 2")
				}
				if err := d.SetItem(pair[0], pair[1]); err != nil {
					return nil, err
				}
			}
		}
	}
	// Keyword order is not preserved by the call machinery, so sort for a
	// deterministic result.
	names := make([]string, 0, len(kwargs))
	for k := range kwargs {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		if err := d.SetItem(object.StrFrom(k), kwargs[k]); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// biSet implements set() and set(iterable).
func biSet(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) > 1 {
		return nil, object.NewTypeError("set expected at most 1 argument, got %d", len(args))
	}
	if len(args) == 0 {
		return object.NewSet(), nil
	}
	items, err := iterateAll(args[0])
	if err != nil {
		return nil, err
	}
	return object.NewSetFrom(items)
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
	if tup, ok := args[1].(*object.TupleObject); ok {
		for _, item := range tup.Items {
			is, usable := object.IsInstanceOfClass(obj, item)
			if !usable {
				return nil, object.NewTypeError("isinstance() arg 2 must be a type or tuple of types")
			}
			if is {
				return object.PyTrue, nil
			}
		}
		return object.PyFalse, nil
	}
	is, usable := object.IsInstanceOfClass(obj, args[1])
	if !usable {
		return nil, object.NewTypeError("isinstance() arg 2 must be a type or tuple of types")
	}
	return object.BoolFrom(is), nil
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

func biMin(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	return minMax("min", args, kwargs, true)
}

func biMax(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	return minMax("max", args, kwargs, false)
}

func minMax(name string, args []object.Object, kwargs map[string]object.Object, wantMin bool) (object.Object, error) {
	var key, def object.Object
	hasDefault := false
	for k, v := range kwargs {
		switch k {
		case "key":
			key = v
		case "default":
			def, hasDefault = v, true
		default:
			return nil, object.NewTypeError("%s() got an unexpected keyword argument '%s'", name, k)
		}
	}
	if len(args) == 0 {
		return nil, object.NewTypeError("%s expected at least 1 argument, got 0", name)
	}
	var items []object.Object
	if len(args) == 1 {
		var err error
		items, err = iterateAll(args[0])
		if err != nil {
			return nil, err
		}
	} else {
		if hasDefault {
			return nil, object.NewTypeError("Cannot specify a default for %s() with multiple positional arguments", name)
		}
		items = args
	}
	if len(items) == 0 {
		if hasDefault {
			return def, nil
		}
		return nil, object.NewValueError("%s() iterable argument is empty", name)
	}
	keyOf := func(o object.Object) (object.Object, error) {
		if key == nil || object.IsNone(key) {
			return o, nil
		}
		return object.Invoke(key, []object.Object{o})
	}
	best := items[0]
	bestKey, err := keyOf(best)
	if err != nil {
		return nil, err
	}
	op := object.OpLt
	if !wantMin {
		op = object.OpGt
	}
	for _, it := range items[1:] {
		k, err := keyOf(it)
		if err != nil {
			return nil, err
		}
		res, err := object.CompareObjects(k, bestKey, cmpToBytecode(op))
		if err != nil {
			return nil, err
		}
		if t, _ := object.IsTruthy(res); t {
			best, bestKey = it, k
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
	nums := make([]int64, len(args))
	for i, a := range args {
		n, err := rangeArg(a)
		if err != nil {
			return nil, err
		}
		nums[i] = n
	}
	switch len(args) {
	case 1:
		stop = nums[0]
	case 2:
		start, stop = nums[0], nums[1]
	case 3:
		start, stop, step = nums[0], nums[1], nums[2]
	default:
		return nil, object.NewTypeError("range expected 1 to 3 arguments, got %d", len(args))
	}
	if step == 0 {
		return nil, object.NewValueError("range() arg 3 must not be zero")
	}
	return object.NewRange(start, stop, step), nil
}

// rangeArg converts a range() argument to int64, rejecting non-integers.
func rangeArg(o object.Object) (int64, error) {
	switch v := o.(type) {
	case *object.IntObject:
		if n, ok := v.Int64(); ok {
			return n, nil
		}
		return 0, object.NewException(object.TypeOverflowError, "Python int too large to convert to C long")
	case *object.BoolObject:
		if v.Value {
			return 1, nil
		}
		return 0, nil
	}
	return 0, object.NewTypeError("'%s' object cannot be interpreted as an integer", o.Type().Name())
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

// round(x[, ndigits]) rounds to the nearest value, or to ndigits decimals.
func biRound(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) == 0 {
		return nil, object.NewTypeError("round() missing required argument")
	}
	ndigits := 0
	haveND := len(args) > 1
	if haveND {
		if n, ok := args[1].(*object.IntObject); ok {
			v, _ := n.Int64()
			ndigits = int(v)
		}
	}
	switch v := args[0].(type) {
	case *object.IntObject:
		return v, nil
	case *object.FloatObject:
		f := v.Value
		pow := 1.0
		for i := 0; i < absInt(ndigits); i++ {
			pow *= 10
		}
		if ndigits < 0 {
			pow = 1 / pow
		}
		rounded := bankersRound(f*pow) / pow
		if !haveND {
			return object.IntFrom(int64(rounded)), nil
		}
		return object.FloatFrom(rounded), nil
	}
	return nil, object.NewTypeError("type %s doesn't define __round__", args[0].Type().Name())
}

// bankersRound implements round-half-to-even (Python's rounding mode).
func bankersRound(x float64) float64 {
	floor := float64(int64(x))
	diff := x - floor
	switch {
	case diff > 0.5:
		return floor + 1
	case diff < 0.5:
		return floor
	default:
		// exactly .5 → round to even
		if int64(floor)%2 == 0 {
			return floor
		}
		return floor + 1
	}
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func biSorted(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("sorted expected 1 argument, got %d", len(args))
	}
	items, err := iterateAll(args[0])
	if err != nil {
		return nil, err
	}
	key, reverse, err := object.SortOptions(kwargs)
	if err != nil {
		return nil, err
	}
	out := append([]object.Object{}, items...) // never mutate the source
	if err := object.SortObjects(out, key, reverse); err != nil {
		return nil, err
	}
	return object.NewList(out), nil
}

func biReversed(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	items, err := iterateAll(args[0])
	if err != nil {
		return nil, err
	}
	out := make([]object.Object, len(items))
	for i, it := range items {
		out[len(items)-1-i] = it
	}
	return object.NewList(out), nil
}

func biAny(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	return anyAll(args, true)
}

func biAll(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	return anyAll(args, false)
}

// anyAll implements any() (wantTrue) and all() lazily: it pulls items one at a
// time and stops at the first deciding element, so it works on infinite
// iterators and generators.
func anyAll(args []object.Object, wantTrue bool) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("expected 1 argument, got %d", len(args))
	}
	it, err := object.GetIter(args[0])
	if err != nil {
		return nil, err
	}
	for {
		v, err := it.Next()
		if object.IsStopIteration(err) {
			break
		}
		if err != nil {
			return nil, err
		}
		truthy, terr := object.IsTruthy(v)
		if terr != nil {
			return nil, terr
		}
		if wantTrue && truthy {
			return object.PyTrue, nil
		}
		if !wantTrue && !truthy {
			return object.PyFalse, nil
		}
	}
	return object.BoolFrom(!wantTrue), nil
}

// sortStable is a simple insertion sort (stable) over Objects using less.

// property(fget[, fset]) — creates a property descriptor.
func biProperty(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	var getter, setter object.Object = object.None, object.None
	if len(args) > 0 {
		getter = args[0]
	}
	if len(args) > 1 {
		setter = args[1]
	}
	return object.NewProperty(getter, setter), nil
}

// classmethod(func) — wraps func as a classmethod descriptor.
func biClassMethod(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("classmethod expected 1 argument")
	}
	return object.NewClassMethod(args[0]), nil
}

// staticmethod(func) — wraps func as a staticmethod descriptor.
func biStaticMethod(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("staticmethod expected 1 argument")
	}
	return object.NewStaticMethod(args[0]), nil
}

// hash(obj) — returns the object's hash.
func biHash(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("hash() takes exactly one argument")
	}
	h, ok := args[0].(object.Hashable)
	if !ok {
		return nil, object.NewTypeError("unhashable type: '%s'", args[0].Type().Name())
	}
	v, err := h.Hash()
	if err != nil {
		return nil, err
	}
	return object.IntFrom(v), nil
}

// iter(obj) — returns an iterator over obj.
func biIter(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("iter() takes 1 argument")
	}
	it, err := object.GetIter(args[0])
	if err != nil {
		return nil, err
	}
	return it, nil
}

// next(iterator[, default]) — advances an iterator.
func biNext(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) == 0 {
		return nil, object.NewTypeError("next() takes at least 1 argument")
	}
	it, ok := args[0].(object.Iterator)
	if !ok {
		return nil, object.NewTypeError("'%s' object is not an iterator", args[0].Type().Name())
	}
	v, err := it.Next()
	if object.IsStopIteration(err) {
		if len(args) > 1 {
			return args[1], nil
		}
		return nil, object.NewException(object.TypeStopIteration, "")
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

// getattr(obj, name[, default]) — attribute access by name.
func biGetAttr(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) < 2 {
		return nil, object.NewTypeError("getattr expected at least 2 arguments")
	}
	name, ok := args[1].(*object.StrObject)
	if !ok {
		return nil, object.NewTypeError("attribute name must be string")
	}
	holder, ok := args[0].(object.AttributeHolder)
	if !ok {
		if len(args) > 2 {
			return args[2], nil
		}
		return nil, object.NewAttributeError(args[0].Type().Name(), name.Value)
	}
	v, err := holder.GetAttr(name.Value)
	if err != nil {
		if len(args) > 2 {
			return args[2], nil
		}
		return nil, err
	}
	return v, nil
}

// setattr(obj, name, value) — attribute assignment by name.
func biSetAttr(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 3 {
		return nil, object.NewTypeError("setattr expected 3 arguments")
	}
	name, ok := args[1].(*object.StrObject)
	if !ok {
		return nil, object.NewTypeError("attribute name must be string")
	}
	holder, ok := args[0].(object.AttributeHolder)
	if !ok {
		return nil, object.NewTypeError("'%s' object has no settable attributes", args[0].Type().Name())
	}
	if err := holder.SetAttr(name.Value, args[2]); err != nil {
		return nil, err
	}
	return object.None, nil
}

// hasattr(obj, name) — whether attribute access succeeds.
func biHasAttr(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, object.NewTypeError("hasattr expected 2 arguments")
	}
	name, ok := args[1].(*object.StrObject)
	if !ok {
		return nil, object.NewTypeError("attribute name must be string")
	}
	holder, ok := args[0].(object.AttributeHolder)
	if !ok {
		return object.PyFalse, nil
	}
	if _, err := holder.GetAttr(name.Value); err != nil {
		return object.PyFalse, nil
	}
	return object.PyTrue, nil
}

// super([type, object]) returns a proxy that dispatches attribute access to
// the next class in object's MRO after type. The zero-argument form infers
// both from the executing method's frame.
func biSuper(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	switch len(args) {
	case 0:
		ctx := object.CurrentFrameContext()
		if ctx == nil || ctx.Self == nil || ctx.DefiningClass == nil {
			return nil, object.NewRuntimeError("super(): no arguments and no enclosing method context")
		}
		return object.NewSuper(ctx.DefiningClass, ctx.Self, ctx.Self.Type()), nil
	case 2:
		cls, ok := args[0].(*object.TypeObject)
		if !ok {
			return nil, object.NewTypeError("super() argument 1 must be a type")
		}
		return object.NewSuper(cls, args[1], args[1].Type()), nil
	default:
		return nil, object.NewTypeError("super() takes 0 or 2 arguments")
	}
}

// bytes(...) constructs a bytes object:
//
//	bytes()                     -> empty
//	bytes(int)                  -> that many zero bytes
//	bytes(str, encoding)        -> encoded (UTF-8 only)
//	bytes(iterable_of_ints)     -> those byte values
//	bytes(bytes)                -> a copy
func biBytes(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) == 0 {
		return object.NewBytes(nil), nil
	}
	switch v := args[0].(type) {
	case *object.BytesObject:
		return object.NewBytes(v.Value), nil
	case *object.IntObject:
		n, _ := v.Int64()
		if n < 0 {
			return nil, object.NewValueError("negative count")
		}
		return object.NewBytes(make([]byte, n)), nil
	case *object.StrObject:
		// Requires an encoding argument, matching Python.
		if len(args) < 2 {
			return nil, object.NewTypeError("string argument without an encoding")
		}
		return object.BytesFromString(v.Value), nil
	default:
		// Iterable of ints.
		items, err := iterateAll(args[0])
		if err != nil {
			return nil, object.NewTypeError("cannot convert '%s' object to bytes", args[0].Type().Name())
		}
		out := make([]byte, len(items))
		for i, it := range items {
			n, ok := it.(*object.IntObject)
			if !ok {
				return nil, object.NewTypeError("'%s' object cannot be interpreted as an integer", it.Type().Name())
			}
			iv, _ := n.Int64()
			if iv < 0 || iv > 255 {
				return nil, object.NewValueError("bytes must be in range(0, 256)")
			}
			out[i] = byte(iv)
		}
		return object.NewBytes(out), nil
	}
}
