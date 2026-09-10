package importer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/alvinjaison/pygo/internal/object"
)

// TypeJSONDecodeError is the exception raised by json.loads on malformed input.
// CPython makes it a subclass of ValueError, so scripts that catch ValueError
// still catch decode failures. We mirror that by raising a ValueError-typed
// exception; the module also exposes the name json.JSONDecodeError for scripts
// that reference it explicitly.
//
// The module provides a curated subset of CPython's json:
//   - loads(s)                    parse a JSON string into PyGo objects
//   - dumps(obj, indent=None, sort_keys=False)   serialize PyGo objects to JSON
//   - JSONDecodeError             the decode-failure exception type
//
// It intentionally does not implement load/dump (file objects), custom
// encoders/decoders, or the object_hook family yet. Those are additive and can
// follow without changing this surface.

// NewJSONModule creates the built-in json module.
func NewJSONModule() *object.ModuleObject {
	mod := object.NewModule("json")

	_ = mod.SetAttr("loads", object.NewBuiltin("loads", jsonLoads))
	_ = mod.SetAttr("dumps", object.NewBuiltin("dumps", jsonDumps))

	// Expose the decode-error type so scripts can reference json.JSONDecodeError.
	// It is a ValueError subclass, matching CPython.
	_ = mod.SetAttr("JSONDecodeError", object.TypeValueError)

	return mod
}

// ---------------------------------------------------------------------------
// loads
// ---------------------------------------------------------------------------

func jsonLoads(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("loads() takes exactly 1 argument (%d given)", len(args))
	}
	src, ok := args[0].(*object.StrObject)
	if !ok {
		return nil, object.NewTypeError(
			"the JSON object must be str, not '%s'", args[0].Type().Name())
	}

	// Use a Decoder with UseNumber so integers do not silently become float64,
	// which matters for release variables that carry large ids or version codes.
	dec := json.NewDecoder(strings.NewReader(src.Value))
	dec.UseNumber()

	var raw interface{}
	if err := dec.Decode(&raw); err != nil {
		return nil, object.NewValueError("%s", cleanJSONError(err))
	}
	// Reject trailing content, as CPython's json.loads does.
	if dec.More() {
		return nil, object.NewValueError("Extra data")
	}

	return goToPy(raw)
}

// goToPy converts a value produced by encoding/json into a PyGo object.
func goToPy(v interface{}) (object.Object, error) {
	switch t := v.(type) {
	case nil:
		return object.None, nil
	case bool:
		return object.BoolFrom(t), nil
	case string:
		return object.StrFrom(t), nil
	case json.Number:
		return numberToPy(t)
	case []interface{}:
		items := make([]object.Object, len(t))
		for i, elem := range t {
			pv, err := goToPy(elem)
			if err != nil {
				return nil, err
			}
			items[i] = pv
		}
		return object.NewList(items), nil
	case map[string]interface{}:
		d := object.NewDict()
		// encoding/json decodes objects into a map, losing source order. Sort
		// keys so output is deterministic; CPython preserves insertion order,
		// but a JSON object has no guaranteed order to preserve anyway.
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			pv, err := goToPy(t[k])
			if err != nil {
				return nil, err
			}
			if err := d.SetItem(object.StrFrom(k), pv); err != nil {
				return nil, err
			}
		}
		return d, nil
	default:
		return nil, object.NewValueError("unsupported JSON value of Go type %T", v)
	}
}

// numberToPy turns a json.Number into an int or float PyGo object, preserving
// integer precision for values that do not fit int64 via big.Int.
func numberToPy(n json.Number) (object.Object, error) {
	s := string(n)
	if !strings.ContainsAny(s, ".eE") {
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return object.IntFrom(i), nil
		}
		if bi, ok := new(big.Int).SetString(s, 10); ok {
			return object.IntFromBig(bi), nil
		}
	}
	f, err := n.Float64()
	if err != nil {
		return nil, object.NewValueError("invalid number literal: %s", s)
	}
	return object.FloatFrom(f), nil
}

// cleanJSONError rewrites Go's json error text into something closer to what a
// Python developer expects, without pretending to match CPython exactly.
func cleanJSONError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "unexpected end of JSON input"):
		return "Expecting value: unexpected end of input"
	case strings.HasPrefix(msg, "invalid character"):
		return "Expecting value: " + msg
	default:
		return msg
	}
}

// ---------------------------------------------------------------------------
// dumps
// ---------------------------------------------------------------------------

func jsonDumps(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("dumps() takes exactly 1 positional argument (%d given)", len(args))
	}

	indent, err := indentFromKwarg(kwargs["indent"])
	if err != nil {
		return nil, err
	}
	sortKeys := false
	if sk, ok := kwargs["sort_keys"]; ok {
		b, terr := object.IsTruthy(sk)
		if terr != nil {
			return nil, terr
		}
		sortKeys = b
	}

	enc := &jsonEncoder{indent: indent, sortKeys: sortKeys}
	if err := enc.encode(args[0], 0); err != nil {
		return nil, err
	}
	return object.StrFrom(enc.buf.String()), nil
}

// indentFromKwarg interprets the indent keyword: None means compact output, an
// int means that many spaces per level. A string indent (CPython allows it) is
// used verbatim.
func indentFromKwarg(v object.Object) (string, error) {
	if v == nil || object.IsNone(v) {
		return "", nil
	}
	switch t := v.(type) {
	case *object.IntObject:
		n, ok := t.Int64()
		if !ok || n < 0 {
			n = 0
		}
		return strings.Repeat(" ", int(n)), nil
	case *object.StrObject:
		return t.Value, nil
	default:
		return "", object.NewTypeError("indent must be int, str or None, not '%s'", v.Type().Name())
	}
}

type jsonEncoder struct {
	buf      bytes.Buffer
	indent   string // one level of indentation, "" means compact
	sortKeys bool
}

func (e *jsonEncoder) pretty() bool { return e.indent != "" }

func (e *jsonEncoder) newline(level int) {
	if !e.pretty() {
		return
	}
	e.buf.WriteByte('\n')
	e.buf.WriteString(strings.Repeat(e.indent, level))
}

func (e *jsonEncoder) encode(obj object.Object, level int) error {
	switch v := obj.(type) {
	case *object.NoneObject:
		e.buf.WriteString("null")
	case *object.BoolObject:
		if v.Value {
			e.buf.WriteString("true")
		} else {
			e.buf.WriteString("false")
		}
	case *object.IntObject:
		e.buf.WriteString(v.String())
	case *object.FloatObject:
		e.encodeFloat(v.Value)
	case *object.StrObject:
		e.encodeString(v.Value)
	case *object.ListObject:
		return e.encodeSeq(v.Items, level)
	case *object.TupleObject:
		return e.encodeSeq(v.Items, level)
	case *object.DictObject:
		return e.encodeDict(v, level)
	default:
		return object.NewTypeError("Object of type %s is not JSON serializable", obj.Type().Name())
	}
	return nil
}

func (e *jsonEncoder) encodeFloat(f float64) {
	// encoding/json rejects NaN and Inf; CPython's json emits NaN/Infinity.
	// Match CPython for compatibility with scripts that round-trip such values.
	switch {
	case math.IsNaN(f):
		e.buf.WriteString("NaN")
	case math.IsInf(f, 1):
		e.buf.WriteString("Infinity")
	case math.IsInf(f, -1):
		e.buf.WriteString("-Infinity")
	default:
		e.buf.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
	}
}

func (e *jsonEncoder) encodeString(s string) {
	// json.Marshal on a string produces a correctly escaped JSON string.
	// It never fails for a string input, so the error is safe to drop.
	b, _ := json.Marshal(s)
	e.buf.Write(b)
}

func (e *jsonEncoder) encodeSeq(items []object.Object, level int) error {
	if len(items) == 0 {
		e.buf.WriteString("[]")
		return nil
	}
	e.buf.WriteByte('[')
	for i, item := range items {
		if i > 0 {
			e.writeItemSep()
		}
		e.newline(level + 1)
		if err := e.encode(item, level+1); err != nil {
			return err
		}
	}
	e.newline(level)
	e.buf.WriteByte(']')
	return nil
}

// writeItemSep writes the separator between array/object members. CPython's
// default is ", " in compact mode; in pretty mode the trailing space is
// dropped because a newline follows.
func (e *jsonEncoder) writeItemSep() {
	if e.pretty() {
		e.buf.WriteByte(',')
	} else {
		e.buf.WriteString(", ")
	}
}

func (e *jsonEncoder) encodeDict(d *object.DictObject, level int) error {
	items := d.Items()
	if len(items) == 0 {
		e.buf.WriteString("{}")
		return nil
	}
	if e.sortKeys {
		sort.SliceStable(items, func(i, j int) bool {
			return dictKeyString(items[i].Key) < dictKeyString(items[j].Key)
		})
	}

	e.buf.WriteByte('{')
	for i, it := range items {
		if i > 0 {
			e.writeItemSep()
		}
		e.newline(level + 1)

		key, err := jsonKey(it.Key)
		if err != nil {
			return err
		}
		e.encodeString(key)
		// CPython's default key separator is ": " (with the space) in both
		// compact and pretty modes.
		e.buf.WriteString(": ")
		if err := e.encode(it.Value, level+1); err != nil {
			return err
		}
	}
	e.newline(level)
	e.buf.WriteByte('}')
	return nil
}

// jsonKey converts a dict key to the string form JSON requires. CPython allows
// str, int, float, bool and None keys, coercing the non-string ones.
func jsonKey(k object.Object) (string, error) {
	switch v := k.(type) {
	case *object.StrObject:
		return v.Value, nil
	case *object.IntObject:
		return v.String(), nil
	case *object.BoolObject:
		if v.Value {
			return "true", nil
		}
		return "false", nil
	case *object.FloatObject:
		return strconv.FormatFloat(v.Value, 'g', -1, 64), nil
	case *object.NoneObject:
		return "null", nil
	default:
		return "", object.NewTypeError("keys must be str, int, float, bool or None, not %s", k.Type().Name())
	}
}

// dictKeyString is a best-effort ordering key for sort_keys. Non-string keys
// sort by their string form.
func dictKeyString(k object.Object) string {
	if s, ok := k.(*object.StrObject); ok {
		return s.Value
	}
	return fmt.Sprint(k.String())
}
