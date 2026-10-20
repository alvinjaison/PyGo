package stdlib

import (
	"regexp"
	"strings"

	"github.com/pygo/pygo/internal/object"
)

// buildRe constructs the `re` module.
//
// PyGo's regular expressions are backed by Go's regexp package (RE2 syntax).
// This covers the common Python patterns but does not support backreferences
// or lookaround, which RE2 omits by design. Divergences are documented in
// docs/compatibility.md.
func buildRe(name string) *object.ModuleObject {
	mod, set := newModule(name)

	set("compile", fn("re.compile", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		pat, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		return compilePattern(pat)
	}))

	set("match", fn("re.match", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		p, s, err := patternAndText(args)
		if err != nil {
			return nil, err
		}
		return p.matchAt(s, true), nil
	}))

	set("search", fn("re.search", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		p, s, err := patternAndText(args)
		if err != nil {
			return nil, err
		}
		return p.matchAt(s, false), nil
	}))

	set("fullmatch", fn("re.fullmatch", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		p, s, err := patternAndText(args)
		if err != nil {
			return nil, err
		}
		return p.fullmatch(s), nil
	}))

	set("findall", fn("re.findall", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		p, s, err := patternAndText(args)
		if err != nil {
			return nil, err
		}
		return p.findall(s), nil
	}))

	set("sub", fn("re.sub", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		// sub(pattern, repl, string)
		pat, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		repl, err := argStr(args, 1)
		if err != nil {
			return nil, err
		}
		s, err := argStr(args, 2)
		if err != nil {
			return nil, err
		}
		p, err := compilePattern(pat)
		if err != nil {
			return nil, err
		}
		// Translate Python backrefs (\1) to Go's ($1) form.
		return object.StrFrom(p.re.ReplaceAllString(s, translateRepl(repl))), nil
	}))

	set("split", fn("re.split", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		pat, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		s, err := argStr(args, 1)
		if err != nil {
			return nil, err
		}
		p, err := compilePattern(pat)
		if err != nil {
			return nil, err
		}
		parts := p.re.Split(s, -1)
		return strListToObject(parts), nil
	}))

	return mod
}

// ---- Pattern object ----

type rePattern struct {
	re  *regexp.Regexp
	src string
}

func compilePattern(pat string) (*rePattern, error) {
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, object.NewException(object.TypeValueError, "re.error: "+err.Error())
	}
	return &rePattern{re: re, src: pat}, nil
}

func (p *rePattern) Type() *object.TypeObject { return object.ObjectType }
func (p *rePattern) String() string           { return "re.compile('" + p.src + "')" }
func (p *rePattern) Repr() string              { return p.String() }

// GetAttr exposes pattern methods: match, search, findall, sub, split, pattern.
func (p *rePattern) GetAttr(name string) (object.Object, error) {
	switch name {
	case "pattern":
		return object.StrFrom(p.src), nil
	case "match":
		return fn("Pattern.match", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			s, err := argStr(args, 0)
			if err != nil {
				return nil, err
			}
			return p.matchAt(s, true), nil
		}), nil
	case "search":
		return fn("Pattern.search", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			s, err := argStr(args, 0)
			if err != nil {
				return nil, err
			}
			return p.matchAt(s, false), nil
		}), nil
	case "fullmatch":
		return fn("Pattern.fullmatch", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			s, err := argStr(args, 0)
			if err != nil {
				return nil, err
			}
			return p.fullmatch(s), nil
		}), nil
	case "findall":
		return fn("Pattern.findall", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			s, err := argStr(args, 0)
			if err != nil {
				return nil, err
			}
			return p.findall(s), nil
		}), nil
	case "sub":
		return fn("Pattern.sub", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			repl, err := argStr(args, 0)
			if err != nil {
				return nil, err
			}
			s, err := argStr(args, 1)
			if err != nil {
				return nil, err
			}
			return object.StrFrom(p.re.ReplaceAllString(s, translateRepl(repl))), nil
		}), nil
	case "split":
		return fn("Pattern.split", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			s, err := argStr(args, 0)
			if err != nil {
				return nil, err
			}
			return strListToObject(p.re.Split(s, -1)), nil
		}), nil
	}
	return nil, object.NewAttributeError("Pattern", name)
}

func (p *rePattern) SetAttr(string, object.Object) error {
	return object.NewTypeError("'Pattern' object attributes are read-only")
}
func (p *rePattern) DelAttr(string) error {
	return object.NewTypeError("'Pattern' object attributes are read-only")
}

// matchAt returns a Match object, or None. If anchored, the match must begin
// at index 0 (re.match semantics); otherwise it may start anywhere (re.search).
func (p *rePattern) matchAt(s string, anchored bool) object.Object {
	loc := p.re.FindStringSubmatchIndex(s)
	if loc == nil {
		return object.None
	}
	if anchored && loc[0] != 0 {
		return object.None
	}
	return newMatch(p.re, s, loc)
}

func (p *rePattern) fullmatch(s string) object.Object {
	loc := p.re.FindStringSubmatchIndex(s)
	if loc == nil || loc[0] != 0 || loc[1] != len(s) {
		return object.None
	}
	return newMatch(p.re, s, loc)
}

// findall mirrors Python: with no groups, returns whole matches; with one
// group, returns that group's captures; with multiple groups, returns tuples.
func (p *rePattern) findall(s string) object.Object {
	ngroups := p.re.NumSubexp()
	all := p.re.FindAllStringSubmatch(s, -1)
	var out []object.Object
	for _, m := range all {
		switch {
		case ngroups == 0:
			out = append(out, object.StrFrom(m[0]))
		case ngroups == 1:
			out = append(out, object.StrFrom(m[1]))
		default:
			tuple := make([]object.Object, ngroups)
			for i := 1; i <= ngroups; i++ {
				tuple[i-1] = object.StrFrom(m[i])
			}
			out = append(out, object.NewTuple(tuple))
		}
	}
	return object.NewList(out)
}

// ---- Match object ----

type reMatch struct {
	re   *regexp.Regexp
	text string
	loc  []int // submatch index pairs from FindStringSubmatchIndex
}

func newMatch(re *regexp.Regexp, text string, loc []int) *reMatch {
	return &reMatch{re: re, text: text, loc: loc}
}

func (m *reMatch) Type() *object.TypeObject { return object.ObjectType }
func (m *reMatch) String() string {
	return "<re.Match object; span=(" + itoa(m.loc[0]) + ", " + itoa(m.loc[1]) + "), match='" + m.groupString(0) + "'>"
}
func (m *reMatch) Repr() string { return m.String() }

// GetAttr exposes match methods: group, groups, start, end, span.
func (m *reMatch) GetAttr(name string) (object.Object, error) {
	switch name {
	case "group":
		return fn("Match.group", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			if len(args) == 0 {
				return object.StrFrom(m.groupString(0)), nil
			}
			if len(args) == 1 {
				n, err := argInt(args, 0)
				if err != nil {
					return nil, err
				}
				return m.group(int(n))
			}
			// Multiple indices → tuple.
			out := make([]object.Object, len(args))
			for i := range args {
				n, err := argInt(args, i)
				if err != nil {
					return nil, err
				}
				g, err := m.group(int(n))
				if err != nil {
					return nil, err
				}
				out[i] = g
			}
			return object.NewTuple(out), nil
		}), nil
	case "groups":
		return fn("Match.groups", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			ng := m.re.NumSubexp()
			out := make([]object.Object, ng)
			for i := 1; i <= ng; i++ {
				g, _ := m.group(i)
				out[i-1] = g
			}
			return object.NewTuple(out), nil
		}), nil
	case "start":
		return fn("Match.start", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			idx := 0
			if len(args) > 0 {
				n, _ := argInt(args, 0)
				idx = int(n)
			}
			return object.IntFrom(int64(m.loc[2*idx])), nil
		}), nil
	case "end":
		return fn("Match.end", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
			idx := 0
			if len(args) > 0 {
				n, _ := argInt(args, 0)
				idx = int(n)
			}
			return object.IntFrom(int64(m.loc[2*idx+1])), nil
		}), nil
	case "span":
		return fn("Match.span", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			return object.NewTuple([]object.Object{
				object.IntFrom(int64(m.loc[0])),
				object.IntFrom(int64(m.loc[1])),
			}), nil
		}), nil
	}
	return nil, object.NewAttributeError("Match", name)
}

func (m *reMatch) SetAttr(string, object.Object) error {
	return object.NewTypeError("'Match' object attributes are read-only")
}
func (m *reMatch) DelAttr(string) error {
	return object.NewTypeError("'Match' object attributes are read-only")
}

func (m *reMatch) group(n int) (object.Object, error) {
	if n < 0 || 2*n+1 >= len(m.loc) {
		return nil, object.NewException(object.TypeIndexError, "no such group")
	}
	start, end := m.loc[2*n], m.loc[2*n+1]
	if start < 0 {
		return object.None, nil // group did not participate
	}
	return object.StrFrom(m.text[start:end]), nil
}

func (m *reMatch) groupString(n int) string {
	if 2*n+1 >= len(m.loc) {
		return ""
	}
	start, end := m.loc[2*n], m.loc[2*n+1]
	if start < 0 {
		return ""
	}
	return m.text[start:end]
}

// ---- helpers ----

func patternAndText(args []object.Object) (*rePattern, string, error) {
	if len(args) < 2 {
		return nil, "", object.NewTypeError("expected (pattern, string)")
	}
	s, err := argStr(args, 1)
	if err != nil {
		return nil, "", err
	}
	// The pattern may be a compiled Pattern or a string.
	if p, ok := args[0].(*rePattern); ok {
		return p, s, nil
	}
	pat, err := argStr(args, 0)
	if err != nil {
		return nil, "", err
	}
	p, err := compilePattern(pat)
	if err != nil {
		return nil, "", err
	}
	return p, s, nil
}

func argStr(args []object.Object, i int) (string, error) {
	if i >= len(args) {
		return "", object.NewTypeError("missing string argument")
	}
	s, ok := args[i].(*object.StrObject)
	if !ok {
		return "", object.NewTypeError("expected str, got '%s'", args[i].Type().Name())
	}
	return s.Value, nil
}

// translateRepl converts Python-style backreferences (\1, \g<name>) in a
// replacement string into Go's $1 / ${name} form.
func translateRepl(repl string) string {
	var b strings.Builder
	runes := []rune(repl)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c == '\\' && i+1 < len(runes) && runes[i+1] >= '0' && runes[i+1] <= '9' {
			b.WriteString("${")
			i++
			for i < len(runes) && runes[i] >= '0' && runes[i] <= '9' {
				b.WriteRune(runes[i])
				i++
			}
			i--
			b.WriteString("}")
			continue
		}
		if c == '$' {
			b.WriteString("$$") // escape literal $ for Go's Expand
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
