package commands

import (
    "fmt"

    "github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
    Use:   "login",
    Short: "Authenticate via OIDC (PKCE)",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Println("taco login: not implemented yet (OIDC PKCE stub)")
        return nil
    },
}

func init() { rootCmd.AddCommand(loginCmd) }

