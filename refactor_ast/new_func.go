package main

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

func modifyNewFunc(newFunc *dst.FuncDecl) {
	dstutil.Apply(newFunc, func(cursor *dstutil.Cursor) bool {
		node := cursor.Node()
		returnStmt, ok := node.(*dst.ReturnStmt)
		if ok {
			cursor.Replace(&dst.ReturnStmt{
				Results: []dst.Expr{
					&dst.CallExpr{
						Fun:  dst.NewIdent("BlockSetToServiceAttributeDefinition"),
						Args: returnStmt.Results,
					},
				},
			})
			return false
		}
		return true
	}, nil)
}
