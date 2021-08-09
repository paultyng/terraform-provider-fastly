package main

import (
	"log"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

func generateNewCRUDFunctions(attributeHandlerName string, processFunc *dst.FuncDecl) (map[string]*dst.FuncDecl, error) {
	// Create new functions for the missing CRUD methods
	functions := map[string]*dst.FuncDecl{}
	functions["Create"] = newCRUDFunc(attributeHandlerName, "Create")
	functions["Update"] = newCRUDFunc(attributeHandlerName, "Update")
	functions["Delete"] = newCRUDFunc(attributeHandlerName, "Delete")

	// Search the Process function for For loops, to pull the bodies into the CRUD functions
	// For loops should be of the format "for x, y := range diffResult.z {"
	dst.Inspect(processFunc, func(node dst.Node) bool {
		forLoop, ok := node.(*dst.RangeStmt)
		if ok {
			selector, ok := forLoop.X.(*dst.SelectorExpr)
			if ok {
				firstBitOfSelector, ok := selector.X.(*dst.Ident)
				if ok {
					// Check we've got "for x, y := range diffResult.z {"
					if firstBitOfSelector.String() == "diffResult" {
						log.Printf("Found for loop for diffResult.%s in Process func\n", selector.Sel.String())
						// Modify body and add it to the corresponding function
						newBody := dst.Clone(forLoop.Body).(*dst.BlockStmt)
						funcBody := tweakFuncBody(newBody)
						functions[getFuncName(selector.Sel.String())].Body = funcBody
					}
				}
			}
		}
		return true
	})

	for _, f := range functions {
		// If any of the loops weren't found, populate the function with "return nil"
		if f.Body == nil {
			f.Body = &dst.BlockStmt{
				List: []dst.Stmt{
					getReturnNilStmt(),
				},
			}
		}

		// Ensure there is a blank line before each function
		f.Decorations().Before = dst.EmptyLine
	}

	return functions, nil
}

// Makes requisite tweaks to the body of the For loop to adapt it to being a function body
func tweakFuncBody(body *dst.BlockStmt) *dst.BlockStmt {
	// Remove the first line (usually an unneeded type cast of the `resource` variable)
	forBody := body.List[1:]

	// Ensure there is no random blank line at the start of the body
	forBody[0].Decorations().Before = dst.NewLine

	// Add a return nil statement to the end
	forBody = append(forBody, getReturnNilStmt())

	// Delete any statements declaring a variable called `modified`
	var funcBody = &dst.BlockStmt{}
	for _, stmt := range forBody {
		assignment, ok := stmt.(*dst.AssignStmt)
		if ok {
			identifier, ok := assignment.Lhs[0].(*dst.Ident)
			if ok {
				if identifier.String() == "modified" {
					continue
				}
			}
		}
		funcBody.List = append(funcBody.List, stmt)
	}

	// Rename any references to `latestVersion` to `serviceVersion`
	dstutil.Apply(funcBody, func(cursor *dstutil.Cursor) bool {
		node := cursor.Node()
		identifier, ok := node.(*dst.Ident)
		if ok {
			if identifier.String() == "latestVersion" {
				cursor.Replace(dst.NewIdent("serviceVersion"))
			}
		}
		return true
	}, nil)

	return funcBody
}

// Extract the name of the function receiver, e.g. `func (x *thisBit) Name() {}`
func getFuncRecv(f *dst.FuncDecl) string {
	if f.Recv.NumFields() == 0 {
		return ""
	}

	star, ok := f.Recv.List[0].Type.(*dst.StarExpr)
	if !ok {
		return ""
	}

	ident, ok := star.X.(*dst.Ident)
	if !ok {
		return ""
	}

	return ident.String()
}

// Map the names of diffResult's members to the relevant CRUD names
func getFuncName(sel string) string {
	switch sel {
	case "Added":
		return "Create"
	case "Deleted":
		return "Delete"
	case "Modified":
		return "Update"
	default:
		return "other"
	}
}

// Create a new function declaration with the necessary signature
func newCRUDFunc(recv, name string) *dst.FuncDecl {
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
		Name: dst.NewIdent(name),
		Type: &dst.FuncType{
			Params: getFuncParams(name),
			Results: &dst.FieldList{
				List: []*dst.Field{
					{
						Type: dst.NewIdent("error"),
					},
				},
			},
		},
	}
}

// Generate the necessary function parameters for newCRUDFunc
func getFuncParams(name string) *dst.FieldList {
	if name == "Update" {
		return &dst.FieldList{
			List: []*dst.Field{
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
						dst.NewIdent("resource"),
						dst.NewIdent("modified"),
					},
					Type: &dst.MapType{
						Key: dst.NewIdent("string"),
						Value: &dst.InterfaceType{
							Methods: &dst.FieldList{},
						},
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
			},
		}
	}

	return &dst.FieldList{
		List: []*dst.Field{
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
					dst.NewIdent("resource"),
				},
				Type: &dst.MapType{
					Key: dst.NewIdent("string"),
					Value: &dst.InterfaceType{
						Methods: &dst.FieldList{},
					},
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
		},
	}
}

func getReturnNilStmt() dst.Stmt {
	return &dst.ReturnStmt{
		Results: []dst.Expr{
			dst.NewIdent("nil"),
		},
	}
}
