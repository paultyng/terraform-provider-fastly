package main

import (
	"fmt"

	"github.com/dave/dst"
)

func generateGetSchemaFunc(attributeHandlerName string, registerFunc *dst.FuncDecl) (dst.Decl, error) {
	// Generate empty GetSchema function with no statements inside
	schemaFunc := getEmptyGetSchemaFunc(attributeHandlerName)
	schemaFunc.Decorations().Before = dst.EmptyLine

	var returnStatementAdded = false

	// Loop through statements in Register function
	// If they don't match the pattern `s.Schema[*] = *` then add them verbatim to the new GetSchema function.
	// If they DO match the pattern, then turn it into a return statement and add it to the end of the function
	for _, stmt := range registerFunc.Body.List {
		assignStmt, ok := stmt.(*dst.AssignStmt)
		if ok {
			indexExpr, ok := assignStmt.Lhs[0].(*dst.IndexExpr)
			if ok {
				selectorExpr, ok := indexExpr.X.(*dst.SelectorExpr)
				if ok {
					xIdent, ok := selectorExpr.X.(*dst.Ident)
					if ok {
						if xIdent.String() == "s" && selectorExpr.Sel.String() == "Schema" {
							assignment := dst.Clone(assignStmt).(*dst.AssignStmt)
							schemaFunc.Body.List = append(schemaFunc.Body.List, &dst.ReturnStmt{
								Results: assignment.Rhs,
							})
							returnStatementAdded = true
							break
						}
					}
				}
			}
		}
		schemaFunc.Body.List = append(schemaFunc.Body.List, dst.Clone(stmt).(dst.Stmt))
	}

	if !returnStatementAdded {
		return nil, fmt.Errorf("could not find assignment to s.Schema[h.key] inside Register function")
	}

	return schemaFunc, nil
}

func getEmptyGetSchemaFunc(recv string) *dst.FuncDecl {
	return &dst.FuncDecl{
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
		Name: dst.NewIdent("GetSchema"),
		Type: &dst.FuncType{
			Results: &dst.FieldList{
				List: []*dst.Field{
					{
						Type: &dst.StarExpr{
							X: &dst.SelectorExpr{
								X:   dst.NewIdent("schema"),
								Sel: dst.NewIdent("Schema"),
							},
						},
					},
				},
			},
		},
		Body: &dst.BlockStmt{},
	}
}
