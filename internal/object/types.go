package object

// TypeObject represents a Python type (class).
// Both built-in types (int, str, list) and user-defined classes
// are *TypeObject values.
//
// TypeObject itself implements Object — types are first-class Python values.
type TypeObject struct {
	name  string
	bases []*TypeObject
	mro   []*TypeObject  // computed C3 linearization; nil until computed
	dict  *DictObject    // type's attribute namespace

	// Slots hold Go-level implementations of Python protocols.
	// When a slot is nil, the default behavior (or AttributeError) applies.
	slots TypeSlots
}

// TypeSlots holds optional Go-level implementations of Python protocols.
// This mirrors CPython's PyTypeObject slot structure.
type TypeSlots struct {
	// Object creation
	New  func(typ *TypeObject, args []Object, kwargs map[string]Object) (Object, error)
	Init func(self Object, args []Object, kwargs map[string]Object) error

	// Representation
	Repr func(self Object) (string, error)
	Str  func(self Object) (string, error)

	// Hashing
	Hash func(self Object) (int64, error)

	// Truth value
	Bool func(self Object) (bool, error)

	// Calling
	Call func(self Object, args []Object, kwargs map[string]Object) (Object, error)

	// Attribute access
	GetAttr func(self Object, name string) (Object, error)
	SetAttr func(self Object, name string, value Object) error
	DelAttr func(self Object, name string) error

	// Rich comparison
	RichCmp func(self, other Object, op CompareOp) (Object, error)

	// Iteration
	Iter func(self Object) (Object, error)
	Next func(self Object) (Object, error)

	// Length
	Len func(self Object) (int, error)

	// Subscript
	GetItem func(self Object, key Object) (Object, error)
	SetItem func(self Object, key Object, value Object) error
	DelItem func(self Object, key Object) error

	// Numeric operations
	Add      func(a, b Object) (Object, error)
	Sub      func(a, b Object) (Object, error)
	Mul      func(a, b Object) (Object, error)
	MatMul   func(a, b Object) (Object, error)
	Div      func(a, b Object) (Object, error)
	FloorDiv func(a, b Object) (Object, error)
	Mod      func(a, b Object) (Object, error)
	Pow      func(a, b Object) (Object, error)
	LShift   func(a, b Object) (Object, error)
	RShift   func(a, b Object) (Object, error)
	BitAnd   func(a, b Object) (Object, error)
	BitOr    func(a, b Object) (Object, error)
	BitXor   func(a, b Object) (Object, error)
	Neg      func(self Object) (Object, error)
	Pos      func(self Object) (Object, error)
	Abs      func(self Object) (Object, error)
	Invert   func(self Object) (Object, error)

	// In-place numeric operations (augmented assignment)
	IAdd      func(a, b Object) (Object, error)
	ISub      func(a, b Object) (Object, error)
	IMul      func(a, b Object) (Object, error)
	IDiv      func(a, b Object) (Object, error)
	IFloorDiv func(a, b Object) (Object, error)
	IMod      func(a, b Object) (Object, error)
	IPow      func(a, b Object) (Object, error)

	// Reflected operations (right-side operand)
	RAdd      func(a, b Object) (Object, error)
	RSub      func(a, b Object) (Object, error)
	RMul      func(a, b Object) (Object, error)
	RDiv      func(a, b Object) (Object, error)
	RFloorDiv func(a, b Object) (Object, error)
	RMod      func(a, b Object) (Object, error)
	RPow      func(a, b Object) (Object, error)

	// Container protocol
	Contains func(self, item Object) (bool, error)

	// Context manager
	Enter func(self Object) (Object, error)
	Exit  func(self Object, excType, excVal, excTb Object) (bool, error)
}

// Type implements Object.
func (t *TypeObject) Type() *TypeObject { return TypeType }

// String implements Object.
func (t *TypeObject) String() string { return "<class '" + t.name + "'>" }

// Repr implements Object.
func (t *TypeObject) Repr() string { return t.String() }

// Name returns the type's name.
func (t *TypeObject) Name() string { return t.name }

// Bases returns the type's direct base types.
func (t *TypeObject) Bases() []*TypeObject { return t.bases }

// MRO returns the Method Resolution Order for this type.
// It is computed lazily on first access.
func (t *TypeObject) MRO() []*TypeObject {
	if t.mro == nil {
		t.mro = computeMRO(t)
	}
	return t.mro
}

// Dict returns the type's attribute dictionary.
func (t *TypeObject) Dict() *DictObject { return t.dict }

// GetAttr implements attribute access on a type itself (e.g. ClassName.attr).
// classmethods bind to the class; staticmethods and plain functions are
// returned unbound; other attributes pass through.
func (t *TypeObject) GetAttr(name string) (Object, error) {
	attr := lookupInMRO(t, name)
	if attr == nil {
		return nil, NewAttributeError(t.name, name)
	}
	switch a := attr.(type) {
	case *ClassMethodObject:
		return &BoundMethodObject{Self: t, Function: a.Func}, nil
	case *StaticMethodObject:
		return a.Func, nil
	default:
		return attr, nil
	}
}

// SetAttr sets a class-level attribute.
func (t *TypeObject) SetAttr(name string, value Object) error {
	return t.dict.SetItem(StrFrom(name), value)
}

// DelAttr deletes a class-level attribute.
func (t *TypeObject) DelAttr(name string) error {
	return t.dict.DelItem(StrFrom(name))
}

// Slots returns the type's protocol slot table.
func (t *TypeObject) Slots() *TypeSlots { return &t.slots }

// IsSubtype reports whether t is a subtype of other (t == other or t inherits from other).
func (t *TypeObject) IsSubtype(other *TypeObject) bool {
	for _, base := range t.MRO() {
		if base == other {
			return true
		}
	}
	return false
}

// computeMRO computes the C3 linearization of a type's MRO.
// See https://www.python.org/download/releases/2.3/mro/ for the algorithm.
func computeMRO(t *TypeObject) []*TypeObject {
	if len(t.bases) == 0 {
		return []*TypeObject{t}
	}

	// Build the input sequences: linearizations of each base + the bases list.
	seqs := make([][]*TypeObject, len(t.bases)+1)
	for i, base := range t.bases {
		seqs[i] = base.MRO()
	}
	seqs[len(t.bases)] = append([]*TypeObject{}, t.bases...)

	result := []*TypeObject{t}
	for {
		// Remove empty sequences.
		var nonEmpty [][]*TypeObject
		for _, s := range seqs {
			if len(s) > 0 {
				nonEmpty = append(nonEmpty, s)
			}
		}
		if len(nonEmpty) == 0 {
			break
		}

		// Find the first head that does not appear in any tail.
		var chosen *TypeObject
		for _, s := range nonEmpty {
			candidate := s[0]
			inTail := false
			for _, other := range nonEmpty {
				for _, tail := range other[1:] {
					if tail == candidate {
						inTail = true
						break
					}
				}
				if inTail {
					break
				}
			}
			if !inTail {
				chosen = candidate
				break
			}
		}

		if chosen == nil {
			// MRO cannot be computed — inconsistent hierarchy.
			// Return a best-effort result; the runtime will report an error.
			break
		}

		result = append(result, chosen)
		// Remove chosen from the head of each sequence.
		for i, s := range seqs {
			if len(s) > 0 && s[0] == chosen {
				seqs[i] = s[1:]
			}
		}
	}

	return result
}

// newTypeObject constructs a new TypeObject with the given name, bases, and slots.
func newTypeObject(name string, bases []*TypeObject, slots TypeSlots) *TypeObject {
	return &TypeObject{
		name:  name,
		bases: bases,
		dict:  NewDict(),
		slots: slots,
	}
}

// ============================================================
// Well-known type objects (singletons)
// ============================================================
// These are initialized in init() to avoid circular initialization.
// TypeType must be its own type: type(type) == type.

var (
	// TypeType is the type of all type objects.
	TypeType *TypeObject

	// ObjectType is the base type of all Python objects.
	ObjectType *TypeObject

	// NoneType is the type of None.
	NoneType *TypeObject

	// BoolType is the type of True and False.
	BoolType *TypeObject

	// IntType is the type of integer objects.
	IntType *TypeObject

	// FloatType is the type of float objects.
	FloatType *TypeObject

	// StrType is the type of string objects.
	StrType *TypeObject

	// BytesType is the type of bytes objects.
	BytesType *TypeObject

	// ListType is the type of list objects.
	ListType *TypeObject

	// TupleType is the type of tuple objects.
	TupleType *TypeObject

	// DictType is the type of dict objects.
	DictType *TypeObject

	// SetType is the type of set objects.
	SetType *TypeObject

	// FrozenSetType is the type of frozenset objects.
	FrozenSetType *TypeObject

	// SliceType is the type of slice objects.
	SliceType *TypeObject

	// FunctionType is the type of user-defined functions.
	FunctionType *TypeObject

	// BuiltinFunctionType is the type of built-in functions.
	BuiltinFunctionType *TypeObject

	// MethodType is the type of bound methods.
	MethodType *TypeObject

	// ModuleType is the type of module objects.
	ModuleType *TypeObject

	// BaseExceptionType is the base type for all exceptions.
	BaseExceptionType *TypeObject

	// ExceptionType is the base type for non-system exceptions.
	ExceptionType *TypeObject

	// GeneratorType is the type of generator objects.
	GeneratorType *TypeObject
)

func init() {
	// Bootstrap the type system. TypeType is its own type. Both carry a dict
	// so MRO walks (lookupInMRO) never dereference a nil namespace.
	ObjectType = &TypeObject{name: "object", bases: nil, dict: NewDict()}
	TypeType = &TypeObject{name: "type", bases: []*TypeObject{ObjectType}, dict: NewDict()}
	// TypeType is its own type — circular reference.
	// This is handled by the Type() method returning TypeType unconditionally
	// for *TypeObject values.

	NoneType = newTypeObject("NoneType", []*TypeObject{ObjectType}, TypeSlots{})
	BoolType = newTypeObject("bool", []*TypeObject{/* IntType — set after IntType init */}, TypeSlots{})
	IntType = newTypeObject("int", []*TypeObject{ObjectType}, TypeSlots{})
	FloatType = newTypeObject("float", []*TypeObject{ObjectType}, TypeSlots{})
	StrType = newTypeObject("str", []*TypeObject{ObjectType}, TypeSlots{})
	BytesType = newTypeObject("bytes", []*TypeObject{ObjectType}, TypeSlots{})
	ListType = newTypeObject("list", []*TypeObject{ObjectType}, TypeSlots{})
	TupleType = newTypeObject("tuple", []*TypeObject{ObjectType}, TypeSlots{})
	DictType = newTypeObject("dict", []*TypeObject{ObjectType}, TypeSlots{})
	SetType = newTypeObject("set", []*TypeObject{ObjectType}, TypeSlots{})
	FrozenSetType = newTypeObject("frozenset", []*TypeObject{ObjectType}, TypeSlots{})
	SliceType = newTypeObject("slice", []*TypeObject{ObjectType}, TypeSlots{})
	FunctionType = newTypeObject("function", []*TypeObject{ObjectType}, TypeSlots{})
	BuiltinFunctionType = newTypeObject("builtin_function_or_method", []*TypeObject{ObjectType}, TypeSlots{})
	MethodType = newTypeObject("method", []*TypeObject{ObjectType}, TypeSlots{})
	ModuleType = newTypeObject("module", []*TypeObject{ObjectType}, TypeSlots{})
	BaseExceptionType = newTypeObject("BaseException", []*TypeObject{ObjectType}, TypeSlots{})
	ExceptionType = newTypeObject("Exception", []*TypeObject{BaseExceptionType}, TypeSlots{})
	GeneratorType = newTypeObject("generator", []*TypeObject{ObjectType}, TypeSlots{})

	// Fix up BoolType base after IntType is initialized.
	BoolType.bases = []*TypeObject{IntType}

	// Build the exception hierarchy now that ExceptionType/BaseExceptionType
	// exist (defined above in this same init).
	installExceptionTypes()

	// Wire numeric/comparison slots now that all type objects exist.
	installCoreSlots()
}
