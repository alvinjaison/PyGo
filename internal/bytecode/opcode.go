// Package bytecode defines PyGo's instruction set architecture.
//
// Instructions are 2 bytes wide: one byte for the opcode and one byte for
// the operand. Operands larger than 255 use a preceding EXTENDED_ARG instruction
// to supply the high bits, matching CPython's encoding strategy.
//
// The instruction set is an internal implementation detail. The bytecode
// format is not part of PyGo's public API and may change between versions.
// Do not serialize bytecode for long-term storage until the format is
// declared stable.
package bytecode

import "fmt"

// Opcode is a single bytecode instruction identifier.
type Opcode uint8

// Instruction encodes one bytecode instruction.
type Instruction struct {
	Op  Opcode
	Arg uint32 // effective argument (after EXTENDED_ARG expansion)
}

// OpInfo describes an opcode's mnemonic and whether it takes an argument.
type OpInfo struct {
	Name   string
	HasArg bool
}

// Opcode constants.
const (
	// Stack manipulation
	POP_TOP     Opcode = 1
	ROT_TWO     Opcode = 2
	ROT_THREE   Opcode = 3
	DUP_TOP     Opcode = 4
	DUP_TOP_TWO Opcode = 5
	ROT_FOUR    Opcode = 6

	// Unary operators
	UNARY_POSITIVE Opcode = 10
	UNARY_NEGATIVE Opcode = 11
	UNARY_NOT      Opcode = 12
	UNARY_INVERT   Opcode = 13

	// Binary operators
	BINARY_MATRIX_MULTIPLY Opcode = 16
	BINARY_POWER           Opcode = 19
	BINARY_MULTIPLY        Opcode = 20
	BINARY_MODULO          Opcode = 22
	BINARY_ADD             Opcode = 23
	BINARY_SUBTRACT        Opcode = 24
	BINARY_SUBSCR          Opcode = 25
	BINARY_FLOOR_DIVIDE    Opcode = 26
	BINARY_TRUE_DIVIDE     Opcode = 27
	BINARY_LSHIFT          Opcode = 62
	BINARY_RSHIFT          Opcode = 63
	BINARY_AND             Opcode = 64
	BINARY_XOR             Opcode = 65
	BINARY_OR              Opcode = 66

	// In-place operators
	INPLACE_MATRIX_MULTIPLY Opcode = 17
	INPLACE_FLOOR_DIVIDE    Opcode = 28
	INPLACE_TRUE_DIVIDE     Opcode = 29
	INPLACE_MODULO          Opcode = 30
	INPLACE_ADD             Opcode = 55
	INPLACE_SUBTRACT        Opcode = 56
	INPLACE_MULTIPLY        Opcode = 57
	INPLACE_POWER           Opcode = 67
	INPLACE_LSHIFT          Opcode = 75
	INPLACE_RSHIFT          Opcode = 76
	INPLACE_AND             Opcode = 77
	INPLACE_XOR             Opcode = 78
	INPLACE_OR              Opcode = 79

	// Subscript / attribute store / delete
	STORE_SUBSCR  Opcode = 60
	DELETE_SUBSCR Opcode = 61

	// Miscellaneous
	GET_LEN    Opcode = 30 // reused — reserved for future
	PRINT_EXPR Opcode = 70 // REPL expression printing

	// Sequence building
	BUILD_TUPLE                  Opcode = 102
	BUILD_LIST                   Opcode = 103
	BUILD_SET                    Opcode = 104
	BUILD_MAP                    Opcode = 105
	LIST_APPEND                  Opcode = 145 // append TOS to the list at TOS1 (comprehensions)
	SET_ADD                      Opcode = 146
	MAP_ADD                      Opcode = 147
	BUILD_CONST_KEY_MAP          Opcode = 141
	BUILD_STRING                 Opcode = 157
	BUILD_TUPLE_UNPACK_WITH_CALL Opcode = 158
	BUILD_MAP_UNPACK             Opcode = 150
	BUILD_SLICE                  Opcode = 133

	// Load / store (argument = index into names/consts/fastlocals)
	LOAD_CONST  Opcode = 100
	LOAD_NAME   Opcode = 101
	STORE_NAME  Opcode = 90
	DELETE_NAME Opcode = 91

	LOAD_FAST   Opcode = 124
	STORE_FAST  Opcode = 125
	DELETE_FAST Opcode = 126

	LOAD_GLOBAL   Opcode = 116
	STORE_GLOBAL  Opcode = 97
	DELETE_GLOBAL Opcode = 98

	// Closures
	LOAD_DEREF   Opcode = 136
	STORE_DEREF  Opcode = 137
	DELETE_DEREF Opcode = 138
	LOAD_CLOSURE Opcode = 135
	MAKE_CELL    Opcode = 135 // alias — Phase 2

	// Attributes
	LOAD_ATTR   Opcode = 106
	STORE_ATTR  Opcode = 95
	DELETE_ATTR Opcode = 96

	// Imports
	IMPORT_NAME Opcode = 108
	IMPORT_FROM Opcode = 109
	IMPORT_STAR Opcode = 84

	// Jumps (argument = absolute target offset)
	JUMP_FORWARD         Opcode = 110
	JUMP_ABSOLUTE        Opcode = 113
	POP_JUMP_IF_TRUE     Opcode = 115
	POP_JUMP_IF_FALSE    Opcode = 114
	JUMP_IF_TRUE_OR_POP  Opcode = 112
	JUMP_IF_FALSE_OR_POP Opcode = 111

	// Iteration
	GET_ITER            Opcode = 68
	GET_YIELD_FROM_ITER Opcode = 69
	FOR_ITER            Opcode = 93

	// Functions
	MAKE_FUNCTION    Opcode = 132
	CALL_FUNCTION    Opcode = 131
	CALL_FUNCTION_KW Opcode = 141
	CALL_FUNCTION_EX Opcode = 142
	RETURN_VALUE     Opcode = 83

	// Exceptions
	RAISE_VARARGS     Opcode = 130
	SETUP_FINALLY     Opcode = 122
	POP_BLOCK         Opcode = 87
	POP_EXCEPT        Opcode = 89
	RERAISE           Opcode = 119
	WITH_EXCEPT_START Opcode = 49

	// Context managers
	SETUP_WITH   Opcode = 143 // enter a with block (TOS = context manager)
	WITH_CLEANUP Opcode = 148 // normal-path cleanup: call __exit__(None,None,None)

	// Classes
	LOAD_BUILD_CLASS Opcode = 71

	// Generators / coroutines
	YIELD_VALUE   Opcode = 86
	YIELD_FROM    Opcode = 72
	GET_AWAITABLE Opcode = 73
	GET_AITER     Opcode = 50
	GET_ANEXT     Opcode = 51
	END_ASYNC_FOR Opcode = 54

	// Misc
	NOP          Opcode = 9
	EXTENDED_ARG Opcode = 144 // extends the argument of the next instruction

	// Comparison
	COMPARE_OP Opcode = 107

	// Unpack
	UNPACK_SEQUENCE Opcode = 92
	UNPACK_EX       Opcode = 94

	// Structural pattern matching. Each tests the value on top of the stack
	// without consuming it, unless stated otherwise.
	MATCH_SEQ     Opcode = 31 // push True if TOS is a list/tuple (not str/bytes)
	MATCH_SEQ_LEN Opcode = 32 // arg = n<<1|star; push len==n (len>=n if star)
	MATCH_MAPPING Opcode = 33 // push True if TOS is a mapping
	MATCH_KEY     Opcode = 34 // pop key; if in mapping TOS push value, True; else push False
	MATCH_REST    Opcode = 35 // pop tuple of keys; push copy of mapping TOS without them
	MATCH_CLASS   Opcode = 36 // arg = positional count; pop kwnames, cls, subject; push attrs tuple, True | False

	// Unpacking in calls and displays: f(*a, **k), [*a, 1], {**d}.
	LIST_EXTEND   Opcode = 37 // pop iterable; extend the list now on top
	DICT_MERGE    Opcode = 38 // arg 1: pop mapping, merge into dict on top, rejecting duplicate keys (calls); arg 0: update
	LIST_TO_TUPLE Opcode = 39 // replace the list on top with a tuple
	LIST_TO_SET   Opcode = 40 // replace the list on top with a set
)

// UNPACK_EX argument layout: the number of targets before the starred target
// in the low byte and the number after it in the next byte.
const (
	UnpackBeforeMask = 0xFF
	UnpackAfterShift = 8
)

// CompareOp identifies the comparison operator for COMPARE_OP.
type CompareOp int

const (
	CmpLt CompareOp = iota
	CmpLtE
	CmpEq
	CmpNotEq
	CmpGt
	CmpGtE
	CmpIn
	CmpNotIn
	CmpIs
	CmpIsNot
	CmpExcMatch // exception-match test used by except clauses
)

func (c CompareOp) String() string {
	return [...]string{"<", "<=", "==", "!=", ">", ">=", "in", "not in", "is", "is not", "exc-match"}[c]
}

// MakeFunctionFlag is a bitmask for the MAKE_FUNCTION instruction's argument.
type MakeFunctionFlag uint8

const (
	MakeFuncHasDefaults    MakeFunctionFlag = 1 << 0
	MakeFuncHasKwDefaults  MakeFunctionFlag = 1 << 1
	MakeFuncHasAnnotations MakeFunctionFlag = 1 << 2
	MakeFuncHasClosure     MakeFunctionFlag = 1 << 3
)

// String returns the mnemonic for an opcode.
func (op Opcode) String() string {
	if info, ok := opInfos[op]; ok {
		return info.Name
	}
	return fmt.Sprintf("UNKNOWN_OP(%d)", int(op))
}

// HasArg reports whether an opcode takes an argument.
func (op Opcode) HasArg() bool {
	if info, ok := opInfos[op]; ok {
		return info.HasArg
	}
	return false
}

var opInfos = map[Opcode]OpInfo{
	POP_TOP:      {"POP_TOP", false},
	ROT_TWO:      {"ROT_TWO", false},
	ROT_THREE:    {"ROT_THREE", false},
	DUP_TOP:      {"DUP_TOP", false},
	NOP:          {"NOP", false},
	RETURN_VALUE: {"RETURN_VALUE", false},

	UNARY_POSITIVE: {"UNARY_POSITIVE", false},
	UNARY_NEGATIVE: {"UNARY_NEGATIVE", false},
	UNARY_NOT:      {"UNARY_NOT", false},
	UNARY_INVERT:   {"UNARY_INVERT", false},

	BINARY_POWER:        {"BINARY_POWER", false},
	BINARY_MULTIPLY:     {"BINARY_MULTIPLY", false},
	BINARY_MODULO:       {"BINARY_MODULO", false},
	BINARY_ADD:          {"BINARY_ADD", false},
	BINARY_SUBTRACT:     {"BINARY_SUBTRACT", false},
	BINARY_SUBSCR:       {"BINARY_SUBSCR", false},
	BINARY_FLOOR_DIVIDE: {"BINARY_FLOOR_DIVIDE", false},
	BINARY_TRUE_DIVIDE:  {"BINARY_TRUE_DIVIDE", false},
	BINARY_LSHIFT:       {"BINARY_LSHIFT", false},
	BINARY_RSHIFT:       {"BINARY_RSHIFT", false},
	BINARY_AND:          {"BINARY_AND", false},
	BINARY_XOR:          {"BINARY_XOR", false},
	BINARY_OR:           {"BINARY_OR", false},

	STORE_SUBSCR:  {"STORE_SUBSCR", false},
	DELETE_SUBSCR: {"DELETE_SUBSCR", false},

	LOAD_CONST:  {"LOAD_CONST", true},
	LOAD_NAME:   {"LOAD_NAME", true},
	STORE_NAME:  {"STORE_NAME", true},
	DELETE_NAME: {"DELETE_NAME", true},

	LOAD_FAST:   {"LOAD_FAST", true},
	STORE_FAST:  {"STORE_FAST", true},
	DELETE_FAST: {"DELETE_FAST", true},

	LOAD_GLOBAL:   {"LOAD_GLOBAL", true},
	STORE_GLOBAL:  {"STORE_GLOBAL", true},
	DELETE_GLOBAL: {"DELETE_GLOBAL", true},

	LOAD_DEREF:   {"LOAD_DEREF", true},
	STORE_DEREF:  {"STORE_DEREF", true},
	LOAD_CLOSURE: {"LOAD_CLOSURE", true},

	LOAD_ATTR:   {"LOAD_ATTR", true},
	STORE_ATTR:  {"STORE_ATTR", true},
	DELETE_ATTR: {"DELETE_ATTR", true},

	IMPORT_NAME: {"IMPORT_NAME", true},
	IMPORT_FROM: {"IMPORT_FROM", true},
	IMPORT_STAR: {"IMPORT_STAR", false},

	JUMP_FORWARD:         {"JUMP_FORWARD", true},
	JUMP_ABSOLUTE:        {"JUMP_ABSOLUTE", true},
	POP_JUMP_IF_TRUE:     {"POP_JUMP_IF_TRUE", true},
	POP_JUMP_IF_FALSE:    {"POP_JUMP_IF_FALSE", true},
	JUMP_IF_TRUE_OR_POP:  {"JUMP_IF_TRUE_OR_POP", true},
	JUMP_IF_FALSE_OR_POP: {"JUMP_IF_FALSE_OR_POP", true},

	GET_ITER: {"GET_ITER", false},
	FOR_ITER: {"FOR_ITER", true},

	MAKE_FUNCTION:    {"MAKE_FUNCTION", true},
	CALL_FUNCTION:    {"CALL_FUNCTION", true},
	CALL_FUNCTION_KW: {"CALL_FUNCTION_KW", true},
	CALL_FUNCTION_EX: {"CALL_FUNCTION_EX", true},

	RAISE_VARARGS: {"RAISE_VARARGS", true},
	SETUP_FINALLY: {"SETUP_FINALLY", true},
	POP_BLOCK:     {"POP_BLOCK", false},
	POP_EXCEPT:    {"POP_EXCEPT", false},
	SETUP_WITH:    {"SETUP_WITH", true},
	WITH_CLEANUP:  {"WITH_CLEANUP", false},
	RERAISE:       {"RERAISE", true},

	LOAD_BUILD_CLASS: {"LOAD_BUILD_CLASS", false},

	YIELD_VALUE:   {"YIELD_VALUE", false},
	YIELD_FROM:    {"YIELD_FROM", false},
	GET_AWAITABLE: {"GET_AWAITABLE", true},

	COMPARE_OP: {"COMPARE_OP", true},

	BUILD_TUPLE: {"BUILD_TUPLE", true},
	BUILD_LIST:  {"BUILD_LIST", true},
	BUILD_SET:   {"BUILD_SET", true},
	BUILD_MAP:   {"BUILD_MAP", true},
	BUILD_SLICE: {"BUILD_SLICE", true},

	UNPACK_SEQUENCE: {"UNPACK_SEQUENCE", true},
	UNPACK_EX:       {"UNPACK_EX", true},

	MATCH_SEQ:     {"MATCH_SEQ", false},
	MATCH_SEQ_LEN: {"MATCH_SEQ_LEN", true},
	MATCH_MAPPING: {"MATCH_MAPPING", false},
	MATCH_KEY:     {"MATCH_KEY", false},
	MATCH_REST:    {"MATCH_REST", false},
	MATCH_CLASS:   {"MATCH_CLASS", true},

	LIST_EXTEND:   {"LIST_EXTEND", false},
	DICT_MERGE:    {"DICT_MERGE", true},
	LIST_TO_TUPLE: {"LIST_TO_TUPLE", false},
	LIST_TO_SET:   {"LIST_TO_SET", false},

	EXTENDED_ARG: {"EXTENDED_ARG", true},
	PRINT_EXPR:   {"PRINT_EXPR", false},
}
