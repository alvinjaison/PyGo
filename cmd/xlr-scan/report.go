package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

func writeText(w io.Writer, rep *Report, top int, details bool) {
	fmt.Fprintf(w, "PyGo compatibility scan\n")
	fmt.Fprintf(w, "=======================\n\n")
	fmt.Fprintf(w, "input        %s\n", rep.Root)
	fmt.Fprintf(w, "files read   %d\n", rep.Containers)
	fmt.Fprintf(w, "python       %d scripts (%d unique by content)\n", rep.ScriptCount, rep.UniqueScripts)
	if len(rep.NonPython) > 0 {
		byLang := map[string]int{}
		for _, s := range rep.NonPython {
			byLang[s.Language]++
		}
		langs := make([]string, 0, len(byLang))
		for l := range byLang {
			langs = append(langs, fmt.Sprintf("%s %d", l, byLang[l]))
		}
		sort.Strings(langs)
		fmt.Fprintf(w, "other langs  %d scripts skipped (%s)\n", len(rep.NonPython), strings.Join(langs, ", "))
	}
	fmt.Fprintf(w, "pygo builtins available: %d (%s)\n\n",
		len(rep.AvailableBuiltins), strings.Join(rep.AvailableBuiltins, " "))

	section(w, "Verdict")
	rows := [][]string{{"scripts", "share", "verdict"}}
	for _, v := range verdictOrder {
		n := rep.Verdicts[v]
		if n == 0 {
			continue
		}
		rows = append(rows, []string{strconv.Itoa(n), pct(n, rep.ScriptCount), v})
	}
	table(w, rows)

	runnable := rep.Verdicts[verdictRuns] + rep.Verdicts[verdictHostAPI]
	fmt.Fprintf(w, "\n%d of %d scripts (%s) need no PyGo language work - only host API registration.\n",
		runnable, rep.ScriptCount, pct(runnable, rep.ScriptCount))
	fmt.Fprintln(w, "\nThat is a static verdict: it means the script parses, compiles, and imports")
	fmt.Fprintln(w, "nothing PyGo lacks. Gaps in individual str or dict methods can still surface at")
	fmt.Fprintln(w, "run time. Use -dump to write the corpus out and replay it once host stubs exist.")

	// ----------------------------------------------------------------- PyGo
	section(w, "Work queue for PyGo, ranked by scripts unblocked")

	if len(rep.MissingFeatures) == 0 && len(rep.ParseErrors) == 0 && len(rep.CompileErrors) == 0 {
		fmt.Fprintln(w, "  nothing: every script parses and compiles on PyGo as it stands.")
	}
	if len(rep.MissingFeatures) > 0 {
		fmt.Fprintln(w, "\nSyntax and language features PyGo does not support yet:")
		rows = [][]string{{"scripts", "share", "feature", "where to implement"}}
		for _, s := range limit(rep.MissingFeatures, top) {
			rows = append(rows, []string{
				strconv.Itoa(s.Scripts), pct(s.Scripts, rep.ScriptCount), s.Name, s.Detail,
			})
		}
		table(w, rows)
	}
	// JVM and host modules are reported in their own sections; they are not
	// PyGo work items.
	pygoModules := excludeByDetail(rep.MissingModules, catJVM, catHost)
	if len(pygoModules) > 0 {
		fmt.Fprintln(w, "\nModules the scripts import that PyGo cannot provide yet:")
		rows = [][]string{{"scripts", "module", "kind", "action"}}
		for _, s := range limit(pygoModules, top) {
			kind, action := splitDetail(s.Detail)
			rows = append(rows, []string{strconv.Itoa(s.Scripts), s.Name, kind, action})
		}
		table(w, rows)
	}
	if len(rep.MissingBuiltins) > 0 {
		fmt.Fprintln(w, "\nBuilt-in functions the scripts use that PyGo has not registered:")
		rows = [][]string{{"scripts", "builtin"}}
		for _, s := range limit(rep.MissingBuiltins, top) {
			rows = append(rows, []string{strconv.Itoa(s.Scripts), s.Name})
		}
		table(w, rows)
		fmt.Fprintln(w, "  all of these live in internal/runtime/builtins.go")
	}
	if len(rep.MissingMethods) > 0 {
		fmt.Fprintln(w, "\nMethods called on built-in types that PyGo does not resolve:")
		rows = [][]string{{"scripts", "method", "where to implement"}}
		for _, s := range limit(rep.MissingMethods, top) {
			rows = append(rows, []string{strconv.Itoa(s.Scripts), "." + s.Name, s.Detail})
		}
		table(w, rows)
		fmt.Fprintln(w, "  Lookups on host objects and imported modules are excluded, so each row is")
		fmt.Fprintln(w, "  a genuine str/list/dict/int method gap.")
	}
	if len(rep.ParseErrors) > 0 {
		fmt.Fprintln(w, "\nParse errors, grouped (the authoritative signal from PyGo's own parser):")
		rows = [][]string{{"scripts", "error"}}
		for _, s := range limit(rep.ParseErrors, top) {
			rows = append(rows, []string{strconv.Itoa(s.Scripts), s.Name})
		}
		table(w, rows)
	}
	if len(rep.CompileErrors) > 0 {
		fmt.Fprintln(w, "\nCompile errors, grouped:")
		rows = [][]string{{"scripts", "error"}}
		for _, s := range limit(rep.CompileErrors, top) {
			rows = append(rows, []string{strconv.Itoa(s.Scripts), s.Name})
		}
		table(w, rows)
	}

	// ------------------------------------------------------------- silent bugs
	if len(rep.MisbehavingFeats) > 0 {
		section(w, "Features that compile but break at run time")
		fmt.Fprintln(w, "Anything marked \"fails silently\" is the dangerous kind: the script runs to")
		fmt.Fprintln(w, "completion and returns wrong data instead of raising.")
		rows = [][]string{{"scripts", "feature", "where to implement"}}
		for _, s := range limit(rep.MisbehavingFeats, top) {
			rows = append(rows, []string{strconv.Itoa(s.Scripts), s.Name, s.Detail})
		}
		table(w, rows)
	}

	// ---------------------------------------------------------------- scripts
	if len(rep.Python2) > 0 {
		section(w, "Python 2 constructs (script-side changes, not PyGo work)")
		rows = [][]string{{"scripts", "hits", "construct", "python 3 form"}}
		for _, s := range limit(rep.Python2, top) {
			rows = append(rows, []string{
				strconv.Itoa(s.Scripts), strconv.Itoa(s.Occurrences), s.Name, s.Detail,
			})
		}
		table(w, rows)
	}

	jvm := filterByDetail(rep.MissingModules, catJVM)
	if len(jvm) > 0 {
		section(w, "JVM dependencies (cannot be fixed inside PyGo)")
		rows = [][]string{{"scripts", "import"}}
		for _, s := range limit(jvm, top) {
			rows = append(rows, []string{strconv.Itoa(s.Scripts), s.Name})
		}
		table(w, rows)
		fmt.Fprintln(w, "  Options: hand-write a Go shim module under the same name, keep a Jython")
		fmt.Fprintln(w, "  fallback runner for these scripts, or rewrite them.")
	}

	// ------------------------------------------------------------------- host
	hostModules := filterByDetail(rep.MissingModules, catHost)
	if len(rep.HostAPI) > 0 || len(rep.UnknownNames) > 0 || len(hostModules) > 0 {
		section(w, "Host API surface your tool must register")
		if len(hostModules) > 0 {
			fmt.Fprintln(w, "\nRelease-specific modules the scripts import:")
			rows = [][]string{{"scripts", "module"}}
			for _, s := range limit(hostModules, top) {
				rows = append(rows, []string{strconv.Itoa(s.Scripts), s.Name})
			}
			table(w, rows)
		}
		if len(rep.HostAPI) > 0 {
			fmt.Fprintln(w, "\nKnown Digital.ai Release globals:")
			rows = [][]string{{"scripts", "name", "provided by"}}
			for _, s := range limit(rep.HostAPI, top) {
				rows = append(rows, []string{strconv.Itoa(s.Scripts), s.Name, s.Detail})
			}
			table(w, rows)
		}
		if len(rep.UnknownNames) > 0 {
			fmt.Fprintln(w, "\nUnresolved names (host-injected, plugin-provided, or genuine bugs):")
			rows = [][]string{{"scripts", "name"}}
			for _, s := range limit(rep.UnknownNames, top) {
				rows = append(rows, []string{strconv.Itoa(s.Scripts), s.Name})
			}
			table(w, rows)
			fmt.Fprintln(w, "  Each of these is read but never assigned or imported in its script, so the")
			fmt.Fprintln(w, "  platform must be supplying it. Confirm every one against the Release script")
			fmt.Fprintln(w, "  context before assuming it is a bug.")
		}
		fmt.Fprintln(w, "\n  Register these with rt.Register / rt.RegisterFunction / rt.RegisterModule.")
	}

	// --------------------------------------------------------------- supported
	if len(rep.SupportedFeatures) > 0 {
		section(w, "Features these scripts use that PyGo already handles")
		var names []string
		for _, s := range rep.SupportedFeatures {
			names = append(names, fmt.Sprintf("%s (%d)", s.Name, s.Scripts))
		}
		fmt.Fprintf(w, "  %s\n", strings.Join(names, ", "))
	}

	// ------------------------------------------------------------------ detail
	if details {
		section(w, "Per-script detail")
		byVerdict := map[string][]ScriptReport{}
		for _, sr := range rep.Scripts {
			byVerdict[sr.Verdict] = append(byVerdict[sr.Verdict], sr)
		}
		for _, v := range verdictOrder {
			list := byVerdict[v]
			if len(list) == 0 {
				continue
			}
			fmt.Fprintf(w, "\n--- %s (%d)\n", v, len(list))
			sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
			for _, sr := range list {
				fmt.Fprintf(w, "\n  %s\n", sr.ID)
				if sr.Title != "" || sr.TaskType != "" {
					fmt.Fprintf(w, "    task:     %s %s\n", sr.TaskType, quoteIf(sr.Title))
				}
				fmt.Fprintf(w, "    field:    %s, %d lines\n", sr.Field, sr.Lines)
				for _, b := range sr.Blockers {
					fmt.Fprintf(w, "    blocker:  %s\n", b)
				}
				for _, f := range sr.Features {
					if f.Supported {
						continue
					}
					silent := ""
					if f.Silent {
						silent = " (fails silently)"
					}
					fmt.Fprintf(w, "    feature:  %s%s at line(s) %s\n", f.Name, silent, intList(f.Lines))
				}
				for _, d := range sr.Python2 {
					fmt.Fprintf(w, "    python2:  %s at line(s) %s -> %s\n", d.Name, intList(d.Lines), d.Fix)
				}
			}
		}
	}

	if len(rep.NonPython) > 0 {
		section(w, "Non-Python scripts (skipped, not PyGo's problem)")
		rows = [][]string{{"language", "task type", "location"}}
		for _, s := range rep.NonPython {
			rows = append(rows, []string{s.Language, s.TaskType, s.Origin + "#" + s.Location})
		}
		table(w, rows)
		fmt.Fprintln(w, "\n  Your tool still needs a runner for these, just not a Python one.")
	}

	if len(rep.Notes) > 0 {
		section(w, "Notes")
		for _, n := range rep.Notes {
			fmt.Fprintf(w, "  %s\n    %s\n", n.Origin, n.Message)
		}
	}
}

func section(w io.Writer, title string) {
	fmt.Fprintf(w, "\n%s\n%s\n", title, strings.Repeat("-", len(title)))
}

// table prints rows with the first row as a header, padded to align.
func table(w io.Writer, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	cols := 0
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	widths := make([]int, cols)
	for _, r := range rows {
		for i, cell := range r {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	for ri, r := range rows {
		var b strings.Builder
		b.WriteString("  ")
		for i, cell := range r {
			if i == len(r)-1 {
				b.WriteString(cell)
			} else {
				b.WriteString(cell)
				b.WriteString(strings.Repeat(" ", widths[i]-len(cell)+2))
			}
		}
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
		if ri == 0 {
			var sep strings.Builder
			sep.WriteString("  ")
			for i := 0; i < cols; i++ {
				sep.WriteString(strings.Repeat("-", widths[i]))
				if i < cols-1 {
					sep.WriteString("  ")
				}
			}
			fmt.Fprintln(w, strings.TrimRight(sep.String(), " "))
		}
	}
}

func limit(stats []Stat, n int) []Stat {
	if n <= 0 || len(stats) <= n {
		return stats
	}
	return stats[:n]
}

func filterByDetail(stats []Stat, prefix string) []Stat {
	var out []Stat
	for _, s := range stats {
		if strings.HasPrefix(s.Detail, prefix) {
			out = append(out, s)
		}
	}
	return out
}

func excludeByDetail(stats []Stat, prefixes ...string) []Stat {
	var out []Stat
	for _, s := range stats {
		skip := false
		for _, p := range prefixes {
			if strings.HasPrefix(s.Detail, p) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, s)
		}
	}
	return out
}

func splitDetail(detail string) (string, string) {
	if i := strings.Index(detail, " - "); i >= 0 {
		return detail[:i], detail[i+3:]
	}
	return detail, ""
}

func intList(in []int) string {
	parts := make([]string, 0, len(in))
	for _, n := range in {
		parts = append(parts, strconv.Itoa(n))
	}
	return strings.Join(parts, ", ")
}

func quoteIf(s string) string {
	if s == "" {
		return ""
	}
	return strconv.Quote(s)
}
