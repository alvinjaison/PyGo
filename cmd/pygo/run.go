package main

import (
	"context"
	"fmt"
	"os"

	"github.com/alvinjaison/pygo/pygo"
)

// runScript executes a .py file and returns the process exit code.
func runScript(cfg *config) int {
	// Check file existence early to give a clean error message with exit code 2.
	if _, err := os.Stat(cfg.script); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "pygo: can't open file %q: [Errno 2] No such file or directory\n", cfg.script)
		return 2
	}

	// Build argv: [script, arg1, arg2, ...]
	argv := append([]string{cfg.script}, cfg.scriptArgs...)

	rt := buildRuntime(cfg, argv)
	ctx := buildContext(cfg)

	if err := rt.RunFileContext(ctx, cfg.script); err != nil {
		return handleError(err)
	}
	return 0
}

// runCommand executes a -c source string and returns the process exit code.
func runCommand(cfg *config) int {
	// For -c mode, argv[0] is "-c" per CPython convention.
	argv := append([]string{"-c"}, cfg.scriptArgs...)

	rt := buildRuntime(cfg, argv)
	ctx := buildContext(cfg)

	if err := rt.RunStringContext(ctx, cfg.command); err != nil {
		return handleError(err)
	}
	return 0
}

// buildRuntime creates a pygo.Runtime from the given config.
func buildRuntime(cfg *config, argv []string) *pygo.Runtime {
	opts := []pygo.Option{
		pygo.WithMaxCallDepth(cfg.maxDepth),
		pygo.WithStdlib(!cfg.noStdlib),
		pygo.WithArgv(argv),
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
