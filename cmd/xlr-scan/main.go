// Command xlr-scan analyses Digital.ai Release (XL Release) exports and reports
// whether the Jython scripts inside them can run on PyGo.
//
// It extracts every inline script it can find (.xlr archives, raw JSON exports,
// release-as-code YAML, loose .py files), then runs each one through PyGo's own
// lexer, parser and compiler. Feature support is probed against the live PyGo
// runtime rather than hard-coded, so the report stays accurate as PyGo grows.
//
// Usage:
//
//	xlr-scan [flags] <dir-or-file>
//
// The report answers three questions:
//
//  1. Which scripts run on PyGo today?
//  2. What must change in PyGo to run the rest, ranked by how many scripts
//     each change unblocks?
//  3. What must change in the scripts themselves (Python 2, JVM imports)?
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var (
		jsonOut   string
		dumpDir   string
		details   bool
		run       bool
		top       int
		extraKeys string
	)
	flag.BoolVar(&run, "run", false, "execute the scripts against a stub host and report what breaks")
	flag.StringVar(&jsonOut, "json", "", "write the full report as JSON to this path")
	flag.StringVar(&dumpDir, "dump", "", "write every extracted script to this directory as .py files")
	flag.BoolVar(&details, "details", false, "include a per-script breakdown in the text report")
	flag.IntVar(&top, "top", 25, "maximum number of rows per frequency table")
	flag.StringVar(&extraKeys, "keys", "", "comma-separated extra JSON keys to treat as script fields")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
		os.Exit(2)
	}

	root := flag.Arg(0)
	info, err := os.Stat(root)
	if err != nil {
		fatalf("cannot read %s: %v", root, err)
	}

	ex := newExtractor(splitCSV(extraKeys))
	if err := ex.Scan(root, info.IsDir()); err != nil {
		fatalf("scan failed: %v", err)
	}

	if len(ex.scripts) == 0 {
		fmt.Fprintf(os.Stderr, "no scripts found under %s\n", root)
		if len(ex.notes) > 0 {
			fmt.Fprintln(os.Stderr, "\nnotes:")
			for _, n := range ex.notes {
				fmt.Fprintf(os.Stderr, "  %s: %s\n", n.Origin, n.Message)
			}
		}
		fmt.Fprintln(os.Stderr, "\nIf your export uses a field name this tool does not know,")
		fmt.Fprintln(os.Stderr, "add it with -keys (for example: -keys myScriptField).")
		os.Exit(1)
	}

	rep := Analyse(root, ex.scripts, ex.notes, ex.containers, ex.skipped)
	if run {
		rep.RunResults = RunScripts(rep.Scripts)
	}
	writeText(os.Stdout, rep, top, details)
	if run {
		writeRunReport(os.Stdout, rep.RunResults)
	}

	if dumpDir != "" {
		if err := dumpScripts(dumpDir, ex.scripts); err != nil {
			fatalf("dump failed: %v", err)
		}
		fmt.Fprintf(os.Stderr, "\nwrote %d scripts to %s\n", len(ex.scripts), dumpDir)
	}

	if jsonOut != "" {
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fatalf("encode report: %v", err)
		}
		if err := os.WriteFile(jsonOut, append(data, '\n'), 0o644); err != nil {
			fatalf("write %s: %v", jsonOut, err)
		}
		fmt.Fprintf(os.Stderr, "wrote JSON report to %s\n", jsonOut)
	}
}

// dumpScripts writes each extracted script to dir as a standalone .py file so
// the corpus can be replayed against pygo, CPython or Jython individually.
func dumpScripts(dir string, scripts []Script) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	used := make(map[string]int)
	for _, s := range scripts {
		base := sanitiseFilename(s.ID)
		if len(base) > 120 {
			base = base[:120]
		}
		used[base]++
		if n := used[base]; n > 1 {
			base = fmt.Sprintf("%s_%d", base, n)
		}
		path := filepath.Join(dir, base+".py")
		header := fmt.Sprintf("# origin:   %s\n# location: %s\n", s.Origin, s.Location)
		if s.TaskType != "" {
			header += fmt.Sprintf("# type:     %s\n", s.TaskType)
		}
		if s.Title != "" {
			header += fmt.Sprintf("# title:    %s\n", s.Title)
		}
		body := header + "\n" + s.Source
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func sanitiseFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "script"
	}
	return out
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func usage() {
	fmt.Fprintf(os.Stderr, `xlr-scan - check XL Release Jython scripts against PyGo

usage: xlr-scan [flags] <dir-or-file>

Accepts a directory of exports, or a single file. Recognised inputs:
  *.xlr, *.zip, *.jar   archive containing JSON (the normal XLR template export)
  *.json                raw XLR export JSON
  *.yaml, *.yml         release-as-code (block scalars only, best effort)
  *.py, *.jy            a loose script

flags:
`)
	flag.PrintDefaults()
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "xlr-scan: "+format+"\n", args...)
	os.Exit(1)
}
