/*
Copyright © 2024 diggerhq

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/invopop/jsonschema"
	"github.com/spf13/cobra"
)

var outputFile string

// schemaCmd represents the schema command
var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Generate JSON Schema for digger.yml",
	Long: `Generate a JSON Schema from the digger.yml configuration structure.

Examples:
  dgctl schema                    # Output schema to stdout
  dgctl schema -o schema.json     # Write schema to file
  dgctl schema --output docs/schema.json`,
	Run: func(cmd *cobra.Command, args []string) {
		schema := generateSchema()

		jsonBytes, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling schema: %v\n", err)
			os.Exit(1)
		}

		if outputFile != "" {
			err = os.WriteFile(outputFile, jsonBytes, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing schema to file: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Schema written to %s\n", outputFile)
		} else {
			fmt.Println(string(jsonBytes))
		}
	},
}

func generateSchema() *jsonschema.Schema {
	r := &jsonschema.Reflector{
		KeyNamer:                   jsonschema.ToSnakeCase,
		RequiredFromJSONSchemaTags: true,
		AllowAdditionalProperties:  true,
		DoNotReference:             true,
		Namer: func(t reflect.Type) string {
			return t.Name()
		},
	}

	schema := r.Reflect(&digger_config.DiggerConfigYaml{})
	schema.ID = jsonschema.ID("https://digger.dev/schemas/digger.yml.json")

	return schema
}

func init() {
	rootCmd.AddCommand(schemaCmd)
	schemaCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path (default: stdout)")
}
