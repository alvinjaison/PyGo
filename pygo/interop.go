package pygo

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pygo/pygo/internal/object"
)

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// wrapGoFunction wraps a Go function value as a callable Python object.
// Supported signatures: any number of convertible parameters; return values
// may end with an error, which is raised as a Python exception.
func wrapGoFunction(name string, fn interface{}) (object.Object, error) {
	rv := reflect.ValueOf(fn)
	if rv.Kind() != reflect.Func {
		return nil, fmt.Errorf("RegisterFunction: %q is not a function (got %T)", name, fn)
	}
	ft := rv.Type()

	call := func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		numIn := ft.NumIn()
		variadic := ft.IsVariadic()

		if !variadic && len(args) != numIn {
			return nil, object.NewTypeError("%s() takes %d arguments but %d were given", name, numIn, len(args))
		}
		if variadic && len(args) < numIn-1 {
			return nil, object.NewTypeError("%s() takes at least %d arguments", name, numIn-1)
		}

		in := make([]reflect.Value, len(args))
		for i, a := range args {
			var paramType reflect.Type
			if variadic && i >= numIn-1 {
				paramType = ft.In(numIn - 1).Elem()
			} else {
				paramType = ft.In(i)
			}
			gv, err := pythonToGo(a, paramType)
			if err != nil {
				return nil, object.NewTypeError("%s() argument %d: %v", name, i+1, err)
			}
			in[i] = gv
		}

		out := rv.Call(in)
		return goResultsToPython(out, ft)
	}

	return object.NewBuiltin(name, call), nil
}

// goResultsToPython converts a Go function's return values to a Python object,
// raising the trailing error (if any) as a Python exception.
func goResultsToPython(out []reflect.Value, ft reflect.Type) (object.Object, error) {
	n := len(out)
	// Trailing error handling.
	if n > 0 && ft.Out(n-1) == errorType {
		if errVal := out[n-1]; !errVal.IsNil() {
			goErr := errVal.Interface().(error)
			return nil, goErrorToException(goErr)
		}
		out = out[:n-1]
		n--
	}
	switch n {
	case 0:
		return object.None, nil
	case 1:
		return goToPython(out[0].Interface())
	default:
		items := make([]object.Object, n)
		for i := 0; i < n; i++ {
			o, err := goToPython(out[i].Interface())
			if err != nil {
				return nil, err
			}
			items[i] = o
		}
		return object.NewTuple(items), nil
	}
}

// goErrorToException converts a Go error to a Python exception. If the error
// implements PyError, the specified exception type is used.
func goErrorToException(err error) *object.ExceptionObject {
	if pe, ok := err.(PyError); ok {
		if t := exceptionTypeByName(pe.PyExceptionType()); t != nil {
			return object.NewException(t, pe.PyMessage())
		}
	}
	return object.NewRuntimeError("%s", err.Error())
}

// PyError lets a Go error specify which Python exception type it maps to.
type PyError interface {
	error
	PyExceptionType() string
	PyMessage() string
}

func exceptionTypeByName(name string) *object.TypeObject {
	switch name {
	case "ValueError":
		return object.TypeValueError
	case "TypeError":
		return object.TypeTypeError
	case "KeyError":
		return object.TypeKeyError
	case "IndexError":
		return object.TypeIndexError
	case "RuntimeError":
		return object.TypeRuntimeError
	case "NameError":
		return object.TypeNameError
	case "AttributeError":
		return object.TypeAttributeError
	}
	return nil
}

// ============================================================
// Go struct/pointer wrapper
// ============================================================

// goObject wraps a Go struct or pointer, exposing its exported fields and
// methods as Python attributes.
type goObject struct {
	rv reflect.Value
}

func newGoObject(rv reflect.Value) *goObject {
	return &goObject{rv: rv}
}

func (g *goObject) Type() *object.TypeObject { return object.ObjectType }
func (g *goObject) String() string {
	return fmt.Sprintf("<Go %s>", g.rv.Type())
}
func (g *goObject) Repr() string { return g.String() }

// GetAttr resolves a Python attribute to a Go field or method.
func (g *goObject) GetAttr(name string) (object.Object, error) {
	// Try methods (on the value and its pointer).
	if m := g.method(name); m.IsValid() {
		return wrapGoMethod(name, m)
	}
	// Try fields on the underlying struct.
	sv := g.structValue()
	if sv.IsValid() {
		if f := fieldByName(sv, name); f.IsValid() {
			return goToPython(f.Interface())
		}
	}
	return nil, object.NewAttributeError(g.rv.Type().String(), name)
}

func (g *goObject) SetAttr(name string, value object.Object) error {
	sv := g.structValue()
	if !sv.IsValid() {
		return object.NewAttributeError(g.rv.Type().String(), name)
	}
	f := fieldByName(sv, name)
	if !f.IsValid() || !f.CanSet() {
		return object.NewAttributeError(g.rv.Type().String(), name)
	}
	gv, err := pythonToGo(value, f.Type())
	if err != nil {
		return object.NewTypeError("cannot assign to field %s: %v", name, err)
	}
	f.Set(gv)
	return nil
}

func (g *goObject) DelAttr(name string) error {
	return object.NewTypeError("cannot delete Go object attributes")
}

func (g *goObject) method(name string) reflect.Value {
	// Try exact and normalized names on value and pointer receivers.
	candidates := nameCandidates(name)
	for _, cand := range candidates {
		if m := g.rv.MethodByName(cand); m.IsValid() {
			return m
		}
		if g.rv.Kind() != reflect.Ptr && g.rv.CanAddr() {
			if m := g.rv.Addr().MethodByName(cand); m.IsValid() {
				return m
			}
		}
	}
	return reflect.Value{}
}

func (g *goObject) structValue() reflect.Value {
	v := g.rv
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		return v
	}
	return reflect.Value{}
}

func fieldByName(sv reflect.Value, name string) reflect.Value {
	for _, cand := range nameCandidates(name) {
		f := sv.FieldByName(cand)
		if f.IsValid() {
			return f
		}
	}
	return reflect.Value{}
}

// nameCandidates returns possible Go identifiers for a Python attribute name:
// the exact name, the Capitalized form, and a snake_case→CamelCase form.
func nameCandidates(name string) []string {
	cands := []string{name}
	if name != "" {
		cands = append(cands, strings.ToUpper(name[:1])+name[1:])
	}
	if strings.Contains(name, "_") {
		cands = append(cands, snakeToCamel(name))
	}
	return cands
}

func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}

// wrapGoMethod wraps a bound Go method value as a callable Python object.
func wrapGoMethod(name string, m reflect.Value) (object.Object, error) {
	mt := m.Type()
	call := func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		variadic := mt.IsVariadic()
		numIn := mt.NumIn()
		if !variadic && len(args) != numIn {
			return nil, object.NewTypeError("%s() takes %d arguments but %d were given", name, numIn, len(args))
		}
		in := make([]reflect.Value, len(args))
		for i, a := range args {
			var pt reflect.Type
			if variadic && i >= numIn-1 {
				pt = mt.In(numIn - 1).Elem()
			} else {
				pt = mt.In(i)
			}
			gv, err := pythonToGo(a, pt)
			if err != nil {
				return nil, object.NewTypeError("%s() argument %d: %v", name, i+1, err)
			}
			in[i] = gv
		}
		out := m.Call(in)
		return goResultsToPython(out, mt)
	}
	return object.NewBuiltin(name, call), nil
}
