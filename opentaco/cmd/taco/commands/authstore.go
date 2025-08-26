package commands

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/diggerhq/digger/opentaco/pkg/sdk"
)

type credsFile struct {
    Profiles map[string]tokens `json:"profiles"`
}

type tokens struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
}

func configDir() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil { return "", err }
    dir := filepath.Join(home, ".config", "opentaco")
    if err := os.MkdirAll(dir, 0o755); err != nil { return "", err }
    return dir, nil
}

func credsPath() (string, error) {
    dir, err := configDir()
    if err != nil { return "", err }
    return filepath.Join(dir, "credentials.json"), nil
}

func loadCreds() (*credsFile, error) {
    path, err := credsPath()
    if err != nil { return nil, err }
    b, err := os.ReadFile(path)
    if errors.Is(err, os.ErrNotExist) {
        return &credsFile{Profiles: map[string]tokens{}}, nil
    }
    if err != nil { return nil, err }
    var f credsFile
    if err := json.Unmarshal(b, &f); err != nil { return nil, err }
    if f.Profiles == nil { f.Profiles = map[string]tokens{} }
    return &f, nil
}

func saveCreds(cf *credsFile) error {
    path, err := credsPath()
    if err != nil { return err }
    b, err := json.MarshalIndent(cf, "", "  ")
    if err != nil { return err }
    return os.WriteFile(path, b, 0o600)
}

func normalizedBase(base string) string {
    return strings.TrimRight(base, "/")
}

func newAuthedClient() *sdk.Client {
    base := normalizedBase(serverURL)
    c := sdk.NewClient(base)
    cf, err := loadCreds()
    if err == nil {
        if t, ok := cf.Profiles[base]; ok {
            if t.AccessToken != "" { c.SetBearerToken(t.AccessToken); return c }
        }
        // Fallback: if only one profile exists, use it
        if len(cf.Profiles) == 1 {
            for _, t := range cf.Profiles {
                if t.AccessToken != "" { c.SetBearerToken(t.AccessToken) }
            }
        }
    }
    return c
}
