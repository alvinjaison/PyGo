package stdlib

import (
	"math"

	"github.com/pygo/pygo/internal/object"
)

// buildMath constructs the `math` module.
func buildMath(name string) *object.ModuleObject {
	mod, set := newModule(name)

	// Constants.
	set("pi", object.FloatFrom(math.Pi))
	set("e", object.FloatFrom(math.E))
	set("tau", object.FloatFrom(2*math.Pi))
	set("inf", object.FloatFrom(math.Inf(1)))
	set("nan", object.FloatFrom(math.NaN()))

	// Single-argument float functions.
	unary := func(fname string, g func(float64) float64) {
		set(fname, fn("math."+fname, func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			x, err := argFloat(args, 0)
			if err != nil {
				return nil, err
			}
			return object.FloatFrom(g(x)), nil
		}))
	}
	unary("sqrt", math.Sqrt)
	unary("sin", math.Sin)
	unary("cos", math.Cos)
	unary("tan", math.Tan)
	unary("asin", math.Asin)
	unary("acos", math.Acos)
	unary("atan", math.Atan)
	unary("sinh", math.Sinh)
	unary("cosh", math.Cosh)
	unary("tanh", math.Tanh)
	unary("exp", math.Exp)
	unary("log2", math.Log2)
	unary("log10", math.Log10)
	unary("degrees", func(r float64) float64 { return r * 180 / math.Pi })
	unary("radians", func(d float64) float64 { return d * math.Pi / 180 })
	unary("fabs", math.Abs)

	// floor and ceil return ints in Python 3.
	set("floor", fn("math.floor", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		x, err := argFloat(args, 0)
		if err != nil {
			return nil, err
		}
		return object.IntFrom(int64(math.Floor(x))), nil
	}))
	set("ceil", fn("math.ceil", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		x, err := argFloat(args, 0)
		if err != nil {
			return nil, err
		}
		return object.IntFrom(int64(math.Ceil(x))), nil
	}))
	set("trunc", fn("math.trunc", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		x, err := argFloat(args, 0)
		if err != nil {
			return nil, err
		}
		return object.IntFrom(int64(math.Trunc(x))), nil
	}))

	// log supports an optional base.
	set("log", fn("math.log", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		x, err := argFloat(args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) > 1 {
			base, err := argFloat(args, 1)
			if err != nil {
				return nil, err
			}
			return object.FloatFrom(math.Log(x) / math.Log(base)), nil
		}
		return object.FloatFrom(math.Log(x)), nil
	}))

	// pow (always returns float, like math.pow).
	set("pow", fn("math.pow", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		x, err := argFloat(args, 0)
		if err != nil {
			return nil, err
		}
		y, err := argFloat(args, 1)
		if err != nil {
			return nil, err
		}
		return object.FloatFrom(math.Pow(x, y)), nil
	}))

	// hypot, atan2, fmod, copysign — two-argument float functions.
	binary := func(fname string, g func(a, b float64) float64) {
		set(fname, fn("math."+fname, func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			a, err := argFloat(args, 0)
			if err != nil {
				return nil, err
			}
			b, err := argFloat(args, 1)
			if err != nil {
				return nil, err
			}
			return object.FloatFrom(g(a, b)), nil
		}))
	}
	binary("hypot", math.Hypot)
	binary("atan2", math.Atan2)
	binary("fmod", math.Mod)
	binary("copysign", math.Copysign)

	// gcd and factorial operate on ints.
	set("gcd", fn("math.gcd", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		a, err := argInt(args, 0)
		if err != nil {
			return nil, err
		}
		b, err := argInt(args, 1)
		if err != nil {
			return nil, err
		}
		return object.IntFrom(gcd(abs64(a), abs64(b))), nil
	}))
	set("factorial", fn("math.factorial", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		n, err := argInt(args, 0)
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return nil, object.NewValueError("factorial() not defined for negative values")
		}
		result := int64(1)
		for i := int64(2); i <= n; i++ {
			result *= i
		}
		return object.IntFrom(result), nil
	}))

	// isnan, isinf predicates.
	set("isnan", fn("math.isnan", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		x, err := argFloat(args, 0)
		if err != nil {
			return nil, err
		}
		return object.BoolFrom(math.IsNaN(x)), nil
	}))
	set("isinf", fn("math.isinf", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		x, err := argFloat(args, 0)
		if err != nil {
			return nil, err
		}
		return object.BoolFrom(math.IsInf(x, 0)), nil
	}))

	return mod
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
