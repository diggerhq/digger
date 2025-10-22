// +build ignore

package main

import (
	"io"
	"os"

	"ariga.io/atlas-provider-gorm/gormschema"
	"github.com/diggerhq/digger/opentaco/internal/query/types"
)

func main() {
	stmts, err := gormschema.New("sqlite").Load(types.DefaultModels...)
	if err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}
	io.WriteString(os.Stdout, stmts)
}

