package vm

import (
	"github.com/pygo/pygo/internal/compiler"
	"github.com/pygo/pygo/internal/object"
)

// CallExternal invokes a callable object from outside the VM (from Go host
// code via the runtime). It sets up a throwaway frame context for the call.
func (v *VM) CallExternal(callee object.Object, args []object.Object, globals, builtins *object.DictObject) (object.Object, error) {
	caller := &Frame{Globals: globals, Builtins: builtins}
	return v.callObject(caller, callee, args, nil)
}

// findListBelow returns the nearest ListObject on the value stack below the
// current top. Used by LIST_APPEND in comprehensions, where the accumulator
// list sits beneath the loop's iterator and value.
func (f *Frame) findListBelow() *object.ListObject {
	for i := f.StackTop - 1; i >= 0; i-- {
		if lst, ok := f.Stack[i].(*object.ListObject); ok {
			return lst
		}
	}
	return nil
}

// extractKwNames recovers the keyword-argument names from the constant the
// compiler pushed before CALL_FUNCTION_KW.
func extractKwNames(o object.Object) []string {
	raw, ok := o.(*rawConstant)
	if !ok {
		return nil
	}
	names, _ := compiler.KwNames(raw.value)
	return names
}

// excMatches reports whether a raised exception matches an except clause's
// type (which may be a type or a tuple of types).
func excMatches(exc, clause object.Object) bool {
	e, ok := exc.(*object.ExceptionObject)
	if !ok {
		return false
	}
	switch c := clause.(type) {
	case *object.TypeObject:
		return e.Type().IsSubtype(c)
	case *object.TupleObject:
		for _, item := range c.Items {
			if t, ok := item.(*object.TypeObject); ok && e.Type().IsSubtype(t) {
				return true
			}
		}
	}
	return false
}

// instantiate creates an instance of a class type and runs its __init__.
// Calling an exception type produces an ExceptionObject instead.
func (v *VM) instantiate(caller *Frame, cls *object.TypeObject, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	// Exception types: build an ExceptionObject with the first arg as message.
	if cls.IsSubtype(object.BaseExceptionType) {
		msg := ""
		if len(args) > 0 {
			msg = args[0].String()
		}
		return object.NewException(cls, msg), nil
	}

	inst := object.NewInstance(cls)

	// Run __init__ if defined.
	if init, ok := object.LookupInMRO(cls, "__init__"); ok {
		initArgs := append([]object.Object{inst}, args...)
		if _, err := v.callObject(caller, init, initArgs, kwargs); err != nil {
			return nil, err
		}
	}
	return inst, nil
}

// seedCellParams copies parameter values from local slots into their matching
// cell variables. A parameter that is captured by a nested function is both a
// local (bound by argument passing) and a cell (accessed via LOAD_DEREF in the
// closure); this bridges the two at function entry.
func seedCellParams(frame *Frame) {
	for ci, cellName := range frame.Code.Cellvars {
		for vi, varName := range frame.Code.Varnames {
			if varName == cellName {
				if frame.Cells[ci] == nil {
					frame.Cells[ci] = &object.CellObject{}
				}
				frame.Cells[ci].Value = frame.Locals[vi]
			}
		}
	}
}

// debugPanics, when true, lets internal panics propagate with their original
// stack trace instead of being wrapped. Toggled by tests for debugging.
var debugPanics = false

// SetDebugPanics enables or disables panic propagation (test helper).
func SetDebugPanics(v bool) { debugPanics = v }

// makeGenerator wraps a suspended frame as a GeneratorObject. Each call to the
// generator's Next() resumes the frame via runFrame until the next yield or
// the frame returns.
func (v *VM) makeGenerator(name string, frame *Frame) *object.GeneratorObject {
	done := false
	return object.NewGenerator(name, func() (object.Object, bool, error) {
		if done {
			return nil, false, nil
		}
		val, yielded, err := v.runFrame(frame)
		if err != nil {
			done = true
			return nil, false, err
		}
		if yielded {
			return val, true, nil
		}
		// Frame returned (or fell off the end): generator is exhausted.
		done = true
		return nil, false, nil
	})
}

// findSetBelow returns the nearest SetObject on the stack (for SET_ADD).
func (f *Frame) findSetBelow() *object.SetObject {
	for i := f.StackTop - 1; i >= 0; i-- {
		if s, ok := f.Stack[i].(*object.SetObject); ok {
			return s
		}
	}
	return nil
}

// findDictBelow returns the nearest DictObject on the stack (for MAP_ADD).
func (f *Frame) findDictBelow() *object.DictObject {
	for i := f.StackTop - 1; i >= 0; i-- {
		if d, ok := f.Stack[i].(*object.DictObject); ok {
			return d
		}
	}
	return nil
}
