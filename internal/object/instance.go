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
}

// NewInstance creates an instance of the given class.
func NewInstance(cls *TypeObject) *InstanceObject {
	return &InstanceObject{cls: cls, dict: NewDict()}
}

func (o *InstanceObject) Type() *TypeObject { return o.cls }

func (o *InstanceObject) String() string {
	// Honor a user-defined __str__ if present, else fall back to __repr__.
	if m := lookupInMRO(o.cls, "__str__"); m != nil {
		if s, ok := callUnary(m, o); ok {
			return s
		}
	}
	return o.Repr()
}

func (o *InstanceObject) Repr() string {
	if m := lookupInMRO(o.cls, "__repr__"); m != nil {
		if s, ok := callUnary(m, o); ok {
			return s
		}
	}
	return fmt.Sprintf("<%s object>", o.cls.Name())
}

// GetAttr implements instance attribute lookup: instance dict first, then the
// class MRO. Functions found on the class become bound methods.
func (o *InstanceObject) GetAttr(name string) (Object, error) {
	if v, err := o.dict.GetItem(StrFrom(name)); err == nil {
		return v, nil
	}
	if attr := lookupInMRO(o.cls, name); attr != nil {
		// Bind functions as methods.
		if fn, ok := attr.(*FunctionObject); ok {
			return &BoundMethodObject{Self: o, Function: fn}, nil
		}
		return attr, nil
	}
	return nil, NewAttributeError(o.cls.Name(), name)
}

func (o *InstanceObject) SetAttr(name string, value Object) error {
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

// callUnary is a helper that calls a zero-arg (self-only) method and returns
// its string result. It is used for __str__/__repr__ fallbacks where invoking
// the VM is not available; functions are not executed here (they require the
// VM), so this returns false for FunctionObject and only handles builtins.
func callUnary(m Object, self Object) (string, bool) {
	if bm, ok := m.(*BuiltinFuncObject); ok {
		res, err := bm.Fn([]Object{self}, nil)
		if err == nil {
			if s, ok := res.(*StrObject); ok {
				return s.Value, true
			}
		}
	}
	// User-defined __str__/__repr__ require the VM; handled at a higher level.
	return "", false
}
