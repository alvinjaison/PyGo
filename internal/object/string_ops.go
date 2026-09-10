package object

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ============================================================
// String operator slot implementations
// ============================================================

// strAdd implements str.__add__ (string concatenation).
// Returns TypeError if the right operand is not a string.
func strAdd(a, b Object) (Object, error) {
	as, ok := a.(*StrObject)
	if !ok {
		return NotImplemented, nil
	}
	bs, ok := b.(*StrObject)
	if !ok {
		return NotImplemented, nil
	}
	return StrFrom(as.Value + bs.Value), nil
}

// strMul implements str.__mul__ (string repetition).
// Repeats the string N times where N is an integer.
// If N <= 0, returns the empty string.
func strMul(a, b Object) (Object, error) {
	as, ok := a.(*StrObject)
	if !ok {
		return NotImplemented, nil
	}
	n, err := toRepeatCount(b)
	if err != nil {
		return NotImplemented, nil
	}
	if n <= 0 {
		return StrFrom(""), nil
	}
	return StrFrom(strings.Repeat(as.Value, n)), nil
}

// strRMul implements str.__rmul__ (e.g., 5 * "abc").
func strRMul(a, b Object) (Object, error) {
	as, ok := a.(*StrObject)
	if !ok {
		return NotImplemented, nil
	}
	n, err := toRepeatCount(b)
	if err != nil {
		return NotImplemented, nil
	}
	if n <= 0 {
		return StrFrom(""), nil
	}
	return StrFrom(strings.Repeat(as.Value, n)), nil
}

// strGetItem implements str.__getitem__ (indexing by code point).
// Supports negative indices. Raises IndexError for out-of-range.
func strGetItem(self Object, key Object) (Object, error) {
	s, ok := self.(*StrObject)
	if !ok {
		return nil, fmt.Errorf("TypeError: descriptor '__getitem__' requires a 'str' object")
	}
	idx, ok := key.(*IntObject)
	if !ok {
		if b, isBool := key.(*BoolObject); isBool {
			v := int64(0)
			if b.Value {
				v = 1
			}
			idx = IntFrom(v)
		} else {
			return nil, fmt.Errorf("TypeError: string indices must be integers, not '%s'", key.Type().Name())
		}
	}
	if !idx.Value.IsInt64() {
		return nil, fmt.Errorf("IndexError: string index out of range")
	}
	i := int(idx.Value.Int64())
	length := utf8.RuneCountInString(s.Value)
	if i < 0 {
		i += length
	}
	if i < 0 || i >= length {
		return nil, fmt.Errorf("IndexError: string index out of range")
	}
	// Walk to the i-th rune.
	pos := 0
	for j := 0; j < i; j++ {
		_, size := utf8.DecodeRuneInString(s.Value[pos:])
		pos += size
	}
	_, size := utf8.DecodeRuneInString(s.Value[pos:])
	return StrFrom(s.Value[pos : pos+size]), nil
}

// strRichCmp implements str rich comparison (Unicode code-point ordering).
func strRichCmp(self, other Object, op CompareOp) (Object, error) {
	s, ok := self.(*StrObject)
	if !ok {
		return NotImplemented, nil
	}
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

// strContains implements the `in` operator for strings.
// "sub" in "substring" → true if substring contains sub.
func strContains(self, item Object) (bool, error) {
	s, ok := self.(*StrObject)
	if !ok {
		return false, fmt.Errorf("TypeError: descriptor '__contains__' requires a 'str' object")
	}
	sub, ok := item.(*StrObject)
	if !ok {
		return false, fmt.Errorf("TypeError: 'in <string>' requires string as left operand, not %s", item.Type().Name())
	}
	return strings.Contains(s.Value, sub.Value), nil
}

// strLen implements str.__len__ (number of Unicode code points).
func strLen(self Object) (int, error) {
	s, ok := self.(*StrObject)
	if !ok {
		return 0, fmt.Errorf("TypeError: descriptor '__len__' requires a 'str' object")
	}
	return utf8.RuneCountInString(s.Value), nil
}

// strHash implements str.__hash__.
func strHash(self Object) (int64, error) {
	s, ok := self.(*StrObject)
	if !ok {
		return 0, fmt.Errorf("TypeError: descriptor '__hash__' requires a 'str' object")
	}
	return s.Hash()
}

// ============================================================
// StrObject interface implementations for Subscriptable
// ============================================================

// GetItem implements Subscriptable for StrObject.
func (s *StrObject) GetItem(key Object) (Object, error) {
	return strGetItem(s, key)
}

// SetItem implements Subscriptable for StrObject (strings are immutable).
func (s *StrObject) SetItem(key Object, value Object) error {
	return fmt.Errorf("TypeError: 'str' object does not support item assignment")
}

// DelItem implements Subscriptable for StrObject (strings are immutable).
func (s *StrObject) DelItem(key Object) error {
	return fmt.Errorf("TypeError: 'str' object does not support item deletion")
}

// ============================================================
// Slot registration
// ============================================================

// registerStringSlots registers string operator slots on StrType.
// Called from registerOperatorSlots() in ops.go.
func registerStringSlots() {
	StrType.slots.Add = strAdd
	StrType.slots.Mul = strMul
	StrType.slots.RMul = strRMul
	StrType.slots.GetItem = strGetItem
	StrType.slots.RichCmp = strRichCmp
	StrType.slots.Contains = strContains
	StrType.slots.Len = strLen
	StrType.slots.Hash = strHash
}
