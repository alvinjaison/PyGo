package object

import (
	"math"
	"math/big"
)

// This file wires the numeric and comparison protocol slots for the built-in
// int, float, bool, and str types. It runs after the type objects are created
// in types.go (init order within a package: by file, then declaration; we use
// a dedicated init here which runs after the types.go init because this file
// sorts after "types.go"? — Go does not guarantee cross-file init order, so we
// install slots lazily via InstallCoreSlots called from types.go init.)

// installCoreSlots wires the numeric/comparison protocol slots onto the
// built-in types. It is called from the types.go init() after the type
// objects are constructed, to guarantee correct initialization order
// (Go does not guarantee cross-file init ordering).
func installCoreSlots() {
	installIntSlots()
	installFloatSlots()
	installStrSlots()
	installBoolSlots()
}

// ---- helpers to coerce operands ----

func toBig(o Object) (*big.Int, bool) {
	switch v := o.(type) {
	case *IntObject:
		return v.Value, true
	case *BoolObject:
		if v.Value {
			return big.NewInt(1), true
		}
		return big.NewInt(0), true
	}
	return nil, false
}

func toFloat(o Object) (float64, bool) {
	switch v := o.(type) {
	case *FloatObject:
		return v.Value, true
	case *IntObject:
		f := new(big.Float).SetInt(v.Value)
		out, _ := f.Float64()
		return out, true
	case *BoolObject:
		if v.Value {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func isIntLike(o Object) bool {
	switch o.(type) {
	case *IntObject, *BoolObject:
		return true
	}
	return false
}

// numericBinary dispatches an arithmetic op with Python's int/float coercion.
func numericBinary(a, b Object, op string) (Object, error) {
	// Both int-like: integer arithmetic (except true division).
	if isIntLike(a) && isIntLike(b) && op != "/" {
		ai, _ := toBig(a)
		bi, _ := toBig(b)
		return intArith(ai, bi, op)
	}
	// Otherwise, if both are numeric, use float arithmetic.
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if aok && bok {
		return floatArith(af, bf, op)
	}
	return NotImplemented, nil
}

func intArith(a, b *big.Int, op string) (Object, error) {
	switch op {
	case "+":
		return IntFromBig(new(big.Int).Add(a, b)), nil
	case "-":
		return IntFromBig(new(big.Int).Sub(a, b)), nil
	case "*":
		return IntFromBig(new(big.Int).Mul(a, b)), nil
	case "//":
		if b.Sign() == 0 {
			return nil, NewZeroDivisionError()
		}
		return IntFromBig(floorDivBig(a, b)), nil
	case "%":
		if b.Sign() == 0 {
			return nil, NewException(TypeZeroDivisionError, "integer modulo by zero")
		}
		return IntFromBig(floorModBig(a, b)), nil
	case "**":
		if b.Sign() < 0 {
			af := new(big.Float).SetInt(a)
			bf, _ := af.Float64()
			ei, _ := b.Float64()
			return FloatFrom(math.Pow(bf, ei)), nil
		}
		return IntFromBig(new(big.Int).Exp(a, b, nil)), nil
	case "&":
		return IntFromBig(new(big.Int).And(a, b)), nil
	case "|":
		return IntFromBig(new(big.Int).Or(a, b)), nil
	case "^":
		return IntFromBig(new(big.Int).Xor(a, b)), nil
	case "<<":
		return IntFromBig(new(big.Int).Lsh(a, uint(b.Int64()))), nil
	case ">>":
		return IntFromBig(new(big.Int).Rsh(a, uint(b.Int64()))), nil
	}
	return NotImplemented, nil
}

// floorDivBig implements Python floor division (rounds toward −∞).
func floorDivBig(a, b *big.Int) *big.Int {
	q := new(big.Int)
	m := new(big.Int)
	q.DivMod(a, b, m) // Go's DivMod is Euclidean; adjust for Python semantics
	// Python: result rounds toward negative infinity.
	if (m.Sign() != 0) && ((a.Sign() < 0) != (b.Sign() < 0)) {
		// DivMod already gives Euclidean; convert to floor.
	}
	// Use truncated division then adjust.
	tq := new(big.Int).Quo(a, b)
	tr := new(big.Int).Rem(a, b)
	if tr.Sign() != 0 && (tr.Sign() < 0) != (b.Sign() < 0) {
		tq.Sub(tq, big.NewInt(1))
	}
	return tq
}

// floorModBig implements Python modulo (result has the sign of the divisor).
func floorModBig(a, b *big.Int) *big.Int {
	r := new(big.Int).Rem(a, b)
	if r.Sign() != 0 && (r.Sign() < 0) != (b.Sign() < 0) {
		r.Add(r, b)
	}
	return r
}

func floatArith(a, b float64, op string) (Object, error) {
	switch op {
	case "+":
		return FloatFrom(a + b), nil
	case "-":
		return FloatFrom(a - b), nil
	case "*":
		return FloatFrom(a * b), nil
	case "/":
		if b == 0 {
			return nil, NewException(TypeZeroDivisionError, "float division by zero")
		}
		return FloatFrom(a / b), nil
	case "//":
		if b == 0 {
			return nil, NewException(TypeZeroDivisionError, "float floor division by zero")
		}
		return FloatFrom(math.Floor(a / b)), nil
	case "%":
		if b == 0 {
			return nil, NewException(TypeZeroDivisionError, "float modulo")
		}
		m := math.Mod(a, b)
		if m != 0 && (m < 0) != (b < 0) {
			m += b
		}
		return FloatFrom(m), nil
	case "**":
		return FloatFrom(math.Pow(a, b)), nil
	}
	return NotImplemented, nil
}

// numericCompare implements rich comparison for numbers.
func numericCompare(a, b Object, op CompareOp) (Object, error) {
	// Integer-exact comparison when both are int-like.
	if isIntLike(a) && isIntLike(b) {
		ai, _ := toBig(a)
		bi, _ := toBig(b)
		return boolFromCmp(ai.Cmp(bi), op), nil
	}
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if aok && bok {
		c := 0
		switch {
		case af < bf:
			c = -1
		case af > bf:
			c = 1
		}
		return boolFromCmp(c, op), nil
	}
	return NotImplemented, nil
}

func boolFromCmp(c int, op CompareOp) Object {
	switch op {
	case OpLt:
		return BoolFrom(c < 0)
	case OpLtE:
		return BoolFrom(c <= 0)
	case OpEq:
		return BoolFrom(c == 0)
	case OpNotEq:
		return BoolFrom(c != 0)
	case OpGt:
		return BoolFrom(c > 0)
	case OpGtE:
		return BoolFrom(c >= 0)
	}
	return PyFalse
}

// ---- slot installation ----

func installIntSlots() {
	s := &IntType.slots
	bind := func(op string) func(a, b Object) (Object, error) {
		return func(a, b Object) (Object, error) { return numericBinary(a, b, op) }
	}
	s.Add = bind("+")
	s.Sub = bind("-")
	s.Mul = bind("*")
	s.Div = bind("/")
	s.FloorDiv = bind("//")
	s.Mod = bind("%")
	s.Pow = bind("**")
	s.BitAnd = bind("&")
	s.BitOr = bind("|")
	s.BitXor = bind("^")
	s.LShift = bind("<<")
	s.RShift = bind(">>")
	s.RichCmp = func(a, b Object, op CompareOp) (Object, error) { return numericCompare(a, b, op) }
}

func installFloatSlots() {
	s := &FloatType.slots
	bind := func(op string) func(a, b Object) (Object, error) {
		return func(a, b Object) (Object, error) { return numericBinary(a, b, op) }
	}
	s.Add = bind("+")
	s.Sub = bind("-")
	s.Mul = bind("*")
	s.Div = bind("/")
	s.FloorDiv = bind("//")
	s.Mod = bind("%")
	s.Pow = bind("**")
	s.RichCmp = func(a, b Object, op CompareOp) (Object, error) { return numericCompare(a, b, op) }
}

func installBoolSlots() {
	// bool inherits int arithmetic; reuse the same numeric dispatch.
	s := &BoolType.slots
	bind := func(op string) func(a, b Object) (Object, error) {
		return func(a, b Object) (Object, error) { return numericBinary(a, b, op) }
	}
	s.Add = bind("+")
	s.Sub = bind("-")
	s.Mul = bind("*")
	s.RichCmp = func(a, b Object, op CompareOp) (Object, error) { return numericCompare(a, b, op) }
}

func installStrSlots() {
	s := &StrType.slots
	s.Add = func(a, b Object) (Object, error) {
		as, ok1 := a.(*StrObject)
		bs, ok2 := b.(*StrObject)
		if ok1 && ok2 {
			return StrFrom(as.Value + bs.Value), nil
		}
		return NotImplemented, nil
	}
	s.Mul = func(a, b Object) (Object, error) {
		// str * int repetition.
		if as, ok := a.(*StrObject); ok {
			if n, ok := toBig(b); ok {
				return StrFrom(repeatStr(as.Value, int(n.Int64()))), nil
			}
		}
		if bs, ok := b.(*StrObject); ok {
			if n, ok := toBig(a); ok {
				return StrFrom(repeatStr(bs.Value, int(n.Int64()))), nil
			}
		}
		return NotImplemented, nil
	}
	s.RichCmp = func(a, b Object, op CompareOp) (Object, error) {
		as, ok1 := a.(*StrObject)
		bs, ok2 := b.(*StrObject)
		if !ok1 || !ok2 {
			return NotImplemented, nil
		}
		c := 0
		switch {
		case as.Value < bs.Value:
			c = -1
		case as.Value > bs.Value:
			c = 1
		}
		return boolFromCmp(c, op), nil
	}
	s.Contains = func(self, item Object) (bool, error) {
		ss, _ := self.(*StrObject)
		is, ok := item.(*StrObject)
		if !ok {
			return false, NewTypeError("'in <string>' requires string as left operand")
		}
		return indexOf(ss.Value, is.Value) >= 0, nil
	}
}

func repeatStr(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
