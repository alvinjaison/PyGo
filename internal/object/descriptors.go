package object

// This file implements the three most common descriptors: property,
// classmethod, and staticmethod. They are returned by the built-in
// constructors of the same name (registered by the runtime) and are stored as
// class attributes. InstanceObject.GetAttr recognizes them and applies the
// descriptor protocol during attribute lookup.

// PropertyObject implements the `property` descriptor: attribute access on an
// instance invokes the getter; assignment invokes the setter.
type PropertyObject struct {
	Getter Object
	Setter Object
	Doc    string
}

func NewProperty(getter, setter Object) *PropertyObject {
	return &PropertyObject{Getter: getter, Setter: setter}
}

func (p *PropertyObject) Type() *TypeObject { return propertyType }
func (p *PropertyObject) String() string    { return "<property object>" }
func (p *PropertyObject) Repr() string       { return p.String() }

// get invokes the property getter with the instance.
func (p *PropertyObject) get(instance Object) (Object, error) {
	if p.Getter == nil || IsNone(p.Getter) {
		return nil, NewAttributeError(instance.Type().Name(), "unreadable attribute")
	}
	return Invoke(p.Getter, []Object{instance})
}

// set invokes the property setter with the instance and value.
func (p *PropertyObject) set(instance, value Object) error {
	if p.Setter == nil || IsNone(p.Setter) {
		return NewAttributeError(instance.Type().Name(), "can't set attribute")
	}
	_, err := Invoke(p.Setter, []Object{instance, value})
	return err
}

// ClassMethodObject implements `classmethod`: the wrapped function is bound to
// the class rather than the instance.
type ClassMethodObject struct {
	Func Object
}

func NewClassMethod(fn Object) *ClassMethodObject { return &ClassMethodObject{Func: fn} }

func (c *ClassMethodObject) Type() *TypeObject { return classmethodType }
func (c *ClassMethodObject) String() string    { return "<classmethod object>" }
func (c *ClassMethodObject) Repr() string       { return c.String() }

// StaticMethodObject implements `staticmethod`: the wrapped function receives
// no implicit first argument.
type StaticMethodObject struct {
	Func Object
}

func NewStaticMethod(fn Object) *StaticMethodObject { return &StaticMethodObject{Func: fn} }

func (s *StaticMethodObject) Type() *TypeObject { return staticmethodType }
func (s *StaticMethodObject) String() string    { return "<staticmethod object>" }
func (s *StaticMethodObject) Repr() string       { return s.String() }

// Descriptor type singletons.
var (
	propertyType     = newTypeObject("property", []*TypeObject{ObjectType}, TypeSlots{})
	classmethodType  = newTypeObject("classmethod", []*TypeObject{ObjectType}, TypeSlots{})
	staticmethodType = newTypeObject("staticmethod", []*TypeObject{ObjectType}, TypeSlots{})
)
