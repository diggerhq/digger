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
	Long:  `Validate a digger.yml file`,
	Run: func(cmd *cobra.Command, args []string) {
		_, configYaml, _, err := digger_config.LoadDiggerConfig("./", true)
		if err != nil {
			log.Printf("Invalid digger config file: %v. Exiting.", err)
			os.Exit(1)
		}
		log.Printf("digger.yml loaded successfully, here is your configuration:")
		s, _ := json.MarshalIndent(configYaml, "", "\t")
		fmt.Println(string(s))
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// validateCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// validateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
