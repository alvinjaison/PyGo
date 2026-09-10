package compiler

import "github.com/alvinjaison/pygo/internal/bytecode"

// scopeType distinguishes module-level from function-level compilation.
type scopeType int

const (
	scopeModule   scopeType = iota
	scopeFunction
)

// loopContext tracks the state of a loop for break/continue jump patching.
type loopContext struct {
	// breakJumps holds instruction indices of JUMP_ABSOLUTE
	// instructions emitted for break statements that need patching.
	breakJumps []int
	// continueTarget is the instruction offset where continue should jump (loop top).
	continueTarget int
	// isForLoop indicates this is a for-loop which has an iterator on the stack
	// that must be popped on break.
	isForLoop bool
}

// scopeCompiler compiles a single scope (module or function) into a Code object.
// It manages instruction emission, constant pool, and line tracking.
type scopeCompiler struct {
	filename     string
	name         string
	symtab       *symbolTable
	scopeType    scopeType
	compiler     *compiler
	instructions []bytecode.Instruction
	constants    []interface{}
	lineTable    []int
	lastLine     int
	lastOffset   int
	// loopStack tracks nested loop contexts for break/continue.
	loopStack []*loopContext
	// currentLine tracks the current source line for line table generation.
	currentLine int
}

// newScopeCompiler creates a scope compiler for a given scope.
func newScopeCompiler(filename, name string, symtab *symbolTable, st scopeType) *scopeCompiler {
	return &scopeCompiler{
		filename:  filename,
		name:      name,
		symtab:    symtab,
		scopeType: st,
	}
}

// emit appends an instruction to the instruction sequence.
func (sc *scopeCompiler) emit(op bytecode.Opcode, arg uint32) int {
	idx := len(sc.instructions)
	sc.instructions = append(sc.instructions, bytecode.Instruction{Op: op, Arg: arg})
	return idx
}

// emitLoadNone loads a None constant onto the stack.
func (sc *scopeCompiler) emitLoadNone() {
	idx := sc.addConstant(nil)
	sc.emit(bytecode.LOAD_CONST, uint32(idx))
}

// addConstant adds a value to the constant pool and returns its index.
// Deduplicates constants by value.
func (sc *scopeCompiler) addConstant(value interface{}) int {
	// Check for existing constant (simple deduplication).
	for i, c := range sc.constants {
		if constantsEqual(c, value) {
			return i
		}
	}
	idx := len(sc.constants)
	sc.constants = append(sc.constants, value)
	return idx
}

// constantsEqual checks if two constant values are the same.
func constantsEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	switch av := a.(type) {
	case int64:
		bv, ok := b.(int64)
		return ok && av == bv
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	default:
		// For *bytecode.Code and other types, don't deduplicate.
		return false
	}
}

// finalize produces the completed Code object for this scope.
func (sc *scopeCompiler) finalize(firstLine int) *bytecode.Code {
	code := &bytecode.Code{
		Name:         sc.name,
		Filename:     sc.filename,
		FirstLine:    firstLine,
		Instructions: sc.instructions,
		Constants:    sc.constants,
		Names:        sc.symtab.names,
		Varnames:     sc.symtab.varnames,
		Cellvars:     sc.symtab.cellvars,
		Freevars:     sc.symtab.freevars,
		LineTable:    sc.lineTable,
		StackSize:    sc.calculateStackSize(),
	}

	if sc.scopeType == scopeFunction {
		code.Flags = bytecode.FlagOptimized | bytecode.FlagNewLocals
		if sc.symtab.hasYield {
			code.Flags |= bytecode.FlagGenerator
		}
	}

	return code
}

// patchJump patches a jump instruction at `idx` to jump to the current instruction offset.
// Used for POP_JUMP_IF_FALSE, POP_JUMP_IF_TRUE, JUMP_IF_TRUE_OR_POP, JUMP_IF_FALSE_OR_POP
// which use absolute targets.
func (sc *scopeCompiler) patchJump(idx int) {
	sc.instructions[idx].Arg = uint32(len(sc.instructions))
}

// patchJumpForward patches a JUMP_FORWARD instruction at `idx` to jump to the current offset.
// JUMP_FORWARD's arg is the number of instructions to skip forward from the next instruction.
func (sc *scopeCompiler) patchJumpForward(idx int) {
	sc.instructions[idx].Arg = uint32(len(sc.instructions) - idx - 1)
}

// calculateStackSize estimates the maximum stack depth.
// This is a conservative estimate — a more precise calculation can be done
// by simulating the stack effect of each instruction.
func (sc *scopeCompiler) calculateStackSize() int {
	// Simple heuristic: count maximum possible stack depth.
	// A proper implementation would do a stack-depth analysis pass.
	size := 0
	current := 0
	for _, instr := range sc.instructions {
		effect := stackEffect(instr.Op)
		current += effect
		if current > size {
			size = current
		}
		if current < 0 {
			current = 0
		}
	}
	if size < 1 {
		size = 1
	}
	return size
}

// stackEffect returns the net stack effect of an opcode.
// Positive means items pushed, negative means items consumed.
func stackEffect(op bytecode.Opcode) int {
	switch op {
	case bytecode.POP_TOP:
		return -1
	case bytecode.ROT_TWO, bytecode.ROT_THREE, bytecode.DUP_TOP:
		return 0
	case bytecode.LOAD_CONST, bytecode.LOAD_NAME, bytecode.LOAD_FAST,
		bytecode.LOAD_GLOBAL, bytecode.LOAD_ATTR, bytecode.LOAD_CLOSURE,
		bytecode.LOAD_DEREF:
		return 1
	case bytecode.STORE_NAME, bytecode.STORE_FAST, bytecode.STORE_GLOBAL,
		bytecode.STORE_ATTR, bytecode.STORE_DEREF:
		return -1
	case bytecode.RETURN_VALUE:
		return -1
	case bytecode.BINARY_ADD, bytecode.BINARY_SUBTRACT, bytecode.BINARY_MULTIPLY,
		bytecode.BINARY_TRUE_DIVIDE, bytecode.BINARY_FLOOR_DIVIDE,
		bytecode.BINARY_MODULO, bytecode.BINARY_POWER,
		bytecode.BINARY_LSHIFT, bytecode.BINARY_RSHIFT,
		bytecode.BINARY_AND, bytecode.BINARY_XOR, bytecode.BINARY_OR,
		bytecode.BINARY_SUBSCR, bytecode.COMPARE_OP:
		return -1
	case bytecode.UNARY_POSITIVE, bytecode.UNARY_NEGATIVE,
		bytecode.UNARY_NOT, bytecode.UNARY_INVERT:
		return 0
	case bytecode.MAKE_FUNCTION:
		return -1 // consumes code + name, pushes function
	case bytecode.CALL_FUNCTION:
		return 0 // net effect depends on arg count; conservative estimate
	case bytecode.RAISE_VARARGS:
		return -1 // consumes exception (for arg=1)
	case bytecode.RERAISE:
		return -1 // pops TOS (exception or None)
	case bytecode.SETUP_FINALLY:
		return 0
	case bytecode.POP_BLOCK:
		return 0
	case bytecode.POP_EXCEPT:
		return 0
	case bytecode.IMPORT_NAME:
		return 1 // pushes module
	case bytecode.IMPORT_FROM:
		return 1 // pushes attribute (module stays on stack)
	case bytecode.UNPACK_SEQUENCE:
		return 0 // pops 1, pushes N; conservative
	case bytecode.UNPACK_EX:
		return 0 // pops 1, pushes N; conservative
	case bytecode.BUILD_LIST, bytecode.BUILD_TUPLE, bytecode.BUILD_MAP:
		return 1 // pushes the collection (pops N elements; net ≈ 1 for estimates)
	case bytecode.BUILD_TUPLE_UNPACK_WITH_CALL, bytecode.BUILD_MAP_UNPACK:
		return 1 // merges N items into one (net ≈ 1 for estimates)
	case bytecode.CALL_FUNCTION_KW:
		return 0 // net effect depends on arg count; conservative estimate
	case bytecode.CALL_FUNCTION_EX:
		return 0 // net effect depends on arg count; conservative estimate
	case bytecode.GET_ITER:
		return 0
	case bytecode.FOR_ITER:
		return 1
	default:
		return 0
	}
}

// closureIndex returns the LOAD_CLOSURE operand index for a variable name.
// The index is into the current scope's cellvars (first) then freevars.
// This is used when a nested function references a name that is a cell or free
// variable in the enclosing (current) scope.
func (sc *scopeCompiler) closureIndex(name string) int {
	// Check cellvars first.
	for i, cv := range sc.symtab.cellvars {
		if cv == name {
			return i
		}
	}
	// Then check freevars (offset by len(cellvars)).
	for i, fv := range sc.symtab.freevars {
		if fv == name {
			return len(sc.symtab.cellvars) + i
		}
	}
	// Should not happen if symbol table analysis is correct.
	// Return 0 as a fallback (will produce an incorrect but non-crashing result).
	return 0
}

// pushLoop pushes a new loop context onto the loop stack.
// continueTarget is the instruction offset for continue statements.
// isForLoop indicates whether this is a for-loop with an iterator on the stack.
func (sc *scopeCompiler) pushLoop(continueTarget int, isForLoop bool) {
	sc.loopStack = append(sc.loopStack, &loopContext{
		continueTarget: continueTarget,
		isForLoop:      isForLoop,
	})
}

// popLoop pops the current loop context and returns it.
// The caller uses the returned context to patch break jumps.
func (sc *scopeCompiler) popLoop() *loopContext {
	if len(sc.loopStack) == 0 {
		return nil
	}
	ctx := sc.loopStack[len(sc.loopStack)-1]
	sc.loopStack = sc.loopStack[:len(sc.loopStack)-1]
	return ctx
}

// currentLoop returns the current (innermost) loop context, or nil if not in a loop.
func (sc *scopeCompiler) currentLoop() *loopContext {
	if len(sc.loopStack) == 0 {
		return nil
	}
	return sc.loopStack[len(sc.loopStack)-1]
}

// setLine sets the current source line for line table generation.
// Call this before emitting instructions for a new source line.
func (sc *scopeCompiler) setLine(line int) {
	if line <= 0 || line == sc.currentLine {
		return
	}
	offset := len(sc.instructions)
	if sc.currentLine == 0 {
		// First line entry — just record.
		sc.currentLine = line
		sc.lastLine = line
		sc.lastOffset = 0
		return
	}
	// Emit a line table entry: (bytecode_delta, line_delta).
	bytecodeDelta := offset - sc.lastOffset
	lineDelta := line - sc.lastLine
	if bytecodeDelta > 0 {
		sc.lineTable = append(sc.lineTable, bytecodeDelta, lineDelta)
		sc.lastOffset = offset
		sc.lastLine = line
	}
	sc.currentLine = line
}
