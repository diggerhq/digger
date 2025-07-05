package git_utils

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

func createTempDir() string {
	tempDir, err := os.MkdirTemp("", "repo")
	if err != nil {
		slog.Error("Failed to create temporary directory", "error", err)
		panic(err)
	}
	return tempDir
}

type action func(string) error

func CloneGitRepoAndDoAction(repoUrl string, branch string, commitHash string, token string, tokenUsername string, action action) error {
	dir := createTempDir()

	slog.Debug("Cloning git repository",
		"repoUrl", repoUrl,
		"branch", branch,
		"commitHash", commitHash,
		"directory", dir,
	)

	git := NewGitShellWithTokenAuth(dir, token, tokenUsername)
	err := git.Clone(repoUrl, branch)
	if err != nil {
		slog.Error("Failed to clone repository",
			"repoUrl", repoUrl,
			"branch", branch,
			"error", err,
		)
		return err
	}

	if commitHash != "" {
		err := git.Checkout(commitHash)
		if err != nil {
			slog.Error("Failed to checkout commit",
				"commitHash", commitHash,
				"error", err,
			)
			return err
		}
	}

	defer func() {
		slog.Debug("Removing cloned directory", "directory", dir)
		ferr := os.RemoveAll(dir)
		if ferr != nil {
			slog.Warn("Failed to remove directory", "directory", dir, "error", ferr)
		}
	}()

	err = action(dir)
	if err != nil {
		slog.Error("Error performing action on repository", "directory", dir, "error", err)
		return err
	}

	return nil
}

type GitAuth struct {
	Username      string
	Password      string // Can be either password or access token
	TokenUsername string // if set will replace x-access-token (needed for bitbucket which uses x-token-auth)
	Token         string // x-access-token
}

type GitShell struct {
	workDir     string
	timeout     time.Duration
	environment []string
	auth        *GitAuth
}

func NewGitShell(workDir string, auth *GitAuth) *GitShell {
	env := os.Environ()

	// If authentication is provided, set up credential helper
	if auth != nil {
		// Add credential helper to avoid interactive password prompts
		env = append(env, "GIT_TERMINAL_PROMPT=0")
	}

	return &GitShell{
		workDir:     workDir,
		timeout:     30 * time.Second,
		environment: env,
		auth:        auth,
	}
}

func NewGitShellWithTokenAuth(workDir string, token string, tokenUsername string) *GitShell {
	auth := GitAuth{
		Username:      "x-access-token",
		Password:      "",
		TokenUsername: tokenUsername,
		Token:         token,
	}
	return NewGitShell(workDir, &auth)
}

// formatAuthURL injects credentials into the Git URL
func (g *GitShell) formatAuthURL(repoURL string) (string, error) {
	if g.auth == nil {
		return repoURL, nil
	}

	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %v", err)
	}

	// Handle different auth types
	if g.auth.Token != "" {
		// X-Access-Token authentication
		tokenUsername := g.auth.TokenUsername
		if tokenUsername == "" {
			tokenUsername = "x-access-token"
		}
		parsedURL.User = url.UserPassword(tokenUsername, g.auth.Token)
	} else if g.auth.Username != "" {
		// Username/password or personal access token
		parsedURL.User = url.UserPassword(g.auth.Username, g.auth.Password)
	}

	return parsedURL.String(), nil
}

func (g *GitShell) runCommand(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), g.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.workDir
	cmd.Env = g.environment

	// Set up credential helper for HTTPS
	if g.auth != nil {
		cmd.Env = append(cmd.Env, "GIT_ASKPASS=echo")
		if g.auth.Token != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_ACCESS_TOKEN=%s", g.auth.Token))
		}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("git command failed: %v: %s", err, stderr.String())
		}
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (g *GitShell) Checkout(branchOrCommit string) error {
	args := []string{"checkout"}
	args = append(args, branchOrCommit)
	_, err := g.runCommand(args...)
	return err
}

// Clone with authentication
func (g *GitShell) Clone(repoURL, branch string) error {
	authURL, err := g.formatAuthURL(repoURL)
	if err != nil {
		return err
	}

	args := []string{"clone"}
	if branch != "" {
		args = append(args, "-b", branch)
	}

	args = append(args, "--depth", "1")
	args = append(args, "--single-branch", authURL, g.workDir)

	_, err = g.runCommand(args...)
	return err
}
