package object

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ============================================================
// NoneObject
// ============================================================

// NoneObject is the type of Python's None singleton.
type NoneObject struct{}

func (n *NoneObject) Type() *TypeObject    { return NoneType }
func (n *NoneObject) String() string        { return "None" }
func (n *NoneObject) Repr() string          { return "None" }
func (n *NoneObject) Hash() (int64, error)  { return 0, nil }
func (n *NoneObject) IsTrue() (bool, error) { return false, nil }

// RichCompare implements Comparable for NoneObject.
// None supports only identity-based == and !=.
func (n *NoneObject) RichCompare(other Object, op CompareOp) (Object, error) {
	switch op {
	case OpEq:
		return BoolFrom(IsNone(other)), nil
	case OpNotEq:
		return BoolFrom(!IsNone(other)), nil
	default:
		return NotImplemented, nil
	}
}

// ============================================================
// notImplementedObject
// ============================================================

type notImplementedObject struct{}

func (n *notImplementedObject) Type() *TypeObject { return ObjectType } // placeholder
func (n *notImplementedObject) String() string    { return "NotImplemented" }
func (n *notImplementedObject) Repr() string      { return "NotImplemented" }

// ============================================================
// BoolObject
// ============================================================

// BoolObject represents Python's bool type.
// True and False are the only instances — use the package-level singletons.
type BoolObject struct {
	Value bool
}

// PyTrue and PyFalse are the singleton bool values.
var (
	PyTrue  = &BoolObject{Value: true}
	PyFalse = &BoolObject{Value: false}
)

// BoolFrom returns PyTrue or PyFalse.
func BoolFrom(v bool) *BoolObject {
	if v {
		return PyTrue
	}
	return PyFalse
}

func (b *BoolObject) Type() *TypeObject { return BoolType }
func (b *BoolObject) String() string {
	if b.Value {
		return "True"
	}
	return "False"
}
func (b *BoolObject) Repr() string { return b.String() }
func (b *BoolObject) Hash() (int64, error) {
	if b.Value {
		return 1, nil
	}
	return 0, nil
}
func (b *BoolObject) IsTrue() (bool, error) { return b.Value, nil }

// RichCompare implements Comparable for BoolObject.
// Bool is a subtype of int, so comparisons work numerically.
func (b *BoolObject) RichCompare(other Object, op CompareOp) (Object, error) {
	var bInt int64
	if b.Value {
		bInt = 1
	}
	switch o := other.(type) {
	case *BoolObject:
		var oInt int64
		if o.Value {
			oInt = 1
		}
		return boolCompareInt64(bInt, oInt, op), nil
	case *IntObject:
		return intCompare(big.NewInt(bInt), o.Value, op), nil
	case *FloatObject:
		return floatCompareVals(float64(bInt), o.Value, op), nil
	}
	return NotImplemented, nil
}

// ============================================================
// IntObject
// ============================================================

// IntObject represents a Python integer (arbitrary precision).
type IntObject struct {
	Value *big.Int
}

// small-integer cache: -5 through 256, matching CPython.
const (
	smallIntMin = -5
	smallIntMax = 256
)

var smallInts [smallIntMax - smallIntMin + 1]*IntObject

func init() {
	for i := range smallInts {
		smallInts[i] = &IntObject{Value: big.NewInt(int64(i + smallIntMin))}
	}
}

// IntFrom returns an IntObject for the given int64, using the small-integer
// cache when possible.
func IntFrom(n int64) *IntObject {
	if n >= smallIntMin && n <= smallIntMax {
		return smallInts[n-smallIntMin]
	}
	return &IntObject{Value: big.NewInt(n)}
}

// IntFromBig returns an IntObject wrapping a *big.Int.
func IntFromBig(n *big.Int) *IntObject {
	if n.IsInt64() {
		v := n.Int64()
		if v >= smallIntMin && v <= smallIntMax {
			return smallInts[v-smallIntMin]
		}
	}
	return &IntObject{Value: new(big.Int).Set(n)}
}

func (i *IntObject) Type() *TypeObject { return IntType }
func (i *IntObject) String() string    { return i.Value.String() }
func (i *IntObject) Repr() string      { return i.String() }
func (i *IntObject) Hash() (int64, error) {
	// Must match: hash(1) == hash(1.0) == hash(True)
	// For integers that fit in int64, use the value directly.
	if i.Value.IsInt64() {
		return i.Value.Int64(), nil
	}
	// For big integers, reduce modulo 2^61 - 1 (Python's HASH_MODULUS).
	const hashModulus = (1 << 61) - 1
	mod := new(big.Int).Mod(i.Value, big.NewInt(hashModulus))
	h := mod.Int64()
	if h == -1 {
		h = -2 // -1 is reserved as the error sentinel in CPython
	}
	return h, nil
}
func (i *IntObject) IsTrue() (bool, error) { return i.Value.Sign() != 0, nil }

// RichCompare implements Comparable for IntObject.
func (i *IntObject) RichCompare(other Object, op CompareOp) (Object, error) {
	switch o := other.(type) {
	case *IntObject:
		return intCompare(i.Value, o.Value, op), nil
	case *BoolObject:
		var oInt int64
		if o.Value {
			oInt = 1
		}
		return intCompare(i.Value, big.NewInt(oInt), op), nil
	case *FloatObject:
		// Compare int to float: convert int to float64 if it fits precisely,
		// otherwise use big.Float comparison for correctness.
		return intFloatCompare(i.Value, o.Value, op), nil
	}
	return NotImplemented, nil
}

// Int64 returns the int64 value of this integer, and whether it fits.
func (i *IntObject) Int64() (int64, bool) {
	if i.Value.IsInt64() {
		return i.Value.Int64(), true
	}
	return 0, false
}

// ============================================================
// FloatObject
// ============================================================

// FloatObject represents a Python float (IEEE 754 double precision).
type FloatObject struct {
	Value float64
}

func FloatFrom(v float64) *FloatObject { return &FloatObject{Value: v} }

func (f *FloatObject) Type() *TypeObject { return FloatType }
func (f *FloatObject) String() string    { return strconv.FormatFloat(f.Value, 'g', -1, 64) }
func (f *FloatObject) Repr() string      { return f.String() }
func (f *FloatObject) Hash() (int64, error) {
	// hash(1.0) must equal hash(1): integers and floats that are equal share a hash.
	// If the float is an exact integer, use the integer hash.
	if f.Value == math.Trunc(f.Value) && !math.IsInf(f.Value, 0) {
		// Value is an integer — delegate to IntObject.Hash() for consistency.
		intVal := int64(f.Value)
		if float64(intVal) == f.Value {
			return IntFrom(intVal).Hash()
		}
		// For very large float integers that don't fit in int64, use big.Int.
		bf := new(big.Float).SetFloat64(f.Value)
		bi, _ := bf.Int(nil)
		return IntFromBig(bi).Hash()
	}
	// Non-integer float: use a stable hash of the bit pattern.
	bits := math.Float64bits(f.Value)
	h := int64(bits>>32) ^ int64(bits&0xffffffff)
	if h == -1 {
		h = -2
	}
	return h, nil
}
func (f *FloatObject) IsTrue() (bool, error) { return f.Value != 0.0, nil }

// RichCompare implements Comparable for FloatObject.
func (f *FloatObject) RichCompare(other Object, op CompareOp) (Object, error) {
	switch o := other.(type) {
	case *FloatObject:
		return floatCompareVals(f.Value, o.Value, op), nil
	case *IntObject:
		return intFloatCompare(o.Value, f.Value, reflectCompareOpPrim(op)), nil
	case *BoolObject:
		var oFloat float64
		if o.Value {
			oFloat = 1
		}
		return floatCompareVals(f.Value, oFloat, op), nil
	}
	return NotImplemented, nil
}

// ============================================================
// StrObject
// ============================================================

// StrObject represents a Python str (Unicode string).
type StrObject struct {
	Value string
}

// strInternCache caches short strings. Strings up to 32 bytes are interned.
var strInternCache = make(map[string]*StrObject, 256)

// StrFrom returns a StrObject, interning short strings.
func StrFrom(s string) *StrObject {
	if len(s) <= 32 {
		if cached, ok := strInternCache[s]; ok {
			return cached
		}
		obj := &StrObject{Value: s}
		if len(strInternCache) < 10000 { // cap intern cache size
			strInternCache[s] = obj
		}
		return obj
	}
	return &StrObject{Value: s}
}

func (s *StrObject) Type() *TypeObject { return StrType }
func (s *StrObject) String() string    { return s.Value }
func (s *StrObject) Repr() string {
	// Use Go's %q but with single quotes to match Python's repr preference.
	q := fmt.Sprintf("%q", s.Value)
	// Convert double-quoted Go string to single-quoted Python style.
	if len(q) >= 2 && q[0] == '"' {
		inner := q[1 : len(q)-1]
		inner = strings.ReplaceAll(inner, `\"`, `"`)
		inner = strings.ReplaceAll(inner, `'`, `\'`)
		return "'" + inner + "'"
	}
	return q
}
func (s *StrObject) Hash() (int64, error) {
	// TODO: Use seeded SipHash for hash randomization (Phase 2).
	// Placeholder: FNV-1a. Use uint64 arithmetic, then cast to int64.
	var h uint64 = 14695981039346656037 // FNV-1a offset basis
	for _, c := range s.Value {
		h ^= uint64(c)
		h *= 1099511628211
	}
	result := int64(h)
	if result == -1 {
		result = -2
	}
	return result, nil
}
func (s *StrObject) IsTrue() (bool, error) { return len(s.Value) > 0, nil }
func (s *StrObject) Len() int              { return utf8.RuneCountInString(s.Value) } // Unicode codepoint length

// Index returns the i-th Unicode code point as a single-character string.
// Supports negative indexing (from end). Returns IndexError if out of range.
func (s *StrObject) Index(i int) (*StrObject, error) {
	length := s.Len()
	if i < 0 {
		i += length
	}
	if i < 0 || i >= length {
		return nil, fmt.Errorf("IndexError: string index out of range")
	}
	// Walk code points to find the i-th one.
	pos := 0
	for j := 0; j < i; j++ {
		_, size := utf8.DecodeRuneInString(s.Value[pos:])
		pos += size
	}
	_, size := utf8.DecodeRuneInString(s.Value[pos:])
	return StrFrom(s.Value[pos : pos+size]), nil
}

// Iter returns a string iterator that yields individual code points.
func (s *StrObject) Iter() (Iterator, error) {
	return &strIterator{str: s.Value, pos: 0}, nil
}

// RichCompare implements Comparable for StrObject.
// Strings are compared lexicographically by Unicode code points.
func (s *StrObject) RichCompare(other Object, op CompareOp) (Object, error) {
	o, ok := other.(*StrObject)
	if !ok {
		return NotImplemented, nil
	}
	switch op {
	case OpEq:
		return BoolFrom(s.Value == o.Value), nil
	case OpNotEq:
		return BoolFrom(s.Value != o.Value), nil
	case OpLt:
		return BoolFrom(s.Value < o.Value), nil
	case OpLtE:
		return BoolFrom(s.Value <= o.Value), nil
	case OpGt:
		return BoolFrom(s.Value > o.Value), nil
	case OpGtE:
		return BoolFrom(s.Value >= o.Value), nil
	default:
		return NotImplemented, nil
	}
}

// Contains implements the `in` operator for substrings.
func (s *StrObject) Contains(item Object) (bool, error) {
	sub, ok := item.(*StrObject)
	if !ok {
		return false, fmt.Errorf("TypeError: 'in <string>' requires string as left operand, not %s", item.Type().Name())
	}
	return strings.Contains(s.Value, sub.Value), nil
}

// ============================================================
// strIterator
// ============================================================

// strIterator yields individual Unicode code points from a string.
type strIterator struct {
	str string
	pos int
}

func (it *strIterator) Type() *TypeObject { return ObjectType } // TODO: str_iterator type
func (it *strIterator) String() string    { return "<str_iterator>" }
func (it *strIterator) Repr() string      { return it.String() }
func (it *strIterator) Next() (Object, error) {
	if it.pos >= len(it.str) {
		return nil, ErrStopIteration
	}
	r, size := utf8.DecodeRuneInString(it.str[it.pos:])
	it.pos += size
	return StrFrom(string(r)), nil
}

// ============================================================
// Comparison helpers
// ============================================================

// boolCompareInt64 compares two int64 values according to op.
func boolCompareInt64(a, b int64, op CompareOp) Object {
	switch op {
	case OpEq:
		return BoolFrom(a == b)
	case OpNotEq:
		return BoolFrom(a != b)
	case OpLt:
		return BoolFrom(a < b)
	case OpLtE:
		return BoolFrom(a <= b)
	case OpGt:
		return BoolFrom(a > b)
	case OpGtE:
		return BoolFrom(a >= b)
	default:
		return NotImplemented
	}
}

// intCompare compares two *big.Int values according to op.
func intCompare(a, b *big.Int, op CompareOp) Object {
	cmp := a.Cmp(b)
	switch op {
	case OpEq:
		return BoolFrom(cmp == 0)
	case OpNotEq:
		return BoolFrom(cmp != 0)
	case OpLt:
		return BoolFrom(cmp < 0)
	case OpLtE:
		return BoolFrom(cmp <= 0)
	case OpGt:
		return BoolFrom(cmp > 0)
	case OpGtE:
		return BoolFrom(cmp >= 0)
	default:
		return NotImplemented
	}
}

// intFloatCompare compares an integer to a float value.
// The result is "int op float". For exact comparison when the integer
// doesn't fit in float64 precisely, we use big.Float.
func intFloatCompare(intVal *big.Int, floatVal float64, op CompareOp) Object {
	// Handle NaN: NaN is not equal to anything, and all ordering
	// comparisons with NaN return false.
	if math.IsNaN(floatVal) {
		switch op {
		case OpNotEq:
			return PyTrue
		default:
			return PyFalse
		}
	}

	// Handle infinities.
	if math.IsInf(floatVal, 1) {
		// +Inf is greater than any integer.
		switch op {
		case OpLt, OpLtE, OpNotEq:
			return PyTrue
		default:
			return PyFalse
		}
	}
	if math.IsInf(floatVal, -1) {
		// -Inf is less than any integer.
		switch op {
		case OpGt, OpGtE, OpNotEq:
			return PyTrue
		default:
			return PyFalse
		}
	}

	// Use big.Float for exact comparison.
	bigInt := new(big.Float).SetInt(intVal)
	bigFloat := new(big.Float).SetFloat64(floatVal)
	cmp := bigInt.Cmp(bigFloat)

	switch op {
	case OpEq:
		return BoolFrom(cmp == 0)
	case OpNotEq:
		return BoolFrom(cmp != 0)
	case OpLt:
		return BoolFrom(cmp < 0)
	case OpLtE:
		return BoolFrom(cmp <= 0)
	case OpGt:
		return BoolFrom(cmp > 0)
	case OpGtE:
		return BoolFrom(cmp >= 0)
	default:
		return NotImplemented
	}
}

// floatCompareVals compares two float64 values according to op.
func floatCompareVals(a, b float64, op CompareOp) Object {
	switch op {
	case OpEq:
		return BoolFrom(a == b)
	case OpNotEq:
		return BoolFrom(a != b)
	case OpLt:
		return BoolFrom(a < b)
	case OpLtE:
		return BoolFrom(a <= b)
	case OpGt:
		return BoolFrom(a > b)
	case OpGtE:
		return BoolFrom(a >= b)
	default:
		return NotImplemented
	}
}

// reflectCompareOpPrim reflects a comparison op (swaps left/right sense).
// Used when we need to compare "int op float" but have the function
// expecting "float op int".
func reflectCompareOpPrim(op CompareOp) CompareOp {
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
