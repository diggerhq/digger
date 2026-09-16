package drift

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
)

type GithubActionOutputNotification struct{}

func writeToGithubOutput(key, value string) error {
	outputPath := os.Getenv("GITHUB_OUTPUT")
	if outputPath == "" {
		slog.Info(fmt.Sprintf("GITHUB_OUTPUT is not set. Would have written: %s=%s", key, value))
		return nil
	}

	f, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open GITHUB_OUTPUT file: %w", err)
	}
	defer f.Close()

	// Generate a random delimiter for multi-line support
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	delimiter := hex.EncodeToString(b)

	content := fmt.Sprintf("%s<<%s\n%s\n%s\n", key, delimiter, value, delimiter)

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("failed to write to GITHUB_OUTPUT file: %w", err)
	}
	return nil
}

func (n *GithubActionOutputNotification) SendNotificationForProject(projectId string, driftMessage string) error {
	if err := writeToGithubOutput(fmt.Sprintf("%s_drift_detected", projectId), "true"); err != nil {
		return err
	}
	if err := writeToGithubOutput(fmt.Sprintf("%s_drift_message", projectId), driftMessage); err != nil {
		return err
	}
	return nil
}

func (n *GithubActionOutputNotification) SendErrorNotificationForProject(projectId string, errorMsg string) error {
	if err := writeToGithubOutput(fmt.Sprintf("%s_drift_error", projectId), errorMsg); err != nil {
		return err
	}
	return nil
}
