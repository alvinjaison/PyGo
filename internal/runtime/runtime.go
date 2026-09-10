// Package runtime provides the internal execution context that owns the
// full Lexer → Parser → Compiler → VM pipeline. It bridges all internal
// packages to execute Python source end-to-end.
//
// This package is NOT the public API — that lives in pygo/. The internal
// runtime is the machinery that pygo/ delegates to.
package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alvinjaison/pygo/internal/compiler"
	"github.com/alvinjaison/pygo/internal/importer"
	"github.com/alvinjaison/pygo/internal/object"
	"github.com/alvinjaison/pygo/internal/parser"
	"github.com/alvinjaison/pygo/internal/vm"
)

// defaultMaxCallDepth is the default maximum call stack depth.
const defaultMaxCallDepth = 500

// Runtime is the internal execution context for PyGo. It owns the global
// namespace, builtins namespace, and configuration needed to execute Python
// source through the full pipeline.
//
// A Runtime is NOT safe for concurrent use. Create one per goroutine, or
// protect access with external synchronization.
type Runtime struct {
	// globals is the module-level global namespace.
	globals *object.DictObject

	// builtins is the built-in namespace (__builtins__).
	builtins *object.DictObject

	// output is the writer used by print and other output builtins.
	output io.Writer

	// maxCallDepth is the maximum call stack depth for the VM.
	maxCallDepth int

	// importer is the module import system.
	importer *importer.Importer

	// searchPaths is the list of directories to search for .py modules.
	searchPaths []string

	// argv is the command-line arguments for sys.argv.
	argv []string
}

// Option configures a Runtime at creation time.
type Option func(*Runtime)

// WithOutput sets the writer used by print and other output operations.
// Defaults to os.Stdout if not specified.
func WithOutput(w io.Writer) Option {
	return func(r *Runtime) {
		r.output = w
	}
}

// WithMaxCallDepth sets the maximum call stack depth.
// Defaults to 500 if not specified or if n <= 0.
func WithMaxCallDepth(n int) Option {
	return func(r *Runtime) {
		if n > 0 {
			r.maxCallDepth = n
		}
	}
}

// WithSearchPath sets the module search paths.
// Defaults to the current directory if not specified.
func WithSearchPath(paths []string) Option {
	return func(r *Runtime) {
		r.searchPaths = paths
	}
}

// WithArgv sets the command-line arguments for sys.argv.
// Defaults to empty if not specified.
func WithArgv(argv []string) Option {
	return func(r *Runtime) {
		r.argv = argv
	}
}

// New creates a new Runtime with the given options.
// The Runtime is ready to execute Python source immediately after creation.
func New(opts ...Option) *Runtime {
	r := &Runtime{
		globals:      object.NewDict(),
		builtins:     object.NewDict(),
		output:       os.Stdout,
		maxCallDepth: defaultMaxCallDepth,
		searchPaths:  []string{"."},
	}

	for _, opt := range opts {
		opt(r)
	}

	// Register built-in functions.
	r.registerBuiltins()

	// Set __name__ to "__main__" for top-level execution.
	_ = r.globals.SetItem(object.StrFrom("__name__"), object.StrFrom("__main__"))

	// Initialize the import system.
	r.initImporter()

	return r
}

// ExecString executes Python source code through the full pipeline:
// source → Parser → Compiler → VM.
//
// Returns nil on success. Returns a wrapped error on parse, compile, or
// runtime failure.
func (r *Runtime) ExecString(source, filename string) error {
	// Phase 1: Parse
	module, err := parser.Parse(source, filename)
	if err != nil {
		return &SyntaxError{Filename: filename, Err: err}
	}

	// Phase 2: Compile
	code, err := compiler.Compile(module)
	if err != nil {
		return &CompileError{Filename: filename, Err: err}
	}

	// Phase 3: Execute
	frame := vm.NewFrame(code, r.globals, r.builtins, nil)
	ctx := context.Background()
	machine := vm.New(ctx, r.maxCallDepth)
	_, execErr := machine.Run(frame)
	if execErr != nil {
		return &RuntimeError{Err: execErr}
	}

	return nil
}

// Globals returns the runtime's global namespace dict.
func (r *Runtime) Globals() *object.DictObject {
	return r.globals
}

// Builtins returns the runtime's builtins namespace dict.
func (r *Runtime) Builtins() *object.DictObject {
	return r.builtins
}

// registerBuiltins initializes the builtins namespace with core built-in
// functions available to all Python code.
func (r *Runtime) registerBuiltins() {
	// print builtin
	printFn := object.NewBuiltin("print", r.builtinPrint)
	_ = r.builtins.SetItem(object.StrFrom("print"), printFn)

	// type builtin
	typeFn := object.NewBuiltin("type", r.builtinType)
	_ = r.builtins.SetItem(object.StrFrom("type"), typeFn)

	// len builtin
	lenFn := object.NewBuiltin("len", r.builtinLen)
	_ = r.builtins.SetItem(object.StrFrom("len"), lenFn)

	// range builtin
	rangeFn := object.NewBuiltin("range", r.builtinRange)
	_ = r.builtins.SetItem(object.StrFrom("range"), rangeFn)

	// Type conversion builtins
	intFn := object.NewBuiltin("int", r.builtinInt)
	_ = r.builtins.SetItem(object.StrFrom("int"), intFn)

	floatFn := object.NewBuiltin("float", r.builtinFloat)
	_ = r.builtins.SetItem(object.StrFrom("float"), floatFn)

	strFn := object.NewBuiltin("str", r.builtinStr)
	_ = r.builtins.SetItem(object.StrFrom("str"), strFn)

	boolFn := object.NewBuiltin("bool", r.builtinBool)
	_ = r.builtins.SetItem(object.StrFrom("bool"), boolFn)

	// isinstance builtin
	isinstanceFn := object.NewBuiltin("isinstance", r.builtinIsinstance)
	_ = r.builtins.SetItem(object.StrFrom("isinstance"), isinstanceFn)

	// repr builtin
	reprFn := object.NewBuiltin("repr", r.builtinRepr)
	_ = r.builtins.SetItem(object.StrFrom("repr"), reprFn)

	// abs builtin
	absFn := object.NewBuiltin("abs", r.builtinAbs)
	_ = r.builtins.SetItem(object.StrFrom("abs"), absFn)

	// min/max builtins
	minFn := object.NewBuiltin("min", r.builtinMin)
	_ = r.builtins.SetItem(object.StrFrom("min"), minFn)

	maxFn := object.NewBuiltin("max", r.builtinMax)
	_ = r.builtins.SetItem(object.StrFrom("max"), maxFn)

	// Register constants
	_ = r.builtins.SetItem(object.StrFrom("None"), object.None)
	_ = r.builtins.SetItem(object.StrFrom("True"), object.PyTrue)
	_ = r.builtins.SetItem(object.StrFrom("False"), object.PyFalse)

	// Register type objects for isinstance() usage.
	// These are registered under names that won't conflict with the builtin
	// functions above (list, tuple, dict, object don't have conversion builtins).
	_ = r.builtins.SetItem(object.StrFrom("list"), object.ListType)
	_ = r.builtins.SetItem(object.StrFrom("tuple"), object.TupleType)
	_ = r.builtins.SetItem(object.StrFrom("dict"), object.DictType)
	_ = r.builtins.SetItem(object.StrFrom("object"), object.ObjectType)

	// Register exception type objects so except clauses can reference them by name.
	r.registerExceptionTypes()
}

// registerExceptionTypes adds all built-in exception type objects to the
// builtins namespace so that they're accessible by name in except clauses.
func (r *Runtime) registerExceptionTypes() {
	excTypes := map[string]*object.TypeObject{
		"BaseException":         object.BaseExceptionType,
		"Exception":             object.ExceptionType,
		"TypeError":             object.TypeTypeError,
		"ValueError":            object.TypeValueError,
		"NameError":             object.TypeNameError,
		"AttributeError":        object.TypeAttributeError,
		"IndexError":            object.TypeIndexError,
		"KeyError":              object.TypeKeyError,
		"ZeroDivisionError":     object.TypeZeroDivisionError,
		"RuntimeError":          object.TypeRuntimeError,
		"RecursionError":        object.TypeRecursionError,
		"NotImplementedError":   object.TypeNotImplementedError,
		"OverflowError":         object.TypeOverflowError,
		"SyntaxError":           object.TypeSyntaxError,
		"ImportError":           object.TypeImportError,
		"ModuleNotFoundError":   object.TypeModuleNotFoundError,
		"OSError":               object.TypeOSError,
		"StopIteration":         object.TypeStopIteration,
		"SystemExit":            object.TypeSystemExit,
		"KeyboardInterrupt":     object.TypeKeyboardInterrupt,
		"GeneratorExit":         object.TypeGeneratorExit,
		"AssertionError":        object.TypeAssertionError,
		"UnboundLocalError":     object.TypeUnboundLocalError,
	}
	for name, typ := range excTypes {
		_ = r.builtins.SetItem(object.StrFrom(name), typ)
	}
}

// builtinType implements Python's type(obj) function (single-argument form).
//
// Returns the type object of the given argument. The single-argument form
// is the only form supported in MVP (the three-argument class-creation form
// is Post-MVP).
func (r *Runtime) builtinType(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, object.NewTypeError("type() takes 1 argument (%d given)", len(args))
	}
	return args[0].Type(), nil
}

// builtinPrint implements Python's print() function.
//
// Signature: print(*args, sep=' ', end='\n')
// - Joins args with sep (default: space)
// - Appends end (default: newline)
// - Writes to the runtime's configured output writer
func (r *Runtime) builtinPrint(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	sep := " "
	end := "\n"

	// Handle keyword arguments.
	if kwargs != nil {
		if v, ok := kwargs["sep"]; ok {
			if s, ok := v.(*object.StrObject); ok {
				sep = s.Value
			} else if object.IsNone(v) {
				// sep=None means use default
			} else {
				return nil, object.NewTypeError("sep must be None or a string, not %s", v.Type().Name())
			}
		}
		if v, ok := kwargs["end"]; ok {
			if s, ok := v.(*object.StrObject); ok {
				end = s.Value
			} else if object.IsNone(v) {
				// end=None means use default
			} else {
				return nil, object.NewTypeError("end must be None or a string, not %s", v.Type().Name())
			}
		}
	}

	// Build the output string from args.
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = arg.String()
	}

	output := strings.Join(parts, sep) + end
	_, err := fmt.Fprint(r.output, output)
	if err != nil {
		return nil, object.NewRuntimeError("write error: %s", err.Error())
	}

	return object.None, nil
}

// initImporter sets up the module import system with built-in modules
// and registers the __import__ function in builtins.
func (r *Runtime) initImporter() {
	r.importer = importer.New(r.searchPaths)

	// Register the compile-and-exec callback for file-based imports.
	r.importer.SetCompileAndExec(func(source, filename string, globals *object.DictObject) error {
		module, err := parser.Parse(source, filename)
		if err != nil {
			return err
		}
		code, err := compiler.Compile(module)
		if err != nil {
			return err
		}
		frame := vm.NewFrame(code, globals, r.builtins, nil)
		ctx := context.Background()
		machine := vm.New(ctx, r.maxCallDepth)
		_, execErr := machine.Run(frame)
		return execErr
	})

	// Register the built-in sys module.
	r.importer.RegisterBuiltin("sys", func() *object.ModuleObject {
		return importer.NewSysModule(r.argv, r.searchPaths)
	})

	// Register the built-in json module.
	r.importer.RegisterBuiltin("json", func() *object.ModuleObject {
		return importer.NewJSONModule()
	})

	// Register __import__ in builtins.
	importFn := object.NewBuiltin("__import__", r.builtinImport)
	_ = r.builtins.SetItem(object.StrFrom("__import__"), importFn)
}

// builtinImport implements the __import__ builtin function.
// Signature: __import__(name) -> module
func (r *Runtime) builtinImport(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if len(args) < 1 {
		return nil, object.NewTypeError("__import__() requires at least 1 argument")
	}
	nameObj, ok := args[0].(*object.StrObject)
	if !ok {
		return nil, object.NewTypeError("__import__() argument must be a string, not '%s'", args[0].Type().Name())
	}
	mod, err := r.importer.Import(nameObj.Value)
	if err != nil {
		return nil, err
	}
	return mod, nil
}

// Importer returns the runtime's importer (for testing and advanced use).
func (r *Runtime) Importer() *importer.Importer {
	return r.importer
}
