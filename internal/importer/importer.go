// Package importer implements Python's module import resolution for PyGo.
//
// Resolution order for a top-level module name:
//  1. the module cache (equivalent to sys.modules)
//  2. built-in Go modules (internal/stdlib)
//  3. a <name>.py file on the search path
//  4. a package directory <name>/__init__.py on the search path
//
// Executing .py module code requires compiling and running it in a fresh
// namespace; the importer delegates that to an Execer supplied by the runtime,
// avoiding an import cycle with internal/vm.
package importer

import (
	"os"
	"path/filepath"

	"github.com/pygo/pygo/internal/object"
	"github.com/pygo/pygo/internal/stdlib"
)

// Execer compiles and executes Python module source in the given module's
// namespace (module.Dict). It is implemented by the runtime.
type Execer interface {
	ExecModule(mod *object.ModuleObject, source, filename string) error
}

// Importer resolves and caches modules for one runtime.
type Importer struct {
	searchPath []string
	cache      map[string]*object.ModuleObject
	inProgress map[string]bool // for circular-import detection
	exec       Execer
	restricted bool // when true, OS-capable modules cannot be imported
}

// New creates an Importer with the given search path and module executor.
// When restricted is true, OS-capable built-in modules (e.g. os) are blocked.
func New(searchPath []string, exec Execer, restricted bool) *Importer {
	if len(searchPath) == 0 {
		searchPath = []string{"."}
	}
	return &Importer{
		searchPath: searchPath,
		cache:      map[string]*object.ModuleObject{},
		inProgress: map[string]bool{},
		exec:       exec,
		restricted: restricted,
	}
}

// Cache returns the module cache (sys.modules equivalent).
func (im *Importer) Cache() map[string]*object.ModuleObject { return im.cache }

// Import resolves and returns the named module, loading it if necessary.
func (im *Importer) Import(name string) (*object.ModuleObject, error) {
	// 1. Cache.
	if mod, ok := im.cache[name]; ok {
		return mod, nil
	}

	// Circular-import guard.
	if im.inProgress[name] {
		// Return the partially-initialized module if present; otherwise error.
		if mod, ok := im.cache[name]; ok {
			return mod, nil
		}
		return nil, object.NewException(object.TypeImportError,
			"circular import detected for module '"+name+"'")
	}

	// 2. Built-in Go module.
	if stdlib.Has(name) {
		if im.restricted && stdlib.IsOSCapable(name) {
			return nil, object.NewException(object.TypeImportError,
				"import of '"+name+"' is not allowed in restricted mode")
		}
		mod := stdlib.Build(name)
		im.cache[name] = mod
		return mod, nil
	}

	// 3 & 4. File on the search path.
	if path, ok := im.findFile(name); ok {
		return im.loadFile(name, path)
	}

	return nil, object.NewException(object.TypeModuleNotFoundError,
		"No module named '"+name+"'")
}

// findFile locates a module's source file on the search path. It checks for
// <name>.py and <name>/__init__.py in each search directory.
func (im *Importer) findFile(name string) (string, bool) {
	rel := filepath.FromSlash(name)
	for _, dir := range im.searchPath {
		if p := filepath.Join(dir, rel+".py"); fileExists(p) {
			return p, true
		}
		if p := filepath.Join(dir, rel, "__init__.py"); fileExists(p) {
			return p, true
		}
	}
	return "", false
}

// loadFile compiles and executes a .py module file in a fresh namespace.
func (im *Importer) loadFile(name, path string) (*object.ModuleObject, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, object.NewException(object.TypeImportError, err.Error())
	}

	mod := object.NewModule(name)
	_ = mod.SetAttr("__file__", object.StrFrom(path))

	// Register in the cache before executing so that circular imports resolve
	// to the in-progress module.
	im.cache[name] = mod
	im.inProgress[name] = true
	defer delete(im.inProgress, name)

	if err := im.exec.ExecModule(mod, string(src), path); err != nil {
		delete(im.cache, name) // failed import: do not keep a broken module
		return nil, err
	}
	return mod, nil
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
