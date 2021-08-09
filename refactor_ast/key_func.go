package main

import (
	"github.com/dave/dst"
)

// getKeyFunc returns a function that looks like:
// ```
// func (h *recv) Key() string {
//      return h.key
// }
// ```
func getKeyFunc(recv string) *dst.FuncDecl {
	f := &dst.FuncDecl{
		Recv: &dst.FieldList{
			List: []*dst.Field{
				{
					Names: []*dst.Ident{
						dst.NewIdent("h"),
					},
					Type: &dst.StarExpr{
						X: dst.NewIdent(recv),
					},
				},
			},
		},
		Name: dst.NewIdent("Key"),
		Type: &dst.FuncType{
			Results: &dst.FieldList{
				List: []*dst.Field{
					{
						Type: dst.NewIdent("string"),
					},
				},
			},
		},
		Body: &dst.BlockStmt{
			List: []dst.Stmt{
				&dst.ReturnStmt{
					Results: []dst.Expr{
						&dst.SelectorExpr{
							X:   dst.NewIdent("h"),
							Sel: dst.NewIdent("key"),
						},
					},
				},
			},
		},
	}

	f.Decorations().Before = dst.EmptyLine
	return f
}
