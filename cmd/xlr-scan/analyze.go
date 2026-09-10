package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/alvinjaison/pygo/internal/ast"
	"github.com/alvinjaison/pygo/internal/compiler"
	"github.com/alvinjaison/pygo/internal/object"
	"github.com/alvinjaison/pygo/internal/parser"
	"github.com/alvinjaison/pygo/internal/runtime"
)

// Verdicts, worst first.
const (
	verdictPython2  = "blocked: python 2 syntax"
	verdictSyntax   = "blocked: syntax PyGo cannot parse"
	verdictCompiler = "blocked: syntax PyGo cannot compile"
	verdictJVM      = "blocked: needs the JVM"
	verdictSemantic = "blocked: compiles, then fails or misbehaves at run time"
	verdictStdlib   = "needs stdlib modules"
	verdictBuiltins = "needs builtins or type methods"
	verdictHostAPI  = "ready once the host API is registered"
	verdictRuns     = "runs on PyGo today"
)

var verdictOrder = []string{
	verdictPython2, verdictSyntax, verdictCompiler, verdictJVM, verdictSemantic,
	verdictStdlib, verdictBuiltins, verdictHostAPI, verdictRuns,
}

// ImportRef is one import statement found in a script.
type ImportRef struct {
	Module     string   `json:"module"`
	Names      []string `json:"names,omitempty"`
	Line       int      `json:"line"`
	Category   string   `json:"category"`
	Available  bool     `json:"availableInPyGo"`
	Suggestion string   `json:"suggestion,omitempty"`
}

// Detection is one feature or Python 2 marker found in a script.
type Detection struct {
	Name      string `json:"name"`
	Lines     []int  `json:"lines"`
	Supported bool   `json:"supportedInPyGo"`
	Silent    bool   `json:"failsSilently,omitempty"`
	Fix       string `json:"fix,omitempty"`
}

// ScriptReport is the analysis of a single script.
type ScriptReport struct {
	Script
	Hash            string      `json:"hash"`
	Lines           int         `json:"lines"`
	Parses          bool        `json:"parses"`
	Compiles        bool        `json:"compiles"`
	ParseError      string      `json:"parseError,omitempty"`
	CompileError    string      `json:"compileError,omitempty"`
	Features        []Detection `json:"features,omitempty"`
	Python2         []Detection `json:"python2,omitempty"`
	Imports         []ImportRef `json:"imports,omitempty"`
	MissingBuiltins []string    `json:"missingBuiltins,omitempty"`
	MissingMethods  []string    `json:"missingMethods,omitempty"`
	HostNames       []string    `json:"hostNames,omitempty"`
	UnknownNames    []string    `json:"unknownNames,omitempty"`
	Verdict         string      `json:"verdict"`
	Blockers        []string    `json:"blockers,omitempty"`
}

// Stat is a frequency row: how many scripts and how many occurrences.
type Stat struct {
	Name        string `json:"name"`
	Scripts     int    `json:"scripts"`
	Occurrences int    `json:"occurrences"`
	Detail      string `json:"detail,omitempty"`
}

// Report is the full scan result.
type Report struct {
	Root              string         `json:"root"`
	Containers        int            `json:"containersScanned"`
	ScriptCount       int            `json:"scriptCount"`
	UniqueScripts     int            `json:"uniqueScripts"`
	NonPython         []Skipped      `json:"nonPythonScripts,omitempty"`
	Verdicts          map[string]int `json:"verdicts"`
	MissingFeatures   []Stat         `json:"missingFeatures"`
	MisbehavingFeats  []Stat         `json:"misbehavingFeatures"`
	SupportedFeatures []Stat         `json:"supportedFeatures"`
	ParseErrors       []Stat         `json:"parseErrors"`
	CompileErrors     []Stat         `json:"compileErrors"`
	MissingModules    []Stat         `json:"missingModules"`
	MissingBuiltins   []Stat         `json:"missingBuiltins"`
	MissingMethods    []Stat         `json:"missingMethods"`
	HostAPI           []Stat         `json:"hostApi"`
	UnknownNames      []Stat         `json:"unknownNames"`
	Python2           []Stat         `json:"python2"`
	AvailableBuiltins []string       `json:"pygoBuiltinsAvailable"`
	Scripts           []ScriptReport `json:"scripts"`
	RunResults        []RunResult    `json:"runResults,omitempty"`
	Notes             []Note         `json:"notes,omitempty"`
}

// ---------------------------------------------------------------------------
// probe: asks the live PyGo runtime what it can actually do
// ---------------------------------------------------------------------------

type probe struct {
	builtins map[string]bool
	features map[string]bool
	modules  map[string]bool
	methods  map[string]bool
}

func newProbe() *probe {
	p := &probe{
		builtins: map[string]bool{},
		features: map[string]bool{},
		modules:  map[string]bool{},
		methods:  map[string]bool{},
	}
	rt := runtime.New(runtime.WithOutput(io.Discard))
	for _, k := range rt.Builtins().Keys() {
		if s, ok := k.(*object.StrObject); ok {
			p.builtins[s.Value] = true
		}
	}
	for _, f := range featureDefs {
		p.features[f.Name] = p.runFeature(f)
	}
	return p
}

// runFeature reports whether PyGo handles a feature. Features with an expected
// output are executed and compared; the rest only need to parse and compile.
func (p *probe) runFeature(f featureDef) bool {
	mod, err := parser.Parse(f.Probe, "<probe>")
	if err != nil {
		return false
	}
	if _, err := compiler.Compile(mod); err != nil {
		return false
	}
	if f.Expect == "" {
		return true
	}
	var buf strings.Builder
	rt := runtime.New(runtime.WithOutput(&buf))
	if err := rt.ExecString(f.Probe, "<probe>"); err != nil {
		return false
	}
	return buf.String() == f.Expect
}

func (p *probe) supportsFeature(name string) bool { return p.features[name] }

func (p *probe) hasBuiltin(name string) bool { return p.builtins[name] }

// builtinReceivers are the literal forms used to test whether PyGo implements a
// method on any of its built-in types.
var builtinReceivers = []string{`''`, `[]`, `{}`, `(1,)`, `1`, `1.0`, `True`, `None`}

// hasMethodOnAnyBuiltin reports whether PyGo resolves the named attribute on at
// least one built-in type. Used to tell "str method PyGo has not implemented"
// apart from "method on a host-provided object".
func (p *probe) hasMethodOnAnyBuiltin(name string) bool {
	if v, ok := p.methods[name]; ok {
		return v
	}
	found := false
	for _, recv := range builtinReceivers {
		rt := runtime.New(runtime.WithOutput(io.Discard))
		if rt.ExecString(recv+"."+name+"\n", "<probe>") == nil {
			found = true
			break
		}
	}
	p.methods[name] = found
	return found
}

// hasModule reports whether "import module" succeeds on PyGo.
func (p *probe) hasModule(module string) bool {
	if v, ok := p.modules[module]; ok {
		return v
	}
	rt := runtime.New(runtime.WithOutput(io.Discard))
	ok := rt.ExecString("import "+module+"\n", "<probe>") == nil
	p.modules[module] = ok
	return ok
}

// ---------------------------------------------------------------------------
// analysis
// ---------------------------------------------------------------------------

// Analyse runs every script through PyGo and aggregates the results.
func Analyse(root string, scripts []Script, notes []Note, containers int, skipped []Skipped) *Report {
	p := newProbe()

	rep := &Report{
		Root:        root,
		Containers:  containers,
		ScriptCount: len(scripts),
		Verdicts:    map[string]int{},
		Notes:       notes,
		NonPython:   skipped,
	}
	for name := range p.builtins {
		rep.AvailableBuiltins = append(rep.AvailableBuiltins, name)
	}
	sort.Strings(rep.AvailableBuiltins)

	unique := map[string]bool{}
	agg := newAggregator()

	for _, s := range scripts {
		sr := analyseScript(s, p)
		rep.Scripts = append(rep.Scripts, sr)
		rep.Verdicts[sr.Verdict]++
		unique[sr.Hash] = true
		agg.add(sr)
	}
	rep.UniqueScripts = len(unique)

	rep.MissingFeatures = agg.missingFeatures.stats()
	rep.MisbehavingFeats = agg.misbehaving.stats()
	rep.SupportedFeatures = agg.supported.stats()
	rep.ParseErrors = agg.parseErrors.stats()
	rep.CompileErrors = agg.compileErrors.stats()
	rep.MissingModules = agg.missingModules.stats()
	rep.MissingBuiltins = agg.missingBuiltins.stats()
	rep.MissingMethods = agg.missingMethods.stats()
	rep.HostAPI = agg.hostAPI.stats()
	rep.UnknownNames = agg.unknownNames.stats()
	rep.Python2 = agg.python2.stats()
	return rep
}

func analyseScript(s Script, p *probe) ScriptReport {
	sum := sha256.Sum256([]byte(s.Source))
	sr := ScriptReport{
		Script: s,
		Hash:   hex.EncodeToString(sum[:8]),
		Lines:  strings.Count(strings.TrimRight(s.Source, "\n"), "\n") + 1,
	}

	stripped := stripLiterals(s.Source)
	rawIdx := newLineIndex(s.Source)
	strippedIdx := newLineIndex(stripped)

	// Feature detection, independent of whether the script parses.
	for _, f := range featureDefs {
		text, idx := stripped, strippedIdx
		if f.OnRaw {
			text, idx = s.Source, rawIdx
		}
		lines := matchLines(f.Detect, text, idx)
		if len(lines) == 0 {
			continue
		}
		sr.Features = append(sr.Features, Detection{
			Name:      f.Name,
			Lines:     lines,
			Supported: p.supportsFeature(f.Name),
			Silent:    f.Silent,
			Fix:       f.Fix,
		})
	}

	// Python 2 markers.
	for _, d := range py2Defs {
		text, idx := stripped, strippedIdx
		if d.OnRaw {
			text, idx = s.Source, rawIdx
		}
		lines := matchLines(d.Detect, text, idx)
		if len(lines) == 0 {
			continue
		}
		sr.Python2 = append(sr.Python2, Detection{Name: d.Name, Lines: lines, Fix: d.Fix})
	}

	// Parse and compile with PyGo itself.
	mod, perr := parser.Parse(s.Source, s.ID)
	if perr != nil {
		sr.ParseError = cleanError(perr.Error(), s.ID)
	} else {
		sr.Parses = true
		if _, cerr := compiler.Compile(mod); cerr != nil {
			sr.CompileError = cleanError(cerr.Error(), s.ID)
		} else {
			sr.Compiles = true
		}
	}

	// Imports, free names and attribute lookups.
	var imports []ImportRef
	var attrs []attrUse
	var loads, binds map[string]bool
	if sr.Parses {
		ns := &nameScan{loads: map[string]bool{}, binds: map[string]bool{}}
		ast.Walk(ns, mod)
		imports = ns.imports
		attrs = ns.attrs
		loads, binds = ns.loads, ns.binds
	} else {
		imports = fallbackImports(stripped, strippedIdx)
		loads, binds = map[string]bool{}, map[string]bool{}
	}

	for i := range imports {
		cat, sug := classifyModule(imports[i].Module)
		imports[i].Category = cat
		imports[i].Suggestion = sug
		imports[i].Available = p.hasModule(imports[i].Module)
	}
	sr.Imports = imports

	// Free names: loaded but never bound in this script.
	var free []string
	for name := range loads {
		if binds[name] {
			continue
		}
		free = append(free, name)
	}
	sort.Strings(free)

	for _, name := range free {
		switch {
		case p.hasBuiltin(name):
			// available already
		case knownPythonBuiltins[name]:
			sr.MissingBuiltins = append(sr.MissingBuiltins, name)
		case releaseAPINames[name] != "":
			sr.HostNames = append(sr.HostNames, name)
		default:
			sr.UnknownNames = append(sr.UnknownNames, name)
		}
	}

	// Method surface: which method lookups would fail on PyGo's built-in types.
	// Lookups on host objects and on imported modules are somebody else's
	// problem, so they are excluded.
	external := map[string]bool{}
	for _, n := range sr.HostNames {
		external[n] = true
	}
	for _, n := range sr.UnknownNames {
		external[n] = true
	}
	for _, im := range imports {
		external[topLevel(im.Module)] = true
		for _, n := range im.Names {
			external[n] = true
		}
	}
	seenAttr := map[string]bool{}
	for _, use := range attrs {
		if use.RecvName != "" && external[use.RecvName] {
			continue
		}
		if seenAttr[use.Attr] || p.hasMethodOnAnyBuiltin(use.Attr) {
			continue
		}
		seenAttr[use.Attr] = true
		sr.MissingMethods = append(sr.MissingMethods, use.Attr)
	}
	sort.Strings(sr.MissingMethods)

	sr.Verdict, sr.Blockers = verdict(&sr)
	return sr
}

func verdict(sr *ScriptReport) (string, []string) {
	var blockers []string

	if len(sr.Python2) > 0 {
		names := make([]string, 0, len(sr.Python2))
		for _, d := range sr.Python2 {
			names = append(names, d.Name)
		}
		blockers = append(blockers, "python 2: "+strings.Join(names, ", "))
	}

	var missingFeats, misbehaving []string
	for _, f := range sr.Features {
		if f.Supported {
			continue
		}
		if !isSemanticFeature(f.Name) {
			missingFeats = append(missingFeats, f.Name)
			continue
		}
		if f.Silent {
			misbehaving = append(misbehaving, f.Name+" (fails silently)")
		} else {
			misbehaving = append(misbehaving, f.Name)
		}
	}
	if len(missingFeats) > 0 {
		blockers = append(blockers, "unsupported syntax: "+strings.Join(missingFeats, ", "))
	}
	if len(misbehaving) > 0 {
		blockers = append(blockers, "run-time gap: "+strings.Join(misbehaving, ", "))
	}

	var jvm, missingMods []string
	for _, im := range sr.Imports {
		if im.Category == catJVM {
			jvm = append(jvm, im.Module)
			continue
		}
		if !im.Available {
			missingMods = append(missingMods, im.Module)
		}
	}
	if len(jvm) > 0 {
		blockers = append(blockers, "jvm imports: "+strings.Join(dedupe(jvm), ", "))
	}
	if len(missingMods) > 0 {
		blockers = append(blockers, "missing modules: "+strings.Join(dedupe(missingMods), ", "))
	}
	if len(sr.MissingBuiltins) > 0 {
		blockers = append(blockers, "missing builtins: "+strings.Join(sr.MissingBuiltins, ", "))
	}
	if len(sr.MissingMethods) > 0 {
		blockers = append(blockers, "missing methods: "+strings.Join(sr.MissingMethods, ", "))
	}
	if sr.ParseError != "" {
		blockers = append(blockers, "parse: "+sr.ParseError)
	}
	if sr.CompileError != "" {
		blockers = append(blockers, "compile: "+sr.CompileError)
	}

	switch {
	case len(sr.Python2) > 0:
		return verdictPython2, blockers
	case !sr.Parses:
		return verdictSyntax, blockers
	case !sr.Compiles:
		return verdictCompiler, blockers
	case len(jvm) > 0:
		return verdictJVM, blockers
	case len(misbehaving) > 0:
		return verdictSemantic, blockers
	case len(missingMods) > 0:
		return verdictStdlib, blockers
	case len(sr.MissingBuiltins) > 0, len(sr.MissingMethods) > 0:
		return verdictBuiltins, blockers
	case len(sr.HostNames) > 0 || len(sr.UnknownNames) > 0:
		return verdictHostAPI, blockers
	default:
		return verdictRuns, blockers
	}
}

// isSemanticFeature reports whether a missing feature fails silently (produces
// wrong output) rather than loudly (fails to parse or compile).
func isSemanticFeature(name string) bool {
	switch name {
	case "f-string interpolation", "printf-style % formatting", "str.format()":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// AST name and import collection
// ---------------------------------------------------------------------------

type nameScan struct {
	loads   map[string]bool
	binds   map[string]bool
	imports []ImportRef
	attrs   []attrUse
}

// attrUse is one attribute or method lookup, with the receiver's name when the
// receiver is a plain identifier.
type attrUse struct {
	Attr     string
	RecvName string
	Line     int
}

func (v *nameScan) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.Name:
		if n.Ctx == ast.Load {
			v.loads[n.Id] = true
		} else {
			v.binds[n.Id] = true
		}

	case *ast.FunctionDef:
		v.binds[n.Name] = true
		v.bindArgs(n.Args)

	case *ast.ClassDef:
		v.binds[n.Name] = true

	case *ast.Attribute:
		if n.Ctx == ast.Load {
			use := attrUse{Attr: n.Attr, Line: n.Pos().Line}
			if recv, ok := n.Value.(*ast.Name); ok {
				use.RecvName = recv.Id
			}
			v.attrs = append(v.attrs, use)
		}

	// PyGo's parser marks assignment targets with ast.Load, so bindings have to
	// be read off the statement rather than off Name.Ctx.
	case *ast.Assign:
		for _, t := range n.Targets {
			v.bindTarget(t)
		}

	case *ast.AugAssign:
		v.bindTarget(n.Target)

	case *ast.AnnAssign:
		v.bindTarget(n.Target)

	case *ast.For:
		v.bindTarget(n.Target)

	case *ast.With:
		for _, item := range n.Items {
			if item != nil && item.OptionalVar != nil {
				v.bindTarget(item.OptionalVar)
			}
		}

	case *ast.ExceptHandler:
		if n.Name != "" {
			v.binds[n.Name] = true
		}

	case *ast.Global:
		for _, name := range n.Names {
			v.binds[name] = true
		}

	case *ast.Nonlocal:
		for _, name := range n.Names {
			v.binds[name] = true
		}

	case *ast.Import:
		for _, a := range n.Names {
			v.imports = append(v.imports, ImportRef{Module: a.Name, Line: n.Pos().Line})
			if a.AsName != "" {
				v.binds[a.AsName] = true
			} else {
				v.binds[topLevel(a.Name)] = true
			}
		}

	case *ast.ImportFrom:
		ref := ImportRef{Module: n.Module, Line: n.Pos().Line}
		for _, a := range n.Names {
			ref.Names = append(ref.Names, a.Name)
			if a.AsName != "" {
				v.binds[a.AsName] = true
			} else {
				v.binds[a.Name] = true
			}
		}
		v.imports = append(v.imports, ref)
	}
	return v
}

// bindTarget records the names an assignment target introduces. Attribute and
// subscript targets bind nothing: "x.y = 1" still requires x to already exist.
func (v *nameScan) bindTarget(target ast.Expr) {
	switch t := target.(type) {
	case *ast.Name:
		v.binds[t.Id] = true
	case *ast.Tuple:
		for _, e := range t.Elts {
			v.bindTarget(e)
		}
	case *ast.List:
		for _, e := range t.Elts {
			v.bindTarget(e)
		}
	case *ast.Starred:
		v.bindTarget(t.Value)
	}
}

func (v *nameScan) bindArgs(args *ast.Arguments) {
	if args == nil {
		return
	}
	for _, group := range [][]*ast.Arg{args.PosonlyArgs, args.Args, args.KwonlyArgs} {
		for _, a := range group {
			if a != nil {
				v.binds[a.Arg] = true
			}
		}
	}
	if args.Vararg != nil {
		v.binds[args.Vararg.Arg] = true
	}
	if args.Kwarg != nil {
		v.binds[args.Kwarg.Arg] = true
	}
}

// fallbackImports recovers import statements from scripts PyGo cannot parse.
var (
	reImport     = regexp.MustCompile(`(?m)^[ \t]*import[ \t]+([\w.]+(?:[ \t]+as[ \t]+\w+)?(?:[ \t]*,[ \t]*[\w.]+(?:[ \t]+as[ \t]+\w+)?)*)`)
	reFromImport = regexp.MustCompile(`(?m)^[ \t]*from[ \t]+([\w.]+)[ \t]+import[ \t]+([\w.*, \t]+)`)
)

func fallbackImports(stripped string, idx lineIndex) []ImportRef {
	var out []ImportRef
	for _, m := range reImport.FindAllStringSubmatchIndex(stripped, -1) {
		line := idx.line(m[0])
		clause := stripped[m[2]:m[3]]
		for _, part := range strings.Split(clause, ",") {
			name := strings.TrimSpace(part)
			if i := strings.Index(name, " as "); i >= 0 {
				name = strings.TrimSpace(name[:i])
			}
			name = strings.Fields(name + " ")[0]
			if name != "" {
				out = append(out, ImportRef{Module: name, Line: line})
			}
		}
	}
	for _, m := range reFromImport.FindAllStringSubmatchIndex(stripped, -1) {
		line := idx.line(m[0])
		ref := ImportRef{Module: strings.TrimSpace(stripped[m[2]:m[3]]), Line: line}
		for _, part := range strings.Split(stripped[m[4]:m[5]], ",") {
			if name := strings.TrimSpace(part); name != "" {
				ref.Names = append(ref.Names, name)
			}
		}
		out = append(out, ref)
	}
	return out
}

// ---------------------------------------------------------------------------
// aggregation
// ---------------------------------------------------------------------------

type counter struct {
	scripts     map[string]int
	occurrences map[string]int
	detail      map[string]string
}

func newCounter() *counter {
	return &counter{
		scripts:     map[string]int{},
		occurrences: map[string]int{},
		detail:      map[string]string{},
	}
}

func (c *counter) hit(name string, occurrences int, detail string) {
	c.scripts[name]++
	c.occurrences[name] += occurrences
	if detail != "" {
		c.detail[name] = detail
	}
}

func (c *counter) stats() []Stat {
	out := make([]Stat, 0, len(c.scripts))
	for name, n := range c.scripts {
		out = append(out, Stat{
			Name:        name,
			Scripts:     n,
			Occurrences: c.occurrences[name],
			Detail:      c.detail[name],
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Scripts != out[j].Scripts {
			return out[i].Scripts > out[j].Scripts
		}
		return out[i].Name < out[j].Name
	})
	return out
}

type aggregator struct {
	missingFeatures *counter
	misbehaving     *counter
	supported       *counter
	parseErrors     *counter
	compileErrors   *counter
	missingModules  *counter
	missingBuiltins *counter
	missingMethods  *counter
	hostAPI         *counter
	unknownNames    *counter
	python2         *counter
}

func newAggregator() *aggregator {
	return &aggregator{
		missingFeatures: newCounter(),
		misbehaving:     newCounter(),
		supported:       newCounter(),
		parseErrors:     newCounter(),
		compileErrors:   newCounter(),
		missingModules:  newCounter(),
		missingBuiltins: newCounter(),
		missingMethods:  newCounter(),
		hostAPI:         newCounter(),
		unknownNames:    newCounter(),
		python2:         newCounter(),
	}
}

func (a *aggregator) add(sr ScriptReport) {
	for _, f := range sr.Features {
		switch {
		case f.Supported:
			a.supported.hit(f.Name, len(f.Lines), "")
		case isSemanticFeature(f.Name):
			name := f.Name
			if f.Silent {
				name += " (fails silently)"
			}
			a.misbehaving.hit(name, len(f.Lines), f.Fix)
		default:
			a.missingFeatures.hit(f.Name, len(f.Lines), f.Fix)
		}
	}
	for _, d := range sr.Python2 {
		a.python2.hit(d.Name, len(d.Lines), d.Fix)
	}
	if sr.ParseError != "" {
		a.parseErrors.hit(normaliseError(sr.ParseError), 1, "")
	}
	if sr.CompileError != "" {
		a.compileErrors.hit(normaliseError(sr.CompileError), 1, "")
	}
	seenMod := map[string]bool{}
	for _, im := range sr.Imports {
		if im.Available || seenMod[im.Module] {
			continue
		}
		seenMod[im.Module] = true
		a.missingModules.hit(im.Module, 1, im.Category+" - "+im.Suggestion)
	}
	for _, b := range sr.MissingBuiltins {
		a.missingBuiltins.hit(b, 1, "internal/runtime/builtins.go")
	}
	for _, m := range sr.MissingMethods {
		a.missingMethods.hit(m, 1, "internal/object (string_methods.go / collection_methods.go)")
	}
	for _, h := range sr.HostNames {
		a.hostAPI.hit(h, 1, releaseAPINames[h])
	}
	for _, u := range sr.UnknownNames {
		a.unknownNames.hit(u, 1, "")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// cleanError strips the script id prefix so error text aggregates cleanly.
func cleanError(msg, id string) string {
	msg = strings.ReplaceAll(msg, id+":", "")
	return strings.TrimSpace(msg)
}

var reLineCol = regexp.MustCompile(`\b\d+:\d+\b`)
var reQuoted = regexp.MustCompile(`"[^"]*"`)

// normaliseError removes positions and quoted identifiers so that the same
// underlying gap groups into one row.
func normaliseError(msg string) string {
	msg = reLineCol.ReplaceAllString(msg, "L:C")
	msg = reQuoted.ReplaceAllString(msg, `"..."`)
	msg = strings.TrimPrefix(msg, "L:C:")
	if i := strings.Index(msg, " at L:C"); i >= 0 {
		msg = msg[:i]
	}
	return strings.TrimSpace(msg)
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func pct(n, total int) string {
	if total == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.0f%%", float64(n)*100/float64(total))
}
