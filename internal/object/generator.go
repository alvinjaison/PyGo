package object

// GenResume describes how a suspended generator should be resumed.
type GenResume struct {
	// Send is the value the pending `yield` expression evaluates to. It is nil
	// for a plain next() (the yield expression then evaluates to None).
	Send Object
	// Throw, when non-nil, is raised inside the generator at the point where
	// it is suspended.
	Throw *ExceptionObject
}

// GeneratorObject represents a Python generator — a function containing
// `yield`. Calling a generator function does not run its body; it returns a
// GeneratorObject whose frame is advanced on each next()/send()/throw() call.
//
// Frame suspension/resumption is implemented by the VM, which supplies a
// resume closure. The object package stays free of any VM dependency.
type GeneratorObject struct {
	Name string

	// resume advances the generator. It returns (value, more, err):
	//   more == true  → value is the next yielded value
	//   more == false → the generator is exhausted (StopIteration)
	resume func(GenResume) (Object, bool, error)

	started  bool
	finished bool
}

// NewGenerator creates a generator backed by the given resume closure.
func NewGenerator(name string, resume func(GenResume) (Object, bool, error)) *GeneratorObject {
	return &GeneratorObject{Name: name, resume: resume}
}

func (g *GeneratorObject) Type() *TypeObject { return GeneratorType }
func (g *GeneratorObject) String() string    { return "<generator object " + g.Name + ">" }
func (g *GeneratorObject) Repr() string      { return g.String() }

// Iter returns the generator itself (generators are their own iterator).
func (g *GeneratorObject) Iter() (Iterator, error) { return g, nil }

// step resumes the generator once and updates its lifecycle state.
func (g *GeneratorObject) step(r GenResume) (Object, error) {
	if g.finished {
		return nil, ErrStopIteration
	}
	g.started = true
	val, more, err := g.resume(r)
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

// Next advances the generator, returning the next value or ErrStopIteration.
func (g *GeneratorObject) Next() (Object, error) {
	return g.step(GenResume{})
}

// Send resumes the generator, making the pending yield evaluate to v.
// Sending a non-None value to a just-started generator is a TypeError.
func (g *GeneratorObject) Send(v Object) (Object, error) {
	if !g.started && !IsNone(v) {
		return nil, NewTypeError("can't send non-None value to a just-started generator")
	}
	if IsNone(v) {
		v = nil
	}
	return g.step(GenResume{Send: v})
}

// Throw raises exc inside the generator at its current suspension point.
func (g *GeneratorObject) Throw(exc *ExceptionObject) (Object, error) {
	if g.finished || !g.started {
		// Nothing is suspended to receive it: the exception just propagates and
		// the generator is finished.
		g.finished = true
		return nil, exc
	}
	return g.step(GenResume{Throw: exc})
}

// Close stops the generator by raising GeneratorExit inside it. It is an error
// for the generator to yield a value in response.
func (g *GeneratorObject) Close() error {
	if g.finished || !g.started {
		g.finished = true
		return nil
	}
	_, err := g.step(GenResume{Throw: NewException(TypeGeneratorExit, "")})
	switch {
	case err == nil:
		return NewRuntimeError("generator ignored GeneratorExit")
	case IsStopIteration(err):
		return nil
	}
	if exc, ok := err.(*ExceptionObject); ok && exc.Type().IsSubtype(TypeGeneratorExit) {
		return nil
	}
	return err
}

// stopIterationError converts the internal exhaustion sentinel into a real
// Python StopIteration exception, for methods called directly from Python.
func stopIterationError(err error) error {
	if IsStopIteration(err) {
		return NewException(TypeStopIteration, "")
	}
	return err
}

// GetAttr exposes the generator methods: send, throw, close and __next__.
func (g *GeneratorObject) GetAttr(name string) (Object, error) {
	switch name {
	case "send":
		return NewBuiltin("generator.send", func(args []Object, _ map[string]Object) (Object, error) {
			if len(args) != 1 {
				return nil, NewTypeError("send() takes exactly one argument (%d given)", len(args))
			}
			v, err := g.Send(args[0])
			return v, stopIterationError(err)
		}), nil
	case "throw":
		return NewBuiltin("generator.throw", func(args []Object, _ map[string]Object) (Object, error) {
			if len(args) < 1 {
				return nil, NewTypeError("throw() expects an exception")
			}
			var exc *ExceptionObject
			switch e := args[0].(type) {
			case *ExceptionObject:
				exc = e
			case *TypeObject:
				if len(args) > 1 {
					exc = NewException(e, args[1].String())
				} else {
					exc = NewException(e, "")
				}
			default:
				return nil, NewTypeError("exceptions must be classes or instances deriving from BaseException")
			}
			v, err := g.Throw(exc)
			return v, stopIterationError(err)
		}), nil
	case "close":
		return NewBuiltin("generator.close", func(_ []Object, _ map[string]Object) (Object, error) {
			if err := g.Close(); err != nil {
				return nil, err
			}
			return None, nil
		}), nil
	case "__next__":
		return NewBuiltin("generator.__next__", func(_ []Object, _ map[string]Object) (Object, error) {
			v, err := g.Next()
			return v, stopIterationError(err)
		}), nil
	}
	return nil, NewAttributeError("generator", name)
}

func (g *GeneratorObject) SetAttr(string, Object) error {
	return NewTypeError("'generator' object attributes are read-only")
}
func (g *GeneratorObject) DelAttr(string) error {
	return NewTypeError("'generator' object attributes are read-only")
}
