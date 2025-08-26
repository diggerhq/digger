package commands

import (
    "fmt"

    "github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
    Use:   "logout",
    Short: "Remove saved tokens for the current server",
    RunE: func(cmd *cobra.Command, args []string) error {
        base := normalizedBase(serverURL)
        cf, err := loadCreds()
        if err != nil { return err }
        if _, ok := cf.Profiles[base]; !ok {
            fmt.Printf("No saved tokens for %s\n", base)
            return nil
        }
        delete(cf.Profiles, base)
        if err := saveCreds(cf); err != nil { return err }
        fmt.Printf("Logged out from %s (tokens removed)\n", base)
        return nil
    },
}

func init() { rootCmd.AddCommand(logoutCmd) }

