// Package importer implements the PyGo module import system.
//
// Module resolution follows this order:
//  1. Module cache — return previously imported module
//  2. Built-in Go modules — modules implemented in Go (e.g., sys)
//  3. File search path — .py files on the configured search paths
//
// The importer is owned by a single Runtime instance and is not safe for
// concurrent use.
package importer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/alvinjaison/pygo/internal/object"
)

// CompileAndExecFunc is the callback type used by the importer to compile and
// execute a Python source file. This avoids a circular dependency between the
// importer and the runtime/compiler/vm packages.
type CompileAndExecFunc func(source, filename string, globals *object.DictObject) error

// Importer manages module resolution and caching for a single Runtime.
type Importer struct {
	// cache stores already-imported modules keyed by their canonical name.
	cache map[string]*object.ModuleObject

	// builtins maps module names to factory functions that create the module.
	builtins map[string]func() *object.ModuleObject

	// searchPaths is the list of directories to search for .py files.
	searchPaths []string

	// compileAndExec is the callback to compile and execute a .py file.
	compileAndExec CompileAndExecFunc
}

// New creates a new Importer with the given search paths.
func New(searchPaths []string) *Importer {
	imp := &Importer{
		cache:       make(map[string]*object.ModuleObject),
		builtins:    make(map[string]func() *object.ModuleObject),
		searchPaths: searchPaths,
	}
	return imp
}

// SetCompileAndExec sets the callback used to compile and execute .py modules.
func (imp *Importer) SetCompileAndExec(fn CompileAndExecFunc) {
	imp.compileAndExec = fn
}

// RegisterBuiltin registers a built-in module factory.
// The factory is called once on first import; the result is then cached.
func (imp *Importer) RegisterBuiltin(name string, factory func() *object.ModuleObject) {
	imp.builtins[name] = factory
}

// Import resolves a module by name using the resolution order:
// cache → built-in → file search path.
//
// Returns the module object on success, or a Python exception on failure.
func (imp *Importer) Import(name string) (*object.ModuleObject, error) {
	// 1. Check the cache.
	if mod, ok := imp.cache[name]; ok {
		return mod, nil
	}

	// 2. Check built-in modules.
	if factory, ok := imp.builtins[name]; ok {
		mod := factory()
		imp.cache[name] = mod
		return mod, nil
	}

	// 3. Search the file system.
	mod, err := imp.importFromFile(name)
	if err != nil {
		return nil, err
	}
	if mod != nil {
		imp.cache[name] = mod
		return mod, nil
	}

	// Module not found.
	return nil, object.NewException(object.TypeModuleNotFoundError,
		fmt.Sprintf("No module named '%s'", name))
}

// ImportFrom imports a specific attribute from a module.
// Equivalent to: from <moduleName> import <attrName>
func (imp *Importer) ImportFrom(moduleName, attrName string) (object.Object, error) {
	mod, err := imp.Import(moduleName)
	if err != nil {
		return nil, err
	}

	val, getErr := mod.GetAttr(attrName)
	if getErr != nil {
		return nil, object.NewException(object.TypeImportError,
			fmt.Sprintf("cannot import name '%s' from '%s'", attrName, moduleName))
	}
	return val, nil
}

// SearchPaths returns the current search paths (for sys.path).
func (imp *Importer) SearchPaths() []string {
	return imp.searchPaths
}

// importFromFile searches the file system for a module .py file.
func (imp *Importer) importFromFile(name string) (*object.ModuleObject, error) {
	if imp.compileAndExec == nil {
		return nil, nil
	}

	filename := name + ".py"
	for _, dir := range imp.searchPaths {
		path := filepath.Join(dir, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			continue // file doesn't exist in this directory
		}

		// Create the module object with its own namespace.
		mod := object.NewModule(name)

		// Execute the module source with the module's dict as globals.
		execErr := imp.compileAndExec(string(data), path, mod.Dict)
		if execErr != nil {
			return nil, execErr
		}

		return mod, nil
	}

	return nil, nil // not found
}
