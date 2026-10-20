package runtime

import (
	"fmt"
	"io"

	"github.com/pygo/pygo/internal/bytecode"
	"github.com/pygo/pygo/internal/parser"
)

// Disassemble compiles source and writes a human-readable disassembly.
func (r *Runtime) Disassemble(source string, w io.Writer) error {
	code, err := r.compile(source, "<disasm>")
	if err != nil {
		return err
	}
	bytecode.Disassemble(w, code)
	return nil
}

// DumpAST parses source and writes a text representation of the AST.
func (r *Runtime) DumpAST(source string, w io.Writer) error {
	p := parser.New("<ast>", source)
	mod, err := p.Parse()
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "Module with %d statement(s):\n", len(mod.Body))
	for i, s := range mod.Body {
		fmt.Fprintf(w, "  [%d] %T @ line %d\n", i, s, s.Pos().Line)
	}
	return nil
}
