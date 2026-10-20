package stdlib

import (
	"strings"

	"github.com/pygo/pygo/internal/object"
)

// namedTupleConstructor implements collections.namedtuple(typename, field_names).
// field_names may be a single space/comma-separated string or a list of names.
// It returns a factory (itself callable) that builds named-tuple instances.
func namedTupleConstructor(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) < 2 {
		return nil, object.NewTypeError("namedtuple expected at least 2 arguments")
	}
	typeName, err := argStr(args, 0)
	if err != nil {
		return nil, err
	}

	var fields []string
	switch f := args[1].(type) {
	case *object.StrObject:
		normalized := strings.ReplaceAll(f.Value, ",", " ")
		fields = strings.Fields(normalized)
	default:
		items, err := drain(args[1])
		if err != nil {
			return nil, object.NewTypeError("namedtuple field_names must be a string or iterable")
		}
		for _, it := range items {
			s, ok := it.(*object.StrObject)
			if !ok {
				return nil, object.NewTypeError("namedtuple field names must be strings")
			}
			fields = append(fields, s.Value)
		}
	}

	return &NamedTupleFactory{typeName: typeName, fields: fields}, nil
}

// NamedTupleFactory is the class-like object returned by namedtuple(). Calling
// it builds a NamedTupleInstance.
type NamedTupleFactory struct {
	typeName string
	fields   []string
}

func (f *NamedTupleFactory) Type() *object.TypeObject { return object.TypeType }
func (f *NamedTupleFactory) String() string           { return "<class '" + f.typeName + "'>" }
func (f *NamedTupleFactory) Repr() string              { return f.String() }

// Call constructs an instance from positional (or keyword) field values.
func (f *NamedTupleFactory) Call(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	values := make([]object.Object, len(f.fields))

	// Positional first.
	for i := range args {
		if i >= len(f.fields) {
			return nil, object.NewTypeError("__new__() takes %d positional arguments but %d were given",
				len(f.fields), len(args))
		}
		values[i] = args[i]
	}
	// Keyword fills remaining.
	for i := len(args); i < len(f.fields); i++ {
		if v, ok := kwargs[f.fields[i]]; ok {
			values[i] = v
		} else {
			return nil, object.NewTypeError("__new__() missing argument: '%s'", f.fields[i])
		}
	}
	return &NamedTupleInstance{factory: f, values: values}, nil
}

// GetAttr exposes _fields on the factory.
func (f *NamedTupleFactory) GetAttr(name string) (object.Object, error) {
	switch name {
	case "_fields":
		items := make([]object.Object, len(f.fields))
		for i, fld := range f.fields {
			items[i] = object.StrFrom(fld)
		}
		return object.NewTuple(items), nil
	case "__name__":
		return object.StrFrom(f.typeName), nil
	}
	return nil, object.NewAttributeError(f.typeName, name)
}
func (f *NamedTupleFactory) SetAttr(string, object.Object) error {
	return object.NewTypeError("cannot set attributes on a namedtuple type")
}
func (f *NamedTupleFactory) DelAttr(string) error {
	return object.NewTypeError("cannot delete attributes on a namedtuple type")
}

// NamedTupleInstance is an immutable record with positional and named access.
type NamedTupleInstance struct {
	factory *NamedTupleFactory
	values  []object.Object
}

func (nt *NamedTupleInstance) Type() *object.TypeObject { return object.TupleType }
func (nt *NamedTupleInstance) String() string           { return nt.Repr() }
func (nt *NamedTupleInstance) Repr() string {
	var b strings.Builder
	b.WriteString(nt.factory.typeName)
	b.WriteByte('(')
	for i, fld := range nt.factory.fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fld)
		b.WriteByte('=')
		b.WriteString(nt.values[i].Repr())
	}
	b.WriteByte(')')
	return b.String()
}
func (nt *NamedTupleInstance) Len() int              { return len(nt.values) }
func (nt *NamedTupleInstance) IsTrue() (bool, error) { return len(nt.values) > 0, nil }

// GetItem supports positional indexing (nt[0]).
func (nt *NamedTupleInstance) GetItem(key object.Object) (object.Object, error) {
	return object.NewTuple(nt.values).GetItem(key)
}

// Iter iterates over the field values.
func (nt *NamedTupleInstance) Iter() (object.Iterator, error) {
	return object.NewTuple(nt.values).Iter()
}

// GetAttr supports named field access (nt.fieldname) and _asdict/_fields.
func (nt *NamedTupleInstance) GetAttr(name string) (object.Object, error) {
	for i, fld := range nt.factory.fields {
		if fld == name {
			return nt.values[i], nil
		}
	}
	switch name {
	case "_fields":
		return nt.factory.GetAttr("_fields")
	case "_asdict":
		return fn(nt.factory.typeName+"._asdict", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
			d := object.NewDict()
			for i, fld := range nt.factory.fields {
				_ = d.SetItem(object.StrFrom(fld), nt.values[i])
			}
			return d, nil
		}), nil
	}
	return nil, object.NewAttributeError(nt.factory.typeName, name)
}
func (nt *NamedTupleInstance) SetAttr(string, object.Object) error {
	return object.NewTypeError("can't set attribute on a namedtuple instance")
}
func (nt *NamedTupleInstance) DelAttr(string) error {
	return object.NewTypeError("can't delete attribute on a namedtuple instance")
}
