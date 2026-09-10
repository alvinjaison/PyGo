package main

import (
	"regexp"
	"sort"
	"strings"
)

// featureDef describes one syntax feature the scanner looks for.
//
// Detect finds the feature in a script. Probe is a snippet used to ask the live
// PyGo runtime whether the feature works, so the report reflects PyGo as it is
// today rather than a hard-coded assumption. When Expect is non-empty the probe
// is executed and its stdout compared, which catches features that parse and
// compile but do the wrong thing at runtime.
type featureDef struct {
	Name   string
	Detect *regexp.Regexp
	OnRaw  bool // match against the raw source instead of the literal-stripped copy
	Probe  string
	Expect string
	Silent bool // when unsupported, the script still runs and returns wrong data
	Fix    string
}

var featureDefs = []featureDef{
	{
		Name:   "class definitions",
		Detect: regexp.MustCompile(`(?m)^[ \t]*class[ \t]+\w+`),
		Probe:  "class A(object):\n    def m(self):\n        return 1\n",
		Fix:    "parser (parseClassDef), compiler (ClassDef case), object model (user-defined types)",
	},
	{
		Name:   "with statement",
		Detect: regexp.MustCompile(`(?m)^[ \t]*with[ \t]+\S`),
		Probe:  "with a as b:\n    pass\n",
		Fix:    "parser (parseWithStmt), compiler (SETUP_WITH), object protocol (__enter__/__exit__)",
	},
	{
		Name:   "lambda",
		Detect: regexp.MustCompile(`\blambda\b`),
		Probe:  "f = lambda x: x + 1\n",
		Fix:    "parser (parseLambda), compiler (reuse function scope machinery)",
	},
	{
		Name:   "list comprehension",
		Detect: regexp.MustCompile(`(?s)\[[^\[\]]*\bfor\b[^\[\]]*\bin\b`),
		Probe:  "y = [i for i in (1, 2) if i]\n",
		Fix:    "parser (comprehension clauses), compiler (implicit comprehension scope)",
	},
	{
		Name:   "dict/set comprehension",
		Detect: regexp.MustCompile(`(?s)\{[^{}]*\bfor\b[^{}]*\bin\b`),
		Probe:  "y = {k: 1 for k in (1, 2)}\n",
		Fix:    "parser (comprehension clauses), compiler (implicit comprehension scope)",
	},
	{
		Name:   "generator expression",
		Detect: regexp.MustCompile(`(?s)\([^()]*\bfor\b[^()]*\bin\b`),
		Probe:  "y = sum(i for i in (1, 2))\n",
		Fix:    "parser (comprehension clauses), compiler + VM (generator frames)",
	},
	{
		Name:   "generators (yield)",
		Detect: regexp.MustCompile(`\byield\b`),
		Probe:  "def g():\n    yield 1\n",
		Fix:    "compiler (YIELD_VALUE), VM (suspendable frames)",
	},
	{
		Name:   "assert",
		Detect: regexp.MustCompile(`(?m)^[ \t]*assert\b`),
		Probe:  "assert True\n",
		Fix:    "parser (parseAssert), compiler (raise AssertionError)",
	},
	{
		Name:   "f-string interpolation",
		Detect: regexp.MustCompile(`(?i)(^|[^\w'"])(?:f|rf|fr)['"]`),
		OnRaw:  true,
		Probe:  "x = 41\nprint(f\"v={x + 1}\")\n",
		Expect: "v=42\n",
		Silent: true, // PyGo currently prints the f-string body verbatim
		Fix:    "lexer (f-string scanning), parser (JoinedStr/FormattedValue), compiler (FORMAT_VALUE)",
	},
	{
		Name:   "printf-style % formatting",
		Detect: regexp.MustCompile(`(?s)['"][^'"\n]*%[-#0-9.*]*[sdifrxXeEgGc][^'"\n]*['"][ \t]*%`),
		OnRaw:  true,
		Probe:  "print(\"%s-%d\" % (\"a\", 2))\n",
		Expect: "a-2\n",
		Fix:    "object model (str.__mod__)",
	},
	{
		Name:   "str.format()",
		Detect: regexp.MustCompile(`\.format\s*\(`),
		Probe:  "print(\"{0}/{1}\".format(\"a\", 2))\n",
		Expect: "a/2\n",
		Fix:    "object model (str.format)",
	},
	{
		Name:   "try/except/finally",
		Detect: regexp.MustCompile(`(?m)^[ \t]*except\b`),
		Probe:  "try:\n    pass\nexcept Exception as e:\n    pass\nfinally:\n    pass\n",
		Fix:    "already supported",
	},
	{
		Name:   "decorators",
		Detect: regexp.MustCompile(`(?m)^[ \t]*@\w`),
		Probe:  "def d(f):\n    return f\n\n@d\ndef g():\n    pass\n",
		Fix:    "already supported for functions; classes need ClassDef first",
	},
	{
		Name:   "async/await",
		Detect: regexp.MustCompile(`(?m)^[ \t]*(async[ \t]+def|await[ \t]+)`),
		Probe:  "async def f():\n    pass\n",
		Fix:    "out of scope for now (VISION non-goal ordering); reject with a clear error",
	},
	{
		Name:   "star-args in calls or defs",
		Detect: regexp.MustCompile(`\(\s*\*\*?\w`),
		Probe:  "def f(*a, **k):\n    return len(a) + len(k)\n\nf(*[1], **{})\n",
		Fix:    "already supported",
	},
}

// py2Def describes a Python-2-only construct. These are script-side problems:
// no amount of PyGo work makes them run on a Python 3 runtime.
type py2Def struct {
	Name   string
	Detect *regexp.Regexp
	OnRaw  bool
	Fix    string
}

var py2Defs = []py2Def{
	{
		Name:   "print statement",
		Detect: regexp.MustCompile(`(?m)^[ \t]*print([ \t]+[^(=\n]|[ \t]*>>|[ \t]*$)`),
		Fix:    "print(...)",
	},
	{
		Name:   "except X, e:",
		Detect: regexp.MustCompile(`(?m)^[ \t]*except[ \t]+[^\n:]*,[ \t]*\w+[ \t]*:`),
		Fix:    "except X as e:",
	},
	{
		Name:   "raise X, msg",
		Detect: regexp.MustCompile(`(?m)^[ \t]*raise[ \t]+[\w.]+[ \t]*,`),
		Fix:    "raise X(msg)",
	},
	{
		Name:   "dict.iteritems/iterkeys/itervalues",
		Detect: regexp.MustCompile(`\.(iteritems|iterkeys|itervalues)\s*\(`),
		Fix:    "items()/keys()/values()",
	},
	{
		Name:   "dict.has_key()",
		Detect: regexp.MustCompile(`\.has_key\s*\(`),
		Fix:    "key in dict",
	},
	{
		Name:   "unicode()",
		Detect: regexp.MustCompile(`\bunicode\s*\(`),
		Fix:    "str()",
	},
	{
		Name:   "basestring",
		Detect: regexp.MustCompile(`\bbasestring\b`),
		Fix:    "str",
	},
	{
		Name:   "xrange()",
		Detect: regexp.MustCompile(`\bxrange\s*\(`),
		Fix:    "range()",
	},
	{
		Name:   "raw_input()",
		Detect: regexp.MustCompile(`\braw_input\s*\(`),
		Fix:    "input()",
	},
	{
		Name:   "long()",
		Detect: regexp.MustCompile(`\blong\s*\(`),
		Fix:    "int()",
	},
	{
		Name:   "cmp()/reduce()/apply()/execfile()",
		Detect: regexp.MustCompile(`\b(cmp|reduce|apply|execfile|unichr)\s*\(`),
		Fix:    "rewrite (reduce moved to functools)",
	},
	{
		Name:   "backtick repr",
		Detect: regexp.MustCompile("`"),
		Fix:    "repr(x)",
	},
	{
		Name:   "<> inequality",
		Detect: regexp.MustCompile(`<>`),
		Fix:    "!=",
	},
	{
		Name:   "exec statement",
		Detect: regexp.MustCompile(`(?m)^[ \t]*exec[ \t]+[^(]`),
		Fix:    "exec(...) - and consider removing it entirely",
	},
	{
		Name:   "octal literal 0NNN",
		Detect: regexp.MustCompile(`(?m)(^|[^\w.])0[0-7]{2,}\b`),
		Fix:    "0oNNN",
	},
	{
		Name:   "integer literal suffix L",
		Detect: regexp.MustCompile(`\b\d+[lL]\b`),
		Fix:    "drop the suffix",
	},
}

// stripLiterals blanks out string contents and comments while preserving byte
// offsets and newlines, so regex probes do not fire on prose inside strings and
// reported line numbers stay correct.
func stripLiterals(src string) string {
	b := []byte(src)
	out := make([]byte, len(b))
	copy(out, b)

	blank := func(i int) {
		if out[i] != '\n' {
			out[i] = ' '
		}
	}

	i := 0
	for i < len(b) {
		c := b[i]
		switch c {
		case '#':
			for i < len(b) && b[i] != '\n' {
				blank(i)
				i++
			}
		case '\'', '"':
			q := c
			n := 1
			if i+2 < len(b) && b[i+1] == q && b[i+2] == q {
				n = 3
			}
			for k := 0; k < n; k++ {
				blank(i + k)
			}
			i += n
			for i < len(b) {
				if b[i] == '\\' && i+1 < len(b) {
					blank(i)
					blank(i + 1)
					i += 2
					continue
				}
				if b[i] == q {
					if n == 1 {
						blank(i)
						i++
						break
					}
					if i+2 < len(b) && b[i+1] == q && b[i+2] == q {
						blank(i)
						blank(i + 1)
						blank(i + 2)
						i += 3
						break
					}
				}
				if n == 1 && b[i] == '\n' {
					i++ // unterminated single-quoted string; stop at end of line
					break
				}
				blank(i)
				i++
			}
		default:
			i++
		}
	}
	return string(out)
}

// lineIndex converts byte offsets into 1-indexed line numbers.
type lineIndex []int

func newLineIndex(s string) lineIndex {
	starts := []int{0}
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

func (li lineIndex) line(offset int) int {
	n := sort.SearchInts(li, offset+1)
	if n < 1 {
		return 1
	}
	return n
}

// matchLines returns the line numbers of every match, de-duplicated.
func matchLines(re *regexp.Regexp, text string, li lineIndex) []int {
	locs := re.FindAllStringIndex(text, -1)
	if locs == nil {
		return nil
	}
	seen := map[int]bool{}
	var lines []int
	for _, loc := range locs {
		l := li.line(loc[0])
		if !seen[l] {
			seen[l] = true
			lines = append(lines, l)
		}
	}
	sort.Ints(lines)
	return lines
}

// ---------------------------------------------------------------------------
// module classification
// ---------------------------------------------------------------------------

const (
	catStdlib = "python-stdlib"
	catPy2    = "python2-only-stdlib"
	catJVM    = "jvm"
	catHost   = "release-host"
	catThird  = "third-party"
)

var jvmPrefixes = []string{"java", "javax", "org", "com", "net", "jarray", "groovy", "sun", "scala"}

var hostPrefixes = []string{"xlrelease", "xl_release", "xlr", "xldeploy", "com.xebialabs", "nl.javadude"}

var py2OnlyModules = map[string]string{
	"urllib2":          "urllib.request / requests",
	"urlparse":         "urllib.parse",
	"httplib":          "http.client",
	"StringIO":         "io.StringIO",
	"cStringIO":        "io.StringIO",
	"cPickle":          "pickle",
	"ConfigParser":     "configparser",
	"Queue":            "queue",
	"commands":         "subprocess",
	"HTMLParser":       "html.parser",
	"Cookie":           "http.cookies",
	"cookielib":        "http.cookiejar",
	"SocketServer":     "socketserver",
	"SimpleHTTPServer": "http.server",
	"BaseHTTPServer":   "http.server",
	"CGIHTTPServer":    "http.server",
	"xmlrpclib":        "xmlrpc.client",
	"__builtin__":      "builtins",
	"thread":           "threading",
	"copy_reg":         "copyreg",
	"robotparser":      "urllib.robotparser",
	"repr":             "reprlib",
	"Tkinter":          "tkinter",
	"md5":              "hashlib",
	"sha":              "hashlib",
	"sets":             "the built-in set type",
	"dummy_thread":     "_thread",
	"exceptions":       "built-in exceptions",
	"whichdb":          "dbm",
	"anydbm":           "dbm",
	"itertools.izip":   "zip",
	"UserDict":         "collections",
	"mimetools":        "email",
	"rfc822":           "email",
	"urllib.quote":     "urllib.parse.quote",
	"new":              "types",
	"string.maketrans": "str.maketrans",
	"platform.dist":    "distro or platform",
}

// stdlibModules lists top-level standard library modules that a Release script
// might plausibly reach for. It does not need to be exhaustive; anything not
// listed and not JVM/host is reported as third-party, which is also actionable.
var stdlibModules = map[string]bool{
	"abc": true, "argparse": true, "array": true, "ast": true, "base64": true,
	"binascii": true, "bisect": true, "calendar": true, "cgi": true, "cmath": true,
	"codecs": true, "collections": true, "configparser": true, "contextlib": true,
	"copy": true, "csv": true, "ctypes": true, "datetime": true, "decimal": true,
	"difflib": true, "dis": true, "email": true, "enum": true, "errno": true,
	"filecmp": true, "fileinput": true, "fnmatch": true, "fractions": true,
	"ftplib": true, "functools": true, "getpass": true, "gettext": true, "glob": true,
	"gzip": true, "hashlib": true, "heapq": true, "hmac": true, "html": true,
	"http": true, "imaplib": true, "importlib": true, "inspect": true, "io": true,
	"ipaddress": true, "itertools": true, "json": true, "keyword": true, "locale": true,
	"logging": true, "math": true, "mimetypes": true, "multiprocessing": true,
	"operator": true, "os": true, "pathlib": true, "pickle": true, "pipes": true,
	"pkgutil": true, "platform": true, "poplib": true, "posixpath": true,
	"pprint": true, "queue": true, "random": true, "re": true, "select": true,
	"shlex": true, "shutil": true, "signal": true, "smtplib": true, "socket": true,
	"socketserver": true, "sqlite3": true, "ssl": true, "stat": true, "string": true,
	"stringprep": true, "struct": true, "subprocess": true, "sys": true, "tarfile": true,
	"tempfile": true, "textwrap": true, "threading": true, "time": true, "timeit": true,
	"traceback": true, "types": true, "typing": true, "unicodedata": true,
	"unittest": true, "urllib": true, "uuid": true, "warnings": true, "weakref": true,
	"xml": true, "xmlrpc": true, "zipfile": true, "zlib": true,
}

func topLevel(module string) string {
	if i := strings.Index(module, "."); i >= 0 {
		return module[:i]
	}
	return module
}

func classifyModule(module string) (category, suggestion string) {
	lower := strings.ToLower(module)
	for _, p := range hostPrefixes {
		if lower == p || strings.HasPrefix(lower, p+".") {
			return catHost, "implement in Go and expose via rt.RegisterModule"
		}
	}
	top := topLevel(module)
	for _, p := range jvmPrefixes {
		if top == p {
			return catJVM, "no JVM in PyGo; replace with a Go-backed equivalent or reject the script"
		}
	}
	if repl, ok := py2OnlyModules[module]; ok {
		return catPy2, "Python 3 equivalent: " + repl
	}
	if repl, ok := py2OnlyModules[top]; ok {
		return catPy2, "Python 3 equivalent: " + repl
	}
	if stdlibModules[top] {
		return catStdlib, "implement in internal/stdlib/" + top
	}
	return catThird, "pure-Python package, or replace with a Go-backed module"
}

// knownPythonBuiltins separates "a builtin PyGo has not implemented yet" from
// "a name the Release host injects".
var knownPythonBuiltins = map[string]bool{
	"abs": true, "aiter": true, "all": true, "anext": true, "any": true, "ascii": true,
	"bin": true, "bool": true, "breakpoint": true, "bytearray": true, "bytes": true,
	"callable": true, "chr": true, "classmethod": true, "compile": true, "complex": true,
	"delattr": true, "dict": true, "dir": true, "divmod": true, "enumerate": true,
	"eval": true, "exec": true, "filter": true, "float": true, "format": true,
	"frozenset": true, "getattr": true, "globals": true, "hasattr": true, "hash": true,
	"help": true, "hex": true, "id": true, "input": true, "int": true, "isinstance": true,
	"issubclass": true, "iter": true, "len": true, "list": true, "locals": true,
	"map": true, "max": true, "memoryview": true, "min": true, "next": true,
	"object": true, "oct": true, "open": true, "ord": true, "pow": true, "print": true,
	"property": true, "range": true, "repr": true, "reversed": true, "round": true,
	"set": true, "setattr": true, "slice": true, "sorted": true, "staticmethod": true,
	"str": true, "sum": true, "super": true, "tuple": true, "type": true, "vars": true,
	"zip": true, "__import__": true, "__name__": true, "__file__": true, "__doc__": true,
	"True": true, "False": true, "None": true, "NotImplemented": true, "Ellipsis": true,
	// exceptions
	"BaseException": true, "Exception": true, "ArithmeticError": true,
	"AssertionError": true, "AttributeError": true, "EOFError": true,
	"FileNotFoundError": true, "FloatingPointError": true, "ImportError": true,
	"IndentationError": true, "IndexError": true, "IOError": true, "KeyError": true,
	"KeyboardInterrupt": true, "LookupError": true, "MemoryError": true,
	"NameError": true, "NotImplementedError": true, "OSError": true,
	"OverflowError": true, "RecursionError": true, "ReferenceError": true,
	"RuntimeError": true, "StopIteration": true, "SyntaxError": true,
	"SystemError": true, "SystemExit": true, "TabError": true, "TypeError": true,
	"UnboundLocalError": true, "UnicodeDecodeError": true, "UnicodeEncodeError": true,
	"UnicodeError": true, "ValueError": true, "ZeroDivisionError": true,
	"Warning": true, "DeprecationWarning": true, "UserWarning": true,
	// python 2 leftovers, flagged elsewhere but still not host names
	"basestring": true, "unicode": true, "xrange": true, "raw_input": true,
	"long": true, "cmp": true, "reduce": true, "apply": true, "execfile": true,
	"unichr": true, "StandardError": true, "file": true,
}

// releaseAPINames are the globals Digital.ai Release injects into Jython script
// tasks. Anything here is work for your host application, not for PyGo.
var releaseAPINames = map[string]string{
	"releaseApi":        "Release API",
	"taskApi":           "Release API",
	"phaseApi":          "Release API",
	"userApi":           "Release API",
	"roleApi":           "Release API",
	"teamApi":           "Release API",
	"folderApi":         "Release API",
	"templateApi":       "Release API",
	"triggerApi":        "Release API",
	"calendarApi":       "Release API",
	"reportApi":         "Release API",
	"riskApi":           "Release API",
	"dashboardApi":      "Release API",
	"environmentApi":    "Release API",
	"applicationApi":    "Release API",
	"deliveryApi":       "Release API",
	"attachmentApi":     "Release API",
	"activityLogApi":    "Release API",
	"permissionApi":     "Release API",
	"configurationApi":  "Release API",
	"releaseGroupApi":   "Release API",
	"variableApi":       "Release API",
	"releaseVariables":  "Release variables",
	"globalVariables":   "Release variables",
	"release":           "current release",
	"task":              "current task",
	"phase":             "current phase",
	"getCurrentRelease": "Release API",
	"getCurrentTask":    "Release API",
	// Deploy-side names, in case XLD control tasks land in the same corpus
	"deployed":          "Deploy context",
	"container":         "Deploy context",
	"thisCi":            "Deploy context",
	"context":           "Deploy context",
	"repositoryService": "Deploy API",
	"deploymentService": "Deploy API",
	"taskService":       "Deploy API",
	"deployitClient":    "Deploy API",
}
