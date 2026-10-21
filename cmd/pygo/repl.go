package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/alvinjaison/PyGo/pygo"
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
	inBlock := false // true once we've started a compound (indented) statement
	prompt := ">>> "

	flush := func() {
		src := buf.String()
		buf.Reset()
		inBlock = false
		prompt = ">>> "
		if strings.TrimSpace(src) == "" {
			return
		}
		runREPLChunk(rt, src)
	}

	for {
		fmt.Print(prompt)
		if !scanner.Scan() {
			fmt.Println()
			if buf.Len() > 0 {
				flush()
			}
			break
		}

		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if buf.Len() == 0 && (trimmed == "exit()" || trimmed == "quit()") {
			break
		}

		if inBlock {
			// Inside a compound statement: a blank line ends it; otherwise keep
			// accumulating.
			if trimmed == "" {
				flush()
				continue
			}
			buf.WriteString(line)
			buf.WriteByte('\n')
			prompt = "... "
			continue
		}

		buf.WriteString(line)
		buf.WriteByte('\n')

		if opensBlock(line) || unclosedBrackets(buf.String()) {
			inBlock = true
			prompt = "... "
			continue
		}

		flush()
	}

	return 0
}

// runREPLChunk evaluates a complete source chunk, printing an expression
// result when applicable and reporting errors.
func runREPLChunk(rt *pygo.Runtime, src string) {
	// Single-line expressions get their repr printed (CPython REPL behavior).
	if !strings.ContainsRune(strings.TrimRight(src, "\n"), '\n') {
		if result, err := rt.Eval(strings.TrimSpace(src)); err == nil {
			if result != nil && !result.IsNone() {
				fmt.Println(result.Repr())
			}
			return
		}
	}
	if err := rt.RunString(src); err != nil {
		if pyErr := pygo.AsError(err); pyErr != nil {
			fmt.Fprint(os.Stderr, pyErr.Format())
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

// opensBlock reports whether a line opens a compound statement (ends with ':').
func opensBlock(line string) bool {
	s := strings.TrimRight(line, " \t")
	// Ignore trailing comments for this check.
	if i := strings.IndexByte(s, '#'); i >= 0 {
		s = strings.TrimRight(s[:i], " \t")
	}
	return strings.HasSuffix(s, ":")
}

// unclosedBrackets reports whether the source has unbalanced (), [], or {}.
func unclosedBrackets(source string) bool {
	depth := 0
	for _, ch := range source {
		switch ch {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		}
	}
	return depth > 0
}
