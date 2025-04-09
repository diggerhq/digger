package cli

import (
	"fmt"

	"github.com/go-substrate/strate/backend/version"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of st-server",
	Long:  `Display version information for the st-server application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("st-server version %s\n", version.Version)

		if showFull, _ := cmd.Flags().GetBool("full"); showFull {
			if version.Meta != "" {
				fmt.Printf("Git commit: %s\n", version.Meta)
			}

			if version.BuildDate != "" {
				fmt.Printf("Build date: %s\n", version.BuildDate)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	versionCmd.Flags().BoolP("full", "f", false, "Display full version information")
}
