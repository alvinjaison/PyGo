// Package vm implements the PyGo bytecode virtual machine.
//
// The VM executes Code objects produced by the compiler. It maintains a
// call stack of Frames, each holding the state of one function invocation.
//
// Execution model: stack-based. Each Frame has a value stack, a block stack
// for exception and loop context, and a local variable array.
//
// See docs/architecture.md §3.6 for the design rationale.
package vm

import (
	"github.com/pygo/pygo/internal/bytecode"
	"github.com/pygo/pygo/internal/object"
)

// Frame holds the execution state of a single function invocation.
// Frames are chained via the Back pointer to form the call stack.
type Frame struct {
	// Code is the code object being executed.
	Code *bytecode.Code

	// PC is the program counter — the index of the next instruction to execute.
	PC int

	// Stack is the value stack for this frame.
	// Values are pushed and popped during instruction execution.
	Stack []object.Object

	// StackTop is the current top-of-stack index (Stack[0..StackTop-1] are live).
	StackTop int

	// Locals holds local variable values indexed by variable slot number.
	// Indexed by the operand of LOAD_FAST / STORE_FAST.
	Locals []object.Object

	// Cells holds cell objects for variables referenced by nested functions.
	// Indexed by cellvar slot number.
	Cells []*object.CellObject

	// FreeVars holds cell objects for variables captured from enclosing scopes.
	// Indexed by freevar slot number.
	FreeVars []*object.CellObject

	// Globals is the global namespace (a module's __dict__).
	Globals *object.DictObject

	// Builtins is the built-in namespace (__builtins__.__dict__).
	Builtins *object.DictObject

	// Back is the calling frame, or nil for the top-level frame.
	Back *Frame

	// BlockStack holds exception/loop context blocks.
	BlockStack []Block

	// CurrentException holds the exception currently being handled in an
	// except clause, or nil if none.
	CurrentException *object.ExceptionObject

	// lastPoppedWith holds the most recently popped with-block, so the
	// following WITH_CLEANUP can invoke its __exit__ on the normal path.
	lastPoppedWith *Block

	// Yielded is true when this frame has yielded and is suspended.
	// Used for generator frames.
	Yielded bool
}

// NewFrame creates a new Frame for the given Code object.
func NewFrame(code *bytecode.Code, globals, builtins *object.DictObject, back *Frame) *Frame {
	f := &Frame{
		Code:     code,
		PC:       0,
		Stack:    make([]object.Object, code.StackSize+16), // small headroom
		Locals:   make([]object.Object, len(code.Varnames)),
		Cells:    make([]*object.CellObject, len(code.Cellvars)),
		FreeVars: make([]*object.CellObject, len(code.Freevars)),
		Globals:  globals,
		Builtins: builtins,
		Back:     back,
	}
	// Initialize cell objects.
	for i := range f.Cells {
		f.Cells[i] = &object.CellObject{}
	}
	return f
}

// Push pushes a value onto the value stack.
func (f *Frame) Push(v object.Object) {
	f.Stack[f.StackTop] = v
	f.StackTop++
}

// Pop pops and returns the top value from the value stack.
// Panics if the stack is empty (indicates a compiler bug).
func (f *Frame) Pop() object.Object {
	f.StackTop--
	v := f.Stack[f.StackTop]
	f.Stack[f.StackTop] = nil // allow GC
	return v
}

// Peek returns the top value without popping it.
func (f *Frame) Peek() object.Object {
	return f.Stack[f.StackTop-1]
}

// PeekN returns the n-th value from the top (0 = top).
func (f *Frame) PeekN(n int) object.Object {
	return f.Stack[f.StackTop-1-n]
}

// PopN pops and discards n values from the stack.
func (f *Frame) PopN(n int) {
	for i := 0; i < n; i++ {
		f.Stack[f.StackTop-1-i] = nil
	}
	f.StackTop -= n
}

// ============================================================
// Block stack
// ============================================================

// BlockType identifies the type of a block stack entry.
type BlockType int

const (
	// BlockLoop is a for/while loop block.
	BlockLoop BlockType = iota

	// BlockExcept is a try/except block.
	BlockExcept

	// BlockFinally is a try/finally block.
	BlockFinally

	// BlockWith is a with statement block.
	BlockWith
)

// Block represents an active exception-handling or loop context.
type Block struct {
	Type    BlockType
	Handler int // bytecode offset of the handler (SETUP_FINALLY target)
	Level   int // value stack depth when block was entered

	// Mgr holds the context-manager object for a BlockWith, so the VM can
	// call its __exit__ during unwinding. Nil for non-with blocks.
	Mgr object.Object
}

// PushBlock pushes a new block onto the block stack.
func (f *Frame) PushBlock(bt BlockType, handler int) {
	f.BlockStack = append(f.BlockStack, Block{
		Type:    bt,
		Handler: handler,
		Level:   f.StackTop,
	})
}

// PushWithBlock pushes a with-statement block that remembers its context
// manager so __exit__ can be invoked on both the normal and exception paths.
func (f *Frame) PushWithBlock(handler int, mgr object.Object) {
	f.BlockStack = append(f.BlockStack, Block{
		Type:    BlockWith,
		Handler: handler,
		Level:   f.StackTop,
		Mgr:     mgr,
	})
}

// PopBlock pops and returns the top block.
func (f *Frame) PopBlock() Block {
	b := f.BlockStack[len(f.BlockStack)-1]
	f.BlockStack = f.BlockStack[:len(f.BlockStack)-1]
	return b
}

// HasBlock reports whether the block stack is non-empty.
func (f *Frame) HasBlock() bool { return len(f.BlockStack) > 0 }

// UnwindToHandler searches the block stack for an exception handler and
// prepares the frame to jump to it. Returns the handler block if found,
// and false if the frame has no handler for this exception.
func (f *Frame) UnwindToHandler() (Block, bool) {
	for i := len(f.BlockStack) - 1; i >= 0; i-- {
		b := f.BlockStack[i]
		if b.Type == BlockExcept || b.Type == BlockFinally {
			f.BlockStack = f.BlockStack[:i]
			// Restore the stack to the saved depth.
			for f.StackTop > b.Level {
				f.Pop()
			}
			return b, true
		}
	}
	return Block{}, false
}

// Traceback builds a TracebackObject from this frame chain. Synthetic frames
// without a Code object (e.g. the throwaway caller used by CallExternal) are
// skipped.
func (f *Frame) Traceback() *object.TracebackObject {
	var frames []object.TracebackFrame
	for cur := f; cur != nil; cur = cur.Back {
		if cur.Code == nil {
			continue
		}
		frames = append([]object.TracebackFrame{{
			Filename: cur.Code.Filename,
			FuncName: cur.Code.Name,
			Line:     cur.Code.LineForOffset(cur.PC - 1),
		}}, frames...)
	}
	return &object.TracebackObject{Frames: frames}
}
