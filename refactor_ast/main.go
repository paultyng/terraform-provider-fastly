package main

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"log"
)

const filepathBase = "/Users/bengesoff/code/clients/fastly/terraform-provider-fastly/fastly/"

func main() {
	fileNames := []string{
		"block_fastly_service_v1_blobstoragelogging.go",
	}
	for _, name := range fileNames {
		newFile, err := transformFile(name)
		if err != nil {
			panic(err)
		}

		fmt.Println(newFile)
	}
}

func transformFile(filename string) (string, error) {
	// Open and parse the file
	log.Println("Parsing file", filename)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepathBase+filename, nil, parser.ParseComments)
	if err != nil {
		return "", err
	}

	funcs, err := generateNewCRUDFuncs(file)
	if err != nil {
		return "", err
	}

	log.Println("Generated Create, Update, and Delete functions. Adding them to the file")
	file.Decls = append(file.Decls, funcs["Create"], funcs["Update"], funcs["Delete"])

	var buf bytes.Buffer
	err = format.Node(&buf, fset, file)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
