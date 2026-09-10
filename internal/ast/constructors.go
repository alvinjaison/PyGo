// Package ast constructors for building AST nodes from the parser.
// These functions set the unexported position field on AST nodes.
package ast

// NewBinOp creates a BinOp node.
func NewBinOp(left Expr, op Operator, right Expr, pos Position) *BinOp {
	return &BinOp{baseExpr: baseExpr{baseNode{pos}}, Left: left, Op: op, Right: right}
}

// NewUnaryOp creates a UnaryOp node.
func NewUnaryOp(op UnaryOperator, operand Expr, pos Position) *UnaryOp {
	return &UnaryOp{baseExpr: baseExpr{baseNode{pos}}, Op: op, Operand: operand}
}

// NewBoolOp creates a BoolOp node.
func NewBoolOp(op BoolOperator, values []Expr, pos Position) *BoolOp {
	return &BoolOp{baseExpr: baseExpr{baseNode{pos}}, Op: op, Values: values}
}

// NewCompare creates a Compare node.
func NewCompare(left Expr, ops []CmpOp, comparators []Expr, pos Position) *Compare {
	return &Compare{baseExpr: baseExpr{baseNode{pos}}, Left: left, Ops: ops, Comparators: comparators}
}

// NewCall creates a Call node.
func NewCall(fn Expr, args []Expr, keywords []*Keyword, pos Position) *Call {
	return &Call{baseExpr: baseExpr{baseNode{pos}}, Func: fn, Args: args, Keywords: keywords}
}

// NewName creates a Name node.
func NewName(id string, ctx ExprContext, pos Position) *Name {
	return &Name{baseExpr: baseExpr{baseNode{pos}}, Id: id, Ctx: ctx}
}

// NewConstant creates a Constant node.
func NewConstant(value interface{}, kind string, pos Position) *Constant {
	return &Constant{baseExpr: baseExpr{baseNode{pos}}, Value: value, Kind: kind}
}

// NewAttribute creates an Attribute node.
func NewAttribute(value Expr, attr string, ctx ExprContext, pos Position) *Attribute {
	return &Attribute{baseExpr: baseExpr{baseNode{pos}}, Value: value, Attr: attr, Ctx: ctx}
}

// NewSubscript creates a Subscript node.
func NewSubscript(value Expr, slice Expr, ctx ExprContext, pos Position) *Subscript {
	return &Subscript{baseExpr: baseExpr{baseNode{pos}}, Value: value, Slice: slice, Ctx: ctx}
}

// NewList creates a List node.
func NewList(elts []Expr, ctx ExprContext, pos Position) *List {
	return &List{baseExpr: baseExpr{baseNode{pos}}, Elts: elts, Ctx: ctx}
}

// NewTuple creates a Tuple node.
func NewTuple(elts []Expr, ctx ExprContext, pos Position) *Tuple {
	return &Tuple{baseExpr: baseExpr{baseNode{pos}}, Elts: elts, Ctx: ctx}
}

// NewDict creates a Dict node.
func NewDict(keys, values []Expr, pos Position) *Dict {
	return &Dict{baseExpr: baseExpr{baseNode{pos}}, Keys: keys, Values: values}
}

// NewIfExp creates an IfExp node (ternary expression).
func NewIfExp(test, body, orelse Expr, pos Position) *IfExp {
	return &IfExp{baseExpr: baseExpr{baseNode{pos}}, Test: test, Body: body, Orelse: orelse}
}

// NewExprStmt creates an ExprStmt node.
func NewExprStmt(value Expr, pos Position) *ExprStmt {
	return &ExprStmt{baseStmt: baseStmt{baseNode{pos}}, Value: value}
}

// NewAssign creates an Assign node.
func NewAssign(targets []Expr, value Expr, pos Position) *Assign {
	return &Assign{baseStmt: baseStmt{baseNode{pos}}, Targets: targets, Value: value}
}

// NewAugAssign creates an AugAssign node.
func NewAugAssign(target Expr, op Operator, value Expr, pos Position) *AugAssign {
	return &AugAssign{baseStmt: baseStmt{baseNode{pos}}, Target: target, Op: op, Value: value}
}

// NewFunctionDef creates a FunctionDef node.
func NewFunctionDef(name string, args *Arguments, body []Stmt, decorators []Expr, returns Expr, pos Position) *FunctionDef {
	return &FunctionDef{
		baseStmt:   baseStmt{baseNode{pos}},
		Name:       name,
		Args:       args,
		Body:       body,
		Decorators: decorators,
		Returns:    returns,
	}
}

// NewReturn creates a Return node.
func NewReturn(value Expr, pos Position) *Return {
	return &Return{baseStmt: baseStmt{baseNode{pos}}, Value: value}
}

// NewIf creates an If node.
func NewIf(test Expr, body, orelse []Stmt, pos Position) *If {
	return &If{baseStmt: baseStmt{baseNode{pos}}, Test: test, Body: body, Orelse: orelse}
}

// NewWhile creates a While node.
func NewWhile(test Expr, body, orelse []Stmt, pos Position) *While {
	return &While{baseStmt: baseStmt{baseNode{pos}}, Test: test, Body: body, Orelse: orelse}
}

// NewFor creates a For node.
func NewFor(target, iter Expr, body, orelse []Stmt, pos Position) *For {
	return &For{baseStmt: baseStmt{baseNode{pos}}, Target: target, Iter: iter, Body: body, Orelse: orelse}
}

// NewPass creates a Pass node.
func NewPass(pos Position) *Pass {
	return &Pass{baseStmt: baseStmt{baseNode{pos}}}
}

// NewBreak creates a Break node.
func NewBreak(pos Position) *Break {
	return &Break{baseStmt: baseStmt{baseNode{pos}}}
}

// NewContinue creates a Continue node.
func NewContinue(pos Position) *Continue {
	return &Continue{baseStmt: baseStmt{baseNode{pos}}}
}

// NewRaise creates a Raise node.
func NewRaise(exc, cause Expr, pos Position) *Raise {
	return &Raise{baseStmt: baseStmt{baseNode{pos}}, Exc: exc, Cause: cause}
}

// NewTry creates a Try node.
func NewTry(body []Stmt, handlers []*ExceptHandler, orelse, finalbody []Stmt, pos Position) *Try {
	return &Try{baseStmt: baseStmt{baseNode{pos}}, Body: body, Handlers: handlers, Orelse: orelse, Finalbody: finalbody}
}

// NewExceptHandler creates an ExceptHandler node.
func NewExceptHandler(typ Expr, name string, body []Stmt, pos Position) *ExceptHandler {
	return &ExceptHandler{baseStmt: baseStmt{baseNode{pos}}, Type: typ, Name: name, Body: body}
}

// NewImport creates an Import node.
func NewImport(names []*Alias, pos Position) *Import {
	return &Import{baseStmt: baseStmt{baseNode{pos}}, Names: names}
}

// NewImportFrom creates an ImportFrom node.
func NewImportFrom(module string, names []*Alias, level int, pos Position) *ImportFrom {
	return &ImportFrom{baseStmt: baseStmt{baseNode{pos}}, Module: module, Names: names, Level: level}
}

// NewDelete creates a Delete node.
func NewDelete(targets []Expr, pos Position) *Delete {
	return &Delete{baseStmt: baseStmt{baseNode{pos}}, Targets: targets}
}

// NewGlobal creates a Global node.
func NewGlobal(names []string, pos Position) *Global {
	return &Global{baseStmt: baseStmt{baseNode{pos}}, Names: names}
}

// NewNonlocal creates a Nonlocal node.
func NewNonlocal(names []string, pos Position) *Nonlocal {
	return &Nonlocal{baseStmt: baseStmt{baseNode{pos}}, Names: names}
}

// NewArg creates an Arg node.
func NewArg(name string, annotation Expr, pos Position) *Arg {
	return &Arg{Arg: name, Annotation: annotation, pos: pos}
}

// NewSet creates a Set node.
func NewSet(elts []Expr, pos Position) *Set {
	return &Set{baseExpr: baseExpr{baseNode{pos}}, Elts: elts}
}

// NewSlice creates a Slice node.
func NewSlice(lower, upper, step Expr, pos Position) *Slice {
	return &Slice{baseExpr: baseExpr{baseNode{pos}}, Lower: lower, Upper: upper, Step: step}
}

// NewStarred creates a Starred node.
func NewStarred(value Expr, ctx ExprContext, pos Position) *Starred {
	return &Starred{baseExpr: baseExpr{baseNode{pos}}, Value: value, Ctx: ctx}
}
