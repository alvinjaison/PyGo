package main

import (
	"regexp"
	"strings"
)

// Release templates put Jython, Groovy and shell scripts in fields with the same
// name, so the scanner has to decide what language it is looking at before it
// hands anything to PyGo's parser. Task type is the primary signal; content
// sniffing is the fallback for plugin task types we do not recognise.

const (
	langPython = "python"
	langGroovy = "groovy"
	langShell  = "shell"
	langSQL    = "sql"
)

// scriptLanguage decides what language a script field holds.
func scriptLanguage(taskType, field, source string) string {
	// Preconditions are always evaluated as Python, whatever the task type is.
	if field == "precondition" {
		return langPython
	}

	t := strings.ToLower(taskType)
	switch {
	case t == "":
		return sniffLanguage(source)
	case strings.Contains(t, "groovy"):
		return langGroovy
	case strings.HasPrefix(t, "remotescript."),
		strings.Contains(t, "shell"),
		strings.Contains(t, "powershell"),
		strings.Contains(t, "bash"),
		strings.Contains(t, "batch"),
		strings.Contains(t, "cmdscript"):
		return langShell
	case strings.Contains(t, "sqlscript"), strings.Contains(t, "database"):
		return langSQL
	case strings.Contains(t, "jython"),
		strings.Contains(t, "python"),
		strings.Contains(t, "scripttask"):
		return langPython
	default:
		return sniffLanguage(source)
	}
}

var (
	groovyMarkers = []*regexp.Regexp{
		regexp.MustCompile(`(?m)^[ \t]*def[ \t]+\w+[ \t]*=`),
		regexp.MustCompile(`\bprintln\s*\(`),
		regexp.MustCompile(`\}[ \t]*else[ \t]*\{`),
		regexp.MustCompile(`(?m)^[ \t]*//`),
		regexp.MustCompile(`(?m)\{[ \t]*$`),
		regexp.MustCompile(`\bnew\s+\w+\s*\(`),
	}
	shellMarkers = []*regexp.Regexp{
		regexp.MustCompile(`^#![ \t]*/`),
		regexp.MustCompile(`(?m)^[ \t]*(echo|export|sudo|source|curl|mkdir|chmod)[ \t]`),
		regexp.MustCompile(`\$\(`),
		regexp.MustCompile(`(?m)^[ \t]*fi[ \t]*$`),
		regexp.MustCompile(`(?m)^[ \t]*done[ \t]*$`),
	}
	pythonMarkers = []*regexp.Regexp{
		regexp.MustCompile(`(?m)^[ \t]*(import|from)[ \t]+\w`),
		regexp.MustCompile(`(?m)^[ \t]*def[ \t]+\w+[ \t]*\(`),
		regexp.MustCompile(`\bprint[ \t]*\(`),
		regexp.MustCompile(`(?m)^[ \t]*(if|for|while|try|elif|else|except)\b[^\n{]*:[ \t]*$`),
		regexp.MustCompile(`\breleaseVariables\s*\[`),
		regexp.MustCompile(`(?m)^[ \t]*#[^!]`),
	}
)

// sniffLanguage guesses from content. It defaults to Python: a wrong Python
// guess surfaces as a visible parse error, whereas a wrong Groovy guess would
// silently drop a script from the report.
func sniffLanguage(source string) string {
	g := score(source, groovyMarkers)
	s := score(source, shellMarkers)
	p := score(source, pythonMarkers)

	switch {
	case g > p && g >= s:
		return langGroovy
	case s > p && s > g:
		return langShell
	default:
		return langPython
	}
}

func score(source string, markers []*regexp.Regexp) int {
	n := 0
	for _, re := range markers {
		if re.MatchString(source) {
			n++
		}
	}
	return n
}
