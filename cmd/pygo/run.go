package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pygo/pygo/pygo"
)

// runScript executes a .py file and returns the process exit code.
func runScript(cfg *config) int {
	rt := buildRuntime(cfg)
	ctx := buildContext(cfg)

	// Set sys.argv.
	argv := append([]string{cfg.script}, cfg.scriptArgs...)
	if err := rt.Set("__argv__", argv); err != nil {
		fmt.Fprintln(os.Stderr, "pygo: failed to set sys.argv:", err)
		return 2
	}

	if err := rt.RunFileContext(ctx, cfg.script); err != nil {
		return handleError(err)
	}
	return 0
}

// runCommand executes a -c source string and returns the process exit code.
func runCommand(cfg *config) int {
	rt := buildRuntime(cfg)
	ctx := buildContext(cfg)

	if err := rt.RunStringContext(ctx, cfg.command); err != nil {
		return handleError(err)
	}
	return 0
}

// buildRuntime creates a pygo.Runtime from the given config.
func buildRuntime(cfg *config) *pygo.Runtime {
	opts := []pygo.Option{
		pygo.WithMaxCallDepth(cfg.maxDepth),
		pygo.WithStdlib(!cfg.noStdlib),
	}
	if len(cfg.searchPath) > 0 {
		opts = append(opts, pygo.WithSearchPath(cfg.searchPath))
	}
	if cfg.timeout > 0 {
		opts = append(opts, pygo.WithExecutionTimeout(cfg.timeout))
	}
	return pygo.New(opts...)
}

// buildContext creates a context.Context applying any timeout from the config.
func buildContext(cfg *config) context.Context {
	if cfg.timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
		_ = cancel // the process will exit before the context needs cancelling
		return ctx
	}
	return context.Background()
}

// handleError prints a Python traceback or error to stderr and returns the
// appropriate exit code.
func handleError(err error) int {
	if pyErr := pygo.AsError(err); pyErr != nil {
		// Python exception: print CPython-style traceback.
		fmt.Fprint(os.Stderr, pyErr.Format())
		// SystemExit: use its exit code.
		if pyErr.Type == "SystemExit" {
			return parseSystemExitCode(pyErr.Message)
		}
		return 1
	}
	// Non-Python error (IO error, etc.)
	fmt.Fprintf(os.Stderr, "pygo: %v\n", err)
	return 2
}

// parseSystemExitCode parses the exit code from a SystemExit message.
func parseSystemExitCode(msg string) int {
	if msg == "" || msg == "0" || msg == "None" {
		return 0
	}
	var code int
	if _, err := fmt.Sscanf(msg, "%d", &code); err == nil {
		return code
	}
	// Non-integer message: print it to stderr and exit 1.
	fmt.Fprintln(os.Stderr, msg)
	return 1
}

// ensure time import is used
var _ = time.Second
