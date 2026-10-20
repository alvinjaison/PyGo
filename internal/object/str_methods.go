package object

import (
	"strings"
)

// GetAttr resolves string methods. StrObject is immutable, so every method
// returns a new object. Methods are returned as builtin functions already
// bound to this string's value.
func (s *StrObject) GetAttr(name string) (Object, error) {
	m, ok := strMethods[name]
	if !ok {
		return nil, NewAttributeError("str", name)
	}
	self := s.Value
	return NewBuiltin("str."+name, func(args []Object, kwargs map[string]Object) (Object, error) {
		return m(self, args)
	}), nil
}

func (s *StrObject) SetAttr(string, Object) error {
	return NewTypeError("'str' object attributes are read-only")
}
func (s *StrObject) DelAttr(string) error {
	return NewTypeError("'str' object attributes are read-only")
}

// GetItem supports indexing and slicing on strings.
func (s *StrObject) GetItem(key Object) (Object, error) {
	runes := []rune(s.Value)
	if sl, ok := key.(*SliceObject); ok {
		start, stop, step, err := sl.Resolve(len(runes))
		if err != nil {
			return nil, err
		}
		var out []rune
		if step > 0 {
			for i := start; i < stop; i += step {
				out = append(out, runes[i])
			}
		} else {
			for i := start; i > stop; i += step {
				out = append(out, runes[i])
			}
		}
		return StrFrom(string(out)), nil
	}
	idx, err := resolveIndex(key, len(runes))
	if err != nil {
		return nil, err
	}
	return StrFrom(string(runes[idx])), nil
}

func (s *StrObject) SetItem(Object, Object) error {
	return NewTypeError("'str' object does not support item assignment")
}
func (s *StrObject) DelItem(Object) error {
	return NewTypeError("'str' object doesn't support item deletion")
}

// Iter iterates over a string's characters.
func (s *StrObject) Iter() (Iterator, error) {
	runes := []rune(s.Value)
	items := make([]Object, len(runes))
	for i, r := range runes {
		items[i] = StrFrom(string(r))
	}
	return NewList(items).Iter()
}

// strMethod is a string method implementation bound at call time.
type strMethod func(self string, args []Object) (Object, error)

func strArg(args []Object, i int) (string, error) {
	if i >= len(args) {
		return "", NewTypeError("missing string argument")
	}
	s, ok := args[i].(*StrObject)
	if !ok {
		return "", NewTypeError("expected str, got '%s'", args[i].Type().Name())
	}
	return s.Value, nil
}

var strMethods = map[string]strMethod{
	"upper": func(self string, _ []Object) (Object, error) {
		return StrFrom(strings.ToUpper(self)), nil
	},
	"lower": func(self string, _ []Object) (Object, error) {
		return StrFrom(strings.ToLower(self)), nil
	},
	"title": func(self string, _ []Object) (Object, error) {
		return StrFrom(strings.Title(self)), nil //nolint:staticcheck
	},
	"capitalize": func(self string, _ []Object) (Object, error) {
		if self == "" {
			return StrFrom(""), nil
		}
		r := []rune(self)
		return StrFrom(strings.ToUpper(string(r[0])) + strings.ToLower(string(r[1:]))), nil
	},
	"strip": func(self string, args []Object) (Object, error) {
		if len(args) > 0 {
			cut, err := strArg(args, 0)
			if err != nil {
				return nil, err
			}
			return StrFrom(strings.Trim(self, cut)), nil
		}
		return StrFrom(strings.TrimSpace(self)), nil
	},
	"lstrip": func(self string, args []Object) (Object, error) {
		if len(args) > 0 {
			cut, _ := strArg(args, 0)
			return StrFrom(strings.TrimLeft(self, cut)), nil
		}
		return StrFrom(strings.TrimLeft(self, " \t\n\r")), nil
	},
	"rstrip": func(self string, args []Object) (Object, error) {
		if len(args) > 0 {
			cut, _ := strArg(args, 0)
			return StrFrom(strings.TrimRight(self, cut)), nil
		}
		return StrFrom(strings.TrimRight(self, " \t\n\r")), nil
	},
	"split": func(self string, args []Object) (Object, error) {
		if len(args) == 0 {
			parts := strings.Fields(self)
			return strListToObject(parts), nil
		}
		sep, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return strListToObject(strings.Split(self, sep)), nil
	},
	"rsplit": func(self string, args []Object) (Object, error) {
		if len(args) == 0 {
			return strListToObject(strings.Fields(self)), nil
		}
		sep, _ := strArg(args, 0)
		return strListToObject(strings.Split(self, sep)), nil
	},
	"splitlines": func(self string, _ []Object) (Object, error) {
		trimmed := strings.TrimSuffix(self, "\n")
		if trimmed == "" {
			return NewList(nil), nil
		}
		return strListToObject(strings.Split(trimmed, "\n")), nil
	},
	"join": func(self string, args []Object) (Object, error) {
		items, err := iterateStrings(args)
		if err != nil {
			return nil, err
		}
		return StrFrom(strings.Join(items, self)), nil
	},
	"replace": func(self string, args []Object) (Object, error) {
		old, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		nw, err := strArg(args, 1)
		if err != nil {
			return nil, err
		}
		n := -1
		if len(args) > 2 {
			if ni, ok := args[2].(*IntObject); ok {
				v, _ := ni.Int64()
				n = int(v)
			}
		}
		return StrFrom(strings.Replace(self, old, nw, n)), nil
	},
	"startswith": func(self string, args []Object) (Object, error) {
		p, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return BoolFrom(strings.HasPrefix(self, p)), nil
	},
	"endswith": func(self string, args []Object) (Object, error) {
		p, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return BoolFrom(strings.HasSuffix(self, p)), nil
	},
	"find": func(self string, args []Object) (Object, error) {
		sub, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return IntFrom(int64(strings.Index(self, sub))), nil
	},
	"count": func(self string, args []Object) (Object, error) {
		sub, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return IntFrom(int64(strings.Count(self, sub))), nil
	},
	"index": func(self string, args []Object) (Object, error) {
		sub, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		i := strings.Index(self, sub)
		if i < 0 {
			return nil, NewValueError("substring not found")
		}
		return IntFrom(int64(i)), nil
	},
	"isdigit": func(self string, _ []Object) (Object, error) {
		if self == "" {
			return PyFalse, nil
		}
		for _, r := range self {
			if r < '0' || r > '9' {
				return PyFalse, nil
			}
		}
		return PyTrue, nil
	},
	"isalpha": func(self string, _ []Object) (Object, error) {
		if self == "" {
			return PyFalse, nil
		}
		for _, r := range self {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
				return PyFalse, nil
			}
		}
		return PyTrue, nil
	},
	"zfill": func(self string, args []Object) (Object, error) {
		n := 0
		if ni, ok := args[0].(*IntObject); ok {
			v, _ := ni.Int64()
			n = int(v)
		}
		if len(self) >= n {
			return StrFrom(self), nil
		}
		return StrFrom(strings.Repeat("0", n-len(self)) + self), nil
	},
	"encode": func(self string, _ []Object) (Object, error) {
		// UTF-8 encode (the default and only supported codec for now).
		return BytesFromString(self), nil
	},
	"format": strFormat,
}

func strListToObject(parts []string) Object {
	items := make([]Object, len(parts))
	for i, p := range parts {
		items[i] = StrFrom(p)
	}
	return NewList(items)
}

func iterateStrings(args []Object) ([]string, error) {
	if len(args) != 1 {
		return nil, NewTypeError("join() takes exactly one argument")
	}
	it, err := GetIter(args[0])
	if err != nil {
		return nil, err
	}
	var out []string
	for {
		v, err := it.Next()
		if err == ErrStopIteration {
			break
		}
		if err != nil {
			return nil, err
		}
		s, ok := v.(*StrObject)
		if !ok {
			return nil, NewTypeError("sequence item: expected str, got '%s'", v.Type().Name())
		}
		out = append(out, s.Value)
	}
	return out, nil
}

// strFormat implements a practical subset of str.format(): positional {},
// indexed {0}, and keyword {name} fields. Format specs are ignored.
func strFormat(self string, args []Object) (Object, error) {
	var b strings.Builder
	runes := []rune(self)
	auto := 0
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c == '{' {
			if i+1 < len(runes) && runes[i+1] == '{' {
				b.WriteRune('{')
				i++
				continue
			}
			j := i + 1
			var field strings.Builder
			for j < len(runes) && runes[j] != '}' {
				field.WriteRune(runes[j])
				j++
			}
			name := field.String()
			if idx := strings.IndexAny(name, ":!"); idx >= 0 {
				name = name[:idx]
			}
			val, err := resolveFormatField(name, args, &auto)
			if err != nil {
				return nil, err
			}
			b.WriteString(val.String())
			i = j
			continue
		}
		if c == '}' {
			if i+1 < len(runes) && runes[i+1] == '}' {
				b.WriteRune('}')
				i++
				continue
			}
		}
		b.WriteRune(c)
	}
	return StrFrom(b.String()), nil
}

func resolveFormatField(name string, args []Object, auto *int) (Object, error) {
	if name == "" {
		if *auto >= len(args) {
			return nil, NewException(TypeIndexError, "Replacement index out of range for positional args")
		}
		v := args[*auto]
		*auto++
		return v, nil
	}
	// Numeric index.
	isNum := true
	for _, r := range name {
		if r < '0' || r > '9' {
			isNum = false
			break
		}
	}
	if isNum {
		var idx int
		for _, r := range name {
			idx = idx*10 + int(r-'0')
		}
		if idx >= len(args) {
			return nil, NewException(TypeIndexError, "Replacement index out of range")
		}
		return args[idx], nil
	}
	// Keyword fields are not supported without kwargs plumbing; report clearly.
	return nil, NewException(TypeKeyError, name)
}
