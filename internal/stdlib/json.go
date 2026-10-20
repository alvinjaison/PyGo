package stdlib

import (
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/pygo/pygo/internal/object"
)

// buildJSON constructs the `json` module (dumps/loads).
func buildJSON(name string) *object.ModuleObject {
	mod, set := newModule(name)

	set("dumps", fn("json.dumps", func(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) < 1 {
			return nil, object.NewTypeError("dumps() missing required argument: 'obj'")
		}
		indent := -1
		if v, ok := kwargs["indent"]; ok {
			if n, ok := v.(*object.IntObject); ok {
				iv, _ := n.Int64()
				indent = int(iv)
			}
		}
		var b strings.Builder
		if err := jsonEncode(&b, args[0], indent, 0); err != nil {
			return nil, err
		}
		return object.StrFrom(b.String()), nil
	}))

	set("loads", fn("json.loads", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		s, ok := args[0].(*object.StrObject)
		if !ok {
			return nil, object.NewTypeError("loads() argument must be a string")
		}
		p := &jsonParser{src: []rune(s.Value)}
		p.skipWS()
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		p.skipWS()
		if p.pos != len(p.src) {
			return nil, object.NewValueError("Extra data in JSON")
		}
		return v, nil
	}))

	return mod
}

// ---- encoding ----

func jsonEncode(b *strings.Builder, o object.Object, indent, depth int) error {
	switch v := o.(type) {
	case *object.NoneObject:
		b.WriteString("null")
	case *object.BoolObject:
		if v.Value {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case *object.IntObject:
		b.WriteString(v.Value.String())
	case *object.FloatObject:
		b.WriteString(strconv.FormatFloat(v.Value, 'g', -1, 64))
	case *object.StrObject:
		writeJSONString(b, v.Value)
	case *object.ListObject:
		return encodeArray(b, v.Items, indent, depth)
	case *object.TupleObject:
		return encodeArray(b, v.Items, indent, depth)
	case *object.DictObject:
		return encodeObject(b, v, indent, depth)
	default:
		return object.NewTypeError("Object of type %s is not JSON serializable", o.Type().Name())
	}
	return nil
}

func encodeArray(b *strings.Builder, items []object.Object, indent, depth int) error {
	if len(items) == 0 {
		b.WriteString("[]")
		return nil
	}
	b.WriteByte('[')
	for i, it := range items {
		if i > 0 {
			b.WriteByte(',')
			if indent < 0 {
				b.WriteByte(' ') // default separator ", " in compact mode
			}
		}
		writeIndent(b, indent, depth+1)
		if err := jsonEncode(b, it, indent, depth+1); err != nil {
			return err
		}
	}
	writeIndent(b, indent, depth)
	b.WriteByte(']')
	return nil
}

func encodeObject(b *strings.Builder, d *object.DictObject, indent, depth int) error {
	keys := d.Keys()
	if len(keys) == 0 {
		b.WriteString("{}")
		return nil
	}
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
			if indent < 0 {
				b.WriteByte(' ')
			}
		}
		writeIndent(b, indent, depth+1)
		// JSON keys must be strings.
		switch kv := k.(type) {
		case *object.StrObject:
			writeJSONString(b, kv.Value)
		default:
			writeJSONString(b, k.String())
		}
		b.WriteByte(':')
		if indent >= 0 {
			b.WriteByte(' ')
		}
		val, _ := d.GetItem(k)
		if err := jsonEncode(b, val, indent, depth+1); err != nil {
			return err
		}
	}
	writeIndent(b, indent, depth)
	b.WriteByte('}')
	return nil
}

func writeIndent(b *strings.Builder, indent, depth int) {
	if indent < 0 {
		return
	}
	b.WriteByte('\n')
	b.WriteString(strings.Repeat(" ", indent*depth))
}

func writeJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\t':
			b.WriteString("\\t")
		case '\r':
			b.WriteString("\\r")
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
}

// sortedKeys is unused by default but kept for a future sort_keys option.
var _ = sort.Strings

// ---- decoding ----

type jsonParser struct {
	src []rune
	pos int
}

func (p *jsonParser) skipWS() {
	for p.pos < len(p.src) {
		switch p.src[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func (p *jsonParser) parseValue() (object.Object, error) {
	if p.pos >= len(p.src) {
		return nil, object.NewValueError("Expecting value in JSON")
	}
	c := p.src[p.pos]
	switch {
	case c == '{':
		return p.parseObject()
	case c == '[':
		return p.parseArray()
	case c == '"':
		s, err := p.parseString()
		if err != nil {
			return nil, err
		}
		return object.StrFrom(s), nil
	case c == 't' || c == 'f':
		return p.parseBool()
	case c == 'n':
		return p.parseNull()
	default:
		return p.parseNumber()
	}
}

func (p *jsonParser) parseObject() (object.Object, error) {
	d := object.NewDict()
	p.pos++ // {
	p.skipWS()
	if p.pos < len(p.src) && p.src[p.pos] == '}' {
		p.pos++
		return d, nil
	}
	for {
		p.skipWS()
		if p.pos >= len(p.src) || p.src[p.pos] != '"' {
			return nil, object.NewValueError("Expecting property name in JSON")
		}
		key, err := p.parseString()
		if err != nil {
			return nil, err
		}
		p.skipWS()
		if p.pos >= len(p.src) || p.src[p.pos] != ':' {
			return nil, object.NewValueError("Expecting ':' in JSON")
		}
		p.pos++
		p.skipWS()
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		if err := d.SetItem(object.StrFrom(key), val); err != nil {
			return nil, err
		}
		p.skipWS()
		if p.pos >= len(p.src) {
			return nil, object.NewValueError("Unterminated object in JSON")
		}
		if p.src[p.pos] == ',' {
			p.pos++
			continue
		}
		if p.src[p.pos] == '}' {
			p.pos++
			return d, nil
		}
		return nil, object.NewValueError("Expecting ',' delimiter in JSON")
	}
}

func (p *jsonParser) parseArray() (object.Object, error) {
	var items []object.Object
	p.pos++ // [
	p.skipWS()
	if p.pos < len(p.src) && p.src[p.pos] == ']' {
		p.pos++
		return object.NewList(items), nil
	}
	for {
		p.skipWS()
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		items = append(items, val)
		p.skipWS()
		if p.pos >= len(p.src) {
			return nil, object.NewValueError("Unterminated array in JSON")
		}
		if p.src[p.pos] == ',' {
			p.pos++
			continue
		}
		if p.src[p.pos] == ']' {
			p.pos++
			return object.NewList(items), nil
		}
		return nil, object.NewValueError("Expecting ',' delimiter in JSON")
	}
}

func (p *jsonParser) parseString() (string, error) {
	p.pos++ // opening quote
	var b strings.Builder
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == '"' {
			p.pos++
			return b.String(), nil
		}
		if c == '\\' && p.pos+1 < len(p.src) {
			p.pos++
			switch p.src[p.pos] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case '/':
				b.WriteByte('/')
			default:
				b.WriteRune(p.src[p.pos])
			}
			p.pos++
			continue
		}
		b.WriteRune(c)
		p.pos++
	}
	return "", object.NewValueError("Unterminated string in JSON")
}

func (p *jsonParser) parseBool() (object.Object, error) {
	if strings.HasPrefix(string(p.src[p.pos:]), "true") {
		p.pos += 4
		return object.PyTrue, nil
	}
	if strings.HasPrefix(string(p.src[p.pos:]), "false") {
		p.pos += 5
		return object.PyFalse, nil
	}
	return nil, object.NewValueError("Invalid JSON literal")
}

func (p *jsonParser) parseNull() (object.Object, error) {
	if strings.HasPrefix(string(p.src[p.pos:]), "null") {
		p.pos += 4
		return object.None, nil
	}
	return nil, object.NewValueError("Invalid JSON literal")
}

func (p *jsonParser) parseNumber() (object.Object, error) {
	start := p.pos
	isFloat := false
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c >= '0' && c <= '9' || c == '-' || c == '+' {
			p.pos++
		} else if c == '.' || c == 'e' || c == 'E' {
			isFloat = true
			p.pos++
		} else {
			break
		}
	}
	tok := string(p.src[start:p.pos])
	if tok == "" {
		return nil, object.NewValueError("Expecting value in JSON")
	}
	if isFloat {
		f, err := strconv.ParseFloat(tok, 64)
		if err != nil {
			return nil, object.NewValueError("Invalid number in JSON")
		}
		return object.FloatFrom(f), nil
	}
	bi := new(big.Int)
	if _, ok := bi.SetString(tok, 10); !ok {
		return nil, object.NewValueError("Invalid number in JSON")
	}
	return object.IntFromBig(bi), nil
}
