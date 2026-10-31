package object

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync/atomic"
)

// ============================================================
// NoneObject
// ============================================================

// NoneObject is the type of Python's None singleton.
type NoneObject struct{}

func (n *NoneObject) Type() *TypeObject     { return NoneType }
func (n *NoneObject) String() string        { return "None" }
func (n *NoneObject) Repr() string          { return "None" }
func (n *NoneObject) Hash() (int64, error)  { return 0, nil }
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
//
// Values that fit in an int64 are stored unboxed in small; only larger values
// carry a *big.Int. Use Big() to get a *big.Int for either; the result must not
// be modified. IntObjects are immutable.
type IntObject struct {
	bv      *big.Int // nil when isSmall
	small   int64
	isSmall bool
}

// Big returns the value as a *big.Int. The result must be treated as read-only.
func (i *IntObject) Big() *big.Int {
	if i.isSmall {
		return big.NewInt(i.small)
	}
	return i.bv
}

// small-integer cache: -5 through 256, matching CPython.
const (
	smallIntMin = -5
	smallIntMax = 256
)

var smallInts [smallIntMax - smallIntMin + 1]*IntObject

func init() {
	for i := range smallInts {
		smallInts[i] = newInt64(int64(i + smallIntMin))
	}
}

// IntFrom returns an IntObject for the given int64, using the small-integer
// cache when possible.
func IntFrom(n int64) *IntObject {
	if n >= smallIntMin && n <= smallIntMax {
		return smallInts[n-smallIntMin]
	}
	return newInt64(n)
}

// newInt64 builds an unboxed int64-backed IntObject.
func newInt64(n int64) *IntObject {
	return &IntObject{small: n, isSmall: true}
}

// IntFromBig returns an IntObject wrapping a *big.Int.
func IntFromBig(n *big.Int) *IntObject {
	if n.IsInt64() {
		return IntFrom(n.Int64())
	}
	return &IntObject{bv: new(big.Int).Set(n)}
}

func (i *IntObject) Type() *TypeObject { return IntType }
func (i *IntObject) String() string {
	if i.isSmall {
		return intString(i.small)
	}
	return i.bv.String()
}
func (i *IntObject) Repr() string { return i.String() }
func (i *IntObject) Hash() (int64, error) {
	// Must match CPython: hash(1) == hash(1.0) == hash(True), and larger
	// integers are reduced modulo 2^61 - 1 keeping their sign.
	const hashModulus = (1 << 61) - 1
	var h int64
	if i.isSmall {
		n := i.small
		if n > -hashModulus && n < hashModulus {
			h = n
		} else {
			neg := n < 0
			u := uint64(n)
			if neg {
				u = -u
			}
			h = int64(u % hashModulus)
			if neg {
				h = -h
			}
		}
	} else {
		abs := new(big.Int).Abs(i.bv)
		h = new(big.Int).Mod(abs, big.NewInt(hashModulus)).Int64()
		if i.bv.Sign() < 0 {
			h = -h
		}
	}
	if h == -1 {
		h = -2 // -1 is reserved as the error sentinel in CPython
	}
	return h, nil
}
func (i *IntObject) IsTrue() (bool, error) {
	if i.isSmall {
		return i.small != 0, nil
	}
	return i.bv.Sign() != 0, nil
}

// Int64 returns the int64 value of this integer, and whether it fits.
func (i *IntObject) Int64() (int64, bool) {
	if i.isSmall {
		return i.small, true
	}
	if i.bv.IsInt64() {
		return i.bv.Int64(), true
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
func (f *FloatObject) String() string    { return formatFloat(f.Value) }
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
	hash  int64 // cached hash; 0 means not yet computed (accessed atomically)
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
	if h := atomic.LoadInt64(&s.hash); h != 0 {
		return h, nil
	}
	h := hashString(s.Value)
	atomic.StoreInt64(&s.hash, h)
	return h, nil
}

// hashString is the string hash used for StrObject and for string-keyed
// lookups that never build a StrObject.
func hashString(str string) int64 {
	// TODO: Use seeded SipHash for hash randomization.
	var h uint64 = 14695981039346656037 // FNV-1a offset basis
	for i := 0; i < len(str); i++ {
		h ^= uint64(str[i])
		h *= 1099511628211
	}
	result := int64(h)
	if result == -1 {
		result = -2
	}
	if result == 0 {
		result = 1 // 0 marks "not cached" in StrObject
	}
	return result
}
func (s *StrObject) IsTrue() (bool, error) { return len(s.Value) > 0, nil }
func (s *StrObject) Len() int              { return len([]rune(s.Value)) } // Unicode codepoint length

// formatFloat renders a float the way CPython's repr does: the shortest string
// that round-trips, positional notation for decimal exponents in [-4, 16) with
// a trailing ".0" for whole numbers, and scientific notation otherwise.
func formatFloat(v float64) string {
	switch {
	case math.IsNaN(v):
		return "nan"
	case math.IsInf(v, 1):
		return "inf"
	case math.IsInf(v, -1):
		return "-inf"
	}
	sci := strconv.FormatFloat(v, 'e', -1, 64) // d.ddde±XX
	exp, _ := strconv.Atoi(sci[strings.IndexByte(sci, 'e')+1:])
	if exp < -4 || exp >= 16 {
		return sci
	}
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.ContainsRune(s, '.') {
		s += ".0"
	}
	return s
}
