package runtime

import (
	"strings"
	"testing"
)

// ============================================================
// Extended slice assignment regressions — Task 5.3
// Validates: Requirements 9.7, 9.8, 9.12
//
// SliceSetList originally treated step == -1 as a "simple" assignment that
// could grow or shrink the list. Python treats every step != 1 as an extended
// slice whose replacement must match the slice length exactly, so the old
// behaviour silently discarded elements instead of raising ValueError.
// Broader slicing coverage belongs to task 5.5.
// ============================================================

func TestSliceAssign_ReversedWrongLength_RaisesValueError(t *testing.T) {
	msg := runErr(t, `
b = [0, 1, 2, 3, 4]
b[::-1] = [1, 2]
`)
	if !strings.Contains(msg, "extended slice") {
		t.Errorf("expected an extended slice ValueError, got: %s", msg)
	}
	if !strings.Contains(msg, "size 2") || !strings.Contains(msg, "size 5") {
		t.Errorf("expected sizes 2 and 5 in message, got: %s", msg)
	}
}

func TestSliceAssign_ReversedWrongLength_LeavesListIntact(t *testing.T) {
	// The failed assignment must not have partially mutated the list.
	got := runOK(t, `
b = [0, 1, 2, 3, 4]
try:
    b[::-1] = [1, 2]
except ValueError:
    pass
print(b)
`)
	if got != "[0, 1, 2, 3, 4]" {
		t.Errorf("list was mutated by a failed assignment: got %q", got)
	}
}

func TestSliceAssign_ReversedExactLength(t *testing.T) {
	got := runOK(t, `
b = [0, 1, 2, 3, 4]
b[::-1] = [10, 20, 30, 40, 50]
print(b)
`)
	if got != "[50, 40, 30, 20, 10]" {
		t.Errorf("got %q, want %q", got, "[50, 40, 30, 20, 10]")
	}
}

func TestSliceAssign_NegativeStepExactLength(t *testing.T) {
	got := runOK(t, `
f = [0, 1, 2, 3, 4, 5]
f[::-2] = [10, 20, 30]
print(f)
`)
	if got != "[0, 30, 2, 20, 4, 10]" {
		t.Errorf("got %q, want %q", got, "[0, 30, 2, 20, 4, 10]")
	}
}

func TestSliceAssign_NegativeStepWrongLength_RaisesValueError(t *testing.T) {
	msg := runErr(t, `
d = [0, 1, 2, 3, 4]
d[::-2] = [9, 9]
`)
	if !strings.Contains(msg, "extended slice") {
		t.Errorf("expected an extended slice ValueError, got: %s", msg)
	}
}

func TestSliceAssign_PositiveStepWrongLength_RaisesValueError(t *testing.T) {
	msg := runErr(t, `
a = [0, 1, 2, 3, 4, 5]
a[::2] = [1, 2]
`)
	if !strings.Contains(msg, "extended slice") {
		t.Errorf("expected an extended slice ValueError, got: %s", msg)
	}
}

func TestSliceAssign_NonIterable_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `
c = [0, 1, 2]
c[0:2] = 5
`)
	// CPython: "must assign iterable to extended slice"
	if !strings.Contains(msg, "must assign iterable") {
		t.Errorf("expected 'must assign iterable', got: %s", msg)
	}
}

func TestSliceAssign_Step1MayChangeLength(t *testing.T) {
	// step == 1 remains the one case that can grow or shrink the list.
	got := runOK(t, `
a = [0, 1, 2, 3, 4]
a[1:3] = [10, 20, 30]
print(a)
b = [0, 1, 2, 3, 4]
b[1:4] = [9]
print(b)
`)
	want := "[0, 10, 20, 30, 3, 4]\n[0, 9, 4]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
