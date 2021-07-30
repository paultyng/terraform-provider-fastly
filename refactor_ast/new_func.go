package main

import (
	"go/ast"

	"golang.org/x/tools/go/ast/astutil"
)

func modifyNewFunc(newFunc *ast.FuncDecl) {
	astutil.Apply(newFunc, func(cursor *astutil.Cursor) bool {
		node := cursor.Node()
		returnStmt, ok := node.(*ast.ReturnStmt)
		if ok {
			cursor.Replace(&ast.ReturnStmt{
				Results: []ast.Expr{
					&ast.CallExpr{
						Fun:  ast.NewIdent("BlockSetToServiceAttributeDefinition"),
						Args: returnStmt.Results,
					},
				},
			})
			return false
		}
		return true
	}, nil)
}
