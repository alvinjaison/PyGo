// Package pygo is the public API for the PyGo Python 3 runtime.
//
// PyGo executes Python source code in a pure Go process without requiring
// a CPython installation. It is designed to be embedded in Go applications
// that need to expose their APIs to Python scripts.
//
// # Quick Start
//
//	rt := pygo.New()
//	if err := rt.RunString(`print("Hello from Python!")`); err != nil {
//	    log.Fatal(err)
//	}
//
// # Exposing Go Functions
//
//	rt.MustRegisterFunction("add", func(a, b int) int { return a + b })
//	rt.RunString(`print(add(3, 4))`)  // prints 7
//
// # Exposing Go Structs
//
//	type Config struct { Debug bool; Port int }
//	rt.MustRegister("cfg", &Config{Debug: true, Port: 8080})
//	rt.RunString(`print(cfg.Port)`)  // prints 8080
//
// # Error Handling
//
//	err := rt.RunString(`raise ValueError("oops")`)
//	if pygo.IsError(err, "ValueError") {
//	    fmt.Println("caught ValueError:", err)
//	}
//
// See the docs/ directory for the full architecture, API, and compatibility
// documentation.
package pygo

import (
	"context"
	"io"
	"time"
)

// Runtime is a single Python execution context.
//
// Each Runtime has its own globals, module registry, and object state.
// Multiple Runtime instances can coexist safely in the same process —
// they share no mutable state.
//
// A Runtime is NOT safe for concurrent use by multiple goroutines.
// If concurrent Python execution is needed, create separate Runtime instances.
type Runtime struct {
	cfg *runtimeConfig
	rt  *internalRuntime
}

// New creates and initializes a new Runtime with the given options.
//
// New initializes the built-in namespace, standard type objects, and any
// enabled standard library modules. It is safe to call concurrently.
func New(opts ...Option) *Runtime {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	r := &Runtime{cfg: cfg}
	r.rt = newInternalRuntime(cfg)
	return r
}

// ============================================================
// Execution
// ============================================================

// RunString compiles and executes a Python source string.
// The source is executed in the runtime's global namespace.
// Returns a non-nil error if compilation fails or a Python exception is raised.
func (r *Runtime) RunString(source string) error {
	return r.rt.runString(context.Background(), source, "<string>")
}

// RunStringContext is like RunString but respects ctx for cancellation.
// If ctx is cancelled while Python code is running, execution is interrupted
// and a context error is returned.
func (r *Runtime) RunStringContext(ctx context.Context, source string) error {
	return r.rt.runString(ctx, source, "<string>")
}

// RunFile reads and executes a Python source file.
// The file path is used in tracebacks.
func (r *Runtime) RunFile(path string) error {
	return r.rt.runFile(context.Background(), path)
}

// RunFileContext is like RunFile but respects ctx for cancellation.
func (r *Runtime) RunFileContext(ctx context.Context, path string) error {
	return r.rt.runFile(ctx, path)
}

// Eval compiles and evaluates a single Python expression, returning the result.
// The expression must not contain statements (no assignments, no def, no if blocks).
func (r *Runtime) Eval(expr string) (Value, error) {
	return r.rt.eval(context.Background(), expr)
}

// EvalContext is like Eval but respects ctx for cancellation.
func (r *Runtime) EvalContext(ctx context.Context, expr string) (Value, error) {
	return r.rt.eval(ctx, expr)
}

// ============================================================
// Variable Access
// ============================================================

// Set sets a variable in the runtime's global namespace.
// value is converted from Go to Python using standard type conversion rules.
// Returns an error if value cannot be converted.
func (r *Runtime) Set(name string, value interface{}) error {
	return r.rt.set(name, value)
}

// Get retrieves a variable from the runtime's global namespace.
// Returns a Value, or an error if the name is not defined.
func (r *Runtime) Get(name string) (Value, error) {
	return r.rt.get(name)
}

// Delete removes a variable from the runtime's global namespace.
// Does nothing if the variable does not exist.
func (r *Runtime) Delete(name string) {
	r.rt.delete(name)
}

// ============================================================
// Registration
// ============================================================

// RegisterFunction exposes a Go function to Python under the given name
// in the runtime's global namespace.
//
// fn must be a function value. Accepted parameter and return types are
// documented in docs/interop.md. Returns an error if fn is not a function
// or contains unsupported parameter/return types.
func (r *Runtime) RegisterFunction(name string, fn interface{}) error {
	return r.rt.registerFunction(name, fn)
}

// Register exposes a Go value (struct pointer, primitive, etc.) to Python
// under the given name in the runtime's global namespace.
// The value's exported fields and methods become Python attributes.
func (r *Runtime) Register(name string, value interface{}) error {
	return r.rt.register(name, value)
}

// RegisterModule creates a Python module and populates it with the provided
// attributes. The module becomes importable as `import name`.
// attrs values are converted using standard Go→Python conversion rules.
func (r *Runtime) RegisterModule(name string, attrs map[string]interface{}) error {
	return r.rt.registerModule(name, attrs)
}

// MustRegisterFunction is like RegisterFunction but panics on error.
// Useful in init() registration where an error indicates a programming mistake.
func (r *Runtime) MustRegisterFunction(name string, fn interface{}) {
	if err := r.RegisterFunction(name, fn); err != nil {
		panic("pygo: RegisterFunction(" + name + "): " + err.Error())
	}
}

// MustRegister is like Register but panics on error.
func (r *Runtime) MustRegister(name string, value interface{}) {
	if err := r.Register(name, value); err != nil {
		panic("pygo: Register(" + name + "): " + err.Error())
	}
}

// ============================================================
// Calling Python from Go
// ============================================================

// Call invokes a Python callable with positional arguments.
// args are converted from Go to Python using standard type conversion rules.
// Returns the Python result wrapped in a Value.
func (r *Runtime) Call(callable Value, args ...interface{}) (Value, error) {
	return r.rt.call(context.Background(), callable, args)
}

// CallByName looks up a name in the global namespace and calls it.
func (r *Runtime) CallByName(name string, args ...interface{}) (Value, error) {
	return r.rt.callByName(context.Background(), name, args)
}

// CallMethod looks up an attribute on obj and calls it with args.
func (r *Runtime) CallMethod(obj Value, method string, args ...interface{}) (Value, error) {
	return r.rt.callMethod(context.Background(), obj, method, args)
}

// ============================================================
// Diagnostics
// ============================================================

// Disassemble compiles source and writes the bytecode disassembly to w.
// Useful for debugging compiler output.
func (r *Runtime) Disassemble(source string, w io.Writer) error {
	return r.rt.disassemble(source, w)
}

// DumpAST parses source and writes a text representation of the AST to w.
// Useful for debugging parser output.
func (r *Runtime) DumpAST(source string, w io.Writer) error {
	return r.rt.dumpAST(source, w)
}

// ============================================================
// Options
// ============================================================

// Option is a functional option for configuring a Runtime.
type Option func(*runtimeConfig)

// WithStdlib controls whether standard library modules are pre-loaded.
// Default: true.
func WithStdlib(enabled bool) Option {
	return func(c *runtimeConfig) { c.stdlib = enabled }
}

// WithSearchPath sets the Python module search path (sys.path).
// Default: current working directory.
func WithSearchPath(paths []string) Option {
	return func(c *runtimeConfig) { c.searchPath = paths }
}

// WithMaxCallDepth sets the maximum call stack depth.
// Default: 500. Exceeding this raises RecursionError.
func WithMaxCallDepth(depth int) Option {
	return func(c *runtimeConfig) { c.maxCallDepth = depth }
}

// WithDiagnostics sets a writer for diagnostic output.
// Diagnostics include bytecode disassembly and AST dumps when requested.
// Default: nil (diagnostics disabled).
func WithDiagnostics(w io.Writer) Option {
	return func(c *runtimeConfig) { c.diagnostics = w }
}

// WithHashSeed sets a fixed seed for hash randomization.
// Only use this in tests requiring deterministic hash ordering.
// Default: random seed chosen at New() time.
func WithHashSeed(seed uint64) Option {
	return func(c *runtimeConfig) { c.hashSeed = &seed }
}

// WithRestrictedBuiltins creates a runtime with I/O-capable built-ins
// (open, input) and OS-level modules disabled.
// Appropriate for executing scripts from untrusted sources.
func WithRestrictedBuiltins() Option {
	return func(c *runtimeConfig) { c.restricted = true }
}

// WithExecutionTimeout sets a maximum execution time for all Run calls.
// After the timeout, a KeyboardInterrupt-equivalent is raised.
func WithExecutionTimeout(d time.Duration) Option {
	return func(c *runtimeConfig) { c.timeout = d }
}

// WithMaxInstructions sets a limit on bytecode instructions per Run call.
// Exceeding this raises a RuntimeError.
// Default: 0 (unlimited).
func WithMaxInstructions(n int64) Option {
	return func(c *runtimeConfig) { c.maxInstructions = n }
}
