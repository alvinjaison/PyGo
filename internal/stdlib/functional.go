package stdlib

import "github.com/pygo/pygo/internal/object"

// buildFunctools constructs a small `functools` module.
func buildFunctools(name string) *object.ModuleObject {
	mod, set := newModule(name)

	// reduce(function, iterable[, initializer])
	set("reduce", fn("functools.reduce", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		if len(args) < 2 {
			return nil, object.NewTypeError("reduce expected at least 2 arguments")
		}
		f := args[0]
		items, err := drain(args[1])
		if err != nil {
			return nil, err
		}
		var acc object.Object
		start := 0
		if len(args) >= 3 {
			acc = args[2]
		} else {
			if len(items) == 0 {
				return nil, object.NewTypeError("reduce() of empty iterable with no initial value")
			}
			acc = items[0]
			start = 1
		}
		for _, it := range items[start:] {
			acc, err = object.Invoke(f, []object.Object{acc, it})
			if err != nil {
				return nil, err
			}
		}
		return acc, nil
	}))

	// partial(func, *args) returns a callable that prepends the bound args.
	set("partial", fn("functools.partial", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		if len(args) < 1 {
			return nil, object.NewTypeError("partial expected at least 1 argument")
		}
		f := args[0]
		bound := append([]object.Object{}, args[1:]...)
		return object.NewBuiltin("partial", func(callArgs []object.Object, _ map[string]object.Object) (object.Object, error) {
			full := append(append([]object.Object{}, bound...), callArgs...)
			return object.Invoke(f, full)
		}), nil
	}))

	return mod
}

// buildItertools constructs a small `itertools` module (eager variants).
func buildItertools(name string) *object.ModuleObject {
	mod, set := newModule(name)

	// chain(*iterables) -> list of all items.
	set("chain", fn("itertools.chain", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		var out []object.Object
		for _, a := range args {
			items, err := drain(a)
			if err != nil {
				return nil, err
			}
			out = append(out, items...)
		}
		return object.NewList(out), nil
	}))

	// repeat(obj, n) -> list of n copies.
	set("repeat", fn("itertools.repeat", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		if len(args) < 2 {
			return nil, object.NewTypeError("repeat (eager) requires a count")
		}
		n, err := argInt(args, 1)
		if err != nil {
			return nil, err
		}
		out := make([]object.Object, 0, n)
		for i := int64(0); i < n; i++ {
			out = append(out, args[0])
		}
		return object.NewList(out), nil
	}))

	// islice(iterable, stop) -> first `stop` items.
	set("islice", fn("itertools.islice", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		items, err := drain(args[0])
		if err != nil {
			return nil, err
		}
		stop, err := argInt(args, 1)
		if err != nil {
			return nil, err
		}
		if int(stop) < len(items) {
			items = items[:stop]
		}
		return object.NewList(items), nil
	}))

	return mod
}

// drain fully consumes an iterable into a slice.
func drain(o object.Object) ([]object.Object, error) {
	it, err := object.GetIter(o)
	if err != nil {
		return nil, err
	}
	var out []object.Object
	for {
		v, err := it.Next()
		if err == object.ErrStopIteration {
			break
		}
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
