// Command pygo is the PyGo standalone Python 3 interpreter.
//
// Usage:
//
//	pygo [flags] script.py [args...]   Run a Python script
//	pygo -c "source"                   Execute a source string
//	pygo                               Start an interactive REPL
//
// See pygo --help for all flags.
package main

import (
	"fmt"
	"os"
)

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "pygo: %v\n", err)
		os.Exit(2)
	}

	var exitCode int
	switch cfg.mode {
	case modeScript:
		exitCode = runScript(cfg)
	case modeCommand:
		exitCode = runCommand(cfg)
	case modeREPL:
		exitCode = runREPL(cfg)
	default:
		fmt.Fprintln(os.Stderr, "pygo: unknown mode")
		exitCode = 2
	}

	os.Exit(exitCode)
}
