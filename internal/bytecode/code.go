package bytecode

// Code is a compiled Python code object — the output of the compiler
// and the input to the virtual machine.
//
// One Code object is produced per function, class body, comprehension,
// generator, or module. Nested functions produce nested Code objects stored
// in the Constants pool.
type Code struct {
	// Name is the qualified name of the code object.
	// For modules: "<module>". For functions: the function name.
	// For classes: the class name. For lambdas: "<lambda>".
	Name string

	// Filename is the source file this code was compiled from.
	// "<stdin>" for REPL input, "<string>" for RunString input.
	Filename string

	// FirstLine is the first line number of this code object in the source.
	FirstLine int

	// Instructions is the flat bytecode stream.
	Instructions []Instruction

	// Constants is the constant pool. Values may be:
	//   int64, float64, complex128, string, bool, nil (for None), *Code
	Constants []interface{}

	// Names is the pool of global/attribute/import name strings.
	Names []string

	// Varnames is the list of local variable names (indexed by LOAD_FAST argument).
	Varnames []string

	// Freevars is the list of free variable names (from enclosing scopes).
	Freevars []string

	// Cellvars is the list of cell variable names (referenced by nested functions).
	Cellvars []string

	// ArgCount is the number of positional parameters (not including *args, **kwargs).
	ArgCount int

	// PosOnlyArgCount is the number of positional-only parameters (before /).
	PosOnlyArgCount int

	// KwOnlyArgCount is the number of keyword-only parameters.
	KwOnlyArgCount int

	// Flags is a bitmask of CodeFlags.
	Flags CodeFlag

	// LineTable maps bytecode instruction offsets to source line numbers.
	// Encoded as alternating (bytecode_delta, line_delta) pairs.
	// Use LineForOffset to decode.
	LineTable []int

	// ExceptionTable holds exception handler entries.
	// The VM consults this table when an exception is raised.
	ExceptionTable []ExceptionEntry

	// StackSize is the maximum stack depth required by this code object.
	// Used to pre-allocate the value stack.
	StackSize int
}

// CodeFlag is a bitmask of properties for a Code object.
type CodeFlag uint32

const (
	// FlagOptimized means the code uses LOAD_FAST / STORE_FAST for locals.
	FlagOptimized CodeFlag = 1 << 0

	// FlagNewLocals means a new locals dict should be created on entry.
	FlagNewLocals CodeFlag = 1 << 1

	// FlagVarargs means the function accepts *args.
	FlagVarargs CodeFlag = 1 << 2

	// FlagVarkeywords means the function accepts **kwargs.
	FlagVarkeywords CodeFlag = 1 << 3

	// FlagNested means the code is defined inside another function.
	FlagNested CodeFlag = 1 << 4

	// FlagGenerator means the function body contains yield.
	FlagGenerator CodeFlag = 1 << 5

	// FlagCoroutine means the function is defined with async def.
	FlagCoroutine CodeFlag = 1 << 6

	// FlagAsyncGenerator means async def + yield.
	FlagAsyncGenerator CodeFlag = 1 << 7
)

// ExceptionEntry describes one exception handler in a Code object.
// When an exception is raised at a bytecode offset within [Start, End),
// execution jumps to Handler.
type ExceptionEntry struct {
	Start   int // first instruction offset covered (inclusive)
	End     int // last instruction offset covered (exclusive)
	Handler int // target instruction offset for the handler
	Depth   int // value stack depth when entering the handler
}

// LineForOffset returns the source line number for a bytecode instruction
// at the given offset. Returns FirstLine if the offset is not found.
func (c *Code) LineForOffset(offset int) int {
	line := c.FirstLine
	cur := 0
	for i := 0; i+1 < len(c.LineTable); i += 2 {
		cur += c.LineTable[i]
		if cur > offset {
			break
		}
		line += c.LineTable[i+1]
	}
	return line
}

// HasFlag reports whether a flag is set.
func (c *Code) HasFlag(f CodeFlag) bool {
	return c.Flags&f != 0
}

// IsGenerator reports whether this code object is a generator function.
func (c *Code) IsGenerator() bool { return c.HasFlag(FlagGenerator) }

// IsCoroutine reports whether this code object is a coroutine function.
func (c *Code) IsCoroutine() bool { return c.HasFlag(FlagCoroutine) }
