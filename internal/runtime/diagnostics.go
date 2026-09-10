// diagnostics.go provides developer-facing introspection utilities for
// disassembling bytecode and dumping AST trees from Python source.
package runtime

import (
	"fmt"
	"io"
	"strings"

	"github.com/alvinjaison/pygo/internal/ast"
	"github.com/alvinjaison/pygo/internal/bytecode"
	"github.com/alvinjaison/pygo/internal/compiler"
	"github.com/alvinjaison/pygo/internal/parser"
)

// Disassemble parses and compiles source, then writes a human-readable
// bytecode listing to w. Returns an error without partial output if the
// source has syntax or compilation errors.
func Disassemble(source string, w io.Writer) error {
	module, err := parser.Parse(source, "<disassembly>")
	if err != nil {
		return err
	}

	code, err := compiler.Compile(module)
	if err != nil {
		return err
	}

	disassembleCode(code, w)
	return nil
}

// disassembleCode writes the bytecode listing for a single Code object.
func disassembleCode(code *bytecode.Code, w io.Writer) {
	fmt.Fprintf(w, "Disassembly of %s:\n", code.Name)

	for i, instr := range code.Instructions {
		line := code.LineForOffset(i)
		if instr.Op == bytecode.EXTENDED_ARG {
			// Skip EXTENDED_ARG in the listing — its effect is merged into the next instruction.
			continue
		}

		if instr.Op.HasArg() {
			decoded := decodeArg(code, instr)
			fmt.Fprintf(w, "%4d %4d  %-20s %d (%s)\n", i, line, instr.Op.String(), instr.Arg, decoded)
		} else {
			fmt.Fprintf(w, "%4d %4d  %-20s\n", i, line, instr.Op.String())
		}
	}
}

// decodeArg returns a human-readable decoding of an instruction's argument.
func decodeArg(code *bytecode.Code, instr bytecode.Instruction) string {
	arg := int(instr.Arg)

	switch instr.Op {
	case bytecode.LOAD_CONST:
		if arg >= 0 && arg < len(code.Constants) {
			return formatConstant(code.Constants[arg])
		}
		return "?"

	case bytecode.LOAD_NAME, bytecode.STORE_NAME, bytecode.DELETE_NAME,
		bytecode.LOAD_GLOBAL, bytecode.STORE_GLOBAL, bytecode.DELETE_GLOBAL,
		bytecode.LOAD_ATTR, bytecode.STORE_ATTR, bytecode.DELETE_ATTR,
		bytecode.IMPORT_NAME, bytecode.IMPORT_FROM:
		if arg >= 0 && arg < len(code.Names) {
			return code.Names[arg]
		}
		return "?"

	case bytecode.LOAD_FAST, bytecode.STORE_FAST, bytecode.DELETE_FAST:
		if arg >= 0 && arg < len(code.Varnames) {
			return code.Varnames[arg]
		}
		return "?"

	case bytecode.COMPARE_OP:
		if arg >= 0 && arg <= int(bytecode.CmpExcMatch) {
			return bytecode.CompareOp(arg).String()
		}
		return "?"

	case bytecode.JUMP_FORWARD, bytecode.JUMP_ABSOLUTE,
		bytecode.POP_JUMP_IF_TRUE, bytecode.POP_JUMP_IF_FALSE,
		bytecode.JUMP_IF_TRUE_OR_POP, bytecode.JUMP_IF_FALSE_OR_POP,
		bytecode.FOR_ITER, bytecode.SETUP_FINALLY:
		return fmt.Sprintf("to %d", arg)

	default:
		return fmt.Sprintf("%d", arg)
	}
}

// formatConstant returns a string representation of a constant pool value.
func formatConstant(v interface{}) string {
	switch c := v.(type) {
	case nil:
		return "None"
	case bool:
		if c {
			return "True"
		}
		return "False"
	case int64:
		return fmt.Sprintf("%d", c)
	case float64:
		return fmt.Sprintf("%g", c)
	case string:
		return fmt.Sprintf("%q", c)
	case *bytecode.Code:
		return fmt.Sprintf("<code %s>", c.Name)
	default:
		return fmt.Sprintf("%v", c)
	}
}

// DumpAST parses source and writes an indented text representation of the
// AST to w. Returns an error without partial output if the source has
// syntax errors.
func DumpAST(source string, w io.Writer) error {
	module, err := parser.Parse(source, "<ast>")
	if err != nil {
		return err
	}

	dumpNode(w, module, 0)
	return nil
}

// dumpNode recursively writes an AST node with indentation.
func dumpNode(w io.Writer, node ast.Node, depth int) {
	indent := strings.Repeat("  ", depth)

	switch n := node.(type) {
	case *ast.Module:
		fmt.Fprintf(w, "%sModule\n", indent)
		for _, stmt := range n.Body {
			dumpNode(w, stmt, depth+1)
		}

	// Statements
	case *ast.FunctionDef:
		fmt.Fprintf(w, "%sFunctionDef [line %d]\n", indent, n.Pos().Line)
		fmt.Fprintf(w, "%s  Name: %s\n", indent, n.Name)
		if n.Args != nil {
			dumpArgs(w, n.Args, depth+1)
		}
		fmt.Fprintf(w, "%s  Body:\n", indent)
		for _, stmt := range n.Body {
			dumpNode(w, stmt, depth+2)
		}

	case *ast.Return:
		fmt.Fprintf(w, "%sReturn [line %d]\n", indent, n.Pos().Line)
		if n.Value != nil {
			fmt.Fprintf(w, "%s  Value: ", indent)
			dumpExprInline(w, n.Value)
			fmt.Fprintln(w)
		}

	case *ast.Assign:
		fmt.Fprintf(w, "%sAssign [line %d]\n", indent, n.Pos().Line)
		for _, target := range n.Targets {
			fmt.Fprintf(w, "%s  Target: ", indent)
			dumpExprInline(w, target)
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s  Value: ", indent)
		dumpExprInline(w, n.Value)
		fmt.Fprintln(w)

	case *ast.AugAssign:
		fmt.Fprintf(w, "%sAugAssign [line %d]\n", indent, n.Pos().Line)
		fmt.Fprintf(w, "%s  Target: ", indent)
		dumpExprInline(w, n.Target)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s  Op: %s\n", indent, n.Op.String())
		fmt.Fprintf(w, "%s  Value: ", indent)
		dumpExprInline(w, n.Value)
		fmt.Fprintln(w)

	case *ast.ExprStmt:
		fmt.Fprintf(w, "%sExprStmt [line %d]\n", indent, n.Pos().Line)
		fmt.Fprintf(w, "%s  Value: ", indent)
		dumpExprInline(w, n.Value)
		fmt.Fprintln(w)

	case *ast.If:
		fmt.Fprintf(w, "%sIf [line %d]\n", indent, n.Pos().Line)
		fmt.Fprintf(w, "%s  Test: ", indent)
		dumpExprInline(w, n.Test)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s  Body:\n", indent)
		for _, stmt := range n.Body {
			dumpNode(w, stmt, depth+2)
		}
		if len(n.Orelse) > 0 {
			fmt.Fprintf(w, "%s  Else:\n", indent)
			for _, stmt := range n.Orelse {
				dumpNode(w, stmt, depth+2)
			}
		}

	case *ast.While:
		fmt.Fprintf(w, "%sWhile [line %d]\n", indent, n.Pos().Line)
		fmt.Fprintf(w, "%s  Test: ", indent)
		dumpExprInline(w, n.Test)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s  Body:\n", indent)
		for _, stmt := range n.Body {
			dumpNode(w, stmt, depth+2)
		}
		if len(n.Orelse) > 0 {
			fmt.Fprintf(w, "%s  Else:\n", indent)
			for _, stmt := range n.Orelse {
				dumpNode(w, stmt, depth+2)
			}
		}

	case *ast.For:
		fmt.Fprintf(w, "%sFor [line %d]\n", indent, n.Pos().Line)
		fmt.Fprintf(w, "%s  Target: ", indent)
		dumpExprInline(w, n.Target)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s  Iter: ", indent)
		dumpExprInline(w, n.Iter)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s  Body:\n", indent)
		for _, stmt := range n.Body {
			dumpNode(w, stmt, depth+2)
		}
		if len(n.Orelse) > 0 {
			fmt.Fprintf(w, "%s  Else:\n", indent)
			for _, stmt := range n.Orelse {
				dumpNode(w, stmt, depth+2)
			}
		}

	case *ast.Try:
		fmt.Fprintf(w, "%sTry [line %d]\n", indent, n.Pos().Line)
		fmt.Fprintf(w, "%s  Body:\n", indent)
		for _, stmt := range n.Body {
			dumpNode(w, stmt, depth+2)
		}
		for _, handler := range n.Handlers {
			dumpNode(w, handler, depth+1)
		}
		if len(n.Orelse) > 0 {
			fmt.Fprintf(w, "%s  Else:\n", indent)
			for _, stmt := range n.Orelse {
				dumpNode(w, stmt, depth+2)
			}
		}
		if len(n.Finalbody) > 0 {
			fmt.Fprintf(w, "%s  Finally:\n", indent)
			for _, stmt := range n.Finalbody {
				dumpNode(w, stmt, depth+2)
			}
		}

	case *ast.ExceptHandler:
		fmt.Fprintf(w, "%sExceptHandler [line %d]\n", indent, n.Pos().Line)
		if n.Type != nil {
			fmt.Fprintf(w, "%s  Type: ", indent)
			dumpExprInline(w, n.Type)
			fmt.Fprintln(w)
		}
		if n.Name != "" {
			fmt.Fprintf(w, "%s  Name: %s\n", indent, n.Name)
		}
		fmt.Fprintf(w, "%s  Body:\n", indent)
		for _, stmt := range n.Body {
			dumpNode(w, stmt, depth+2)
		}

	case *ast.Raise:
		fmt.Fprintf(w, "%sRaise [line %d]\n", indent, n.Pos().Line)
		if n.Exc != nil {
			fmt.Fprintf(w, "%s  Exc: ", indent)
			dumpExprInline(w, n.Exc)
			fmt.Fprintln(w)
		}

	case *ast.Import:
		fmt.Fprintf(w, "%sImport [line %d]\n", indent, n.Pos().Line)
		for _, alias := range n.Names {
			if alias.AsName != "" {
				fmt.Fprintf(w, "%s  %s as %s\n", indent, alias.Name, alias.AsName)
			} else {
				fmt.Fprintf(w, "%s  %s\n", indent, alias.Name)
			}
		}

	case *ast.ImportFrom:
		fmt.Fprintf(w, "%sImportFrom [line %d]\n", indent, n.Pos().Line)
		fmt.Fprintf(w, "%s  Module: %s\n", indent, n.Module)
		for _, alias := range n.Names {
			if alias.AsName != "" {
				fmt.Fprintf(w, "%s  %s as %s\n", indent, alias.Name, alias.AsName)
			} else {
				fmt.Fprintf(w, "%s  %s\n", indent, alias.Name)
			}
		}

	case *ast.Delete:
		fmt.Fprintf(w, "%sDelete [line %d]\n", indent, n.Pos().Line)
		for _, target := range n.Targets {
			fmt.Fprintf(w, "%s  Target: ", indent)
			dumpExprInline(w, target)
			fmt.Fprintln(w)
		}

	case *ast.Global:
		fmt.Fprintf(w, "%sGlobal [line %d]\n", indent, n.Pos().Line)
		fmt.Fprintf(w, "%s  Names: %s\n", indent, strings.Join(n.Names, ", "))

	case *ast.Pass:
		fmt.Fprintf(w, "%sPass [line %d]\n", indent, n.Pos().Line)

	case *ast.Break:
		fmt.Fprintf(w, "%sBreak [line %d]\n", indent, n.Pos().Line)

	case *ast.Continue:
		fmt.Fprintf(w, "%sContinue [line %d]\n", indent, n.Pos().Line)

	case *ast.Assert:
		fmt.Fprintf(w, "%sAssert [line %d]\n", indent, n.Pos().Line)
		fmt.Fprintf(w, "%s  Test: ", indent)
		dumpExprInline(w, n.Test)
		fmt.Fprintln(w)
		if n.Msg != nil {
			fmt.Fprintf(w, "%s  Msg: ", indent)
			dumpExprInline(w, n.Msg)
			fmt.Fprintln(w)
		}

	default:
		fmt.Fprintf(w, "%s%T [line %d]\n", indent, node, node.Pos().Line)
	}
}

// dumpExprInline writes a compact inline representation of an expression.
func dumpExprInline(w io.Writer, expr ast.Expr) {
	switch e := expr.(type) {
	case *ast.Name:
		fmt.Fprintf(w, "Name(%s)", e.Id)
	case *ast.Constant:
		fmt.Fprintf(w, "Constant(%s)", formatConstant(e.Value))
	case *ast.BinOp:
		fmt.Fprintf(w, "BinOp(")
		dumpExprInline(w, e.Left)
		fmt.Fprintf(w, " %s ", e.Op.String())
		dumpExprInline(w, e.Right)
		fmt.Fprintf(w, ")")
	case *ast.UnaryOp:
		fmt.Fprintf(w, "UnaryOp(%s ", formatUnaryOp(e.Op))
		dumpExprInline(w, e.Operand)
		fmt.Fprintf(w, ")")
	case *ast.BoolOp:
		op := "and"
		if e.Op == ast.Or {
			op = "or"
		}
		fmt.Fprintf(w, "BoolOp(%s", op)
		for _, v := range e.Values {
			fmt.Fprintf(w, " ")
			dumpExprInline(w, v)
		}
		fmt.Fprintf(w, ")")
	case *ast.Compare:
		fmt.Fprintf(w, "Compare(")
		dumpExprInline(w, e.Left)
		for i, op := range e.Ops {
			fmt.Fprintf(w, " %s ", op.String())
			dumpExprInline(w, e.Comparators[i])
		}
		fmt.Fprintf(w, ")")
	case *ast.Call:
		fmt.Fprintf(w, "Call(")
		dumpExprInline(w, e.Func)
		fmt.Fprintf(w, "(")
		for i, arg := range e.Args {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			dumpExprInline(w, arg)
		}
		fmt.Fprintf(w, "))")
	case *ast.Attribute:
		fmt.Fprintf(w, "Attribute(")
		dumpExprInline(w, e.Value)
		fmt.Fprintf(w, ".%s)", e.Attr)
	case *ast.Subscript:
		fmt.Fprintf(w, "Subscript(")
		dumpExprInline(w, e.Value)
		fmt.Fprintf(w, "[")
		dumpExprInline(w, e.Slice)
		fmt.Fprintf(w, "])")
	case *ast.List:
		fmt.Fprintf(w, "List([")
		for i, elt := range e.Elts {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			dumpExprInline(w, elt)
		}
		fmt.Fprintf(w, "])")
	case *ast.Tuple:
		fmt.Fprintf(w, "Tuple((")
		for i, elt := range e.Elts {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			dumpExprInline(w, elt)
		}
		fmt.Fprintf(w, "))")
	case *ast.Dict:
		fmt.Fprintf(w, "Dict({")
		for i, key := range e.Keys {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			dumpExprInline(w, key)
			fmt.Fprintf(w, ": ")
			dumpExprInline(w, e.Values[i])
		}
		fmt.Fprintf(w, "})")
	case *ast.IfExp:
		fmt.Fprintf(w, "IfExp(")
		dumpExprInline(w, e.Body)
		fmt.Fprintf(w, " if ")
		dumpExprInline(w, e.Test)
		fmt.Fprintf(w, " else ")
		dumpExprInline(w, e.Orelse)
		fmt.Fprintf(w, ")")
	default:
		fmt.Fprintf(w, "%T", expr)
	}
}

// dumpArgs writes function arguments.
func dumpArgs(w io.Writer, args *ast.Arguments, depth int) {
	indent := strings.Repeat("  ", depth)
	if len(args.Args) > 0 {
		names := make([]string, len(args.Args))
		for i, a := range args.Args {
			names[i] = a.Arg
		}
		fmt.Fprintf(w, "%s  Args: [%s]\n", indent, strings.Join(names, ", "))
	}
}

// formatUnaryOp returns the string representation of a unary operator.
func formatUnaryOp(op ast.UnaryOperator) string {
	switch op {
	case ast.USub:
		return "-"
	case ast.UAdd:
		return "+"
	case ast.Not:
		return "not"
	case ast.Invert:
		return "~"
	default:
		return "?"
	}
}
