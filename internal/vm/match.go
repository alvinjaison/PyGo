package vm

import (
	"github.com/alvinjaison/PyGo/internal/bytecode"
	"github.com/alvinjaison/PyGo/internal/object"
)

// execMatchOp executes the structural-pattern-matching opcodes and UNPACK_EX.
//
// The compiler (see internal/compiler/match.go) keeps the value being matched
// on the stack, so most of these inspect the top of the stack without popping
// it and push a boolean for POP_JUMP_IF_FALSE to consume.
func (v *VM) execMatchOp(f *Frame, instr bytecode.Instruction) error {
	switch instr.Op {

	case bytecode.MATCH_SEQ:
		f.Push(object.BoolFrom(isMatchSequence(f.Peek())))

	case bytecode.MATCH_SEQ_LEN:
		want := int(instr.Arg >> 1)
		star := instr.Arg&1 == 1
		lp, ok := f.Peek().(object.LenProvider)
		if !ok {
			f.Push(object.PyFalse)
			return nil
		}
		n := lp.Len()
		f.Push(object.BoolFrom((star && n >= want) || (!star && n == want)))

	case bytecode.MATCH_MAPPING:
		f.Push(object.BoolFrom(isMatchMapping(f.Peek())))

	case bytecode.MATCH_KEY:
		key := f.Pop()
		val, found, err := mappingGet(f.Peek(), key)
		if err != nil {
			return err
		}
		if !found {
			f.Push(object.PyFalse)
			return nil
		}
		f.Push(val)
		f.Push(object.PyTrue)

	case bytecode.MATCH_REST:
		keys, ok := f.Pop().(*object.TupleObject)
		if !ok {
			return object.NewTypeError("internal error: MATCH_REST expects a tuple of keys")
		}
		rest, err := mappingWithout(f.Peek(), keys.Items)
		if err != nil {
			return err
		}
		f.Push(rest)

	case bytecode.MATCH_CLASS:
		return v.matchClass(f, int(instr.Arg))

	case bytecode.UNPACK_EX:
		return unpackEx(f, instr.Arg)
	}
	return nil
}

// isMatchSequence reports whether a sequence pattern may match obj. Strings,
// bytes and mappings are deliberately excluded, as in CPython.
func isMatchSequence(obj object.Object) bool {
	switch obj.(type) {
	case *object.ListObject, *object.TupleObject:
		return true
	}
	_, ok := obj.(object.MatchSequencer)
	return ok
}

// isMatchMapping reports whether a mapping pattern may match obj.
func isMatchMapping(obj object.Object) bool {
	if !obj.Type().IsSubtype(object.DictType) {
		return false
	}
	_, hasKey := obj.(object.Container)
	_, hasGet := obj.(object.GetItemer)
	return hasKey && hasGet
}

// mappingGet looks key up without triggering __missing__-style defaults
// (defaultdict and Counter would otherwise invent a value).
func mappingGet(m, key object.Object) (object.Object, bool, error) {
	c, ok := m.(object.Container)
	if !ok {
		return nil, false, nil
	}
	has, err := c.Contains(key)
	if err != nil {
		return nil, false, err
	}
	if !has {
		return nil, false, nil
	}
	val, err := m.(object.GetItemer).GetItem(key)
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

// mappingWithout returns a new dict holding the items of m whose keys are not
// in exclude (the **rest capture of a mapping pattern).
func mappingWithout(m object.Object, exclude []object.Object) (object.Object, error) {
	skip := object.NewDict()
	for _, k := range exclude {
		if err := skip.SetItem(k, object.None); err != nil {
			return nil, err
		}
	}
	it, err := object.GetIter(m)
	if err != nil {
		return nil, err
	}
	out := object.NewDict()
	for {
		k, err := it.Next()
		if object.IsStopIteration(err) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		if has, _ := skip.Contains(k); has {
			continue
		}
		val, found, err := mappingGet(m, k)
		if err != nil {
			return nil, err
		}
		if found {
			if err := out.SetItem(k, val); err != nil {
				return nil, err
			}
		}
	}
}

// matchSelfTypes are the built-in types whose single positional class
// sub-pattern matches the subject itself: `case int(n):`, `case str() as s:`.
func isMatchSelfType(t *object.TypeObject) bool {
	for _, base := range []*object.TypeObject{
		object.BoolType, object.IntType, object.FloatType, object.StrType,
		object.BytesType, object.ListType, object.TupleType, object.DictType,
		object.SetType, object.FrozenSetType,
	} {
		if t.IsSubtype(base) {
			return true
		}
	}
	return false
}

func className(cls object.Object) string {
	if t, ok := object.AsClass(cls); ok {
		return t.Name()
	}
	return cls.String()
}

// matchArgs returns the __match_args__ names for a class, and whether the
// class defines them.
func matchArgs(cls object.Object) ([]string, bool, error) {
	if ma, ok := cls.(object.MatchArgser); ok {
		return ma.MatchArgs(), true, nil
	}
	t, ok := object.AsClass(cls)
	if !ok {
		return nil, false, nil
	}
	raw, found := object.LookupInMRO(t, "__match_args__")
	if !found {
		return nil, false, nil
	}
	tup, ok := raw.(*object.TupleObject)
	if !ok {
		return nil, false, object.NewTypeError("%s.__match_args__ must be a tuple (got %s)",
			t.Name(), raw.Type().Name())
	}
	names := make([]string, len(tup.Items))
	for i, item := range tup.Items {
		s, ok := item.(*object.StrObject)
		if !ok {
			return nil, false, object.NewTypeError(
				"__match_args__ elements must be strings (got %s)", item.Type().Name())
		}
		names[i] = s.Value
	}
	return names, true, nil
}

func isAttributeError(err error) bool {
	exc, ok := err.(*object.ExceptionObject)
	return ok && exc.Type().IsSubtype(object.TypeAttributeError)
}

// matchClass implements MATCH_CLASS. Stack on entry (top last):
//
//	subject, cls, kwnames
//
// On a match it pushes a tuple of the attribute values (positional ones first,
// then keyword ones) followed by True. Otherwise it pushes False.
func (v *VM) matchClass(f *Frame, npos int) error {
	kwTuple, ok := f.Pop().(*object.TupleObject)
	if !ok {
		return object.NewTypeError("internal error: MATCH_CLASS expects keyword names")
	}
	cls := f.Pop()
	subject := f.Pop()

	isInst, usable := object.IsInstanceOfClass(subject, cls)
	if !usable {
		return object.NewTypeError("called match pattern must be a class")
	}
	if !isInst {
		f.Push(object.PyFalse)
		return nil
	}

	attrs := make([]object.Object, 0, npos+len(kwTuple.Items))

	if npos > 0 {
		names, defined, err := matchArgs(cls)
		if err != nil {
			return err
		}
		switch {
		case defined:
			if npos > len(names) {
				return object.NewTypeError("%s() accepts %d positional sub-patterns (%d given)",
					className(cls), len(names), npos)
			}
			for _, name := range names[:npos] {
				val, err := getAttribute(subject, name)
				if err != nil {
					if isAttributeError(err) {
						f.Push(object.PyFalse)
						return nil
					}
					return err
				}
				attrs = append(attrs, val)
			}
		default:
			if t, ok := object.AsClass(cls); ok && isMatchSelfType(t) {
				if npos != 1 {
					return object.NewTypeError("%s() accepts 1 positional sub-pattern (%d given)",
						t.Name(), npos)
				}
				attrs = append(attrs, subject)
			} else {
				return object.NewTypeError("%s() accepts 0 positional sub-patterns (%d given)",
					className(cls), npos)
			}
		}
	}

	for _, kw := range kwTuple.Items {
		name := kw.(*object.StrObject).Value
		val, err := getAttribute(subject, name)
		if err != nil {
			if isAttributeError(err) {
				f.Push(object.PyFalse)
				return nil
			}
			return err
		}
		attrs = append(attrs, val)
	}

	f.Push(object.NewTuple(attrs))
	f.Push(object.PyTrue)
	return nil
}

// unpackEx implements UNPACK_EX for `a, *b, c = seq`. The argument holds the
// number of targets before the starred one (low byte) and after it (next
// byte). It pops the iterable and pushes the targets in reverse order, so the
// first target is on top of the stack, with the starred target as a list.
func unpackEx(f *Frame, arg uint32) error {
	before := int(arg & bytecode.UnpackBeforeMask)
	after := int(arg >> bytecode.UnpackAfterShift)

	it, err := object.GetIter(f.Pop())
	if err != nil {
		return err
	}
	var items []object.Object
	for {
		item, err := it.Next()
		if object.IsStopIteration(err) {
			break
		}
		if err != nil {
			return err
		}
		items = append(items, item)
	}
	if len(items) < before+after {
		return object.NewValueError("not enough values to unpack (expected at least %d, got %d)",
			before+after, len(items))
	}

	middle := object.NewList(items[before : len(items)-after])
	seq := make([]object.Object, 0, before+1+after)
	seq = append(seq, items[:before]...)
	seq = append(seq, middle)
	seq = append(seq, items[len(items)-after:]...)
	for i := len(seq) - 1; i >= 0; i-- {
		f.Push(seq[i])
	}
	return nil
}
