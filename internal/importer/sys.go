package importer

import (
	"fmt"
	"os"
	"strconv"

	"github.com/alvinjaison/pygo/internal/object"
)

// PyGoVersion is the version string exposed via sys.version.
const PyGoVersion = "0.1.0 (PyGo)"

// NewSysModule creates the built-in sys module.
// argv and path are provided by the runtime at creation time.
func NewSysModule(argv []string, searchPaths []string) *object.ModuleObject {
	mod := object.NewModule("sys")

	// sys.argv — list of command-line arguments
	argvItems := make([]object.Object, len(argv))
	for i, a := range argv {
		argvItems[i] = object.StrFrom(a)
	}
	_ = mod.SetAttr("argv", object.NewList(argvItems))

	// sys.version — version string
	_ = mod.SetAttr("version", object.StrFrom(PyGoVersion))

	// sys.path — module search paths
	pathItems := make([]object.Object, len(searchPaths))
	for i, p := range searchPaths {
		pathItems[i] = object.StrFrom(p)
	}
	_ = mod.SetAttr("path", object.NewList(pathItems))

	// sys.exit([code]) — raise SystemExit
	exitFn := object.NewBuiltin("exit", func(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		code := 0
		if len(args) > 0 {
			switch v := args[0].(type) {
			case *object.IntObject:
				if n, ok := v.Int64(); ok {
					code = int(n)
				}
			case *object.StrObject:
				// sys.exit("message") prints the message and exits with code 1
				fmt.Fprintln(os.Stderr, v.Value)
				code = 1
			}
		}
		exc := object.NewException(object.TypeSystemExit, strconv.Itoa(code))
		return nil, exc
	})
	_ = mod.SetAttr("exit", exitFn)

	return mod
}
