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
	"github.com/pygo/pygo/internal/importer"
	"github.com/pygo/pygo/internal/object"
	"github.com/pygo/pygo/internal/parser"
	"github.com/pygo/pygo/internal/vm"
)

// Config configures a Runtime.
type Config struct {
	Stdlib          bool
	SearchPath      []string
	MaxCallDepth    int
	MaxInstructions int64
	MaxMemory       int64
	Restricted      bool
	Stdout          io.Writer
	Stderr          io.Writer
}

// Runtime is a single Python execution context.
type Runtime struct {
	cfg      Config
	globals  *object.DictObject
	builtins *object.DictObject
	modules  map[string]*object.ModuleObject
	stdout   io.Writer
	importer *importer.Importer
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

	// Set up the module importer. In restricted mode imports are still allowed
	// for safe modules, but OS-capable modules (e.g. os) are blocked.
	r.importer = importer.New(cfg.SearchPath, r, cfg.Restricted)

	// Install the global callable-invocation hook so stdlib/interop code can
	// call Python callables (including user functions) through the VM.
	object.SetInvoker(func(callable object.Object, args []object.Object) (object.Object, error) {
		return r.Call(context.Background(), callable, args)
	})

	return r
}

// newVM constructs a VM wired with this runtime's importer and resource limits.
func (r *Runtime) newVM(ctx context.Context) *vm.VM {
	machine := vm.New(ctx, vm.Limits{
		MaxCallDepth:    r.cfg.MaxCallDepth,
		MaxInstructions: r.cfg.MaxInstructions,
		MaxMemory:       r.cfg.MaxMemory,
	})
	if r.importer != nil {
		machine.SetImporter(importerAdapter{r.importer})
	}
	return machine
}

// importerAdapter adapts *importer.Importer to the vm.Importer interface
// (widening the concrete return type to object.Object).
type importerAdapter struct{ im *importer.Importer }

func (a importerAdapter) Import(name string) (object.Object, error) {
	mod, err := a.im.Import(name)
	if err != nil {
		return nil, err
	}
	return mod, nil
}

// RegisterModule registers a pre-built module so it is both bound in globals
// and importable by name via the import system.
func (r *Runtime) RegisterModule(mod *object.ModuleObject) {
	r.Set(mod.Name, mod)
	if r.importer != nil {
		r.importer.Cache()[mod.Name] = mod
	}
}

// ExecModule compiles and runs module source in the module's namespace,
// satisfying importer.Execer.
func (r *Runtime) ExecModule(mod *object.ModuleObject, source, filename string) error {
	code, err := r.compile(source, filename)
	if err != nil {
		return err
	}
	machine := r.newVM(context.Background())
	// The module's globals are its own dict, so top-level assignments populate
	// the module namespace.
	frame := vm.NewFrame(code, mod.Dict, r.builtins, nil)
	_, err = machine.Run(frame)
	return err
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
	machine := r.newVM(ctx)
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
	machine := r.newVM(ctx)
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
	machine := r.newVM(ctx)
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
