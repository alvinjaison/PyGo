package object

// SuperObject implements super(): a proxy that resolves attributes starting
// from the class *after* a given class in the instance's MRO. This supports
// cooperative method calls to a base-class implementation.
type SuperObject struct {
	// startAfter is the class whose position in the MRO the lookup begins
	// after (i.e. the class in which the calling method is defined).
	startAfter *TypeObject
	// instance is the bound object (self).
	instance Object
	// instanceType is the MRO to search (type(self)).
	instanceType *TypeObject
}

// NewSuper builds a super proxy. startAfter is the defining class; instance is
// self; instanceType is type(self).
func NewSuper(startAfter *TypeObject, instance Object, instanceType *TypeObject) *SuperObject {
	return &SuperObject{startAfter: startAfter, instance: instance, instanceType: instanceType}
}

func (s *SuperObject) Type() *TypeObject { return superType }
func (s *SuperObject) String() string {
	return "<super: " + s.startAfter.Name() + ">"
}
func (s *SuperObject) Repr() string { return s.String() }

// GetAttr resolves an attribute in the MRO after startAfter, binding methods
// to the original instance.
func (s *SuperObject) GetAttr(name string) (Object, error) {
	mro := s.instanceType.MRO()

	// Find the position of startAfter in the MRO, then search subsequent types.
	start := 0
	for i, t := range mro {
		if t == s.startAfter {
			start = i + 1
			break
		}
	}

	for _, t := range mro[start:] {
		if v, err := t.dict.GetItem(StrFrom(name)); err == nil {
			// Bind functions/descriptors to the original instance.
			switch attr := v.(type) {
			case *FunctionObject:
				return &BoundMethodObject{Self: s.instance, Function: attr}, nil
			case *ClassMethodObject:
				return &BoundMethodObject{Self: s.instanceType, Function: attr.Func}, nil
			case *StaticMethodObject:
				return attr.Func, nil
			case *PropertyObject:
				return attr.get(s.instance)
			default:
				return v, nil
			}
		}
	}
	return nil, NewAttributeError("super", name)
}

func (s *SuperObject) SetAttr(string, Object) error {
	return NewTypeError("cannot set attributes on a super object")
}
func (s *SuperObject) DelAttr(string) error {
	return NewTypeError("cannot delete attributes on a super object")
}

var superType = newTypeObject("super", []*TypeObject{ObjectType}, TypeSlots{})

// FrameContext describes the currently-executing Python function frame, used
// by zero-argument super() to discover the defining class and the instance.
type FrameContext struct {
	// Self is the frame's first local argument (the instance), or nil.
	Self Object
	// DefiningClass is the class whose method body is currently executing.
	DefiningClass *TypeObject
}

// frameContextHook returns the current frame context, installed by the VM.
var frameContextHook func() *FrameContext

// SetFrameContextHook installs the hook used by zero-argument super().
func SetFrameContextHook(h func() *FrameContext) { frameContextHook = h }

// CurrentFrameContext returns the executing frame's context, or nil.
func CurrentFrameContext() *FrameContext {
	if frameContextHook == nil {
		return nil
	}
	return frameContextHook()
}

// SuperType exposes the super type object.
func SuperType() *TypeObject { return superType }
