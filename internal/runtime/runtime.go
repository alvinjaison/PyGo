// Package runtime is the central coordinator that ties the lexer, parser,
// compiler, and VM together for a single execution context.
//
// It owns the global namespace, the builtin namespace, and the module
// registry for one embedded runtime instance.
package runtime

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/pygo/pygo/internal/bytecode"
	"github.com/pygo/pygo/internal/compiler"
	"github.com/pygo/pygo/internal/object"
	"github.com/pygo/pygo/internal/parser"
	"github.com/pygo/pygo/internal/vm"
)

// Config configures a Runtime.
type Config struct {
	Stdlib       bool
	SearchPath   []string
	MaxCallDepth int
	Restricted   bool
	Stdout       io.Writer
	Stderr       io.Writer
}

// Runtime is a single Python execution context.
type Runtime struct {
	cfg      Config
	globals  *object.DictObject
	builtins *object.DictObject
	modules  map[string]*object.ModuleObject
	stdout   io.Writer
}

// New creates and initializes a Runtime.
func New(cfg Config) *Runtime {
	if cfg.MaxCallDepth <= 0 {
		cfg.MaxCallDepth = 500
	}
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	r := &Runtime{
		cfg:      cfg,
		globals:  object.NewDict(),
		builtins: object.NewDict(),
		modules:  map[string]*object.ModuleObject{},
		stdout:   cfg.Stdout,
	}
	r.installBuiltins()
	// Expose the builtins dict under __builtins__ for completeness.
	_ = r.globals.SetItem(object.StrFrom("__builtins__"), r.builtins)
	_ = r.globals.SetItem(object.StrFrom("__name__"), object.StrFrom("__main__"))
	return r
}

// Globals returns the runtime's global namespace.
func (r *Runtime) Globals() *object.DictObject { return r.globals }

// Builtins returns the runtime's builtin namespace.
func (r *Runtime) Builtins() *object.DictObject { return r.builtins }

// RunString compiles and executes Python source in the global namespace.
func (r *Runtime) RunString(ctx context.Context, source, filename string) error {
	code, err := r.compile(source, filename)
	if err != nil {
		return err
	}
	machine := vm.New(ctx, r.cfg.MaxCallDepth)
	frame := vm.NewFrame(code, r.globals, r.builtins, nil)
	_, err = machine.Run(frame)
	return err
}

// RunFile reads and executes a Python source file.
func (r *Runtime) RunFile(ctx context.Context, path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return r.RunString(ctx, string(src), path)
}

// Eval compiles and evaluates a single expression, returning its value.
func (r *Runtime) Eval(ctx context.Context, exprSrc string) (object.Object, error) {
	p := parser.New("<eval>", exprSrc)
	expr, err := p.ParseExpression()
	if err != nil {
		return nil, err
	}
	code, cerr := compiler.CompileExpression(expr, "<eval>")
	if cerr != nil {
		return nil, cerr
	}
	machine := vm.New(ctx, r.cfg.MaxCallDepth)
	frame := vm.NewFrame(code, r.globals, r.builtins, nil)
	return machine.Run(frame)
}

// Set binds a Python object in the global namespace.
func (r *Runtime) Set(name string, value object.Object) {
	_ = r.globals.SetItem(object.StrFrom(name), value)
}

// Get retrieves a Python object from the global namespace.
func (r *Runtime) Get(name string) (object.Object, bool) {
	v, err := r.globals.GetItem(object.StrFrom(name))
	if err != nil {
		return nil, false
	}
	return v, true
}

// Delete removes a name from the global namespace.
func (r *Runtime) Delete(name string) {
	_ = r.globals.DelItem(object.StrFrom(name))
}

// Call invokes a callable Python object from Go.
func (r *Runtime) Call(ctx context.Context, callable object.Object, args []object.Object) (object.Object, error) {
	machine := vm.New(ctx, r.cfg.MaxCallDepth)
	return machine.CallExternal(callable, args, r.globals, r.builtins)
}

// compile runs the lexer, parser, and compiler, returning a Code object.
func (r *Runtime) compile(source, filename string) (*bytecode.Code, error) {
	p := parser.New(filename, source)
	mod, err := p.Parse()
	if err != nil {
		return nil, err
	}
	code, cerr := compiler.Compile(mod, filename)
	if cerr != nil {
		return nil, cerr
	}
	return code, nil
}

var _ = fmt.Sprintf
