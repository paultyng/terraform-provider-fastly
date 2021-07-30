package main

import "go/ast"

// getKeyFunc returns a function that looks like:
// ```
// func (h *recv) Key() string {
//      return h.key
// }
// ```
func getKeyFunc(recv string) *ast.FuncDecl {
	return &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{
						ast.NewIdent("h"),
					},
					Type: &ast.StarExpr{
						X: ast.NewIdent(recv),
					},
				},
			},
		},
		Name: ast.NewIdent("Key"),
		Type: &ast.FuncType{
			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: ast.NewIdent("string"),
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.SelectorExpr{
							X:   ast.NewIdent("h"),
							Sel: ast.NewIdent("key"),
						},
					},
				},
			},
		},
	}
}
