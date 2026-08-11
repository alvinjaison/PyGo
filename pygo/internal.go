package pygo

import (
	"context"
	"io"
	"time"
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

// internalRuntime is the internal implementation of the runtime.
// It connects the public API to the internal packages.
//
// TODO(Phase 1): Replace these stubs with real implementations backed by
// internal/lexer, internal/parser, internal/compiler, internal/vm, and
// internal/object as those packages are built out.
type internalRuntime struct {
	cfg *runtimeConfig
}

func newInternalRuntime(cfg *runtimeConfig) *internalRuntime {
	return &internalRuntime{cfg: cfg}
}

func (r *internalRuntime) runString(ctx context.Context, source, filename string) error {
	// TODO(Phase 1): Implement: lex → parse → compile → vm.Run
	_ = ctx
	_ = source
	_ = filename
	return nil
}

func (r *internalRuntime) runFile(ctx context.Context, path string) error {
	// TODO(Phase 1): Read file, then runString.
	_ = ctx
	_ = path
	return nil
}

func (r *internalRuntime) eval(ctx context.Context, expr string) (Value, error) {
	// TODO(Phase 1): Compile as expression, run, return result.
	_ = ctx
	_ = expr
	return nil, nil
}

func (r *internalRuntime) set(name string, value interface{}) error {
	// TODO(Phase 1): Convert value to object.Object and store in globals.
	_ = name
	_ = value
	return nil
}

func (r *internalRuntime) get(name string) (Value, error) {
	// TODO(Phase 1): Look up name in globals, return wrapped Value.
	_ = name
	return nil, nil
}

func (r *internalRuntime) delete(name string) {
	// TODO(Phase 1): Remove from globals.
	_ = name
}

func (r *internalRuntime) registerFunction(name string, fn interface{}) error {
	// TODO(Phase 4): Wrap fn via internal/interop and store in globals.
	_ = name
	_ = fn
	return nil
}

func (r *internalRuntime) register(name string, value interface{}) error {
	// TODO(Phase 4): Wrap value via internal/interop and store in globals.
	_ = name
	_ = value
	return nil
}

func (r *internalRuntime) registerModule(name string, attrs map[string]interface{}) error {
	// TODO(Phase 5): Create module object, populate, register with importer.
	_ = name
	_ = attrs
	return nil
}

func (r *internalRuntime) call(ctx context.Context, callable Value, args []interface{}) (Value, error) {
	// TODO(Phase 4): Convert args, invoke callable, return result.
	_ = ctx
	_ = callable
	_ = args
	return nil, nil
}

func (r *internalRuntime) callByName(ctx context.Context, name string, args []interface{}) (Value, error) {
	// TODO(Phase 4): Look up name, then call.
	_ = ctx
	_ = name
	_ = args
	return nil, nil
}

func (r *internalRuntime) callMethod(ctx context.Context, obj Value, method string, args []interface{}) (Value, error) {
	// TODO(Phase 4): GetAttr(method) on obj, then call.
	_ = ctx
	_ = obj
	_ = method
	_ = args
	return nil, nil
}

func (r *internalRuntime) disassemble(source string, w io.Writer) error {
	// TODO(Phase 1): Compile source, write disassembly via bytecode.Disassemble.
	_ = source
	_ = w
	return nil
}

func (r *internalRuntime) dumpAST(source string, w io.Writer) error {
	// TODO(Phase 1): Parse source, write AST dump via ast.Sprint.
	_ = source
	_ = w
	return nil
}
