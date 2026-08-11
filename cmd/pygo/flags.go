package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"
)

// runMode identifies how the interpreter was invoked.
type runMode int

const (
	modeREPL    runMode = iota // no arguments → interactive REPL
	modeScript                 // first non-flag argument is a .py file
	modeCommand                // -c "source"
)

// config holds the parsed command-line configuration.
type config struct {
	mode       runMode
	script     string        // path to .py file (modeScript)
	command    string        // inline Python source (modeCommand)
	scriptArgs []string      // arguments passed to sys.argv[1:]
	searchPath []string      // extra entries for sys.path
	maxDepth   int           // max call stack depth
	timeout    time.Duration // execution timeout (0 = none)
	noStdlib   bool          // disable standard library
	verbose    bool          // verbose/diagnostic output
}

// parseFlags parses command-line arguments and returns a config.
// It does not call os.Exit; errors are returned to the caller.
func parseFlags(args []string) (*config, error) {
	fs := flag.NewFlagSet("pygo", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		cmdFlag      = fs.String("c", "", "Execute `source` string and exit")
		maxDepth     = fs.Int("max-depth", 500, "Maximum call stack `depth`")
		timeoutStr   = fs.String("timeout", "", "Execution `timeout` (e.g. 30s, 5m)")
		noStdlib     = fs.Bool("no-stdlib", false, "Disable standard library modules")
		verbose      = fs.Bool("verbose", false, "Enable verbose/diagnostic output")
		versionFlag  = fs.Bool("version", false, "Print version and exit")
		vFlag        = fs.Bool("v", false, "Print version and exit")
	)

	// Custom usage message.
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  pygo [flags] script.py [args...]  Run a Python script\n")
		fmt.Fprintf(os.Stderr, "  pygo -c \"source\"                  Execute source string\n")
		fmt.Fprintf(os.Stderr, "  pygo                               Interactive REPL\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		return nil, err
	}

	if *versionFlag || *vFlag {
		fmt.Printf("PyGo %s\n", version())
		os.Exit(0)
	}

	cfg := &config{
		maxDepth: *maxDepth,
		noStdlib: *noStdlib,
		verbose:  *verbose,
	}

	if *timeoutStr != "" {
		d, err := time.ParseDuration(*timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout %q: %v", *timeoutStr, err)
		}
		cfg.timeout = d
	}

	remaining := fs.Args()

	switch {
	case *cmdFlag != "":
		cfg.mode = modeCommand
		cfg.command = *cmdFlag
		cfg.scriptArgs = remaining

	case len(remaining) > 0:
		cfg.mode = modeScript
		cfg.script = remaining[0]
		cfg.scriptArgs = remaining[1:]

	default:
		cfg.mode = modeREPL
	}

	return cfg, nil
}

// version returns the PyGo version string.
func version() string {
	return "0.1.0-dev"
}
