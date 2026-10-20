package compiler

import "github.com/pygo/pygo/internal/bytecode"

// stackEffect returns the net change in value-stack depth produced by one
// instruction. Used only to size the frame's value stack conservatively.
func stackEffect(in bytecode.Instruction) int {
	switch in.Op {
	case bytecode.LOAD_CONST, bytecode.LOAD_NAME, bytecode.LOAD_FAST,
		bytecode.LOAD_GLOBAL, bytecode.LOAD_DEREF, bytecode.LOAD_CLOSURE,
		bytecode.LOAD_BUILD_CLASS, bytecode.DUP_TOP:
		return 1

	case bytecode.POP_TOP, bytecode.STORE_NAME, bytecode.STORE_FAST,
		bytecode.STORE_GLOBAL, bytecode.STORE_DEREF, bytecode.RETURN_VALUE,
		bytecode.POP_JUMP_IF_FALSE, bytecode.POP_JUMP_IF_TRUE,
		bytecode.YIELD_VALUE, bytecode.LIST_APPEND, bytecode.IMPORT_FROM:
		return -1

	case bytecode.STORE_ATTR:
		return -2

	case bytecode.STORE_SUBSCR:
		return -3

	case bytecode.BINARY_ADD, bytecode.BINARY_SUBTRACT, bytecode.BINARY_MULTIPLY,
		bytecode.BINARY_TRUE_DIVIDE, bytecode.BINARY_FLOOR_DIVIDE,
		bytecode.BINARY_MODULO, bytecode.BINARY_POWER, bytecode.BINARY_AND,
		bytecode.BINARY_OR, bytecode.BINARY_XOR, bytecode.BINARY_LSHIFT,
		bytecode.BINARY_RSHIFT, bytecode.BINARY_SUBSCR, bytecode.COMPARE_OP:
		return -1

	case bytecode.DUP_TOP_TWO:
		return 2

	case bytecode.BUILD_LIST, bytecode.BUILD_TUPLE, bytecode.BUILD_SET:
		return 1 - int(in.Arg)

	case bytecode.BUILD_MAP:
		return 1 - 2*int(in.Arg)

	case bytecode.BUILD_SLICE:
		return 1 - int(in.Arg)

	case bytecode.CALL_FUNCTION:
		return -int(in.Arg)

	case bytecode.CALL_FUNCTION_KW:
		return -int(in.Arg) - 1

	case bytecode.MAKE_FUNCTION:
		// pops code + name (+ optional defaults/closure), pushes function.
		return -1

	case bytecode.FOR_ITER:
		return 1 // pushes next value (iterator stays)

	case bytecode.GET_ITER, bytecode.UNARY_NEGATIVE, bytecode.UNARY_POSITIVE,
		bytecode.UNARY_NOT, bytecode.UNARY_INVERT, bytecode.LOAD_ATTR,
		bytecode.IMPORT_NAME:
		return 0
	}
	return 0
}
