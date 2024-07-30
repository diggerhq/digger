package utils

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func ArchiveGitRepo(sourcePath string) (string, error) {
	// Generate a unique ID for the temp directory
	tempID := fmt.Sprintf("%d", time.Now().UnixNano())
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("temp_%s", tempID))

	// Create the temp directory
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Copy the source directory to the temp location
	if err := copyDir(sourcePath, tempDir); err != nil {
		return "", fmt.Errorf("failed to copy directory: %w", err)
	}

	// Initialize a new git repo in the copied location
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to initialize git repo: %w", err)
	}

	// Add all files to the git repo
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to add files to git repo: %w", err)
	}

	// Commit the files
	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to commit files: %w", err)
	}

	// Create a zip file using git archive
	zipFile := filepath.Join(os.TempDir(), fmt.Sprintf("archive_%s.zip", tempID))
	cmd = exec.Command("git", "archive", "--format=zip", "-o", zipFile, "HEAD")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create zip archive: %w", err)
	}

	return zipFile, nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
