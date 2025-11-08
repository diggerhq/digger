package service_clients

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
)

type LocalExecJobClient struct {}

// TriggerProjectsRefreshLocal starts a local binary with the required environment.
// Binary path is taken from PROJECTS_REFRESH_BIN (fallback: ./projects-refresh-service).
// It does NOT wait for completion; it returns as soon as the process starts successfully.
func (f LocalExecJobClient) TriggerProjectsRefreshService(
	cloneUrl, branch, githubToken, repoFullName, orgId string,
) (*BackgroundJobTriggerResponse, error) {

	slog.Debug("starting local projects-refresh-service job")

	// Resolve binary path from env or default.
	bin := os.Getenv("PROJECTS_REFRESH_BIN")
	if bin == "" {
		bin = "../../background/projects-refresh-service/projects_refesh_main"
	}

	// Optional: working directory (set via env if you want), otherwise current dir.
	workingDir := os.Getenv("PROJECTS_REFRESH_WORKDIR")
	if workingDir == "" {
		wd, _ := os.Getwd()
		workingDir = wd
	}

	// Build environment for the child process.
	// Keep existing env and append required vars.
	env := append(os.Environ(),
		"DIGGER_GITHUB_REPO_CLONE_URL="+cloneUrl,
		"DIGGER_GITHUB_REPO_CLONE_BRANCH="+branch,
		"DIGGER_GITHUB_TOKEN="+githubToken,
		"DIGGER_REPO_FULL_NAME="+repoFullName,
		"DIGGER_ORG_ID="+orgId,
		"DATABASE_URL="+os.Getenv("DATABASE_URL"),
	)

	// Optional: add any tuning flags you previously used in the container world.
	// env = append(env, "GODEBUG=off", "GOFIPS140=off")

	// If your binary needs args, add them here. Empty for now.
	cmd := exec.Command(bin)
	cmd.Dir = workingDir
	cmd.Env = env

	// Pipe stdout/stderr to slog for observability.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Error("allocating stdout pipe failed", "error", err)
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		slog.Error("allocating stderr pipe failed", "error", err)
		return nil, err
	}

	// Start process.
	if err := cmd.Start(); err != nil {
		slog.Error("failed to start local job", "binary", bin, "dir", workingDir, "error", err)
		return nil, err
	}

	// Stream logs in background goroutines tied to a short-lived context so we don't leak.
	ctx, cancel := context.WithCancel(context.Background())
	go pipeToSlog(ctx, stdout, slog.LevelInfo, "projects-refresh")
	go pipeToSlog(ctx, stderr, slog.LevelError, "projects-refresh")

	// Optionally, you can watch for process exit in a goroutine if you want to log completion.
	go func() {
		defer cancel()
		waitErr := cmd.Wait()
		if waitErr != nil {
			slog.Error("local job exited with error", "pid", cmd.Process.Pid, "error", waitErr)
			return
		}
		slog.Info("local job completed", "pid", cmd.Process.Pid)
	}()

	slog.Debug("triggered local projects refresh", "pid", cmd.Process.Pid, "binary", bin, "workdir", workingDir)

	return &BackgroundJobTriggerResponse{ID: fmt.Sprintf("%d", cmd.Process.Pid)}, nil
}

// pipeToSlog streams a reader line-by-line into slog at the given level.
func pipeToSlog(ctx context.Context, r io.Reader, level slog.Level, comp string) {
	br := bufio.NewScanner(r)
	// Increase the Scanner buffer in case the tool emits long lines.
	buf := make([]byte, 0, 64*1024)
	br.Buffer(buf, 10*1024*1024)

	for br.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			slog.Log(context.Background(), level, br.Text(), "component", comp)
		}
	}
	if err := br.Err(); err != nil {
		slog.Error("log stream error", "component", comp, "error", err)
	}
}
