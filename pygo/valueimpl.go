package pygo

import (
	"fmt"

	"github.com/alvinjaison/PyGo/internal/object"
)

// valueImpl is the concrete implementation of the public Value interface,
// wrapping an internal object.Object.
type valueImpl struct {
	obj object.Object
}

func newValue(o object.Object) Value {
	if o == nil {
		o = object.None
	}
	return &valueImpl{obj: o}
}

func (v *valueImpl) Type() string   { return v.obj.Type().Name() }
func (v *valueImpl) String() string { return v.obj.String() }
func (v *valueImpl) Repr() string   { return v.obj.Repr() }
func (v *valueImpl) IsNone() bool   { return object.IsNone(v.obj) }

func (v *valueImpl) Int() (int64, error) {
	n, err := asInt64(v.obj)
	if err != nil {
		return 0, &Error{Type: "TypeError", Message: err.Error()}
	}
	return n, nil
}

func (v *valueImpl) Float() (float64, error) {
	f, err := asFloat64(v.obj)
	if err != nil {
		return 0, &Error{Type: "TypeError", Message: err.Error()}
	}
	return f, nil
}

func (v *valueImpl) Bool() bool {
	t, _ := object.IsTruthy(v.obj)
	return t
}

func (v *valueImpl) Str() (string, error) {
	if s, ok := v.obj.(*object.StrObject); ok {
		return s.Value, nil
	}
	return "", &Error{Type: "TypeError", Message: fmt.Sprintf("expected str, got %s", v.Type())}
}

func (v *valueImpl) Bytes() ([]byte, error) {
	if b, ok := v.obj.(*object.BytesObject); ok {
		return b.Value, nil
	}
	if s, ok := v.obj.(*object.StrObject); ok {
		return []byte(s.Value), nil
	}
	return nil, &Error{Type: "TypeError", Message: "value is not bytes"}
}

func (v *valueImpl) List() ([]Value, error) {
	items, err := iterate(v.obj)
	if err != nil {
		return nil, toPublicError(err)
	}
	out := make([]Value, len(items))
	for i, it := range items {
		out[i] = newValue(it)
	}
	return out, nil
}

func (v *valueImpl) Dict() (map[string]Value, error) {
	d, ok := v.obj.(*object.DictObject)
	if !ok {
		return nil, &Error{Type: "TypeError", Message: "value is not a dict"}
	}
	out := map[string]Value{}
	for _, key := range d.Keys() {
		ks, ok := key.(*object.StrObject)
		if !ok {
			return nil, &Error{Type: "TypeError", Message: "dict key is not a string"}
		}
		val, _ := d.GetItem(key)
		out[ks.Value] = newValue(val)
	}
	return out, nil
}

func (v *valueImpl) Unwrap() interface{} {
	if g, ok := v.obj.(*goObject); ok {
		return g.rv.Interface()
	}
	return nil
}
