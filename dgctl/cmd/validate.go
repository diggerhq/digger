/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/spf13/cobra"
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a digger.yml file",
	Long:  `Validate the structure and contents of a digger.yml file.`,
	Run: func(cmd *cobra.Command, args []string) {
		configPath := "./"
		log.Printf("Starting validation of digger.yml in path: %s", configPath)

		// Load the Digger config file
		_, configYaml, _, err := digger_config.LoadDiggerConfig(configPath, true, nil)
		if err != nil {
			log.Printf("Error loading digger.yml: %v", err)
			fmt.Fprintf(os.Stderr, "Invalid digger config file: %v\n", err)
			os.Exit(1)
		}

		// Log success message
		log.Printf("digger.yml loaded successfully.")

		// Display the configuration in a pretty JSON format
		prettyConfig, err := json.MarshalIndent(configYaml, "", "\t")
		if err != nil {
			log.Printf("Error formatting digger.yml: %v", err)
			fmt.Fprintf(os.Stderr, "Error formatting digger config file: %v\n", err)
			os.Exit(1)
		}

		// Print the formatted configuration
		fmt.Println("Configuration:")
		fmt.Println(string(prettyConfig))
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)

	// Optionally define any flags that you want to use with the command here
	// e.g.:
	// validateCmd.Flags().StringP("config", "c", "", "Specify custom config file path")
}
