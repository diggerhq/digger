// +build ignore

package main

import (
	"fmt"
	"io"
	"os"

	"ariga.io/atlas-provider-gorm/gormschema"
	"github.com/diggerhq/digger/opentaco/internal/query/types"
)

func main() {
	// Default to sqlite if no argument provided
	dialect := "sqlite"
	if len(os.Args) > 1 {
		dialect = os.Args[1]
	}

	stmts, err := gormschema.New(dialect).Load(types.DefaultModels...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading schema for %s: %v\n", dialect, err)
		os.Exit(1)
	}
	io.WriteString(os.Stdout, stmts)
}

