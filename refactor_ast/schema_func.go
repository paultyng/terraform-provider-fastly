package main

import (
	"fmt"
	"go/ast"
)

func generateGetSchemaFunc(attributeHandlerName string, registerFunc *ast.FuncDecl) (*ast.FuncDecl, error) {
	// Generate empty GetSchema function with no statements inside
	schemaFunc := getEmptyGetSchemaFunc(attributeHandlerName)

	var returnStatementAdded = false

	// Loop through statements in Register function
	// If they don't match the pattern `s.Schema[*] = *` then add them verbatim to the new GetSchema function.
	// If they DO match the pattern, then turn it into a return statement and add it to the end of the function
	for _, stmt := range registerFunc.Body.List {
		assignStmt, ok := stmt.(*ast.AssignStmt)
		if ok {
			indexExpr, ok := assignStmt.Lhs[0].(*ast.IndexExpr)
			if ok {
				selectorExpr, ok := indexExpr.X.(*ast.SelectorExpr)
				if ok {
					xIdent, ok := selectorExpr.X.(*ast.Ident)
					if ok {
						if xIdent.String() == "s" && selectorExpr.Sel.String() == "Schema" {
							schemaFunc.Body.List = append(schemaFunc.Body.List, &ast.ReturnStmt{
								Results: assignStmt.Rhs,
							})
							returnStatementAdded = true
							break
						}
					}
				}
			}
		}
		schemaFunc.Body.List = append(schemaFunc.Body.List, stmt)
	}

	if !returnStatementAdded {
		return nil, fmt.Errorf("could not find assignment to s.Schema[h.key] inside Register function")
	}

	return schemaFunc, nil
}

func getEmptyGetSchemaFunc(recv string) *ast.FuncDecl {
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
		Name: ast.NewIdent("GetSchema"),
		Type: &ast.FuncType{
			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: &ast.StarExpr{
							X: &ast.SelectorExpr{
								X:   ast.NewIdent("schema"),
								Sel: ast.NewIdent("Schema"),
							},
						},
					},
				},
			},
		},
		Body: &ast.BlockStmt{},
	}
}
