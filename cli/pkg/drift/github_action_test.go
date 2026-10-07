package drift

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteToGithubOutput(t *testing.T) {
	// Create a temporary file to act as our GITHUB_OUTPUT
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "github_output.txt")
	os.Setenv("GITHUB_OUTPUT", outputPath)
	defer os.Unsetenv("GITHUB_OUTPUT")

	key := "test_project_drift_message"
	value := "line1\nline2\nline3"

	err := writeToGithubOutput(key, value)
	if err != nil {
		t.Fatalf("writeToGithubOutput returned unexpected error: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	
	contentStr := string(content)

	if !strings.HasPrefix(contentStr, key+"<<") {
		t.Errorf("expected content to start with valid multiline key syntax")
	}

	if !strings.Contains(contentStr, value) {
		t.Errorf("expected content to contain the value")
	}
}

func TestSendNotificationForProjectOutput(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "github_output.txt")
	os.Setenv("GITHUB_OUTPUT", outputPath)
	defer os.Unsetenv("GITHUB_OUTPUT")

	notification := &GithubActionOutputNotification{}

	err := notification.SendNotificationForProject("projectA", "hello\nworld")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "projectA_drift_detected") {
		t.Errorf("missing drift detected key")
	}
	if !strings.Contains(contentStr, "projectA_drift_message") {
		t.Errorf("missing drift message key")
	}
	if !strings.Contains(contentStr, "hello\nworld") {
		t.Errorf("missing drift message content")
	}
}
