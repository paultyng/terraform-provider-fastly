package main

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

func modifyReadFunc(readFunc *dst.FuncDecl) {
	readFunc.Type.Params.List = []*dst.Field{
		{
			Names: []*dst.Ident{
				dst.NewIdent("_"),
			},
			Type: &dst.SelectorExpr{
				X:   dst.NewIdent("context"),
				Sel: dst.NewIdent("Context"),
			},
		},
		{
			Names: []*dst.Ident{
				dst.NewIdent("d"),
			},
			Type: &dst.StarExpr{
				X: &dst.SelectorExpr{
					X:   dst.NewIdent("schema"),
					Sel: dst.NewIdent("ResourceData"),
				},
			},
		},
		{
			Names: []*dst.Ident{
				dst.NewIdent("_"),
			},
			Type: &dst.MapType{
				Key:   dst.NewIdent("string"),
				Value: dst.NewIdent("interface{}"),
			},
		},
		{
			Names: []*dst.Ident{
				dst.NewIdent("serviceVersion"),
			},
			Type: dst.NewIdent("int"),
		},
		{
			Names: []*dst.Ident{
				dst.NewIdent("conn"),
			},
			Type: &dst.StarExpr{
				X: &dst.SelectorExpr{
					X:   dst.NewIdent("gofastly"),
					Sel: dst.NewIdent("Client"),
				},
			},
		},
	}

	dstutil.Apply(readFunc, func(cursor *dstutil.Cursor) bool {
		node := cursor.Node()
		parentSelector, ok := node.(*dst.SelectorExpr)
		if ok {
			selector, ok := parentSelector.X.(*dst.SelectorExpr)
			if ok {
				ident, ok := selector.X.(*dst.Ident)
				if ok {
					if ident.String() == "s" {
						if selector.Sel.String() != "ActiveVersion" {
							panic(selector)
						}

						cursor.Replace(dst.NewIdent("serviceVersion"))
					}
				}
			}
		}
		return true
	}, nil)
}
