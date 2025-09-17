package commands

import (
    "encoding/json"
    "errors"
    "fmt"
    "net/http"

    "github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
    Use:   "whoami",
    Short: "Show current identity and roles",
    RunE: func(cmd *cobra.Command, args []string) error {
        base := normalizedBase(serverURL)
        cf, err := loadCreds()
        if err != nil { return err }
        tok, ok := cf.Profiles[base]
        if !ok || tok.AccessToken == "" {
            // Fallback: if only one profile exists, use it
            if len(cf.Profiles) == 1 {
                for base2, t := range cf.Profiles {
                    tok = t
                    fmt.Fprintf(cmd.ErrOrStderr(), "[INFO] Using tokens for %s (only profile)\n", base2)
                    ok = true
                    break
                }
            }
            if !ok || tok.AccessToken == "" {
                return errors.New("not logged in; run 'taco login' first")
            }
        }
        req, _ := http.NewRequest("GET", base+"/v1/auth/me", nil)
        req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
        resp, err := http.DefaultClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode != 200 { return fmt.Errorf("HTTP %d", resp.StatusCode) }
        var data map[string]any
        if err := json.NewDecoder(resp.Body).Decode(&data); err != nil { return err }
        b, _ := json.MarshalIndent(data, "", "  ")
        fmt.Println(string(b))
        return nil
    },
}

func init() { rootCmd.AddCommand(whoamiCmd) }
