package stdlib

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pygo/pygo/internal/object"
)

// buildOS constructs the `os` module (a restricted subset).
//
// This module touches the host filesystem and environment. It is considered
// OS-capable and is blocked by the import system when a runtime is created in
// restricted mode (see stdlib.IsOSCapable and the importer).
func buildOS(name string) *object.ModuleObject {
	mod, set := newModule(name)

	// os.name and separators.
	set("name", object.StrFrom("posix"))
	set("sep", object.StrFrom(string(filepath.Separator)))
	set("linesep", object.StrFrom("\n"))
	set("curdir", object.StrFrom("."))
	set("pardir", object.StrFrom(".."))

	// getcwd() → current working directory.
	set("getcwd", fn("os.getcwd", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
		wd, err := os.Getwd()
		if err != nil {
			return nil, object.NewException(object.TypeOSError, err.Error())
		}
		return object.StrFrom(wd), nil
	}))

	// getenv(key[, default]) → environment variable value or default.
	set("getenv", fn("os.getenv", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		key, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		if v, ok := os.LookupEnv(key); ok {
			return object.StrFrom(v), nil
		}
		if len(args) > 1 {
			return args[1], nil
		}
		return object.None, nil
	}))

	// environ as a dict snapshot.
	environ := object.NewDict()
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			_ = environ.SetItem(object.StrFrom(kv[:i]), object.StrFrom(kv[i+1:]))
		}
	}
	set("environ", environ)

	// listdir(path='.') → list of directory entries.
	set("listdir", fn("os.listdir", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		dir := "."
		if len(args) > 0 {
			var err error
			dir, err = argStr(args, 0)
			if err != nil {
				return nil, err
			}
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, object.NewException(object.TypeOSError, err.Error())
		}
		names := make([]object.Object, len(entries))
		for i, e := range entries {
			names[i] = object.StrFrom(e.Name())
		}
		return object.NewList(names), nil
	}))

	// mkdir / makedirs / remove / rmdir.
	set("mkdir", fn("os.mkdir", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		p, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		if err := os.Mkdir(p, 0o755); err != nil {
			return nil, object.NewException(object.TypeOSError, err.Error())
		}
		return object.None, nil
	}))
	set("makedirs", fn("os.makedirs", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		p, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(p, 0o755); err != nil {
			return nil, object.NewException(object.TypeOSError, err.Error())
		}
		return object.None, nil
	}))
	set("remove", fn("os.remove", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		p, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		if err := os.Remove(p); err != nil {
			return nil, object.NewException(object.TypeOSError, err.Error())
		}
		return object.None, nil
	}))
	set("rmdir", fn("os.rmdir", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		p, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		if err := os.Remove(p); err != nil {
			return nil, object.NewException(object.TypeOSError, err.Error())
		}
		return object.None, nil
	}))

	// getpid().
	set("getpid", fn("os.getpid", func(_ []object.Object, _ map[string]object.Object) (object.Object, error) {
		return object.IntFrom(int64(os.Getpid())), nil
	}))

	// os.path submodule.
	set("path", buildOSPath())

	return mod
}

// buildOSPath constructs the os.path submodule.
func buildOSPath() *object.ModuleObject {
	mod, set := newModule("posixpath")

	set("join", fn("os.path.join", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		parts := make([]string, len(args))
		for i := range args {
			s, err := argStr(args, i)
			if err != nil {
				return nil, err
			}
			parts[i] = s
		}
		return object.StrFrom(filepath.Join(parts...)), nil
	}))
	set("exists", fn("os.path.exists", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		p, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		_, statErr := os.Stat(p)
		return object.BoolFrom(statErr == nil), nil
	}))
	set("isfile", fn("os.path.isfile", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		p, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		info, statErr := os.Stat(p)
		return object.BoolFrom(statErr == nil && !info.IsDir()), nil
	}))
	set("isdir", fn("os.path.isdir", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		p, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		info, statErr := os.Stat(p)
		return object.BoolFrom(statErr == nil && info.IsDir()), nil
	}))
	set("basename", fn("os.path.basename", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		p, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		return object.StrFrom(filepath.Base(p)), nil
	}))
	set("dirname", fn("os.path.dirname", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		p, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		return object.StrFrom(filepath.Dir(p)), nil
	}))
	set("abspath", fn("os.path.abspath", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		p, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		abs, absErr := filepath.Abs(p)
		if absErr != nil {
			return nil, object.NewException(object.TypeOSError, absErr.Error())
		}
		return object.StrFrom(abs), nil
	}))
	set("splitext", fn("os.path.splitext", func(args []object.Object, _ map[string]object.Object) (object.Object, error) {
		p, err := argStr(args, 0)
		if err != nil {
			return nil, err
		}
		ext := filepath.Ext(p)
		root := p[:len(p)-len(ext)]
		return object.NewTuple([]object.Object{object.StrFrom(root), object.StrFrom(ext)}), nil
	}))

	return mod
}
