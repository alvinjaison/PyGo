package runtime

import (
	"strings"
	"testing"
)

// ============================================================
// Rich comparison for list, tuple and dict
//
// These types previously had no RichCompare, so the comparison protocol fell
// through to its identity fallback: `[1, 2] == [1, 2]` reported False and
// ordering raised TypeError. Expected values below were taken from CPython 3.
// ============================================================

func TestCollectionCompare_ListEquality(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"[1, 2, 3] == [1, 2, 3]", "True"},
		{"[1, 2, 3] == [1, 2, 4]", "False"},
		{"[] == []", "True"},
		{"[1, 2] == [1, 2, 3]", "False"},
		{"[1, 2, 3] != [1, 2, 3]", "False"},
		{"[1, 2, 3] != [1, 2, 4]", "True"},
		{"[1, [2, 3]] == [1, [2, 3]]", "True"},
		{"[1, [2, 3]] == [1, [2, 4]]", "False"},
		{"[1, 2] == [1.0, 2.0]", "True"},
		{"[True, False] == [1, 0]", "True"},
	}
	for _, c := range cases {
		if got := runOK(t, "print("+c.expr+")"); got != c.want {
			t.Errorf("%s: got %s, want %s", c.expr, got, c.want)
		}
	}
}

func TestCollectionCompare_ListOrdering(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"[1, 2] < [1, 3]", "True"},
		{"[1, 3] < [1, 2]", "False"},
		{"[1, 2] < [1, 2, 3]", "True"},
		{"[1, 2, 3] < [1, 2]", "False"},
		{"[] < [1]", "True"},
		{"[1, 2] <= [1, 2]", "True"},
		{"[1, 2] >= [1, 2]", "True"},
		{"[2] > [1, 9, 9]", "True"},
		{"[1] < [2.5]", "True"},
	}
	for _, c := range cases {
		if got := runOK(t, "print("+c.expr+")"); got != c.want {
			t.Errorf("%s: got %s, want %s", c.expr, got, c.want)
		}
	}
}

func TestCollectionCompare_TupleEqualityAndOrdering(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"(1, 2) == (1, 2)", "True"},
		{"(1, 2) == (1, 3)", "False"},
		{"() == ()", "True"},
		{"(1, 2) < (1, 3)", "True"},
		{"(1, 2) <= (1, 2)", "True"},
		{"(1,) < (1, 0)", "True"},
	}
	for _, c := range cases {
		if got := runOK(t, "print("+c.expr+")"); got != c.want {
			t.Errorf("%s: got %s, want %s", c.expr, got, c.want)
		}
	}
}

func TestCollectionCompare_DictEquality(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{`{"a": 1} == {"a": 1}`, "True"},
		{`{"a": 1, "b": 2} == {"b": 2, "a": 1}`, "True"},
		{`{"a": 1} == {"a": 2}`, "False"},
		{`{"a": 1} == {"b": 1}`, "False"},
		{`{"a": 1} == {"a": 1, "b": 2}`, "False"},
		{`{} == {}`, "True"},
		{`{"a": 1} != {"a": 2}`, "True"},
	}
	for _, c := range cases {
		if got := runOK(t, "print("+c.expr+")"); got != c.want {
			t.Errorf("%s: got %s, want %s", c.expr, got, c.want)
		}
	}
}

func TestCollectionCompare_DictOrdering_RaisesTypeError(t *testing.T) {
	// Python defines no ordering for dicts.
	msg := runErr(t, `print({"a": 1} < {"b": 2})`)
	if !strings.Contains(msg, "not supported between instances") {
		t.Errorf("expected an unsupported-ordering TypeError, got: %s", msg)
	}
}

func TestCollectionCompare_CrossTypeEqualityIsFalse(t *testing.T) {
	// Mismatched container types are unequal rather than an error.
	cases := []struct {
		expr string
		want string
	}{
		{"[1, 2] == (1, 2)", "False"},
		{"(1, 2) == [1, 2]", "False"},
		{`[1] == "1"`, "False"},
		{"[1] == 1", "False"},
	}
	for _, c := range cases {
		if got := runOK(t, "print("+c.expr+")"); got != c.want {
			t.Errorf("%s: got %s, want %s", c.expr, got, c.want)
		}
	}
}

func TestCollectionCompare_CrossTypeOrdering_RaisesTypeError(t *testing.T) {
	msg := runErr(t, `print([1, 2] < (1, 2))`)
	if !strings.Contains(msg, "not supported between instances") {
		t.Errorf("expected an unsupported-ordering TypeError, got: %s", msg)
	}
}

func TestCollectionCompare_MembershipUsesEquality(t *testing.T) {
	got := runOK(t, `
print([1, 2] in [[1, 2], [3]])
print([9] in [[1, 2], [3]])
print({"a": 1} in [{"a": 1}])
`)
	want := "True\nFalse\nTrue"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCollectionCompare_SortUsesOrdering(t *testing.T) {
	got := runOK(t, `
data = [[2, 1], [1, 9], [1, 2]]
data.sort()
print(data)
pairs = [(2, "b"), (1, "c"), (1, "a")]
pairs.sort()
print(pairs)
`)
	want := "[[1, 2], [1, 9], [2, 1]]\n[(1, 'a'), (1, 'c'), (2, 'b')]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCollectionCompare_IndexAndCountUseEquality(t *testing.T) {
	got := runOK(t, `
xs = [[1], [2], [1]]
print(xs.index([2]))
print(xs.count([1]))
`)
	want := "1\n2"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCollectionCompare_IdentityShortcut(t *testing.T) {
	// An element that is the same object counts as equal without consulting ==.
	got := runOK(t, `
a = [1, 2]
print([a] == [a])
b = [1, 2]
print(b == b)
`)
	want := "True\nTrue"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
