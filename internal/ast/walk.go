package ast

// Visitor is implemented by types that wish to traverse an AST.
// Visit is called for each node. If Visit returns nil, the node's children
// are not visited. If Visit returns a non-nil Visitor, that visitor is used
// for the children.
type Visitor interface {
	Visit(node Node) Visitor
}

// Walk traverses an AST in depth-first order, calling v.Visit for each node.
// If v.Visit returns nil for a node, its children are skipped.
func Walk(v Visitor, node Node) {
	if node == nil {
		return
	}
	w := v.Visit(node)
	if w == nil {
		return
	}
	walkChildren(w, node)
}

// walkChildren visits the children of a node.
func walkChildren(v Visitor, node Node) {
	switch n := node.(type) {
	case *Module:
		walkStmts(v, n.Body)

	case *FunctionDef:
		walkExprs(v, n.Decorators)
		walkArgs(v, n.Args)
		if n.Returns != nil {
			Walk(v, n.Returns)
		}
		walkStmts(v, n.Body)

	case *ClassDef:
		walkExprs(v, n.Decorators)
		walkExprs(v, n.Bases)
		walkKeywords(v, n.Keywords)
		walkStmts(v, n.Body)

	case *Return:
		if n.Value != nil {
			Walk(v, n.Value)
		}

	case *Delete:
		walkExprs(v, n.Targets)

	case *Assign:
		walkExprs(v, n.Targets)
		Walk(v, n.Value)

	case *AugAssign:
		Walk(v, n.Target)
		Walk(v, n.Value)

	case *AnnAssign:
		Walk(v, n.Target)
		Walk(v, n.Annotation)
		if n.Value != nil {
			Walk(v, n.Value)
		}

	case *For:
		Walk(v, n.Target)
		Walk(v, n.Iter)
		walkStmts(v, n.Body)
		walkStmts(v, n.Orelse)

	case *While:
		Walk(v, n.Test)
		walkStmts(v, n.Body)
		walkStmts(v, n.Orelse)

	case *If:
		Walk(v, n.Test)
		walkStmts(v, n.Body)
		walkStmts(v, n.Orelse)

	case *With:
		for _, item := range n.Items {
			Walk(v, item.ContextExpr)
			if item.OptionalVar != nil {
				Walk(v, item.OptionalVar)
			}
		}
		walkStmts(v, n.Body)

	case *Raise:
		if n.Exc != nil {
			Walk(v, n.Exc)
		}
		if n.Cause != nil {
			Walk(v, n.Cause)
		}

	case *Try:
		walkStmts(v, n.Body)
		for _, h := range n.Handlers {
			Walk(v, h)
		}
		walkStmts(v, n.Orelse)
		walkStmts(v, n.Finalbody)

	case *ExceptHandler:
		if n.Type != nil {
			Walk(v, n.Type)
		}
		walkStmts(v, n.Body)

	case *Import:
		// no children to walk

	case *ImportFrom:
		// no children to walk

	case *Global, *Nonlocal, *Pass, *Break, *Continue:
		// no children

	case *Assert:
		Walk(v, n.Test)
		if n.Msg != nil {
			Walk(v, n.Msg)
		}

	case *ExprStmt:
		Walk(v, n.Value)

	// Expressions
	case *BoolOp:
		walkExprs(v, n.Values)

	case *BinOp:
		Walk(v, n.Left)
		Walk(v, n.Right)

	case *UnaryOp:
		Walk(v, n.Operand)

	case *IfExp:
		Walk(v, n.Test)
		Walk(v, n.Body)
		Walk(v, n.Orelse)

	case *Lambda:
		walkArgs(v, n.Args)
		Walk(v, n.Body)

	case *NamedExpr:
		Walk(v, n.Target)
		Walk(v, n.Value)

	case *Dict:
		for i, k := range n.Keys {
			if k != nil {
				Walk(v, k)
			}
			Walk(v, n.Values[i])
		}

	case *Set:
		walkExprs(v, n.Elts)

	case *ListComp:
		Walk(v, n.Elt)
		walkComprehensions(v, n.Generators)

	case *SetComp:
		Walk(v, n.Elt)
		walkComprehensions(v, n.Generators)

	case *DictComp:
		Walk(v, n.Key)
		Walk(v, n.Value)
		walkComprehensions(v, n.Generators)

	case *GeneratorExp:
		Walk(v, n.Elt)
		walkComprehensions(v, n.Generators)

	case *Await:
		Walk(v, n.Value)

	case *Yield:
		if n.Value != nil {
			Walk(v, n.Value)
		}

	case *YieldFrom:
		Walk(v, n.Value)

	case *Compare:
		Walk(v, n.Left)
		walkExprs(v, n.Comparators)

	case *Call:
		Walk(v, n.Func)
		walkExprs(v, n.Args)
		walkKeywords(v, n.Keywords)

	case *Attribute:
		Walk(v, n.Value)

	case *Subscript:
		Walk(v, n.Value)
		Walk(v, n.Slice)

	case *Starred:
		Walk(v, n.Value)

	case *Name, *Constant:
		// leaf nodes — no children

	case *List:
		walkExprs(v, n.Elts)

	case *Tuple:
		walkExprs(v, n.Elts)

	case *Slice:
		if n.Lower != nil {
			Walk(v, n.Lower)
		}
		if n.Upper != nil {
			Walk(v, n.Upper)
		}
		if n.Step != nil {
			Walk(v, n.Step)
		}
	}
}

func walkStmts(v Visitor, stmts []Stmt) {
	for _, s := range stmts {
		Walk(v, s)
	}
}

func walkExprs(v Visitor, exprs []Expr) {
	for _, e := range exprs {
		Walk(v, e)
	}
}

func walkArgs(v Visitor, args *Arguments) {
	if args == nil {
		return
	}
	for _, a := range args.PosonlyArgs {
		if a.Annotation != nil {
			Walk(v, a.Annotation)
		}
	}
	for _, a := range args.Args {
		if a.Annotation != nil {
			Walk(v, a.Annotation)
		}
	}
	walkExprs(v, args.Defaults)
	for _, a := range args.KwonlyArgs {
		if a.Annotation != nil {
			Walk(v, a.Annotation)
		}
	}
	for _, d := range args.KwDefaults {
		if d != nil {
			Walk(v, d)
		}
	}
}

func walkKeywords(v Visitor, kws []*Keyword) {
	for _, kw := range kws {
		Walk(v, kw.Value)
	}
}

func walkComprehensions(v Visitor, comps []*Comprehension) {
	for _, c := range comps {
		Walk(v, c.Target)
		Walk(v, c.Iter)
		walkExprs(v, c.Ifs)
	}
}
