package main

import (
	"bytes"
	"fmt"
	"go/parser"
	"log"
	"os"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
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
		path := filepathBase + name
		newFileContents, err := transformFile(path)
		if err != nil {
			panic(err)
		}

		log.Println("Opening file for writing new contents", path)
		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
		if err != nil {
			panic(err)
		}

		log.Println("Writing contents to file")
		_, err = file.WriteString(newFileContents)
		if err != nil {
			panic(err)
		}
		log.Println("Finished processing", path)
	}
}

func transformFile(filename string) (string, error) {
	// Open and parse the file
	log.Println("Parsing file", filename)

	dec := decorator.NewDecorator(nil)
	file, err := dec.ParseFile(filename, nil, parser.ParseComments)
	if err != nil {
		return "", err
	}

	// Search file for a function named Process
	processFunc := findFunc(file, "Process")
	if processFunc == nil {
		return "", fmt.Errorf("no Process function found in %s", filename)
	}

	registerFunc := findFunc(file, "Register")
	if registerFunc == nil {
		return "", fmt.Errorf("no Register function found in %s", filename)
	}

	readFunc := findFunc(file, "Read")
	if readFunc == nil {
		return "", fmt.Errorf("no Read function found in %s", filename)
	}

	newFunc := findFunc(file, "NewService")
	if newFunc == nil {
		return "", fmt.Errorf("no NewService* function found in %s", filename)
	}

	// Extract the method receiver to get the name of the struct for the AttributeDefinition
	attributeHandlerName := getFuncRecv(processFunc)
	log.Println("Found Process func with receiver", attributeHandlerName)
	log.Println("Found Register func")
	log.Println("Found Read func")
	log.Println("Found New* func")

	crudFunctions, err := generateNewCRUDFunctions(attributeHandlerName, processFunc)
	if err != nil {
		return "", err
	}

	log.Println("Generated Create, Update, and Delete functions")

	keyFunc := getKeyFunc(attributeHandlerName)
	log.Println("Generated Key function")

	getSchemaFunc, err := generateGetSchemaFunc(attributeHandlerName, registerFunc)
	if err != nil {
		return "", err
	}
	log.Println("Generated GetSchema function")

	modifyReadFunc(readFunc)
	log.Println("Modified Read function")

	modifyNewFunc(newFunc)
	log.Println("Modified New* function")

	// Delete the Process and Read functions, and insert the new functions
	dstutil.Apply(file, func(cursor *dstutil.Cursor) bool {
		node := cursor.Node()

		funcDecl, ok := node.(*dst.FuncDecl)
		if ok {
			name := funcDecl.Name.String()

			if name == "Process" || name == "Register" {
				cursor.Delete()
			}

			if name == newFunc.Name.String() {
				// Insert in reverse order
				cursor.InsertAfter(crudFunctions["Create"])
				cursor.InsertAfter(getSchemaFunc)
				cursor.InsertAfter(keyFunc)
			}

			if name == "Read" {
				// Insert in reverse order
				cursor.InsertAfter(crudFunctions["Delete"])
				cursor.InsertAfter(crudFunctions["Update"])
			}

			return false
		}

		return true
	}, nil)

	log.Println("Added functions to file and removed old ones")

	var buf bytes.Buffer
	restorer := decorator.NewRestorer()
	err = restorer.Fprint(&buf, file)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// Loop through file and find a function with a name containing the given string
func findFunc(file *dst.File, name string) *dst.FuncDecl {
	var processFunc *dst.FuncDecl
	for _, decl := range file.Decls {
		function, ok := decl.(*dst.FuncDecl)
		if !ok {
			continue
		}

		if strings.Contains(function.Name.Name, name) {
			processFunc = function
		}
	}
	return processFunc
}
