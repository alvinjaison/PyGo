package pygo

import (
	"context"
	"io"
	"time"

	"github.com/pygo/pygo/internal/object"
	"github.com/pygo/pygo/internal/runtime"
)

// runtimeConfig holds all configuration for a Runtime.
type runtimeConfig struct {
	stdlib          bool
	searchPath      []string
	maxCallDepth    int
	diagnostics     io.Writer
	hashSeed        *uint64
	restricted      bool
	timeout         time.Duration
	maxInstructions int64
}

func defaultConfig() *runtimeConfig {
	return &runtimeConfig{
		stdlib:       true,
		maxCallDepth: 500,
	}
}

// internalRuntime connects the public API to the internal runtime.
type internalRuntime struct {
	cfg *runtimeConfig
	rt  *runtime.Runtime
}

func newInternalRuntime(cfg *runtimeConfig) *internalRuntime {
	rt := runtime.New(runtime.Config{
		Stdlib:       cfg.stdlib,
		SearchPath:   cfg.searchPath,
		MaxCallDepth: cfg.maxCallDepth,
		Restricted:   cfg.restricted,
	})
	return &internalRuntime{cfg: cfg, rt: rt}
}

// withTimeout applies the configured execution timeout to ctx.
func (r *internalRuntime) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.cfg.timeout > 0 {
		return context.WithTimeout(ctx, r.cfg.timeout)
	}
	return ctx, func() {}
}

func (r *internalRuntime) runString(ctx context.Context, source, filename string) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()
	return toPublicError(r.rt.RunString(ctx, source, filename))
}

func (r *internalRuntime) runFile(ctx context.Context, path string) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()
	return toPublicError(r.rt.RunFile(ctx, path))
}

func (r *internalRuntime) eval(ctx context.Context, expr string) (Value, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()
	result, err := r.rt.Eval(ctx, expr)
	if err != nil {
		return nil, toPublicError(err)
	}
	return newValue(result), nil
}

func (r *internalRuntime) set(name string, value interface{}) error {
	obj, err := goToPython(value)
	if err != nil {
		return err
	}
	r.rt.Set(name, obj)
	return nil
}

func (r *internalRuntime) get(name string) (Value, error) {
	v, ok := r.rt.Get(name)
	if !ok {
		return nil, &Error{Type: "NameError", Message: "name '" + name + "' is not defined"}
	}
	return newValue(v), nil
}

func (r *internalRuntime) delete(name string) {
	r.rt.Delete(name)
}

func (r *internalRuntime) registerFunction(name string, fn interface{}) error {
	wrapped, err := wrapGoFunction(name, fn)
	if err != nil {
		return err
	}
	r.rt.Set(name, wrapped)
	return nil
}

func (r *internalRuntime) register(name string, value interface{}) error {
	obj, err := goToPython(value)
	if err != nil {
		return err
	}
	r.rt.Set(name, obj)
	return nil
}

func (r *internalRuntime) registerModule(name string, attrs map[string]interface{}) error {
	mod := object.NewModule(name)
	for k, v := range attrs {
		var obj object.Object
		var err error
		// Functions are wrapped as callables; everything else converted directly.
		obj, err = wrapGoFunction(k, v)
		if err != nil {
			obj, err = goToPython(v)
			if err != nil {
				return err
			}
		}
		if err := mod.SetAttr(k, obj); err != nil {
			return err
		}
	}
	r.rt.Set(name, mod)
	return nil
}

func (r *internalRuntime) call(ctx context.Context, callable Value, args []interface{}) (Value, error) {
	v, ok := callable.(*valueImpl)
	if !ok {
		return nil, &Error{Type: "TypeError", Message: "not a callable value"}
	}
	return r.callObject(ctx, v.obj, args)
}

func (r *internalRuntime) callByName(ctx context.Context, name string, args []interface{}) (Value, error) {
	obj, ok := r.rt.Get(name)
	if !ok {
		return nil, &Error{Type: "NameError", Message: "name '" + name + "' is not defined"}
	}
	return r.callObject(ctx, obj, args)
}

func (r *internalRuntime) callMethod(ctx context.Context, obj Value, method string, args []interface{}) (Value, error) {
	v, ok := obj.(*valueImpl)
	if !ok {
		return nil, &Error{Type: "TypeError", Message: "not a value"}
	}
	holder, ok := v.obj.(object.AttributeHolder)
	if !ok {
		return nil, &Error{Type: "AttributeError", Message: "object has no attributes"}
	}
	m, err := holder.GetAttr(method)
	if err != nil {
		return nil, toPublicError(err)
	}
	return r.callObject(ctx, m, args)
}

func (r *internalRuntime) callObject(ctx context.Context, callable object.Object, args []interface{}) (Value, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()
	objArgs := make([]object.Object, len(args))
	for i, a := range args {
		obj, err := goToPython(a)
		if err != nil {
			return nil, err
		}
		objArgs[i] = obj
	}
	result, err := r.rt.Call(ctx, callable, objArgs)
	if err != nil {
		return nil, toPublicError(err)
	}
	return newValue(result), nil
}

func (r *internalRuntime) disassemble(source string, w io.Writer) error {
	return r.rt.Disassemble(source, w)
}

func (r *internalRuntime) dumpAST(source string, w io.Writer) error {
	return r.rt.DumpAST(source, w)
}
