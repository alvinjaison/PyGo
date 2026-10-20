package object

// GeneratorObject represents a Python generator — a function containing
// `yield`. Calling a generator function does not run its body; it returns a
// GeneratorObject whose frame is advanced on each next() call.
//
// Frame suspension/resumption is implemented by the VM, which supplies a
// resume closure. The object package stays free of any VM dependency.
type GeneratorObject struct {
	Name string

	// resume advances the generator. It returns (value, more, err):
	//   more == true  → value is the next yielded value
	//   more == false → the generator is exhausted (StopIteration)
	resume func() (Object, bool, error)

	finished bool
}

// NewGenerator creates a generator backed by the given resume closure.
func NewGenerator(name string, resume func() (Object, bool, error)) *GeneratorObject {
	return &GeneratorObject{Name: name, resume: resume}
}

func (g *GeneratorObject) Type() *TypeObject { return GeneratorType }
func (g *GeneratorObject) String() string    { return "<generator object " + g.Name + ">" }
func (g *GeneratorObject) Repr() string       { return g.String() }

// Iter returns the generator itself (generators are their own iterator).
func (g *GeneratorObject) Iter() (Iterator, error) { return g, nil }

// Next advances the generator, returning the next value or ErrStopIteration.
func (g *GeneratorObject) Next() (Object, error) {
	if g.finished {
		return nil, ErrStopIteration
	}
	val, more, err := g.resume()
	if err != nil {
		g.finished = true
		return nil, err
	}
	if !more {
		g.finished = true
		return nil, ErrStopIteration
	}
	return val, nil
}
