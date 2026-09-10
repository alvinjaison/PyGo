package vm

import "github.com/alvinjaison/pygo/internal/object"

// kwargSet carries the keyword arguments of a single call, together with the
// order in which they appeared at the call site.
//
// The order matters: unmatched keywords are collected into a **kwargs dict, and
// Python guarantees that dict reflects call order. A plain map[string]Object
// cannot provide that because Go randomises map iteration order, which also made
// "unexpected keyword argument" errors report an arbitrary offender when several
// keywords were wrong. Names is the authoritative iteration order; Values is
// kept alongside it so builtins (which take a map) need no conversion.
type kwargSet struct {
	// Names lists keyword names in first-seen call-site order.
	Names []string

	// Values maps each name in Names to its argument value.
	Values map[string]object.Object
}

// newKwargSet returns an empty set sized for n keyword arguments.
func newKwargSet(n int) *kwargSet {
	return &kwargSet{
		Names:  make([]string, 0, n),
		Values: make(map[string]object.Object, n),
	}
}

// Set records a keyword argument. A repeated name keeps its original position
// and overwrites the value, matching dict-update semantics.
func (k *kwargSet) Set(name string, value object.Object) {
	if _, seen := k.Values[name]; !seen {
		k.Names = append(k.Names, name)
	}
	k.Values[name] = value
}

// Order returns the keyword names in call-site order. Safe on a nil receiver so
// callers that pass no keywords need no special-casing.
func (k *kwargSet) Order() []string {
	if k == nil {
		return nil
	}
	return k.Names
}

// Get returns the value bound to name.
func (k *kwargSet) Get(name string) object.Object {
	if k == nil {
		return nil
	}
	return k.Values[name]
}

// Len returns the number of keyword arguments.
func (k *kwargSet) Len() int {
	if k == nil {
		return 0
	}
	return len(k.Names)
}

// Map returns the keyword arguments as a map for Go-implemented callables,
// which take map[string]object.Object. Returns nil when there are none.
func (k *kwargSet) Map() map[string]object.Object {
	if k == nil || len(k.Values) == 0 {
		return nil
	}
	return k.Values
}
