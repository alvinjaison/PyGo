package vm

import (
	"testing"

	"github.com/alvinjaison/pygo/internal/bytecode"
	"github.com/alvinjaison/pygo/internal/object"
)

// helper to create a minimal frame for testing stack operations.
func newTestFrame() *Frame {
	code := &bytecode.Code{
		Name:      "<test>",
		Filename:  "<test>",
		StackSize: 32,
		Varnames:  []string{},
	}
	globals := object.NewDict()
	builtins := object.NewDict()
	return NewFrame(code, globals, builtins, nil)
}

// ============================================================
// Stack operations
// ============================================================

func TestPushPop(t *testing.T) {
	f := newTestFrame()

	f.Push(object.IntFrom(42))
	if f.StackTop != 1 {
		t.Fatalf("expected StackTop=1, got %d", f.StackTop)
	}

	v := f.Pop()
	if f.StackTop != 0 {
		t.Fatalf("expected StackTop=0 after pop, got %d", f.StackTop)
	}
	if v.(*object.IntObject).Value.Int64() != 42 {
		t.Fatalf("expected 42, got %s", v.String())
	}
}

func TestPeek(t *testing.T) {
	f := newTestFrame()
	f.Push(object.IntFrom(10))
	f.Push(object.IntFrom(20))

	v := f.Peek()
	if v.(*object.IntObject).Value.Int64() != 20 {
		t.Fatalf("expected 20, got %s", v.String())
	}
	// Peek should not remove the value.
	if f.StackTop != 2 {
		t.Fatalf("expected StackTop=2 after peek, got %d", f.StackTop)
	}
}

func TestPeekN(t *testing.T) {
	f := newTestFrame()
	f.Push(object.IntFrom(1))
	f.Push(object.IntFrom(2))
	f.Push(object.IntFrom(3))

	// PeekN(0) is the top.
	v := f.PeekN(0)
	if v.(*object.IntObject).Value.Int64() != 3 {
		t.Fatalf("PeekN(0): expected 3, got %s", v.String())
	}
	v = f.PeekN(1)
	if v.(*object.IntObject).Value.Int64() != 2 {
		t.Fatalf("PeekN(1): expected 2, got %s", v.String())
	}
	v = f.PeekN(2)
	if v.(*object.IntObject).Value.Int64() != 1 {
		t.Fatalf("PeekN(2): expected 1, got %s", v.String())
	}
}

func TestPopN(t *testing.T) {
	f := newTestFrame()
	f.Push(object.IntFrom(1))
	f.Push(object.IntFrom(2))
	f.Push(object.IntFrom(3))

	f.PopN(2)
	if f.StackTop != 1 {
		t.Fatalf("expected StackTop=1, got %d", f.StackTop)
	}
	v := f.Pop()
	if v.(*object.IntObject).Value.Int64() != 1 {
		t.Fatalf("expected 1, got %s", v.String())
	}
}

func TestRotTwo(t *testing.T) {
	f := newTestFrame()
	f.Push(object.IntFrom(1)) // bottom
	f.Push(object.IntFrom(2)) // top

	f.RotTwo()

	// After RotTwo: top should be 1, second should be 2.
	top := f.Pop()
	second := f.Pop()
	if top.(*object.IntObject).Value.Int64() != 1 {
		t.Fatalf("expected top=1 after RotTwo, got %s", top.String())
	}
	if second.(*object.IntObject).Value.Int64() != 2 {
		t.Fatalf("expected second=2 after RotTwo, got %s", second.String())
	}
}

func TestRotThree(t *testing.T) {
	f := newTestFrame()
	f.Push(object.IntFrom(1)) // third (bottom)
	f.Push(object.IntFrom(2)) // second
	f.Push(object.IntFrom(3)) // TOS

	f.RotThree()

	// CPython ROT_THREE: TOS moves to third, second→TOS, third→second.
	// Before: [1, 2, 3] (3=TOS)
	// After:  [3, 1, 2] (2=TOS)
	top := f.Pop()
	second := f.Pop()
	third := f.Pop()
	if top.(*object.IntObject).Value.Int64() != 2 {
		t.Fatalf("expected TOS=2 after RotThree, got %s", top.String())
	}
	if second.(*object.IntObject).Value.Int64() != 1 {
		t.Fatalf("expected second=1 after RotThree, got %s", second.String())
	}
	if third.(*object.IntObject).Value.Int64() != 3 {
		t.Fatalf("expected third=3 after RotThree, got %s", third.String())
	}
}

func TestDupTop(t *testing.T) {
	f := newTestFrame()
	f.Push(object.IntFrom(7))

	f.DupTop()

	if f.StackTop != 2 {
		t.Fatalf("expected StackTop=2 after DupTop, got %d", f.StackTop)
	}
	a := f.Pop()
	b := f.Pop()
	if a.(*object.IntObject).Value.Int64() != 7 {
		t.Fatalf("expected 7 from DupTop copy, got %s", a.String())
	}
	if b.(*object.IntObject).Value.Int64() != 7 {
		t.Fatalf("expected 7 from original, got %s", b.String())
	}
}

// ============================================================
// Block stack operations
// ============================================================

func TestPushPopBlock(t *testing.T) {
	f := newTestFrame()

	f.PushBlock(BlockLoop, 10)
	if !f.HasBlock() {
		t.Fatal("expected HasBlock() == true")
	}

	b := f.PopBlock()
	if b.Type != BlockLoop {
		t.Fatalf("expected BlockLoop, got %d", b.Type)
	}
	if b.Handler != 10 {
		t.Fatalf("expected handler=10, got %d", b.Handler)
	}
	if b.Level != 0 {
		t.Fatalf("expected level=0, got %d", b.Level)
	}

	if f.HasBlock() {
		t.Fatal("expected HasBlock() == false after pop")
	}
}

func TestBlockStackLevel(t *testing.T) {
	f := newTestFrame()
	f.Push(object.IntFrom(1))
	f.Push(object.IntFrom(2))

	f.PushBlock(BlockExcept, 20)

	b := f.PopBlock()
	if b.Level != 2 {
		t.Fatalf("expected level=2 (stack depth when block was entered), got %d", b.Level)
	}
}

func TestBlockTypes(t *testing.T) {
	// Verify block type constants have expected values.
	if BlockLoop != 0 {
		t.Fatalf("expected BlockLoop=0, got %d", BlockLoop)
	}
	if BlockExcept != 1 {
		t.Fatalf("expected BlockExcept=1, got %d", BlockExcept)
	}
	if BlockFinally != 2 {
		t.Fatalf("expected BlockFinally=2, got %d", BlockFinally)
	}
}

func TestUnwindToHandler(t *testing.T) {
	f := newTestFrame()
	f.Push(object.IntFrom(1))
	f.PushBlock(BlockExcept, 50)
	f.Push(object.IntFrom(2))
	f.Push(object.IntFrom(3))

	b, found := f.UnwindToHandler()
	if !found {
		t.Fatal("expected to find handler")
	}
	if b.Type != BlockExcept {
		t.Fatalf("expected BlockExcept, got %d", b.Type)
	}
	if b.Handler != 50 {
		t.Fatalf("expected handler=50, got %d", b.Handler)
	}
	// Stack should be unwound to level 1 (depth when block was entered).
	if f.StackTop != 1 {
		t.Fatalf("expected StackTop=1 after unwind, got %d", f.StackTop)
	}
}

func TestUnwindToHandlerNoHandler(t *testing.T) {
	f := newTestFrame()
	f.PushBlock(BlockLoop, 10) // loops are not exception handlers

	_, found := f.UnwindToHandler()
	if found {
		t.Fatal("expected no handler found for loop-only block stack")
	}
}

// ============================================================
// NewFrame constructor
// ============================================================

func TestNewFrame(t *testing.T) {
	code := &bytecode.Code{
		Name:      "test_func",
		Filename:  "test.py",
		StackSize: 8,
		Varnames:  []string{"a", "b", "c"},
		Cellvars:  []string{"x"},
		Freevars:  []string{"y"},
	}
	globals := object.NewDict()
	builtins := object.NewDict()

	f := NewFrame(code, globals, builtins, nil)

	if f.Code != code {
		t.Fatal("Code not set")
	}
	if f.PC != 0 {
		t.Fatalf("expected PC=0, got %d", f.PC)
	}
	if len(f.Stack) < code.StackSize {
		t.Fatalf("stack too small: %d < %d", len(f.Stack), code.StackSize)
	}
	if f.StackTop != 0 {
		t.Fatalf("expected StackTop=0, got %d", f.StackTop)
	}
	if len(f.Locals) != 3 {
		t.Fatalf("expected 3 locals, got %d", len(f.Locals))
	}
	if len(f.Cells) != 1 {
		t.Fatalf("expected 1 cell, got %d", len(f.Cells))
	}
	if f.Cells[0] == nil {
		t.Fatal("cells not initialized")
	}
	if len(f.FreeVars) != 1 {
		t.Fatalf("expected 1 freevar, got %d", len(f.FreeVars))
	}
	if f.Globals != globals {
		t.Fatal("Globals not set")
	}
	if f.Builtins != builtins {
		t.Fatal("Builtins not set")
	}
	if f.Back != nil {
		t.Fatal("expected Back=nil for top-level frame")
	}
}
