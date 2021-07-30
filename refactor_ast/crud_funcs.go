package main

import (
	"fmt"
	"go/ast"
	"log"

	"golang.org/x/tools/go/ast/astutil"
)

func generateNewCRUDFuncs(file *ast.File) (map[string]*ast.FuncDecl, error) {
	// Search file for a function named Process
	processFunc := findProcessFunc(file)
	if processFunc == nil {
		return nil, fmt.Errorf("no Process function found in %s", file.Name)
	}

	// Extract the method receiver to get the name of the struct for the AttributeDefinition
	attributeHandlerName := getFuncRecv(processFunc)
	log.Println("Found Process func with receiver", attributeHandlerName)

	// Create new functions for the missing CRUD methods
	funcs := map[string]*ast.FuncDecl{}
	funcs["Create"] = newCRUDFunc(attributeHandlerName, "Create")
	funcs["Update"] = newCRUDFunc(attributeHandlerName, "Update")
	funcs["Delete"] = newCRUDFunc(attributeHandlerName, "Delete")

	// Search the Process function for For loops, to pull the bodies into the CRUD functions
	// For loops should be of the format "for x, y := range diffResult.z {"
	ast.Inspect(processFunc, func(node ast.Node) bool {
		forLoop, ok := node.(*ast.RangeStmt)
		if ok {
			selector, ok := forLoop.X.(*ast.SelectorExpr)
			if ok {
				firstBitOfSelector, ok := selector.X.(*ast.Ident)
				if ok {
					// Check we've got "for x, y := range diffResult.z {"
					if firstBitOfSelector.String() == "diffResult" {
						log.Printf("Found for loop for diffResult.%s in Process func\n", selector.Sel.String())
						// Modify body and add it to the corresponding function
						funcBody := tweakFuncBody(forLoop.Body)
						funcs[getFuncName(selector.Sel.String())].Body = funcBody
					}
				}
			}
		}
		return true
	})

	// If any of the loops weren't found, populate the function with "return nil"
	for _, f := range funcs {
		if f.Body == nil {
			f.Body = &ast.BlockStmt{
				List: []ast.Stmt{
					getReturnNilStmt(),
				},
			}
		}
	}

	return funcs, nil
}

// Loop through file and find a function named Process
func findProcessFunc(file *ast.File) *ast.FuncDecl {
	var processFunc *ast.FuncDecl
	for _, decl := range file.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if function.Name.Name == "Process" {
			processFunc = function
		}
	}
	return processFunc
}

// Makes requisite tweaks to the body of the For loop to adapt it to being a function body
func tweakFuncBody(body *ast.BlockStmt) *ast.BlockStmt {
	// Remove the first line (usually an unneeded type cast of the `resource` variable)
	forBody := body.List[1:]

	// Add a return nil statement to the end
	forBody = append(forBody, getReturnNilStmt())

	// Delete any statements declaring a variable called `modified`
	var funcBody = &ast.BlockStmt{}
	for _, stmt := range forBody {
		assignment, ok := stmt.(*ast.AssignStmt)
		if ok {
			identifier, ok := assignment.Lhs[0].(*ast.Ident)
			if ok {
				if identifier.String() == "modified" {
					continue
				}
			}
		}
		funcBody.List = append(funcBody.List, stmt)
	}

	// Rename any references to `latestVersion` to `serviceVersion`
	astutil.Apply(funcBody, func(cursor *astutil.Cursor) bool {
		node := cursor.Node()
		identifier, ok := node.(*ast.Ident)
		if ok {
			if identifier.String() == "latestVersion" {
				cursor.Replace(ast.NewIdent("serviceVersion"))
			}
		}
		return true
	}, nil)

	return funcBody
}

// Extract the name of the function receiver, e.g. `func (x *thisBit) Name() {}`
func getFuncRecv(f *ast.FuncDecl) string {
	if f.Recv.NumFields() == 0 {
		return ""
	}

	star, ok := f.Recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return ""
	}

	ident, ok := star.X.(*ast.Ident)
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
func newCRUDFunc(recv, name string) *ast.FuncDecl {
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
		Name: &ast.Ident{Name: name},
		Type: &ast.FuncType{
			Params: getFuncParams(name),
			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: &ast.Ident{Name: "error"},
					},
				},
			},
		},
	}
}

// Generate the necessary function parameters for newCRUDFunc
func getFuncParams(name string) *ast.FieldList {
	if name == "Update" {
		return &ast.FieldList{
			List: []*ast.Field{
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
						ast.NewIdent("resource"),
						ast.NewIdent("modified"),
					},
					Type: &ast.MapType{
						Key: ast.NewIdent("string"),
						Value: &ast.InterfaceType{
							Methods: &ast.FieldList{},
						},
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
			},
		}
	}

	return &ast.FieldList{
		List: []*ast.Field{
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
					ast.NewIdent("resource"),
				},
				Type: &ast.MapType{
					Key: ast.NewIdent("string"),
					Value: &ast.InterfaceType{
						Methods: &ast.FieldList{},
					},
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
		},
	}
}

func getReturnNilStmt() ast.Stmt {
	return &ast.ReturnStmt{
		Results: []ast.Expr{
			ast.NewIdent("nil"),
		},
	}
}
