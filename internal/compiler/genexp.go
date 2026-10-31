package compiler

import (
	"github.com/alvinjaison/PyGo/internal/ast"
	"github.com/alvinjaison/PyGo/internal/bytecode"
)

// genexpIterName is the synthetic parameter that receives the first iterable's
// iterator, mirroring CPython's ".0" argument for generator expressions.
const genexpIterName = ".0"

// genexpFunction desugars a generator expression into the parameter list and
// body of an equivalent generator function:
//
//	(elt for t1 in it1 if c1 for t2 in it2)
//
// becomes
//
//	def <genexpr>(.0):
//	    for t1 in .0:
//	        if c1:
//	            for t2 in it2:
//	                yield elt
//
// The first iterable is evaluated eagerly in the enclosing scope and passed in
// as the argument; everything else runs lazily inside the generator.
func genexpFunction(elt ast.Expr, gens []*ast.Comprehension, pos ast.Position) (*ast.Arguments, []ast.Stmt) {
	var inner ast.Stmt = ast.SetPos(&ast.ExprStmt{
		Value: ast.SetPos(&ast.Yield{Value: elt}, pos),
	}, pos)

	for i := len(gens) - 1; i >= 0; i-- {
		g := gens[i]
		body := []ast.Stmt{inner}
		for j := len(g.Ifs) - 1; j >= 0; j-- {
			body = []ast.Stmt{ast.SetPos(&ast.If{Test: g.Ifs[j], Body: body}, pos)}
		}
		iter := g.Iter
		if i == 0 {
			iter = ast.SetPos(&ast.Name{Id: genexpIterName, Ctx: ast.Load}, pos)
		}
		inner = ast.SetPos(&ast.For{Target: g.Target, Iter: iter, Body: body}, pos)
	}

	args := &ast.Arguments{Args: []*ast.Arg{ast.NewArg(genexpIterName, nil, pos)}}
	return args, []ast.Stmt{inner}
}

// compileGeneratorExp emits a lazy generator expression: it builds the
// synthetic generator function, evaluates the first iterable in the enclosing
// scope, and calls the function with that iterable's iterator.
func (c *compiler) compileGeneratorExp(n *ast.GeneratorExp) {
	if len(n.Generators) == 0 {
		c.fail(n.Pos().Line, "generator expression without a for clause")
		return
	}
	args, body := genexpFunction(n.Elt, n.Generators, n.Pos())
	c.compileFunctionObject("<genexpr>", args, body)
	c.compileExpr(n.Generators[0].Iter)
	c.emit(bytecode.GET_ITER, 0)
	c.emit(bytecode.CALL_FUNCTION, 1)
}
