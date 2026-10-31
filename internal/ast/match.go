package ast

// Structural pattern matching (PEP 634).

// Match represents a match statement.
//
//	match subject:
//	    case pattern [if guard]:
//	        body
type Match struct {
	baseStmt
	Subject Expr
	Cases   []*MatchCase
}

// MatchCase is one case clause of a match statement.
type MatchCase struct {
	baseStmt
	Pattern Pattern
	Guard   Expr // optional `if` guard, or nil
	Body    []Stmt
}

// Pattern is the interface implemented by all pattern nodes.
type Pattern interface {
	Node
	patternMarker()
}

type basePattern struct{ baseNode }

func (basePattern) patternMarker() {}

// MatchValue matches when subject == Value. Value is a literal (possibly
// negated) or a dotted name such as Color.RED.
type MatchValue struct {
	basePattern
	Value Expr
}

// MatchSingleton matches None, True or False by identity.
type MatchSingleton struct {
	basePattern
	Value interface{} // nil, true or false
}

// MatchSequence matches a list or tuple; at most one element is a *MatchStar.
type MatchSequence struct {
	basePattern
	Patterns []Pattern
}

// MatchMapping matches a mapping that contains every key in Keys and whose
// values match the corresponding sub-patterns. Rest, when non-empty, captures
// the remaining items as a dict (**rest).
type MatchMapping struct {
	basePattern
	Keys     []Expr
	Patterns []Pattern
	Rest     string
}

// MatchClass matches isinstance(subject, Cls) and then its attributes.
//
//	Point(0, y=1)   ->   Patterns=[0], KwdAttrs=["y"], KwdPatterns=[1]
type MatchClass struct {
	basePattern
	Cls         Expr
	Patterns    []Pattern
	KwdAttrs    []string
	KwdPatterns []Pattern
}

// MatchStar is a *name element inside a sequence pattern. An empty Name is
// the wildcard *_.
type MatchStar struct {
	basePattern
	Name string
}

// MatchAs is a capture or wildcard pattern, or `pattern as name`.
//
//	_            ->  Pattern=nil, Name=""
//	x            ->  Pattern=nil, Name="x"
//	[a, b] as p  ->  Pattern=[a, b], Name="p"
type MatchAs struct {
	basePattern
	Pattern Pattern
	Name    string
}

// MatchOr matches if any alternative matches (p1 | p2 | ...).
type MatchOr struct {
	basePattern
	Patterns []Pattern
}
