package main

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/alvinjaison/pygo/internal/runtime"
)

// A static verdict only proves a script parses, compiles, and imports nothing
// missing. This runner goes further: it actually executes the scripts whose only
// host dependency is releaseVariables (plus sys), with a stub host in place, and
// reports what breaks. That is the closest thing to ground truth available
// without a real Release server.

// RunResult is the outcome of executing one script.
type RunResult struct {
	ID      string `json:"id"`
	Title   string `json:"title,omitempty"`
	Ran     bool   `json:"ran"`
	Skipped string `json:"skipped,omitempty"`
	Error   string `json:"error,omitempty"`
	Class   string `json:"class,omitempty"` // pygo-gap | data-dependent | unknown
	Output  string `json:"output,omitempty"`
}

// stubbableNames are the host globals the runner can fake convincingly.
// releaseVariables is a plain dict; sys comes from PyGo's own sys module.
var stubbableNames = map[string]bool{
	"releaseVariables": true,
	"globalVariables":  true,
	"sys":              true,
}

const runTimeout = 3 * time.Second

// reReleaseVarKey finds the keys a script reads out of releaseVariables so the
// stub dict can be pre-populated.
var reReleaseVarKey = regexp.MustCompile(`(?:releaseVariables|globalVariables)\s*\[\s*['"]([^'"]+)['"]\s*\]`)

// RunScripts executes every script that only needs stubbable host names.
func RunScripts(reports []ScriptReport) []RunResult {
	results := make([]RunResult, 0, len(reports))
	for _, sr := range reports {
		res := RunResult{ID: sr.ID, Title: sr.Title}

		if !sr.Compiles {
			res.Skipped = "does not compile"
			results = append(results, res)
			continue
		}
		if blocking := nonStubbable(sr); len(blocking) > 0 {
			res.Skipped = "needs real host objects: " + strings.Join(blocking, ", ")
			results = append(results, res)
			continue
		}

		out, err := execWithStubHost(sr.Source, keysFor(sr.Source))
		res.Ran = true
		res.Output = trimOutput(out)
		if err != nil {
			res.Error = err.Error()
			res.Class = classifyRuntimeError(res.Error)
		}
		results = append(results, res)
	}
	return results
}

func nonStubbable(sr ScriptReport) []string {
	var out []string
	for _, n := range append(append([]string{}, sr.HostNames...), sr.UnknownNames...) {
		if !stubbableNames[n] {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return dedupe(out)
}

func keysFor(source string) []string {
	var keys []string
	seen := map[string]bool{}
	for _, m := range reReleaseVarKey.FindAllStringSubmatch(source, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			keys = append(keys, m[1])
		}
	}
	sort.Strings(keys)
	return keys
}

// execWithStubHost seeds a runtime with stub host globals and runs the script
// under a timeout. The goroutine is abandoned on timeout; this is a one-shot
// diagnostic process, so that is acceptable.
func execWithStubHost(source string, keys []string) (string, error) {
	type outcome struct {
		out string
		err error
	}
	done := make(chan outcome, 1)

	go func() {
		var buf strings.Builder
		rt := runtime.New(runtime.WithOutput(&buf))

		// sys is bound the way the platform appears to bind it: as a global.
		_ = rt.ExecString("import sys\n", "<stub>")

		var b strings.Builder
		b.WriteString("releaseVariables = {")
		for i, k := range keys {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%q: %q", k, "stub-value")
		}
		b.WriteString("}\nglobalVariables = {}\n")
		if err := rt.ExecString(b.String(), "<stub>"); err != nil {
			done <- outcome{"", fmt.Errorf("stub setup failed: %w", err)}
			return
		}

		err := rt.ExecString(source, "<script>")
		done <- outcome{buf.String(), err}
	}()

	select {
	case o := <-done:
		return o.out, o.err
	case <-time.After(runTimeout):
		return "", fmt.Errorf("timed out after %s (possible infinite loop, or a VM hang)", runTimeout)
	}
}

// classifyRuntimeError separates PyGo gaps from failures caused by the stub's
// placeholder data. Only the first kind is work for PyGo.
func classifyRuntimeError(msg string) string {
	m := strings.ToLower(msg)
	switch {
	case strings.Contains(m, "nameerror"),
		strings.Contains(m, "attributeerror"),
		strings.Contains(m, "modulenotfounderror"),
		strings.Contains(m, "importerror"),
		strings.Contains(m, "unsupported operand"),
		strings.Contains(m, "not supported"),
		strings.Contains(m, "not implemented"),
		strings.Contains(m, "no len()"),
		strings.Contains(m, "timed out"):
		return "pygo-gap"
	case strings.Contains(m, "indexerror"),
		strings.Contains(m, "keyerror"),
		strings.Contains(m, "valueerror"),
		strings.Contains(m, "zerodivisionerror"),
		strings.Contains(m, "systemexit"):
		return "data-dependent"
	default:
		return "unknown"
	}
}

func trimOutput(s string) string {
	s = strings.TrimRight(s, "\n")
	if len(s) > 400 {
		return s[:400] + "\n... (truncated)"
	}
	return s
}

func writeRunReport(w io.Writer, results []RunResult) {
	var ran, ok, gaps, data, unknown, skipped int
	for _, r := range results {
		switch {
		case r.Skipped != "":
			skipped++
		case r.Error == "":
			ran++
			ok++
		default:
			ran++
			switch r.Class {
			case "pygo-gap":
				gaps++
			case "data-dependent":
				data++
			default:
				unknown++
			}
		}
	}

	section(w, "Execution against a stub host")
	fmt.Fprintf(w, "Each script below was executed for real, with releaseVariables stubbed as a dict\n")
	fmt.Fprintf(w, "of placeholder strings and sys bound as a global.\n\n")

	rows := [][]string{
		{"scripts", "outcome"},
		{fmt.Sprintf("%d", ok), "ran to completion with no error"},
		{fmt.Sprintf("%d", gaps), "failed on a PyGo gap"},
		{fmt.Sprintf("%d", data), "failed on stub placeholder data (not a PyGo problem)"},
		{fmt.Sprintf("%d", unknown), "failed, cause unclassified"},
		{fmt.Sprintf("%d", skipped), "not executable without real host objects"},
	}
	table(w, rows)

	byError := newCounter()
	for _, r := range results {
		if r.Error != "" && r.Class == "pygo-gap" {
			byError.hit(normaliseError(r.Error), 1, "")
		}
	}
	if stats := byError.stats(); len(stats) > 0 {
		fmt.Fprintln(w, "\nPyGo gaps hit at run time, grouped:")
		rows = [][]string{{"scripts", "error"}}
		for _, s := range stats {
			rows = append(rows, []string{fmt.Sprintf("%d", s.Scripts), s.Name})
		}
		table(w, rows)
	}

	fmt.Fprintln(w, "\nPer script:")
	for _, r := range results {
		switch {
		case r.Skipped != "":
			fmt.Fprintf(w, "  SKIP  %s\n          %s\n", r.ID, r.Skipped)
		case r.Error == "":
			fmt.Fprintf(w, "  OK    %s\n", r.ID)
		default:
			fmt.Fprintf(w, "  FAIL  %s\n          [%s] %s\n", r.ID, r.Class, firstLine(r.Error))
		}
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
