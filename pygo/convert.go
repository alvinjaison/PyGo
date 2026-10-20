package pygo

import (
	"fmt"
	"math/big"
	"reflect"

	"github.com/pygo/pygo/internal/object"
)

// toPublicError converts an internal error (possibly an *object.ExceptionObject)
// into a public *Error, preserving type, message, and traceback.
func toPublicError(err error) error {
	if err == nil {
		return nil
	}
	if exc, ok := err.(*object.ExceptionObject); ok {
		pe := &Error{
			Type:    exc.Type().Name(),
			Message: exc.Message,
		}
		if exc.Traceback != nil {
			for _, f := range exc.Traceback.Frames {
				pe.Traceback = append(pe.Traceback, TracebackEntry{
					Filename: f.Filename,
					FuncName: f.FuncName,
					Line:     f.Line,
				})
			}
		}
		return pe
	}
	// Parser/compiler syntax errors and others become SyntaxError-ish values.
	return &Error{Type: "SyntaxError", Message: err.Error()}
}

// goToPython converts a Go value to a Python object.
func goToPython(v interface{}) (object.Object, error) {
	if v == nil {
		return object.None, nil
	}
	switch x := v.(type) {
	case object.Object:
		return x, nil
	case bool:
		return object.BoolFrom(x), nil
	case int:
		return object.IntFrom(int64(x)), nil
	case int8:
		return object.IntFrom(int64(x)), nil
	case int16:
		return object.IntFrom(int64(x)), nil
	case int32:
		return object.IntFrom(int64(x)), nil
	case int64:
		return object.IntFrom(x), nil
	case uint:
		return object.IntFrom(int64(x)), nil
	case uint8:
		return object.IntFrom(int64(x)), nil
	case uint16:
		return object.IntFrom(int64(x)), nil
	case uint32:
		return object.IntFrom(int64(x)), nil
	case uint64:
		return object.IntFromBig(new(big.Int).SetUint64(x)), nil
	case float32:
		return object.FloatFrom(float64(x)), nil
	case float64:
		return object.FloatFrom(x), nil
	case string:
		return object.StrFrom(x), nil
	case []byte:
		return object.NewBytes(x), nil
	}

	// Reflection-based conversion for slices, maps, pointers, structs.
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		items := make([]object.Object, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			o, err := goToPython(rv.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			items[i] = o
		}
		return object.NewList(items), nil
	case reflect.Map:
		d := object.NewDict()
		iter := rv.MapRange()
		for iter.Next() {
			k, err := goToPython(iter.Key().Interface())
			if err != nil {
				return nil, err
			}
			val, err := goToPython(iter.Value().Interface())
			if err != nil {
				return nil, err
			}
			if err := d.SetItem(k, val); err != nil {
				return nil, err
			}
		}
		return d, nil
	case reflect.Ptr:
		if rv.IsNil() {
			return object.None, nil
		}
		// Struct pointer → GoObject wrapper.
		return newGoObject(rv), nil
	case reflect.Struct:
		return newGoObject(rv), nil
	case reflect.Func:
		return wrapGoFunction("<func>", v)
	}

	return nil, fmt.Errorf("cannot convert Go value of type %T to a Python object", v)
}

// pythonToGo converts a Python object into a Go value assignable to targetType.
func pythonToGo(o object.Object, target reflect.Type) (reflect.Value, error) {
	switch target.Kind() {
	case reflect.Interface:
		// interface{} — produce a natural Go value.
		gv := pythonToGoNatural(o)
		return reflect.ValueOf(gv), nil
	case reflect.Bool:
		t, _ := object.IsTruthy(o)
		if b, ok := o.(*object.BoolObject); ok {
			t = b.Value
		}
		return reflect.ValueOf(t), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := asInt64(o)
		if err != nil {
			return reflect.Value{}, err
		}
		v := reflect.New(target).Elem()
		v.SetInt(n)
		return v, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := asInt64(o)
		if err != nil {
			return reflect.Value{}, err
		}
		v := reflect.New(target).Elem()
		v.SetUint(uint64(n))
		return v, nil
	case reflect.Float32, reflect.Float64:
		f, err := asFloat64(o)
		if err != nil {
			return reflect.Value{}, err
		}
		v := reflect.New(target).Elem()
		v.SetFloat(f)
		return v, nil
	case reflect.String:
		s, ok := o.(*object.StrObject)
		if !ok {
			return reflect.Value{}, fmt.Errorf("cannot convert '%s' to string", o.Type().Name())
		}
		return reflect.ValueOf(s.Value), nil
	case reflect.Slice:
		// Fast path: a []byte target from a Python bytes value.
		if target.Elem().Kind() == reflect.Uint8 {
			if b, ok := o.(*object.BytesObject); ok {
				cp := make([]byte, len(b.Value))
				copy(cp, b.Value)
				return reflect.ValueOf(cp), nil
			}
		}
		items, err := iterate(o)
		if err != nil {
			return reflect.Value{}, err
		}
		slice := reflect.MakeSlice(target, len(items), len(items))
		for i, it := range items {
			ev, err := pythonToGo(it, target.Elem())
			if err != nil {
				return reflect.Value{}, err
			}
			slice.Index(i).Set(ev)
		}
		return slice, nil
	}
	return reflect.Value{}, fmt.Errorf("unsupported Go parameter type %s", target)
}

// pythonToGoNatural converts a Python object to the most natural Go value.
func pythonToGoNatural(o object.Object) interface{} {
	switch x := o.(type) {
	case *object.NoneObject:
		return nil
	case *object.BoolObject:
		return x.Value
	case *object.IntObject:
		if n, ok := x.Int64(); ok {
			return n
		}
		return x.Value // *big.Int
	case *object.FloatObject:
		return x.Value
	case *object.StrObject:
		return x.Value
	case *object.BytesObject:
		return x.Value
	case *object.ListObject:
		out := make([]interface{}, len(x.Items))
		for i, it := range x.Items {
			out[i] = pythonToGoNatural(it)
		}
		return out
	case *object.TupleObject:
		out := make([]interface{}, len(x.Items))
		for i, it := range x.Items {
			out[i] = pythonToGoNatural(it)
		}
		return out
	}
	return o
}

func asInt64(o object.Object) (int64, error) {
	switch x := o.(type) {
	case *object.IntObject:
		n, ok := x.Int64()
		if !ok {
			return 0, fmt.Errorf("OverflowError: int too large to convert")
		}
		return n, nil
	case *object.BoolObject:
		if x.Value {
			return 1, nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("cannot convert '%s' to int", o.Type().Name())
}

func asFloat64(o object.Object) (float64, error) {
	switch x := o.(type) {
	case *object.FloatObject:
		return x.Value, nil
	case *object.IntObject:
		f := new(big.Float).SetInt(x.Value)
		out, _ := f.Float64()
		return out, nil
	case *object.BoolObject:
		if x.Value {
			return 1, nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("cannot convert '%s' to float", o.Type().Name())
}

func iterate(o object.Object) ([]object.Object, error) {
	it, err := object.GetIter(o)
	if err != nil {
		return nil, err
	}
	var out []object.Object
	for {
		v, err := it.Next()
		if object.IsStopIteration(err) {
			break
		}
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
