package pygo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"time"

	"github.com/alvinjaison/pygo/internal/interop"
	"github.com/alvinjaison/pygo/internal/object"
	"github.com/alvinjaison/pygo/internal/runtime"
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
	argv            []string
}

func defaultConfig() *runtimeConfig {
	return &runtimeConfig{
		stdlib:       true,
		maxCallDepth: 500,
	}
}

// internalRuntime is the internal implementation of the runtime.
// It connects the public API to the internal packages.
type internalRuntime struct {
	cfg *runtimeConfig
	rt  *runtime.Runtime
}

func newInternalRuntime(cfg *runtimeConfig) *internalRuntime {
	// Build internal runtime options from our config.
	var opts []runtime.Option
	if cfg.maxCallDepth > 0 {
		opts = append(opts, runtime.WithMaxCallDepth(cfg.maxCallDepth))
	}
	if len(cfg.searchPath) > 0 {
		opts = append(opts, runtime.WithSearchPath(cfg.searchPath))
	}
	if len(cfg.argv) > 0 {
		opts = append(opts, runtime.WithArgv(cfg.argv))
	}

	rt := runtime.New(opts...)
	return &internalRuntime{cfg: cfg, rt: rt}
}

func (r *internalRuntime) runString(ctx context.Context, source, filename string) error {
	_ = ctx // TODO: wire context for cancellation support
	err := r.rt.ExecString(source, filename)
	if err != nil {
		return wrapError(err)
	}
	return nil
}

func (r *internalRuntime) runFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("pygo: cannot read file %q: %w", path, err)
	}
	return r.runString(ctx, string(data), path)
}

func (r *internalRuntime) eval(ctx context.Context, expr string) (Value, error) {
	// Wrap the expression in a special assignment so we can retrieve the result.
	// We use a unique internal variable name unlikely to conflict.
	varName := "__pygo_eval_result__"
	source := fmt.Sprintf("%s = %s", varName, expr)

	err := r.runString(ctx, source, "<eval>")
	if err != nil {
		return nil, err
	}

	// Retrieve the result from globals.
	result, getErr := r.rt.Globals().GetItem(object.StrFrom(varName))
	if getErr != nil {
		return nil, fmt.Errorf("pygo: eval failed to retrieve result: %w", getErr)
	}

	// Clean up the temp variable.
	_ = r.rt.Globals().DelItem(object.StrFrom(varName))

	return &pyValue{obj: result}, nil
}

func (r *internalRuntime) set(name string, value interface{}) error {
	pyObj, err := interop.GoToPython(value)
	if err != nil {
		return fmt.Errorf("pygo: cannot convert value for %q: %w", name, err)
	}
	return r.rt.Globals().SetItem(object.StrFrom(name), pyObj)
}

func (r *internalRuntime) get(name string) (Value, error) {
	obj, err := r.rt.Globals().GetItem(object.StrFrom(name))
	if err != nil {
		return nil, &Error{
			Type:    "NameError",
			Message: fmt.Sprintf("name '%s' is not defined", name),
		}
	}
	return &pyValue{obj: obj}, nil
}

func (r *internalRuntime) delete(name string) {
	_ = r.rt.Globals().DelItem(object.StrFrom(name))
}

func (r *internalRuntime) registerFunction(name string, fn interface{}) error {
	wrapped, err := interop.WrapGoFunction(name, fn)
	if err != nil {
		return fmt.Errorf("pygo: RegisterFunction(%q): %w", name, err)
	}
	return r.rt.Globals().SetItem(object.StrFrom(name), wrapped)
}

func (r *internalRuntime) register(name string, value interface{}) error {
	// For now, register converts Go values to Python objects via GoToPython.
	pyObj, err := interop.GoToPython(value)
	if err != nil {
		return fmt.Errorf("pygo: Register(%q): %w", name, err)
	}
	return r.rt.Globals().SetItem(object.StrFrom(name), pyObj)
}

func (r *internalRuntime) registerModule(name string, attrs map[string]interface{}) error {
	// TODO(Phase 5): Create module object, populate, register with importer.
	_ = name
	_ = attrs
	return fmt.Errorf("pygo: RegisterModule not yet implemented")
}

func (r *internalRuntime) call(ctx context.Context, callable Value, args []interface{}) (Value, error) {
	// TODO(Phase 4): Convert args, invoke callable, return result.
	_ = ctx
	_ = callable
	_ = args
	return nil, fmt.Errorf("pygo: Call not yet implemented")
}

func (r *internalRuntime) callByName(ctx context.Context, name string, args []interface{}) (Value, error) {
	// TODO(Phase 4): Look up name, then call.
	_ = ctx
	_ = name
	_ = args
	return nil, fmt.Errorf("pygo: CallByName not yet implemented")
}

func (r *internalRuntime) callMethod(ctx context.Context, obj Value, method string, args []interface{}) (Value, error) {
	// TODO(Phase 4): GetAttr(method) on obj, then call.
	_ = ctx
	_ = obj
	_ = method
	_ = args
	return nil, fmt.Errorf("pygo: CallMethod not yet implemented")
}

func (r *internalRuntime) disassemble(source string, w io.Writer) error {
	// TODO(Phase 1): Compile source, write disassembly via bytecode.Disassemble.
	_ = source
	_ = w
	return fmt.Errorf("pygo: Disassemble not yet implemented")
}

func (r *internalRuntime) dumpAST(source string, w io.Writer) error {
	// TODO(Phase 1): Parse source, write AST dump via ast.Sprint.
	_ = source
	_ = w
	return fmt.Errorf("pygo: DumpAST not yet implemented")
}

// ============================================================
// Error wrapping: internal errors → public *pygo.Error
// ============================================================

// wrapError converts an internal runtime error to a public *Error.
func wrapError(err error) error {
	if err == nil {
		return nil
	}

	// Check if the error wraps an ExceptionObject.
	var excObj *object.ExceptionObject
	if errors.As(err, &excObj) {
		return exceptionToError(excObj)
	}

	// Check for runtime.RuntimeError which wraps ExceptionObject.
	var rtErr *runtime.RuntimeError
	if errors.As(err, &rtErr) {
		// Try to unwrap to ExceptionObject.
		var innerExc *object.ExceptionObject
		if errors.As(rtErr.Unwrap(), &innerExc) {
			return exceptionToError(innerExc)
		}
		// Generic runtime error.
		return &Error{
			Type:    "RuntimeError",
			Message: rtErr.Err.Error(),
		}
	}

	// Check for syntax errors.
	var syntaxErr *runtime.SyntaxError
	if errors.As(err, &syntaxErr) {
		return &Error{
			Type:    "SyntaxError",
			Message: syntaxErr.Err.Error(),
		}
	}

	// Check for compile errors.
	var compileErr *runtime.CompileError
	if errors.As(err, &compileErr) {
		return &Error{
			Type:    "SyntaxError",
			Message: compileErr.Err.Error(),
		}
	}

	// Fallback: wrap as generic RuntimeError.
	return &Error{
		Type:    "RuntimeError",
		Message: err.Error(),
	}
}

// exceptionToError converts an ExceptionObject to a public *Error.
func exceptionToError(exc *object.ExceptionObject) *Error {
	pyErr := &Error{
		Type:    exc.ExcType.Name(),
		Message: exc.Message,
	}
	if exc.Traceback != nil {
		for _, f := range exc.Traceback.Frames {
			pyErr.Traceback = append(pyErr.Traceback, TracebackEntry{
				Filename: f.Filename,
				FuncName: f.FuncName,
				Line:     f.Line,
			})
		}
	}
	return pyErr
}

// ============================================================
// pyValue: implementation of the public Value interface
// ============================================================

// pyValue wraps an internal object.Object as a public Value.
type pyValue struct {
	obj object.Object
}

func (v *pyValue) Type() string {
	return v.obj.Type().Name()
}

func (v *pyValue) String() string {
	return v.obj.String()
}

func (v *pyValue) Repr() string {
	return v.obj.Repr()
}

func (v *pyValue) IsNone() bool {
	return object.IsNone(v.obj)
}

func (v *pyValue) Int() (int64, error) {
	switch o := v.obj.(type) {
	case *object.IntObject:
		if o.Value.IsInt64() {
			return o.Value.Int64(), nil
		}
		return 0, fmt.Errorf("pygo: integer too large for int64")
	case *object.BoolObject:
		if o.Value {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("pygo: cannot convert %s to int", v.obj.Type().Name())
	}
}

func (v *pyValue) Float() (float64, error) {
	switch o := v.obj.(type) {
	case *object.FloatObject:
		return o.Value, nil
	case *object.IntObject:
		if o.Value.IsInt64() {
			return float64(o.Value.Int64()), nil
		}
		bf := new(big.Float).SetInt(o.Value)
		f, _ := bf.Float64()
		return f, nil
	case *object.BoolObject:
		if o.Value {
			return 1.0, nil
		}
		return 0.0, nil
	default:
		return 0, fmt.Errorf("pygo: cannot convert %s to float", v.obj.Type().Name())
	}
}

func (v *pyValue) Bool() bool {
	result, _ := object.IsTruthy(v.obj)
	return result
}

func (v *pyValue) Str() (string, error) {
	switch o := v.obj.(type) {
	case *object.StrObject:
		return o.Value, nil
	default:
		return "", fmt.Errorf("pygo: cannot convert %s to string", v.obj.Type().Name())
	}
}

func (v *pyValue) Bytes() ([]byte, error) {
	// Bytes type not yet implemented in the object model.
	return nil, fmt.Errorf("pygo: cannot convert %s to bytes", v.obj.Type().Name())
}

func (v *pyValue) List() ([]Value, error) {
	switch o := v.obj.(type) {
	case *object.ListObject:
		result := make([]Value, len(o.Items))
		for i, item := range o.Items {
			result[i] = &pyValue{obj: item}
		}
		return result, nil
	case *object.TupleObject:
		result := make([]Value, len(o.Items))
		for i, item := range o.Items {
			result[i] = &pyValue{obj: item}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("pygo: cannot convert %s to list", v.obj.Type().Name())
	}
}

func (v *pyValue) Dict() (map[string]Value, error) {
	d, ok := v.obj.(*object.DictObject)
	if !ok {
		return nil, fmt.Errorf("pygo: cannot convert %s to dict", v.obj.Type().Name())
	}
	result := make(map[string]Value)
	keys := d.Keys()
	vals := d.Values()
	for i, k := range keys {
		s, ok := k.(*object.StrObject)
		if !ok {
			return nil, fmt.Errorf("pygo: dict key is %s, not str", k.Type().Name())
		}
		result[s.Value] = &pyValue{obj: vals[i]}
	}
	return result, nil
}

func (v *pyValue) Unwrap() interface{} {
	// For now, no GoObject wrapping — return nil.
	return nil
}
