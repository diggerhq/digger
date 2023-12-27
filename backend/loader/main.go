package loader

import (
	"ariga.io/atlas-provider-gorm/gormschema"
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"io"
	"os"
)

package main

import (
"io"
"os"

"ariga.io/atlas-provider-gorm/gormschema"
_ "ariga.io/atlas-provider-gorm/recordriver"
"github.com/<yourorg>/<yourrepo>/path/to/models"
)

func main() {
	stmts, err := gormschema.New("postgres").Load(
		&models.User{},
		&models.Pet{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load gorm schema: %v\n", err)
		os.Exit(1)
	}
	io.WriteString(os.Stdout, stmts)
}
