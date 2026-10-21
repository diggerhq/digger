package generic

import (
	"fmt"
	"os"
)

func IsForgejoActions() bool {
	return os.Getenv("FORGEJO_ACTIONS") == "true" || os.Getenv("GITEA_ACTIONS") == "true"
}

func AppendToEnvFile(key string, value string) error {
	envFile := os.Getenv("GITHUB_ENV")
	if envFile == "" {
		return fmt.Errorf("GITHUB_ENV not set, could not set the output in digger step")
	}
	f, err := os.OpenFile(envFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("could not open %s for writing: %w", envFile, err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%v=%v\n", key, value)
	if err != nil {
		return fmt.Errorf("could not write to %s: %w", envFile, err)
	}
	return nil
}
