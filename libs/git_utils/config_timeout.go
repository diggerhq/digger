package git_utils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// RepositoryFileReader reads a file at a branch or commit through the provider API.
// Missing files must return an error wrapping os.ErrNotExist.
type RepositoryFileReader func(ctx context.Context, path, ref string) ([]byte, error)

// CloneGitRepoAndDoActionWithConfig reads git_timeout before starting the clone.
func CloneGitRepoAndDoActionWithConfig(repoURL, branch, commitHash, token, tokenUsername string, readFile RepositoryFileReader, action action) error {
	ref := branch
	if commitHash != "" {
		ref = commitHash
	}
	timeout, err := readGitTimeout(readFile, ref)
	if err != nil {
		return err
	}
	return CloneGitRepoAndDoActionWithTimeout(repoURL, branch, commitHash, token, tokenUsername, timeout, action)
}

func readGitTimeout(readFile RepositoryFileReader, ref string) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultGitTimeout)
	defer cancel()
	for _, path := range []string{"digger.yml", "digger.yaml"} {
		contents, err := readFile(ctx, path, ref)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("read %s before cloning: %w", path, err)
		}
		// Project generation needs a checkout; only decode the timeout here.
		var config struct {
			GitTimeout int64 `yaml:"git_timeout"`
		}
		if err := yaml.Unmarshal(contents, &config); err != nil {
			return 0, fmt.Errorf("parse %s before cloning: %w", path, err)
		}
		if config.GitTimeout <= 0 {
			return defaultGitTimeout, nil
		}
		if config.GitTimeout > int64((1<<63-1)/time.Second) {
			return 0, fmt.Errorf("git_timeout in %s is too large", path)
		}
		return time.Duration(config.GitTimeout) * time.Second, nil
	}
	return defaultGitTimeout, nil
}
