package object

import (
	"fmt"
	"strings"
)

// BytesObject represents an immutable Python bytes value.
type BytesObject struct {
	Value []byte
}

// NewBytes creates a bytes object from a byte slice (copied).
func NewBytes(b []byte) *BytesObject {
	cp := make([]byte, len(b))
	copy(cp, b)
	return &BytesObject{Value: cp}
}

// BytesFromString creates a bytes object from a string's raw bytes.
func BytesFromString(s string) *BytesObject {
	return &BytesObject{Value: []byte(s)}
}

func (b *BytesObject) Type() *TypeObject { return BytesType }
func (b *BytesObject) String() string    { return b.Repr() }

// Repr renders bytes as Python does: b'...' with non-printable bytes escaped.
func (b *BytesObject) Repr() string {
	var sb strings.Builder
	sb.WriteString("b'")
	for _, c := range b.Value {
		switch {
		case c == '\\':
			sb.WriteString(`\\`)
		case c == '\'':
			sb.WriteString(`\'`)
		case c == '\n':
			sb.WriteString(`\n`)
		case c == '\r':
			sb.WriteString(`\r`)
		case c == '\t':
			sb.WriteString(`\t`)
		case c >= 0x20 && c < 0x7f:
			sb.WriteByte(c)
		default:
			sb.WriteString(fmt.Sprintf(`\x%02x`, c))
		}
	}
	sb.WriteString("'")
	return sb.String()
}

func (b *BytesObject) Len() int              { return len(b.Value) }
func (b *BytesObject) IsTrue() (bool, error) { return len(b.Value) > 0, nil }

func (b *BytesObject) Hash() (int64, error) {
	var h uint64 = 14695981039346656037
	for _, c := range b.Value {
		h ^= uint64(c)
		h *= 1099511628211
	}
	r := int64(h)
	if r == -1 {
		r = -2
	}
	return r, nil
}

// GetItem: integer index yields an int (the byte value); a slice yields bytes.
func (b *BytesObject) GetItem(key Object) (Object, error) {
	if sl, ok := key.(*SliceObject); ok {
		start, stop, step, err := sl.Resolve(len(b.Value))
		if err != nil {
			return nil, err
		}
		var out []byte
		if step > 0 {
			for i := start; i < stop; i += step {
				out = append(out, b.Value[i])
			}
		} else {
			for i := start; i > stop; i += step {
				out = append(out, b.Value[i])
			}
		}
		return &BytesObject{Value: out}, nil
	}
	idx, err := resolveIndex(key, len(b.Value))
	if err != nil {
		return nil, err
	}
	return IntFrom(int64(b.Value[idx])), nil
}

// Iter yields successive byte values as ints.
func (b *BytesObject) Iter() (Iterator, error) {
	items := make([]Object, len(b.Value))
	for i, c := range b.Value {
		items[i] = IntFrom(int64(c))
	}
	return NewList(items).Iter()
}

// Contains: `x in b` where x is an int (a byte) or a bytes subsequence.
func (b *BytesObject) Contains(item Object) (bool, error) {
	switch v := item.(type) {
	case *IntObject:
		n, _ := v.Int64()
		if n < 0 || n > 255 {
			return false, NewValueError("byte must be in range(0, 256)")
		}
		for _, c := range b.Value {
			if int64(c) == n {
				return true, nil
			}
		}
		return false, nil
	case *BytesObject:
		return strings.Contains(string(b.Value), string(v.Value)), nil
	}
	return false, NewTypeError("a bytes-like object is required, not '%s'", item.Type().Name())
}

// GetAttr exposes bytes methods.
func (b *BytesObject) GetAttr(name string) (Object, error) {
	m, ok := bytesMethods[name]
	if !ok {
		return nil, NewAttributeError("bytes", name)
	}
	return NewBuiltin("bytes."+name, func(args []Object, _ map[string]Object) (Object, error) {
		return m(b, args)
	}), nil
}

func (b *BytesObject) SetAttr(string, Object) error {
	return NewTypeError("'bytes' object attributes are read-only")
}
func (b *BytesObject) DelAttr(string) error {
	return NewTypeError("'bytes' object attributes are read-only")
}

type bytesMethod func(b *BytesObject, args []Object) (Object, error)

var bytesMethods = map[string]bytesMethod{
	"decode": func(b *BytesObject, _ []Object) (Object, error) {
		// UTF-8 decode (the default and only supported codec for now).
		return StrFrom(string(b.Value)), nil
	},
	"hex": func(b *BytesObject, _ []Object) (Object, error) {
		const hexdigits = "0123456789abcdef"
		out := make([]byte, 0, len(b.Value)*2)
		for _, c := range b.Value {
			out = append(out, hexdigits[c>>4], hexdigits[c&0x0f])
		}
		return StrFrom(string(out)), nil
	},
	"startswith": func(b *BytesObject, args []Object) (Object, error) {
		p, ok := args[0].(*BytesObject)
		if !ok {
			return nil, NewTypeError("startswith first arg must be bytes")
		}
		return BoolFrom(strings.HasPrefix(string(b.Value), string(p.Value))), nil
	},
	"endswith": func(b *BytesObject, args []Object) (Object, error) {
		p, ok := args[0].(*BytesObject)
		if !ok {
			return nil, NewTypeError("endswith first arg must be bytes")
		}
		return BoolFrom(strings.HasSuffix(string(b.Value), string(p.Value))), nil
	},
	"upper": func(b *BytesObject, _ []Object) (Object, error) {
		return &BytesObject{Value: []byte(strings.ToUpper(string(b.Value)))}, nil
	},
	"lower": func(b *BytesObject, _ []Object) (Object, error) {
		return &BytesObject{Value: []byte(strings.ToLower(string(b.Value)))}, nil
	},
	"split": func(b *BytesObject, args []Object) (Object, error) {
		var parts []string
		if len(args) == 0 {
			parts = strings.Fields(string(b.Value))
		} else {
			sep, ok := args[0].(*BytesObject)
			if !ok {
				return nil, NewTypeError("split separator must be bytes")
			}
			parts = strings.Split(string(b.Value), string(sep.Value))
		}
		items := make([]Object, len(parts))
		for i, p := range parts {
			items[i] = BytesFromString(p)
		}
		return NewList(items), nil
	},
	"replace": func(b *BytesObject, args []Object) (Object, error) {
		old, ok1 := args[0].(*BytesObject)
		nw, ok2 := args[1].(*BytesObject)
		if !ok1 || !ok2 {
			return nil, NewTypeError("replace args must be bytes")
		}
		return BytesFromString(strings.ReplaceAll(string(b.Value), string(old.Value), string(nw.Value))), nil
	},
}

// installBytesSlots wires concatenation, repetition, and comparison onto the
// bytes type.
func installBytesSlots() {
	s := BytesType.Slots()
	s.Add = func(a, bb Object) (Object, error) {
		x, ok1 := a.(*BytesObject)
		y, ok2 := bb.(*BytesObject)
		if !ok1 || !ok2 {
			return NotImplemented, nil
		}
		out := make([]byte, 0, len(x.Value)+len(y.Value))
		out = append(out, x.Value...)
		out = append(out, y.Value...)
		return &BytesObject{Value: out}, nil
	}
	s.Mul = func(a, bb Object) (Object, error) {
		if x, ok := a.(*BytesObject); ok {
			if n, ok := toBig(bb); ok {
				return bytesRepeat(x, int(n.Int64())), nil
			}
		}
		if y, ok := bb.(*BytesObject); ok {
			if n, ok := toBig(a); ok {
				return bytesRepeat(y, int(n.Int64())), nil
			}
		}
		return NotImplemented, nil
	}
	s.RichCmp = func(a, bb Object, op CompareOp) (Object, error) {
		x, ok1 := a.(*BytesObject)
		y, ok2 := bb.(*BytesObject)
		if !ok1 || !ok2 {
			return NotImplemented, nil
		}
		c := bytesCompare(x.Value, y.Value)
		return boolFromCmp(c, op), nil
	}
	s.Contains = func(self, item Object) (bool, error) {
		return self.(*BytesObject).Contains(item)
	}
}

func bytesRepeat(b *BytesObject, n int) *BytesObject {
	if n <= 0 {
		return &BytesObject{Value: []byte{}}
	}
	out := make([]byte, 0, len(b.Value)*n)
	for i := 0; i < n; i++ {
		out = append(out, b.Value...)
	}
	return &BytesObject{Value: out}
}

func bytesCompare(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}
