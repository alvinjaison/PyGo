package runtime

import (
	"math/big"
	"reflect"
	"strings"

	"github.com/alvinjaison/PyGo/internal/object"
)

// installMoreBuiltins registers the second batch of core builtins: object,
// issubclass, callable, zip/map/filter, divmod/pow, bin/hex/oct, id, delattr
// and slice.
func (r *Runtime) installMoreBuiltins() {
	reg := func(name string, fn func([]object.Object, map[string]object.Object) (object.Object, error)) {
		_ = r.builtins.SetItem(object.StrFrom(name), object.NewBuiltin(name, fn))
	}
	obj := object.NewBuiltin("object", biObject)
	obj.Class = object.ObjectType
	_ = r.builtins.SetItem(object.StrFrom("object"), obj)

	reg("issubclass", biIsSubclass)
	reg("callable", biCallable)
	reg("zip", biZip)
	reg("map", biMap)
	reg("filter", biFilter)
	reg("divmod", biDivmod)
	reg("pow", biPow)
	reg("bin", func(a []object.Object, _ map[string]object.Object) (object.Object, error) {
		return biRadix(a, "bin", 2, "0b")
	})
	reg("oct", func(a []object.Object, _ map[string]object.Object) (object.Object, error) {
		return biRadix(a, "oct", 8, "0o")
	})
	reg("hex", func(a []object.Object, _ map[string]object.Object) (object.Object, error) {
		return biRadix(a, "hex", 16, "0x")
	})
	reg("id", biID)
	reg("delattr", biDelAttr)
	reg("slice", biSlice)
}

// lazyIter is a one-shot iterator backed by a Go function (zip, map, filter).
type lazyIter struct {
	name string
	next func() (object.Object, error)
}

func (l *lazyIter) Type() *object.TypeObject       { return object.ObjectType }
func (l *lazyIter) String() string                 { return "<" + l.name + " object>" }
func (l *lazyIter) Repr() string                   { return l.String() }
func (l *lazyIter) Iter() (object.Iterator, error) { return l, nil }
func (l *lazyIter) Next() (object.Object, error)   { return l.next() }

// iterators turns each argument into an iterator.
func iterators(objs []object.Object) ([]object.Iterator, error) {
	its := make([]object.Iterator, len(objs))
	for i, o := range objs {
		it, err := object.GetIter(o)
		if err != nil {
			return nil, err
		}
		its[i] = it
	}
	return its, nil
}

func biObject(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 0 {
		return nil, object.NewTypeError("object() takes no arguments")
	}
	return object.NewInstance(object.ObjectType), nil
}

func biIsSubclass(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, object.NewTypeError("issubclass expected 2 arguments")
	}
	sub, ok := object.AsClass(args[0])
	if !ok {
		return nil, object.NewTypeError("issubclass() arg 1 must be a class")
	}
	check := func(o object.Object) (bool, error) {
		parent, ok := object.AsClass(o)
		if !ok {
			return false, object.NewTypeError("issubclass() arg 2 must be a class or tuple of classes")
		}
		return sub.IsSubtype(parent), nil
	}
	if tup, ok := args[1].(*object.TupleObject); ok {
		for _, item := range tup.Items {
			is, err := check(item)
			if err != nil {
				return nil, err
			}
			if is {
				return object.PyTrue, nil
			}
		}
		return object.PyFalse, nil
	}
	is, err := check(args[1])
	if err != nil {
		return nil, err
	}
	return object.BoolFrom(is), nil
}

func biCallable(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("callable() takes exactly one argument")
	}
	switch o := args[0].(type) {
	case *object.TypeObject, object.Callable:
		return object.PyTrue, nil
	case *object.InstanceObject:
		_, found := o.Dunder("__call__")
		return object.BoolFrom(found), nil
	}
	return object.PyFalse, nil
}

func biZip(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	its, err := iterators(args)
	if err != nil {
		return nil, err
	}
	done := len(its) == 0
	return &lazyIter{name: "zip", next: func() (object.Object, error) {
		if done {
			return nil, object.ErrStopIteration
		}
		row := make([]object.Object, len(its))
		for i, it := range its {
			v, err := it.Next()
			if err != nil {
				done = true
				return nil, err
			}
			row[i] = v
		}
		return object.NewTuple(row), nil
	}}, nil
}

func biMap(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) < 2 {
		return nil, object.NewTypeError("map() must have at least two arguments")
	}
	fn := args[0]
	its, err := iterators(args[1:])
	if err != nil {
		return nil, err
	}
	return &lazyIter{name: "map", next: func() (object.Object, error) {
		call := make([]object.Object, len(its))
		for i, it := range its {
			v, err := it.Next()
			if err != nil {
				return nil, err
			}
			call[i] = v
		}
		return object.Invoke(fn, call)
	}}, nil
}

func biFilter(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, object.NewTypeError("filter expected 2 arguments, got %d", len(args))
	}
	fn := args[0]
	it, err := object.GetIter(args[1])
	if err != nil {
		return nil, err
	}
	return &lazyIter{name: "filter", next: func() (object.Object, error) {
		for {
			v, err := it.Next()
			if err != nil {
				return nil, err
			}
			var keep bool
			if object.IsNone(fn) {
				keep, err = object.IsTruthy(v)
			} else {
				var res object.Object
				if res, err = object.Invoke(fn, []object.Object{v}); err == nil {
					keep, err = object.IsTruthy(res)
				}
			}
			if err != nil {
				return nil, err
			}
			if keep {
				return v, nil
			}
		}
	}}, nil
}

func biDivmod(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, object.NewTypeError("divmod expected 2 arguments, got %d", len(args))
	}
	q, err := object.BinaryOp(args[0], args[1], "//")
	if err != nil {
		return nil, err
	}
	m, err := object.BinaryOp(args[0], args[1], "%")
	if err != nil {
		return nil, err
	}
	return object.NewTuple([]object.Object{q, m}), nil
}

func biPow(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	switch len(args) {
	case 2:
		return object.BinaryOp(args[0], args[1], "**")
	case 3:
		b, ok1 := args[0].(*object.IntObject)
		e, ok2 := args[1].(*object.IntObject)
		m, ok3 := args[2].(*object.IntObject)
		if !ok1 || !ok2 || !ok3 {
			return nil, object.NewTypeError("pow() 3rd argument not allowed unless all arguments are integers")
		}
		if m.Big().Sign() == 0 {
			return nil, object.NewValueError("pow() 3rd argument cannot be 0")
		}
		if e.Big().Sign() < 0 {
			return nil, object.NewValueError("pow() with a negative exponent and modulus is not supported")
		}
		// Python's result takes the sign of the modulus.
		res := new(big.Int).Exp(b.Big(), e.Big(), new(big.Int).Abs(m.Big()))
		if res.Sign() != 0 && m.Big().Sign() < 0 {
			res.Add(res, m.Big())
		}
		return object.IntFromBig(res), nil
	}
	return nil, object.NewTypeError("pow expected 2 or 3 arguments, got %d", len(args))
}

func biRadix(args []object.Object, name string, base int, prefix string) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("%s() takes exactly one argument", name)
	}
	var n *big.Int
	switch v := args[0].(type) {
	case *object.IntObject:
		n = v.Big()
	case *object.BoolObject:
		n = big.NewInt(0)
		if v.Value {
			n = big.NewInt(1)
		}
	default:
		return nil, object.NewTypeError("'%s' object cannot be interpreted as an integer", args[0].Type().Name())
	}
	digits := strings.TrimPrefix(n.Text(base), "-")
	sign := ""
	if n.Sign() < 0 {
		sign = "-"
	}
	return object.StrFrom(sign + prefix + digits), nil
}

func biID(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("id() takes exactly one argument")
	}
	v := reflect.ValueOf(args[0])
	if v.Kind() == reflect.Ptr {
		return object.IntFrom(int64(v.Pointer())), nil
	}
	return object.IntFrom(0), nil
}

func biDelAttr(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, object.NewTypeError("delattr expected 2 arguments")
	}
	name, ok := args[1].(*object.StrObject)
	if !ok {
		return nil, object.NewTypeError("attribute name must be string")
	}
	holder, ok := args[0].(object.AttributeHolder)
	if !ok {
		return nil, object.NewAttributeError(args[0].Type().Name(), name.Value)
	}
	if err := holder.DelAttr(name.Value); err != nil {
		return nil, err
	}
	return object.None, nil
}

func biSlice(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	switch len(args) {
	case 1:
		return object.NewSlice(object.None, args[0], object.None), nil
	case 2:
		return object.NewSlice(args[0], args[1], object.None), nil
	case 3:
		return object.NewSlice(args[0], args[1], args[2]), nil
	}
	return nil, object.NewTypeError("slice expected 1 to 3 arguments, got %d", len(args))
}
