package main

import (
	"go/ast"

	"golang.org/x/tools/go/ast/astutil"
)

func modifyReadFunc(readFunc *ast.FuncDecl) {
	readFunc.Type.Params.List = []*ast.Field{
		{
			Names: []*ast.Ident{
				ast.NewIdent("_"),
			},
			Type: &ast.SelectorExpr{
				X:   ast.NewIdent("context"),
				Sel: ast.NewIdent("Context"),
			},
		},
		{
			Names: []*ast.Ident{
				ast.NewIdent("d"),
			},
			Type: &ast.StarExpr{
				X: &ast.SelectorExpr{
					X:   ast.NewIdent("schema"),
					Sel: ast.NewIdent("ResourceData"),
				},
			},
		},
		{
			Names: []*ast.Ident{
				ast.NewIdent("_"),
			},
			Type: &ast.MapType{
				Key:   ast.NewIdent("string"),
				Value: ast.NewIdent("interface{}"),
			},
		},
		{
			Names: []*ast.Ident{
				ast.NewIdent("serviceVersion"),
			},
			Type: ast.NewIdent("int"),
		},
		{
			Names: []*ast.Ident{
				ast.NewIdent("conn"),
			},
			Type: &ast.StarExpr{
				X: &ast.SelectorExpr{
					X:   ast.NewIdent("gofastly"),
					Sel: ast.NewIdent("Client"),
				},
			},
		},
	}

	astutil.Apply(readFunc, func(cursor *astutil.Cursor) bool {
		node := cursor.Node()
		parentSelector, ok := node.(*ast.SelectorExpr)
		if ok {
			selector, ok := parentSelector.X.(*ast.SelectorExpr)
			if ok {
				ident, ok := selector.X.(*ast.Ident)
				if ok {
					if ident.String() == "s" {
						if selector.Sel.String() != "ActiveVersion" {
							panic(selector)
						}

						cursor.Replace(ast.NewIdent("serviceVersion"))
					}
				}
			}
		}
		return true
	}, nil)
}
