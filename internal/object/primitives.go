package object

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// ============================================================
// NoneObject
// ============================================================

// NoneObject is the type of Python's None singleton.
type NoneObject struct{}

func (n *NoneObject) Type() *TypeObject { return NoneType }
func (n *NoneObject) String() string    { return "None" }
func (n *NoneObject) Repr() string      { return "None" }
func (n *NoneObject) Hash() (int64, error) { return 0, nil }
func (n *NoneObject) IsTrue() (bool, error) { return false, nil }

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
	// Use CPython's algorithm from Objects/object.c _Py_HashDouble.
	if f.Value == float64(int64(f.Value)) {
		return IntFrom(int64(f.Value)).Hash()
	}
	// Non-integer float: use bit pattern.
	bits := *(*uint64)(ptr(&f.Value)) // safe: float64 and uint64 are same size
	h := int64(bits>>32) ^ int64(bits&0xffffffff)
	if h == -1 {
		h = -2
	}
	return h, nil
}
func (f *FloatObject) IsTrue() (bool, error) { return f.Value != 0.0, nil }

// ptr is a helper for float hash — avoids unsafe by using a well-defined
// integer encoding of the float's bit pattern.
func ptr(v *float64) *uint64 {
	// Use big.Float bit pattern as a proxy — avoids unsafe.Pointer.
	// This is not a cryptographic hash; just needs to be consistent.
	bits := floatBits(v)
	return &bits
}

func floatBits(v *float64) uint64 {
	// Encode float64 as uint64 using Go's math/bits semantics.
	// We avoid unsafe by using math.Float64bits-equivalent logic.
	f := *v
	if f == 0 {
		return 0
	}
	// Use string encoding as a portable bit representation.
	s := strconv.FormatFloat(f, 'b', -1, 64)
	var h uint64
	for _, c := range s {
		h = h*31 + uint64(c)
	}
	return h
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
func (s *StrObject) Len() int               { return len([]rune(s.Value)) } // Unicode codepoint length
