package vm

import (
	"context"
	"fmt"
	"math/big"

	"github.com/pygo/pygo/internal/bytecode"
	"github.com/pygo/pygo/internal/object"
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

	// importer resolves modules for IMPORT_NAME / IMPORT_FROM. It is optional;
	// when nil, import statements raise ImportError.
	importer Importer

	// frames is the active frame stack, maintained so zero-argument super()
	// can discover the executing method's instance and defining class.
	frames []*Frame
}

// pushFrame / popFrame maintain the active frame stack and keep this VM marked
// as the active VM while it has frames executing.
func (v *VM) pushFrame(f *Frame) {
	v.frames = append(v.frames, f)
	pushActiveVM(v)
}
func (v *VM) popFrame() {
	if len(v.frames) > 0 {
		v.frames = v.frames[:len(v.frames)-1]
	}
	popActiveVM()
}

// topFrame returns the currently-executing frame, or nil.
func (v *VM) topFrame() *Frame {
	if len(v.frames) == 0 {
		return nil
	}
	return v.frames[len(v.frames)-1]
}

// FrameContext reports the executing frame's instance (first local) and the
// class whose method body is running, for zero-argument super().
func (v *VM) FrameContext() *object.FrameContext {
	f := v.topFrame()
	if f == nil || f.Code == nil || len(f.Locals) == 0 {
		return nil
	}
	self := f.Locals[0]
	if self == nil {
		return nil
	}
	inst, ok := self.(*object.InstanceObject)
	if !ok {
		return &object.FrameContext{Self: self}
	}
	// Discover the defining class: the class in type(self).__mro__ whose *own*
	// dict holds a function whose code matches the running frame's code.
	for _, cls := range inst.Type().MRO() {
		if fn, err := cls.Dict().GetItem(object.StrFrom(f.Code.Name)); err == nil {
			if fo, ok := fn.(*object.FunctionObject); ok && fo.Code == f.Code {
				return &object.FrameContext{Self: self, DefiningClass: cls}
			}
		}
	}
	return &object.FrameContext{Self: self}
}

// Importer resolves a module name to a module object. The runtime supplies an
// implementation; the VM calls it to execute import statements.
type Importer interface {
	Import(name string) (object.Object, error)
}

// SetImporter installs the module importer used by import statements.
func (v *VM) SetImporter(im Importer) { v.importer = im }

// New creates a new VM with the given options.
func New(ctx context.Context, maxDepth int) *VM {
	if ctx == nil {
		ctx = context.Background()
	}
	if maxDepth <= 0 {
		maxDepth = 500
	}
	installFrameContextHook()
	return &VM{ctx: ctx, maxDepth: maxDepth}
}

// activeVMs is a stack of VMs currently executing a frame. The topmost entry
// serves zero-argument super()'s frame-context queries. A single Runtime runs
// on one goroutine at a time, so this stack reflects the active call chain.
var activeVMs []*VM

func pushActiveVM(v *VM) { activeVMs = append(activeVMs, v) }
func popActiveVM() {
	if n := len(activeVMs); n > 0 {
		activeVMs = activeVMs[:n-1]
	}
}
func currentActiveVM() *VM {
	if n := len(activeVMs); n > 0 {
		return activeVMs[n-1]
	}
	return nil
}

var frameHookInstalled bool

// installFrameContextHook wires object.CurrentFrameContext to the active VM
// once, so zero-argument super() can resolve its instance and defining class.
func installFrameContextHook() {
	if frameHookInstalled {
		return
	}
	frameHookInstalled = true
	object.SetFrameContextHook(func() *object.FrameContext {
		if v := currentActiveVM(); v != nil {
			return v.FrameContext()
		}
		return nil
	})
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
			if debugPanics {
				panic(r)
			}
			err = fmt.Errorf("internal VM error: %v", r)
		}
	}()

	v.depth++
	defer func() { v.depth-- }()

	v.pushFrame(frame)
	defer v.popFrame()

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
				// Wrap non-exception errors as RuntimeError.
				exc = object.NewRuntimeError(stepErr.Error())
			}
			// Unwind the block stack, running with-statement __exit__ handlers
			// and jumping to any exception handler found.
			handled, propagate := v.unwind(frame, exc)
			if handled {
				continue
			}
			if propagate.Traceback == nil {
				propagate.Traceback = frame.Traceback()
			}
			return nil, propagate
		}

		// A non-nil result signals RETURN_VALUE (or YIELD_VALUE) was executed.
		if result != nil {
			return result, nil
		}
	}
}

// runFrame advances a frame until it yields or returns. It is the core used by
// both Run (for ordinary calls) and generator resumption.
//
// It returns:
//
//	value   — the yielded or returned value
//	yielded — true if execution stopped at a YIELD_VALUE (frame is suspended)
//	err     — a Python exception or cancellation error
func (v *VM) runFrame(frame *Frame) (value object.Object, yielded bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			if debugPanics {
				panic(r)
			}
			err = fmt.Errorf("internal VM error: %v", r)
		}
	}()

	v.depth++
	defer func() { v.depth-- }()

	v.pushFrame(frame)
	defer v.popFrame()

	if v.depth > v.maxDepth {
		return nil, false, object.NewException(object.TypeRecursionError,
			fmt.Sprintf("maximum recursion depth exceeded (%d)", v.maxDepth))
	}

	for {
		select {
		case <-v.ctx.Done():
			return nil, false, v.ctx.Err()
		default:
		}

		if frame.PC >= len(frame.Code.Instructions) {
			return object.None, false, nil
		}

		instr := frame.Code.Instructions[frame.PC]
		frame.PC++
		v.instrCount++
		if v.maxInstr > 0 && v.instrCount > v.maxInstr {
			return nil, false, object.NewRuntimeError("instruction limit exceeded")
		}

		result, stepErr := v.execute(frame, instr)
		if stepErr != nil {
			exc, isExc := stepErr.(*object.ExceptionObject)
			if !isExc {
				exc = object.NewRuntimeError(stepErr.Error())
			}
			handled, propagate := v.unwind(frame, exc)
			if handled {
				continue
			}
			if propagate.Traceback == nil {
				propagate.Traceback = frame.Traceback()
			}
			return nil, false, propagate
		}

		if result != nil {
			if frame.Yielded {
				frame.Yielded = false
				return result, true, nil
			}
			return result, false, nil
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
		a, b := f.Pop(), f.Pop()
		f.Push(a)
		f.Push(b)

	case bytecode.ROT_THREE:
		a, b, c := f.Pop(), f.Pop(), f.Pop()
		f.Push(a)
		f.Push(c)
		f.Push(b)

	case bytecode.DUP_TOP:
		f.Push(f.Peek())

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

	case bytecode.LOAD_CLOSURE:
		// Push the cell object itself (used to build a closure tuple for
		// MAKE_FUNCTION), not the value inside it.
		idx := int(instr.Arg)
		var cell *object.CellObject
		if idx < len(f.Cells) {
			cell = f.Cells[idx]
		} else if idx-len(f.Cells) < len(f.FreeVars) {
			cell = f.FreeVars[idx-len(f.Cells)]
		} else {
			cell = &object.CellObject{}
		}
		f.Push(cell)

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
		if s, ok := container.(object.GetItemer); ok {
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
		if s, ok := container.(object.Subscriptable); ok {
			return nil, s.SetItem(key, val)
		}
		return nil, object.NewTypeError("'%s' object does not support item assignment", container.Type().Name())

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
		if bytecode.CompareOp(instr.Arg) == bytecode.CmpExcMatch {
			f.Push(object.BoolFrom(excMatches(left, right)))
			break
		}
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
		count := int(instr.Arg)
		items := make([]object.Object, count)
		for i := count - 1; i >= 0; i-- {
			items[i] = f.Pop()
		}
		set, err := object.NewSetFrom(items)
		if err != nil {
			return nil, err
		}
		f.Push(set)

	case bytecode.BUILD_SLICE:
		step := f.Pop()
		stop := f.Pop()
		start := f.Pop()
		f.Push(object.NewSlice(start, stop, step))

	case bytecode.LIST_APPEND:
		// TOS is the value; the list is below the comprehension's loop state.
		// The compiler arranges the list to be at a fixed depth; here the list
		// is the value just under TOS after FOR_ITER pushes. We locate it as
		// the nearest ListObject below TOS.
		val := f.Pop()
		lst := f.findListBelow()
		if lst == nil {
			return nil, object.NewRuntimeError("LIST_APPEND: no list on stack")
		}
		lst.Append(val)

	case bytecode.SET_ADD:
		val := f.Pop()
		set := f.findSetBelow()
		if set == nil {
			return nil, object.NewRuntimeError("SET_ADD: no set on stack")
		}
		if err := set.Add(val); err != nil {
			return nil, err
		}

	case bytecode.MAP_ADD:
		// Dict comprehension pushes key then value; both are on top.
		val := f.Pop()
		key := f.Pop()
		d := f.findDictBelow()
		if d == nil {
			return nil, object.NewRuntimeError("MAP_ADD: no dict on stack")
		}
		if err := d.SetItem(key, val); err != nil {
			return nil, err
		}

	case bytecode.DUP_TOP_TWO:
		a := f.PeekN(1)
		b := f.PeekN(0)
		f.Push(a)
		f.Push(b)

	case bytecode.ROT_FOUR:
		a := f.Pop()
		b := f.Pop()
		c := f.Pop()
		d := f.Pop()
		f.Push(a)
		f.Push(d)
		f.Push(c)
		f.Push(b)

	case bytecode.CALL_FUNCTION_KW:
		total := int(instr.Arg)
		kwConst := f.Pop()
		kwNames := extractKwNames(kwConst)
		nkw := len(kwNames)
		npos := total - nkw

		kwargs := map[string]object.Object{}
		kwVals := make([]object.Object, nkw)
		for i := nkw - 1; i >= 0; i-- {
			kwVals[i] = f.Pop()
		}
		for i, name := range kwNames {
			kwargs[name] = kwVals[i]
		}
		posArgs := make([]object.Object, npos)
		for i := npos - 1; i >= 0; i-- {
			posArgs[i] = f.Pop()
		}
		callee := f.Pop()
		result, err := v.callObject(f, callee, posArgs, kwargs)
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.RERAISE:
		if f.CurrentException != nil {
			return nil, f.CurrentException
		}
		exc := f.Pop()
		return nil, toException(exc)

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
		// The compiler emits the handler as an absolute instruction offset.
		f.PushBlock(BlockExcept, int(instr.Arg))

	case bytecode.POP_BLOCK:
		if f.HasBlock() {
			b := f.PopBlock()
			if b.Type == BlockWith {
				// Remember it so the following WITH_CLEANUP can run __exit__.
				bc := b
				f.lastPoppedWith = &bc
			}
		}

	case bytecode.POP_EXCEPT:
		// Clears the exception currently being handled. The try block was
		// already removed from the block stack during unwinding.
		f.CurrentException = nil

	// ---- Context managers ----

	case bytecode.SETUP_WITH:
		// TOS is the context manager. Call __enter__, push a with-block that
		// remembers the manager (for __exit__ on unwind), and push the
		// __enter__ result for binding.
		mgr := f.Pop()
		enter, err := getAttribute(mgr, "__enter__")
		if err != nil {
			return nil, object.NewTypeError(
				"'%s' object does not support the context manager protocol", mgr.Type().Name())
		}
		// Verify __exit__ exists up front for a clear error.
		if _, err := getAttribute(mgr, "__exit__"); err != nil {
			return nil, object.NewTypeError(
				"'%s' object does not support the context manager protocol (missing __exit__)",
				mgr.Type().Name())
		}
		f.PushWithBlock(int(instr.Arg), mgr)
		result, err := v.callObject(f, enter, nil, nil)
		if err != nil {
			return nil, err
		}
		f.Push(result)

	case bytecode.WITH_CLEANUP:
		// Normal (no-exception) completion of a with body: call
		// __exit__(None, None, None). POP_BLOCK already removed the block.
		if b := f.lastPoppedWith; b != nil {
			if _, exitErr := v.callExit(f, b.Mgr, nil); exitErr != nil {
				f.lastPoppedWith = nil
				return nil, exitErr
			}
			f.lastPoppedWith = nil
		}

	// ---- Import ----

	case bytecode.IMPORT_NAME:
		name := f.Code.Names[instr.Arg]
		if v.importer == nil {
			return nil, object.NewException(object.TypeImportError,
				"import is not available in this runtime")
		}
		mod, err := v.importer.Import(name)
		if err != nil {
			return nil, err
		}
		f.Push(mod)

	case bytecode.IMPORT_FROM:
		// TOS is the module; read an attribute off it, leaving the module in
		// place (the compiler pops it afterward).
		name := f.Code.Names[instr.Arg]
		mod := f.Peek()
		holder, ok := mod.(object.AttributeHolder)
		if !ok {
			return nil, object.NewException(object.TypeImportError,
				"cannot import name '"+name+"'")
		}
		val, err := holder.GetAttr(name)
		if err != nil {
			return nil, object.NewException(object.TypeImportError,
				"cannot import name '"+name+"'")
		}
		f.Push(val)

	// ---- Generator ----

	case bytecode.YIELD_VALUE:
		// Pop the value to yield and suspend the frame. On resume, the yield
		// expression evaluates to None (the value from a bare next(); .send()
		// is not yet supported), so push None for the resumed continuation.
		val := f.Pop()
		f.Push(object.None)
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
func (v *VM) callObject(caller *Frame, callee object.Object, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	switch fn := callee.(type) {
	case *object.FunctionObject:
		newFrame, err := buildFrame(fn, args, kwargs, caller)
		if err != nil {
			return nil, err
		}
		if fn.Code.IsGenerator() {
			return v.makeGenerator(fn.Name, newFrame), nil
		}
		return v.Run(newFrame)

	case *object.BuiltinFuncObject:
		return fn.Call(args, kwargs)

	case *object.BoundMethodObject:
		fullArgs := make([]object.Object, len(args)+1)
		fullArgs[0] = fn.Self
		copy(fullArgs[1:], args)
		return v.callObject(caller, fn.Function, fullArgs, kwargs)

	case *object.TypeObject:
		return v.instantiate(caller, fn, args, kwargs)

	default:
		if c, ok := callee.(object.Callable); ok {
			return c.Call(args, kwargs)
		}
		return nil, object.NewTypeError("'%s' object is not callable", callee.Type().Name())
	}
}

// buildFrame creates a new Frame for a Python function call, binding arguments to locals.
func buildFrame(fn *object.FunctionObject, args []object.Object, kwargs map[string]object.Object, caller *Frame) (*Frame, error) {
	code := fn.Code
	frame := NewFrame(code, fn.Globals, caller.Builtins, caller)

	// Bind free variables from closure.
	copy(frame.FreeVars, fn.Closure)

	nargs := code.ArgCount
	nposonly := code.PosOnlyArgCount
	defer seedCellParams(frame)

	// Bind positional arguments.
	if len(args) < nargs-len(fn.Defaults) {
		return nil, object.NewTypeError(
			"%s() missing %d required positional argument(s)",
			fn.Name, nargs-len(fn.Defaults)-len(args))
	}
	_ = nposonly
	for i := 0; i < len(args) && i < nargs; i++ {
		frame.Locals[i] = args[i]
	}

	// Bind default values for unbound positional args.
	for i := len(args); i < nargs; i++ {
		defaultIdx := i - (nargs - len(fn.Defaults))
		if defaultIdx >= 0 && defaultIdx < len(fn.Defaults) {
			frame.Locals[i] = fn.Defaults[defaultIdx]
		}
	}

	// Handle *args.
	if code.HasFlag(bytecode.FlagVarargs) {
		varargStart := nargs
		varargItems := args[varargStart:]
		frame.Locals[nargs] = object.NewTuple(varargItems)
	}

	// Handle **kwargs.
	if code.HasFlag(bytecode.FlagVarkeywords) {
		d := object.NewDict()
		for k, v := range kwargs {
			if err := d.SetItem(object.StrFrom(k), v); err != nil {
				return nil, err
			}
		}
		kwargSlot := nargs
		if code.HasFlag(bytecode.FlagVarargs) {
			kwargSlot++
		}
		kwargSlot += code.KwOnlyArgCount
		frame.Locals[kwargSlot] = d
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
	case *big.Int:
		return object.IntFromBig(v)
	case *bytecode.Code:
		return &codeConstant{code: v}
	}
	// Keyword-name tuple produced by the compiler for CALL_FUNCTION_KW is
	// handled specially in the VM, not converted here. Any other constant is
	// wrapped as a passthrough so it can be recovered via type assertion.
	return &rawConstant{value: c}
}

// rawConstant carries a non-Python compiler constant (e.g., the keyword-name
// tuple) through the value stack so the VM can recover it by type assertion.
type rawConstant struct{ value interface{} }

func (r *rawConstant) Type() *object.TypeObject { return object.ObjectType }
func (r *rawConstant) String() string           { return "<raw-constant>" }
func (r *rawConstant) Repr() string             { return r.String() }

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
