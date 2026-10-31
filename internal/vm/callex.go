package vm

import (
	"github.com/alvinjaison/PyGo/internal/bytecode"
	"github.com/alvinjaison/PyGo/internal/object"
)

// execUnpackOp executes the opcodes behind f(*args, **kwargs) and displays
// such as [*a, 1] and {**d}.
func (v *VM) execUnpackOp(f *Frame, instr bytecode.Instruction) (object.Object, error) {
	switch instr.Op {

	case bytecode.LIST_EXTEND:
		src := f.Pop()
		lst := f.Peek().(*object.ListObject)
		it, err := object.GetIter(src)
		if err != nil {
			return nil, object.NewTypeError("Value after * must be an iterable, not %s", src.Type().Name())
		}
		for {
			item, err := it.Next()
			if object.IsStopIteration(err) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			lst.Items = append(lst.Items, item)
		}

	case bytecode.DICT_MERGE:
		src := f.Pop()
		dst := f.Peek().(*object.DictObject)
		return nil, mergeMapping(dst, src, instr.Arg == 1)

	case bytecode.LIST_TO_TUPLE:
		lst := f.Pop().(*object.ListObject)
		f.Push(object.NewTuple(lst.Items))

	case bytecode.LIST_TO_SET:
		lst := f.Pop().(*object.ListObject)
		set, err := object.NewSetFrom(lst.Items)
		if err != nil {
			return nil, err
		}
		f.Push(set)

	case bytecode.CALL_FUNCTION_EX:
		var kwargs map[string]object.Object
		if instr.Arg&1 != 0 {
			d := f.Pop().(*object.DictObject)
			kwargs = make(map[string]object.Object, d.Len())
			for _, k := range d.Keys() {
				val, _ := d.GetItem(k)
				kwargs[k.(*object.StrObject).Value] = val
			}
		}
		args := append([]object.Object(nil), f.Pop().(*object.ListObject).Items...)
		callee := f.Pop()
		result, err := v.callObject(f, callee, args, kwargs)
		if err != nil {
			return nil, err
		}
		f.Push(result)
	}
	return nil, nil
}

// mergeMapping copies the items of src into dst. For call keywords
// (strict=true) keys must be strings and may not repeat.
func mergeMapping(dst *object.DictObject, src object.Object, strict bool) error {
	getter, ok := src.(object.GetItemer)
	if !ok || !src.Type().IsSubtype(object.DictType) {
		if strict {
			return object.NewTypeError("argument after ** must be a mapping, not %s", src.Type().Name())
		}
		return object.NewTypeError("'%s' object is not a mapping", src.Type().Name())
	}
	it, err := object.GetIter(src)
	if err != nil {
		return err
	}
	for {
		k, err := it.Next()
		if object.IsStopIteration(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if strict {
			name, isStr := k.(*object.StrObject)
			if !isStr {
				return object.NewTypeError("keywords must be strings")
			}
			if _, dup := dst.LookupStr(name.Value); dup {
				return object.NewTypeError("got multiple values for keyword argument '%s'", name.Value)
			}
		}
		val, err := getter.GetItem(k)
		if err != nil {
			return err
		}
		if err := dst.SetItem(k, val); err != nil {
			return err
		}
	}
}
