package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Script is one Python/Jython snippet found inside an export.
type Script struct {
	ID       string `json:"id"`
	Origin   string `json:"origin"`             // container file, relative to the scan root
	Location string `json:"location"`           // path to the field inside the container
	TaskType string `json:"taskType,omitempty"` // e.g. xlrelease.ScriptTask
	Title    string `json:"title,omitempty"`
	Field    string `json:"field"` // script | precondition
	Source   string `json:"-"`
}

// Note records something the scanner saw but could not analyse.
type Note struct {
	Origin  string `json:"origin"`
	Message string `json:"message"`
}

// Skipped is a script field that turned out not to hold Python. Release
// templates mix Jython, Groovy and shell in identically named fields, so these
// are reported rather than dropped.
type Skipped struct {
	Origin   string `json:"origin"`
	Location string `json:"location"`
	TaskType string `json:"taskType,omitempty"`
	Title    string `json:"title,omitempty"`
	Language string `json:"language"`
}

// scriptFields maps a lower-cased JSON/YAML key to the kind of script it holds.
// "external" means the field points at a script that lives outside the export.
var scriptFields = map[string]string{
	"script":         "script",
	"pythonscript":   "script",
	"jythonscript":   "script",
	"precondition":   "precondition",
	"scriptlocation": "external",
}

const maxArchiveDepth = 4

type extractor struct {
	fields     map[string]string
	scripts    []Script
	skipped    []Skipped
	notes      []Note
	containers int
	seenNote   map[string]bool
}

func newExtractor(extraKeys []string) *extractor {
	fields := make(map[string]string, len(scriptFields)+len(extraKeys))
	for k, v := range scriptFields {
		fields[k] = v
	}
	for _, k := range extraKeys {
		fields[strings.ToLower(k)] = "script"
	}
	return &extractor{fields: fields, seenNote: map[string]bool{}}
}

// Scan walks root (a directory or a single file) and collects every script it
// can reach.
func (e *extractor) Scan(root string, isDir bool) error {
	if !isDir {
		data, err := os.ReadFile(root)
		if err != nil {
			return err
		}
		e.containers++
		e.consume(filepath.Base(root), filepath.Ext(root), data, 0)
		e.finalise()
		return nil
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", ".idea", "__pycache__":
				return fs.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			e.note(rel, "unreadable: "+rerr.Error())
			return nil
		}
		e.containers++
		e.consume(rel, filepath.Ext(path), data, 0)
		return nil
	})
	if err != nil {
		return err
	}
	e.finalise()
	return nil
}

func (e *extractor) finalise() {
	for i := range e.scripts {
		e.scripts[i].ID = e.scripts[i].Origin + "#" + e.scripts[i].Location
	}
	sort.SliceStable(e.scripts, func(i, j int) bool {
		return e.scripts[i].ID < e.scripts[j].ID
	})
}

func (e *extractor) consume(origin, ext string, data []byte, depth int) {
	switch strings.ToLower(ext) {
	case ".xlr", ".zip", ".jar", ".dar":
		e.consumeZip(origin, data, depth)
	case ".json":
		e.consumeJSON(origin, data)
	case ".yaml", ".yml":
		e.consumeYAML(origin, string(data))
	case ".py", ".jy":
		e.add(Script{Origin: origin, Location: "<file>", Field: "script", Source: string(data)})
	case ".md", ".txt", ".xml", ".properties", ".png", ".jpg", ".gif", ".svg", ".class", ".so", ".dll":
		// Not script carriers; stay quiet about them.
	default:
		e.sniff(origin, data, depth)
	}
}

// sniff handles extensionless files, which XLR exports do contain.
func (e *extractor) sniff(origin string, data []byte, depth int) {
	trimmed := bytes.TrimSpace(data)
	switch {
	case len(trimmed) == 0:
		return
	case trimmed[0] == '{' || trimmed[0] == '[':
		e.consumeJSON(origin, data)
	case bytes.HasPrefix(trimmed, []byte("PK")):
		e.consumeZip(origin, data, depth)
	default:
		e.note(origin, "skipped (unrecognised file type)")
	}
}

func (e *extractor) consumeZip(origin string, data []byte, depth int) {
	if depth >= maxArchiveDepth {
		e.note(origin, "skipped (archive nested too deeply)")
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		e.note(origin, "not a readable archive: "+err.Error())
		return
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			e.note(origin+"!"+f.Name, "cannot open archive entry: "+err.Error())
			continue
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			e.note(origin+"!"+f.Name, "cannot read archive entry: "+err.Error())
			continue
		}
		e.consume(origin+"!"+f.Name, filepath.Ext(f.Name), body, depth+1)
	}
}

func (e *extractor) consumeJSON(origin string, data []byte) {
	var root interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		e.note(origin, "invalid JSON: "+err.Error())
		return
	}
	e.walkJSON(root, origin, "$", nodeCtx{})
}

type nodeCtx struct {
	TaskType string
	Title    string
}

func (e *extractor) walkJSON(node interface{}, origin, path string, ctx nodeCtx) {
	switch n := node.(type) {
	case map[string]interface{}:
		local := ctx
		ownType, _ := n["type"].(string)
		if ownType != "" {
			local.TaskType = ownType
		}
		if s, ok := n["title"].(string); ok && s != "" {
			local.Title = s
		} else if s, ok := n["name"].(string); ok && s != "" {
			local.Title = s
		}

		keys := make([]string, 0, len(n))
		for k := range n {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		inline := false
		for _, k := range keys {
			v := n[k]
			field, isScript := e.fields[strings.ToLower(k)]
			if isScript {
				s, ok := v.(string)
				if ok && strings.TrimSpace(s) != "" {
					if field == "external" {
						e.note(origin, fmt.Sprintf("%s points at an external script %q which is not part of the export", describe(local, path), s))
						continue
					}
					inline = true
					e.add(Script{
						Origin:   origin,
						Location: path + "." + k,
						TaskType: local.TaskType,
						Title:    local.Title,
						Field:    field,
						Source:   s,
					})
					continue
				}
			}
			e.walkJSON(v, origin, path+"."+k, local)
		}

		if !inline && isScriptTaskType(ownType) {
			e.note(origin, fmt.Sprintf("%s has no inline script (its logic lives in a plugin, not the export)", describe(local, path)))
		}

	case []interface{}:
		for i, v := range n {
			e.walkJSON(v, origin, fmt.Sprintf("%s[%d]", path, i), ctx)
		}
	}
}

func isScriptTaskType(t string) bool {
	if t == "" {
		return false
	}
	return strings.Contains(t, "ScriptTask") || strings.Contains(t, "PythonScript")
}

func describe(ctx nodeCtx, path string) string {
	switch {
	case ctx.Title != "" && ctx.TaskType != "":
		return fmt.Sprintf("%s %q", ctx.TaskType, ctx.Title)
	case ctx.Title != "":
		return fmt.Sprintf("task %q", ctx.Title)
	case ctx.TaskType != "":
		return ctx.TaskType
	default:
		return path
	}
}

var (
	// yamlBlockKey matches a YAML key introducing a block scalar, e.g. "script: |".
	yamlBlockKey = regexp.MustCompile(`^(\s*)(?:-\s+)?([A-Za-z_][\w-]*)\s*:\s*[|>][-+0-9]*\s*$`)
	yamlTypeKey  = regexp.MustCompile(`^\s*(?:-\s+)?type:\s*(\S+)\s*$`)
	yamlNameKey  = regexp.MustCompile(`^\s*(?:-\s+)?(?:name|title):\s*(.+?)\s*$`)
	yamlSeqItem  = regexp.MustCompile(`^(\s*)-\s`)

	// yamlInlineKey matches a key whose value is a single-line scalar, e.g.
	//     precondition: "releaseVariables[\"rollback\"]"
	// Release uses this form for one-line preconditions, so block-scalar-only
	// extraction silently loses them.
	yamlInlineKey = regexp.MustCompile(`^(\s*)(?:-\s+)?([A-Za-z_][\w-]*)\s*:\s*(\S.*?)\s*$`)
)

// unquoteYAMLScalar removes surrounding quotes and unescapes a single-line YAML
// scalar. Plain (unquoted) scalars are returned as-is.
func unquoteYAMLScalar(s string) string {
	if len(s) < 2 {
		return s
	}
	switch s[0] {
	case '"':
		if s[len(s)-1] != '"' {
			return s
		}
		inner := s[1 : len(s)-1]
		var b strings.Builder
		for i := 0; i < len(inner); i++ {
			if inner[i] == '\\' && i+1 < len(inner) {
				switch inner[i+1] {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				case 'r':
					b.WriteByte('\r')
				default:
					b.WriteByte(inner[i+1])
				}
				i++
				continue
			}
			b.WriteByte(inner[i])
		}
		return b.String()
	case '\'':
		if s[len(s)-1] != '\'' {
			return s
		}
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'")
	default:
		return s
	}
}

// consumeYAML pulls block scalars out of release-as-code YAML without taking on
// a YAML dependency. It only understands the block form (script: | ...), which
// is how the xl CLI writes scripts. It also tracks the sibling "type" key so
// Groovy and shell tasks are not mistaken for Jython.
func (e *extractor) consumeYAML(origin, text string) {
	lines := strings.Split(text, "\n")
	types := map[int]string{}
	titles := map[int]string{}
	found := 0

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// A new sequence item invalidates the sibling context recorded for the
		// previous item.
		if m := yamlSeqItem.FindStringSubmatch(line); m != nil {
			from := len(expandTabs(m[1])) + 1
			clearFrom(types, from)
			clearFrom(titles, from)
		}
		if m := yamlTypeKey.FindStringSubmatch(line); m != nil {
			types[yamlKeyIndent(line)] = strings.Trim(m[1], `"'`)
			continue
		}
		if m := yamlNameKey.FindStringSubmatch(line); m != nil {
			titles[yamlKeyIndent(line)] = strings.Trim(m[1], `"'`)
			continue
		}

		m := yamlBlockKey.FindStringSubmatch(line)
		if m == nil {
			// Not a block scalar. It may still be a one-line script field.
			e.tryYAMLInline(origin, line, i+1, types, titles, &found)
			continue
		}
		field, ok := e.fields[strings.ToLower(m[2])]
		if !ok || field == "external" {
			continue
		}
		bodyMinIndent := len(expandTabs(m[1]))
		keyCol := yamlKeyIndent(line)

		var body []string
		j := i + 1
		for ; j < len(lines); j++ {
			l := lines[j]
			if strings.TrimSpace(l) == "" {
				body = append(body, "")
				continue
			}
			if indentWidth(l) <= bodyMinIndent {
				break
			}
			body = append(body, expandTabs(l))
		}
		source := dedent(body)
		if strings.TrimSpace(source) != "" {
			found++
			e.add(Script{
				Origin:   origin,
				Location: fmt.Sprintf("line %d: %s", i+1, m[2]),
				TaskType: types[keyCol],
				Title:    titles[keyCol],
				Field:    field,
				Source:   source,
			})
		}
		i = j - 1
	}
	if found > 0 {
		e.note(origin, "YAML read with a best-effort block-scalar scanner; verify anything surprising by hand")
	}
}

// tryYAMLInline handles script fields written as a single-line scalar.
func (e *extractor) tryYAMLInline(origin, line string, lineNo int, types, titles map[int]string, found *int) {
	m := yamlInlineKey.FindStringSubmatch(line)
	if m == nil {
		return
	}
	field, ok := e.fields[strings.ToLower(m[2])]
	if !ok {
		return
	}
	value := unquoteYAMLScalar(m[3])
	if strings.TrimSpace(value) == "" {
		return
	}
	keyCol := yamlKeyIndent(line)

	if field == "external" {
		e.note(origin, fmt.Sprintf("line %d: %s points at an external script %q which is not part of the export", lineNo, m[2], value))
		return
	}
	*found++
	e.add(Script{
		Origin:   origin,
		Location: fmt.Sprintf("line %d: %s", lineNo, m[2]),
		TaskType: types[keyCol],
		Title:    titles[keyCol],
		Field:    field,
		Source:   value,
	})
}

// yamlKeyIndent returns the column at which a line's key begins, treating
// "- key: value" as a key nested inside the sequence item.
func yamlKeyIndent(line string) int {
	l := expandTabs(line)
	trimmed := strings.TrimLeft(l, " ")
	indent := len(l) - len(trimmed)
	if strings.HasPrefix(trimmed, "- ") {
		rest := strings.TrimLeft(trimmed[2:], " ")
		return indent + 2 + (len(trimmed[2:]) - len(rest))
	}
	return indent
}

func clearFrom(m map[int]string, indent int) {
	for k := range m {
		if k >= indent {
			delete(m, k)
		}
	}
}

func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "        ")
}

func indentWidth(line string) int {
	line = expandTabs(line)
	return len(line) - len(strings.TrimLeft(line, " "))
}

func dedent(lines []string) string {
	min := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		w := indentWidth(l)
		if min < 0 || w < min {
			min = w
		}
	}
	if min <= 0 {
		return strings.Join(lines, "\n")
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		if len(l) >= min {
			out[i] = l[min:]
		} else {
			out[i] = strings.TrimLeft(l, " ")
		}
	}
	return strings.Join(out, "\n")
}

func (e *extractor) add(s Script) {
	if strings.TrimSpace(s.Source) == "" {
		return
	}
	if lang := scriptLanguage(s.TaskType, s.Field, s.Source); lang != langPython {
		e.skipped = append(e.skipped, Skipped{
			Origin:   s.Origin,
			Location: s.Location,
			TaskType: s.TaskType,
			Title:    s.Title,
			Language: lang,
		})
		return
	}
	e.scripts = append(e.scripts, s)
}

func (e *extractor) note(origin, msg string) {
	key := origin + "|" + msg
	if e.seenNote[key] {
		return
	}
	e.seenNote[key] = true
	e.notes = append(e.notes, Note{Origin: origin, Message: msg})
}
