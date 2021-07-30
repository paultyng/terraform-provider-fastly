package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
)

const filepathBase = "/Users/bengesoff/code/clients/fastly/terraform-provider-fastly/"

func main() {
	fileNames := []string{
		"fastly/block_fastly_service_v1_blobstoragelogging.go",
		"fastly/block_fastly_service_v1_cachesetting.go",
		"fastly/block_fastly_service_v1_condition.go",
		"fastly/block_fastly_service_v1_dictionary.go",
		"fastly/block_fastly_service_v1_director.go",
		"fastly/block_fastly_service_v1_domain.go",
		"fastly/block_fastly_service_v1_dynamicsnippet.go",
		"fastly/block_fastly_service_v1_gcslogging.go",
		"fastly/block_fastly_service_v1_gzip.go",
		"fastly/block_fastly_service_v1_header.go",
		"fastly/block_fastly_service_v1_healthcheck.go",
		"fastly/block_fastly_service_v1_httpslogging.go",
		"fastly/block_fastly_service_v1_logentries.go",
		"fastly/block_fastly_service_v1_logging_cloudfiles.go",
		"fastly/block_fastly_service_v1_logging_datadog.go",
		"fastly/block_fastly_service_v1_logging_digitalocean.go",
		"fastly/block_fastly_service_v1_logging_elasticsearch.go",
		"fastly/block_fastly_service_v1_logging_ftp.go",
		"fastly/block_fastly_service_v1_logging_googlepubsub.go",
		"fastly/block_fastly_service_v1_logging_heroku.go",
		"fastly/block_fastly_service_v1_logging_honeycomb.go",
		"fastly/block_fastly_service_v1_logging_kafka.go",
		"fastly/block_fastly_service_v1_logging_kinesis.go",
		"fastly/block_fastly_service_v1_logging_loggly.go",
		"fastly/block_fastly_service_v1_logging_logshuttle.go",
		"fastly/block_fastly_service_v1_logging_newrelic.go",
		"fastly/block_fastly_service_v1_logging_openstack.go",
		"fastly/block_fastly_service_v1_logging_scalyr.go",
		"fastly/block_fastly_service_v1_logging_sftp.go",
		"fastly/block_fastly_service_v1_package.go",
		"fastly/block_fastly_service_v1_papertrail.go",
		"fastly/block_fastly_service_v1_requestsetting.go",
		"fastly/block_fastly_service_v1_responseobject.go",
		"fastly/block_fastly_service_v1_s3logging.go",
		"fastly/block_fastly_service_v1_snippet.go",
		"fastly/block_fastly_service_v1_splunk.go",
		"fastly/block_fastly_service_v1_sumologic.go",
		"fastly/block_fastly_service_v1_syslog.go",
		"fastly/block_fastly_service_v1_vcl.go",
	}
	for _, name := range fileNames {
		newFile, err := transformFile(filepathBase + name)
		if err != nil {
			panic(err)
		}

		fmt.Println(newFile)
	}
}

func transformFile(filename string) (string, error) {
	// Open and parse the file
	log.Println("Parsing file", filename)
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filename, nil, parser.ParseComments)
	if err != nil {
		return "", err
	}

	// Search file for a function named Process
	processFunc := findFunc(file, "Process")
	if processFunc == nil {
		return "", fmt.Errorf("no Process function found in %s", file.Name)
	}

	registerFunc := findFunc(file, "Register")
	if registerFunc == nil {
		return "", fmt.Errorf("no Register function found in %s", file.Name)
	}

	readFunc := findFunc(file, "Read")
	if readFunc == nil {
		return "", fmt.Errorf("no Read function found in %s", file.Name)
	}

	// Extract the method receiver to get the name of the struct for the AttributeDefinition
	attributeHandlerName := getFuncRecv(processFunc)
	log.Println("Found Process func with receiver", attributeHandlerName)
	log.Println("Found Register func")
	log.Println("Found Read func")

	crudFunctions, err := generateNewCRUDFunctions(attributeHandlerName, processFunc)
	if err != nil {
		return "", err
	}

	log.Println("Generated Create, Update, and Delete functions. Adding them to the file")
	file.Decls = append(file.Decls, crudFunctions["Create"], crudFunctions["Update"], crudFunctions["Delete"])

	log.Println("Adding Key function")
	file.Decls = append(file.Decls, getKeyFunc(attributeHandlerName))

	getSchemaFunc, err := generateGetSchemaFunc(attributeHandlerName, registerFunc)
	if err != nil {
		return "", err
	}

	log.Println("Generated GetSchema function. Adding it to the file")
	file.Decls = append(file.Decls, getSchemaFunc)

	modifyReadFunc(readFunc)
	log.Println("Modified Read function")

	var buf bytes.Buffer
	err = format.Node(&buf, fileSet, file)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// Loop through file and find a function with a given name
func findFunc(file *ast.File, name string) *ast.FuncDecl {
	var processFunc *ast.FuncDecl
	for _, decl := range file.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if function.Name.Name == name {
			processFunc = function
		}
	}
	return processFunc
}
