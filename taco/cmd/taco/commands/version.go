package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Long:  `Print the version and build information for the taco CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("taco CLI version %s (commit: %s)\n", Version, Commit)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
