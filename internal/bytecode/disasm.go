package bytecode

import (
	"fmt"
	"io"
)

// Disassemble writes a human-readable disassembly of a Code object (and,
// recursively, any nested Code objects in its constant pool) to w.
func Disassemble(w io.Writer, code *Code) {
	fmt.Fprintf(w, "Disassembly of %s (%s):\n", code.Name, code.Filename)
	disassembleOne(w, code, "  ")

	// Recurse into nested code objects.
	for _, c := range code.Constants {
		if nested, ok := c.(*Code); ok {
			fmt.Fprintln(w)
			Disassemble(w, nested)
		}
	}
}

func disassembleOne(w io.Writer, code *Code, indent string) {
	for i, in := range code.Instructions {
		line := code.LineForOffset(i)
		if in.Op.HasArg() {
			fmt.Fprintf(w, "%s%4d  %-22s %4d   %s\n",
				indent, i, in.Op.String(), in.Arg, argComment(code, in))
		} else {
			fmt.Fprintf(w, "%s%4d  %-22s\n", indent, i, in.Op.String())
		}
		_ = line
	}
}

// argComment produces a helpful comment describing the instruction argument.
func argComment(code *Code, in Instruction) string {
	switch in.Op {
	case LOAD_CONST:
		if int(in.Arg) < len(code.Constants) {
			return fmt.Sprintf("(%v)", code.Constants[in.Arg])
		}
	case LOAD_NAME, STORE_NAME, LOAD_GLOBAL, STORE_GLOBAL, LOAD_ATTR, STORE_ATTR,
		IMPORT_NAME, IMPORT_FROM, DELETE_NAME:
		if int(in.Arg) < len(code.Names) {
			return fmt.Sprintf("(%s)", code.Names[in.Arg])
		}
	case LOAD_FAST, STORE_FAST, DELETE_FAST:
		if int(in.Arg) < len(code.Varnames) {
			return fmt.Sprintf("(%s)", code.Varnames[in.Arg])
		}
	case COMPARE_OP:
		return fmt.Sprintf("(%s)", CompareOp(in.Arg).String())
	}
	return ""
}
