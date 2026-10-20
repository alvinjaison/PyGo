package runtime

import (
	"github.com/pygo/pygo/internal/bytecode"
	"github.com/pygo/pygo/internal/object"
)

// cmpToBytecode maps an object.CompareOp to the bytecode compare op used by
// the VM's comparison dispatch.
func cmpToBytecode(op object.CompareOp) bytecode.CompareOp {
	switch op {
	case object.OpLt:
		return bytecode.CmpLt
	case object.OpLtE:
		return bytecode.CmpLtE
	case object.OpEq:
		return bytecode.CmpEq
	case object.OpNotEq:
		return bytecode.CmpNotEq
	case object.OpGt:
		return bytecode.CmpGt
	case object.OpGtE:
		return bytecode.CmpGtE
	}
	return bytecode.CmpEq
}

// biBuildClass implements the __build_class__ builtin.
//
// Signature (as emitted by the compiler): __build_class__(body_func, name, *bases)
//
// It runs the class-body function to collect the class namespace, then creates
// a new user-defined type from that namespace. This is a pragmatic Phase 3
// implementation covering single/multiple inheritance, methods, and class
// attributes. The descriptor protocol and metaclasses are not yet supported.
func (r *Runtime) biBuildClass(args []object.Object, _ map[string]object.Object) (object.Object, error) {
	if len(args) < 2 {
		return nil, object.NewTypeError("__build_class__ requires at least 2 arguments")
	}
	bodyFn, ok := args[0].(*object.FunctionObject)
	if !ok {
		return nil, object.NewTypeError("__build_class__: first argument must be a function")
	}
	nameObj, ok := args[1].(*object.StrObject)
	if !ok {
		return nil, object.NewTypeError("__build_class__: second argument must be a string")
	}

	var bases []*object.TypeObject
	for _, b := range args[2:] {
		bt, ok := b.(*object.TypeObject)
		if !ok {
			return nil, object.NewTypeError("base is not a type: %s", b.Repr())
		}
		bases = append(bases, bt)
	}
	if len(bases) == 0 {
		bases = []*object.TypeObject{object.ObjectType}
	}

	// Execute the class body in a fresh namespace dict used as its globals,
	// so STORE_NAME in the body populates the class dict.
	ns := object.NewDict()
	bodyFn.Globals = ns
	if _, err := r.Call(nil, bodyFn, nil); err != nil {
		return nil, err
	}

	return object.NewClass(nameObj.Value, bases, ns), nil
}
