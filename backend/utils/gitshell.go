package utils

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

type GitAuth struct {
	Username string
	Password string // Can be either password or access token
	Token    string // x-access-token
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

func NewGitShellWithTokenAuth(workDir string, token string) *GitShell {
	auth := GitAuth{
		Username: "x-access-token",
		Password: "",
		Token:    token,
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
		parsedURL.User = url.UserPassword("x-access-token", g.auth.Token)
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
