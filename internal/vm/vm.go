package vm

import (
	"context"
	"fmt"
	"strings"

	"github.com/alvinjaison/pygo/internal/bytecode"
	"github.com/alvinjaison/pygo/internal/object"
)

// VM is the PyGo bytecode virtual machine.
//
// It executes Code objects by dispatching over the instruction stream.
// Each call to Run executes a single Frame to completion (or until a yield
// or exception unwinds past the frame boundary).
//
// The VM is not safe for concurrent use. Create one VM per goroutine, or
// protect access with an external lock.
type VM struct {
	// ctx is checked at loop backedges and call sites for cancellation.
	ctx context.Context

	// maxDepth is the maximum call stack depth (default 500).
	maxDepth int

	// depth is the current call stack depth.
	depth int

	// instrCount counts executed instructions (for limits, Phase 2+).
	instrCount int64

	// maxInstr is the instruction execution limit (0 = unlimited).
	maxInstr int64
}

// New creates a new VM with the given options.
func New(ctx context.Context, maxDepth int) *VM {
	if ctx == nil {
		ctx = context.Background()
	}
	if maxDepth <= 0 {
		maxDepth = 500
	}
	return &VM{ctx: ctx, maxDepth: maxDepth}
}

// SetMaxInstructions sets the maximum instruction count for execution.
// If n <= 0, instruction counting is disabled (unlimited).
func (v *VM) SetMaxInstructions(n int64) {
	v.maxInstr = n
}

// Run executes frame until it returns, yields, or an unhandled exception occurs.
//
// Returns (result, nil) on normal return.
// Returns (nil, *object.ExceptionObject) on unhandled Python exception.
// Returns (nil, context.Canceled) or (nil, context.DeadlineExceeded) on cancellation.
func (v *VM) Run(frame *Frame) (result object.Object, err error) {
	// Recover from unexpected panics so the host application is not crashed.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("internal VM error: %v", r)
		}
	}()

	v.depth++
	defer func() { v.depth-- }()

	if v.depth > v.maxDepth {
		exc := object.NewException(object.TypeRecursionError,
			fmt.Sprintf("maximum recursion depth exceeded (%d)", v.maxDepth))
		return nil, exc
	}

	for {
		// Cancellation check (at every instruction for simplicity in Phase 1;
		// optimize to backedges-only in Phase 2).
		select {
		case <-v.ctx.Done():
			return nil, v.ctx.Err()
		default:
		}

		if frame.PC >= len(frame.Code.Instructions) {
			// Fell off the end of the instruction array.
			return object.None, nil
		}

		instr := frame.Code.Instructions[frame.PC]
		frame.PC++
		v.instrCount++

		if v.maxInstr > 0 && v.instrCount > v.maxInstr {
			exc := object.NewRuntimeError("instruction limit exceeded")
			return nil, exc
		}

		var stepErr error
		result, stepErr = v.execute(frame, instr)

		if stepErr != nil {
			// stepErr is a Python exception wrapped as a Go error.
			exc, isExc := stepErr.(*object.ExceptionObject)
			if !isExc {
				// Try to detect exception type from error message prefix.
				exc = wrapGoError(stepErr)
			}
			// Attempt to find an exception handler in this frame.
			if handler, found := frame.UnwindToHandler(); found {
				// Push exception info onto the stack and jump to handler.
				frame.Push(exc)
				frame.CurrentException = exc
				frame.PC = handler.Handler
				continue
			}
			// No handler in this frame — propagate to caller.
			if exc.Traceback == nil {
				exc.Traceback = frame.Traceback()
			}
			return nil, exc
		}

		// A non-nil result signals RETURN_VALUE was executed.
		if result != nil {
			return result, nil
		}
	}
}

// execute dispatches a single instruction and returns either:
//   - (nil, nil)  — instruction executed, continue the loop
//   - (value, nil) — RETURN_VALUE: frame is done, return the value
//   - (nil, error) — an exception was raised
func (v *VM) execute(f *Frame, instr bytecode.Instruction) (object.Object, error) {
	switch instr.Op {

	// ---- Stack manipulation ----

	case bytecode.POP_TOP:
		f.Pop()

	case bytecode.ROT_TWO:
		f.RotTwo()

	case bytecode.ROT_THREE:
		f.RotThree()

	case bytecode.DUP_TOP:
		f.DupTop()

	case bytecode.NOP:
		// do nothing

	// ---- Load / store ----

	case bytecode.LOAD_CONST:
		f.Push(constantToObject(f.Code.Constants[instr.Arg]))

	case bytecode.LOAD_NAME:
		name := f.Code.Names[instr.Arg]
		val, err := lookupName(f, name)
		if err != nil {
			return nil, err
		}
		f.Push(val)

	case bytecode.STORE_NAME:
		name := f.Code.Names[instr.Arg]
		val := f.Pop()
		return nil, f.Globals.SetItem(object.StrFrom(name), val)

	case bytecode.DELETE_NAME:
		name := f.Code.Names[instr.Arg]
		err := f.Globals.DelItem(object.StrFrom(name))
		if err != nil {
			return nil, object.NewNameError(name)
		}

	case bytecode.LOAD_FAST:
		val := f.Locals[instr.Arg]
		if val == nil {
			name := f.Code.Varnames[instr.Arg]
			return nil, object.NewException(object.TypeUnboundLocalError,
				fmt.Sprintf("local variable '%s' referenced before assignment", name))
		}
		f.Push(val)

	case bytecode.STORE_FAST:
		f.Locals[instr.Arg] = f.Pop()

	case bytecode.DELETE_FAST:
		f.Locals[instr.Arg] = nil

	case bytecode.LOAD_GLOBAL:
		name := f.Code.Names[instr.Arg]
		val, err := f.Globals.GetItem(object.StrFrom(name))
		if err != nil {
			val, err = f.Builtins.GetItem(object.StrFrom(name))
			if err != nil {
				return nil, object.NewNameError(name)
			}
		}
		f.Push(val)

	case bytecode.STORE_GLOBAL:
		name := f.Code.Names[instr.Arg]
		return nil, f.Globals.SetItem(object.StrFrom(name), f.Pop())

	case bytecode.DELETE_GLOBAL:
		name := f.Code.Names[instr.Arg]
		err := f.Globals.DelItem(object.StrFrom(name))
		if err != nil {
			return nil, object.NewNameError(name)
		}

	case bytecode.LOAD_CLOSURE:
		// LOAD_CLOSURE pushes the cell object itself (not its value) onto the stack.
		// The arg indexes into cellvars first, then freevars.
		idx := int(instr.Arg)
		var cell *object.CellObject
		if idx < len(f.Cells) {
			cell = f.Cells[idx]
		} else {
			cell = f.FreeVars[idx-len(f.Cells)]
		}
		f.Push(cell)

	case bytecode.LOAD_DEREF:
		idx := int(instr.Arg)
		var cell *object.CellObject
		if idx < len(f.Cells) {
			cell = f.Cells[idx]
		} else {
			cell = f.FreeVars[idx-len(f.Cells)]
		}
		if cell.IsEmpty() {
			name := cellName(f, idx)
			return nil, object.NewException(object.TypeNameError,
				fmt.Sprintf("free variable '%s' referenced before assignment in enclosing scope", name))
		}
		f.Push(cell.Value)

	case bytecode.STORE_DEREF:
		idx := int(instr.Arg)
		val := f.Pop()
		if idx < len(f.Cells) {
			f.Cells[idx].Value = val
		} else {
			f.FreeVars[idx-len(f.Cells)].Value = val
		}

	case bytecode.DELETE_DEREF:
		idx := int(instr.Arg)
		if idx < len(f.Cells) {
			f.Cells[idx].Value = nil
		} else {
			f.FreeVars[idx-len(f.Cells)].Value = nil
		}

	// ---- Attribute access ----

	case bytecode.LOAD_ATTR:
		name := f.Code.Names[instr.Arg]
		obj := f.Pop()
		val, err := getAttribute(obj, name)
		if err != nil {
			return nil, err
		}
		f.Push(val)

	case bytecode.STORE_ATTR:
		name := f.Code.Names[instr.Arg]
		obj := f.Pop()
		val := f.Pop()
		return nil, setAttribute(obj, name, val)

	// ---- Subscript ----

	case bytecode.BINARY_SUBSCR:
		key := f.Pop()
		container := f.Pop()
		// Slice dispatch: if key is a SliceObject, use slice-specific handlers.
		if sl, ok := key.(*object.SliceObject); ok {
			val, err := handleSliceGet(container, sl)
			if err != nil {
				return nil, err
			}
			f.Push(val)
		} else if s, ok := container.(object.Subscriptable); ok {
			val, err := s.GetItem(key)
			if err != nil {
				return nil, err
			}
			f.Push(val)
		} else {
			return nil, object.NewTypeError("'%s' object is not subscriptable", container.Type().Name())
		}

	case bytecode.STORE_SUBSCR:
		key := f.Pop()
		container := f.Pop()
		val := f.Pop()
		// Slice dispatch: if key is a SliceObject, use slice-specific handlers.
		if sl, ok := key.(*object.SliceObject); ok {
			return nil, handleSliceSet(container, sl, val)
		}
		if s, ok := container.(object.Subscriptable); ok {
			return nil, s.SetItem(key, val)
		}
		return nil, object.NewTypeError("'%s' object does not support item assignment", container.Type().Name())

	case bytecode.DELETE_SUBSCR:
		key := f.Pop()
		container := f.Pop()
		// Slice dispatch: if key is a SliceObject, use slice-specific handlers.
		if sl, ok := key.(*object.SliceObject); ok {
			return nil, handleSliceDel(container, sl)
		}
		if s, ok := container.(object.Subscriptable); ok {
			return nil, s.DelItem(key)
		}
		return nil, object.NewTypeError("'%s' object does not support item deletion", container.Type().Name())

	// ---- Binary operators ----

	case bytecode.BINARY_ADD:
		right, left := f.Pop(), f.Pop()
		result, err := object.BinaryOp(left, right, "+")
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.BINARY_SUBTRACT:
		right, left := f.Pop(), f.Pop()
		result, err := object.BinaryOp(left, right, "-")
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.BINARY_MULTIPLY:
		right, left := f.Pop(), f.Pop()
		result, err := object.BinaryOp(left, right, "*")
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.BINARY_TRUE_DIVIDE:
		right, left := f.Pop(), f.Pop()
		result, err := object.BinaryOp(left, right, "/")
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.BINARY_FLOOR_DIVIDE:
		right, left := f.Pop(), f.Pop()
		result, err := object.BinaryOp(left, right, "//")
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.BINARY_MODULO:
		right, left := f.Pop(), f.Pop()
		result, err := object.BinaryOp(left, right, "%")
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.BINARY_POWER:
		right, left := f.Pop(), f.Pop()
		result, err := object.BinaryOp(left, right, "**")
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.BINARY_AND:
		right, left := f.Pop(), f.Pop()
		result, err := object.BinaryOp(left, right, "&")
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.BINARY_OR:
		right, left := f.Pop(), f.Pop()
		result, err := object.BinaryOp(left, right, "|")
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.BINARY_XOR:
		right, left := f.Pop(), f.Pop()
		result, err := object.BinaryOp(left, right, "^")
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.BINARY_LSHIFT:
		right, left := f.Pop(), f.Pop()
		result, err := object.BinaryOp(left, right, "<<")
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.BINARY_RSHIFT:
		right, left := f.Pop(), f.Pop()
		result, err := object.BinaryOp(left, right, ">>")
		if err != nil {
			return nil, err
		}
		f.Push(result)

	// ---- Unary operators ----

	case bytecode.UNARY_NEGATIVE:
		val := f.Pop()
		result, err := object.UnaryOp(val, "-")
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.UNARY_NOT:
		val := f.Pop()
		truth, err := object.IsTruthy(val)
		if err != nil {
			return nil, err
		}
		f.Push(object.BoolFrom(!truth))

	case bytecode.UNARY_INVERT:
		val := f.Pop()
		result, err := object.UnaryOp(val, "~")
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.UNARY_POSITIVE:
		val := f.Pop()
		result, err := object.UnaryOp(val, "+")
		if err != nil {
			return nil, err
		}
		f.Push(result)

	// ---- Compare ----

	case bytecode.COMPARE_OP:
		right, left := f.Pop(), f.Pop()
		result, err := object.CompareObjects(left, right, bytecode.CompareOp(instr.Arg))
		if err != nil {
			return nil, err
		}
		f.Push(result)

	// ---- Jumps ----

	case bytecode.JUMP_ABSOLUTE:
		f.PC = int(instr.Arg)

	case bytecode.JUMP_FORWARD:
		f.PC += int(instr.Arg)

	case bytecode.POP_JUMP_IF_FALSE:
		val := f.Pop()
		truth, err := object.IsTruthy(val)
		if err != nil {
			return nil, err
		}
		if !truth {
			f.PC = int(instr.Arg)
		}

	case bytecode.POP_JUMP_IF_TRUE:
		val := f.Pop()
		truth, err := object.IsTruthy(val)
		if err != nil {
			return nil, err
		}
		if truth {
			f.PC = int(instr.Arg)
		}

	case bytecode.JUMP_IF_FALSE_OR_POP:
		val := f.Peek()
		truth, err := object.IsTruthy(val)
		if err != nil {
			return nil, err
		}
		if !truth {
			f.PC = int(instr.Arg)
		} else {
			f.Pop()
		}

	case bytecode.JUMP_IF_TRUE_OR_POP:
		val := f.Peek()
		truth, err := object.IsTruthy(val)
		if err != nil {
			return nil, err
		}
		if truth {
			f.PC = int(instr.Arg)
		} else {
			f.Pop()
		}

	// ---- Iteration ----

	case bytecode.GET_ITER:
		val := f.Pop()
		it, err := object.GetIter(val)
		if err != nil {
			return nil, err
		}
		f.Push(it)

	case bytecode.FOR_ITER:
		it, ok := f.Peek().(object.Iterator)
		if !ok {
			return nil, object.NewTypeError("object is not an iterator")
		}
		val, err := it.Next()
		if object.IsStopIteration(err) {
			f.Pop() // pop the iterator
			f.PC += int(instr.Arg)
		} else if err != nil {
			return nil, err
		} else {
			f.Push(val)
		}

	// ---- Building collections ----

	case bytecode.BUILD_LIST:
		count := int(instr.Arg)
		items := make([]object.Object, count)
		for i := count - 1; i >= 0; i-- {
			items[i] = f.Pop()
		}
		f.Push(object.NewList(items))

	case bytecode.BUILD_TUPLE:
		count := int(instr.Arg)
		items := make([]object.Object, count)
		for i := count - 1; i >= 0; i-- {
			items[i] = f.Pop()
		}
		f.Push(object.NewTuple(items))

	case bytecode.BUILD_MAP:
		count := int(instr.Arg)
		d := object.NewDict()
		pairs := make([]object.Object, count*2)
		for i := count*2 - 1; i >= 0; i-- {
			pairs[i] = f.Pop()
		}
		for i := 0; i < count; i++ {
			if err := d.SetItem(pairs[i*2], pairs[i*2+1]); err != nil {
				return nil, err
			}
		}
		f.Push(d)

	case bytecode.BUILD_SET:
		// TODO(Phase 2): Implement SetObject and BUILD_SET.
		return nil, fmt.Errorf("set literals not yet implemented")

	// ---- Comprehension accumulators ----

	case bytecode.LIST_APPEND:
		// Append TOS to the list `arg` slots below (after the value is popped).
		value := f.Pop()
		target := f.PeekN(int(instr.Arg) - 1)
		lst, ok := target.(*object.ListObject)
		if !ok {
			return nil, object.NewTypeError(
				"LIST_APPEND expected list, got '%s'", target.Type().Name())
		}
		lst.Append(value)

	case bytecode.MAP_ADD:
		// stack[-arg][key] = value, where value is TOS and key is TOS1.
		// This matches CPython 3.8+ ordering (key pushed before value).
		value := f.Pop()
		key := f.Pop()
		target := f.PeekN(int(instr.Arg) - 1)
		d, ok := target.(*object.DictObject)
		if !ok {
			return nil, object.NewTypeError(
				"MAP_ADD expected dict, got '%s'", target.Type().Name())
		}
		if err := d.SetItem(key, value); err != nil {
			return nil, err
		}

	case bytecode.SET_ADD:
		// Sets are not yet implemented; set comprehensions are rejected at
		// compile time, so this should be unreachable.
		return nil, fmt.Errorf("set comprehensions not yet implemented")

	case bytecode.BUILD_SLICE:
		argc := int(instr.Arg)
		var step object.Object = object.None
		if argc == 3 {
			step = f.Pop()
		}
		stop := f.Pop()
		start := f.Pop()
		f.Push(object.NewSlice(start, stop, step))

	// ---- Unpacking ----

	case bytecode.UNPACK_SEQUENCE:
		count := int(instr.Arg)
		val := f.Pop()
		it, err := object.GetIter(val)
		if err != nil {
			return nil, err
		}
		items := make([]object.Object, 0, count)
		for {
			item, err := it.Next()
			if object.IsStopIteration(err) {
				break
			}
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		if len(items) != count {
			return nil, object.NewValueError(
				"not enough values to unpack (expected %d, got %d)", count, len(items))
		}
		for i := len(items) - 1; i >= 0; i-- {
			f.Push(items[i])
		}

	case bytecode.UNPACK_EX:
		// Starred unpacking: a, *b, c = iterable
		// Arg encodes: before | (after << 8)
		before := int(instr.Arg & 0xFF)
		after := int(instr.Arg >> 8)
		val := f.Pop()
		it, err := object.GetIter(val)
		if err != nil {
			return nil, err
		}
		// Collect all items from the iterator.
		var items []object.Object
		for {
			item, err := it.Next()
			if object.IsStopIteration(err) {
				break
			}
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		minRequired := before + after
		if len(items) < minRequired {
			return nil, object.NewValueError(
				"not enough values to unpack (expected at least %d, got %d)", minRequired, len(items))
		}
		// Push in reverse order: after targets, starred list, before targets
		// (so that the first STORE_* pops the first "before" target)
		for i := len(items) - 1; i >= len(items)-after; i-- {
			f.Push(items[i])
		}
		// The starred portion becomes a list.
		starredItems := items[before : len(items)-after]
		f.Push(object.NewList(starredItems))
		for i := before - 1; i >= 0; i-- {
			f.Push(items[i])
		}

	// ---- Functions ----

	case bytecode.MAKE_FUNCTION:
		// Arg is a flags bitmask (MakeFunctionFlag).
		flags := object.MakeFunctionFlags(instr.Arg)
		qualName := f.Pop().(*object.StrObject).Value
		codeObj := f.Pop().(*codeConstant).code
		fn := &object.FunctionObject{
			Code:     codeObj,
			Globals:  f.Globals,
			Name:     codeObj.Name,
			QualName: qualName,
		}
		if flags&object.MakeFuncHasClosure != 0 {
			closureTuple := f.Pop().(*object.TupleObject)
			cells := make([]*object.CellObject, len(closureTuple.Items))
			for i, item := range closureTuple.Items {
				cells[i] = item.(*object.CellObject)
			}
			fn.Closure = cells
		}
		if flags&object.MakeFuncHasKwDefaults != 0 {
			kwDefaultsDict := f.Pop().(*object.DictObject)
			fn.KwDefaults = make(map[string]object.Object)
			for _, key := range kwDefaultsDict.Keys() {
				val, _ := kwDefaultsDict.GetItem(key)
				fn.KwDefaults[key.(*object.StrObject).Value] = val
			}
		}
		if flags&object.MakeFuncHasDefaults != 0 {
			defaultsTuple := f.Pop().(*object.TupleObject)
			fn.Defaults = defaultsTuple.Items
		}
		f.Push(fn)

	case bytecode.CALL_FUNCTION:
		argc := int(instr.Arg)
		args := make([]object.Object, argc)
		for i := argc - 1; i >= 0; i-- {
			args[i] = f.Pop()
		}
		callee := f.Pop()
		result, err := v.callObject(f, callee, args, nil)
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.CALL_FUNCTION_KW:
		// TOS is a tuple of keyword argument names.
		kwNamesTuple := f.Pop().(*object.TupleObject)
		argc := int(instr.Arg)
		nKw := len(kwNamesTuple.Items)
		nPos := argc - nKw

		// Pop keyword values in reverse (stack order), then record them in
		// call-site order so **kwargs collection is deterministic.
		kwValues := make([]object.Object, nKw)
		for i := nKw - 1; i >= 0; i-- {
			kwValues[i] = f.Pop()
		}
		kw := newKwargSet(nKw)
		for i := 0; i < nKw; i++ {
			kw.Set(kwNamesTuple.Items[i].(*object.StrObject).Value, kwValues[i])
		}
		// Pop positional arguments (in reverse order).
		args := make([]object.Object, nPos)
		for i := nPos - 1; i >= 0; i-- {
			args[i] = f.Pop()
		}
		callee := f.Pop()
		result, err := v.callObject(f, callee, args, kw)
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.BUILD_TUPLE_UNPACK_WITH_CALL:
		// Merge N positional segments (plain tuples and *expr iterables) into
		// one argument tuple for CALL_FUNCTION_EX.
		count := int(instr.Arg)
		segments := make([]object.Object, count)
		for i := count - 1; i >= 0; i-- {
			segments[i] = f.Pop()
		}
		var items []object.Object
		for _, seg := range segments {
			segItems, err := unpackStarArg(seg)
			if err != nil {
				return nil, err
			}
			items = append(items, segItems...)
		}
		f.Push(object.NewTuple(items))

	case bytecode.BUILD_MAP_UNPACK:
		// Merge N keyword mappings (literal dicts and **expr) into one dict.
		count := int(instr.Arg)
		segments := make([]object.Object, count)
		for i := count - 1; i >= 0; i-- {
			segments[i] = f.Pop()
		}
		merged := object.NewDict()
		for _, seg := range segments {
			d, ok := seg.(*object.DictObject)
			if !ok {
				return nil, object.NewTypeError(
					"argument after ** must be a mapping, not %s", seg.Type().Name())
			}
			for _, key := range d.Keys() {
				val, err := d.GetItem(key)
				if err != nil {
					return nil, err
				}
				if _, found := merged.GetItem(key); found == nil {
					return nil, object.NewTypeError(
						"got multiple values for keyword argument '%s'", key.String())
				}
				if err := merged.SetItem(key, val); err != nil {
					return nil, err
				}
			}
		}
		f.Push(merged)

	case bytecode.CALL_FUNCTION_EX:
		// Stack layout: callee, args_iterable, [kwargs_dict].
		// Arg bit 0 signals that a keyword dict is on TOS.
		var kw *kwargSet
		if instr.Arg&1 != 0 {
			kwObj := f.Pop()
			d, ok := kwObj.(*object.DictObject)
			if !ok {
				return nil, object.NewTypeError(
					"argument after ** must be a mapping, not %s", kwObj.Type().Name())
			}
			// DictObject preserves insertion order, so Keys() gives call order.
			kw = newKwargSet(d.Len())
			for _, key := range d.Keys() {
				name, ok := key.(*object.StrObject)
				if !ok {
					return nil, object.NewTypeError("keywords must be strings")
				}
				val, err := d.GetItem(key)
				if err != nil {
					return nil, err
				}
				kw.Set(name.Value, val)
			}
		}
		args, err := unpackStarArg(f.Pop())
		if err != nil {
			return nil, err
		}
		callee := f.Pop()
		result, err := v.callObject(f, callee, args, kw)
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.RETURN_VALUE:
		return f.Pop(), nil

	// ---- Exceptions ----

	case bytecode.RAISE_VARARGS:
		argc := int(instr.Arg)
		var exc *object.ExceptionObject
		switch argc {
		case 0:
			// bare raise: re-raise current exception
			if f.CurrentException != nil {
				return nil, f.CurrentException
			}
			return nil, object.NewRuntimeError("no active exception to re-raise")
		case 1:
			raised := f.Pop()
			exc = toException(raised)
		case 2:
			cause := f.Pop()
			raised := f.Pop()
			exc = toException(raised)
			exc.Cause = toException(cause)
		}
		return nil, exc

	case bytecode.SETUP_FINALLY:
		f.PushBlock(BlockExcept, int(instr.Arg)+f.PC)

	case bytecode.POP_BLOCK:
		if f.HasBlock() {
			f.PopBlock()
		}

	case bytecode.POP_EXCEPT:
		// Pop the except block from the block stack.
		// Note: CurrentException is NOT cleared here — it remains accessible
		// for bare 'raise' statements within the handler body.
		if f.HasBlock() {
			f.PopBlock()
		}

	case bytecode.RERAISE:
		// RERAISE: pop TOS; if it's an exception, re-raise it.
		// Used at the end of finally blocks to propagate unhandled exceptions.
		// If TOS is None (normal flow pushed None), do nothing.
		tos := f.Pop()
		if exc, ok := tos.(*object.ExceptionObject); ok {
			return nil, exc
		}
		// If TOS is None or not an exception, continue normally.

	// ---- Import ----

	case bytecode.IMPORT_NAME:
		// IMPORT_NAME loads a module by name. The module name is in Names[arg].
		// The importer is looked up from builtins["__import__"].
		name := f.Code.Names[instr.Arg]
		importFn, err := f.Builtins.GetItem(object.StrFrom("__import__"))
		if err != nil {
			return nil, object.NewException(object.TypeModuleNotFoundError,
				fmt.Sprintf("No module named '%s' (import system not available)", name))
		}
		callable, ok := importFn.(object.Callable)
		if !ok {
			return nil, object.NewException(object.TypeModuleNotFoundError,
				fmt.Sprintf("No module named '%s' (__import__ is not callable)", name))
		}
		mod, callErr := callable.Call([]object.Object{object.StrFrom(name)}, nil)
		if callErr != nil {
			return nil, callErr
		}
		f.Push(mod)

	case bytecode.IMPORT_FROM:
		// IMPORT_FROM loads an attribute from the module on TOS (without popping it).
		name := f.Code.Names[instr.Arg]
		mod := f.Peek()
		ah, ok := mod.(object.AttributeHolder)
		if !ok {
			return nil, object.NewException(object.TypeImportError,
				fmt.Sprintf("cannot import name '%s' (module has no attributes)", name))
		}
		val, err := ah.GetAttr(name)
		if err != nil {
			// Get the module name for a better error message.
			modName := "unknown"
			if m, ok := mod.(*object.ModuleObject); ok {
				modName = m.Name
			}
			return nil, object.NewException(object.TypeImportError,
				fmt.Sprintf("cannot import name '%s' from '%s'", name, modName))
		}
		f.Push(val)

	// ---- Generator ----

	case bytecode.YIELD_VALUE:
		// Mark the frame as yielded and return the value.
		// The VM caller (GeneratorObject.next()) handles resumption.
		val := f.Pop()
		f.Yielded = true
		return val, nil

	// ---- Misc ----

	case bytecode.LOAD_BUILD_CLASS:
		// Push the __build_class__ built-in.
		bc, err := f.Builtins.GetItem(object.StrFrom("__build_class__"))
		if err != nil {
			return nil, object.NewNameError("__build_class__")
		}
		f.Push(bc)

	case bytecode.EXTENDED_ARG:
		// EXTENDED_ARG extends the argument of the next instruction.
		// The current instruction's arg provides the high bits.
		// We shift it left 8 bits and OR with the next instruction's arg.
		if f.PC < len(f.Code.Instructions) {
			next := &f.Code.Instructions[f.PC]
			next.Arg = (instr.Arg << 8) | next.Arg
		}

	case bytecode.PRINT_EXPR:
		// REPL: print expression result.
		val := f.Pop()
		if !object.IsNone(val) {
			fmt.Println(val.Repr())
		}

	default:
		return nil, fmt.Errorf("NotImplementedError: opcode %s not implemented", instr.Op)
	}

	return nil, nil
}

// ============================================================
// VM helpers
// ============================================================

// callObject invokes a callable from within the VM.
// For Python functions, a new frame is created and Run is called recursively.
// For Go built-ins, the Go function is called directly.
func (v *VM) callObject(caller *Frame, callee object.Object, args []object.Object, kw *kwargSet) (object.Object, error) {
	switch fn := callee.(type) {
	case *object.FunctionObject:
		newFrame, err := buildFrame(fn, args, kw, caller)
		if err != nil {
			return nil, err
		}
		return v.Run(newFrame)

	case *object.BuiltinFuncObject:
		return fn.Call(args, kw.Map())

	case *object.BoundMethodObject:
		fullArgs := make([]object.Object, len(args)+1)
		fullArgs[0] = fn.Self
		copy(fullArgs[1:], args)
		return v.callObject(caller, fn.Function, fullArgs, kw)

	case *object.TypeObject:
		// Calling a type creates an instance.
		// For exception types, create an ExceptionObject with the given message.
		if fn.IsSubtype(object.BaseExceptionType) {
			var msg string
			if len(args) > 0 {
				msg = args[0].String()
			}
			return object.NewException(fn, msg), nil
		}
		// TODO(Phase 3): Implement full __new__ / __init__ protocol for non-exception types.
		return nil, fmt.Errorf("TypeError: cannot call type '%s' (class instantiation not yet implemented)", fn.Name())

	default:
		if c, ok := callee.(object.Callable); ok {
			return c.Call(args, kw.Map())
		}
		return nil, object.NewTypeError("'%s' object is not callable", callee.Type().Name())
	}
}

// unpackStarArg expands a *expr call argument into a slice of positional
// arguments. Any iterable is accepted; a non-iterable is a TypeError.
func unpackStarArg(obj object.Object) ([]object.Object, error) {
	if t, ok := obj.(*object.TupleObject); ok {
		return t.Items, nil
	}
	it, err := object.GetIter(obj)
	if err != nil {
		return nil, object.NewTypeError(
			"argument after * must be an iterable, not %s", obj.Type().Name())
	}
	var items []object.Object
	for {
		item, err := it.Next()
		if object.IsStopIteration(err) {
			return items, nil
		}
		if err != nil {
			// A genuine error from the iterable itself — propagate as-is.
			return nil, err
		}
		items = append(items, item)
	}
}

// buildFrame creates a new Frame for a Python function call, binding arguments to locals.
func buildFrame(fn *object.FunctionObject, args []object.Object, kw *kwargSet, caller *Frame) (*Frame, error) {
	code := fn.Code
	frame := NewFrame(code, fn.Globals, caller.Builtins, caller)

	// Bind free variables from closure.
	copy(frame.FreeVars, fn.Closure)

	nargs := code.ArgCount
	nKwOnly := code.KwOnlyArgCount
	hasVarargs := code.HasFlag(bytecode.FlagVarargs)
	hasVarkw := code.HasFlag(bytecode.FlagVarkeywords)

	// Total named parameters = positional + keyword-only.
	totalNamed := nargs + nKwOnly

	// --- 1. Bind positional arguments to parameter slots ---
	nPosSupplied := len(args)
	if nPosSupplied > nargs && !hasVarargs {
		return nil, object.NewTypeError(
			"%s() takes %d positional argument(s) but %d were given",
			fn.Name, nargs, nPosSupplied)
	}
	for i := 0; i < nPosSupplied && i < nargs; i++ {
		frame.Locals[i] = args[i]
	}

	// --- 2. Handle *args collection ---
	if hasVarargs {
		varargItems := []object.Object{}
		if nPosSupplied > nargs {
			// Copy: args may alias a caller-visible tuple (CALL_FUNCTION_EX),
			// and the *args tuple must be an independent immutable snapshot.
			varargItems = make([]object.Object, nPosSupplied-nargs)
			copy(varargItems, args[nargs:])
		}
		frame.Locals[nargs] = object.NewTuple(varargItems)
	}

	// --- 3. Match keyword arguments to parameter names ---
	// Keyword-only parameters start after *args slot (if any).
	kwOnlyBase := nargs
	if hasVarargs {
		kwOnlyBase++ // skip the *args slot
	}

	// Keywords that match no parameter, in call-site order. These go to
	// **kwargs in step 6, or are an error if the function has no **kwargs.
	var extraKwNames []string

	for _, kwName := range kw.Order() {
		kwVal := kw.Get(kwName)
		matched := false
		// Check positional parameter names.
		for i := 0; i < nargs; i++ {
			if code.Varnames[i] == kwName {
				// Check for duplicate: positional arg already supplied for this param.
				if frame.Locals[i] != nil {
					return nil, object.NewTypeError(
						"%s() got multiple values for argument '%s'",
						fn.Name, kwName)
				}
				frame.Locals[i] = kwVal
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		// Check keyword-only parameter names.
		for i := 0; i < nKwOnly; i++ {
			localIdx := kwOnlyBase + i
			if code.Varnames[localIdx] == kwName {
				if frame.Locals[localIdx] != nil {
					return nil, object.NewTypeError(
						"%s() got multiple values for argument '%s'",
						fn.Name, kwName)
				}
				frame.Locals[localIdx] = kwVal
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		// Keyword arg doesn't match any parameter name.
		if !hasVarkw {
			return nil, object.NewTypeError(
				"%s() got an unexpected keyword argument '%s'",
				fn.Name, kwName)
		}
		extraKwNames = append(extraKwNames, kwName)
	}

	// --- 4. Fill in default values for unbound positional parameters ---
	for i := 0; i < nargs; i++ {
		if frame.Locals[i] == nil {
			defaultIdx := i - (nargs - len(fn.Defaults))
			if defaultIdx >= 0 && defaultIdx < len(fn.Defaults) {
				frame.Locals[i] = fn.Defaults[defaultIdx]
			}
		}
	}

	// Check for missing required positional arguments.
	for i := 0; i < nargs; i++ {
		if frame.Locals[i] == nil {
			return nil, object.NewTypeError(
				"%s() missing required positional argument: '%s'",
				fn.Name, code.Varnames[i])
		}
	}

	// --- 5. Fill in default values for unbound keyword-only parameters ---
	for i := 0; i < nKwOnly; i++ {
		localIdx := kwOnlyBase + i
		if frame.Locals[localIdx] == nil {
			kwName := code.Varnames[localIdx]
			if fn.KwDefaults != nil {
				if defVal, ok := fn.KwDefaults[kwName]; ok {
					frame.Locals[localIdx] = defVal
				}
			}
		}
	}

	// Check for missing required keyword-only arguments.
	for i := 0; i < nKwOnly; i++ {
		localIdx := kwOnlyBase + i
		if frame.Locals[localIdx] == nil {
			return nil, object.NewTypeError(
				"%s() missing required keyword-only argument: '%s'",
				fn.Name, code.Varnames[localIdx])
		}
	}

	// --- 6. Handle **kwargs collection ---
	if hasVarkw {
		// Insert in call-site order so the dict matches CPython's ordering.
		d := object.NewDict()
		for _, name := range extraKwNames {
			if err := d.SetItem(object.StrFrom(name), kw.Get(name)); err != nil {
				return nil, err
			}
		}
		kwargSlot := nargs
		if hasVarargs {
			kwargSlot++
		}
		kwargSlot += nKwOnly
		frame.Locals[kwargSlot] = d
	}

	// --- 7. Initialize cell objects for parameters that are cell variables ---
	for i, cellName := range code.Cellvars {
		for j, localName := range code.Varnames {
			if j < totalNamed && cellName == localName && frame.Locals[j] != nil {
				frame.Cells[i].Value = frame.Locals[j]
				break
			}
		}
	}

	return frame, nil
}

// lookupName looks up a name in locals (via the frame's globals for module scope),
// then globals, then builtins.
func lookupName(f *Frame, name string) (object.Object, error) {
	key := object.StrFrom(name)
	if val, err := f.Globals.GetItem(key); err == nil {
		return val, nil
	}
	if val, err := f.Builtins.GetItem(key); err == nil {
		return val, nil
	}
	return nil, object.NewNameError(name)
}

// getAttribute performs Python attribute lookup on obj.
// TODO(Phase 3): Implement the full descriptor protocol and MRO search.
func getAttribute(obj object.Object, name string) (object.Object, error) {
	if ah, ok := obj.(object.AttributeHolder); ok {
		return ah.GetAttr(name)
	}
	return nil, object.NewAttributeError(obj.Type().Name(), name)
}

// setAttribute performs Python attribute assignment on obj.
func setAttribute(obj object.Object, name string, value object.Object) error {
	if ah, ok := obj.(object.AttributeHolder); ok {
		return ah.SetAttr(name, value)
	}
	return object.NewAttributeError(obj.Type().Name(), name)
}

// constantToObject converts a constant pool value to an Object.
func constantToObject(c interface{}) object.Object {
	switch v := c.(type) {
	case nil:
		return object.None
	case bool:
		return object.BoolFrom(v)
	case int64:
		return object.IntFrom(v)
	case float64:
		return object.FloatFrom(v)
	case string:
		return object.StrFrom(v)
	case *bytecode.Code:
		return &codeConstant{code: v}
	case []interface{}:
		// Used for keyword argument name tuples in CALL_FUNCTION_KW.
		items := make([]object.Object, len(v))
		for i, item := range v {
			items[i] = constantToObject(item)
		}
		return object.NewTuple(items)
	}
	return object.None
}

// codeConstant wraps a *bytecode.Code as an Object for the constant pool.
// It is only used internally during MAKE_FUNCTION execution.
type codeConstant struct{ code *bytecode.Code }

func (c *codeConstant) Type() *object.TypeObject { return object.ObjectType }
func (c *codeConstant) String() string           { return fmt.Sprintf("<code object %s>", c.code.Name) }
func (c *codeConstant) Repr() string             { return c.String() }

// cellName returns the name of a cell variable by its index.
func cellName(f *Frame, idx int) string {
	if idx < len(f.Code.Cellvars) {
		return f.Code.Cellvars[idx]
	}
	freeIdx := idx - len(f.Code.Cellvars)
	if freeIdx < len(f.Code.Freevars) {
		return f.Code.Freevars[freeIdx]
	}
	return "?"
}

// toException converts an Object to an ExceptionObject.
// If obj is a type, instantiates it; if it is already an exception, returns it.
func toException(obj object.Object) *object.ExceptionObject {
	if exc, ok := obj.(*object.ExceptionObject); ok {
		return exc
	}
	if typ, ok := obj.(*object.TypeObject); ok {
		return object.NewException(typ, "")
	}
	return object.NewRuntimeError("invalid exception: %s", obj.Repr())
}

// wrapGoError converts a Go error to a Python ExceptionObject.
// It detects known exception type prefixes (e.g., "ZeroDivisionError: ...")
// from the error message and wraps with the correct exception type.
// Falls back to RuntimeError if no prefix is detected.
func wrapGoError(err error) *object.ExceptionObject {
	msg := err.Error()

	// Map of known exception type prefixes to their type objects.
	prefixes := []struct {
		prefix string
		typ    *object.TypeObject
	}{
		{"ZeroDivisionError: ", object.TypeZeroDivisionError},
		{"TypeError: ", object.TypeTypeError},
		{"ValueError: ", object.TypeValueError},
		{"NameError: ", object.TypeNameError},
		{"AttributeError: ", object.TypeAttributeError},
		{"IndexError: ", object.TypeIndexError},
		{"KeyError: ", object.TypeKeyError},
		{"RuntimeError: ", object.TypeRuntimeError},
		{"RecursionError: ", object.TypeRecursionError},
		{"ImportError: ", object.TypeImportError},
		{"ModuleNotFoundError: ", object.TypeModuleNotFoundError},
		{"StopIteration", object.TypeStopIteration},
		{"OverflowError: ", object.TypeOverflowError},
		{"NotImplementedError: ", object.TypeNotImplementedError},
	}

	for _, p := range prefixes {
		if strings.HasPrefix(msg, p.prefix) {
			excMsg := strings.TrimPrefix(msg, p.prefix)
			return object.NewException(p.typ, excMsg)
		}
	}

	return object.NewRuntimeError(msg)
}

// ============================================================
// Slice operation dispatchers
// ============================================================

// handleSliceGet dispatches a slice get operation to the appropriate type handler.
func handleSliceGet(container object.Object, sl *object.SliceObject) (object.Object, error) {
	switch c := container.(type) {
	case *object.ListObject:
		return object.SliceGetList(c, sl)
	case *object.TupleObject:
		return object.SliceGetTuple(c, sl)
	case *object.StrObject:
		return object.SliceGetStr(c, sl)
	default:
		return nil, object.NewTypeError("'%s' object is not subscriptable", container.Type().Name())
	}
}

// handleSliceSet dispatches a slice set operation to the appropriate type handler.
// Only mutable sequences (list) support slice assignment.
func handleSliceSet(container object.Object, sl *object.SliceObject, value object.Object) error {
	switch c := container.(type) {
	case *object.ListObject:
		return object.SliceSetList(c, sl, value)
	case *object.TupleObject:
		return object.NewTypeError("'tuple' object does not support item assignment")
	case *object.StrObject:
		return object.NewTypeError("'str' object does not support item assignment")
	default:
		return object.NewTypeError("'%s' object does not support item assignment", container.Type().Name())
	}
}

// handleSliceDel dispatches a slice delete operation to the appropriate type handler.
// Only mutable sequences (list) support slice deletion.
func handleSliceDel(container object.Object, sl *object.SliceObject) error {
	switch c := container.(type) {
	case *object.ListObject:
		return object.SliceDelList(c, sl)
	case *object.TupleObject:
		return object.NewTypeError("'tuple' object does not support item deletion")
	case *object.StrObject:
		return object.NewTypeError("'str' object does not support item deletion")
	default:
		return object.NewTypeError("'%s' object does not support item deletion", container.Type().Name())
	}
}
