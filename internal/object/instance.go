package object

import "fmt"

// NewClass creates a user-defined class (a *TypeObject) from a name, base
// types, and a namespace dict produced by executing the class body.
func NewClass(name string, bases []*TypeObject, ns *DictObject) *TypeObject {
	t := newTypeObject(name, bases, TypeSlots{})
	// Copy the namespace entries into the type's dict.
	for _, key := range ns.Keys() {
		val, _ := ns.GetItem(key)
		_ = t.dict.SetItem(key, val)
	}
	return t
}

// InstanceObject is an instance of a user-defined class.
type InstanceObject struct {
	cls  *TypeObject
	dict *DictObject
	id   int64 // unique id for identity hashing / id()
}

// instanceCounter assigns a unique id to each created instance.
var instanceCounter int64

// NewInstance creates an instance of the given class.
func NewInstance(cls *TypeObject) *InstanceObject {
	instanceCounter++
	return &InstanceObject{cls: cls, dict: NewDict(), id: instanceCounter}
}

func (o *InstanceObject) Type() *TypeObject { return o.cls }

func (o *InstanceObject) String() string {
	// Honor a user-defined __str__ if present, else fall back to __repr__.
	if s, ok := o.callStrDunder("__str__"); ok {
		return s
	}
	return o.Repr()
}

func (o *InstanceObject) Repr() string {
	if s, ok := o.callStrDunder("__repr__"); ok {
		return s
	}
	return fmt.Sprintf("<%s object>", o.cls.Name())
}

// callStrDunder invokes a user-defined __str__/__repr__ (via the VM) and
// returns its string result. Returns ("", false) if the method is absent or
// does not return a string.
func (o *InstanceObject) callStrDunder(name string) (string, bool) {
	m := lookupDunder(o, name)
	if m == nil {
		return "", false
	}
	res, err := callDunder(m)
	if err != nil {
		return "", false
	}
	if s, ok := res.(*StrObject); ok {
		return s.Value, true
	}
	return "", false
}

// ---- Protocol interfaces backed by dunder methods ----

// IsTrue implements truthiness via __bool__, then __len__, else true.
func (o *InstanceObject) IsTrue() (bool, error) {
	if m := lookupDunder(o, "__bool__"); m != nil {
		res, err := callDunder(m)
		if err != nil {
			return false, err
		}
		if b, ok := res.(*BoolObject); ok {
			return b.Value, nil
		}
		return IsTruthy(res)
	}
	if m := lookupDunder(o, "__len__"); m != nil {
		res, err := callDunder(m)
		if err != nil {
			return false, err
		}
		if n, ok := res.(*IntObject); ok {
			v, _ := n.Int64()
			return v != 0, nil
		}
	}
	return true, nil
}

// Len implements len() via __len__.
func (o *InstanceObject) Len() int {
	if m := lookupDunder(o, "__len__"); m != nil {
		res, err := callDunder(m)
		if err == nil {
			if n, ok := res.(*IntObject); ok {
				v, _ := n.Int64()
				return int(v)
			}
		}
	}
	return 0
}

// HasLen reports whether the instance defines __len__.
func (o *InstanceObject) HasLen() bool { return hasDunder(o, "__len__") }

// GetItem implements obj[key] via __getitem__.
func (o *InstanceObject) GetItem(key Object) (Object, error) {
	m := lookupDunder(o, "__getitem__")
	if m == nil {
		return nil, NewTypeError("'%s' object is not subscriptable", o.cls.Name())
	}
	return callDunder(m, key)
}

// SetItem implements obj[key] = value via __setitem__.
func (o *InstanceObject) SetItem(key, value Object) error {
	m := lookupDunder(o, "__setitem__")
	if m == nil {
		return NewTypeError("'%s' object does not support item assignment", o.cls.Name())
	}
	_, err := callDunder(m, key, value)
	return err
}

// DelItem implements del obj[key] via __delitem__.
func (o *InstanceObject) DelItem(key Object) error {
	m := lookupDunder(o, "__delitem__")
	if m == nil {
		return NewTypeError("'%s' object doesn't support item deletion", o.cls.Name())
	}
	_, err := callDunder(m, key)
	return err
}

// Contains implements the `in` operator via __contains__.
// Callers should check HasContains first; containsItem handles the
// iteration fallback when __contains__ is absent.
func (o *InstanceObject) Contains(item Object) (bool, error) {
	m := lookupDunder(o, "__contains__")
	if m == nil {
		return false, NewTypeError("argument of type '%s' is not a container", o.cls.Name())
	}
	res, err := callDunder(m, item)
	if err != nil {
		return false, err
	}
	return IsTruthy(res)
}

// HasContains reports whether the instance defines __contains__.
func (o *InstanceObject) HasContains() bool { return hasDunder(o, "__contains__") }

// Iter implements iteration via __iter__ (returning an iterator) and falls back
// to the __getitem__/IndexError sequence protocol.
func (o *InstanceObject) Iter() (Iterator, error) {
	if m := lookupDunder(o, "__iter__"); m != nil {
		res, err := callDunder(m)
		if err != nil {
			return nil, err
		}
		if it, ok := res.(Iterator); ok {
			return it, nil
		}
		// The returned object must itself support __next__.
		return &dunderIterator{obj: res}, nil
	}
	if hasDunder(o, "__next__") {
		return &dunderIterator{obj: o}, nil
	}
	return nil, NewTypeError("'%s' object is not iterable", o.cls.Name())
}

// Next implements the iterator protocol via __next__.
func (o *InstanceObject) Next() (Object, error) {
	m := lookupDunder(o, "__next__")
	if m == nil {
		return nil, NewTypeError("'%s' object is not an iterator", o.cls.Name())
	}
	res, err := callDunder(m)
	if err != nil {
		// A StopIteration exception ends iteration.
		if exc, ok := err.(*ExceptionObject); ok && exc.Type().IsSubtype(TypeStopIteration) {
			return nil, ErrStopIteration
		}
		return nil, err
	}
	return res, nil
}

// Call implements calling an instance via __call__.
func (o *InstanceObject) Call(args []Object, kwargs map[string]Object) (Object, error) {
	m := lookupDunder(o, "__call__")
	if m == nil {
		return nil, NewTypeError("'%s' object is not callable", o.cls.Name())
	}
	return Invoke(m, args)
}

// Hash implements hash() via __hash__.
func (o *InstanceObject) Hash() (int64, error) {
	if m := lookupDunder(o, "__hash__"); m != nil {
		res, err := callDunder(m)
		if err != nil {
			return 0, err
		}
		if n, ok := res.(*IntObject); ok {
			v, _ := n.Int64()
			return v, nil
		}
	}
	// Default: identity-based hash (stable per object).
	return identityHash(o), nil
}

// dunderIterator wraps an object exposing __next__ as a Go Iterator.
type dunderIterator struct{ obj Object }

func (it *dunderIterator) Type() *TypeObject { return it.obj.Type() }
func (it *dunderIterator) String() string    { return "<iterator>" }
func (it *dunderIterator) Repr() string       { return it.String() }
func (it *dunderIterator) Next() (Object, error) {
	m := lookupInMRO(it.obj.Type(), "__next__")
	if m == nil {
		return nil, ErrStopIteration
	}
	bound := m
	if fn, ok := m.(*FunctionObject); ok {
		bound = &BoundMethodObject{Self: it.obj, Function: fn}
	}
	res, err := Invoke(bound, nil)
	if err != nil {
		if exc, ok := err.(*ExceptionObject); ok && exc.Type().IsSubtype(TypeStopIteration) {
			return nil, ErrStopIteration
		}
		return nil, err
	}
	return res, nil
}

// GetAttr implements instance attribute lookup following Python's protocol:
// data descriptors on the type win over the instance dict, then the instance
// dict, then non-data descriptors and plain class attributes.
func (o *InstanceObject) GetAttr(name string) (Object, error) {
	clsAttr := lookupInMRO(o.cls, name)

	// Data descriptors (property with a getter) take precedence over the
	// instance dict.
	if prop, ok := clsAttr.(*PropertyObject); ok {
		return prop.get(o)
	}

	// Instance dict.
	if v, err := o.dict.GetItem(StrFrom(name)); err == nil {
		return v, nil
	}

	if clsAttr == nil {
		return nil, NewAttributeError(o.cls.Name(), name)
	}

	// Apply the descriptor protocol / method binding to class attributes.
	switch attr := clsAttr.(type) {
	case *FunctionObject:
		return &BoundMethodObject{Self: o, Function: attr}, nil
	case *ClassMethodObject:
		return &BoundMethodObject{Self: o.cls, Function: attr.Func}, nil
	case *StaticMethodObject:
		return attr.Func, nil
	default:
		return clsAttr, nil
	}
}

func (o *InstanceObject) SetAttr(name string, value Object) error {
	// A property with a setter intercepts assignment.
	if clsAttr := lookupInMRO(o.cls, name); clsAttr != nil {
		if prop, ok := clsAttr.(*PropertyObject); ok {
			return prop.set(o, value)
		}
	}
	return o.dict.SetItem(StrFrom(name), value)
}

func (o *InstanceObject) DelAttr(name string) error {
	return o.dict.DelItem(StrFrom(name))
}

// Dict exposes the instance's attribute dictionary.
func (o *InstanceObject) Dict() *DictObject { return o.dict }

// lookupInMRO searches a type's MRO for an attribute, returning nil if absent.
func lookupInMRO(t *TypeObject, name string) Object {
	for _, base := range t.MRO() {
		if v, err := base.dict.GetItem(StrFrom(name)); err == nil {
			return v
		}
	}
	return nil
}

// LookupInMRO is the exported form used by the VM for method resolution.
func LookupInMRO(t *TypeObject, name string) (Object, bool) {
	v := lookupInMRO(t, name)
	return v, v != nil
}



// identityHash returns a stable per-object hash. Instances carry a unique id
// assigned at creation, used as the default __hash__ when no user-defined one
// exists. This mirrors CPython's default identity-based object hashing without
// requiring unsafe pointer access.
func identityHash(o Object) int64 {
	if inst, ok := o.(*InstanceObject); ok {
		return inst.id
	}
	return 0
}

// seqIterator implements the legacy sequence iteration protocol: it calls
// __getitem__ with 0, 1, 2, … until an IndexError is raised.
type seqIterator struct {
	obj   Object
	index int64
}

func newSeqIterator(obj Object) *seqIterator {
	return &seqIterator{obj: obj}
}

func (it *seqIterator) Type() *TypeObject { return it.obj.Type() }
func (it *seqIterator) String() string    { return "<sequence iterator>" }
func (it *seqIterator) Repr() string       { return it.String() }
func (it *seqIterator) Next() (Object, error) {
	m := lookupDunder(it.obj, "__getitem__")
	if m == nil {
		return nil, ErrStopIteration
	}
	res, err := callDunder(m, IntFrom(it.index))
	if err != nil {
		if exc, ok := err.(*ExceptionObject); ok && exc.Type().IsSubtype(TypeIndexError) {
			return nil, ErrStopIteration
		}
		return nil, err
	}
	it.index++
	return res, nil
}
