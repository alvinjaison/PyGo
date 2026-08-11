package object

import (
	"fmt"
	"strings"

	"github.com/pygo/pygo/internal/bytecode"
)

// ============================================================
// FunctionObject
// ============================================================

// FunctionObject represents a Python function defined with `def`.
type FunctionObject struct {
	Code       *bytecode.Code
	Globals    *DictObject
	Defaults   []Object            // default positional argument values
	KwDefaults map[string]Object   // keyword-only argument defaults
	Closure    []*CellObject       // free variable cells
	Name       string
	QualName   string
	Doc        string
	Annotations map[string]Object  // parameter and return annotations
}

func (f *FunctionObject) Type() *TypeObject { return FunctionType }
func (f *FunctionObject) String() string    { return fmt.Sprintf("<function %s>", f.Name) }
func (f *FunctionObject) Repr() string      { return f.String() }

// Call implements the Callable interface.
// The actual frame creation and execution is delegated to the VM.
// This method panics if called directly — the VM must handle function calls
// by detecting *FunctionObject and creating a new frame.
func (f *FunctionObject) Call(_ []Object, _ map[string]Object) (Object, error) {
	// This should not be reached in normal operation.
	// The VM detects *FunctionObject and handles it specially.
	panic("FunctionObject.Call called directly — use VM frame dispatch")
}

// ============================================================
// BuiltinFuncObject
// ============================================================

// BuiltinFuncObject represents a built-in function implemented in Go.
type BuiltinFuncObject struct {
	Name string
	Doc  string
	Fn   func(args []Object, kwargs map[string]Object) (Object, error)
}

func (b *BuiltinFuncObject) Type() *TypeObject { return BuiltinFunctionType }
func (b *BuiltinFuncObject) String() string    { return fmt.Sprintf("<built-in function %s>", b.Name) }
func (b *BuiltinFuncObject) Repr() string      { return b.String() }
func (b *BuiltinFuncObject) Call(args []Object, kwargs map[string]Object) (Object, error) {
	return b.Fn(args, kwargs)
}

// NewBuiltin creates a BuiltinFuncObject.
func NewBuiltin(name string, fn func(args []Object, kwargs map[string]Object) (Object, error)) *BuiltinFuncObject {
	return &BuiltinFuncObject{Name: name, Fn: fn}
}

// ============================================================
// BoundMethodObject
// ============================================================

// BoundMethodObject represents a method bound to an instance (instance.method).
type BoundMethodObject struct {
	Self     Object
	Function Object // *FunctionObject or *BuiltinFuncObject
}

func (m *BoundMethodObject) Type() *TypeObject { return MethodType }
func (m *BoundMethodObject) String() string {
	return fmt.Sprintf("<bound method %s of %s>", methodName(m.Function), m.Self.Repr())
}
func (m *BoundMethodObject) Repr() string { return m.String() }
func (m *BoundMethodObject) Call(args []Object, kwargs map[string]Object) (Object, error) {
	// Prepend self to args.
	fullArgs := make([]Object, len(args)+1)
	fullArgs[0] = m.Self
	copy(fullArgs[1:], args)

	if c, ok := m.Function.(Callable); ok {
		return c.Call(fullArgs, kwargs)
	}
	return nil, fmt.Errorf("TypeError: '%s' is not callable", m.Function.Type().Name())
}

func methodName(fn Object) string {
	switch f := fn.(type) {
	case *FunctionObject:
		return f.Name
	case *BuiltinFuncObject:
		return f.Name
	}
	return "?"
}

// ============================================================
// CellObject
// ============================================================

// CellObject holds a free variable shared between a function and its closure.
// Both the outer function's local scope and the inner function's free variable
// list hold a reference to the same *CellObject.
type CellObject struct {
	Value Object // nil if not yet bound
}

func (c *CellObject) Type() *TypeObject { return ObjectType } // placeholder
func (c *CellObject) String() string    { return fmt.Sprintf("<cell at %p>", c) }
func (c *CellObject) Repr() string      { return c.String() }

// IsEmpty reports whether the cell has not yet been bound to a value.
func (c *CellObject) IsEmpty() bool { return c.Value == nil }

// ============================================================
// ModuleObject
// ============================================================

// ModuleObject represents a Python module.
type ModuleObject struct {
	Name   string
	Dict   *DictObject // module's global namespace
	Loader string      // for __spec__
}

func NewModule(name string) *ModuleObject {
	m := &ModuleObject{Name: name, Dict: NewDict()}
	// Set __name__ in the module's dict.
	_ = m.Dict.SetItem(StrFrom("__name__"), StrFrom(name))
	return m
}

func (m *ModuleObject) Type() *TypeObject { return ModuleType }
func (m *ModuleObject) String() string    { return fmt.Sprintf("<module '%s'>", m.Name) }
func (m *ModuleObject) Repr() string      { return m.String() }

func (m *ModuleObject) GetAttr(name string) (Object, error) {
	return m.Dict.GetItem(StrFrom(name))
}

func (m *ModuleObject) SetAttr(name string, value Object) error {
	return m.Dict.SetItem(StrFrom(name), value)
}

func (m *ModuleObject) DelAttr(name string) error {
	return m.Dict.DelItem(StrFrom(name))
}

// ============================================================
// ExceptionObject
// ============================================================

// ExceptionObject represents a Python exception.
type ExceptionObject struct {
	ExcType   *TypeObject
	Message   string
	Args      *TupleObject
	Traceback *TracebackObject
	Cause     *ExceptionObject // __cause__ (from x from y)
	Context   *ExceptionObject // __context__ (implicit chaining)
}

func NewException(excType *TypeObject, message string) *ExceptionObject {
	return &ExceptionObject{
		ExcType: excType,
		Message: message,
		Args:    NewTuple([]Object{StrFrom(message)}),
	}
}

func (e *ExceptionObject) Type() *TypeObject { return e.ExcType }
func (e *ExceptionObject) String() string    { return e.Message }
func (e *ExceptionObject) Repr() string {
	return fmt.Sprintf("%s(%s)", e.ExcType.Name(), e.Message)
}

// Error implements the Go error interface, allowing ExceptionObject to be
// used directly as a Go error where convenient.
func (e *ExceptionObject) Error() string {
	return fmt.Sprintf("%s: %s", e.ExcType.Name(), e.Message)
}

// TracebackObject holds the call stack captured when an exception was raised.
type TracebackObject struct {
	Frames []TracebackFrame
}

func (t *TracebackObject) Type() *TypeObject { return ObjectType }
func (t *TracebackObject) String() string    { return "<traceback>" }
func (t *TracebackObject) Repr() string      { return t.String() }

// TracebackFrame is one frame in a Python traceback.
type TracebackFrame struct {
	Filename string
	FuncName string
	Line     int
}

// Format returns the traceback as a human-readable string matching CPython's
// traceback format.
func (t *TracebackObject) Format() string {
	var sb strings.Builder
	sb.WriteString("Traceback (most recent call last):\n")
	for _, f := range t.Frames {
		sb.WriteString(fmt.Sprintf("  File %q, line %d, in %s\n", f.Filename, f.Line, f.FuncName))
	}
	return sb.String()
}

// ============================================================
// Well-known exception type singletons
// ============================================================

var (
	TypeStopIteration    *TypeObject
	TypeValueError       *TypeObject
	TypeTypeError        *TypeObject
	TypeNameError        *TypeObject
	TypeAttributeError   *TypeObject
	TypeIndexError       *TypeObject
	TypeKeyError         *TypeObject
	TypeZeroDivisionError *TypeObject
	TypeRuntimeError     *TypeObject
	TypeRecursionError   *TypeObject
	TypeNotImplementedError *TypeObject
	TypeOverflowError    *TypeObject
	TypeSyntaxError      *TypeObject
	TypeImportError      *TypeObject
	TypeModuleNotFoundError *TypeObject
	TypeOSError          *TypeObject
	TypeSystemExit       *TypeObject
	TypeKeyboardInterrupt *TypeObject
	TypeGeneratorExit    *TypeObject
	TypeAssertionError   *TypeObject
	TypeUnboundLocalError *TypeObject
)

func init() {
	arithType := newTypeObject("ArithmeticError", []*TypeObject{ExceptionType}, TypeSlots{})
	lookupType := newTypeObject("LookupError", []*TypeObject{ExceptionType}, TypeSlots{})
	nameErrType := newTypeObject("NameError", []*TypeObject{ExceptionType}, TypeSlots{})

	TypeStopIteration = newTypeObject("StopIteration", []*TypeObject{ExceptionType}, TypeSlots{})
	TypeValueError = newTypeObject("ValueError", []*TypeObject{ExceptionType}, TypeSlots{})
	TypeTypeError = newTypeObject("TypeError", []*TypeObject{ExceptionType}, TypeSlots{})
	TypeNameError = nameErrType
	TypeAttributeError = newTypeObject("AttributeError", []*TypeObject{ExceptionType}, TypeSlots{})
	TypeIndexError = newTypeObject("IndexError", []*TypeObject{lookupType}, TypeSlots{})
	TypeKeyError = newTypeObject("KeyError", []*TypeObject{lookupType}, TypeSlots{})
	TypeZeroDivisionError = newTypeObject("ZeroDivisionError", []*TypeObject{arithType}, TypeSlots{})
	TypeRuntimeError = newTypeObject("RuntimeError", []*TypeObject{ExceptionType}, TypeSlots{})
	TypeRecursionError = newTypeObject("RecursionError", []*TypeObject{TypeRuntimeError}, TypeSlots{})
	TypeNotImplementedError = newTypeObject("NotImplementedError", []*TypeObject{TypeRuntimeError}, TypeSlots{})
	TypeOverflowError = newTypeObject("OverflowError", []*TypeObject{arithType}, TypeSlots{})
	TypeSyntaxError = newTypeObject("SyntaxError", []*TypeObject{ExceptionType}, TypeSlots{})
	TypeImportError = newTypeObject("ImportError", []*TypeObject{ExceptionType}, TypeSlots{})
	TypeModuleNotFoundError = newTypeObject("ModuleNotFoundError", []*TypeObject{TypeImportError}, TypeSlots{})
	TypeOSError = newTypeObject("OSError", []*TypeObject{ExceptionType}, TypeSlots{})
	TypeSystemExit = newTypeObject("SystemExit", []*TypeObject{BaseExceptionType}, TypeSlots{})
	TypeKeyboardInterrupt = newTypeObject("KeyboardInterrupt", []*TypeObject{BaseExceptionType}, TypeSlots{})
	TypeGeneratorExit = newTypeObject("GeneratorExit", []*TypeObject{BaseExceptionType}, TypeSlots{})
	TypeAssertionError = newTypeObject("AssertionError", []*TypeObject{ExceptionType}, TypeSlots{})
	TypeUnboundLocalError = newTypeObject("UnboundLocalError", []*TypeObject{nameErrType}, TypeSlots{})
}

// NewTypeError creates a TypeError exception.
func NewTypeError(msg string, args ...interface{}) *ExceptionObject {
	return NewException(TypeTypeError, fmt.Sprintf(msg, args...))
}

// NewValueError creates a ValueError exception.
func NewValueError(msg string, args ...interface{}) *ExceptionObject {
	return NewException(TypeValueError, fmt.Sprintf(msg, args...))
}

// NewNameError creates a NameError exception.
func NewNameError(name string) *ExceptionObject {
	return NewException(TypeNameError, fmt.Sprintf("name '%s' is not defined", name))
}

// NewAttributeError creates an AttributeError exception.
func NewAttributeError(typeName, attr string) *ExceptionObject {
	return NewException(TypeAttributeError, fmt.Sprintf("'%s' object has no attribute '%s'", typeName, attr))
}

// NewIndexError creates an IndexError exception.
func NewIndexError(msg string) *ExceptionObject {
	return NewException(TypeIndexError, msg)
}

// NewKeyError creates a KeyError exception.
func NewKeyError(key Object) *ExceptionObject {
	return NewException(TypeKeyError, key.Repr())
}

// NewZeroDivisionError creates a ZeroDivisionError.
func NewZeroDivisionError() *ExceptionObject {
	return NewException(TypeZeroDivisionError, "division by zero")
}

// NewRuntimeError creates a RuntimeError exception.
func NewRuntimeError(msg string, args ...interface{}) *ExceptionObject {
	return NewException(TypeRuntimeError, fmt.Sprintf(msg, args...))
}

// IsInstance reports whether obj is an instance of excType or a subtype of it.
func IsInstance(obj Object, excType *TypeObject) bool {
	return obj.Type().IsSubtype(excType)
}
