package vm

import (
	"github.com/alvinjaison/PyGo/internal/bytecode"
	"github.com/alvinjaison/PyGo/internal/object"
)

// maxPooledFrames bounds the per-VM free list.
const maxPooledFrames = 128

// acquireFrame returns a ready-to-use frame for code, reusing a pooled one when
// available. It is equivalent to NewFrame.
func (v *VM) acquireFrame(code *bytecode.Code, globals, builtins *object.DictObject, back *Frame) *Frame {
	n := len(v.framePool)
	if n == 0 {
		return NewFrame(code, globals, builtins, back)
	}
	f := v.framePool[n-1]
	v.framePool[n-1] = nil
	v.framePool = v.framePool[:n-1]

	nlocals := len(code.Varnames)
	need := nlocals + code.StackSize + 16
	if cap(f.slots) < need {
		f.slots = make([]object.Object, need)
	} else {
		f.slots = f.slots[:need] // already cleared by releaseFrame
	}
	f.Code = code
	f.Locals = f.slots[:nlocals:nlocals]
	f.Stack = f.slots[nlocals:]
	f.Globals = globals
	f.Builtins = builtins
	f.Back = back
	if n := len(code.Cellvars); n > 0 {
		f.Cells = make([]*object.CellObject, n)
		cells := make([]object.CellObject, n)
		for i := range f.Cells {
			f.Cells[i] = &cells[i]
		}
	}
	if n := len(code.Freevars); n > 0 {
		f.FreeVars = make([]*object.CellObject, n)
	}
	return f
}

// releaseFrame returns a finished frame to the pool. The caller must be sure
// nothing retains it: generator frames are never released, and closures
// capture cells rather than frames.
func (v *VM) releaseFrame(f *Frame) {
	if len(v.framePool) >= maxPooledFrames {
		return
	}
	clear(f.slots) // drop references so values can be collected
	f.Code = nil
	f.PC = 0
	f.Stack = nil
	f.StackTop = 0
	f.Locals = nil
	f.Cells = nil
	f.FreeVars = nil
	f.Globals = nil
	f.Builtins = nil
	f.Back = nil
	f.BlockStack = f.BlockStack[:0]
	f.CurrentException = nil
	f.lastPoppedWith = nil
	f.genStarted = false
	f.pendingThrow = nil
	f.Yielded = false
	v.framePool = append(v.framePool, f)
}
