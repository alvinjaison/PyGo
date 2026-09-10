package object

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ============================================================
// String methods (accessible via LOAD_ATTR)
//
// Index-taking and index-returning methods work in code points, not bytes, to
// stay consistent with str indexing, len() and slicing elsewhere in the object
// model. Go's strings package reports byte offsets, so results are converted.
// ============================================================

// GetAttr implements AttributeHolder for StrObject.
func (s *StrObject) GetAttr(name string) (Object, error) {
	switch name {
	case "upper":
		return strBoundMethod(s, "upper", strMethodUpper), nil
	case "lower":
		return strBoundMethod(s, "lower", strMethodLower), nil
	case "strip":
		return strBoundMethod(s, "strip", strMethodStrip), nil
	case "lstrip":
		return strBoundMethod(s, "lstrip", strMethodLStrip), nil
	case "rstrip":
		return strBoundMethod(s, "rstrip", strMethodRStrip), nil
	case "split":
		return strBoundMethod(s, "split", strMethodSplit), nil
	case "join":
		return strBoundMethod(s, "join", strMethodJoin), nil
	case "replace":
		return strBoundMethod(s, "replace", strMethodReplace), nil
	case "startswith":
		return strBoundMethod(s, "startswith", strMethodStartswith), nil
	case "endswith":
		return strBoundMethod(s, "endswith", strMethodEndswith), nil
	case "find":
		return strBoundMethod(s, "find", strMethodFind), nil
	case "format":
		return strBoundMethod(s, "format", strMethodFormat), nil
	default:
		return nil, NewAttributeError("str", name)
	}
}

// SetAttr returns an error since strings do not support attribute assignment.
func (s *StrObject) SetAttr(name string, value Object) error {
	return NewAttributeError("str", name)
}

// DelAttr returns an error since strings do not support attribute deletion.
func (s *StrObject) DelAttr(name string) error {
	return NewAttributeError("str", name)
}

// strBoundMethod creates a bound method for a string instance.
func strBoundMethod(s *StrObject, name string, fn func(*StrObject, []Object, map[string]Object) (Object, error)) *BuiltinFuncObject {
	return &BuiltinFuncObject{
		Name: name,
		Fn: func(args []Object, kwargs map[string]Object) (Object, error) {
			return fn(s, args, kwargs)
		},
	}
}

// ============================================================
// Argument helpers
// ============================================================

// strArg extracts a string argument. pos is the 1-based argument position, used
// to match CPython's "argument 1 must be str, not int" phrasing.
func strArg(method string, pos int, arg Object) (string, error) {
	s, ok := arg.(*StrObject)
	if !ok {
		return "", NewTypeError("%s() argument %d must be str, not %s",
			method, pos, arg.Type().Name())
	}
	return s.Value, nil
}

// intArg extracts an int argument, accepting bool as Python does.
func intArg(method string, arg Object) (int, error) {
	switch v := arg.(type) {
	case *IntObject:
		n, fits := v.Int64()
		if !fits {
			// Out of machine range: saturate, which callers then clamp.
			if v.Value.Sign() > 0 {
				return int(^uint(0) >> 1), nil
			}
			return -int(^uint(0)>>1) - 1, nil
		}
		return int(n), nil
	case *BoolObject:
		if v.Value {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, NewTypeError("%s() argument must be int, not %s",
			method, arg.Type().Name())
	}
}

// noKwargs rejects keyword arguments for methods that take none.
func noKwargs(method string, kwargs map[string]Object) error {
	for name := range kwargs {
		return NewTypeError("%s() takes no keyword arguments (got '%s')", method, name)
	}
	return nil
}

// checkArgRange rejects an argument count outside [min, max].
func checkArgRange(method string, args []Object, min, max int) error {
	if len(args) < min {
		return NewTypeError("%s() takes at least %d argument(s) (%d given)",
			method, min, len(args))
	}
	if len(args) > max {
		return NewTypeError("%s() takes at most %d argument(s) (%d given)",
			method, max, len(args))
	}
	return nil
}

// ============================================================
// Case conversion (Requirements 8.1, 8.2)
// ============================================================

func strMethodUpper(s *StrObject, args []Object, kwargs map[string]Object) (Object, error) {
	if err := checkArgRange("upper", args, 0, 0); err != nil {
		return nil, err
	}
	if err := noKwargs("upper", kwargs); err != nil {
		return nil, err
	}
	return StrFrom(strings.ToUpper(s.Value)), nil
}

func strMethodLower(s *StrObject, args []Object, kwargs map[string]Object) (Object, error) {
	if err := checkArgRange("lower", args, 0, 0); err != nil {
		return nil, err
	}
	if err := noKwargs("lower", kwargs); err != nil {
		return nil, err
	}
	return StrFrom(strings.ToLower(s.Value)), nil
}

// ============================================================
// Stripping (Requirements 8.3, 8.4, 8.16)
// ============================================================

// stripMode selects which ends of the string a strip operation affects.
type stripMode int

const (
	stripBoth stripMode = iota
	stripLeft
	stripRight
)

// stripWith removes leading and/or trailing characters. With no argument it
// strips whitespace; with a string argument it strips any character in that set
// (Go's cutset semantics match Python's).
func stripWith(s *StrObject, args []Object, kwargs map[string]Object, method string, mode stripMode) (Object, error) {
	if err := checkArgRange(method, args, 0, 1); err != nil {
		return nil, err
	}
	if err := noKwargs(method, kwargs); err != nil {
		return nil, err
	}

	useWhitespace := len(args) == 0 || IsNone(args[0])
	var cutset string
	if !useWhitespace {
		cut, ok := args[0].(*StrObject)
		if !ok {
			// CPython phrases this one differently from the other methods
			// because the argument is optional and accepts None.
			return nil, NewTypeError("%s arg must be None or str", method)
		}
		cutset = cut.Value
	}

	switch mode {
	case stripLeft:
		if useWhitespace {
			return StrFrom(strings.TrimLeftFunc(s.Value, unicode.IsSpace)), nil
		}
		return StrFrom(strings.TrimLeft(s.Value, cutset)), nil
	case stripRight:
		if useWhitespace {
			return StrFrom(strings.TrimRightFunc(s.Value, unicode.IsSpace)), nil
		}
		return StrFrom(strings.TrimRight(s.Value, cutset)), nil
	default:
		if useWhitespace {
			return StrFrom(strings.TrimFunc(s.Value, unicode.IsSpace)), nil
		}
		return StrFrom(strings.Trim(s.Value, cutset)), nil
	}
}

func strMethodStrip(s *StrObject, args []Object, kwargs map[string]Object) (Object, error) {
	return stripWith(s, args, kwargs, "strip", stripBoth)
}

func strMethodLStrip(s *StrObject, args []Object, kwargs map[string]Object) (Object, error) {
	return stripWith(s, args, kwargs, "lstrip", stripLeft)
}

func strMethodRStrip(s *StrObject, args []Object, kwargs map[string]Object) (Object, error) {
	return stripWith(s, args, kwargs, "rstrip", stripRight)
}

// ============================================================
// Splitting (Requirements 8.5, 8.6)
// ============================================================

func strMethodSplit(s *StrObject, args []Object, kwargs map[string]Object) (Object, error) {
	if err := checkArgRange("split", args, 0, 2); err != nil {
		return nil, err
	}
	if err := noKwargs("split", kwargs); err != nil {
		return nil, err
	}

	maxsplit := -1
	if len(args) == 2 {
		n, err := intArg("split", args[1])
		if err != nil {
			return nil, err
		}
		maxsplit = n
	}

	// No separator (or None): split on runs of whitespace, discarding leading
	// and trailing whitespace entirely.
	if len(args) == 0 || IsNone(args[0]) {
		return StrFrom("").splitWhitespace(s.Value, maxsplit), nil
	}

	sep, err := strArg("split", 1, args[0])
	if err != nil {
		return nil, err
	}
	if sep == "" {
		return nil, NewValueError("empty separator")
	}

	var parts []string
	if maxsplit < 0 {
		parts = strings.Split(s.Value, sep)
	} else {
		parts = strings.SplitN(s.Value, sep, maxsplit+1)
	}
	return stringsToList(parts), nil
}

// splitWhitespace splits on runs of whitespace. The receiver is unused; the
// method exists only to keep this helper next to its caller.
func (*StrObject) splitWhitespace(value string, maxsplit int) *ListObject {
	if maxsplit < 0 {
		return stringsToList(strings.Fields(value))
	}

	var parts []string
	runes := []rune(value)
	i := 0
	for len(parts) < maxsplit {
		// Skip whitespace before the next field.
		for i < len(runes) && unicode.IsSpace(runes[i]) {
			i++
		}
		if i >= len(runes) {
			return stringsToList(parts)
		}
		start := i
		for i < len(runes) && !unicode.IsSpace(runes[i]) {
			i++
		}
		parts = append(parts, string(runes[start:i]))
	}
	// The remainder keeps its internal whitespace but loses leading whitespace.
	for i < len(runes) && unicode.IsSpace(runes[i]) {
		i++
	}
	if i < len(runes) {
		parts = append(parts, string(runes[i:]))
	}
	return stringsToList(parts)
}

// stringsToList converts Go strings to a Python list of str.
func stringsToList(parts []string) *ListObject {
	items := make([]Object, len(parts))
	for i, p := range parts {
		items[i] = StrFrom(p)
	}
	return &ListObject{Items: items}
}

// ============================================================
// Joining (Requirement 8.7)
// ============================================================

func strMethodJoin(s *StrObject, args []Object, kwargs map[string]Object) (Object, error) {
	if err := checkArgRange("join", args, 1, 1); err != nil {
		return nil, err
	}
	if err := noKwargs("join", kwargs); err != nil {
		return nil, err
	}

	items, err := iterableToSlice(args[0])
	if err != nil {
		return nil, NewTypeError("can only join an iterable, not %s",
			args[0].Type().Name())
	}

	parts := make([]string, len(items))
	for i, item := range items {
		str, ok := item.(*StrObject)
		if !ok {
			return nil, NewTypeError("sequence item %d: expected str instance, %s found",
				i, item.Type().Name())
		}
		parts[i] = str.Value
	}
	return StrFrom(strings.Join(parts, s.Value)), nil
}

// ============================================================
// Replacement (Requirements 8.8, 8.9)
// ============================================================

func strMethodReplace(s *StrObject, args []Object, kwargs map[string]Object) (Object, error) {
	if err := checkArgRange("replace", args, 2, 3); err != nil {
		return nil, err
	}
	if err := noKwargs("replace", kwargs); err != nil {
		return nil, err
	}

	old, err := strArg("replace", 1, args[0])
	if err != nil {
		return nil, err
	}
	new_, err := strArg("replace", 2, args[1])
	if err != nil {
		return nil, err
	}

	count := -1
	if len(args) == 3 {
		n, err := intArg("replace", args[2])
		if err != nil {
			return nil, err
		}
		// A negative count means "replace every occurrence".
		if n < 0 {
			n = -1
		}
		count = n
	}
	return StrFrom(strings.Replace(s.Value, old, new_, count)), nil
}

// ============================================================
// Prefix and suffix tests (Requirements 8.10, 8.11)
// ============================================================

// affixCandidates returns the prefixes/suffixes to test. Python accepts either
// a single string or a tuple of them.
func affixCandidates(method string, arg Object) ([]string, error) {
	switch v := arg.(type) {
	case *StrObject:
		return []string{v.Value}, nil
	case *TupleObject:
		out := make([]string, len(v.Items))
		for i, item := range v.Items {
			str, ok := item.(*StrObject)
			if !ok {
				return nil, NewTypeError("tuple for %s must only contain str, not %s",
					method, item.Type().Name())
			}
			out[i] = str.Value
		}
		return out, nil
	default:
		return nil, NewTypeError("%s first arg must be str or a tuple of str, not %s",
			method, arg.Type().Name())
	}
}

func strMethodStartswith(s *StrObject, args []Object, kwargs map[string]Object) (Object, error) {
	if err := checkArgRange("startswith", args, 1, 3); err != nil {
		return nil, err
	}
	if err := noKwargs("startswith", kwargs); err != nil {
		return nil, err
	}
	candidates, err := affixCandidates("startswith", args[0])
	if err != nil {
		return nil, err
	}
	target, err := affixTarget(s.Value, "startswith", args)
	if err != nil {
		return nil, err
	}
	for _, c := range candidates {
		if strings.HasPrefix(target, c) {
			return PyTrue, nil
		}
	}
	return PyFalse, nil
}

func strMethodEndswith(s *StrObject, args []Object, kwargs map[string]Object) (Object, error) {
	if err := checkArgRange("endswith", args, 1, 3); err != nil {
		return nil, err
	}
	if err := noKwargs("endswith", kwargs); err != nil {
		return nil, err
	}
	candidates, err := affixCandidates("endswith", args[0])
	if err != nil {
		return nil, err
	}
	target, err := affixTarget(s.Value, "endswith", args)
	if err != nil {
		return nil, err
	}
	for _, c := range candidates {
		if strings.HasSuffix(target, c) {
			return PyTrue, nil
		}
	}
	return PyFalse, nil
}

// affixTarget applies the optional start/end arguments of startswith/endswith,
// which are code-point indices interpreted like slice bounds.
func affixTarget(value, method string, args []Object) (string, error) {
	if len(args) == 1 {
		return value, nil
	}
	runes := []rune(value)
	start, end, err := resolveSearchBounds(method, args[1:], len(runes))
	if err != nil {
		return "", err
	}
	return string(runes[start:end]), nil
}

// ============================================================
// Searching (Requirements 8.12, 8.13)
// ============================================================

// resolveSearchBounds converts optional start/end arguments into clamped rune
// offsets, following the same rules slices use for negative and out-of-range
// values. args holds the start/end values only.
func resolveSearchBounds(method string, args []Object, length int) (int, int, error) {
	start, end := 0, length

	if len(args) >= 1 && !IsNone(args[0]) {
		n, err := intArg(method, args[0])
		if err != nil {
			return 0, 0, err
		}
		start = clampSearchIndex(n, length)
	}
	if len(args) >= 2 && !IsNone(args[1]) {
		n, err := intArg(method, args[1])
		if err != nil {
			return 0, 0, err
		}
		end = clampSearchIndex(n, length)
	}
	if start > end {
		// An inverted window selects nothing.
		start = end
	}
	return start, end, nil
}

// clampSearchIndex resolves a possibly negative index and clamps it to
// [0, length].
func clampSearchIndex(i, length int) int {
	if i < 0 {
		i += length
		if i < 0 {
			return 0
		}
		return i
	}
	if i > length {
		return length
	}
	return i
}

func strMethodFind(s *StrObject, args []Object, kwargs map[string]Object) (Object, error) {
	if err := checkArgRange("find", args, 1, 3); err != nil {
		return nil, err
	}
	if err := noKwargs("find", kwargs); err != nil {
		return nil, err
	}
	sub, err := strArg("find", 1, args[0])
	if err != nil {
		return nil, err
	}

	runes := []rune(s.Value)
	start, end, err := resolveSearchBounds("find", args[1:], len(runes))
	if err != nil {
		return nil, err
	}

	window := string(runes[start:end])
	byteIdx := strings.Index(window, sub)
	if byteIdx < 0 {
		return IntFrom(-1), nil
	}
	// Convert the byte offset within the window to a code-point index within
	// the whole string, which is what Python reports.
	return IntFrom(int64(start + utf8.RuneCountInString(window[:byteIdx]))), nil
}

// ============================================================
// Formatting (Requirements 8.14, 8.15)
// ============================================================

// strMethodFormat implements str.format for the placeholder forms in the
// requirements: automatic ({}), indexed ({0}) and named ({name}), plus the
// {{ and }} escapes. Format specifications and conversions ({:>8}, {!r}) are
// rejected rather than silently ignored, since ignoring them would produce
// wrong output.
func strMethodFormat(s *StrObject, args []Object, kwargs map[string]Object) (Object, error) {
	var out strings.Builder
	value := s.Value

	autoIndex := 0
	usedAuto := false
	usedManual := false

	for i := 0; i < len(value); {
		c, size := utf8.DecodeRuneInString(value[i:])

		switch c {
		case '{':
			// "{{" is a literal brace.
			if i+size < len(value) && value[i+size] == '{' {
				out.WriteByte('{')
				i += size + 1
				continue
			}
			closing := strings.IndexByte(value[i:], '}')
			if closing < 0 {
				return nil, NewValueError("Single '{' encountered in format string")
			}
			field := value[i+size : i+closing]
			i += closing + 1

			if strings.ContainsAny(field, ":!") {
				return nil, NewValueError(
					"format specifications and conversions are not supported: '{%s}'", field)
			}

			var replacement Object
			switch {
			case field == "":
				// Automatic numbering.
				if usedManual {
					return nil, NewValueError(
						"cannot switch from manual field specification to automatic field numbering")
				}
				usedAuto = true
				if autoIndex >= len(args) {
					return nil, NewIndexError(fmt.Sprintf(
						"Replacement index %d out of range for positional args tuple", autoIndex))
				}
				replacement = args[autoIndex]
				autoIndex++
			case isAllDigits(field):
				if usedAuto {
					return nil, NewValueError(
						"cannot switch from automatic field numbering to manual field specification")
				}
				usedManual = true
				idx, err := parseFieldIndex(field)
				if err != nil {
					return nil, err
				}
				if idx < 0 || idx >= len(args) {
					return nil, NewIndexError(fmt.Sprintf(
						"Replacement index %d out of range for positional args tuple", idx))
				}
				replacement = args[idx]
			default:
				val, ok := kwargs[field]
				if !ok {
					return nil, NewKeyError(StrFrom(field))
				}
				replacement = val
			}
			out.WriteString(replacement.String())

		case '}':
			// "}}" is a literal brace; a lone '}' is an error.
			if i+size < len(value) && value[i+size] == '}' {
				out.WriteByte('}')
				i += size + 1
				continue
			}
			return nil, NewValueError("Single '}' encountered in format string")

		default:
			out.WriteRune(c)
			i += size
		}
	}

	return StrFrom(out.String()), nil
}

// isAllDigits reports whether s is a non-empty run of ASCII digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// parseFieldIndex converts a digit-only field name to an index. Absurdly large
// indices are reported as out of range rather than overflowing.
func parseFieldIndex(field string) (int, error) {
	idx := 0
	for _, c := range field {
		idx = idx*10 + int(c-'0')
		if idx > 1<<20 {
			return 0, NewIndexError(
				"Replacement index out of range for positional args tuple")
		}
	}
	return idx, nil
}
