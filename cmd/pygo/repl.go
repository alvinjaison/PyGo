package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pygo/pygo/pygo"
)

// runREPL starts the interactive Read-Eval-Print Loop and returns an exit code.
//
// The REPL uses the same pygo.Runtime as script execution — there is no
// separate interpreter implementation. It accumulates input lines until a
// complete Python statement is formed, then executes it.
//
// TODO(Phase 6): Replace the basic stdin reader with the readline library
// (github.com/chzyer/readline) to support line history, editing, and
// completion.
func runREPL(cfg *config) int {
	rt := buildRuntime(cfg)

	fmt.Printf("PyGo %s\nType \"exit()\" or Ctrl-D to quit.\n", version())

	scanner := bufio.NewScanner(os.Stdin)
	var buf strings.Builder
	prompt := ">>> "

	for {
		fmt.Print(prompt)
		if !scanner.Scan() {
			// EOF (Ctrl-D)
			fmt.Println()
			break
		}

		line := scanner.Text()

		// Handle exit() / quit() as special cases before parsing.
		trimmed := strings.TrimSpace(line)
		if trimmed == "exit()" || trimmed == "quit()" {
			break
		}

		buf.WriteString(line)
		buf.WriteByte('\n')
		source := buf.String()

		// Detect incomplete input: if the line ends with ':' or the buffer
		// has unclosed brackets, wait for more input.
		if isIncomplete(source) {
			prompt = "... "
			continue
		}

		// Execute the accumulated source.
		prompt = ">>> "
		src := buf.String()
		buf.Reset()

		if strings.TrimSpace(src) == "" {
			continue
		}

		// Try to evaluate as an expression first (for REPL result display).
		// If that fails, run as a statement.
		result, evalErr := rt.Eval(strings.TrimSpace(src))
		if evalErr == nil && result != nil && !result.IsNone() {
			fmt.Println(result.Repr())
			continue
		}

		if err := rt.RunString(src); err != nil {
			if pyErr := pygo.AsError(err); pyErr != nil {
				fmt.Fprint(os.Stderr, pyErr.Format())
			} else {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}

	return 0
}

// isIncomplete reports whether source is an incomplete Python statement that
// needs more input. This is a simplified heuristic; Phase 6 will use the
// parser's ParseIncomplete result for accurate detection.
func isIncomplete(source string) bool {
	lines := strings.Split(strings.TrimRight(source, "\n"), "\n")
	if len(lines) == 0 {
		return false
	}
	last := strings.TrimRight(lines[len(lines)-1], " \t")

	// A line ending with ':' opens a block.
	if strings.HasSuffix(last, ":") {
		return true
	}

	// Unclosed brackets/parens/braces anywhere in the buffer.
	depth := 0
	for _, ch := range source {
		switch ch {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		}
	}
	if depth > 0 {
		return true
	}

	// A non-empty continuation block: if we already have indented lines and
	// the last line is non-empty, we may still be inside a block.
	if len(lines) > 1 && last != "" && (strings.HasPrefix(last, " ") || strings.HasPrefix(last, "\t")) {
		return true
	}

	return false
}
