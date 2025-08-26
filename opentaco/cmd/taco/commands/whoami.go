package commands

import (
    "fmt"

    "github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
    Use:   "whoami",
    Short: "Show current identity and roles",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Println("anonymous (roles: [], groups: [])")
        return nil
    },
}

func init() { rootCmd.AddCommand(whoamiCmd) }

